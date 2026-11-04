package mcp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/middleware"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/routes"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// These subtests need the wire layer (tools/list, instructions, skills, error text), so they live in the
// external test package and call the real route over httptest with in-memory fakes. They are named like the
// matching TestMCPTextParity and TestMCPFieldFindings subtests; a -run filter on those two names selects both.

type catTokens struct{ user primitive.ObjectID }

func (c catTokens) Create(*models.User, string) (string, models.McpTokenPublic, error) {
	return "", models.McpTokenPublic{}, nil
}
func (c catTokens) List(primitive.ObjectID) ([]models.McpTokenPublic, error) { return nil, nil }
func (c catTokens) Revoke(primitive.ObjectID, primitive.ObjectID) error      { return nil }
func (c catTokens) GetActive(string) (*models.McpToken, error) {
	return &models.McpToken{UserID: c.user}, nil
}

type catUsers struct{ user *models.User }

func (c catUsers) GetByID(string) *models.User    { return c.user }
func (c catUsers) GetByEmail(string) *models.User { return nil }

// catEvents serves one muster. Everything else is nil: these tests never write.
type catEvents struct {
	mcp.EventStore
	event *models.Event
}

func (c catEvents) GetByEitherID(id string) *models.Event {
	if c.event != nil && (id == c.event.Id.Hex()) {
		return c.event
	}
	return nil
}
func (c catEvents) Attendees(primitive.ObjectID) []models.Attendee { return nil }
func (c catEvents) Responses(primitive.ObjectID) []models.EventResponse {
	return nil
}
func (c catEvents) ResponseFlags(string, []primitive.ObjectID) map[string]mcp.ResponseFlags {
	return map[string]mcp.ResponseFlags{}
}

type wire struct {
	r     http.Handler
	event *models.Event
}

func newWire(app bool) *wire {
	gin.SetMode(gin.TestMode)
	middleware.ResetMCPLimits()
	owner := &models.User{Id: primitive.NewObjectID(), Email: "owner@x.com", FirstName: "Olive"}
	hours, slot := float32(8), 30
	ev := &models.Event{Id: primitive.NewObjectID(), OwnerId: owner.Id, Name: "Plan", Type: models.SPECIFIC_DATES,
		Dates: []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))}, Duration: &hours, TimeIncrement: &slot}
	deps := mcp.Deps{Tokens: catTokens{owner.Id}, Users: catUsers{owner}, Events: catEvents{event: ev}}
	r := gin.New()
	if app {
		routes.InitMcp(r, true, deps, routes.McpAppsOptions{Enabled: true, LoadHTML: func() ([]byte, error) {
			return []byte("<!doctype html><html><body>app</body></html>"), nil
		}})
	} else {
		routes.InitMcp(r, true, deps)
	}
	return &wire{r: r, event: ev}
}

func (w *wire) post(t *testing.T, method string, params any) (map[string]any, []byte) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer muster_live")
	rec := httptest.NewRecorder()
	w.r.ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s: %v: %s", method, err, rec.Body.String())
	}
	return out, rec.Body.Bytes()
}

func (w *wire) tools(t *testing.T) (map[string]map[string]any, []string, int) {
	t.Helper()
	out, _ := w.post(t, "tools/list", nil)
	result := out["result"].(map[string]any)
	raw, _ := json.Marshal(result)
	byName, order := map[string]map[string]any{}, []string{}
	for _, x := range result["tools"].([]any) {
		m := x.(map[string]any)
		byName[m["name"].(string)] = m
		order = append(order, m["name"].(string))
	}
	return byName, order, len(raw)
}

func (w *wire) call(t *testing.T, tool string, args map[string]any) map[string]any {
	t.Helper()
	out, _ := w.post(t, "tools/call", map[string]any{"name": tool, "arguments": args})
	return out["result"].(map[string]any)
}

