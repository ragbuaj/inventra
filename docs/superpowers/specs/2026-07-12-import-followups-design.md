# Import module — follow-ups (dept/position, brand/model/unit targets, MinIO error reports, e2e)

**Date:** 2026-07-12
**Status:** Design — approved scope, pending spec review
**Branch:** `feat/import-followups`
**Predecessor:** `docs/superpowers/specs/2026-07-12-import-module-design.md` (item 44 in PROGRESS)

## 1. Context & goal

The bulk-import module (`internal/importer` + per-domain targets) shipped in #61 with several
explicitly-tracked follow-ups (PROGRESS item 44, "Honest limitations / follow-ups"). This effort clears
four of them:

1. **Employee import — department/position columns** (importer currently hardcodes
   `DepartmentID: nil, PositionID: nil`).
2. **New reference import targets — brand, model, unit** (only provinces/cities wired today; the
   `reference:` engine is extensible).
3. **Error reports stored to MinIO** — the `error_report_key` column (migration `000009`) is never
   populated; reports are built on-demand only.
4. **Playwright e2e for office + reference import paths** — only asset+employee are e2e-driven today.

**Out of scope (deferred, confirmed with user):** room/floor import targets (hierarchical + data-scoped
— the most complex and least-commonly bulk-imported). Leave the room/floor follow-up open in PROGRESS.

**Non-goals:** incremental Redis validate progress; the case-sensitivity TOCTOU nuance; the
`employee`→`masterdata.employee.manage` permission alignment (already resolved by migration `000032`).

## 2. Established patterns this builds on

- **`TargetImporter` contract** (`internal/importer/target.go`): `Target()`, `Columns()`,
  `ValidateRows()`, `Execute(qtx *sqlc.Queries, ...)`, `NeedsApproval()`. The worker passes a
  **tx-bound `*sqlc.Queries` (`qtx`)**, never a raw `pgx.Tx` — so every target uses dedicated sqlc
  queries, not dynamic SQL. New flat targets therefore follow the **provinces/cities** shape, not a new
  generic engine.
- **Anti-tx-poisoning discipline** (see `reference/importer.go` and `employee/importer.go`): a 23505
  inside the shared batch tx poisons the whole transaction, so every insert is preceded by a
  side-effect-free existence pre-check (`GetXByCode`/`GetXByName`) + an in-batch `usedKeys` set; a taken
  key is `MarkRowFailed`, never inserted. A residual concurrent-race 23505 returns (aborts the attempt)
  rather than being swallowed.
- **Split validate**: DB step (`buildXLookups`) separate from the pure step (`validateXRows`) so
  business rules are unit-testable without a database.
- **`PermissionKey`** (`service.go`): `reference:*` → `masterdata.global.manage` (already covers the new
  reference targets — no change needed).

## 3. Work item 1 — Employee import: department/position

**Columns** (both **optional**, matching the Pegawai CRUD form where both FKs are nullable):

| column (id) | required | kind | resolves against |
|---|---|---|---|
| `departemen` | no | lookup | department **name OR code** (case-insensitive) |
| `jabatan` | no | lookup | position **name** (case-insensitive) |

Departments carry a `code` column (`referenceResources`); positions carry name only.

**Changes** (`internal/masterdata/employee/importer.go`):
- `Columns()`: append `departemen`, `jabatan` (both `Required:false`, `Kind:"lookup"`).
- `employeeLookups`: add `departments map[string]uuid.UUID` (keyed by name AND code) and
  `positions map[string]uuid.UUID` (keyed by name).
- `buildEmployeeLookups`: load both via new sqlc queries.
- `validateEmployeeRows`: when a cell is non-empty, resolve it; a miss adds `CellError{col,"departemen"}`
  / `{col,"jabatan"}`. Empty → skip (optional). Stamp `_department_id` / `_position_id` into `Data` only
  when resolved; strip stamps from invalid rows (existing pattern).
- `Execute`: parse the optional stamps into `*uuid.UUID` (nil when absent) and pass to
  `CreateEmployee` instead of the hardcoded nils. No new dedup concern (dept/position are not unique
  keys of an employee).

**New sqlc queries** (`db/queries/reference_import.sql`, alongside the existing lookup queries):
```sql
-- name: ListDepartmentsLookup :many
SELECT id, name, code FROM masterdata.departments WHERE deleted_at IS NULL;
-- name: ListPositionsLookup :many
SELECT id, name FROM masterdata.positions WHERE deleted_at IS NULL;
```
(These are read-only lookups shared conceptually with the reference importer; placing them in
`reference_import.sql` keeps import-only queries together. Regenerate with `sqlc generate`.)

**Template/UX:** template gains the two header columns; the ImportWizard renders them generically (no
asset-specific formatting). No frontend page change — employee import already exists.

## 4. Work item 2 — Reference targets: brand, model, unit

Extend `internal/masterdata/reference/importer.go` (the `switch r.resource` covering
provinces/cities) with three resources. **Dedup is by name** (per the unique indexes), unlike
provinces/cities (dedup by optional `code`):

