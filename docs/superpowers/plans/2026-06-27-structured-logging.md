# Structured Logging & Request Correlation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Gin's default logging with structured `log/slog`, end-to-end request correlation via `X-Request-ID` (FE↔BE), centralized redaction of sensitive fields, and a structured panic recovery.

**Architecture:** A new `internal/logging` package builds the slog logger (JSON prod / text dev, level via config, sensitive keys redacted at the handler) and provides context helpers. Three gin middlewares (`RequestID`, `RequestLogger`, `Recovery`) live in `internal/middleware`. Wiring in `cmd/api/main.go` + `internal/server/router.go` swaps in the new middleware and replaces stdlib `log`. The frontend `useApiClient` attaches an `X-Request-ID` per call.

**Tech Stack:** Go 1.25, `log/slog` (stdlib), Gin, `github.com/google/uuid` (already a dep), Nuxt 4 / Vitest.

## Global Constraints

- Logger: **JSON in prod, text in dev**, format from `cfg.Env` (development→text, else json), overridable by `LOG_FORMAT` ∈ {`json`,`text`}; level via `LOG_LEVEL` ∈ {`debug`,`info`,`warn`,`error`}, default `info`.
- **Redaction** (handler `ReplaceAttr`): keys (case-insensitive) `password, password_hash, token, access_token, refresh_token, secret, authorization, google_id, api_key` → value `"[REDACTED]"`.
- Request log line attrs: `method, path, status, latency_ms, request_id` (+ `user_id`/`role_id` when set). Level by status: ≥500 error, ≥400 warn, else info. **Skip** `/health` and `/health/ready`.
- Correlation header is `X-Request-ID` (read inbound or generate, echo in response).
- Recovery: log panic at error with `request_id` + `stack`; respond `500 {"error":"internal server error"}`; never leak the stack to the client.
- No new dependencies (use stdlib + existing `google/uuid`). No new OpenAPI path (no `openapi.yaml` change required).
- Backend gate: `go build ./... && go vet ./... && go test ./...`. Frontend gate: `pnpm lint && pnpm typecheck && pnpm test`. (Run from `backend/` and `frontend/` respectively.)
- Commits use Conventional Commits with a scope; no AI/co-author trailers.

---

### Task 1: `internal/logging` package + config fields

**Files:**
- Modify: `backend/internal/config/config.go` (add `LogLevel`, `LogFormat` fields + `Load` mapping)
- Modify: `backend/.env.example` (append logging vars)
- Create: `backend/internal/logging/logging.go`
- Test: `backend/internal/logging/logging_test.go`, `backend/internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config.LogLevel string`, `config.Config.LogFormat string`; `logging.New(cfg *config.Config) *slog.Logger`; `logging.WithLogger(ctx, *slog.Logger) context.Context`; `logging.FromContext(ctx) *slog.Logger`.

- [ ] **Step 1: Write the failing test (logging package)**

Create `backend/internal/logging/logging_test.go`:

