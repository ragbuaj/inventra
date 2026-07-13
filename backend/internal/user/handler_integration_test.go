//go:build integration

package user_test

import (
	"bytes"
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

// seedUserWithOfficeStatus inserts an identity.users row with an explicit
// office and status (for filter tests) and returns its id.
func seedUserWithOfficeStatus(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, officeID *uuid.UUID, status, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		email, email, roleID, officeID, status).Scan(&id))
	return id
}

// newTestRouter builds a fresh gin engine wired to the real user handler (via
// user.RegisterRoutes) with a stub auth middleware that injects the given
// caller identity directly (bypassing real JWT). Shared by doRequest and
// doJSON so both request styles use identical router/auth wiring.
func newTestRouter(h *user.Handler, roleID, userID uuid.UUID) *gin.Engine {
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	r := gin.New()
	v1 := r.Group("/api/v1")
	user.RegisterRoutes(v1, h, stubAuth)
	return r
}

// doRequest drives an HTTP request through a fresh router (see newTestRouter)
// and decodes the JSON body into a map for inspection. The stub caller's
// CtxUserID is a random, DB-unbacked uuid — fine for the read-only paths this
// helper exercises, since GET handlers never trigger audit.Record (which is
// the thing that requires a real identity.users-backed actor).
func doRequest(t *testing.T, h *user.Handler, method, path string, roleID uuid.UUID) (int, map[string]any) {
	t.Helper()
	r := newTestRouter(h, roleID, uuid.New())
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

// doJSON is doRequest's write-path counterpart: same router/stub-auth wiring
// (via newTestRouter), but for POST/PUT/DELETE requests that carry a JSON
// body. Pass a nil body for requests with no payload (e.g. DELETE). Unlike
// doRequest, callers must pass a real identity.users-backed userID (seeded via
// seedUserDirect) — create/update/delete call audit.Record on success, which
// writes actor_id = CtxUserID into audit_logs; a DB-unbacked actor id would
// violate audit_logs_actor_id_fkey and log a WARN on every run.
func doJSON(t *testing.T, h *user.Handler, method, path string, roleID, userID uuid.UUID, body any) (int, map[string]any) {
	t.Helper()
	r := newTestRouter(h, roleID, userID)

	var req *http.Request
	var err error
	if body != nil {
		raw, mErr := json.Marshal(body)
		require.NoError(t, mErr)
		req, err = http.NewRequest(method, path, bytes.NewReader(raw))
	} else {
		req, err = http.NewRequest(method, path, nil)
	}
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var respBody map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &respBody)
	}
	return w.Code, respBody
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

