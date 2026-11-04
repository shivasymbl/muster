package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// The subtests below are registered here and run from TestMCPTextParity and
// TestMCPFieldFindings (text_parity_test.go), so their names are
// TestMCPTextParity/REQ_TXT_0NN and TestMCPFieldFindings/FEn. They use in-memory
// fakes only. Catalog, instruction and wire-text checks live in
// text_catalog_test.go because they need the routes package.

type matrixCase struct {
	name string
	fn   func(*testing.T)
}

var matrixParity, matrixField []matrixCase

func parity(name string, fn func(*testing.T)) {
	matrixParity = append(matrixParity, matrixCase{name, fn})
}
func field(name string, fn func(*testing.T)) { matrixField = append(matrixField, matrixCase{name, fn}) }

// ---- fixtures -------------------------------------------------------------

type mxStore struct {
	events    map[string]*models.Event
	attendees map[string][]models.Attendee
	responses map[string][]models.EventResponse
	listed    []ListedEvent
	patches   []map[string]any
	added     []string
	removed   []string
	declined  int
	writes    int
	casCalls  int
	dropPatch bool
	changes   []models.EventResponse
	invites   []models.Attendee
}

func (s *mxStore) GetByEitherID(id string) *models.Event {
	for _, e := range s.events {
		if e.Id.Hex() == id || (e.ShortId != nil && *e.ShortId == id) {
			return e
		}
	}
	return nil
}
func (s *mxStore) Responses(id primitive.ObjectID) []models.EventResponse {
	return s.responses[id.Hex()]
}
func (s *mxStore) Attendees(id primitive.ObjectID) []models.Attendee { return s.attendees[id.Hex()] }
func (s *mxStore) ListForUser(_ *models.User, _ string, limit int, cursor string) ([]ListedEvent, string, error) {
	start := 0
	if cursor != "" {
		for i, row := range s.listed {
			if row.Event.Id.Hex() == cursor {
				start = i + 1
			}
		}
	}
	end := start + limit
	next := ""
	if end < len(s.listed) {
		next = s.listed[end-1].Event.Id.Hex()
	} else {
		end = len(s.listed)
	}
	return s.listed[start:end], next, nil
}
func (s *mxStore) ResponseFlags(userHex string, ids []primitive.ObjectID) map[string]ResponseFlags {
	out := map[string]ResponseFlags{}
	for _, id := range ids {
		for _, row := range s.responses[id.Hex()] {
			if row.UserId == userHex && row.Response != nil {
				out[id.Hex()] = ResponseFlags{Exists: true, NonEmpty: len(row.Response.Availability)+len(row.Response.IfNeeded) > 0}
			}
		}
	}
	return out
}
func (s *mxStore) put(eventID primitive.ObjectID, user *models.User, a, n []primitive.DateTime) {
	rows := s.responses[eventID.Hex()][:0:0]
	for _, row := range s.responses[eventID.Hex()] {
		if row.UserId != user.Id.Hex() {
			rows = append(rows, row)
		}
	}
	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	s.responses[eventID.Hex()] = append(rows, models.EventResponse{UserId: user.Id.Hex(), Response: &models.Response{
		UserId: user.Id, Name: name, Availability: a, IfNeeded: n}})
}
func (s *mxStore) ReplaceAvailability(user *models.User, id primitive.ObjectID, a, n []primitive.DateTime) error {
	s.writes++
	s.put(id, user, a, n)
	return nil
}
func (s *mxStore) ReplaceAvailabilityIf(user *models.User, id primitive.ObjectID, snap AnswerSnapshot, a, n []primitive.DateTime) (bool, error) {
	s.casCalls++
	cur := callerSnapshot(Deps{Events: s}, user, id)
	if cur.Exists != snap.Exists || !sameInstants(cur.Availability, snap.Availability) || !sameInstants(cur.IfNeeded, snap.IfNeeded) {
		return false, nil
	}
	s.writes++
	s.put(id, user, a, n)
	return true, nil
}
func (s *mxStore) Decline(*models.User, primitive.ObjectID) error { s.declined++; return nil }
func (s *mxStore) AddAttendees(id primitive.ObjectID, emails []string) ([]string, error) {
	s.added = append(s.added, emails...)
	for _, e := range emails {
		s.attendees[id.Hex()] = append(s.attendees[id.Hex()], models.Attendee{Email: e, EventId: id})
	}
	return emails, nil
}
func (s *mxStore) Create(user *models.User, ev models.Event, emails []string) (*models.Event, error) {
	ev.Id = primitive.NewObjectID()
	ev.OwnerId = user.Id
	short := "new123"
	ev.ShortId = &short
	s.events[ev.Id.Hex()] = &ev
	if ev.Type == models.GROUP {
		s.attendees[ev.Id.Hex()] = []models.Attendee{{Email: user.Email, EventId: ev.Id}}
		for _, e := range emails {
			s.attendees[ev.Id.Hex()] = append(s.attendees[ev.Id.Hex()], models.Attendee{Email: e, EventId: ev.Id})
		}
	}
	return &ev, nil
}
func (s *mxStore) Patch(id primitive.ObjectID, set map[string]any) error {
	s.patches = append(s.patches, set)
	if s.dropPatch {
		return nil
	}
	ev := s.events[id.Hex()]
	if v, ok := set["name"].(string); ok {
		ev.Name = v
	}
	if v, ok := set["isArchived"].(bool); ok {
		ev.IsArchived = &v
	}
	if v, ok := set["blindAvailabilityEnabled"].(bool); ok {
		ev.BlindAvailabilityEnabled = &v
	}
	if v, ok := set["scheduledEvent"].(models.CalendarEvent); ok {
		ev.ScheduledEvent = &v
	}
	if v, ok := set["dates"].([]primitive.DateTime); ok {
		ev.Dates = v
	}
	if v, ok := set["duration"].(float32); ok {
		ev.Duration = &v
	}
	return nil
}
func (s *mxStore) RemoveAttendee(id primitive.ObjectID, email, _ string) error {
	s.removed = append(s.removed, email)
	kept := []models.Attendee{}
	for _, a := range s.attendees[id.Hex()] {
		if a.Email != email {
			kept = append(kept, a)
		}
	}
	s.attendees[id.Hex()] = kept
	return nil
}
func (s *mxStore) Changes(primitive.ObjectID, time.Time) ([]models.EventResponse, []models.Attendee, error) {
	return s.changes, s.invites, nil
}

type mxUsers struct{ all []*models.User }

func (u mxUsers) GetByID(id string) *models.User {
	for _, x := range u.all {
		if x.Id.Hex() == id {
			return x
		}
	}
	return nil
}
func (u mxUsers) GetByEmail(email string) *models.User {
	for _, x := range u.all {
		if strings.EqualFold(x.Email, email) {
			return x
		}
	}
	return nil
}

type mxWorld struct {
	s                          *mxStore
	d                          Deps
	owner, alice, bob, outside *models.User
}

func day(d, h, m int) primitive.DateTime {
	return primitive.NewDateTimeFromTime(time.Date(2026, 6, d, h, m, 0, 0, time.UTC))
}

func newUser(first, email string) *models.User {
	return &models.User{Id: primitive.NewObjectID(), Email: email, FirstName: first}
}

func newWorld() *mxWorld {
	w := &mxWorld{
		s:     &mxStore{events: map[string]*models.Event{}, attendees: map[string][]models.Attendee{}, responses: map[string][]models.EventResponse{}},
		owner: newUser("Olive", "owner@x.com"), alice: newUser("Alice", "alice@x.com"),
		bob: newUser("Bob", "bob@x.com"), outside: newUser("Eve", "eve@x.com"),
	}
	w.d = Deps{Events: w.s, Users: mxUsers{[]*models.User{w.owner, w.alice, w.bob, w.outside}}}
	return w
}

// event adds a muster on 2026-06-01 and 2026-06-02, 09:00 for 8 hours, 30-minute slots.
func (w *mxWorld) event(kind models.EventType, mutate ...func(*models.Event)) *models.Event {
	hours, slot, short := float32(8), 30, "ev"+primitive.NewObjectID().Hex()[18:]
	ev := &models.Event{Id: primitive.NewObjectID(), ShortId: &short, OwnerId: w.owner.Id, Name: "Plan", Type: kind,
		Dates: []primitive.DateTime{day(1, 9, 0), day(2, 9, 0)}, Duration: &hours, TimeIncrement: &slot}
	for _, m := range mutate {
		m(ev)
	}
	w.s.events[ev.Id.Hex()] = ev
	return ev
}

// group adds a group muster with the owner, alice and bob on the invite list.
func (w *mxWorld) group(mutate ...func(*models.Event)) *models.Event {
	ev := w.event(models.GROUP, mutate...)
	w.s.attendees[ev.Id.Hex()] = []models.Attendee{{Email: w.owner.Email}, {Email: w.alice.Email}, {Email: w.bob.Email}}
	return ev
}

// run calls a tool and returns the result as the JSON object a client would parse.
func (w *mxWorld) ok(t *testing.T, user *models.User, tool string, ev *models.Event, args map[string]any) map[string]any {
	t.Helper()
	out, bad := w.run(user, tool, ev, args)
	if bad != nil {
		t.Fatalf("%s: %s: %s (%s)", tool, bad.Code, bad.Message, bad.WhatToDoNext)
	}
	return out
}

