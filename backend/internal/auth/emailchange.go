package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const emailChangePrefix = "auth:emailchange:" // hashed email-change token -> {userID, newEmail}

// ErrEmailChangeNotFound is returned when an email-change token is unknown, expired, or already used.
var ErrEmailChangeNotFound = errors.New("email change token not found")

// GenerateEmailChangeToken returns a URL-safe random token and its storage hash.
func GenerateEmailChangeToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashEmailChangeToken(raw), nil
}

// HashEmailChangeToken returns the hex SHA-256 of a raw token (what we store at rest).
func HashEmailChangeToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type emailChangePayload struct {
	UserID   string `json:"user_id"`
	NewEmail string `json:"new_email"`
}

// SaveEmailChange stores a single-use email-change token hash for the user with a TTL.
func (s *TokenStore) SaveEmailChange(ctx context.Context, hash, userID, newEmail string, ttl time.Duration) error {
	b, err := json.Marshal(emailChangePayload{UserID: userID, NewEmail: newEmail})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, emailChangePrefix+hash, b, ttl).Err()
}

// ConsumeEmailChange atomically reads and deletes an email-change token (single use).
func (s *TokenStore) ConsumeEmailChange(ctx context.Context, hash string) (string, string, error) {
	v, err := s.rdb.GetDel(ctx, emailChangePrefix+hash).Result()
	if err != nil {
		return "", "", ErrEmailChangeNotFound
	}
	var p emailChangePayload
	if err := json.Unmarshal([]byte(v), &p); err != nil {
		return "", "", ErrEmailChangeNotFound
	}
	return p.UserID, p.NewEmail, nil
}
