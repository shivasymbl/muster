package db

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/services/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeTokenRepo struct {
	rows    []models.McpToken
	inserts int
}

func (f *fakeTokenRepo) CountActive(userID primitive.ObjectID, now time.Time) (int, error) {
	n := 0
	for _, row := range f.rows {
		if row.UserID == userID && row.RevokedAt == nil && row.ExpiresAt.After(now) {
			n++
		}
	}
	return n, nil
}

func (f *fakeTokenRepo) Insert(row models.McpToken) error {
	f.inserts++
	f.rows = append(f.rows, row)
	return nil
}

func (f *fakeTokenRepo) FindByHash(hash []byte) (*models.McpToken, error) {
	for i := range f.rows {
		if string(f.rows[i].TokenHash) == string(hash) {
			row := f.rows[i]
			return &row, nil
		}
	}
	return nil, nil
}

func (f *fakeTokenRepo) List(userID primitive.ObjectID) ([]models.McpToken, error) {
	out := make([]models.McpToken, 0)
	for _, row := range f.rows {
		if row.UserID == userID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeTokenRepo) Revoke(userID, tokenID primitive.ObjectID, now time.Time) (bool, error) {
	for i := range f.rows {
		if f.rows[i].ID == tokenID && f.rows[i].UserID == userID {
			t := now
			f.rows[i].RevokedAt = &t
			return true, nil
		}
	}
	return false, nil
}

func TestMcpTokenHashCapAndExpiry(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	repo := &fakeTokenRepo{}
	store := &tokenStore{repo: repo, now: func() time.Time { return now }}
	user := &models.User{Id: primitive.NewObjectID(), Email: "a@b.com"}

	var plaintext string
	for i := 0; i < 5; i++ {
		token, _, err := store.Create(user, "phone")
		if err != nil {
			t.Fatal(err)
		}
		plaintext = token
	}
	before := repo.inserts
	if _, _, err := store.Create(user, "sixth"); err != mcp.ErrActiveTokenCap {
		t.Fatalf("sixth create: %v", err)
	}
	if repo.inserts != before {
		t.Fatal("sixth create wrote a row")
	}
	if len(repo.rows) != 5 || repo.rows[0].RevokedAt != nil {
		t.Fatal("oldest token was revoked")
	}

	sum := sha256.Sum256([]byte(plaintext))
	if string(repo.rows[4].TokenHash) != string(sum[:]) {
		t.Fatal("hash is not SHA-256 of the full muster_ token")
	}
	rawOnly := plaintext[len("muster_"):]
	rawSum := sha256.Sum256([]byte(rawOnly))
	if string(repo.rows[4].TokenHash) == string(rawSum[:]) {
		t.Fatal("hash ignored the prefix")
	}
	if string(repo.rows[4].TokenHash) == plaintext {
		t.Fatal("stored the plaintext")
	}

	active, err := store.GetActive(plaintext)
	if err != nil || active == nil {
		t.Fatal("expected the new token to be active")
	}

	revokedAt := now
	repo.rows[4].RevokedAt = &revokedAt
	if row, err := store.GetActive(plaintext); err != nil || row != nil {
		t.Fatal("revoked token was accepted")
	}
	repo.rows[4].RevokedAt = nil
	repo.rows[4].ExpiresAt = now.Add(-time.Minute)
	if row, err := store.GetActive(plaintext); err != nil || row != nil {
		t.Fatal("expired token was accepted")
	}
	if row, err := store.GetActive("muster_unknown"); err != nil || row != nil {
		t.Fatal("unknown token was accepted")
	}
}
