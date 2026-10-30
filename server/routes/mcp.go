package routes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shivasymbl/muster/server/logger"
	"github.com/shivasymbl/muster/server/middleware"
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
)

const mcpBodyLimit = 16384

type rpcRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// InitMcp registers POST /mcp and GET /mcp. The route stays registered when the flag is off
// so the website fallback cannot answer it with HTML.
func InitMcp(router *gin.Engine, enabled bool, deps mcp.Deps) {
	router.GET("/mcp", func(c *gin.Context) {
		c.Status(http.StatusMethodNotAllowed)
	})
	router.POST("/mcp", func(c *gin.Context) {
		handleMcp(c, enabled, deps)
	})
}

func handleMcp(c *gin.Context, enabled bool, deps mcp.Deps) {
	start := time.Now()
	requestID := newRequestID()
	toolName := "-"
	userID := "-"
	eventID := "-"
	outcome := "ok"
	wrote := false

	defer func() {
		if rec := recover(); rec != nil && !wrote {
			outcome = "internal"
			writeToolResult(c, nil, nil, &mcp.ToolError{
				Code: "INTERNAL", Message: "Try again.", WhatToDoNext: "Wait and retry.", Retryable: true,
			})
		}
		logMcp(requestID, toolName, userID, eventID, outcome, start)
	}()

	if !enabled {
		outcome = "disabled"
		wrote = true
		c.JSON(http.StatusNotFound, gin.H{"code": "MCP_DISABLED", "message": "MCP is off."})
		return
	}

	body, tooBig := readLimited(c.Request.Body, mcpBodyLimit)
	if tooBig {
		outcome = "too_large"
		wrote = true
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"code": "TOO_LARGE", "message": "Request body is over 16384 bytes.", "retryable": false,
		})
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Method == "" {
		outcome = "bad_arguments"
		wrote = true
		c.JSON(http.StatusBadRequest, gin.H{
			"code": "BAD_ARGUMENTS", "message": "Body is not JSON.", "retryable": false,
		})
		return
	}
	toolName = req.Method

	switch req.Method {
	case "notifications/initialized":
		wrote = true
		c.Status(http.StatusAccepted)
		return
	case "initialize":
		wrote = true
		c.JSON(http.StatusOK, gin.H{
			"jsonrpc": "2.0",
			"id":      rpcID(req.ID),
			"result": gin.H{
				"protocolVersion": "2025-03-26",
				"capabilities":    gin.H{"tools": gin.H{"listChanged": false}},
				"serverInfo":      gin.H{"name": "muster", "version": "1"},
			},
		})
		return
	case "tools/list", "tools/call", "ping":
	default:
		outcome = "bad_arguments"
		wrote = true
		c.JSON(http.StatusOK, gin.H{
			"jsonrpc": "2.0",
			"id":      rpcID(req.ID),
			"error":   gin.H{"code": -32601, "message": "Method not found"},
		})
		return
	}

	now := time.Now()
	if deps.Now != nil {
		now = deps.Now()
	}
	if middleware.AuthBlocked(c, now) {
		outcome = "rate_limited"
		wrote = true
		writeRPCError(c, req.ID, http.StatusTooManyRequests, -32029, "Too many sign-in attempts.", gin.H{
			"code": "RATE_LIMITED", "message": "Too many sign-in attempts.", "what_to_do_next": "Wait and retry.", "retryable": true,
		})
		return
	}
	user, ok := middleware.AuthenticateMCP(c, deps)
	if !ok {
		middleware.NoteAuthFail(c, now)
		outcome = "not_signed_in"
		wrote = true
		writeRPCError(c, req.ID, http.StatusUnauthorized, -32001, "Sign in by creating a token in Muster Settings.", gin.H{
			"code":            "NOT_SIGNED_IN",
			"message":         "Sign in by creating a token in Muster Settings.",
			"what_to_do_next": "Create a token in Muster Settings and set Authorization: Bearer.",
			"retryable":       false,
		})
		return
	}
	userID = user.Id.Hex()
	if !middleware.AllowUser(userID, now) {
		outcome = "rate_limited"
		wrote = true
		writeRPCError(c, req.ID, http.StatusTooManyRequests, -32029, "Too many tool calls.", gin.H{
			"code": "RATE_LIMITED", "message": "Too many tool calls.", "what_to_do_next": "Wait and retry.", "retryable": true,
		})
		return
	}

	switch req.Method {
	case "ping":
		wrote = true
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": rpcID(req.ID), "result": gin.H{}})
	case "tools/list":
		wrote = true
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": rpcID(req.ID), "result": gin.H{"tools": toolDefs()}})
	case "tools/call":
		result, toolErr, calledEvent := callTool(deps, user, req.Params)
		if calledEvent != "" {
			eventID = calledEvent
		}
		if toolErr != nil {
			outcome = outcomeFor(toolErr.Code)
		}
		wrote = true
		writeToolResult(c, rpcID(req.ID), result, toolErr)
	}
}

