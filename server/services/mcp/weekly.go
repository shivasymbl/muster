package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// referenceDay is the June 2018 date used when creating a weekly time.
// Sunday is the 24th. A stored Sunday on the 17th is still accepted when answering.
var referenceDay = map[string]int{
	"monday": 18, "tuesday": 19, "wednesday": 20, "thursday": 21,
	"friday": 22, "saturday": 23, "sunday": 24,
}

func markList(event *models.Event, eventType models.EventType, args map[string]any, key string) ([]primitive.DateTime, *ToolError) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return []primitive.DateTime{}, nil
	}
	list, isList := raw.([]any)
	if !isList {
		return nil, toolErr("BAD_ARGUMENTS", key+" must be an array.", "Pass RFC3339 timestamps, or {weekday, time} on a weekly muster.")
	}
	if len(list) > maxMarks {
		return nil, toolErr("BAD_ARGUMENTS", key+" is too long.", "Send at most 2000 times.")
	}
	out := make([]primitive.DateTime, 0, len(list))
	for i, item := range list {
		parsed, bad := oneMark(event, eventType, item, key)
		if bad != nil {
			if bad.path == "" {
				text, isText := item.(string)
				if !isText {
					raw, _ := json.Marshal(item)
					text = string(raw)
				}
				bad.at(fmt.Sprintf("%s[%d]", key, i), i, text)
			}
			return nil, bad
		}
		out = append(out, primitive.NewDateTimeFromTime(parsed))
	}
	return out, nil
}

func oneMark(event *models.Event, eventType models.EventType, item any, key string) (time.Time, *ToolError) {
	if text, ok := item.(string); ok {
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return time.Time{}, toolErr("BAD_ARGUMENTS", key+" must be RFC3339 strings.", "Use timestamps like 2026-05-01T15:00:00Z.")
		}
		return parsed.UTC(), nil
	}
	obj, ok := item.(map[string]any)
	if !ok {
		return time.Time{}, toolErr("BAD_ARGUMENTS", key+" must be RFC3339 strings or {weekday, time}.", "Use a timestamp, or a weekday and a UTC clock.")
	}
	weekday, _ := obj["weekday"].(string)
	clock, _ := obj["time"].(string)
	weekday = strings.TrimSpace(weekday)
	clock = strings.TrimSpace(clock)
	if weekday == "" || clock == "" || len(obj) != 2 {
		return time.Time{}, toolErr("BAD_ARGUMENTS", "A weekly time needs weekday and time.", "Pass {weekday: Monday, time: 15:00}.")
	}
	hour, min, sec, bad := parseClock(clock)
	if bad != nil {
		return time.Time{}, bad
	}
	if event != nil {
		return resolveStoredWeekday(event, weekday, hour, min, sec)
	}
	if eventType != models.DOW && eventType != models.GROUP {
		return time.Time{}, toolErr("BAD_ARGUMENTS", "A weekday time is only for a weekly muster.", "Pass RFC3339 dates, or set type to dow.")
	}
	return referenceInstant(weekday, hour, min, sec)
}

func parseClock(clock string) (int, int, int, *ToolError) {
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		parsed, err = time.Parse("15:04:05", clock)
	}
	if err != nil {
		return 0, 0, 0, toolErr("BAD_ARGUMENTS", "time must be HH:MM in UTC.", "Use a clock like 15:00.")
	}
	return parsed.Hour(), parsed.Minute(), parsed.Second(), nil
}

func referenceInstant(weekday string, hour, min, sec int) (time.Time, *ToolError) {
	day, ok := referenceDay[strings.ToLower(weekday)]
	if !ok {
		return time.Time{}, toolErr("BAD_ARGUMENTS", "weekday must be a day name.", "Use Monday through Sunday.")
	}
	return time.Date(2018, 6, day, hour, min, sec, 0, time.UTC), nil
}

