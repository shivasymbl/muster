package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/logger"
	"github.com/shivasymbl/muster/server/middleware"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

type fakeTokens struct {
	active  map[string]*models.McpToken
	lookups int
}

func (f *fakeTokens) Create(*models.User, string) (string, models.McpTokenPublic, error) {
	return "", models.McpTokenPublic{}, nil
}
func (f *fakeTokens) List(primitive.ObjectID) ([]models.McpTokenPublic, error) {
	return nil, nil
}
func (f *fakeTokens) Revoke(primitive.ObjectID, primitive.ObjectID) error { return nil }
func (f *fakeTokens) GetActive(plaintext string) (*models.McpToken, error) {
	f.lookups++
	if f.active == nil {
		return nil, nil
	}
	return f.active[plaintext], nil
}

type fakeUsers struct {
	byID map[string]*models.User
}

func (f *fakeUsers) GetByID(id string) *models.User {
	if f.byID == nil {
		return nil
	}
	return f.byID[id]
}

func (f *fakeUsers) GetByEmail(email string) *models.User {
	for _, user := range f.byID {
		if user != nil && strings.EqualFold(user.Email, email) {
			return user
		}
	}
	return nil
}

type storedEvent struct {
	event    *models.Event
	owner    primitive.ObjectID
	emails   []string
	declined map[string]*bool
}

type fakeEvents struct {
	byHex        map[string]*models.Event
	byShort      map[string]*models.Event
	attendees    map[string][]models.Attendee
	responses    map[string][]models.EventResponse
	flags        map[string]map[string]mcp.ResponseFlags
	listed       []mcp.ListedEvent
	gets         int
	responseGets int
	lastUser     primitive.ObjectID
	lastLimit    int
	lastName     string
	panicOnGet   bool
	replaced     []primitive.DateTime
	declined     bool
	added        []string
}

func (f *fakeEvents) GetByEitherID(id string) *models.Event {
	f.gets++
	if f.panicOnGet {
		panic("db down")
	}
	if len(id) <= 10 {
		return f.byShort[id]
	}
	return f.byHex[id]
}
func (f *fakeEvents) Responses(id primitive.ObjectID) []models.EventResponse {
	f.responseGets++
	return f.responses[id.Hex()]
}
func (f *fakeEvents) Attendees(id primitive.ObjectID) []models.Attendee {
	return f.attendees[id.Hex()]
}
func (f *fakeEvents) ListForUser(user *models.User, name string, limit int, cursor string) ([]mcp.ListedEvent, string, error) {
	f.lastUser = user.Id
	f.lastLimit = limit
	f.lastName = name
	out := make([]mcp.ListedEvent, 0)
	for _, row := range f.listed {
		if row.Event.OwnerId != user.Id && !row.IsAttendee {
			continue
		}
		if name != "" && !strings.Contains(strings.ToLower(row.Event.Name), strings.ToLower(name)) {
			continue
		}
		if cursor != "" && row.Event.Id.Hex() >= cursor {
			continue
		}
		out = append(out, row)
	}
	return out, "", nil
}
func (f *fakeEvents) ReplaceAvailability(user *models.User, eventID primitive.ObjectID, availability, ifNeeded []primitive.DateTime) error {
	f.replaced = availability
	f.responses[eventID.Hex()] = []models.EventResponse{{
		UserId: user.Id.Hex(),
		Response: &models.Response{
			UserId:       user.Id,
			Name:         user.FirstName,
			Availability: availability,
			IfNeeded:     ifNeeded,
		},
	}}
	return nil
}

func (f *fakeEvents) Decline(user *models.User, eventID primitive.ObjectID) error {
	f.declined = true
	f.lastUser = user.Id
	return nil
}

func (f *fakeEvents) AddAttendees(eventID primitive.ObjectID, emails []string) ([]string, error) {
	f.added = emails
	return emails, nil
}

func (f *fakeEvents) Create(user *models.User, event models.Event, emails []string) (*models.Event, error) {
	event.Id = primitive.NewObjectID()
	event.OwnerId = user.Id
	shortID := "new123"
	event.ShortId = &shortID
	if f.byHex == nil {
		f.byHex = map[string]*models.Event{}
	}
	f.byHex[event.Id.Hex()] = &event
	if event.Type == models.GROUP {
		people := []models.Attendee{{Email: user.Email, EventId: event.Id}}
		for _, email := range emails {
			people = append(people, models.Attendee{Email: email, EventId: event.Id})
		}
		if f.attendees == nil {
			f.attendees = map[string][]models.Attendee{}
		}
		f.attendees[event.Id.Hex()] = people
	}
	f.added = emails
	return &event, nil
}