func (w *mxWorld) run(user *models.User, tool string, ev *models.Event, args map[string]any) (map[string]any, *ToolError) {
	if args == nil {
		args = map[string]any{}
	}
	if ev != nil {
		args["event_id"] = ev.Id.Hex()
	}
	out, bad, _ := Call(w.d, user, tool, args)
	if bad != nil {
		return nil, bad
	}
	return asMap(out), nil
}

func asMap(v any) map[string]any {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		panic(err)
	}
	return m
}

func must(t *testing.T, out map[string]any, bad *ToolError) map[string]any {
	t.Helper()
	if bad != nil {
		t.Fatalf("%s: %s (%s)", bad.Code, bad.Message, bad.WhatToDoNext)
	}
	return out
}

func wantCode(t *testing.T, bad *ToolError, code string) {
	t.Helper()
	if bad == nil || bad.Code != code {
		t.Fatalf("want %s, got %#v", code, bad)
	}
}

func errData(t *testing.T, bad *ToolError) map[string]any {
	t.Helper()
	if bad == nil {
		t.Fatal("no error")
	}
	m := asMap(bad)
	data, ok := m["data"].(map[string]any)
	if !ok {
		t.Fatalf("no canonical data object: %#v", m)
	}
	return data
}

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not an object: %#v", v)
	}
	return m
}

func list(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("not a list: %#v", v)
	}
	return l
}

func strs(t *testing.T, v any) []string {
	t.Helper()
	out := []string{}
	for _, x := range list(t, v) {
		out = append(out, x.(string))
	}
	return out
}

func rfc(h, m int) string { return time.Date(2026, 6, 1, h, m, 0, 0, time.UTC).Format(time.RFC3339) }

// ---- REQ_TXT_002 / FE2 ----------------------------------------------------

func updateEchoCase(t *testing.T) {
	w := newWorld()
	ev := w.group()
	got := w.ok(t, w.owner, "muster_get_event", ev, nil)
	out := w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"name": "Renamed"})
	if out["event_id"] != ev.Id.Hex() || out["updated"] != true {
		t.Fatalf("legacy keys lost: %#v", out)
	}
	echo := obj(t, out["event"])
	if echo["name"] != "Renamed" {
		t.Fatalf("event echo not post-write: %#v", echo)
	}
	for k := range got {
		if _, ok := echo[k]; !ok {
			t.Fatalf("event echo lacks get_event key %q", k)
		}
	}
	out = w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"archive": true})
	if obj(t, out["event"])["is_archived"] != true {
		t.Fatalf("archive not echoed: %#v", out)
	}
	// The echo is a read of what persisted, not the submitted input.
	w.s.dropPatch = true
	out = w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"name": "Dropped"})
	if obj(t, out["event"])["name"] == "Dropped" {
		t.Fatalf("echo repeated submitted input: %#v", out)
	}
}

func init() {
	parity("REQ_TXT_002", updateEchoCase)
	field("FE2", updateEchoCase)
}

// ---- REQ_TXT_004 / FE4 ----------------------------------------------------

func defaultWindowCase(t *testing.T) {
	for _, tool := range []string{"muster_get_responses", "muster_summarize_overlap"} {
		w := newWorld()
		ev := w.event(models.SPECIFIC_DATES)
		w.s.put(ev.Id, w.owner, []primitive.DateTime{day(1, 10, 0), day(1, 10, 30)}, nil)
		for name, args := range map[string]map[string]any{
			"omitted": {}, "null": {"start": nil, "end": nil},
		} {
			out := w.ok(t, w.owner, tool, ev, args)
			if out["start"] != "2026-06-01T09:00:00Z" {
				t.Fatalf("%s %s: window start %v", tool, name, out["start"])
			}
			end, err := time.Parse(time.RFC3339, out["end"].(string))
			if err != nil || end.Sub(time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)) > 7*24*time.Hour || !end.After(time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)) {
				t.Fatalf("%s %s: window end %v", tool, name, out["end"])
			}
		}
		// Explicit windows keep working and are echoed.
		out := w.ok(t, w.owner, tool, ev, map[string]any{"start": rfc(9, 0), "end": rfc(12, 0)})
		if out["start"] != rfc(9, 0) || out["end"] != rfc(12, 0) {
			t.Fatalf("%s explicit: %#v", tool, out)
		}
		for name, args := range map[string]map[string]any{
			"start only":    {"start": rfc(9, 0)},
			"end only":      {"end": rfc(12, 0)},
			"start+null":    {"start": rfc(9, 0), "end": nil},
			"backwards":     {"start": rfc(12, 0), "end": rfc(9, 0)},
			"not a time":    {"start": "tomorrow", "end": "later"},
			"empty strings": {"start": "", "end": ""},
		} {
			_, bad := w.run(w.owner, tool, ev, args)
			if bad == nil || bad.Code != "BAD_ARGUMENTS" {
				t.Fatalf("%s %s: %#v", tool, name, bad)
			}
		}
		_, bad := w.run(w.owner, tool, ev, map[string]any{"start": rfc(0, 0), "end": "2026-06-09T00:00:00Z"})
		wantCode(t, bad, "WINDOW_TOO_WIDE")

		// A ten-day muster still defaults to at most seven days.
		long := w.event(models.SPECIFIC_DATES, func(e *models.Event) {
			e.Dates = []primitive.DateTime{day(1, 9, 0), day(11, 9, 0)}
		})
		out = w.ok(t, w.owner, tool, long, nil)
		s, _ := time.Parse(time.RFC3339, out["start"].(string))
		e, _ := time.Parse(time.RFC3339, out["end"].(string))
		if e.Sub(s) > 7*24*time.Hour {
			t.Fatalf("%s default window %v..%v is over 7 days", tool, s, e)
		}
		// An undated muster has no window to derive: actionable error, no invented dates.
		undated := w.group(func(e *models.Event) { e.Dates = nil })
		_, bad = w.run(w.owner, tool, undated, nil)
		if bad == nil || bad.Code != "BAD_ARGUMENTS" || bad.WhatToDoNext == "" || !strings.Contains(bad.WhatToDoNext, "RFC3339") {
			t.Fatalf("%s undated: %#v", tool, bad)
		}
	}
}

func init() {
	parity("REQ_TXT_004", defaultWindowCase)
	field("FE4", defaultWindowCase)
}

// ---- REQ_TXT_005: canonical error data ------------------------------------

func errorDataCase(t *testing.T) {
	w := newWorld()
	ev := w.event(models.SPECIFIC_DATES)
	grp := w.group()
	check := func(bad *ToolError, flat, canonical string) map[string]any {
		t.Helper()
		wantCode(t, bad, flat)
		if bad.Message == "" || bad.WhatToDoNext == "" {
			t.Fatalf("flat fields lost: %#v", bad)
		}
		data := errData(t, bad)
		if data["code"] != canonical || data["message"] != bad.Message || data["what_to_do_next"] != bad.WhatToDoNext {
			t.Fatalf("data %#v for flat %#v, want canonical %s", data, bad, canonical)
		}
		return data
	}

	_, bad := w.run(w.owner, "muster_set_availability", ev, map[string]any{"availability": []any{rfc(9, 0), rfc(9, 10)}, "if_needed": []any{}})
	data := check(bad, "SLOT_OFF_GRID", "SLOT_OFF_GRID")
	if data["value"] != rfc(9, 10) || data["index"] != float64(1) || !strings.HasPrefix(data["path"].(string), "availability") {
		t.Fatalf("offending value/index/path missing: %#v", data)
	}
	if b := obj(t, data["bounds"]); b["slot_minutes"] != float64(30) || len(list(t, b["dates"])) != 2 || b["duration_hours"] != float64(8) {
		t.Fatalf("valid bounds missing for a member: %#v", data)
	}
	_, bad = w.run(w.owner, "muster_set_availability", ev, map[string]any{"availability": []any{"2026-07-01T09:00:00Z"}, "if_needed": []any{}})
	check(bad, "SLOT_OUTSIDE", "SLOT_OUTSIDE")
	_, bad = w.run(w.owner, "muster_set_availability", ev, map[string]any{"availability": []any{rfc(9, 0)}, "if_needed": []any{rfc(9, 0)}})
	check(bad, "SLOT_IN_BOTH", "SLOT_IN_BOTH")
	_, bad = w.run(w.owner, "muster_set_availability", ev, map[string]any{"ranges": []any{
		rng(rfc(9, 0), rfc(9, 30), "available"), rng(rfc(10, 0), rfc(10, 30), "maybe")}})
	data = check(bad, "BAD_ARGUMENTS", "BAD_ARGUMENTS")
	if data["index"] != float64(1) || data["path"] != "ranges[1]" {
		t.Fatalf("range index: %#v", data)
	}
	_, bad = w.run(w.owner, "muster_summarize_overlap", ev, map[string]any{"start": rfc(0, 0), "end": "2026-06-09T00:00:00Z"})
	check(bad, "WINDOW_TOO_WIDE", "WINDOW_TOO_LONG")
	_, bad = w.run(w.owner, "muster_decline", ev, nil)
	check(bad, "BAD_ARGUMENTS", "NOT_GROUP")
	_, bad = w.run(w.owner, "muster_remove_attendee", grp, map[string]any{"email": w.owner.Email})
	check(bad, "BAD_ARGUMENTS", "OWNER_NOT_REMOVABLE")
	_, bad = w.run(w.alice, "muster_finalize", grp, map[string]any{"start": rfc(9, 0), "end": rfc(10, 0)})
	check(bad, "NOT_OWNER", "NOT_OWNER")
	blind := w.event(models.SPECIFIC_DATES, func(e *models.Event) { e.BlindAvailabilityEnabled = utils.TruePtr() })
	w.s.attendees[blind.Id.Hex()] = []models.Attendee{{Email: w.alice.Email}}
	_, bad = w.run(w.alice, "muster_summarize_overlap", blind, nil)
	check(bad, "OVERLAP_HIDDEN", "OVERLAP_HIDDEN")

	// Nondisclosing: unknown and inaccessible look the same and carry no bounds.
	_, hidden := w.run(w.outside, "muster_set_availability", ev, map[string]any{"availability": []any{rfc(9, 10)}, "if_needed": []any{}})
	check(hidden, "EVENT_NOT_FOUND_OR_NO_ACCESS", "NOT_FOUND")
	_, bad2, _ := Call(w.d, w.outside, "muster_get_event", map[string]any{"event_id": "does-not-exist"})
	if hidden.Message != bad2.Message || hidden.WhatToDoNext != bad2.WhatToDoNext {
		t.Fatalf("discloses existence: %#v vs %#v", hidden, bad2)
	}
	if d := errData(t, hidden); d["bounds"] != nil || d["value"] != nil {
		t.Fatalf("stranger learned bounds: %#v", d)
	}
	// Offending values are bounded.
	_, bad = w.run(w.owner, "muster_set_availability", ev, map[string]any{"availability": []any{strings.Repeat("x", 5000)}, "if_needed": []any{}})
	if v, _ := errData(t, bad)["value"].(string); v == "" || utf8.RuneCountInString(v) > 100 {
		t.Fatalf("offending value is not bounded: %d runes", utf8.RuneCountInString(v))
	}
}

