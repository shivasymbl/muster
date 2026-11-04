package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const maxRanges = 100

// answerSnapshot is the caller's stored answer as read before a ranged write.
// It is the comparison value for the conditional update.
type answerSnapshot struct {
	Exists       bool
	UpdatedAt    primitive.DateTime
	Availability []primitive.DateTime
	IfNeeded     []primitive.DateTime
}

// AnswerSnapshot is the exported form used by stores.
type AnswerSnapshot = answerSnapshot

type rangeChange struct {
	start, end time.Time
	state      string
}

// parseRanges validates the ranges argument: shape, state, positive span and the 100-range cap.
func parseRanges(raw any) ([]rangeChange, *ToolError) {
	list, ok := raw.([]any)
	if !ok {
		return nil, toolErr("BAD_ARGUMENTS", "ranges must be an array.", "Pass [{start, end, state}].")
	}
	if len(list) > maxRanges {
		return nil, toolErr("BAD_ARGUMENTS", "At most 100 ranges.", "Split the answer.")
	}
	out := make([]rangeChange, 0, len(list))
	for i, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, indexed(i, "Each range needs start, end and state.", "Pass RFC3339 start and end and a state.")
		}
		startText, _ := obj["start"].(string)
		endText, _ := obj["end"].(string)
		state, _ := obj["state"].(string)
		start, err := time.Parse(time.RFC3339, startText)
		if err != nil {
			return nil, indexed(i, "range start must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
		}
		end, err := time.Parse(time.RFC3339, endText)
		if err != nil {
			return nil, indexed(i, "range end must be RFC3339.", "Pass start and end as RFC3339 timestamps.")
		}
		if !end.After(start) {
			return nil, indexed(i, "range end must be after start.", "Ranges are half-open [start,end); end must be later.")
		}
		if state != "available" && state != "if_needed" && state != "unavailable" {
			return nil, indexed(i, "range state must be available, if_needed or unavailable.", "Pass one of those three states.")
		}
		out = append(out, rangeChange{start.UTC(), end.UTC(), state})
	}
	return out, nil
}

func indexed(i int, message, next string) *ToolError {
	return toolErr("BAD_ARGUMENTS", fmt.Sprintf("ranges[%d]: %s", i, message), next).at(fmt.Sprintf("ranges[%d]", i), i, "")
}

// applyRanges expands half-open ranges onto the muster grid and assigns each
// covered slot its state. Replace starts from an empty answer; merge starts
// from the stored one. Overlapping ranges with different states are rejected.
// Unavailable removes a slot from both lists.
func applyRanges(event *models.Event, changes []rangeChange, merge bool, current *answerSnapshot) (avail, ifNeeded []primitive.DateTime, bad *ToolError) {
	inc := increment(event)
	state := map[int64]string{}
	total := 0
	for i, c := range changes {
		for t := c.start; t.Before(c.end); t = t.Add(inc) {
			total++
			if total > maxMarks {
				return nil, nil, toolErr("BAD_ARGUMENTS", "The ranges expand to too many slots.", "Send a shorter range.")
			}
			key := t.UnixMilli()
			if prev, ok := state[key]; ok && prev != c.state {
				return nil, nil, indexed(i, "overlaps another range with a different state at "+t.Format(time.RFC3339)+".", "Make ranges disjoint or give them the same state.")
			}
			state[key] = c.state
		}
	}
	final := map[int64]string{}
	if merge && current != nil {
		for _, m := range current.Availability {
			final[m.Time().UTC().UnixMilli()] = "available"
		}
		for _, m := range current.IfNeeded {
			final[m.Time().UTC().UnixMilli()] = "if_needed"
		}
	}
	for k, s := range state {
		if s == "unavailable" {
			delete(final, k)
		} else {
			final[k] = s
		}
	}
	if len(final) > maxMarks {
		return nil, nil, toolErr("BAD_ARGUMENTS", "The answer would have too many slots.", "Send fewer slots.")
	}
	keys := make([]int64, 0, len(final))
	for k := range final {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	avail, ifNeeded = []primitive.DateTime{}, []primitive.DateTime{}
	for _, k := range keys {
		if final[k] == "available" {
			avail = append(avail, primitive.DateTime(k))
		} else {
			ifNeeded = append(ifNeeded, primitive.DateTime(k))
		}
	}
	return avail, ifNeeded, nil
}

// stateFingerprint is an opaque digest of an answer's two lists, order-insensitive.
func stateFingerprint(avail, ifNeeded []primitive.DateTime) string {
	h := sha256.New()
	for _, list := range [][]primitive.DateTime{avail, ifNeeded} {
		keys := make([]int64, 0, len(list))
		for _, m := range list {
			keys = append(keys, m.Time().UTC().UnixMilli())
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		fmt.Fprintf(h, "%d:", len(keys))
		for _, k := range keys {
			fmt.Fprintf(h, "%d,", k)
		}
		h.Write([]byte("|"))
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// affectedWindow is the smallest interval covering every requested range.
func affectedWindow(changes []rangeChange) (time.Time, time.Time) {
	start, end := changes[0].start, changes[0].end
	for _, c := range changes[1:] {
		if c.start.Before(start) {
			start = c.start
		}
		if c.end.After(end) {
			end = c.end
		}
	}
	return start, end
}
