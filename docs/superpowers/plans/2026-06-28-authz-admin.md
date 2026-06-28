# Authorization Admin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Superadmin module to manage roles, role-permissions, data-scope policies, and field-permissions over HTTP — with correct Redis cache invalidation, a canonical permission catalog, and a seed fix so built-in roles actually work.

**Architecture:** New `internal/authzadmin` package (four-file split) consuming `*sqlc.Queries`, `*pgxpool.Pool` (for transactional replace-set), and the three `internal/authz` services (to invalidate their caches after mutations). `internal/authz` stays the pure engine — it only gains two `Invalidate` methods. Permission keys are validated against a canonical Go catalog; the `000005` seed is realigned to the keys the code actually enforces.

**Tech Stack:** Go 1.25, Gin, pgx/v5, sqlc, Redis (go-redis), testify + testcontainers (integration).

## Global Constraints

- Manage 4 resources: `identity.roles`, `role_permissions`, `data_scope_policies`, `field_permissions`. Full CRUD on custom roles; built-in roles (`is_system=true`) are protected (no delete, `code` immutable).
- Permission gates (already enforced via `middleware.RequirePermission`): roles + role_permissions → `role.manage`; data scope → `scope.manage`; field permissions → `fieldperm.manage`; catalog → `role.manage`. Mount under `/api/v1/authz`.
- Permission keys assigned to a role MUST be in the canonical catalog (else 400). Catalog is the single source of truth.
- Replace-set semantics for permissions/scope/fields: in one transaction, soft-delete the role's active rows, then insert the new set; then invalidate the relevant service cache for that role. Empty set allowed.
- Cache invalidation is mandatory after every mutation: role_permissions→`PermissionService.Invalidate`; scope→`ScopeService.Invalidate`; fields→`FieldService.Invalidate`; role delete→all three.
- Roles: `code` unique (23505→409 conflict); `is_system` role → `code` immutable + cannot delete; custom role delete rejected if any user references it (409).
- Audit every mutation: entity `roles`/`role_permissions`/`data_scope_policies`/`field_permissions`, `officeID = nil` (global config), `audit.Diff(before, after)`.
- Do not hand-edit `backend/db/sqlc/` — edit `db/queries/identity.sql` then `sqlc generate`.
- Scope levels enum: `global`, `office_subtree`, `office`, `own`. Known scope modules (verified via grep of `CallerOfficeScope`/`scopeModule`): `*`, `offices`, `employees`, `assets`, `requests`.
- Verify gates: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./...`, Spectral lint — all green.

---

### Task 1: `authz` cache invalidation (ScopeService + FieldService)

**Files:**
- Modify: `backend/internal/authz/scope.go` (add `Invalidate`)
- Modify: `backend/internal/authz/fields.go` (add `Invalidate`)
- Test: `backend/internal/authz/invalidate_integration_test.go` (new, `//go:build integration`)

**Interfaces:**
- Produces: `(*ScopeService).Invalidate(ctx context.Context, roleID uuid.UUID) error`; `(*FieldService).Invalidate(ctx context.Context, roleID uuid.UUID) error`.

- [ ] **Step 1: Write the failing integration test**

Create `backend/internal/authz/invalidate_integration_test.go` (mirror `scope_integration_test.go` harness: `testsupport.NewPostgres`/`NewRedis`/`Reset`/seed helpers):

```go
//go:build integration

package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestScopeService_Invalidate(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	testsupport.Reset(t, pool)

	role := testsupport.SeedRole(t, pool, "r-inval-scope")
	testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOwn)
	q := sqlc.New(pool)
	svc := authz.NewScopeService(q, rdb)

	// Warm the cache: resolves to 'own'.
	sc, err := svc.Resolve(ctx, role, nil, "assets")
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)

	// Change the policy directly in the DB to 'global'.
	_, err = pool.Exec(ctx, `UPDATE identity.data_scope_policies SET scope_level='global' WHERE role_id=$1 AND module='*'`, role)
	require.NoError(t, err)

	// Still cached as 'own' until invalidated.
	sc, _ = svc.Resolve(ctx, role, nil, "assets")
	require.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)

	require.NoError(t, svc.Invalidate(ctx, role))

	sc, err = svc.Resolve(ctx, role, nil, "assets")
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedScopeLevelGlobal, sc.Level)
}

func TestFieldService_Invalidate(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	testsupport.Reset(t, pool)

	role := testsupport.SeedRole(t, pool, "r-inval-field")
	q := sqlc.New(pool)
	svc := authz.NewFieldService(q, rdb)

	// Warm cache: no policy for entity "assets" -> empty.
	pol, err := svc.ForEntity(ctx, role, "assets")
	require.NoError(t, err)
	require.Empty(t, pol)

	// Insert a hiding policy directly.
	_, err = pool.Exec(ctx, `INSERT INTO identity.field_permissions (entity, field, role_id, can_view, can_edit) VALUES ('assets','purchase_cost',$1,false,false)`, role)
	require.NoError(t, err)

	// Still cached empty.
	pol, _ = svc.ForEntity(ctx, role, "assets")
	require.Empty(t, pol)

	require.NoError(t, svc.Invalidate(ctx, role))

	pol, err = svc.ForEntity(ctx, role, "assets")
	require.NoError(t, err)
	require.Contains(t, pol, "purchase_cost")
}
```

> If `testsupport.SeedRole`/`SeedScopePolicy` signatures differ, read `internal/testsupport/` and adapt the calls. The assertion logic stays the same.

- [ ] **Step 2: Run to verify it fails**

Run (from `backend/`): `go test -tags=integration ./internal/authz/ -run 'TestScopeService_Invalidate|TestFieldService_Invalidate' -v`
Expected: FAIL — `svc.Invalidate undefined`.

- [ ] **Step 3: Add the two methods**

In `backend/internal/authz/scope.go`, append:

```go
// Invalidate clears the cached data-scope policies for a role
// (call after changing data_scope_policies). The office-subtree cache is
// keyed by office, not role, so it is unaffected by policy changes.
func (s *ScopeService) Invalidate(ctx context.Context, roleID uuid.UUID) error {
	return s.rdb.Del(ctx, "authz:scope:"+roleID.String()).Err()
}
```

In `backend/internal/authz/fields.go`, append:

```go
// Invalidate clears the cached field permissions for a role
// (call after changing field_permissions).
func (s *FieldService) Invalidate(ctx context.Context, roleID uuid.UUID) error {
	return s.rdb.Del(ctx, "authz:fields:"+roleID.String()).Err()
}
```

- [ ] **Step 4: Run to verify it passes**

