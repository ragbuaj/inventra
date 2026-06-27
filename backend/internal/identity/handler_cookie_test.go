package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/auth"
)

// refresh with no cookie must 401 before touching the (nil) service.
func TestRefreshMissingCookieReturns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{} // svc nil — the cookie guard must return first
	r := gin.New()
	r.POST("/auth/refresh", h.refresh)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/refresh", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a refresh cookie, got %d", w.Code)
	}
}

// the token response must never serialize a refresh_token.
func TestTokenResponseOmitsRefreshToken(t *testing.T) {
	b, err := json.Marshal(newTokenResponse(auth.TokenPair{AccessToken: "a", AccessExpiresAt: time.Now().Add(time.Minute)}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "refresh_token") {
		t.Fatalf("token response must not contain refresh_token: %s", b)
	}
	if !strings.Contains(string(b), "access_token") {
		t.Fatalf("token response missing access_token: %s", b)
	}
}
