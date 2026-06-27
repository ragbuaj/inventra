# Backend Testing Stack (ADR-0001) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up `testify` + `testcontainers-go` testing infrastructure (ADR-0001) and prove it with an office data-scope integration suite against real Postgres, plus a CI job.

**Architecture:** A new `internal/testsupport` package (all behind `//go:build integration`) boots throwaway Postgres/Redis containers, applies the production migrations with the `golang-migrate` library, seeds data, and resets between tests by truncating app tables. One integration suite drives the real `office.Service` over `sqlc` against the container. A new untagged-vs-tagged CI split keeps unit tests fast while a `backend-integration` job runs the tagged suite on every PR.

**Tech Stack:** Go 1.25, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go` (+ `modules/postgres`, `modules/redis`), `github.com/golang-migrate/migrate/v4` (library), pgx/v5, go-redis/v9.

## Global Constraints

- Every `testsupport` file and every integration test file MUST begin with `//go:build integration` so the default `go test ./...` stays unit-only and fast.
- New deps (`testify`, `testcontainers-go` + `modules/postgres` + `modules/redis`, `golang-migrate/v4`) are imported ONLY from tagged files.
- Do NOT modify any of the 20 existing stdlib `_test.go` files.
- Assert real behavior (CLAUDE.md): exact row sets/counts with identity, sentinel errors — no hollow `len > 0`.
- Follow DATABASE.md: money/numeric columns are Go `string`; respect soft-delete + partial-unique + `set_updated_at`.
- No hardcoded container credentials beyond the throwaway values the helper sets; the DSN comes from the module's `ConnectionString`.
- Reset between tests truncates app tables (`identity`, `masterdata`, `audit` schemas) — this realizes the spec's "fast clean reset between tests" intent in a pgx-pool-safe way (preferred over the postgres module's Snapshot/Restore, which strands pooled connections).
- Module path is `github.com/ragbuaj/inventra`. Run all `go`/`migrate` commands from `backend/`.
- Verification before done: `go build ./...`, `go vet ./...`, `go test ./...` (unit, fast, green) AND `go test -tags=integration ./...` (needs a local Docker daemon) green.

---

### Task 1: Dependencies + Postgres container helper (with migrations)

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum` (via `go get`)
- Create: `backend/internal/testsupport/postgres.go`
- Test: `backend/internal/testsupport/postgres_smoke_test.go`

**Interfaces:**
- Produces:
  - `testsupport.NewPostgres(t *testing.T) *pgxpool.Pool` — starts `postgres:16-alpine`, applies all `db/migrations`, returns a connected pool; container + pool are torn down via `t.Cleanup`.

- [ ] **Step 1: Add dependencies**

Run (from `backend/`):
```bash
go get github.com/stretchr/testify@latest
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
go get github.com/testcontainers/testcontainers-go/modules/redis@latest
go get github.com/golang-migrate/migrate/v4@latest
```
Expected: `go.mod` now requires these modules. (They become indirect until imported; Step 3 imports them, then `go mod tidy` in Step 6 settles them as direct.)

- [ ] **Step 2: Write the failing smoke test**

Create `backend/internal/testsupport/postgres_smoke_test.go`:
```go
//go:build integration

package testsupport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestNewPostgresAppliesMigrations(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	ctx := context.Background()

	// A table from a late migration proves the full migration set ran.
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'masterdata' AND table_name = 'offices'
		)`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "masterdata.offices should exist after migrations")
}
```

- [ ] **Step 3: Write the Postgres helper**

Create `backend/internal/testsupport/postgres.go`:
```go
//go:build integration

// Package testsupport provides container-backed fixtures for integration tests
// (build tag `integration`). It boots throwaway Postgres/Redis, applies the
// production migrations, seeds data, and resets between tests (ADR-0001).
package testsupport

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5" scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// NewPostgres starts a throwaway postgres:16-alpine, applies every migration in
// backend/db/migrations, and returns a connected pool. The container and pool are
// terminated via t.Cleanup.
func NewPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("inventra_test"),
		tcpostgres.WithUsername("inventra"),
		tcpostgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	applyMigrations(t, dsn)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func applyMigrations(t *testing.T, dsn string) {
	t.Helper()

	// Locate backend/db/migrations relative to THIS source file, so the CWD does
	// not matter. This file lives at backend/internal/testsupport/postgres.go.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations")

	src, err := iofs.New(os.DirFS(migrationsDir), ".")
	require.NoError(t, err)

	// golang-migrate's pgx/v5 driver registers the "pgx5" scheme.
	dbURL := strings.Replace(dsn, "postgres://", "pgx5://", 1)

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()

	require.NoError(t, m.Up())
}
```

