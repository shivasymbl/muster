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

// maxListBytes keeps one list page under 20,000 characters. A cursor continues the list.
const maxListBytes = 18000

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
	case "muster_set_availability":
		payload, err := setAvailability(deps, user, args)
		return payload, err, eventID
	case "muster_decline":
		payload, err := decline(deps, user, args)
		return payload, err, eventID
	case "muster_add_attendees":
		payload, err := addAttendees(deps, user, args)
		return payload, err, eventID
	case "muster_summarize_overlap":
		payload, err := summarizeOverlap(deps, user, args)
		return payload, err, eventID
	case "muster_create_event":
		payload, err := createEvent(deps, user, args)
		return payload, err, eventID
	case "muster_list_attendees":
		payload, err := listAttendees(deps, user, args)
		return payload, err, eventID
	case "muster_find_best_times":
		payload, err := findBestTimes(deps, user, args)
		return payload, err, eventID
	case "muster_finalize":
		payload, err := finalizeEvent(deps, user, args)
		return payload, err, eventID
	case "muster_update_event":
		payload, err := updateEvent(deps, user, args)
		return payload, err, eventID
	case "muster_remind":
		payload, err := remindAttendees(deps, user, args)
		return payload, err, eventID
	case "muster_remove_attendee":
		payload, err := removeAttendee(deps, user, args)
		return payload, err, eventID
	case "muster_list_changes":
		payload, err := listChanges(deps, user, args)
		return payload, err, eventID
	case "muster_ui_prepare_action", "muster_ui_commit_action", "muster_ui_get_action":
		if !WritesEnabled() {
			return nil, toolErr("BAD_ARGUMENTS", "Unknown tool.", "Call tools/list."), eventID
		}
		payload, err := callAction(deps, user, name, args)
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

var listArgNames = map[string]bool{
	"name": true, "status": true, "limit": true, "cursor": true,
	"needs_my_answer": true, "projection": true,
}

func listEvents(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	if _, ok := args["archived"]; ok {
		return nil, toolErr("BAD_ARGUMENTS", "There is no archived boolean.", "Use status open or status archived. status all includes archived musters.")
	}
	for key := range args {
		if !listArgNames[key] {
			return nil, toolErr("BAD_ARGUMENTS", "Unknown argument "+key+".", "muster_list_my_events accepts name, status, needs_my_answer, projection, limit, and cursor.")
		}
	}
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
	status := "needs_answer"
	if raw, ok := args["status"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", "status must be a string.", "Omit status, or pass needs_answer, all, open or archived.")
		}
		status = strings.TrimSpace(s)
		if status == "" {
			status = "needs_answer"
		}
	}
	needsMine := false
	if raw, ok := args["needs_my_answer"]; ok && raw != nil {
		b, ok := raw.(bool)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", "needs_my_answer must be true or false.", "Pass a boolean, or omit it.")
		}
		needsMine = b
	}
	projection := "full"
	if raw, ok := args["projection"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok || (s != "full" && s != "compact") {
			return nil, toolErr("BAD_ARGUMENTS", "projection must be full or compact.", "Omit projection for the full grid, or pass compact.")
		}
		projection = s
	}
	match, bad := statusMatch(status)
	if bad != nil {
		return nil, bad
	}
	if needsMine {
		inner := match
		match = func(got string) bool { return inner(got) && (got == "no_answer" || got == "invited_no_answer") }
	}
	filters := ListFilters{Status: status, NeedsMyAnswer: needsMine, Projection: projection}
	page := &ListPage{Events: []EventRow{}, Filters: filters}
	size := 0
	for len(page.Events) < limit {
		batch := limit
		if status != "all" || needsMine {
			batch = 50
		}
		rows, next, err := deps.Events.ListForUser(user, name, batch, cursor)
		if err != nil {
			return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
		}
		if len(rows) == 0 {
			break
		}
		filled := false
		for i, row := range rows {
			item := eventRow(user, &row.Event, row.IsAttendee, row.Declined, ResponseFlags{Exists: row.HasResponse, NonEmpty: row.NonEmpty})
			if !match(item.MyStatus) || !archiveMatch(status, item.IsArchived) {
				continue
			}
			size += len(mustJSON(item))
			if size > maxListBytes && len(page.Events) > 0 {
				// The page is full by size. The next page starts after the last row kept.
				page.NextCursor = page.Events[len(page.Events)-1].EventID
				filled = true
				break
			}
			page.Events = append(page.Events, item)
			if len(page.Events) == limit {
				if i < len(rows)-1 || next != "" {
					page.NextCursor = item.EventID
				}
				filled = true
				break
			}
		}
		if filled || next == "" {
			break
		}
		cursor = next
	}
	if projection == "compact" {
		out := &CompactListPage{Events: make([]CompactRow, 0, len(page.Events)), NextCursor: page.NextCursor, Filters: filters}
		for _, r := range page.Events {
			out.Events = append(out.Events, CompactRow{EventID: r.EventID, ShortID: r.ShortID, Name: r.Name, Type: r.Type,
				IsArchived: r.IsArchived, Closed: r.Closed, Scheduled: r.Scheduled, MyRoles: r.MyRoles, MyStatus: r.MyStatus, PageURL: r.PageURL})
		}
		return out, nil
	}
	return page, nil
}

