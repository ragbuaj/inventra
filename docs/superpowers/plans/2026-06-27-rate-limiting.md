# Rate Limiting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Redis-backed rate limiting (anti-brute-force on auth + global per-IP throttle) with fail-open and an in-memory backstop, standard `429`/`Retry-After`/`RateLimit-*` responses, and limit-hit logging.

**Architecture:** A new `internal/ratelimit` package wraps `redis_rate` (GCRA) behind an `Allower` interface, with a bounded in-memory `x/time/rate` backstop used only when Redis fails on auth paths. A gin `PerIP` middleware enforces global + auth IP limits; the login handler adds an account-keyed (IP+email) check post body-parse. Bands come from env (ADR-0003). Limit hits log via the logging layer (ADR-0002).

**Tech Stack:** Go 1.25, `github.com/go-redis/redis_rate/v10`, `golang.org/x/time/rate`, `go-redis/v9` (existing), Gin, `log/slog`.

## Global Constraints

- New deps: `github.com/go-redis/redis_rate/v10`, `golang.org/x/time/rate` (`go get` + `go mod tidy`). No new services (reuse Redis).
- Bands via env, defaults: `RATELIMIT_ENABLED=true`, `RATELIMIT_TIMEOUT_MS=50`, `RATELIMIT_GLOBAL_PER_MIN=120` (per IP), `RATELIMIT_LOGIN_PER_MIN=5` (per IP+account), `RATELIMIT_LOGIN_IP_PER_MIN=20` (per IP), `RATELIMIT_REFRESH_PER_MIN=30` (per IP).
- On Redis error/timeout: log `warn` via `logging.FromContext` and fail open; on **auth** paths consult the in-memory backstop first. Disabled (`enabled=false`) → always allow.
- **Never log credentials/email**: log only the non-sensitive key prefix (e.g. `login:acct`, `auth:ip`, `global`) + IP — never the full key or the email.
- Deny response: `429 {"error":"too many requests"}` + `Retry-After: <ceil seconds, ≥1>` + `RateLimit-Limit`/`RateLimit-Remaining`/`RateLimit-Reset`.
- Consumers depend on `ratelimit.Allower` (interface), not the concrete `*Limiter`, so tests inject fakes.
- Limiting must NOT break the CI e2e (it logs in repeatedly as one admin from one IP): compose limit is env-overridable (default on) and the CI e2e job disables it.
- Backend gate: `go build ./... && go vet ./... && go test ./...` + Spectral lint of `backend/api/openapi.yaml`. Commits: Conventional Commits with scope, no AI/co-author trailers. Branch: `feat/rate-limiting` (already checked out).

---

### Task 1: Config fields

**Files:**
- Modify: `backend/internal/config/config.go` (Config fields + Load mapping)
- Modify: `backend/.env.example`
- Test: `backend/internal/config/config_test.go` (append)

**Interfaces:**
- Produces: `config.Config.{RateLimitEnabled bool, RateLimitTimeoutMS int, RateLimitGlobalPerMin int, RateLimitLoginPerMin int, RateLimitLoginIPPerMin int, RateLimitRefreshPerMin int}`.

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/config/config_test.go`:

```go
func TestLoadRateLimitDefaults(t *testing.T) {
	t.Setenv("RATELIMIT_ENABLED", "")
	t.Setenv("RATELIMIT_GLOBAL_PER_MIN", "")
	cfg := Load()
	if !cfg.RateLimitEnabled {
		t.Fatal("RateLimitEnabled default should be true")
	}
	if cfg.RateLimitGlobalPerMin != 120 || cfg.RateLimitLoginPerMin != 5 ||
		cfg.RateLimitLoginIPPerMin != 20 || cfg.RateLimitRefreshPerMin != 30 || cfg.RateLimitTimeoutMS != 50 {
		t.Fatalf("unexpected rate-limit defaults: %+v", cfg)
	}
}

