package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mcpTokenRepo interface {
	CountActive(userID primitive.ObjectID, now time.Time) (int, error)
	Insert(row models.McpToken) error
	FindByHash(hash []byte) (*models.McpToken, error)
	List(userID primitive.ObjectID) ([]models.McpToken, error)
	Revoke(userID, tokenID primitive.ObjectID, now time.Time) (bool, error)
}

type tokenStore struct {
	repo mcpTokenRepo
	now  func() time.Time
}

// NewMongoTokenStore is the production credential store.
func NewMongoTokenStore() mcp.TokenStore {
	return &tokenStore{repo: mongoTokenRepo{}, now: func() time.Time { return time.Now().UTC() }}
}

func (s *tokenStore) clock() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *tokenStore) Create(user *models.User, label string) (string, models.McpTokenPublic, error) {
	now := s.clock()
	active, err := s.repo.CountActive(user.Id, now)
	if err != nil {
		return "", models.McpTokenPublic{}, err
	}
	if active >= 5 {
		return "", models.McpTokenPublic{}, mcp.ErrActiveTokenCap
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", models.McpTokenPublic{}, err
	}
	plaintext := "muster_" + base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plaintext))
	row := models.McpToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.Id,
		TokenHash: append([]byte(nil), sum[:]...),
		Label:     label,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	if err := s.repo.Insert(row); err != nil {
		return "", models.McpTokenPublic{}, err
	}
	return plaintext, row.Public(), nil
}

func (s *tokenStore) List(userID primitive.ObjectID) ([]models.McpTokenPublic, error) {
	rows, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]models.McpTokenPublic, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Public())
	}
	return out, nil
}

func (s *tokenStore) Revoke(userID, tokenID primitive.ObjectID) error {
	ok, err := s.repo.Revoke(userID, tokenID, s.clock())
	if err != nil {
		return err
	}
	if !ok {
		return mcp.ErrTokenNotFound
	}
	return nil
}

func (s *tokenStore) GetActive(plaintext string) (*models.McpToken, error) {
	sum := sha256.Sum256([]byte(plaintext))
	row, err := s.repo.FindByHash(sum[:])
	if err != nil {
		return nil, err
	}
	if row == nil {
		subtle.ConstantTimeCompare(sum[:], sum[:])
		return nil, nil
	}
	if subtle.ConstantTimeCompare(row.TokenHash, sum[:]) != 1 {
		return nil, nil
	}
	now := s.clock()
	if row.RevokedAt != nil || !row.ExpiresAt.After(now) {
		return nil, nil
	}
	return row, nil
}

type mongoTokenRepo struct{}

func (mongoTokenRepo) CountActive(userID primitive.ObjectID, now time.Time) (int, error) {
	n, err := McpTokensCollection.CountDocuments(context.Background(), bson.M{
		"userId":    userID,
		"revokedAt": nil,
		"expiresAt": bson.M{"$gt": now},
	})
	return int(n), err
}

func (mongoTokenRepo) Insert(row models.McpToken) error {
	_, err := McpTokensCollection.InsertOne(context.Background(), row)
	return err
}

func (mongoTokenRepo) FindByHash(hash []byte) (*models.McpToken, error) {
	var row models.McpToken
	err := McpTokensCollection.FindOne(context.Background(), bson.M{"tokenHash": hash}).Decode(&row)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (mongoTokenRepo) List(userID primitive.ObjectID) ([]models.McpToken, error) {
	cursor, err := McpTokensCollection.Find(context.Background(), bson.M{"userId": userID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	rows := make([]models.McpToken, 0)
	if err := cursor.All(context.Background(), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (mongoTokenRepo) Revoke(userID, tokenID primitive.ObjectID, now time.Time) (bool, error) {
	res, err := McpTokensCollection.UpdateOne(context.Background(), bson.M{"_id": tokenID, "userId": userID}, bson.M{"$set": bson.M{"revokedAt": now}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}
