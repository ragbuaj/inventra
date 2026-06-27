# Spec — Rate Limiting (ADR-0004)

| | |
|---|---|
| **Tanggal** | 2026-06-27 |
| **ADR** | [0004](../../adr/0004-rate-limiting.md) (Accepted) · config per [0003](../../adr/0003-configuration.md) · log per [0002](../../adr/0002-structured-logging.md) · tests per [0001](../../adr/0001-go-testing-stack.md) |
| **Bagian dari** | Trio cross-cutting A→B→C: A=logging (✅ merged) → **B=rate limiting (ini)** → C=Google OAuth (ADR-0009) |
| **PRD** | FR-1.8 |
| **Status** | Disetujui — siap menulis implementation plan |

## 1. Tujuan & ruang lingkup

Menambah **rate limiting berbasis Redis** (anti brute-force pada auth + throttle umum per-IP), konsisten lintas
instance, dengan **fail-open + backstop in-memory** saat Redis tumbang, respons `429` standar, dan **log limit-hit**
(via logging ADR-0002, tanpa kredensial).

**Dalam ruang lingkup:**
- Paket `internal/ratelimit` (wrapper `redis_rate` GCRA + backstop `x/time/rate`).
- Middleware `PerIP` untuk throttle global (`/api/v1`) + IP-limit auth (`/auth/login`, `/auth/refresh`).
- Limit **account-key** login (IP+email) di handler login (post body-parse).
- Config band via env (ADR-0003) + kill-switch.
- Header `429`/`Retry-After`/`RateLimit-*`; test.

**Di luar ruang lingkup (fase lain):** throttle endpoint password-reset/verifikasi email (fiturnya belum ada —
ditambahkan saat fitur itu dibangun); rate-limit berbasis user terautentikasi; band runtime-tunable via
`app_settings`/UI (env sudah cukup per ADR-0004); gateway/proxy-level limiting.

## 2. Keputusan desain (disepakati)

1. **`redis_rate/v10` (GCRA)** di atas `go-redis/v9` yang sudah ada; dep baru `golang.org/x/time/rate` untuk backstop.
2. **Fail-open + backstop in-memory penuh** pada jalur auth (proteksi terdegradasi, bukan hilang, saat Redis error).
3. **Config band via env** (ADR-0003) dengan default aman + `RATELIMIT_ENABLED` kill-switch.
4. **Account-key login di `identity.Handler`** (limiter di-inject; cek setelah bind email) — tanpa double-parse.
5. Header standar `429` + `Retry-After` + `RateLimit-*`; limit-hit di-log `warn` (request_id).

## 3. Berkas

```
backend/internal/ratelimit/ratelimit.go        ← Limiter (redis_rate + backstop), Allow(...) Result
backend/internal/ratelimit/backstop.go          ← per-key x/time/rate (mutex-map + janitor + cap)
backend/internal/ratelimit/ratelimit_test.go     ← fail-open / backstop-select / header math (fake limiter)
backend/internal/ratelimit/backstop_test.go      ← deterministic token-bucket behavior
backend/internal/middleware/ratelimit.go         ← PerIP(limiter, perMin, prefix, withBackstop) gin middleware
backend/internal/middleware/ratelimit_test.go    ← 200 under / 429 over + headers + per-IP keying (fake)
backend/internal/config/config.go                ← + RateLimit* fields (env)
backend/internal/identity/handler.go             ← login: account-key check (post-parse)
backend/internal/identity/routes.go              ← mount auth IP-limit middleware on /auth/login,/auth/refresh
backend/internal/server/router.go                ← build/inject limiter; global throttle on /api/v1; pass to identity
backend/cmd/api/main.go                           ← build ratelimit.New(rdb,cfg)
backend/.env.example                              ← + RATELIMIT_* vars
backend/api/openapi.yaml                          ← document 429 + RateLimit-* on /auth/login,/auth/refresh (+ generic)
```

## 4. Paket `internal/ratelimit`

