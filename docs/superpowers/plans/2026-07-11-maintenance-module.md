# Maintenance (Jadwal · Catatan · Laporan Kerusakan) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the maintenance lifecycle — recurring schedules (`next_due_date`), maintenance records whose status transitions drive the asset state machine (`in_progress → under_maintenance`, `completed → available`), a Staf damage-report path via the approval engine (`maintenance` executor), an explicit "Perlu Tindak Lanjut" queue for flagged assets, and a stock-opname `damaged` follow-up that creates a record directly — plus the `/maintenance` screen (3 tabs, 1:1 with `docs/design/Maintenance.dc.html`), an asset-detail history tab, and e2e.

**Architecture:** New backend module `internal/maintenance` (ADR-0008 four-file split + `executor.go`), mirroring `internal/assignment`. Schedules + records live in one package (completing a linked record updates the schedule's `last_done_date`/`next_due_date` via the new `schedule_id` FK). Damage reports submit through `POST /maintenance/reports` (multipart; optional photo stored via `asset.Service.UploadAttachment`) opening a `maintenance`-type approval request; on approval the registered executor inserts a corrective `scheduled` record (no asset flip — flip happens when work starts). `internal/stockopname` gets a `damaged` follow-up branch through a small `MaintenanceCreator` interface defined in the stockopname package, wired in `NewRouter`. `internal/assignment` is NOT touched.

**Tech Stack:** Go 1.25 + Gin, pgx/v5, sqlc, golang-migrate, testify + testcontainers-go (integration, `-tags=integration`); Nuxt 4 SPA + Nuxt UI v4, Vitest + @nuxt/test-utils, Playwright.

## Global Constraints

- Go module path `github.com/ragbuaj/inventra`; sqlc output `db/sqlc` — never hand-edit; edit `db/queries/*.sql` or migrations then `sqlc generate`.
- Every endpoint enforces data scope on **read and write**; maintenance scope basis is the **asset's office** (`asset.assets.office_id`), resolved via `common.ScopedDeps.CallerOfficeScope(c, "maintenance")`.
- Money/numeric columns are Go `string` (`cost numeric(18,2)` → `*string`); damage report is **not** value-tiered → approval `amount = "0"`.
- Soft-delete + partial-unique + `set_updated_at` conventions; both `maintenance.*` tables already exist (migration `000012`) — migration `000027` only adds link columns + seeds.
- Enum values (verify in `db/sqlc/models.go` after generate): `sqlc.SharedRequestTypeMaintenance`, `sqlc.SharedMaintenanceTypePreventive`/`Corrective`, `sqlc.SharedMaintenanceStatusScheduled`/`InProgress`/`Completed`/`Cancelled`, `sqlc.SharedAssetStatusAvailable`/`Assigned`/`UnderMaintenance`/`InTransfer`/`Disposed`/`Lost`, `sqlc.SharedOpnameItemResultDamaged`.
- Conventional Commits, lowercase scope: `feat(maintenance): …`. No Claude/AI attribution in commits.
- Frontend: i18n mandatory (`i18n/locales/{id,en}.json`, default `id`); theme via semantic tokens; build on `U*` components; ESLint `commaDangle: 'never'` + 1tbs.
- Branch: `feat/maintenance-module` (already created; spec committed there).
- Approved mockup deviations (record in PROGRESS.md at the end): "Tambah Jadwal" UI, row-click edit slideover, "Perlu Tindak Lanjut" section, vendor as `vendor_id` select, client-side due banner, in-page reminder only.

---

## File Structure

**Backend — create:**
- `backend/db/migrations/000027_maintenance_module.up.sql` / `.down.sql` — `schedule_id` + `followup_record_id` columns, `maintenance.view` permission, `maintenance` scope rows, `maintenance` threshold band.
- `backend/db/queries/maintenance.sql` — sqlc queries.
- `backend/internal/maintenance/service.go` — business rules + record state machine (Gin-free).
- `backend/internal/maintenance/dto.go` — request structs + serialization + payload.
- `backend/internal/maintenance/executor.go` — `maintenance` approval executor.
- `backend/internal/maintenance/handler.go` — HTTP ↔ service (incl. multipart report submit).
- `backend/internal/maintenance/routes.go` — route registration.
- `backend/internal/maintenance/dto_test.go` — unit tests (payload marshal, transition table).
- `backend/internal/maintenance/maintenance_integration_test.go` — integration tests (`//go:build integration`).

**Backend — modify:**
- `backend/internal/authzadmin/catalog.go` — `maintenance.view` + group; `"maintenance"` in `ScopeModules()`.
- `backend/internal/authzadmin/catalog_test.go` — if it counts groups/keys.
- `backend/db/queries/stockopname.sql` — `SetItemFollowupRecord` query.
- `backend/internal/stockopname/service.go` — `MaintenanceCreator` interface + `damaged` branch in `GenerateFollowup`; `NewService` gains the dependency.
- `backend/internal/stockopname/handler.go` — followup response includes `record_id` when set.
- `backend/internal/stockopname/stockopname_integration_test.go` + `backend/internal/stockopname/*_test.go` — `NewService` call sites gain the new arg.
- `backend/internal/server/router.go` — construct + wire the maintenance module; register executor; pass creator into stockopname.
- `backend/api/openapi.yaml` — schemas + 11 paths + followup response change.

**Frontend — create:**
- `frontend/app/constants/maintenanceMeta.ts` — status/type/due tone maps + due-diff helper.
- `frontend/app/composables/api/useMaintenance.ts` — real `$fetch` composable.
- `frontend/app/components/maintenance/ScheduleSlideover.vue` — create/edit schedule.
- `frontend/app/components/maintenance/RecordSlideover.vue` — create/edit record (prefillable).
- `frontend/app/pages/maintenance.vue` — the screen (3 tabs + banner + attention).
- `frontend/test/unit/maintenance-meta.spec.ts`, `frontend/test/nuxt/maintenance.spec.ts`, `frontend/test/nuxt/maintenance-slideovers.spec.ts`.
- `frontend/e2e/maintenance.spec.ts`.

**Frontend — modify:**
- `frontend/app/constants/approvalMeta.ts` — `RequestType` union += `'assignment' | 'maintenance'` + `TYPE_META`/`REQUEST_TYPE_KEYS` entries.
- `frontend/app/pages/approval.vue` — render `maintenance` payload (asset, problem category, description, photo link).
- `frontend/app/pages/assets/[tag]/index.vue` — "Riwayat Maintenance" tab.
- `frontend/app/pages/stock-opname.vue` + `frontend/app/composables/api/useStockOpname.ts` — enable `damaged` follow-up.
- `frontend/app/utils/nav.ts` + `frontend/test/unit/nav-model.spec.ts` — Maintenance nav item.
- `frontend/i18n/locales/{id,en}.json` — `maintenance.*` + `approval.type.*` additions.

---

## Task 1: Migration `000027` + permission catalog

**Files:**
- Create: `backend/db/migrations/000027_maintenance_module.up.sql`, `backend/db/migrations/000027_maintenance_module.down.sql`
- Modify: `backend/internal/authzadmin/catalog.go`, `backend/internal/authzadmin/catalog_test.go`

**Interfaces:**
- Produces: `maintenance_records.schedule_id`, `stock_opname_items.followup_record_id`; permission key `maintenance.view`; `data_scope_policies` rows for module `maintenance`; threshold row for `request_type='maintenance'`; `ScopeModules()` includes `"maintenance"`.

- [ ] **Step 1: Write the up migration**

`backend/db/migrations/000027_maintenance_module.up.sql`:
```sql
-- Maintenance module: schema links + seed. Tables exist since 000012.

-- Explicit record → schedule link: completing a linked record updates the
-- schedule's last_done/next_due (no implicit asset+category guessing).
ALTER TABLE maintenance.maintenance_records
  ADD COLUMN schedule_id uuid REFERENCES maintenance.maintenance_schedules (id);
CREATE INDEX idx_mrec_schedule_id ON maintenance.maintenance_records (schedule_id);

-- Traceability + idempotency for the stock-opname 'damaged' follow-up
-- (mirrors followup_request_id from 000025).
ALTER TABLE stockopname.stock_opname_items
  ADD COLUMN followup_record_id uuid REFERENCES maintenance.maintenance_records (id);
CREATE INDEX idx_opnitem_followup_record ON stockopname.stock_opname_items (followup_record_id);

-- Permissions: maintenance.view (read). maintenance.manage is already seeded
-- (000005) for Superadmin + Manager. Kepala get view (assignment.view precedent).
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('maintenance.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Data-scope for the 'maintenance' module (mirror 'assignments', 000026).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'maintenance', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- Damage report is not value-tiered: a single office-level approval step.
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order)
VALUES ('maintenance', 0, NULL, 'office', 1)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Write the down migration**

`backend/db/migrations/000027_maintenance_module.down.sql`:
```sql
DELETE FROM approval.approval_thresholds WHERE request_type = 'maintenance';
DELETE FROM identity.data_scope_policies WHERE module = 'maintenance';
DELETE FROM identity.role_permissions WHERE permission_key = 'maintenance.view';
DROP INDEX IF EXISTS stockopname.idx_opnitem_followup_record;
ALTER TABLE stockopname.stock_opname_items DROP COLUMN IF EXISTS followup_record_id;
DROP INDEX IF EXISTS maintenance.idx_mrec_schedule_id;
ALTER TABLE maintenance.maintenance_records DROP COLUMN IF EXISTS schedule_id;
```

- [ ] **Step 3: Update the permission catalog + scope modules**

In `backend/internal/authzadmin/catalog.go`, move `maintenance.manage` out of the `Cadangan` group into a real group and add `maintenance.view`. Replace:
```go
	{Group: "Cadangan", Items: []PermissionItem{
		{"report.view", "Lihat laporan"},
		{"report.export", "Ekspor laporan"},
		{"maintenance.manage", "Kelola maintenance"},
		{"valuation.exclude.approve", "Setujui pengecualian valuasi"},
	}},
```
with:
```go
	{Group: "Maintenance", Items: []PermissionItem{
		{"maintenance.view", "Lihat jadwal & catatan maintenance"},
		{"maintenance.manage", "Kelola jadwal & catatan maintenance"},
	}},
	{Group: "Cadangan", Items: []PermissionItem{
		{"report.view", "Lihat laporan"},
		{"report.export", "Ekspor laporan"},
		{"valuation.exclude.approve", "Setujui pengecualian valuasi"},
	}},
```

And update `ScopeModules()`:
```go
func ScopeModules() []string {
	return []string{"*", "offices", "employees", "assets", "requests", "audit", "transfers", "disposals", "depreciation", "assignments", "stockopname", "maintenance"}
}
```
(Keep `"stockopname"` if it is already present in the current file — do not drop existing entries; only append `"maintenance"`.)

- [ ] **Step 4: Run the catalog test**

Run: `cd backend && go test ./internal/authzadmin/ -v`
Expected: PASS. If `catalog_test.go` asserts fixed group/key counts, update them (one new group "Maintenance", one new key `maintenance.view`, `maintenance.manage` moved not added).

- [ ] **Step 5: Apply the migration against the dev DB**

Run:
```bash
cd backend && export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable" && migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: `000027_maintenance_module` applied. (If the dev stack is down, skip — integration tests apply migrations via testsupport.)

- [ ] **Step 6: Commit**

```bash
git add backend/db/migrations/000027_maintenance_module.up.sql backend/db/migrations/000027_maintenance_module.down.sql backend/internal/authzadmin/catalog.go backend/internal/authzadmin/catalog_test.go
git commit -m "feat(maintenance): migration 000027 (schedule_id, followup_record_id, seeds) + catalog"
```

---

## Task 2: sqlc queries

**Files:**
- Create: `backend/db/queries/maintenance.sql`
- Modify: `backend/db/queries/stockopname.sql` (append one query), (generated) `backend/db/sqlc/*` via `sqlc generate`

**Interfaces:**
- Produces (sqlc-generated Go): `CreateMaintSchedule`, `GetMaintScheduleScoped`, `ListMaintSchedulesEnriched`, `CountMaintSchedules`, `UpdateMaintSchedule`, `SoftDeleteMaintSchedule`, `TouchMaintScheduleDone`, `CreateMaintRecord`, `GetMaintRecordScoped`, `GetMaintRecordEnriched`, `ListMaintRecordsEnriched`, `CountMaintRecords`, `UpdateMaintRecord`, `ListMaintRecordsByAssetEnriched`, `CountActiveMaintRecordsByAsset`, `ListMaintAttentionAssets`, `CountPendingMaintRequests`, `SetItemFollowupRecord`.
- Enriched rows embed the base row (`sqlc.embed`) + `AssetName, AssetTag string`, `OfficeName, CategoryName, ProblemName, VendorName, ReportedByName *string`.

- [ ] **Step 1: Write `backend/db/queries/maintenance.sql`**

```sql
-- name: CreateMaintSchedule :one
INSERT INTO maintenance.maintenance_schedules (
  asset_id, maintenance_category_id, interval_months, next_due_date
) VALUES (
  sqlc.arg(asset_id), sqlc.narg(maintenance_category_id), sqlc.arg(interval_months), sqlc.arg(next_due_date)
)
RETURNING *;

-- name: GetMaintScheduleScoped :one
SELECT ms.* FROM maintenance.maintenance_schedules ms
JOIN asset.assets a ON a.id = ms.asset_id AND a.deleted_at IS NULL
WHERE ms.id = sqlc.arg(id) AND ms.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListMaintSchedulesEnriched :many
SELECT sqlc.embed(ms),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name
FROM maintenance.maintenance_schedules ms
JOIN asset.assets a ON a.id = ms.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = ms.maintenance_category_id AND mc.deleted_at IS NULL
WHERE ms.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(is_active)::boolean IS NULL OR ms.is_active = sqlc.narg(is_active))
ORDER BY ms.next_due_date ASC NULLS LAST
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountMaintSchedules :one
SELECT count(*)
FROM maintenance.maintenance_schedules ms
JOIN asset.assets a ON a.id = ms.asset_id AND a.deleted_at IS NULL
WHERE ms.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(is_active)::boolean IS NULL OR ms.is_active = sqlc.narg(is_active));

-- name: UpdateMaintSchedule :one
UPDATE maintenance.maintenance_schedules
SET maintenance_category_id = COALESCE(sqlc.narg(maintenance_category_id), maintenance_category_id),
    interval_months         = COALESCE(sqlc.narg(interval_months), interval_months),
    is_active               = COALESCE(sqlc.narg(is_active), is_active),
    next_due_date           = COALESCE(sqlc.narg(next_due_date), next_due_date)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMaintSchedule :execrows
UPDATE maintenance.maintenance_schedules
SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: TouchMaintScheduleDone :one
UPDATE maintenance.maintenance_schedules
SET last_done_date = sqlc.arg(last_done_date), next_due_date = sqlc.arg(next_due_date)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: CreateMaintRecord :one
INSERT INTO maintenance.maintenance_records (
  asset_id, schedule_id, maintenance_category_id, problem_category_id,
  type, status, scheduled_date, completed_date, cost, vendor_id,
  performed_by, description, reported_by_id
) VALUES (
  sqlc.arg(asset_id), sqlc.narg(schedule_id), sqlc.narg(maintenance_category_id), sqlc.narg(problem_category_id),
  sqlc.arg(type), sqlc.arg(status), sqlc.narg(scheduled_date), sqlc.narg(completed_date), sqlc.narg(cost), sqlc.narg(vendor_id),
  sqlc.narg(performed_by), sqlc.arg(description), sqlc.narg(reported_by_id)
)
RETURNING *;

-- name: GetMaintRecordScoped :one
SELECT mr.* FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
WHERE mr.id = sqlc.arg(id) AND mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: GetMaintRecordEnriched :one
SELECT sqlc.embed(mr),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name,
       pc.name     AS problem_name,
       v.name      AS vendor_name,
       u.name      AS reported_by_name
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = mr.maintenance_category_id AND mc.deleted_at IS NULL
LEFT JOIN masterdata.problem_categories pc ON pc.id = mr.problem_category_id AND pc.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
LEFT JOIN identity.users u ON u.id = mr.reported_by_id AND u.deleted_at IS NULL
WHERE mr.id = sqlc.arg(id) AND mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListMaintRecordsEnriched :many
SELECT sqlc.embed(mr),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name,
       pc.name     AS problem_name,
       v.name      AS vendor_name,
       u.name      AS reported_by_name
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = mr.maintenance_category_id AND mc.deleted_at IS NULL
LEFT JOIN masterdata.problem_categories pc ON pc.id = mr.problem_category_id AND pc.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
LEFT JOIN identity.users u ON u.id = mr.reported_by_id AND u.deleted_at IS NULL
WHERE mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.maintenance_status IS NULL OR mr.status = sqlc.narg(status))
  AND (sqlc.narg(mtype)::shared.maintenance_type IS NULL OR mr.type = sqlc.narg(mtype))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR v.name ILIKE '%' || sqlc.narg(search) || '%')
ORDER BY COALESCE(mr.scheduled_date, mr.created_at::date) DESC, mr.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountMaintRecords :one
SELECT count(*)
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
WHERE mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.maintenance_status IS NULL OR mr.status = sqlc.narg(status))
  AND (sqlc.narg(mtype)::shared.maintenance_type IS NULL OR mr.type = sqlc.narg(mtype))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR v.name ILIKE '%' || sqlc.narg(search) || '%');

-- name: UpdateMaintRecord :one
UPDATE maintenance.maintenance_records
SET status                  = COALESCE(sqlc.narg(status), status),
    maintenance_category_id = COALESCE(sqlc.narg(maintenance_category_id), maintenance_category_id),
    scheduled_date          = COALESCE(sqlc.narg(scheduled_date), scheduled_date),
    completed_date          = COALESCE(sqlc.narg(completed_date), completed_date),
    cost                    = COALESCE(sqlc.narg(cost), cost),
    vendor_id               = COALESCE(sqlc.narg(vendor_id), vendor_id),
    performed_by            = COALESCE(sqlc.narg(performed_by), performed_by),
    description             = COALESCE(sqlc.narg(description), description)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: ListMaintRecordsByAssetEnriched :many
SELECT sqlc.embed(mr),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name,
       pc.name     AS problem_name,
       v.name      AS vendor_name,
       u.name      AS reported_by_name
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = mr.maintenance_category_id AND mc.deleted_at IS NULL
LEFT JOIN masterdata.problem_categories pc ON pc.id = mr.problem_category_id AND pc.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
LEFT JOIN identity.users u ON u.id = mr.reported_by_id AND u.deleted_at IS NULL
WHERE mr.asset_id = sqlc.arg(asset_id) AND mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY mr.created_at DESC;

-- name: CountActiveMaintRecordsByAsset :one
-- Active = scheduled or in_progress. exclude_id lets the caller ignore the row
-- it is about to transition (release check).
SELECT count(*)
FROM maintenance.maintenance_records
WHERE asset_id = sqlc.arg(asset_id) AND deleted_at IS NULL
  AND status IN ('scheduled', 'in_progress')
  AND (sqlc.narg(exclude_id)::uuid IS NULL OR id <> sqlc.narg(exclude_id));

-- name: ListMaintAttentionAssets :many
-- Assets flagged under_maintenance (e.g. by assignment check-in) with no active
-- maintenance record — the "Perlu Tindak Lanjut" queue.
SELECT a.id, a.asset_tag, a.name, a.office_id, o.name AS office_name
FROM asset.assets a
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
WHERE a.deleted_at IS NULL AND a.status = 'under_maintenance'
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND NOT EXISTS (
    SELECT 1 FROM maintenance.maintenance_records mr
    WHERE mr.asset_id = a.id AND mr.deleted_at IS NULL
      AND mr.status IN ('scheduled', 'in_progress')
  )
ORDER BY a.updated_at DESC
LIMIT 100;

-- name: CountPendingMaintRequests :one
-- Duplicate-guard: pending maintenance request for the same asset by the same maker.
SELECT count(*)
FROM approval.requests
WHERE type = 'maintenance' AND status = 'pending' AND deleted_at IS NULL
  AND target_id = sqlc.arg(asset_id) AND requested_by_id = sqlc.arg(requested_by_id);
```

> If any column reference fails `sqlc generate` (e.g. `approval.requests` column names differ), open the generating migration (`000010_approval`) and fix the query to the actual names — do not guess.

- [ ] **Step 2: Append to `backend/db/queries/stockopname.sql`**

```sql
-- name: SetItemFollowupRecord :one
UPDATE stockopname.stock_opname_items
SET followup_record_id = sqlc.arg(followup_record_id)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id)
RETURNING *;
```

- [ ] **Step 3: Generate + build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: clean generate; build OK.

- [ ] **Step 4: Commit**

```bash
git add backend/db/queries/maintenance.sql backend/db/queries/stockopname.sql backend/db/sqlc
git commit -m "feat(maintenance): sqlc queries (schedules, records, attention, followup record)"
```

---

## Task 3: Service + DTO (schedules, record state machine)

**Files:**
- Create: `backend/internal/maintenance/service.go`, `backend/internal/maintenance/dto.go`, `backend/internal/maintenance/dto_test.go`

**Interfaces:**
- Consumes: sqlc queries (Task 2), `common.InScope`, `approval.Service.Submit`, `asset.Service.UploadAttachment(ctx, asset.UploadInput{AssetID, Filename, ContentType, Data, CreatedBy})`.
- Produces (used by handler/executor/stockopname):
  - `NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service, assets *asset.Service) *Service`
  - `CreateSchedule(ctx, all bool, ids []uuid.UUID, in ScheduleInput) (sqlc.MaintenanceMaintenanceSchedule, error)`
  - `UpdateSchedule(ctx, all, ids, id uuid.UUID, in ScheduleUpdateInput) (sqlc.MaintenanceMaintenanceSchedule, error)`
  - `DeleteSchedule(ctx, all, ids, id uuid.UUID) error`
  - `ListSchedules(ctx, all, ids, isActive *bool, limit, offset int32) ([]sqlc.ListMaintSchedulesEnrichedRow, int64, error)`
  - `CreateRecord(ctx, all, ids, createdBy uuid.UUID, in RecordInput) (sqlc.MaintenanceMaintenanceRecord, error)`
  - `UpdateRecord(ctx, all, ids, id uuid.UUID, in RecordUpdateInput) (sqlc.MaintenanceMaintenanceRecord, error)`
  - `GetRecord(ctx, id uuid.UUID, all, ids) (sqlc.GetMaintRecordEnrichedRow, error)`
  - `ListRecords(ctx, all, ids, status, mtype, search string, limit, offset int32) ([]sqlc.ListMaintRecordsEnrichedRow, int64, error)`
  - `ListByAsset(ctx, assetID uuid.UUID, all, ids) ([]sqlc.ListMaintRecordsByAssetEnrichedRow, error)`
  - `Attention(ctx, all, ids) ([]sqlc.ListMaintAttentionAssetsRow, error)`
  - `SubmitReport(ctx, caller approval.Caller, in ReportInput) (sqlc.ApprovalRequest, error)`
  - `CreateCorrectiveFromOpname(ctx, caller approval.Caller, assetID uuid.UUID, note *string) (uuid.UUID, error)`
  - Sentinels: `ErrNotFound, ErrOutOfScope, ErrInvalidRef, ErrAssetNotMaintainable, ErrAssetBusy, ErrInvalidTransition, ErrTerminal, ErrScheduleMismatch, ErrDuplicatePending, ErrInvalidInterval`
  - `MaintenancePayload{AssetID, ProblemCategoryID string; Description, AttachmentID *string}` + `marshalReportPayload`

- [ ] **Step 1: Write `dto.go`**

Mirror `internal/assignment/dto.go`: request structs with binding tags, JSON payload struct, map serializers.

```go
package maintenance

import (
	"encoding/json"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// CreateScheduleRequest is the POST /maintenance/schedules body.
type CreateScheduleRequest struct {
	AssetID               string  `json:"asset_id" binding:"required,uuid"`
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	IntervalMonths        int32   `json:"interval_months" binding:"required,min=1"`
	StartDate             string  `json:"start_date" binding:"required"` // "2006-01-02" → first next_due_date
}

// UpdateScheduleRequest is the PATCH /maintenance/schedules/:id body.
type UpdateScheduleRequest struct {
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	IntervalMonths        *int32  `json:"interval_months" binding:"omitempty,min=1"`
	IsActive              *bool   `json:"is_active"`
}

// CreateRecordRequest is the POST /maintenance/records body (Tambah Catatan slideover).
type CreateRecordRequest struct {
	AssetID               string  `json:"asset_id" binding:"required,uuid"`
	ScheduleID            *string `json:"schedule_id" binding:"omitempty,uuid"`
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	ProblemCategoryID     *string `json:"problem_category_id" binding:"omitempty,uuid"`
	Type                  string  `json:"type" binding:"required,oneof=preventive corrective"`
	Status                string  `json:"status" binding:"omitempty,oneof=scheduled in_progress completed cancelled"`
	ScheduledDate         *string `json:"scheduled_date"` // "2006-01-02"
	CompletedDate         *string `json:"completed_date"`
	Cost                  *string `json:"cost"`
	VendorID              *string `json:"vendor_id" binding:"omitempty,uuid"`
	Description           string  `json:"description" binding:"required"`
}

// UpdateRecordRequest is the PATCH /maintenance/records/:id body (edit slideover).
type UpdateRecordRequest struct {
	Status                *string `json:"status" binding:"omitempty,oneof=scheduled in_progress completed cancelled"`
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	ScheduledDate         *string `json:"scheduled_date"`
	CompletedDate         *string `json:"completed_date"`
	Cost                  *string `json:"cost"`
	VendorID              *string `json:"vendor_id" binding:"omitempty,uuid"`
	Description           *string `json:"description"`
}

// ReportForm is the POST /maintenance/reports multipart form (Staf damage report).
// The optional photo file arrives as form file "photo" (read in the handler).
type ReportForm struct {
	AssetID           string  `form:"asset_id" binding:"required,uuid"`
	ProblemCategoryID string  `form:"problem_category_id" binding:"required,uuid"`
	Description       *string `form:"description"`
}

// MaintenancePayload is the JSON stored in approval.requests.payload.
type MaintenancePayload struct {
	AssetID           string  `json:"asset_id"`
	ProblemCategoryID string  `json:"problem_category_id"`
	Description       *string `json:"description"`
	AttachmentID      *string `json:"attachment_id"`
}

func marshalReportPayload(assetID, problemID string, desc, attachmentID *string) ([]byte, error) {
	return json.Marshal(MaintenancePayload{AssetID: assetID, ProblemCategoryID: problemID, Description: desc, AttachmentID: attachmentID})
}

// toScheduleResponse serializes a schedule row.
func toScheduleResponse(s sqlc.MaintenanceMaintenanceSchedule) map[string]any {
	return map[string]any{
		"id":                      s.ID.String(),
		"asset_id":                s.AssetID.String(),
		"maintenance_category_id": uuidPtrStr(s.MaintenanceCategoryID),
		"interval_months":         s.IntervalMonths,
		"last_done_date":          common.DateStr(s.LastDoneDate),
		"next_due_date":           common.DateStr(s.NextDueDate),
		"is_active":               s.IsActive,
		"created_at":              common.TsStr(s.CreatedAt),
		"updated_at":              common.TsStr(s.UpdatedAt),
	}
}

// toRecordResponse serializes a record row.
func toRecordResponse(r sqlc.MaintenanceMaintenanceRecord) map[string]any {
	return map[string]any{
		"id":                      r.ID.String(),
		"asset_id":                r.AssetID.String(),
		"schedule_id":             uuidPtrStr(r.ScheduleID),
		"maintenance_category_id": uuidPtrStr(r.MaintenanceCategoryID),
		"problem_category_id":     uuidPtrStr(r.ProblemCategoryID),
		"type":                    string(r.Type),
		"status":                  string(r.Status),
		"scheduled_date":          common.DateStr(r.ScheduledDate),
		"completed_date":          common.DateStr(r.CompletedDate),
		"cost":                    r.Cost,
		"vendor_id":               uuidPtrStr(r.VendorID),
		"performed_by":            r.PerformedBy,
		"description":             r.Description,
		"reported_by_id":          uuidPtrStr(r.ReportedByID),
		"created_at":              common.TsStr(r.CreatedAt),
		"updated_at":              common.TsStr(r.UpdatedAt),
	}
}

func uuidPtrStr(u *uuid.UUID) any {
	if u == nil {
		return nil
	}
	return u.String()
}

// enrichScheduleMap / enrichRecordMap add resolved display names.
func enrichScheduleMap(m map[string]any, assetName, assetTag string, officeName, categoryName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["office_name"] = officeName
	m["category_name"] = categoryName
	return m
}

func enrichRecordMap(m map[string]any, assetName, assetTag string, officeName, categoryName, problemName, vendorName, reportedByName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["office_name"] = officeName
	m["category_name"] = categoryName
	m["problem_name"] = problemName
	m["vendor_name"] = vendorName
	m["reported_by_name"] = reportedByName
	return m
}
```
(Add the `github.com/google/uuid` import; adjust pointer-vs-value field shapes to whatever sqlc actually generated — nullable columns are pointers under `emit_pointers_for_null_types`.)

- [ ] **Step 2: Write `service.go`**

Mirror `internal/assignment/service.go` (sentinels + `mapDBError` + pgxpool tx). Key content:

```go
package maintenance

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

var (
	ErrNotFound             = errors.New("maintenance: not found")
	ErrOutOfScope           = errors.New("maintenance: office out of scope")
	ErrInvalidRef           = errors.New("maintenance: invalid reference")
	ErrAssetNotMaintainable = errors.New("maintenance: asset is disposed or lost")
	ErrAssetBusy            = errors.New("maintenance: asset is in transfer")
	ErrInvalidTransition    = errors.New("maintenance: invalid status transition")
	ErrTerminal             = errors.New("maintenance: record is completed/cancelled")
	ErrScheduleMismatch     = errors.New("maintenance: schedule belongs to another asset")
	ErrDuplicatePending     = errors.New("maintenance: a pending report already exists for this asset")
	ErrInvalidInterval      = errors.New("maintenance: interval must be >= 1 month")
)

type Service struct {
	q      *sqlc.Queries
	pool   *pgxpool.Pool
	appr   *approval.Service
	assets *asset.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service, assets *asset.Service) *Service {
	return &Service{q: q, pool: pool, appr: appr, assets: assets}
}
```

Inputs:
```go
type ScheduleInput struct {
	AssetID               uuid.UUID
	MaintenanceCategoryID *uuid.UUID
	IntervalMonths        int32
	StartDate             string // first next_due_date
}

type ScheduleUpdateInput struct {
	MaintenanceCategoryID *uuid.UUID
	IntervalMonths        *int32
	IsActive              *bool
}

type RecordInput struct {
	AssetID               uuid.UUID
	ScheduleID            *uuid.UUID
	MaintenanceCategoryID *uuid.UUID
	ProblemCategoryID     *uuid.UUID
	Type                  sqlc.SharedMaintenanceType
	Status                sqlc.SharedMaintenanceStatus // "" → scheduled
	ScheduledDate         *string
	CompletedDate         *string
	Cost                  *string
	VendorID              *uuid.UUID
	PerformedBy           *string
	Description           string
	ReportedByID          *uuid.UUID
}

type RecordUpdateInput struct {
	Status                *sqlc.SharedMaintenanceStatus
	MaintenanceCategoryID *uuid.UUID
	ScheduledDate         *string
	CompletedDate         *string
	Cost                  *string
	VendorID              *uuid.UUID
	Description           *string
}

type ReportInput struct {
	AssetID           uuid.UUID
	ProblemCategoryID uuid.UUID
	Description       *string
	Photo             *PhotoInput // nil when no file uploaded
}

type PhotoInput struct {
	Filename    string
	ContentType string
	Data        []byte
}
```

Core rules (each mutation loads the asset via `s.q.GetAsset`, checks `common.InScope(all, ids, asset.OfficeID)` → `ErrOutOfScope`, and rejects `disposed`/`lost` assets → `ErrAssetNotMaintainable`):

- `CreateSchedule`: also `IntervalMonths >= 1` (`ErrInvalidInterval`); parse `StartDate` (`ErrInvalidRef` on bad date) → `next_due_date`. Plain insert (no tx needed).
- `UpdateSchedule`: `GetMaintScheduleScoped` first (`ErrNotFound` covers out-of-scope). If `IntervalMonths` changes and `last_done_date` is set, recompute `next_due_date = last_done_date.AddDate(0, int(interval), 0)` and pass it; otherwise leave `next_due_date` nil (COALESCE keeps it).
- `DeleteSchedule`: `GetMaintScheduleScoped` then `SoftDeleteMaintSchedule` (0 rows → `ErrNotFound`).
- **Record status transitions** — validate with a package-level helper (unit-tested in `dto_test.go`):

```go
// validTransition reports whether a record may move from → to.
// scheduled → scheduled|in_progress|completed|cancelled
// in_progress → in_progress|completed|cancelled
// completed / cancelled are terminal.
func validTransition(from, to sqlc.SharedMaintenanceStatus) bool {
	if from == to {
		return from != sqlc.SharedMaintenanceStatusCompleted && from != sqlc.SharedMaintenanceStatusCancelled
	}
	switch from {
	case sqlc.SharedMaintenanceStatusScheduled:
		return true
	case sqlc.SharedMaintenanceStatusInProgress:
		return to == sqlc.SharedMaintenanceStatusCompleted || to == sqlc.SharedMaintenanceStatusCancelled
	default:
		return false
	}
}
```

- `CreateRecord`: default `Status` to `scheduled`; if `ScheduleID` set, `GetMaintScheduleScoped` and compare `AssetID` (`ErrScheduleMismatch`). Then in a tx: `CreateMaintRecord` + `applyStatusEffects(ctx, qtx, rec, asset, "")` (below). Commit.
- `UpdateRecord`: `GetMaintRecordScoped` (`ErrNotFound`); if current status is completed/cancelled → `ErrTerminal`; if `in.Status != nil && !validTransition(cur, *in.Status)` → `ErrInvalidTransition`. Tx: `UpdateMaintRecord` + `applyStatusEffects(ctx, qtx, updated, asset, cur)`.
- `applyStatusEffects(ctx, qtx, rec, asset, prev)` — the FR-4.3 engine, all inside the caller's tx:

```go
// applyStatusEffects flips the asset + touches the linked schedule after a
// record lands in status rec.Status. prev is the pre-update status ("" on create).
func (s *Service) applyStatusEffects(ctx context.Context, qtx *sqlc.Queries, rec sqlc.MaintenanceMaintenanceRecord, a sqlc.AssetAsset, prev sqlc.SharedMaintenanceStatus) error {
	switch rec.Status {
	case sqlc.SharedMaintenanceStatusInProgress:
		switch a.Status {
		case sqlc.SharedAssetStatusAvailable, sqlc.SharedAssetStatusAssigned:
			if _, err := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: a.ID, Status: sqlc.SharedAssetStatusUnderMaintenance}); err != nil {
				return mapDBError(err)
			}
		case sqlc.SharedAssetStatusUnderMaintenance: // already flagged — no-op
		default: // in_transfer etc.
			return ErrAssetBusy
		}
	case sqlc.SharedMaintenanceStatusCompleted, sqlc.SharedMaintenanceStatusCancelled:
		// Completed: ensure completed_date + touch the linked schedule.
		if rec.Status == sqlc.SharedMaintenanceStatusCompleted {
			if rec.CompletedDate == nil || !rec.CompletedDate.Valid {
				// set today inside the same UPDATE the caller just did — do it here:
				// qtx.UpdateMaintRecord with completed_date = today (see note below)
			}
			if rec.ScheduleID != nil {
				sched, err := qtx.GetMaintScheduleScoped(ctx, sqlc.GetMaintScheduleScopedParams{ID: *rec.ScheduleID, AllScope: true, OfficeIds: []uuid.UUID{}})
				if err != nil {
					return mapDBError(err)
				}
				done := completedOrToday(rec)
				next := done.AddDate(0, int(sched.IntervalMonths), 0)
				if _, err := qtx.TouchMaintScheduleDone(ctx, sqlc.TouchMaintScheduleDoneParams{
					ID: sched.ID,
					LastDoneDate: pgtype.Date{Time: done, Valid: true},
					NextDueDate:  pgtype.Date{Time: next, Valid: true},
				}); err != nil {
					return mapDBError(err)
				}
			}
		}
		// Release the asset only if it is under_maintenance and this was its last
		// active record.
		if a.Status == sqlc.SharedAssetStatusUnderMaintenance {
			n, err := qtx.CountActiveMaintRecordsByAsset(ctx, sqlc.CountActiveMaintRecordsByAssetParams{AssetID: a.ID, ExcludeID: &rec.ID})
			if err != nil {
				return mapDBError(err)
			}
			if n == 0 {
				if _, err := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: a.ID, Status: sqlc.SharedAssetStatusAvailable}); err != nil {
					return mapDBError(err)
				}
			}
		}
	}
	return nil
}
```
Implementation note: the cleanest way to guarantee `completed_date` is set is in the **service** before the insert/update — when target status is `completed` and no `CompletedDate` input, pass today (`time.Now().Format("2006-01-02")`). Do that; then `applyStatusEffects` never needs to write the record itself (`completedOrToday` just reads `rec.CompletedDate`).

- `SubmitReport`: scope + maintainability guards; `CountPendingMaintRequests(assetID, caller.UserID) > 0` → `ErrDuplicatePending`. If `in.Photo != nil` → `s.assets.UploadAttachment(ctx, asset.UploadInput{AssetID: in.AssetID, Filename: in.Photo.Filename, ContentType: in.Photo.ContentType, Data: in.Photo.Data, CreatedBy: caller.UserID})`; take `att.ID.String()` as `attachmentID` (asset-service sentinels bubble up — the handler maps `asset.ErrUnsupportedType`/`ErrTooLarge` to 422). Then:

```go
	entity := "asset"
	targetID := in.AssetID
	return s.appr.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeMaintenance,
		Amount:       "0",
		OfficeID:     a.OfficeID,
		TargetEntity: &entity,
		TargetID:     &targetID,
		Payload:      payload,
		Reason:       in.Description,
		Maker:        caller.UserID,
	})
```

- `CreateCorrectiveFromOpname` (used by stockopname `damaged` follow-up): scope + maintainability guards with `caller.AllScope/OfficeIDs`; description = `*note` or fallback `"Tindak lanjut stock opname: aset rusak"`; insert (no tx needed — single statement, no status effects since status is `scheduled`):

```go
func (s *Service) CreateCorrectiveFromOpname(ctx context.Context, caller approval.Caller, assetID uuid.UUID, note *string) (uuid.UUID, error) {
	a, err := s.q.GetAsset(ctx, assetID)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	if !common.InScope(caller.AllScope, caller.OfficeIDs, a.OfficeID) {
		return uuid.Nil, ErrOutOfScope
	}
	if a.Status == sqlc.SharedAssetStatusDisposed || a.Status == sqlc.SharedAssetStatusLost {
		return uuid.Nil, ErrAssetNotMaintainable
	}
	desc := "Tindak lanjut stock opname: aset rusak"
	if note != nil && *note != "" {
		desc = *note
	}
	today := pgtype.Date{Time: time.Now(), Valid: true}
	rec, err := s.q.CreateMaintRecord(ctx, sqlc.CreateMaintRecordParams{
		AssetID:      assetID,
		Type:         sqlc.SharedMaintenanceTypeCorrective,
		Status:       sqlc.SharedMaintenanceStatusScheduled,
		ScheduledDate: today,
		Description:  desc,
		ReportedByID: &caller.UserID,
	})
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return rec.ID, nil
}
```

Plus straightforward `ListSchedules`/`ListRecords`/`GetRecord`/`ListByAsset`/`Attention` wrappers (nil-safe `ids`, `mapDBError`) mirroring assignment's `List`/`Get`/`ListByAsset`.

- [ ] **Step 3: Write the unit tests**

`backend/internal/maintenance/dto_test.go`:
```go
package maintenance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestMarshalReportPayload(t *testing.T) {
	desc := "layar pecah"
	att := "9f0d1c8e-0000-0000-0000-000000000001"
	b, err := marshalReportPayload("a-id", "p-id", &desc, &att)
	require.NoError(t, err)
	var p MaintenancePayload
	require.NoError(t, json.Unmarshal(b, &p))
	require.Equal(t, "a-id", p.AssetID)
	require.Equal(t, "p-id", p.ProblemCategoryID)
	require.Equal(t, "layar pecah", *p.Description)
	require.Equal(t, att, *p.AttachmentID)

	b, err = marshalReportPayload("a-id", "p-id", nil, nil)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &p))
	require.Nil(t, p.Description)
	require.Nil(t, p.AttachmentID)
}

func TestValidTransition(t *testing.T) {
	sch, prog := sqlc.SharedMaintenanceStatusScheduled, sqlc.SharedMaintenanceStatusInProgress
	done, canc := sqlc.SharedMaintenanceStatusCompleted, sqlc.SharedMaintenanceStatusCancelled
	cases := []struct {
		from, to sqlc.SharedMaintenanceStatus
		ok       bool
	}{
		{sch, sch, true}, {sch, prog, true}, {sch, done, true}, {sch, canc, true},
		{prog, prog, true}, {prog, done, true}, {prog, canc, true}, {prog, sch, false},
		{done, prog, false}, {done, done, false}, {done, canc, false},
		{canc, sch, false}, {canc, canc, false},
	}
	for _, c := range cases {
		require.Equal(t, c.ok, validTransition(c.from, c.to), "%s -> %s", c.from, c.to)
	}
}
```

- [ ] **Step 4: Build + run unit tests**

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/maintenance/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/maintenance
git commit -m "feat(maintenance): service + dto (schedules, record state machine, report submit)"
```

---

## Task 4: Executor + handler + routes

**Files:**
- Create: `backend/internal/maintenance/executor.go`, `backend/internal/maintenance/handler.go`, `backend/internal/maintenance/routes.go`

**Interfaces:**
- Consumes: `Service` (Task 3), `approval.Executor` contract (`Execute(ctx, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error` — see `internal/assignment/executor.go`), `common.ScopedDeps`, `audit.Record`.
- Produces: `(*Service).Executor() approval.Executor`; `NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler`; `RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView, requireCreate gin.HandlerFunc)`.

- [ ] **Step 1: Write `executor.go`**

```go
package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// maintenanceExec creates the corrective 'scheduled' record on final approval of
// a Staf damage report, inside the commit tx. The asset is NOT flipped here —
// it flips when the Manager starts the work (record → in_progress, FR-4.3).
type maintenanceExec struct{ s *Service }

func (e maintenanceExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return approval.ErrInvalidRef
	}
	var p MaintenancePayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return approval.ErrInvalidRef
	}
	asset, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
	}
	if asset.Status == sqlc.SharedAssetStatusDisposed || asset.Status == sqlc.SharedAssetStatusLost {
		return approval.ErrConflict // asset no longer maintainable at approval time
	}
	problemID, err := uuid.Parse(p.ProblemCategoryID)
	if err != nil {
		return approval.ErrInvalidRef
	}
	desc := "Laporan kerusakan"
	if p.Description != nil && *p.Description != "" {
		desc = *p.Description
	}
	_, err = qtx.CreateMaintRecord(ctx, sqlc.CreateMaintRecordParams{
		AssetID:           *req.TargetID,
		ProblemCategoryID: &problemID,
		Type:              sqlc.SharedMaintenanceTypeCorrective,
		Status:            sqlc.SharedMaintenanceStatusScheduled,
		ScheduledDate:     pgtype.Date{Time: time.Now(), Valid: true},
		Description:       desc,
		ReportedByID:      &req.RequestedByID,
	})
	return err
}