// modelText is every string a model reads before it calls anything.
func (w *wire) modelText(t *testing.T) []string {
	t.Helper()
	all := []string{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			all = append(all, x)
		case []any:
			for _, i := range x {
				walk(i)
			}
		case map[string]any:
			for _, i := range x {
				walk(i)
			}
		}
	}
	tools, _, _ := w.tools(t)
	for _, tool := range tools {
		walk(tool)
	}
	init, _ := w.post(t, "initialize", map[string]any{"protocolVersion": "2025-03-26"})
	walk(init["result"])
	res, _ := w.post(t, "resources/list", nil)
	for _, r := range res["result"].(map[string]any)["resources"].([]any) {
		uri := r.(map[string]any)["uri"].(string)
		if strings.HasPrefix(uri, "muster://") {
			read, _ := w.post(t, "resources/read", map[string]any{"uri": uri})
			walk(read["result"])
		}
	}
	return all
}

var baseTools = []string{
	"muster_whoami", "muster_list_my_events", "muster_get_event", "muster_get_responses", "muster_set_availability",
	"muster_decline", "muster_add_attendees", "muster_create_event", "muster_list_attendees", "muster_find_best_times",
	"muster_finalize", "muster_update_event", "muster_remind", "muster_remove_attendee", "muster_list_changes", "muster_summarize_overlap",
}

func hint(t *testing.T, tool map[string]any, key string) bool {
	t.Helper()
	ann, _ := tool["annotations"].(map[string]any)
	v, ok := ann[key].(bool)
	if !ok {
		t.Fatalf("%v: annotation %s missing", tool["name"], key)
	}
	return v
}