Run (from `backend/`): `go test -tags=integration ./internal/authz/ -run 'TestScopeService_Invalidate|TestFieldService_Invalidate' -v`
Expected: PASS (Docker required).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/authz/scope.go backend/internal/authz/fields.go backend/internal/authz/invalidate_integration_test.go
git commit -m "feat(authz): cache invalidation for scope and field services"
```

---

### Task 2: Permission catalog + validation helpers (pure, unit-tested)

**Files:**
- Create: `backend/internal/authzadmin/catalog.go`
- Test: `backend/internal/authzadmin/catalog_test.go`

**Interfaces:**
- Produces: types `PermissionItem{Key,Label string}`, `PermissionGroup{Group string; Items []PermissionItem}`; `var permissionCatalog []PermissionGroup`; `IsKnownPermission(key string) bool`; `ScopeLevels() []string`; `ScopeModules() []string`; `CatalogResponse() map[string]any`.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/authzadmin/catalog_test.go`:

```go
package authzadmin

import "testing"

func TestCatalog_NoDuplicatesAndLabeled(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range permissionCatalog {
		if g.Group == "" {
			t.Error("group name must not be empty")
		}
		if len(g.Items) == 0 {
			t.Errorf("group %q has no items", g.Group)
		}
		for _, it := range g.Items {
			if it.Key == "" || it.Label == "" {
				t.Errorf("item in %q missing key or label: %+v", g.Group, it)
			}
			if seen[it.Key] {
				t.Errorf("duplicate permission key: %s", it.Key)
			}
			seen[it.Key] = true
		}
	}
}

func TestIsKnownPermission(t *testing.T) {
	for _, k := range []string{"asset.view", "asset.manage", "role.manage", "request.decide", "approval.config.manage"} {
		if !IsKnownPermission(k) {
			t.Errorf("%s should be known", k)
		}
	}
	for _, k := range []string{"asset.create", "request.approve", "bogus.key", ""} {
		if IsKnownPermission(k) {
			t.Errorf("%s should NOT be known", k)
		}
	}
}

func TestCatalogResponse_Shape(t *testing.T) {
	r := CatalogResponse()
	if _, ok := r["permissions"]; !ok {
		t.Error("missing permissions")
	}
	levels, _ := r["scope_levels"].([]string)
	if len(levels) != 4 {
		t.Errorf("want 4 scope levels, got %v", levels)
	}
	mods, _ := r["scope_modules"].([]string)
	if len(mods) == 0 || mods[0] != "*" {
		t.Errorf("scope_modules should start with '*', got %v", mods)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run (from `backend/`): `go test ./internal/authzadmin/ -run 'TestCatalog|TestIsKnown' -v`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Write `catalog.go`**

Create `backend/internal/authzadmin/catalog.go`:

```go
// Package authzadmin implements Superadmin management of the configurable
// authorization layer: roles, role-permissions, data-scope policies, and
// field-permissions, with Redis cache invalidation on every change.
package authzadmin

// PermissionItem is one assignable permission key with a human label.
type PermissionItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// PermissionGroup groups related permission keys for display.
type PermissionGroup struct {
	Group string           `json:"group"`
	Items []PermissionItem `json:"items"`
}

// permissionCatalog is the canonical source of truth for assignable permission
// keys. Keys here must match what the code enforces via RequirePermission; the
// "Cadangan" group reserves keys for modules not yet built so the seed can grant
// them forward-looking without failing validation.
var permissionCatalog = []PermissionGroup{
	{Group: "Sistem", Items: []PermissionItem{
		{"user.manage", "Kelola user"},
		{"role.manage", "Kelola peran & RBAC"},
		{"scope.manage", "Kelola data scope"},
		{"fieldperm.manage", "Kelola field permission"},
		{"audit.view", "Lihat audit trail"},
	}},
	{Group: "Master Data", Items: []PermissionItem{
		{"masterdata.global.manage", "Kelola master data global"},
		{"masterdata.office.manage", "Kelola kantor & pegawai"},
	}},
	{Group: "Aset", Items: []PermissionItem{
		{"asset.view", "Lihat aset"},
		{"asset.manage", "Kelola aset"},
	}},
	{Group: "Persetujuan", Items: []PermissionItem{
		{"request.create", "Buat pengajuan"},
		{"request.decide", "Setujui/tolak pengajuan"},
		{"approval.config.manage", "Kelola ambang persetujuan"},
	}},
	{Group: "Cadangan", Items: []PermissionItem{
		{"report.view", "Lihat laporan"},
		{"report.export", "Ekspor laporan"},
		{"maintenance.manage", "Kelola maintenance"},
		{"depreciation.manage", "Kelola penyusutan"},
		{"valuation.exclude.approve", "Setujui pengecualian valuasi"},
		{"assignment.manage", "Kelola penugasan aset"},
	}},
}

// knownPermissions is the flattened set of catalog keys for O(1) validation.
var knownPermissions = func() map[string]bool {
	m := map[string]bool{}
	for _, g := range permissionCatalog {
		for _, it := range g.Items {
			m[it.Key] = true
		}
	}
	return m
}()

// IsKnownPermission reports whether key is an assignable catalog permission.
func IsKnownPermission(key string) bool { return knownPermissions[key] }

// ScopeLevels returns the valid data-scope levels (matches shared.scope_level enum).
func ScopeLevels() []string {
	return []string{"global", "office_subtree", "office", "own"}
}

// ScopeModules returns the known data-scope module strings the handlers resolve
// scope for, plus the '*' default sentinel.
func ScopeModules() []string {
	return []string{"*", "offices", "employees", "assets", "requests"}
}