func init() { parity("REQ_TXT_005", errorDataCase) }

// ---- REQ_TXT_007 / FE7 ----------------------------------------------------

func scheduledClosedCase(t *testing.T) {
	w := newWorld()
	ev := w.group()
	get := func() map[string]any { return w.ok(t, w.owner, "muster_get_event", ev, nil) }
	row := get()
	for _, k := range []string{"event_id", "name", "type", "is_archived", "blind_availability", "my_roles", "my_status", "page_url", "dates", "slot_minutes", "duration_hours", "days_only", "start_on_monday", "is_group"} {
		if _, ok := row[k]; !ok {
			t.Fatalf("legacy key %s lost", k)
		}
	}
	if v, ok := row["scheduled"]; !ok || v != nil || row["closed"] != false {
		t.Fatalf("open muster: scheduled=%v closed=%v", row["scheduled"], row["closed"])
	}
	// Manual archive is not a finalization.
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"archive": true})
	if row = get(); row["is_archived"] != true || row["closed"] != false || row["scheduled"] != nil {
		t.Fatalf("archive only: %#v", row)
	}
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"archive": false})

	fin := w.ok(t, w.owner, "muster_finalize", ev, map[string]any{"start": rfc(15, 0), "end": rfc(16, 0)})
	for _, k := range []string{"event_id", "start", "end", "closed", "email_sent"} {
		if _, ok := fin[k]; !ok {
			t.Fatalf("finalize lost %s: %#v", k, fin)
		}
	}
	if s := obj(t, fin["scheduled"]); s["start"] != rfc(15, 0) || s["end"] != rfc(16, 0) || fin["email_sent"] != false {
		t.Fatalf("finalize receipt: %#v", fin)
	}
	row = get()
	if s := obj(t, row["scheduled"]); s["start"] != rfc(15, 0) || s["end"] != rfc(16, 0) || row["closed"] != true {
		t.Fatalf("after finalize: %#v", row)
	}
	// archive:false clears only is_archived; the stored interval and closed stay.
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"archive": false})
	if _, has := w.s.patches[len(w.s.patches)-1]["scheduledEvent"]; has {
		t.Fatal("archive:false touched the scheduled interval")
	}
	if row = get(); row["is_archived"] != false || row["closed"] != true || row["scheduled"] == nil {
		t.Fatalf("reopened: %#v", row)
	}
	// List rows carry the same fields.
	w.s.listed = []ListedEvent{{Event: *ev, HasResponse: true, NonEmpty: true}}
	page := w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all"})
	first := obj(t, list(t, page["events"])[0])
	if first["closed"] != true || first["scheduled"] == nil {
		t.Fatalf("list row: %#v", first)
	}
}

func init() {
	parity("REQ_TXT_007", scheduledClosedCase)
	field("FE7", scheduledClosedCase)
}

// ---- REQ_TXT_008 / FE8 ----------------------------------------------------