```go
package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/ragbuaj/inventra/internal/config"
)

func bufJSON(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level, ReplaceAttr: redactAttr})
	return slog.New(h), &buf
}

func TestNewHonorsLevel(t *testing.T) {
	l := New(&config.Config{Env: "production", LogLevel: "warn"})
	if l.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info must be disabled at warn level")
	}
	if !l.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("warn must be enabled")
	}
}

func TestUseJSONResolution(t *testing.T) {
	if useJSON(&config.Config{Env: "development"}) {
		t.Fatal("dev default should be text")
	}
	if !useJSON(&config.Config{Env: "production"}) {
		t.Fatal("prod default should be json")
	}
	if !useJSON(&config.Config{Env: "development", LogFormat: "json"}) {
		t.Fatal("LOG_FORMAT=json overrides dev")
	}
	if useJSON(&config.Config{Env: "production", LogFormat: "text"}) {
		t.Fatal("LOG_FORMAT=text overrides prod")
	}
}

func TestRedactionTopLevel(t *testing.T) {
	l, buf := bufJSON(slog.LevelInfo)
	l.Info("login", "email", "a@b.com", "password", "hunter2", "google_id", "xyz")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["password"] != "[REDACTED]" || m["google_id"] != "[REDACTED]" {
		t.Fatalf("sensitive keys not redacted: %v", m)
	}
	if m["email"] != "a@b.com" {
		t.Fatalf("non-sensitive key altered: %v", m["email"])
	}
}

func TestRedactionInGroup(t *testing.T) {
	l, buf := bufJSON(slog.LevelInfo)
	l.Info("req", slog.Group("auth", "token", "secret-value"))
	s := buf.String()
	if strings.Contains(s, "secret-value") || !strings.Contains(s, "[REDACTED]") {
		t.Fatalf("token inside group not redacted: %s", s)
	}
}

func TestContextRoundTrip(t *testing.T) {
	if FromContext(context.Background()) == nil {
		t.Fatal("must fall back to a non-nil default")
	}
	custom, _ := bufJSON(slog.LevelInfo)
	ctx := WithLogger(context.Background(), custom)
	if FromContext(ctx) != custom {
		t.Fatal("must return the stored logger")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/logging/`
Expected: FAIL — package/functions undefined (won't compile).

- [ ] **Step 3: Create the logging package**

Create `backend/internal/logging/logging.go`:

```go
// Package logging builds the application's structured slog logger and provides
// request-scoped logger propagation via context. Sensitive fields are redacted
// at the handler level (see ADR-0002).
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/ragbuaj/inventra/internal/config"
)

const redacted = "[REDACTED]"

// sensitiveKeys are never written in clear text.
var sensitiveKeys = map[string]struct{}{
	"password": {}, "password_hash": {}, "token": {}, "access_token": {},
	"refresh_token": {}, "secret": {}, "authorization": {}, "google_id": {}, "api_key": {},
}

type ctxKey struct{}

// New builds the app logger from config. It does NOT call slog.SetDefault —
// the caller (main) does that explicitly.
func New(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel), ReplaceAttr: redactAttr}
	var h slog.Handler
	if useJSON(cfg) {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

// useJSON resolves the output format: explicit LOG_FORMAT wins, else json
// unless the environment is development.
func useJSON(cfg *config.Config) bool {
	switch strings.ToLower(cfg.LogFormat) {
	case "json":
		return true
	case "text":
		return false
	default:
		return cfg.Env != "development"
	}
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// redactAttr masks sensitive values regardless of type or nesting depth.
func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if _, ok := sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, redacted)
	}
	return a
}

// WithLogger stores a request-scoped logger on the context.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the context's logger, or slog.Default() if absent.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
```

- [ ] **Step 4: Add config fields**

In `backend/internal/config/config.go`, add two fields to the `Config` struct (near the other groups):

```go
	// Logging (ADR-0002).
	LogLevel  string
	LogFormat string
```

And in `Load()` (alongside the other `getEnv` assignments):

```go
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", ""),
```

- [ ] **Step 5: Write the config test**

Create `backend/internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadLoggingDefaults(t *testing.T) {
	cfg := Load()
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel default: %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "" {
		t.Fatalf("LogFormat default should be empty (auto): %q", cfg.LogFormat)
	}
}

func TestLoadLoggingFromEnv(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")
	cfg := Load()
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Fatalf("env not applied: level=%q format=%q", cfg.LogLevel, cfg.LogFormat)
	}
}
```

- [ ] **Step 6: Append env example**

Append to `backend/.env.example`:

```
# Logging (ADR-0002)
LOG_LEVEL=info
# LOG_FORMAT: blank = auto (text in development, json otherwise); set json or text to override
LOG_FORMAT=
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd backend && go test ./internal/logging/ ./internal/config/`
Expected: PASS (all tests).

- [ ] **Step 8: Build/vet + commit**

Run: `cd backend && go build ./... && go vet ./...`
Expected: clean.

```bash
git add backend/internal/logging/ backend/internal/config/config.go backend/internal/config/config_test.go backend/.env.example
git commit -m "feat(logging): slog logger with redaction + request-scoped context (ADR-0002)"
```

---

### Task 2: RequestID + RequestLogger middleware

**Files:**
- Create: `backend/internal/middleware/requestlog.go`
- Test: `backend/internal/middleware/requestlog_test.go`

**Interfaces:**
- Consumes: `logging.WithLogger`; existing `middleware.CtxUserID` (`"user_id"`), `middleware.CtxRoleID` (`"role_id"`).
- Produces: `middleware.CtxRequestID` (`"request_id"`); `middleware.RequestHeaderID` (`"X-Request-ID"`); `middleware.RequestID() gin.HandlerFunc`; `middleware.RequestLogger(base *slog.Logger) gin.HandlerFunc`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/middleware/requestlog_test.go`:

```go
package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func bufLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func TestRequestIDGeneratesAndEchoes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get(RequestHeaderID) == "" {
		t.Fatal("expected a generated X-Request-ID echoed in the response")
	}
}

