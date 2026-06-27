# Backend Testing Stack (ADR-0001) — Design Spec

**Status:** Approved (brainstorming) · **Date:** 2026-06-27 · **Implements:** [ADR-0001](../../adr/0001-go-testing-stack.md) (Backlog #9)

## Goal

Stand up the testing tooling ADR-0001 decided but never implemented — `testify` for readable
assertions and `testcontainers-go` for integration tests against **real Postgres & Redis** — and prove
it with one high-value suite: **office data-scope enforcement**. This builds reusable container
infrastructure (`internal/testsupport`) that every later integration suite reuses, plus a dedicated CI
job, without retrofitting the existing stdlib unit tests.

## Scope

**In scope (this cycle):**
- New test-only dependencies: `testify`, `testcontainers-go` (+ `modules/postgres`, `modules/redis`),
  `golang-migrate/v4` (as a library).
- `internal/testsupport` package: Postgres + Redis container bootstrap, migration apply, baseline seed,
  fast per-test reset.
- One integration suite: `office` data-scope (read **and** write), recursive subtree CTE,
  partial-unique/soft-delete reuse, `updated_at` trigger.
- A `backend-integration` CI job running `go test -tags=integration ./...` on every PR.

**Out of scope (deliberate, follow-up cycles):**
- Integration suites for employee, floor/room, category, identity scope, field-permission.
- Retrofitting the 20 existing stdlib `_test.go` files to `testify`.
- Full HTTP+authz-path integration (caller JWT → `CallerOfficeScope` → query). This cycle tests at the
  **service + sqlc** layer; the HTTP+authz path is a later suite.

## Architecture

All new test infrastructure and integration tests live behind the build tag `//go:build integration`,
so the default `go build ./...` and `go test ./...` never compile testcontainers and stay fast. CI runs
two backend jobs: the existing untagged unit job, and a new tagged integration job.

### Component 1 — `internal/testsupport` (reusable foundation)

Files (all `//go:build integration`):

- **`postgres.go`** — `NewPostgres(t *testing.T) (*pgxpool.Pool, func())`:
  - Spins `postgres:16-alpine` via the testcontainers `modules/postgres`.
  - Applies the **same 15 migrations as production** (`db/migrations`, `000001`–`000015`) using the
    **`golang-migrate/v4` library** (file source + pgx/stdlib database driver). The first migration
    runs `CREATE EXTENSION IF NOT EXISTS pgcrypto` — supported by the stock image.
  - Takes a `Snapshot()` after migrations (+ any baseline seed) so each test can `Restore()` to a clean,
    isolated state quickly (postgres module feature). Returns a connected `*pgxpool.Pool` and teardown.
  - **Rationale for golang-migrate as a library** (not reading `.sql` via pgx, not `WithInitScripts`):
    pgx's extended protocol rejects the multi-statement SQL the migration files contain; the migrate
    library is the *same runner* production uses, giving true migration fidelity.
- **`redis.go`** — `NewRedis(t *testing.T) (*redis.Client, func())`: spins `redis:7-alpine`, returns a
  client + teardown. Built now so Redis-backed suites (token store, rate limiting, OAuth state) reuse it
  later; the office suite itself does not need it.
- **`seed.go`** — typed seed helpers used by suites, e.g. `SeedOfficeTree(ctx, pool) OfficeTree` that
  inserts a Pusat → Wilayah → Cabang hierarchy and returns their UUIDs.

**Sharing model:** one container per **test package** via `TestMain` (start once, reuse across the
package's tests, `Restore()` between tests) — the amortization ADR-0001 calls for. Go runs each package
as its own process, so per-package is the natural unit.

### Component 2 — office data-scope integration suite

`internal/masterdata/office/office_integration_test.go` (`//go:build integration`). Constructs the real
`office.Service` over `sqlc.New(pool)` against the container, seeds an office tree, and asserts real
behavior (using `require` for preconditions, `assert` for follow-on checks — never a hollow
`len > 0`):

1. **`office_subtree` scope:** a caller scoped to a mid-tree office sees exactly that office and its
   descendants — exercises the recursive `GetOfficeSubtree` CTE and `AllScope=false, OfficeIds=[…]`
   filtering in `ListOffices`/`CountOffices`.
2. **`office` / `own` scope:** caller sees only the single in-scope office; `GetOffice` for an
   out-of-scope id returns not-found.
3. **Create (scoped):** a scoped caller may create an office only under a parent within scope; out of
   scope returns the service's sentinel error.
4. **Update / Delete (scoped):** out-of-scope target → not-found/forbidden — enforcement on **write**,
   not only read.
5. **Partial-unique + soft-delete:** an office code reused after the prior row is soft-deleted succeeds
   — proves `UNIQUE … WHERE deleted_at IS NULL`.
6. **`set_updated_at` trigger:** `updated_at` advances on update.

### Component 3 — CI job

Add to `.github/workflows/ci.yml`:

```yaml
backend-integration:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version-file: backend/go.mod
        cache-dependency-path: backend/go.sum
    - run: go test -tags=integration ./...
      working-directory: backend
```

`ubuntu-latest` ships a working Docker daemon, which testcontainers uses automatically. The existing
untagged `backend` job is unchanged and stays fast; the new job gates every PR.

## Global Constraints

- **Build tag `//go:build integration`** on every `testsupport` file and every integration test file.
  The default `go test ./...` must remain unit-only and fast.
- **New deps are test-only** in practice (imported solely from tagged files): `testify`,
  `testcontainers-go` + `modules/postgres` + `modules/redis`, `golang-migrate/v4`.
- **No retrofit:** the 20 existing stdlib `_test.go` files are not modified.
- **Assert real behavior** (CLAUDE.md): rendered/returned values, row counts with identity, sentinel
  errors — no tautological assertions.
- **Follow DATABASE.md conventions:** money/numeric columns are Go `string`; respect soft-delete +
  partial-unique + `set_updated_at`.
- **No hardcoded container credentials** — take them from the testcontainers module's connection string.
- **Verification before done:** `go build ./...`, `go vet ./...`, `go test ./...` (unit, fast, green)
  **and** `go test -tags=integration ./...` (needs a local Docker daemon) green.

## Testing

The deliverable *is* tests. Verification is the dual run above. The integration suite must fail loudly
if scope filtering, the subtree CTE, the partial-unique index, or the trigger regress — that is the
whole point of testing against real Postgres rather than mocks.
