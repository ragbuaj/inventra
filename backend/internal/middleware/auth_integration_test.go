//go:build integration

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

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
		c.JSON(http.StatusOK, gin.H{"session": c.GetString(CtxSessionID), "audience": c.GetString(CtxAudience)})
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

	pair, err := tm.Issue("user-1", "role-1", "sess-1", auth.AudienceWeb)
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
	pair, err := tm.Issue("user-2", "role-2", "", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if w := serveWithAuth(tm, store, pair.AccessToken); w.Code != http.StatusOK {
		t.Fatalf("legacy no-sid token: want 200, got %d: %s", w.Code, w.Body.String())
	}
}

// audienceOf decodes the echoed CtxAudience from the protected handler.
func audienceOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Audience string `json:"audience"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w.Body.String())
	}
	return body.Audience
}

// RequireAuth must stamp the token's audience into the context for
// RequireAudience to consume (ADR-0017).
func TestRequireAuth_StampsAudience(t *testing.T) {
	ctx := context.Background()
	tm, store := newAuthDeps(t)

	for _, aud := range []string{auth.AudienceWeb, auth.AudienceMobile} {
		sid := "sess-" + aud
		pair, err := tm.Issue("user-3", "role-3", sid, aud)
		if err != nil {
			t.Fatalf("Issue(%s): %v", aud, err)
		}
		if err := store.SaveSession(ctx, sid, auth.SessionMeta{UserID: "user-3", RefreshJTI: pair.RefreshJTI}, time.Hour); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}
		w := serveWithAuth(tm, store, pair.AccessToken)
		if w.Code != http.StatusOK {
			t.Fatalf("%s token: want 200, got %d: %s", aud, w.Code, w.Body.String())
		}
		if got := audienceOf(t, w); got != aud {
			t.Fatalf("CtxAudience: want %q, got %q", aud, got)
		}
	}
}

// A legacy token with NO aud claim (pre-audience production session) must pass
// RequireAuth and resolve to the web audience.
func TestRequireAuth_LegacyNoAudienceTokenIsWeb(t *testing.T) {
	tm, store := newAuthDeps(t)

	now := time.Now()
	legacy := auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "inventra",
			Subject:   "user-4",
			ID:        "jti-legacy",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		Type: auth.TokenAccess,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}
	w := serveWithAuth(tm, store, token)
	if w.Code != http.StatusOK {
		t.Fatalf("legacy no-aud token: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := audienceOf(t, w); got != auth.AudienceWeb {
		t.Fatalf("legacy token audience: want web, got %q", got)
	}
}