// TestUser_ListFilters_RoleOfficeStatus proves GET /users narrows results by
// the role_id/office_id/status query params (server-side filters), alone and
// combined, and rejects malformed values with 400.
func TestUser_ListFilters_RoleOfficeStatus(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)

	gin.SetMode(gin.TestMode)

	tree := testsupport.SeedOfficeTree(t, pool)
	roleA := testsupport.SeedRole(t, pool, "r-filter-a")
	roleB := testsupport.SeedRole(t, pool, "r-filter-b")
	caller := testsupport.SeedRole(t, pool, "r-filter-caller")

	officeA := tree.Cabang
	officeB := tree.Cabang2

	u1 := seedUserWithOfficeStatus(t, pool, roleA, &officeA, "active", "filter.u1@test.local")
	u2 := seedUserWithOfficeStatus(t, pool, roleB, &officeB, "inactive", "filter.u2@test.local")
	u3 := seedUserWithOfficeStatus(t, pool, roleA, &officeB, "suspended", "filter.u3@test.local")

	rowIDs := func(body map[string]any) []string {
		rows, ok := body["data"].([]any)
		require.True(t, ok)
		ids := make([]string, 0, len(rows))
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			require.True(t, ok)
			id, _ := row["id"].(string)
			ids = append(ids, id)
		}
		return ids
	}

	t.Run("role_id narrows to matching role", func(t *testing.T) {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/users?role_id="+roleA.String(), caller)
		require.Equal(t, http.StatusOK, code)
		assert.ElementsMatch(t, []string{u1.String(), u3.String()}, rowIDs(body))
	})

	t.Run("office_id narrows to matching office", func(t *testing.T) {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/users?office_id="+officeB.String(), caller)
		require.Equal(t, http.StatusOK, code)
		assert.ElementsMatch(t, []string{u2.String(), u3.String()}, rowIDs(body))
	})

	t.Run("status narrows to matching status", func(t *testing.T) {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/users?status=inactive", caller)
		require.Equal(t, http.StatusOK, code)
		assert.ElementsMatch(t, []string{u2.String()}, rowIDs(body))
	})

	t.Run("combined filters narrow to the single matching user", func(t *testing.T) {
		path := "/api/v1/users?role_id=" + roleA.String() + "&office_id=" + officeB.String() + "&status=suspended"
		code, body := doRequest(t, h, http.MethodGet, path, caller)
		require.Equal(t, http.StatusOK, code)
		assert.ElementsMatch(t, []string{u3.String()}, rowIDs(body))
	})

	t.Run("malformed role_id responds 400", func(t *testing.T) {
		code, _ := doRequest(t, h, http.MethodGet, "/api/v1/users?role_id=not-a-uuid", caller)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("malformed office_id responds 400", func(t *testing.T) {
		code, _ := doRequest(t, h, http.MethodGet, "/api/v1/users?office_id=not-a-uuid", caller)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("invalid status responds 400", func(t *testing.T) {
		code, _ := doRequest(t, h, http.MethodGet, "/api/v1/users?status=bogus", caller)
		assert.Equal(t, http.StatusBadRequest, code)
	})
}

// TestUser_Create_Success proves POST /users creates a user (201) and that the
// row is actually persisted in identity.users with the submitted fields.
func TestUser_Create_Success(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	role := testsupport.SeedRole(t, pool, "r-create-success")
	actor := seedUserDirect(t, pool, role, "actor.create-success@test.local")

	body := map[string]any{
		"name":     "Created User",
		"email":    "create.success@test.local",
		"password": "secret123",
		"role_id":  role.String(),
	}
	code, respBody := doJSON(t, h, http.MethodPost, "/api/v1/users", role, actor, body)
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "Created User", respBody["name"])
	assert.Equal(t, "create.success@test.local", respBody["email"])

	id, ok := respBody["id"].(string)
	require.True(t, ok, "response must include the new user's id")
	require.NotEmpty(t, id)

	var dbName, dbEmail string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT name, email FROM identity.users WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&dbName, &dbEmail))
	assert.Equal(t, "Created User", dbName)
	assert.Equal(t, "create.success@test.local", dbEmail)
}

// TestUser_Create_ValidationError proves POST /users rejects a request
// missing required fields (name, email, role_id — see dto.go binding tags)
// with 400, before any row is written.
func TestUser_Create_ValidationError(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	role := testsupport.SeedRole(t, pool, "r-create-validation")
	actor := seedUserDirect(t, pool, role, "actor.create-validation@test.local")

	// countByEmail asserts no phantom identity.users row was inserted for the
	// given (well-formed) email despite the 400 response — proves validation
	// failure short-circuits before any write, not just that the status code
	// happens to be 400.
	countByEmail := func(t *testing.T, email string) int {
		t.Helper()
		var count int
		require.NoError(t, pool.QueryRow(context.Background(),
			`SELECT count(*) FROM identity.users WHERE email = $1`, email).
			Scan(&count))
		return count
	}

	t.Run("missing name responds 400", func(t *testing.T) {
		const email = "missing.name@test.local"
		body := map[string]any{
			"email":   email,
			"role_id": role.String(),
		}
		code, _ := doJSON(t, h, http.MethodPost, "/api/v1/users", role, actor, body)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, 0, countByEmail(t, email), "rejected create must not insert a row")
	})

	t.Run("missing role_id responds 400", func(t *testing.T) {
		const email = "no.role@test.local"
		body := map[string]any{
			"name":  "No Role",
			"email": email,
		}
		code, _ := doJSON(t, h, http.MethodPost, "/api/v1/users", role, actor, body)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, 0, countByEmail(t, email), "rejected create must not insert a row")
	})

	t.Run("invalid email format responds 400", func(t *testing.T) {
		// No DB-absence check here: the submitted email ("not-an-email") is
		// itself malformed, so there's no well-formed email to look up — the
		// 400 assertion already covers the only meaningful behavior.
		body := map[string]any{
			"name":    "Bad Email",
			"email":   "not-an-email",
			"role_id": role.String(),
		}
		code, _ := doJSON(t, h, http.MethodPost, "/api/v1/users", role, actor, body)
		assert.Equal(t, http.StatusBadRequest, code)
	})
}