- [ ] **Step 4: Run the smoke test (Docker required)**

Run: `go test -tags=integration ./internal/testsupport/ -run TestNewPostgresAppliesMigrations -v`
Expected: PASS (container starts, migrations apply, `masterdata.offices` exists). First run pulls images.

- [ ] **Step 5: Verify the default build stays clean and tag-free**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, and the untagged build does NOT compile testcontainers (no testsupport package in the untagged graph).

- [ ] **Step 6: Tidy and commit**

```bash
go mod tidy
git add go.mod go.sum internal/testsupport/postgres.go internal/testsupport/postgres_smoke_test.go
git commit -m "test(backend): testcontainers Postgres helper + migration apply (ADR-0001)"
```

---

### Task 2: Reset helper + Redis container helper

**Files:**
- Create: `backend/internal/testsupport/reset.go`
- Create: `backend/internal/testsupport/redis.go`
- Test: `backend/internal/testsupport/reset_test.go`
- Test: `backend/internal/testsupport/redis_smoke_test.go`

**Interfaces:**
- Consumes: `testsupport.NewPostgres(t) *pgxpool.Pool` (Task 1).
- Produces:
  - `testsupport.Reset(t *testing.T, pool *pgxpool.Pool)` — truncates all base tables in the `identity`, `masterdata`, and `audit` schemas (`RESTART IDENTITY CASCADE`).
  - `testsupport.NewRedis(t *testing.T) *redis.Client` — starts `redis:7-alpine`, returns a connected client; container + client torn down via `t.Cleanup`.

- [ ] **Step 1: Write the failing reset test**

Create `backend/internal/testsupport/reset_test.go`:
```go
//go:build integration

package testsupport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestResetTruncatesAppTables(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO masterdata.office_types (name) VALUES ('Kantor')`)
	require.NoError(t, err)

	testsupport.Reset(t, pool)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM masterdata.office_types`).Scan(&count))
	assert.Equal(t, 0, count, "Reset should truncate masterdata tables")
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags=integration ./internal/testsupport/ -run TestResetTruncatesAppTables -v`
Expected: FAIL — `undefined: testsupport.Reset`.

- [ ] **Step 3: Write the reset helper**

Create `backend/internal/testsupport/reset.go`:
```go
//go:build integration

package testsupport

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Reset truncates every base table in the application schemas, restoring the
// database to its post-migration empty state. Use it between tests that share a
// container so each starts from a clean slate.
func Reset(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT format('%I.%I', schemaname, tablename)
		FROM pg_tables
		WHERE schemaname IN ('identity', 'masterdata', 'audit')`)
	require.NoError(t, err)

	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())
	if len(tables) == 0 {
		return
	}

	_, err = pool.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE")
	require.NoError(t, err)
}
```

- [ ] **Step 4: Run the reset test**

Run: `go test -tags=integration ./internal/testsupport/ -run TestResetTruncatesAppTables -v`
Expected: PASS.

- [ ] **Step 5: Write the failing Redis smoke test**

Create `backend/internal/testsupport/redis_smoke_test.go`:
```go
//go:build integration