func skippedDetailsCase(t *testing.T) {
	w := newWorld()
	ev := w.group()
	out := w.ok(t, w.owner, "muster_add_attendees", ev, map[string]any{"emails": []any{
		"alice@x.com", "owner@x.com", "new@x.com", "new@x.com", "NEW@x.com", "fresh@x.com"}})
	if got := strs(t, out["skipped"]); len(got) == 0 || got[0] != "alice@x.com" {
		t.Fatalf("skipped must stay a string list: %#v", out["skipped"])
	}
	if got := strs(t, out["added"]); len(got) != 2 || got[0] != "new@x.com" || got[1] != "fresh@x.com" {
		t.Fatalf("added: %#v", out["added"])
	}
	type detail struct {
		email, reason, basis string
		index                int
	}
	got := []detail{}
	for _, raw := range list(t, out["skipped_details"]) {
		d := obj(t, raw)
		got = append(got, detail{d["email"].(string), d["reason"].(string), d["basis"].(string), int(d["input_index"].(float64))})
	}
	want := []detail{
		{"alice@x.com", "already_invited", "existing_attendee", 0},
		{"owner@x.com", "owner", "owner", 1},
		{"new@x.com", "already_invited", "earlier_input", 3},
		{"new@x.com", "already_invited", "earlier_input", 4},
	}
	if len(got) != len(want) {
		t.Fatalf("details %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("detail %d: %#v want %#v", i, got[i], want[i])
		}
	}
	// A new address repeated in the input is in added AND skipped: the arrays are not disjoint.
	if !contains(strs(t, out["added"]), "new@x.com") || !contains(strs(t, out["skipped"]), "new@x.com") {
		t.Fatalf("legacy arrays should both name the repeated new address: %#v", out)
	}

	// An invalid address rejects the whole batch with a per-input diagnostic and no partial write.
	before := len(w.s.attendees[ev.Id.Hex()])
	_, bad := w.run(w.owner, "muster_add_attendees", ev, map[string]any{"emails": []any{"good@x.com", "not-an-email", "also good@"}})
	wantCode(t, bad, "BAD_ARGUMENTS")
	d := errData(t, bad)
	if d["index"] != float64(1) || d["path"] != "emails[1]" || d["value"] != "not-an-email" {
		t.Fatalf("invalid-input diagnostic: %#v", d)
	}
	if len(w.s.attendees[ev.Id.Hex()]) != before {
		t.Fatal("invalid batch partially applied")
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func init() {
	parity("REQ_TXT_008", skippedDetailsCase)
	field("FE8", skippedDetailsCase)
}

// ---- REQ_TXT_010 / FE10 ---------------------------------------------------

func rankingCase(t *testing.T) {
	w := newWorld()
	ev := w.event(models.SPECIFIC_DATES, func(e *models.Event) { slot := 30; e.TimeIncrement = &slot })
	names := []string{"Sam", "Sam", "Ann", "Ann"}
	users := []*models.User{}
	for _, n := range names {
		users = append(users, newUser(n, n+primitive.NewObjectID().Hex()+"@x.com"))
	}
	w.s.put(ev.Id, w.owner, []primitive.DateTime{day(1, 15, 0), day(1, 15, 30)}, nil)  // free
	w.s.put(ev.Id, users[0], nil, []primitive.DateTime{day(1, 15, 0), day(1, 15, 30)}) // conditional only
	w.s.put(ev.Id, users[1], nil, nil)                                                 // unavailable, same display name
	w.s.put(ev.Id, users[2], []primitive.DateTime{day(1, 15, 0)}, []primitive.DateTime{day(1, 15, 30)})
	w.s.put(ev.Id, users[3], nil, nil)
	out := w.ok(t, w.owner, "muster_find_best_times", ev, map[string]any{"duration_minutes": float64(60)})
	slot := obj(t, list(t, out["slots"])[0])
	if slot["start"] != rfc(15, 0) || slot["available"] != float64(1) {
		t.Fatalf("slot %#v", slot)
	}
	// Legacy meaning is untouched: missing is everyone not fully available, if_needed counts conditional people.
	if len(strs(t, slot["missing"])) != 4 || slot["if_needed"] != float64(2) {
		t.Fatalf("legacy categories changed: %#v", slot)
	}
	cond, unavail := strs(t, slot["if_needed_names"]), strs(t, slot["unavailable_names"])
	if len(cond) != 2 || len(unavail) != 2 {
		t.Fatalf("disjoint names: %#v", slot)
	}
	if slot["if_needed_count"] != float64(2) || slot["unavailable_count"] != float64(2) {
		t.Fatalf("counts: %#v", slot)
	}
	// Duplicate display names are separate people, never merged.
	if cond[0] != "Ann" && cond[0] != "Sam" {
		t.Fatalf("names: %v", cond)
	}
	// Invalid durations are rejected, not rounded or promoted.
	for _, minutes := range []any{float64(0), float64(-30), float64(20), float64(45), 30.5, "30"} {
		_, bad := w.run(w.owner, "muster_find_best_times", ev, map[string]any{"duration_minutes": minutes})
		if bad == nil || bad.Code != "BAD_ARGUMENTS" {
			t.Fatalf("duration %v: %#v", minutes, bad)
		}
	}
	// 20 minutes on a 30-minute grid must not become a 30-minute result for a conditional-only respondent.
	solo := w.event(models.SPECIFIC_DATES)
	w.s.put(solo.Id, users[0], nil, []primitive.DateTime{day(1, 15, 0)})
	_, bad := w.run(w.owner, "muster_find_best_times", solo, map[string]any{"duration_minutes": float64(20)})
	wantCode(t, bad, "BAD_ARGUMENTS")
}

func init() {
	parity("REQ_TXT_010", rankingCase)
	field("FE10", rankingCase)
}

// ---- REQ_TXT_011 / FE13 ---------------------------------------------------

func listFilterCase(t *testing.T) {
	w := newWorld()
	mk := func(name string, archived bool) *models.Event {
		return w.event(models.SPECIFIC_DATES, func(e *models.Event) { e.Name = name; e.IsArchived = &archived })
	}
	answered, openA, openB, arch1, arch2 := mk("Answered", false), mk("OpenA", false), mk("OpenB", false), mk("Arch1", true), mk("Arch2", true)
	w.s.listed = []ListedEvent{
		{Event: *arch1}, {Event: *arch2}, {Event: *answered, HasResponse: true, NonEmpty: true}, {Event: *openA}, {Event: *openB},
	}
	names := func(page map[string]any) []string {
		out := []string{}
		for _, r := range list(t, page["events"]) {
			out = append(out, obj(t, r)["name"].(string))
		}
		return out
	}
	eq := func(got []string, want ...string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	// Owner decision: default is needs_answer; status all returns everything.
	eq(names(w.ok(t, w.owner, "muster_list_my_events", nil, nil)), "Arch1", "Arch2", "OpenA", "OpenB")
	eq(names(w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all"})), "Arch1", "Arch2", "Answered", "OpenA", "OpenB")
	eq(names(w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "open"})), "Answered", "OpenA", "OpenB")
	eq(names(w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "archived"})), "Arch1", "Arch2")
	eq(names(w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all", "needs_my_answer": true})), "Arch1", "Arch2", "OpenA", "OpenB")
	eq(names(w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "open", "needs_my_answer": true})), "OpenA", "OpenB")
	eq(names(w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all", "needs_my_answer": false})), "Arch1", "Arch2", "Answered", "OpenA", "OpenB")

	// Filtering happens before the page is cut: limit 1 of "open" is the first open muster, with a cursor to the next.
	page := w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "open", "limit": float64(1)})
	eq(names(page), "Answered")
	if page["next_cursor"] == "" || page["next_cursor"] == nil {
		t.Fatalf("filtered page lost its cursor: %#v", page)
	}
	page = w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "open", "limit": float64(1), "cursor": page["next_cursor"]})
	eq(names(page), "OpenA")

	// Applied filters are stated.
	f := obj(t, w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "open", "needs_my_answer": true, "projection": "compact"})["filters"])
	if f["status"] != "open" || f["needs_my_answer"] != true || f["projection"] != "compact" {
		t.Fatalf("filters not stated: %#v", f)
	}
	f = obj(t, w.ok(t, w.owner, "muster_list_my_events", nil, nil)["filters"])
	if f["status"] != "needs_answer" || f["projection"] != "full" {
		t.Fatalf("default filters: %#v", f)
	}

	// Compact is explicit; full keeps the grid.
	full := obj(t, list(t, w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all"})["events"])[0])
	compact := obj(t, list(t, w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all", "projection": "compact"})["events"])[0])
	for _, k := range []string{"dates", "duration_hours", "slot_minutes"} {
		if _, ok := full[k]; !ok {
			t.Fatalf("full projection lost %s", k)
		}
		if _, ok := compact[k]; ok {
			t.Fatalf("compact projection kept %s", k)
		}
	}
	for _, k := range []string{"event_id", "name", "my_status", "is_archived", "page_url", "closed"} {
		if _, ok := compact[k]; !ok {
			t.Fatalf("compact lost %s: %#v", k, compact)
		}
	}
	for _, bad := range []map[string]any{{"status": "sideways"}, {"projection": "tiny"}, {"needs_my_answer": "yes"}} {
		_, err := w.run(w.owner, "muster_list_my_events", nil, bad)
		wantCode(t, err, "BAD_ARGUMENTS")
	}
}

func init() {
	parity("REQ_TXT_011", listFilterCase)
	field("FE13", listFilterCase)
}

// ---- REQ_TXT_012: owner-only totals ---------------------------------------

func ownerTotalsCase(t *testing.T) {
	w := newWorld()
	ev := w.group()
	w.s.put(ev.Id, w.alice, []primitive.DateTime{day(1, 9, 0)}, nil)
	w.s.put(ev.Id, w.bob, nil, nil) // cleared
	totals := obj(t, w.ok(t, w.owner, "muster_get_event", ev, nil)["response_totals"])
	want := map[string]float64{"invited": 3, "responded": 1, "cleared": 1, "declined": 0, "pending": 1}
	for k, v := range want {
		if totals[k] != v {
			t.Fatalf("%s = %v, want %v: %#v", k, totals[k], v, totals)
		}
	}
	if p, _ := totals["population"].(string); p == "" || !strings.Contains(p, "owner") {
		t.Fatalf("population is not defined: %#v", totals)
	}
	if totals["owner_is_participant"] != true || totals["owner_status"] != "invited" {
		t.Fatalf("owner participation: %#v", totals)
	}
	// Declined invitees are counted separately from pending.
	yes := true
	w.s.attendees[ev.Id.Hex()][2].Declined = &yes
	totals = obj(t, w.ok(t, w.owner, "muster_get_event", ev, nil)["response_totals"])
	if totals["declined"] != float64(1) || totals["pending"] != float64(1) {
		t.Fatalf("declined: %#v", totals)
	}
	// Non-owners and hidden views get null, never zero.
	for _, who := range []*models.User{w.alice, w.bob} {
		row := w.ok(t, who, "muster_get_event", ev, nil)
		if v, ok := row["response_totals"]; !ok || v != nil {
			t.Fatalf("non-owner totals must be present and null: %#v", row)
		}
	}
	blind := w.group(func(e *models.Event) { e.BlindAvailabilityEnabled = utils.TruePtr() })
	row := w.ok(t, w.alice, "muster_get_event", blind, nil)
	if v, ok := row["response_totals"]; !ok || v != nil {
		t.Fatalf("blind non-owner totals: %#v", row)
	}
	// A plain (non-group) muster counts the people with a stored answer.
	plain := w.event(models.SPECIFIC_DATES)
	w.s.put(plain.Id, w.owner, []primitive.DateTime{day(1, 9, 0)}, nil)
	totals = obj(t, w.ok(t, w.owner, "muster_get_event", plain, nil)["response_totals"])
	if totals["responded"] != float64(1) || totals["pending"] != float64(0) {
		t.Fatalf("plain muster: %#v", totals)
	}
}

func init() { parity("REQ_TXT_012", ownerTotalsCase) }

// ---- REQ_TXT_014: display timezone ----------------------------------------

func displayTimezoneCase(t *testing.T) {
	w := newWorld()
	created := w.ok(t, w.owner, "muster_create_event", nil, map[string]any{
		"name": "Zoned", "type": "specific_dates", "duration_hours": float64(2), "slot_minutes": float64(30),
		"dates": []any{"2026-06-01T09:00:00Z"}, "display_timezone": "America/New_York"})
	ev := w.s.GetByEitherID(created["event_id"].(string))
	get := func() map[string]any { return w.ok(t, w.owner, "muster_get_event", ev, nil) }
	if get()["display_timezone"] != "America/New_York" {
		t.Fatalf("display_timezone not stored: %#v", get())
	}
	before := get()["dates"]
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"name": "Zoned 2"})
	if get()["display_timezone"] != "America/New_York" {
		t.Fatal("omitted update must preserve the label")
	}
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"display_timezone": "Europe/Paris"})
	row := get()
	if row["display_timezone"] != "Europe/Paris" || !jsonEqual(row["dates"], before) {
		t.Fatalf("label change moved stored instants: %#v", row)
	}
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"display_timezone": nil})
	if v, ok := get()["display_timezone"]; !ok || v != nil {
		t.Fatal("explicit null must clear the label")
	}
	for _, bad := range []string{"Not/AZone", "UTC+2", "+02:00", "EST5EDT garbage", ""} {
		_, err := w.run(w.owner, "muster_update_event", ev, map[string]any{"display_timezone": bad})
		wantCode(t, err, "BAD_ARGUMENTS")
	}
	// Missing on create means unspecified.
	bare := w.ok(t, w.owner, "muster_create_event", nil, map[string]any{
		"name": "Plain", "type": "specific_dates", "duration_hours": float64(2), "dates": []any{"2026-06-01T09:00:00Z"}})
	row = w.ok(t, w.owner, "muster_get_event", w.s.GetByEitherID(bare["event_id"].(string)), nil)
	if v, ok := row["display_timezone"]; !ok || v != nil {
		t.Fatalf("unspecified must be null: %#v", row)
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func init() { parity("REQ_TXT_014", displayTimezoneCase) }

// ---- REQ_TXT_015 / FE11: owner as participant ------------------------------

func ownerParticipantCase(t *testing.T) {
	w := newWorld()
	ev := w.group()
	w.s.put(ev.Id, w.alice, []primitive.DateTime{day(1, 9, 0)}, nil)
	// Owner decision: the owner stays a participant and can appear in the pending list. Remind sends no mail.
	out := w.ok(t, w.owner, "muster_remind", ev, nil)
	pending := strs(t, out["pending"])
	if !contains(pending, w.owner.Email) || !contains(pending, w.bob.Email) || contains(pending, w.alice.Email) || out["email_sent"] != false {
		t.Fatalf("remind: %#v", out)
	}
	yes := true
	w.s.attendees[ev.Id.Hex()][2].Declined = &yes
	out = w.ok(t, w.owner, "muster_remind", ev, nil)
	if got := strs(t, out["pending"]); contains(got, w.bob.Email) || !contains(got, w.owner.Email) {
		t.Fatalf("declined must drop out, owner stays: %v", got)
	}
	// The attendee view and my_roles keep the owner as a participant.
	people := list(t, w.ok(t, w.owner, "muster_list_attendees", ev, nil)["people"])
	found := false
	for _, p := range people {
		if obj(t, p)["email"] == w.owner.Email {
			found = true
		}
	}
	if !found {
		t.Fatalf("owner row missing: %#v", people)
	}
	roles := strs(t, w.ok(t, w.owner, "muster_get_event", ev, nil)["my_roles"])
	if !contains(roles, "owner") || !contains(roles, "invitee") {
		t.Fatalf("roles: %v", roles)
	}
	// Remind is owner-only and group-only.
	_, bad := w.run(w.alice, "muster_remind", ev, nil)
	wantCode(t, bad, "NOT_OWNER")
	_, bad = w.run(w.owner, "muster_remind", w.event(models.SPECIFIC_DATES), nil)
	wantCode(t, bad, "BAD_ARGUMENTS")
}

func init() {
	parity("REQ_TXT_015", ownerParticipantCase)
	field("FE11", ownerParticipantCase)
}

// ---- REQ_TXT_017: authorization and lifecycle matrix ----------------------

func lifecycleMatrixCase(t *testing.T) {
	w := newWorld()
	ev := w.group()
	ownerOnly := map[string]map[string]any{
		"muster_finalize":        {"start": rfc(9, 0), "end": rfc(10, 0)},
		"muster_update_event":    {"name": "Hijack"},
		"muster_remind":          {},
		"muster_remove_attendee": {"email": w.bob.Email},
		"muster_add_attendees":   {"emails": []any{"x@x.com"}},
		"muster_list_changes":    {"since": "2026-01-01T00:00:00Z"},
	}
	for tool, args := range ownerOnly {
		copied := map[string]any{}
		for k, v := range args {
			copied[k] = v
		}
		_, bad := w.run(w.alice, tool, ev, copied)
		if bad == nil || bad.Code != "NOT_OWNER" {
			t.Fatalf("invitee %s: %#v", tool, bad)
		}
		_, bad = w.run(w.outside, tool, ev, args)
		wantCode(t, bad, "EVENT_NOT_FOUND_OR_NO_ACCESS")
	}
	if len(w.s.patches) != 0 || len(w.s.removed) != 0 || len(w.s.added) != 0 {
		t.Fatalf("denied calls had effects: %v %v %v", w.s.patches, w.s.removed, w.s.added)
	}
	for _, tool := range []string{"muster_get_event", "muster_get_responses", "muster_summarize_overlap", "muster_find_best_times", "muster_list_attendees", "muster_set_availability", "muster_decline"} {
		args := map[string]any{"availability": []any{}, "if_needed": []any{}, "start": rfc(9, 0), "end": rfc(12, 0)}
		_, bad := w.run(w.outside, tool, ev, args)
		wantCode(t, bad, "EVENT_NOT_FOUND_OR_NO_ACCESS")
	}

	// Owner decision: archived musters stay editable; a second finalize is EVENT_ARCHIVED.
	w.ok(t, w.owner, "muster_finalize", ev, map[string]any{"start": rfc(9, 0), "end": rfc(10, 0)})
	_, bad := w.run(w.owner, "muster_finalize", ev, map[string]any{"start": rfc(11, 0), "end": rfc(12, 0)})
	wantCode(t, bad, "EVENT_ARCHIVED")
	if len(w.s.patches) != 1 {
		t.Fatalf("second finalize wrote: %v", w.s.patches)
	}
	out := w.ok(t, w.alice, "muster_set_availability", ev, map[string]any{"availability": []any{rfc(9, 0)}, "if_needed": []any{}})
	if out["changed"] != true || w.s.writes != 1 {
		t.Fatalf("archived muster must stay answerable: %#v", out)
	}
	w.ok(t, w.owner, "muster_set_availability", ev, map[string]any{"ranges": []any{rng(rfc(9, 0), rfc(9, 30), "available")}})
	w.ok(t, w.owner, "muster_add_attendees", ev, map[string]any{"emails": []any{"late@x.com"}})
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"name": "Renamed after close"})
	w.ok(t, w.owner, "muster_update_event", ev, map[string]any{"archive": false})
	if last := w.s.patches[len(w.s.patches)-1]; len(last) != 1 || last["isArchived"] != false {
		t.Fatalf("archive:false must only clear is_archived: %#v", last)
	}
	if ev.ScheduledEvent == nil {
		t.Fatal("archive:false erased the stored interval")
	}
	if _, bad := w.run(w.owner, "muster_update_event", ev, map[string]any{"archive": "no"}); bad == nil || bad.Code != "BAD_ARGUMENTS" {
		t.Fatalf("archive must be boolean: %#v", bad)
	}

	// Blind: non-owners see only themselves and cannot rank or overlap.
	blind := w.group(func(e *models.Event) { e.BlindAvailabilityEnabled = utils.TruePtr() })
	w.s.put(blind.Id, w.bob, []primitive.DateTime{day(1, 9, 0)}, nil)
	w.s.put(blind.Id, w.alice, []primitive.DateTime{day(1, 9, 30)}, nil)
	for _, tool := range []string{"muster_summarize_overlap", "muster_find_best_times"} {
		_, bad := w.run(w.alice, tool, blind, nil)
		wantCode(t, bad, "OVERLAP_HIDDEN")
		w.ok(t, w.owner, tool, blind, nil)
	}
	resp := w.ok(t, w.alice, "muster_get_responses", blind, map[string]any{"start": rfc(9, 0), "end": rfc(12, 0)})
	if people := list(t, resp["people"]); len(people) != 1 || obj(t, people[0])["name"] != "Alice" {
		t.Fatalf("blind responses leak: %#v", resp)
	}
	att := w.ok(t, w.alice, "muster_list_attendees", blind, nil)
	if rows := list(t, att["people"]); len(rows) != 1 || obj(t, rows[0])["status"] != "hidden" || strings.Contains(asString(att), "bob@x.com") {
		t.Fatalf("blind attendees leak: %#v", att)
	}
	w.ok(t, w.alice, "muster_set_availability", blind, map[string]any{"availability": []any{rfc(10, 0)}, "if_needed": []any{}})
}