// CatalogResponse is the GET /authz/catalog payload for the admin UI.
func CatalogResponse() map[string]any {
	return map[string]any{
		"permissions":   permissionCatalog,
		"scope_levels":  ScopeLevels(),
		"scope_modules": ScopeModules(),
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run (from `backend/`): `go test ./internal/authzadmin/ -run 'TestCatalog|TestIsKnown' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/authzadmin/catalog.go backend/internal/authzadmin/catalog_test.go
git commit -m "feat(authz): canonical permission catalog + validation"
```

---

### Task 3: sqlc queries for authz-admin

**Files:**
- Modify: `backend/db/queries/identity.sql` (append)
- Generated: `backend/db/sqlc/*` (via `sqlc generate`)

**Interfaces:**
- Produces (generated `Queries` methods): `GetRole(id)`, `CreateRole(CreateRoleParams{Code,Name,Description})`, `UpdateRole(UpdateRoleParams{ID,Code,Name,Description})`, `SoftDeleteRole(id) (int64)`, `CountUsersByRole(roleID) (int64)`, `InsertRolePermission(InsertRolePermissionParams{RoleID,PermissionKey})`, `SoftDeleteRolePermissionsByRole(roleID) (int64)`, `InsertDataScopePolicy(InsertDataScopePolicyParams{RoleID,Module,ScopeLevel})`, `SoftDeleteDataScopePoliciesByRole(roleID) (int64)`, `InsertFieldPermission(InsertFieldPermissionParams{Entity,Field,RoleID,CanView,CanEdit})`, `SoftDeleteFieldPermissionsByRole(roleID) (int64)`.

- [ ] **Step 1: Append the queries**

Append to `backend/db/queries/identity.sql`:

```sql
-- name: GetRole :one
SELECT * FROM identity.roles WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateRole :one
INSERT INTO identity.roles (code, name, description, is_system)
VALUES ($1, $2, $3, false)
RETURNING *;

-- name: UpdateRole :one
UPDATE identity.roles
SET code = $2, name = $3, description = $4
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteRole :execrows
UPDATE identity.roles SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: CountUsersByRole :one
SELECT count(*) FROM identity.users WHERE role_id = $1 AND deleted_at IS NULL;

-- name: InsertRolePermission :one
INSERT INTO identity.role_permissions (role_id, permission_key)
VALUES ($1, $2)
RETURNING *;

-- name: SoftDeleteRolePermissionsByRole :execrows
UPDATE identity.role_permissions SET deleted_at = now()
WHERE role_id = $1 AND deleted_at IS NULL;

-- name: InsertDataScopePolicy :one
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
VALUES ($1, $2, $3)
RETURNING *;

-- name: SoftDeleteDataScopePoliciesByRole :execrows
UPDATE identity.data_scope_policies SET deleted_at = now()
WHERE role_id = $1 AND deleted_at IS NULL;

-- name: InsertFieldPermission :one
INSERT INTO identity.field_permissions (entity, field, role_id, can_view, can_edit)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: SoftDeleteFieldPermissionsByRole :execrows
UPDATE identity.field_permissions SET deleted_at = now()
WHERE role_id = $1 AND deleted_at IS NULL;
```

- [ ] **Step 2: Regenerate + build**

Run (from `backend/`): `sqlc generate && go build ./...`
Expected: exit 0. If `sqlc` is not on PATH, report BLOCKED with the exact error.

- [ ] **Step 3: Commit**

```bash
git add backend/db/queries/identity.sql backend/db/sqlc/
git commit -m "feat(authz): sqlc queries for role/permission/scope/field admin"
```

---

### Task 4: authzadmin service layer

**Files:**
- Create: `backend/internal/authzadmin/service.go`
- Test: `backend/internal/authzadmin/service_test.go`

**Interfaces:**
- Consumes: `IsKnownPermission` (Task 2); generated queries (Task 3); `authz.PermissionService`/`ScopeService`/`FieldService` (with `Invalidate`); `sqlc.Queries.WithTx`.
- Produces: sentinel errors `ErrNotFound`, `ErrConflict`, `ErrSystemRole`, `ErrRoleInUse`, `ErrUnknownPermission`, `ErrValidation`; `Service` + `NewService`; input types `RoleInput{Code,Name,Description *string}`, `ScopePolicyInput{Module string; ScopeLevel string}`, `FieldPermInput{Entity,Field string; CanView,CanEdit bool}`; methods `ListRoles`, `GetRole`, `CreateRole`, `UpdateRole`, `DeleteRole`, `GetRolePermissions`, `SetRolePermissions`, `GetScopePolicies`, `SetScopePolicies`, `GetFieldPermissions`, `SetFieldPermissions`; pure validators `dedupePermissions`, `validateScopePolicies`, `validateFieldPerms`.

- [ ] **Step 1: Write the failing unit tests (pure validators + is_system rules)**

Create `backend/internal/authzadmin/service_test.go`:

```go
package authzadmin

import (
	"errors"
	"testing"
)

func TestDedupePermissions_RejectsUnknownAndDeduplicates(t *testing.T) {
	out, err := dedupePermissions([]string{"asset.view", "asset.view", "role.manage"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 deduped, got %v", out)
	}
	if _, err := dedupePermissions([]string{"asset.view", "bogus.key"}); !errors.Is(err, ErrUnknownPermission) {
		t.Fatalf("want ErrUnknownPermission, got %v", err)
	}
}

func TestValidateScopePolicies(t *testing.T) {
	ok, err := validateScopePolicies([]ScopePolicyInput{{Module: "*", ScopeLevel: "global"}, {Module: "assets", ScopeLevel: "own"}})
	if err != nil || len(ok) != 2 {
		t.Fatalf("expected 2 valid, got %v err=%v", ok, err)
	}
	// invalid level
	if _, err := validateScopePolicies([]ScopePolicyInput{{Module: "*", ScopeLevel: "nope"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for bad level, got %v", err)
	}
	// empty module
	if _, err := validateScopePolicies([]ScopePolicyInput{{Module: "", ScopeLevel: "own"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for empty module, got %v", err)
	}
	// duplicate module
	if _, err := validateScopePolicies([]ScopePolicyInput{{Module: "assets", ScopeLevel: "own"}, {Module: "assets", ScopeLevel: "global"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for dup module, got %v", err)
	}
}

func TestValidateFieldPerms(t *testing.T) {
	if _, err := validateFieldPerms([]FieldPermInput{{Entity: "", Field: "x", CanView: true}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for empty entity, got %v", err)
	}
	if _, err := validateFieldPerms([]FieldPermInput{{Entity: "assets", Field: "cost"}, {Entity: "assets", Field: "cost"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for dup (entity,field), got %v", err)
	}
	ok, err := validateFieldPerms([]FieldPermInput{{Entity: "assets", Field: "purchase_cost", CanView: false}})
	if err != nil || len(ok) != 1 {
		t.Fatalf("expected 1 valid, got %v err=%v", ok, err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run (from `backend/`): `go test ./internal/authzadmin/ -run 'TestDedupe|TestValidate' -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Write `service.go`**

Create `backend/internal/authzadmin/service.go`:

```go
package authzadmin

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
)

var (
	ErrNotFound          = errors.New("role not found")
	ErrConflict          = errors.New("role code already in use")
	ErrSystemRole        = errors.New("system role cannot be modified or deleted")
	ErrRoleInUse         = errors.New("role is assigned to one or more users")
	ErrUnknownPermission = errors.New("unknown permission key")
	ErrValidation        = errors.New("invalid request")
)

// Service manages roles and the three authorization-config tables, invalidating
// the authz caches after each mutation.
type Service struct {
	q     *sqlc.Queries
	pool  *pgxpool.Pool
	perm  *authz.PermissionService
	scope *authz.ScopeService
	field *authz.FieldService
}

// NewService builds the authzadmin Service.
func NewService(q *sqlc.Queries, pool *pgxpool.Pool, perm *authz.PermissionService, scope *authz.ScopeService, field *authz.FieldService) *Service {
	return &Service{q: q, pool: pool, perm: perm, scope: scope, field: field}
}

// RoleInput holds create/update fields for a role.
type RoleInput struct {
	Code        string
	Name        string
	Description *string
}

// ScopePolicyInput is one data-scope policy row in a replace-set.
type ScopePolicyInput struct {
	Module     string
	ScopeLevel string
}

// FieldPermInput is one field-permission row in a replace-set.
type FieldPermInput struct {
	Entity  string
	Field   string
	CanView bool
	CanEdit bool
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

// ── Roles ────────────────────────────────────────────────────────────────────

func (s *Service) ListRoles(ctx context.Context) ([]sqlc.IdentityRole, error) {
	rows, err := s.q.ListRoles(ctx)
	return rows, err
}

func (s *Service) GetRole(ctx context.Context, id uuid.UUID) (sqlc.IdentityRole, error) {
	r, err := s.q.GetRole(ctx, id)
	return r, mapDBError(err)
}

func (s *Service) CreateRole(ctx context.Context, in RoleInput) (sqlc.IdentityRole, error) {
	if in.Code == "" || in.Name == "" {
		return sqlc.IdentityRole{}, ErrValidation
	}
	r, err := s.q.CreateRole(ctx, sqlc.CreateRoleParams{Code: in.Code, Name: in.Name, Description: in.Description})
	return r, mapDBError(err)
}

func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, in RoleInput) (before, after sqlc.IdentityRole, err error) {
	if in.Name == "" {
		return before, after, ErrValidation
	}
	before, err = s.q.GetRole(ctx, id)
	if err != nil {
		return before, after, mapDBError(err)
	}
	code := in.Code
	if before.IsSystem {
		// System roles: code is immutable. Reject an attempted change; otherwise keep it.
		if in.Code != "" && in.Code != before.Code {
			return before, after, ErrSystemRole
		}
		code = before.Code
	}
	after, err = s.q.UpdateRole(ctx, sqlc.UpdateRoleParams{ID: id, Code: code, Name: in.Name, Description: in.Description})
	return before, after, mapDBError(err)
}

// DeleteRole soft-deletes a custom role and cascade-soft-deletes its config rows,
// then invalidates all three caches. System or in-use roles are rejected.
func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) (sqlc.IdentityRole, error) {
	role, err := s.q.GetRole(ctx, id)
	if err != nil {
		return role, mapDBError(err)
	}
	if role.IsSystem {
		return role, ErrSystemRole
	}
	n, err := s.q.CountUsersByRole(ctx, id)
	if err != nil {
		return role, err
	}
	if n > 0 {
		return role, ErrRoleInUse
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return role, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteRolePermissionsByRole(ctx, id); err != nil {
		return role, err
	}
	if _, err := qtx.SoftDeleteDataScopePoliciesByRole(ctx, id); err != nil {
		return role, err
	}
	if _, err := qtx.SoftDeleteFieldPermissionsByRole(ctx, id); err != nil {
		return role, err
	}
	rows, err := qtx.SoftDeleteRole(ctx, id)
	if err != nil {
		return role, err
	}
	if rows == 0 {
		return role, ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return role, err
	}
	_ = s.perm.Invalidate(ctx, id)
	_ = s.scope.Invalidate(ctx, id)
	_ = s.field.Invalidate(ctx, id)
	return role, nil
}

// ── Role permissions ─────────────────────────────────────────────────────────

func (s *Service) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return nil, mapDBError(err)
	}
	return s.q.ListRolePermissions(ctx, roleID)
}

// SetRolePermissions replaces the role's permission set (validated against the catalog).
func (s *Service) SetRolePermissions(ctx context.Context, roleID uuid.UUID, keys []string) error {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return mapDBError(err)
	}
	clean, err := dedupePermissions(keys)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteRolePermissionsByRole(ctx, roleID); err != nil {
		return err
	}
	for _, k := range clean {
		if _, err := qtx.InsertRolePermission(ctx, sqlc.InsertRolePermissionParams{RoleID: roleID, PermissionKey: k}); err != nil {
			return mapDBError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.perm.Invalidate(ctx, roleID)
}

// ── Data scope ───────────────────────────────────────────────────────────────

func (s *Service) GetScopePolicies(ctx context.Context, roleID uuid.UUID) ([]sqlc.IdentityDataScopePolicy, error) {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return nil, mapDBError(err)
	}
	return s.q.ListDataScopePolicies(ctx, roleID)
}

func (s *Service) SetScopePolicies(ctx context.Context, roleID uuid.UUID, in []ScopePolicyInput) error {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return mapDBError(err)
	}
	clean, err := validateScopePolicies(in)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteDataScopePoliciesByRole(ctx, roleID); err != nil {
		return err
	}
	for _, p := range clean {
		if _, err := qtx.InsertDataScopePolicy(ctx, sqlc.InsertDataScopePolicyParams{
			RoleID: roleID, Module: p.Module, ScopeLevel: sqlc.SharedScopeLevel(p.ScopeLevel),
		}); err != nil {
			return mapDBError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.scope.Invalidate(ctx, roleID)
}

// ── Field permissions ────────────────────────────────────────────────────────

func (s *Service) GetFieldPermissions(ctx context.Context, roleID uuid.UUID) ([]sqlc.IdentityFieldPermission, error) {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return nil, mapDBError(err)
	}
	return s.q.ListFieldPermissionsByRole(ctx, roleID)
}

func (s *Service) SetFieldPermissions(ctx context.Context, roleID uuid.UUID, in []FieldPermInput) error {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return mapDBError(err)
	}
	clean, err := validateFieldPerms(in)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteFieldPermissionsByRole(ctx, roleID); err != nil {
		return err
	}
	for _, f := range clean {
		if _, err := qtx.InsertFieldPermission(ctx, sqlc.InsertFieldPermissionParams{
			Entity: f.Entity, Field: f.Field, RoleID: roleID, CanView: f.CanView, CanEdit: f.CanEdit,
		}); err != nil {
			return mapDBError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.field.Invalidate(ctx, roleID)
}

// ── Pure validators ──────────────────────────────────────────────────────────

func dedupePermissions(keys []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !IsKnownPermission(k) {
			return nil, ErrUnknownPermission
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}

var validScopeLevel = map[string]bool{"global": true, "office_subtree": true, "office": true, "own": true}

func validateScopePolicies(in []ScopePolicyInput) ([]ScopePolicyInput, error) {
	seen := map[string]bool{}
	out := make([]ScopePolicyInput, 0, len(in))
	for _, p := range in {
		if p.Module == "" || !validScopeLevel[p.ScopeLevel] {
			return nil, ErrValidation
		}
		if seen[p.Module] {
			return nil, ErrValidation
		}
		seen[p.Module] = true
		out = append(out, p)
	}
	return out, nil
}

func validateFieldPerms(in []FieldPermInput) ([]FieldPermInput, error) {
	seen := map[string]bool{}
	out := make([]FieldPermInput, 0, len(in))
	for _, f := range in {
		if f.Entity == "" || f.Field == "" {
			return nil, ErrValidation
		}
		key := f.Entity + "\x00" + f.Field
		if seen[key] {
			return nil, ErrValidation
		}
		seen[key] = true
		out = append(out, f)
	}
	return out, nil
}
```

- [ ] **Step 4: Run to verify it passes + build/vet**

Run (from `backend/`): `go test ./internal/authzadmin/ -run 'TestDedupe|TestValidate|TestCatalog|TestIsKnown' -v && go build ./... && go vet ./...`
Expected: PASS, exit 0.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/authzadmin/service.go backend/internal/authzadmin/service_test.go
git commit -m "feat(authz): authzadmin service (role CRUD + replace-set + invalidation)"
```

---

### Task 5: DTO + handler + routes + router wiring

**Files:**
- Create: `backend/internal/authzadmin/dto.go`
- Create: `backend/internal/authzadmin/handler.go`
- Create: `backend/internal/authzadmin/routes.go`
- Modify: `backend/internal/server/router.go` (construct + register)
- Test: `backend/internal/authzadmin/dto_test.go`

**Interfaces:**
- Consumes: service methods + input types (Task 4); `audit.Record`/`audit.ActionCreate|Update|Delete`/`audit.Diff`; `middleware.RequirePermission`/`RequireAuth`.
- Produces: request structs `roleCreateRequest`/`roleUpdateRequest`/`permissionsRequest`/`scopeRequest`/`fieldsRequest`; `roleToMap`; `Handler` + `NewHandler`; `RegisterRoutes(rg, h, authMW, requireRole, requireScope, requireField gin.HandlerFunc)`.

- [ ] **Step 1: Write the failing DTO test**

Create `backend/internal/authzadmin/dto_test.go`:

```go
package authzadmin

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRoleToMap(t *testing.T) {
	desc := "desc"
	r := sqlc.IdentityRole{
		ID: uuid.New(), Code: "auditor", Name: "Auditor", Description: &desc, IsSystem: false,
		CreatedAt: pgtype.Timestamptz{Valid: false},
	}
	m := roleToMap(r)
	if m["code"] != "auditor" || m["name"] != "Auditor" || m["is_system"] != false {
		t.Fatalf("unexpected map: %v", m)
	}
	if m["description"] != &desc {
		t.Fatalf("description not carried")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run (from `backend/`): `go test ./internal/authzadmin/ -run TestRoleToMap -v`
Expected: FAIL — `roleToMap` undefined.

- [ ] **Step 3: Write `dto.go`**

Create `backend/internal/authzadmin/dto.go`:

```go
package authzadmin

import (
	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

type roleCreateRequest struct {
	Code        string  `json:"code" binding:"required"`
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

type roleUpdateRequest struct {
	Code        string  `json:"code"`
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

type permissionsRequest struct {
	Permissions []string `json:"permissions"`
}

type scopePolicyBody struct {
	Module     string `json:"module" binding:"required"`
	ScopeLevel string `json:"scope_level" binding:"required"`
}
type scopeRequest struct {
	Policies []scopePolicyBody `json:"policies"`
}

type fieldPermBody struct {
	Entity  string `json:"entity" binding:"required"`
	Field   string `json:"field" binding:"required"`
	CanView bool   `json:"can_view"`
	CanEdit bool   `json:"can_edit"`
}
type fieldsRequest struct {
	Fields []fieldPermBody `json:"fields"`
}

func roleToMap(r sqlc.IdentityRole) map[string]any {
	return map[string]any{
		"id":          r.ID.String(),
		"code":        r.Code,
		"name":        r.Name,
		"description": r.Description,
		"is_system":   r.IsSystem,
		"created_at":  common.TsStr(r.CreatedAt),
		"updated_at":  common.TsStr(r.UpdatedAt),
	}
}

func scopePolicyToMap(p sqlc.IdentityDataScopePolicy) map[string]any {
	return map[string]any{"module": p.Module, "scope_level": string(p.ScopeLevel)}
}

func fieldPermToMap(f sqlc.IdentityFieldPermission) map[string]any {
	return map[string]any{"entity": f.Entity, "field": f.Field, "can_view": f.CanView, "can_edit": f.CanEdit}
}

func (r scopeRequest) toInputs() []ScopePolicyInput {
	out := make([]ScopePolicyInput, 0, len(r.Policies))
	for _, p := range r.Policies {
		out = append(out, ScopePolicyInput{Module: p.Module, ScopeLevel: p.ScopeLevel})
	}
	return out
}

func (r fieldsRequest) toInputs() []FieldPermInput {
	out := make([]FieldPermInput, 0, len(r.Fields))
	for _, f := range r.Fields {
		out = append(out, FieldPermInput{Entity: f.Entity, Field: f.Field, CanView: f.CanView, CanEdit: f.CanEdit})
	}
	return out
}
```

- [ ] **Step 4: Write `handler.go`**

Create `backend/internal/authzadmin/handler.go`:

```go
package authzadmin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/audit"
)

// Handler exposes the authorization-admin HTTP endpoints.
type Handler struct {
	svc *Service
	aud *audit.Service
}

func NewHandler(svc *Service, aud *audit.Service) *Handler { return &Handler{svc: svc, aud: aud} }

func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrSystemRole), errors.Is(err, ErrRoleInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUnknownPermission), errors.Is(err, ErrValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func (h *Handler) catalog(c *gin.Context) { c.JSON(http.StatusOK, CatalogResponse()) }

func (h *Handler) listRoles(c *gin.Context) {
	rows, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, roleToMap(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) getRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	r, err := h.svc.GetRole(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, roleToMap(r))
}

func (h *Handler) createRole(c *gin.Context) {
	var req roleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r, err := h.svc.CreateRole(c.Request.Context(), RoleInput{Code: req.Code, Name: req.Name, Description: req.Description})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "roles", r.ID, nil, audit.Diff(nil, roleToMap(r)))
	c.JSON(http.StatusCreated, roleToMap(r))
}

func (h *Handler) updateRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req roleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, after, err := h.svc.UpdateRole(c.Request.Context(), id, RoleInput{Code: req.Code, Name: req.Name, Description: req.Description})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "roles", after.ID, nil, audit.Diff(roleToMap(before), roleToMap(after)))
	c.JSON(http.StatusOK, roleToMap(after))
}

func (h *Handler) deleteRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	r, err := h.svc.DeleteRole(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "roles", id, nil, audit.Diff(roleToMap(r), nil))
	c.Status(http.StatusNoContent)
}

func (h *Handler) getPermissions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	keys, err := h.svc.GetRolePermissions(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": keys})
}

func (h *Handler) setPermissions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req permissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, _ := h.svc.GetRolePermissions(c.Request.Context(), id)
	if err := h.svc.SetRolePermissions(c.Request.Context(), id, req.Permissions); err != nil {
		h.svcError(c, err)
		return
	}
	after, _ := h.svc.GetRolePermissions(c.Request.Context(), id)
	audit.Record(c, h.aud, audit.ActionUpdate, "role_permissions", id, nil,
		audit.Diff(map[string]any{"permissions": before}, map[string]any{"permissions": after}))
	c.JSON(http.StatusOK, gin.H{"permissions": after})
}

func (h *Handler) getScope(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.svc.GetScopePolicies(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		data = append(data, scopePolicyToMap(p))
	}
	c.JSON(http.StatusOK, gin.H{"policies": data})
}

func (h *Handler) setScope(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req scopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetScopePolicies(c.Request.Context(), id, req.toInputs()); err != nil {
		h.svcError(c, err)
		return
	}
	rows, _ := h.svc.GetScopePolicies(c.Request.Context(), id)
	data := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		data = append(data, scopePolicyToMap(p))
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "data_scope_policies", id, nil, audit.Diff(nil, map[string]any{"policies": data}))
	c.JSON(http.StatusOK, gin.H{"policies": data})
}

func (h *Handler) getFields(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.svc.GetFieldPermissions(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		data = append(data, fieldPermToMap(f))
	}
	c.JSON(http.StatusOK, gin.H{"fields": data})
}

func (h *Handler) setFields(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req fieldsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetFieldPermissions(c.Request.Context(), id, req.toInputs()); err != nil {
		h.svcError(c, err)
		return
	}
	rows, _ := h.svc.GetFieldPermissions(c.Request.Context(), id)
	data := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		data = append(data, fieldPermToMap(f))
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "field_permissions", id, nil, audit.Diff(nil, map[string]any{"fields": data}))
	c.JSON(http.StatusOK, gin.H{"fields": data})
}
```

- [ ] **Step 5: Write `routes.go`**

Create `backend/internal/authzadmin/routes.go`:

```go
package authzadmin

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the authorization-admin endpoints under /authz.
// requireRole gates role + role_permissions, requireScope gates data scope,
// requireField gates field permissions.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireRole, requireScope, requireField gin.HandlerFunc) {
	g := rg.Group("/authz")
	g.GET("/catalog", authMW, requireRole, h.catalog)

	g.GET("/roles", authMW, requireRole, h.listRoles)
	g.POST("/roles", authMW, requireRole, h.createRole)
	g.GET("/roles/:id", authMW, requireRole, h.getRole)
	g.PUT("/roles/:id", authMW, requireRole, h.updateRole)
	g.DELETE("/roles/:id", authMW, requireRole, h.deleteRole)

	g.GET("/roles/:id/permissions", authMW, requireRole, h.getPermissions)
	g.PUT("/roles/:id/permissions", authMW, requireRole, h.setPermissions)

	g.GET("/roles/:id/scope", authMW, requireScope, h.getScope)
	g.PUT("/roles/:id/scope", authMW, requireScope, h.setScope)

	g.GET("/roles/:id/fields", authMW, requireField, h.getFields)
	g.PUT("/roles/:id/fields", authMW, requireField, h.setFields)
}
```

- [ ] **Step 6: Wire into `NewRouter`**

In `backend/internal/server/router.go`, after the existing module registrations (near the `audit`/`approval` wiring), add (using the existing `queries`, `d.Pool`, `permSvc`, `scopeSvc`, `fieldSvc`, `auditSvc`, `requireAuth` already constructed there):

```go
		authzAdminSvc := authzadmin.NewService(queries, d.Pool, permSvc, scopeSvc, fieldSvc)
		authzAdminHandler := authzadmin.NewHandler(authzAdminSvc, auditSvc)
		authzadmin.RegisterRoutes(api, authzAdminHandler, requireAuth,
			middleware.RequirePermission(permSvc, "role.manage"),
			middleware.RequirePermission(permSvc, "scope.manage"),
			middleware.RequirePermission(permSvc, "fieldperm.manage"),
		)
```

Add the import `"github.com/ragbuaj/inventra/internal/authzadmin"`. Confirm the local variable names for the three authz services (`permSvc`, `scopeSvc`, `fieldSvc`) match what `router.go` already defines — read the file and adapt if they differ (e.g. `fieldSvc` is passed to the asset/user handlers already).

- [ ] **Step 7: Run tests + build/vet**

Run (from `backend/`): `go test ./internal/authzadmin/ && go build ./... && go vet ./...`
Expected: PASS, exit 0. A Gin route-registration panic here would indicate a `:id` param conflict — the `/authz/roles/:id...` tree is self-consistent, so this should not occur.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/authzadmin/dto.go backend/internal/authzadmin/handler.go backend/internal/authzadmin/routes.go backend/internal/authzadmin/dto_test.go backend/internal/server/router.go
git commit -m "feat(authz): authzadmin DTO, handlers, routes, wiring"
```

---

### Task 6: Realign the `000005` seed to enforced permission keys

**Files:**
- Modify: `backend/db/migrations/000005_seed_identity.up.sql`
- Modify: `backend/db/migrations/000005_seed_identity.down.sql` (only if it references the changed keys)
- Test: `backend/internal/authzadmin/seed_integration_test.go` (new, `//go:build integration`)

**Interfaces:** none (data/seed).

- [ ] **Step 1: Rewrite the `role_permissions` seed block**

In `backend/db/migrations/000005_seed_identity.up.sql`, replace the `INSERT INTO identity.role_permissions ... VALUES (...)` matrix so every key is a catalog key and asset/approval keys match enforcement. Use exactly this matrix (superadmin = full catalog):

```sql
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, v.perm
FROM identity.roles r
JOIN (VALUES
  -- Superadmin: full catalog.
  ('superadmin', 'user.manage'),
  ('superadmin', 'role.manage'),
  ('superadmin', 'scope.manage'),
  ('superadmin', 'fieldperm.manage'),
  ('superadmin', 'audit.view'),
  ('superadmin', 'masterdata.global.manage'),
  ('superadmin', 'masterdata.office.manage'),
  ('superadmin', 'asset.view'),
  ('superadmin', 'asset.manage'),
  ('superadmin', 'request.create'),
  ('superadmin', 'request.decide'),
  ('superadmin', 'approval.config.manage'),
  ('superadmin', 'report.view'),
  ('superadmin', 'report.export'),
  ('superadmin', 'maintenance.manage'),
  ('superadmin', 'depreciation.manage'),
  ('superadmin', 'valuation.exclude.approve'),
  ('superadmin', 'assignment.manage'),
  -- Kepala Kanwil: oversight + approvals within wilayah.
  ('kepala_kanwil', 'masterdata.office.manage'),
  ('kepala_kanwil', 'asset.view'),
  ('kepala_kanwil', 'request.create'),
  ('kepala_kanwil', 'request.decide'),
  ('kepala_kanwil', 'valuation.exclude.approve'),
  ('kepala_kanwil', 'report.view'),
  ('kepala_kanwil', 'report.export'),
  ('kepala_kanwil', 'audit.view'),
  -- Kepala Unit: approvals + reports within unit.
  ('kepala_unit', 'asset.view'),
  ('kepala_unit', 'request.create'),
  ('kepala_unit', 'request.decide'),
  ('kepala_unit', 'report.view'),
  ('kepala_unit', 'report.export'),
  ('kepala_unit', 'audit.view'),
  -- Manager: day-to-day asset operations.
  ('manager', 'asset.view'),
  ('manager', 'asset.manage'),
  ('manager', 'request.create'),
  ('manager', 'request.decide'),
  ('manager', 'maintenance.manage'),
  ('manager', 'assignment.manage'),
  ('manager', 'report.view'),
  ('manager', 'report.export'),
  -- Staf: read own + submit requests.
  ('staf', 'asset.view'),
  ('staf', 'request.create'),
  ('staf', 'report.view')
) AS v(code, perm) ON v.code = r.code;
```

Leave the `roles` and `data_scope_policies` seed blocks unchanged. Check `000005_seed_identity.down.sql` — if it deletes by specific permission_key values, update those to match; if it deletes by role or truncates, no change needed.

- [ ] **Step 2: Write the seed verification test**

Create `backend/internal/authzadmin/seed_integration_test.go`:

```go
//go:build integration

package authzadmin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

// The 000005 seed must grant built-in roles the keys the code actually enforces.
func TestSeed_BuiltinRolesUseEnforcedKeys(t *testing.T) {
	pool := testsupport.NewFreshMigratedPostgres(t) // applies all migrations incl. 000005, no truncation
	ctx := context.Background()

	has := func(roleCode, key string) bool {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT count(*) FROM identity.role_permissions rp
			JOIN identity.roles r ON r.id = rp.role_id
			WHERE r.code=$1 AND rp.permission_key=$2 AND rp.deleted_at IS NULL`, roleCode, key).Scan(&n))
		return n > 0
	}

	require.True(t, has("superadmin", "asset.manage"), "superadmin must have asset.manage")
	require.True(t, has("superadmin", "role.manage"))
	require.True(t, has("manager", "asset.manage"))
	require.True(t, has("kepala_unit", "request.decide"))
	require.False(t, has("superadmin", "asset.create"), "stale asset.create must be gone")
	require.False(t, has("kepala_unit", "request.approve"), "stale request.approve must be gone")
}
```

> `testsupport.NewFreshMigratedPostgres` may not exist by that name. Read `internal/testsupport/` and use whichever helper returns a pool with all migrations applied WITHOUT running `Reset` (which truncates the seeded rows). If only `NewPostgres` exists and it does NOT truncate, use it directly and do not call `Reset`. If every path truncates the seed, instead assert against the seed by re-applying the seed file's content in the test, or query immediately after `NewPostgres` before any reset. Pick the approach that actually exercises the committed seed file; describe which in your report.

- [ ] **Step 3: Run the verification**

Run (from `backend/`): `go test -tags=integration ./internal/authzadmin/ -run TestSeed -v`
Expected: PASS (Docker required).

- [ ] **Step 4: Commit**

```bash
git add backend/db/migrations/000005_seed_identity.up.sql backend/db/migrations/000005_seed_identity.down.sql backend/internal/authzadmin/seed_integration_test.go
git commit -m "fix(authz): realign seed RBAC to enforced permission keys"
```

---

### Task 7: Integration tests (real Postgres + Redis, full HTTP path)

**Files:**
- Create: `backend/internal/authzadmin/integration_test.go` (`//go:build integration`, package `authzadmin_test`)

