# Approval Engine + Asset Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the value-tiered maker-checker approval engine integrated with an Asset core module so that submitting a request routes through a configurable multi-step chain and final approval executes the real side effect (asset created / disposed / flagged).

**Architecture:** Two new four-file modules — `internal/approval/` (generic engine + executor registry) and `internal/asset/` (read/update + tag generator + state machine + executors). Asset write operations (`asset_create`, `asset_disposal`, `valuation_exclusion`) flow only through `POST /requests`; asset read/update are direct. The approval engine runs each final-step executor inside the same DB transaction as the approval commit (atomic).

**Tech Stack:** Go 1.25, Gin, pgx/v5, sqlc, PostgreSQL 16, Redis 7, testify + testcontainers-go.

## Global Constraints

- Go module path `github.com/ragbuaj/inventra`; backend commands run from `backend/`.
- Money/numeric columns are Go `string` (sqlc override) — never float.
- Soft-delete everywhere; all reads filter `deleted_at IS NULL`.
- Never hand-edit `backend/db/sqlc/` — edit `db/queries/*.sql` or migrations, then `sqlc generate`.
- Enforce authorization on **read and write**: `RequirePermission` for the action + office data-scope threaded into queries.
- List endpoints return `{data, total, limit, offset}` with `limit` clamped 1–100 (`common.ClampInt`).
- Field-permission filtering uses the **map form** (`assetToMap`) so `authz.FilterView` can drop fields.
- Default `go test ./...` stays unit-only; integration suites use `//go:build integration` + `internal/testsupport`.
- Conventional Commits with scope: `feat(approval):`, `feat(asset):`, `feat(db):`, `feat(authz):`.
- Keep `backend/api/openapi.yaml` in sync (Spectral-linted). No Claude/AI co-author trailers in commits.
- Reference spec: `docs/superpowers/specs/2026-06-28-approval-engine-asset-core-design.md`.

---

## File Structure

**Migrations / queries / seed**
- `backend/db/migrations/000016_office_tier.up.sql` / `.down.sql` — add `office_types.tier` + all feature seeds.
- `backend/db/queries/assets.sql` — asset CRUD + tag counter.
- `backend/db/queries/approval.sql` — requests, approvals, thresholds.
- `backend/db/queries/offices.sql` (extend) — `GetOfficeAncestors`.

**Asset module** (`backend/internal/asset/`)
- `service.go` — list/get/update, `GenerateAssetTag`, state machine, executors, sentinels.
- `dto.go` — request DTOs + `assetToMap`.
- `handler.go` — read/update handlers + field masking.
- `routes.go` — route registration.
- `*_test.go` (unit) + `integration_test.go` (`//go:build integration`).

**Approval module** (`backend/internal/approval/`)
- `service.go` — Submit/Decide/Cancel/Inbox, chain build, tier resolve, eligibility, registry, sentinels.
- `dto.go` — submit/decide/threshold DTOs + serializers.
- `handler.go` — handlers + `svcError`.
- `routes.go` — route registration.
- `executor.go` — `Executor` interface + registry type.
- `*_test.go` + `integration_test.go`.

**Wiring**
- `backend/internal/server/router.go` (modify) — construct + register both modules.
- `backend/api/openapi.yaml` (modify) — new paths/schemas.
- `docs/PROGRESS.md` (modify) — tick items.

---

## Phase A — Migration + seeds

### Task 1: Migration `000016_office_tier` + feature seeds

**Files:**
- Create: `backend/db/migrations/000016_office_tier.up.sql`
- Create: `backend/db/migrations/000016_office_tier.down.sql`
- Test: `backend/internal/testsupport/migrate_test.go` (extend existing migration-apply assertion if present; otherwise verify via Task 20 integration)

**Interfaces:**
- Produces: `masterdata.office_types.tier shared.approver_level`; seeded `approval.approval_thresholds`, `identity.role_permissions` (keys `request.create`, `request.decide`, `approval.config.manage`, `asset.view`, `asset.manage`), `identity.data_scope_policies` (modules `assets`, `requests`), `identity.field_permissions` (entity `assets`).

- [ ] **Step 1: Inspect existing seed conventions**

Read the seed blocks in `backend/db/migrations/000001_*.up.sql` … `000010_approval.up.sql` to copy the exact column names of `role_permissions`, `data_scope_policies`, `field_permissions`, and the role rows (their `id`/`name`). Confirm role names (Superadmin / Manager / Kepala Kanwil / Kepala Unit / Staf) and how permissions are linked (by `role_id` + `permission_key` text, per CLAUDE.md).

- [ ] **Step 2: Write `000016_office_tier.up.sql`**

```sql
-- Add explicit office tier (reuses shared.approver_level; cabang & outlet => 'office')
ALTER TABLE masterdata.office_types ADD COLUMN tier shared.approver_level;

-- Backfill tier for seeded office types (idempotent by name)
UPDATE masterdata.office_types SET tier = 'pusat'   WHERE name ILIKE '%pusat%'   AND deleted_at IS NULL;
UPDATE masterdata.office_types SET tier = 'wilayah' WHERE name ILIKE '%wilayah%' AND deleted_at IS NULL;
UPDATE masterdata.office_types SET tier = 'office'  WHERE (name ILIKE '%cabang%' OR name ILIKE '%unit%' OR name ILIKE '%outlet%') AND deleted_at IS NULL;

-- Approval thresholds (placeholder bands per PRD bagian 2.4 — confirm with bank policy)
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order) VALUES
  ('asset_create', 0,          10000000,  'office',  1),
  ('asset_create', 10000000,   100000000, 'office',  1),
  ('asset_create', 10000000,   100000000, 'wilayah', 2),
  ('asset_create', 100000000,  NULL,      'office',  1),
  ('asset_create', 100000000,  NULL,      'wilayah', 2),
  ('asset_create', 100000000,  NULL,      'pusat',   3),
  ('asset_disposal', 0,        5000000,   'office',  1),
  ('asset_disposal', 5000000,  50000000,  'office',  1),
  ('asset_disposal', 5000000,  50000000,  'wilayah', 2),
  ('asset_disposal', 50000000, NULL,      'office',  1),
  ('asset_disposal', 50000000, NULL,      'wilayah', 2),
  ('asset_disposal', 50000000, NULL,      'pusat',   3),
  ('valuation_exclusion', 0,   NULL,      'wilayah', 1);

-- Permissions (insert keys then grant to roles). Use the project's role_permissions shape.
-- Adjust column names to match earlier migrations discovered in Step 1.
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES
  ('request.create'), ('request.decide'), ('approval.config.manage'),
  ('asset.view'), ('asset.manage')
) AS p(key)
WHERE
  (r.name = 'Superadmin')
  OR (r.name IN ('Manager','Kepala Kanwil','Kepala Unit') AND p.key IN ('request.decide','asset.view','asset.manage'))
  OR (r.name = 'Staf' AND p.key IN ('request.create','asset.view'))
ON CONFLICT DO NOTHING;

-- Data-scope defaults for new modules (assets, requests)
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, m.module, (CASE
    WHEN r.name = 'Superadmin' THEN 'global'
    WHEN r.name IN ('Kepala Kanwil','Kepala Unit','Manager') THEN 'office_subtree'
    WHEN r.name = 'Staf' AND m.module = 'requests' THEN 'own'
    ELSE 'office' END)::shared.scope_level
FROM identity.roles r
CROSS JOIN (VALUES ('assets'), ('requests')) AS m(module)
ON CONFLICT DO NOTHING;

-- Field permissions: mask cost/value on assets
INSERT INTO identity.field_permissions (entity, field, role_id, can_view, can_edit)
SELECT 'assets', f.field, r.id,
       (CASE
          WHEN f.field = 'purchase_cost'             AND r.name IN ('Superadmin','Manager') THEN true
          WHEN f.field = 'book_value'                AND r.name IN ('Superadmin','Manager') THEN true
          WHEN f.field = 'accumulated_depreciation'  AND r.name = 'Superadmin'              THEN true
          ELSE false END),
       false
FROM identity.roles r
CROSS JOIN (VALUES ('purchase_cost'), ('book_value'), ('accumulated_depreciation')) AS f(field)
ON CONFLICT DO NOTHING;
```

> Note: the exact table/column names for `role_permissions`, `data_scope_policies`, `field_permissions` MUST be reconciled with Step 1 findings before running. If `permission_key` is modeled via a `permissions` table + join, adapt the INSERTs accordingly (keep the same role→key grant matrix).

- [ ] **Step 3: Write `000016_office_tier.down.sql`**

```sql
DELETE FROM identity.field_permissions WHERE entity = 'assets';
DELETE FROM identity.data_scope_policies WHERE module IN ('assets','requests');
DELETE FROM identity.role_permissions WHERE permission_key IN
  ('request.create','request.decide','approval.config.manage','asset.view','asset.manage');
DELETE FROM approval.approval_thresholds
  WHERE request_type IN ('asset_create','asset_disposal','valuation_exclusion');
ALTER TABLE masterdata.office_types DROP COLUMN tier;
```

- [ ] **Step 4: Apply migration against the dev DB and verify**

Run:
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: applies clean to version 16. Verify with `psql`: `\d masterdata.office_types` shows `tier`; `SELECT count(*) FROM approval.approval_thresholds;` ≥ 13.

- [ ] **Step 5: Verify down migration reverses cleanly**

Run: `migrate -path db/migrations -database "$DATABASE_URL" down 1` then `up` again. Expected: no error; column drops then re-adds.

- [ ] **Step 6: Commit**

