package oauth

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ID        string
	Name      string
	Redirects []string
}

type Code struct {
	UserID    primitive.ObjectID
	ClientID  string
	Redirect  string
	Challenge string
	Expires   time.Time
}

type Refresh struct {
	UserID   primitive.ObjectID
	ClientID string
	TokenID  primitive.ObjectID
	Expires  time.Time
}

// Store keeps OAuth clients, one-time codes, and refresh tokens.
type Store interface {
	SaveClient(Client) error
	Client(id string) (*Client, error)
	SaveCode(plain string, code Code) error
	TakeCode(plain string) (*Code, error)
	SaveRefresh(plain string, row Refresh) error
	TakeRefresh(plain string) (*Refresh, error)
}
