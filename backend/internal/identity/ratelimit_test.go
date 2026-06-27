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
