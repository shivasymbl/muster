package mcp

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shivasymbl/muster/server/models"
	eventlogic "github.com/shivasymbl/muster/server/services/events"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const maxResponseBytes = 4096

// Call runs one tool for the token user. An extra user_id argument is ignored.
func Call(deps Deps, user *models.User, name string, args map[string]any) (any, *ToolError, string) {
	eventID := "-"
	if args != nil {
		if raw, ok := args["event_id"].(string); ok && raw != "" && !strings.Contains(raw, "@") {
			eventID = raw
		}
	}
	switch name {
	case "muster_whoami":
		return whoami(user), nil, eventID
	case "muster_list_my_events":
		page, err := listEvents(deps, user, args)
		return page, err, "-"
	case "muster_get_event":
		row, err := getEvent(deps, user, args)
		return row, err, eventID
	case "muster_get_responses":
		payload, err := getResponses(deps, user, args)
		return payload, err, eventID
	default:
		return nil, toolErr("BAD_ARGUMENTS", "Unknown tool.", "Call tools/list."), eventID
	}
}

func whoami(user *models.User) Whoami {
	return Whoami{
		UserID: user.Id.Hex(),
		Email:  user.Email,
		Name:   strings.TrimSpace(user.FirstName + " " + user.LastName),
	}
}

func listEvents(deps Deps, user *models.User, args map[string]any) (*ListPage, *ToolError) {
	limit := 20
	if raw, ok := args["limit"]; ok && raw != nil {
		n, ok := wholeNumber(raw)
		if !ok || n < 1 {
			return nil, toolErr("BAD_ARGUMENTS", "limit must be an integer of at least 1.", "Omit limit or pass an integer from 1 to 50.")
		}
		if n > 50 {
			n = 50
		}
		limit = n
	}
	name := ""
	if raw, ok := args["name"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", "name must be a string.", "Omit name or pass a string of at most 80 characters.")
		}
		if utf8.RuneCountInString(s) > 80 {
			return nil, toolErr("BAD_ARGUMENTS", "name is too long.", "Use a name of at most 80 characters.")
		}
		name = s
	}
	cursor := ""
	if raw, ok := args["cursor"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok || !validCursor(s) {
			return nil, toolErr("BAD_ARGUMENTS", "cursor must be a 24-character hex id.", "Use next_cursor from the previous page, or omit it.")
		}
		cursor = s
	}
	rows, next, err := deps.Events.ListForUser(user, name, limit, cursor)
	if err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	// toolErr INTERNAL sets Retryable only when code is INTERNAL. toolErr does that.
	page := &ListPage{Events: make([]EventRow, 0, len(rows)), NextCursor: next}
	for _, row := range rows {
		page.Events = append(page.Events, eventRow(user, &row.Event, row.IsAttendee, row.Declined, ResponseFlags{Exists: row.HasResponse, NonEmpty: row.NonEmpty}))
	}
	return page, nil
}

