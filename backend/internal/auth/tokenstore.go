package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key prefixes for token state.
const (
	refreshPrefix  = "auth:refresh:"  // whitelist of valid refresh JTIs
	denylistPrefix = "auth:denylist:" // revoked access JTIs (logout)
)

// TokenStore tracks refresh tokens (whitelist) and revoked access tokens (denylist) in Redis.
type TokenStore struct {
	rdb *redis.Client
}

// NewTokenStore builds a TokenStore over the given Redis client.
func NewTokenStore(rdb *redis.Client) *TokenStore {
	return &TokenStore{rdb: rdb}
}

// SaveRefresh records a valid refresh JTI for the user with the given TTL.
func (s *TokenStore) SaveRefresh(ctx context.Context, jti, userID string, ttl time.Duration) error {
	return s.rdb.Set(ctx, refreshPrefix+jti, userID, ttl).Err()
}

// RefreshValid reports whether the refresh JTI is still whitelisted.
func (s *TokenStore) RefreshValid(ctx context.Context, jti string) (bool, error) {
	n, err := s.rdb.Exists(ctx, refreshPrefix+jti).Result()
	return n > 0, err
}

// DeleteRefresh removes a refresh JTI from the whitelist (rotation/logout).
func (s *TokenStore) DeleteRefresh(ctx context.Context, jti string) error {
	return s.rdb.Del(ctx, refreshPrefix+jti).Err()
}

// DenyAccess revokes an access JTI until its natural expiry.
func (s *TokenStore) DenyAccess(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return s.rdb.Set(ctx, denylistPrefix+jti, "1", ttl).Err()
}

// AccessDenied reports whether the access JTI has been revoked.
func (s *TokenStore) AccessDenied(ctx context.Context, jti string) (bool, error) {
	n, err := s.rdb.Exists(ctx, denylistPrefix+jti).Result()
	return n > 0, err
}