| target (`Target()`) | columns | unique index | dedup key |
|---|---|---|---|
| `reference:brands` | `nama`* | `uq_brands_name` | `name` |
| `reference:units`  | `nama`*, `simbol` | `uq_units_name` | `name` |
| `reference:models` | `merek`* (lookup), `nama`* | `uq_models_brand_name` | `(brand_id, name)` |

**Validation** (pure, per resource):
- **brands/units**: `nama` required; a `nama` duplicating an existing DB name or an earlier in-batch row
  (case-insensitive) fails `dupNama` (name IS uniquely constrained, so this is authoritative). `simbol`
  optional, free text.
- **models**: `nama` + `merek` required. `merek` resolved by brand **name** (case-insensitive) against a
  brands lookup; a miss fails `merek`. Duplicate `(brand_id, lower(name))` within the batch or in DB
  fails `dupNama`. Resolved brand id stamped into `_brand_id`.

**Execute** (anti-poisoning, one tx for the batch):
- brands/units: pre-check name availability via `GetBrandByName`/`GetUnitByName` + in-batch `usedNames`
  set; taken → `MarkRowFailed{col:"nama",key:"dupNama"}`; else `CreateBrand`/`CreateUnit`.
- models: parse `_brand_id`; pre-check `(brand_id,name)` via `GetModelByBrandAndName` + in-batch
  `usedPairs` set keyed by `brandID+"\x00"+lower(name)`; taken → fail; else `CreateModel`.

**New sqlc queries** (`db/queries/reference_import.sql`):
```sql
-- name: CreateBrand :one
INSERT INTO masterdata.brands (name) VALUES ($1) RETURNING *;
-- name: GetBrandByName :one
SELECT * FROM masterdata.brands WHERE lower(name) = lower($1) AND deleted_at IS NULL LIMIT 1;
-- name: ListBrandsLookup :many
SELECT id, name FROM masterdata.brands WHERE deleted_at IS NULL;

-- name: CreateUnit :one
INSERT INTO masterdata.units (name, symbol) VALUES ($1, $2) RETURNING *;
-- name: GetUnitByName :one
SELECT * FROM masterdata.units WHERE lower(name) = lower($1) AND deleted_at IS NULL LIMIT 1;

-- name: CreateModel :one
INSERT INTO masterdata.models (brand_id, name) VALUES ($1, $2) RETURNING *;
-- name: GetModelByBrandAndName :one
SELECT * FROM masterdata.models
WHERE brand_id = $1 AND lower(name) = lower($2) AND deleted_at IS NULL LIMIT 1;
```
(Confirm actual column sets against migration `000006` during implementation — e.g. whether `units` has
`symbol` and whether `brands`/`models` have additional NOT NULL columns; adjust the INSERTs accordingly.
The name-dedup queries use `lower()` to match the importer's case-insensitive rule; the DB unique index
is case-sensitive, so the residual-race path still returns on a genuine concurrent 23505, same as
provinces/cities.)

**Registration** (`internal/server/router.go`, after the provinces/cities lines):
```go
importerSvc.RegisterTarget(reference.NewImporter(refSvc, "brands"))
importerSvc.RegisterTarget(reference.NewImporter(refSvc, "models"))
importerSvc.RegisterTarget(reference.NewImporter(refSvc, "units"))
```

**Frontend:**
- `frontend/app/pages/master/import.vue`: extend `MasterImportTarget` union, `VALID_TARGETS`,
  `PERMISSION_BY_TARGET` (all three → `masterdata.global.manage`), `LABEL_KEY_BY_TARGET`.
- `frontend/app/pages/master/reference.vue`: append `'brands','models','units'` to
  `IMPORTABLE_RESOURCES` so the Import button appears for them.
- i18n `i18n/locales/{id,en}.json`: `masterdata.import.targets.{brands,models,units}` +
  any new cell-error keys (`dupNama`, `merek`) if not already present in the wizard's error map.

## 5. Work item 3 — Error reports persisted to MinIO

**Generation points (confirmed):** at the **end of validate** (so the user can download → fix →
re-import before deciding to confirm) **and** at the **end of execute** (to fold in execute-time dup
failures). Both reuse the existing `BuildErrorReport` helper.

**Flow** (in `internal/importer/worker.go`):
- After the validate phase commits, if the job has any failed rows, call a new
  `storeErrorReport(ctx, jobID, target, format)` helper that: reads the committed error rows
  (`ListImportRows{OnlyErrors:true}`), builds the report in `job.Format`, `store.Put`s it under
  `imports/<jobID>/errors.<ext>`, and sets `error_report_key` via a new `SetJobErrorReportKey` query.
  **MinIO I/O happens outside the DB tx** (after commit), in its own short-lived update.
- After the execute phase commits, if `failed_rows > 0`, call the same helper (overwrites the object at
  the same key and re-sets it — idempotent).