**Interfaces:**
- Consumes: `internal/testsupport` (Postgres/Redis containers, migrate, seed), `authzadmin` constructors + `RegisterRoutes`, the three `authz` services, `audit` service. Mirror the router/seed setup used by `internal/asset/integration_test.go` and `internal/authz/scope_integration_test.go`.

- [ ] **Step 1: Study the harness**

Read `internal/asset/integration_test.go` (how it builds a gin router with real middleware + injects auth context) and `internal/authz/scope_integration_test.go` + `internal/testsupport/` (Postgres/Redis, `Reset`, `SeedRole`, `SeedScopePolicy`, and how to seed `role_permissions` so `RequirePermission("role.manage")` passes for the acting Superadmin). Reuse these; do not reinvent container setup.

- [ ] **Step 2: Write the integration tests**

Create `backend/internal/authzadmin/integration_test.go` implementing these cases as real `t.Run` subtests with substantive assertions (auth context = a seeded user whose role holds `role.manage`/`scope.manage`/`fieldperm.manage`):

```go
//go:build integration

package authzadmin_test

// Cases (mirror the asset integration harness: build api router group, register
// authzadmin routes with real RequirePermission middleware backed by permSvc,
// inject CtxUserID/CtxRoleID for a seeded admin role that has the gate perms):
//
// 1. Catalog_OK: GET /authz/catalog -> 200; body has permissions[], scope_levels (4), scope_modules (starts "*").
// 2. RoleCRUD: POST /authz/roles {code:"auditor",name:"Auditor"} -> 201 is_system=false;
//    GET /authz/roles/:id -> 200; PUT name -> 200 reflects; GET list -> contains it.
// 3. RoleCode_Conflict: POST same code twice -> 409.
// 4. SystemRole_Protected: resolve a seeded is_system role id; DELETE -> 409;
//    PUT with a changed code -> 409 (ErrSystemRole); PUT changing only name -> 200.
// 5. DeleteRole_InUse: create role, create a user with that role_id, DELETE role -> 409;
//    delete the user, DELETE role -> 204; GET role -> 404.
// 6. SetPermissions_ReplaceAndInvalidate: warm permSvc.Has(role,"asset.manage")=false;
//    PUT /authz/roles/:id/permissions {permissions:["asset.view","asset.manage"]} -> 200;
//    permSvc.Has(role,"asset.manage") -> true IMMEDIATELY (cache invalidated), and a removed
//    key returns false after a second PUT without it.
// 7. SetPermissions_UnknownKey: PUT {permissions:["asset.create"]} -> 400.
// 8. SetScope_ReplaceAndInvalidate: PUT /authz/roles/:id/scope {policies:[{module:"*",scope_level:"global"}]} -> 200;
//    scopeSvc.Resolve(role,nil,"assets").Level == global IMMEDIATELY; invalid scope_level -> 400; duplicate module -> 400.
// 9. SetFields_ReplaceAndInvalidate: PUT /authz/roles/:id/fields {fields:[{entity:"assets",field:"purchase_cost",can_view:false}]} -> 200;
//    fieldSvc.ForEntity(role,"assets") contains purchase_cost IMMEDIATELY; empty entity -> 400; dup (entity,field) -> 400.
// 10. Audit_Recorded: after a role create + a permissions PUT, audit_logs has rows with
//     entity_type "roles" and "role_permissions" for the acting user.
// 11. Forbidden_WithoutPermission: act as a role lacking role.manage -> GET /authz/roles -> 403.
//
// Replace the comment block with executable subtests.
```

