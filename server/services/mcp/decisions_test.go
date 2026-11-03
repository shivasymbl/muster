package mcp

import (
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type decisionEvents struct {
	event        *models.Event
	answers      []models.EventResponse
	replaced     []primitive.DateTime
	replacedNeed []primitive.DateTime
	casMiss      bool
	created      []primitive.DateTime
	listed       []ListedEvent
}

func (f *decisionEvents) GetByEitherID(string) *models.Event { return f.event }
func (f *decisionEvents) Responses(primitive.ObjectID) []models.EventResponse {
	return f.answers
}
func (f *decisionEvents) Attendees(primitive.ObjectID) []models.Attendee { return nil }
func (f *decisionEvents) ListForUser(*models.User, string, int, string) ([]ListedEvent, string, error) {
	return f.listed, "", nil
}
func (f *decisionEvents) ResponseFlags(string, []primitive.ObjectID) map[string]ResponseFlags {
	return map[string]ResponseFlags{}
}
func (f *decisionEvents) ReplaceAvailability(_ *models.User, _ primitive.ObjectID, availability, _ []primitive.DateTime) error {
	f.replaced = availability
	return nil
}
func (f *decisionEvents) Decline(*models.User, primitive.ObjectID) error { panic("unused") }
func (f *decisionEvents) AddAttendees(primitive.ObjectID, []string) ([]string, error) {
	panic("unused")
}
func (f *decisionEvents) Create(_ *models.User, event models.Event, _ []string) (*models.Event, error) {
	event.Id = primitive.NewObjectID()
	shortID := "abc"
	event.ShortId = &shortID
	f.created = event.Dates
	f.event = &event
	return &event, nil
}
func (f *decisionEvents) Patch(primitive.ObjectID, map[string]any) error { panic("unused") }
func (f *decisionEvents) RemoveAttendee(primitive.ObjectID, string, string) error {
	panic("unused")
}
func (f *decisionEvents) Changes(primitive.ObjectID, time.Time) ([]models.EventResponse, []models.Attendee, error) {
	panic("unused")
}

func TestWeeklyShorthandAndGuardedReplace(t *testing.T) {
	user := &models.User{Id: primitive.NewObjectID(), Email: "owner@x.com"}
	events := &decisionEvents{}
	deps := Deps{Events: events}
	created, bad, _ := Call(deps, user, "muster_create_event", map[string]any{
		"name": "Week", "type": "dow", "duration_hours": float64(8),
		"dates": []any{map[string]any{"weekday": "Monday", "time": "15:00"}},
	})
	if bad != nil {
		t.Fatal(bad.Message)
	}
	if len(events.created) != 1 || events.created[0].Time().UTC().Format(time.RFC3339) != "2018-06-18T15:00:00Z" {
		t.Fatalf("created %#v", events.created)
	}
	if created.(map[string]any)["event_id"] == "" {
		t.Fatal(created)
	}

	day := primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 0, 0, 0, 0, time.UTC))
	hours := float32(24)
	slot := 30
	events.event = &models.Event{
		Id: user.Id, OwnerId: user.Id, Type: models.DOW, Dates: []primitive.DateTime{day},
		Duration: &hours, TimeIncrement: &slot,
	}
	_, bad, _ = Call(deps, user, "muster_set_availability", map[string]any{
		"event_id":     events.event.Id.Hex(),
		"availability": []any{map[string]any{"weekday": "Monday", "time": "15:00"}},
		"if_needed":    []any{},
	})
	if bad != nil {
		t.Fatal(bad.Message)
	}
	if events.replaced[0].Time().UTC().Format(time.RFC3339) != "2018-06-18T15:00:00Z" {
		t.Fatalf("%v", events.replaced)
	}

	events.answers = []models.EventResponse{{
		UserId: user.Id.Hex(),
		Response: &models.Response{Availability: []primitive.DateTime{
			primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 15, 0, 0, 0, time.UTC)),
		}},
	}}
	events.replaced = nil
	_, bad, _ = Call(deps, user, "muster_set_availability", map[string]any{
		"event_id": events.event.Id.Hex(),
		"availability_ranges": []any{map[string]any{
			"start": "2018-06-18T15:00:00Z", "end": "2018-06-18T16:00:00Z",
		}},
		"if_needed": []any{},
		"if_match":  map[string]any{"availability": []any{"2018-06-18T14:00:00Z"}, "if_needed": []any{}},
	})
	if bad == nil || bad.Code != "CONFLICT" || events.replaced != nil {
		t.Fatalf("bad %#v replaced %#v", bad, events.replaced)
	}

	_, bad, _ = Call(deps, user, "muster_set_availability", map[string]any{
		"event_id": events.event.Id.Hex(),
		"availability_ranges": []any{map[string]any{
			"start": "2018-06-18T15:00:00Z", "end": "2018-06-18T16:00:00Z",
		}},
		"if_needed": []any{},
		"if_match":  map[string]any{"availability": []any{"2018-06-18T15:00:00Z"}, "if_needed": []any{}},
	})
	if bad != nil || len(events.replaced) != 2 {
		t.Fatalf("bad %#v replaced %#v", bad, events.replaced)
	}
	events.replaced = nil
	_, bad, _ = Call(deps, user, "muster_set_availability", map[string]any{
		"event_id":     events.event.Id.Hex(),
		"availability": []any{"2018-06-18T16:00:00Z"},
		"if_needed":    []any{},
		"if_match":     "00000000000000000000000000000000",
	})
	if bad == nil || bad.Code != "CONFLICT" || events.replaced != nil {
		t.Fatalf("string if_match %#v replaced %#v", bad, events.replaced)
	}
	fp := stateFingerprint(events.answers[0].Response.Availability, nil)
	_, bad, _ = Call(deps, user, "muster_set_availability", map[string]any{
		"event_id": events.event.Id.Hex(),
		"ranges": []any{map[string]any{
			"start": "2018-06-18T15:00:00Z", "end": "2018-06-18T16:00:00Z", "state": "available",
		}},
		"mode":     "merge",
		"if_match": "bf42notarealfingerprint",
	})
	if bad == nil || bad.Code != "CONFLICT" {
		t.Fatalf("range if_match %#v", bad)
	}
	_, bad, _ = Call(deps, user, "muster_set_availability", map[string]any{
		"event_id": events.event.Id.Hex(),
		"ranges": []any{map[string]any{
			"start": "2018-06-18T15:00:00Z", "end": "2018-06-18T16:00:00Z", "state": "available",
		}},
		"mode":     "merge",
		"if_match": fp,
	})
	if bad != nil {
		t.Fatal(bad.Message)
	}

	sunday := primitive.NewDateTimeFromTime(time.Date(2018, 6, 17, 0, 0, 0, 0, time.UTC))
	sunday2 := primitive.NewDateTimeFromTime(time.Date(2018, 6, 24, 0, 0, 0, 0, time.UTC))
	events.event.Dates = []primitive.DateTime{sunday, sunday2}
	_, bad, _ = Call(deps, user, "muster_set_availability", map[string]any{
		"event_id":     events.event.Id.Hex(),
		"availability": []any{map[string]any{"weekday": "Sunday", "time": "15:00"}},
		"if_needed":    []any{},
	})
	if bad == nil || bad.Code != "AMBIGUOUS_WEEKDAY" {
		t.Fatalf("%#v", bad)
	}
}