func getEvent(deps Deps, user *models.User, args map[string]any) (*EventRow, *ToolError) {
	id, err := requiredString(args, "event_id")
	if err != nil {
		return nil, err
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	attendee, declined := attendeeState(deps, user, event.Id)
	flags := deps.Events.ResponseFlags(user.Id.Hex(), []primitive.ObjectID{event.Id})[event.Id.Hex()]
	row := eventRow(user, event, attendee, declined, flags)
	return &row, nil
}

func getResponses(deps Deps, user *models.User, args map[string]any) (*ResponsesPayload, *ToolError) {
	id, bad := requiredString(args, "event_id")
	if bad != nil {
		return nil, bad
	}
	startRaw, bad := requiredString(args, "start")
	if bad != nil {
		return nil, bad
	}
	endRaw, bad := requiredString(args, "end")
	if bad != nil {
		return nil, bad
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return nil, toolErr("BAD_ARGUMENTS", "start must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return nil, toolErr("BAD_ARGUMENTS", "end must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
	}
	if !end.After(start) {
		return nil, toolErr("BAD_ARGUMENTS", "end must be after start.", "Pass an end timestamp after start.")
	}
	if end.Sub(start) > 7*24*time.Hour {
		return nil, toolErr("WINDOW_TOO_WIDE", "The window must stay within 7 days.", "Narrow start and end. The window must stay within 7 days.")
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	if utils.Coalesce(event.IsSignUpForm) {
		return nil, toolErr("SIGN_UP_FORM", "This muster is a sign-up form.", utils.GetBaseUrl()+"/e/"+event.GetId())
	}
	responses := deps.Events.Responses(event.Id)
	attendees := attendeeList(deps, event)
	viewer := eventlogic.Viewer{
		UserID: user.Id.Hex(),
		Email:  strings.ToLower(strings.TrimSpace(user.Email)),
	}
	filtered := eventlogic.FilterResponses(event, viewer, responses, attendees, start, end)
	payload := buildResponses(event, user.Id.Hex(), start, end, filtered)
	applyCap(payload, user.Id.Hex())
	return payload, nil
}

func member(deps Deps, user *models.User, event *models.Event) bool {
	if event == nil || user == nil {
		return false
	}
	if event.OwnerId == user.Id {
		return true
	}
	for _, a := range deps.Events.Attendees(event.Id) {
		if a.Email == user.Email {
			return true
		}
	}
	return deps.Events.ResponseFlags(user.Id.Hex(), []primitive.ObjectID{event.Id})[event.Id.Hex()].Exists
}

func attendeeList(deps Deps, event *models.Event) []models.Attendee {
	if event.Attendees != nil {
		return *event.Attendees
	}
	return deps.Events.Attendees(event.Id)
}

func attendeeState(deps Deps, user *models.User, eventID primitive.ObjectID) (bool, *bool) {
	for _, a := range deps.Events.Attendees(eventID) {
		if a.Email == user.Email {
			return true, a.Declined
		}
	}
	return false, nil
}

func eventRow(user *models.User, event *models.Event, isAttendee bool, declined *bool, flags ResponseFlags) EventRow {
	row := EventRow{
		EventID:           event.Id.Hex(),
		Name:              truncateRunes(event.Name, 200),
		Type:              string(event.Type),
		IsArchived:        utils.Coalesce(event.IsArchived),
		BlindAvailability: utils.Coalesce(event.BlindAvailabilityEnabled),
		MyRoles:           MyRoles(event.OwnerId == user.Id, isAttendee),
		MyStatus:          MyStatus(flags, isAttendee, declined),
		PageURL:           utils.GetBaseUrl() + "/e/" + event.GetId(),
	}
	if event.ShortId != nil {
		row.ShortID = *event.ShortId
	}
	return row
}

func buildResponses(event *models.Event, callerKey string, start, end time.Time, filtered map[string]*models.Response) *ResponsesPayload {
	inc := increment(event)
	people := make([]Person, 0, len(filtered))
	for key, response := range filtered {
		if response == nil {
			continue
		}
		person := Person{
			Key:       key,
			Name:      personName(response),
			Available: mergeMarks(dateTimes(response.Availability), inc),
			IfNeeded:  mergeMarks(dateTimes(response.IfNeeded), inc),
			Manual:    mergeMarks(manualMarks(response, start, end), inc),
		}
		if email := personEmail(response); email != "" {
			person.Email = email
		}
		people = append(people, person)
	}
	sort.Slice(people, func(i, j int) bool { return people[i].Key < people[j].Key })
	for i, person := range people {
		if person.Key == callerKey {
			if i != 0 {
				people = append([]Person{person}, append(people[:i:i], people[i+1:]...)...)
			}
			break
		}
	}
	return &ResponsesPayload{
		EventID:   event.Id.Hex(),
		Start:     start.UTC().Format(time.RFC3339),
		End:       end.UTC().Format(time.RFC3339),
		Truncated: false,
		Hint:      "",
		People:    people,
	}
}

func personName(response *models.Response) string {
	if response.Name != "" {
		return response.Name
	}
	if response.User != nil {
		return strings.TrimSpace(response.User.FirstName + " " + response.User.LastName)
	}
	return ""
}

func personEmail(response *models.Response) string {
	if response.Email != "" {
		return response.Email
	}
	if response.User != nil {
		return response.User.Email
	}
	return ""
}

func manualMarks(response *models.Response, start, end time.Time) []time.Time {
	out := make([]time.Time, 0)
	for day, values := range utils.Coalesce(response.ManualAvailability) {
		if day.Time().Before(start) || day.Time().After(end) {
			continue
		}
		for _, value := range values {
			t := value.Time()
			if !t.Before(start) && !t.After(end) {
				out = append(out, t)
			}
		}
	}
	return out
}

func applyCap(payload *ResponsesPayload, callerKey string) {
	if len(mustJSON(payload)) <= maxResponseBytes {
		return
	}
	payload.Truncated = true
	payload.Hint = "Narrow start and end. The window must stay within 7 days."
	for len(payload.People) > 1 && len(mustJSON(payload)) > maxResponseBytes {
		last := payload.People[len(payload.People)-1]
		if last.Key == callerKey {
			break
		}
		payload.People = payload.People[:len(payload.People)-1]
	}
	if len(mustJSON(payload)) > maxResponseBytes && len(payload.People) >= 1 {
		payload.People[0].Available = []TimeRange{}
		payload.People[0].IfNeeded = []TimeRange{}
		payload.People[0].Manual = []TimeRange{}
		if payload.People[0].Key != callerKey {
			payload.People = payload.People[:1]
		}
	}
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte{'{', '}'}
	}
	return raw
}

func requiredString(args map[string]any, key string) (string, *ToolError) {
	raw, ok := args[key]
	s, isString := raw.(string)
	if !ok || !isString || strings.TrimSpace(s) == "" {
		return "", toolErr("BAD_ARGUMENTS", key+" is required.", "Pass "+key+" as a string.")
	}
	return s, nil
}

func wholeNumber(raw any) (int, bool) {
	switch n := raw.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func validCursor(s string) bool {
	if len(s) != 24 {
		return false
	}
	_, err := primitive.ObjectIDFromHex(s)
	return err == nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func notFound() *ToolError {
	return toolErr("EVENT_NOT_FOUND_OR_NO_ACCESS", "No muster found.", "Call muster_list_my_events and use an id from that list.")
}