Assert real status codes, parsed JSON, and — critically — that the authz services reflect changes IMMEDIATELY after each mutation (proving cache invalidation), by calling `permSvc.Has`/`scopeSvc.Resolve`/`fieldSvc.ForEntity` directly against the same Redis-backed services the handler invalidated.

- [ ] **Step 3: Run**

Run (from `backend/`): `go test -tags=integration ./internal/authzadmin/ -v`
Expected: PASS (Docker required).

- [ ] **Step 4: Full integration safety gate**

Run (from `backend/`): `go test -tags=integration ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/authzadmin/integration_test.go
git commit -m "test(authz): integration coverage for authzadmin endpoints"
```

---

### Task 8: OpenAPI + PROGRESS + final verification

**Files:**
- Modify: `backend/api/openapi.yaml`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Add OpenAPI paths + schemas**

In `backend/api/openapi.yaml`, add the `/authz/*` paths mirroring an existing JSON-CRUD resource (e.g. the `/users` paths) for style — reuse the `bearerJWT` security scheme and the shared error response/schema components:
- `GET /authz/catalog` → `AuthzCatalog`.
- `GET/POST /authz/roles`, `GET/PUT/DELETE /authz/roles/{id}` (`Role`, `RoleCreateRequest`, `RoleUpdateRequest`).
- `GET/PUT /authz/roles/{id}/permissions` (`PermissionSet` = `{permissions: string[]}`).
- `GET/PUT /authz/roles/{id}/scope` (`ScopePolicySet` = `{policies: [{module, scope_level}]}`).
- `GET/PUT /authz/roles/{id}/fields` (`FieldPermissionSet` = `{fields: [{entity, field, can_view, can_edit}]}`).
Document the gating permission per path in its description (`role.manage` / `scope.manage` / `fieldperm.manage`). `Role` schema: `id, code, name, description, is_system, created_at, updated_at`.

