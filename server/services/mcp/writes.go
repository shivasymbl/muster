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
	_, hasNewRanges := args["ranges"]
	if hasNewRanges {
		return setRanges(deps, user, event, args)
	}
	if mode, ok := args["mode"].(string); ok && mode == "merge" {
		return nil, toolErr("BAD_ARGUMENTS", "mode merge needs ranges.", "Pass ranges, or use replace with full lists.")
	}
	if mode, ok := args["mode"]; ok && mode != nil && mode != "replace" && mode != "merge" {
		return nil, toolErr("BAD_ARGUMENTS", "mode must be replace or merge.", "Pass mode replace or merge.")
	}
	hasRanges := args["availability_ranges"] != nil
	hasAvailability := presentList(args, "availability") || hasRanges
	hasIfNeeded := presentList(args, "if_needed")
	if !hasAvailability && !hasIfNeeded {
		return nil, toolErr("BAD_ARGUMENTS", "Pass availability or if_needed.", "An empty list clears that list. Omitting both leaves the answer unchanged.")
	}
	if hasRanges && presentList(args, "availability") {
		return nil, toolErr("BAD_ARGUMENTS", "Pass availability or availability_ranges, not both.", "Use timestamps, or ranges, for the available times.")
	}
	var availability []primitive.DateTime
	if hasRanges {
		availability, bad = expandRanges(event, args["availability_ranges"])
	} else {
		availability, bad = markList(event, event.Type, args, "availability")
	}
	if bad != nil {
		return nil, bad
	}
	ifNeeded, bad := markList(event, event.Type, args, "if_needed")
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
	snap := callerSnapshot(deps, user, event.Id)
	if bad := guardMatch(args, snap); bad != nil {
		return nil, bad
	}
	changed := !sameInstants(snap.Availability, availability) || !sameInstants(snap.IfNeeded, ifNeeded)
	if hasRanges {
		// Range writes are conditional on the answer read above.
		if bad := conditionalWrite(deps, user, event.Id, snap, availability, ifNeeded); bad != nil {
			return nil, bad
		}
	} else if err := deps.Events.ReplaceAvailability(user, event.Id, availability, ifNeeded); err != nil {
		// Legacy arrays stay an unconditional whole-answer replace (owner decision).
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	out := map[string]any{
		"event_id":           event.Id.Hex(),
		"availability":       len(availability),
		"if_needed":          len(ifNeeded),
		"duplicates_ignored": droppedA + droppedI,
		"replaced":           true,
		"changed":            changed,
	}
	addEcho(out, deps, user, event, availability, ifNeeded)
	return out, nil
}

// guardMatch refuses a write when if_match or expected_state_fingerprint does not
// match the answer just read. A string if_match is the state_fingerprint. An
// object if_match is the current availability and if_needed lists. A wrong type
// is an error, not a silent skip.
func guardMatch(args map[string]any, snap AnswerSnapshot) *ToolError {
	if raw, present := args["if_match"]; present && raw != nil {
		switch match := raw.(type) {
		case map[string]any:
			same, bad := answerMatches(snapResponse(snap), match)
			if bad != nil {
				return bad
			}
			if !same {
				return toolErr("CONFLICT", "The stored answer changed.", "Read the current answer and pass it as if_match, or pass its state_fingerprint.")
			}
		case string:
			if match == "" || stateFingerprint(snap.Availability, snap.IfNeeded) != match {
				return toolErr("CONFLICT", "The stored answer changed.", "Read the current answer and pass its state_fingerprint as if_match or expected_state_fingerprint.")
			}
		default:
			return toolErr("BAD_ARGUMENTS", "if_match must be a state_fingerprint or the current lists.", "Pass the state_fingerprint string, or {availability, if_needed}.")
		}
	}
	if want, ok := args["expected_state_fingerprint"].(string); ok && want != "" {
		if stateFingerprint(snap.Availability, snap.IfNeeded) != want {
			return toolErr("CONFLICT", "The stored answer changed.", "Read the current answer, then retry with its state_fingerprint.")
		}
	} else if raw, present := args["expected_state_fingerprint"]; present && raw != nil {
		return toolErr("BAD_ARGUMENTS", "expected_state_fingerprint must be a non-empty string.", "Pass the state_fingerprint from a previous receipt.")
	}
	return nil
}