func undocumented(tool map[string]any) []string {
	schema, _ := tool["inputSchema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	missing := []string{}
	for name, raw := range props {
		p, _ := raw.(map[string]any)
		if d, _ := p["description"].(string); strings.TrimSpace(d) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

func annotationsAndDescriptions(t *testing.T) {
	tools, order, _ := newWire(false).tools(t)
	if len(order) != 16 {
		t.Fatalf("want the 16 base tools, got %d: %v", len(order), order)
	}
	for _, name := range baseTools {
		tool := tools[name]
		if tool == nil {
			t.Fatalf("%s missing", name)
		}
		for _, k := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			hint(t, tool, k)
		}
		if d, _ := tool["description"].(string); len(d) < 40 {
			t.Errorf("%s description is too thin: %q", name, d)
		}
		if missing := undocumented(tool); len(missing) > 0 {
			t.Errorf("%s: parameters without a description: %v", name, missing)
		}
	}
	reads := []string{"muster_whoami", "muster_list_my_events", "muster_get_event", "muster_get_responses", "muster_list_attendees",
		"muster_find_best_times", "muster_list_changes", "muster_summarize_overlap", "muster_remind"}
	for _, name := range reads {
		tool := tools[name]
		if !hint(t, tool, "readOnlyHint") || !hint(t, tool, "idempotentHint") || hint(t, tool, "destructiveHint") {
			t.Errorf("%s must be read-only, idempotent, not destructive", name)
		}
	}
	for _, name := range []string{"muster_set_availability", "muster_update_event"} {
		tool := tools[name]
		if hint(t, tool, "readOnlyHint") || !hint(t, tool, "destructiveHint") || !hint(t, tool, "idempotentHint") {
			t.Errorf("%s must be destructive and idempotent", name)
		}
	}
	add := tools["muster_add_attendees"]
	if hint(t, add, "readOnlyHint") || hint(t, add, "destructiveHint") || !hint(t, add, "idempotentHint") {
		t.Errorf("add_attendees: nondestructive, idempotent after dedupe")
	}
	for _, name := range []string{"muster_finalize", "muster_remove_attendee", "muster_decline"} {
		if hint(t, tools[name], "readOnlyHint") || !hint(t, tools[name], "destructiveHint") {
			t.Errorf("%s must be destructive", name)
		}
	}
	create := tools["muster_create_event"]
	if hint(t, create, "readOnlyHint") || hint(t, create, "idempotentHint") {
		t.Errorf("create_event makes a new muster every call, so it is not idempotent")
	}
	// Replace and merge semantics are spelled out where the model reads them.
	set := tools["muster_set_availability"]
	text := set["description"].(string)
	props := set["inputSchema"].(map[string]any)["properties"].(map[string]any)
	modeDesc := props["mode"].(map[string]any)["description"].(string)
	if !strings.Contains(text, "replace") || !strings.Contains(text, "merge") || !strings.Contains(modeDesc, "merge") {
		t.Errorf("replace/merge semantics missing")
	}
	// Time parameters say RFC3339 and give an example somewhere in the tool.
	for _, name := range []string{"muster_get_responses", "muster_find_best_times", "muster_summarize_overlap", "muster_finalize", "muster_list_changes"} {
		if raw, _ := json.Marshal(tools[name]); !strings.Contains(string(raw), "RFC3339") {
			t.Errorf("%s does not name the time format", name)
		}
	}
	// The four App tools are checked on their own.
	t.Setenv("MCP_APPS_WRITES_ENABLED", "true")
	app, appOrder, _ := newWire(true).tools(t)
	four := []string{"muster_open_view", "muster_ui_prepare_action", "muster_ui_commit_action", "muster_ui_get_action"}
	if len(appOrder) != 20 {
		t.Fatalf("full-build catalog is %d tools, want 20", len(appOrder))
	}
	for _, name := range four {
		tool := app[name]
		if tool == nil {
			t.Fatalf("%s missing", name)
		}
		for _, k := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			hint(t, tool, k)
		}
		if missing := undocumented(tool); len(missing) > 0 {
			t.Errorf("%s: parameters without a description: %v", name, missing)
		}
	}
	if !hint(t, app["muster_open_view"], "readOnlyHint") || hint(t, app["muster_ui_prepare_action"], "readOnlyHint") ||
		!hint(t, app["muster_ui_prepare_action"], "destructiveHint") || !hint(t, app["muster_ui_get_action"], "readOnlyHint") {
		t.Errorf("App tool annotations are not truthful")
	}
}

var vendors = regexp.MustCompile(`(?i)listmonk|sendgrid|mailgun|postmark|mailchimp|sparkpost|brevo|amazon ses|\bses\b|resend\.com|smtp`)

func noVendorNames(t *testing.T) {
	w := newWire(true)
	for _, text := range w.modelText(t) {
		if m := vendors.FindString(text); m != "" {
			t.Errorf("model-visible text names a mail vendor %q: %.120s", m, text)
		}
	}
	tools, _, _ := w.tools(t)
	for _, name := range []string{"muster_create_event", "muster_add_attendees"} {
		d := tools[name]["description"].(string)
		if !strings.Contains(d, "mail delivery") {
			t.Errorf("%s must say \"mail delivery\": %s", name, d)
		}
	}
	// Truthful email_sent qualifications stay.
	for name, tool := range tools {
		d := tool["description"].(string)
		if name == "muster_finalize" || name == "muster_remind" {
			if !strings.Contains(d, "email_sent is false") {
				t.Errorf("%s lost its email_sent qualification: %s", name, d)
			}
		}
	}
}

func textError(t *testing.T) {
	w := newWire(false)
	// Read only the text content, as a text-only client does.
	res := w.call(t, "muster_set_availability", map[string]any{
		"event_id": w.event.Id.Hex(), "availability": []any{"2026-06-01T09:00:00Z", "2026-06-01T09:10:00Z"}, "if_needed": []any{}})
	if res["isError"] != true {
		t.Fatalf("%#v", res)
	}
	flat := res["structuredContent"].(map[string]any)
	if flat["code"] != "SLOT_OFF_GRID" || flat["what_to_do_next"] == "" {
		t.Fatalf("flat fields: %#v", flat)
	}
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("error text is not parseable JSON: %q", text)
	}
	data, _ := parsed["data"].(map[string]any)
	if parsed["code"] != "SLOT_OFF_GRID" || data["code"] != "SLOT_OFF_GRID" || data["index"] != float64(1) ||
		data["value"] != "2026-06-01T09:10:00Z" || data["what_to_do_next"] == "" || data["message"] == "" {
		t.Fatalf("text error lost its structure: %s", text)
	}
	if b, _ := data["bounds"].(map[string]any); b["slot_minutes"] != float64(30) {
		t.Fatalf("valid bounds missing from text: %s", text)
	}
	// A stranger's text carries no bounds and cannot tell a hidden muster from a missing one.
	missing := w.call(t, "muster_set_availability", map[string]any{"event_id": "nope", "availability": []any{}})
	mt := missing["content"].([]any)[0].(map[string]any)["text"].(string)
	var mp map[string]any
	if err := json.Unmarshal([]byte(mt), &mp); err != nil || mp["code"] != "EVENT_NOT_FOUND_OR_NO_ACCESS" {
		t.Fatalf("not-found text: %q", mt)
	}
	if d, _ := mp["data"].(map[string]any); d["code"] != "NOT_FOUND" || d["bounds"] != nil {
		t.Fatalf("not-found data: %q", mt)
	}
}