// TestUser_Create_DuplicateEmail proves POST /users maps the unique-email
// constraint violation (mapDBError -> ErrEmailExists) to 409 via svcError,
// exercising create's write path against an already-seeded email.
func TestUser_Create_DuplicateEmail(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	role := testsupport.SeedRole(t, pool, "r-create-dup")
	const existingEmail = "dup.email@test.local"
	seedUserDirect(t, pool, role, existingEmail)
	actor := seedUserDirect(t, pool, role, "actor.create-dup@test.local")

	body := map[string]any{
		"name":    "Dup User",
		"email":   existingEmail,
		"role_id": role.String(),
	}
	code, respBody := doJSON(t, h, http.MethodPost, "/api/v1/users", role, actor, body)
	assert.Equal(t, http.StatusConflict, code)
	assert.Contains(t, respBody, "error")

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM identity.users WHERE email = $1 AND deleted_at IS NULL`, existingEmail).
		Scan(&count))
	assert.Equal(t, 1, count, "duplicate create must not insert a second row")
}

// TestUser_Update_Success proves PUT /users/:id replaces the mutable fields
// (name/role_id/status) and that the change is persisted, not just echoed
// back in the response.
func TestUser_Update_Success(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	roleOld := testsupport.SeedRole(t, pool, "r-update-old")
	roleNew := testsupport.SeedRole(t, pool, "r-update-new")
	target := seedUserDirect(t, pool, roleOld, "update.target@test.local")
	actor := seedUserDirect(t, pool, roleOld, "actor.update-success@test.local")

	body := map[string]any{
		"name":    "Updated Name",
		"role_id": roleNew.String(),
		"status":  "inactive",
	}
	code, respBody := doJSON(t, h, http.MethodPut, "/api/v1/users/"+target.String(), roleOld, actor, body)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Updated Name", respBody["name"])
	assert.Equal(t, roleNew.String(), respBody["role_id"])
	assert.Equal(t, "inactive", respBody["status"])

	var dbName, dbStatus string
	var dbRole uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT name, role_id, status FROM identity.users WHERE id = $1 AND deleted_at IS NULL`, target).
		Scan(&dbName, &dbRole, &dbStatus))
	assert.Equal(t, "Updated Name", dbName)
	assert.Equal(t, roleNew, dbRole)
	assert.Equal(t, "inactive", dbStatus)
}

// TestUser_Update_NotFound proves PUT /users/:id responds 404 for an id that
// does not exist (handler fetches "before" via svc.Get first, mapping
// ErrNotFound through svcError).
func TestUser_Update_NotFound(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	role := testsupport.SeedRole(t, pool, "r-update-notfound")
	actor := seedUserDirect(t, pool, role, "actor.update-notfound@test.local")

	body := map[string]any{
		"name":    "Ghost",
		"role_id": role.String(),
		"status":  "active",
	}
	code, respBody := doJSON(t, h, http.MethodPut, "/api/v1/users/"+uuid.New().String(), role, actor, body)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, respBody, "error")
}

// TestUser_Delete_Success proves DELETE /users/:id responds 204, soft-deletes
// the row (deleted_at set, row no longer visible to GetUserByID's
// "deleted_at IS NULL" filter), and does not hard-delete it.
func TestUser_Delete_Success(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	role := testsupport.SeedRole(t, pool, "r-delete-success")
	target := seedUserDirect(t, pool, role, "delete.target@test.local")
	actor := seedUserDirect(t, pool, role, "actor.delete-success@test.local")

	code, body := doJSON(t, h, http.MethodDelete, "/api/v1/users/"+target.String(), role, actor, nil)
	require.Equal(t, http.StatusNoContent, code)
	assert.Empty(t, body)

	var isDeleted bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT deleted_at IS NOT NULL FROM identity.users WHERE id = $1`, target).
		Scan(&isDeleted))
	assert.True(t, isDeleted, "delete must soft-delete (set deleted_at), not leave the row live")

	var rowStillExists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM identity.users WHERE id = $1)`, target).
		Scan(&rowStillExists))
	assert.True(t, rowStillExists, "delete must not hard-delete the row")

	// GET after DELETE must now report not-found, proving the soft-delete is
	// honored by the read path too.
	getCode, _ := doRequest(t, h, http.MethodGet, "/api/v1/users/"+target.String(), role)
	assert.Equal(t, http.StatusNotFound, getCode)
}

// TestUser_Delete_NotFound proves DELETE /users/:id responds 404 for an id
// that does not exist (handler fetches "before" via svc.Get first).
func TestUser_Delete_NotFound(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	q := sqlc.New(pool)
	svc := user.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	h := user.NewHandler(svc, fieldSvc, auditSvc)
	gin.SetMode(gin.TestMode)

	role := testsupport.SeedRole(t, pool, "r-delete-notfound")
	actor := seedUserDirect(t, pool, role, "actor.delete-notfound@test.local")

	code, respBody := doJSON(t, h, http.MethodDelete, "/api/v1/users/"+uuid.New().String(), role, actor, nil)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, respBody, "error")
}
