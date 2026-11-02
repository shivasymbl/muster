package mcp

import (
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRankSlotsPrefersTheFullerTime(t *testing.T) {
	event := &models.Event{}
	mark := func(h, m int) primitive.DateTime {
		return primitive.NewDateTimeFromTime(time.Date(2033, 11, 22, h, m, 0, 0, time.UTC))
	}
	responses := []models.EventResponse{
		{UserId: "ada", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark(15, 0), mark(15, 15)}}},
		{UserId: "bea", Response: &models.Response{Name: "Bea", Availability: []primitive.DateTime{mark(15, 0)}}},
	}
	start := time.Date(2033, 11, 22, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	slots := rankSlots(event, responses, start, end, 30*time.Minute)
	if len(slots) != 1 || slots[0].Available != 1 || slots[0].Start != "2033-11-22T15:00:00Z" {
		t.Fatal(slots)
	}
	if len(slots[0].Missing) != 1 || slots[0].Missing[0] != "Bea" {
		t.Fatal(slots[0].Missing)
	}
}
