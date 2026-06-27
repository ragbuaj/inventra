# Integration Coverage (floor + room + field-permission) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three integration suites on the existing `testsupport` foundation — floor (office-scoped), room (transitive floor→office scope), and field-permission (`FieldService.ForEntity` + `FilterView`, with Redis caching).

**Architecture:** Append three seed helpers to `internal/testsupport/seed.go`, then write three `//go:build integration` suites driving the real services against containers. No production code changes; the existing `backend-integration` CI job already runs `go test -tags=integration ./...`.

**Tech Stack:** Go 1.25, testify, testcontainers-go (Postgres + Redis), pgx/v5, go-redis/v9, sqlc.

## Global Constraints

- Every new/added file or function lives behind `//go:build integration`; default `go test ./...` stays unit-only and fast.
- No production `.go` changes; do not modify existing tests; no CI change.
- Assert real behavior: row identity (specific UUIDs), exact map contents, sentinel errors via `assert.ErrorIs`, stale-cache proof, default-allow retention. No hollow `len > 0`; never weaken an assertion to pass.
- `testsupport.Reset(t, pool)` resets Postgres only. The field-permission caching subtest must `rdb.FlushDB(ctx)` first. Every reseed yields fresh UUIDs, so per-role/per-office cache keys never bleed.
- Module path `github.com/ragbuaj/inventra`; run commands from `backend/`.
- Verify BOTH: `go build ./... && go vet ./... && go test ./...` (unit, fast, green) AND `go test -tags=integration ./...` (needs Docker) green.

