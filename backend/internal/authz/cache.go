// Package authz implements the configurable authorization layer:
// per-action RBAC, per-row data scoping, and per-field permissions.
// Lookups are cached in Redis (complementary; a cache miss falls back to Postgres).
package authz

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 10 * time.Minute

// cacheGetJSON loads a JSON value from Redis. Returns false on miss or any error
// (callers then read from Postgres — Redis is never the source of truth).
func cacheGetJSON[T any](ctx context.Context, rdb *redis.Client, key string, out *T) bool {
	b, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(b, out) == nil
}

// cacheSetJSON stores a JSON value in Redis with a TTL (best-effort).
func cacheSetJSON(ctx context.Context, rdb *redis.Client, key string, v any, ttl time.Duration) {
	if b, err := json.Marshal(v); err == nil {
		_ = rdb.Set(ctx, key, b, ttl).Err()
	}
}
