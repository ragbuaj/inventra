# Integration Coverage: authz scope resolution + employee — Design Spec

**Status:** Approved (brainstorming) · **Date:** 2026-06-27 · **Builds on:** [ADR-0001](../../adr/0001-go-testing-stack.md) testing stack (`internal/testsupport`)

## Goal

Extend the integration-test coverage built in the ADR-0001 cycle to two high-value targets the office
suite did not exercise: the **authz scope-resolution brain** (`authz.ScopeService.Resolve`, including its
Redis caching) and the **employee** data-scope CRUD. Both run against real Postgres (and real Redis for
the scope suite) via the existing `internal/testsupport` foundation.

## Why these two

- The office suite (ADR-0001) tested `office.Service` with `(all, ids)` supplied directly — it **stubbed**
  the resolution that turns a role's `data_scope_policies` into that `(all, ids)`. `ScopeService.Resolve`
  is the actual authorization logic (4 scope levels, per-module override, `GetOfficeSubtree`, **Redis
  caching**) and is currently untested against real data. Highest-value gap.
- `employee` is the next office-scoped CRUD module; it has a distinct sentinel (`ErrOfficeOutOfScope`) and
  scopes on the employee's `office_id`. Broadens the data-scope proof to a second module.

## Scope

**In scope:** seed helpers (`SeedRole`, `SeedScopePolicy`, `SeedEmployee`); a `ScopeService.Resolve`
integration suite (Postgres + Redis); an `employee` data-scope CRUD integration suite (Postgres).

**Out of scope (later cycles):** floor/room suites, field-permission (`FilterView`) suite, the full
HTTP+JWT+authz request path, retrofitting existing stdlib tests.

## Architecture

All new files carry `//go:build integration`. No production code changes. The existing
`backend-integration` CI job (`go test -tags=integration ./...`) already covers the new suites — no CI
change needed. `testsupport.Reset` truncates Postgres app schemas only; Redis is flushed explicitly in
the caching subtests, and every reseed produces fresh UUIDs (so per-role / per-office cache keys never
bleed across subtests).

### Component 1 — seed helpers (`internal/testsupport/seed.go`, append)

- `SeedRole(t, pool, code string) uuid.UUID` — inserts `identity.roles (code, name)`, returns id.
- `SeedScopePolicy(t, pool, roleID uuid.UUID, module string, level sqlc.SharedScopeLevel)` — inserts
  `identity.data_scope_policies (role_id, module, scope_level)`.
- `SeedEmployee(t, pool, officeID uuid.UUID, code string) uuid.UUID` — inserts
  `masterdata.employees (code, name, office_id, status='active')`, returns id.

`SeedOfficeTree` (existing) is reused by both suites.

### Component 2 — `ScopeService.Resolve` suite (`internal/authz/scope_integration_test.go`)

Uses `testsupport.NewPostgres` + `testsupport.NewRedis`; constructs
`authz.NewScopeService(sqlc.New(pool), redisClient)`. `Scope` is `{Level sqlc.SharedScopeLevel; OfficeIDs
[]uuid.UUID}`. Each subtest `Reset`s Postgres and reseeds. Asserts:

1. **global** policy → `Scope{Level: global}`, `OfficeIDs` empty.
2. **own** policy → `Scope{Level: own}`.
3. **office** policy with `officeID=&Wilayah` → `Scope{Level: office, OfficeIDs: [Wilayah]}`.
4. **office_subtree** policy with `officeID=&Wilayah` → `Scope{Level: office_subtree, OfficeIDs: {Wilayah,
   Cabang}}` (set identity; via `GetOfficeSubtree`).
5. **nil-office fallback**: `office` and `office_subtree` with `officeID=nil` → `Scope{Level: own}`.
6. **no-policy fallback**: a role with zero policies → `Scope{Level: own}` for any module.
7. **per-module override beats `'*'`**: role with `'*'=own` and `'employees'=office_subtree` →
   `Resolve(..., "employees")` is office_subtree; `Resolve(..., "offices")` is own.
8. **Redis caching (policy)**: `FlushDB`; Resolve once (populates `authz:scope:<roleID>`); change the
   policy row in DB; Resolve again → returns the **stale cached** level (proves the cache is consulted).
9. **Redis caching (subtree)**: `FlushDB`; Resolve office_subtree once (populates
   `authz:subtree:<officeID>`); insert a new child office under that node in DB; Resolve again → the
   subtree is unchanged (**stale cached** set), proving subtree caching.

### Component 3 — employee data-scope suite (`internal/masterdata/employee/employee_integration_test.go`)

Uses `testsupport.NewPostgres`; constructs `employee.NewService(sqlc.New(pool))`. Seeds an office tree
plus employees in Wilayah / Cabang / Wilayah2. Each subtest `Reset`s and reseeds. Asserts with UUID
identity and `assert.ErrorIs`:

1. scoped `List(all=false, ids={Wilayah,Cabang})` → only the employees in those offices; total matches.
2. global `List(all=true, nil)` → all seeded employees.
3. `Get` out-of-scope → `common.ErrNotFound`; in-scope → the row.
4. `Create` with `OfficeID` out of scope (Pusat) → `employee.ErrOfficeOutOfScope`; in-scope (Wilayah) →
   success with that office.
5. `Update` moving an employee to an out-of-scope office → `employee.ErrOfficeOutOfScope`.
6. `Delete` out-of-scope → `common.ErrNotFound`.
7. soft-deleted employee `code` reused → succeeds (partial-unique `WHERE deleted_at IS NULL`).

## Global Constraints

- Every new file begins with `//go:build integration`; default `go test ./...` stays unit-only.
- No production `.go` changes; no existing test modified; no CI change (existing job covers the suites).
- Assert real behavior: exact scope levels, set identity of `OfficeIDs`, sentinel errors via
  `assert.ErrorIs`, stale-cache proof. No hollow assertions; no assertion weakened to pass.
- `testsupport.Reset` resets Postgres only; caching subtests `FlushDB` Redis explicitly; fresh UUIDs per
  reseed prevent cross-subtest cache bleed.
- Follow DATABASE.md conventions (soft-delete, partial-unique, enums in `shared`).
- Verification: `go build ./...`, `go vet ./...`, `go test ./...` (unit, fast, green) AND
  `go test -tags=integration ./...` (Docker) green.

## Testing

The deliverable is tests. The scope suite must fail if any of the four levels resolves wrong, if the
per-module override is ignored, if the nil-office/no-policy fallbacks regress, or if caching stops being
consulted. The employee suite must fail if scope filtering or write-side enforcement regresses.
