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

// EventStore reads and updates musters. It does not use the website handlers.
type EventStore interface {
	GetByEitherID(id string) *models.Event
	Responses(id primitive.ObjectID) []models.EventResponse
	Attendees(id primitive.ObjectID) []models.Attendee
	ListForUser(user *models.User, name string, limit int, cursor string) ([]ListedEvent, string, error)
	ResponseFlags(userHex string, ids []primitive.ObjectID) map[string]ResponseFlags
	ReplaceAvailability(user *models.User, eventID primitive.ObjectID, availability, ifNeeded []primitive.DateTime) error
	Decline(user *models.User, eventID primitive.ObjectID) error
	AddAttendees(eventID primitive.ObjectID, emails []string) ([]string, error)
	Create(user *models.User, event models.Event, emails []string) (*models.Event, error)
	Patch(eventID primitive.ObjectID, set map[string]any) error
	RemoveAttendee(eventID primitive.ObjectID, email, responseUserHex string) error
	Changes(eventID primitive.ObjectID, since time.Time) ([]models.EventResponse, []models.Attendee, error)
}

// UserStore loads the account that owns a token.
type UserStore interface {
	GetByID(id string) *models.User
	GetByEmail(email string) *models.User
}

// ActionRecord is one prepared App write. It is not an authorization token.
type ActionRecord struct {
	ID          primitive.ObjectID `bson:"_id"`
	UserID      primitive.ObjectID `bson:"userId"`
	Operation   string             `bson:"operation"`
	EventID     string             `bson:"eventId,omitempty"`
	Fingerprint string             `bson:"fingerprint,omitempty"`
	Payload     map[string]any     `bson:"payload"`
	Preview     map[string]any     `bson:"preview"`
	Status      string             `bson:"status"`
	Result      map[string]any     `bson:"result,omitempty"`
	ErrCode     string             `bson:"errCode,omitempty"`
	ErrMessage  string             `bson:"errMessage,omitempty"`
	ExpiresAt   time.Time          `bson:"expiresAt"`
}

// ActionStore persists prepared writes. Claim must succeed at most once.
type ActionStore interface {
	Create(rec *ActionRecord) error
	Claim(userID, id primitive.ObjectID, now time.Time) (*ActionRecord, error)
	Get(userID, id primitive.ObjectID) (*ActionRecord, error)
	Finish(userID, id primitive.ObjectID, status string, result map[string]any, errCode, errMessage string) error
}

// Deps is what the route passes into every tool. Now is nil in production and means time.Now.
type Deps struct {
	Tokens  TokenStore
	Events  EventStore
	Users   UserStore
	Actions ActionStore
	Now     func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}
