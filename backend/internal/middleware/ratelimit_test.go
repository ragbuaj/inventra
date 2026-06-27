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
	if w.Header().Get("RateLimit-Reset") != "30" {
		t.Fatalf("RateLimit-Reset: %q", w.Header().Get("RateLimit-Reset"))
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
	if w.Header().Get("RateLimit-Limit") != "5" {
		t.Fatalf("deny RateLimit-Limit: %q", w.Header().Get("RateLimit-Limit"))
	}
	if w.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("deny RateLimit-Remaining: %q", w.Header().Get("RateLimit-Remaining"))
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

func TestSetRateLimitHeadersClampsNegativeRemaining(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		SetRateLimitHeaders(c, ratelimit.Result{Limit: 5, Remaining: -3, ResetAfter: 0})
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("negative Remaining must clamp to 0, got %q", w.Header().Get("RateLimit-Remaining"))
	}
}
