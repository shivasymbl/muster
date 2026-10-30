package mcp

// ToolError is the object returned in structuredContent when isError is true.
type ToolError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	WhatToDoNext string `json:"what_to_do_next"`
	Retryable    bool   `json:"retryable"`
}

func toolErr(code, message, next string) *ToolError {
	return &ToolError{Code: code, Message: message, WhatToDoNext: next, Retryable: code == "INTERNAL"}
}

// EventRow is one muster on the caller's list. It is not a models.Event.
type EventRow struct {
	EventID           string   `json:"event_id"`
	ShortID           string   `json:"short_id,omitempty"`
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	IsArchived        bool     `json:"is_archived"`
	BlindAvailability bool     `json:"blind_availability"`
	MyRoles           []string `json:"my_roles"`
	MyStatus          string   `json:"my_status"`
	PageURL           string   `json:"page_url"`
}

// ListPage is the muster_list_my_events result.
type ListPage struct {
	Events     []EventRow `json:"events"`
	NextCursor string     `json:"next_cursor"`
}

// Whoami is the muster_whoami result.
type Whoami struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// TimeRange is a merged block of availability.
type TimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Person is one respondent the caller is allowed to see.
type Person struct {
	Key       string      `json:"key"`
	Name      string      `json:"name"`
	Email     string      `json:"email,omitempty"`
	Available []TimeRange `json:"available"`
	IfNeeded  []TimeRange `json:"if_needed"`
	Manual    []TimeRange `json:"manual"`
}

// ResponsesPayload is the muster_get_responses result.
type ResponsesPayload struct {
	EventID   string   `json:"event_id"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Truncated bool     `json:"truncated"`
	Hint      string   `json:"hint"`
	People    []Person `json:"people"`
}
