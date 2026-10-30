package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
)

// AuthenticateMCP accepts only a bearer token. It does not read the website session.
// ok is false for a missing, unknown, expired, revoked, or deleted-user token.
func AuthenticateMCP(c *gin.Context, deps mcp.Deps) (*models.User, bool) {
	header := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return nil, false
	}
	plaintext := strings.TrimSpace(header[len(prefix):])
	if !strings.HasPrefix(plaintext, "muster_") || deps.Tokens == nil {
		return nil, false
	}
	row, err := deps.Tokens.GetActive(plaintext)
	if err != nil || row == nil {
		return nil, false
	}
	if deps.Users == nil {
		return nil, false
	}
	user := deps.Users.GetByID(row.UserID.Hex())
	if user == nil {
		return nil, false
	}
	c.Set("mcpUser", user)
	return user, true
}
