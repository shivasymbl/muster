package mcp

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/listmonk"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func createEvent(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	name, bad := requiredString(args, "name")
	if bad != nil {
		return nil, bad
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		return nil, toolErr("BAD_ARGUMENTS", "name must be 1 to 80 characters.", "Pass a shorter name.")
	}
	kind, bad := requiredString(args, "type")
	if bad != nil {
		return nil, bad
	}
	eventType := models.EventType(kind)
	if eventType != models.SPECIFIC_DATES && eventType != models.DOW && eventType != models.GROUP {
		return nil, toolErr("BAD_ARGUMENTS", "type must be specific_dates, dow, or group.", "Use one of those three.")
	}
	duration, bad := requiredHours(args, "duration_hours")
	if bad != nil {
		return nil, bad
	}
	minutes, bad := requiredIncrement(args)
	if bad != nil {
		return nil, bad
	}
	dates, bad := markList(nil, eventType, args, "dates")
	if bad != nil {
		return nil, bad
	}
	if eventType != models.GROUP && len(dates) == 0 {
		return nil, toolErr("BAD_ARGUMENTS", "dates is required.", "Pass the start of each day as RFC3339.")
	}
	if len(dates) > 31 {
		return nil, toolErr("BAD_ARGUMENTS", "At most 31 dates.", "Shorten the muster.")
	}
	if eventType == models.DOW {
		for _, date := range dates {
			if !onReferenceWeek(date.Time()) {
				return nil, toolErr("BAD_ARGUMENTS", "A weekly muster uses the reference week 2018-06-17 through 2018-06-24.", "Build dates on that week, with the UTC clock you want each weekday.")
			}
		}
	}
	emails := []string{}
	if raw, ok := args["emails"]; ok && raw != nil {
		if eventType != models.GROUP {
			return nil, toolErr("BAD_ARGUMENTS", "Only a group muster has an invite list.", "Set type to group, or omit emails.")
		}
		list, isList := raw.([]any)
		if !isList {
			return nil, toolErr("BAD_ARGUMENTS", "emails must be an array.", "Pass plain email addresses.")
		}
		seen := map[string]bool{}
		for i, item := range list {
			address, ok := item.(string)
			if !ok {
				return nil, toolErr("BAD_ARGUMENTS", "Each email must be a string.", "Pass plain email addresses.").at(fmt.Sprintf("emails[%d]", i), i, "")
			}
			raw := address
			address, ok = canonicalEmail(address)
			if !ok {
				return nil, toolErr("BAD_ARGUMENTS", "One address is not an email.", "Fix the address and call again.").at(fmt.Sprintf("emails[%d]", i), i, raw)
			}
			if address == canonicalOrLower(user.Email) || seen[address] {
				continue
			}
			seen[address] = true
			emails = append(emails, address)
		}
		if len(emails) > maxInvitees {
			return nil, toolErr("BAD_ARGUMENTS", "At most 20 emails per call.", "Split the list and call again.")
		}
	}
	event := models.Event{
		Name:                     name,
		Type:                     eventType,
		Duration:                 &duration,
		TimeIncrement:            &minutes,
		Dates:                    dates,
		BlindAvailabilityEnabled: boolPtr(args, "blind"),
		DaysOnly:                 boolPtr(args, "days_only"),
		StartOnMonday:            boolPtr(args, "start_on_monday"),
	}
	created, err := deps.Events.Create(user, event, emails)
	if err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	emailSent := os.Getenv("LISTMONK_URL") != "" && os.Getenv("LISTMONK_ENABLED") != "false" && len(emails) > 0
	if emailSent {
		ownerName := strings.TrimSpace(user.FirstName + " " + user.LastName)
		if ownerName == "" {
			ownerName = "Somebody"
		}
		for _, address := range emails {
			listmonk.SendEmailAddSubscriberIfNotExist(address, 9, bson.M{
				"ownerName": ownerName,
				"groupName": created.Name,
				"groupUrl":  utils.GetBaseUrl() + "/g/" + created.GetId(),
			}, false, listmonk.EmailOptions{ReplyTo: listmonk.ReplyToForUser(user)})
		}
	}
	return map[string]any{
		"event_id":   created.Id.Hex(),
		"short_id":   deref(created.ShortId),
		"page_url":   utils.GetBaseUrl() + "/e/" + created.GetId(),
		"is_group":   created.Type == models.GROUP,
		"email_sent": emailSent,
		"invited":    emails,
		"event":      eventDetail(deps, user, created),
	}, nil
}

