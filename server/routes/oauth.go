package routes

import (
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"github.com/shivasymbl/muster/server/services/oauth"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const oauthReturnCookie = "muster_oauth_return"
const claudeTokenLabel = "Claude"

// InitOAuth registers the sign-in Claude uses. It stays dark until MCP is enabled.
func InitOAuth(router gin.IRoutes, api gin.IRoutes, enabled bool, deps mcp.Deps, store oauth.Store) {
	router.GET("/.well-known/oauth-protected-resource", func(c *gin.Context) { protectedResource(c, enabled) })
	router.GET("/.well-known/oauth-protected-resource/mcp", func(c *gin.Context) { protectedResource(c, enabled) })
	router.GET("/.well-known/oauth-authorization-server", func(c *gin.Context) { authorizationServer(c, enabled) })
	router.POST("/oauth/register", func(c *gin.Context) { registerClient(c, enabled, store) })
	router.GET("/oauth/authorize", func(c *gin.Context) { authorize(c, enabled, deps, store) })
	router.POST("/oauth/authorize", func(c *gin.Context) { approve(c, enabled, deps, store) })
	router.POST("/oauth/token", func(c *gin.Context) { token(c, enabled, deps, store) })
	api.GET("/oauth/return", oauthReturn)
}

func oauthChallenge() string {
	return `Bearer resource_metadata="` + utils.GetBaseUrl() + `/.well-known/oauth-protected-resource/mcp"`
}

func protectedResource(c *gin.Context, enabled bool) {
	if !enabled {
		c.Status(http.StatusNotFound)
		return
	}
	base := utils.GetBaseUrl()
	c.JSON(http.StatusOK, gin.H{
		"resource":                 base + "/mcp",
		"authorization_servers":    []string{base},
		"scopes_supported":         []string{"muster"},
		"bearer_methods_supported": []string{"header"},
	})
}

func authorizationServer(c *gin.Context, enabled bool) {
	if !enabled {
		c.Status(http.StatusNotFound)
		return
	}
	base := utils.GetBaseUrl()
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"registration_endpoint":                 base + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{"offline_access"},
	})
}

func registerClient(c *gin.Context, enabled bool, store oauth.Store) {
	if !enabled {
		c.Status(http.StatusNotFound)
		return
	}
	var body struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if err := c.BindJSON(&body); err != nil {
		return
	}
	if len(body.RedirectURIs) == 0 || len(body.RedirectURIs) > 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_redirect_uri"})
		return
	}
	for _, raw := range body.RedirectURIs {
		if !oauth.AllowedRedirect(raw) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_redirect_uri"})
			return
		}
	}
	name, ok := oauth.ClientName(body.ClientName)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_metadata"})
		return
	}
	id, err := randomToken("mcpclient_")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	if err := store.SaveClient(oauth.Client{ID: id, Name: name, Redirects: body.RedirectURIs}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"client_id":                  id,
		"client_name":                name,
		"redirect_uris":              body.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
}

func authorize(c *gin.Context, enabled bool, deps mcp.Deps, store oauth.Store) {
	if !enabled {
		c.Status(http.StatusNotFound)
		return
	}
	client, redirect, challenge, state, ok := authRequest(c, store)
	if !ok {
		return
	}
	user := sessionUser(c, deps)
	if user == nil {
		rememberAuthorize(c)
		c.Redirect(http.StatusFound, "/sign-in")
		return
	}
	renderConsent(c, client, user, redirect, challenge, state)
}

func approve(c *gin.Context, enabled bool, deps mcp.Deps, store oauth.Store) {
	if !enabled {
		c.Status(http.StatusNotFound)
		return
	}
	client, redirect, challenge, state, ok := authRequest(c, store)
	if !ok {
		return
	}
	user := sessionUser(c, deps)
	if user == nil {
		rememberAuthorize(c)
		c.Redirect(http.StatusFound, "/sign-in")
		return
	}
	if c.PostForm("decision") != "approve" {
		c.Redirect(http.StatusFound, callbackURL(redirect, "", state, "access_denied"))
		return
	}
	plain, err := randomToken("code_")
	if err != nil {
		c.String(http.StatusInternalServerError, "Try again.")
		return
	}
	err = store.SaveCode(plain, oauth.Code{
		UserID: user.Id, ClientID: client.ID, Redirect: redirect, Challenge: challenge, Expires: time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "Try again.")
		return
	}
	c.Redirect(http.StatusFound, callbackURL(redirect, plain, state, ""))
}