func requiredIsOnlyEvent(t *testing.T, tool map[string]any) {
	t.Helper()
	req, _ := tool["inputSchema"].(map[string]any)["required"].([]any)
	if len(req) != 1 || req[0] != "event_id" {
		t.Errorf("%v: start and end are optional together, but required is %v", tool["name"], req)
	}
}

func TestMCPTextParityCatalog(t *testing.T) {
	t.Run("REQ_TXT_004", func(t *testing.T) {
		tools, _, _ := newWire(false).tools(t)
		requiredIsOnlyEvent(t, tools["muster_get_responses"])
		requiredIsOnlyEvent(t, tools["muster_summarize_overlap"])
	})
	t.Run("REQ_TXT_005", textError)
	t.Run("REQ_TXT_006", annotationsAndDescriptions)
	t.Run("REQ_TXT_011", func(t *testing.T) {
		tools, _, _ := newWire(false).tools(t)
		props := tools["muster_list_my_events"]["inputSchema"].(map[string]any)["properties"].(map[string]any)
		for _, k := range []string{"status", "needs_my_answer", "projection", "limit", "cursor", "name"} {
			if _, ok := props[k]; !ok {
				t.Errorf("list schema lacks %s", k)
			}
		}
		if d := props["status"].(map[string]any)["description"].(string); !strings.Contains(d, "needs_answer") || !strings.Contains(d, "all") || !strings.Contains(d, "archived") {
			t.Errorf("status description: %s", d)
		}
	})
	t.Run("REQ_TXT_016", noVendorNames)
	t.Run("REQ_EFF_002", func(t *testing.T) {
		_, order, size := newWire(false).tools(t)
		t.Logf("text-only catalog: %d tools, %d bytes (final-tip baseline 16 tools, 12666 bytes)", len(order), size)
		if len(order) != 16 {
			t.Errorf("tool count %d, baseline 16", len(order))
		}
		if size > 32*1024 {
			t.Errorf("catalog is %d bytes, over the 32 KiB hard ceiling", size)
		}
		if size > 24*1024 {
			t.Errorf("catalog is %d bytes, over the 24 KiB soft target", size)
		}
		// Approved 2026-11-04. 12,666 -> 15,608 is the descriptions, list filters, the timezone label,
		// and the note that archive is a status, not a boolean. Still under the 24 KiB soft target.
		const approvedText = 15608
		const approvedFull = 19884
		if size != approvedText {
			t.Errorf("text catalog is %d bytes; the approved size is %d (was 12,666)", size, approvedText)
		}
		t.Setenv("MCP_APPS_WRITES_ENABLED", "true")
		_, full, fullSize := newWire(true).tools(t)
		t.Logf("full-build catalog: %d tools, %d bytes", len(full), fullSize)
		if len(full) != 20 || fullSize != approvedFull || fullSize > 32*1024 {
			t.Errorf("full build is %d tools, %d bytes; approved 20 tools, %d bytes", len(full), fullSize, approvedFull)
		}
	})
	t.Run("REQ_EFF_005", func(t *testing.T) {
		middleware.ResetMCPLimits()
		now := time.Now()
		for i := 1; i <= 60; i++ {
			if !middleware.AllowUser("u1", now) {
				t.Fatalf("call %d refused; the bucket is 60 per minute", i)
			}
		}
		if middleware.AllowUser("u1", now) {
			t.Fatal("61st call in a minute was allowed")
		}
		if !middleware.AllowUser("u1", now.Add(61*time.Second)) {
			t.Fatal("bucket did not reset")
		}
	})
	t.Run("REQ_EFF_006", func(t *testing.T) {
		t.Setenv("MCP_APPS_WRITES_ENABLED", "true")
		tools, _, _ := newWire(true).tools(t)
		for _, name := range []string{"muster_ui_prepare_action", "muster_ui_commit_action", "muster_ui_get_action"} {
			tool := tools[name]
			if !strings.HasPrefix(tool["description"].(string), "App only") {
				t.Errorf("%s description must say App only: %v", name, tool["description"])
			}
			vis := tool["_meta"].(map[string]any)["ui"].(map[string]any)["visibility"].([]any)
			if len(vis) != 1 || vis[0] != "app" {
				t.Errorf("%s visibility %v", name, vis)
			}
		}
	})
}

