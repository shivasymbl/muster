package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Case 1 (duplicate own answers) is already fully asserted in
// TestMCPAppsSnapshot "duplicate own records hide only the edit", including
// that the event name still opens, so it is not repeated here.

// Case 2: a blind muster hides both the people and candidates payloads from a
// non-owner member. snapshot_test.go checks People; this pins Candidates and
// the serialized output.
func TestGapBlindMemberCandidatesHaveNoPayload(t *testing.T) {
	owner := &models.User{Id: primitive.NewObjectID(), Email: "owner@x.com", FirstName: "Olive"}
	member := &models.User{Id: primitive.NewObjectID(), Email: "member@x.com", FirstName: "Mel"}
	other := &models.User{Id: primitive.NewObjectID(), Email: "other@x.com", FirstName: "Otto"}
	hours := float32(2)
	slot := 30
	blind := true
	ev := &models.Event{
		Id: primitive.NewObjectID(), OwnerId: owner.Id, Name: "Plan", Type: models.GROUP,
		Dates: []primitive.DateTime{dt(9, 0)}, Duration: &hours, TimeIncrement: &slot,
		BlindAvailabilityEnabled: &blind,
	}
	row := func(u *models.User) models.EventResponse {
		return models.EventResponse{UserId: u.Id.Hex(), Response: &models.Response{
			Name: u.FirstName, Email: u.Email, UserId: u.Id, Availability: []primitive.DateTime{dt(9, 0)},
		}}
	}
	fe := &snapEvents{attendees: []models.Attendee{{Email: member.Email}, {Email: other.Email}}}
	fe.event = ev
	fe.answers = []models.EventResponse{row(member), row(other)}
	d := Deps{Events: fe, Users: snapUsers{owner.Email: owner, member.Email: member, other.Email: other}}

	s, bad := BuildSnapshot(d, member, SnapshotRequest{Workflow: "pick-a-time", EventID: "x"})
	if bad != nil {
		t.Fatal(bad)
	}
	for name, sec := range map[string]Section{"people": s.Sections.People, "candidates": s.Sections.Candidates} {
		if sec.Status != SectionHidden || sec.Data != nil {
			t.Fatalf("%s not hidden without payload: %+v", name, sec)
		}
	}
	raw, _ := json.Marshal(s)
	if strings.Contains(string(raw), "Otto") || strings.Contains(string(raw), "other@x.com") {
		t.Fatalf("leak: %s", raw)
	}
}

// Case 3: a boolean if_match is a malformed argument; the string "true" is a
// well-formed fingerprint that does not match. Neither may write.
func TestGapIfMatchBooleanVsString(t *testing.T) {
	user := &models.User{Id: primitive.NewObjectID(), Email: "o@x.com"}
	day := primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 0, 0, 0, 0, time.UTC))
	hours := float32(24)
	slot := 30
	events := &decisionEvents{
		event: &models.Event{
			Id: user.Id, OwnerId: user.Id, Type: models.DOW, Dates: []primitive.DateTime{day},
			Duration: &hours, TimeIncrement: &slot,
		},
		answers: []models.EventResponse{{
			UserId: user.Id.Hex(),
			Response: &models.Response{Availability: []primitive.DateTime{
				primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 15, 0, 0, 0, time.UTC)),
			}},
		}},
	}
	deps := Deps{Events: events}
	call := func(ifMatch any) *ToolError {
		_, bad, _ := Call(deps, user, "muster_set_availability", map[string]any{
			"event_id":     events.event.Id.Hex(),
			"availability": []any{"2018-06-18T16:00:00Z"},
			"if_needed":    []any{},
			"if_match":     ifMatch,
		})
		return bad
	}
	if bad := call(true); bad == nil || bad.Code != "BAD_ARGUMENTS" || events.replaced != nil || events.replacedNeed != nil {
		t.Fatalf("bool: %#v replaced %#v", bad, events.replaced)
	}
	if bad := call("true"); bad == nil || bad.Code != "CONFLICT" || events.replaced != nil || events.replacedNeed != nil {
		t.Fatalf("string: %#v replaced %#v", bad, events.replaced)
	}
}
