# Integration Coverage: audit + reference engine — Design Spec

**Status:** Approved (brainstorming) · **Date:** 2026-06-27 · **Builds on:** [ADR-0001](../../adr/0001-go-testing-stack.md) testing stack + prior integration cycles (office, authz scope, employee, floor, room, field-permission)

## Goal

Complete the backend integration-test surface with two suites on the `internal/testsupport` foundation:
the **audit** service (office-scoped append-only trail — `Log` + scoped `List` + the pure `Diff`) and the
**reference** generic CRUD engine (dynamic SQL for flat masterdata tables). Both run against real
Postgres; neither needs Redis.

## Why these

- **audit** is security-relevant: the `audit.audit_logs` read model is office-scoped (`AllScope` /
  `OfficeIDs` filter), and that scoping has not been tested against real data. The `Log → List` round-trip
  and the JSON `Changes` marshalling are also untested end-to-end.
- The **reference engine** builds **dynamic SQL** (`SELECT`/`INSERT`/`UPDATE`/soft-delete) from `resource`
  definitions plus a `coerce` validation/type-conversion step. Dynamic-SQL and coercion bugs are exactly
  what a real-Postgres test catches and mocks hide.

## Scope

**In scope:** an audit integration suite and a reference-engine integration suite. **No new seed helpers**
— each suite creates its data through the API under test (`svc.Log`, `engine.write`).

**Out of scope:** the category sub-package; the audit HTTP handler/route; retrofitting existing tests.

## Architecture

All new files carry `//go:build integration`; no production code changes; no CI change (the existing
`backend-integration` job runs `go test -tags=integration ./...`). Each subtest `Reset`s Postgres. The
audit suite uses external package `audit_test` (all needed symbols are exported). The reference suite is
**white-box** — package `reference` (not `reference_test`) — because `engine`, `resource`, and the
`list`/`get`/`write`/`del` methods are unexported.

### Component 1 — audit suite (`internal/audit/audit_integration_test.go`, package `audit_test`)

`audit.NewService(sqlc.New(pool))`. `audit.LogInput{ActorID *uuid.UUID; EntityType string; EntityID
uuid.UUID; Action audit.Action; Changes any; IP string; OfficeID *uuid.UUID}`; `audit.ListFilter{AllScope
bool; OfficeIDs []uuid.UUID; ActorID *uuid.UUID; EntityType *string; Action *audit.Action; From, To
*time.Time; Search string; Limit, Offset int32}`; actions `audit.ActionCreate/ActionUpdate/ActionDelete`;
`audit.Diff(before, after any) map[string]map[string]any`. `audit_logs.office_id`, `actor_id`, `entity_id`
carry **no FK constraint that the test must satisfy** (actor_id nullable; office_id a plain indexed uuid),
so the suite uses freshly generated UUIDs for two offices (officeA, officeB) and `ActorID: nil`. Asserts:

1. **office scope:** `Log` two rows under officeA and one under officeB; `List(AllScope=false,
   OfficeIDs={officeA}, Limit=100)` → exactly the two officeA rows (total 2), none from officeB;
   `List(AllScope=true, …)` → all three.
2. **filters:** with create and update rows of differing `EntityType`, `List` filtered by `Action` returns
   only that action; filtered by `EntityType` returns only that type.
3. **newest-first:** `List` returns rows with non-increasing `CreatedAt` (ORDER BY created_at DESC).
4. **Diff round-trip:** `Diff(before, after)` for an update → `Log` with `Changes` = that diff → `List` →
   unmarshal `rows[0].Changes` (JSON) and assert it equals the diff. Plus direct `Diff` assertions: create
   `(nil, after)` yields every field as `{after}`; update yields only changed fields.

### Component 2 — reference engine suite (`internal/masterdata/reference/engine_integration_test.go`, package `reference`)

White-box. Constructs `e := engine{pool: pool}` and selects the registered `resource` whose `Path` is
`"office-types"` from `referenceResources` (so the real config is exercised), backed by the
`masterdata.office_types` table (columns: `name` text required+search, `is_active` bool default true).
Asserts:

1. **write → get → list:** `e.write(ctx, res, nil, {"name": "Tipe A", "is_active": true})` creates a row;
   `e.get(ctx, res, id)` returns it; `e.list(ctx, res, "", 100, 0)` includes it (total 1). (`id` comes back
   as a text field in the map; parse with `uuid.Parse`.)
2. **search:** `e.list(ctx, res, "Tipe A", …)` → 1 row; `e.list(ctx, res, "nope", …)` → 0 (ILIKE clause).
3. **update:** `e.write(ctx, res, &id, {"name": "Tipe B", "is_active": false})` → name updated.
4. **soft delete:** `e.del(ctx, res, id)` → true; `e.get(ctx, res, id)` → `common.ErrNotFound`;
   `e.list(…)` → 0 (the `deleted_at IS NULL` filter hides it).
5. **coerce validation:** `e.write(ctx, res, nil, {"is_active": true})` (missing required `name`) → a
   non-nil error mentioning `name`.
6. **partial-unique reuse:** create `"X"`, `del`, create `"X"` again → succeeds (unique
   `(name) WHERE deleted_at IS NULL`).

## Global Constraints

- Every new file begins with `//go:build integration`; default `go test ./...` stays unit-only.
- No production `.go` changes; no existing test modified; no CI change.
- Assert real behavior: row identity / counts, exact scope filtering, JSON round-trip equality, sentinel
  errors via `assert.ErrorIs`, dynamic-SQL correctness. No hollow assertions; never weaken one to pass.
- The reference suite is package `reference` (white-box) to reach the unexported engine; the audit suite is
  package `audit_test`. Each subtest `Reset`s Postgres first.
- Follow DATABASE.md (soft-delete, partial-unique). audit_logs is append-only (no soft-delete/updated_at).
- Verify BOTH: `go build ./...`, `go vet ./...`, `go test ./...` (unit, fast, green) AND
  `go test -tags=integration ./...` (Docker) green.

## Testing

The deliverable is tests. The audit suite must fail if office-scoping on the read regresses, if a filter is
ignored, or if the `Changes` JSON round-trip breaks. The reference suite must fail if the engine's dynamic
SQL, `coerce` validation, search clause, or soft-delete/partial-unique behavior regresses.