```bash
git add backend/db/migrations/000016_office_tier.up.sql backend/db/migrations/000016_office_tier.down.sql
git commit -m "feat(db): office_types.tier + approval/asset seeds (000016)"
```

---

## Phase B — Queries + sqlc

### Task 2: Asset queries

**Files:**
- Create: `backend/db/queries/assets.sql`
- Modify (generated): `backend/db/sqlc/*` via `sqlc generate`

**Interfaces:**
- Produces sqlc methods: `ListAssets`, `CountAssets`, `GetAsset`, `CreateAsset`, `UpdateAsset`, `SetAssetStatus`, `SetAssetValuationExclusion`, `BumpAssetTagCounter`, `GetOfficeCode`, `GetCategoryCode`.

- [ ] **Step 1: Write `db/queries/assets.sql`**

```sql
-- name: ListAssets :many
SELECT * FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(search)::text IS NULL OR name ILIKE '%' || sqlc.narg(search) || '%'
       OR asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR serial_number ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(asset_class)::shared.asset_class IS NULL OR asset_class = sqlc.narg(asset_class))
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAssets :one
SELECT count(*) FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(search)::text IS NULL OR name ILIKE '%' || sqlc.narg(search) || '%'
       OR asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR serial_number ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(asset_class)::shared.asset_class IS NULL OR asset_class = sqlc.narg(asset_class));

-- name: GetAsset :one
SELECT * FROM asset.assets WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateAsset :one
INSERT INTO asset.assets (
  asset_tag, name, category_id, brand_id, model_id, room_id, office_id, unit_id,
  status, serial_number, purchase_date, purchase_cost, vendor_id, po_number,
  funding_source, warranty_expiry, specifications, asset_class, capitalized,
  acquisition_bast_no, created_by_id, notes
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,'available',$9,$10,$11,$12,$13,$14,$15,
  COALESCE($16,'{}')::jsonb,$17,$18,$19,$20,$21
) RETURNING *;

-- name: UpdateAsset :one
UPDATE asset.assets SET
  name = $2, category_id = $3, brand_id = $4, model_id = $5, room_id = $6,
  unit_id = $7, serial_number = $8, purchase_date = $9, vendor_id = $10,
  po_number = $11, funding_source = $12, warranty_expiry = $13,
  specifications = COALESCE($14,'{}')::jsonb, notes = $15
WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- name: SetAssetStatus :one
UPDATE asset.assets SET status = $2 WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- name: SetAssetValuationExclusion :one
UPDATE asset.assets SET excluded_from_valuation = $2, valuation_exclusion_reason = $3
WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- name: BumpAssetTagCounter :one
INSERT INTO asset.asset_tag_counters (office_id, category_id, year, last_seq)
VALUES ($1, $2, $3, 1)
ON CONFLICT (office_id, category_id, year)
DO UPDATE SET last_seq = asset.asset_tag_counters.last_seq + 1
RETURNING last_seq;

-- name: GetOfficeCode :one
SELECT code FROM masterdata.offices WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCategoryCode :one
SELECT code FROM masterdata.categories WHERE id = $1 AND deleted_at IS NULL;
```

- [ ] **Step 2: Generate + build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: no errors; `db/sqlc/assets.sql.go` now defines the methods above.

- [ ] **Step 3: Commit**

```bash
git add backend/db/queries/assets.sql backend/db/sqlc/
git commit -m "feat(db): asset core queries + tag counter"
```

### Task 3: Approval queries

**Files:**
- Create: `backend/db/queries/approval.sql`
- Modify (generated): `backend/db/sqlc/*`

**Interfaces:**
- Produces: `MatchThresholdSteps`, `ListThresholds`, `CreateThreshold`, `UpdateThreshold`, `SoftDeleteThreshold`, `CreateRequest`, `GetRequest`, `ListRequests`, `CountRequests`, `SetRequestDecision`, `AdvanceRequestStep`, `CancelRequest`, `CreateRequestApproval`, `ListRequestApprovals`, `DecideRequestApproval`, `ListInboxCandidates`.

- [ ] **Step 1: Write `db/queries/approval.sql`**

```sql
-- name: MatchThresholdSteps :many
SELECT * FROM approval.approval_thresholds
WHERE request_type = $1 AND is_active AND deleted_at IS NULL
  AND amount_from <= sqlc.arg(amount)
  AND (amount_to IS NULL OR sqlc.arg(amount) < amount_to)
ORDER BY step_order;

-- name: ListThresholds :many
SELECT * FROM approval.approval_thresholds WHERE deleted_at IS NULL
ORDER BY request_type, amount_from, step_order;

-- name: CreateThreshold :one
INSERT INTO approval.approval_thresholds
  (request_type, amount_from, amount_to, required_level, step_order, is_active)
VALUES ($1,$2,$3,$4,$5,COALESCE($6,true)) RETURNING *;

-- name: UpdateThreshold :one
UPDATE approval.approval_thresholds SET
  amount_from=$2, amount_to=$3, required_level=$4, step_order=$5, is_active=$6
WHERE id=$1 AND deleted_at IS NULL RETURNING *;

-- name: SoftDeleteThreshold :execrows
UPDATE approval.approval_thresholds SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL;

-- name: CreateRequest :one
INSERT INTO approval.requests
  (type, office_id, amount, current_step, target_entity, target_id, payload, reason, requested_by_id)
VALUES ($1,$2,$3,1,$4,$5,COALESCE($6,'{}')::jsonb,$7,$8) RETURNING *;

-- name: GetRequest :one
SELECT * FROM approval.requests WHERE id=$1 AND deleted_at IS NULL;

-- name: ListRequests :many
SELECT * FROM approval.requests
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.request_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(type)::shared.request_type IS NULL OR type = sqlc.narg(type))
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountRequests :one
SELECT count(*) FROM approval.requests
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.request_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(type)::shared.request_type IS NULL OR type = sqlc.narg(type));

-- name: SetRequestDecision :one
UPDATE approval.requests SET status=$2, decided_by_id=$3, decision_note=$4, decided_at=now()
WHERE id=$1 AND deleted_at IS NULL RETURNING *;

-- name: AdvanceRequestStep :one
UPDATE approval.requests SET current_step=current_step+1
WHERE id=$1 AND deleted_at IS NULL RETURNING *;

-- name: CancelRequest :one
UPDATE approval.requests SET status='cancelled'
WHERE id=$1 AND requested_by_id=$2 AND status='pending' AND deleted_at IS NULL RETURNING *;

-- name: CreateRequestApproval :one
INSERT INTO approval.request_approvals (request_id, step_order, required_level)
VALUES ($1,$2,$3) RETURNING *;

-- name: ListRequestApprovals :many
SELECT * FROM approval.request_approvals
WHERE request_id=$1 AND deleted_at IS NULL ORDER BY step_order;

-- name: DecideRequestApproval :one
UPDATE approval.request_approvals SET approver_id=$3, decision=$4, note=$5, decided_at=now()
WHERE request_id=$1 AND step_order=$2 AND deleted_at IS NULL RETURNING *;

-- name: ListInboxCandidates :many
SELECT * FROM approval.requests
WHERE deleted_at IS NULL AND status='pending'
ORDER BY created_at ASC;
```

