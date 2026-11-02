package mcp

import (
	"math"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/utils"
)

// Grid is the schedule an assistant can see. Times are the stored UTC instants.
// Muster does not keep a timezone on the muster.
type Grid struct {
	Dates         []string `json:"dates,omitempty"`
	Weekdays      []string `json:"weekdays,omitempty"`
	DurationHours float64  `json:"duration_hours,omitempty"`
	SlotMinutes   int      `json:"slot_minutes"`
	DaysOnly      bool     `json:"days_only"`
	StartOnMonday bool     `json:"start_on_monday"`
	IsGroup       bool     `json:"is_group"`
}

func eventGrid(event *models.Event) Grid {
	g := Grid{
		SlotMinutes:   int(increment(event) / time.Minute),
		DaysOnly:      utils.Coalesce(event.DaysOnly),
		StartOnMonday: utils.Coalesce(event.StartOnMonday),
		IsGroup:       event.Type == models.GROUP,
		Dates:         []string{},
	}
	if event.Duration != nil && *event.Duration > 0 {
		g.DurationHours = float64(*event.Duration)
	}
	for _, d := range event.Dates {
		t := d.Time().UTC()
		g.Dates = append(g.Dates, t.Format(time.RFC3339))
		if event.Type == models.DOW {
			g.Weekdays = append(g.Weekdays, t.Weekday().String())
		}
	}
	if g.Weekdays == nil {
		g.Weekdays = []string{}
	}
	return g
}

func slotProblem(event *models.Event, t time.Time) string {
	if event == nil || len(event.Dates) == 0 {
		return ""
	}
	t = t.UTC()
	inc := increment(event)
	dur := slotSpan(event, inc)
	off := false
	for _, d := range event.Dates {
		start := d.Time().UTC()
		delta, inside := slotDelta(event, start, t)
		if !inside {
			continue
		}
		if delta >= dur {
			continue
		}
		if delta%inc != 0 {
			off = true
			continue
		}
		return ""
	}
	if off {
		return "off"
	}
	return "outside"
}

func slotSpan(event *models.Event, inc time.Duration) time.Duration {
	if event.Duration == nil || *event.Duration <= 0 {
		return inc
	}
	mins := math.Round(float64(*event.Duration) * 60)
	if mins <= 0 {
		return inc
	}
	return time.Duration(mins) * time.Minute
}

func slotDelta(event *models.Event, start, t time.Time) (time.Duration, bool) {
	if event.Type == models.DOW {
		if t.Weekday() != start.Weekday() {
			return 0, false
		}
		delta := clockOf(t) - clockOf(start)
		if delta < 0 {
			return 0, false
		}
		return delta, true
	}
	if t.Before(start) {
		return 0, false
	}
	return t.Sub(start), true
}

func clockOf(t time.Time) time.Duration {
	t = t.UTC()
	return time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second +
		time.Duration(t.Nanosecond())
}