func asString(v any) string { raw, _ := json.Marshal(v); return string(raw) }

func init() { parity("REQ_TXT_017", lifecycleMatrixCase) }

// ---- W series: preserved behavior -----------------------------------------

func init() {
	// W2: grids, windows, owner removal keep every original refusal and limit.
	field("W2", func(t *testing.T) {
		w := newWorld()
		ev := w.group()
		for name, args := range map[string]map[string]any{
			"outside": {"availability": []any{"2026-06-03T09:00:00Z"}},
			"off":     {"availability": []any{rfc(9, 15)}},
			"both":    {"availability": []any{rfc(9, 0)}, "if_needed": []any{rfc(9, 0)}},
		} {
			args["if_needed"] = orEmpty(args["if_needed"])
			_, bad := w.run(w.owner, "muster_set_availability", ev, args)
			want := map[string]string{"outside": "SLOT_OUTSIDE", "off": "SLOT_OFF_GRID", "both": "SLOT_IN_BOTH"}[name]
			wantCode(t, bad, want)
		}
		many := make([]any, maxMarks+1)
		for i := range many {
			many[i] = rfc(9, 0)
		}
		_, bad := w.run(w.owner, "muster_set_availability", ev, map[string]any{"availability": many, "if_needed": []any{}})
		wantCode(t, bad, "BAD_ARGUMENTS")
		ranges := make([]any, 101)
		for i := range ranges {
			ranges[i] = rng(rfc(9, 0), rfc(9, 30), "available")
		}
		_, bad = w.run(w.owner, "muster_set_availability", ev, map[string]any{"ranges": ranges})
		wantCode(t, bad, "BAD_ARGUMENTS")
		_, bad = w.run(w.owner, "muster_get_responses", ev, map[string]any{"start": rfc(0, 0), "end": "2026-06-08T00:00:01Z"})
		wantCode(t, bad, "WINDOW_TOO_WIDE")
		_, bad = w.run(w.owner, "muster_remove_attendee", ev, map[string]any{"email": "OWNER@x.com"})
		wantCode(t, bad, "BAD_ARGUMENTS")
		twenty := make([]any, 21)
		for i := range twenty {
			twenty[i] = "p" + string(rune('a'+i)) + "@x.com"
		}
		_, bad = w.run(w.owner, "muster_add_attendees", ev, map[string]any{"emails": twenty})
		wantCode(t, bad, "BAD_ARGUMENTS")
		if len(w.s.removed) != 0 || len(w.s.added) != 0 || w.s.writes != 0 {
			t.Fatal("refused calls wrote")
		}
		// A request exactly at the 7-day limit and a valid slot still pass.
		w.ok(t, w.owner, "muster_get_responses", ev, map[string]any{"start": rfc(0, 0), "end": "2026-06-08T00:00:00Z"})
		w.ok(t, w.owner, "muster_set_availability", ev, map[string]any{"availability": []any{rfc(9, 0)}, "if_needed": []any{}})
	})

	// W3: ranking defaults, shared with the new defaulted read window.
	field("W3", func(t *testing.T) {
		w := newWorld()
		ev := w.event(models.SPECIFIC_DATES)
		w.s.put(ev.Id, w.owner, []primitive.DateTime{day(1, 9, 0), day(1, 9, 30)}, nil)
		out := w.ok(t, w.owner, "muster_find_best_times", ev, nil)
		if out["start"] != "2026-06-01T09:00:00Z" || len(list(t, out["slots"])) != 2 {
			t.Fatalf("defaults: %#v", out)
		}
		first := obj(t, list(t, out["slots"])[0])
		if first["end"] != rfc(9, 30) { // duration defaults to slot_minutes
			t.Fatalf("default duration: %#v", first)
		}
		_, bad := w.run(w.owner, "muster_find_best_times", ev, map[string]any{"start": rfc(9, 0)})
		wantCode(t, bad, "BAD_ARGUMENTS")
		shared := w.ok(t, w.owner, "muster_get_responses", ev, nil)
		if shared["start"] != out["start"] || shared["end"] != out["end"] {
			t.Fatalf("reads must share one default window: %v vs %v", shared, out)
		}
	})

	// W4: full grids stay on get and list; compact is explicit only.
	field("W4", func(t *testing.T) {
		w := newWorld()
		weekly := w.event(models.DOW, func(e *models.Event) {
			e.Dates = []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 9, 0, 0, 0, time.UTC))}
			e.DaysOnly, e.StartOnMonday = utils.TruePtr(), utils.TruePtr()
		})
		w.s.listed = []ListedEvent{{Event: *weekly}}
		get := w.ok(t, w.owner, "muster_get_event", weekly, nil)
		row := obj(t, list(t, w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all"})["events"])[0])
		for _, r := range []map[string]any{get, row} {
			if len(list(t, r["dates"])) != 1 || strs(t, r["weekdays"])[0] != "Monday" || r["duration_hours"] != float64(8) ||
				r["slot_minutes"] != float64(30) || r["days_only"] != true || r["start_on_monday"] != true || r["is_group"] != false {
				t.Fatalf("grid lost: %#v", r)
			}
		}
		// Default projection is full.
		if _, ok := obj(t, list(t, w.ok(t, w.owner, "muster_list_my_events", nil, map[string]any{"status": "all"})["events"])[0])["dates"]; !ok {
			t.Fatal("default list must keep the full grid")
		}
	})

	// W5: display names and merged ranges.
	field("W5", func(t *testing.T) {
		w := newWorld()
		ev := w.event(models.SPECIFIC_DATES)
		w.s.responses[ev.Id.Hex()] = []models.EventResponse{{UserId: w.alice.Id.Hex(), Response: &models.Response{
			UserId: w.alice.Id, Availability: []primitive.DateTime{day(1, 9, 0), day(1, 9, 30), day(1, 10, 0), day(1, 12, 0)}}}}
		out := w.ok(t, w.owner, "muster_get_responses", ev, map[string]any{"start": rfc(9, 0), "end": rfc(18, 0)})
		person := obj(t, list(t, out["people"])[0])
		if person["name"] != "Alice" {
			t.Fatalf("display name from the account: %#v", person)
		}
		avail := list(t, person["available"])
		if len(avail) != 2 || obj(t, avail[0])["start"] != rfc(9, 0) || obj(t, avail[0])["end"] != rfc(10, 30) {
			t.Fatalf("ranges not merged: %#v", avail)
		}
		// The set receipt echoes stored answers in merged-friendly canonical instants, and ranking names people.
		w.s.put(ev.Id, w.owner, []primitive.DateTime{day(1, 9, 0)}, nil)
		best := w.ok(t, w.owner, "muster_find_best_times", ev, nil)
		if !contains(strs(t, obj(t, list(t, best["slots"])[1])["missing"]), "Olive") {
			t.Fatalf("ranking must name people: %#v", best)
		}
		ov := w.ok(t, w.owner, "muster_summarize_overlap", ev, nil)
		if strings.Contains(asString(ov), w.alice.Id.Hex()) {
			t.Fatalf("overlap leaks user ids: %#v", ov)
		}
	})

	// W6: receipts never claim more than happened.
	field("W6", func(t *testing.T) {
		t.Setenv("LISTMONK_URL", "")
		w := newWorld()
		created := w.ok(t, w.owner, "muster_create_event", nil, map[string]any{
			"name": "G", "type": "group", "duration_hours": float64(2), "dates": []any{"2026-06-01T09:00:00Z"},
			"emails": []any{"a@x.com", "b@x.com"}})
		if created["email_sent"] != false || created["page_url"] == "" || created["event_id"] == "" {
			t.Fatalf("create receipt: %#v", created)
		}
		ev := w.s.GetByEitherID(created["event_id"].(string))
		add := w.ok(t, w.owner, "muster_add_attendees", ev, map[string]any{"emails": []any{"c@x.com"}})
		if add["email_sent"] != false || len(list(t, add["added"])) != 1 {
			t.Fatalf("add: %#v", add)
		}
		none := w.ok(t, w.owner, "muster_add_attendees", ev, map[string]any{"emails": []any{"c@x.com"}})
		if len(list(t, none["added"])) != 0 || none["email_sent"] != false {
			t.Fatalf("nothing new: %#v", none)
		}
		if r := w.ok(t, w.owner, "muster_remind", ev, nil); r["email_sent"] != false {
			t.Fatalf("remind: %#v", r)
		}
		if f := w.ok(t, w.owner, "muster_finalize", ev, map[string]any{"start": rfc(9, 0), "end": rfc(10, 0)}); f["email_sent"] != false || f["closed"] != true {
			t.Fatalf("finalize: %#v", f)
		}
		// A store that loses the write is reported as incomplete, not as saved.
		w2 := newWorld()
		ev2 := w2.event(models.SPECIFIC_DATES)
		w2.d.Events = &dropsWrites2{w2.s}
		out := w2.ok(t, w2.owner, "muster_set_availability", ev2, map[string]any{"availability": []any{rfc(9, 0)}, "if_needed": []any{}})
		if out["echo_complete"] != false || out["changed"] != true {
			t.Fatalf("lost write: %#v", out)
		}
		// A refused write has no receipt.
		_, bad := w.run(w.owner, "muster_set_availability", ev, map[string]any{"availability": []any{rfc(9, 7)}, "if_needed": []any{}})
		wantCode(t, bad, "SLOT_OFF_GRID")
	})

	// W7: timestamped changes, not an event log.
	field("W7", func(t *testing.T) {
		w := newWorld()
		ev := w.group()
		stamp := primitive.NewDateTimeFromTime(time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC))
		w.s.changes = []models.EventResponse{{UserId: w.alice.Id.Hex(), UpdatedAt: stamp, Response: &models.Response{UserId: w.alice.Id}}}
		no := false
		w.s.invites = []models.Attendee{{Email: "bob@x.com", Declined: &no, UpdatedAt: stamp}}
		out := w.ok(t, w.owner, "muster_list_changes", ev, map[string]any{"since": "2026-06-01T00:00:00Z"})
		a := obj(t, list(t, out["answers"])[0])
		i := obj(t, list(t, out["invites"])[0])
		if a["name"] != "Alice" || a["updated_at"] != "2026-06-01T08:30:00Z" || i["email"] != "bob@x.com" || i["updated_at"] != "2026-06-01T08:30:00Z" || i["declined"] != false {
			t.Fatalf("changes: %#v", out)
		}
		for k := range out {
			if k != "event_id" && k != "answers" && k != "invites" {
				t.Fatalf("not an event log, unexpected key %s", k)
			}
		}
		_, bad := w.run(w.owner, "muster_list_changes", ev, map[string]any{"since": "yesterday"})
		wantCode(t, bad, "BAD_ARGUMENTS")
		_, bad = w.run(w.owner, "muster_list_changes", ev, nil)
		wantCode(t, bad, "BAD_ARGUMENTS")
	})

	// W8: weekly raw reference rules; shorthand is additive and never narrows a raw interval.
	field("W8", func(t *testing.T) {
		w := newWorld()
		made := w.ok(t, w.owner, "muster_create_event", nil, map[string]any{
			"name": "Wk", "type": "dow", "duration_hours": float64(2), "slot_minutes": float64(30),
			"dates": []any{map[string]any{"weekday": "Monday", "time": "15:00"}, "2018-06-24T15:00:00Z", map[string]any{"weekday": "Sunday", "time": "09:00"}}})
		ev := w.s.GetByEitherID(made["event_id"].(string))
		got := []string{}
		for _, d := range ev.Dates {
			got = append(got, d.Time().UTC().Format(time.RFC3339))
		}
		if strings.Join(got, ",") != "2018-06-18T15:00:00Z,2018-06-24T15:00:00Z,2018-06-24T09:00:00Z" {
			t.Fatalf("canonical dates: %v", got)
		}
		for name, raw := range map[string]any{"later week": "2018-07-02T15:00:00Z", "before": "2018-06-16T15:00:00Z", "current week": time.Now().Format(time.RFC3339)} {
			_, bad := w.run(w.owner, "muster_create_event", nil, map[string]any{
				"name": "Bad", "type": "dow", "duration_hours": float64(2), "dates": []any{raw}})
			if bad == nil || bad.Code != "BAD_ARGUMENTS" {
				t.Fatalf("%s accepted: %#v", name, bad)
			}
		}
		// Shorthand is only for weekly musters; dated ones are never remapped.
		_, bad := w.run(w.owner, "muster_create_event", nil, map[string]any{
			"name": "Dated", "type": "specific_dates", "duration_hours": float64(2), "dates": []any{map[string]any{"weekday": "Monday", "time": "15:00"}}})
		wantCode(t, bad, "BAD_ARGUMENTS")
		// On a stored reference-week grid, the legacy Sunday 17th resolves, and the raw instant works unchanged.
		legacy := w.event(models.DOW, func(e *models.Event) {
			hours := float32(24)
			e.Duration = &hours
			e.Dates = []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2018, 6, 17, 0, 0, 0, 0, time.UTC))}
		})
		out := w.ok(t, w.owner, "muster_set_availability", legacy, map[string]any{
			"availability": []any{map[string]any{"weekday": "Sunday", "time": "15:00"}, "2018-06-17T16:00:00Z"}, "if_needed": []any{}})
		views := list(t, obj(t, out["answer"])["availability"])
		if len(views) != 2 || obj(t, views[0])["instant"] != "2018-06-17T15:00:00Z" || obj(t, views[0])["weekday"] != "Sunday" || obj(t, views[1])["instant"] != "2018-06-17T16:00:00Z" {
			t.Fatalf("stored Sunday: %#v", views)
		}
		// A weekday with no stored time is refused with candidates, not guessed.
		_, bad = w.run(w.owner, "muster_set_availability", legacy, map[string]any{
			"availability": []any{map[string]any{"weekday": "Wednesday", "time": "15:00"}}, "if_needed": []any{}})
		wantCode(t, bad, "SLOT_OUTSIDE")
		for _, shorthand := range []map[string]any{{"weekday": "Funday", "time": "15:00"}, {"weekday": "Monday", "time": "25:00"}, {"weekday": "Monday"}} {
			_, bad = w.run(w.owner, "muster_set_availability", legacy, map[string]any{"availability": []any{shorthand}, "if_needed": []any{}})
			wantCode(t, bad, "BAD_ARGUMENTS")
		}
	})
}

