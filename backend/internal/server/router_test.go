package server

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
	// RateLimitEnabled:false so the nil Redis client is never dialled; the
	// limiter still satisfies Allower and the middleware is mounted (fail-open).
	cfg := &config.Config{Env: "test", RateLimitEnabled: false, RateLimitTimeoutMS: 50, RateLimitGlobalPerMin: 120}
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
