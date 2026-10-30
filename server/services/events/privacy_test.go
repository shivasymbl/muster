package events

import (
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func dt(t time.Time) primitive.DateTime {
	return primitive.NewDateTimeFromTime(t)
}

func testEvent(owner primitive.ObjectID, blind, collect bool, typ models.EventType) *models.Event {
	return &models.Event{
		Id:                       primitive.NewObjectID(),
		OwnerId:                  owner,
		Type:                     typ,
		BlindAvailabilityEnabled: &blind,
		CollectEmails:            &collect,
	}
}

func TestFilterResponsesBlindAndEmail(t *testing.T) {
	owner := primitive.NewObjectID()
	other := primitive.NewObjectID()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	in := dt(start.Add(time.Hour))
	out := dt(start.Add(48 * time.Hour))

	responses := []models.EventResponse{
		{UserId: owner.Hex(), Response: &models.Response{Availability: []primitive.DateTime{in, out}, Email: "owner@x.com", User: &models.User{Email: "owner@x.com", CalendarAccounts: map[string]models.CalendarAccount{"a": {}}}}},
		{UserId: other.Hex(), Response: &models.Response{Availability: []primitive.DateTime{in}, Email: "other@x.com", User: &models.User{Email: "other@x.com"}}},
		{UserId: "Guest", Response: &models.Response{Availability: []primitive.DateTime{in}, Email: "g@x.com", Name: "Guest"}},
	}
	attendees := []models.Attendee{{Email: "other@x.com"}}

	t.Run("blind off returns others", func(t *testing.T) {
		ev := testEvent(owner, false, true, models.SPECIFIC_DATES)
		got := FilterResponses(ev, Viewer{UserID: other.Hex(), Email: "other@x.com"}, clone(responses), attendees, start, end)
		if _, ok := got[owner.Hex()]; !ok {
			t.Fatal("expected the owner's response")
		}
		if len(got[owner.Hex()].Availability) != 1 {
			t.Fatalf("window filter: got %d marks", len(got[owner.Hex()].Availability))
		}
	})

	t.Run("blind on owner returns others", func(t *testing.T) {
		ev := testEvent(owner, true, true, models.SPECIFIC_DATES)
		got := FilterResponses(ev, Viewer{UserID: owner.Hex(), Email: "owner@x.com"}, clone(responses), attendees, start, end)
		if len(got) != 3 {
			t.Fatalf("owner saw %d people", len(got))
		}
		if got[owner.Hex()].User.CalendarAccounts != nil {
			t.Fatal("calendar accounts were not stripped")
		}
	})

	t.Run("blind on non-owner returns only self", func(t *testing.T) {
		ev := testEvent(owner, true, false, models.SPECIFIC_DATES)
		got := FilterResponses(ev, Viewer{UserID: other.Hex(), Email: "other@x.com"}, clone(responses), attendees, start, end)
		if len(got) != 1 {
			t.Fatalf("non-owner saw %d people", len(got))
		}
		if _, ok := got[other.Hex()]; !ok {
			t.Fatal("missing own response")
		}
	})

	t.Run("guest name returns only that guest", func(t *testing.T) {
		ev := testEvent(owner, true, false, models.SPECIFIC_DATES)
		got := FilterResponses(ev, Viewer{GuestName: "Guest"}, clone(responses), nil, start, end)
		if len(got) != 1 || got["Guest"] == nil {
			t.Fatalf("guest map: %#v", got)
		}
	})

	t.Run("empty viewer returns an empty map", func(t *testing.T) {
		ev := testEvent(owner, true, false, models.SPECIFIC_DATES)
		got := FilterResponses(ev, Viewer{}, clone(responses), nil, start, end)
		if len(got) != 0 {
			t.Fatalf("got %d", len(got))
		}
	})

	t.Run("group invitee keeps email when collect is false", func(t *testing.T) {
		ev := testEvent(owner, false, false, models.GROUP)
		got := FilterResponses(ev, Viewer{UserID: other.Hex(), Email: "other@x.com"}, clone(responses), attendees, start, end)
		if got[other.Hex()].User.Email != "other@x.com" {
			t.Fatalf("email %q", got[other.Hex()].User.Email)
		}
	})

	t.Run("non-group strips email", func(t *testing.T) {
		ev := testEvent(owner, false, false, models.SPECIFIC_DATES)
		got := FilterResponses(ev, Viewer{UserID: other.Hex(), Email: "other@x.com"}, clone(responses), attendees, start, end)
		if got[other.Hex()].User.Email != "" {
			t.Fatalf("email %q", got[other.Hex()].User.Email)
		}
	})
}

func TestKeepGroupEmailDoesNotNeedMongo(t *testing.T) {
	if KeepGroupEmail(models.SPECIFIC_DATES, "a@b.com", false, nil) {
		t.Fatal("non-group")
	}
	if !KeepGroupEmail(models.GROUP, "", true, nil) {
		t.Fatal("owner")
	}
	if !KeepGroupEmail(models.GROUP, "a@b.com", false, []models.Attendee{{Email: " A@b.com "}}) {
		t.Fatal("invitee")
	}
}

func clone(in []models.EventResponse) []models.EventResponse {
	out := make([]models.EventResponse, len(in))
	for i, row := range in {
		out[i] = row
		if row.Response != nil {
			cp := *row.Response
			if row.Response.User != nil {
				u := *row.Response.User
				cp.User = &u
			}
			out[i].Response = &cp
		}
	}
	return out
}
