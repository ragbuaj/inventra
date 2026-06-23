// Package cache provides the Redis client (caching, sessions, rate limiting).
package cache

import (
	"context"
	"time"

	"github.com/ragbuaj/inventra/internal/config"
	"github.com/redis/go-redis/v9"
)

// NewClient builds a Redis client from configuration. Connections are lazy;
// use Ping to verify connectivity.
func NewClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}

// Ping verifies Redis is reachable within the given timeout.
func Ping(ctx context.Context, rdb *redis.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return rdb.Ping(ctx).Err()
}
