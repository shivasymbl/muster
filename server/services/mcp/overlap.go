package mcp

import (
	"fmt"
	"sort"
	"time"

	"github.com/shivasymbl/muster/server/models"
)

type slot struct {
	Start string   `json:"start"`
	End   string   `json:"end"`
	Names []string `json:"names"`
}

type span struct {
	start time.Time
	end   time.Time
	names []string
}

// overlapSlots returns times every respondent in the window can make.
// People with no availability in the window are left out. Names are included
// because the caller is the owner or blind availability is off.
type respondent struct {
	name  string
	marks []time.Time
}

func overlapSlots(event *models.Event, responses []models.EventResponse, start, end time.Time) []slot {
	inc := increment(event)
	byKey := map[string]*respondent{}
	order := make([]string, 0)
	for i, row := range responses {
		if row.Response == nil {
			continue
		}
		key := row.UserId
		if key == "" && !row.Response.UserId.IsZero() {
			key = row.Response.UserId.Hex()
		}
		if key == "" {
			key = fmt.Sprintf("row-%d", i)
		}
		person, ok := byKey[key]
		if !ok {
			name := personName(row.Response)
			if name == "" {
				name = "Unnamed"
			}
			person = &respondent{name: name}
			byKey[key] = person
			order = append(order, key)
		}
		for _, mark := range row.Response.Availability {
			t := mark.Time()
			if !t.Before(start) && !t.After(end) {
				person.marks = append(person.marks, t)
			}
		}
	}
	sets := []span{}
	names := make([]string, 0)
	started := false
	for _, key := range order {
		parsed := marksToSpans(mergeMarks(byKey[key].marks, inc))
		if len(parsed) == 0 {
			continue
		}
		if !started {
			sets = parsed
			started = true
		} else {
			sets = intersectSpans(sets, parsed)
		}
		names = append(names, byKey[key].name)
	}
	if !started {
		return []slot{}
	}
	sort.Strings(names)
	out := make([]slot, 0, len(sets))
	for _, item := range sets {
		out = append(out, slot{
			Start: item.start.UTC().Format(time.RFC3339),
			End:   item.end.UTC().Format(time.RFC3339),
			Names: append([]string{}, names...),
		})
	}
	return out
}

func marksToSpans(ranges []TimeRange) []span {
	parsed := make([]span, 0, len(ranges))
	for _, item := range ranges {
		s, err1 := time.Parse(time.RFC3339, item.Start)
		e, err2 := time.Parse(time.RFC3339, item.End)
		if err1 != nil || err2 != nil {
			continue
		}
		parsed = append(parsed, span{start: s, end: e})
	}
	return parsed
}

func intersectSpans(left, right []span) []span {
	out := make([]span, 0)
	for _, a := range left {
		for _, b := range right {
			start := a.start
			if b.start.After(start) {
				start = b.start
			}
			end := a.end
			if b.end.Before(end) {
				end = b.end
			}
			if end.After(start) {
				out = append(out, span{start: start, end: end})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start.Before(out[j].start) })
	return out
}