func TestListDefaultsToNeedsAnswer(t *testing.T) {
	user := &models.User{Id: primitive.NewObjectID()}
	openID := primitive.NewObjectID()
	doneID := primitive.NewObjectID()
	events := &decisionEvents{listed: []ListedEvent{
		{Event: models.Event{Id: doneID, OwnerId: user.Id, Name: "Done"}, HasResponse: true, NonEmpty: true},
		{Event: models.Event{Id: openID, OwnerId: user.Id, Name: "Open"}},
	}}
	page, bad, _ := Call(Deps{Events: events}, user, "muster_list_my_events", map[string]any{})
	if bad != nil {
		t.Fatal(bad.Message)
	}
	rows := page.(*ListPage).Events
	if len(rows) != 1 || rows[0].EventID != openID.Hex() {
		t.Fatalf("%#v", rows)
	}
	all, bad, _ := Call(Deps{Events: events}, user, "muster_list_my_events", map[string]any{"status": "all"})
	if bad != nil || len(all.(*ListPage).Events) != 2 {
		t.Fatalf("%#v %v", bad, all)
	}
}

// ReplaceAvailabilityIf is a route-level fake: it records the write and can
// simulate a lost compare. The real conditional update is proven against Mongo
// in db.TestMCPAvailabilityCAS.
func (f *decisionEvents) ReplaceAvailabilityIf(_ *models.User, _ primitive.ObjectID, _ AnswerSnapshot, availability, ifNeeded []primitive.DateTime) (bool, error) {
	if f.casMiss {
		return false, nil
	}
	f.replaced, f.replacedNeed = availability, ifNeeded
	return true, nil
}
