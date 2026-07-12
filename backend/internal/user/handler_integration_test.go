//go:build integration

package user_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/testsupport"
	"github.com/ragbuaj/inventra/internal/user"
)

// seedUserDirect inserts an identity.users row directly and returns its id.
func seedUserDirect(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, status)
		 VALUES ($1, $2, $3, 'active') RETURNING id`,
		email, email, roleID).Scan(&id))
	return id
}

// doRequest builds a fresh gin engine wired to the real user handler (via
// user.RegisterRoutes) with a stub auth middleware that injects the caller's
// role directly (bypassing real JWT), then drives an HTTP request and decodes
// the JSON body into a map for inspection.
func doRequest(t *testing.T, h *user.Handler, method, path string, roleID uuid.UUID) (int, map[string]any) {
	t.Helper()
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, uuid.New().String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	r := gin.New()
	v1 := r.Group("/api/v1")
	user.RegisterRoutes(v1, h, stubAuth)
	w := httptest.NewRecorder()
	req, err := http.NewRequest(method, path, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)
	var body map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &body)
	}
	return w.Code, body
}

// TestUser_FieldMasking_HandlerWiring drives the real HTTP handler (gin engine
// built via user.RegisterRoutes) end-to-end and proves that field-permission
// masking actually executes on the list/get response path — not just that the
// underlying authz.FilterEntity/FilterView helpers work in isolation. If
// someone deleted the handler's filterMaps/one call sites, this test must fail.
func TestUser_FieldMasking_HandlerWiring(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)

	// Deny view on "email" for this role on the "users" entity.
	role := testsupport.SeedRole(t, pool, "r-user-masking")
	testsupport.SeedFieldPermission(t, pool, role, "users", "email", false, false)

	target := seedUserDirect(t, pool, role, "wiring.target@test.local")

	gin.SetMode(gin.TestMode)

	t.Run("get masks email but keeps name", func(t *testing.T) {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/users/"+target.String(), role)
		require.Equal(t, http.StatusOK, code)
		assert.NotContains(t, body, "email", "email not viewable -> dropped")
		assert.Contains(t, body, "name", "name has no policy -> default-allow kept")
	})

	t.Run("list masks email on every row", func(t *testing.T) {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/users", role)
		require.Equal(t, http.StatusOK, code)
		rows, ok := body["data"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rows)
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			require.True(t, ok)
			assert.NotContains(t, row, "email")
			assert.Contains(t, row, "name")
		}
	})
}

// TestUser_FieldMasking_FailsClosed proves the fail-closed regression fix:
// when the field-permission policy lookup fails (e.g. Postgres unreachable),
// the user handler must respond 500 and never serve the record unfiltered.
// Before this change, user.filterMaps silently swallowed a ForEntity error
// and returned the record unmasked (fail-open) — this test would have passed
// on the old fail-open code too (it never asserted the body was empty), so
// the key assertion is the 500 status: on the pre-fix handler this sub-test
// fails because the pre-fix handler always answers 200 with the record data
// no matter what the field-permission lookup does.
//
// The FieldService's *sqlc.Queries is backed by a second pool ("poisoned")
// pointed at the same database as the handler's own pool, so the user lookup
// itself (via the healthy pool) still succeeds and only the field-permission
// lookup fails — isolating the exact failure path under test.
func TestUser_FieldMasking_FailsClosed(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()

	q := sqlc.New(pool)
	svc := user.NewService(q)
	auditSvc := audit.NewService(q)

	role := testsupport.SeedRole(t, pool, "r-user-failclosed")
	testsupport.SeedFieldPermission(t, pool, role, "users", "email", false, false)
	target := seedUserDirect(t, pool, role, "victim@test.local")

	// A second pool to the same database, used only by the FieldService, so it
	// can be closed independently of the pool the user Service relies on.
	dsn := pool.Config().ConnConfig.ConnString()
	poisonPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	poisonQ := sqlc.New(poisonPool)
	fieldSvc := authz.NewFieldService(poisonQ, rdb)

	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	// This role's field-permission cache is cold (first-ever lookup), so
	// Redis is guaranteed to miss and ForEntity falls through to Postgres via
	// the now-closed poisoned pool, deterministically failing the lookup.
	poisonPool.Close()

	t.Run("get responds 500, not the unfiltered record", func(t *testing.T) {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/users/"+target.String(), role)
		require.Equal(t, http.StatusInternalServerError, code, "must fail closed (500) on a field-policy lookup error")
		assert.NotContains(t, body, "email", "unfiltered user data must never leak on a lookup error")
		assert.NotContains(t, body, "name", "unfiltered user data must never leak on a lookup error")
	})

	t.Run("list responds 500, not the unfiltered records", func(t *testing.T) {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/users", role)
		require.Equal(t, http.StatusInternalServerError, code, "must fail closed (500) on a field-policy lookup error")
		assert.NotContains(t, body, "data", "unfiltered user list must never leak on a lookup error")
	})
}

// TestUser_FieldMasking_FailsClosed_InvalidRoleID proves the fail-closed fix
// for the second failure mode covered by the post-review hardening pass: an
// unparseable/missing CtxRoleID (e.g. a malformed or absent role claim) must
// also cause the handler to respond 500 rather than silently falling back to
// serving the record unmasked.
//
// Before this fix, user.filterMaps swallowed the uuid.Parse error on
// CtxRoleID and returned nil (no error), so the handler responded 200 with
// the fully unmasked record — the exact fail-open bug this test guards
// against. It reproduces two ways CtxRoleID can be unusable: absent entirely
// (auth middleware bug/edge case) and present but not a valid UUID.
func TestUser_FieldMasking_FailsClosed_InvalidRoleID(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)

	role := testsupport.SeedRole(t, pool, "r-user-badrole")
	testsupport.SeedFieldPermission(t, pool, role, "users", "email", false, false)
	target := seedUserDirect(t, pool, role, "badrole.target@test.local")

	gin.SetMode(gin.TestMode)

	// stubAuthMissingRole authenticates a user but never sets CtxRoleID.
	stubAuthMissingRole := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, uuid.New().String())
		c.Next()
	}
	// stubAuthInvalidRole sets CtxRoleID to a non-UUID string.
	stubAuthInvalidRole := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, uuid.New().String())
		c.Set(middleware.CtxRoleID, "not-a-uuid")
		c.Next()
	}

	doWith := func(t *testing.T, authMW gin.HandlerFunc, method, path string) (int, map[string]any) {
		t.Helper()
		r := gin.New()
		v1 := r.Group("/api/v1")
		user.RegisterRoutes(v1, h, authMW)
		w := httptest.NewRecorder()
		req, err := http.NewRequest(method, path, nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		var body map[string]any
		if w.Body.Len() > 0 {
			_ = json.Unmarshal(w.Body.Bytes(), &body)
		}
		return w.Code, body
	}

	t.Run("missing role id: get responds 500, not the unfiltered record", func(t *testing.T) {
		code, body := doWith(t, stubAuthMissingRole, http.MethodGet, "/api/v1/users/"+target.String())
		require.Equal(t, http.StatusInternalServerError, code, "must fail closed (500) when CtxRoleID is missing")
		assert.NotContains(t, body, "email", "unfiltered user data must never leak when role id is missing")
		assert.NotContains(t, body, "name", "unfiltered user data must never leak when role id is missing")
	})

	t.Run("invalid role id: get responds 500, not the unfiltered record", func(t *testing.T) {
		code, body := doWith(t, stubAuthInvalidRole, http.MethodGet, "/api/v1/users/"+target.String())
		require.Equal(t, http.StatusInternalServerError, code, "must fail closed (500) when CtxRoleID is unparseable")
		assert.NotContains(t, body, "email", "unfiltered user data must never leak when role id is unparseable")
		assert.NotContains(t, body, "name", "unfiltered user data must never leak when role id is unparseable")
	})

	t.Run("missing role id: list responds 500, not the unfiltered records", func(t *testing.T) {
		code, body := doWith(t, stubAuthMissingRole, http.MethodGet, "/api/v1/users")
		require.Equal(t, http.StatusInternalServerError, code, "must fail closed (500) when CtxRoleID is missing")
		assert.NotContains(t, body, "data", "unfiltered user list must never leak when role id is missing")
	})
}
