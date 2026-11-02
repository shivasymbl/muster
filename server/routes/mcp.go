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
		if !enabled {
			c.Status(http.StatusMethodNotAllowed)
			return
		}
		c.Header("WWW-Authenticate", oauthChallenge())
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": "NOT_SIGNED_IN", "message": "Sign in by creating a token in Muster Settings.",
		})
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
	if req.Method == "tools/call" {
		var called struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(req.Params, &called) == nil && called.Name != "" {
			toolName = called.Name
		}
	}

	switch req.Method {
	case "notifications/initialized":
		wrote = true
		c.Status(http.StatusAccepted)
		return
	case "initialize", "tools/list", "tools/call", "ping":
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
		c.Header("WWW-Authenticate", oauthChallenge())
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
	case "NOT_OWNER", "NOT_AN_INVITEE", "OVERLAP_HIDDEN", "SLOT_OUTSIDE", "SLOT_OFF_GRID", "SLOT_IN_BOTH":
		return "bad_arguments"
	case "INTERNAL":
		return "internal"
	default:
		return "bad_arguments"
	}
}

func toolDefs() []gin.H {
	id := gin.H{"type": "string", "description": "24-character event_id or the short id, for example QTTyVU."}
	window := "RFC3339 timestamp. start and end must be at most 7 days apart. A weekly muster stores its times on the reference week 2018-06-17 through 2018-06-24. Use the dates from muster_get_event, not a later calendar week."
	read := gin.H{"readOnlyHint": true, "idempotentHint": true, "destructiveHint": false, "openWorldHint": false}
	return []gin.H{
		{"name": "muster_whoami", "description": "Who the token belongs to. Returns user_id, email, and name.", "annotations": read, "inputSchema": gin.H{"type": "object", "properties": gin.H{}}},
		{"name": "muster_list_my_events", "description": "Musters this signed-in user owns, was invited to, or answered. name is a case-insensitive substring. limit is 1 to 50 and defaults to 20. cursor is the next_cursor from the previous page. my_status is one of no_answer, invited_no_answer, responded, cleared, declined. type is specific_dates, dow, or group. is_group is true only when type is group. cleared means an answer was saved and then emptied. It is not no_answer.", "annotations": read, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"name":   gin.H{"type": "string", "description": "Case-insensitive name substring. At most 80 characters."},
				"limit":  gin.H{"type": "integer", "description": "Page size from 1 to 50. Defaults to 20."},
				"cursor": gin.H{"type": "string", "description": "24-character hex next_cursor from the previous page."},
			},
		}},
		{"name": "muster_get_event", "description": "One muster the caller is a member of. Returns the grid: dates as UTC instants, duration_hours, slot_minutes, days_only, start_on_monday, and is_group. A weekly muster also returns weekdays, and its dates are on the reference week 2018-06-17 through 2018-06-24. Muster does not store a timezone. A group muster is the only kind with an invite list.", "annotations": read, "inputSchema": gin.H{
			"type":       "object",
			"properties": gin.H{"event_id": id},
			"required":   []string{"event_id"},
		}},
		{"name": "muster_get_responses", "description": "Availability the caller is allowed to see, merged into ranges. People include a display name when one is allowed. manual is day-level marks and is empty when the person only used the slot grid. A hidden muster shows the caller only their own answer.", "annotations": read, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id": id,
				"start":    gin.H{"type": "string", "description": window + " Example: 2026-10-23T16:00:00Z."},
				"end":      gin.H{"type": "string", "description": window},
			},
			"required": []string{"event_id", "start", "end"},
		}},
		{"name": "muster_set_availability", "description": "Replace the caller's whole answer. This does not merge. Pass at least one of availability or if_needed. An empty list clears that list. Omitting both, or passing null, leaves the answer unchanged. Omitting one list clears that list. An empty answer leaves my_status as cleared, not no_answer. Each time is an RFC3339 start of one slot, for example 2026-10-23T16:00:00Z. When the muster has dates, a time outside those windows or off slot_minutes is rejected. The same time cannot be in both lists. Duplicate times are stored once and reported as duplicates_ignored.", "annotations": gin.H{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": true, "openWorldHint": false}, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id":     id,
				"availability": gin.H{"type": "array", "description": "RFC3339 slot starts that replace the previous available times.", "items": gin.H{"type": "string"}},
				"if_needed":    gin.H{"type": "array", "description": "RFC3339 slot starts that replace the previous if-needed times.", "items": gin.H{"type": "string"}},
			},
			"required": []string{"event_id"},
		}},
		{"name": "muster_decline", "description": "Decline a group muster the caller was invited to. is_group must be true. This does not delete an availability answer.", "annotations": gin.H{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": true, "openWorldHint": false}, "inputSchema": gin.H{
			"type":       "object",
			"properties": gin.H{"event_id": id},
			"required":   []string{"event_id"},
		}},
		{"name": "muster_add_attendees", "description": "Owner only. Add email addresses to a group muster (is_group true). Does not remove anyone. An address already invited is skipped. Invite mail is sent only when Listmonk is configured. email_sent is false when it is not. Returns the addresses actually added.", "annotations": gin.H{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": true}, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id": id,
				"emails":   gin.H{"type": "array", "description": "Plain email addresses, at most 20.", "items": gin.H{"type": "string"}},
			},
			"required": []string{"event_id", "emails"},
		}},
		{"name": "muster_create_event", "description": "Create a muster owned by the signed-in user. type is specific_dates, dow, or group. dates are RFC3339 starts. A weekly muster must use the reference week 2018-06-17 through 2018-06-24. duration_hours is the length of each day. slot_minutes is 15, 30, or 60. emails are allowed only when type is group. Invite mail is sent only when Listmonk is on. Returns event_id, short_id, and page_url.", "annotations": gin.H{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": true}, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"name":            gin.H{"type": "string", "description": "1 to 80 characters."},
				"type":            gin.H{"type": "string", "description": "specific_dates, dow, or group."},
				"dates":           gin.H{"type": "array", "items": gin.H{"type": "string"}, "description": "RFC3339 start of each day."},
				"duration_hours":  gin.H{"type": "number", "description": "Hours in each day, greater than 0 and at most 24."},
				"slot_minutes":    gin.H{"type": "integer", "description": "15, 30, or 60. Defaults to 15."},
				"blind":           gin.H{"type": "boolean"},
				"days_only":       gin.H{"type": "boolean"},
				"start_on_monday": gin.H{"type": "boolean"},
				"emails":          gin.H{"type": "array", "items": gin.H{"type": "string"}, "description": "Invitees. Group musters only. At most 20."},
			},
			"required": []string{"name", "type", "duration_hours"},
		}},
		{"name": "muster_list_attendees", "description": "Who was invited and who answered. status is invited, responded, declined, or cleared. Email is included for the owner only. A hidden muster shows a non-owner only their own row.", "annotations": read, "inputSchema": gin.H{
			"type": "object", "properties": gin.H{"event_id": id}, "required": []string{"event_id"},
		}},
		{"name": "muster_find_best_times", "description": "Rank slot starts by how many respondents can make a meeting of duration_minutes. Includes if-needed counts and who is missing. Defaults the window to the muster's dates, capped at 7 days. Does not change the unanimous muster_summarize_overlap result. Refuses when availability is hidden and the caller is not the owner.", "annotations": read, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id":         id,
				"start":            gin.H{"type": "string", "description": window},
				"end":              gin.H{"type": "string", "description": window},
				"duration_minutes": gin.H{"type": "integer", "description": "Meeting length. Defaults to slot_minutes."},
			},
			"required": []string{"event_id"},
		}},
		{"name": "muster_finalize", "description": "Owner only. Store the chosen start and end and mark the muster closed. Both ends must sit on the grid. No finalize mail is sent, so email_sent is false. This does not create a calendar event.", "annotations": gin.H{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": true, "openWorldHint": false}, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id": id,
				"start":    gin.H{"type": "string", "description": "RFC3339 start on the grid."},
				"end":      gin.H{"type": "string", "description": "RFC3339 end on the grid, after start."},
			},
			"required": []string{"event_id", "start", "end"},
		}},
		{"name": "muster_update_event", "description": "Owner only. Rename, replace dates, change duration_hours, archive, or set blind. It cannot change the owner or the type.", "annotations": gin.H{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": true, "openWorldHint": false}, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id":       id,
				"name":           gin.H{"type": "string"},
				"dates":          gin.H{"type": "array", "items": gin.H{"type": "string"}},
				"duration_hours": gin.H{"type": "number"},
				"archive":        gin.H{"type": "boolean"},
				"blind":          gin.H{"type": "boolean"},
			},
			"required": []string{"event_id"},
		}},
		{"name": "muster_remind", "description": "Owner only. List group invitees who have not answered. No reminder mail is sent, because there is no reminder template, so email_sent is false.", "annotations": gin.H{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}, "inputSchema": gin.H{
			"type": "object", "properties": gin.H{"event_id": id}, "required": []string{"event_id"},
		}},
		{"name": "muster_remove_attendee", "description": "Owner only. Remove one invitee from a group muster and delete that person's answer, matching the website. The owner cannot be removed.", "annotations": gin.H{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": true, "openWorldHint": false}, "inputSchema": gin.H{
			"type":       "object",
			"properties": gin.H{"event_id": id, "email": gin.H{"type": "string"}},
			"required":   []string{"event_id", "email"},
		}},
		{"name": "muster_list_changes", "description": "Owner only. Answers and invites recorded at or after since. since is RFC3339. Older website edits that have no timestamp are left out. There is no webhook.", "annotations": read, "inputSchema": gin.H{
			"type":       "object",
			"properties": gin.H{"event_id": id, "since": gin.H{"type": "string", "description": "RFC3339 timestamp."}},
			"required":   []string{"event_id", "since"},
		}},
		{"name": "muster_summarize_overlap", "description": "Times every stored answer in the window can make. Returns those slots with display names, not user ids. An answer with no free time in the window blocks the result. People who have not answered are left out. It returns nothing when no slot suits every stored answer. A mark exactly at the window end is outside. Refuses when availability is hidden and the caller is not the owner.", "annotations": read, "inputSchema": gin.H{
			"type": "object",
			"properties": gin.H{
				"event_id": id,
				"start":    gin.H{"type": "string", "description": window},
				"end":      gin.H{"type": "string", "description": window},
			},
			"required": []string{"event_id", "start", "end"},
		}},
	}
}

func mcpErr(code, message, next string) *mcp.ToolError {
	return &mcp.ToolError{Code: code, Message: message, WhatToDoNext: next, Retryable: code == "INTERNAL"}
}
