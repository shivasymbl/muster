package mcp

import (
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/listmonk"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const maxMarks = 2000
const maxInvitees = 20

func setAvailability(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	id, bad := requiredString(args, "event_id")
	if bad != nil {
		return nil, bad
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	if utils.Coalesce(event.IsSignUpForm) {
		return nil, toolErr("SIGN_UP_FORM", "This muster is a sign-up form.", utils.GetBaseUrl()+"/e/"+event.GetId())
	}
	availability, bad := timeList(args, "availability")
	if bad != nil {
		return nil, bad
	}
	ifNeeded, bad := timeList(args, "if_needed")
	if bad != nil {
		return nil, bad
	}
	availability, droppedA := dedupeTimes(availability)
	ifNeeded, droppedI := dedupeTimes(ifNeeded)
	if timesOverlap(availability, ifNeeded) {
		return nil, toolErr("SLOT_IN_BOTH", "A time cannot be both available and if-needed.", "Put each timestamp in only one list.")
	}
	if bad := slotsOnGrid(event, availability, ifNeeded); bad != nil {
		return nil, bad
	}
	if err := deps.Events.ReplaceAvailability(user, event.Id, availability, ifNeeded); err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	return map[string]any{
		"event_id":           event.Id.Hex(),
		"availability":       len(availability),
		"if_needed":          len(ifNeeded),
		"duplicates_ignored": droppedA + droppedI,
		"replaced":           true,
	}, nil
}

func decline(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	id, bad := requiredString(args, "event_id")
	if bad != nil {
		return nil, bad
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	if event.Type != models.GROUP {
		return nil, toolErr("BAD_ARGUMENTS", "Only a group muster can be declined.", "Open the muster page to change a regular muster.")
	}
	isAttendee := false
	for _, attendee := range deps.Events.Attendees(event.Id) {
		if attendee.Email == user.Email {
			isAttendee = true
			break
		}
	}
	if !isAttendee {
		return nil, toolErr("NOT_AN_INVITEE", "This account is not on the invite list.", "Ask the owner to add your email.")
	}
	if err := deps.Events.Decline(user, event.Id); err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	return map[string]any{"event_id": event.Id.Hex(), "declined": true}, nil
}

func addAttendees(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	id, bad := requiredString(args, "event_id")
	if bad != nil {
		return nil, bad
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	if event.OwnerId != user.Id {
		return nil, toolErr("NOT_OWNER", "Only the owner can add people.", "Ask the owner to send the invite.")
	}
	if event.Type != models.GROUP {
		return nil, toolErr("BAD_ARGUMENTS", "Only a group muster has an invite list.", "Duplicate is not available. Create a group muster on the website.")
	}
	raw, ok := args["emails"]
	list, isList := raw.([]any)
	if !ok || !isList || len(list) == 0 {
		return nil, toolErr("BAD_ARGUMENTS", "emails is required.", "Pass emails as an array of addresses.")
	}
	if len(list) > maxInvitees {
		return nil, toolErr("BAD_ARGUMENTS", "At most 20 emails per call.", "Split the list and call again.")
	}
	emails := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		address, ok := item.(string)
		address = strings.TrimSpace(address)
		if !ok || address == "" {
			return nil, toolErr("BAD_ARGUMENTS", "Each email must be a string.", "Pass plain email addresses.")
		}
		address, ok = canonicalEmail(address)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", "One address is not an email.", "Fix the address and call again.")
		}
		if seen[address] || address == canonicalOrLower(user.Email) || alreadyInvited(deps, event.Id, address) {
			continue
		}
		seen[address] = true
		emails = append(emails, address)
	}
	added, err := deps.Events.AddAttendees(event.Id, emails)
	if err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	emailSent := os.Getenv("LISTMONK_URL") != "" && os.Getenv("LISTMONK_ENABLED") != "false"
	if emailSent {
		ownerName := strings.TrimSpace(user.FirstName + " " + user.LastName)
		if ownerName == "" {
			ownerName = "Somebody"
		}
		for _, address := range added {
			listmonk.SendEmailAddSubscriberIfNotExist(address, 9, bson.M{
				"ownerName": ownerName,
				"groupName": event.Name,
				"groupUrl":  utils.GetBaseUrl() + "/g/" + event.GetId(),
			}, false, listmonk.EmailOptions{ReplyTo: listmonk.ReplyToForUser(user)})
		}
	}
	return map[string]any{
		"event_id":   event.Id.Hex(),
		"added":      added,
		"email_sent": emailSent && len(added) > 0,
	}, nil
}

func summarizeOverlap(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	id, bad := requiredString(args, "event_id")
	if bad != nil {
		return nil, bad
	}
	start, end, bad := window(args)
	if bad != nil {
		return nil, bad
	}
	event := deps.Events.GetByEitherID(id)
	if !member(deps, user, event) {
		return nil, notFound()
	}
	if utils.Coalesce(event.IsSignUpForm) {
		return nil, toolErr("SIGN_UP_FORM", "This muster is a sign-up form.", utils.GetBaseUrl()+"/e/"+event.GetId())
	}
	if utils.Coalesce(event.BlindAvailabilityEnabled) && event.OwnerId != user.Id {
		return nil, toolErr("OVERLAP_HIDDEN", "Hidden availability cannot be summarized.", "Ask the owner to read the muster.")
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
	slots := overlapSlots(event, responses, start, end)
	return map[string]any{
		"event_id": event.Id.Hex(),
		"start":    start.UTC().Format(time.RFC3339),
		"end":      end.UTC().Format(time.RFC3339),
		"slots":    slots,
	}, nil
}

func canonicalEmail(raw string) (string, bool) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	address := strings.ToLower(strings.TrimSpace(parsed.Address))
	if address == "" || !strings.Contains(address, "@") {
		return "", false
	}
	return address, true
}

func canonicalOrLower(raw string) string {
	if address, ok := canonicalEmail(raw); ok {
		return address
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func alreadyInvited(deps Deps, eventID primitive.ObjectID, address string) bool {
	for _, attendee := range deps.Events.Attendees(eventID) {
		if canonicalOrLower(attendee.Email) == address {
			return true
		}
	}
	return false
}

func dedupeTimes(in []primitive.DateTime) ([]primitive.DateTime, int) {
	seen := map[int64]bool{}
	out := make([]primitive.DateTime, 0, len(in))
	dropped := 0
	for _, t := range in {
		key := t.Time().UTC().UnixMilli()
		if seen[key] {
			dropped++
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out, dropped
}

func timesOverlap(a, b []primitive.DateTime) bool {
	seen := map[int64]bool{}
	for _, t := range a {
		seen[t.Time().UTC().UnixMilli()] = true
	}
	for _, t := range b {
		if seen[t.Time().UTC().UnixMilli()] {
			return true
		}
	}
	return false
}

func slotsOnGrid(event *models.Event, lists ...[]primitive.DateTime) *ToolError {
	for _, list := range lists {
		for _, mark := range list {
			switch slotProblem(event, mark.Time()) {
			case "outside":
				return toolErr("SLOT_OUTSIDE", "slot outside this muster's dates", "Read dates and duration_hours from muster_get_event and pick a time inside one of those windows.")
			case "off":
				return toolErr("SLOT_OFF_GRID", "slot is off this muster's grid", "Use a start that matches slot_minutes from muster_get_event.")
			}
		}
	}
	return nil
}

func timeList(args map[string]any, key string) ([]primitive.DateTime, *ToolError) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return []primitive.DateTime{}, nil
	}
	list, isList := raw.([]any)
	if !isList {
		return nil, toolErr("BAD_ARGUMENTS", key+" must be an array of RFC3339 timestamps.", "Pass "+key+" as an array, or omit it to clear it.")
	}
	if len(list) > maxMarks {
		return nil, toolErr("BAD_ARGUMENTS", key+" is too long.", "Send at most 2000 timestamps.")
	}
	out := make([]primitive.DateTime, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", key+" must be RFC3339 strings.", "Use timestamps like 2026-05-01T15:00:00Z.")
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return nil, toolErr("BAD_ARGUMENTS", key+" must be RFC3339 strings.", "Use timestamps like 2026-05-01T15:00:00Z.")
		}
		out = append(out, primitive.NewDateTimeFromTime(parsed))
	}
	return out, nil
}

func window(args map[string]any) (time.Time, time.Time, *ToolError) {
	startRaw, bad := requiredString(args, "start")
	if bad != nil {
		return time.Time{}, time.Time{}, bad
	}
	endRaw, bad := requiredString(args, "end")
	if bad != nil {
		return time.Time{}, time.Time{}, bad
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, toolErr("BAD_ARGUMENTS", "start must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, toolErr("BAD_ARGUMENTS", "end must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, toolErr("BAD_ARGUMENTS", "end must be after start.", "Pass an end timestamp after start.")
	}
	if end.Sub(start) > 7*24*time.Hour {
		return time.Time{}, time.Time{}, toolErr("WINDOW_TOO_WIDE", "The window must stay within 7 days.", "Narrow start and end. The window must stay within 7 days.")
	}
	return start, end, nil
}