func TestLoadRateLimitFromEnv(t *testing.T) {
	t.Setenv("RATELIMIT_ENABLED", "false")
	t.Setenv("RATELIMIT_LOGIN_PER_MIN", "9")
	cfg := Load()
	if cfg.RateLimitEnabled {
		t.Fatal("RATELIMIT_ENABLED=false not applied")
	}
	if cfg.RateLimitLoginPerMin != 9 {
		t.Fatalf("RATELIMIT_LOGIN_PER_MIN: %d", cfg.RateLimitLoginPerMin)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/config/`
Expected: FAIL — Config has no `RateLimit*` fields (won't compile).

- [ ] **Step 3: Add the config fields**

In `backend/internal/config/config.go`, add to the `Config` struct:

```go
	// Rate limiting (ADR-0004).
	RateLimitEnabled       bool
	RateLimitTimeoutMS     int
	RateLimitGlobalPerMin  int
	RateLimitLoginPerMin   int
	RateLimitLoginIPPerMin int
	RateLimitRefreshPerMin int
```

And in `Load()` (with the other assignments):

```go
		RateLimitEnabled:       getEnvBool("RATELIMIT_ENABLED", true),
		RateLimitTimeoutMS:     getEnvInt("RATELIMIT_TIMEOUT_MS", 50),
		RateLimitGlobalPerMin:  getEnvInt("RATELIMIT_GLOBAL_PER_MIN", 120),
		RateLimitLoginPerMin:   getEnvInt("RATELIMIT_LOGIN_PER_MIN", 5),
		RateLimitLoginIPPerMin: getEnvInt("RATELIMIT_LOGIN_IP_PER_MIN", 20),
		RateLimitRefreshPerMin: getEnvInt("RATELIMIT_REFRESH_PER_MIN", 30),
```

- [ ] **Step 4: Append env example**

Append to `backend/.env.example`:

```
# Rate limiting (ADR-0004) — ⚠️ placeholder bands; tune per bank policy
RATELIMIT_ENABLED=true
RATELIMIT_TIMEOUT_MS=50
RATELIMIT_GLOBAL_PER_MIN=120
RATELIMIT_LOGIN_PER_MIN=5
RATELIMIT_LOGIN_IP_PER_MIN=20
RATELIMIT_REFRESH_PER_MIN=30
```

- [ ] **Step 5: Run tests + build + commit**

Run: `cd backend && go test ./internal/config/ && go build ./... && go vet ./...`
Expected: PASS, clean.

```bash
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/.env.example
git commit -m "feat(security): rate-limit config bands (ADR-0004)"
```

---

### Task 2: In-memory backstop

**Files:**
- Create: `backend/internal/ratelimit/backstop.go`
- Test: `backend/internal/ratelimit/backstop_test.go`

**Interfaces:**
- Produces (package-internal): `newBackstop(max int) *backstop`; `(*backstop).Allow(key string, perMin int) bool`; `(*backstop).evictIdle(idle time.Duration)`; `(*backstop).len() int`; field `now func() time.Time` (injectable for tests).

- [ ] **Step 1: Add the dependency**

Run: `cd backend && go get golang.org/x/time/rate`
Expected: go.mod/go.sum updated.

- [ ] **Step 2: Write the failing test**

Create `backend/internal/ratelimit/backstop_test.go`:

```go
package ratelimit

import (
	"testing"
	"time"
)

func TestBackstopAllowsWithinBurst(t *testing.T) {
	b := newBackstop(100)
	for i := 0; i < 60; i++ {
		if !b.Allow("k", 60) {
			t.Fatalf("call %d should pass within burst of 60", i)
		}
	}
	if b.Allow("k", 60) {
		t.Fatal("61st immediate call should be denied (bucket drained)")
	}
}

func TestBackstopKeysIndependent(t *testing.T) {
	b := newBackstop(100)
	for i := 0; i < 60; i++ {
		b.Allow("a", 60)
	}
	if !b.Allow("b", 60) {
		t.Fatal("a different key must have its own bucket")
	}
}

func TestBackstopCapFailsOpen(t *testing.T) {
	b := newBackstop(2)
	b.Allow("a", 60)
	b.Allow("b", 60)
	if !b.Allow("c", 60) {
		t.Fatal("above the cap a new key must fail open (true)")
	}
	if b.len() != 2 {
		t.Fatalf("cap must bound the map at 2, got %d", b.len())
	}
}

func TestBackstopEvictsIdle(t *testing.T) {
	b := newBackstop(100)
	cur := time.Unix(1000, 0)
	b.now = func() time.Time { return cur }
	b.Allow("k", 60)
	cur = cur.Add(20 * time.Minute)
	b.evictIdle(10 * time.Minute)
	if b.len() != 0 {
		t.Fatalf("idle entry should be evicted, len=%d", b.len())
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd backend && go test ./internal/ratelimit/`
Expected: FAIL — package/`newBackstop` undefined.

- [ ] **Step 4: Implement the backstop**

Create `backend/internal/ratelimit/backstop.go`:

```go
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type bsEntry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// backstop is a per-instance, per-key token-bucket fallback consulted only when
// Redis is unavailable on auth paths. It is bounded by a hard cap plus idle
// eviction so it cannot grow without limit during an outage.
type backstop struct {
	mu      sync.Mutex
	buckets map[string]*bsEntry
	max     int
	now     func() time.Time
}

func newBackstop(max int) *backstop {
	return &backstop{buckets: make(map[string]*bsEntry), max: max, now: time.Now}
}

// Allow reports whether key is within perMin on this instance. Above the cap it
// returns true (fail-open) rather than allocating a new bucket.
func (b *backstop) Allow(key string, perMin int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.buckets[key]
	if !ok {
		if len(b.buckets) >= b.max {
			return true
		}
		e = &bsEntry{lim: rate.NewLimiter(rate.Limit(float64(perMin)/60.0), perMin)}
		b.buckets[key] = e
	}
	e.lastSeen = b.now()
	return e.lim.Allow()
}

// evictIdle removes entries unused for longer than idle.
func (b *backstop) evictIdle(idle time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	cutoff := b.now().Add(-idle)
	for k, e := range b.buckets {
		if e.lastSeen.Before(cutoff) {
			delete(b.buckets, k)
		}
	}
}

func (b *backstop) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buckets)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/ratelimit/`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ratelimit/backstop.go backend/internal/ratelimit/backstop_test.go backend/go.mod backend/go.sum
git commit -m "feat(security): bounded in-memory rate-limit backstop"
```

---

### Task 3: Limiter (redis_rate wrapper) + Allower interface

**Files:**
- Create: `backend/internal/ratelimit/ratelimit.go`
- Test: `backend/internal/ratelimit/ratelimit_test.go`

**Interfaces:**
- Consumes: `newBackstop` (Task 2); `config.Config.RateLimit*` (Task 1); `logging.FromContext`.
- Produces: `ratelimit.Result{Allowed bool, Limit int, Remaining int, RetryAfter time.Duration, ResetAfter time.Duration, Degraded bool}`; `ratelimit.Allower` interface (`Allow(ctx, key string, perMin int, withBackstop bool) Result`); `ratelimit.New(rdb *redis.Client, cfg *config.Config) *Limiter`; `*Limiter` satisfies `Allower`.

- [ ] **Step 1: Add the dependency**

Run: `cd backend && go get github.com/go-redis/redis_rate/v10`
Expected: go.mod/go.sum updated.

- [ ] **Step 2: Write the failing test**

Create `backend/internal/ratelimit/ratelimit_test.go`:

```go
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

func TestKeyPrefixHidesValue(t *testing.T) {
	if got := keyPrefix("login:acct:user@example.com"); got != "login:acct" {
		t.Fatalf("key prefix leaked the value: %q", got)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd backend && go test ./internal/ratelimit/ -run Allow`
Expected: FAIL — `Limiter`/`keyPrefix` undefined.

- [ ] **Step 4: Implement the limiter**

Create `backend/internal/ratelimit/ratelimit.go`:

```go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/ratelimit/`
Expected: PASS (all backstop + limiter tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ratelimit/ratelimit.go backend/internal/ratelimit/ratelimit_test.go backend/go.mod backend/go.sum
git commit -m "feat(security): redis_rate limiter with fail-open + backstop (Allower)"
```

---

### Task 4: PerIP middleware + shared 429 helpers

**Files:**
- Create: `backend/internal/middleware/ratelimit.go`
- Test: `backend/internal/middleware/ratelimit_test.go`

**Interfaces:**
- Consumes: `ratelimit.Allower`, `ratelimit.Result` (Task 3); `logging.FromContext`.
- Produces: `middleware.PerIP(l ratelimit.Allower, perMin int, prefix string, withBackstop bool) gin.HandlerFunc`; `middleware.SetRateLimitHeaders(c *gin.Context, res ratelimit.Result)`; `middleware.WriteRateLimited(c *gin.Context, res ratelimit.Result)`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/middleware/ratelimit_test.go`:

```go
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/ratelimit"
)

type captureLimiter struct {
	res     ratelimit.Result
	lastKey string
}

func (f *captureLimiter) Allow(_ context.Context, key string, _ int, _ bool) ratelimit.Result {
	f.lastKey = key
	return f.res
}

func TestPerIPUnderLimitSetsHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := &captureLimiter{res: ratelimit.Result{Allowed: true, Limit: 5, Remaining: 4, ResetAfter: 30 * time.Second}}
	r := gin.New()
	r.Use(PerIP(l, 5, "global", false))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("under limit status: %d", w.Code)
	}
	if w.Header().Get("RateLimit-Limit") != "5" || w.Header().Get("RateLimit-Remaining") != "4" {
		t.Fatalf("RateLimit headers: %v", w.Header())
	}
}

func TestPerIPOverLimitReturns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := &captureLimiter{res: ratelimit.Result{Allowed: false, Limit: 5, RetryAfter: 3 * time.Second}}
	r := gin.New()
	r.Use(PerIP(l, 5, "auth", true))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over limit status: %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "3" {
		t.Fatalf("Retry-After: %q", w.Header().Get("Retry-After"))
	}
	if !strings.Contains(w.Body.String(), "too many requests") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestPerIPKeyIncludesClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := &captureLimiter{res: ratelimit.Result{Allowed: true, Limit: 5}}
	r := gin.New()
	r.Use(PerIP(l, 5, "global", false))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "203.0.113.7:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if l.lastKey != "global:ip:203.0.113.7" {
		t.Fatalf("key did not include client IP: %q", l.lastKey)
	}
}

func TestWriteRateLimitedFloorsRetryAfterToOne(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		WriteRateLimited(c, ratelimit.Result{Limit: 5, RetryAfter: 0})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After floor: %q", w.Header().Get("Retry-After"))
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/middleware/ -run "PerIP|WriteRateLimited"`
Expected: FAIL — `PerIP`/`WriteRateLimited` undefined.

- [ ] **Step 3: Implement the middleware + helpers**

Create `backend/internal/middleware/ratelimit.go`:

```go
package middleware

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/logging"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// PerIP limits requests per client IP. prefix namespaces the key; withBackstop
// enables the in-memory fallback (auth paths). It sets RateLimit-* on allow and
// returns 429 (+ Retry-After) on deny.
func PerIP(l ratelimit.Allower, perMin int, prefix string, withBackstop bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := prefix + ":ip:" + c.ClientIP()
		res := l.Allow(c.Request.Context(), key, perMin, withBackstop)
		if !res.Allowed {
			logging.FromContext(c.Request.Context()).Warn("rate limit exceeded", "prefix", prefix, "ip", c.ClientIP())
			WriteRateLimited(c, res)
			return
		}
		SetRateLimitHeaders(c, res)
		c.Next()
	}
}

// SetRateLimitHeaders writes the IETF draft RateLimit-* response headers.
func SetRateLimitHeaders(c *gin.Context, res ratelimit.Result) {
	h := c.Writer.Header()
	h.Set("RateLimit-Limit", strconv.Itoa(res.Limit))
	remaining := res.Remaining
	if remaining < 0 {
		remaining = 0
	}
	h.Set("RateLimit-Remaining", strconv.Itoa(remaining))
	h.Set("RateLimit-Reset", strconv.Itoa(int(math.Ceil(res.ResetAfter.Seconds()))))
}

// WriteRateLimited aborts with 429 + Retry-After (≥1s) + RateLimit-* headers.
// Shared by PerIP and the login account-key check so the 429 shape is identical.
func WriteRateLimited(c *gin.Context, res ratelimit.Result) {
	SetRateLimitHeaders(c, res)
	retry := int(math.Ceil(res.RetryAfter.Seconds()))
	if retry < 1 {
		retry = 1
	}
	c.Writer.Header().Set("Retry-After", strconv.Itoa(retry))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/middleware/ -run "PerIP|WriteRateLimited"`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/ratelimit.go backend/internal/middleware/ratelimit_test.go
git commit -m "feat(security): PerIP rate-limit middleware + 429 helpers"
```

---

### Task 5: Wiring — login account-key, auth IP-limits, global throttle, CI env

**Files:**
- Modify: `backend/internal/identity/handler.go` (Handler limiter field + `NewHandler` signature + login account-key)
- Modify: `backend/internal/identity/routes.go` (`RegisterRoutes` mounts auth IP-limit middleware)
- Modify: `backend/internal/server/router.go` (`Deps.Limiter`; global throttle on `/api/v1`; pass limiter + bands to identity)
- Modify: `backend/internal/server/router_test.go` (give Deps a Limiter; add `/api/v1` throttle-mounted test)
- Modify: `backend/cmd/api/main.go` (build `ratelimit.New`; pass `Limiter`)
- Modify: `backend/api/openapi.yaml` (429 response on `/auth/login`, `/auth/refresh`)
- Modify: `docker-compose.yml` (backend env: env-overridable `RATELIMIT_ENABLED`)
- Modify: `.github/workflows/ci.yml` (e2e job disables rate limiting)
- Test: `backend/internal/identity/ratelimit_test.go`

**Interfaces:**
- Consumes: `ratelimit.New`, `ratelimit.Allower` (Task 3); `middleware.PerIP`, `middleware.WriteRateLimited` (Task 4); `config.RateLimit*` (Task 1).
- Produces: `server.Deps.Limiter *ratelimit.Limiter`; `identity.NewHandler(svc, perms, scopes, limiter ratelimit.Allower, loginPerMin int)`; `identity.RegisterRoutes(rg, h, authMW, limiter ratelimit.Allower, loginIPPerMin, refreshPerMin int)`.

- [ ] **Step 1: Write the failing test (login account-key)**

Create `backend/internal/identity/ratelimit_test.go`:

```go
package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/ratelimit"
)

type denyLimiter struct{}

func (denyLimiter) Allow(_ context.Context, _ string, _ int, _ bool) ratelimit.Result {
	return ratelimit.Result{Allowed: false, Limit: 5}
}

// When the account-key limit denies, login must return 429 before touching the
// service (svc is nil here — it must not be reached).
func TestLoginAccountKeyRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{limiter: denyLimiter{}, loginPerMin: 5}
	r := gin.New()
	r.POST("/auth/login", h.login)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"a@b.com","password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d (body %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "too many requests") {
		t.Fatalf("body: %s", w.Body.String())
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/identity/ -run RateLimited`
Expected: FAIL — `Handler` has no `limiter`/`loginPerMin` fields.

- [ ] **Step 3: Update the identity Handler**

In `backend/internal/identity/handler.go`:

(a) add the imports `"strings"` and the ratelimit + middleware packages (keep existing imports):

```go
	"strings"

	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/ratelimit"
```
(`middleware` is already imported — do not duplicate.)

(b) extend the struct and constructor:

```go
type Handler struct {
	svc         *Service
	perms       *authz.PermissionService
	scopes      *authz.ScopeService
	limiter     ratelimit.Allower
	loginPerMin int
}

func NewHandler(svc *Service, perms *authz.PermissionService, scopes *authz.ScopeService, limiter ratelimit.Allower, loginPerMin int) *Handler {
	return &Handler{svc: svc, perms: perms, scopes: scopes, limiter: limiter, loginPerMin: loginPerMin}
}
```

(c) add the account-key check at the top of `login`, right after the bind:

```go
func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	key := "login:acct:" + strings.ToLower(strings.TrimSpace(req.Email))
	if res := h.limiter.Allow(c.Request.Context(), key, h.loginPerMin, true); !res.Allowed {
		middleware.WriteRateLimited(c, res)
		return
	}
	pair, _, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.authError(c, err)
		return
	}
	c.JSON(http.StatusOK, newTokenResponse(pair))
}
```

- [ ] **Step 4: Run the identity test to verify it passes**

Run: `cd backend && go test ./internal/identity/ -run RateLimited`
Expected: PASS. (Build of `./internal/server` will fail until Step 6 updates the `NewHandler` call — that's next.)

- [ ] **Step 5: Mount auth IP-limits in identity routes**

Replace `backend/internal/identity/routes.go` with:

```go
package identity

import (
	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// RegisterRoutes mounts the identity endpoints. authMW protects authed routes;
// the limiter applies per-IP throttles on the unauthenticated auth endpoints.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc, limiter ratelimit.Allower, loginIPPerMin, refreshPerMin int) {
	grp := rg.Group("/auth")
	grp.POST("/login", middleware.PerIP(limiter, loginIPPerMin, "auth_login", true), h.login)
	grp.POST("/refresh", middleware.PerIP(limiter, refreshPerMin, "auth_refresh", true), h.refresh)

	authed := grp.Group("")
	authed.Use(authMW)
	authed.POST("/logout", h.logout)
	authed.GET("/me", h.me)
	authed.GET("/permissions", h.permissions)
	authed.GET("/scope/:module", h.scope)
}
```

- [ ] **Step 6: Wire the limiter into the router**

In `backend/internal/server/router.go`:

(a) add imports `"github.com/ragbuaj/inventra/internal/ratelimit"` (and ensure `middleware` is imported — it is).

(b) add the field to `Deps`:

```go
	Limiter *ratelimit.Limiter
```

(c) mount the global throttle on the `/api/v1` group — change the `api := r.Group("/api/v1")` line so the group uses the middleware:

```go
	api := r.Group("/api/v1")
	api.Use(middleware.PerIP(d.Limiter, d.Cfg.RateLimitGlobalPerMin, "global", false))
	{
```

(d) update the identity construction + registration:

```go
		identityHandler := identity.NewHandler(identitySvc, permSvc, scopeSvc, d.Limiter, d.Cfg.RateLimitLoginPerMin)
		identity.RegisterRoutes(api, identityHandler, requireAuth, d.Limiter, d.Cfg.RateLimitLoginIPPerMin, d.Cfg.RateLimitRefreshPerMin)
```

- [ ] **Step 7: Update the router test (Deps now needs a Limiter)**

In `backend/internal/server/router_test.go`, update the existing `TestRouterHealthEchoesRequestID` to give `Deps` a limiter, and add a throttle-mounted test. Replace the file's test body so both tests construct a limiter (add the `ratelimit` import):

```go
import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

func testDeps() Deps {
	cfg := &config.Config{Env: "test", RateLimitEnabled: true, RateLimitTimeoutMS: 50, RateLimitGlobalPerMin: 120}
	return Deps{Cfg: cfg, Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Limiter: ratelimit.New(nil, cfg)}
}

func TestRouterHealthEchoesRequestID(t *testing.T) {
	r := NewRouter(testDeps())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/health status: %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("router did not echo X-Request-ID")
	}
}

// The global throttle is mounted on /api/v1; with a nil Redis client the limiter
// fails open, so a single /api/v1/health request is allowed (200) — proving the
// middleware is mounted without breaking the request.
func TestRouterGlobalThrottleMounted(t *testing.T) {
	r := NewRouter(testDeps())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/api/v1/health status: %d", w.Code)
	}
}
```

- [ ] **Step 8: Build the limiter in main.go**

In `backend/cmd/api/main.go`, add the import `"github.com/ragbuaj/inventra/internal/ratelimit"`, build the limiter after Redis is set up, and pass it in `Deps`:

```go
	limiter := ratelimit.New(rdb, cfg)
