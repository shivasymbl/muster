package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestAllowedRedirect(t *testing.T) {
	if !AllowedRedirect(ClaudeCallback) {
		t.Fatal("claude callback")
	}
	if !AllowedRedirect("http://127.0.0.1:3118/callback") || !AllowedRedirect("http://localhost:3118/callback") {
		t.Fatal("loopback")
	}
	for _, bad := range []string{
		"https://evil.example/api/mcp/auth_callback",
		"https://claude.ai.evil/api/mcp/auth_callback",
		"http://localhost/callback",
		"http://localhost:3118/steal",
		"https://claude.ai/api/mcp/auth_callback?x=1",
		"http://127.0.0.1:3118/callback#x",
	} {
		if AllowedRedirect(bad) {
			t.Fatal(bad)
		}
	}
}

func TestPKCE(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~abcd"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if !PKCE(verifier, challenge) {
		t.Fatal("expected match")
	}
	if PKCE(verifier, challenge+"x") || PKCE("short", challenge) {
		t.Fatal("expected mismatch")
	}
}