func (f *fakeEvents) Patch(eventID primitive.ObjectID, set map[string]any) error {
	event := f.byHex[eventID.Hex()]
	if event == nil {
		return nil
	}
	if name, ok := set["name"].(string); ok {
		event.Name = name
	}
	if archived, ok := set["isArchived"].(bool); ok {
		event.IsArchived = &archived
	}
	if _, ok := set["scheduledEvent"]; ok {
		event.IsArchived = utils.TruePtr()
	}
	return nil
}

func (f *fakeEvents) RemoveAttendee(eventID primitive.ObjectID, email, responseUserHex string) error {
	kept := []models.Attendee{}
	for _, attendee := range f.attendees[eventID.Hex()] {
		if !strings.EqualFold(attendee.Email, email) {
			kept = append(kept, attendee)
		}
	}
	f.attendees[eventID.Hex()] = kept
	if responseUserHex != "" {
		keptRows := []models.EventResponse{}
		for _, row := range f.responses[eventID.Hex()] {
			if row.UserId != responseUserHex {
				keptRows = append(keptRows, row)
			}
		}
		f.responses[eventID.Hex()] = keptRows
	}
	return nil
}

func (f *fakeEvents) Changes(eventID primitive.ObjectID, since time.Time) ([]models.EventResponse, []models.Attendee, error) {
	return f.responses[eventID.Hex()], f.attendees[eventID.Hex()], nil
}

func (f *fakeEvents) ResponseFlags(userHex string, ids []primitive.ObjectID) map[string]mcp.ResponseFlags {
	out := map[string]mcp.ResponseFlags{}
	for _, id := range ids {
		if flag, ok := f.flags[userHex][id.Hex()]; ok {
			out[id.Hex()] = flag
		}
	}
	return out
}

func engine(deps mcp.Deps, enabled bool) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
	InitMcp(r, enabled, deps)
	return r
}

func postMCP(r http.Handler, body, token, remote, forwarded string, cookieHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if remote != "" {
		req.RemoteAddr = remote
	}
	if forwarded != "" {
		req.Header.Set("X-Forwarded-For", forwarded)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("status %d body %s: %v", rec.Code, rec.Body.String(), err)
	}
	return body
}

func dataCode(t *testing.T, body map[string]any) string {
	t.Helper()
	errObj, _ := body["error"].(map[string]any)
	data, _ := errObj["data"].(map[string]any)
	code, _ := data["code"].(string)
	return code
}

