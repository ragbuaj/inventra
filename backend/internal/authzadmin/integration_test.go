//go:build integration

package authzadmin_test

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
	"github.com/ragbuaj/inventra/internal/authzadmin"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── harness ─────────────────────────────────────────────────────────────────

// testHarness holds all shared plumbing for one integration test function.
type testHarness struct {
	pool        *pgxpool.Pool
	q           *sqlc.Queries
	permSvc     *authz.PermissionService
	scopeSvc    *authz.ScopeService
	fieldSvc    *authz.FieldService
	auditSvc    *audit.Service
	svc         *authzadmin.Service
	handler     *authzadmin.Handler
	adminUserID uuid.UUID
	adminRoleID uuid.UUID
	router      *gin.Engine
}

// newTestHarness spins up Postgres + Redis containers, runs migrations, seeds an
// acting-admin role with role.manage / scope.manage / fieldperm.manage, and wires
// the full gin engine with real RequirePermission middleware so cache behaviour is
// exercised end-to-end.
func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	// Each test gets a fresh container (NewPostgres spins up a new DB), so the
	// post-migration state is already clean — we do NOT call Reset here, which
	// would truncate identity.* and destroy the migration-seeded system roles
	// (is_system=true) that Case 4 relies on.
	ctx := context.Background()

	q := sqlc.New(pool)
	permSvc := authz.NewPermissionService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	fieldSvc := authz.NewFieldService(q, rdb)
	auditSvc := audit.NewService(q)
	svc := authzadmin.NewService(q, pool, permSvc, scopeSvc, fieldSvc)
	handler := authzadmin.NewHandler(svc, auditSvc)

	// Seed the acting-admin role with all three gate permissions.
	adminRoleID := testsupport.SeedRole(t, pool, "test-admin-"+uuid.New().String()[:8])
	for _, key := range []string{"role.manage", "scope.manage", "fieldperm.manage"} {
		_, err := pool.Exec(ctx,
			`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, $2)`,
			adminRoleID, key)
		require.NoError(t, err)
	}

	// Seed the acting-admin user.
	var adminUserID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, status)
		 VALUES ('Test Admin', 'admin@authzadmin.test', $1, 'active') RETURNING id`,
		adminRoleID).Scan(&adminUserID))

	// Stub auth MW: inject admin identity, bypassing real JWT.
	adminAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, adminUserID.String())
		c.Set(middleware.CtxRoleID, adminRoleID.String())
		c.Next()
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	authzadmin.RegisterRoutes(v1, handler,
		adminAuth,
		middleware.RequirePermission(permSvc, "role.manage"),
		middleware.RequirePermission(permSvc, "scope.manage"),
		middleware.RequirePermission(permSvc, "fieldperm.manage"),
	)

	return &testHarness{
		pool:        pool,
		q:           q,
		permSvc:     permSvc,
		scopeSvc:    scopeSvc,
		fieldSvc:    fieldSvc,
		auditSvc:    auditSvc,
		svc:         svc,
		handler:     handler,
		adminUserID: adminUserID,
		adminRoleID: adminRoleID,
		router:      router,
	}
}

// call performs an HTTP request against the harness router as the default admin.
func (h *testHarness) call(method, path string, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			panic(err)
		}
	}
	req, err := http.NewRequest(method, path, bytes.NewBuffer(b))
	if err != nil {
		panic(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// callAs performs an HTTP request as a specific user/role using a fresh router.
func (h *testHarness) callAs(userID, roleID uuid.UUID, method, path string, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			panic(err)
		}
	}
	req, err := http.NewRequest(method, path, bytes.NewBuffer(b))
	if err != nil {
		panic(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	authzadmin.RegisterRoutes(v1, h.handler,
		stubAuth,
		middleware.RequirePermission(h.permSvc, "role.manage"),
		middleware.RequirePermission(h.permSvc, "scope.manage"),
		middleware.RequirePermission(h.permSvc, "fieldperm.manage"),
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// body parses the response body into map[string]any.
func body(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m), "response body must be valid JSON: %s", w.Body.String())
	return m
}

// ─── tests ───────────────────────────────────────────────────────────────────

// Case 1: GET /authz/catalog → 200; body has permissions[], scope_levels (4 items), scope_modules starting with "*".
func TestAuthzAdmin_Catalog_OK(t *testing.T) {
	h := newTestHarness(t)
	w := h.call(http.MethodGet, "/api/v1/authz/catalog", nil)
	require.Equal(t, http.StatusOK, w.Code)

	b := body(t, w)

	perms, ok := b["permissions"].([]any)
	require.True(t, ok, "permissions must be a JSON array")
	assert.NotEmpty(t, perms, "catalog must have at least one permission group")

	levels, ok := b["scope_levels"].([]any)
	require.True(t, ok, "scope_levels must be a JSON array")
	assert.Len(t, levels, 4, "expected 4 scope levels: global, office_subtree, office, own")

	modules, ok := b["scope_modules"].([]any)
	require.True(t, ok, "scope_modules must be a JSON array")
	require.NotEmpty(t, modules, "scope_modules must not be empty")
	assert.Equal(t, "*", modules[0], "first scope_module must be the default sentinel '*'")
}

// Case 2: Role CRUD — POST → 201 is_system=false; GET → 200; PUT name → 200 reflects; list contains it.
func TestAuthzAdmin_RoleCRUD(t *testing.T) {
	h := newTestHarness(t)

	t.Run("POST creates role is_system=false", func(t *testing.T) {
		w := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
			"code": "auditor",
			"name": "Auditor",
		})
		require.Equal(t, http.StatusCreated, w.Code)
		b := body(t, w)
		assert.Equal(t, "auditor", b["code"])
		assert.Equal(t, "Auditor", b["name"])
		assert.Equal(t, false, b["is_system"])
		assert.NotEmpty(t, b["id"])
	})

	t.Run("GET retrieves created role", func(t *testing.T) {
		wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
			"code": "get-me",
			"name": "Get Me",
		})
		require.Equal(t, http.StatusCreated, wCreate.Code)
		id := body(t, wCreate)["id"].(string)

		wGet := h.call(http.MethodGet, "/api/v1/authz/roles/"+id, nil)
		require.Equal(t, http.StatusOK, wGet.Code)
		b := body(t, wGet)
		assert.Equal(t, id, b["id"])
		assert.Equal(t, "get-me", b["code"])
	})

	t.Run("PUT updates name and GET reflects it", func(t *testing.T) {
		wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
			"code": "upd-me",
			"name": "Original Name",
		})
		require.Equal(t, http.StatusCreated, wCreate.Code)
		id := body(t, wCreate)["id"].(string)

		wPut := h.call(http.MethodPut, "/api/v1/authz/roles/"+id, map[string]any{
			"name": "Updated Name",
		})
		require.Equal(t, http.StatusOK, wPut.Code)
		assert.Equal(t, "Updated Name", body(t, wPut)["name"])

		wGet := h.call(http.MethodGet, "/api/v1/authz/roles/"+id, nil)
		require.Equal(t, http.StatusOK, wGet.Code)
		assert.Equal(t, "Updated Name", body(t, wGet)["name"])
	})

	t.Run("GET list contains created role", func(t *testing.T) {
		wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
			"code": "list-me",
			"name": "List Me",
		})
		require.Equal(t, http.StatusCreated, wCreate.Code)
		id := body(t, wCreate)["id"].(string)

		wList := h.call(http.MethodGet, "/api/v1/authz/roles", nil)
		require.Equal(t, http.StatusOK, wList.Code)
		b := body(t, wList)
		data, ok := b["data"].([]any)
		require.True(t, ok, "data must be an array")

		found := false
		for _, item := range data {
			if item.(map[string]any)["id"].(string) == id {
				found = true
				break
			}
		}
		assert.True(t, found, "list must contain the newly created role")
		assert.Equal(t, float64(len(data)), b["total"], "total must match data length")
	})
}

// Case 3: POST same code twice → 409.
func TestAuthzAdmin_RoleCode_Conflict(t *testing.T) {
	h := newTestHarness(t)

	w1 := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "dup-code",
		"name": "First",
	})
	require.Equal(t, http.StatusCreated, w1.Code)

	w2 := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "dup-code",
		"name": "Second",
	})
	assert.Equal(t, http.StatusConflict, w2.Code, "duplicate role code must return 409 Conflict")
}

// Case 4: System-role is protected — DELETE → 409; PUT changing code → 409; PUT changing only name → 200.
func TestAuthzAdmin_SystemRole_Protected(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	// The migrations seed system roles (is_system = true). Find one.
	var sysRoleID uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT id FROM identity.roles WHERE is_system = true AND deleted_at IS NULL LIMIT 1`).
		Scan(&sysRoleID))
	id := sysRoleID.String()

	t.Run("DELETE system role returns 409", func(t *testing.T) {
		w := h.call(http.MethodDelete, "/api/v1/authz/roles/"+id, nil)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("PUT changing code returns 409 ErrSystemRole", func(t *testing.T) {
		// Fetch current state so we don't change the name accidentally.
		wGet := h.call(http.MethodGet, "/api/v1/authz/roles/"+id, nil)
		require.Equal(t, http.StatusOK, wGet.Code)
		current := body(t, wGet)

		w := h.call(http.MethodPut, "/api/v1/authz/roles/"+id, map[string]any{
			"code": "hacky-new-code",   // attempt to change code
			"name": current["name"].(string),
		})
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("PUT changing only name returns 200", func(t *testing.T) {
		wGet := h.call(http.MethodGet, "/api/v1/authz/roles/"+id, nil)
		require.Equal(t, http.StatusOK, wGet.Code)
		current := body(t, wGet)

		w := h.call(http.MethodPut, "/api/v1/authz/roles/"+id, map[string]any{
			"code": current["code"].(string),  // keep the same code
			"name": "System Role Renamed",
		})
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "System Role Renamed", body(t, w)["name"])
	})
}

// Case 5: DeleteRole_InUse — role assigned to a user → 409; soft-delete user → 204; GET → 404.
func TestAuthzAdmin_DeleteRole_InUse(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	// Create a custom role.
	wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "in-use-role",
		"name": "In Use Role",
	})
	require.Equal(t, http.StatusCreated, wCreate.Code)
	roleIDStr := body(t, wCreate)["id"].(string)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)

	// Assign a user to this role.
	var userID uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, status)
		 VALUES ('Placeholder User', 'inuse@authzadmin.test', $1, 'active') RETURNING id`,
		roleID).Scan(&userID))

	t.Run("DELETE role with active user returns 409", func(t *testing.T) {
		w := h.call(http.MethodDelete, "/api/v1/authz/roles/"+roleIDStr, nil)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	// Soft-delete the user so the role is no longer in use.
	_, err = h.pool.Exec(ctx,
		`UPDATE identity.users SET deleted_at = now() WHERE id = $1`, userID)
	require.NoError(t, err)

	t.Run("DELETE role after user deleted returns 204", func(t *testing.T) {
		w := h.call(http.MethodDelete, "/api/v1/authz/roles/"+roleIDStr, nil)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("GET deleted role returns 404", func(t *testing.T) {
		w := h.call(http.MethodGet, "/api/v1/authz/roles/"+roleIDStr, nil)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// Case 6: SetPermissions_ReplaceAndInvalidate — warm permSvc cache; PUT; cache reflects
// change IMMEDIATELY (proving invalidation); second PUT removing a key leaves it absent.
func TestAuthzAdmin_SetPermissions_ReplaceAndInvalidate(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	// Create a fresh role with no permissions.
	wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "perm-cache-role",
		"name": "Perm Cache Role",
	})
	require.Equal(t, http.StatusCreated, wCreate.Code)
	roleIDStr := body(t, wCreate)["id"].(string)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)

	// Warm the cache by calling Has — asset.manage must be absent.
	has, err := h.permSvc.Has(ctx, roleID, "asset.manage")
	require.NoError(t, err)
	require.False(t, has, "cache warm: role must not have asset.manage before PUT")

	// PUT permissions: grant asset.view + asset.manage.
	wPut := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/permissions", map[string]any{
		"permissions": []string{"asset.view", "asset.manage"},
	})
	require.Equal(t, http.StatusOK, wPut.Code)
	putBody := body(t, wPut)
	perms, ok := putBody["permissions"].([]any)
	require.True(t, ok, "response must have permissions array")
	permStrs := toStringSlice(perms)
	assert.Contains(t, permStrs, "asset.manage", "PUT response must include asset.manage")
	assert.Contains(t, permStrs, "asset.view", "PUT response must include asset.view")

	t.Run("asset.manage visible in cache immediately after first PUT", func(t *testing.T) {
		has, err := h.permSvc.Has(ctx, roleID, "asset.manage")
		require.NoError(t, err)
		assert.True(t, has, "asset.manage must be visible immediately via permSvc (cache invalidated)")
	})

	// Second PUT: remove asset.manage, keep only asset.view.
	wPut2 := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/permissions", map[string]any{
		"permissions": []string{"asset.view"},
	})
	require.Equal(t, http.StatusOK, wPut2.Code)

	t.Run("asset.manage absent in cache immediately after second PUT", func(t *testing.T) {
		has, err := h.permSvc.Has(ctx, roleID, "asset.manage")
		require.NoError(t, err)
		assert.False(t, has, "asset.manage must be gone immediately after second PUT (cache invalidated)")
	})

	t.Run("asset.view still present after second PUT", func(t *testing.T) {
		has, err := h.permSvc.Has(ctx, roleID, "asset.view")
		require.NoError(t, err)
		assert.True(t, has, "asset.view must remain after second PUT")
	})
}

// Case 7: SetPermissions_UnknownKey → 400.
func TestAuthzAdmin_SetPermissions_UnknownKey(t *testing.T) {
	h := newTestHarness(t)

	wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "unk-perm-role",
		"name": "Unknown Perm Role",
	})
	require.Equal(t, http.StatusCreated, wCreate.Code)
	roleID := body(t, wCreate)["id"].(string)

	w := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleID+"/permissions", map[string]any{
		"permissions": []string{"asset.create"}, // stale / unknown key
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "unknown permission key must return 400 Bad Request")
}

// Case 8: SetScope_ReplaceAndInvalidate — warm scopeSvc cache; PUT scope; Resolve reflects
// change IMMEDIATELY (proving invalidation); invalid scope_level → 400; dup module → 400.
func TestAuthzAdmin_SetScope_ReplaceAndInvalidate(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "scope-cache-role",
		"name": "Scope Cache Role",
	})
	require.Equal(t, http.StatusCreated, wCreate.Code)
	roleIDStr := body(t, wCreate)["id"].(string)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)

	// Warm the scope cache: no policy → resolves to "own" (conservative fallback).
	sc, err := h.scopeSvc.Resolve(ctx, roleID, nil, "assets")
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level, "pre-PUT: scope must be own (cold cache)")

	// PUT scope granting global for module "*".
	wPut := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/scope", map[string]any{
		"policies": []map[string]any{
			{"module": "*", "scope_level": "global"},
		},
	})
	require.Equal(t, http.StatusOK, wPut.Code)
	putBody := body(t, wPut)
	policies, ok := putBody["policies"].([]any)
	require.True(t, ok, "response must have policies array")
	require.Len(t, policies, 1, "response must reflect one policy")
	p0 := policies[0].(map[string]any)
	assert.Equal(t, "*", p0["module"])
	assert.Equal(t, "global", p0["scope_level"])

	t.Run("scope resolves to global immediately after PUT", func(t *testing.T) {
		sc, err := h.scopeSvc.Resolve(ctx, roleID, nil, "assets")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelGlobal, sc.Level,
			"scope must be global immediately via scopeSvc (cache invalidated)")
	})

	t.Run("invalid scope_level returns 400", func(t *testing.T) {
		w := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/scope", map[string]any{
			"policies": []map[string]any{
				{"module": "*", "scope_level": "universe"}, // not a valid level
			},
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("duplicate module in policies returns 400", func(t *testing.T) {
		w := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/scope", map[string]any{
			"policies": []map[string]any{
				{"module": "*", "scope_level": "global"},
				{"module": "*", "scope_level": "own"}, // duplicate module
			},
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Case 9: SetFields_ReplaceAndInvalidate — warm fieldSvc cache; PUT fields;
// ForEntity reflects change IMMEDIATELY; empty entity → 400; dup (entity,field) → 400.
func TestAuthzAdmin_SetFields_ReplaceAndInvalidate(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "field-cache-role",
		"name": "Field Cache Role",
	})
	require.Equal(t, http.StatusCreated, wCreate.Code)
	roleIDStr := body(t, wCreate)["id"].(string)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)

	// Warm the field cache: no policies → empty.
	pol, err := h.fieldSvc.ForEntity(ctx, roleID, "assets")
	require.NoError(t, err)
	require.Empty(t, pol, "pre-PUT: field policy must be empty (cold cache)")

	// PUT fields hiding purchase_cost.
	wPut := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/fields", map[string]any{
		"fields": []map[string]any{
			{"entity": "assets", "field": "purchase_cost", "can_view": false, "can_edit": false},
		},
	})
	require.Equal(t, http.StatusOK, wPut.Code)
	putBody := body(t, wPut)
	fields, ok := putBody["fields"].([]any)
	require.True(t, ok, "response must have fields array")
	require.Len(t, fields, 1)
	f0 := fields[0].(map[string]any)
	assert.Equal(t, "assets", f0["entity"])
	assert.Equal(t, "purchase_cost", f0["field"])
	assert.Equal(t, false, f0["can_view"])
	assert.Equal(t, false, f0["can_edit"])

	t.Run("purchase_cost appears in field policy immediately after PUT", func(t *testing.T) {
		pol, err := h.fieldSvc.ForEntity(ctx, roleID, "assets")
		require.NoError(t, err)
		require.Contains(t, pol, "purchase_cost",
			"purchase_cost must appear in fieldSvc immediately (cache invalidated)")
		assert.False(t, pol["purchase_cost"].CanView)
		assert.False(t, pol["purchase_cost"].CanEdit)
	})

	t.Run("empty entity returns 400", func(t *testing.T) {
		w := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/fields", map[string]any{
			"fields": []map[string]any{
				{"entity": "", "field": "purchase_cost", "can_view": false},
			},
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("duplicate entity+field returns 400", func(t *testing.T) {
		w := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/fields", map[string]any{
			"fields": []map[string]any{
				{"entity": "assets", "field": "purchase_cost", "can_view": false},
				{"entity": "assets", "field": "purchase_cost", "can_view": true}, // dup
			},
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Case 10: Audit_Recorded — after a role create + a permissions PUT, audit_logs contains
// rows with entity_type "roles" and "role_permissions" for the acting admin user.
func TestAuthzAdmin_Audit_Recorded(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	// Create role via HTTP.
	wCreate := h.call(http.MethodPost, "/api/v1/authz/roles", map[string]any{
		"code": "audit-role",
		"name": "Audit Role",
	})
	require.Equal(t, http.StatusCreated, wCreate.Code)
	roleIDStr := body(t, wCreate)["id"].(string)

	// PUT permissions via HTTP.
	wPerms := h.call(http.MethodPut, "/api/v1/authz/roles/"+roleIDStr+"/permissions", map[string]any{
		"permissions": []string{"asset.view"},
	})
	require.Equal(t, http.StatusOK, wPerms.Code)

	// audit_logs is append-only (no deleted_at). Verify it has a "roles" create entry
	// for the acting admin.
	var countRoles int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit.audit_logs
		 WHERE entity_type = 'roles'
		   AND actor_id = $1
		   AND action = 'create'`,
		h.adminUserID).Scan(&countRoles))
	assert.GreaterOrEqual(t, countRoles, 1,
		"audit_logs must have at least one 'create' row for entity_type=roles")

	// Verify audit_logs has a "role_permissions" update entry for the acting admin.
	var countPerms int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit.audit_logs
		 WHERE entity_type = 'role_permissions'
		   AND actor_id = $1
		   AND action = 'update'`,
		h.adminUserID).Scan(&countPerms))
	assert.GreaterOrEqual(t, countPerms, 1,
		"audit_logs must have at least one 'update' row for entity_type=role_permissions")
}

// Case 11: Forbidden_WithoutPermission — a role lacking role.manage receives 403 on
// GET /authz/roles, and the response carries required_permission = "role.manage".
func TestAuthzAdmin_Forbidden_WithoutPermission(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	// Seed a role with no gate permissions (only asset.view, not role.manage).
	unprivRoleID := testsupport.SeedRole(t, h.pool, "unpriv-"+uuid.New().String()[:8])
	_, err := h.pool.Exec(ctx,
		`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, 'asset.view')`,
		unprivRoleID)
	require.NoError(t, err)

	// Seed a user with the unprivileged role.
	var unprivUserID uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, status)
		 VALUES ('Unpriv User', 'unpriv@authzadmin.test', $1, 'active') RETURNING id`,
		unprivRoleID).Scan(&unprivUserID))

	w := h.callAs(unprivUserID, unprivRoleID, http.MethodGet, "/api/v1/authz/roles", nil)
	require.Equal(t, http.StatusForbidden, w.Code,
		"role without role.manage must receive 403 Forbidden")

	b := body(t, w)
	assert.Equal(t, "forbidden", b["error"])
	assert.Equal(t, "role.manage", b["required_permission"],
		"response must name the missing permission")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// toStringSlice converts a []any (from JSON unmarshalling) to []string.
func toStringSlice(a []any) []string {
	out := make([]string, 0, len(a))
	for _, v := range a {
		out = append(out, v.(string))
	}
	return out
}
