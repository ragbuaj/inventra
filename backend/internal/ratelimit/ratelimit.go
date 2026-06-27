// Package ratelimit enforces request rate limits using Redis (GCRA via
// redis_rate), with a bounded in-memory backstop on auth paths and fail-open
// behaviour when Redis is unavailable (ADR-0004).
package ratelimit

import (
	"context"
	"strings"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/logging"
)

// Result is the outcome of a limit check.
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
	ResetAfter time.Duration
	Degraded   bool
}

// Allower is the behaviour consumers depend on (satisfied by *Limiter).
type Allower interface {
	Allow(ctx context.Context, key string, perMin int, withBackstop bool) Result
}

// redisAllower is the seam over redis_rate for testability.
type redisAllower interface {
	Allow(ctx context.Context, key string, limit redis_rate.Limit) (*redis_rate.Result, error)
}

// Limiter enforces limits via Redis with an in-memory backstop fallback.
type Limiter struct {
	rl       redisAllower
	backstop *backstop
	timeout  time.Duration
	enabled  bool
}

// New builds a Limiter from config and starts the backstop janitor.
func New(rdb *redis.Client, cfg *config.Config) *Limiter {
	l := &Limiter{
		rl:       redis_rate.NewLimiter(rdb),
		backstop: newBackstop(10000),
		timeout:  time.Duration(cfg.RateLimitTimeoutMS) * time.Millisecond,
		enabled:  cfg.RateLimitEnabled,
	}
	go l.runJanitor()
	return l
}

func (l *Limiter) runJanitor() {
	t := time.NewTicker(5 * time.Minute)
	for range t.C {
		l.backstop.evictIdle(10 * time.Minute)
	}
}

// Allow checks key against perMin. On Redis error it logs a degraded warning and
// either consults the backstop (withBackstop) or fails open. Disabled → allow.
func (l *Limiter) Allow(ctx context.Context, key string, perMin int, withBackstop bool) Result {
	if !l.enabled || perMin <= 0 {
		return Result{Allowed: true, Limit: perMin}
	}
	cctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	res, err := l.rl.Allow(cctx, key, redis_rate.PerMinute(perMin))
	if err != nil {
		logging.FromContext(ctx).Warn("rate limiter degraded", "key_prefix", keyPrefix(key), "error", err)
		if withBackstop {
			return Result{Allowed: l.backstop.Allow(key, perMin), Limit: perMin, Degraded: true}
		}
		return Result{Allowed: true, Limit: perMin, Degraded: true}
	}
	return Result{
		Allowed:    res.Allowed > 0,
		Limit:      perMin,
		Remaining:  res.Remaining,
		RetryAfter: res.RetryAfter,
		ResetAfter: res.ResetAfter,
	}
}

// keyPrefix returns everything up to the last ':' so an embedded account/IP value
// is never logged.
func keyPrefix(key string) string {
	if i := strings.LastIndex(key, ":"); i >= 0 {
		return key[:i]
	}
	return key
}
