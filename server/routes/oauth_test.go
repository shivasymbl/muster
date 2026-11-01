package routes

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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

type memOAuth struct {
	clients map[string]oauth.Client
	codes   map[string]oauth.Code
	refresh map[string]oauth.Refresh
}

func (m *memOAuth) SaveClient(client oauth.Client) error {
	if m.clients == nil {
		m.clients = map[string]oauth.Client{}
	}
	m.clients[client.ID] = client
	return nil
}
func (m *memOAuth) Client(id string) (*oauth.Client, error) {
	client, ok := m.clients[id]
	if !ok {
		return nil, nil
	}
	return &client, nil
}
func (m *memOAuth) SaveCode(plain string, code oauth.Code) error {
	if m.codes == nil {
		m.codes = map[string]oauth.Code{}
	}
	m.codes[plain] = code
	return nil
}
func (m *memOAuth) TakeCode(plain string) (*oauth.Code, error) {
	code, ok := m.codes[plain]
	if !ok {
		return nil, nil
	}
	delete(m.codes, plain)
	return &code, nil
}
func (m *memOAuth) SaveRefresh(plain string, row oauth.Refresh) error {
	if m.refresh == nil {
		m.refresh = map[string]oauth.Refresh{}
	}
	m.refresh[plain] = row
	return nil
}
func (m *memOAuth) TakeRefresh(plain string) (*oauth.Refresh, error) {
	row, ok := m.refresh[plain]
	if !ok {
		return nil, nil
	}
	delete(m.refresh, plain)
	return &row, nil
}

type memTokens struct {
	rows []models.McpTokenPublic
	live map[string]string
}

func (m *memTokens) Create(user *models.User, label string) (string, models.McpTokenPublic, error) {
	active := 0
	for _, row := range m.rows {
		if row.RevokedAt == nil {
			active++
		}
	}
	if active >= 5 {
		return "", models.McpTokenPublic{}, mcp.ErrActiveTokenCap
	}
	id := primitive.NewObjectID()
	plain := "muster_" + id.Hex()
	row := models.McpTokenPublic{ID: id.Hex(), Label: label, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)}
	m.rows = append(m.rows, row)
	if m.live == nil {
		m.live = map[string]string{}
	}
	m.live[plain] = user.Id.Hex()
	return plain, row, nil
}
func (m *memTokens) List(primitive.ObjectID) ([]models.McpTokenPublic, error) {
	return m.rows, nil
}
func (m *memTokens) Revoke(_, tokenID primitive.ObjectID) error {
	now := time.Now().UTC()
	for i := range m.rows {
		if m.rows[i].ID == tokenID.Hex() {
			m.rows[i].RevokedAt = &now
		}
	}
	return nil
}
func (m *memTokens) GetActive(string) (*models.McpToken, error) { return nil, nil }

func TestOAuthSignIn(t *testing.T) {
	userID := primitive.NewObjectID()
	user := &models.User{Id: userID, Email: "ada@example.com", FirstName: "Ada"}
	tokens := &memTokens{}
	store := &memOAuth{}
	deps := mcp.Deps{Tokens: tokens, Users: &fakeUsers{byID: map[string]*models.User{userID.Hex(): user}}}
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))))
	InitOAuth(r, r.Group("/api"), true, deps, store)
	r.GET("/prime", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("userId", userID.Hex())
		_ = session.Save()
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "registration_endpoint") || !strings.Contains(rec.Body.String(), "S256") {
		t.Fatal(rec.Body.String())
	}

	evil := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"redirect_uris":["https://evil.example/callback"],"client_name":"Nope"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(evil, req)
	if evil.Code != http.StatusBadRequest {
		t.Fatal(evil.Body.String())
	}

	ok := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"redirect_uris":["https://claude.ai/api/mcp/auth_callback"],"client_name":"Claude"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(ok, req)
	if ok.Code != http.StatusCreated {
		t.Fatal(ok.Body.String())
	}
	var registered map[string]any
	if err := json.Unmarshal(ok.Body.Bytes(), &registered); err != nil {
		t.Fatal(err)
	}
	clientID := registered["client_id"].(string)

	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~abcd"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	query := url.Values{
		"response_type": {"code"}, "client_id": {clientID},
		"redirect_uri": {oauth.ClaudeCallback}, "code_challenge": {challenge},
		"code_challenge_method": {"S256"}, "state": {"xyz"},
	}.Encode()

	guest := httptest.NewRecorder()
	r.ServeHTTP(guest, httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query, nil))
	if guest.Code != http.StatusFound || guest.Header().Get("Location") != "/sign-in" || guest.Result().Cookies()[0].Name != oauthReturnCookie {
		t.Fatalf("status %d location %s", guest.Code, guest.Header().Get("Location"))
	}

	prime := httptest.NewRecorder()
	r.ServeHTTP(prime, httptest.NewRequest(http.MethodGet, "/prime", nil))
	sessionCookie := prime.Result().Cookies()[0]

	consent := httptest.NewRecorder()
	creq := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query, nil)
	creq.AddCookie(sessionCookie)
	r.ServeHTTP(consent, creq)
	page := consent.Body.String()
	if consent.Code != http.StatusOK || !strings.Contains(page, "ada@example.com") || !strings.Contains(page, "claude.ai") {
		t.Fatal(page)
	}

	form := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {oauth.ClaudeCallback},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"xyz"}, "decision": {"approve"},
	}
	approved := httptest.NewRecorder()
	areq := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	areq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	areq.AddCookie(sessionCookie)
	r.ServeHTTP(approved, areq)
	location := approved.Header().Get("Location")
	if approved.Code != http.StatusFound || !strings.Contains(location, "code=") || !strings.Contains(location, "state=xyz") {
		t.Fatal(location)
	}
	parsed, _ := url.Parse(location)
	code := parsed.Query().Get("code")

	bad := httptest.NewRecorder()
	body := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {oauth.ClaudeCallback}, "client_id": {clientID}, "code_verifier": {verifier}}.Encode()
	treq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	treq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(bad, treq)
	if bad.Code != http.StatusOK || !strings.Contains(bad.Body.String(), `"token_type":"Bearer"`) || !strings.Contains(bad.Body.String(), "muster_") {
		t.Fatal(bad.Body.String())
	}
	var issued map[string]any
	if err := json.Unmarshal(bad.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	refresh := issued["refresh_token"].(string)

	again := httptest.NewRecorder()
	body = url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}}.Encode()
	treq = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	treq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(again, treq)
	if again.Code != http.StatusOK || !strings.Contains(again.Body.String(), "mrefresh_") {
		t.Fatal(again.Body.String())
	}
	reused := httptest.NewRecorder()
	treq = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	treq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(reused, treq)
	if reused.Code != http.StatusBadRequest || !strings.Contains(reused.Body.String(), "invalid_grant") {
		t.Fatal(reused.Body.String())
	}
	claude := 0
	for _, row := range tokens.rows {
		if row.Label == "Claude" && row.RevokedAt == nil {
			claude++
		}
	}
	if claude != 1 {
		t.Fatalf("active claude tokens %d", claude)
	}
}
