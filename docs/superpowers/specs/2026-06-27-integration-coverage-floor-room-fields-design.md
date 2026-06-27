# Integration Coverage: floor + room + field-permission — Design Spec

**Status:** Approved (brainstorming) · **Date:** 2026-06-27 · **Builds on:** [ADR-0001](../../adr/0001-go-testing-stack.md) testing stack + prior integration cycles (office, authz scope, employee)

## Goal

Continue integration-test coverage on the `internal/testsupport` foundation with three suites: **floor**
(office-scoped CRUD), **room** (scope enforced *transitively* through the floor's office — novel coverage),
and **field-permission** (`authz.FieldService.ForEntity` + `FilterView`, with Redis caching). All run
against real Postgres (and real Redis for fields) via the existing containers.

## Why these

- `floor` and `room` are the remaining office-scoped masterdata modules; **room is the high-value target**
  — its scope is enforced through the FK chain room → floor → office (`floorInScope`), a pattern no prior
  suite exercised.
- `field-permission` is the third authorization layer (after action-permissions and data-scope, the latter
  now covered). `ForEntity` loads `field_permissions`, caches per-role in Redis, and `FilterView` applies
  default-allow field stripping — none of it tested against real data.

## Scope

**In scope:** seed helpers (`SeedFloor`, `SeedRoom`, `SeedFieldPermission`); three integration suites
(floor, room, field-permission).

**Out of scope (later/none):** category and reference-engine suites; audit-service suite; HTTP+JWT path;
retrofitting existing stdlib tests.

## Architecture

All new files carry `//go:build integration`; no production code changes; no CI change (the existing
`backend-integration` job runs `go test -tags=integration ./...`). Each subtest `Reset`s Postgres; the
field-permission caching subtest `FlushDB`s Redis; fresh UUIDs per reseed prevent per-role/per-office
cache-key bleed.

### Component 1 — seed helpers (`internal/testsupport/seed.go`, append)

- `SeedFloor(t, pool, officeID uuid.UUID, name string) uuid.UUID` — inserts `masterdata.floors(office_id, name)`.
- `SeedRoom(t, pool, floorID uuid.UUID, name string) uuid.UUID` — inserts `masterdata.rooms(floor_id, name)`.
- `SeedFieldPermission(t, pool, roleID uuid.UUID, entity, field string, canView, canEdit bool)` — inserts `identity.field_permissions`.

`SeedOfficeTree` and `SeedRole` (existing) are reused.

### Component 2 — floor suite (`internal/masterdata/floor/floor_integration_test.go`)

`floor.NewService(sqlc.New(pool))`. `floor.CreateInput{OfficeID uuid.UUID; Name string; Level *int32}`;
sentinel `floor.ErrOfficeOutOfScope`; `common.ErrNotFound`. Seed office tree + floors in Wilayah and
Wilayah2; `ids = {Wilayah, Cabang}`. Asserts:

1. `List(all=false, ids, officeID=Wilayah, …)` → that office's floor(s); `List(…, officeID=Wilayah2, …)`
   → `floor.ErrOfficeOutOfScope`.
2. `Get` out-of-scope → `common.ErrNotFound`; in-scope → the row by ID.
3. `Create` with `OfficeID` out of scope (Pusat) → `floor.ErrOfficeOutOfScope`; in-scope (Wilayah) →
   success with that office.
4. `Update` with `OfficeID` out of scope → `floor.ErrOfficeOutOfScope`.
5. `Delete` out-of-scope → `common.ErrNotFound`.
6. partial-unique `(office_id, name)`: a name reused after soft-delete → succeeds.

### Component 3 — room suite (`internal/masterdata/room/room_integration_test.go`)

`room.NewService(sqlc.New(pool))`. `room.CreateInput{FloorID uuid.UUID; Name string; Code *string}`;
sentinel `room.ErrFloorOutOfScope`; `common.ErrNotFound`. Seed office tree + a floor under Wilayah
(in-scope) and a floor under Wilayah2 (out-of-scope) + a room on each; `ids = {Wilayah, Cabang}`. Asserts:

1. `List(all=false, ids, floorID=floorInWilayah, …)` → its room(s); `List(…, floorID=floorInWilayah2, …)`
   → `room.ErrFloorOutOfScope` — **the transitive FK-chain check**.
2. `Get`/`Delete` a room whose floor is out of scope → `common.ErrNotFound` (scope enforced via the
   room → floor → office join in SQL); in-scope `Get` → the row.
3. `Create` with `FloorID` out of scope → `room.ErrFloorOutOfScope`; in-scope → success.
4. `Update` moving a room to an out-of-scope floor → `room.ErrFloorOutOfScope`.
5. partial-unique `(floor_id, name)`: a name reused after soft-delete → succeeds.

### Component 4 — field-permission suite (`internal/authz/fields_integration_test.go`)

`authz.NewFieldService(sqlc.New(pool), rdb)`; `authz.FieldPolicy{CanView, CanEdit bool}`. Uses
`testsupport.NewPostgres` + `testsupport.NewRedis`. Seed a role + `field_permissions` rows. Asserts:

1. `ForEntity(role, "employee")` → a map containing `email{CanView:false}`, `name{CanView:true}`,
   `salary{CanView:false}`; a field with no seeded row is **absent** from the map.
2. `FilterView`: a record map `{email, name, code, salary}` after `FilterView(ForEntity result, data)` →
   `email` and `salary` removed (CanView=false), `name` retained, `code` retained (default-allow, no
   policy).
3. **Redis caching**: `FlushDB`; `ForEntity` once (populates `authz:fields:<roleID>`); soft-delete all of
   the role's `field_permissions` in DB; `ForEntity` again → still returns the **stale cached** (non-empty)
   map — proving the cache is consulted (without it the map would be empty).

## Global Constraints

- Every new file begins with `//go:build integration`; default `go test ./...` stays unit-only.
- No production `.go` changes; no existing test modified; no CI change.
- Assert real behavior: row identity (specific UUIDs), exact map contents, sentinel errors via
  `assert.ErrorIs`, stale-cache proof, default-allow retention. No hollow assertions; never weaken one to
  pass.
- `testsupport.Reset` resets Postgres only; the caching subtest `FlushDB`s Redis; fresh UUIDs per reseed
  prevent cross-subtest cache bleed.
- Follow DATABASE.md (soft-delete, partial-unique, enums in `shared`).
- Verify BOTH: `go build ./...`, `go vet ./...`, `go test ./...` (unit, fast, green) AND
  `go test -tags=integration ./...` (Docker) green.

## Testing

The deliverable is tests. The floor suite must fail if office-scope enforcement regresses; the room suite
must fail if the transitive floor→office scope stops being enforced on read or write; the field suite must
fail if `ForEntity` mis-structures policies, if `FilterView` stops being default-allow, or if caching stops
being consulted.