- Failures to store the report are **non-fatal** (logged, job stays completed/validated) — the
  on-demand endpoint remains as a fallback, so a MinIO hiccup never fails a job.

**Endpoint** (`handler.go` `errorReport`): if `job.ErrorReportKey` is set **and** the requested
`format` matches `job.Format`, stream the stored object from MinIO (`store.Get`) with the existing
headers. Otherwise (key null — older jobs — or a different `?format=` requested) fall back to the
current on-demand `BuildErrorReport`. This preserves the format-override affordance and back-compat.

**New sqlc query** (`db/queries/importer.sql`):
```sql
-- name: SetJobErrorReportKey :exec
UPDATE import.import_jobs SET error_report_key = $2 WHERE id = $1;
```
(Confirm schema/column names against migration `000009`/`000030` — the job table location and the
`error_report_key` column already exist.)

**Storage interface:** confirm `storage.Storage` exposes a `Get(ctx, key) (io.ReadCloser, ...)` (or
equivalent) for streaming; if only `Put` exists, add a `Get`. Reuse whatever the attachments/BAST code
already uses to read objects back.

## 6. Work item 4 — Playwright e2e (representative paths)

New spec `frontend/e2e/import-masterdata.spec.ts` (or extend `import.spec.ts`), real backend + seeded
admin, covering the distinct code paths:

| flow | exercises |
|---|---|
| **office** | scoped/complex target, existing but un-e2e'd |
| **reference:provinces** | flat, no FK, code-dedup |
| **reference:cities** | FK lookup (`provinsi`) + code-dedup |
| **reference:brands** | name-dedup (new pattern) |
| **reference:models** | FK lookup (`merek`) + composite `(brand,name)` dedup |

`units` is covered by unit/integration only (same shape as brands). Each flow: build a CSV/XLSX fixture
with **unique names/codes per run** (unique-constraint discipline — see the e2e memories), upload →
validate → confirm → assert the resulting rows appear (assert-after-search, wait for the wizard to reach
the result step). Include at least one **validation-rejection** assertion (a bad FK / duplicate row →
error count + downloadable error report). Follow the USelectMenu focus-trap + wait-modal-closed
conventions. Reuse `frontend/e2e/fixtures/import-fixtures.ts` helpers where possible.

Also add/extend **backend integration tests** (`import_integration_test.go`) for the new targets
(brand/model/unit create + dup-skip) and employee dept/position resolution, and **unit tests** for every
new `validateXRows` branch (required, miss, optional-empty, dup, FK-miss, scope where applicable).

## 7. Testing strategy (proactive & broad — per CLAUDE.md)

- **Unit** (Go): each new `validateXRows` — happy, required-missing, lookup-miss, optional-empty (dept/
  position/symbol), in-file dup, DB dup; `Execute` anti-poison pre-check (in-batch + DB-existing skip).
- **Unit** (Vitest): `master/import.vue` target/permission mapping for the 3 new targets;
  `reference.vue` Import-button visibility for brands/models/units; error map renders `dupNama`/`merek`.
- **Integration** (Go, `-tags=integration`): full validate→execute for brand/model/unit incl. mid-batch
  dup-collision (proving no tx poisoning); employee import with dept/position resolved and with
  unknown dept/position → row failed; error-report object written to MinIO + `error_report_key` set +
  endpoint streams it.
- **E2E** (Playwright): bagian 6.

## 8. Verification gate (must be green before done)

- Backend: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./... -p 1`
  (all packages — full integration gate).
- Spectral: `npx @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
  (update `openapi.yaml` only if a request/response contract changes — error-report streaming/query
  params, new target enum values in docs).
- Frontend: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`; e2e `pnpm test:e2e` (separate
  job, needs stack + seeded admin).

## 9. Docs & tracking

- `docs/PROGRESS.md`: in item 44's follow-up list, tick the four cleared items (dept/position;
  brand/model/unit targets; MinIO error reports; office/reference e2e) with the PR number; leave
  room/floor open; refresh the item-45 "Next session" block.
- Obsidian vault: update `Modul/Peta Modul.md` (import target list) and add a `Catatan/2026-07-12-*`
  session note; record the "reference import dedups by name, not code (brand/unit/model)" decision if it
  belongs in `Keputusan/Produk/`.
- Conventional commits, scope `import`: e.g. `feat(import): employee dept/position columns`,
  `feat(import): brand/model/unit reference targets`, `feat(import): persist error reports to MinIO`,
  `test(import): e2e office + reference import paths`.

## 10. Risks / open questions (resolve during implementation)

- Exact NOT NULL/optional column sets for `brands`/`models`/`units` in migration `000006` — the INSERT
  queries must match (e.g. does `brands` have a `code`? does `models` require anything beyond
  `brand_id,name`?). Verify before writing queries.
- `storage.Storage.Get` availability (bagian 5) — add if missing, mirroring attachments read-back.
- Whether the ImportWizard's client-side error map already knows `dupNama`/`merek` keys, or needs them
  added for human-readable messages.