```
and change the `server.NewRouter(server.Deps{...})` call to include `Limiter: limiter`:

```go
		Handler:           server.NewRouter(server.Deps{Cfg: cfg, Pool: pool, Redis: rdb, Log: logger, Limiter: limiter}),
```

- [ ] **Step 9: Build, vet, full test**

Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: all pass (config, ratelimit, middleware, identity, server).

- [ ] **Step 10: Document the 429 in OpenAPI**

In `backend/api/openapi.yaml`, add a reusable response under `components.responses` and reference it from `POST /auth/login` and `POST /auth/refresh`. Add:

```yaml
    TooManyRequests:
      description: Rate limit exceeded.
      headers:
        Retry-After:
          schema: { type: integer }
          description: Seconds until the client may retry.
        RateLimit-Limit:
          schema: { type: integer }
        RateLimit-Remaining:
          schema: { type: integer }
        RateLimit-Reset:
          schema: { type: integer }
      content:
        application/json:
          schema:
            type: object
            properties:
              error: { type: string }
```

Then under the `responses:` of the `/auth/login` and `/auth/refresh` POST operations add:

```yaml
        '429':
          $ref: '#/components/responses/TooManyRequests'
```

Run: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors (warnings tolerated only if pre-existing).

- [ ] **Step 11: Keep rate limiting from breaking the CI e2e**

The e2e logs in repeatedly as one admin from one IP; the default bands would 429 it. Make the compose limit env-overridable (default on) and disable it for the e2e job.

In `docker-compose.yml`, under the `backend` service `environment:` block, add:

```yaml
      RATELIMIT_ENABLED: "${RATELIMIT_ENABLED:-true}"
