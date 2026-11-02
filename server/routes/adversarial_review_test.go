package routes

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"github.com/shivasymbl/muster/server/services/oauth"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestAdversarialRevokedGrantCannotRefresh(t *testing.T) {
	id := primitive.NewObjectID()
	user := &models.User{Id: id, Email: "review@example.invalid"}
	tokens := &memTokens{}
	_, token, _ := tokens.Create(user, "Claude")
	tokenID, _ := primitive.ObjectIDFromHex(token.ID)
	store := &memOAuth{}
	_ = store.SaveRefresh("review_refresh", oauth.Refresh{UserID: id, ClientID: "review_client", TokenID: tokenID, Expires: time.Now().Add(time.Hour)})
	_ = tokens.Revoke(id, tokenID)
	deps := mcp.Deps{Tokens: tokens, Users: &fakeUsers{byID: map[string]*models.User{id.Hex(): user}}}
	r := gin.New()
	InitOAuth(r, r.Group("/api"), true, deps, store)
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"review_refresh"}, "client_id": {"review_client"}}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("revoked grant refreshed: %d", rec.Code)
	}
}

func TestAdversarialConsentMustBindRequestAndResource(t *testing.T) {
	id := primitive.NewObjectID()
	user := &models.User{Id: id, Email: "review@example.invalid"}
	store := &memOAuth{}
	_ = store.SaveClient(oauth.Client{ID: "review_client", Name: "Review", Redirects: []string{oauth.ClaudeCallback}})
	deps := mcp.Deps{Tokens: &memTokens{}, Users: &fakeUsers{byID: map[string]*models.User{id.Hex(): user}}}
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))))
	InitOAuth(r, r.Group("/api"), true, deps, store)
	r.GET("/prime", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("userId", id.Hex())
		_ = s.Save()
		c.Status(http.StatusNoContent)
	})
	prime := httptest.NewRecorder()
	r.ServeHTTP(prime, httptest.NewRequest(http.MethodGet, "/prime", nil))
	verifier := strings.Repeat("a", 43)
	sum := sha256.Sum256([]byte(verifier))
	form := url.Values{
		"response_type": {"code"}, "client_id": {"review_client"}, "redirect_uri": {oauth.ClaudeCallback},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"},
		"decision": {"approve"}, "resource": {"https://wrong.example.invalid/mcp"}, "scope": {"unexpected"},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(prime.Result().Cookies()[0])
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusFound && strings.Contains(rec.Header().Get("Location"), "code=") {
		t.Fatal("code issued without a consent nonce")
	}
}

func TestAdversarialAvailabilityRequiresExplicitReplacement(t *testing.T) {
	id, eventID := primitive.NewObjectID(), primitive.NewObjectID()
	user := &models.User{Id: id}
	mark := primitive.NewDateTimeFromTime(time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC))
	events := &fakeEvents{
		byHex:     map[string]*models.Event{eventID.Hex(): {Id: eventID, OwnerId: id}},
		responses: map[string][]models.EventResponse{eventID.Hex(): {{UserId: id.Hex(), Response: &models.Response{Availability: []primitive.DateTime{mark}}}}},
	}
	_, err, _ := mcp.Call(mcp.Deps{Events: events}, user, "muster_set_availability", map[string]any{"event_id": eventID.Hex()})
	if err == nil {
		t.Fatal("event_id alone changed the answer")
	}
	if len(events.responses[eventID.Hex()][0].Response.Availability) != 1 {
		t.Fatal("stored answer was cleared")
	}
}

func TestAdversarialAvailabilityRejectsOutsideGrid(t *testing.T) {
	id, eventID := primitive.NewObjectID(), primitive.NewObjectID()
	user := &models.User{Id: id}
	duration := float32(2)
	events := &fakeEvents{
		byHex: map[string]*models.Event{eventID.Hex(): {Id: eventID, OwnerId: id, Type: models.SPECIFIC_DATES, Duration: &duration, Dates: []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC))}}},
	}
	_, err, _ := mcp.Call(mcp.Deps{Events: events}, user, "muster_set_availability", map[string]any{"event_id": eventID.Hex(), "availability": []any{"2030-01-01T10:07:00Z"}})
	if err == nil {
		t.Fatal("out-of-window availability accepted")
	}
}
