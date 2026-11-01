package mcp

import (
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestOverlapIntersection(t *testing.T) {
	event := &models.Event{}
	ada := primitive.NewObjectID()
	bea := primitive.NewObjectID()
	mark := func(h, m int) primitive.DateTime {
		return primitive.NewDateTimeFromTime(time.Date(2033, 11, 22, h, m, 0, 0, time.UTC))
	}
	responses := []models.EventResponse{
		{UserId: ada.Hex(), Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark(15, 0), mark(15, 15), mark(16, 0)}}},
		{UserId: bea.Hex(), Response: &models.Response{Name: "Bea", Availability: []primitive.DateTime{mark(15, 15), mark(16, 0)}}},
	}
	slots := overlapSlots(event, responses, time.Date(2033, 11, 22, 0, 0, 0, 0, time.UTC), time.Date(2033, 11, 23, 0, 0, 0, 0, time.UTC))
	if len(slots) != 2 || slots[0].Start != "2033-11-22T15:15:00Z" || slots[0].End != "2033-11-22T15:30:00Z" || slots[1].Start != "2033-11-22T16:00:00Z" {
		t.Fatal(slots)
	}
	if len(slots[0].Names) != 2 || slots[0].Names[0] != "Ada" || slots[0].Names[1] != "Bea" {
		t.Fatal(slots[0].Names)
	}
}

func TestOverlapSameNameStaysTwoPeople(t *testing.T) {
	event := &models.Event{}
	mark := func(h int) primitive.DateTime {
		return primitive.NewDateTimeFromTime(time.Date(2033, 11, 22, h, 0, 0, 0, time.UTC))
	}
	windowStart := time.Date(2033, 11, 22, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2033, 11, 23, 0, 0, 0, 0, time.UTC)
	shared := []models.EventResponse{
		{UserId: "one", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark(15)}}},
		{UserId: "two", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark(15)}}},
	}
	slots := overlapSlots(event, shared, windowStart, windowEnd)
	if len(slots) != 1 || len(slots[0].Names) != 2 {
		t.Fatal(slots)
	}
	apart := []models.EventResponse{
		{UserId: "one", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark(15)}}},
		{UserId: "two", Response: &models.Response{Name: "Ada", Availability: []primitive.DateTime{mark(16)}}},
	}
	if slots := overlapSlots(event, apart, windowStart, windowEnd); len(slots) != 0 {
		t.Fatal(slots)
	}
}
