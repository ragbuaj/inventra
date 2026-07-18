//go:build integration

package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// allowLimiter always allows (the rate-limit paths have their own tests).
type allowLimiter struct{}

func (allowLimiter) Allow(_ context.Context, _ string, _ int, _ bool) ratelimit.Result {
	return ratelimit.Result{Allowed: true}
}

// newAuthRouter wires the login/refresh/logout handlers over a real-Redis
// service with the REAL RequireAuth middleware, mirroring routes.go.
func newAuthRouter(t *testing.T) (*gin.Engine, *Service, *auth.TokenManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, store := newIntegrationService(t, fs, &fakeMailer{})
	// Same config as newIntegrationService, so this manager verifies the
	// service's tokens.
	tm := auth.NewTokenManager(&config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour})
	h := &Handler{svc: svc, limiter: allowLimiter{}, loginPerMin: 5, refreshTTL: time.Hour}

	r := gin.New()
	r.POST("/auth/login", h.login)
	r.POST("/auth/refresh", h.refresh)
	r.POST("/auth/logout", middleware.RequireAuth(tm, store), h.logout)
	return r, svc, tm
}

type tokenBody struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func postJSON(t *testing.T, r http.Handler, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeTokens(t *testing.T, w *httptest.ResponseRecorder) tokenBody {
	t.Helper()
	var b tokenBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("unmarshal token body: %v (%s)", err, w.Body.String())
	}
	return b
}

// Mobile end-to-end: login (body refresh token, no cookie) -> refresh from the
// body (rotation) -> logout from the body -> the session is fully dead.
func TestMobileLoginRefreshLogout_EndToEnd(t *testing.T) {
	r, _, tm := newAuthRouter(t)
	mobileHdr := map[string]string{"X-Client-Type": "mobile"}

	// Login as mobile: refresh token in the body, never a cookie.
	w := postJSON(t, r, "/auth/login", `{"email":"u@x.com","password":"oldpassword"}`, mobileHdr)
	if w.Code != http.StatusOK {
		t.Fatalf("mobile login: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if sc := w.Header().Get("Set-Cookie"); sc != "" {
		t.Fatalf("mobile login must not set any cookie, got %q", sc)
	}
	tokens := decodeTokens(t, w)
	if tokens.RefreshToken == "" {
		t.Fatalf("mobile login must return refresh_token in the body: %s", w.Body.String())
	}
	claims, err := tm.Parse(tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.ClientAudience() != auth.AudienceMobile {
		t.Fatalf("mobile login must stamp aud=mobile, got %q", claims.ClientAudience())
	}

	// Refresh from the body: rotated pair, still body-only.
	w = postJSON(t, r, "/auth/refresh", `{"refresh_token":"`+tokens.RefreshToken+`"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("mobile refresh: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if sc := w.Header().Get("Set-Cookie"); sc != "" {
		t.Fatalf("mobile refresh must not set any cookie, got %q", sc)
	}
	rotated := decodeTokens(t, w)
	if rotated.RefreshToken == "" || rotated.RefreshToken == tokens.RefreshToken {
		t.Fatalf("mobile refresh must rotate the body refresh token")
	}
	rc, err := tm.Parse(rotated.RefreshToken)
	if err != nil {
		t.Fatalf("parse rotated refresh: %v", err)
	}
	if rc.ClientAudience() != auth.AudienceMobile {
		t.Fatalf("rotation must propagate aud=mobile, got %q", rc.ClientAudience())
	}

	// The pre-rotation refresh token is spent.
	w = postJSON(t, r, "/auth/refresh", `{"refresh_token":"`+tokens.RefreshToken+`"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("spent refresh token: want 401, got %d", w.Code)
	}

	// Logout with the refresh token in the body; no cookie is cleared (none was set).
	w = postJSON(t, r, "/auth/logout", `{"refresh_token":"`+rotated.RefreshToken+`"}`,
		map[string]string{"Authorization": "Bearer " + rotated.AccessToken})
	if w.Code != http.StatusOK {
		t.Fatalf("mobile logout: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if sc := w.Header().Get("Set-Cookie"); sc != "" {
		t.Fatalf("mobile logout must not touch cookies, got %q", sc)
	}

	// The session is dead: the refresh token no longer rotates.
	w = postJSON(t, r, "/auth/refresh", `{"refresh_token":"`+rotated.RefreshToken+`"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: want 401, got %d: %s", w.Code, w.Body.String())
	}
}

// Web regression: the pre-ADR-0017 behavior is byte-for-byte unchanged —
// refresh token only as an httpOnly cookie, never in the body.
func TestWebLoginRefreshLogout_Regression(t *testing.T) {
	r, _, tm := newAuthRouter(t)

	w := postJSON(t, r, "/auth/login", `{"email":"u@x.com","password":"oldpassword"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("web login: want 200, got %d: %s", w.Code, w.Body.String())
	}
	cookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, refreshCookieName+"=") {
		t.Fatalf("web login must set the refresh cookie, got %q", cookie)
	}
	if strings.Contains(w.Body.String(), "refresh_token") {
		t.Fatalf("web login body must never contain refresh_token: %s", w.Body.String())
	}
	tokens := decodeTokens(t, w)
	claims, err := tm.Parse(tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.ClientAudience() != auth.AudienceWeb {
		t.Fatalf("web login must stamp aud=web, got %q", claims.ClientAudience())
	}

	// Refresh via the cookie.
	rt := cookieValue(t, cookie)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: rt})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("web refresh: want 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Header().Get("Set-Cookie"), refreshCookieName+"=") {
		t.Fatalf("web refresh must rotate the cookie, got %q", w2.Header().Get("Set-Cookie"))
	}
	if strings.Contains(w2.Body.String(), "refresh_token") {
		t.Fatalf("web refresh body must never contain refresh_token: %s", w2.Body.String())
	}

	// Logout clears the cookie (unchanged web behavior).
	rotated := decodeTokens(t, w2)
	req = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+rotated.AccessToken)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: cookieValue(t, w2.Header().Get("Set-Cookie"))})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req)
	if w3.Code != http.StatusOK {
		t.Fatalf("web logout: want 200, got %d: %s", w3.Code, w3.Body.String())
	}
	if sc := w3.Header().Get("Set-Cookie"); !strings.Contains(sc, "Max-Age=0") {
		t.Fatalf("web logout must clear the refresh cookie, got %q", sc)
	}
}

