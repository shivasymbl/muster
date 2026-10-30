package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// McpToken is the stored credential. Handlers must not JSON-encode this struct.
type McpToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	UserID    primitive.ObjectID `bson:"userId" json:"-"`
	TokenHash []byte             `bson:"tokenHash" json:"-"`
	Label     string             `bson:"label" json:"-"`
	CreatedAt time.Time          `bson:"createdAt" json:"-"`
	ExpiresAt time.Time          `bson:"expiresAt" json:"-"`
	RevokedAt *time.Time         `bson:"revokedAt,omitempty" json:"-"`
}

// McpTokenPublic is the Settings list row. It has no secret.
type McpTokenPublic struct {
	ID        string     `json:"id"`
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

func (t McpToken) Public() McpTokenPublic {
	return McpTokenPublic{
		ID:        t.ID.Hex(),
		Label:     t.Label,
		CreatedAt: t.CreatedAt,
		ExpiresAt: t.ExpiresAt,
		RevokedAt: t.RevokedAt,
	}
}
