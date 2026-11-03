package mcp

import (
	"sync"
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type memActions struct {
	mu   sync.Mutex
	rows map[string]*ActionRecord
}

func newMemActions() *memActions {
	return &memActions{rows: map[string]*ActionRecord{}}
}

func (m *memActions) Create(rec *ActionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *rec
	m.rows[rec.ID.Hex()] = &copy
	return nil
}

func (m *memActions) Claim(userID, id primitive.ObjectID, now time.Time) (*ActionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.rows[id.Hex()]
	if rec == nil || rec.UserID != userID || rec.Status != "prepared" || !rec.ExpiresAt.After(now) {
		return nil, mongo.ErrNoDocuments
	}
	rec.Status = "claimed"
	copy := *rec
	return &copy, nil
}

func (m *memActions) Get(userID, id primitive.ObjectID) (*ActionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.rows[id.Hex()]
	if rec == nil || rec.UserID != userID {
		return nil, mongo.ErrNoDocuments
	}
	copy := *rec
	return &copy, nil
}

func (m *memActions) Finish(userID, id primitive.ObjectID, status string, result map[string]any, errCode, errMessage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.rows[id.Hex()]
	if rec == nil || rec.UserID != userID {
		return mongo.ErrNoDocuments
	}
	rec.Status = status
	rec.Result = result
	rec.ErrCode = errCode
	rec.ErrMessage = errMessage
	return nil
}

type actionEvents struct {
	decisionEvents
	writes int
}

func (f *actionEvents) ReplaceAvailability(user *models.User, eventID primitive.ObjectID, availability, ifNeeded []primitive.DateTime) error {
	f.writes++
	return f.decisionEvents.ReplaceAvailability(user, eventID, availability, ifNeeded)
}

func TestPreparedEditCommitsOnce(t *testing.T) {
	t.Setenv("MCP_APPS_WRITES_ENABLED", "true")
	user := &models.User{Id: primitive.NewObjectID(), Email: "owner@x.com"}
	hours := float32(24)
	slot := 60
	event := &models.Event{
		Id: primitive.NewObjectID(), OwnerId: user.Id, Type: models.DOW,
		Dates:    []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 0, 0, 0, 0, time.UTC))},
		Duration: &hours, TimeIncrement: &slot,
	}
	events := &actionEvents{decisionEvents: decisionEvents{event: event}}
	deps := Deps{Events: events, Actions: newMemActions()}
	fp := StateFingerprint(deps, user, event)
	_, bad, _ := Call(deps, user, "muster_ui_prepare_action", map[string]any{
		"operation": "edit_own_answer", "event_id": event.Id.Hex(),
		"expected_state_fingerprint": "stale",
		"payload":                    map[string]any{"changes": []any{}},
	})
	if bad == nil || bad.Code != "CONFLICT" || events.writes != 0 {
		t.Fatalf("stale %#v writes %d", bad, events.writes)
	}
	prepared, bad, _ := Call(deps, user, "muster_ui_prepare_action", map[string]any{
		"operation": "edit_own_answer", "event_id": event.Id.Hex(),
		"expected_state_fingerprint": fp,
		"payload": map[string]any{"changes": []any{map[string]any{
			"start": "2018-06-18T15:00:00Z", "end": "2018-06-18T16:00:00Z", "state": "available",
		}}},
	})
	if bad != nil {
		t.Fatal(bad.Message)
	}
	actionID := prepared.(map[string]any)["action_id"].(string)
	first, bad, _ := Call(deps, user, "muster_ui_commit_action", map[string]any{"action_id": actionID})
	if bad != nil || events.writes != 1 {
		t.Fatalf("first %#v bad %#v writes %d", first, bad, events.writes)
	}
	second, bad, _ := Call(deps, user, "muster_ui_commit_action", map[string]any{"action_id": actionID})
	if bad != nil || events.writes != 1 {
		t.Fatalf("second %#v bad %#v writes %d", second, bad, events.writes)
	}
	if second.(map[string]any)["status"] != "completed" {
		t.Fatalf("%#v", second)
	}
}
