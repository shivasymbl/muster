package events

import (
	"strings"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Viewer is the person looking at responses. Email is already trimmed and lowercased.
// The caller loads the user and the attendees. This package does not call Mongo.
type Viewer struct {
	UserID    string
	GuestName string
	Email     string
}

func ResponsesMap(responses []models.EventResponse) map[string]*models.Response {
	result := make(map[string]*models.Response)
	for _, resp := range responses {
		result[resp.UserId] = resp.Response
	}
	return result
}

func StripSensitiveUserFields(user *models.User) {
	if user == nil {
		return
	}
	user.CalendarAccounts = nil
	user.CalendarOptions = nil
	user.PrimaryAccountKey = nil
}

// KeepGroupEmail matches the old shouldKeepGroupResponseUserEmails result
// when the caller has already loaded the viewer email and the attendee list.
func KeepGroupEmail(eventType models.EventType, viewerEmail string, isOwner bool, attendees []models.Attendee) bool {
	if eventType != models.GROUP || (viewerEmail == "" && !isOwner) {
		return false
	}
	if isOwner {
		return true
	}
	for _, a := range attendees {
		if strings.ToLower(strings.TrimSpace(a.Email)) == viewerEmail {
			return true
		}
	}
	return false
}

func FilterResponses(event *models.Event, viewer Viewer, responses []models.EventResponse, attendees []models.Attendee, start, end time.Time) map[string]*models.Response {
	responsesMap := ResponsesMap(responses)
	for userID, response := range responsesMap {
		if response == nil {
			continue
		}
		subsetAvailability := make([]primitive.DateTime, 0)
		for _, timestamp := range response.Availability {
			t := timestamp.Time()
			if !t.Before(start) && !t.After(end) {
				subsetAvailability = append(subsetAvailability, timestamp)
			}
		}
		response.Availability = subsetAvailability

		subsetIfNeeded := make([]primitive.DateTime, 0)
		for _, timestamp := range response.IfNeeded {
			t := timestamp.Time()
			if !t.Before(start) && !t.After(end) {
				subsetIfNeeded = append(subsetIfNeeded, timestamp)
			}
		}
		response.IfNeeded = subsetIfNeeded

		subsetManual := make(map[primitive.DateTime][]primitive.DateTime)
		for timestamp := range utils.Coalesce(response.ManualAvailability) {
			t := timestamp.Time()
			if !t.Before(start) && !t.After(end) {
				subsetManual[timestamp] = (*response.ManualAvailability)[timestamp]
			}
		}
		response.ManualAvailability = &subsetManual
		responsesMap[userID] = response
	}

	isOwner := viewer.UserID != "" && event.OwnerId.Hex() == viewer.UserID
	showEmails := isOwner && utils.Coalesce(event.CollectEmails)
	keepGroup := KeepGroupEmail(event.Type, viewer.Email, isOwner, attendees)
	for userID, response := range responsesMap {
		if response == nil {
			continue
		}
		StripSensitiveUserFields(response.User)
		if !showEmails {
			response.Email = ""
			if response.User != nil && !keepGroup {
				response.User.Email = ""
			}
		}
		responsesMap[userID] = response
	}

	if !utils.Coalesce(event.BlindAvailabilityEnabled) {
		return responsesMap
	}
	if viewer.UserID != "" {
		if isOwner {
			return responsesMap
		}
		filtered := make(map[string]*models.Response)
		if userResponse, exists := responsesMap[viewer.UserID]; exists {
			filtered[viewer.UserID] = userResponse
		}
		return filtered
	}
	if viewer.GuestName != "" {
		filtered := make(map[string]*models.Response)
		if guestResponse, exists := responsesMap[viewer.GuestName]; exists {
			filtered[viewer.GuestName] = guestResponse
		}
		return filtered
	}
	return map[string]*models.Response{}
}