// Executor returns the maintenance approval executor.
func (s *Service) Executor() approval.Executor { return maintenanceExec{s} }
```
(Match `CreateMaintRecordParams` field shapes to the generated code — nullable date params are `pgtype.Date`.)

- [ ] **Step 2: Write `handler.go`**

Mirror `internal/assignment/handler.go` exactly (`scopeModule = "maintenance"`, `caller()` helper, `svcError`). Specifics:

- `svcError` mapping: `ErrNotFound` → 404; `ErrOutOfScope` → 403; `ErrInvalidTransition`, `ErrTerminal`, `ErrDuplicatePending` → 409; `ErrAssetNotMaintainable`, `ErrAssetBusy`, `ErrInvalidRef`, `ErrScheduleMismatch`, `ErrInvalidInterval`, `asset.ErrUnsupportedType`, `asset.ErrTooLarge` → 422; default `common.WriteError`.
- Handlers: `createSchedule`, `listSchedules` (query `is_active`, `limit`, `offset` via `common.ClampInt`), `updateSchedule`, `deleteSchedule` (204), `createRecord`, `listRecords` (query `status`, `type`, `q`/search, pagination), `getRecord`, `updateRecord`, `attention`, `submitReport`, `listByAsset`. List responses: `gin.H{"data": …, "total": total, "limit": limit, "offset": offset}`; attention + listByAsset return `gin.H{"data": …}`.
- `submitReport` reads the multipart form:

```go
func (h *Handler) submitReport(c *gin.Context) {
	var form ReportForm
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, _, _, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	assetID, _ := uuid.Parse(form.AssetID)
	problemID, _ := uuid.Parse(form.ProblemCategoryID)
	in := ReportInput{AssetID: assetID, ProblemCategoryID: problemID, Description: form.Description}
	if fh, ferr := c.FormFile("photo"); ferr == nil && fh != nil {
		f, oerr := fh.Open()
		if oerr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo"})
			return
		}
		data, rerr := io.ReadAll(f)
		f.Close()
		if rerr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo"})
			return
		}
		in.Photo = &PhotoInput{Filename: fh.Filename, ContentType: fh.Header.Get("Content-Type"), Data: data}
	}
	out, err := h.svc.SubmitReport(c.Request.Context(), caller, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "requests", out.ID, out.OfficeID, audit.Diff(nil, map[string]any{"request_id": out.ID.String(), "type": "maintenance", "asset_id": form.AssetID}))
	c.JSON(http.StatusCreated, gin.H{"request_id": out.ID.String(), "status": string(out.Status)})
}
```
- Audit every mutation: `audit.Record(c, h.aud, audit.ActionCreate|Update|Delete, "maintenance_schedules"|"maintenance_records", id, nil, audit.Diff(before, after))` (mirror assignment's checkout/checkin handlers).

- [ ] **Step 3: Write `routes.go`**

```go
package maintenance

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts maintenance endpoints. Reads require maintenance.view;
// writes require maintenance.manage; the Staf damage-report submit requires
// request.create. Per-asset history is under /assets/:id/maintenance.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView, requireCreate gin.HandlerFunc) {
	g := rg.Group("/maintenance")
	g.GET("/schedules", authMW, requireView, h.listSchedules)
	g.POST("/schedules", authMW, requireManage, h.createSchedule)
	g.PATCH("/schedules/:id", authMW, requireManage, h.updateSchedule)
	g.DELETE("/schedules/:id", authMW, requireManage, h.deleteSchedule)
	g.GET("/records", authMW, requireView, h.listRecords)
	g.POST("/records", authMW, requireManage, h.createRecord)
	g.GET("/records/:id", authMW, requireView, h.getRecord)
	g.PATCH("/records/:id", authMW, requireManage, h.updateRecord)
	g.GET("/attention", authMW, requireView, h.attention)
	g.POST("/reports", authMW, requireCreate, h.submitReport)

	rg.GET("/assets/:id/maintenance", authMW, requireView, h.listByAsset)
}
```

- [ ] **Step 4: Build + vet**

Run: `cd backend && go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/maintenance
git commit -m "feat(maintenance): executor + handler + routes"
```

---

## Task 5: Stock-opname `damaged` follow-up + router wiring

**Files:**
- Modify: `backend/internal/stockopname/service.go`, `backend/internal/stockopname/handler.go`, `backend/internal/server/router.go`, plus every `stockopname.NewService(` call site in `backend/internal/stockopname/*_test.go` / `backend/internal/approval/integration_test.go` (grep first).

**Interfaces:**
- Consumes: `maintenance.Service.CreateCorrectiveFromOpname` (Task 3), `SetItemFollowupRecord` (Task 2).
- Produces: `stockopname.MaintenanceCreator` interface; `stockopname.NewService(q, pool, disp, tr, maint MaintenanceCreator)`; `GenerateFollowup` returns `(uuid.UUID, string, error)` where type is now `"asset_disposal" | "asset_transfer" | "maintenance_record"`.

- [ ] **Step 1: Define the interface + extend `NewService`**

In `backend/internal/stockopname/service.go`:
```go
// MaintenanceCreator creates a corrective maintenance record for a damaged
// opname item. Defined here (consumer side) so stockopname does not import the
// maintenance package; *maintenance.Service satisfies it, wired in NewRouter.
type MaintenanceCreator interface {
	CreateCorrectiveFromOpname(ctx context.Context, caller approval.Caller, assetID uuid.UUID, note *string) (uuid.UUID, error)
}
```
Add a `maint MaintenanceCreator` field to `Service` and a fifth `NewService` parameter.

- [ ] **Step 2: Add the `damaged` branch in `GenerateFollowup`**

Guard both link columns, then branch. Replace the `if item.FollowupRequestID != nil { … }` guard with:
```go
	if item.FollowupRequestID != nil || item.FollowupRecordID != nil {
		return uuid.Nil, "", ErrAlreadyFollowedUp
	}
```
Add before `default:`:
```go
	case sqlc.SharedOpnameItemResultDamaged:
		recID, err := s.maint.CreateCorrectiveFromOpname(ctx, caller, item.AssetID, in.Reason)
		if err != nil {
			return uuid.Nil, "", err
		}
		if _, err := s.q.SetItemFollowupRecord(ctx, sqlc.SetItemFollowupRecordParams{
			ID: itemID, SessionID: sessionID, FollowupRecordID: &recID,
		}); err != nil {
			return uuid.Nil, "", mapDBError(err)
		}
		return recID, "maintenance_record", nil
```
Update the function comment (damaged no longer rejected). The shared `SetItemFollowup` call at the bottom stays for the two request-creating branches only — restructure so each branch does its own link write and returns (the disposal/transfer branches keep `SetItemFollowup`), or keep the tail write guarded by `reqType != "maintenance_record"`; prefer per-branch writes for clarity.

- [ ] **Step 3: Surface `record_id` in the handler + item serialization**

In `backend/internal/stockopname/handler.go` `followup()`: response becomes
```go
	body := gin.H{"type": reqType}
	if reqType == "maintenance_record" {
		body["record_id"] = reqID.String()
	} else {
		body["request_id"] = reqID.String()
	}
	c.JSON(http.StatusOK, body)
```
Adjust the audit diff key accordingly (`followup_record_id` vs `followup_request_id`). Wherever items are serialized with `followup_request_id`, also emit `followup_record_id` (grep `followup_request_id` in the package).

- [ ] **Step 4: Wire the router**

In `backend/internal/server/router.go`, before the stockopname block:
```go
		maintenanceSvc := maintenance.NewService(queries, d.Pool, approvalSvc, assetSvc)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeMaintenance, maintenanceSvc.Executor())
```
Change the stockopname construction:
```go
		stockopnameSvc := stockopname.NewService(queries, d.Pool, disposalSvc, transferSvc, maintenanceSvc)
```
After the assignment block (order irrelevant, keep together):
```go
		maintenanceHandler := maintenance.NewHandler(maintenanceSvc, scopeSvc, queries, auditSvc)
		maintenance.RegisterRoutes(api, maintenanceHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "maintenance.manage"),
			middleware.RequirePermission(permSvc, "maintenance.view"),
			middleware.RequirePermission(permSvc, "request.create"),
		)
```
Add the `"github.com/ragbuaj/inventra/internal/maintenance"` import.

- [ ] **Step 5: Fix `NewService` call sites in tests**

Run: `cd backend && go build ./... && go vet ./... && go test ./... 2>&1 | head -50`
Grep `stockopname.NewService(` across the repo; existing tests pass `nil` or a stub for the new argument only where the damaged path is not exercised — pass `maintenanceSvc` in integration tests that have one, else `nil` (the damaged branch is the only consumer).
Expected after fixes: build + vet + unit tests green.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/stockopname backend/internal/server/router.go
git commit -m "feat(maintenance): stock-opname damaged follow-up creates record + router wiring"
```

---

## Task 6: Integration tests (maintenance module)

**Files:**
- Create: `backend/internal/maintenance/maintenance_integration_test.go` (`//go:build integration`)
- Modify (only if the damaged-followup test lives better there): `backend/internal/stockopname/stockopname_integration_test.go`

**Interfaces:**
- Consumes: `internal/testsupport` harness (see `assignment_integration_test.go` for the setup: container Postgres + migrations + seeded roles/offices/users + `approval.Service` with registered executor).

- [ ] **Step 1: Write the integration tests**

Mirror the setup in `backend/internal/assignment/assignment_integration_test.go` (testsupport bootstrap, helper builders for office/asset/users per role). Register the executor: `apprSvc.RegisterExecutor(sqlc.SharedRequestTypeMaintenance, msvc.Executor())`. Cover — one focused test function each:

1. `TestScheduleCRUD_HappyPath` — create (asset in scope) → next_due = start date; list returns enriched row; update interval with last_done set → next_due recomputed; soft delete → list empty.
2. `TestScheduleCreate_Guards` — out-of-scope asset → `ErrOutOfScope`; disposed asset → `ErrAssetNotMaintainable`; interval 0 rejected at binding/service (`ErrInvalidInterval`).
3. `TestRecordCreate_StatusEffects` — create `in_progress` on available asset → asset `under_maintenance`; create `scheduled` → asset untouched.
4. `TestRecordComplete_ReleasesAndTouchesSchedule` — record linked to schedule, complete it → asset back `available`, schedule `last_done_date = completed_date`, `next_due_date = completed + interval` (assert exact dates).
5. `TestRecordComplete_KeepsAssetWhenAnotherActive` — two records on one asset (one in_progress, one scheduled); complete the in_progress → asset **stays** `under_maintenance`; complete/cancel the second → asset `available`.
6. `TestRecordTransition_Invalid` — completed → in_progress rejected `ErrInvalidTransition`; update on completed record rejected `ErrTerminal`; in_progress on `in_transfer` asset rejected `ErrAssetBusy`; schedule of another asset → `ErrScheduleMismatch`.
7. `TestRecordScope_ReadAndWrite` — out-of-scope caller: list excludes, get → `ErrNotFound`, create/update → `ErrOutOfScope`/`ErrNotFound`.
8. `TestSubmitReport_ApproveCreatesRecord` — Staf submits (payload asserted); duplicate pending → `ErrDuplicatePending`; Manager (different user, same office) approves via `apprSvc` → corrective `scheduled` record exists with `reported_by_id` = maker, `problem_category_id` from payload; asset status unchanged.
9. `TestSubmitReport_RejectLeavesNoRecord` — reject → zero records.
10. `TestAttention_Queue` — asset flipped `under_maintenance` directly (simulate check-in flag via `SetAssetStatus`) → appears in `Attention`; create a scheduled record for it → disappears; out-of-scope asset never appears.
11. `TestOpnameDamagedFollowup` — opname session + damaged item → `GenerateFollowup` returns `("maintenance_record", recID)`; item has `followup_record_id`; second call → `ErrAlreadyFollowedUp`; `not_found`/`misplaced` behavior unchanged (regression assert on one of them).

- [ ] **Step 2: Run the module integration tests**

Run: `cd backend && go test -tags=integration ./internal/maintenance/ ./internal/stockopname/ -v -count=1`
Expected: PASS (Docker required).

- [ ] **Step 3: Run the FULL integration suite (repo rule for shared-signature changes)**

Run: `cd backend && go test -tags=integration ./... -count=1`
Expected: ALL packages PASS (stockopname `NewService` signature changed — this is mandatory, not optional).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/maintenance backend/internal/stockopname
git commit -m "test(maintenance): integration coverage (state machine, executor, attention, opname followup)"
```

---

## Task 7: OpenAPI + backend gate

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Document the API**

Follow the existing style (see the `Assignment` schema + paths). Add:
- Schemas: `MaintenanceSchedule` (id, asset_id, maintenance_category_id, interval_months, last_done_date, next_due_date, is_active, asset_name, asset_tag, office_name, category_name, timestamps), `MaintenanceRecord` (all record fields + enriched names), `MaintenanceAttentionItem` (id, asset_tag, name, office_id, office_name).
- Paths: `GET|POST /maintenance/schedules`, `PATCH|DELETE /maintenance/schedules/{id}`, `GET|POST /maintenance/records`, `GET|PATCH /maintenance/records/{id}`, `GET /maintenance/attention`, `POST /maintenance/reports` (multipart/form-data: asset_id, problem_category_id, description, photo binary → 201 `{request_id, status}`), `GET /assets/{id}/maintenance`.
- Update the stock-opname follow-up response: `oneOf`-style description or two optional properties `request_id`/`record_id` + `type` enum now includes `maintenance_record`.

- [ ] **Step 2: Lint + full backend gate**

Run:
```bash
npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
cd backend && go build ./... && go vet ./... && go test ./...
```
Expected: Spectral 0 errors (the pre-existing `AssetCreatePayload` unused-component warning persists — unrelated); build/vet/test green.

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(maintenance): OpenAPI paths + schemas"
```

---

## Task 8: Frontend composable + meta constants + RequestType union

**Files:**
- Create: `frontend/app/constants/maintenanceMeta.ts`, `frontend/app/composables/api/useMaintenance.ts`, `frontend/test/unit/maintenance-meta.spec.ts`
- Modify: `frontend/app/constants/approvalMeta.ts`

**Interfaces:**
- Consumes: `useApiClient().request` (see `useAssignment.ts`), `formatDateID` (from `assignmentMeta` — reuse, do not duplicate).
- Produces:
  - `useMaintenance()` → `{ schedules, createSchedule, updateSchedule, deleteSchedule, records, record, createRecord, updateRecord, attention, listByAsset, submitReport, myReports }`
  - Types `MaintenanceSchedule`, `MaintenanceRecord`, `AttentionItem`, `SchedulePage`, `RecordPage`, input types.
  - `maintenanceMeta`: `MAINT_STATUS_TONE`, `MAINT_TYPE_TONE`, `dueKind(diffDays)`, `dueDiffDays(nextDue, today?)`, `formatRupiah(v)`.
  - `approvalMeta.RequestType` includes `'assignment' | 'maintenance'`.

- [ ] **Step 1: Extend `approvalMeta.ts`**

```ts
export type RequestType = 'asset_create' | 'asset_disposal' | 'asset_transfer' | 'assignment' | 'maintenance' | 'valuation_exclusion'
```
Add both to `REQUEST_TYPE_KEYS` and `TYPE_META` (`assignment`: icon `i-lucide-hand`, tone `info`, sensitive `false`; `maintenance`: icon `i-lucide-wrench`, tone `warning`, sensitive `false`). Remove the local `as RequestType`-style cast in the peminjaman path (grep `'assignment' as` in `frontend/app` — delete the workaround noted in PROGRESS item 36). Add i18n keys `approval.type.assignment` / `approval.type.maintenance` in both locales (Task 12 consolidates i18n; add these now so typecheck/tests pass).

- [ ] **Step 2: Write `maintenanceMeta.ts`**

```ts
import type { BadgeColor } from '~/types'

export type MaintenanceStatus = 'scheduled' | 'in_progress' | 'completed' | 'cancelled'
export type MaintenanceType = 'preventive' | 'corrective'

export const MAINT_STATUS_TONE: Record<MaintenanceStatus, BadgeColor> = {
  scheduled: 'neutral',
  in_progress: 'info',
  completed: 'success',
  cancelled: 'error'
}

export const MAINT_TYPE_TONE: Record<MaintenanceType, BadgeColor> = {
  preventive: 'info',
  corrective: 'warning'
}

export type DueKind = 'overdue' | 'today' | 'soon' | 'normal'

/** Whole-day difference next_due - today (negative = overdue). */
export function dueDiffDays(nextDue: string | null | undefined, today: Date = new Date()): number | null {
  if (!nextDue) return null
  const d = new Date(nextDue)
  if (Number.isNaN(d.getTime())) return null
  const t0 = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate())
  const t1 = Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())
  return Math.round((t1 - t0) / 86400000)
}

