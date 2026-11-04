package mcp

import (
	"encoding/json"
	"strings"
)

// ToolError is the object returned in structuredContent when isError is true.
// The flat fields keep their original values. Marshaling adds a canonical data object that
// repeats them with a canonical code and, where it helps a repair, the offending path, index,
// bounded value and the valid bounds. The detail fields stay unexported so ToolError is comparable.
type ToolError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	WhatToDoNext string `json:"what_to_do_next"`
	Retryable    bool   `json:"retryable"`

	path     string
	index    int
	hasIndex bool
	value    string
	bounds   string // JSON object
}

// ErrorData is the canonical, text-serializable error detail.
type ErrorData struct {
	Code         string          `json:"code"`
	Message      string          `json:"message"`
	WhatToDoNext string          `json:"what_to_do_next"`
	Path         string          `json:"path,omitempty"`
	Index        *int            `json:"index,omitempty"`
	Value        string          `json:"value,omitempty"`
	Bounds       json.RawMessage `json:"bounds,omitempty"`
}

// Detail is the canonical data object.
func (e ToolError) Detail() ErrorData {
	d := ErrorData{Code: canonicalCode(e.Code, e.Message), Message: e.Message, WhatToDoNext: e.WhatToDoNext, Path: e.path, Value: e.value}
	if e.hasIndex {
		i := e.index
		d.Index = &i
	}
	if e.bounds != "" {
		d.Bounds = json.RawMessage(e.bounds)
	}
	return d
}

type toolErrorWire struct {
	Code         string    `json:"code"`
	Message      string    `json:"message"`
	WhatToDoNext string    `json:"what_to_do_next"`
	Retryable    bool      `json:"retryable"`
	Data         ErrorData `json:"data"`
}

// MarshalJSON adds the canonical data object next to the flat fields.
func (e ToolError) MarshalJSON() ([]byte, error) {
	return json.Marshal(toolErrorWire{e.Code, e.Message, e.WhatToDoNext, e.Retryable, e.Detail()})
}

func toolErr(code, message, next string) *ToolError {
	return &ToolError{Code: code, Message: message, WhatToDoNext: next, Retryable: code == "INTERNAL"}
}

// canonicalCode maps the flat codes that differ from the canonical list. Flat codes stay as they are.
func canonicalCode(code, message string) string {
	switch code {
	case "EVENT_NOT_FOUND_OR_NO_ACCESS":
		return "NOT_FOUND"
	case "WINDOW_TOO_WIDE":
		return "WINDOW_TOO_LONG"
	case "BAD_ARGUMENTS":
		if strings.HasPrefix(message, "Only a group muster") {
			return "NOT_GROUP"
		}
		if strings.HasPrefix(message, "The owner stays") {
			return "OWNER_NOT_REMOVABLE"
		}
	}
	return code
}

// at records where the problem is. index < 0 means no index. The value is cut to 80 characters.
func (e *ToolError) at(path string, index int, value string) *ToolError {
	e.path = path
	e.index, e.hasIndex = index, index >= 0
	e.value = cleanValue(value)
	return e
}

// within adds the valid bounds. Call it only after the caller is known to be a member.
func (e *ToolError) within(bounds map[string]any) *ToolError {
	raw, err := json.Marshal(bounds)
	if err == nil {
		e.bounds = string(raw)
	}
	return e
}

func cleanValue(v string) string {
	out := make([]rune, 0, 80)
	for _, r := range v {
		if len(out) == 80 {
			break
		}
		if r < 0x20 || r == 0x7f {
			r = ' '
		}
		out = append(out, r)
	}
	return string(out)
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
	// Scheduled is the stored final interval, or null. Closed means that interval exists.
	// It is independent of IsArchived, which a manual archive also sets.
	Scheduled *ScheduledSpan `json:"scheduled"`
	Closed    bool           `json:"closed"`
	// DisplayTimezone is a presentation label. Null means unspecified, not UTC.
	DisplayTimezone *string `json:"display_timezone"`
	Grid
}

// EventDetail is the muster_get_event result: the row plus owner-only totals.
type EventDetail struct {
	EventRow
	// ResponseTotals is null for anyone but the owner, never a fabricated zero.
	ResponseTotals *ResponseTotals `json:"response_totals"`
}

// ResponseTotals counts the owner's roster. Population says who is counted.
type ResponseTotals struct {
	Population         string `json:"population"`
	Invited            int    `json:"invited"`
	Responded          int    `json:"responded"`
	Cleared            int    `json:"cleared"`
	Declined           int    `json:"declined"`
	Pending            int    `json:"pending"`
	OwnerIsParticipant bool   `json:"owner_is_participant"`
	OwnerStatus        string `json:"owner_status,omitempty"`
}

// CompactRow is one muster in the compact list projection. It leaves out the grid.
type CompactRow struct {
	EventID    string         `json:"event_id"`
	ShortID    string         `json:"short_id,omitempty"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	IsArchived bool           `json:"is_archived"`
	Closed     bool           `json:"closed"`
	Scheduled  *ScheduledSpan `json:"scheduled"`
	MyRoles    []string       `json:"my_roles"`
	MyStatus   string         `json:"my_status"`
	PageURL    string         `json:"page_url"`
}

// ListFilters states what the list applied.
type ListFilters struct {
	Status        string `json:"status"`
	NeedsMyAnswer bool   `json:"needs_my_answer"`
	Projection    string `json:"projection"`
}

// CompactListPage is the muster_list_my_events result with projection compact.
type CompactListPage struct {
	Events     []CompactRow `json:"events"`
	NextCursor string       `json:"next_cursor"`
	Filters    ListFilters  `json:"filters"`
}

// ListPage is the muster_list_my_events result.
type ListPage struct {
	Events     []EventRow  `json:"events"`
	NextCursor string      `json:"next_cursor"`
	Filters    ListFilters `json:"filters"`
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