func listAttendees(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	event, bad := ownedOrMember(deps, user, args, false)
	if bad != nil {
		return nil, bad
	}
	owner := event.OwnerId == user.Id
	if utils.Coalesce(event.BlindAvailabilityEnabled) && !owner {
		return map[string]any{
			"event_id": event.Id.Hex(),
			"people": []map[string]any{{
				"name":   strings.TrimSpace(user.FirstName + " " + user.LastName),
				"status": "hidden",
			}},
			"totals": map[string]int{},
		}, nil
	}
	people, totals := roster(deps, event, owner)
	return map[string]any{"event_id": event.Id.Hex(), "people": people, "totals": totals}, nil
}

// rosterRow is one person on the owner's roster. Email is set for accounts and invites.
type rosterRow struct {
	email, name, status string
}

// rosterRows lists the invite list (group) or the people with a stored answer (other musters).
// Status is invited (no answer yet), responded, cleared, or declined.
func rosterRows(deps Deps, event *models.Event) []rosterRow {
	responses := deps.Events.Responses(event.Id)
	byUser := map[string]*models.Response{}
	for _, row := range responses {
		if row.Response != nil {
			byUser[row.UserId] = row.Response
		}
	}
	rows := []rosterRow{}
	seen := map[string]bool{}
	add := func(email, name, status string) {
		if seen[email+name+status] {
			return
		}
		seen[email+name+status] = true
		rows = append(rows, rosterRow{email, name, status})
	}
	for _, attendee := range deps.Events.Attendees(event.Id) {
		status := "invited"
		if attendee.Declined != nil && *attendee.Declined {
			status = "declined"
		}
		name := ""
		if deps.Users != nil {
			if account := deps.Users.GetByEmail(attendee.Email); account != nil {
				name = strings.TrimSpace(account.FirstName + " " + account.LastName)
				if response := byUser[account.Id.Hex()]; response != nil && status != "declined" {
					status = "cleared"
					if len(response.Availability)+len(response.IfNeeded) > 0 {
						status = "responded"
					}
				}
			}
		}
		add(attendee.Email, name, status)
	}
	if event.Type != models.GROUP {
		for _, response := range byUser {
			status := "cleared"
			if len(response.Availability)+len(response.IfNeeded) > 0 {
				status = "responded"
			}
			add("", displayName(deps, response), status)
		}
	}
	return rows
}

func roster(deps Deps, event *models.Event, owner bool) ([]map[string]any, map[string]int) {
	people := []map[string]any{}
	totals := map[string]int{}
	for _, r := range rosterRows(deps, event) {
		row := map[string]any{"name": r.name, "status": r.status}
		if owner && r.email != "" {
			row["email"] = r.email
		}
		people = append(people, row)
		totals[r.status]++
	}
	return people, totals
}

// responseTotals is the owner-only count. Pending is the roster members with no answer and no decline.
// The owner is on a group's invite list, so the owner counts as pending until they answer.
func responseTotals(deps Deps, owner *models.User, event *models.Event) *ResponseTotals {
	rows := rosterRows(deps, event)
	out := &ResponseTotals{Population: "people with a stored answer; the owner counts only after answering"}
	if event.Type == models.GROUP {
		out.Population = "group invite list, including the owner"
	}
	for _, r := range rows {
		out.Invited++
		switch r.status {
		case "responded":
			out.Responded++
		case "cleared":
			out.Cleared++
		case "declined":
			out.Declined++
		default:
			out.Pending++
		}
		if event.Type == models.GROUP && r.email != "" && canonicalOrLower(r.email) == canonicalOrLower(owner.Email) {
			out.OwnerIsParticipant = true
			out.OwnerStatus = r.status
		}
	}
	if event.Type != models.GROUP {
		if snap := callerSnapshot(deps, owner, event.Id); snap.Exists {
			out.OwnerIsParticipant = true
			out.OwnerStatus = "cleared"
			if len(snap.Availability)+len(snap.IfNeeded) > 0 {
				out.OwnerStatus = "responded"
			}
		}
	}
	return out
}

