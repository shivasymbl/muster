package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shivasymbl/muster/server/middleware"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestReviewRegressionGroupEmptyAnswerIsCleared(t *testing.T) {
	ownerID, guestID, eventID := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	owner := &models.User{Id: ownerID, Email: "owner@example.invalid"}
	guest := &models.User{Id: guestID, Email: "guest@example.invalid", FirstName: "Guest"}
	events := &fakeEvents{
		byHex:     map[string]*models.Event{eventID.Hex(): {Id: eventID, OwnerId: ownerID, Type: models.GROUP}},
		attendees: map[string][]models.Attendee{eventID.Hex(): {{Email: guest.Email}}},
		responses: map[string][]models.EventResponse{eventID.Hex(): {{UserId: guestID.Hex(), Response: &models.Response{UserId: guestID}}}},
	}
	deps := mcp.Deps{Events: events, Users: &fakeUsers{byID: map[string]*models.User{ownerID.Hex(): owner, guestID.Hex(): guest}}}
	got, bad, _ := mcp.Call(deps, owner, "muster_list_attendees", map[string]any{"event_id": eventID.Hex()})
	if bad != nil {
		t.Fatal(bad)
	}
	people := got.(map[string]any)["people"].([]map[string]any)
	if len(people) != 1 || people[0]["status"] != "cleared" {
		t.Fatalf("group empty answer reported as not yet answered: %+v", got)
	}
}

func TestReviewRegressionGETCannotBypassExistingAuthBlock(t *testing.T) {
	middleware.ResetMCPLimits()
	tokens := &fakeTokens{}
	r := engine(mcp.Deps{Tokens: tokens}, true)
	for i := 0; i < 31; i++ {
		postMCP(r, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, "muster_invalid", "203.0.113.10:1234", "", "")
	}
	before := tokens.lookups
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("Authorization", "Bearer muster_invalid")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests || tokens.lookups != before {
		t.Fatalf("blocked IP still reached token lookup through GET: status=%d lookups_before=%d after=%d", rec.Code, before, tokens.lookups)
	}
}

func TestReviewRegressionGETFailedAuthJoinsTheBlock(t *testing.T) {
	middleware.ResetMCPLimits()
	tokens := &fakeTokens{}
	r := engine(mcp.Deps{Tokens: tokens}, true)
	var last *httptest.ResponseRecorder
	for i := 0; i < 31; i++ {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.RemoteAddr = "203.0.113.11:1234"
		req.Header.Set("Authorization", "Bearer muster_invalid")
		last = httptest.NewRecorder()
		r.ServeHTTP(last, req)
	}
	if last.Code != http.StatusTooManyRequests || tokens.lookups != 30 {
		t.Fatalf("GET failures did not spend the auth budget: status=%d lookups=%d", last.Code, tokens.lookups)
	}
}

func TestReviewRegressionPanicPreservesRequestID(t *testing.T) {
	middleware.ResetMCPLimits()
	id := primitive.NewObjectID()
	deps := mcp.Deps{
		Tokens: &fakeTokens{active: map[string]*models.McpToken{"muster_test": {UserID: id}}},
		Users:  &fakeUsers{byID: map[string]*models.User{id.Hex(): {Id: id}}},
		Events: &fakeEvents{panicOnGet: true},
	}
	r := engine(deps, true)
	rec := postMCP(r, `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"muster_get_event","arguments":{"event_id":"abc"}}}`, "muster_test", "", "", "")
	body := decode(t, rec)
	if body["id"] != float64(42) {
		t.Fatalf("panic response cannot be correlated with request 42: %s", rec.Body.String())
	}
}