```go
package ratelimit

// Result is the outcome of a limit check.
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	RetryAfter time.Duration // until the denied request may retry (for Retry-After)
	ResetAfter time.Duration // until the window fully resets (for RateLimit-Reset)
	Degraded  bool // true when Redis failed and we fell back (backstop or fail-open)
}

// redisAllower is the seam over redis_rate for testability.
type redisAllower interface {
	Allow(ctx context.Context, key string, limit redis_rate.Limit) (*redis_rate.Result, error)
}

type Limiter struct {
	rl       redisAllower      // *redis_rate.Limiter in prod
	backstop *backstop         // in-memory fallback (auth only)
	timeout  time.Duration     // short per-call Redis timeout
	enabled  bool
}

func New(rdb *redis.Client, cfg *config.Config) *Limiter

// Allow checks `key` against `perMin`. On Redis error/timeout it logs a warning
// (via logging.FromContext) and, if withBackstop, consults the in-memory backstop;
// otherwise it fails open. When disabled, always allows.
func (l *Limiter) Allow(ctx context.Context, key string, perMin int, withBackstop bool) Result
```

- **Redis path:** `ctx, cancel := context.WithTimeout(ctx, l.timeout)`; `res, err := l.rl.Allow(ctx, key, redis_rate.PerMinute(perMin))`.
  - `err == nil` → `Result{Allowed: res.Allowed > 0, Limit: perMin, Remaining: res.Remaining, RetryAfter: res.RetryAfter}`.
  - `err != nil` (incl. timeout) → `logging.FromContext(ctx).Warn("rate limiter degraded", "key_prefix", prefixOf(key), "error", err)` (NEVER the full key if it embeds an email → log only the prefix); then:
    - `withBackstop` → `Result{Allowed: l.backstop.Allow(key, perMin), Degraded: true, Limit: perMin}`.
    - else → `Result{Allowed: true, Degraded: true, Limit: perMin}` (fail-open).
- `enabled == false` → `Result{Allowed: true, Limit: perMin}` immediately (kill-switch).

> Redaction note: the request logger already redacts by key (ADR-0002), but the limiter **must not** put an
> email into a log attribute value. Log only `key_prefix` (e.g. `login:acct`, `auth:ip`, `global`), never the
> full key.

## 5. Backstop (`backstop.go`)

```go
type backstop struct {
	mu      sync.Mutex
	buckets map[string]*entry // entry{ lim *rate.Limiter; lastSeen time.Time }
	max     int               // hard cap (e.g. 10000)
}

// Allow returns whether the key is within perMin on this instance. Above the cap
// it returns true (fail-open) rather than unbounded-allocate. A background janitor
// evicts idle entries (>10m) periodically.
func (b *backstop) Allow(key string, perMin int) bool
```

- Per-key `rate.NewLimiter(rate.Limit(perMin)/60.0, perMin)` (burst = perMin). Mutex-guarded map.
- Bounded: hard cap → fail-open when exceeded; janitor goroutine (started in `New`) evicts entries idle >10m every ~5m.
- Per-instance; only consulted during Redis failure on auth keys. Acceptable that it isn't cross-instance — it is the degraded path.

## 6. Middleware (`middleware/ratelimit.go`)

```go
// PerIP limits requests per client IP. prefix namespaces the key (e.g. "global",
// "auth"); withBackstop enables the in-memory fallback (auth paths). On deny it
// writes RateLimit-* + Retry-After and aborts 429; under the limit it sets the
// headers and continues.
func PerIP(l *ratelimit.Limiter, perMin int, prefix string, withBackstop bool) gin.HandlerFunc
```

- Key: `prefix + ":ip:" + c.ClientIP()`.
- `res := l.Allow(c.Request.Context(), key, perMin, withBackstop)`.
- Always set: `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` (seconds until reset).
- Deny (`!res.Allowed`): `Retry-After: ceil(res.RetryAfter seconds)` (≥1), `logging.FromContext(...).Warn("rate limit exceeded", "prefix", prefix, "ip", c.ClientIP())`, `c.AbortWithStatusJSON(429, gin.H{"error":"too many requests"})`.
- `c.ClientIP()` already honors Gin's trusted-proxy config; no extra header parsing.

## 7. Account-key login (handler)

`identity.Handler` gains a `limiter *ratelimit.Limiter` field (injected in `NewHandler`/router wiring). In `login`,
**after** `ShouldBindJSON`:

```go
key := "login:acct:" + strings.ToLower(strings.TrimSpace(req.Email))
if res := h.limiter.Allow(c.Request.Context(), key, h.loginPerMin, true); !res.Allowed {
	writeRateLimited(c, res) // sets Retry-After + RateLimit-* + 429
	return
}
```