func orEmpty(v any) any {
	if v == nil {
		return []any{}
	}
	return v
}

type dropsWrites2 struct{ *mxStore }

func (p *dropsWrites2) ReplaceAvailability(*models.User, primitive.ObjectID, []primitive.DateTime, []primitive.DateTime) error {
	return nil
}

// ---- REQ_EFF: text efficiency ---------------------------------------------

// counted counts every model-initiated tool attempt, failed ones included.
type counted struct {
	w     *mxWorld
	calls int
}

func (c *counted) ok(t *testing.T, user *models.User, tool string, ev *models.Event, args map[string]any) map[string]any {
	t.Helper()
	c.calls++
	return c.w.ok(t, user, tool, ev, args)
}

func (c *counted) run(user *models.User, tool string, ev *models.Event, args map[string]any) (map[string]any, *ToolError) {
	c.calls++
	return c.w.run(user, tool, ev, args)
}

// reference tasks from the amendment efficiency table. ceiling is the maximum number of calls.
func referenceTasks(t *testing.T) []struct {
	name    string
	ceiling int
	run     func(c *counted) map[string]any // returns the receipt of the last call
} {
	type task = struct {
		name    string
		ceiling int
		run     func(c *counted) map[string]any
	}
	return []task{
		{"mark 15:00-17:00 on a known grid", 1, func(c *counted) map[string]any {
			ev := c.w.event(models.SPECIFIC_DATES)
			out := c.ok(t, c.w.owner, "muster_set_availability", ev, map[string]any{"ranges": []any{rng(rfc(15, 0), rfc(17, 0), "available")}})
			views := list(t, obj(t, out["answer"])["availability"])
			if len(views) != 4 || obj(t, views[0])["instant"] != rfc(15, 0) || obj(t, views[3])["instant"] != rfc(16, 30) || out["echo_complete"] != true {
				t.Fatalf("stored answer not proven: %#v", out)
			}
			return out
		}},
		{"mark 15:00-17:00 on an unknown grid", 2, func(c *counted) map[string]any {
			ev := c.w.event(models.SPECIFIC_DATES)
			grid := c.ok(t, c.w.owner, "muster_get_event", ev, nil)
			first := strs(t, grid["dates"])[0]
			start, _ := time.Parse(time.RFC3339, first)
			out := c.ok(t, c.w.owner, "muster_set_availability", ev, map[string]any{"ranges": []any{
				rng(start.Add(6*time.Hour).Format(time.RFC3339), start.Add(8*time.Hour).Format(time.RFC3339), "available")}})
			if out["echo_complete"] != true || len(list(t, obj(t, out["answer"])["availability"])) != 4 {
				t.Fatalf("%#v", out)
			}
			return out
		}},
		{"when should we meet", 1, func(c *counted) map[string]any {
			ev := c.w.event(models.SPECIFIC_DATES)
			c.w.s.put(ev.Id, c.w.owner, []primitive.DateTime{day(1, 15, 0), day(1, 15, 30)}, nil)
			c.w.s.put(ev.Id, c.w.alice, []primitive.DateTime{day(1, 15, 0)}, nil)
			out := c.ok(t, c.w.owner, "muster_find_best_times", ev, nil)
			if len(list(t, out["slots"])) == 0 || out["start"] == nil || out["end"] == nil || out["duration_minutes"] != float64(30) {
				t.Fatalf("window and duration not stated: %#v", out)
			}
			return out
		}},
		{"owner asks how it is going", 1, func(c *counted) map[string]any {
			ev := c.w.group()
			c.w.s.put(ev.Id, c.w.alice, []primitive.DateTime{day(1, 9, 0)}, nil)
			out := c.ok(t, c.w.owner, "muster_get_event", ev, nil)
			if obj(t, out["response_totals"])["responded"] != float64(1) {
				t.Fatalf("owner counts: %#v", out)
			}
			return out
		}},
		{"create a group with invitees", 1, func(c *counted) map[string]any {
			t.Setenv("LISTMONK_URL", "")
			out := c.ok(t, c.w.owner, "muster_create_event", nil, map[string]any{
				"name": "Crew", "type": "group", "duration_hours": float64(2), "dates": []any{"2026-06-01T09:00:00Z"},
				"emails": []any{"A@X.com", "a@x.com", "b@x.com", "owner@x.com"}})
			if out["page_url"] == "" || out["email_sent"] != false {
				t.Fatalf("share link / outcome: %#v", out)
			}
			if got := strs(t, out["invited"]); strings.Join(got, ",") != "a@x.com,b@x.com" {
				t.Fatalf("canonical targets: %#v", out)
			}
			if ev := obj(t, out["event"]); ev["name"] != "Crew" || ev["is_group"] != true {
				t.Fatalf("created event echo: %#v", out)
			}
			return out
		}},
		{"finalize", 1, func(c *counted) map[string]any {
			ev := c.w.group()
			out := c.ok(t, c.w.owner, "muster_finalize", ev, map[string]any{"start": rfc(15, 0), "end": rfc(16, 0)})
			if obj(t, out["scheduled"])["start"] != rfc(15, 0) {
				t.Fatalf("stored interval: %#v", out)
			}
			return out
		}},
		{"rename", 1, func(c *counted) map[string]any {
			ev := c.w.group()
			out := c.ok(t, c.w.owner, "muster_update_event", ev, map[string]any{"name": "New name"})
			if obj(t, out["event"])["name"] != "New name" {
				t.Fatalf("%#v", out)
			}
			return out
		}},
		{"archive", 1, func(c *counted) map[string]any {
			ev := c.w.group()
			out := c.ok(t, c.w.owner, "muster_update_event", ev, map[string]any{"archive": true})
			if obj(t, out["event"])["is_archived"] != true {
				t.Fatalf("%#v", out)
			}
			return out
		}},
	}
}