func findBestTimes(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	event, bad := ownedOrMember(deps, user, args, false)
	if bad != nil {
		return nil, bad
	}
	if utils.Coalesce(event.IsSignUpForm) {
		return nil, toolErr("SIGN_UP_FORM", "This muster is a sign-up form.", utils.GetBaseUrl()+"/e/"+event.GetId())
	}
	if utils.Coalesce(event.BlindAvailabilityEnabled) && event.OwnerId != user.Id {
		return nil, toolErr("OVERLAP_HIDDEN", "Hidden availability cannot be summarized.", "Ask the owner to read the muster.")
	}
	start, end, bad := optionalWindow(event, args)
	if bad != nil {
		return nil, bad
	}
	minutes := int(increment(event) / time.Minute)
	if raw, ok := args["duration_minutes"]; ok && raw != nil {
		n, ok := wholeNumber(raw)
		if !ok || n < 1 {
			return nil, toolErr("BAD_ARGUMENTS", "duration_minutes must be a positive integer.", "Pass the meeting length in minutes.")
		}
		minutes = n
	}
	if time.Duration(minutes)*time.Minute%increment(event) != 0 {
		return nil, toolErr("BAD_ARGUMENTS", "duration_minutes must be a positive multiple of slot_minutes.", "Read slot_minutes from muster_get_event.")
	}
	responses := deps.Events.Responses(event.Id)
	for i := range responses {
		if responses[i].Response == nil {
			continue
		}
		if strings.TrimSpace(personName(responses[i].Response)) == "" {
			responses[i].Response.Name = displayName(deps, responses[i].Response)
		}
	}
	slots := rankSlots(event, responses, start, end, time.Duration(minutes)*time.Minute)
	return map[string]any{
		"event_id":         event.Id.Hex(),
		"start":            start.UTC().Format(time.RFC3339),
		"end":              end.UTC().Format(time.RFC3339),
		"duration_minutes": minutes,
		"slots":            slots,
	}, nil
}

func finalizeEvent(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	event, bad := ownedOrMember(deps, user, args, true)
	if bad != nil {
		return nil, bad
	}
	if utils.Coalesce(event.IsArchived) {
		return nil, toolErr("EVENT_ARCHIVED", "This muster is already closed.", "The answer is still editable. Finalize does not record another time.")
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
	if !spanOnGrid(event, start, end) {
		return nil, toolErr("SLOT_OUTSIDE", "That time is not on this muster's grid.", "Read dates and slot_minutes from muster_get_event.")
	}
	err = deps.Events.Patch(event.Id, map[string]any{
		"isArchived": true,
		"scheduledEvent": models.CalendarEvent{
			Summary:   event.Name,
			StartDate: primitive.NewDateTimeFromTime(start.UTC()),
			EndDate:   primitive.NewDateTimeFromTime(end.UTC()),
		},
	})
	if err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	// The stored interval is read back, so the receipt proves it. null means the readback did not show it.
	var stored *ScheduledSpan
	if fresh := deps.Events.GetByEitherID(event.Id.Hex()); fresh != nil {
		stored = snapshotEvent(fresh, false, false).Scheduled
	}
	return map[string]any{
		"event_id":   event.Id.Hex(),
		"start":      start.UTC().Format(time.RFC3339),
		"end":        end.UTC().Format(time.RFC3339),
		"closed":     true,
		"email_sent": false,
		"scheduled":  stored,
	}, nil
}

func updateEvent(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	event, bad := ownedOrMember(deps, user, args, true)
	if bad != nil {
		return nil, bad
	}
	set := map[string]any{}
	if raw, ok := args["name"]; ok && raw != nil {
		name, ok := raw.(string)
		name = strings.TrimSpace(name)
		if !ok || name == "" || utf8.RuneCountInString(name) > 80 {
			return nil, toolErr("BAD_ARGUMENTS", "name must be 1 to 80 characters.", "Pass a shorter name.")
		}
		set["name"] = name
	}
	if raw, ok := args["dates"]; ok && raw != nil {
		dates, bad := markList(event, event.Type, args, "dates")
		if bad != nil {
			return nil, bad
		}
		if event.Type == models.DOW {
			for _, date := range dates {
				if !onReferenceWeek(date.Time()) {
					return nil, toolErr("BAD_ARGUMENTS", "A weekly muster uses the reference week 2018-06-17 through 2018-06-24.", "Build dates on that week.")
				}
			}
		}
		set["dates"] = dates
	}
	if _, ok := args["duration_hours"]; ok {
		hours, bad := requiredHours(args, "duration_hours")
		if bad != nil {
			return nil, bad
		}
		set["duration"] = hours
	}
	if raw, ok := args["archive"]; ok && raw != nil {
		value, ok := raw.(bool)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", "archive must be true or false.", "Pass a boolean.")
		}
		set["isArchived"] = value
	}
	if raw, ok := args["blind"]; ok && raw != nil {
		value, ok := raw.(bool)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", "blind must be true or false.", "Pass a boolean.")
		}
		set["blindAvailabilityEnabled"] = value
	}
	if len(set) == 0 {
		return nil, toolErr("BAD_ARGUMENTS", "Nothing to change.", "Pass name, dates, duration_hours, archive, or blind.")
	}
	if err := deps.Events.Patch(event.Id, set); err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	// The echo is a read after the write, not the submitted input.
	fresh := deps.Events.GetByEitherID(event.Id.Hex())
	if fresh == nil {
		fresh = event
	}
	return map[string]any{"event_id": event.Id.Hex(), "updated": true, "event": eventDetail(deps, user, fresh)}, nil
}