// setRanges handles the ranges/mode arguments. It never accepts the legacy list keys.
func setRanges(deps Deps, user *models.User, event *models.Event, args map[string]any) (any, *ToolError) {
	if _, ok := args["availability"]; ok {
		return nil, toolErr("BAD_ARGUMENTS", "Pass ranges or availability, not both.", "Use ranges alone, or the legacy lists alone.")
	}
	if _, ok := args["if_needed"]; ok {
		return nil, toolErr("BAD_ARGUMENTS", "Pass ranges or if_needed, not both.", "Use ranges alone, or the legacy lists alone.")
	}
	if _, ok := args["availability_ranges"]; ok {
		return nil, toolErr("BAD_ARGUMENTS", "Pass ranges or availability_ranges, not both.", "Use ranges alone.")
	}
	merge := false
	if raw, ok := args["mode"]; ok && raw != nil {
		switch raw {
		case "replace":
		case "merge":
			merge = true
		default:
			return nil, toolErr("BAD_ARGUMENTS", "mode must be replace or merge.", "Pass mode replace or merge.")
		}
	}
	changes, bad := parseRanges(args["ranges"])
	if bad != nil {
		return nil, bad
	}
	snap := callerSnapshot(deps, user, event.Id)
	if bad := guardMatch(args, snap); bad != nil {
		return nil, bad
	}
	out := map[string]any{"event_id": event.Id.Hex(), "mode": map[bool]string{true: "merge", false: "replace"}[merge]}
	if len(changes) == 0 {
		// Nothing to apply: merge is a no-op; replace clears the stored answer.
		if merge {
			out["changed"] = false
			addEcho(out, deps, user, event, snap.Availability, snap.IfNeeded)
			return out, nil
		}
		return finishRanges(deps, user, event, snap, nil, nil, out, nil)
	}
	requested, requestedIfNeeded, bad := applyRanges(event, changes, false, nil)
	if bad != nil {
		return nil, bad
	}
	if bad := slotsOnGrid(event, requested, requestedIfNeeded); bad != nil {
		return nil, bad
	}
	avail, ifNeeded, bad := applyRanges(event, changes, merge, &snap)
	if bad != nil {
		return nil, bad
	}
	return finishRanges(deps, user, event, snap, avail, ifNeeded, out, changes)
}

func finishRanges(deps Deps, user *models.User, event *models.Event, snap AnswerSnapshot, avail, ifNeeded []primitive.DateTime, out map[string]any, changes []rangeChange) (any, *ToolError) {
	if avail == nil {
		avail = []primitive.DateTime{}
	}
	if ifNeeded == nil {
		ifNeeded = []primitive.DateTime{}
	}
	if changes != nil {
		start, end := affectedWindow(changes)
		out["window"] = map[string]string{"start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339)}
	}
	same := sameInstants(snap.Availability, avail) && sameInstants(snap.IfNeeded, ifNeeded)
	out["availability"] = len(avail)
	out["if_needed"] = len(ifNeeded)
	if same {
		// Repeating the same intent writes nothing.
		out["changed"] = false
		out["replaced"] = false
		addEcho(out, deps, user, event, avail, ifNeeded)
		return out, nil
	}
	if bad := conditionalWrite(deps, user, event.Id, snap, avail, ifNeeded); bad != nil {
		return nil, bad
	}
	out["changed"] = true
	out["replaced"] = true
	addEcho(out, deps, user, event, avail, ifNeeded)
	return out, nil
}

func conditionalWrite(deps Deps, user *models.User, eventID primitive.ObjectID, snap AnswerSnapshot, avail, ifNeeded []primitive.DateTime) *ToolError {
	ok, err := deps.Events.ReplaceAvailabilityIf(user, eventID, snap, avail, ifNeeded)
	if err != nil {
		return toolErr("INTERNAL", "The write may not have been saved.", "Read the current answer before retrying.")
	}
	if !ok {
		return toolErr("CONFLICT", "The stored answer changed.", "Read the current answer and send the ranges again.")
	}
	return nil
}

// addEcho reads the stored answer back and adds it, so the caller needs no second call.
// echo_complete is false when the readback is missing or differs from what was written.
func addEcho(out map[string]any, deps Deps, user *models.User, event *models.Event, wantAvail, wantNeed []primitive.DateTime) {
	stored := callerSnapshot(deps, user, event.Id)
	out["answer"] = map[string]any{
		"availability": slotViews(event, stored.Availability),
		"if_needed":    slotViews(event, stored.IfNeeded),
	}
	out["state_fingerprint"] = stateFingerprint(stored.Availability, stored.IfNeeded)
	complete := stored.Exists && sameInstants(stored.Availability, wantAvail) && sameInstants(stored.IfNeeded, wantNeed)
	if !stored.Exists && len(wantAvail) == 0 && len(wantNeed) == 0 {
		complete = true
	}
	out["echo_complete"] = complete
}

func callerSnapshot(deps Deps, user *models.User, eventID primitive.ObjectID) AnswerSnapshot {
	for _, row := range deps.Events.Responses(eventID) {
		if row.UserId == user.Id.Hex() && row.Response != nil {
			return AnswerSnapshot{Exists: true, UpdatedAt: row.UpdatedAt, Availability: row.Response.Availability, IfNeeded: row.Response.IfNeeded}
		}
	}
	return AnswerSnapshot{}
}

func snapResponse(snap AnswerSnapshot) *models.Response {
	if !snap.Exists {
		return nil
	}
	return &models.Response{Availability: snap.Availability, IfNeeded: snap.IfNeeded}
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
	skipped := make([]string, 0)
	seen := map[string]bool{}
	skippedSeen := map[string]bool{}
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
			if !skippedSeen[address] {
				skippedSeen[address] = true
				skipped = append(skipped, address)
			}
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
		"skipped":    skipped,
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

func presentList(args map[string]any, key string) bool {
	raw, ok := args[key]
	return ok && raw != nil
}

func callerAnswer(deps Deps, user *models.User, eventID primitive.ObjectID) *models.Response {
	for _, row := range deps.Events.Responses(eventID) {
		if row.UserId == user.Id.Hex() && row.Response != nil {
			return row.Response
		}
	}
	return nil
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