func init() {
	// EFF_001: call ceilings, counting failures and retries.
	parity("REQ_EFF_001", func(t *testing.T) {
		for _, task := range referenceTasks(t) {
			c := &counted{w: newWorld()}
			task.run(c)
			if c.calls > task.ceiling {
				t.Errorf("%s: %d calls, ceiling %d", task.name, c.calls, task.ceiling)
			}
		}
		// A failed first attempt still counts against the ceiling.
		c := &counted{w: newWorld()}
		ev := c.w.event(models.SPECIFIC_DATES)
		_, bad := c.run(c.w.owner, "muster_set_availability", ev, map[string]any{"ranges": []any{rng(rfc(15, 0), rfc(17, 0), "maybe")}})
		if bad == nil || c.calls != 1 {
			t.Fatalf("%#v %d", bad, c.calls)
		}
	})

	// EFF_003: typical receipts are at most 2,000 code points on the 25-person, seven-day fixture;
	// large pages are at most 20,000.
	parity("REQ_EFF_003", func(t *testing.T) {
		w := newWorld()
		dates := []primitive.DateTime{}
		for d := 1; d <= 7; d++ {
			dates = append(dates, day(d, 9, 0))
		}
		ev := w.group(func(e *models.Event) { e.Dates = dates })
		w.s.attendees[ev.Id.Hex()] = nil
		for i := 0; i < 25; i++ {
			u := newUser(strings.Repeat("P", 6)+string(rune('a'+i)), "p"+string(rune('a'+i))+"@x.com")
			w.d.Users = mxUsers{append(w.d.Users.(mxUsers).all, u)}
			w.s.attendees[ev.Id.Hex()] = append(w.s.attendees[ev.Id.Hex()], models.Attendee{Email: u.Email})
			w.s.put(ev.Id, u, []primitive.DateTime{day(1+i%7, 15, 0), day(1+i%7, 15, 30)}, nil)
		}
		size := func(name string, v any) {
			t.Helper()
			if n := utf8.RuneCountInString(asString(v)); n > 2000 {
				t.Errorf("%s is %d code points, over 2000", name, n)
			}
		}
		for _, task := range referenceTasks(t) {
			size(task.name, task.run(&counted{w: newWorld()}))
		}
		size("get_event totals on 25 people", w.ok(t, w.owner, "muster_get_event", ev, nil))
		// A compact page is the designated model-safe projection.
		w.s.listed = nil
		for i := 0; i < 5; i++ {
			w.s.listed = append(w.s.listed, ListedEvent{Event: *ev})
		}
		page, bad, _ := Call(w.d, w.owner, "muster_list_my_events", map[string]any{"status": "needs_answer", "projection": "compact", "limit": float64(5)})
		wantNilErr(t, bad)
		size("compact needs-answer page of 5", page)
		// Large pages: every full page, at the 50 row maximum, stays within 20,000 code points.
		w.s.listed = nil
		for i := 0; i < 120; i++ {
			row := *ev
			row.Id = primitive.NewObjectID()
			w.s.listed = append(w.s.listed, ListedEvent{Event: row})
		}
		cursor, pages, rows := any(nil), 0, 0
		for {
			args := map[string]any{"status": "all", "limit": float64(50)}
			if cursor != nil {
				args["cursor"] = cursor
			}
			out := w.ok(t, w.owner, "muster_list_my_events", nil, args)
			if n := utf8.RuneCountInString(asString(out)); n > 20000 {
				t.Fatalf("page %d is %d code points", pages, n)
			}
			pages++
			rows += len(list(t, out["events"]))
			if pages > 120 {
				t.Fatal("pagination does not advance")
			}
			if out["next_cursor"] == "" || out["next_cursor"] == nil {
				break
			}
			cursor = out["next_cursor"]
		}
		if pages < 3 || rows != 120 {
			t.Fatalf("pagination: %d pages carried %d of 120 rows", pages, rows)
		}
		for _, tool := range []string{"muster_get_responses", "muster_find_best_times", "muster_summarize_overlap", "muster_list_attendees"} {
			if n := utf8.RuneCountInString(asString(w.ok(t, w.owner, tool, ev, nil))); n > 20000 {
				t.Errorf("%s page is %d code points", tool, n)
			}
		}
	})

	// EFF_004: required truth stays in the model-visible text result, never moved to _meta; summaries are small and private.
	parity("REQ_EFF_004", func(t *testing.T) {
		for _, task := range referenceTasks(t) {
			out := task.run(&counted{w: newWorld()})
			if _, has := out["_meta"]; has {
				t.Errorf("%s moved truth into _meta", task.name)
			}
		}
		w := newWorld()
		ev := w.group()
		for i := 0; i < 25; i++ {
			u := newUser("Person"+string(rune('A'+i)), "q"+string(rune('a'+i))+"@x.com")
			w.d.Users = mxUsers{append(w.d.Users.(mxUsers).all, u)}
			w.s.attendees[ev.Id.Hex()] = append(w.s.attendees[ev.Id.Hex()], models.Attendee{Email: u.Email})
			w.s.put(ev.Id, u, []primitive.DateTime{day(1, 9, 0)}, nil)
		}
		snap, bad := BuildSnapshot(w.d, w.owner, SnapshotRequest{Workflow: "pick_a_time", EventID: ev.Id.Hex()})
		wantNilErr(t, bad)
		summary := snap.Summary()
		if utf8.RuneCountInString(summary) > 2000 || strings.Contains(summary, "Person") {
			t.Fatalf("model-safe summary: %q", summary)
		}
	})

	// EFF_005: no hidden retries; a lost compare is one attempt and the caller decides.
	parity("REQ_EFF_005", func(t *testing.T) {
		w := newWorld()
		ev := w.event(models.SPECIFIC_DATES)
		w.s.put(ev.Id, w.owner, []primitive.DateTime{day(1, 9, 0)}, nil)
		racing := &racingStore{mxStore: w.s}
		w.d.Events = racing
		_, bad := w.run(w.owner, "muster_set_availability", ev, map[string]any{"mode": "merge", "ranges": []any{rng(rfc(15, 0), rfc(15, 30), "available")}})
		wantCode(t, bad, "CONFLICT")
		if w.s.casCalls != 1 || w.s.writes != 0 {
			t.Fatalf("hidden retry: %d compares, %d writes", w.s.casCalls, w.s.writes)
		}
		total := 0
		for _, task := range referenceTasks(t) {
			c := &counted{w: newWorld()}
			task.run(c)
			total += c.calls
		}
		if total > 12 {
			t.Fatalf("reference tasks used %d calls against a 60 per minute bucket", total)
		}
	})

	// EFF_006: a commit without a valid actor-bound prepared action id fails on the server.
	parity("REQ_EFF_006", func(t *testing.T) {
		t.Setenv("MCP_APPS_WRITES_ENABLED", "true")
		w := newWorld()
		w.d.Actions = &mxActions{rows: map[primitive.ObjectID]*ActionRecord{}}
		ev := w.group()
		fp := StateFingerprint(w.d, w.owner, ev)
		prep, bad, _ := Call(w.d, w.owner, "muster_ui_prepare_action", map[string]any{
			"operation": "finalize_time", "event_id": ev.Id.Hex(), "expected_state_fingerprint": fp,
			"payload": map[string]any{"start": rfc(15, 0), "end": rfc(16, 0)}})
		wantNilErr(t, bad)
		id := asMap(prep)["action_id"].(string)
		commit := func(user *models.User, args map[string]any) *ToolError {
			_, bad, _ := Call(w.d, user, "muster_ui_commit_action", args)
			return bad
		}
		for name, args := range map[string]map[string]any{
			"no id": {}, "not hex": {"action_id": "nope"}, "unknown id": {"action_id": primitive.NewObjectID().Hex()},
			"event id instead": {"action_id": ev.Id.Hex()}, "empty": {"action_id": ""},
		} {
			if bad := commit(w.owner, args); bad == nil {
				t.Fatalf("%s committed", name)
			}
		}
		if bad := commit(w.alice, map[string]any{"action_id": id}); bad == nil {
			t.Fatal("another actor committed the owner's prepared action")
		}
		if len(w.s.patches) != 0 {
			t.Fatalf("a refused commit wrote: %v", w.s.patches)
		}
		if bad := commit(w.owner, map[string]any{"action_id": id}); bad != nil {
			t.Fatal(bad.Message)
		}
		if bad := commit(w.owner, map[string]any{"action_id": id}); bad != nil || len(w.s.patches) != 1 {
			t.Fatalf("second commit must return the receipt without writing again: %v %d", bad, len(w.s.patches))
		}
		// With writes disabled the helpers do not exist at all.
		t.Setenv("MCP_APPS_WRITES_ENABLED", "false")
		if bad := commit(w.owner, map[string]any{"action_id": id}); bad == nil || bad.Message != "Unknown tool." {
			t.Fatalf("%#v", bad)
		}
	})
}

