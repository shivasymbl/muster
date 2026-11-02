package mcp

import (
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func reviewFixture() (*models.Event, time.Time, primitive.DateTime) {
	start := time.Date(2033, 11, 22, 15, 0, 0, 0, time.UTC)
	hours := float32(1)
	mark := primitive.NewDateTimeFromTime(start)
	return &models.Event{Duration: &hours, Dates: []primitive.DateTime{mark}}, start, mark
}

func TestReviewRegressionRankRetainsEmptyRespondent(t *testing.T) {
	event, start, mark := reviewFixture()
	rows := []models.EventResponse{
		{UserId: "ada", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark}}},
		{UserId: "bea", Response: &models.Response{Name: "Bea"}},
	}
	got := rankSlots(event, rows, start, start.Add(time.Hour), 15*time.Minute)
	if len(got) != 1 || len(got[0].Missing) != 1 || got[0].Missing[0] != "Bea" {
		t.Fatalf("empty stored answer disappeared from missing: %+v", got)
	}
}

func TestReviewRegressionRankRetainsOutsideWindowRespondent(t *testing.T) {
	event, start, mark := reviewFixture()
	rows := []models.EventResponse{
		{UserId: "ada", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark}}},
		{UserId: "bea", Response: &models.Response{Name: "Bea", Availability: []primitive.DateTime{primitive.NewDateTimeFromTime(start.Add(24 * time.Hour))}}},
	}
	got := rankSlots(event, rows, start, start.Add(time.Hour), 15*time.Minute)
	if len(got) != 1 || len(got[0].Missing) != 1 {
		t.Fatalf("out-of-window respondent disappeared from missing: %+v", got)
	}
}

func TestReviewRegressionRankIncludesIfNeededOnlySlot(t *testing.T) {
	event, start, mark := reviewFixture()
	rows := []models.EventResponse{
		{UserId: "ada", Response: &models.Response{Name: "Ada", IfNeeded: []primitive.DateTime{mark}}},
	}
	got := rankSlots(event, rows, start, start.Add(time.Hour), 15*time.Minute)
	if len(got) != 1 || got[0].IfNeeded != 1 {
		t.Fatalf("if-needed-only fit omitted: %+v", got)
	}
}

func TestReviewRegressionRankDurationCoverageAndFinalizeAgree(t *testing.T) {
	event, start, mark := reviewFixture()
	rows := []models.EventResponse{
		{UserId: "ada", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark}}},
	}
	got := rankSlots(event, rows, start, start.Add(time.Hour), 20*time.Minute)
	for _, slot := range got {
		s, _ := time.Parse(time.RFC3339, slot.Start)
		e, _ := time.Parse(time.RFC3339, slot.End)
		if slot.Available > 0 && !spanOnGrid(event, s, e) {
			t.Fatalf("15 minutes of availability counted for a 20-minute fit that finalize rejects: %+v", slot)
		}
	}
}

func TestReviewRegressionRankRejectsOldGridMarks(t *testing.T) {
	event, start, _ := reviewFixture()
	hours := float32(0.25)
	event.Duration = &hours
	rows := []models.EventResponse{
		{UserId: "ada", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{primitive.NewDateTimeFromTime(start.Add(30 * time.Minute))}}},
	}
	got := rankSlots(event, rows, start, start.Add(time.Hour), 15*time.Minute)
	for _, slot := range got {
		s, _ := time.Parse(time.RFC3339, slot.Start)
		e, _ := time.Parse(time.RFC3339, slot.End)
		if !spanOnGrid(event, s, e) {
			t.Fatalf("old grid mark recommended although finalize rejects it: %+v", slot)
		}
	}
}
