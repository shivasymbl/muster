package db

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/shivasymbl/muster/server/services/oauth"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type OAuthGrant struct {
	Kind      string             `bson:"kind"`
	ID        string             `bson:"id,omitempty"`
	Name      string             `bson:"name,omitempty"`
	Redirects []string           `bson:"redirects,omitempty"`
	Hash      []byte             `bson:"hash,omitempty"`
	UserID    primitive.ObjectID `bson:"userId,omitempty"`
	ClientID  string             `bson:"clientId,omitempty"`
	Redirect  string             `bson:"redirect,omitempty"`
	Challenge string             `bson:"challenge,omitempty"`
	TokenID   primitive.ObjectID `bson:"tokenId,omitempty"`
	ExpiresAt time.Time          `bson:"expiresAt,omitempty"`
	CreatedAt time.Time          `bson:"createdAt"`
}

type mongoOAuth struct{}

func NewOAuthStore() oauth.Store {
	return mongoOAuth{}
}

func oauthCollection() *mongo.Collection {
	return Db.Collection("oauthGrants")
}

func (mongoOAuth) SaveClient(client oauth.Client) error {
	_, err := oauthCollection().InsertOne(context.Background(), OAuthGrant{
		Kind: "client", ID: client.ID, Name: client.Name, Redirects: client.Redirects, CreatedAt: time.Now().UTC(),
	})
	return err
}

func (mongoOAuth) Client(id string) (*oauth.Client, error) {
	var row OAuthGrant
	err := oauthCollection().FindOne(context.Background(), bson.M{"kind": "client", "id": id}).Decode(&row)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &oauth.Client{ID: row.ID, Name: row.Name, Redirects: row.Redirects}, nil
}

func (mongoOAuth) SaveCode(plain string, code oauth.Code) error {
	sum := sha256.Sum256([]byte(plain))
	_, err := oauthCollection().InsertOne(context.Background(), OAuthGrant{
		Kind: "code", Hash: sum[:], UserID: code.UserID, ClientID: code.ClientID, Redirect: code.Redirect, Challenge: code.Challenge, ExpiresAt: code.Expires, CreatedAt: time.Now().UTC(),
	})
	return err
}

func (mongoOAuth) TakeCode(plain string) (*oauth.Code, error) {
	sum := sha256.Sum256([]byte(plain))
	var row OAuthGrant
	err := oauthCollection().FindOneAndDelete(context.Background(), bson.M{"kind": "code", "hash": sum[:]}).Decode(&row)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &oauth.Code{UserID: row.UserID, ClientID: row.ClientID, Redirect: row.Redirect, Challenge: row.Challenge, Expires: row.ExpiresAt}, nil
}

func (mongoOAuth) SaveRefresh(plain string, row oauth.Refresh) error {
	sum := sha256.Sum256([]byte(plain))
	_, err := oauthCollection().InsertOne(context.Background(), OAuthGrant{
		Kind: "refresh", Hash: sum[:], UserID: row.UserID, ClientID: row.ClientID, TokenID: row.TokenID, ExpiresAt: row.Expires, CreatedAt: time.Now().UTC(),
	})
	return err
}

func (mongoOAuth) TakeRefresh(plain string) (*oauth.Refresh, error) {
	sum := sha256.Sum256([]byte(plain))
	var row OAuthGrant
	err := oauthCollection().FindOneAndDelete(context.Background(), bson.M{"kind": "refresh", "hash": sum[:]}).Decode(&row)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &oauth.Refresh{UserID: row.UserID, ClientID: row.ClientID, TokenID: row.TokenID, Expires: row.ExpiresAt}, nil
}
