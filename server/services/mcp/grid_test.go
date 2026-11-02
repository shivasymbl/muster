package mcp

import (
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSlotProblemSpecificDates(t *testing.T) {
	start := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)
	hours := float32(1)
	inc := 15
	event := &models.Event{
		Type:          models.SPECIFIC_DATES,
		Duration:      &hours,
		TimeIncrement: &inc,
		Dates:         []primitive.DateTime{primitive.NewDateTimeFromTime(start)},
	}
	if slotProblem(event, start) != "" {
		t.Fatal("start")
	}
	if slotProblem(event, start.Add(15*time.Minute)) != "" {
		t.Fatal("next slot")
	}
	if slotProblem(event, start.Add(45*time.Minute)) != "" {
		t.Fatal("last slot")
	}
	if slotProblem(event, start.Add(time.Hour)) != "outside" {
		t.Fatal("end is exclusive")
	}
	if slotProblem(event, time.Date(2030, 1, 1, 4, 30, 0, 0, time.UTC)) != "outside" {
		t.Fatal("2030")
	}
	if slotProblem(event, start.Add(7*time.Minute)) != "off" {
		t.Fatal("off grid")
	}
}

func TestSlotProblemWeeklyAndUndated(t *testing.T) {
	// Monday of the reference week the website actually stores.
	ref := time.Date(2018, 6, 18, 16, 0, 0, 0, time.UTC)
	hours := float32(1)
	inc := 15
	event := &models.Event{
		Type:          models.DOW,
		Duration:      &hours,
		TimeIncrement: &inc,
		Dates:         []primitive.DateTime{primitive.NewDateTimeFromTime(ref)},
	}
	if slotProblem(event, ref.Add(15*time.Minute)) != "" {
		t.Fatal("reference week")
	}
	later := time.Date(2026, 11, 2, 16, 15, 0, 0, time.UTC)
	if slotProblem(event, later) != "outside" {
		t.Fatal("later week is not stored")
	}
	other := time.Date(2018, 6, 19, 16, 15, 0, 0, time.UTC)
	if slotProblem(event, other) != "outside" {
		t.Fatal("other reference day")
	}

	nightHours := float32(2)
	night := time.Date(2018, 6, 18, 23, 0, 0, 0, time.UTC)
	event.Duration = &nightHours
	event.Dates = []primitive.DateTime{primitive.NewDateTimeFromTime(night)}
	if slotProblem(event, time.Date(2018, 6, 19, 0, 0, 0, 0, time.UTC)) != "" {
		t.Fatal("midnight crossing")
	}
	if slotProblem(event, time.Date(2018, 6, 19, 0, 45, 0, 0, time.UTC)) != "" {
		t.Fatal("after midnight")
	}
	if slotProblem(event, time.Date(2018, 6, 19, 1, 0, 0, 0, time.UTC)) != "outside" {
		t.Fatal("end is exclusive")
	}

	open := &models.Event{Type: models.GROUP}
	if slotProblem(open, time.Date(2030, 1, 1, 4, 30, 0, 0, time.UTC)) != "" {
		t.Fatal("undated group stays open")
	}
}

func TestEventGrid(t *testing.T) {
	start := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)
	hours := float32(2)
	event := &models.Event{
		Type:     models.GROUP,
		Duration: &hours,
		Name:     "Test ",
		Dates:    []primitive.DateTime{primitive.NewDateTimeFromTime(start)},
	}
	g := eventGrid(event)
	if !g.IsGroup || g.SlotMinutes != 15 || g.DurationHours != 2 || len(g.Dates) != 1 {
		t.Fatalf("%+v", g)
	}
}
