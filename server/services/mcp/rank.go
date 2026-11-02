package mcp

import (
	"sort"
	"time"

	"github.com/shivasymbl/muster/server/models"
)

type bestSlot struct {
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Available int      `json:"available"`
	IfNeeded  int      `json:"if_needed"`
	Missing   []string `json:"missing"`
}

func rankSlots(event *models.Event, responses []models.EventResponse, start, end time.Time, duration time.Duration) []bestSlot {
	inc := increment(event)
	if duration < inc {
		duration = inc
	}
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
			if !t.Before(start) && t.Before(end) {
				p.free[t.Unix()] = true
			}
		}
		for _, mark := range row.Response.IfNeeded {
			t := mark.Time().UTC()
			if !t.Before(start) && t.Before(end) {
				p.ifNeeded[t.Unix()] = true
			}
		}
		if len(p.free) == 0 && len(p.ifNeeded) == 0 {
			continue
		}
		people = append(people, p)
	}
	seen := map[int64]bool{}
	starts := []time.Time{}
	for _, p := range people {
		for unix := range p.free {
			if seen[unix] {
				continue
			}
			seen[unix] = true
			starts = append(starts, time.Unix(unix, 0).UTC())
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
	out := []bestSlot{}
	steps := int(duration / inc)
	if steps < 1 {
		steps = 1
	}
	for _, slotStart := range starts {
		slotEnd := slotStart.Add(duration)
		if slotEnd.After(end) {
			continue
		}
		available := 0
		ifNeeded := 0
		missing := []string{}
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
			}
		}
		if available == 0 && ifNeeded == 0 {
			continue
		}
		sort.Strings(missing)
		out = append(out, bestSlot{
			Start:     slotStart.Format(time.RFC3339),
			End:       slotEnd.Format(time.RFC3339),
			Available: available,
			IfNeeded:  ifNeeded,
			Missing:   missing,
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