**Reference facts (verified against the repo):**
- `testsupport.NewPostgres(t) *pgxpool.Pool`, `testsupport.NewRedis(t) *redis.Client`, `testsupport.Reset(t, pool)`, `testsupport.SeedOfficeTree(t, pool) testsupport.OfficeTree` (fields `OfficeTypeID, Pusat, Wilayah, Cabang, Wilayah2, Cabang2`), `testsupport.SeedRole(t, pool, code string) uuid.UUID`.
- `seed.go` already has `//go:build integration` and imports `context`, `testing`, `github.com/google/uuid`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/stretchr/testify/require`, `github.com/ragbuaj/inventra/db/sqlc`.
- `floor.NewService(q *sqlc.Queries) *floor.Service`; `List(ctx, all bool, ids []uuid.UUID, officeID uuid.UUID, search string, limit, offset int32) ([]sqlc.MasterdataFloor, int64, error)`, `Get(ctx,id,all,ids)`, `Create(ctx,all,ids,floor.CreateInput) (sqlc.MasterdataFloor, error)`, `Update(ctx,id,all,ids,floor.UpdateInput) (before,after,err)`, `Delete(ctx,id,all,ids)`. `floor.CreateInput{OfficeID uuid.UUID; Name string; Level *int32}`; `floor.UpdateInput{floor.CreateInput}`; sentinel `floor.ErrOfficeOutOfScope`. `sqlc.MasterdataFloor{ID, OfficeID uuid.UUID; Name string; Level *int32}`.
- `room.NewService(q *sqlc.Queries) *room.Service`; `List(ctx, all, ids, floorID uuid.UUID, search, limit, offset)`, `Get(ctx,id,all,ids)`, `Create(ctx,all,ids,room.CreateInput)`, `Update(ctx,id,all,ids,room.UpdateInput) (before,after,err)`, `Delete(ctx,id,all,ids)`. `room.CreateInput{FloorID uuid.UUID; Name string; Code *string}`; `room.UpdateInput{room.CreateInput}`; sentinel `room.ErrFloorOutOfScope`. `sqlc.MasterdataRoom{ID, FloorID uuid.UUID; Name string; Code *string}`.
- `authz.NewFieldService(q *sqlc.Queries, rdb *redis.Client) *authz.FieldService`; `ForEntity(ctx, roleID uuid.UUID, entity string) (map[string]authz.FieldPolicy, error)`; `authz.FieldPolicy{CanView, CanEdit bool}`; `authz.FilterView(policies map[string]authz.FieldPolicy, data map[string]any)`. Cache key `authz:fields:<roleID>`.
- `common.ErrNotFound` for out-of-scope Get/Delete.
- Tables: `masterdata.floors(office_id, name, level)` unique `(office_id, name) WHERE deleted_at IS NULL`; `masterdata.rooms(floor_id, name, code)` unique `(floor_id, name) WHERE deleted_at IS NULL`; `identity.field_permissions(entity, field, role_id, can_view, can_edit)`.
- **Package note:** `internal/authz/scope_integration_test.go` is package `authz_test` and already declares a helper `idSet`. The new `internal/authz/fields_integration_test.go` is ALSO package `authz_test` — do NOT redeclare `idSet` or any existing symbol there.

---

### Task 1: floor office-scoped integration suite (+ SeedFloor)

**Files:**
- Modify: `backend/internal/testsupport/seed.go` (append `SeedFloor`)
- Test: `backend/internal/masterdata/floor/floor_integration_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.Reset`, `testsupport.SeedOfficeTree`; `floor.*`; `common.ErrNotFound`.
- Produces: `testsupport.SeedFloor(t, pool, officeID uuid.UUID, name string) uuid.UUID`.

- [ ] **Step 1: Append the seed helper**

Append to `backend/internal/testsupport/seed.go`:
```go
// SeedFloor inserts a masterdata.floors row in the given office and returns its id.
func SeedFloor(t *testing.T, pool *pgxpool.Pool, officeID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.floors (office_id, name) VALUES ($1, $2) RETURNING id`,
		officeID, name).Scan(&id))
	return id
}
```

- [ ] **Step 2: Write the floor suite**

Create `backend/internal/masterdata/floor/floor_integration_test.go`:
```go
//go:build integration

package floor_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/floor"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func floorIDs(rows []sqlc.MasterdataFloor) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestFloorDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := floor.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("scoped List returns the office's floors; out-of-scope office rejected", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW := testsupport.SeedFloor(t, pool, tree.Wilayah, "Lantai 1")
		testsupport.SeedFloor(t, pool, tree.Wilayah2, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, tree.Wilayah, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.True(t, floorIDs(rows)[fW])

		_, _, err = svc.List(ctx, false, ids, tree.Wilayah2, "", 100, 0)
		assert.ErrorIs(t, err, floor.ErrOfficeOutOfScope)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW := testsupport.SeedFloor(t, pool, tree.Wilayah, "Lantai 1")
		fW2 := testsupport.SeedFloor(t, pool, tree.Wilayah2, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, fW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, fW, false, ids)
		require.NoError(t, err)
		assert.Equal(t, fW, got.ID)
	})

	t.Run("Create rejects out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, floor.CreateInput{OfficeID: tree.Pusat, Name: "Bad"})
		assert.ErrorIs(t, err, floor.ErrOfficeOutOfScope)

		created, err := svc.Create(ctx, false, ids, floor.CreateInput{OfficeID: tree.Wilayah, Name: "Lantai OK"})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, created.OfficeID)
	})

	t.Run("Update rejects out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW := testsupport.SeedFloor(t, pool, tree.Wilayah, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, fW, false, ids, floor.UpdateInput{
			CreateInput: floor.CreateInput{OfficeID: tree.Pusat, Name: "Lantai 1"},
		})
		assert.ErrorIs(t, err, floor.ErrOfficeOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW2 := testsupport.SeedFloor(t, pool, tree.Wilayah2, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, fW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted (office,name) can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, floor.CreateInput{OfficeID: tree.Wilayah, Name: "Reuse"})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, floor.CreateInput{OfficeID: tree.Wilayah, Name: "Reuse"})
		assert.NoError(t, err, "(office_id, name) reusable after soft delete")
	})
}
```

- [ ] **Step 3: Run the suite (Docker required)**

Run: `go test -tags=integration ./internal/masterdata/floor/ -run TestFloorDataScope -v`
Expected: PASS (6 subtests).

- [ ] **Step 4: Confirm the untagged build is unaffected**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, fast, integration files excluded.

- [ ] **Step 5: Commit**

```bash
git add internal/testsupport/seed.go internal/masterdata/floor/floor_integration_test.go
git commit -m "test(masterdata): floor office-scope integration suite on real Postgres"
```

---

### Task 2: room transitive-scope integration suite (+ SeedRoom)

**Files:**
- Modify: `backend/internal/testsupport/seed.go` (append `SeedRoom`)
- Test: `backend/internal/masterdata/room/room_integration_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.Reset`, `testsupport.SeedOfficeTree`, `testsupport.SeedFloor` (Task 1); `room.*`; `common.ErrNotFound`.
- Produces: `testsupport.SeedRoom(t, pool, floorID uuid.UUID, name string) uuid.UUID`.

- [ ] **Step 1: Append the seed helper**

Append to `backend/internal/testsupport/seed.go`:
```go
// SeedRoom inserts a masterdata.rooms row on the given floor and returns its id.
func SeedRoom(t *testing.T, pool *pgxpool.Pool, floorID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.rooms (floor_id, name) VALUES ($1, $2) RETURNING id`,
		floorID, name).Scan(&id))
	return id
}
```

- [ ] **Step 2: Write the room suite**

Create `backend/internal/masterdata/room/room_integration_test.go`:
```go
//go:build integration

