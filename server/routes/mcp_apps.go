package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
)

const (
	appsMime          = "text/html;profile=mcp-app"
	appsOpenerTool    = "muster_open_view"
	appsMaxHTMLBytes  = 2 << 20
	legacyCoreVersion = "2025-03-26"
	newestCoreVersion = "2025-11-25"
)

// supportedCoreVersions are the core MCP revisions this route speaks. Nothing is remembered between requests.
var supportedCoreVersions = []string{"2025-03-26", "2025-06-18", "2025-11-25"}

func isSupportedCoreVersion(v string) bool {
	for _, s := range supportedCoreVersions {
		if s == v {
			return true
		}
	}
	return false
}

// negotiateCoreVersion echoes a supported requested version, otherwise offers the newest.
func negotiateCoreVersion(requested string) string {
	if isSupportedCoreVersion(requested) {
		return requested
	}
	return newestCoreVersion
}

// requestCoreVersion reads the per-request header. Absent means the legacy default.
func requestCoreVersion(c *gin.Context) (string, bool) {
	v := c.GetHeader("MCP-Protocol-Version")
	if v == "" {
		return legacyCoreVersion, true
	}
	return v, isSupportedCoreVersion(v)
}

// coreHasStructuredOutput is true for revisions that define structuredContent.
func coreHasStructuredOutput(v string) bool { return v != legacyCoreVersion }

// McpAppsOptions configures the optional App. The zero value means Apps are off.
type McpAppsOptions struct {
	Enabled bool
	// LoadHTML returns the bundled single-file App. Tests inject bytes here.
	LoadHTML func() ([]byte, error)
}

// McpAppsFromEnv reads MCP_APPS_ENABLED (default off) and loads the bundle from distDir/mcp-app/index.html.
func McpAppsFromEnv(distDir string) McpAppsOptions {
	return McpAppsOptions{
		Enabled: os.Getenv("MCP_APPS_ENABLED") == "true",
		LoadHTML: func() ([]byte, error) {
			return os.ReadFile(distDir + "/mcp-app/index.html")
		},
	}
}

type appAsset struct {
	uri  string
	html string
}

var externalRef = regexp.MustCompile(`(?i)<script[^>]*\ssrc\s*=|<link[^>]*\shref\s*=\s*["']?(https?:)?//|\s(src|href)\s*=\s*["']?(https?:)?//|@import|url\(\s*["']?(https?:)?//`)

// LoadMcpApp validates the bundle and derives its hash URI. It returns (nil, nil) when Apps are off.
// Callers use the error to fail readiness when Apps are on and the bundle is unusable.
func LoadMcpApp(opts McpAppsOptions) (*appAsset, error) {
	if !opts.Enabled {
		return nil, nil
	}
	if opts.LoadHTML == nil {
		return nil, errors.New("mcp apps enabled but no bundle loader")
	}
	raw, err := opts.LoadHTML()
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw) > appsMaxHTMLBytes {
		return nil, errors.New("mcp app bundle is empty or too large")
	}
	html := string(raw)
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(html)), "<!doctype html") {
		return nil, errors.New("mcp app bundle is not an HTML document")
	}
	if externalRef.MatchString(html) {
		return nil, errors.New("mcp app bundle references an external resource")
	}
	sum := sha256.Sum256(raw)
	return &appAsset{uri: "ui://muster/view/" + hex.EncodeToString(sum[:8]) + ".html", html: html}, nil
}

func (a *appAsset) resourceListing() gin.H {
	return gin.H{
		"uri":         a.uri,
		"name":        "Muster view",
		"description": "Read-only Muster App.",
		"mimeType":    appsMime,
	}
}

func (a *appAsset) contents() gin.H {
	return gin.H{
		"uri":      a.uri,
		"mimeType": appsMime,
		"text":     a.html,
		"_meta": gin.H{"ui": gin.H{
			"prefersBorder": true,
			"csp": gin.H{
				"connectDomains": []string{}, "resourceDomains": []string{},
				"frameDomains": []string{}, "baseUriDomains": []string{},
			},
		}},
	}
}

// withAppsVisibility marks the legacy tools model-only and appends the opener.
func withAppsVisibility(tools []gin.H, a *appAsset) []gin.H {
	for _, t := range tools {
		t["_meta"] = gin.H{"ui": gin.H{"visibility": []string{"model"}}}
	}
	return append(tools, openerDef(a))
}

func openerDef(a *appAsset) gin.H {
	return gin.H{
		"name":        appsOpenerTool,
		"title":       "Open Muster view",
		"description": "Open the read-only Muster view for a workflow. Shows text only in hosts that cannot render Apps. Changes nothing.",
		"annotations": gin.H{"readOnlyHint": true, "idempotentHint": true, "destructiveHint": false, "openWorldHint": false},
		"inputSchema": gin.H{
			"type":                 "object",
			"additionalProperties": false,
			"properties": gin.H{
				"workflow": gin.H{"type": "string", "enum": []string{"pick-a-time", "answer", "run-a-group"}},
				"event_id": gin.H{"type": "string", "minLength": 1, "pattern": `\S`, "description": "Canonical or short ID. Required for pick-a-time and answer."},
				"window": gin.H{"oneOf": []gin.H{
					{"type": "null"},
					{
						"type": "object", "additionalProperties": false,
						"properties": gin.H{
							"start": gin.H{"type": "string", "format": "date-time"},
							"end":   gin.H{"type": "string", "format": "date-time"},
						},
						"required": []string{"start", "end"},
					},
				}, "description": "Null or omitted lets the server choose. start must be before end, at most 7 days apart."},
				"duration_minutes": gin.H{"type": []string{"integer", "null"}, "minimum": 1},
				"people_cursor":    gin.H{"type": []string{"string", "null"}},
			},
			"required": []string{"workflow"},
			"allOf": []gin.H{{
				"if":   gin.H{"properties": gin.H{"workflow": gin.H{"enum": []string{"pick-a-time", "answer"}}}, "required": []string{"workflow"}},
				"then": gin.H{"properties": gin.H{"event_id": gin.H{"type": "string"}}, "required": []string{"event_id"}},
			}},
		},
		"_meta": gin.H{"ui": gin.H{"resourceUri": a.uri, "visibility": []string{"model", "app"}}},
	}
}