func TestMcpProtocol(t *testing.T) {
	middleware.ResetMCPLimits()
	owner := primitive.NewObjectID()
	other := primitive.NewObjectID()
	caller := &models.User{Id: owner, Email: "owner@x.com", FirstName: "Ada", LastName: "Lovelace"}
	stranger := &models.User{Id: other, Email: "hidden-person@example.com", FirstName: "Hidden"}
	eventID := primitive.NewObjectID()
	short := "abc"
	event := &models.Event{Id: eventID, OwnerId: owner, Name: "Interview", Type: models.SPECIFIC_DATES, ShortId: &short}
	foreignID := primitive.NewObjectID()
	foreign := models.Event{Id: foreignID, OwnerId: other, Name: "Shared title", Type: models.SPECIFIC_DATES}
	mine := models.Event{Id: eventID, OwnerId: owner, Name: "Mine", Type: models.SPECIFIC_DATES, ShortId: &short}

	tokens := &fakeTokens{active: map[string]*models.McpToken{
		"muster_live": {UserID: owner},
	}}
	users := &fakeUsers{byID: map[string]*models.User{owner.Hex(): caller}}
	events := &fakeEvents{
		byHex:   map[string]*models.Event{eventID.Hex(): event, foreignID.Hex(): &foreign},
		byShort: map[string]*models.Event{"abc": event},
		attendees: map[string][]models.Attendee{
			eventID.Hex(): {{Email: caller.Email}},
		},
		listed: []mcp.ListedEvent{
			{Event: mine, IsAttendee: false},
			{Event: foreign, IsAttendee: false},
		},
	}
	deps := mcp.Deps{Tokens: tokens, Users: users, Events: events}
	r := engine(deps, true)

	t.Run("disabled is json not html", func(t *testing.T) {
		rec := postMCP(engine(deps, false), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "", "", "", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<html") {
			t.Fatal(rec.Body.String())
		}
		if decode(t, rec)["code"] != "MCP_DISABLED" {
			t.Fatal(rec.Body.String())
		}
	})

	t.Run("get asks for sign-in", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "resource_metadata=") {
			t.Fatalf("status %d header %q body %q", rec.Code, rec.Header().Get("WWW-Authenticate"), rec.Body.String())
		}
	})

	t.Run("signed-in get has no event stream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer muster_live")
		req.Header.Set("Accept", "text/event-stream")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
			t.Fatalf("status %d allow %q body %s", rec.Code, rec.Header().Get("Allow"), rec.Body.String())
		}
		if strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
			t.Fatal(rec.Header().Get("Content-Type"))
		}
	})

	t.Run("initialize without a token is the sign-in challenge", func(t *testing.T) {
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, "", "", "", "")
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "resource_metadata=") {
			t.Fatalf("status %d header %q body %s", rec.Code, rec.Header().Get("WWW-Authenticate"), rec.Body.String())
		}
		signed := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, "muster_live", "", "", "")
		body := decode(t, signed)
		result, _ := body["result"].(map[string]any)
		if signed.Code != http.StatusOK || result["protocolVersion"] != "2025-11-25" || !strings.Contains(result["instructions"].(string), "muster://skills/pick-a-time") {
			t.Fatal(signed.Body.String())
		}
	})

	t.Run("skills are resources", func(t *testing.T) {
		listed := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`, "muster_live", "", "", "")
		resources := decode(t, listed)["result"].(map[string]any)["resources"].([]any)
		if listed.Code != http.StatusOK || len(resources) != 3 {
			t.Fatal(listed.Body.String())
		}
		read := postMCP(r, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"muster://skills/pick-a-time"}}`, "muster_live", "", "", "")
		contents := decode(t, read)["result"].(map[string]any)["contents"].([]any)
		text := contents[0].(map[string]any)["text"].(string)
		if read.Code != http.StatusOK || !strings.Contains(text, "muster_finalize") || !strings.Contains(text, "does not create a calendar event") {
			t.Fatal(read.Body.String())
		}
		missing := postMCP(r, `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"muster://skills/missing"}}`, "muster_live", "", "", "")
		if decode(t, missing)["error"].(map[string]any)["code"].(float64) != -32002 {
			t.Fatal(missing.Body.String())
		}
	})

	t.Run("initialized is 202", func(t *testing.T) {
		rec := postMCP(r, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, "", "", "", "")
		if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
			t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
		}
	})

	notSigned := func(token string) map[string]any {
		rec := postMCP(r, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"muster_whoami"}}`, token, "203.0.113.50:9", "198.51.100.8", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		body := decode(t, rec)
		data := body["error"].(map[string]any)["data"].(map[string]any)
		if data["code"] != "NOT_SIGNED_IN" || data["retryable"] != false {
			t.Fatal(data)
		}
		return data
	}
	sameData := func(a, b map[string]any) bool {
		return a["code"] == b["code"] && a["message"] == b["message"] && a["what_to_do_next"] == b["what_to_do_next"] && a["retryable"] == b["retryable"]
	}
	t.Run("missing revoked expired unknown and deleted user match", func(t *testing.T) {
		missing := notSigned("")
		unknown := notSigned("muster_unknown")
		tokens.active["muster_revoked"] = nil
		revoked := notSigned("muster_revoked")
		tokens.active["muster_expired"] = nil
		expired := notSigned("muster_expired")
		gone := primitive.NewObjectID()
		tokens.active["muster_deleted"] = &models.McpToken{UserID: gone}
		deleted := notSigned("muster_deleted")
		if !sameData(missing, unknown) || !sameData(unknown, revoked) || !sameData(revoked, expired) || !sameData(expired, deleted) {
			t.Fatalf("%v vs %v", missing, deleted)
		}
	})

	t.Run("bearer is not a website session", func(t *testing.T) {
		web := gin.New()
		web.Use(sessions.Sessions("session", cookie.NewStore([]byte("01234567890123456789012345678901"))))
		web.GET("/api/user/events", middleware.AuthRequired(), func(c *gin.Context) { c.Status(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, "/api/user/events", nil)
		req.Header.Set("Authorization", "Bearer muster_live")
		rec := httptest.NewRecorder()
		web.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("cookie user A and token user B acts as B", func(t *testing.T) {
		web := gin.New()
		_ = web.SetTrustedProxies([]string{"127.0.0.1", "::1"})
		store := cookie.NewStore([]byte("01234567890123456789012345678901"))
		web.Use(sessions.Sessions("session", store))
		web.GET("/prime", func(c *gin.Context) {
			s := sessions.Default(c)
			s.Set("userId", other.Hex())
			_ = s.Save()
			c.Status(http.StatusNoContent)
		})
		InitMcp(web, true, deps)
		prime := httptest.NewRecorder()
		web.ServeHTTP(prime, httptest.NewRequest(http.MethodGet, "/prime", nil))
		cookieHeader := ""
		for i, c := range prime.Result().Cookies() {
			if i > 0 {
				cookieHeader += "; "
			}
			cookieHeader += c.Name + "=" + c.Value
		}
		rec := postMCP(web, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_whoami","arguments":{"user_id":"`+other.Hex()+`"}}}`, "muster_live", "", "", cookieHeader)
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		if result["isError"] == true {
			t.Fatal(rec.Body.String())
		}
		structured := result["structuredContent"].(map[string]any)
		if structured["user_id"] != owner.Hex() || structured["email"] != "owner@x.com" {
			t.Fatalf("%v", structured)
		}
	})

	t.Run("missing and foreign events share one error", func(t *testing.T) {
		call := func(id string) map[string]any {
			rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_get_event","arguments":{"event_id":"`+id+`"}}}`, "muster_live", "", "", "")
			body := decode(t, rec)
			result := body["result"].(map[string]any)
			return result["structuredContent"].(map[string]any)
		}
		missing := call(primitive.NewObjectID().Hex())
		foreignBody := call(foreignID.Hex())
		if !sameData(missing, foreignBody) || missing["code"] != "EVENT_NOT_FOUND_OR_NO_ACCESS" || missing["message"] != "No muster found." {
			t.Fatalf("%v vs %v", missing, foreignBody)
		}
	})

	t.Run("a member can get the event", func(t *testing.T) {
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_get_event","arguments":{"event_id":"`+eventID.Hex()+`"}}}`, "muster_live", "", "", "")
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		if result["isError"] == true {
			t.Fatal(rec.Body.String())
		}
		structured := result["structuredContent"].(map[string]any)
		if structured["event_id"] != eventID.Hex() || structured["short_id"] != "abc" {
			t.Fatal(structured)
		}
	})

	t.Run("limit 1000 is 50 and a foreign name is not listed", func(t *testing.T) {
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_list_my_events","arguments":{"limit":1000,"name":"Shared title"}}}`, "muster_live", "", "", "")
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		if result["isError"] == true {
			t.Fatal(rec.Body.String())
		}
		if events.lastLimit != 50 || events.lastUser != owner || events.lastName != "Shared title" {
			t.Fatalf("limit %d user %s name %s", events.lastLimit, events.lastUser.Hex(), events.lastName)
		}
		structured := result["structuredContent"].(map[string]any)
		rows := structured["events"].([]any)
		if len(rows) != 0 {
			t.Fatalf("rows %#v", rows)
		}
	})

	t.Run("eight day window does not load responses", func(t *testing.T) {
		before := events.responseGets
		gets := events.gets
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_get_responses","arguments":{"event_id":"`+eventID.Hex()+`","start":"2026-01-01T00:00:00Z","end":"2026-01-09T00:00:00Z"}}}`, "muster_live", "", "", "")
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		structured := result["structuredContent"].(map[string]any)
		if result["isError"] != true || structured["code"] != "WINDOW_TOO_WIDE" {
			t.Fatal(rec.Body.String())
		}
		if events.responseGets != before || events.gets != gets {
			t.Fatalf("lookups responses %d gets %d", events.responseGets-before, events.gets-gets)
		}
	})

	t.Run("blind non-owner has no other ranges and the log is safe", func(t *testing.T) {
		var buf bytes.Buffer
		logger.StdOut = log.New(&buf, "", 0)
		defer func() { logger.StdOut = nil }()
		blind := true
		secret := "2033-11-22T04:05:06Z"
		mark, _ := time.Parse(time.RFC3339, secret)
		event.BlindAvailabilityEnabled = &blind
		event.OwnerId = other
		if events.responses == nil {
			events.responses = map[string][]models.EventResponse{}
		}
		events.attendees[eventID.Hex()] = []models.Attendee{{Email: caller.Email}}
		events.responses[eventID.Hex()] = []models.EventResponse{
			{UserId: other.Hex(), Response: &models.Response{Availability: []primitive.DateTime{primitive.NewDateTimeFromTime(mark)}, Email: stranger.Email, Name: "Hidden"}},
			{UserId: owner.Hex(), Response: &models.Response{Availability: []primitive.DateTime{primitive.NewDateTimeFromTime(mark.Add(time.Hour))}, Email: caller.Email, Name: "Ada"}},
		}
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_get_responses","arguments":{"event_id":"`+eventID.Hex()+`","start":"2033-11-22T00:00:00Z","end":"2033-11-23T00:00:00Z"}}}`, "muster_live", "", "", "")
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		if result["isError"] == true {
			t.Fatal(rec.Body.String())
		}
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		if strings.Contains(text, other.Hex()) || strings.Contains(text, stranger.Email) {
			t.Fatal(text)
		}
		logLine := buf.String()
		if strings.Contains(logLine, "muster_live") || strings.Contains(logLine, stranger.Email) || strings.Contains(logLine, secret) || strings.Contains(logLine, caller.Email) {
			t.Fatal(logLine)
		}
		event.OwnerId = owner
		event.BlindAvailabilityEnabled = nil
	})

	t.Run("oversize payload is truncated under 4096", func(t *testing.T) {
		event.OwnerId = owner
		event.BlindAvailabilityEnabled = nil
		marks := make([]primitive.DateTime, 0, 200)
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 200; i++ {
			marks = append(marks, primitive.NewDateTimeFromTime(base.Add(time.Duration(i)*time.Hour)))
		}
		events.responses[eventID.Hex()] = []models.EventResponse{
			{UserId: owner.Hex(), Response: &models.Response{Availability: marks, Name: "Ada"}},
			{UserId: other.Hex(), Response: &models.Response{Availability: marks, Name: "Other"}},
		}
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_get_responses","arguments":{"event_id":"`+eventID.Hex()+`","start":"2026-05-01T00:00:00Z","end":"2026-05-08T00:00:00Z"}}}`, "muster_live", "", "", "")
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		structured := result["structuredContent"].(map[string]any)
		if structured["truncated"] != true || len(text) > 4096 {
			t.Fatalf("truncated %v len %d", structured["truncated"], len(text))
		}
	})

	t.Run("61st call is rate limited", func(t *testing.T) {
		middleware.ResetMCPLimits()
		var last *httptest.ResponseRecorder
		for i := 0; i < 61; i++ {
			last = postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "muster_live", "203.0.113.77:9", "", "")
		}
		body := decode(t, last)
		if last.Code != http.StatusTooManyRequests || dataCode(t, body) != "RATE_LIMITED" {
			t.Fatalf("status %d %s", last.Code, last.Body.String())
		}
		data := body["error"].(map[string]any)["data"].(map[string]any)
		if data["retryable"] != true || data["message"] != "Too many tool calls." {
			t.Fatal(data)
		}
	})

	t.Run("31st failed auth is before the hash and ignores spoofed forwarded-for", func(t *testing.T) {
		middleware.ResetMCPLimits()
		tokens.lookups = 0
		var last *httptest.ResponseRecorder
		for i := 0; i < 31; i++ {
			last = postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "muster_nope", "203.0.113.9:4000", "198.51.100.20", "")
		}
		if last.Code != http.StatusTooManyRequests || dataCode(t, decode(t, last)) != "RATE_LIMITED" {
			t.Fatalf("status %d %s", last.Code, last.Body.String())
		}
		if tokens.lookups != 30 {
			t.Fatalf("lookups %d", tokens.lookups)
		}
		data := decode(t, last)["error"].(map[string]any)["data"].(map[string]any)
		if data["message"] != "Too many sign-in attempts." || data["retryable"] != true {
			t.Fatal(data)
		}
		spoof := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "muster_nope", "203.0.113.10:4000", "203.0.113.9", "")
		if spoof.Code != http.StatusUnauthorized {
			t.Fatalf("spoof status %d %s", spoof.Code, spoof.Body.String())
		}
		// A Docker bridge peer is trusted, so the forwarded client is the bucket.
		middleware.ResetMCPLimits()
		tokens.lookups = 0
		var bridged *httptest.ResponseRecorder
		for i := 0; i < 31; i++ {
			bridged = postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "muster_nope", "172.18.0.1:4000", "198.51.100.40", "")
		}
		if bridged.Code != http.StatusTooManyRequests {
			t.Fatalf("bridge status %d %s", bridged.Code, bridged.Body.String())
		}
		otherClient := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "muster_nope", "172.18.0.1:4000", "198.51.100.41", "")
		if otherClient.Code != http.StatusUnauthorized {
			t.Fatalf("forwarded client status %d %s", otherClient.Code, otherClient.Body.String())
		}
	})
}