package room_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/room"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func roomIDs(rows []sqlc.MasterdataRoom) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestRoomDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := room.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("List via in-scope floor returns rooms; out-of-scope floor rejected", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		rIn := testsupport.SeedRoom(t, pool, fIn, "R-in")
		testsupport.SeedRoom(t, pool, fOut, "R-out")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, fIn, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.True(t, roomIDs(rows)[rIn])

		_, _, err = svc.List(ctx, false, ids, fOut, "", 100, 0)
		assert.ErrorIs(t, err, room.ErrFloorOutOfScope)
	})

	t.Run("Get/Delete a room on an out-of-scope floor is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		rIn := testsupport.SeedRoom(t, pool, fIn, "R-in")
		rOut := testsupport.SeedRoom(t, pool, fOut, "R-out")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, rOut, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, rIn, false, ids)
		require.NoError(t, err)
		assert.Equal(t, rIn, got.ID)

		_, err = svc.Delete(ctx, rOut, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("Create rejects out-of-scope floor", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, room.CreateInput{FloorID: fOut, Name: "Bad"})
		assert.ErrorIs(t, err, room.ErrFloorOutOfScope)

		created, err := svc.Create(ctx, false, ids, room.CreateInput{FloorID: fIn, Name: "R-OK"})
		require.NoError(t, err)
		assert.Equal(t, fIn, created.FloorID)
	})

	t.Run("Update rejects move to out-of-scope floor", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		rIn := testsupport.SeedRoom(t, pool, fIn, "R-in")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, rIn, false, ids, room.UpdateInput{
			CreateInput: room.CreateInput{FloorID: fOut, Name: "R-in"},
		})
		assert.ErrorIs(t, err, room.ErrFloorOutOfScope)
	})

	t.Run("soft-deleted (floor,name) can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")

		first, err := svc.Create(ctx, true, nil, room.CreateInput{FloorID: fIn, Name: "Reuse"})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, room.CreateInput{FloorID: fIn, Name: "Reuse"})
		assert.NoError(t, err, "(floor_id, name) reusable after soft delete")
	})
}
```

- [ ] **Step 3: Run the suite**

Run: `go test -tags=integration ./internal/masterdata/room/ -run TestRoomDataScope -v`
Expected: PASS (5 subtests).

- [ ] **Step 4: Confirm the untagged build is unaffected**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, fast.

- [ ] **Step 5: Commit**

```bash
git add internal/testsupport/seed.go internal/masterdata/room/room_integration_test.go
git commit -m "test(masterdata): room transitive floor-scope integration suite on real Postgres"
```

---

### Task 3: field-permission integration suite (+ SeedFieldPermission)

**Files:**
- Modify: `backend/internal/testsupport/seed.go` (append `SeedFieldPermission`)
- Test: `backend/internal/authz/fields_integration_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.NewRedis`, `testsupport.Reset`, `testsupport.SeedRole`; `authz.NewFieldService`, `authz.ForEntity`, `authz.FieldPolicy`, `authz.FilterView`.
- Produces: `testsupport.SeedFieldPermission(t, pool, roleID uuid.UUID, entity, field string, canView, canEdit bool)`.

> **Package caution:** this file is package `authz_test`, the SAME package as `scope_integration_test.go`, which already declares `idSet`. Do NOT redeclare `idSet` or any other existing helper here. The suite below needs no shared helper.

- [ ] **Step 1: Append the seed helper**

Append to `backend/internal/testsupport/seed.go`:
```go
// SeedFieldPermission inserts an identity.field_permissions row for a role.
func SeedFieldPermission(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, entity, field string, canView, canEdit bool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.field_permissions (role_id, entity, field, can_view, can_edit)
		 VALUES ($1, $2, $3, $4, $5)`,
		roleID, entity, field, canView, canEdit)
	require.NoError(t, err)
}
```

- [ ] **Step 2: Write the field-permission suite**

Create `backend/internal/authz/fields_integration_test.go`:
```go
//go:build integration

package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestFieldPermissions(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	svc := authz.NewFieldService(sqlc.New(pool), rdb)
	ctx := context.Background()

	t.Run("ForEntity structures policies; unmapped field absent", func(t *testing.T) {
		testsupport.Reset(t, pool)
		role := testsupport.SeedRole(t, pool, "r-fields-1")
		testsupport.SeedFieldPermission(t, pool, role, "employee", "email", false, false)
		testsupport.SeedFieldPermission(t, pool, role, "employee", "name", true, false)
		testsupport.SeedFieldPermission(t, pool, role, "employee", "salary", false, false)

		pol, err := svc.ForEntity(ctx, role, "employee")
		require.NoError(t, err)
		assert.False(t, pol["email"].CanView)
		assert.True(t, pol["name"].CanView)
		assert.False(t, pol["salary"].CanView)
		_, ok := pol["code"]
		assert.False(t, ok, "a field with no policy row is absent from the map")
	})

	t.Run("FilterView drops non-viewable; default-allow keeps unmapped", func(t *testing.T) {
		testsupport.Reset(t, pool)
		role := testsupport.SeedRole(t, pool, "r-fields-2")
		testsupport.SeedFieldPermission(t, pool, role, "employee", "email", false, false)
		testsupport.SeedFieldPermission(t, pool, role, "employee", "name", true, false)
		testsupport.SeedFieldPermission(t, pool, role, "employee", "salary", false, false)

		pol, err := svc.ForEntity(ctx, role, "employee")
		require.NoError(t, err)

		data := map[string]any{"email": "a@b.c", "name": "Budi", "code": "E1", "salary": 100}
		authz.FilterView(pol, data)

		_, hasEmail := data["email"]
		_, hasSalary := data["salary"]
		_, hasName := data["name"]
		_, hasCode := data["code"]
		assert.False(t, hasEmail, "email not viewable -> dropped")
		assert.False(t, hasSalary, "salary not viewable -> dropped")
		assert.True(t, hasName, "name viewable -> kept")
		assert.True(t, hasCode, "code has no policy -> default-allow kept")
	})

	t.Run("field policies are cached (stale after DB change)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		require.NoError(t, rdb.FlushDB(ctx).Err())
		role := testsupport.SeedRole(t, pool, "r-fields-cache")
		testsupport.SeedFieldPermission(t, pool, role, "employee", "email", false, false)
		testsupport.SeedFieldPermission(t, pool, role, "employee", "name", true, false)
		testsupport.SeedFieldPermission(t, pool, role, "employee", "salary", false, false)

		first, err := svc.ForEntity(ctx, role, "employee")
		require.NoError(t, err)
		require.Len(t, first, 3)

		// Remove every policy for the role; without caching ForEntity would return an empty map.
		_, err = pool.Exec(ctx,
			`UPDATE identity.field_permissions SET deleted_at = now() WHERE role_id = $1`, role)
		require.NoError(t, err)

		second, err := svc.ForEntity(ctx, role, "employee")
		require.NoError(t, err)
		assert.Len(t, second, 3, "field cache should still serve the pre-change policies")
		assert.False(t, second["email"].CanView)
	})
}
```

- [ ] **Step 3: Run the suite (Docker required)**

Run: `go test -tags=integration ./internal/authz/ -run TestFieldPermissions -v`
Expected: PASS (3 subtests).

- [ ] **Step 4: Run the whole tagged suite + confirm untagged stays green**

Run: `go test -tags=integration ./...` then `go build ./... && go vet ./... && go test ./...`
Expected: both PASS (floor + room + field suites + prior suites under the tag; fast unit run untagged).

- [ ] **Step 5: Commit**

```bash
git add internal/testsupport/seed.go internal/authz/fields_integration_test.go
git commit -m "test(authz): field-permission ForEntity + FilterView integration suite (Postgres+Redis)"
```

---

## Notes for the executor

- **Docker required** for every `-tags=integration` run. The default untagged suite needs no Docker and must stay green.
- All three tasks append to `internal/testsupport/seed.go` (sequential, no conflict). Task 2's room suite uses `SeedFloor` from Task 1.
- The field suite shares package `authz_test` with the existing scope suite — do not redeclare `idSet`.
- Do not weaken the caching assertion in Task 3: after the DB rows are soft-deleted, a non-cached `ForEntity` would return an empty map; the test asserts the cached 3-field map. If it fails, the cache wiring changed — investigate, don't delete the check.
