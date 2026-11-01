package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const ClaudeCallback = "https://claude.ai/api/mcp/auth_callback"

// AllowedRedirect accepts Claude's hosted callback and a loopback callback on any port.
func AllowedRedirect(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return false
	}
	if raw == ClaudeCallback && parsed.Scheme == "https" && parsed.Host == "claude.ai" && parsed.Path == "/api/mcp/auth_callback" {
		return true
	}
	if parsed.Scheme != "http" || parsed.Path != "/callback" {
		return false
	}
	host := parsed.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		return false
	}
	port, err := strconv.Atoi(parsed.Port())
	return err == nil && port >= 1 && port <= 65535
}

// PKCE checks an S256 challenge. The verifier is the secret the client still holds.
func PKCE(verifier, challenge string) bool {
	if verifier == "" || challenge == "" || len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

// ClientName trims a registration name. An empty name becomes Claude.
func ClientName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "Claude", true
	}
	if utf8.RuneCountInString(name) > 80 || strings.ContainsAny(name, "\r\n") {
		return "", false
	}
	return name, true
}
