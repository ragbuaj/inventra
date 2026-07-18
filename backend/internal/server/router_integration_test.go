//go:build integration

// Full-router integration tests for ADR-0017's per-client auth: the mobile
// login-refresh-logout path end to end over the REAL wiring in NewRouter
// (RequireAuth + RequireAudience + identity handlers), the aud=mobile deny
// list on /authz and /imports, and the unchanged web cookie path.
package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/ratelimit"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

const (
	itEmail    = "m0@inventra.test"
	itPassword = "m0-password-123"
)

// newIntegrationRouter builds the production router over throwaway Postgres +
// Redis and seeds one active user whose role holds role.manage (so the web
// path can positively reach /authz).
func newIntegrationRouter(t *testing.T) http.Handler {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	cfg := &config.Config{
		Env:           "test",
		JWTSecret:     "router-it-secret",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: time.Hour,

		RateLimitEnabled:      false,
		RateLimitTimeoutMS:    50,
		RateLimitGlobalPerMin: 10000,

		FrontendURL:      "http://localhost:3000",
		PasswordResetTTL: 30 * time.Minute,

		AvatarMaxBytes:     1 << 20,
		AttachmentMaxBytes: 1 << 20,
		ImportMaxRows:      100,
		ImportMaxBytes:     1 << 20,
		ImportWorkerPoll:   time.Hour,

		NotificationRelayPoll:     time.Hour,
		NotificationStreamMaxLen:  100,
		NotificationClaimMinIdle:  time.Minute,
		NotificationSweepPoll:     time.Hour,
		NotificationRetentionDays: 1,
	}

	seedAdminUser(t, pool)

	r, _ := NewRouter(Deps{
		Cfg:     cfg,
		Pool:    pool,
		Redis:   rdb,
		Log:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Limiter: ratelimit.New(rdb, cfg),
	})
	return r
}

// seedAdminUser inserts a role holding role.manage and one active user with a
// password login.
func seedAdminUser(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	roleID := testsupport.SeedRole(t, pool, "m0-admin-"+uuid.New().String()[:8])
	_, err := pool.Exec(ctx,
		`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, 'role.manage')`, roleID)
	require.NoError(t, err)
	hash, err := auth.HashPassword(itPassword)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO identity.users (name, email, password_hash, role_id, status)
		 VALUES ('M0 Admin', $1, $2, $3, 'active')`, itEmail, hash, roleID)
	require.NoError(t, err)
}

type itTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func itRequest(t *testing.T, r http.Handler, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func itTokensOf(t *testing.T, w *httptest.ResponseRecorder) itTokens {
	t.Helper()
	var tk itTokens
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tk), "token body: %s", w.Body.String())
	return tk
}

func TestRouter_MobileAuthFlowAndAudienceDeny(t *testing.T) {
	r := newIntegrationRouter(t)
	loginBody := `{"email":"` + itEmail + `","password":"` + itPassword + `"}`

	// 1. Mobile login: refresh token in the body, no cookie.
	w := itRequest(t, r, http.MethodPost, "/api/v1/auth/login", loginBody,
		map[string]string{"X-Client-Type": "mobile"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Empty(t, w.Header().Get("Set-Cookie"), "mobile login must not set a cookie")
	mobile := itTokensOf(t, w)
	require.NotEmpty(t, mobile.RefreshToken, "mobile login must return refresh_token in the body")

	bearer := func(tok string) map[string]string {
		return map[string]string{"Authorization": "Bearer " + tok}
	}

	// 2. Shared endpoints stay open to mobile...
	w = itRequest(t, r, http.MethodGet, "/api/v1/auth/me", "", bearer(mobile.AccessToken))
	require.Equal(t, http.StatusOK, w.Code, "shared endpoint must allow mobile: %s", w.Body.String())

	// ...but the ADR-0017 deny list rejects the same (fully-permissioned)
	// account with 403 on authzadmin and importer routes.
	w = itRequest(t, r, http.MethodGet, "/api/v1/authz/roles", "", bearer(mobile.AccessToken))
	require.Equal(t, http.StatusForbidden, w.Code, "authz must deny aud=mobile: %s", w.Body.String())
	w = itRequest(t, r, http.MethodGet, "/api/v1/imports", "", bearer(mobile.AccessToken))
	require.Equal(t, http.StatusForbidden, w.Code, "imports must deny aud=mobile: %s", w.Body.String())

	// 3. Refresh from the body: rotated, still cookieless, audience preserved
	// (the rotated access token is still denied on /authz).
	w = itRequest(t, r, http.MethodPost, "/api/v1/auth/refresh",
		`{"refresh_token":"`+mobile.RefreshToken+`"}`, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Empty(t, w.Header().Get("Set-Cookie"), "mobile refresh must not set a cookie")
	rotated := itTokensOf(t, w)
	require.NotEmpty(t, rotated.RefreshToken)
	require.NotEqual(t, mobile.RefreshToken, rotated.RefreshToken, "refresh must rotate")
	w = itRequest(t, r, http.MethodGet, "/api/v1/authz/roles", "", bearer(rotated.AccessToken))
	require.Equal(t, http.StatusForbidden, w.Code, "audience must survive rotation")

	// The spent refresh token no longer works.
	w = itRequest(t, r, http.MethodPost, "/api/v1/auth/refresh",
		`{"refresh_token":"`+mobile.RefreshToken+`"}`, nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// 4. Logout from the body kills the session.
	w = itRequest(t, r, http.MethodPost, "/api/v1/auth/logout",
		`{"refresh_token":"`+rotated.RefreshToken+`"}`, bearer(rotated.AccessToken))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Empty(t, w.Header().Get("Set-Cookie"), "mobile logout must not touch cookies")
	w = itRequest(t, r, http.MethodPost, "/api/v1/auth/refresh",
		`{"refresh_token":"`+rotated.RefreshToken+`"}`, nil)
	require.Equal(t, http.StatusUnauthorized, w.Code, "refresh after logout must fail")
	w = itRequest(t, r, http.MethodGet, "/api/v1/auth/me", "", bearer(rotated.AccessToken))
	require.Equal(t, http.StatusUnauthorized, w.Code, "the access token dies with its session")
}

func TestRouter_WebAuthFlowRegression(t *testing.T) {
	r := newIntegrationRouter(t)
	loginBody := `{"email":"` + itEmail + `","password":"` + itPassword + `"}`

	// Web login (no X-Client-Type): cookie set, body clean of refresh_token.
	w := itRequest(t, r, http.MethodPost, "/api/v1/auth/login", loginBody, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	setCookie := w.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "inventra_refresh=", "web login must set the refresh cookie")
	require.NotContains(t, w.Body.String(), "refresh_token", "web body must never carry refresh_token")
	web := itTokensOf(t, w)

	// Web reaches authzadmin (role.manage seeded) — the deny list is
	// audience-scoped, not a blanket lockdown.
	w = itRequest(t, r, http.MethodGet, "/api/v1/authz/roles", "",
		map[string]string{"Authorization": "Bearer " + web.AccessToken})
	require.Equal(t, http.StatusOK, w.Code, "web admin must still reach /authz: %s", w.Body.String())

	// Refresh via the cookie still works and still answers with a cookie.
	cookie := setCookie[:strings.IndexByte(setCookie, ';')]
	w = itRequest(t, r, http.MethodPost, "/api/v1/auth/refresh", "",
		map[string]string{"Cookie": cookie})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Header().Get("Set-Cookie"), "inventra_refresh=", "web refresh must rotate the cookie")
	require.NotContains(t, w.Body.String(), "refresh_token")
}
