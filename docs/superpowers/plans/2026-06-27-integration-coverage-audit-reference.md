# Integration Coverage (audit + reference engine) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two integration suites on the existing `testsupport` foundation — the office-scoped `audit` trail (`Log` + scoped `List` + `Diff` round-trip) and the generic `reference` CRUD engine (dynamic SQL + `coerce`).

**Architecture:** Two new `//go:build integration` test files. The audit suite is external package `audit_test` (all symbols exported); the reference suite is **white-box** package `reference` (engine/resource/methods are unexported). No new seed helpers, no production code, no CI change.

**Tech Stack:** Go 1.25, testify, testcontainers-go (Postgres), pgx/v5, sqlc.

## Global Constraints

- Every new file begins with `//go:build integration`; default `go test ./...` stays unit-only and fast.
- No production `.go` changes; do not modify existing tests; no CI change.
- Assert real behavior: row identity/counts, exact office-scope filtering, JSON round-trip equality, sentinel errors via `assert.ErrorIs`, dynamic-SQL correctness. No hollow `len > 0`; never weaken to pass.
- The reference suite MUST be package `reference` (white-box) to reach the unexported `engine`/`resource`. The audit suite is package `audit_test`. Each subtest `Reset`s Postgres first.
- Module path `github.com/ragbuaj/inventra`; run commands from `backend/`.
- Verify BOTH: `go build ./... && go vet ./... && go test ./...` (unit, fast, green) AND `go test -tags=integration ./...` (Docker) green.

**Reference facts (verified against the repo):**
- `testsupport.NewPostgres(t) *pgxpool.Pool`, `testsupport.Reset(t, pool)`.
- `audit.NewService(q *sqlc.Queries) *audit.Service`; `Log(ctx, audit.LogInput) error`; `List(ctx, audit.ListFilter) ([]sqlc.ListAuditLogsRow, int64, error)`; `audit.Diff(before, after any) map[string]map[string]any`.
- `audit.LogInput{ActorID *uuid.UUID; EntityType string; EntityID uuid.UUID; Action audit.Action; Changes any; IP string; OfficeID *uuid.UUID}`. `audit.Action` consts: `audit.ActionCreate`, `audit.ActionUpdate`, `audit.ActionDelete`.
- `audit.ListFilter{AllScope bool; OfficeIDs []uuid.UUID; ActorID *uuid.UUID; EntityType *string; Action *audit.Action; From, To *time.Time; Search string; Limit, Offset int32}`.
- `sqlc.ListAuditLogsRow` fields used: `EntityType string`, `Action sqlc.SharedAuditAction`, `Changes []byte`, `OfficeID *uuid.UUID`, `CreatedAt pgtype.Timestamptz`.
- `audit_logs.office_id` is a plain indexed `uuid` (NO FK); `actor_id` is nullable; `entity_id` has no FK — so the suite may use freshly generated UUIDs and `ActorID: nil`.
- `Diff` is already unit-tested in `internal/audit/audit_test.go` (package `audit`: `TestDiffCreate`, `TestDiffDelete`, `TestDiffUpdateOnlyChangedFields`, `TestDiffIgnoresTimestamps`). The integration suite therefore centers on the **DB round-trip**, with only a minimal direct `Diff` assertion to build the round-trip input.
- `Diff(before, after)` returns `map[string]map[string]any` where the inner map is `{"before": bv, "after": av}` (a create has only `"after"`); values are JSON-normalized (strings stay strings); `created_at`/`updated_at` keys are ignored.
- reference (package `reference`): `engine{pool *pgxpool.Pool}`; methods `list(ctx, r resource, search string, limit, offset int32) ([]map[string]any, int64, error)`, `get(ctx, r resource, id uuid.UUID) (map[string]any, error)`, `write(ctx, r resource, id *uuid.UUID, body map[string]any) (map[string]any, error)`, `del(ctx, r resource, id uuid.UUID) (bool, error)`. Unexported `referenceResources []resource` contains the entry `{Path: "office-types", Table: "office_types", …}` with columns `name` (text, required, search) and `is_active` (bool, default true).
- `write` returns a `map[string]any` keyed by the select aliases: `id` (string, from `id::text`), `name` (string), `is_active` (bool), `created_at`, `updated_at`. `get` returns `common.ErrNotFound` (via `common.MapDBError(pgx.ErrNoRows)`) when the row is missing/deleted. office_types unique index is `(name) WHERE deleted_at IS NULL`.
- `common.ErrNotFound` from `github.com/ragbuaj/inventra/internal/masterdata/common`.