/** Mockup badge semantics: overdue/today = red, <=7 days = yellow, else neutral. */
export function dueKind(diff: number | null): DueKind {
  if (diff === null) return 'normal'
  if (diff < 0) return 'overdue'
  if (diff === 0) return 'today'
  if (diff <= 7) return 'soon'
  return 'normal'
}

/** "2350000" → "Rp 2.350.000"; empty/zero-ish → "—". */
export function formatRupiah(v: string | number | null | undefined): string {
  const n = typeof v === 'string' ? Number(v) : v
  if (!n || Number.isNaN(n)) return '—'
  return `Rp ${n.toLocaleString('id-ID')}`
}
```

- [ ] **Step 3: Write `useMaintenance.ts`**

Mirror `useAssignment.ts` (`useApiClient().request`). Endpoints: `/maintenance/schedules` (GET list `{is_active?, limit?, offset?}`, POST, PATCH `/maintenance/schedules/${id}`, DELETE), `/maintenance/records` (GET `{status?, type?, q?, limit?, offset?}`, POST, GET `/${id}`, PATCH `/${id}`), `/maintenance/attention` (GET), `/assets/${assetId}/maintenance` (GET), `/maintenance/reports` (POST **FormData**: append `asset_id`, `problem_category_id`, `description`, `photo` file when set — pass the `FormData` as `body`, no explicit content-type), `myReports` = `/requests?mine=true&type=maintenance` (mirror `myRequests` in `useAssignment`). Types carry the enriched fields the backend returns (`asset_name`, `asset_tag`, `office_name`, `category_name`, `problem_name`, `vendor_name`, `reported_by_name`).

- [ ] **Step 4: Write the meta unit test**

`frontend/test/unit/maintenance-meta.spec.ts` — assert every tone map entry; `dueDiffDays` for overdue/today/future + null/garbage input; `dueKind` boundaries (-1→overdue, 0→today, 1 & 7→soon, 8→normal, null→normal); `formatRupiah('2350000') === 'Rp 2.350.000'`, `formatRupiah(null) === '—'`, `formatRupiah('0') === '—'`.

- [ ] **Step 5: Run + commit**

Run: `cd frontend && pnpm test -- maintenance-meta && pnpm typecheck`
Expected: PASS; typecheck clean (the removed cast compiles because the union now includes `'assignment'`).

```bash
git add frontend/app/constants/maintenanceMeta.ts frontend/app/constants/approvalMeta.ts frontend/app/composables/api/useMaintenance.ts frontend/test/unit/maintenance-meta.spec.ts frontend/i18n/locales
git commit -m "feat(maintenance): composable + meta constants + RequestType union"
```

---

## Task 9: Nav item

**Files:**
- Modify: `frontend/app/utils/nav.ts`, `frontend/test/unit/nav-model.spec.ts`

- [ ] **Step 1: Add/enable the Maintenance item**

In the Operasional group (both `superadminNav` and `staffNav` — mirror how `assignment`/`peminjaman` are placed): item `{ labelKey: 'nav.maintenance', icon: 'i-lucide-wrench', to: '/maintenance', permission: 'request.create' }` for staff; for the manager/superadmin nav use `permission: 'maintenance.view'`. If a disabled placeholder `nav.maintenance` item exists, enable it instead of adding a duplicate.

- [ ] **Step 2: Update `nav-model.spec.ts`, run, commit**

Add assertions for the item (both navs, `to: '/maintenance'`, correct permission). Run: `cd frontend && pnpm test -- nav-model`. Expected: PASS.

```bash
git add frontend/app/utils/nav.ts frontend/test/unit/nav-model.spec.ts
git commit -m "feat(maintenance): nav item"
```

---

## Task 10: Slideover components (schedule + record)

**Files:**
- Create: `frontend/app/components/maintenance/ScheduleSlideover.vue`, `frontend/app/components/maintenance/RecordSlideover.vue`, `frontend/test/nuxt/maintenance-slideovers.spec.ts`

**Interfaces:**
- Consumes: `useMaintenance()`, `useReferenceData()`-style composables for `maintenance_categories` / `vendors` (grep `composables/api` for the existing reference composable used by masterdata screens), `useAssignment().available()`-style asset picker — use the existing scoped asset search/list composable the transfer screen uses for its asset picker (grep `pages/transfers.vue`).
- Produces:
  - `<MaintenanceScheduleSlideover v-model:open :schedule="s | null" @saved>` — create when `schedule` null, edit otherwise.
  - `<MaintenanceRecordSlideover v-model:open :record="r | null" :prefill="{ asset?: {id,name,asset_tag}, scheduleId?, maintenanceCategoryId?, type? } | null" @saved>` — create (optionally prefilled/locked asset) or edit; terminal records render read-only.

- [ ] **Step 1: Build `ScheduleSlideover.vue`**

`USlideover` (mirror the app's existing slideover usage — grep `USlideover` in `app/`): fields Aset (`USelectMenu`, searchable, from the scoped asset picker; locked in edit mode), Kategori Perawatan (`USelectMenu` from `maintenance_categories`), Interval bulan (`UInput type="number"`, min 1), Tanggal Mulai / Jatuh Tempo Berikut (`UInput type="date"`, required on create), Aktif (`USwitch`, edit only). Footer Batal + Simpan (disabled until asset + interval + date valid; loading state). On submit call `createSchedule`/`updateSchedule`, emit `saved`, toast success, close.

- [ ] **Step 2: Build `RecordSlideover.vue`**

Fields per the mockup slideover: Aset (required; locked when `prefill.asset` or edit), Tipe (`USelect` preventive/corrective), Kategori Perawatan (`USelectMenu`), Tanggal (`UInput type="date"`, required → `scheduled_date`), Status (`USelect` of the 4 statuses; on edit only transitions allowed from the current status are enabled — compute with the same rules as backend `validTransition`), Biaya (`UInput` with `Rp` prefix, numeric), Vendor/Teknisi (`USelectMenu` from `vendors`), Deskripsi (`UTextarea`, required). When status becomes `completed`, show Tanggal Selesai (`completed_date`, default today). Terminal record (`completed`/`cancelled`) → all fields disabled + no save button. Submit → `createRecord`/`updateRecord` (include `schedule_id` from `prefill.scheduleId`), emit `saved`, toast, close.

- [ ] **Step 3: Runtime tests**

`frontend/test/nuxt/maintenance-slideovers.spec.ts` (`// @vitest-environment nuxt`, `mountSuspended`, mock `useMaintenance` + reference composables — mirror `test/nuxt/ajukan-peminjaman-modal.spec.ts` mocking pattern). Assert: create-mode save disabled until required fields set; edit mode locks asset + only-valid status options (completed record → read-only, no save); schedule create submits `{asset_id, interval_months, start_date}`; record create from prefill locks asset and passes `schedule_id`; completed selection reveals tanggal selesai and submits `completed_date`; error from API → toast/error rendered, slideover stays open.