func callTool(deps mcp.Deps, user *models.User, params json.RawMessage) (any, *mcp.ToolError, string) {
	var body struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &body); err != nil {
			return nil, mcpErr("BAD_ARGUMENTS", "arguments are not an object.", "Pass arguments as a JSON object."), "-"
		}
	}
	if body.Arguments == nil {
		body.Arguments = map[string]any{}
	}
	result, toolErr, eventID := mcp.Call(deps, user, body.Name, body.Arguments)
	return result, toolErr, eventID
}

func writeToolResult(c *gin.Context, id any, result any, toolErr *mcp.ToolError) {
	if toolErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"jsonrpc": "2.0",
			"id":      id,
			"result": gin.H{
				"content":           []gin.H{{"type": "text", "text": toolErr.Message}},
				"structuredContent": toolErr,
				"isError":           true,
			},
		})
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		writeToolResult(c, id, nil, &mcp.ToolError{Code: "INTERNAL", Message: "Try again.", WhatToDoNext: "Wait and retry.", Retryable: true})
		return
	}
	var structured any
	_ = json.Unmarshal(raw, &structured)
	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"id":      id,
		"result": gin.H{
			"content":           []gin.H{{"type": "text", "text": string(raw)}},
			"structuredContent": structured,
			"isError":           false,
		},
	})
}

func writeRPCError(c *gin.Context, id json.RawMessage, status, code int, message string, data any) {
	c.JSON(status, gin.H{
		"jsonrpc": "2.0",
		"id":      rpcID(id),
		"error":   gin.H{"code": code, "message": message, "data": data},
	})
}

func rpcID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(id, &v); err != nil {
		return nil
	}
	return v
}

func readLimited(body io.ReadCloser, limit int) ([]byte, bool) {
	if body == nil {
		return []byte("{}"), false
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, int64(limit)+1))
	if err != nil {
		return nil, false
	}
	return raw, len(raw) > limit
}

func newRequestID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func logMcp(requestID, tool, userID, eventID, outcome string, start time.Time) {
	if logger.StdOut == nil {
		return
	}
	logger.StdOut.Printf("mcp request_id=%s tool=%s user_id=%s event_id=%s outcome=%s latency_ms=%d",
		requestID, tool, userID, eventID, outcome, time.Since(start).Milliseconds())
}

func outcomeFor(code string) string {
	switch code {
	case "EVENT_NOT_FOUND_OR_NO_ACCESS":
		return "not_found"
	case "BAD_ARGUMENTS":
		return "bad_arguments"
	case "WINDOW_TOO_WIDE":
		return "window"
	case "SIGN_UP_FORM":
		return "sign_up"
	case "INTERNAL":
		return "internal"
	default:
		return "bad_arguments"
	}
}

func toolDefs() []gin.H {
	return []gin.H{
		{"name": "muster_whoami", "description": "Who the token belongs to.", "inputSchema": gin.H{"type": "object", "properties": gin.H{}}},
		{"name": "muster_list_my_events", "description": "Musters this signed-in user owns, was invited to, or answered.", "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"name":   gin.H{"type": "string"},
				"limit":  gin.H{"type": "integer"},
				"cursor": gin.H{"type": "string"},
			},
		}},
		{"name": "muster_get_event", "description": "One muster the caller is a member of.", "inputSchema": gin.H{
			"type":       "object",
			"properties": gin.H{"event_id": gin.H{"type": "string"}},
			"required":   []string{"event_id"},
		}},
		{"name": "muster_get_responses", "description": "Availability the caller is allowed to see, merged into ranges.", "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id": gin.H{"type": "string"},
				"start":    gin.H{"type": "string"},
				"end":      gin.H{"type": "string"},
			},
			"required": []string{"event_id", "start", "end"},
		}},
	}
}

func mcpErr(code, message, next string) *mcp.ToolError {
	return &mcp.ToolError{Code: code, Message: message, WhatToDoNext: next, Retryable: code == "INTERNAL"}
}
