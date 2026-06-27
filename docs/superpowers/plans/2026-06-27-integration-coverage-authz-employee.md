# Integration Coverage (authz scope + employee) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two integration suites on the existing `testsupport` foundation — `authz.ScopeService.Resolve` (Postgres + Redis, incl. caching) and `employee` data-scope CRUD (Postgres).

**Architecture:** Append three seed helpers to `internal/testsupport/seed.go`, then write two `//go:build integration` suites that drive the real services against containers. No production code changes; the existing `backend-integration` CI job already runs `go test -tags=integration ./...`.

**Tech Stack:** Go 1.25, testify, testcontainers-go (Postgres + Redis), pgx/v5, go-redis/v9, sqlc.

## Global Constraints

- Every new/added file or function lives behind `//go:build integration`; default `go test ./...` stays unit-only and fast.
- No production `.go` changes; do not modify existing tests; no CI change.
- Assert real behavior: exact scope `Level`, set-identity of `OfficeIDs`, sentinel errors via `assert.ErrorIs`, stale-cache proof. No hollow `len > 0`; never weaken an assertion to pass.
- `testsupport.Reset(t, pool)` resets Postgres only. Caching subtests must `rdb.FlushDB(ctx)` first. Every reseed yields fresh UUIDs, so per-role/per-office cache keys never bleed across subtests.
- Module path `github.com/ragbuaj/inventra`; run commands from `backend/`.
- Verify BOTH: `go build ./... && go vet ./... && go test ./...` (unit, fast, green) AND `go test -tags=integration ./...` (needs Docker) green.

**Reference facts (verified against the repo):**
- `authz.NewScopeService(q *sqlc.Queries, rdb *redis.Client) *authz.ScopeService`; `Resolve(ctx, roleID uuid.UUID, officeID *uuid.UUID, module string) (authz.Scope, error)`.
- `authz.Scope{ Level sqlc.SharedScopeLevel; OfficeIDs []uuid.UUID }`.
- Scope-level consts: `sqlc.SharedScopeLevelGlobal` ("global"), `sqlc.SharedScopeLevelOfficeSubtree` ("office_subtree"), `sqlc.SharedScopeLevelOffice` ("office"), `sqlc.SharedScopeLevelOwn` ("own").
- Cache keys: policies at `authz:scope:<roleID>`, subtree at `authz:subtree:<officeID>`; TTL 10m.
- `employee.NewService(q *sqlc.Queries) *employee.Service`; methods `List(ctx, all bool, ids []uuid.UUID, search string, limit, offset int32)`, `Get(ctx,id,all,ids)`, `Create(ctx,all,ids,employee.CreateInput)`, `Update(ctx,id,all,ids,employee.UpdateInput) (before,after,err)`, `Delete(ctx,id,all,ids)`. Sentinel `employee.ErrOfficeOutOfScope`; `common.ErrNotFound` for out-of-scope Get/Delete.
- `employee.CreateInput{ Code, Name string; Email, AvatarKey *string; DepartmentID, PositionID *uuid.UUID; OfficeID uuid.UUID; Status sqlc.SharedUserStatus }`; `employee.UpdateInput{employee.CreateInput}`; `sqlc.SharedUserStatusActive`.
- `testsupport.NewPostgres(t) *pgxpool.Pool`, `testsupport.NewRedis(t) *redis.Client`, `testsupport.Reset(t, pool)`, `testsupport.SeedOfficeTree(t, pool) testsupport.OfficeTree` (fields `OfficeTypeID, Pusat, Wilayah, Cabang, Wilayah2, Cabang2`).
- Tables: `identity.roles(code, name)`, `identity.data_scope_policies(role_id, module, scope_level)`, `masterdata.employees(code, name, office_id, status default 'active')`; employees partial-unique on `code WHERE deleted_at IS NULL`.

---

### Task 1: authz ScopeService.Resolve integration suite (+ SeedRole, SeedScopePolicy)

**Files:**
- Modify: `backend/internal/testsupport/seed.go` (append two helpers)
- Test: `backend/internal/authz/scope_integration_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.NewRedis`, `testsupport.Reset`, `testsupport.SeedOfficeTree`; `authz.NewScopeService`, `authz.Resolve`, `authz.Scope`; sqlc consts.
- Produces: `testsupport.SeedRole(t, pool, code string) uuid.UUID`; `testsupport.SeedScopePolicy(t, pool, roleID uuid.UUID, module string, level sqlc.SharedScopeLevel)`.

- [ ] **Step 1: Append the seed helpers**