func remindAttendees(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	event, bad := ownedOrMember(deps, user, args, true)
	if bad != nil {
		return nil, bad
	}
	if event.Type != models.GROUP {
		return nil, toolErr("BAD_ARGUMENTS", "Only a group muster has an invite list.", "Open the muster page for a regular muster.")
	}
	answered := map[string]bool{}
	for _, row := range deps.Events.Responses(event.Id) {
		if row.Response != nil && len(row.Response.Availability)+len(row.Response.IfNeeded) > 0 {
			answered[row.UserId] = true
		}
	}
	pending := []string{}
	for _, attendee := range deps.Events.Attendees(event.Id) {
		if attendee.Declined != nil && *attendee.Declined {
			continue
		}
		if deps.Users != nil {
			if account := deps.Users.GetByEmail(attendee.Email); account != nil && answered[account.Id.Hex()] {
				continue
			}
		}
		pending = append(pending, attendee.Email)
	}
	return map[string]any{
		"event_id":   event.Id.Hex(),
		"pending":    pending,
		"email_sent": false,
	}, nil
}

func removeAttendee(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	event, bad := ownedOrMember(deps, user, args, true)
	if bad != nil {
		return nil, bad
	}
	if event.Type != models.GROUP {
		return nil, toolErr("BAD_ARGUMENTS", "Only a group muster has an invite list.", "Open the muster page for a regular muster.")
	}
	email, bad := requiredString(args, "email")
	if bad != nil {
		return nil, bad
	}
	email, ok := canonicalEmail(email)
	if !ok {
		return nil, toolErr("BAD_ARGUMENTS", "One address is not an email.", "Fix the address and call again.")
	}
	if email == canonicalOrLower(user.Email) {
		return nil, toolErr("BAD_ARGUMENTS", "The owner stays on the muster.", "Remove someone else.")
	}
	responseUser := ""
	if deps.Users != nil {
		if account := deps.Users.GetByEmail(email); account != nil && account.Id != user.Id {
			responseUser = account.Id.Hex()
		}
	}
	if err := deps.Events.RemoveAttendee(event.Id, email, responseUser); err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	return map[string]any{"event_id": event.Id.Hex(), "removed": email}, nil
}

