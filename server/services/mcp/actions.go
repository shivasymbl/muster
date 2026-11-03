package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const actionTTL = 15 * time.Minute
const maxActionBytes = 12288

// WritesEnabled is the App write gate. Text tools do not read it.
func WritesEnabled() bool {
	return os.Getenv("MCP_APPS_WRITES_ENABLED") == "true"
}

func callAction(deps Deps, user *models.User, name string, args map[string]any) (any, *ToolError) {
	switch name {
	case "muster_ui_prepare_action":
		return prepareAction(deps, user, args)
	case "muster_ui_commit_action":
		return commitAction(deps, user, args)
	case "muster_ui_get_action":
		return getAction(deps, user, args)
	default:
		return nil, toolErr("BAD_ARGUMENTS", "Unknown tool.", "Call tools/list.")
	}
}

func prepareAction(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	if deps.Actions == nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	raw, err := json.Marshal(args)
	if err != nil || len(raw) > maxActionBytes {
		return nil, toolErr("BAD_ARGUMENTS", "The action is too large.", "Send a smaller change.")
	}
	op, _ := args["operation"].(string)
	payload, _ := args["payload"].(map[string]any)
	if payload == nil {
		return nil, toolErr("BAD_ARGUMENTS", "payload is required.", "Pass the operation payload.")
	}
	eventID, _ := args["event_id"].(string)
	fingerprint, _ := args["expected_state_fingerprint"].(string)
	if op == "create_group" {
		if eventID != "" || fingerprint != "" {
			return nil, toolErr("BAD_ARGUMENTS", "create_group does not take an event or a fingerprint.", "Omit event_id and expected_state_fingerprint.")
		}
	} else if eventID == "" || fingerprint == "" {
		return nil, toolErr("BAD_ARGUMENTS", "event_id and expected_state_fingerprint are required.", "Open the view again and use the fingerprint it returns.")
	}
	preview := map[string]any{"operation": op}
	var event *models.Event
	if op != "create_group" {
		event = deps.Events.GetByEitherID(eventID)
		if !member(deps, user, event) {
			return nil, notFound()
		}
		current := StateFingerprint(deps, user, event)
		if current != fingerprint {
			return nil, toolErr("CONFLICT", "The muster changed.", "Open the view again and review the new state.")
		}
		preview["event_id"] = event.Id.Hex()
	}
	switch op {
	case "edit_own_answer":
		changes, _ := payload["changes"].([]any)
		if len(changes) == 0 {
			preview["changed"] = false
			preview["effect"] = "No slots change. This does not clear the answer."
		} else {
			preview["changed"] = true
			preview["effect"] = "Replace the listed slots on the stored answer. Marks outside the changes stay."
			preview["ranges"] = len(changes)
		}
	case "clear_own_answer":
		preview["changed"] = true
		preview["effect"] = "Clear availability and if-needed. Manual calendar marks stay."
	case "decline_own_invitation":
		preview["changed"] = true
		preview["effect"] = "Decline the invitation. The stored answer is kept."
	case "finalize_time":
		preview["changed"] = true
		preview["effect"] = "Record this time and archive the muster. No calendar event is created. No email is sent."
		preview["start"] = payload["start"]
		preview["end"] = payload["end"]
	case "create_group":
		preview["changed"] = true
		preview["effect"] = "Create a group muster. Invite mail is sent only when mail is configured, and delivery is not verified."
		preview["name"] = payload["name"]
	case "update_group":
		preview["changed"] = true
		preview["effect"] = "Update the group fields in this payload. The owner and the type stay the same."
	case "add_attendees":
		preview["changed"] = true
		preview["effect"] = "Invite these addresses. People already invited are skipped. Delivery is not verified."
	case "remove_attendee":
		preview["changed"] = true
		preview["effect"] = "Remove this person and their stored answer. The owner cannot be removed."
		preview["email"] = payload["email"]
	default:
		return nil, toolErr("BAD_ARGUMENTS", "Unknown operation.", "Use one of the eight review operations.")
	}
	rec := &ActionRecord{
		ID:          primitive.NewObjectID(),
		UserID:      user.Id,
		Operation:   op,
		EventID:     eventID,
		Fingerprint: fingerprint,
		Payload:     payload,
		Preview:     preview,
		Status:      "prepared",
		ExpiresAt:   deps.now().Add(actionTTL),
	}
	if event != nil {
		rec.EventID = event.Id.Hex()
	}
	if err := deps.Actions.Create(rec); err != nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	return map[string]any{
		"action_id":  rec.ID.Hex(),
		"status":     rec.Status,
		"expires_at": rec.ExpiresAt.UTC().Format(time.RFC3339),
		"preview":    preview,
	}, nil
}

