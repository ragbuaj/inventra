package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const pwResetPrefix = "auth:pwreset:" // hashed reset token -> userID

// ErrResetNotFound is returned when a reset token is unknown, expired, or already used.
var ErrResetNotFound = errors.New("password reset token not found")

// GenerateResetToken returns a URL-safe random token and its storage hash.
func GenerateResetToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashResetToken(raw), nil
}

// HashResetToken returns the hex SHA-256 of a raw token (what we store at rest).
func HashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// SavePasswordReset stores a single-use reset token hash for the user with a TTL.
func (s *TokenStore) SavePasswordReset(ctx context.Context, hash, userID string, ttl time.Duration) error {
	return s.rdb.Set(ctx, pwResetPrefix+hash, userID, ttl).Err()
}

// ConsumePasswordReset atomically reads and deletes a reset token (single use).
func (s *TokenStore) ConsumePasswordReset(ctx context.Context, hash string) (string, error) {
	userID, err := s.rdb.GetDel(ctx, pwResetPrefix+hash).Result()
	if err != nil {
		return "", ErrResetNotFound
	}
	return userID, nil
}