package testsupport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestNewRedisPingsAndStores(t *testing.T) {
	client := testsupport.NewRedis(t)
	ctx := context.Background()

	require.NoError(t, client.Set(ctx, "k", "v", 0).Err())
	got, err := client.Get(ctx, "k").Result()
	require.NoError(t, err)
	assert.Equal(t, "v", got)
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test -tags=integration ./internal/testsupport/ -run TestNewRedisPingsAndStores -v`
Expected: FAIL — `undefined: testsupport.NewRedis`.

- [ ] **Step 7: Write the Redis helper**

Create `backend/internal/testsupport/redis.go`:
```go
//go:build integration

package testsupport

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// NewRedis starts a throwaway redis:7-alpine and returns a connected client.
// The container and client are torn down via t.Cleanup.
func NewRedis(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	t.Cleanup(func() { _ = client.Close() })
	return client
}
```

- [ ] **Step 8: Run the Redis smoke test**

Run: `go test -tags=integration ./internal/testsupport/ -run TestNewRedisPingsAndStores -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
go mod tidy
git add internal/testsupport/reset.go internal/testsupport/redis.go internal/testsupport/reset_test.go internal/testsupport/redis_smoke_test.go go.mod go.sum
git commit -m "test(backend): testsupport Reset + Redis container helper (ADR-0001)"
```

---

### Task 3: Office data-scope integration suite

**Files:**
- Create: `backend/internal/testsupport/seed.go`
- Test: `backend/internal/masterdata/office/office_integration_test.go`

**Interfaces:**
- Consumes:
  - `testsupport.NewPostgres(t) *pgxpool.Pool`, `testsupport.Reset(t, pool)` (Tasks 1-2).
  - `sqlc.New(db sqlc.DBTX) *sqlc.Queries`; `*pgxpool.Pool` satisfies `sqlc.DBTX`.
  - `office.NewService(q *sqlc.Queries) *office.Service`.
  - `office.Service` methods: `List(ctx, all bool, ids []uuid.UUID, search string, limit, offset int32) ([]sqlc.MasterdataOffice, int64, error)`, `Get(ctx, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataOffice, error)`, `Create(ctx, all bool, ids []uuid.UUID, in office.CreateInput) (sqlc.MasterdataOffice, error)`, `Update(ctx, id uuid.UUID, all bool, ids []uuid.UUID, in office.UpdateInput) (before, after sqlc.MasterdataOffice, err error)`, `Delete(ctx, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataOffice, error)`.
  - `office.CreateInput{ParentID *uuid.UUID; OfficeTypeID uuid.UUID; ProvinceID, CityID *uuid.UUID; Name, Code string; Address *string; IsActive bool}`; `office.UpdateInput{office.CreateInput}`.
  - Sentinels: `office.ErrParentOutOfScope`, `office.ErrReparentOutOfScope`; `common.ErrNotFound` (returned by `Get`/`Delete` out of scope).
  - `q.GetOfficeSubtree(ctx, id uuid.UUID) ([]uuid.UUID, error)` — recursive CTE returning the office plus all descendants.
- Produces:
  - `testsupport.SeedOfficeTree(t, pool) testsupport.OfficeTree` with fields `OfficeTypeID, Pusat, Wilayah, Cabang, Wilayah2, Cabang2 uuid.UUID`. Tree shape: `Pusat → {Wilayah → Cabang, Wilayah2 → Cabang2}`.

- [ ] **Step 1: Write the seed helper**

Create `backend/internal/testsupport/seed.go`:
```go
//go:build integration

package testsupport

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// OfficeTree holds the IDs seeded by SeedOfficeTree. Shape:
//
//	Pusat
//	├── Wilayah  → Cabang
//	└── Wilayah2 → Cabang2
type OfficeTree struct {
	OfficeTypeID uuid.UUID
	Pusat        uuid.UUID
	Wilayah      uuid.UUID
	Cabang       uuid.UUID
	Wilayah2     uuid.UUID
	Cabang2      uuid.UUID
}

// SeedOfficeTree inserts one office type and a two-branch office hierarchy,
// returning their IDs. Call testsupport.Reset first if reusing a pool.
func SeedOfficeTree(t *testing.T, pool *pgxpool.Pool) OfficeTree {
	t.Helper()
	ctx := context.Background()

	var tree OfficeTree
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ('Kantor') RETURNING id`).
		Scan(&tree.OfficeTypeID))

	ins := func(name, code string, parent *uuid.UUID) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			parent, tree.OfficeTypeID, name, code).Scan(&id))
		return id
	}

	tree.Pusat = ins("Pusat", "P", nil)
	tree.Wilayah = ins("Wilayah 1", "W1", &tree.Pusat)
	tree.Cabang = ins("Cabang 1", "C1", &tree.Wilayah)
	tree.Wilayah2 = ins("Wilayah 2", "W2", &tree.Pusat)
	tree.Cabang2 = ins("Cabang 2", "C2", &tree.Wilayah2)
	return tree
}
```

- [ ] **Step 2: Write the integration suite (failing until seed compiles)**

Create `backend/internal/masterdata/office/office_integration_test.go`:
```go
//go:build integration

package office_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/office"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func idSet(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func rowIDs(rows []sqlc.MasterdataOffice) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestOfficeDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("GetOfficeSubtree returns self + descendants only", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		sub, err := q.GetOfficeSubtree(ctx, tree.Wilayah)
		require.NoError(t, err)
		got := idSet(sub)
		assert.Len(t, sub, 2)
		assert.True(t, got[tree.Wilayah], "subtree includes itself")
		assert.True(t, got[tree.Cabang], "subtree includes descendant")
		assert.False(t, got[tree.Pusat], "subtree excludes ancestor")
		assert.False(t, got[tree.Wilayah2], "subtree excludes sibling")

		full, err := q.GetOfficeSubtree(ctx, tree.Pusat)
		require.NoError(t, err)
		assert.Len(t, full, 5, "root subtree spans the whole tree")
	})

	t.Run("scoped List returns only in-scope offices", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		got := rowIDs(rows)
		assert.True(t, got[tree.Wilayah] && got[tree.Cabang])
		assert.False(t, got[tree.Pusat] || got[tree.Wilayah2] || got[tree.Cabang2])
	})

	t.Run("global List returns all offices", func(t *testing.T) {
		testsupport.Reset(t, pool)
		testsupport.SeedOfficeTree(t, pool)

		rows, total, err := svc.List(ctx, true, nil, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, rows, 5)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, tree.Pusat, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, tree.Cabang, false, ids)
		require.NoError(t, err)
		assert.Equal(t, tree.Cabang, got.ID)
	})

	t.Run("Create rejects out-of-scope parent", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Bad", Code: "BAD", IsActive: true,
		})
		assert.ErrorIs(t, err, office.ErrParentOutOfScope)

		created, err := svc.Create(ctx, false, ids, office.CreateInput{
			ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
			Name: "Outlet", Code: "O1", IsActive: true,
		})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, *created.ParentID)
	})

	t.Run("Update rejects reparent outside scope", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, tree.Cabang, false, ids, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1", Code: "C1", IsActive: true,
			},
		})
		assert.ErrorIs(t, err, office.ErrReparentOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, tree.Wilayah2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted code can be reused (partial-unique)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Reuse", Code: "REUSE", IsActive: true,
		})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Reuse Again", Code: "REUSE", IsActive: true,
		})
		assert.NoError(t, err, "code reusable after soft delete")
	})

	t.Run("update advances updated_at (set_updated_at trigger)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		before, after, err := svc.Update(ctx, tree.Cabang, true, nil, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1 Renamed", Code: "C1", IsActive: true,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Cabang 1 Renamed", after.Name)
		assert.False(t, after.UpdatedAt.Before(before.UpdatedAt), "updated_at must not regress")
	})
}
```

- [ ] **Step 3: Run the suite (Docker required)**

Run: `go test -tags=integration ./internal/masterdata/office/ -run TestOfficeDataScope -v`
Expected: PASS — all subtests green.

- [ ] **Step 4: Run the whole tagged suite**

Run: `go test -tags=integration ./...`
Expected: PASS (testsupport smoke tests + office suite).

- [ ] **Step 5: Confirm the default suite is unaffected**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, fast, with the integration files excluded.

- [ ] **Step 6: Commit**

```bash
git add internal/testsupport/seed.go internal/masterdata/office/office_integration_test.go
git commit -m "test(masterdata): office data-scope integration suite on real Postgres (ADR-0001)"
```

---

### Task 4: CI integration job + progress docs

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/PROGRESS.md`

**Interfaces:**
- Consumes: the `integration` build tag and `go test -tags=integration ./...` (Tasks 1-3).

- [ ] **Step 1: Add the integration CI job**

In `.github/workflows/ci.yml`, add a new job alongside `backend` (sibling key under `jobs:`). Match the existing backend job's checkout/setup-go style:
```yaml
  backend-integration:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: backend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum
      - run: go test -tags=integration ./...
```
> `ubuntu-latest` GitHub runners ship a working Docker daemon, which testcontainers-go uses automatically — no `services:` block needed.

- [ ] **Step 2: Validate the workflow YAML**

Run: `python -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('ok')"` (from repo root)
Expected: `ok` (no YAML syntax error). If `python` is unavailable, visually confirm indentation matches the sibling `backend:` job.

- [ ] **Step 3: Update PROGRESS.md**

In `docs/PROGRESS.md`, the pending item on line 197 currently reads:
```
- [ ] Broaden backend test coverage (services, handlers, integration)
```
Replace it with a checked item recording this work, keeping the file's wording style:
```
- [x] Backend testing stack (ADR-0001): testify + testcontainers-go; `internal/testsupport` (Postgres/Redis containers, migration apply, reset, seed) + office data-scope integration suite on real Postgres + `backend-integration` CI job (`-tags=integration`). Broader service/handler coverage continues per phase.
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml docs/PROGRESS.md
git commit -m "ci(backend): integration job (-tags=integration) + mark ADR-0001 done"
```

---

## Notes for the executor

- **Docker is required** for every `-tags=integration` run (Tasks 1-4 verification). The default untagged `go test ./...` needs no Docker and must stay green throughout.
- First integration run pulls `postgres:16-alpine` and `redis:7-alpine` — allow time.
- If `go get` leaves the new modules marked `// indirect`, `go mod tidy` after they are imported (done in Tasks 1-2) promotes them to direct requires. That is expected, not an error.
- Do not weaken assertions to make a flaky test pass — if the `updated_at` assertion ever flakes, investigate the trigger, do not delete the check.