func wantNilErr(t *testing.T, bad *ToolError) {
	t.Helper()
	if bad != nil {
		t.Fatalf("%s: %s", bad.Code, bad.Message)
	}
}

// racingStore lets another writer change the answer between the caller's read and its compare.
type racingStore struct{ *mxStore }

func (r *racingStore) ReplaceAvailabilityIf(user *models.User, id primitive.ObjectID, snap AnswerSnapshot, a, n []primitive.DateTime) (bool, error) {
	r.mxStore.put(id, user, []primitive.DateTime{day(1, 10, 0)}, nil)
	return r.mxStore.ReplaceAvailabilityIf(user, id, snap, a, n)
}

type mxActions struct {
	rows map[primitive.ObjectID]*ActionRecord
}

func (a *mxActions) Create(rec *ActionRecord) error { a.rows[rec.ID] = rec; return nil }
func (a *mxActions) Claim(user, id primitive.ObjectID, _ time.Time) (*ActionRecord, error) {
	rec := a.rows[id]
	if rec == nil || rec.UserID != user || rec.Status != "prepared" {
		return nil, ErrTokenNotFound
	}
	rec.Status = "claimed"
	return rec, nil
}
func (a *mxActions) Get(user, id primitive.ObjectID) (*ActionRecord, error) {
	if rec := a.rows[id]; rec != nil && rec.UserID == user {
		return rec, nil
	}
	return nil, ErrTokenNotFound
}
func (a *mxActions) Finish(user, id primitive.ObjectID, status string, result map[string]any, code, msg string) error {
	if rec := a.rows[id]; rec != nil && rec.UserID == user {
		rec.Status, rec.Result, rec.ErrCode, rec.ErrMessage = status, result, code, msg
	}
	return nil
}
