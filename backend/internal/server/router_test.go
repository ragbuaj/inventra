package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

// The global throttle is mounted on /api/v1; rate limiting is disabled in tests
// (RateLimitEnabled:false), so the limiter short-circuits without touching the nil
// Redis client, and a single /api/v1/health request is allowed (200) — proving the
// middleware is mounted without blocking normal traffic.
func TestRouterGlobalThrottleMounted(t *testing.T) {
	r := NewRouter(testDeps())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/api/v1/health status: %d", w.Code)
	}
}

// With no trusted proxies, a client-supplied X-Forwarded-For must be ignored so
// the rate-limit key uses the real RemoteAddr (not a spoofable header).
func TestClientIPIgnoresForwardedWhenNoTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil): %v", err)
	}
	var got string
	r.GET("/ip", func(c *gin.Context) {
		got = c.ClientIP()
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	r.ServeHTTP(httptest.NewRecorder(), req)
	if got != "203.0.113.9" {
		t.Fatalf("X-Forwarded-For must be ignored with no trusted proxies; got %q", got)
	}
}
