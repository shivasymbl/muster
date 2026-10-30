package routes

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var mcpTokenStore mcp.TokenStore

// @Summary Create an assistant token
// @Tags user
// @Accept json
// @Produce json
// @Param body body object true "label"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /user/mcp-tokens [post]
func createMcpToken(c *gin.Context) {
	var body struct {
		Label string `json:"label"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label is required"})
		return
	}
	label := strings.TrimSpace(body.Label)
	if label == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label is required"})
		return
	}
	if utf8.RuneCountInString(label) > 40 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label is too long"})
		return
	}
	plaintext, public, err := mcpTokenStore.Create(utils.GetAuthUser(c), label)
	if errors.Is(err, mcp.ErrActiveTokenCap) {
		c.JSON(http.StatusConflict, gin.H{"error": "revoke a token first"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Try again."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token":     plaintext,
		"id":        public.ID,
		"label":     public.Label,
		"createdAt": public.CreatedAt,
		"expiresAt": public.ExpiresAt,
	})
}

// @Summary List assistant tokens
// @Tags user
// @Produce json
// @Success 200 {array} models.McpTokenPublic
// @Router /user/mcp-tokens [get]
func listMcpTokens(c *gin.Context) {
	rows, err := mcpTokenStore.List(utils.GetAuthUser(c).Id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Try again."})
		return
	}
	if rows == nil {
		rows = []models.McpTokenPublic{}
	}
	c.JSON(http.StatusOK, rows)
}

// @Summary Revoke an assistant token
// @Tags user
// @Success 204
// @Failure 404 {object} map[string]string
// @Param tokenId path string true "Token id"
// @Router /user/mcp-tokens/{tokenId} [delete]
func revokeMcpToken(c *gin.Context) {
	tokenID, err := primitive.ObjectIDFromHex(c.Param("tokenId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	err = mcpTokenStore.Revoke(utils.GetAuthUser(c).Id, tokenID)
	if errors.Is(err, mcp.ErrTokenNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Try again."})
		return
	}
	c.Status(http.StatusNoContent)
}
