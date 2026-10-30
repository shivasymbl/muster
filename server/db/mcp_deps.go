package db

import (
	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type mongoEvents struct{}

func (mongoEvents) GetByEitherID(id string) *models.Event {
	return GetEventByEitherId(id)
}

func (mongoEvents) Responses(id primitive.ObjectID) []models.EventResponse {
	return GetEventResponses(id.Hex())
}

func (mongoEvents) Attendees(id primitive.ObjectID) []models.Attendee {
	return GetAttendees(id.Hex())
}

func (mongoEvents) ListForUser(user *models.User, name string, limit int, cursor string) ([]mcp.ListedEvent, string, error) {
	return ListEventsForUser(user, name, limit, cursor)
}

func (mongoEvents) ResponseFlags(userHex string, ids []primitive.ObjectID) map[string]mcp.ResponseFlags {
	return ResponseFlags(userHex, ids)
}

type mongoUsers struct{}

func (mongoUsers) GetByID(id string) *models.User {
	return GetUserById(id)
}

// NewMcpDeps wires the tool route to Mongo.
func NewMcpDeps() mcp.Deps {
	return mcp.Deps{
		Tokens: NewMongoTokenStore(),
		Events: mongoEvents{},
		Users:  mongoUsers{},
	}
}