`writeRateLimited(c, res)` is a small shared helper **in the `middleware` package** (identity already imports
`middleware`), used by both `PerIP` and the login handler so the 429 shape/headers are identical. Email is
normalized (lower+trim) so the account key is stable.

## 8. Config (env)

`config.Config` + `Load()`:
```
RATELIMIT_ENABLED        bool  default true
RATELIMIT_TIMEOUT_MS     int   default 50     // Redis limiter call timeout
RATELIMIT_GLOBAL_PER_MIN int   default 120    // per IP, global /api/v1
RATELIMIT_LOGIN_PER_MIN  int   default 5      // per IP+account, login
RATELIMIT_LOGIN_IP_PER_MIN int default 20     // per IP, login
RATELIMIT_REFRESH_PER_MIN int  default 30     // per IP, refresh
```
`.env.example` documents each (⚠️ placeholder values; tune per bank policy).

## 9. Wiring

- `main.go`: `limiter := ratelimit.New(rdb, cfg)` → pass into `server.Deps` (new field `Limiter *ratelimit.Limiter`).
- `router.go`: mount `middleware.PerIP(d.Limiter, cfg.RateLimitGlobalPerMin, "global", false)` on the `/api/v1`
  group (before feature routes). Pass the limiter (+ login band) into `identity.NewHandler`. Identity `RegisterRoutes`
  gains the limiter to attach `PerIP(... "auth" ...)` on `/auth/login` (login-IP band) and `/auth/refresh` (refresh band).
- Order: global throttle runs before `RequireAuth`; the auth IP-limit runs before the login/refresh handler; the
  account-key check runs first inside the handler (post body-parse). All limiters run after `RequestID`/`RequestLogger`
  so limit-hit logs carry `request_id`.

## 10. Testing (proaktif & luas)

**`ratelimit` (unit, fake `redisAllower`):**
- allowed/denied mapping from a canned `redis_rate.Result` (Allowed>0, Remaining, RetryAfter).
- Redis error + `withBackstop=false` → fail-open (`Allowed:true, Degraded:true`).
- Redis error + `withBackstop=true` → consults backstop (allow then deny once bucket drained).
- `enabled=false` → always allowed.
- timeout path (fake returns `context.DeadlineExceeded`) treated as Redis error → degraded.

**`backstop` (unit, deterministic):**
- within `perMin` allows, beyond drains to deny; distinct keys independent; above `max` cap → fail-open.

**`middleware.PerIP` (httptest, fake limiter):**
- under limit → 200 + `RateLimit-*` headers present; over → 429 + `Retry-After` (≥1) + body `{"error":"too many requests"}`.
- key includes client IP (two IPs counted separately).

**Login account-key (identity handler test, fake limiter):**
- N allowed then 429 for the same email; different emails independent; 429 body/headers match middleware.

**Integration (real Redis):** verified via the running dev stack / e2e manual check (login flood → 429), not a unit
test — redis_rate's Lua needs a real Redis. (Optional `//go:build integration` testcontainers test may be added later.)

**Gate:** `go build ./... && go vet ./... && go test ./...` + Spectral lint of the updated `openapi.yaml`.

## 11. Risiko & catatan

- **Jangan log kredensial/email**: limiter log hanya `key_prefix`/`prefix` + IP, tak pernah email atau key penuh.
- **Tidak menggagalkan request karena limiter**: jalur limiter selalu fail-open saat Redis error (auth via backstop); timeout pendek menjaga hot path.
- **`c.ClientIP()` & trusted proxies**: bergantung konfigurasi Gin trusted-proxy yang ada; di belakang LB pastikan proxy tepercaya agar IP klien benar (catatan ops, bukan kode baru di sini).
- **Backstop per-instance**: tidak konsisten lintas instance — sengaja (jalur degradasi). Cap + janitor mencegah kebocoran memori.
- **openapi.yaml**: tambah komponen response 429 + header `RateLimit-*`/`Retry-After` pada `/auth/login` & `/auth/refresh`; Spectral harus tetap hijau.
- **Deps baru**: `redis_rate/v10`, `golang.org/x/time/rate` — `go mod tidy`; tidak menambah layanan baru (reuse Redis).