---

### Task 1: audit office-scoped integration suite

**Files:**
- Test: `backend/internal/audit/audit_integration_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.Reset`; `audit.NewService`, `audit.Log`, `audit.List`, `audit.Diff`, `audit.LogInput`, `audit.ListFilter`, `audit.Action*`; `sqlc.New`.
- Produces: nothing (leaf test).

- [ ] **Step 1: Write the audit suite**

Create `backend/internal/audit/audit_integration_test.go`:
```go
//go:build integration

package audit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestAuditService(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := audit.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("List is office-scoped", func(t *testing.T) {
		testsupport.Reset(t, pool)
		officeA, officeB := uuid.New(), uuid.New()
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &officeA}))
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionUpdate, OfficeID: &officeA}))
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &officeB}))

		rows, total, err := svc.List(ctx, audit.ListFilter{AllScope: false, OfficeIDs: []uuid.UUID{officeA}, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, rows, 2)
		for _, r := range rows {
			require.NotNil(t, r.OfficeID)
			assert.Equal(t, officeA, *r.OfficeID)
		}

		all, totalAll, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(3), totalAll)
		assert.Len(t, all, 3)
	})

	t.Run("filter by action and entity_type", func(t *testing.T) {
		testsupport.Reset(t, pool)
		office := uuid.New()
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &office}))
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "employee", EntityID: uuid.New(), Action: audit.ActionUpdate, OfficeID: &office}))

		actionCreate := audit.ActionCreate
		byAction, totalA, err := svc.List(ctx, audit.ListFilter{AllScope: true, Action: &actionCreate, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(1), totalA)
		require.Len(t, byAction, 1)
		assert.Equal(t, audit.ActionCreate, byAction[0].Action)
		assert.Equal(t, "office", byAction[0].EntityType)

		entity := "employee"
		byType, totalT, err := svc.List(ctx, audit.ListFilter{AllScope: true, EntityType: &entity, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(1), totalT)
		require.Len(t, byType, 1)
		assert.Equal(t, "employee", byType[0].EntityType)
	})

	t.Run("List is newest-first", func(t *testing.T) {
		testsupport.Reset(t, pool)
		office := uuid.New()
		for i := 0; i < 3; i++ {
			require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &office}))
		}
		rows, _, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
		require.NoError(t, err)
		require.Len(t, rows, 3)
		for i := 0; i < len(rows)-1; i++ {
			assert.False(t, rows[i].CreatedAt.Time.Before(rows[i+1].CreatedAt.Time), "rows must be ordered newest-first")
		}
	})

	t.Run("Diff round-trips through Changes JSON", func(t *testing.T) {
		testsupport.Reset(t, pool)
		office := uuid.New()

		before := map[string]any{"name": "A", "code": "X"}
		after := map[string]any{"name": "B", "code": "X"}
		diff := audit.Diff(before, after)
		// Minimal direct sanity (full Diff logic is unit-tested elsewhere): only the
		// changed field is present, with before/after recorded.
		require.Len(t, diff, 1)
		assert.Equal(t, map[string]any{"before": "A", "after": "B"}, diff["name"])

		require.NoError(t, svc.Log(ctx, audit.LogInput{
			EntityType: "office", EntityID: uuid.New(), Action: audit.ActionUpdate,
			Changes: diff, OfficeID: &office,
		}))

		rows, _, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
		require.NoError(t, err)
		require.Len(t, rows, 1)

		var got map[string]map[string]any
		require.NoError(t, json.Unmarshal(rows[0].Changes, &got))
		assert.Equal(t, diff, got)
	})
}
```

- [ ] **Step 2: Run the suite (Docker required)**

Run: `go test -tags=integration ./internal/audit/ -run TestAuditService -v`
Expected: PASS (4 subtests).

- [ ] **Step 3: Confirm the untagged build is unaffected**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, fast, integration file excluded; the existing untagged `audit` unit tests (`TestDiffCreate`, …) still run and pass.

- [ ] **Step 4: Commit**

```bash
git add internal/audit/audit_integration_test.go
git commit -m "test(audit): office-scoped List + Log/Diff round-trip integration suite on real Postgres"
```

---

### Task 2: reference engine integration suite (white-box)