```

In `.github/workflows/ci.yml`, the e2e job's **"Start backend stack"** step — add an `env:` so the compose interpolation disables limiting for that run:

```yaml
      - name: Start backend stack
        env:
          RATELIMIT_ENABLED: "false"
        run: docker compose up -d --build postgres redis minio migrate backend
```

- [ ] **Step 12: Commit**

```bash
git add backend/internal/identity/handler.go backend/internal/identity/routes.go backend/internal/identity/ratelimit_test.go backend/internal/server/router.go backend/internal/server/router_test.go backend/cmd/api/main.go backend/api/openapi.yaml docker-compose.yml .github/workflows/ci.yml
git commit -m "feat(security): wire rate limiting (global + auth IP + login account-key); e2e opt-out"
```

---

## Self-Review

**1. Spec coverage:**
- bagian 3 berkas: config (T1), backstop (T2), ratelimit+Allower (T3), middleware PerIP+helpers (T4), identity handler/routes + router + main + openapi (T5). `.env.example` (T1). All present.
- bagian 4 Limiter/Result/Allower/keyPrefix/fail-open/backstop/disabled — T3. bagian 5 backstop bounded+janitor+eviction — T2 (janitor started in T3 `New`). bagian 6 PerIP + headers + WriteRateLimited — T4. bagian 7 account-key in handler (post-parse, normalized email) — T5. bagian 8 config env — T1. bagian 9 wiring (global throttle, auth IP-limits, main, Deps) — T5. bagian 10 testing — each task. bagian 11 risks (no email in logs via keyPrefix/prefix; fail-open; ClientIP; bounded backstop; openapi; deps) — covered. Added beyond spec: CI e2e opt-out (T5 Step 11) — necessary so limiting doesn't break the existing e2e (not in spec; flagged here).
- Spec bagian 10 "integration via running stack/manual" — not a unit task; the final verification + CI e2e (with limiting off) cover regressions; a manual flood check is recommended at finish.

**2. Placeholder scan:** No TBD/TODO. Every code step has full code; commands have expected output. The OpenAPI step references the actual file structure (`components.responses`, operation `responses`) the implementer edits in place — concrete, not vague.

**3. Type consistency:** `ratelimit.Result` fields used identically across T3/T4/T5. `Allower.Allow(ctx, key string, perMin int, withBackstop bool) Result` signature matches in T3 (def), T4 (`PerIP` param + fake), T5 (`denyLimiter`, handler field, router pass). `middleware.PerIP(l ratelimit.Allower, perMin int, prefix string, withBackstop bool)` and `middleware.WriteRateLimited(c, res)` consistent T4→T5. `identity.NewHandler(..., limiter ratelimit.Allower, loginPerMin int)` and `RegisterRoutes(..., limiter ratelimit.Allower, loginIPPerMin, refreshPerMin int)` defined T5 and called in router T5 Step 6. `Deps.Limiter *ratelimit.Limiter` (concrete) satisfies `ratelimit.Allower` where passed. `config.RateLimit*` names identical T1↔T5. Consistent.