func TestMcpPanicIsInternal(t *testing.T) {
	middleware.ResetMCPLimits()
	owner := primitive.NewObjectID()
	tokens := &fakeTokens{active: map[string]*models.McpToken{"muster_live": {UserID: owner}}}
	users := &fakeUsers{byID: map[string]*models.User{owner.Hex(): {Id: owner, Email: "a@b.com"}}}
	events := &fakeEvents{panicOnGet: true}
	r := engine(mcp.Deps{Tokens: tokens, Users: users, Events: events}, true)
	rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_get_event","arguments":{"event_id":"`+primitive.NewObjectID().Hex()+`"}}}`, "muster_live", "", "", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "db down") {
		t.Fatal(rec.Body.String())
	}
	body := decode(t, rec)
	result := body["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if result["isError"] != true || structured["code"] != "INTERNAL" || structured["retryable"] != true {
		t.Fatal(rec.Body.String())
	}
}

func TestMcpWrites(t *testing.T) {
	middleware.ResetMCPLimits()
	ownerID := primitive.NewObjectID()
	guestID := primitive.NewObjectID()
	strangerID := primitive.NewObjectID()
	eventID := primitive.NewObjectID()
	blind := utils.TruePtr()
	event := &models.Event{Id: eventID, OwnerId: ownerID, Name: "Panel", Type: models.GROUP, BlindAvailabilityEnabled: blind}
	owner := &models.User{Id: ownerID, Email: "owner@x.com", FirstName: "Ada"}
	guest := &models.User{Id: guestID, Email: "guest@x.com", FirstName: "Bea"}
	stranger := &models.User{Id: strangerID, Email: "stranger@x.com", FirstName: "Cara"}
	hiddenWhen := primitive.NewDateTimeFromTime(time.Date(2033, 11, 22, 15, 0, 0, 0, time.UTC))
	events := &fakeEvents{
		byHex: map[string]*models.Event{eventID.Hex(): event},
		attendees: map[string][]models.Attendee{
			eventID.Hex(): {{Email: "guest@x.com", EventId: eventID}},
		},
		responses: map[string][]models.EventResponse{
			eventID.Hex(): {
				{UserId: ownerID.Hex(), Response: &models.Response{UserId: ownerID, Name: "Ada", Availability: []primitive.DateTime{hiddenWhen}}},
				{UserId: guestID.Hex(), Response: &models.Response{UserId: guestID, Name: "Bea", Availability: []primitive.DateTime{hiddenWhen}}},
			},
		},
	}
	tokens := &fakeTokens{active: map[string]*models.McpToken{
		"muster_owner":    {UserID: ownerID},
		"muster_guest":    {UserID: guestID},
		"muster_stranger": {UserID: strangerID},
	}}
	users := &fakeUsers{byID: map[string]*models.User{
		ownerID.Hex(): owner, guestID.Hex(): guest, strangerID.Hex(): stranger,
	}}
	r := engine(mcp.Deps{Tokens: tokens, Users: users, Events: events}, true)
	tool := func(token, name, args string) (map[string]any, map[string]any) {
		t.Helper()
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":`+args+`}}`, token, "", "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status %d %s", name, rec.Code, rec.Body.String())
		}
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		return result, result["structuredContent"].(map[string]any)
	}

	t.Run("stranger cannot write", func(t *testing.T) {
		result, structured := tool("muster_stranger", "muster_set_availability", `{"event_id":"`+eventID.Hex()+`","availability":["2033-11-22T15:00:00Z"]}`)
		if result["isError"] != true || structured["code"] != "EVENT_NOT_FOUND_OR_NO_ACCESS" || events.replaced != nil {
			t.Fatal(structured)
		}
	})

	t.Run("set replaces and does not keep the old mark", func(t *testing.T) {
		_, first := tool("muster_owner", "muster_set_availability", `{"event_id":"`+eventID.Hex()+`","availability":["2033-11-22T15:00:00Z","2033-11-22T15:15:00Z"]}`)
		if first["replaced"] != true || first["availability"] != float64(2) {
			t.Fatal(first)
		}
		_, second := tool("muster_owner", "muster_set_availability", `{"event_id":"`+eventID.Hex()+`","availability":["2033-11-22T16:00:00Z"]}`)
		if second["availability"] != float64(1) || len(events.replaced) != 1 || events.replaced[0].Time().UTC().Format(time.RFC3339) != "2033-11-22T16:00:00Z" {
			t.Fatalf("stored %#v body %#v", events.replaced, second)
		}
	})

	t.Run("guest cannot invite and owner can", func(t *testing.T) {
		result, structured := tool("muster_guest", "muster_add_attendees", `{"event_id":"`+eventID.Hex()+`","emails":["new@x.com"]}`)
		if result["isError"] != true || structured["code"] != "NOT_OWNER" || events.added != nil {
			t.Fatal(structured)
		}
		_, added := tool("muster_owner", "muster_add_attendees", `{"event_id":"`+eventID.Hex()+`","emails":["new@x.com","owner@x.com"]}`)
		if added["email_sent"] != false {
			t.Fatal(added)
		}
		list, _ := added["added"].([]any)
		skipped, _ := added["skipped"].([]any)
		if len(list) != 1 || list[0] != "new@x.com" || len(skipped) != 1 || skipped[0] != "owner@x.com" {
			t.Fatal(added)
		}
		events.attendees[eventID.Hex()] = append(events.attendees[eventID.Hex()], models.Attendee{Email: "new@x.com", EventId: eventID})
		_, again := tool("muster_owner", "muster_add_attendees", `{"event_id":"`+eventID.Hex()+`","emails":["Bob <New@X.com>","guest@x.com"]}`)
		againList, _ := again["added"].([]any)
		againSkipped, _ := again["skipped"].([]any)
		if len(againList) != 0 || len(againSkipped) != 2 || againSkipped[0] != "new@x.com" || againSkipped[1] != "guest@x.com" {
			t.Fatal(again)
		}
	})

	t.Run("only an invitee can decline", func(t *testing.T) {
		result, structured := tool("muster_owner", "muster_decline", `{"event_id":"`+eventID.Hex()+`"}`)
		if result["isError"] != true || structured["code"] != "NOT_AN_INVITEE" || events.declined {
			t.Fatal(structured)
		}
		_, ok := tool("muster_guest", "muster_decline", `{"event_id":"`+eventID.Hex()+`"}`)
		if ok["declined"] != true || !events.declined {
			t.Fatal(ok)
		}
	})

	t.Run("blind non-owner summary has no name time or count", func(t *testing.T) {
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"muster_summarize_overlap","arguments":{"event_id":"`+eventID.Hex()+`","start":"2033-11-22T00:00:00Z","end":"2033-11-23T00:00:00Z"}}}`, "muster_guest", "", "", "")
		raw := rec.Body.String()
		if strings.Contains(raw, "Ada") || strings.Contains(raw, "Bea") || strings.Contains(raw, "2033-11-22T15:00:00Z") || strings.Contains(raw, "count") {
			t.Fatal(raw)
		}
		body := decode(t, rec)
		structured := body["result"].(map[string]any)["structuredContent"].(map[string]any)
		if structured["code"] != "OVERLAP_HIDDEN" {
			t.Fatal(raw)
		}
	})

	t.Run("tools list marks reads and the replace write", func(t *testing.T) {
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "muster_owner", "", "", "")
		body := decode(t, rec)
		tools := body["result"].(map[string]any)["tools"].([]any)
		var set map[string]any
		for _, item := range tools {
			tool := item.(map[string]any)
			if tool["name"] == "muster_set_availability" {
				set = tool
			}
		}
		ann := set["annotations"].(map[string]any)
		if ann["destructiveHint"] != true || ann["readOnlyHint"] != false {
			t.Fatal(ann)
		}
	})

	t.Run("dated muster rejects an outside slot and a duplicate", func(t *testing.T) {
		hours := float32(1)
		event.Type = models.SPECIFIC_DATES
		event.Duration = &hours
		event.Dates = []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2033, 11, 22, 15, 0, 0, 0, time.UTC))}
		result, structured := tool("muster_owner", "muster_set_availability", `{"event_id":"`+eventID.Hex()+`","availability":["2030-01-01T04:30:00Z"]}`)
		if result["isError"] != true || structured["code"] != "SLOT_OUTSIDE" {
			t.Fatal(structured)
		}
		_, stored := tool("muster_owner", "muster_set_availability", `{"event_id":"`+eventID.Hex()+`","availability":["2033-11-22T15:00:00Z","2033-11-22T15:00:00Z","2033-11-22T15:15:00Z"]}`)
		if stored["availability"] != float64(2) || stored["duplicates_ignored"] != float64(1) {
			t.Fatal(stored)
		}
		both, bad := tool("muster_owner", "muster_set_availability", `{"event_id":"`+eventID.Hex()+`","availability":["2033-11-22T15:00:00Z"],"if_needed":["2033-11-22T15:00:00Z"]}`)
		if both["isError"] != true || bad["code"] != "SLOT_IN_BOTH" {
			t.Fatal(bad)
		}
		event.Dates = nil
		event.Type = models.GROUP
	})

	t.Run("get event returns the grid and a trimmed name", func(t *testing.T) {
		hours := float32(2)
		event.Name = "Panel "
		event.Duration = &hours
		event.Dates = []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2033, 11, 22, 15, 0, 0, 0, time.UTC))}
		_, structured := tool("muster_owner", "muster_get_event", `{"event_id":"`+eventID.Hex()+`"}`)
		if structured["name"] != "Panel" || structured["is_group"] != true || structured["slot_minutes"] != float64(15) || structured["duration_hours"] != float64(2) {
			t.Fatal(structured)
		}
		dates, _ := structured["dates"].([]any)
		if len(dates) != 1 {
			t.Fatal(structured)
		}
	})

	t.Run("empty response name is filled from the user", func(t *testing.T) {
		events.responses[eventID.Hex()] = []models.EventResponse{
			{UserId: ownerID.Hex(), Response: &models.Response{UserId: ownerID, Availability: []primitive.DateTime{hiddenWhen}}},
		}
		_, structured := tool("muster_owner", "muster_get_responses", `{"event_id":"`+eventID.Hex()+`","start":"2033-11-22T00:00:00Z","end":"2033-11-23T00:00:00Z"}`)
		raw := fmt.Sprint(structured)
		if !strings.Contains(raw, "Ada") {
			t.Fatal(structured)
		}
	})

	t.Run("owner summary names both people", func(t *testing.T) {
		events.responses[eventID.Hex()] = []models.EventResponse{
			{UserId: ownerID.Hex(), Response: &models.Response{UserId: ownerID, Name: "Ada", Availability: []primitive.DateTime{hiddenWhen}}},
			{UserId: guestID.Hex(), Response: &models.Response{UserId: guestID, Name: "Bea", Availability: []primitive.DateTime{hiddenWhen}}},
		}
		_, structured := tool("muster_owner", "muster_summarize_overlap", `{"event_id":"`+eventID.Hex()+`","start":"2033-11-22T00:00:00Z","end":"2033-11-23T00:00:00Z"}`)
		raw := fmt.Sprint(structured)
		if !strings.Contains(raw, "Ada") || !strings.Contains(raw, "Bea") {
			t.Fatal(structured)
		}
	})
}