- [ ] **Step 4: Run + commit**

Run: `cd frontend && pnpm test -- maintenance-slideovers`
Expected: PASS.

```bash
git add frontend/app/components/maintenance frontend/test/nuxt/maintenance-slideovers.spec.ts
git commit -m "feat(maintenance): schedule + record slideovers"
```

---

## Task 11: `/maintenance` page (3 tabs + banner + attention)

**Files:**
- Create: `frontend/app/pages/maintenance.vue`, `frontend/test/nuxt/maintenance.spec.ts`

**Interfaces:**
- Consumes: `useMaintenance()`, `useCan()`, slideovers (Task 10), `maintenanceMeta`, `formatDateID`, reference composable for `problem_categories`, `useAssignment().list({ status: 'active' })` (Staf's held assets for the report picker — own scope returns exactly their assignments).

- [ ] **Step 1: Build the page 1:1 with `docs/design/Maintenance.dc.html`**

Open the mockup in a browser first. Structure:
- `definePageMeta({ middleware: 'can', permission: 'request.create' })` (all seeded roles hold it; tab visibility does the real gating).
- Header title/subtitle (`maintenance.title` / `maintenance.subtitle`).
- **Due banner** (above tabs, only when due items exist): from `schedules()` list, compute `dueDiffDays ≤ 3` (incl. overdue), sorted ascending; warning-soft container, per-item card (wrench icon, asset name, task = category name · vendor-less), red badge overdue/today (`maintenance.due.overdue {n}` / `maintenance.due.today`), yellow `maintenance.due.inDays {n}`; "Lihat Jadwal" button switches to the Jadwal tab.
- **"Perlu Tindak Lanjut" section** (approved deviation; only when `attention()` non-empty AND `useCan('maintenance.manage')`): card list (asset name/tag/office) + per-item button opening `RecordSlideover` with `prefill={ asset, type: 'corrective' }`.
- **Tabs** (`UTabs` or the button-tab pattern used by `assignment.vue` — match the mockup's underline style): Jadwal (calendar icon) + Catatan (note icon) visible when `useCan('maintenance.view')`; Laporan Kerusakan (alert icon) visible when `useCan('request.create')`. Default tab: first visible.
- **Tab Jadwal**: schedule cards per mockup (icon tile — red-soft when urgent, warning-soft otherwise; asset name + type badge; `category_name`; right-aligned colored due label + `formatDateID(next_due_date)`; "Buat Catatan" button → `RecordSlideover` prefilled `{asset, scheduleId, maintenanceCategoryId, type: 'preventive'}`). Above the list (deviation): "Tambah Jadwal" button (`maintenance.manage` only) → `ScheduleSlideover`; card click (manage only) opens `ScheduleSlideover` in edit. Loading skeleton, error+retry, empty state.
- **Tab Catatan**: search input (`maintenance.searchPlaceholder`, filters via `q` param, debounced) + "Tambah Catatan" (manage only) → `RecordSlideover`. `UTable`-based 7-column table per mockup: Aset (name + mono tag), Tipe badge (`MAINT_TYPE_TONE`), Kategori, Tanggal (`formatDateID(scheduled_date)`), Status badge with dot (`MAINT_STATUS_TONE`), Biaya right-aligned (`formatRupiah`), Vendor/Teknisi. Row click (manage only) → `RecordSlideover` edit. Empty state per mockup. Pagination if the house table pattern has it.
- **Tab Laporan Kerusakan**: two-column grid. Left: "Tampilan Staf" info badge; success `UAlert` (4 s auto-hide) after submit; form card — Aset yang Anda pegang (`USelectMenu` from the caller's **active assignments**: `useAssignment().list({ status: 'active' })` mapped to `{asset_id, label: asset_name · asset_tag}`; own-scope returns exactly the Staf's holdings), Kategori Masalah (`USelectMenu` from `problem_categories`), Deskripsi (`UTextarea`), Foto opsional (`UInput type="file" accept="image/*"` styled as the dashed drop area), submit button disabled until asset+kategori chosen, info note `maintenance.report.queueNote`. Submit → `submitReport(FormData)`. Right: "Riwayat Laporan Saya" — `myReports()` cards (asset best-effort name via `useAssets().get(id)` with 403 fallback to the id/tag — same pattern as peminjaman), status badge, category chip, date, description; dashed empty state per mockup.
- Every string via `$t('maintenance.*')`; add all keys to `i18n/locales/{id,en}.json` in this task.

- [ ] **Step 2: Runtime tests**

`frontend/test/nuxt/maintenance.spec.ts` (`// @vitest-environment nuxt`; mock `useMaintenance`, `useAssignment`, `useAssets`, reference composables, and `useCan` per case). Cover:
- Banner: shown only when a schedule is due ≤ 3 days (incl. overdue, label "Terlambat N hari"); hidden otherwise; "Lihat Jadwal" switches tab.
- Attention: rendered only with `maintenance.manage` + non-empty; button opens the record slideover prefilled.
- Jadwal: loading/error+retry/empty/populated; due colors per `dueKind`; "Tambah Jadwal" hidden without manage.
- Catatan: table renders enriched fields, biaya formatting, status/type badges; search triggers refetch with `q`; row click opens edit only with manage; empty state.
- Laporan: submit disabled until valid; success alert after submit; FormData contains photo when a file is picked; riwayat renders cards + empty state; 403 asset lookup falls back to tag/id.
- Permission variations: `maintenance.view=false` → only Laporan tab; view-only (`manage=false`) → no Tambah buttons, rows not clickable.

- [ ] **Step 3: Run + full suite exit code**

Run: `cd frontend && pnpm test -- maintenance.spec && pnpm test && pnpm typecheck`
Expected: new tests PASS; **full suite exit code 0** (memory: rewiring composables can break other consumers' tests — grep consumers of anything you changed and stub their API).

- [ ] **Step 4: Commit**

```bash
git add frontend/app/pages/maintenance.vue frontend/test/nuxt/maintenance.spec.ts frontend/i18n/locales
git commit -m "feat(maintenance): /maintenance screen (jadwal, catatan, laporan kerusakan)"
```

---

## Task 12: Integrations — Detail Aset tab, approval payload, stock-opname damaged

**Files:**
- Modify: `frontend/app/pages/assets/[tag]/index.vue`, `frontend/app/pages/approval.vue`, `frontend/app/pages/stock-opname.vue`, `frontend/app/composables/api/useStockOpname.ts`, `frontend/i18n/locales/{id,en}.json`
- Modify (tests): `frontend/test/nuxt/asset-detail.spec.ts` (or the existing detail spec file — grep), `frontend/test/nuxt/stock-opname.spec.ts`, `frontend/test/nuxt/approval.spec.ts`

- [ ] **Step 1: Detail Aset "Riwayat Maintenance" tab**

In `pages/assets/[tag]/index.vue`, add a tab next to the existing Riwayat tabs (mirror the assignments-history tab wiring): fetch `useMaintenance().listByAsset(assetId)`; compact table Tanggal (`scheduled_date`/`completed_date`), Tipe badge, Kategori, Status badge, Biaya (`formatRupiah`), Vendor. Empty state + i18n keys `assetDetail.maintenance.*`. Test: tab renders rows / empty state (extend the existing detail spec, same mocking pattern).

- [ ] **Step 2: Approval screen payload rendering**

In `approval.vue` (or its payload subcomponent — grep how `asset_transfer` payload rows render): for `type === 'maintenance'` render asset (via target/payload `asset_id` best-effort lookup), problem category name (lookup in `problem_categories` reference list by id, fallback id), description, and — when `attachment_id` present — a link/thumbnail to `/assets/{asset_id}/attachments/{attachment_id}/content` (the existing attachment content route; grep the gallery component for the URL builder). Also ensure the type filter/labels include the two new `approval.type.*` keys. Test: a maintenance request renders its payload fields.

- [ ] **Step 3: Stock-opname damaged follow-up**

In `useStockOpname.ts`: followup response type → `{ request_id?: string, record_id?: string, type: string }`. In `stock-opname.vue`: the variance list already includes `damaged` — wire its action button to call `opnameApi.followup(sessionId, item.id, { reason })` directly (no modal; add a small optional-reason `UPopover`/prompt only if the existing not_found path has one — mirror it), toast `stockOpname.followup.maintenanceCreated`, disable the button when `followup_request_id || followup_record_id`. Test: damaged item button calls followup and disables after success; already-followed-up item starts disabled.

- [ ] **Step 4: Run + commit**

Run: `cd frontend && pnpm test && pnpm typecheck && pnpm lint`
Expected: all green.

```bash
git add frontend/app frontend/test frontend/i18n
git commit -m "feat(maintenance): asset-detail tab + approval payload + opname damaged wiring"
```

---

## Task 13: E2E + final gate + PROGRESS

**Files:**
- Create: `frontend/e2e/maintenance.spec.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Write the e2e spec**

Mirror `frontend/e2e/assignment.spec.ts` (login helpers, seeded users, **unique names per run** via timestamp suffix, assert-after-search, wait-modal-closed; fill text fields before opening picker popovers — USelectMenu focus-trap memory). Scenarios:
1. **Manager schedule → record → complete**: create asset (API) + schedule via UI (unique category note) → appears in Jadwal tab with due badge → "Buat Catatan" prefilled → save as `in_progress` → asset detail shows "Dalam Perbaikan"/`under_maintenance` → edit record → `completed` with biaya → Catatan row shows Selesai + biaya; Jadwal card's next-due shifted (+interval); asset back "Tersedia".
2. **Staf damage report → approve → record**: Staf logs in (seed an active assignment via API so the picker has an asset) → Laporan tab → pick asset + kategori masalah + deskripsi → submit → success alert + "Riwayat Laporan Saya" shows Menunggu → approve via API as office-level Manager (maker ≠ checker) → Catatan tab (as Manager) shows the corrective `scheduled` record.
3. **Negative**: submit button stays disabled with empty kategori; completed record row opens read-only (no save button).

- [ ] **Step 2: Run e2e locally (stack up + seeded admin)**

Run:
```bash
docker compose -f docker-compose.dev.yml up -d
cd backend && go run ./cmd/createadmin -email admin@inventra.local -password admin12345
cd ../frontend && pnpm test:e2e -- maintenance
```
Expected: 3/3 PASS. If the shared dev DB hits known seed-drift (e.g. missing grants amended into old migrations), do NOT mutate the dev DB — document, rely on CI's fresh-DB run (precedent: assignment e2e).

- [ ] **Step 3: Full gate sweep**

Run and confirm green, in order:
```bash
cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... -count=1
npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
cd ../frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build
```

- [ ] **Step 4: Side-by-side mockup comparison**

Open the running app `/maintenance` next to `docs/design/Maintenance.dc.html` (light **and** dark): verify layout, spacing, tabs, banner, table columns, slideover fields, empty states, Staf form 1:1 — fix gaps before proceeding. Report the comparison result honestly.

- [ ] **Step 5: Update PROGRESS.md**

- Tick Maintenance in §Remaining "Backend — Feature modules" with a summary line (module, endpoints, executor, attention queue, opname damaged followup, screen, e2e).
- Record the approved deviations (Tambah Jadwal UI, row-click edit slideover, Perlu Tindak Lanjut section, vendor_id select / performed_by unexposed, client-side banner, in-page reminder only) and honest limitations.
- Note the closed follow-up: `RequestType` union now includes `assignment` + `maintenance`.
- Refresh the "▶ Next session — start here" block: remaining candidates now (f) global search backend + drop last mock/* and (g) Reporting & Dashboard.

- [ ] **Step 6: Commit + PR**

```bash
git add docs/PROGRESS.md frontend/e2e/maintenance.spec.ts
git commit -m "feat(maintenance): e2e + progress"
git push -u origin feat/maintenance-module
gh pr create --title "feat(maintenance): Maintenance module (jadwal, catatan, laporan kerusakan, tindak lanjut)" --body "<summary per repo convention — no AI attribution>"
```

---

## Self-Review Notes (already applied)

- Spec §1.3 "duplicate pending" guard → `CountPendingMaintRequests` (Task 2) + `ErrDuplicatePending` (Task 3) + integration test 8.
- Spec §1.6 idempotency → double guard on both link columns (Task 5) + integration test 11.
- Spec decision #3 (no auto-create) → no assignment changes anywhere; attention queue is Task 2 (query) + Task 4 (endpoint) + Task 11 (UI).
- `completed_date` defaulting lives in the service (before insert/update), keeping `applyStatusEffects` read-only w.r.t. the record row.
- Frontend picker for the Staf report uses active assignments (own scope) — consistent with spec §2.1 and the peminjaman precedent.
