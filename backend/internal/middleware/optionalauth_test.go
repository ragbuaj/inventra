package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
)

// optionalAuthDeps builds a token manager plus a store pointed at a port nothing
// listens on. Every path exercised here must therefore either never touch Redis,
// or treat a Redis failure as "guest" — which is exactly the property under test.
func optionalAuthDeps(t *testing.T) (*auth.TokenManager, *auth.TokenStore) {
	t.Helper()
	cfg := &config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = client.Close() })
	return auth.NewTokenManager(cfg), auth.NewTokenStore(client)
}

// serveOptional runs one request through OptionalAuth and reports the status
// plus whichever identity the middleware stamped on the context.
func serveOptional(tm *auth.TokenManager, store *auth.TokenStore, authHeader string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/public", OptionalAuth(tm, store), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user":  c.GetString(CtxUserID),
			"role":  c.GetString(CtxRoleID),
			"guest": c.GetString(CtxUserID) == ""})
	})
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// The central property: OptionalAuth never answers 401 and never aborts. A
// public page behind it must keep rendering for readers whose token is missing,
// broken, or stale — a 401 here would make the browser client wipe the session
// and bounce them to the login screen.
func TestOptionalAuthAlwaysContinuesAsGuest(t *testing.T) {
	tm, store := optionalAuthDeps(t)

	expired := &config.Config{JWTSecret: "test-secret", JWTAccessTTL: -time.Minute, JWTRefreshTTL: time.Hour}
	expiredPair, err := auth.NewTokenManager(expired).Issue("user-1", "role-1", "", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Issue (expired): %v", err)
	}
	otherSecret := &config.Config{JWTSecret: "another-secret", JWTAccessTTL: time.Minute, JWTRefreshTTL: time.Hour}
	foreignPair, err := auth.NewTokenManager(otherSecret).Issue("user-1", "role-1", "", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Issue (foreign): %v", err)
	}
	ownPair, err := tm.Issue("user-1", "role-1", "", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Issue (own): %v", err)
	}

	cases := map[string]string{
		"tanpa header":                   "",
		"header kosong":                  " ",
		"skema salah":                    "Basic dXNlcjpwYXNz",
		"kata Bearer saja":               "Bearer",
		"bearer tanpa token":             "Bearer ",
		"token sampah":                   "Bearer not-a-jwt",
		"jwt cacat":                      "Bearer aaa.bbb.ccc",
		"ditandatangani kunci lain":      "Bearer " + foreignPair.AccessToken,
		"token kedaluwarsa":              "Bearer " + expiredPair.AccessToken,
		"refresh dipakai sebagai access": "Bearer " + ownPair.RefreshToken,
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			w := serveOptional(tm, store, header)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, mau 200 (%s)", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"guest":true`) {
				t.Fatalf("body = %s, mau ditandai guest", w.Body.String())
			}
		})
	}
}

// Redis tidak terjangkau: pemeriksaan pencabutan gagal, dan kegagalan itu harus
// menurunkan pemanggil menjadi tamu — bukan 401, dan bukan 500. Tamu melihat
// lebih sedikit, jadi arah kegagalan ini aman.
func TestOptionalAuthFallsBackToGuestWhenRevocationCheckFails(t *testing.T) {
	tm, store := optionalAuthDeps(t)

	pair, err := tm.Issue("user-1", "role-1", "sess-1", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	w := serveOptional(tm, store, "Bearer "+pair.AccessToken)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"guest":true`) {
		t.Fatalf("body = %s, mau turun menjadi guest saat Redis mati", w.Body.String())
	}
}
