package mcp

import (
	"sort"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func increment(event *models.Event) time.Duration {
	mins := 15
	if event.TimeIncrement != nil && *event.TimeIncrement > 0 {
		mins = *event.TimeIncrement
	}
	return time.Duration(mins) * time.Minute
}

func mergeMarks(times []time.Time, inc time.Duration) []TimeRange {
	if len(times) == 0 {
		return []TimeRange{}
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	out := make([]TimeRange, 0)
	start := times[0]
	prev := times[0]
	flush := func() {
		out = append(out, TimeRange{
			Start: start.UTC().Format(time.RFC3339),
			End:   prev.Add(inc).UTC().Format(time.RFC3339),
		})
	}
	for _, t := range times[1:] {
		if t.Equal(prev) {
			continue
		}
		if t.Equal(prev.Add(inc)) {
			prev = t
			continue
		}
		flush()
		start, prev = t, t
	}
	flush()
	return out
}

func dateTimes(values []primitive.DateTime) []time.Time {
	out := make([]time.Time, 0, len(values))
	for _, v := range values {
		out = append(out, v.Time())
	}
	return out
}