func commitAction(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	if deps.Actions == nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	idText, _ := args["action_id"].(string)
	id, err := primitive.ObjectIDFromHex(idText)
	if err != nil {
		return nil, toolErr("BAD_ARGUMENTS", "action_id is not valid.", "Use the action_id from prepare.")
	}
	now := deps.now()
	claimed, err := deps.Actions.Claim(user.Id, id, now)
	if err != nil {
		existing, getErr := deps.Actions.Get(user.Id, id)
		if getErr != nil {
			return nil, toolErr("BAD_ARGUMENTS", "No prepared action found.", "Prepare the action again.")
		}
		if existing.Status == "prepared" && !existing.ExpiresAt.After(now) {
			return nil, toolErr("EXPIRED", "That review expired.", "Prepare the action again.")
		}
		return storedReceipt(existing), nil
	}
	if !claimed.ExpiresAt.After(now) {
		_ = deps.Actions.Finish(user.Id, id, "expired", nil, "EXPIRED", "That review expired.")
		return nil, toolErr("EXPIRED", "That review expired.", "Prepare the action again.")
	}
	result, bad := runPrepared(deps, user, claimed)
	if bad != nil {
		_ = deps.Actions.Finish(user.Id, id, "failed", nil, bad.Code, bad.Message)
		return nil, bad
	}
	if err := deps.Actions.Finish(user.Id, id, "completed", result, "", ""); err != nil {
		return map[string]any{
			"action_id": id.Hex(),
			"status":    "outcome_unknown",
			"result":    result,
		}, toolErr("OUTCOME_UNKNOWN", "The write may have landed but the receipt was not saved.", "Call muster_ui_get_action with this action_id. Do not prepare it again.")
	}
	result["action_id"] = id.Hex()
	result["status"] = "completed"
	return result, nil
}

func getAction(deps Deps, user *models.User, args map[string]any) (any, *ToolError) {
	if deps.Actions == nil {
		return nil, toolErr("INTERNAL", "Try again.", "Wait and retry.")
	}
	idText, _ := args["action_id"].(string)
	id, err := primitive.ObjectIDFromHex(idText)
	if err != nil {
		return nil, toolErr("BAD_ARGUMENTS", "action_id is not valid.", "Use the action_id from prepare.")
	}
	rec, err := deps.Actions.Get(user.Id, id)
	if err != nil {
		return nil, toolErr("BAD_ARGUMENTS", "No prepared action found.", "Prepare the action again.")
	}
	return storedReceipt(rec), nil
}

func storedReceipt(rec *ActionRecord) map[string]any {
	out := map[string]any{
		"action_id": rec.ID.Hex(),
		"status":    rec.Status,
		"operation": rec.Operation,
		"preview":   rec.Preview,
	}
	if rec.Result != nil {
		out["result"] = rec.Result
	}
	if rec.ErrCode != "" {
		out["code"] = rec.ErrCode
		out["message"] = rec.ErrMessage
	}
	return out
}

func runPrepared(deps Deps, user *models.User, rec *ActionRecord) (map[string]any, *ToolError) {
	eventID := rec.EventID
	switch rec.Operation {
	case "edit_own_answer":
		changes, _ := rec.Payload["changes"].([]any)
		if len(changes) == 0 {
			return map[string]any{"changed": false, "event_id": eventID}, nil
		}
		event := deps.Events.GetByEitherID(eventID)
		if !member(deps, user, event) {
			return nil, notFound()
		}
		if StateFingerprint(deps, user, event) != rec.Fingerprint {
			return nil, toolErr("CONFLICT", "The muster changed.", "Prepare the edit again.")
		}
		current := callerAnswer(deps, user, event.Id)
		avail, need := []primitive.DateTime{}, []primitive.DateTime{}
		if current != nil {
			avail, need = current.Availability, current.IfNeeded
		}
		avail, need, bad := applyDraftChanges(event, avail, need, changes)
		if bad != nil {
			return nil, bad
		}
		if err := deps.Events.ReplaceAvailability(user, event.Id, avail, need); err != nil {
			return nil, toolErr("OUTCOME_UNKNOWN", "The save did not confirm.", "Check the muster before trying again.")
		}
		return map[string]any{"changed": true, "event_id": event.Id.Hex(), "availability": len(avail), "if_needed": len(need)}, nil
	case "clear_own_answer":
		_, bad := setAvailability(deps, user, map[string]any{"event_id": eventID, "availability": []any{}, "if_needed": []any{}})
		if bad != nil {
			return nil, bad
		}
		return map[string]any{"changed": true, "event_id": eventID, "cleared": true}, nil
	case "decline_own_invitation":
		_, bad := decline(deps, user, map[string]any{"event_id": eventID})
		if bad != nil {
			return nil, bad
		}
		return map[string]any{"changed": true, "event_id": eventID, "declined": true}, nil
	case "finalize_time":
		_, bad := finalizeEvent(deps, user, map[string]any{"event_id": eventID, "start": rec.Payload["start"], "end": rec.Payload["end"]})
		if bad != nil {
			return nil, bad
		}
		return map[string]any{"changed": true, "event_id": eventID, "closed": true, "email_sent": false, "start": rec.Payload["start"], "end": rec.Payload["end"]}, nil
	case "create_group":
		args := map[string]any{"type": "group"}
		for k, v := range rec.Payload {
			args[k] = v
		}
		created, bad := createEvent(deps, user, args)
		if bad != nil {
			return nil, bad
		}
		out, _ := created.(map[string]any)
		if out == nil {
			out = map[string]any{}
		}
		out["changed"] = true
		return out, nil
	case "update_group":
		args := map[string]any{"event_id": eventID}
		for k, v := range rec.Payload {
			args[k] = v
		}
		_, bad := updateEvent(deps, user, args)
		if bad != nil {
			return nil, bad
		}
		return map[string]any{"changed": true, "event_id": eventID}, nil
	case "add_attendees":
		_, bad := addAttendees(deps, user, map[string]any{"event_id": eventID, "emails": rec.Payload["emails"]})
		if bad != nil {
			return nil, bad
		}
		return map[string]any{"changed": true, "event_id": eventID}, nil
	case "remove_attendee":
		_, bad := removeAttendee(deps, user, map[string]any{"event_id": eventID, "email": rec.Payload["email"]})
		if bad != nil {
			return nil, bad
		}
		return map[string]any{"changed": true, "event_id": eventID}, nil
	default:
		return nil, toolErr("BAD_ARGUMENTS", "Unknown operation.", "Prepare a known operation.")
	}
}

