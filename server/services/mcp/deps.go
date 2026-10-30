package mcp

import (
	"errors"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrActiveTokenCap = errors.New("revoke a token first")
var ErrTokenNotFound = errors.New("not found")

// TokenStore is the credential store. GetActive returns nil when the token is unknown, expired, or revoked.
type TokenStore interface {
	Create(user *models.User, label string) (plaintext string, public models.McpTokenPublic, err error)
	List(userID primitive.ObjectID) ([]models.McpTokenPublic, error)
	Revoke(userID, tokenID primitive.ObjectID) error
	GetActive(plaintext string) (*models.McpToken, error)
}

// ListedEvent is one row of the caller's own list, before it is trimmed for the tool.
type ListedEvent struct {
	Event       models.Event
	IsAttendee  bool
	Declined    *bool
	HasResponse bool
	NonEmpty    bool
}

// EventStore reads musters. It does not use the website handlers.
type EventStore interface {
	GetByEitherID(id string) *models.Event
	Responses(id primitive.ObjectID) []models.EventResponse
	Attendees(id primitive.ObjectID) []models.Attendee
	ListForUser(user *models.User, name string, limit int, cursor string) ([]ListedEvent, string, error)
	ResponseFlags(userHex string, ids []primitive.ObjectID) map[string]ResponseFlags
}

// UserStore loads the account that owns a token.
type UserStore interface {
	GetByID(id string) *models.User
}

// Deps is what the route passes into every tool. Now is nil in production and means time.Now.
type Deps struct {
	Tokens TokenStore
	Events EventStore
	Users  UserStore
	Now    func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}
