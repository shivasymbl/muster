package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/middleware"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const testAppHTML = "<!doctype html><html><body>app</body></html>"

func appsEngine(deps mcp.Deps, opts McpAppsOptions) *gin.Engine {
	r := gin.New()
	InitMcp(r, true, deps, opts)
	return r
}

func postWithVersion(r http.Handler, body, version string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer muster_live")
	if version != "" {
		req.Header.Set("MCP-Protocol-Version", version)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestMCPAppsProtocol(t *testing.T) {
	middleware.ResetMCPLimits()
	owner := primitive.NewObjectID()
	caller := &models.User{Id: owner, Email: "owner@x.com"}
	deps := mcp.Deps{
		Tokens: &fakeTokens{active: map[string]*models.McpToken{"muster_live": {UserID: owner}}},
		Users:  &fakeUsers{byID: map[string]*models.User{owner.Hex(): caller}},
		Events: &fakeEvents{},
	}
	on := McpAppsOptions{Enabled: true, LoadHTML: func() ([]byte, error) { return []byte(testAppHTML), nil }}
	r := appsEngine(deps, on)

	list := func(t *testing.T, eng http.Handler) []any {
		t.Helper()
		res := decode(t, postWithVersion(eng, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, ""))["result"].(map[string]any)
		return res["tools"].([]any)
	}
	findTool := func(tools []any, name string) map[string]any {
		for _, x := range tools {
			if m := x.(map[string]any); m["name"] == name {
				return m
			}
		}
		return nil
	}
	appUI := func(t *testing.T) string {
		opener := findTool(list(t, r), "muster_open_view")
		if opener == nil {
			t.Fatal("opener missing")
		}
		return opener["_meta"].(map[string]any)["ui"].(map[string]any)["resourceUri"].(string)
	}

	t.Run("A1 opener, resource and metadata agree", func(t *testing.T) {
		tools := list(t, r)
		opener := findTool(tools, "muster_open_view")
		ann := opener["annotations"].(map[string]any)
		if ann["readOnlyHint"] != true || ann["destructiveHint"] != false {
			t.Fatal(ann)
		}
		ui := opener["_meta"].(map[string]any)["ui"].(map[string]any)
		uri := ui["resourceUri"].(string)
		if !strings.HasPrefix(uri, "ui://muster/view/") || !strings.HasSuffix(uri, ".html") {
			t.Fatal(uri)
		}
		if vis := ui["visibility"].([]any); len(vis) != 2 || vis[0] != "model" || vis[1] != "app" {
			t.Fatal(vis)
		}
		for _, name := range []string{"muster_whoami", "muster_set_availability", "muster_get_event"} {
			vis := findTool(tools, name)["_meta"].(map[string]any)["ui"].(map[string]any)["visibility"].([]any)
			if len(vis) != 1 || vis[0] != "model" {
				t.Fatalf("%s visibility %v", name, vis)
			}
		}
		for _, x := range tools {
			if n := x.(map[string]any)["name"].(string); strings.HasPrefix(n, "muster_ui_") {
				t.Fatalf("write helper %s advertised", n)
			}
		}
		read := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"`+uri+`"}}`, ""))
		c := read["result"].(map[string]any)["contents"].([]any)[0].(map[string]any)
		if c["mimeType"] != "text/html;profile=mcp-app" || c["text"] != testAppHTML || c["uri"] != uri {
			t.Fatal(c)
		}
		csp := c["_meta"].(map[string]any)["ui"].(map[string]any)["csp"].(map[string]any)
		if len(csp["connectDomains"].([]any)) != 0 || len(csp["resourceDomains"].([]any)) != 0 {
			t.Fatal(csp)
		}
		listed := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":3,"method":"resources/list"}`, ""))["result"].(map[string]any)["resources"].([]any)
		if len(listed) != 4 {
			t.Fatalf("want 3 skills + app, got %d", len(listed))
		}
	})

	t.Run("A2 text fallback, no blank widget, no mutation", func(t *testing.T) {
		res := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"muster_open_view","arguments":{"workflow":"pick-a-time","event_id":"abc"}}}`, "2025-03-26"))["result"].(map[string]any)
		text := res["content"].([]any)[0].(map[string]any)["text"].(string)
		if res["isError"] != false || !strings.Contains(text, "/e/abc") || !strings.Contains(text, "read-only") {
			t.Fatal(res)
		}
		// Result and meta stay separate: _meta is a sibling, not a wrapper around the legacy fields.
		if res["_meta"].(map[string]any)["muster"] == nil || res["_meta"].(map[string]any)["content"] != nil {
			t.Fatal(res["_meta"])
		}
		if _, has := res["structuredContent"]; has {
			t.Fatal("old core must not get structuredContent from the opener")
		}
		newer := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"muster_open_view","arguments":{"workflow":"run-a-group"}}}`, "2025-11-25"))
		if newer["id"] != float64(5) || newer["result"].(map[string]any)["structuredContent"] == nil {
			t.Fatal(newer)
		}
	})

	t.Run("opener input is validated", func(t *testing.T) {
		for name, args := range map[string]string{
			"no workflow":       `{}`,
			"bad workflow":      `{"workflow":"x","event_id":"a"}`,
			"missing event":     `{"workflow":"answer"}`,
			"null event":        `{"workflow":"run-a-group","event_id":null}`,
			"blank event":       `{"workflow":"answer","event_id":"  "}`,
			"extra arg":         `{"workflow":"run-a-group","token":"s"}`,
			"window reversed":   `{"workflow":"answer","event_id":"a","window":{"start":"2026-01-02T00:00:00Z","end":"2026-01-01T00:00:00Z"}}`,
			"window over 7d":    `{"workflow":"answer","event_id":"a","window":{"start":"2026-01-01T00:00:00Z","end":"2026-01-09T00:00:00Z"}}`,
			"fractional length": `{"workflow":"answer","event_id":"a","duration_minutes":1.5}`,
			"zero length":       `{"workflow":"answer","event_id":"a","duration_minutes":0}`,
			"cursor number":     `{"workflow":"answer","event_id":"a","people_cursor":3}`,
		} {
			res := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"muster_open_view","arguments":`+args+`}}`, ""))["result"].(map[string]any)
			if res["isError"] != true {
				t.Fatalf("%s accepted: %v", name, res)
			}
		}
		ok := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"muster_open_view","arguments":{"workflow":"answer","event_id":"a","window":null,"duration_minutes":null,"people_cursor":null}}}`, ""))["result"].(map[string]any)
		if ok["isError"] != false {
			t.Fatal(ok)
		}
	})

	t.Run("version negotiation and header isolation", func(t *testing.T) {
		for in, want := range map[string]string{"2025-03-26": "2025-03-26", "2025-06-18": "2025-06-18", "2025-11-25": "2025-11-25", "1999-01-01": "2025-11-25", "": "2025-11-25"} {
			body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` + in + `"}}`
			got := decode(t, postWithVersion(r, body, "bogus-ignored-on-initialize"))["result"].(map[string]any)["protocolVersion"]
			if got != want {
				t.Fatalf("%q -> %v", in, got)
			}
		}
		bad := postWithVersion(r, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "2024-01-01")
		if bad.Code != http.StatusBadRequest {
			t.Fatalf("status %d", bad.Code)
		}
		// A request after an initialize at a newer version must not inherit it.
		postWithVersion(r, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`, "")
		res := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"muster_open_view","arguments":{"workflow":"run-a-group"}}}`, ""))["result"].(map[string]any)
		if _, has := res["structuredContent"]; has {
			t.Fatal("version leaked across requests")
		}
	})

	t.Run("resource auth, missing and unknown URI", func(t *testing.T) {
		uri := appUI(t)
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"`+uri+`"}}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "<!doctype") {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		for _, params := range []string{`{}`, `{"uri":"ui://muster/view/nope.html"}`, `{"uri":"ui://muster/view/"}`, `{"uri":"` + uri + `x"}`} {
			rec := postWithVersion(r, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":`+params+`}`, "")
			e := decode(t, rec)["error"].(map[string]any)
			if e["code"] != float64(-32002) || strings.Contains(rec.Body.String(), "<html") {
				t.Fatalf("%s -> %s", params, rec.Body.String())
			}
		}
	})

	t.Run("Markdown skills and legacy tools unchanged", func(t *testing.T) {
		rec := postWithVersion(r, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"muster://skills/answer"}}`, "")
		c := decode(t, rec)["result"].(map[string]any)["contents"].([]any)[0].(map[string]any)
		if c["mimeType"] != "text/markdown" || c["text"] == "" {
			t.Fatal(c)
		}
		res := decode(t, postWithVersion(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_whoami"}}`, "2025-03-26"))["result"].(map[string]any)
		if res["structuredContent"] == nil || res["isError"] != false {
			t.Fatalf("legacy structuredContent changed: %v", res)
		}
	})

	t.Run("legacy fallback when apps are off or the bundle is bad", func(t *testing.T) {
		bad := McpAppsOptions{Enabled: true, LoadHTML: func() ([]byte, error) {
			return []byte(`<!doctype html><script src="https://cdn.example/x.js"></script>`), nil
		}}
		for name, eng := range map[string]*gin.Engine{
			"off":     appsEngine(deps, McpAppsOptions{}),
			"no opts": engine(deps, true),
			"bad":     appsEngine(deps, bad),
		} {
			tools := list(t, eng)
			if findTool(tools, "muster_open_view") != nil || findTool(tools, "muster_whoami")["_meta"] != nil {
				t.Fatalf("%s advertised an unusable opener", name)
			}
			res := decode(t, postWithVersion(eng, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_open_view","arguments":{"workflow":"run-a-group"}}}`, ""))["result"].(map[string]any)
			if res["isError"] != true {
				t.Fatalf("%s ran the opener", name)
			}
		}
		if _, err := LoadMcpApp(bad); err == nil {
			t.Fatal("bad bundle passed readiness")
		}
		if _, err := LoadMcpApp(McpAppsOptions{Enabled: true}); err == nil {
			t.Fatal("missing loader passed readiness")
		}
		if a, err := LoadMcpApp(McpAppsOptions{}); a != nil || err != nil {
			t.Fatal("disabled must be a quiet no-op")
		}
	})

	t.Run("GET 401, 405 and 429", func(t *testing.T) {
		middleware.ResetMCPLimits()
		get := func(auth string) int {
			req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			return rec.Code
		}
		if got := get("Bearer muster_live"); got != http.StatusMethodNotAllowed {
			t.Fatal(got)
		}
		var last int
		for i := 0; i < 40; i++ {
			last = get("Bearer wrong")
			if i == 0 && last != http.StatusUnauthorized {
				t.Fatal(last)
			}
		}
		if last != http.StatusTooManyRequests {
			t.Fatalf("GET failures never blocked: %d", last)
		}
		middleware.ResetMCPLimits()
	})
}
