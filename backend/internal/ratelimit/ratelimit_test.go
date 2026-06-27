package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis_rate/v10"
)

type fakeAllower struct {
	res *redis_rate.Result
	err error
}

func (f *fakeAllower) Allow(_ context.Context, _ string, _ redis_rate.Limit) (*redis_rate.Result, error) {
	return f.res, f.err
}

func newTestLimiter(f *fakeAllower, enabled bool) *Limiter {
	return &Limiter{rl: f, backstop: newBackstop(100), timeout: 50 * time.Millisecond, enabled: enabled}
}

func TestAllowMapsRedisAllowed(t *testing.T) {
	f := &fakeAllower{res: &redis_rate.Result{Allowed: 1, Remaining: 4, RetryAfter: -1, ResetAfter: 12 * time.Second}}
	got := newTestLimiter(f, true).Allow(context.Background(), "global:ip:1.2.3.4", 5, false)
	if !got.Allowed || got.Remaining != 4 || got.ResetAfter != 12*time.Second || got.Limit != 5 {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.RetryAfter != -1 {
		t.Fatalf("RetryAfter not copied: %v", got.RetryAfter)
	}
}

func TestAllowMapsRedisDenied(t *testing.T) {
	f := &fakeAllower{res: &redis_rate.Result{Allowed: 0, Remaining: 0, RetryAfter: 3 * time.Second}}
	got := newTestLimiter(f, true).Allow(context.Background(), "auth:ip:1.2.3.4", 5, true)
	if got.Allowed {
		t.Fatal("Allowed=0 must map to denied")
	}
	if got.RetryAfter != 3*time.Second {
		t.Fatalf("RetryAfter: %v", got.RetryAfter)
	}
}

func TestAllowFailOpenWithoutBackstop(t *testing.T) {
	f := &fakeAllower{err: errors.New("redis down")}
	got := newTestLimiter(f, true).Allow(context.Background(), "global:ip:1.2.3.4", 5, false)
	if !got.Allowed || !got.Degraded {
		t.Fatalf("redis error w/o backstop must fail open + degraded: %+v", got)
	}
}

func TestAllowUsesBackstopOnRedisError(t *testing.T) {
	f := &fakeAllower{err: errors.New("redis down")}
	l := newTestLimiter(f, true)
	if !l.Allow(context.Background(), "auth:ip:9", 2, true).Allowed {
		t.Fatal("1st backstop allow should pass")
	}
	l.Allow(context.Background(), "auth:ip:9", 2, true)
	if l.Allow(context.Background(), "auth:ip:9", 2, true).Allowed {
		t.Fatal("backstop must deny after the burst is drained")
	}
}

func TestAllowDisabledAlwaysAllows(t *testing.T) {
	f := &fakeAllower{err: errors.New("must not matter")}
	if !newTestLimiter(f, false).Allow(context.Background(), "x:y", 5, true).Allowed {
		t.Fatal("disabled limiter must always allow")
	}
}

func TestAllowPerMinZeroAlwaysAllows(t *testing.T) {
	f := &fakeAllower{err: errors.New("must not matter")}
	if !newTestLimiter(f, true).Allow(context.Background(), "x:y", 0, false).Allowed {
		t.Fatal("perMin<=0 must always allow without calling Redis")
	}
}

func TestKeyPrefixHidesValue(t *testing.T) {
	if got := keyPrefix("login:acct:user@example.com"); got != "login:acct" {
		t.Fatalf("key prefix leaked the value: %q", got)
	}
}