func TestMCPFieldFindingsCatalog(t *testing.T) {
	t.Run("FE4", func(t *testing.T) {
		tools, _, _ := newWire(false).tools(t)
		requiredIsOnlyEvent(t, tools["muster_get_responses"])
		requiredIsOnlyEvent(t, tools["muster_summarize_overlap"])
	})
	t.Run("FE5", textError)
	t.Run("FE6", annotationsAndDescriptions)
	t.Run("FE12", noVendorNames)
	t.Run("FE13", func(t *testing.T) {
		tools, _, _ := newWire(false).tools(t)
		props := tools["muster_list_my_events"]["inputSchema"].(map[string]any)["properties"].(map[string]any)
		for _, k := range []string{"status", "needs_my_answer", "projection"} {
			p, _ := props[k].(map[string]any)
			if d, _ := p["description"].(string); d == "" {
				t.Errorf("list parameter %s is not documented", k)
			}
		}
	})
	t.Run("W1", func(t *testing.T) {
		w := newWire(false)
		init, _ := w.post(t, "initialize", map[string]any{"protocolVersion": "2025-03-26"})
		instructions, _ := init["result"].(map[string]any)["instructions"].(string)
		if len(instructions) == 0 || len(instructions) > 3000 {
			t.Fatalf("instructions must be concise, got %d bytes", len(instructions))
		}
		skills := map[string]string{}
		res, _ := w.post(t, "resources/list", nil)
		for _, r := range res["result"].(map[string]any)["resources"].([]any) {
			uri := r.(map[string]any)["uri"].(string)
			read, _ := w.post(t, "resources/read", map[string]any{"uri": uri})
			skills[uri] = read["result"].(map[string]any)["contents"].([]any)[0].(map[string]any)["text"].(string)
		}
		if len(skills) != 3 {
			t.Fatalf("want exactly the three skills, got %d", len(skills))
		}
		for _, uri := range []string{"muster://skills/pick-a-time", "muster://skills/answer", "muster://skills/run-a-group"} {
			text := skills[uri]
			if text == "" || len(text) > 4096 {
				t.Errorf("%s: %d bytes", uri, len(text))
			}
			if !strings.Contains(instructions, uri) {
				t.Errorf("instructions do not point at %s", uri)
			}
		}
		for _, want := range []string{"needs_answer", "status all", "omitting both lists is rejected", "no mail", "does not create a calendar event", "2018-06-18T15:00:00Z", "positive multiple of slot_minutes"} {
			if !strings.Contains(strings.ToLower(instructions), strings.ToLower(want)) {
				t.Errorf("instructions lost %q", want)
			}
		}
		// Only the changed F7 semantics were added to the skills.
		for uri, want := range map[string][]string{
			"muster://skills/pick-a-time": {"unavailable_names", "response_totals"},
			"muster://skills/answer":      {"ranges", "merge"},
			"muster://skills/run-a-group": {"skipped_details", "response_totals"},
		} {
			for _, w := range want {
				if !strings.Contains(skills[uri], w) {
					t.Errorf("%s does not mention %s", uri, w)
				}
			}
		}
		for uri, text := range skills {
			for _, stale := range []string{"muster_get_responses for the caller's own visible answer when you need to confirm"} {
				if strings.Contains(text, stale) {
					t.Errorf("%s still tells the model to read back what the receipt already echoes", uri)
				}
			}
		}
	})
}