func token(c *gin.Context, enabled bool, deps mcp.Deps, store oauth.Store) {
	if !enabled {
		c.Status(http.StatusNotFound)
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	switch c.PostForm("grant_type") {
	case "authorization_code":
		exchangeCode(c, deps, store)
	case "refresh_token":
		exchangeRefresh(c, deps, store)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
	}
}

func exchangeCode(c *gin.Context, deps mcp.Deps, store oauth.Store) {
	code, err := store.TakeCode(c.PostForm("code"))
	if err != nil || code == nil || !code.Expires.After(time.Now().UTC()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	if code.ClientID != c.PostForm("client_id") || code.Redirect != c.PostForm("redirect_uri") || !oauth.PKCE(c.PostForm("code_verifier"), code.Challenge) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	user := deps.Users.GetByID(code.UserID.Hex())
	if user == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	access, public, err := replaceClaudeToken(deps, user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	refresh, err := randomToken("mrefresh_")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	tokenID, _ := primitive.ObjectIDFromHex(public.ID)
	if err := store.SaveRefresh(refresh, oauth.Refresh{UserID: user.Id, ClientID: code.ClientID, TokenID: tokenID, Expires: time.Now().UTC().Add(90 * 24 * time.Hour)}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	writeToken(c, access, refresh, public.ExpiresAt)
}

func exchangeRefresh(c *gin.Context, deps mcp.Deps, store oauth.Store) {
	row, err := store.TakeRefresh(c.PostForm("refresh_token"))
	if err != nil || row == nil || !row.Expires.After(time.Now().UTC()) || (c.PostForm("client_id") != "" && c.PostForm("client_id") != row.ClientID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	_ = deps.Tokens.Revoke(row.UserID, row.TokenID)
	user := deps.Users.GetByID(row.UserID.Hex())
	if user == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	access, public, err := deps.Tokens.Create(user, claudeTokenLabel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	refresh, err := randomToken("mrefresh_")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	tokenID, _ := primitive.ObjectIDFromHex(public.ID)
	if err := store.SaveRefresh(refresh, oauth.Refresh{UserID: user.Id, ClientID: row.ClientID, TokenID: tokenID, Expires: time.Now().UTC().Add(90 * 24 * time.Hour)}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	writeToken(c, access, refresh, public.ExpiresAt)
}

func writeToken(c *gin.Context, access, refresh string, expires time.Time) {
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(time.Until(expires).Seconds()),
		"refresh_token": refresh,
		"scope":         "muster offline_access",
	})
}

func replaceClaudeToken(deps mcp.Deps, user *models.User) (string, models.McpTokenPublic, error) {
	rows, err := deps.Tokens.List(user.Id)
	if err != nil {
		return "", models.McpTokenPublic{}, err
	}
	for _, row := range rows {
		if row.Label == claudeTokenLabel && row.RevokedAt == nil {
			id, err := primitive.ObjectIDFromHex(row.ID)
			if err == nil {
				_ = deps.Tokens.Revoke(user.Id, id)
			}
		}
	}
	return deps.Tokens.Create(user, claudeTokenLabel)
}

func authRequest(c *gin.Context, store oauth.Store) (*oauth.Client, string, string, string, bool) {
	redirect := first(c, "redirect_uri")
	clientID := first(c, "client_id")
	challenge := first(c, "code_challenge")
	state := first(c, "state")
	if first(c, "response_type") != "code" || first(c, "code_challenge_method") != "S256" || !oauth.AllowedRedirect(redirect) {
		c.String(http.StatusBadRequest, "This sign-in request is not valid.")
		return nil, "", "", "", false
	}
	client, err := store.Client(clientID)
	if err != nil || client == nil || !redirectListed(client.Redirects, redirect) {
		c.String(http.StatusBadRequest, "This sign-in request is not valid.")
		return nil, "", "", "", false
	}
	if len(challenge) < 43 || len(challenge) > 128 || strings.ContainsAny(state, "\r\n") || len(state) > 512 {
		c.String(http.StatusBadRequest, "This sign-in request is not valid.")
		return nil, "", "", "", false
	}
	return client, redirect, challenge, state, true
}

func redirectListed(list []string, redirect string) bool {
	for _, item := range list {
		if item == redirect {
			return true
		}
	}
	return false
}

func first(c *gin.Context, key string) string {
	if value := c.PostForm(key); value != "" {
		return value
	}
	return c.Query(key)
}

func sessionUser(c *gin.Context, deps mcp.Deps) *models.User {
	raw := sessions.Default(c).Get("userId")
	id, _ := raw.(string)
	if id == "" || deps.Users == nil {
		return nil
	}
	return deps.Users.GetByID(id)
}

func rememberAuthorize(c *gin.Context) {
	query := c.Request.URL.RawQuery
	if c.Request.Method == http.MethodPost {
		query = c.Request.PostForm.Encode()
	}
	if len(query) == 0 || len(query) > 2000 || strings.ContainsAny(query, "\r\n") {
		return
	}
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	http.SetCookie(c.Writer, &http.Cookie{
		Name: oauthReturnCookie, Value: url.QueryEscape(query), Path: "/", MaxAge: 600,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func oauthReturn(c *gin.Context) {
	cookie, err := c.Request.Cookie(oauthReturnCookie)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"redirect": ""})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: oauthReturnCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	query, err := url.QueryUnescape(cookie.Value)
	if err != nil || len(query) == 0 || len(query) > 2000 || strings.ContainsAny(query, "\r\n") {
		c.JSON(http.StatusOK, gin.H{"redirect": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"redirect": "/oauth/authorize?" + query})
}

func callbackURL(redirect, code, state, oauthError string) string {
	values := url.Values{}
	if oauthError != "" {
		values.Set("error", oauthError)
	} else {
		values.Set("code", code)
	}
	if state != "" {
		values.Set("state", state)
	}
	return redirect + "?" + values.Encode()
}

func renderConsent(c *gin.Context, client *oauth.Client, user *models.User, redirect, challenge, state string) {
	host := "unknown"
	if parsed, err := url.Parse(redirect); err == nil {
		host = parsed.Hostname()
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	_ = consentPage.Execute(c.Writer, map[string]string{
		"Name": client.Name, "Email": user.Email, "Host": host,
		"ClientID": client.ID, "Redirect": redirect, "Challenge": challenge, "State": state,
	})
}

var consentPage = template.Must(template.New("consent").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Connect Claude</title>
<h1>{{.Name}} wants to use Muster as {{.Email}}</h1>
<p>It can read and change the musters on this account. It will return to {{.Host}}.</p>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="response_type" value="code">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.Redirect}}">
<input type="hidden" name="code_challenge" value="{{.Challenge}}">
<input type="hidden" name="code_challenge_method" value="S256">
<input type="hidden" name="state" value="{{.State}}">
<button name="decision" value="approve" type="submit">Approve</button>
<button name="decision" value="deny" type="submit">Deny</button>
</form>
`))

func randomToken(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}