func TestRequestIDPreservesInbound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestHeaderID, "abc-123")
	r.ServeHTTP(w, req)
	if got := w.Header().Get(RequestHeaderID); got != "abc-123" {
		t.Fatalf("inbound id not preserved: %s", got)
	}
}

func TestRequestLoggerEmitsStructuredLine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestHeaderID, "rid-1")
	r.ServeHTTP(w, req)

	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("parse %q: %v", buf.String(), err)
	}
	if m["request_id"] != "rid-1" || m["method"] != "GET" || m["path"] != "/x" {
		t.Fatalf("missing attrs: %v", m)
	}
	if _, ok := m["latency_ms"]; !ok {
		t.Fatal("latency_ms missing")
	}
	if m["status"].(float64) != 200 {
		t.Fatalf("status: %v", m["status"])
	}
}

func TestRequestLoggerSkipsHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if buf.Len() != 0 {
		t.Fatalf("/health must not be logged: %s", buf.String())
	}
}

func TestRequestLoggerLevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["level"] != "ERROR" {
		t.Fatalf("status 500 must log at ERROR, got %v", m["level"])
	}
}

func TestRequestLoggerIncludesUserWhenSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/u", func(c *gin.Context) {
		c.Set(CtxUserID, "user-7")
		c.Set(CtxRoleID, "role-3")
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/u", nil))
	var m map[string]any
	json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m)
	if m["user_id"] != "user-7" || m["role_id"] != "role-3" {
		t.Fatalf("user/role not logged: %v", m)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/middleware/ -run RequestID`
Expected: FAIL — `RequestID`/`RequestLogger`/`RequestHeaderID`/`CtxRequestID` undefined.

- [ ] **Step 3: Create the middleware**

Create `backend/internal/middleware/requestlog.go`:

```go
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/logging"
)

// CtxRequestID is the gin context key (and log attribute name) for the request id.
const CtxRequestID = "request_id"

// RequestHeaderID is the inbound/outbound correlation header.
const RequestHeaderID = "X-Request-ID"

// healthPaths are noisy probes excluded from request logging.
var healthPaths = map[string]struct{}{"/health": {}, "/health/ready": {}}

// RequestID reads an inbound X-Request-ID or generates one, stores it on the gin
// context, and echoes it in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestHeaderID)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(CtxRequestID, id)
		c.Writer.Header().Set(RequestHeaderID, id)
		c.Next()
	}
}