func applyDraftChanges(event *models.Event, avail, need []primitive.DateTime, changes []any) ([]primitive.DateTime, []primitive.DateTime, *ToolError) {
	if len(changes) > 100 {
		return nil, nil, toolErr("BAD_ARGUMENTS", "At most 100 ranges.", "Split the edit.")
	}
	have := map[int64]string{}
	for _, t := range avail {
		have[t.Time().UTC().UnixMilli()] = "available"
	}
	for _, t := range need {
		have[t.Time().UTC().UnixMilli()] = "if_needed"
	}
	inc := increment(event)
	for _, item := range changes {
		obj, _ := item.(map[string]any)
		startText, _ := obj["start"].(string)
		endText, _ := obj["end"].(string)
		state, _ := obj["state"].(string)
		start, err := time.Parse(time.RFC3339, startText)
		if err != nil {
			return nil, nil, toolErr("BAD_ARGUMENTS", "change start must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
		}
		end, err := time.Parse(time.RFC3339, endText)
		if err != nil {
			return nil, nil, toolErr("BAD_ARGUMENTS", "change end must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
		}
		if !end.After(start) {
			return nil, nil, toolErr("BAD_ARGUMENTS", "change end must be after start.", "Pass an end after the start.")
		}
		if state != "available" && state != "if_needed" && state != "unavailable" {
			return nil, nil, toolErr("BAD_ARGUMENTS", "state must be available, if_needed, or unavailable.", "Use one of those three.")
		}
		for t := start.UTC(); t.Before(end.UTC()); t = t.Add(inc) {
			if slotProblem(event, t) != "" {
				return nil, nil, toolErr("SLOT_OUTSIDE", "A changed time is not on this muster's grid.", "Use the slots from the open view.")
			}
			key := t.UnixMilli()
			if state == "unavailable" {
				delete(have, key)
				continue
			}
			have[key] = state
		}
	}
	keys := make([]int64, 0, len(have))
	for key := range have {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	avail, need = nil, nil
	for _, key := range keys {
		stamp := primitive.NewDateTimeFromTime(time.UnixMilli(key).UTC())
		if have[key] == "available" {
			avail = append(avail, stamp)
		} else {
			need = append(need, stamp)
		}
	}
	if avail == nil {
		avail = []primitive.DateTime{}
	}
	if need == nil {
		need = []primitive.DateTime{}
	}
	return avail, need, nil
}

// StateFingerprint hashes the event archive, scheduled interval, and the caller's marks.
// It is a comparison value, not an authorization token.
func StateFingerprint(deps Deps, user *models.User, event *models.Event) string {
	answer := callerAnswer(deps, user, event.Id)
	body := map[string]any{
		"event_id":     event.Id.Hex(),
		"archived":     event.IsArchived != nil && *event.IsArchived,
		"availability": stamps(nil),
		"if_needed":    stamps(nil),
	}
	if event.ScheduledEvent != nil {
		body["scheduled_start"] = event.ScheduledEvent.StartDate.Time().UTC().Format(time.RFC3339)
		body["scheduled_end"] = event.ScheduledEvent.EndDate.Time().UTC().Format(time.RFC3339)
	}
	if answer != nil {
		body["availability"] = stamps(answer.Availability)
		body["if_needed"] = stamps(answer.IfNeeded)
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func stamps(list []primitive.DateTime) []string {
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, t.Time().UTC().Format(time.RFC3339))
	}
	sort.Strings(out)
	return out
}

// ActionMiss is the claim miss sentinel stores should return.
func ActionMiss(err error) bool {
	return err == mongo.ErrNoDocuments
}