func listChanges(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	event, bad := ownedOrMember(deps, user, args, true)
	if bad != nil {
		return nil, bad
	}
	sinceRaw, bad := requiredString(args, "since")
	if bad != nil {
		return nil, bad
	}
	since, err := time.Parse(time.RFC3339, sinceRaw)
	if err != nil {
		return nil, toolErr("BAD_ARGUMENTS", "since must be RFC3339.", "Pass the timestamp of the last check.")
	}
	responses, attendees, err := deps.Events.Changes(event.Id, since)
	if err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	answers := []map[string]any{}
	for _, row := range responses {
		name := ""
		if row.Response != nil {
			name = displayName(deps, row.Response)
		}
		answers = append(answers, map[string]any{
			"name":       name,
			"updated_at": row.UpdatedAt.Time().UTC().Format(time.RFC3339),
		})
	}
	invites := []map[string]any{}
	for _, attendee := range attendees {
		invites = append(invites, map[string]any{
			"email":      attendee.Email,
			"declined":   attendee.Declined != nil && *attendee.Declined,
			"updated_at": attendee.UpdatedAt.Time().UTC().Format(time.RFC3339),
		})
	}
	return map[string]any{"event_id": event.Id.Hex(), "answers": answers, "invites": invites}, nil
}

func ownedOrMember(deps Deps, user *models.User, args map[string]any, ownerOnly bool) (*models.Event, *ToolError) {
	id, bad := requiredString(args, "event_id")
	if bad != nil {
		return nil, bad
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	if ownerOnly && event.OwnerId != user.Id {
		return nil, toolErr("NOT_OWNER", "Only the owner can change this muster.", "Ask the owner.")
	}
	return event, nil
}

func optionalWindow(event *models.Event, args map[string]any) (time.Time, time.Time, *ToolError) {
	hasStart, hasEnd := presentList(args, "start"), presentList(args, "end")
	if !hasStart && !hasEnd {
		start, end, ok := eventWindow(event)
		if !ok {
			return time.Time{}, time.Time{}, toolErr("BAD_ARGUMENTS", "start and end are required when the muster has no dates.", "Pass an RFC3339 window of at most 7 days.")
		}
		return start, end, nil
	}
	return window(args)
}

func eventWindow(event *models.Event) (time.Time, time.Time, bool) {
	if event == nil || len(event.Dates) == 0 {
		return time.Time{}, time.Time{}, false
	}
	min := event.Dates[0].Time()
	max := min
	for _, date := range event.Dates {
		t := date.Time()
		if t.Before(min) {
			min = t
		}
		if t.After(max) {
			max = t
		}
	}
	end := max.Add(slotSpan(event, increment(event)))
	if end.Sub(min) > 7*24*time.Hour {
		end = min.Add(7 * 24 * time.Hour)
	}
	if !end.After(min) {
		end = min.Add(time.Hour)
	}
	return min, end, true
}

func spanOnGrid(event *models.Event, start, end time.Time) bool {
	if !end.After(start) {
		return false
	}
	inc := increment(event)
	if end.Sub(start)%inc != 0 {
		return false
	}
	for t := start; t.Before(end); t = t.Add(inc) {
		if slotProblem(event, t) != "" {
			return false
		}
	}
	return true
}

func onReferenceWeek(t time.Time) bool {
	t = t.UTC()
	start := time.Date(2018, 6, 17, 0, 0, 0, 0, time.UTC)
	end := time.Date(2018, 6, 25, 0, 0, 0, 0, time.UTC)
	return !t.Before(start) && t.Before(end)
}

func requiredHours(args map[string]any, key string) (float32, *ToolError) {
	raw, ok := args[key]
	n, isFloat := raw.(float64)
	if !ok || !isFloat || n <= 0 || n > 24 {
		return 0, toolErr("BAD_ARGUMENTS", key+" must be a number from 0 to 24, excluding 0.", "Pass the length of each day in hours.")
	}
	return float32(n), nil
}

func requiredIncrement(args map[string]any) (int, *ToolError) {
	raw, ok := args["slot_minutes"]
	if !ok || raw == nil {
		return 15, nil
	}
	n, ok := wholeNumber(raw)
	if !ok || (n != 15 && n != 30 && n != 60) {
		return 0, toolErr("BAD_ARGUMENTS", "slot_minutes must be 15, 30, or 60.", "Use one of those.")
	}
	return n, nil
}

func boolPtr(args map[string]any, key string) *bool {
	raw, ok := args[key].(bool)
	if !ok {
		return nil
	}
	return &raw
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