Append to `backend/internal/testsupport/seed.go` (the file already has `//go:build integration` and imports `context`, `testing`, `github.com/google/uuid`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/stretchr/testify/require`). Add an import for `github.com/ragbuaj/inventra/db/sqlc`:
```go
// SeedRole inserts an identity.roles row and returns its id.
func SeedRole(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.roles (code, name) VALUES ($1, $1) RETURNING id`,
		code).Scan(&id))
	return id
}

// SeedScopePolicy inserts an identity.data_scope_policies row for a role.
func SeedScopePolicy(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, module string, level sqlc.SharedScopeLevel) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
		 VALUES ($1, $2, $3)`,
		roleID, module, string(level))
	require.NoError(t, err)
}
```
If `sqlc` is not yet imported in `seed.go`, add `"github.com/ragbuaj/inventra/db/sqlc"` to its import block.

- [ ] **Step 2: Write the failing scope suite**

Create `backend/internal/authz/scope_integration_test.go`:
```go
//go:build integration

package authz_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func idSet(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestScopeResolve(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	svc := authz.NewScopeService(sqlc.New(pool), rdb)
	ctx := context.Background()

	t.Run("global", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-global")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelGlobal)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelGlobal, sc.Level)
		assert.Empty(t, sc.OfficeIDs)
	})

	t.Run("own", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-own")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOwn)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)
	})

	t.Run("office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-office")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOffice)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOffice, sc.Level)
		assert.Equal(t, []uuid.UUID{tree.Wilayah}, sc.OfficeIDs)
	})

	t.Run("office_subtree spans descendants", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-sub")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOfficeSubtree)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, sc.Level)
		got := idSet(sc.OfficeIDs)
		assert.Len(t, sc.OfficeIDs, 2)
		assert.True(t, got[tree.Wilayah] && got[tree.Cabang])
		assert.False(t, got[tree.Pusat] || got[tree.Wilayah2])
	})

	t.Run("nil office falls back to own", func(t *testing.T) {
		testsupport.Reset(t, pool)
		testsupport.SeedOfficeTree(t, pool)
		// Two separate roles so each path uses its own (uncached) policy set —
		// seeding a second policy into one role after a Resolve would be hidden
		// by the per-role policy cache.
		subRole := testsupport.SeedRole(t, pool, "r-nil-subtree")
		testsupport.SeedScopePolicy(t, pool, subRole, "*", sqlc.SharedScopeLevelOfficeSubtree)
		offRole := testsupport.SeedRole(t, pool, "r-nil-office")
		testsupport.SeedScopePolicy(t, pool, offRole, "*", sqlc.SharedScopeLevelOffice)

		sub, err := svc.Resolve(ctx, subRole, nil, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, sub.Level, "office_subtree + nil office -> own")

		off, err := svc.Resolve(ctx, offRole, nil, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, off.Level, "office + nil office -> own")
	})

	t.Run("no policy falls back to own", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-empty")

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)
	})

	t.Run("per-module override beats default", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-override")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOwn)
		testsupport.SeedScopePolicy(t, pool, role, "employees", sqlc.SharedScopeLevelOfficeSubtree)

		emp, err := svc.Resolve(ctx, role, &tree.Wilayah, "employees")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, emp.Level)

		off, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, off.Level)
	})

	t.Run("policy result is cached (stale after DB change)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		require.NoError(t, rdb.FlushDB(ctx).Err())
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-cache-policy")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOfficeSubtree)

		first, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		require.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, first.Level)

		// Soft-delete the policy in the DB; without caching this would resolve to own.
		_, err = pool.Exec(ctx,
			`UPDATE identity.data_scope_policies SET deleted_at = now() WHERE role_id = $1`, role)
		require.NoError(t, err)

		second, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, second.Level,
			"policy cache should still serve the pre-change level")
	})

	t.Run("subtree result is cached (stale after new child)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		require.NoError(t, rdb.FlushDB(ctx).Err())
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-cache-subtree")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOfficeSubtree)

		first, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		require.Len(t, first.OfficeIDs, 2)

		// Add a child office under Wilayah after the subtree was cached.
		var child uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
			 VALUES ($1, $2, 'New Child', 'NC') RETURNING id`,
			tree.Wilayah, tree.OfficeTypeID).Scan(&child))

		second, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Len(t, second.OfficeIDs, 2, "subtree cache should not include the new child")
		assert.False(t, idSet(second.OfficeIDs)[child])
	})
}
```

- [ ] **Step 3: Run it to verify it fails to compile, then passes after the helpers exist**

Run: `go test -tags=integration ./internal/authz/ -run TestScopeResolve -v`
Expected: after Step 1's helpers are in place, PASS (all 9 subtests). If Step 1 was skipped it FAILs with `undefined: testsupport.SeedRole`.

- [ ] **Step 4: Confirm the untagged build is unaffected**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, fast, integration files excluded.

- [ ] **Step 5: Commit**

```bash
git add internal/testsupport/seed.go internal/authz/scope_integration_test.go
git commit -m "test(authz): ScopeService.Resolve integration suite incl. Redis caching (Postgres+Redis)"
```

---

### Task 2: employee data-scope integration suite (+ SeedEmployee)

**Files:**
- Modify: `backend/internal/testsupport/seed.go` (append one helper)
- Test: `backend/internal/masterdata/employee/employee_integration_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.Reset`, `testsupport.SeedOfficeTree`; `employee.NewService`, its methods, `employee.CreateInput`/`UpdateInput`, `employee.ErrOfficeOutOfScope`, `common.ErrNotFound`, `sqlc.SharedUserStatusActive`.
- Produces: `testsupport.SeedEmployee(t, pool, officeID uuid.UUID, code string) uuid.UUID`.

- [ ] **Step 1: Append the seed helper**

Append to `backend/internal/testsupport/seed.go`:
```go
// SeedEmployee inserts a masterdata.employees row in the given office (status active)
// and returns its id.
func SeedEmployee(t *testing.T, pool *pgxpool.Pool, officeID uuid.UUID, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.employees (code, name, office_id)
		 VALUES ($1, $1, $2) RETURNING id`,
		code, officeID).Scan(&id))
	return id
}
```

- [ ] **Step 2: Write the employee suite**

Create `backend/internal/masterdata/employee/employee_integration_test.go`:
```go
//go:build integration