// archiveMatch applies the open and archived statuses. Other statuses do not look at the archive flag.
func archiveMatch(status string, archived bool) bool {
	switch status {
	case "open":
		return !archived
	case "archived":
		return archived
	}
	return true
}

func statusMatch(status string) (func(string) bool, *ToolError) {
	switch status {
	case "all", "open", "archived":
		return func(string) bool { return true }, nil
	case "needs_answer":
		return func(got string) bool { return got == "no_answer" || got == "invited_no_answer" }, nil
	case "no_answer", "invited_no_answer", "responded", "cleared", "declined":
		return func(got string) bool { return got == status }, nil
	default:
		return nil, toolErr("BAD_ARGUMENTS", "status must be needs_answer, all, open, archived, or one my_status value.", "Omit status to list musters that still need an answer.")
	}
}

func getEvent(deps Deps, user *models.User, args map[string]any) (*EventDetail, *ToolError) {
	id, err := requiredString(args, "event_id")
	if err != nil {
		return nil, err
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	return eventDetail(deps, user, event), nil
}

// eventDetail is the muster_get_event shape. update and create echo it too.
func eventDetail(deps Deps, user *models.User, event *models.Event) *EventDetail {
	attendee, declined := attendeeState(deps, user, event.Id)
	flags := deps.Events.ResponseFlags(user.Id.Hex(), []primitive.ObjectID{event.Id})[event.Id.Hex()]
	out := &EventDetail{EventRow: eventRow(user, event, attendee, declined, flags)}
	if event.OwnerId == user.Id {
		out.ResponseTotals = responseTotals(deps, user, event)
	}
	return out
}

func getResponses(deps Deps, user *models.User, args map[string]any) (*ResponsesPayload, *ToolError) {
	id, bad := requiredString(args, "event_id")
	if bad != nil {
		return nil, bad
	}
	// An explicit window is checked before any read. A defaulted one needs the event's dates.
	explicit := presentList(args, "start") || presentList(args, "end")
	var start, end time.Time
	if explicit {
		if start, end, bad = window(args); bad != nil {
			return nil, bad
		}
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	if utils.Coalesce(event.IsSignUpForm) {
		return nil, toolErr("SIGN_UP_FORM", "This muster is a sign-up form.", utils.GetBaseUrl()+"/e/"+event.GetId())
	}
	if !explicit {
		if start, end, bad = optionalWindow(event, args); bad != nil {
			return nil, bad
		}
	}
	responses := deps.Events.Responses(event.Id)
	attendees := attendeeList(deps, event)
	viewer := eventlogic.Viewer{
		UserID: user.Id.Hex(),
		Email:  strings.ToLower(strings.TrimSpace(user.Email)),
	}
	filtered := eventlogic.FilterResponses(event, viewer, responses, attendees, start, end)
	payload := buildResponses(deps, event, user.Id.Hex(), start, end, filtered)
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
		Name:              truncateRunes(strings.TrimSpace(event.Name), 200),
		Type:              string(event.Type),
		IsArchived:        utils.Coalesce(event.IsArchived),
		BlindAvailability: utils.Coalesce(event.BlindAvailabilityEnabled),
		MyRoles:           MyRoles(event.OwnerId == user.Id, isAttendee),
		MyStatus:          MyStatus(flags, isAttendee, declined),
		PageURL:           utils.GetBaseUrl() + "/e/" + event.GetId(),
		DisplayTimezone:   event.DisplayTimezone,
		Grid:              eventGrid(event),
	}
	if event.ShortId != nil {
		row.ShortID = *event.ShortId
	}
	row.Scheduled = snapshotEvent(event, false, false).Scheduled
	row.Closed = row.Scheduled != nil
	return row
}

func buildResponses(deps Deps, event *models.Event, callerKey string, start, end time.Time, filtered map[string]*models.Response) *ResponsesPayload {
	inc := increment(event)
	people := make([]Person, 0, len(filtered))
	for key, response := range filtered {
		if response == nil {
			continue
		}
		person := Person{
			Key:       key,
			Name:      displayName(deps, response),
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

func displayName(deps Deps, response *models.Response) string {
	if name := strings.TrimSpace(personName(response)); name != "" {
		return name
	}
	if response.UserId.IsZero() || deps.Users == nil {
		return ""
	}
	user := deps.Users.GetByID(response.UserId.Hex())
	if user == nil {
		return ""
	}
	return strings.TrimSpace(user.FirstName + " " + user.LastName)
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