type openerArgs struct {
	workflow, eventID string
	start, end        *time.Time
	duration          *int
	cursor            string
}

// parseOpenerArgs enforces opener-input.schema.json plus the checks a schema cannot express.
func parseOpenerArgs(args map[string]any) (*openerArgs, *mcp.ToolError) {
	bad := func(msg string) (*openerArgs, *mcp.ToolError) {
		return nil, mcpErr("BAD_ARGUMENTS", msg, "Fix the arguments and call again.")
	}
	for k := range args {
		switch k {
		case "workflow", "event_id", "window", "duration_minutes", "people_cursor":
		default:
			return bad("Unknown argument: " + safeKey(k) + ".")
		}
	}
	wf, _ := args["workflow"].(string)
	if wf != "pick-a-time" && wf != "answer" && wf != "run-a-group" {
		return bad("workflow must be pick-a-time, answer, or run-a-group.")
	}
	out := &openerArgs{workflow: wf}
	if raw, present := args["event_id"]; present {
		id, ok := raw.(string)
		if !ok || strings.TrimSpace(id) == "" {
			return bad("event_id must be a non-empty string when given.")
		}
		out.eventID = id
	} else if wf != "run-a-group" {
		return bad("event_id is required for " + wf + ".")
	}
	if raw, present := args["window"]; present && raw != nil {
		w, ok := raw.(map[string]any)
		if !ok || len(w) != 2 {
			return bad("window must be null or an object with start and end.")
		}
		s, sok := w["start"].(string)
		e, eok := w["end"].(string)
		if !sok || !eok {
			return bad("window.start and window.end must be RFC3339 strings.")
		}
		st, err1 := time.Parse(time.RFC3339, s)
		en, err2 := time.Parse(time.RFC3339, e)
		if err1 != nil || err2 != nil {
			return bad("window.start and window.end must be RFC3339 timestamps.")
		}
		if !st.Before(en) || en.Sub(st) > 7*24*time.Hour {
			return bad("window.start must be before window.end, at most 7 days apart.")
		}
		out.start, out.end = &st, &en
	}
	if raw, present := args["duration_minutes"]; present && raw != nil {
		f, ok := raw.(float64)
		if !ok || f != math.Trunc(f) || f < 1 {
			return bad("duration_minutes must be a whole number of at least 1.")
		}
		n := int(f)
		out.duration = &n
	}
	if raw, present := args["people_cursor"]; present && raw != nil {
		c, ok := raw.(string)
		if !ok {
			return bad("people_cursor must be a string or null.")
		}
		out.cursor = c
	}
	return out, nil
}

func safeKey(k string) string {
	if len(k) > 40 {
		k = k[:40]
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, k)
}

func mcpBaseURL() string {
	if b := strings.TrimRight(os.Getenv("BASE_URL"), "/"); b != "" {
		return b
	}
	return "https://muster.asymbl.app"
}

// openView handles muster_open_view. With event_id it returns an authorized snapshot built from
// the same stores and rules as the text tools. Without one (run-a-group only) it returns an
// unsaved creation draft. It never writes.
func openView(deps mcp.Deps, user *models.User, args map[string]any, core string) (gin.H, *mcp.ToolError) {
	in, toolErr := parseOpenerArgs(args)
	if toolErr != nil {
		return nil, toolErr
	}
	snap, toolErr := mcp.BuildSnapshot(deps, user, mcp.SnapshotRequest{
		Workflow: in.workflow, EventID: in.eventID,
		WindowStart: in.start, WindowEnd: in.end,
		DurationMinutes: in.duration, PeopleCursor: in.cursor,
	})
	if toolErr != nil {
		return nil, toolErr
	}
	link := mcpBaseURL()
	if in.eventID != "" {
		link += "/e/" + url.PathEscape(in.eventID)
	}
	summary := snap.Summary()
	text := summary + " This view is read-only. Use the Muster text tools, or open " + link + " in a browser."
	result := gin.H{
		"content": []gin.H{{"type": "text", "text": text}},
		"isError": false,
		"_meta":   gin.H{"muster": snap},
	}
	if coreHasStructuredOutput(core) {
		sc := gin.H{"schema_version": 1, "workflow": in.workflow, "read_only": true, "summary": summary}
		if snap.Event != nil {
			sc["event_id"] = snap.Event.ID
		}
		result["structuredContent"] = sc
	}
	return result, nil
}

func writeAppsResult(c *gin.Context, id any, result gin.H) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": id, "result": result})
}
