//go:build integration

// Cache-Control coverage over the REAL wiring in NewRouter: every response under
// /api/v1 must carry no-store so authenticated bank asset data never lands in the
// browser's HTTP cache (issue #149). The unit-level matrix for the middleware
// itself lives in internal/middleware/nostore_test.go; this file proves the
// policy holds end to end on real, authenticated list and detail endpoints.
package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/ratelimit"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

const (
	ccEmail    = "cache-control@inventra.test"
	ccPassword = "cache-control-password-123"
)

// ccFixture is a live router plus the seeded data the assertions address.
type ccFixture struct {
	router   http.Handler
	token    string
	officeID uuid.UUID
}

// newCacheControlFixture builds the production router over throwaway Postgres +
// Redis, seeds an office tree and a role with GLOBAL scope on the offices module,
// and logs in. Global scope is what makes the DETAIL endpoint reachable: the
// conservative fallback (own) would answer 404 for an office the caller cannot
// see, which would prove nothing about a populated response.
func newCacheControlFixture(t *testing.T) ccFixture {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	cfg := &config.Config{
		Env:           "test",
		JWTSecret:     "cache-control-it-secret",
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

	tree := testsupport.SeedOfficeTree(t, pool)
	roleID := seedOfficeReaderUser(t, pool)
	testsupport.SeedScopePolicy(t, pool, roleID, "offices", sqlc.SharedScopeLevelGlobal)

	r, _ := NewRouter(Deps{
		Cfg:     cfg,
		Pool:    pool,
		Redis:   rdb,
		Log:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Limiter: ratelimit.New(rdb, cfg),
	})

	w := itRequest(t, r, http.MethodPost, "/api/v1/auth/login",
		`{"email":"`+ccEmail+`","password":"`+ccPassword+`"}`, nil)
	require.Equal(t, http.StatusOK, w.Code, "login: %s", w.Body.String())

	return ccFixture{router: r, token: itTokensOf(t, w).AccessToken, officeID: tree.Pusat}
}

// seedOfficeReaderUser inserts a role plus one active user with a password login
// and returns the role id, so the caller can attach a scope policy to it.
func seedOfficeReaderUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	roleID := testsupport.SeedRole(t, pool, "cc-office-reader-"+uuid.New().String()[:8])
	hash, err := auth.HashPassword(ccPassword)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO identity.users (name, email, password_hash, role_id, status)
		 VALUES ('Cache Control Reader', $1, $2, $3, 'active')`, ccEmail, hash, roleID)
	require.NoError(t, err)
	return roleID
}

// requireNoStore asserts the header is present, correct, and present EXACTLY
// once — a second value would mean the middleware and a handler both appended
// instead of one overwriting the other.
func requireNoStore(t *testing.T, w *httptest.ResponseRecorder, what string) {
	t.Helper()
	values := w.Header().Values("Cache-Control")
	require.Len(t, values, 1, "%s: Cache-Control must appear exactly once, got %v", what, values)
	require.Equal(t, "no-store", values[0], "%s: unexpected Cache-Control", what)
}

// AC1 + AC2: a populated LIST response and a populated DETAIL response, both
// authenticated, both 200, both no-store. Neither endpoint sets the header
// itself today — they are covered only because the policy is cross-cutting.
func TestCacheControl_ListAndDetailAreNoStore(t *testing.T) {
	f := newCacheControlFixture(t)
	bearer := map[string]string{"Authorization": "Bearer " + f.token}

	list := itRequest(t, f.router, http.MethodGet, "/api/v1/offices", "", bearer)
	require.Equal(t, http.StatusOK, list.Code, "offices list: %s", list.Body.String())
	require.Contains(t, list.Body.String(), `"total"`, "list endpoint must return a real page: %s", list.Body.String())
	require.Contains(t, list.Body.String(), "Pusat", "list must contain the seeded office: %s", list.Body.String())
	requireNoStore(t, list, "GET /api/v1/offices")

	detail := itRequest(t, f.router, http.MethodGet, "/api/v1/offices/"+f.officeID.String(), "", bearer)
	require.Equal(t, http.StatusOK, detail.Code, "offices detail: %s", detail.Body.String())
	require.Contains(t, detail.Body.String(), "Pusat", "detail must return the seeded office: %s", detail.Body.String())
	requireNoStore(t, detail, "GET /api/v1/offices/:id")
}

// AC1 "apa pun status codenya": the header must not depend on the request
// succeeding. These are the paths a browser is most likely to retry and cache.
func TestCacheControl_ErrorResponsesAreNoStore(t *testing.T) {
	f := newCacheControlFixture(t)
	bearer := map[string]string{"Authorization": "Bearer " + f.token}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		header map[string]string
		want   int
	}{
		{"unauthenticated list", http.MethodGet, "/api/v1/offices", "", nil, http.StatusUnauthorized},
		{"bad bearer", http.MethodGet, "/api/v1/offices", "", map[string]string{"Authorization": "Bearer nonsense"}, http.StatusUnauthorized},
		{"unknown office", http.MethodGet, "/api/v1/offices/" + uuid.New().String(), "", bearer, http.StatusNotFound},
		{"malformed id", http.MethodGet, "/api/v1/offices/not-a-uuid", "", bearer, http.StatusBadRequest},
		{"unrouted api path", http.MethodGet, "/api/v1/no-such-module", "", bearer, http.StatusNotFound},
		{"failed login", http.MethodPost, "/api/v1/auth/login", `{"email":"` + ccEmail + `","password":"wrong-password"}`, nil, http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := itRequest(t, f.router, tc.method, tc.path, tc.body, tc.header)
			require.Equal(t, tc.want, w.Code, "body: %s", w.Body.String())
			requireNoStore(t, w, tc.name)
		})
	}
}

// AC3: handlers that already set their own stricter value keep it, and it does
// not stack with the middleware's. The avatar endpoint is the live example the
// issue points at (internal/identity/avatar_handler.go).
func TestCacheControl_HandlerSetValueIsNotDuplicated(t *testing.T) {
	f := newCacheControlFixture(t)
	bearer := map[string]string{"Authorization": "Bearer " + f.token}

	w := itRequest(t, f.router, http.MethodGet, "/api/v1/auth/me", "", bearer)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// The avatar route answers 404 for a user with no avatar, but it sets its
	// header before it knows that, so the assertion holds either way.
	avatar := itRequest(t, f.router, http.MethodGet, "/api/v1/auth/avatar", "", bearer)
	values := avatar.Header().Values("Cache-Control")
	require.Len(t, values, 1, "avatar Cache-Control must appear exactly once, got %v", values)
	require.Contains(t, values[0], "no-store", "avatar must stay uncacheable, got %q", values[0])
}

// Writes must be covered too — a POST response body echoes the created record.
func TestCacheControl_MutationResponsesAreNoStore(t *testing.T) {
	f := newCacheControlFixture(t)
	bearer := map[string]string{"Authorization": "Bearer " + f.token}

	// The seeded role holds no masterdata.office.manage, so this is a 403 from
	// the permission gate — still a /api/v1 response, still no-store.
	w := itRequest(t, f.router, http.MethodPost, "/api/v1/offices",
		`{"name":"Cabang Baru","code":"CB9"}`, bearer)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	requireNoStore(t, w, "POST /api/v1/offices")
}

// The policy is scoped to /api/v1. Operational endpoints outside it keep their
// existing behaviour; this guards against the middleware being widened to the
// whole engine by accident.
func TestCacheControl_NonAPIRoutesUnaffected(t *testing.T) {
	f := newCacheControlFixture(t)

	for _, path := range []string{"/health", "/health/ready", "/openapi.yaml"} {
		w := itRequest(t, f.router, http.MethodGet, path, "", nil)
		require.Empty(t, w.Header().Values("Cache-Control"),
			"%s must be left alone by the /api/v1 no-store policy", path)
	}
}