- [ ] **Step 2: Generate + build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/db/queries/approval.sql backend/db/sqlc/
git commit -m "feat(db): approval engine queries (requests, approvals, thresholds)"
```

### Task 4: Office ancestors query

**Files:**
- Modify: `backend/db/queries/offices.sql`
- Modify (generated): `backend/db/sqlc/*`

**Interfaces:**
- Produces: `GetOfficeAncestors(officeID) -> []{ID uuid, ParentID *uuid, Tier *SharedApproverLevel}` (each ancestor incl. the office itself, with its office_type tier).

- [ ] **Step 1: Append to `db/queries/offices.sql`**

```sql
-- name: GetOfficeAncestors :many
WITH RECURSIVE anc AS (
  SELECT o.id, o.parent_id, o.office_type_id
  FROM masterdata.offices o WHERE o.id = $1 AND o.deleted_at IS NULL
  UNION ALL
  SELECT o.id, o.parent_id, o.office_type_id
  FROM masterdata.offices o
  JOIN anc a ON o.id = a.parent_id
  WHERE o.deleted_at IS NULL
)
SELECT anc.id, anc.parent_id, ot.tier
FROM anc JOIN masterdata.office_types ot ON ot.id = anc.office_type_id;
```

- [ ] **Step 2: Generate + build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: `GetOfficeAncestors` method present; `GetOfficeAncestorsRow` has `ID`, `ParentID`, `Tier *SharedApproverLevel`.

- [ ] **Step 3: Commit**

```bash
git add backend/db/queries/offices.sql backend/db/sqlc/
git commit -m "feat(db): GetOfficeAncestors recursive query for tier resolution"
```

---

## Phase C — Asset core service

### Task 5: Asset tag generator (atomic counter)

**Files:**
- Create: `backend/internal/asset/service.go`
- Test: `backend/internal/asset/tag_test.go` (unit, format) + covered atomically by integration (Task 20)

**Interfaces:**
- Produces: `func formatAssetTag(officeCode, categoryCode string, year int, seq int64) string`; `func (s *Service) GenerateAssetTag(ctx context.Context, qtx *sqlc.Queries, officeID, categoryID uuid.UUID, year int32) (string, error)` (takes the tx-bound `*sqlc.Queries` so the counter bump shares the caller's transaction). Consumes `BumpAssetTagCounter`, `GetOfficeCode`, `GetCategoryCode` (Task 2).

- [ ] **Step 1: Write the failing test**

```go
package asset

import "testing"

func TestFormatAssetTag(t *testing.T) {
	got := formatAssetTag("JKT01", "ELK", 2026, 1)
	if got != "JKT01-ELK-2026-00001" {
		t.Fatalf("got %q", got)
	}
	if g := formatAssetTag("BDG02", "KEN", 2026, 12345); g != "BDG02-KEN-2026-12345" {
		t.Fatalf("got %q", g)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run TestFormatAssetTag`
Expected: FAIL (`undefined: formatAssetTag`).

- [ ] **Step 3: Implement `service.go` skeleton + `formatAssetTag` + `GenerateAssetTag`**

```go
package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

var (
	ErrNotFound     = errors.New("asset not found")
	ErrInvalidState = errors.New("invalid status transition")
	ErrConflict     = errors.New("conflict")
	ErrInvalidRef   = errors.New("invalid reference")
	ErrRoomRequired = errors.New("tangible asset requires a room")
)

type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool) *Service { return &Service{q: q, pool: pool} }

func formatAssetTag(officeCode, categoryCode string, year int, seq int64) string {
	return fmt.Sprintf("%s-%s-%d-%05d", officeCode, categoryCode, year, seq)
}

// GenerateAssetTag bumps the per (office, category, year) counter inside tx and formats the tag.
func (s *Service) GenerateAssetTag(ctx context.Context, qtx *sqlc.Queries, officeID, categoryID uuid.UUID, year int32) (string, error) {
	officeCode, err := qtx.GetOfficeCode(ctx, officeID)
	if err != nil {
		return "", ErrInvalidRef
	}
	categoryCode, err := qtx.GetCategoryCode(ctx, categoryID)
	if err != nil || categoryCode == nil {
		return "", ErrInvalidRef
	}
	seq, err := qtx.BumpAssetTagCounter(ctx, sqlc.BumpAssetTagCounterParams{
		OfficeID: officeID, CategoryID: categoryID, Year: year,
	})
	if err != nil {
		return "", err
	}
	return formatAssetTag(officeCode, *categoryCode, int(year), int64(seq)), nil
}
```

> Adjust `GetCategoryCode` nil-handling to the generated return type (category `code` is nullable text → `*string`; office `code` is NOT NULL → `string`). Verify against the generated signatures. `mapDBError` (referenced by later tasks) is added in Task 7 along with the `pgx`/`pgconn` imports it needs.

- [ ] **Step 4: Run to verify pass + build**

Run: `cd backend && go test ./internal/asset/ -run TestFormatAssetTag && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/asset/service.go backend/internal/asset/tag_test.go
git commit -m "feat(asset): asset_tag generator (atomic per office/category/year)"
```

### Task 6: Status state machine

**Files:**
- Modify: `backend/internal/asset/service.go`
- Test: `backend/internal/asset/state_test.go`

**Interfaces:**
- Produces: `func validTransition(from, to sqlc.SharedAssetStatus) bool`.

- [ ] **Step 1: Write the failing test**

```go
package asset

import (
	"testing"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestValidTransition(t *testing.T) {
	ok := [][2]sqlc.SharedAssetStatus{
		{"available", "assigned"}, {"assigned", "available"},
		{"available", "under_maintenance"}, {"under_maintenance", "available"},
		{"available", "lost"}, {"assigned", "lost"}, {"available", "disposed"},
	}
	for _, p := range ok {
		if !validTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s allowed", p[0], p[1])
		}
	}
	bad := [][2]sqlc.SharedAssetStatus{
		{"disposed", "available"}, {"available", "in_transfer"},
		{"available", "retired"}, {"lost", "available"},
	}
	for _, p := range bad {
		if validTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s rejected", p[0], p[1])
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run TestValidTransition`
Expected: FAIL (`undefined: validTransition`).

- [ ] **Step 3: Implement `validTransition`**

```go
// in service.go
var allowedTransitions = map[sqlc.SharedAssetStatus]map[sqlc.SharedAssetStatus]bool{
	"available":         {"assigned": true, "under_maintenance": true, "lost": true, "disposed": true},
	"assigned":          {"available": true, "lost": true, "disposed": true},
	"under_maintenance":  {"available": true, "disposed": true},
}

func validTransition(from, to sqlc.SharedAssetStatus) bool {
	return allowedTransitions[from][to]
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd backend && go test ./internal/asset/ -run TestValidTransition`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/asset/service.go backend/internal/asset/state_test.go
git commit -m "feat(asset): status state machine (module-owned transitions)"
```

### Task 7: Asset read service (List/Get + scope)

**Files:**
- Modify: `backend/internal/asset/service.go`
- Test: covered by integration (Task 20); add a thin unit test for `mapDBError`.

**Interfaces:**
- Produces: `ListInput{Search *string; CategoryID,OfficeFilter *uuid.UUID; Status *sqlc.SharedAssetStatus; AssetClass *sqlc.SharedAssetClass; Limit,Offset int32; AllScope bool; OfficeIDs []uuid.UUID}`; `func (s *Service) List(ctx, in ListInput) ([]sqlc.AssetAsset, int64, error)`; `func (s *Service) Get(ctx, id uuid.UUID) (sqlc.AssetAsset, error)`; `func mapDBError(err error) error`.

- [ ] **Step 1: Implement List/Get + mapDBError**

```go
// service.go
func mapDBError(err error) error {
	if err == nil { return nil }
	if errors.Is(err, pgx.ErrNoRows) { return ErrNotFound }
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": return ErrConflict
		case "23503": return ErrInvalidRef
		case "23514": return ErrRoomRequired // CHECK violation (room_id)
		}
	}
	return err
}

type ListInput struct {
	Search       *string
	CategoryID   *uuid.UUID
	OfficeFilter *uuid.UUID
	Status       *sqlc.SharedAssetStatus
	AssetClass   *sqlc.SharedAssetClass
	Limit, Offset int32
	AllScope     bool
	OfficeIDs    []uuid.UUID
}

func (s *Service) List(ctx context.Context, in ListInput) ([]sqlc.AssetAsset, int64, error) {
	rows, err := s.q.ListAssets(ctx, sqlc.ListAssetsParams{
		AllScope: in.AllScope, OfficeIds: in.OfficeIDs, Search: in.Search,
		CategoryID: in.CategoryID, OfficeFilter: in.OfficeFilter, Status: in.Status,
		AssetClass: in.AssetClass, Lim: in.Limit, Off: in.Offset,
	})
	if err != nil { return nil, 0, mapDBError(err) }
	total, err := s.q.CountAssets(ctx, sqlc.CountAssetsParams{
		AllScope: in.AllScope, OfficeIds: in.OfficeIDs, Search: in.Search,
		CategoryID: in.CategoryID, OfficeFilter: in.OfficeFilter, Status: in.Status,
		AssetClass: in.AssetClass,
	})
	if err != nil { return nil, 0, mapDBError(err) }
	return rows, total, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (sqlc.AssetAsset, error) {
	a, err := s.q.GetAsset(ctx, id)
	return a, mapDBError(err)
}
```

> Add the `github.com/jackc/pgx/v5/pgconn` import. Reconcile every sqlc param struct field name with the generated code (e.g. `Lim`/`Off`, `OfficeIds`) — fix names to match.

- [ ] **Step 2: Build**

Run: `cd backend && go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/asset/service.go
git commit -m "feat(asset): scoped list/get read service"
```

### Task 8: Asset update service

**Files:**
- Modify: `backend/internal/asset/service.go`

**Interfaces:**
- Produces: `UpdateInput{...}`; `func (s *Service) Update(ctx, id uuid.UUID, in UpdateInput) (before, after sqlc.AssetAsset, err error)`.

- [ ] **Step 1: Implement Update (fetch-before for audit diff, then update)**

```go
type UpdateInput struct {
	Name          string
	CategoryID    uuid.UUID
	BrandID       *uuid.UUID
	ModelID       *uuid.UUID
	RoomID        *uuid.UUID
	UnitID        *uuid.UUID
	SerialNumber  *string
	PurchaseDate  pgtype.Date
	VendorID      *uuid.UUID
	PONumber      *string
	FundingSource *string
	WarrantyExpiry pgtype.Date
	Specifications []byte
	Notes         *string
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (sqlc.AssetAsset, sqlc.AssetAsset, error) {
	before, err := s.q.GetAsset(ctx, id)
	if err != nil { return before, before, mapDBError(err) }
	after, err := s.q.UpdateAsset(ctx, sqlc.UpdateAssetParams{
		ID: id, Name: in.Name, CategoryID: in.CategoryID, BrandID: in.BrandID,
		ModelID: in.ModelID, RoomID: in.RoomID, UnitID: in.UnitID,
		SerialNumber: in.SerialNumber, PurchaseDate: in.PurchaseDate, VendorID: in.VendorID,
		PoNumber: in.PONumber, FundingSource: in.FundingSource, WarrantyExpiry: in.WarrantyExpiry,
		Specifications: in.Specifications, Notes: in.Notes,
	})
	return before, after, mapDBError(err)
}
```

> Match field names/types to the generated `UpdateAssetParams` (date columns are `pgtype.Date`; nullable text are `*string`).

- [ ] **Step 2: Build + commit**

Run: `cd backend && go build ./...`
```bash
git add backend/internal/asset/service.go
git commit -m "feat(asset): direct update service with before/after for audit"
```

### Task 9: Asset DTO (map form) + field masking + read/update handlers + routes

**Files:**
- Create: `backend/internal/asset/dto.go`, `backend/internal/asset/handler.go`, `backend/internal/asset/routes.go`
- Test: `backend/internal/asset/dto_test.go`

**Interfaces:**
- Produces: `func assetToMap(a sqlc.AssetAsset) map[string]any`; `type Handler`; `func NewHandler(svc *Service, fieldSvc *authz.FieldService, scoped common.ScopedDeps, aud *audit.Service) *Handler`; `func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireView, requireManage gin.HandlerFunc)`.
- Consumes: `authz.FieldService.FilterView`, `common.ScopedDeps.CallerOfficeScope(c, "assets")`, `common.ClampInt`, `common.WriteError`.

- [ ] **Step 1: Write the failing DTO test**

```go
package asset

import (
	"testing"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/google/uuid"
)

func TestAssetToMap_IncludesSensitiveKeys(t *testing.T) {
	cost := "1500000.00"
	m := assetToMap(sqlc.AssetAsset{ID: uuid.New(), Name: "Laptop", AssetTag: "JKT01-ELK-2026-00001", PurchaseCost: &cost})
	if m["name"] != "Laptop" { t.Fatalf("name missing") }
	if m["asset_tag"] != "JKT01-ELK-2026-00001" { t.Fatalf("tag missing") }
	if _, ok := m["purchase_cost"]; !ok { t.Fatalf("purchase_cost must be present pre-mask") }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run TestAssetToMap`
Expected: FAIL (`undefined: assetToMap`).

- [ ] **Step 3: Implement `dto.go`**

```go
package asset

import (
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

func assetToMap(a sqlc.AssetAsset) map[string]any {
	return map[string]any{
		"id": a.ID.String(), "asset_tag": a.AssetTag, "name": a.Name,
		"category_id": a.CategoryID.String(), "office_id": a.OfficeID.String(),
		"room_id": common.UUIDPtrStr(a.RoomID), "status": string(a.Status),
		"asset_class": string(a.AssetClass), "serial_number": a.SerialNumber,
		"purchase_cost": a.PurchaseCost, "book_value": a.BookValue,
		"accumulated_depreciation": a.AccumulatedDepreciation,
		"excluded_from_valuation": a.ExcludedFromValuation,
		"created_at": common.TsStr(a.CreatedAt), "updated_at": common.TsStr(a.UpdatedAt),
	}
}

// AssetUpdateRequest is the PUT body (non-sensitive attributes only).
type AssetUpdateRequest struct {
	Name          string  `json:"name" binding:"required"`
	CategoryID    string  `json:"category_id" binding:"required,uuid"`
	BrandID       *string `json:"brand_id" binding:"omitempty,uuid"`
	ModelID       *string `json:"model_id" binding:"omitempty,uuid"`
	RoomID        *string `json:"room_id" binding:"omitempty,uuid"`
	UnitID        *string `json:"unit_id" binding:"omitempty,uuid"`
	SerialNumber  *string `json:"serial_number"`
	Notes         *string `json:"notes"`
}
```

> Extend `assetToMap`/`AssetUpdateRequest` with the remaining columns as needed; keep sensitive keys present so `FilterView` can drop them.

- [ ] **Step 4: Implement `handler.go` (read + update with masking)**

```go
package asset

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

type Handler struct {
	svc      *Service
	fieldSvc *authz.FieldService
	scoped   common.ScopedDeps
	aud      *audit.Service
}

func NewHandler(svc *Service, fieldSvc *authz.FieldService, scoped common.ScopedDeps, aud *audit.Service) *Handler {
	return &Handler{svc: svc, fieldSvc: fieldSvc, scoped: scoped, aud: aud}
}

func (h *Handler) list(c *gin.Context) {
	all, ids, err := h.scoped.CallerOfficeScope(c, "assets")
	if err != nil { common.WriteError(c, err); return }
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<30)
	in := ListInput{AllScope: all, OfficeIDs: ids, Limit: limit, Offset: offset}
	if s := c.Query("search"); s != "" { in.Search = &s }
	// parse category_id/office_id/status/asset_class query params similarly...
	rows, total, err := h.svc.List(c, in)
	if err != nil { common.WriteError(c, err); return }
	roleID := c.GetString(string(authz.CtxRoleID)) // adjust to actual ctx accessor
	data := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		data = append(data, h.fieldSvc.FilterView(c, roleID, "assets", assetToMap(a)))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
	a, err := h.svc.Get(c, id)
	if err != nil { common.WriteError(c, err); return }
	all, ids, err := h.scoped.CallerOfficeScope(c, "assets")
	if err != nil { common.WriteError(c, err); return }
	if !common.InScope(all, ids, a.OfficeID) { common.WriteError(c, common.ErrForbidden); return }
	roleID := c.GetString(string(authz.CtxRoleID))
	c.JSON(http.StatusOK, h.fieldSvc.FilterView(c, roleID, "assets", assetToMap(a)))
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
	var req AssetUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	cur, err := h.svc.Get(c, id)
	if err != nil { common.WriteError(c, err); return }
	all, ids, err := h.scoped.CallerOfficeScope(c, "assets")
	if err != nil { common.WriteError(c, err); return }
	if !common.InScope(all, ids, cur.OfficeID) { common.WriteError(c, common.ErrForbidden); return }
	in, err := req.toInput()
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	before, after, err := h.svc.Update(c, id, in)
	if err != nil { common.WriteError(c, err); return }
	audit.Record(c, h.aud, audit.ActionUpdate, "assets", after.ID, &after.OfficeID,
		audit.Diff(assetToMap(before), assetToMap(after)))
	roleID := c.GetString(string(authz.CtxRoleID))
	c.JSON(http.StatusOK, h.fieldSvc.FilterView(c, roleID, "assets", assetToMap(after)))
}
```

> Reconcile the role-id context accessor and `audit.Record`/`audit.Diff`/`audit.ActionUpdate` signatures with `internal/audit` and `internal/masterdata/category/handler.go`. Implement `AssetUpdateRequest.toInput()` (UUID parsing via `common.ParseUUIDPtr`).

- [ ] **Step 5: Implement `routes.go`**

```go
package asset

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireView, requireManage gin.HandlerFunc) {
	g := rg.Group("/assets")
	g.GET("", authMW, requireView, h.list)
	g.GET("/:id", authMW, requireView, h.get)
	g.PUT("/:id", authMW, requireManage, h.update)
}
```

- [ ] **Step 6: Run tests + build**

Run: `cd backend && go test ./internal/asset/ && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/asset/dto.go backend/internal/asset/handler.go backend/internal/asset/routes.go backend/internal/asset/dto_test.go
git commit -m "feat(asset): read/update handlers with field-permission masking"
```

---

## Phase D — Approval engine

### Task 10: Executor interface + registry

**Files:**
- Create: `backend/internal/approval/executor.go`
- Test: `backend/internal/approval/executor_test.go`

**Interfaces:**
- Produces: `type Executor interface { Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error }`; `type registry map[sqlc.SharedRequestType]Executor`.

- [ ] **Step 1: Write the failing test**

```go
package approval

import (
	"context"
	"testing"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

type stubExec struct{ called bool }
func (s *stubExec) Execute(context.Context, *sqlc.Queries, sqlc.ApprovalRequest) error { s.called = true; return nil }

func TestRegistryLookup(t *testing.T) {
	r := registry{}
	e := &stubExec{}
	r[sqlc.SharedRequestTypeAssetCreate] = e
	if r[sqlc.SharedRequestTypeAssetCreate] == nil { t.Fatal("expected executor registered") }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/approval/ -run TestRegistryLookup`
Expected: FAIL (package/types undefined).

- [ ] **Step 3: Implement `executor.go`**

```go
package approval

import (
	"context"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// Executor performs the real side effect of an approved request, inside the approval-commit tx.
type Executor interface {
	Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error
}

type registry map[sqlc.SharedRequestType]Executor
```

- [ ] **Step 4: Run pass + commit**

Run: `cd backend && go test ./internal/approval/ -run TestRegistryLookup`
```bash
git add backend/internal/approval/executor.go backend/internal/approval/executor_test.go
git commit -m "feat(approval): executor interface + registry"
```

### Task 11: Tier resolution

**Files:**
- Create: `backend/internal/approval/service.go`
- Test: `backend/internal/approval/tier_test.go`

**Interfaces:**
- Produces: `func resolveTierOffice(ancestors []sqlc.GetOfficeAncestorsRow, originID uuid.UUID, level sqlc.SharedApproverLevel) (uuid.UUID, bool)`.

- [ ] **Step 1: Write the failing test**

```go
package approval

import (
	"testing"
	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func tier(v sqlc.SharedApproverLevel) *sqlc.SharedApproverLevel { return &v }

func TestResolveTierOffice(t *testing.T) {
	pusat := uuid.New(); wil := uuid.New(); cab := uuid.New(); out := uuid.New()
	// ancestors of `out`: out(office) -> cab(office) -> wil(wilayah) -> pusat(pusat)
	anc := []sqlc.GetOfficeAncestorsRow{
		{ID: out, ParentID: &cab, Tier: tier("office")},
		{ID: cab, ParentID: &wil, Tier: tier("office")},
		{ID: wil, ParentID: &pusat, Tier: tier("wilayah")},
		{ID: pusat, ParentID: nil, Tier: tier("pusat")},
	}
	if got, ok := resolveTierOffice(anc, out, "office"); !ok || got != out {
		t.Errorf("office should resolve to origin, got %v ok=%v", got, ok)
	}
	if got, ok := resolveTierOffice(anc, out, "wilayah"); !ok || got != wil {
		t.Errorf("wilayah should resolve to wil, got %v", got)
	}
	if got, ok := resolveTierOffice(anc, out, "pusat"); !ok || got != pusat {
		t.Errorf("pusat should resolve to pusat, got %v", got)
	}
	// missing tier
	anc2 := []sqlc.GetOfficeAncestorsRow{{ID: out, ParentID: nil, Tier: tier("office")}}
	if _, ok := resolveTierOffice(anc2, out, "pusat"); ok {
		t.Errorf("missing pusat tier should be unsatisfiable")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/approval/ -run TestResolveTierOffice`
Expected: FAIL (`undefined: resolveTierOffice`).

- [ ] **Step 3: Implement `service.go` (start) + `resolveTierOffice`**

```go
package approval

import (
	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// resolveTierOffice returns the ancestor office satisfying the required level.
// office/office_subtree => the origin office itself; wilayah/pusat => nearest ancestor with that tier.
func resolveTierOffice(anc []sqlc.GetOfficeAncestorsRow, originID uuid.UUID, level sqlc.SharedApproverLevel) (uuid.UUID, bool) {
	switch level {
	case sqlc.SharedApproverLevelOffice, sqlc.SharedApproverLevelOfficeSubtree:
		return originID, true
	default:
		for _, a := range anc {
			if a.Tier != nil && *a.Tier == level {
				return a.ID, true
			}
		}
		return uuid.Nil, false
	}
}
```

> Confirm the generated enum constant names (`SharedApproverLevelOffice`, `SharedApproverLevelOfficeSubtree`, `SharedApproverLevelWilayah`, `SharedApproverLevelPusat`) and `GetOfficeAncestorsRow` field types.

- [ ] **Step 4: Run pass + commit**

Run: `cd backend && go test ./internal/approval/ -run TestResolveTierOffice`
```bash
git add backend/internal/approval/service.go backend/internal/approval/tier_test.go
git commit -m "feat(approval): tier resolution from office ancestors"
```

### Task 12: Chain construction from thresholds

**Files:**
- Modify: `backend/internal/approval/service.go`
- Test: `backend/internal/approval/chain_test.go`

**Interfaces:**
- Produces: `func buildChain(steps []sqlc.ApprovalApprovalThreshold) []chainStep`; `type chainStep struct { Order int32; Level sqlc.SharedApproverLevel }`.

- [ ] **Step 1: Write the failing test**

```go
package approval

import (
	"testing"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestBuildChain_Ordered(t *testing.T) {
	steps := []sqlc.ApprovalApprovalThreshold{
		{StepOrder: 2, RequiredLevel: "wilayah"},
		{StepOrder: 1, RequiredLevel: "office"},
		{StepOrder: 3, RequiredLevel: "pusat"},
	}
	chain := buildChain(steps)
	if len(chain) != 3 { t.Fatalf("want 3, got %d", len(chain)) }
	if chain[0].Level != "office" || chain[1].Level != "wilayah" || chain[2].Level != "pusat" {
		t.Fatalf("chain not ordered: %+v", chain)
	}
}

func TestBuildChain_Empty(t *testing.T) {
	if len(buildChain(nil)) != 0 { t.Fatal("empty thresholds -> empty chain") }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/approval/ -run TestBuildChain`
Expected: FAIL.

- [ ] **Step 3: Implement `buildChain`**

```go
import "sort"

type chainStep struct {
	Order int32
	Level sqlc.SharedApproverLevel
}

func buildChain(steps []sqlc.ApprovalApprovalThreshold) []chainStep {
	out := make([]chainStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, chainStep{Order: s.StepOrder, Level: s.RequiredLevel})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}
```

- [ ] **Step 4: Run pass + commit**

Run: `cd backend && go test ./internal/approval/ -run TestBuildChain`
```bash
git add backend/internal/approval/service.go backend/internal/approval/chain_test.go
git commit -m "feat(approval): chain construction from threshold steps"
```

### Task 13: Eligibility + SoD

**Files:**
- Modify: `backend/internal/approval/service.go`
- Test: `backend/internal/approval/eligibility_test.go`

**Interfaces:**
- Produces: `type Caller struct { UserID, RoleID uuid.UUID; AllScope bool; OfficeIDs []uuid.UUID }`; `func eligibleToDecide(caller Caller, req sqlc.ApprovalRequest, step sqlc.ApprovalRequestApproval, priorApprovers []uuid.UUID, tierOffice uuid.UUID, tierOK bool) error` (returns nil if eligible, else `ErrSelfApproval`/`ErrNotEligible`).

- [ ] **Step 1: Write the failing test**

```go
package approval

import (
	"testing"
	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestEligibility(t *testing.T) {
	maker := uuid.New(); office := uuid.New(); approver := uuid.New(); prior := uuid.New()
	req := sqlc.ApprovalRequest{RequestedByID: maker, OfficeID: &office}
	step := sqlc.ApprovalRequestApproval{RequiredLevel: "office"}

	// maker cannot self-approve
	if err := eligibleToDecide(Caller{UserID: maker, AllScope: true}, req, step, nil, office, true); err != ErrSelfApproval {
		t.Errorf("maker self-approve: want ErrSelfApproval, got %v", err)
	}
	// prior approver cannot approve again
	if err := eligibleToDecide(Caller{UserID: prior, AllScope: true}, req, step, []uuid.UUID{prior}, office, true); err != ErrSelfApproval {
		t.Errorf("prior approver: want ErrSelfApproval, got %v", err)
	}
	// tier unsatisfiable
	if err := eligibleToDecide(Caller{UserID: approver, AllScope: true}, req, step, nil, uuid.Nil, false); err != ErrNotEligible {
		t.Errorf("tier missing: want ErrNotEligible, got %v", err)
	}
	// out of scope (does not cover tier office)
	other := uuid.New()
	if err := eligibleToDecide(Caller{UserID: approver, OfficeIDs: []uuid.UUID{other}}, req, step, nil, office, true); err != ErrNotEligible {
		t.Errorf("out of scope: want ErrNotEligible, got %v", err)
	}
	// happy path: global scope, distinct identity
	if err := eligibleToDecide(Caller{UserID: approver, AllScope: true}, req, step, nil, office, true); err != nil {
		t.Errorf("happy: want nil, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/approval/ -run TestEligibility`
Expected: FAIL.

- [ ] **Step 3: Implement `Caller` + `eligibleToDecide`**

```go
import "github.com/ragbuaj/inventra/internal/masterdata/common"

type Caller struct {
	UserID, RoleID uuid.UUID
	AllScope       bool
	OfficeIDs      []uuid.UUID
}

func eligibleToDecide(caller Caller, req sqlc.ApprovalRequest, step sqlc.ApprovalRequestApproval,
	priorApprovers []uuid.UUID, tierOffice uuid.UUID, tierOK bool) error {
	// SoD: not the maker
	if caller.UserID == req.RequestedByID { return ErrSelfApproval }
	// SoD: not a prior approver
	for _, p := range priorApprovers {
		if p == caller.UserID { return ErrSelfApproval }
	}
	if !tierOK { return ErrNotEligible }
	// scope must cover the tier office
	if !common.InScope(caller.AllScope, caller.OfficeIDs, tierOffice) { return ErrNotEligible }
	return nil
}
```

> Add sentinel errors in `service.go`: `ErrSelfApproval`, `ErrNotEligible`, `ErrNoThreshold`, `ErrInvalidState`, `ErrNotFound`, `ErrForbidden`.

- [ ] **Step 4: Run pass + commit**

Run: `cd backend && go test ./internal/approval/ -run TestEligibility`
```bash
git add backend/internal/approval/service.go backend/internal/approval/eligibility_test.go
git commit -m "feat(approval): eligibility + segregation-of-duty checks"
```

### Task 14: Service struct + Submit (chain persisted in tx)

**Files:**
- Modify: `backend/internal/approval/service.go`

**Interfaces:**
- Produces: `type Service struct{...}`; `func NewService(q *sqlc.Queries, pool *pgxpool.Pool, scope *authz.ScopeService, rdb *redis.Client) *Service`; `func (s *Service) RegisterExecutor(t sqlc.SharedRequestType, e Executor)`; `type SubmitInput{ Type sqlc.SharedRequestType; Amount string; OfficeID uuid.UUID; TargetEntity *string; TargetID *uuid.UUID; Payload []byte; Reason *string; Maker uuid.UUID }`; `func (s *Service) Submit(ctx, in SubmitInput) (sqlc.ApprovalRequest, error)`.

- [ ] **Step 1: Implement Service + RegisterExecutor + Submit**

```go
import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/ragbuaj/inventra/internal/authz"
)

type Service struct {
	q     *sqlc.Queries
	pool  *pgxpool.Pool
	scope *authz.ScopeService
	rdb   *redis.Client
	exec  registry
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, scope *authz.ScopeService, rdb *redis.Client) *Service {
	return &Service{q: q, pool: pool, scope: scope, rdb: rdb, exec: registry{}}
}

func (s *Service) RegisterExecutor(t sqlc.SharedRequestType, e Executor) { s.exec[t] = e }

type SubmitInput struct {
	Type         sqlc.SharedRequestType
	Amount       string
	OfficeID     uuid.UUID
	TargetEntity *string
	TargetID     *uuid.UUID
	Payload      []byte
	Reason       *string
	Maker        uuid.UUID
}

func (s *Service) Submit(ctx context.Context, in SubmitInput) (sqlc.ApprovalRequest, error) {
	steps, err := s.q.MatchThresholdSteps(ctx, sqlc.MatchThresholdStepsParams{
		RequestType: in.Type, Amount: in.Amount,
	})
	if err != nil { return sqlc.ApprovalRequest{}, mapDBError(err) }
	chain := buildChain(steps)
	if len(chain) == 0 { return sqlc.ApprovalRequest{}, ErrNoThreshold }

	tx, err := s.pool.Begin(ctx)
	if err != nil { return sqlc.ApprovalRequest{}, err }
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	req, err := qtx.CreateRequest(ctx, sqlc.CreateRequestParams{
		Type: in.Type, OfficeID: &in.OfficeID, Amount: &in.Amount,
		TargetEntity: in.TargetEntity, TargetID: in.TargetID,
		Payload: in.Payload, Reason: in.Reason, RequestedByID: in.Maker,
	})
	if err != nil { return sqlc.ApprovalRequest{}, mapDBError(err) }
	for _, st := range chain {
		if _, err := qtx.CreateRequestApproval(ctx, sqlc.CreateRequestApprovalParams{
			RequestID: req.ID, StepOrder: st.Order, RequiredLevel: st.Level,
		}); err != nil { return sqlc.ApprovalRequest{}, mapDBError(err) }
	}
	if err := tx.Commit(ctx); err != nil { return sqlc.ApprovalRequest{}, err }
	return req, nil
}
```

> Add `mapDBError` to the approval package (mirror the asset one). Confirm `WithTx` returns `*sqlc.Queries` in this codebase.

- [ ] **Step 2: Build**

Run: `cd backend && go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/approval/service.go
git commit -m "feat(approval): submit builds threshold-driven approval chain in tx"
```

### Task 15: Decide (approve/reject/advance + executor in tx) + Cancel + Inbox

**Files:**
- Modify: `backend/internal/approval/service.go`

**Interfaces:**
- Produces: `func (s *Service) Decide(ctx, requestID uuid.UUID, caller Caller, approve bool, note *string) (sqlc.ApprovalRequest, error)`; `func (s *Service) Cancel(ctx, requestID, maker uuid.UUID) (sqlc.ApprovalRequest, error)`; `func (s *Service) Inbox(ctx, caller Caller) ([]sqlc.ApprovalRequest, error)`; helper `func (s *Service) ancestorsFor(ctx, officeID uuid.UUID) ([]sqlc.GetOfficeAncestorsRow, error)`.

- [ ] **Step 1: Implement Decide**

```go
func (s *Service) Decide(ctx context.Context, requestID uuid.UUID, caller Caller, approve bool, note *string) (sqlc.ApprovalRequest, error) {
	req, err := s.q.GetRequest(ctx, requestID)
	if err != nil { return req, mapDBError(err) }
	if req.Status != sqlc.SharedRequestStatusPending { return req, ErrInvalidState }
	approvals, err := s.q.ListRequestApprovals(ctx, requestID)
	if err != nil { return req, mapDBError(err) }

	var step sqlc.ApprovalRequestApproval
	var prior []uuid.UUID
	found := false
	for _, a := range approvals {
		if a.StepOrder < req.CurrentStep && a.ApproverID != nil { prior = append(prior, *a.ApproverID) }
		if a.StepOrder == req.CurrentStep { step = a; found = true }
	}
	if !found { return req, ErrInvalidState }

	// eligibility
	var tierOffice uuid.UUID; tierOK := false
	if req.OfficeID != nil {
		anc, err := s.ancestorsFor(ctx, *req.OfficeID)
		if err != nil { return req, err }
		tierOffice, tierOK = resolveTierOffice(anc, *req.OfficeID, step.RequiredLevel)
	}
	if err := eligibleToDecide(caller, req, step, prior, tierOffice, tierOK); err != nil { return req, err }

	tx, err := s.pool.Begin(ctx)
	if err != nil { return req, err }
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	if !approve {
		if _, err := qtx.DecideRequestApproval(ctx, sqlc.DecideRequestApprovalParams{
			RequestID: requestID, StepOrder: step.StepOrder, ApproverID: &caller.UserID,
			Decision: sqlc.SharedRequestStatusRejected, Note: note,
		}); err != nil { return req, mapDBError(err) }
		out, err := qtx.SetRequestDecision(ctx, sqlc.SetRequestDecisionParams{
			ID: requestID, Status: sqlc.SharedRequestStatusRejected, DecidedByID: &caller.UserID, DecisionNote: note,
		})
		if err != nil { return req, mapDBError(err) }
		if err := tx.Commit(ctx); err != nil { return req, err }
		return out, nil
	}

	if _, err := qtx.DecideRequestApproval(ctx, sqlc.DecideRequestApprovalParams{
		RequestID: requestID, StepOrder: step.StepOrder, ApproverID: &caller.UserID,
		Decision: sqlc.SharedRequestStatusApproved, Note: note,
	}); err != nil { return req, mapDBError(err) }

	isLast := step.StepOrder == int32(len(approvals))
	if !isLast {
		out, err := qtx.AdvanceRequestStep(ctx, requestID)
		if err != nil { return req, mapDBError(err) }
		if err := tx.Commit(ctx); err != nil { return req, err }
		return out, nil
	}
	// final approval: mark approved + run executor in the same tx
	out, err := qtx.SetRequestDecision(ctx, sqlc.SetRequestDecisionParams{
		ID: requestID, Status: sqlc.SharedRequestStatusApproved, DecidedByID: &caller.UserID, DecisionNote: note,
	})
	if err != nil { return req, mapDBError(err) }
	exec, ok := s.exec[req.Type]
	if !ok { return req, ErrInvalidState }
	if err := exec.Execute(ctx, qtx, out); err != nil { return req, err }
	if err := tx.Commit(ctx); err != nil { return req, err }
	return out, nil
}

func (s *Service) ancestorsFor(ctx context.Context, officeID uuid.UUID) ([]sqlc.GetOfficeAncestorsRow, error) {
	return s.q.GetOfficeAncestors(ctx, officeID)
}
```

- [ ] **Step 2: Implement Cancel + Inbox**

```go
func (s *Service) Cancel(ctx context.Context, requestID, maker uuid.UUID) (sqlc.ApprovalRequest, error) {
	out, err := s.q.CancelRequest(ctx, sqlc.CancelRequestParams{ID: requestID, RequestedByID: maker})
	if err != nil { return out, mapDBError(err) } // ErrNoRows => not pending/not maker => ErrNotFound
	return out, nil
}

func (s *Service) Inbox(ctx context.Context, caller Caller) ([]sqlc.ApprovalRequest, error) {
	candidates, err := s.q.ListInboxCandidates(ctx)
	if err != nil { return nil, mapDBError(err) }
	out := make([]sqlc.ApprovalRequest, 0)
	for _, req := range candidates {
		approvals, err := s.q.ListRequestApprovals(ctx, req.ID)
		if err != nil { return nil, mapDBError(err) }
		var step sqlc.ApprovalRequestApproval; var prior []uuid.UUID; found := false
		for _, a := range approvals {
			if a.StepOrder < req.CurrentStep && a.ApproverID != nil { prior = append(prior, *a.ApproverID) }
			if a.StepOrder == req.CurrentStep { step = a; found = true }
		}
		if !found || req.OfficeID == nil { continue }
		anc, err := s.ancestorsFor(ctx, *req.OfficeID)
		if err != nil { return nil, err }
		to, ok := resolveTierOffice(anc, *req.OfficeID, step.RequiredLevel)
		if eligibleToDecide(caller, req, step, prior, to, ok) == nil { out = append(out, req) }
	}
	return out, nil
}
```

- [ ] **Step 3: Build**

Run: `cd backend && go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/approval/service.go
git commit -m "feat(approval): decide/cancel/inbox with atomic executor on final approval"
```

### Task 16: Asset executors (implement approval.Executor)

**Files:**
- Modify: `backend/internal/asset/service.go`
- Create: `backend/internal/asset/executor.go`

**Interfaces:**
- Produces: `func (s *Service) CreateExecutor() approval.Executor`; `func (s *Service) DisposalExecutor() approval.Executor`; `func (s *Service) ExclusionExecutor() approval.Executor`. Consumes `approval.Executor` (Task 10).

> Import direction: `asset` imports `approval` (for the interface). `approval` must NOT import `asset`. Wiring injects asset executors into approval at router construction — no cycle.

- [ ] **Step 1: Define the create payload shape + executors**

```go
package asset

import (
	"context"
	"encoding/json"
	"time"
	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// AssetCreatePayload is the JSON stored in requests.payload for asset_create.
type AssetCreatePayload struct {
	Name          string  `json:"name"`
	CategoryID    string  `json:"category_id"`
	OfficeID      string  `json:"office_id"`
	RoomID        *string `json:"room_id"`
	AssetClass    string  `json:"asset_class"`
	PurchaseCost  *string `json:"purchase_cost"`
	PurchaseDate  *string `json:"purchase_date"` // RFC3339 date
	SerialNumber  *string `json:"serial_number"`
}

type createExec struct{ s *Service }
func (e createExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	var p AssetCreatePayload
	if err := json.Unmarshal(req.Payload, &p); err != nil { return err }
	officeID, err := uuid.Parse(p.OfficeID); if err != nil { return ErrInvalidRef }
	categoryID, err := uuid.Parse(p.CategoryID); if err != nil { return ErrInvalidRef }
	year := int32(time.Now().Year())
	if p.PurchaseDate != nil {
		if t, err := time.Parse("2006-01-02", *p.PurchaseDate); err == nil { year = int32(t.Year()) }
	}
	tag, err := e.s.GenerateAssetTag(ctx, qtx, officeID, categoryID, year)
	if err != nil { return err }
	_, err = qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
		AssetTag: tag, Name: p.Name, CategoryID: categoryID, OfficeID: officeID,
		AssetClass: sqlc.SharedAssetClass(p.AssetClass), Capitalized: true,
		CreatedByID: &req.RequestedByID,
		// room_id, serial_number, purchase_cost, purchase_date filled from payload (parse appropriately)
	})
	return mapDBError(err)
}

type disposalExec struct{ s *Service }
func (e disposalExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil { return ErrInvalidRef }
	cur, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil { return mapDBError(err) }
	if !validTransition(cur.Status, sqlc.SharedAssetStatusDisposed) { return ErrInvalidState }
	_, err = qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: *req.TargetID, Status: sqlc.SharedAssetStatusDisposed})
	return mapDBError(err)
}

type exclusionExec struct{ s *Service }
func (e exclusionExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil { return ErrInvalidRef }
	_, err := qtx.SetAssetValuationExclusion(ctx, sqlc.SetAssetValuationExclusionParams{
		ID: *req.TargetID, ExcludedFromValuation: true, ValuationExclusionReason: req.Reason,
	})
	return mapDBError(err)
}

func (s *Service) CreateExecutor() approval.Executor   { return createExec{s} }
func (s *Service) DisposalExecutor() approval.Executor { return disposalExec{s} }
func (s *Service) ExclusionExecutor() approval.Executor { return exclusionExec{s} }
```

> `GenerateAssetTag` signature from Task 5 takes `*sqlc.Queries` (the tx-bound `qtx`) so the counter bump and insert share the transaction. Fill remaining `CreateAssetParams` fields (room_id, serial_number, purchase_cost as `*string`, purchase_date as `pgtype.Date`) by parsing the payload.

- [ ] **Step 2: Build**

Run: `cd backend && go build ./...`
Expected: no errors; no import cycle.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/asset/executor.go backend/internal/asset/service.go
git commit -m "feat(asset): approval executors for create/disposal/valuation-exclusion"
```

### Task 17: Threshold CRUD + Redis cache invalidation

**Files:**
- Modify: `backend/internal/approval/service.go`

**Interfaces:**
- Produces: `func (s *Service) ListThresholds(ctx) ([]sqlc.ApprovalApprovalThreshold, error)`; `CreateThreshold`/`UpdateThreshold`/`DeleteThreshold` service methods; `func (s *Service) invalidateThresholdCache(ctx) error`.

> The threshold-match query reads the DB directly; if a Redis read-through cache is added for `MatchThresholdSteps`, mutations must call `invalidateThresholdCache`. For this slice, implement CRUD + a cache key stub matching the authz cache pattern (`approval:thresholds`) so the invalidation seam exists even if reads are not yet cached.

- [ ] **Step 1: Implement threshold CRUD service methods**

```go
func (s *Service) ListThresholds(ctx context.Context) ([]sqlc.ApprovalApprovalThreshold, error) {
	rows, err := s.q.ListThresholds(ctx)
	return rows, mapDBError(err)
}
func (s *Service) CreateThreshold(ctx context.Context, p sqlc.CreateThresholdParams) (sqlc.ApprovalApprovalThreshold, error) {
	out, err := s.q.CreateThreshold(ctx, p)
	if err == nil { _ = s.invalidateThresholdCache(ctx) }
	return out, mapDBError(err)
}
func (s *Service) UpdateThreshold(ctx context.Context, p sqlc.UpdateThresholdParams) (sqlc.ApprovalApprovalThreshold, error) {
	out, err := s.q.UpdateThreshold(ctx, p)
	if err == nil { _ = s.invalidateThresholdCache(ctx) }
	return out, mapDBError(err)
}
func (s *Service) DeleteThreshold(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.SoftDeleteThreshold(ctx, id)
	if err != nil { return mapDBError(err) }
	if n == 0 { return ErrNotFound }
	_ = s.invalidateThresholdCache(ctx)
	return nil
}
func (s *Service) invalidateThresholdCache(ctx context.Context) error {
	return s.rdb.Del(ctx, "approval:thresholds").Err()
}
```

- [ ] **Step 2: Build + commit**

Run: `cd backend && go build ./...`
```bash
git add backend/internal/approval/service.go
git commit -m "feat(approval): threshold CRUD with cache invalidation seam"
```

---

## Phase E — Handlers, routes, wiring, OpenAPI

### Task 18: Approval DTO + handler + routes

**Files:**
- Create: `backend/internal/approval/dto.go`, `backend/internal/approval/handler.go`, `backend/internal/approval/routes.go`
- Test: `backend/internal/approval/dto_test.go`

**Interfaces:**
- Produces: `type Handler`; `func NewHandler(svc *Service, fieldSvc *authz.FieldService, scoped common.ScopedDeps, aud *audit.Service) *Handler`; `func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc, permSvc *authz.PermissionService)`.

- [ ] **Step 1: Write a DTO validation unit test**

```go
package approval

import "testing"

func TestSubmitRequest_Validate(t *testing.T) {
	r := SubmitRequest{Type: "asset_create", Amount: "150000000", OfficeID: "not-a-uuid"}
	if err := r.validate(); err == nil { t.Fatal("expected invalid office_id error") }
	r.OfficeID = "11111111-1111-1111-1111-111111111111"
	if err := r.validate(); err != nil { t.Fatalf("expected valid, got %v", err) }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/approval/ -run TestSubmitRequest`
Expected: FAIL.

- [ ] **Step 3: Implement `dto.go`**

```go
package approval

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

type SubmitRequest struct {
	Type     string          `json:"type" binding:"required,oneof=asset_create asset_disposal valuation_exclusion"`
	Amount   string          `json:"amount" binding:"required"`
	OfficeID string          `json:"office_id" binding:"required"`
	TargetID *string         `json:"target_id"`
	Payload  json.RawMessage `json:"payload"`
	Reason   *string         `json:"reason"`
}

func (r SubmitRequest) validate() error {
	if _, err := uuid.Parse(r.OfficeID); err != nil { return errors.New("invalid office_id") }
	if r.TargetID != nil { if _, err := uuid.Parse(*r.TargetID); err != nil { return errors.New("invalid target_id") } }
	return nil
}

type DecideRequest struct {
	Decision string  `json:"decision" binding:"required,oneof=approve reject"`
	Note     *string `json:"note"`
}

func requestToMap(r sqlc.ApprovalRequest) map[string]any {
	return map[string]any{
		"id": r.ID.String(), "type": string(r.Type), "status": string(r.Status),
		"amount": r.Amount, "current_step": r.CurrentStep,
		"office_id": common.UUIDPtrStr(r.OfficeID), "target_id": common.UUIDPtrStr(r.TargetID),
		"reason": r.Reason, "requested_by_id": r.RequestedByID.String(),
		"created_at": common.TsStr(r.CreatedAt),
	}
}
```

- [ ] **Step 4: Implement `handler.go`**

```go
package approval

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

type Handler struct {
	svc      *Service
	fieldSvc *authz.FieldService
	scoped   common.ScopedDeps
	aud      *audit.Service
}

func NewHandler(svc *Service, fieldSvc *authz.FieldService, scoped common.ScopedDeps, aud *audit.Service) *Handler {
	return &Handler{svc: svc, fieldSvc: fieldSvc, scoped: scoped, aud: aud}
}

func (h *Handler) callerFromCtx(c *gin.Context) (Caller, error) {
	uid, err := uuid.Parse(c.GetString(string(authz.CtxUserID))); if err != nil { return Caller{}, err }
	rid, _ := uuid.Parse(c.GetString(string(authz.CtxRoleID)))
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests")
	if err != nil { return Caller{}, err }
	return Caller{UserID: uid, RoleID: rid, AllScope: all, OfficeIDs: ids}, nil
}

func (h *Handler) submit(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	if err := req.validate(); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	uid, _ := uuid.Parse(c.GetString(string(authz.CtxUserID)))
	officeID, _ := uuid.Parse(req.OfficeID)
	in := SubmitInput{Type: sqlc.SharedRequestType(req.Type), Amount: req.Amount, OfficeID: officeID,
		Payload: req.Payload, Reason: req.Reason, Maker: uid}
	if req.TargetID != nil { tid, _ := uuid.Parse(*req.TargetID); in.TargetID = &tid }
	out, err := h.svc.Submit(c, in)
	if err != nil { h.svcError(c, err); return }
	audit.Record(c, h.aud, audit.ActionCreate, "requests", out.ID, out.OfficeID, audit.Diff(nil, requestToMap(out)))
	c.JSON(http.StatusCreated, requestToMap(out))
}

func (h *Handler) approve(c *gin.Context) { h.decide(c, true) }
func (h *Handler) reject(c *gin.Context)  { h.decide(c, false) }

func (h *Handler) decide(c *gin.Context, approve bool) {
	id, err := uuid.Parse(c.Param("id")); if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
	var body DecideRequest
	_ = c.ShouldBindJSON(&body)
	caller, err := h.callerFromCtx(c); if err != nil { common.WriteError(c, err); return }
	out, err := h.svc.Decide(c, id, caller, approve, body.Note)
	if err != nil { h.svcError(c, err); return }
	audit.Record(c, h.aud, audit.ActionUpdate, "requests", out.ID, out.OfficeID, audit.Diff(nil, requestToMap(out)))
	c.JSON(http.StatusOK, requestToMap(out))
}

func (h *Handler) cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id")); if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
	uid, _ := uuid.Parse(c.GetString(string(authz.CtxUserID)))
	out, err := h.svc.Cancel(c, id, uid)
	if err != nil { h.svcError(c, err); return }
	c.JSON(http.StatusOK, requestToMap(out))
}

func (h *Handler) inbox(c *gin.Context) {
	caller, err := h.callerFromCtx(c); if err != nil { common.WriteError(c, err); return }
	rows, err := h.svc.Inbox(c, caller)
	if err != nil { h.svcError(c, err); return }
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows { data = append(data, requestToMap(r)) }
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func (h *Handler) list(c *gin.Context) {
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests"); if err != nil { common.WriteError(c, err); return }
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<30)
	rows, total, err := h.svc.List(c, all, ids, c.Query("status"), c.Query("type"), limit, offset)
	if err != nil { h.svcError(c, err); return }
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows { data = append(data, requestToMap(r)) }
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id")); if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
	r, err := h.svc.q.GetRequest(c, id)
	if err != nil { h.svcError(c, mapDBError(err)); return }
	steps, _ := h.svc.q.ListRequestApprovals(c, id)
	out := requestToMap(r); out["steps"] = steps
	c.JSON(http.StatusOK, out)
}

func (h *Handler) svcError(c *gin.Context, err error) {
	switch err {
	case ErrNotFound: c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case ErrSelfApproval, ErrNotEligible, ErrForbidden: c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case ErrInvalidState: c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case ErrNoThreshold: c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default: common.WriteError(c, err)
	}
}
```

> Add `func (s *Service) List(ctx, all bool, ids []uuid.UUID, status, typ string, limit, offset int32) ([]sqlc.ApprovalRequest, int64, error)` to `service.go` (parse `status`/`type` into nullable enum params; empty string → nil). Confirm `authz.CtxUserID`/`CtxRoleID` accessor types.

- [ ] **Step 5: Implement `routes.go`**

```go
package approval

import (
	"github.com/gin-gonic/gin"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc, permSvc *authz.PermissionService) {
	create := middleware.RequirePermission(permSvc, "request.create")
	decide := middleware.RequirePermission(permSvc, "request.decide")
	cfg := middleware.RequirePermission(permSvc, "approval.config.manage")

	g := rg.Group("/requests")
	g.POST("", authMW, create, h.submit)
	g.GET("", authMW, h.list)
	g.GET("/inbox", authMW, decide, h.inbox)
	g.GET("/:id", authMW, h.get)
	g.POST("/:id/approve", authMW, decide, h.approve)
	g.POST("/:id/reject", authMW, decide, h.reject)
	g.POST("/:id/cancel", authMW, h.cancel)

	t := rg.Group("/approval-thresholds")
	t.GET("", authMW, cfg, h.listThresholds)
	t.POST("", authMW, cfg, h.createThreshold)
	t.PUT("/:id", authMW, cfg, h.updateThreshold)
	t.DELETE("/:id", authMW, cfg, h.deleteThreshold)
}
```

> Implement the four threshold handler methods (`listThresholds`/`createThreshold`/`updateThreshold`/`deleteThreshold`) binding a `ThresholdRequest` DTO and calling the service CRUD from Task 17. Keep them in `handler.go`.

- [ ] **Step 6: Run tests + build**

Run: `cd backend && go test ./internal/approval/ && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/approval/dto.go backend/internal/approval/handler.go backend/internal/approval/routes.go backend/internal/approval/dto_test.go backend/internal/approval/service.go
git commit -m "feat(approval): request + threshold HTTP handlers and routes"
```

### Task 19: Router wiring

**Files:**
- Modify: `backend/internal/server/router.go`

**Interfaces:**
- Consumes: `asset.NewService/NewHandler/RegisterRoutes`, `approval.NewService/NewHandler/RegisterRoutes`, `RegisterExecutor`.

- [ ] **Step 1: Wire both modules in `NewRouter`**

```go
// after existing service construction (queries, permSvc, scopeSvc, fieldSvc, auditSvc, requireAuth)
scopedAssets := common.ScopedDeps{Q: queries, Scope: scopeSvc}

assetSvc := asset.NewService(queries, d.Pool)
approvalSvc := approval.NewService(queries, d.Pool, scopeSvc, d.Redis)
approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())
approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, assetSvc.DisposalExecutor())
approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeValuationExclusion, assetSvc.ExclusionExecutor())

asset.RegisterRoutes(api,
	asset.NewHandler(assetSvc, fieldSvc, scopedAssets, auditSvc),
	requireAuth,
	middleware.RequirePermission(permSvc, "asset.view"),
	middleware.RequirePermission(permSvc, "asset.manage"))

approval.RegisterRoutes(api,
	approval.NewHandler(approvalSvc, fieldSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc),
	requireAuth, permSvc)
```

> Add imports for `internal/asset`, `internal/approval`, `internal/masterdata/common`. Confirm `api` is the `/api/v1` group and `d.Redis` exists in `Deps`.

- [ ] **Step 2: Build + vet**

Run: `cd backend && go build ./... && go vet ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/server/router.go
git commit -m "feat(server): wire asset core + approval engine into router"
```

### Task 20: OpenAPI sync

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add paths + schemas**

Add `/assets` (GET list, with query params + `{data,total,limit,offset}`), `/assets/{id}` (GET, PUT),
`/requests` (POST, GET), `/requests/inbox` (GET), `/requests/{id}` (GET), `/requests/{id}/approve`,
`/requests/{id}/reject`, `/requests/{id}/cancel`, `/approval-thresholds` (GET/POST), `/approval-thresholds/{id}` (PUT/DELETE).
Define schemas `Asset`, `AssetUpdate`, `Request`, `SubmitRequest`, `DecideRequest`, `Threshold`. Mirror an
existing path block (e.g. categories) for response envelope + security scheme.

- [ ] **Step 2: Lint**

Run: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(api): OpenAPI for asset core + approval endpoints"
```

---

## Phase F — Integration tests + progress

### Task 21: Integration suite (real Postgres/Redis)

**Files:**
- Create: `backend/internal/approval/integration_test.go` (`//go:build integration`)
- Create: `backend/internal/asset/integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes `internal/testsupport` (Postgres/Redis containers, migration apply, `Reset`, seed helpers).

- [ ] **Step 1: Write the end-to-end approval+create test**

```go
//go:build integration

package approval_test

import (
	"context"
	"testing"
	// import testsupport, sqlc, approval, asset, authz, uuid, require
)

// Skeleton — fill helpers per internal/testsupport conventions.
func TestAssetCreate_ThreeStepChain_Executes(t *testing.T) {
	ctx := context.Background()
	env := testsupport.New(t)           // boots containers, applies migrations, seeds roles/thresholds
	defer env.Reset(t)

	// seed: 3-tier office tree (pusat>wilayah>cabang), 4 users (maker + 3 approvers at each tier),
	// a category with code, then:
	// 1) maker Submit asset_create amount=150_000_000 (>100jt => 3 steps)
	// 2) approver-office approve, approver-wilayah approve, approver-pusat approve
	// 3) assert an asset row exists with a generated asset_tag and status=available
	// Also assert: maker self-approve => 403/ErrSelfApproval at step 1;
	//   approver of step 1 cannot approve step 2 (SoD).
	require.True(t, true) // replace with real assertions
}
```

- [ ] **Step 2: Write disposal, exclusion, cancel, scope, rollback tests**

Add `TestAssetDisposal_StatusTransition`, `TestValuationExclusion_SetsFlag`,
`TestCancelByMakerOnly`, `TestListRequests_ScopeFiltered`,
`TestExecutorError_RollsBack` (force a disposal on a `disposed` asset → executor returns
`ErrInvalidState` → request stays pending, asset unchanged), and
`TestThresholdEdit_TakesEffect` (edit a band, new submit uses new chain length).

- [ ] **Step 3: Write asset read masking + tag atomicity integration tests**

In `asset/integration_test.go`: seed assets, assert Staf role response omits `purchase_cost`/`book_value`;
Manager omits `accumulated_depreciation`; Superadmin sees all. Add a concurrency test that calls
`GenerateAssetTag` N times for the same (office,category,year) and asserts N distinct sequential tags.

- [ ] **Step 4: Run integration suite**

Run: `cd backend && go test -tags=integration ./internal/approval/ ./internal/asset/`
Expected: PASS (Docker available for testcontainers).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/approval/integration_test.go backend/internal/asset/integration_test.go
git commit -m "test: integration coverage for approval engine + asset core"
```

### Task 22: PROGRESS.md + final verification

**Files:**
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Run the full verification gate**

Run:
```bash
cd backend && go build ./... && go vet ./... && go test ./... \
  && go test -tags=integration ./internal/approval/ ./internal/asset/
npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
```
Expected: all green.

- [ ] **Step 2: Update `docs/PROGRESS.md`**

Tick: **Approval (maker-checker)** and the relevant **Asset core** items (CRUD read/update,
asset_tag generator, status state machine, data-scoping + field-permission, valuation-exclusion flag);
add a sub-note that attachments/barcode/transfer/disposal-accounting remain. Refresh the
"▶ Next session — start here" block to point at the next real step (e.g. Asset attachments / MinIO, or
wiring the frontend Asset screens to these endpoints). Note the PR number when merged.

- [ ] **Step 3: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): approval engine + asset core landed"
```

---

## Self-Review notes (spec coverage)

- Spec bagian 1 migration → Task 1. bagian 2 queries → Tasks 2–4. bagian 3 approval module → Tasks 10–18.
  bagian 4 asset module → Tasks 5–9, 16. bagian 5 field masking → Tasks 1 (seed), 9 (apply).
  bagian 6 authz/eligibility/endpoints → Tasks 11–13, 18, 19. bagian 7 wiring → Task 19. Seeds → Task 1.
  Testing → all unit tasks + Task 21. OpenAPI → Task 20. PROGRESS + gates → Task 22.
- Import-cycle risk (asset↔approval) resolved: `approval.Executor` interface lives in approval; asset
  imports approval; wiring injects (Tasks 10, 16, 19).
- Open placeholders are intentional bank-policy values, flagged in Task 1 and the spec.
```