package employee_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/employee"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func empIDs(rows []sqlc.MasterdataEmployee) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestEmployeeDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := employee.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("scoped List returns only in-scope employees", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eW := testsupport.SeedEmployee(t, pool, tree.Wilayah, "E-W")
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		got := empIDs(rows)
		assert.True(t, got[eW] && got[eC])
		assert.False(t, got[eW2])
	})

	t.Run("global List returns all employees", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		testsupport.SeedEmployee(t, pool, tree.Wilayah, "E-W")
		testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")

		rows, total, err := svc.List(ctx, true, nil, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, rows, 3)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, eW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, eC, false, ids)
		require.NoError(t, err)
		assert.Equal(t, eC, got.ID)
	})

	t.Run("Create rejects out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, employee.CreateInput{
			Code: "E-BAD", Name: "Bad", OfficeID: tree.Pusat, Status: sqlc.SharedUserStatusActive,
		})
		assert.ErrorIs(t, err, employee.ErrOfficeOutOfScope)

		created, err := svc.Create(ctx, false, ids, employee.CreateInput{
			Code: "E-OK", Name: "Ok", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, created.OfficeID)
	})

	t.Run("Update rejects move to out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, eC, false, ids, employee.UpdateInput{
			CreateInput: employee.CreateInput{
				Code: "E-C", Name: "Moved", OfficeID: tree.Pusat, Status: sqlc.SharedUserStatusActive,
			},
		})
		assert.ErrorIs(t, err, employee.ErrOfficeOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, eW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted code can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, employee.CreateInput{
			Code: "E-REUSE", Name: "First", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, employee.CreateInput{
			Code: "E-REUSE", Name: "Second", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		assert.NoError(t, err, "code reusable after soft delete")
	})
}
```

- [ ] **Step 3: Run the suite**

Run: `go test -tags=integration ./internal/masterdata/employee/ -run TestEmployeeDataScope -v`
Expected: PASS (7 subtests).

- [ ] **Step 4: Run the whole tagged suite + confirm untagged stays green**

Run: `go test -tags=integration ./...` then `go build ./... && go vet ./... && go test ./...`
Expected: both PASS (new suites + office suite + testsupport smoke tests under the tag; fast unit run untagged).

- [ ] **Step 5: Commit**

```bash
git add internal/testsupport/seed.go internal/masterdata/employee/employee_integration_test.go
git commit -m "test(masterdata): employee data-scope integration suite on real Postgres"
```

---

## Notes for the executor

- **Docker required** for every `-tags=integration` run. The default untagged suite needs no Docker and must stay green.
- The two tasks both append to `internal/testsupport/seed.go` (sequential, no conflict). If `seed.go` lacks a `db/sqlc` import after Task 1, add it.
- Do not weaken the caching assertions: the point of subtests 8–9 is that the DB changed but the cached result did not. If a caching assertion fails, the cache wiring changed — investigate, do not delete the check.
- The `nil office falls back to own` subtest seeds an extra `assets`/`office` policy mid-test; this is intentional (a second resolve path), not a leftover.
