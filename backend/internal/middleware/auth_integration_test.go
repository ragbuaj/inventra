//go:build integration

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func newAuthDeps(t *testing.T) (*auth.TokenManager, *auth.TokenStore) {
	t.Helper()
	cfg := &config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	return auth.NewTokenManager(cfg), auth.NewTokenStore(testsupport.NewRedis(t))
}

func serveWithAuth(tm *auth.TokenManager, store *auth.TokenStore, token string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", RequireAuth(tm, store), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"session": c.GetString(CtxSessionID)})
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequireAuth_LiveSessionPasses_RevokedIs401(t *testing.T) {
	ctx := context.Background()
	tm, store := newAuthDeps(t)

	pair, err := tm.Issue("user-1", "role-1", "sess-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if err := store.SaveSession(ctx, "sess-1", auth.SessionMeta{UserID: "user-1", RefreshJTI: pair.RefreshJTI}, time.Hour); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	// Live session → 200.
	if w := serveWithAuth(tm, store, pair.AccessToken); w.Code != http.StatusOK {
		t.Fatalf("live session: want 200, got %d: %s", w.Code, w.Body.String())
	}

	// Revoke the session → the same still-unexpired access token must now 401.
	if _, err := store.DeleteSession(ctx, "user-1", "sess-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if w := serveWithAuth(tm, store, pair.AccessToken); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session: want 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequireAuth_LegacyTokenWithoutSidSkipsSessionCheck(t *testing.T) {
	tm, store := newAuthDeps(t)

	// A token minted with an empty sid (pre-device-sessions) has no session
	// record; it must still pass so existing logins are not force-killed.
	pair, err := tm.Issue("user-2", "role-2", "")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if w := serveWithAuth(tm, store, pair.AccessToken); w.Code != http.StatusOK {
		t.Fatalf("legacy no-sid token: want 200, got %d: %s", w.Code, w.Body.String())
	}
}