- [ ] **Step 2: Lint**

Run (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors.

- [ ] **Step 3: Update PROGRESS.md**

In `docs/PROGRESS.md`:
- Under **Backend — Cross-cutting**, flip `- [ ] **Authorization admin endpoints** …` to `- [x]` with a note: `internal/authzadmin — role CRUD (system-role protected), replace-set role_permissions/data_scope/field_permissions with Redis cache invalidation (ScopeService/FieldService gained Invalidate), canonical permission catalog (GET /authz/catalog). **Done — (2026-06-28).**`
- Add a line noting the **seed RBAC drift fix** (asset.view/manage, request.decide, approval.config.manage) under the same section or the Quality/Database notes, so it isn't silently buried.
- Refresh the **"▶ Next session — start here"** block to drop authz-admin and point at the next priority (wire the frontend Peran & RBAC / Data Scope / Field Permission screens to these endpoints, or asset transfer/mutasi).

- [ ] **Step 4: Full verification gate**

Run (from `backend/`):
```
go build ./...
go vet ./...
go test ./...
go test -tags=integration ./...
```
Then (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: all exit 0 / PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/api/openapi.yaml docs/PROGRESS.md
git commit -m "docs(authz): openapi + progress for authorization admin"
```

---

## Self-Review

**Spec coverage:**
- §2 placement + 2 Invalidate methods → Task 1 (methods) + Tasks 2–5 (authzadmin package). ✓
- §3 canonical catalog + `GET /authz/catalog` → Task 2 (catalog) + Task 5 (handler/route). ✓
- §4 seed drift fix → Task 6. ✓
- §5 endpoints (12) → Task 5 routes + handlers. ✓
- §6 rules: replace-set tx + invalidation, is_system protection, code-unique 409, in-use 409, validation, audit (officeID nil) → Task 4 (service) + Task 5 (handler audit). ✓
- §7 sqlc queries (11) → Task 3. ✓
- §8 tests: unit (catalog, validators, dto) Tasks 2/4/5; integration (CRUD, protection, replace-set + immediate cache invalidation, unknown key, forbidden, audit) Task 7; invalidation methods Task 1; seed Task 6. ✓
- §9 OpenAPI + PROGRESS + verification → Task 8. ✓

**Placeholder scan:** Tasks 6 & 7 use guided comment blocks but each Step explicitly requires replacing them with executable tests and names the exact assertions — flagged, not hidden. The `testsupport.NewFreshMigratedPostgres` name is called out as "verify/adapt", not assumed.

**Type consistency:** Service method names + input types (`RoleInput`, `ScopePolicyInput`, `FieldPermInput`) defined in Task 4 are reused verbatim in Task 5 handler. Validator names (`dedupePermissions`, `validateScopePolicies`, `validateFieldPerms`) consistent Tasks 4↔tests. sqlc param structs (`CreateRoleParams`, `InsertRolePermissionParams`, `InsertDataScopePolicyParams`, `InsertFieldPermissionParams`) consistent Task 3↔4. `IsKnownPermission`/`CatalogResponse` consistent Tasks 2↔4↔5. `Invalidate` signatures consistent Task 1↔4. Audit `officeID` is `*uuid.UUID` → `nil` passed (matches `audit.Record` signature). Router var names (`permSvc`/`scopeSvc`/`fieldSvc`) flagged for verify-and-adapt in Task 5 Step 6.