// RequestLogger binds a request-scoped logger (carrying request_id) into the
// request context and emits one structured line per request on completion.
// Level scales with status; /health probes are skipped.
func RequestLogger(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID, _ := c.Get(CtxRequestID)
		id, _ := reqID.(string)
		reqLog := base.With(slog.String("request_id", id))
		c.Request = c.Request.WithContext(logging.WithLogger(c.Request.Context(), reqLog))

		start := time.Now()
		c.Next()

		if _, skip := healthPaths[c.Request.URL.Path]; skip {
			return
		}
		status := c.Writer.Status()
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
		}
		if uid, ok := c.Get(CtxUserID); ok {
			attrs = append(attrs, slog.Any("user_id", uid))
		}
		if rid, ok := c.Get(CtxRoleID); ok {
			attrs = append(attrs, slog.Any("role_id", rid))
		}
		switch {
		case status >= 500:
			reqLog.Error("request", attrs...)
		case status >= 400:
			reqLog.Warn("request", attrs...)
		default:
			reqLog.Info("request", attrs...)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/middleware/ -run "RequestID|RequestLogger"`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/requestlog.go backend/internal/middleware/requestlog_test.go
git commit -m "feat(logging): RequestID + RequestLogger gin middleware (X-Request-ID correlation)"
```

---

### Task 3: Recovery middleware (structured panic recovery)

**Files:**
- Create: `backend/internal/middleware/recovery.go`
- Test: `backend/internal/middleware/recovery_test.go`

**Interfaces:**
- Consumes: `middleware.CtxRequestID`, `middleware.RequestHeaderID`, `middleware.RequestID()` (Task 2).
- Produces: `middleware.Recovery(base *slog.Logger) gin.HandlerFunc`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/middleware/recovery_test.go`:

```go
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryReturns500AndLogsStructured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	r := gin.New()
	r.Use(RequestID(), Recovery(log))
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set(RequestHeaderID, "rid-9")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "internal server error") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "kaboom") || strings.Contains(w.Body.String(), "goroutine") {
		t.Fatalf("panic detail leaked to client: %s", w.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("parse %q: %v", buf.String(), err)
	}
	if m["request_id"] != "rid-9" || m["msg"] != "panic recovered" {
		t.Fatalf("log attrs: %v", m)
	}
	if _, ok := m["stack"]; !ok {
		t.Fatal("stack attr missing from log")
	}
}

func TestRecoveryPassesNormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := gin.New()
	r.Use(RequestID(), Recovery(log))
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("normal request broke: %d", w.Code)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/middleware/ -run Recovery`
Expected: FAIL — `Recovery` undefined.

- [ ] **Step 3: Create the middleware**

Create `backend/internal/middleware/recovery.go`:

```go
package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery converts a panic into a structured error log (with request_id) and a
// clean 500 JSON response, without leaking the stack to the client.
func Recovery(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				rid, _ := c.Get(CtxRequestID)
				base.With(slog.Any("request_id", rid)).Error("panic recovered",
					slog.String("error", fmt.Sprint(r)),
					slog.String("path", c.Request.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/middleware/ -run Recovery`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/recovery.go backend/internal/middleware/recovery_test.go
git commit -m "feat(logging): structured panic Recovery middleware"
```

---

### Task 4: Wire logger into main.go + router.go

**Files:**
- Modify: `backend/internal/server/router.go` (add `Log` to `Deps`; replace `gin.Logger()`+`gin.Recovery()`)
- Modify: `backend/cmd/api/main.go` (build logger, `slog.SetDefault`, stdlib `log` → `slog`, pass `Log` in `Deps`)
- Test: `backend/internal/server/router_test.go`

**Interfaces:**
- Consumes: `logging.New` (Task 1); `middleware.RequestID`, `middleware.RequestLogger`, `middleware.Recovery` (Tasks 2–3).
- Produces: `server.Deps.Log *slog.Logger`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/server/router_test.go`:

```go
package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ragbuaj/inventra/internal/config"
)

// The router must mount the correlation middleware: a request to /health
// returns 200 and the response carries an echoed X-Request-ID. Pool/Redis are
// nil here because /health touches neither; the feature-module constructors
// only store their deps.
func TestRouterHealthEchoesRequestID(t *testing.T) {
	d := Deps{
		Cfg: &config.Config{Env: "test"},
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	r := NewRouter(d)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("/health status: %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("router did not echo X-Request-ID — RequestID middleware not mounted")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/server/`
Expected: FAIL to compile — `Deps` has no field `Log`.

> If this test panics inside `NewRouter` because a feature-module constructor dereferences a nil `Pool`/`Redis`, that is a real finding: report it. (The current constructors — `sqlc.New`, `auth.NewTokenManager/NewTokenStore`, `authz.New*`, `identity.NewService`, `user.NewService`, `audit.NewService` — only store their arguments, so nil is safe.)

- [ ] **Step 3: Update router.go**

In `backend/internal/server/router.go`:

(a) add the slog import to the import block:

```go
	"log/slog"
```

(b) add the `Log` field to `Deps`:

```go
// Deps holds the shared infrastructure passed to feature modules.
type Deps struct {
	Cfg   *config.Config
	Pool  *pgxpool.Pool
	Redis *redis.Client
	Log   *slog.Logger
}
```

(c) replace the base-middleware line

```go
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS(d.Cfg.FrontendURL))
```

with

```go
	r.Use(
		middleware.RequestID(),
		middleware.RequestLogger(d.Log),
		middleware.Recovery(d.Log),
		middleware.CORS(d.Cfg.FrontendURL),
	)
```

- [ ] **Step 4: Run the router test to verify it passes**

Run: `cd backend && go test ./internal/server/`
Expected: PASS.

- [ ] **Step 5: Update main.go**

Rewrite `backend/cmd/api/main.go` to build the logger, set it as default, replace stdlib `log`, and pass it to `Deps`. Replace the import block's `"log"` with `"log/slog"` and add the logging package; the full file becomes:

```go
// Command api is the entry point for the Inventra backend service.
//
// Inventra follows a modular monolith with clean architecture (see docs/PRD.md §7).
// This entry point wires configuration, infrastructure (PostgreSQL, Redis), and the
// HTTP server; feature modules are registered through the router as they are implemented.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ragbuaj/inventra/internal/cache"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/db"
	"github.com/ragbuaj/inventra/internal/logging"
	"github.com/ragbuaj/inventra/internal/server"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg)
	slog.SetDefault(logger)
	ctx := context.Background()

	// PostgreSQL (authoritative store).
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		slog.Error("db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Ping(ctx, pool); err != nil {
		slog.Warn("PostgreSQL not reachable at startup", "error", err)
	} else {
		slog.Info("PostgreSQL connected")
	}

	// Redis (cache/state).
	rdb := cache.NewClient(cfg)
	defer func() { _ = rdb.Close() }()
	if err := cache.Ping(ctx, rdb); err != nil {
		slog.Warn("Redis not reachable at startup", "error", err)
	} else {
		slog.Info("Redis connected")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           server.NewRouter(server.Deps{Cfg: cfg, Pool: pool, Redis: rdb, Log: logger}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Inventra API listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
```

- [ ] **Step 6: Verify gin defaults are gone, build, vet, full test**

Run: `cd backend && grep -rn "gin.Logger()\|gin.Recovery()" internal cmd || echo "none"`
Expected: `none`.
Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/server/router.go backend/internal/server/router_test.go backend/cmd/api/main.go
git commit -m "feat(logging): wire slog + correlation/recovery middleware; drop gin defaults"
```

---

### Task 5: Frontend — propagate `X-Request-ID`

**Files:**
- Modify: `frontend/app/composables/useApiClient.ts` (add header in `request()`)
- Test: `frontend/test/nuxt/useApiClient.spec.ts`

**Interfaces:**
- Consumes: existing `useApiClient().request(path, opts)`.
- Produces: every `request()` call sends an `X-Request-ID` header (caller-provided value preserved).

- [ ] **Step 1: Write the failing test**

Create `frontend/test/nuxt/useApiClient.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { defineComponent } from 'vue'
import { useApiClient } from '~/composables/useApiClient'

const fetchMock = vi.fn(() => Promise.resolve({}))
vi.stubGlobal('$fetch', fetchMock)

const Harness = defineComponent({
  setup() {
    return { api: useApiClient() }
  },
  template: '<div />'
})

function lastHeaders(): Record<string, string> {
  const call = fetchMock.mock.calls.at(-1)
  return (call?.[1] as { headers: Record<string, string> }).headers
}

describe('useApiClient X-Request-ID propagation', () => {
  beforeEach(() => fetchMock.mockClear())

  it('adds a UUID X-Request-ID header when the caller provides none', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api.request('/x')
    expect(lastHeaders()['X-Request-ID']).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i)
  })

  it('preserves a caller-provided X-Request-ID', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api.request('/x', { headers: { 'X-Request-ID': 'fixed-id' } })
    expect(lastHeaders()['X-Request-ID']).toBe('fixed-id')
  })
})
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && pnpm test -- useApiClient`
Expected: FAIL — header absent (first test). If `vi.stubGlobal('$fetch', ...)` does not intercept in this setup, switch to `@nuxt/test-utils`'s `registerEndpoint`/`mockNuxtImport` to capture the request; the assertions (header present / preserved) stay the same.

- [ ] **Step 3: Add the header in `request()`**

In `frontend/app/composables/useApiClient.ts`, inside `request()`, right after the `Authorization` header line, add:

```ts
    if (!headers['X-Request-ID']) headers['X-Request-ID'] = crypto.randomUUID()
```

So the header block reads:

```ts
    const headers: Record<string, string> = { ...(opts.headers as Record<string, string> || {}) }
    if (auth.accessToken) headers.Authorization = `Bearer ${auth.accessToken}`
    if (!headers['X-Request-ID']) headers['X-Request-ID'] = crypto.randomUUID()
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd frontend && pnpm test -- useApiClient`
Expected: PASS (2 tests).

- [ ] **Step 5: Lint, typecheck, commit**

Run: `cd frontend && pnpm lint && pnpm typecheck`
Expected: clean (no trailing commas; the added line has none).

```bash
git add frontend/app/composables/useApiClient.ts frontend/test/nuxt/useApiClient.spec.ts
git commit -m "feat(logging): propagate X-Request-ID from useApiClient"
```

---

## Self-Review

**1. Spec coverage:**
- §3 berkas — logging package (T1), config (T1), `.env.example` (T1), requestlog middleware (T2), recovery (T3), main/router wiring (T4), useApiClient (T5). All covered.
- §4 logging package (New/format/level/redaction/context) — T1. §5 middleware (RequestID/RequestLogger/Recovery, skip health, level-by-status, user/role attrs) — T2 + T3. §6 wiring (SetDefault, stdlib→slog) — T4. §7 config knobs — T1. §8 frontend X-Request-ID (+ preserve caller id) — T5. §9 testing — each task's tests + gates. §10 risks acknowledged (redaction safety net; no request fail; audit follow-up deferred; uuid via existing dep). Covered.

**2. Placeholder scan:** No TBD/TODO; every code step has full code; commands have expected output. The two "if X doesn't work, switch to Y" notes (nil-deps panic in T4 Step 2; `$fetch` stub in T5 Step 2) are explicit fallbacks with concrete alternatives, not vague placeholders.

**3. Type consistency:** `logging.New(*config.Config) *slog.Logger`, `WithLogger`/`FromContext` used consistently across T1→T2→T3→T4. `middleware.CtxRequestID`/`RequestHeaderID`/`RequestID`/`RequestLogger`/`Recovery` names match between definition (T2/T3) and use (T4). `Deps.Log *slog.Logger` defined T4 and set in main.go T4. Config fields `LogLevel`/`LogFormat` defined T1 and read by `logging.New` T1. `redactAttr` referenced by the T1 test is defined in the T1 implementation. Consistent.
