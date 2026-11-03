package routes

import (
	"net/http"
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/middleware"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMcpOrganizer(t *testing.T) {
	middleware.ResetMCPLimits()
	ownerID := primitive.NewObjectID()
	guestID := primitive.NewObjectID()
	eventID := primitive.NewObjectID()
	hours := float32(1)
	event := &models.Event{
		Id: eventID, OwnerId: ownerID, Name: "Panel", Type: models.GROUP, Duration: &hours,
		Dates: []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2033, 11, 22, 15, 0, 0, 0, time.UTC))},
	}
	owner := &models.User{Id: ownerID, Email: "owner@x.com", FirstName: "Ada"}
	guest := &models.User{Id: guestID, Email: "guest@x.com", FirstName: "Bea"}
	events := &fakeEvents{
		byHex: map[string]*models.Event{eventID.Hex(): event},
		attendees: map[string][]models.Attendee{
			eventID.Hex(): {
				{Email: "owner@x.com", EventId: eventID},
				{Email: "guest@x.com", EventId: eventID},
			},
		},
		responses: map[string][]models.EventResponse{
			eventID.Hex(): {
				{UserId: ownerID.Hex(), Response: &models.Response{UserId: ownerID, Name: "Ada", Availability: []primitive.DateTime{event.Dates[0]}}},
			},
		},
	}
	tokens := &fakeTokens{active: map[string]*models.McpToken{
		"muster_owner": {UserID: ownerID},
		"muster_guest": {UserID: guestID},
	}}
	users := &fakeUsers{byID: map[string]*models.User{ownerID.Hex(): owner, guestID.Hex(): guest}}
	r := engine(mcp.Deps{Tokens: tokens, Users: users, Events: events}, true)
	tool := func(token, name, args string) (map[string]any, map[string]any) {
		t.Helper()
		rec := postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":`+args+`}}`, token, "", "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %d %s", name, rec.Code, rec.Body.String())
		}
		body := decode(t, rec)
		result := body["result"].(map[string]any)
		return result, result["structuredContent"].(map[string]any)
	}

	t.Run("guest cannot create and owner can", func(t *testing.T) {
		_, created := tool("muster_owner", "muster_create_event", `{"name":"Standup","type":"specific_dates","duration_hours":1,"dates":["2033-11-22T15:00:00Z"]}`)
		if created["short_id"] != "new123" || created["email_sent"] != false || created["page_url"] == "" {
			t.Fatal(created)
		}
		result, bad := tool("muster_owner", "muster_create_event", `{"name":"Week","type":"dow","duration_hours":1,"dates":["2026-11-02T15:00:00Z"]}`)
		if result["isError"] != true || bad["code"] != "BAD_ARGUMENTS" {
			t.Fatal(bad)
		}
	})

	t.Run("attendees hide the guest email from the guest", func(t *testing.T) {
		_, rows := tool("muster_guest", "muster_list_attendees", `{"event_id":"`+eventID.Hex()+`"}`)
		raw := rows["people"]
		text := ""
		for _, person := range raw.([]any) {
			text += person.(map[string]any)["status"].(string)
			if _, ok := person.(map[string]any)["email"]; ok {
				t.Fatal(person)
			}
		}
		if text == "" {
			t.Fatal(rows)
		}
	})

	t.Run("owner can close a time on the grid", func(t *testing.T) {
		result, bad := tool("muster_guest", "muster_finalize", `{"event_id":"`+eventID.Hex()+`","start":"2033-11-22T15:00:00Z","end":"2033-11-22T15:30:00Z"}`)
		if result["isError"] != true || bad["code"] != "NOT_OWNER" {
			t.Fatal(bad)
		}
		_, closed := tool("muster_owner", "muster_finalize", `{"event_id":"`+eventID.Hex()+`","start":"2033-11-22T15:00:00Z","end":"2033-11-22T15:30:00Z"}`)
		archived := events.byHex[eventID.Hex()].IsArchived
		if closed["closed"] != true || closed["email_sent"] != false || archived == nil || !*archived {
			t.Fatal(closed)
		}
		again, bad := tool("muster_owner", "muster_finalize", `{"event_id":"`+eventID.Hex()+`","start":"2033-11-22T16:00:00Z","end":"2033-11-22T16:30:00Z"}`)
		if again["isError"] != true || bad["code"] != "EVENT_ARCHIVED" {
			t.Fatal(bad)
		}
	})

	t.Run("remind lists the guest and remove drops them", func(t *testing.T) {
		_, pending := tool("muster_owner", "muster_remind", `{"event_id":"`+eventID.Hex()+`"}`)
		if pending["email_sent"] != false {
			t.Fatal(pending)
		}
		list := pending["pending"].([]any)
		if len(list) != 1 || list[0] != "guest@x.com" {
			t.Fatal(pending)
		}
		_, removed := tool("muster_owner", "muster_remove_attendee", `{"event_id":"`+eventID.Hex()+`","email":"guest@x.com"}`)
		if removed["removed"] != "guest@x.com" {
			t.Fatal(removed)
		}
	})

	t.Run("owner can archive", func(t *testing.T) {
		_, updated := tool("muster_owner", "muster_update_event", `{"event_id":"`+eventID.Hex()+`","archive":false,"name":"Panel two"}`)
		if updated["updated"] != true || events.byHex[eventID.Hex()].Name != "Panel two" {
			t.Fatal(updated)
		}
	})

	t.Run("best times name who is missing", func(t *testing.T) {
		_, ranked := tool("muster_owner", "muster_find_best_times", `{"event_id":"`+eventID.Hex()+`","duration_minutes":15}`)
		slots, _ := ranked["slots"].([]any)
		if len(slots) != 1 {
			t.Fatal(ranked)
		}
	})

	t.Run("off-grid duration is rejected", func(t *testing.T) {
		result, bad := tool("muster_owner", "muster_find_best_times", `{"event_id":"`+eventID.Hex()+`","duration_minutes":20}`)
		if result["isError"] != true || bad["code"] != "BAD_ARGUMENTS" {
			t.Fatal(bad)
		}
	})
}
