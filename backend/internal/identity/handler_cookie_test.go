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

// refresh with neither a cookie nor a body token must 401 before touching the
// (nil) service.
func TestRefreshMissingCookieAndBodyReturns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{} // svc nil — the missing-token guard must return first
	r := gin.New()
	r.POST("/auth/refresh", h.refresh)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/refresh", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a refresh token, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing refresh token") {
		t.Fatalf("want the missing-token error, got %s", w.Body.String())
	}
}

// refresh must consume a body refresh_token when the cookie is absent (mobile
// path): a garbage token from the body reaches the service and fails as
// INVALID, not as missing.
func TestRefreshBodyTokenReachesService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{svc: newTestService(t, &fakeStore{}, &fakeMailer{})}
	r := gin.New()
	r.POST("/auth/refresh", h.refresh)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"refresh_token":"garbage"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a garbage body token, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid or expired token") {
		t.Fatalf("the body token must reach the service (invalid, not missing): %s", w.Body.String())
	}
}

// Per-client invariant (ADR-0017 guard rail): the WEB token response must
// never serialize a refresh_token.
func TestWebTokenResponseOmitsRefreshToken(t *testing.T) {
	b, err := json.Marshal(newTokenResponse(auth.TokenPair{
		AccessToken:     "a",
		RefreshToken:    "r", // present on the pair, must never leak into the web body
		AccessExpiresAt: time.Now().Add(time.Minute),
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "refresh_token") {
		t.Fatalf("web token response must not contain refresh_token: %s", b)
	}
	if !strings.Contains(string(b), "access_token") {
		t.Fatalf("token response missing access_token: %s", b)
	}
}

// Per-client invariant: the MOBILE token response must carry the refresh token
// in the body (secure storage on the device replaces the cookie).
func TestMobileTokenResponseIncludesRefreshToken(t *testing.T) {
	b, err := json.Marshal(newMobileTokenResponse(auth.TokenPair{
		AccessToken:     "a",
		RefreshToken:    "r",
		AccessExpiresAt: time.Now().Add(time.Minute),
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["refresh_token"] != "r" {
		t.Fatalf("mobile token response must contain refresh_token: %s", b)
	}
}

// writeTokenPair routing: a web pair sets the cookie and keeps the body clean;
// a mobile pair puts the token in the body and never sets a cookie.
func TestWriteTokenPairPerAudience(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{refreshTTL: time.Hour}

	serve := func(pair auth.TokenPair) *httptest.ResponseRecorder {
		r := gin.New()
		r.POST("/x", func(c *gin.Context) { h.writeTokenPair(c, pair) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))
		return w
	}

	web := serve(auth.TokenPair{AccessToken: "a", RefreshToken: "r", Audience: auth.AudienceWeb, AccessExpiresAt: time.Now().Add(time.Minute)})
	if !strings.Contains(web.Header().Get("Set-Cookie"), refreshCookieName) {
		t.Fatalf("web pair must set the refresh cookie, got %q", web.Header().Get("Set-Cookie"))
	}
	if strings.Contains(web.Body.String(), "refresh_token") {
		t.Fatalf("web body must not contain refresh_token: %s", web.Body.String())
	}

	mobile := serve(auth.TokenPair{AccessToken: "a", RefreshToken: "r", Audience: auth.AudienceMobile, AccessExpiresAt: time.Now().Add(time.Minute)})
	if sc := mobile.Header().Get("Set-Cookie"); sc != "" {
		t.Fatalf("mobile pair must not set any cookie, got %q", sc)
	}
	if !strings.Contains(mobile.Body.String(), `"refresh_token":"r"`) {
		t.Fatalf("mobile body must contain the refresh token: %s", mobile.Body.String())
	}
}

// clientAudience: only the exact header value "mobile" selects the mobile
// audience; anything else (or absence) is web.
func TestClientAudienceHeaderMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		header string
		want   string
	}{
		{"", auth.AudienceWeb},
		{"web", auth.AudienceWeb},
		{"mobile", auth.AudienceMobile},
		{" mobile ", auth.AudienceMobile},
		{"desktop", auth.AudienceWeb},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		if tc.header != "" {
			c.Request.Header.Set("X-Client-Type", tc.header)
		}
		if got := clientAudience(c); got != tc.want {
			t.Fatalf("header %q: want %q, got %q", tc.header, tc.want, got)
		}
	}
}