**Files:**
- Test: `backend/internal/masterdata/reference/engine_integration_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.Reset`; the package-internal `engine`, `resource`, `referenceResources`, and methods `write`/`get`/`list`/`del`; `common.ErrNotFound`.
- Produces: nothing (leaf test).

> **Package:** this file is package `reference` (white-box), NOT `reference_test`, so it can construct `engine{pool: ...}` and read `referenceResources`.

- [ ] **Step 1: Write the reference-engine suite**

Create `backend/internal/masterdata/reference/engine_integration_test.go`:
```go
//go:build integration

package reference

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func officeTypesResource(t *testing.T) resource {
	t.Helper()
	for _, r := range referenceResources {
		if r.Path == "office-types" {
			return r
		}
	}
	t.Fatal("office-types resource not registered")
	return resource{}
}

func TestReferenceEngine(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	e := engine{pool: pool}
	ctx := context.Background()
	res := officeTypesResource(t)
	require.Equal(t, "office_types", res.Table)

	t.Run("write/get/list round trip", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)
		assert.Equal(t, "Tipe A", created["name"])
		id := uuid.MustParse(created["id"].(string))

		got, err := e.get(ctx, res, id)
		require.NoError(t, err)
		assert.Equal(t, "Tipe A", got["name"])

		rows, total, err := e.list(ctx, res, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, rows, 1)
	})

	t.Run("search filters by name", func(t *testing.T) {
		testsupport.Reset(t, pool)
		_, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)

		hit, totalHit, err := e.list(ctx, res, "Tipe A", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), totalHit)
		assert.Len(t, hit, 1)

		miss, totalMiss, err := e.list(ctx, res, "nope", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), totalMiss)
		assert.Empty(t, miss)
	})

	t.Run("update via write", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)
		id := uuid.MustParse(created["id"].(string))

		updated, err := e.write(ctx, res, &id, map[string]any{"name": "Tipe B", "is_active": false})
		require.NoError(t, err)
		assert.Equal(t, "Tipe B", updated["name"])
		assert.Equal(t, false, updated["is_active"])
	})

	t.Run("soft delete hides the row", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)
		id := uuid.MustParse(created["id"].(string))

		ok, err := e.del(ctx, res, id)
		require.NoError(t, err)
		assert.True(t, ok)

		_, err = e.get(ctx, res, id)
		assert.ErrorIs(t, err, common.ErrNotFound)

		rows, total, err := e.list(ctx, res, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, rows)
	})

	t.Run("coerce rejects a missing required field", func(t *testing.T) {
		testsupport.Reset(t, pool)
		_, err := e.write(ctx, res, nil, map[string]any{"is_active": true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("soft-deleted name can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "X", "is_active": true})
		require.NoError(t, err)
		id := uuid.MustParse(created["id"].(string))

		_, err = e.del(ctx, res, id)
		require.NoError(t, err)

		_, err = e.write(ctx, res, nil, map[string]any{"name": "X", "is_active": true})
		assert.NoError(t, err, "name reusable after soft delete")
	})
}
```

- [ ] **Step 2: Run the suite (Docker required)**

Run: `go test -tags=integration ./internal/masterdata/reference/ -run TestReferenceEngine -v`
Expected: PASS (6 subtests).

- [ ] **Step 3: Run the whole tagged suite + confirm untagged stays green**

Run: `go test -tags=integration ./...` then `go build ./... && go vet ./... && go test ./...`
Expected: both PASS (audit + reference + all prior suites under the tag; fast unit run untagged).

- [ ] **Step 4: Commit**

```bash
git add internal/masterdata/reference/engine_integration_test.go
git commit -m "test(masterdata): reference engine CRUD + coerce integration suite (white-box, real Postgres)"
```

---

## Notes for the executor

- **Docker required** for every `-tags=integration` run. The default untagged suite needs no Docker and must stay green.
- No `seed.go` change and no new seed helper this cycle — both suites create their own data via `svc.Log` / `e.write`.
- The reference suite is white-box (`package reference`) on purpose — it must reach the unexported `engine`, `resource`, `referenceResources`, and methods. Do not switch it to `reference_test`.
- The audit suite's `Diff` sanity assertion is intentionally minimal — the full `Diff` behavior is already unit-tested in `audit_test.go` (untagged). The integration value is the `Log → List → json.Unmarshal(Changes)` round-trip.
- `e.write` returns the `id` as a STRING (`id::text`); parse it with `uuid.MustParse(created["id"].(string))` before passing to `e.get`/`e.del`.
