package mcp

import (
	"sort"
	"time"

	"github.com/shivasymbl/muster/server/models"
)

type bestSlot struct {
	Start     string `json:"start"`
	End       string `json:"end"`
	Available int    `json:"available"`
	IfNeeded  int    `json:"if_needed"`
	// Missing keeps its meaning: everyone who is not fully available, conditional people included.
	Missing []string `json:"missing"`
	// IfNeededNames and UnavailableNames split Missing into two disjoint groups, one entry per person.
	IfNeededNames    []string `json:"if_needed_names"`
	UnavailableNames []string `json:"unavailable_names"`
	IfNeededCount    int      `json:"if_needed_count"`
	UnavailableCount int      `json:"unavailable_count"`
}

func rankSlots(event *models.Event, responses []models.EventResponse, start, end time.Time, duration time.Duration) []bestSlot {
	inc := increment(event)
	type person struct {
		name     string
		free     map[int64]bool
		ifNeeded map[int64]bool
	}
	people := []*person{}
	for _, row := range responses {
		if row.Response == nil {
			continue
		}
		p := &person{name: personName(row.Response), free: map[int64]bool{}, ifNeeded: map[int64]bool{}}
		if p.name == "" {
			p.name = "Unnamed"
		}
		for _, mark := range row.Response.Availability {
			t := mark.Time().UTC()
			if !t.Before(start) && t.Before(end) && slotProblem(event, t) == "" {
				p.free[t.Unix()] = true
			}
		}
		for _, mark := range row.Response.IfNeeded {
			t := mark.Time().UTC()
			if !t.Before(start) && t.Before(end) && slotProblem(event, t) == "" {
				p.ifNeeded[t.Unix()] = true
			}
		}
		people = append(people, p)
	}
	seen := map[int64]bool{}
	starts := []time.Time{}
	for _, p := range people {
		for _, marks := range []map[int64]bool{p.free, p.ifNeeded} {
			for unix := range marks {
				if seen[unix] {
					continue
				}
				seen[unix] = true
				starts = append(starts, time.Unix(unix, 0).UTC())
			}
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
	out := []bestSlot{}
	steps := 1
	for _, slotStart := range starts {
		slotEnd := slotStart.Add(duration)
		if slotEnd.After(end) || !spanOnGrid(event, slotStart, slotEnd) {
			continue
		}
		steps = int(slotEnd.Sub(slotStart) / inc)
		if steps < 1 {
			continue
		}
		available := 0
		ifNeeded := 0
		missing := []string{}
		conditional, unavailable := []string{}, []string{}
		for _, p := range people {
			free := true
			backup := true
			for i := 0; i < steps; i++ {
				unix := slotStart.Add(time.Duration(i) * inc).Unix()
				if p.free[unix] {
					continue
				}
				free = false
				if !p.ifNeeded[unix] {
					backup = false
				}
			}
			if free {
				available++
				continue
			}
			missing = append(missing, p.name)
			if backup {
				ifNeeded++
				conditional = append(conditional, p.name)
			} else {
				unavailable = append(unavailable, p.name)
			}
		}
		if available == 0 && ifNeeded == 0 {
			continue
		}
		sort.Strings(missing)
		sort.Strings(conditional)
		sort.Strings(unavailable)
		out = append(out, bestSlot{
			Start:     slotStart.Format(time.RFC3339),
			End:       slotEnd.Format(time.RFC3339),
			Available: available,
			IfNeeded:  ifNeeded,
			Missing:   missing,

			IfNeededNames:    conditional,
			UnavailableNames: unavailable,
			IfNeededCount:    len(conditional),
			UnavailableCount: len(unavailable),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Available != out[j].Available {
			return out[i].Available > out[j].Available
		}
		if out[i].IfNeeded != out[j].IfNeeded {
			return out[i].IfNeeded > out[j].IfNeeded
		}
		return out[i].Start < out[j].Start
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}