func resolveStoredWeekday(event *models.Event, weekday string, hour, min, sec int) (time.Time, *ToolError) {
	if _, ok := referenceDay[strings.ToLower(weekday)]; !ok {
		return time.Time{}, toolErr("BAD_ARGUMENTS", "weekday must be a day name.", "Use Monday through Sunday.")
	}
	hits := []string{}
	var found time.Time
	for _, date := range event.Dates {
		t := date.Time().UTC()
		if !strings.EqualFold(t.Weekday().String(), weekday) {
			continue
		}
		slot := time.Date(t.Year(), t.Month(), t.Day(), hour, min, sec, 0, time.UTC)
		hits = append(hits, slot.Format(time.RFC3339))
		found = slot
	}
	if len(hits) == 1 {
		return found, nil
	}
	if len(hits) == 0 {
		return time.Time{}, toolErr("SLOT_OUTSIDE", "That weekday and time are not on this muster.", "Read dates from muster_get_event, or pass the RFC3339 instant.")
	}
	return time.Time{}, toolErr("AMBIGUOUS_WEEKDAY", "More than one stored time matches that weekday and clock.", "Pass one of the RFC3339 instants: "+strings.Join(hits, ", "))
}

func expandRanges(event *models.Event, raw any) ([]primitive.DateTime, *ToolError) {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, toolErr("BAD_ARGUMENTS", "availability_ranges must be a non-empty array.", "Pass {start, end} ranges, or use availability timestamps.")
	}
	if len(list) > 100 {
		return nil, toolErr("BAD_ARGUMENTS", "At most 100 ranges.", "Split the answer.")
	}
	inc := increment(event)
	out := []primitive.DateTime{}
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, toolErr("BAD_ARGUMENTS", "Each range needs start and end.", "Pass RFC3339 start and end.")
		}
		startText, _ := obj["start"].(string)
		endText, _ := obj["end"].(string)
		start, err := time.Parse(time.RFC3339, startText)
		if err != nil {
			return nil, toolErr("BAD_ARGUMENTS", "range start must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
		}
		end, err := time.Parse(time.RFC3339, endText)
		if err != nil {
			return nil, toolErr("BAD_ARGUMENTS", "range end must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
		}
		if !end.After(start) {
			return nil, toolErr("BAD_ARGUMENTS", "range end must be after start.", "Pass an end after the start.")
		}
		for t := start.UTC(); t.Before(end.UTC()); t = t.Add(inc) {
			out = append(out, primitive.NewDateTimeFromTime(t))
			if len(out) > maxMarks {
				return nil, toolErr("BAD_ARGUMENTS", "The ranges expand to too many slots.", "Send a shorter range.")
			}
		}
	}
	return out, nil
}

func answerMatches(current *models.Response, expected map[string]any) (bool, *ToolError) {
	wantAvail, bad := markList(nil, "", expected, "availability")
	if bad != nil {
		return false, bad
	}
	wantNeed, bad := markList(nil, "", expected, "if_needed")
	if bad != nil {
		return false, bad
	}
	gotAvail, gotNeed := []primitive.DateTime{}, []primitive.DateTime{}
	if current != nil {
		gotAvail, gotNeed = current.Availability, current.IfNeeded
	}
	return sameInstants(gotAvail, wantAvail) && sameInstants(gotNeed, wantNeed), nil
}

func sameInstants(a, b []primitive.DateTime) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[int64]int{}
	for _, t := range a {
		seen[t.Time().UTC().UnixMilli()]++
	}
	for _, t := range b {
		key := t.Time().UTC().UnixMilli()
		seen[key]--
		if seen[key] < 0 {
			return false
		}
	}
	return true
}

// SlotView is one stored mark with its canonical instant. Weekday and Time
// (reference UTC clock) are set only for weekly (dow/group) musters.
type SlotView struct {
	Instant string `json:"instant"`
	Weekday string `json:"weekday,omitempty"`
	Time    string `json:"time,omitempty"`
}

// weeklyShorthand returns the weekday name and HH:MM clock of a stored instant.
func weeklyShorthand(t time.Time) (string, string) {
	t = t.UTC()
	return t.Weekday().String(), t.Format("15:04")
}

// slotViews returns each mark as its canonical RFC3339 instant, plus the weekly
// shorthand when the muster is weekly. Marks are sorted by instant.
func slotViews(event *models.Event, marks []primitive.DateTime) []SlotView {
	times := make([]time.Time, 0, len(marks))
	for _, m := range marks {
		times = append(times, m.Time().UTC())
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	weekly := event != nil && (event.Type == models.DOW || event.Type == models.GROUP)
	out := make([]SlotView, 0, len(times))
	for _, t := range times {
		view := SlotView{Instant: t.Format(time.RFC3339)}
		if weekly {
			view.Weekday, view.Time = weeklyShorthand(t)
		}
		out = append(out, view)
	}
	return out
}