// cookieValue extracts the refresh cookie's value from a Set-Cookie header.
func cookieValue(t *testing.T, setCookie string) string {
	t.Helper()
	prefix := refreshCookieName + "="
	if !strings.HasPrefix(setCookie, prefix) {
		t.Fatalf("unexpected Set-Cookie %q", setCookie)
	}
	rest := setCookie[len(prefix):]
	if i := strings.IndexByte(rest, ';'); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// SessionAlive defense-in-depth (ADR-0017 M-3): a refresh token whose device
// session record is gone must not rotate, even while its JTI is still on the
// refresh whitelist (the two Redis structures can drift).
func TestRefresh_DeadSessionRejected(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, store := newIntegrationService(t, fs, &fakeMailer{})

	pair, _, err := svc.Login(context.Background(), "u@x.com", "oldpassword", "Chrome", "1.1.1.1", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	// Kill ONLY the session record; DeleteSession leaves the refresh JTI on
	// the whitelist, exactly the drift this check closes.
	if _, err := store.DeleteSession(context.Background(), u.ID.String(), pair.SID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if valid, _ := store.RefreshValid(context.Background(), pair.RefreshJTI); !valid {
		t.Fatal("precondition: the refresh JTI must still be whitelisted")
	}
	if _, err := svc.Refresh(context.Background(), pair.RefreshToken, "Chrome", "1.1.1.1"); err != ErrInvalidToken {
		t.Fatalf("refresh on a dead session: want ErrInvalidToken, got %v", err)
	}
}

// Rotation must carry the mobile audience through the service layer.
func TestRefresh_PropagatesMobileAudience(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, _ := newIntegrationService(t, fs, &fakeMailer{})

	pair, _, err := svc.Login(context.Background(), "u@x.com", "oldpassword", "Android", "1.1.1.1", auth.AudienceMobile)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair.Audience != auth.AudienceMobile {
		t.Fatalf("login pair audience: want mobile, got %q", pair.Audience)
	}
	rotated, err := svc.Refresh(context.Background(), pair.RefreshToken, "Android", "1.1.1.1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rotated.Audience != auth.AudienceMobile {
		t.Fatalf("rotated pair audience: want mobile, got %q", rotated.Audience)
	}
	if rotated.SID != pair.SID {
		t.Fatalf("rotation must preserve the sid: was %q now %q", pair.SID, rotated.SID)
	}
}
