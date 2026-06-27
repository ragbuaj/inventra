package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrStateInvalid means the OAuth state was unknown, expired, or already used.
var ErrStateInvalid = errors.New("invalid or expired oauth state")

// errMissing is the sentinel a kv returns when a key is absent.
var errMissing = errors.New("kv: missing")

const statePrefix = "oauth:state:"

// kv is the minimal Redis surface the state store needs (seam for tests).
type kv interface {
	Set(ctx context.Context, key, val string, ttl time.Duration) error
	GetDel(ctx context.Context, key string) (string, error)
}

// redisKV adapts *redis.Client to kv. GetDel atomically reads and deletes.
type redisKV struct{ rdb *redis.Client }

func (r redisKV) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	return r.rdb.Set(ctx, key, val, ttl).Err()
}

func (r redisKV) GetDel(ctx context.Context, key string) (string, error) {
	v, err := r.rdb.GetDel(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", errMissing
	}
	return v, err
}

// stateStore persists state -> PKCE verifier for single use.
type stateStore struct {
	kv  kv
	ttl time.Duration
}

func (s *stateStore) Save(ctx context.Context, state, verifier string) error {
	return s.kv.Set(ctx, statePrefix+state, verifier, s.ttl)
}

// Consume returns the PKCE verifier for state and deletes it (single use).
func (s *stateStore) Consume(ctx context.Context, state string) (string, error) {
	v, err := s.kv.GetDel(ctx, statePrefix+state)
	if err != nil {
		return "", ErrStateInvalid
	}
	return v, nil
}

// randToken returns a URL-safe random string from nBytes of entropy.
func randToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
