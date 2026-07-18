# Assignment (Penugasan / Peminjaman) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the asset assignment lifecycle — Manager direct check-out/check-in/history (wire the existing mock screen to real APIs) plus a Staf borrow-request path via the approval engine (dedicated submit endpoint + `assignment` executor), with two new Staf UI surfaces (`/peminjaman` page + "Ajukan Peminjaman" modal on Asset Detail).

**Architecture:** New backend module `internal/assignment` (ADR-0008 four-file split + `executor.go`), mirroring `internal/transfer`. Direct check-out/check-in are `assignment.manage`-gated operational actions with a `pgx.Tx` state machine (`available↔assigned`). Staf borrow submits through a dedicated `POST /assignments/borrow` (like `POST /transfers`) that opens an `assignment`-type approval request; on final approval the registered executor performs the check-out. Frontend wires the Manager screen and adds the Staf page + modal.

**Tech Stack:** Go 1.25 + Gin, pgx/v5, sqlc, golang-migrate, testify + testcontainers-go (integration, `-tags=integration`); Nuxt 4 SPA + Nuxt UI v4, Vitest + @nuxt/test-utils, Playwright.

## Global Constraints

- Go module path `github.com/ragbuaj/inventra`; sqlc output `db/sqlc` — never hand-edit; edit `db/queries/*.sql` or migrations then `sqlc generate`.
- Every endpoint enforces data scope on **read and write**; assignment scope basis is the **asset's office** (`asset.assets.office_id`), resolved via `common.ScopedDeps.CallerOfficeScope(c, "assignments")`.
- Money/numeric columns are Go `string`; peminjaman is **not** value-tiered → approval `amount = "0"`.
- Soft-delete + partial-unique + `set_updated_at` conventions; the `assignment.assignments` table already exists (migration `000011`) — no new table.
- Enum values (verified in `db/sqlc/models.go`): `sqlc.SharedRequestTypeAssignment`, `sqlc.SharedAssignmentStatusActive`/`Returned`, `sqlc.SharedAssetStatusAvailable`/`Assigned`/`UnderMaintenance`.
- Conventional Commits, lowercase scope: `feat(assignment): …`. No Claude/AI attribution in commits.
- Frontend: i18n mandatory (`i18n/locales/{id,en}.json`, default `id`); theme via semantic tokens; build on `U*` components; ESLint `commaDangle: 'never'` + 1tbs.
- Branch: `feat/assignment-module` (already created; spec + mockups committed there).

---

## File Structure

**Backend — create:**
- `backend/db/migrations/000026_assignment_seed.up.sql` / `.down.sql` — permission `assignment.view` + `assignment` threshold band.
- `backend/db/queries/assignments.sql` — sqlc queries.
- `backend/internal/assignment/service.go` — business rules + state machine (Gin-free).
- `backend/internal/assignment/dto.go` — request structs + serialization + payload.
- `backend/internal/assignment/executor.go` — `assignment` approval executor.
- `backend/internal/assignment/handler.go` — HTTP ↔ service.
- `backend/internal/assignment/routes.go` — route registration.
- `backend/internal/assignment/dto_test.go` — unit tests (payload marshal, condition mapping).
- `backend/internal/assignment/assignment_integration_test.go` — integration tests (`//go:build integration`).

**Backend — modify:**
- `backend/internal/authzadmin/catalog.go` — add `assignment.view` to `permissionCatalog`, `"assignments"` to `ScopeModules()`.
- `backend/internal/authzadmin/catalog_test.go` — if it counts groups/keys.
- `backend/db/queries/approval.sql` — add `requested_by` narg to `ListRequestsEnriched` + `CountRequests` (nil-safe).
- `backend/internal/approval/service.go` — thread `requestedBy *uuid.UUID` into `List`.
- `backend/internal/approval/handler.go` — `mine=true` branch in `list`.
- `backend/internal/server/router.go` — construct + wire the assignment module; register executor.
- `backend/api/openapi.yaml` — `Assignment` schema + paths + `mine` param.

**Frontend — create:**
- `frontend/app/constants/assignmentMeta.ts` — status/condition tone maps + date formatter.
- `frontend/app/components/assignment/AjukanPeminjamanModal.vue` — borrow submit modal (used by page + detail).
- `frontend/app/pages/peminjaman.vue` — Staf borrow page.
- `frontend/test/unit/assignment-meta.spec.ts`, `frontend/test/nuxt/peminjaman.spec.ts`, `frontend/test/nuxt/assignment.spec.ts`, `frontend/test/nuxt/ajukan-peminjaman-modal.spec.ts`.
- `frontend/e2e/assignment.spec.ts`.

**Frontend — modify:**
- `frontend/app/composables/api/useAssignment.ts` — rewrite to real `$fetch`.
- `frontend/app/pages/assignment.vue` — wire to real API; gate `assignment.manage`.
- `frontend/app/pages/assets/[tag]/index.vue` — add "Ajukan Peminjaman" button + modal.
- `frontend/app/utils/nav.ts` — add `peminjaman` item; gate `assignment` item.
- `frontend/i18n/locales/{id,en}.json` — `assignment.*` (extend) + `peminjaman.*`.
- Delete `frontend/app/mock/assignment.ts` after wiring (check `useGlobalSearch` first).

---

## Task 1: Migration + permission catalog

**Files:**
- Create: `backend/db/migrations/000026_assignment_seed.up.sql`, `backend/db/migrations/000026_assignment_seed.down.sql`
- Modify: `backend/internal/authzadmin/catalog.go`, `backend/internal/authzadmin/catalog_test.go`

**Interfaces:**
- Produces: permission key `assignment.view`; `approval.approval_thresholds` row for `request_type='assignment'`; `ScopeModules()` includes `"assignments"`.

- [ ] **Step 1: Write the up migration**

`backend/db/migrations/000026_assignment_seed.up.sql`:
```sql
-- Assignment module seed: assignment.view permission + approval band for peminjaman.
-- assignment.manage is already seeded (000005) for superadmin + manager.

INSERT INTO identity.role_permissions (role_id, permission) VALUES
  ('superadmin',    'assignment.view'),
  ('kepala_kanwil', 'assignment.view'),
  ('kepala_unit',   'assignment.view'),
  ('manager',       'assignment.view')
ON CONFLICT DO NOTHING;

-- Peminjaman is not value-tiered: a single office-level approval step.
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order)
VALUES ('assignment', 0, NULL, 'office', 1);
```

- [ ] **Step 2: Write the down migration**

`backend/db/migrations/000026_assignment_seed.down.sql`:
```sql
DELETE FROM approval.approval_thresholds WHERE request_type = 'assignment';
DELETE FROM identity.role_permissions
WHERE permission = 'assignment.view'
  AND role_id IN ('superadmin','kepala_kanwil','kepala_unit','manager');
```

- [ ] **Step 3: Add `assignment.view` to the permission catalog + `assignments` scope module**

In `backend/internal/authzadmin/catalog.go`, replace the `Penyusutan Aset` group's trailing groups so a dedicated group exists. Change the `Cadangan` group to drop the now-real `assignment.manage` line into a real group and add `assignment.view`. Concretely, replace:
```go
	{Group: "Penyusutan Aset", Items: []PermissionItem{
		{"depreciation.view", "Lihat depresiasi"},
		{"depreciation.manage", "Jalankan & tutup periode depresiasi, catat impairment"},
	}},
	{Group: "Cadangan", Items: []PermissionItem{
		{"report.view", "Lihat laporan"},
		{"report.export", "Ekspor laporan"},
		{"maintenance.manage", "Kelola maintenance"},
		{"valuation.exclude.approve", "Setujui pengecualian valuasi"},
		{"assignment.manage", "Kelola penugasan aset"},
	}},
```
with:
```go
	{Group: "Penyusutan Aset", Items: []PermissionItem{
		{"depreciation.view", "Lihat depresiasi"},
		{"depreciation.manage", "Jalankan & tutup periode depresiasi, catat impairment"},
	}},
	{Group: "Penugasan Aset", Items: []PermissionItem{
		{"assignment.view", "Lihat penugasan aset"},
		{"assignment.manage", "Kelola penugasan aset (check-out/check-in)"},
	}},
	{Group: "Cadangan", Items: []PermissionItem{
		{"report.view", "Lihat laporan"},
		{"report.export", "Ekspor laporan"},
		{"maintenance.manage", "Kelola maintenance"},
		{"valuation.exclude.approve", "Setujui pengecualian valuasi"},
	}},
```

And update `ScopeModules()` to include `"assignments"`:
```go
func ScopeModules() []string {
	return []string{"*", "offices", "employees", "assets", "requests", "audit", "transfers", "disposals", "depreciation", "assignments"}
}
```

- [ ] **Step 4: Run the catalog test to check it still passes**

Run: `cd backend && go test ./internal/authzadmin/ -run TestCatalog -v`
Expected: PASS. If `catalog_test.go` asserts a fixed group/key count, update the expected numbers to match (one new group "Penugasan Aset", one new key `assignment.view`, `assignment.manage` moved not added).

- [ ] **Step 5: Apply the migration against the dev DB**

Run:
```bash
cd backend && export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable" && migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: `000026_assignment_seed` applied, no error. (If the dev stack is down, skip — integration tests apply migrations via testsupport.)

- [ ] **Step 6: Commit**

```bash
git add backend/db/migrations/000026_assignment_seed.up.sql backend/db/migrations/000026_assignment_seed.down.sql backend/internal/authzadmin/catalog.go backend/internal/authzadmin/catalog_test.go
git commit -m "feat(assignment): seed assignment.view + approval band + catalog"
```

---

## Task 2: sqlc queries

**Files:**
- Create: `backend/db/queries/assignments.sql`
- Modify: (generated) `backend/db/sqlc/*` via `sqlc generate`

**Interfaces:**
- Produces (sqlc-generated Go): `CheckoutAssignment(ctx, CheckoutAssignmentParams) (AssignmentAssignment, error)`, `CheckinAssignment(ctx, CheckinAssignmentParams) (AssignmentAssignment, error)`, `GetAssignmentScoped(ctx, GetAssignmentScopedParams) (AssignmentAssignment, error)`, `GetAssignmentEnriched(ctx, GetAssignmentEnrichedParams) (GetAssignmentEnrichedRow, error)`, `ListAssignmentsEnriched(ctx, ListAssignmentsEnrichedParams) ([]ListAssignmentsEnrichedRow, error)`, `CountAssignments(ctx, CountAssignmentsParams) (int64, error)`, `ListAssignmentsByAssetEnriched(ctx, ListAssignmentsByAssetEnrichedParams) ([]ListAssignmentsByAssetEnrichedRow, error)`.
- Enriched rows embed `AssignmentAssignment` (`sqlc.embed(asg)`) + `AssetName, AssetTag *string`, `OfficeID uuid.UUID`, `OfficeName, EmployeeName, AssignedByName *string`.

- [ ] **Step 1: Write the queries file**

`backend/db/queries/assignments.sql`:
```sql
-- name: CheckoutAssignment :one
INSERT INTO assignment.assignments (
  asset_id, employee_id, assigned_by_id, checkout_date, due_date, condition_out, notes, status
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(employee_id), sqlc.arg(assigned_by_id),
  sqlc.arg(checkout_date), sqlc.narg(due_date), sqlc.narg(condition_out), sqlc.narg(notes), 'active'
)
RETURNING *;

-- name: CheckinAssignment :one
UPDATE assignment.assignments
SET status = 'returned', checkin_date = sqlc.arg(checkin_date), condition_in = sqlc.narg(condition_in)
WHERE id = sqlc.arg(id) AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: GetAssignmentScoped :one
-- Plain (unenriched) row, scoped by the asset's office. Used by Checkin to load +
-- validate state before the update.
SELECT asg.* FROM assignment.assignments asg
JOIN asset.assets a ON a.id = asg.asset_id AND a.deleted_at IS NULL
WHERE asg.id = sqlc.arg(id) AND asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: GetAssignmentEnriched :one
SELECT sqlc.embed(asg),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       a.office_id AS office_id,
       e.name      AS employee_name,
       u.name      AS assigned_by_name,
       o.name      AS office_name
FROM assignment.assignments asg
LEFT JOIN asset.assets a         ON a.id = asg.asset_id       AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id    AND e.deleted_at IS NULL
LEFT JOIN identity.users u       ON u.id = asg.assigned_by_id AND u.deleted_at IS NULL
LEFT JOIN masterdata.offices o   ON o.id = a.office_id        AND o.deleted_at IS NULL
WHERE asg.id = sqlc.arg(id) AND asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListAssignmentsEnriched :many
SELECT sqlc.embed(asg),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       a.office_id AS office_id,
       e.name      AS employee_name,
       u.name      AS assigned_by_name,
       o.name      AS office_name
FROM assignment.assignments asg
JOIN asset.assets a              ON a.id = asg.asset_id       AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id    AND e.deleted_at IS NULL
LEFT JOIN identity.users u       ON u.id = asg.assigned_by_id AND u.deleted_at IS NULL
LEFT JOIN masterdata.offices o   ON o.id = a.office_id        AND o.deleted_at IS NULL
WHERE asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.assignment_status IS NULL OR asg.status = sqlc.narg(status))
  AND (sqlc.narg(employee_id)::uuid IS NULL OR asg.employee_id = sqlc.narg(employee_id))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR e.name ILIKE '%' || sqlc.narg(search) || '%')
ORDER BY asg.checkout_date DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAssignments :one
SELECT count(*)
FROM assignment.assignments asg
JOIN asset.assets a              ON a.id = asg.asset_id    AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id AND e.deleted_at IS NULL
WHERE asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.assignment_status IS NULL OR asg.status = sqlc.narg(status))
  AND (sqlc.narg(employee_id)::uuid IS NULL OR asg.employee_id = sqlc.narg(employee_id))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR e.name ILIKE '%' || sqlc.narg(search) || '%');

-- name: ListAssignmentsByAssetEnriched :many
SELECT sqlc.embed(asg),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       a.office_id AS office_id,
       e.name      AS employee_name,
       u.name      AS assigned_by_name,
       o.name      AS office_name
FROM assignment.assignments asg
JOIN asset.assets a              ON a.id = asg.asset_id       AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id    AND e.deleted_at IS NULL
LEFT JOIN identity.users u       ON u.id = asg.assigned_by_id AND u.deleted_at IS NULL
LEFT JOIN masterdata.offices o   ON o.id = a.office_id        AND o.deleted_at IS NULL
WHERE asg.asset_id = sqlc.arg(asset_id) AND asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY asg.checkout_date DESC;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd backend && sqlc generate`
Expected: no error; `db/sqlc/assignments.sql.go` created with the interfaces above.

- [ ] **Step 3: Verify it compiles**

Run: `cd backend && go build ./db/...`
Expected: builds clean.

- [ ] **Step 4: Commit**

```bash
git add backend/db/queries/assignments.sql backend/db/sqlc/
git commit -m "feat(assignment): sqlc queries for assignments"
```

---

## Task 3: Service + DTO (check-out/check-in state machine + borrow submit)

**Files:**
- Create: `backend/internal/assignment/service.go`, `backend/internal/assignment/dto.go`, `backend/internal/assignment/dto_test.go`

**Interfaces:**
- Consumes: `sqlc.Queries` methods from Task 2; `approval.Service.Submit`, `approval.Caller`, `approval.SubmitInput`; `common.InScope`.
- Produces:
  - `NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service) *Service`
  - `Checkout(ctx, all bool, ids []uuid.UUID, assignedBy uuid.UUID, in CheckoutInput) (sqlc.AssignmentAssignment, error)`
  - `Checkin(ctx, all bool, ids []uuid.UUID, id uuid.UUID, in CheckinInput) (before, after sqlc.AssignmentAssignment, err error)`
  - `SubmitBorrow(ctx, caller approval.Caller, in BorrowInput) (sqlc.ApprovalRequest, error)`
  - `Available(ctx, officeID uuid.UUID) ([]sqlc.Asset, error)`
  - `Get/List/ListByAsset` (enriched)
  - Sentinels: `ErrNotFound, ErrOutOfScope, ErrAssetNotAvailable, ErrAlreadyAssigned, ErrNotActive, ErrInvalidRef, ErrNoEmployee`
  - Input structs: `CheckoutInput{AssetID uuid.UUID; EmployeeID uuid.UUID; CheckoutDate string; DueDate, ConditionOut, Notes *string}`, `CheckinInput{CheckinDate *string; ConditionIn *string; NeedsMaintenance bool}`, `BorrowInput{AssetID uuid.UUID; DueDate, ConditionOut, Notes *string}`
  - `BorrowPayload{AssetID, DueDate, ConditionOut, Notes *string}` (JSON stored in the approval request payload)

- [ ] **Step 1: Write the DTO file**

`backend/internal/assignment/dto.go`:
```go
package assignment

import (
	"encoding/json"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// CheckoutRequest is the POST /assignments body (Manager direct check-out).
type CheckoutRequest struct {
	AssetID      string  `json:"asset_id" binding:"required,uuid"`
	EmployeeID   string  `json:"employee_id" binding:"required,uuid"`
	CheckoutDate string  `json:"checkout_date" binding:"required"` // "2006-01-02"
	DueDate      *string `json:"due_date"`
	ConditionOut *string `json:"condition_out"`
	Notes        *string `json:"notes"`
}

// CheckinRequest is the POST /assignments/:id/checkin body.
type CheckinRequest struct {
	CheckinDate      *string `json:"checkin_date"` // "2006-01-02"; defaults to now
	ConditionIn      *string `json:"condition_in"`
	NeedsMaintenance bool    `json:"needs_maintenance"`
}

// BorrowRequest is the POST /assignments/borrow body (Staf peminjaman).
type BorrowRequest struct {
	AssetID      string  `json:"asset_id" binding:"required,uuid"`
	DueDate      *string `json:"due_date"`
	ConditionOut *string `json:"condition_out"`
	Notes        *string `json:"notes"`
}

// BorrowPayload is the JSON stored in approval.requests.payload for an assignment request.
type BorrowPayload struct {
	AssetID      string  `json:"asset_id"`
	DueDate      *string `json:"due_date"`
	ConditionOut *string `json:"condition_out"`
	Notes        *string `json:"notes"`
}

func marshalBorrowPayload(in BorrowInput) ([]byte, error) {
	return json.Marshal(BorrowPayload{
		AssetID:      in.AssetID.String(),
		DueDate:      in.DueDate,
		ConditionOut: in.ConditionOut,
		Notes:        in.Notes,
	})
}

// toResponse serializes an assignment row (no sensitive columns).
func toResponse(a sqlc.AssignmentAssignment) map[string]any {
	return map[string]any{
		"id":             a.ID.String(),
		"asset_id":       a.AssetID.String(),
		"employee_id":    a.EmployeeID.String(),
		"assigned_by_id": a.AssignedByID.String(),
		"checkout_date":  common.TsStr(a.CheckoutDate),
		"due_date":       common.DateStr(a.DueDate),
		"checkin_date":   common.TsStr(a.CheckinDate),
		"condition_out":  a.ConditionOut,
		"condition_in":   a.ConditionIn,
		"status":         string(a.Status),
		"notes":          a.Notes,
		"created_at":     common.TsStr(a.CreatedAt),
		"updated_at":     common.TsStr(a.UpdatedAt),
	}
}

// enrichAssignmentMap adds resolved display names to a serialized assignment.
func enrichAssignmentMap(m map[string]any, assetName, assetTag, employeeName, assignedByName, officeName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["employee_name"] = employeeName
	m["assigned_by_name"] = assignedByName
	m["office_name"] = officeName
	return m
}
```

> Note: verify `common.TsStr`/`common.DateStr` accept the exact pgtype the generated columns use (`checkout_date`/`checkin_date` are `timestamptz` → `pgtype.Timestamptz`; `due_date` is `date` → `pgtype.Date`). These helpers are already used across transfer/disposal for the same types.

- [ ] **Step 2: Write the service file**

`backend/internal/assignment/service.go`:
```go
// Package assignment implements asset check-out/check-in (penugasan) and the
// Staf borrow (peminjaman) path via the generic approval engine. Split into
// dto / service / handler / routes (ADR-0008); the service holds business rules
// + data-scope enforcement (Gin-free), scoped by the asset's office.
package assignment

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
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

var (
	ErrNotFound          = errors.New("assignment: not found")
	ErrOutOfScope        = errors.New("assignment: office out of scope")
	ErrAssetNotAvailable = errors.New("assignment: asset is not available for check-out")
	ErrAlreadyAssigned   = errors.New("assignment: asset already has an active assignment")
	ErrNotActive         = errors.New("assignment: assignment is not active")
	ErrInvalidRef        = errors.New("assignment: invalid reference")
	ErrNoEmployee        = errors.New("assignment: requester has no linked employee")
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrAlreadyAssigned
		case "23503":
			return ErrInvalidRef
		}
	}
	return err
}

type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
	appr *approval.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service) *Service {
	return &Service{q: q, pool: pool, appr: appr}
}

type CheckoutInput struct {
	AssetID      uuid.UUID
	EmployeeID   uuid.UUID
	CheckoutDate string // "2006-01-02"
	DueDate      *string
	ConditionOut *string
	Notes        *string
}

type CheckinInput struct {
	CheckinDate      *string
	ConditionIn      *string
	NeedsMaintenance bool
}

type BorrowInput struct {
	AssetID      uuid.UUID
	DueDate      *string
	ConditionOut *string
	Notes        *string
}

func parseDateArg(s *string, def time.Time) (pgtype.Date, error) {
	if s == nil || *s == "" {
		return pgtype.Date{Time: def, Valid: true}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func parseTs(s *string, def time.Time) (pgtype.Timestamptz, error) {
	if s == nil || *s == "" {
		return pgtype.Timestamptz{Time: def, Valid: true}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, nil
}

// checkoutTx performs the shared check-out mutation: insert the assignment row +
// flip the asset to 'assigned', atomically. Caller must have already validated
// scope + availability. assignedBy is the acting user (Manager) or approver.
func checkoutTx(ctx context.Context, qtx *sqlc.Queries, assetID, employeeID, assignedBy uuid.UUID,
	checkoutDate pgtype.Timestamptz, dueDate pgtype.Date, conditionOut, notes *string) (sqlc.AssignmentAssignment, error) {
	a, err := qtx.CheckoutAssignment(ctx, sqlc.CheckoutAssignmentParams{
		AssetID:      assetID,
		EmployeeID:   employeeID,
		AssignedByID: assignedBy,
		CheckoutDate: checkoutDate,
		DueDate:      dueDate,
		ConditionOut: conditionOut,
		Notes:        notes,
	})
	if err != nil {
		return a, mapDBError(err)
	}
	if _, err := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: assetID, Status: sqlc.SharedAssetStatusAssigned}); err != nil {
		return a, mapDBError(err)
	}
	return a, nil
}

// Checkout assigns an available asset to an employee (Manager direct action).
func (s *Service) Checkout(ctx context.Context, all bool, ids []uuid.UUID, assignedBy uuid.UUID, in CheckoutInput) (sqlc.AssignmentAssignment, error) {
	asset, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.AssignmentAssignment{}, mapDBError(err)
	}
	if !common.InScope(all, ids, asset.OfficeID) {
		return sqlc.AssignmentAssignment{}, ErrOutOfScope
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return sqlc.AssignmentAssignment{}, ErrAssetNotAvailable
	}
	coDate, err := parseTs(&in.CheckoutDate, time.Now())
	if err != nil {
		return sqlc.AssignmentAssignment{}, ErrInvalidRef
	}
	dueDate, err := parseDateArg(in.DueDate, time.Time{})
	if err != nil {
		return sqlc.AssignmentAssignment{}, ErrInvalidRef
	}
	if in.DueDate == nil || *in.DueDate == "" {
		dueDate = pgtype.Date{} // NULL
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.AssignmentAssignment{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	a, err := checkoutTx(ctx, qtx, in.AssetID, in.EmployeeID, assignedBy, coDate, dueDate, in.ConditionOut, in.Notes)
	if err != nil {
		return a, err
	}
	if err := tx.Commit(ctx); err != nil {
		return a, err
	}
	return a, nil
}

// Checkin returns an active assignment; the asset goes back to available, or to
// under_maintenance when NeedsMaintenance is set. Returns (before, after).
func (s *Service) Checkin(ctx context.Context, all bool, ids []uuid.UUID, id uuid.UUID, in CheckinInput) (before, after sqlc.AssignmentAssignment, err error) {
	before, err = s.q.GetAssignmentScoped(ctx, sqlc.GetAssignmentScopedParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, before, mapDBError(err)
	}
	if before.Status != sqlc.SharedAssignmentStatusActive {
		return before, before, ErrNotActive
	}
	ciDate, err := parseTs(in.CheckinDate, time.Now())
	if err != nil {
		return before, before, ErrInvalidRef
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return before, before, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	after, err = qtx.CheckinAssignment(ctx, sqlc.CheckinAssignmentParams{ID: id, CheckinDate: ciDate, ConditionIn: in.ConditionIn})
	if err != nil {
		return before, before, mapDBError(err)
	}
	newStatus := sqlc.SharedAssetStatusAvailable
	if in.NeedsMaintenance {
		newStatus = sqlc.SharedAssetStatusUnderMaintenance
	}
	if _, err = qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: before.AssetID, Status: newStatus}); err != nil {
		return before, before, mapDBError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return before, before, err
	}
	return before, after, nil
}

// SubmitBorrow opens an assignment-type approval request for a Staf borrow. The
// asset must be available; the approval routes to the asset office's approvers.
func (s *Service) SubmitBorrow(ctx context.Context, caller approval.Caller, in BorrowInput) (sqlc.ApprovalRequest, error) {
	asset, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return sqlc.ApprovalRequest{}, ErrAssetNotAvailable
	}
	payload, err := marshalBorrowPayload(in)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	entity := "asset"
	targetID := in.AssetID
	return s.appr.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssignment,
		Amount:       "0",
		OfficeID:     asset.OfficeID,
		TargetEntity: &entity,
		TargetID:     &targetID,
		Payload:      payload,
		Reason:       in.Notes,
		Maker:        caller.UserID,
	})
}

// Available lists available assets in the given office (the borrow picker for a
// Staf whose data scope is 'own'). Scoped explicitly to officeID, not the module.
func (s *Service) Available(ctx context.Context, officeID uuid.UUID) ([]sqlc.Asset, error) {
	st := sqlc.SharedAssetStatusAvailable
	rows, err := s.q.ListAssets(ctx, sqlc.ListAssetsParams{
		AllScope:  false,
		OfficeIds: []uuid.UUID{officeID},
		Status:    &st,
		Lim:       100,
		Off:       0,
	})
	return rows, mapDBError(err)
}

// Get returns one scoped, enriched assignment.
func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.GetAssignmentEnrichedRow, error) {
	r, err := s.q.GetAssignmentEnriched(ctx, sqlc.GetAssignmentEnrichedParams{ID: id, AllScope: all, OfficeIds: ids})
	return r, mapDBError(err)
}

// List returns a scoped, paginated, enriched page + total.
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, status string, employeeID *uuid.UUID, search string, limit, offset int32) ([]sqlc.ListAssignmentsEnrichedRow, int64, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	var st *sqlc.SharedAssignmentStatus
	if status != "" {
		v := sqlc.SharedAssignmentStatus(status)
		st = &v
	}
	var sr *string
	if search != "" {
		sr = &search
	}
	rows, err := s.q.ListAssignmentsEnriched(ctx, sqlc.ListAssignmentsEnrichedParams{
		AllScope: all, OfficeIds: ids, Status: st, EmployeeID: employeeID, Search: sr, Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountAssignments(ctx, sqlc.CountAssignmentsParams{
		AllScope: all, OfficeIds: ids, Status: st, EmployeeID: employeeID, Search: sr,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// ListByAsset returns a scoped, enriched assignment history for one asset.
func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, all bool, ids []uuid.UUID) ([]sqlc.ListAssignmentsByAssetEnrichedRow, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListAssignmentsByAssetEnriched(ctx, sqlc.ListAssignmentsByAssetEnrichedParams{AssetID: assetID, AllScope: all, OfficeIds: ids})
	return rows, mapDBError(err)
}
```

> Note: confirm the generated param field names (`Lim`/`Off`, `OfficeIds`, `Search`, `EmployeeID`) against `db/sqlc/assignments.sql.go` after Task 2 — sqlc derives them from the `sqlc.arg`/`narg` names (`lim`→`Lim`, `off`→`Off`, `office_ids`→`OfficeIds`, `all_scope`→`AllScope`). Adjust the struct field names here if sqlc emitted different casing. Verify `sqlc.ListAssetsParams` field names likewise (it already exists from the asset module).

- [ ] **Step 3: Write the DTO unit test**

`backend/internal/assignment/dto_test.go`:
```go
package assignment

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestMarshalBorrowPayload(t *testing.T) {
	id := uuid.New()
	due := "2026-07-15"
	notes := "presentasi"
	b, err := marshalBorrowPayload(BorrowInput{AssetID: id, DueDate: &due, Notes: &notes})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var p BorrowPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.AssetID != id.String() {
		t.Errorf("asset_id = %q, want %q", p.AssetID, id.String())
	}
	if p.DueDate == nil || *p.DueDate != due {
		t.Errorf("due_date = %v, want %q", p.DueDate, due)
	}
	if p.Notes == nil || *p.Notes != notes {
		t.Errorf("notes = %v, want %q", p.Notes, notes)
	}
}

func TestMarshalBorrowPayload_NilOptionals(t *testing.T) {
	b, err := marshalBorrowPayload(BorrowInput{AssetID: uuid.New()})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var p BorrowPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.DueDate != nil || p.Notes != nil || p.ConditionOut != nil {
		t.Errorf("expected nil optionals, got %+v", p)
	}
}
```

- [ ] **Step 4: Build + run the unit test**

Run: `cd backend && go build ./internal/assignment/ && go test ./internal/assignment/ -run TestMarshalBorrowPayload -v`
Expected: PASS (both cases).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/assignment/service.go backend/internal/assignment/dto.go backend/internal/assignment/dto_test.go
git commit -m "feat(assignment): service state machine + borrow submit + dto"
```

---

## Task 4: Executor + handler + routes

**Files:**
- Create: `backend/internal/assignment/executor.go`, `backend/internal/assignment/handler.go`, `backend/internal/assignment/routes.go`

**Interfaces:**
- Consumes: `Service` (Task 3); `approval.Executor`, `approval.ErrInvalidRef`, `approval.ErrConflict`; `common.ScopedDeps`, `common.ClampInt`, `common.WriteError`; `audit.Service`; `middleware.CtxUserID/CtxRoleID`.
- Produces: `(*Service).Executor() approval.Executor`; `NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler`; `RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView, requireCreate gin.HandlerFunc)`.

- [ ] **Step 1: Write the executor**

`backend/internal/assignment/executor.go`:
```go
package assignment

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

// assignmentExec performs the check-out on final approval of a peminjaman, inside
// the commit tx. Employee = the requester's linked employee; assigned_by = approver.
type assignmentExec struct{ s *Service }

func (e assignmentExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return approval.ErrInvalidRef
	}
	var p BorrowPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return approval.ErrInvalidRef
	}
	// Resolve the requester's linked employee (the borrower).
	u, err := qtx.GetUserByID(ctx, req.RequestedByID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
	}
	if u.EmployeeID == nil {
		return approval.ErrInvalidRef // requester has no employee → cannot borrow
	}
	asset, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return approval.ErrConflict // asset no longer available at approval time
	}
	if req.DecidedByID == nil {
		return approval.ErrInvalidRef
	}
	var due pgtype.Date
	if p.DueDate != nil && *p.DueDate != "" {
		t, perr := time.Parse("2006-01-02", *p.DueDate)
		if perr != nil {
			return approval.ErrInvalidRef
		}
		due = pgtype.Date{Time: t, Valid: true}
	}
	coDate := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	_, err = checkoutTx(ctx, qtx, *req.TargetID, *u.EmployeeID, *req.DecidedByID, coDate, due, p.ConditionOut, p.Notes)
	return err
}

// Executor returns the assignment approval executor.
func (s *Service) Executor() approval.Executor { return assignmentExec{s} }

var _ = uuid.Nil // keep uuid import if unused after edits
```

> Remove the `var _ = uuid.Nil` line if `uuid` ends up used; it is a guard so the file compiles as written. (If `goimports`/`go vet` flags the unused import, delete both the import and the guard.)

- [ ] **Step 2: Write the routes**

`backend/internal/assignment/routes.go`:
```go
package assignment

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts assignment endpoints. Reads require assignment.view;
// direct check-out/check-in require assignment.manage; the Staf borrow submit +
// available-asset picker require request.create. Per-asset history is under
// /assets/:id/assignments.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView, requireCreate gin.HandlerFunc) {
	g := rg.Group("/assignments")
	g.GET("", authMW, requireView, h.list)
	g.GET("/available", authMW, requireCreate, h.available)
	g.GET("/:id", authMW, requireView, h.get)
	g.POST("", authMW, requireManage, h.checkout)
	g.POST("/borrow", authMW, requireCreate, h.borrow)
	g.POST("/:id/checkin", authMW, requireManage, h.checkin)

	rg.GET("/assets/:id/assignments", authMW, requireView, h.listByAsset)
}
```

> Route ordering: `/available` and `/borrow` are registered so they don't collide with `/:id`. Gin matches static segments before `:id` params at the same level, but `/borrow` is a POST and `/:id` GET, and `/available` is GET vs `/:id` GET — register `/available` before `/:id` (done above) to avoid `available` being parsed as an `:id`.

- [ ] **Step 3: Write the handler**

`backend/internal/assignment/handler.go`:
```go
package assignment

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

const scopeModule = "assignments"

type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
	q      *sqlc.Queries
	aud    *audit.Service
}

func NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler {
	return &Handler{svc: svc, scoped: common.ScopedDeps{Q: q, Scope: scope}, q: q, aud: aud}
}

func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrNotActive), errors.Is(err, ErrAlreadyAssigned):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAssetNotAvailable), errors.Is(err, ErrInvalidRef), errors.Is(err, ErrNoEmployee):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// caller mirrors transfer/handler.go's caller(): resolves user id + office scope.
func (h *Handler) caller(c *gin.Context) (approval.Caller, bool, []uuid.UUID, error) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return approval.Caller{}, false, nil, err
	}
	rid, _ := uuid.Parse(c.GetString(middleware.CtxRoleID))
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		return approval.Caller{}, false, nil, err
	}
	return approval.Caller{UserID: uid, RoleID: rid, AllScope: all, OfficeIDs: ids}, all, ids, nil
}

func (h *Handler) checkout(c *gin.Context) {
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	assetID, _ := uuid.Parse(req.AssetID)
	employeeID, _ := uuid.Parse(req.EmployeeID)
	a, err := h.svc.Checkout(c.Request.Context(), all, ids, caller.UserID, CheckoutInput{
		AssetID: assetID, EmployeeID: employeeID, CheckoutDate: req.CheckoutDate,
		DueDate: req.DueDate, ConditionOut: req.ConditionOut, Notes: req.Notes,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "assignments", a.ID, nil, audit.Diff(nil, toResponse(a)))
	c.JSON(http.StatusCreated, toResponse(a))
}

func (h *Handler) checkin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	before, after, err := h.svc.Checkin(c.Request.Context(), all, ids, id, CheckinInput{
		CheckinDate: req.CheckinDate, ConditionIn: req.ConditionIn, NeedsMaintenance: req.NeedsMaintenance,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "assignments", after.ID, nil, audit.Diff(toResponse(before), toResponse(after)))
	c.JSON(http.StatusOK, toResponse(after))
}

func (h *Handler) borrow(c *gin.Context) {
	var req BorrowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, _, _, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	assetID, _ := uuid.Parse(req.AssetID)
	out, err := h.svc.SubmitBorrow(c.Request.Context(), caller, BorrowInput{
		AssetID: assetID, DueDate: req.DueDate, ConditionOut: req.ConditionOut, Notes: req.Notes,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "requests", out.ID, out.OfficeID, audit.Diff(nil, map[string]any{"request_id": out.ID.String(), "type": "assignment", "asset_id": req.AssetID}))
	c.JSON(http.StatusCreated, gin.H{"request_id": out.ID.String(), "status": string(out.Status)})
}

func (h *Handler) available(c *gin.Context) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user"})
		return
	}
	u, err := h.q.GetUserByID(c.Request.Context(), uid)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if u.OfficeID == nil {
		c.JSON(http.StatusOK, gin.H{"data": []any{}})
		return
	}
	rows, err := h.svc.Available(c.Request.Context(), *u.OfficeID)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		data = append(data, map[string]any{"id": a.ID.String(), "asset_tag": a.AssetTag, "name": a.Name})
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	r, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, enrichAssignmentMap(toResponse(r.AssignmentAssignment), r.AssetName, r.AssetTag, r.EmployeeName, r.AssignedByName, r.OfficeName))
}

func (h *Handler) list(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	var empID *uuid.UUID
	if e := c.Query("employee_id"); e != "" {
		if id, perr := uuid.Parse(e); perr == nil {
			empID = &id
		}
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.List(c.Request.Context(), all, ids, c.Query("status"), empID, c.Query("search"), limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichAssignmentMap(toResponse(r.AssignmentAssignment), r.AssetName, r.AssetTag, r.EmployeeName, r.AssignedByName, r.OfficeName))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) listByAsset(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	rows, err := h.svc.ListByAsset(c.Request.Context(), assetID, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichAssignmentMap(toResponse(r.AssignmentAssignment), r.AssetName, r.AssetTag, r.EmployeeName, r.AssignedByName, r.OfficeName))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
```

> Note: `audit.Record(..., officeID *uuid.UUID, ...)` — assignments have no direct office column, so pass `nil` for check-out/check-in (mirrors how the audit office arg is nullable). Verify `audit.Record`/`audit.Diff`/`audit.ActionCreate` signatures against `internal/audit` (already used by transfer). Confirm the enriched row field names (`r.AssetName`, `r.AssetTag`, `r.EmployeeName`, `r.AssignedByName`, `r.OfficeName`, `r.AssignmentAssignment`) against the generated `GetAssignmentEnrichedRow`/`ListAssignmentsEnrichedRow` — sqlc names embedded structs by the table's Go type and aliased columns by their alias in PascalCase.

- [ ] **Step 4: Build**

Run: `cd backend && go build ./internal/assignment/`
Expected: builds clean. Fix any field-name mismatches flagged by the compiler against the generated types (see notes above).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/assignment/executor.go backend/internal/assignment/handler.go backend/internal/assignment/routes.go
git commit -m "feat(assignment): executor + handler + routes"
```

---

## Task 5: Wire the module into the router

**Files:**
- Modify: `backend/internal/server/router.go`

**Interfaces:**
- Consumes: `assignment.NewService`, `assignment.NewHandler`, `assignment.RegisterRoutes`, `(*assignment.Service).Executor()`.

- [ ] **Step 1: Add the import**

In `backend/internal/server/router.go`, add to the import block (alphabetical, near `internal/approval`):
```go
	"github.com/ragbuaj/inventra/internal/assignment"
```

- [ ] **Step 2: Construct + register the module**

After the disposal/transfer wiring block (right after the `stockopname` block around line 197-203, but the executor must be registered before `approvalHandler` is built at line 180 — so place the service construction + `RegisterExecutor` next to the other executors, and the handler/routes after). Concretely:

Immediately after line 179 (`approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetTransfer, transferSvc.Executor())`), add:
```go
		assignmentSvc := assignment.NewService(queries, d.Pool, approvalSvc)
		approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssignment, assignmentSvc.Executor())
```

Then, after the stockopname `RegisterRoutes` block (around line 203), add:
```go
		assignmentHandler := assignment.NewHandler(assignmentSvc, scopeSvc, queries, auditSvc)
		assignment.RegisterRoutes(api, assignmentHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "assignment.manage"),
			middleware.RequirePermission(permSvc, "assignment.view"),
			middleware.RequirePermission(permSvc, "request.create"),
		)
```

- [ ] **Step 3: Build + vet**

Run: `cd backend && go build ./... && go vet ./...`
Expected: builds clean, vet passes.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/server/router.go
git commit -m "feat(assignment): wire module + register executor in router"
```

---

## Task 6: Approval "mine" filter (Pengajuan Saya)

**Files:**
- Modify: `backend/db/queries/approval.sql`, `backend/internal/approval/service.go`, `backend/internal/approval/handler.go`
- (regenerate) `backend/db/sqlc/`

**Interfaces:**
- Consumes: existing `ListRequestsEnriched`/`CountRequests`.
- Produces: `(*approval.Service).List(ctx, all, ids, status, typ, limit, offset, requestedBy *uuid.UUID)` (added trailing param); `GET /requests?mine=true` returns only the caller's own requests.

- [ ] **Step 1: Add the `requested_by` narg to the two queries**

In `backend/db/queries/approval.sql`, find `ListRequestsEnriched` and `CountRequests`. Add a nil-safe filter to each WHERE clause:
```sql
  AND (sqlc.narg(requested_by)::uuid IS NULL OR requested_by_id = sqlc.narg(requested_by))
```
Add it after the existing `type` filter line in **both** `ListRequestsEnriched` and `CountRequests`. (Use the correct table alias if the query aliases `approval.requests` — match the existing `status`/`type` filter's alias in each query.)

- [ ] **Step 2: Regenerate + note the new param field**

Run: `cd backend && sqlc generate`
Expected: `ListRequestsEnrichedParams` and `CountRequestsParams` gain a `RequestedBy *uuid.UUID` field.

- [ ] **Step 3: Thread it through the service `List`**

In `backend/internal/approval/service.go`, change the `List` signature and pass the param. Replace the `List` method's signature line:
```go
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, status, typ string, limit, offset int32) ([]sqlc.ListRequestsEnrichedRow, int64, error) {
```
with:
```go
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, status, typ string, limit, offset int32, requestedBy *uuid.UUID) ([]sqlc.ListRequestsEnrichedRow, int64, error) {
```
and add `RequestedBy: requestedBy,` to **both** the `ListRequestsEnrichedParams` and `CountRequestsParams` literals inside it.

- [ ] **Step 4: Add the `mine` branch in the handler**

In `backend/internal/approval/handler.go`, in `list`, replace:
```go
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests")
	if err != nil {
		common.WriteError(c, err)
		return
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<30)
	rows, total, err := h.svc.List(c, all, ids, c.Query("status"), c.Query("type"), limit, offset)
```
with:
```go
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests")
	if err != nil {
		common.WriteError(c, err)
		return
	}
	var requestedBy *uuid.UUID
	if c.Query("mine") == "true" {
		// Own submitted requests: filter by requester and bypass office scope
		// (a caller can always see their own requests regardless of scope config).
		if uid, perr := uuid.Parse(c.GetString(middleware.CtxUserID)); perr == nil {
			requestedBy = &uid
			all, ids = true, nil
		}
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<30)
	rows, total, err := h.svc.List(c, all, ids, c.Query("status"), c.Query("type"), limit, offset, requestedBy)
```
(`middleware` is already imported in this file.)

- [ ] **Step 5: Fix other callers + build**

Run: `cd backend && go build ./... 2>&1 | head`
Expected: the only compile error (if any) is other callers of `svc.List`. Search and fix:
Run: `cd backend && grep -rn "\.List(c" internal/approval/ ; grep -rn "svc.List(" internal/`
For any non-handler caller (e.g. approval integration tests), append `, nil` as the final arg. Rebuild until clean.

- [ ] **Step 6: Commit**

```bash
git add backend/db/queries/approval.sql backend/db/sqlc/ backend/internal/approval/service.go backend/internal/approval/handler.go
git commit -m "feat(approval): add mine=true requester filter to GET /requests"
```

---

## Task 7: Integration tests (assignment module)

**Files:**
- Create: `backend/internal/assignment/assignment_integration_test.go`

**Interfaces:**
- Consumes: `internal/testsupport` (Postgres/Redis containers, migrate, `Reset`, seed helpers) — inspect `internal/transfer/transfer_integration_test.go` for the exact setup helpers (container bootstrap, seeded roles/offices/assets, how a `Caller` and scope are built). Reuse the same helpers.

- [ ] **Step 1: Write the integration tests**

Create `backend/internal/assignment/assignment_integration_test.go` with `//go:build integration` at the top. Mirror the transfer integration test's harness (same package-level container setup, seed helpers for office/category/asset/user/employee). Cover these cases (write each as a `t.Run` subtest with real assertions):

```go
//go:build integration

package assignment_test

// Use the same testsupport bootstrap as internal/transfer/transfer_integration_test.go.
// Each subtest resets the DB (testsupport.Reset) and seeds: an office O, an available
// asset A in O, a Manager user M (assignment.manage + office scope over O), a Staf user
// S (request.create, linked to an employee E in O), and an approver user P eligible for
// the office-level assignment step (assignment step required_level='office', so an office
// manager in O with request.decide, distinct from S).
```

Required subtests and assertions:
1. **Checkout_flips_asset_to_assigned**: `Checkout(all,ids,M, {A, E, "2026-07-06"})` → assignment `status=active`; `GetAsset(A).Status == assigned`.
2. **Checkout_rejects_unavailable_asset**: set A to `assigned` (checkout once), second `Checkout` for A → `ErrAssetNotAvailable`.
3. **Checkout_unique_active_per_asset**: after A already active, a direct insert path via `Checkout` returns `ErrAssetNotAvailable` (the availability guard fires before the unique index; also assert only one active row exists via `List` status=active).
4. **Checkout_out_of_scope**: Manager whose scope excludes O → `Checkout` → `ErrOutOfScope`.
5. **Checkin_returns_and_frees_asset**: after checkout, `Checkin(all,ids,id,{NeedsMaintenance:false})` → assignment `status=returned`, `checkin_date` set; `GetAsset(A).Status == available`.
6. **Checkin_needs_maintenance_sets_under_maintenance**: `Checkin(...,{NeedsMaintenance:true})` → `GetAsset(A).Status == under_maintenance`.
7. **Checkin_rejects_non_active**: check-in an already-returned assignment → `ErrNotActive`.
8. **Borrow_then_approve_creates_assignment**: `SubmitBorrow(callerS, {A, due, notes})` → pending `assignment` request; approve it as P via `approvalSvc.Decide(req.ID, callerP, true, nil)` → assignment row exists for A with `employee_id == E`, `assigned_by_id == P`, and `GetAsset(A).Status == assigned`.
9. **Borrow_reject_leaves_no_assignment**: submit + `Decide(..., false, nil)` → request `rejected`; no assignment row for A; A still `available`.
10. **Borrow_executor_rejects_unavailable_at_approval**: submit borrow for A; before approving, check out A directly to someone else (A→assigned); approving the borrow → `Decide` returns an error (executor `ErrConflict`) and the request is not approved; A stays as the direct assignee.
11. **List_scoped**: an assignment in office O is visible to a Manager scoped to O and invisible to a Manager scoped only to a sibling office.
12. **ListByAsset_history**: two sequential checkout→checkin cycles on A → `ListByAsset(A)` returns 2 rows newest-first.

Each subtest asserts concrete values (status strings, IDs, counts), never `len > 0` alone.

- [ ] **Step 2: Run the integration tests**

Run: `cd backend && go test -tags=integration ./internal/assignment/ -v`
Expected: all subtests PASS. (Docker must be running for testcontainers.)

- [ ] **Step 3: Commit**

```bash
git add backend/internal/assignment/assignment_integration_test.go
git commit -m "test(assignment): integration coverage for checkout/checkin/borrow"
```

---

## Task 8: OpenAPI + backend gate

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add the Assignment schema + paths**

In `backend/api/openapi.yaml`, add an `Assignment` schema (fields from `toResponse` + the enriched name fields) and document the endpoints: `GET /assignments` (query: `status`, `employee_id`, `search`, `limit`, `offset`), `GET /assignments/available`, `GET /assignments/{id}`, `POST /assignments` (CheckoutRequest), `POST /assignments/borrow` (BorrowRequest), `POST /assignments/{id}/checkin` (CheckinRequest), `GET /assets/{id}/assignments`. Add the `mine` boolean query param to the existing `GET /requests`. Mirror the shape/style of the existing `Transfer` schema + `/transfers` paths already in the file.

- [ ] **Step 2: Lint the spec**

Run: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors (pre-existing `AssetCreatePayload` unused-component warning may persist — unrelated).

- [ ] **Step 3: Full backend gate**

Run: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./...`
Expected: all green. (Full `-tags=integration ./...` is the shared-signature gate — Task 6 changed `approval.Service.List`.)

- [ ] **Step 4: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(assignment): OpenAPI for assignment endpoints + requests mine param"
```

---

## Task 9: Frontend composable + meta constants

**Files:**
- Modify: `frontend/app/composables/api/useAssignment.ts`
- Create: `frontend/app/constants/assignmentMeta.ts`, `frontend/test/unit/assignment-meta.spec.ts`

**Interfaces:**
- Consumes: `useApiClient().request` (see `useTransfers.ts`).
- Produces:
  - `useAssignment()` → `{ list, available, checkout, checkin, borrow, myRequests, cancel }`
  - Types `Assignment`, `AssignmentListPage`, `CheckoutInput`, `CheckinInput`, `BorrowInput`, `AvailableAsset`
  - `assignmentMeta`: `ASSIGNMENT_STATUS_TONE`, `REQUEST_STATUS_TONE`, `CONDITION_TONE`, `CONDITION_KEYS`, `formatDateID(iso)`

- [ ] **Step 1: Write the meta constants**

`frontend/app/constants/assignmentMeta.ts`:
```ts
import type { BadgeColor } from '~/types'

export type AssignmentStatus = 'active' | 'returned'
export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'
export type AssetCondition = 'baik' | 'ringan' | 'berat'

export const ASSIGNMENT_STATUS_TONE: Record<AssignmentStatus, BadgeColor> = {
  active: 'info',
  returned: 'neutral'
}

export const REQUEST_STATUS_TONE: Record<RequestStatus, BadgeColor> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
  cancelled: 'neutral'
}

export const CONDITION_TONE: Record<AssetCondition, BadgeColor> = {
  baik: 'success',
  ringan: 'warning',
  berat: 'error'
}

export const CONDITION_KEYS: AssetCondition[] = ['baik', 'ringan', 'berat']

const MONTHS_ID = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

/** Formats an ISO date/datetime string as "6 Jul 2026"; empty input → "". */
export function formatDateID(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getDate()} ${MONTHS_ID[d.getMonth()]} ${d.getFullYear()}`
}
```

- [ ] **Step 2: Write the meta unit test**

`frontend/test/unit/assignment-meta.spec.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { ASSIGNMENT_STATUS_TONE, REQUEST_STATUS_TONE, CONDITION_TONE, formatDateID } from '~/constants/assignmentMeta'

describe('assignmentMeta', () => {
  it('maps assignment status tones', () => {
    expect(ASSIGNMENT_STATUS_TONE.active).toBe('info')
    expect(ASSIGNMENT_STATUS_TONE.returned).toBe('neutral')
  })

  it('maps request status tones', () => {
    expect(REQUEST_STATUS_TONE.pending).toBe('warning')
    expect(REQUEST_STATUS_TONE.approved).toBe('success')
    expect(REQUEST_STATUS_TONE.rejected).toBe('error')
    expect(REQUEST_STATUS_TONE.cancelled).toBe('neutral')
  })

  it('maps condition tones', () => {
    expect(CONDITION_TONE.baik).toBe('success')
    expect(CONDITION_TONE.berat).toBe('error')
  })

  it('formats ISO dates in Indonesian short form', () => {
    expect(formatDateID('2026-07-06T00:00:00Z')).toMatch(/^6 Jul 2026$/)
    expect(formatDateID('')).toBe('')
    expect(formatDateID(null)).toBe('')
    expect(formatDateID('not-a-date')).toBe('')
  })
})
```

- [ ] **Step 3: Rewrite the composable**

`frontend/app/composables/api/useAssignment.ts`:
```ts
import type { AssignmentStatus, AssetCondition } from '~/constants/assignmentMeta'

export interface Assignment {
  id: string
  asset_id: string
  employee_id: string
  assigned_by_id: string
  checkout_date: string | null
  due_date: string | null
  checkin_date: string | null
  condition_out: string | null
  condition_in: string | null
  status: AssignmentStatus
  notes: string | null
  asset_name: string | null
  asset_tag: string | null
  employee_name: string | null
  assigned_by_name: string | null
  office_name: string | null
  created_at: string | null
  updated_at: string | null
}

export interface AvailableAsset {
  id: string
  asset_tag: string
  name: string
}

export interface AssignmentListPage {
  data: Assignment[]
  total: number
  limit: number
  offset: number
}

export interface CheckoutInput {
  asset_id: string
  employee_id: string
  checkout_date: string
  due_date?: string | null
  condition_out?: string | null
  notes?: string | null
}

export interface CheckinInput {
  checkin_date?: string | null
  condition_in?: AssetCondition | null
  needs_maintenance?: boolean
}

export interface BorrowInput {
  asset_id: string
  due_date?: string | null
  condition_out?: AssetCondition | null
  notes?: string | null
}

export interface SubmitResponse {
  request_id: string
  status: string
}

/** Asset assignment (penugasan) + Staf borrow (peminjaman), wired to /api/v1. */
export function useAssignment() {
  const { request } = useApiClient()

  async function list(q?: { status?: string, employee_id?: string, search?: string, limit?: number, offset?: number }): Promise<AssignmentListPage> {
    const query: Record<string, string | number> = {}
    if (q?.status) query.status = q.status
    if (q?.employee_id) query.employee_id = q.employee_id
    if (q?.search) query.search = q.search
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<AssignmentListPage>('/assignments', { query })
  }

  async function available(): Promise<{ data: AvailableAsset[] }> {
    return request<{ data: AvailableAsset[] }>('/assignments/available')
  }

  async function checkout(input: CheckoutInput): Promise<Assignment> {
    return request<Assignment>('/assignments', { method: 'POST', body: input })
  }

  async function checkin(id: string, input: CheckinInput): Promise<Assignment> {
    return request<Assignment>(`/assignments/${id}/checkin`, { method: 'POST', body: input })
  }

  async function borrow(input: BorrowInput): Promise<SubmitResponse> {
    return request<SubmitResponse>('/assignments/borrow', { method: 'POST', body: input })
  }

  // My submitted borrow requests (assignment type), for the "Pengajuan Saya" list.
  async function myRequests(q?: { status?: string, limit?: number, offset?: number }): Promise<{ data: Record<string, unknown>[], total: number }> {
    const query: Record<string, string | number> = { mine: 'true', type: 'assignment' }
    if (q?.status) query.status = q.status
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<{ data: Record<string, unknown>[], total: number }>('/requests', { query })
  }

  async function cancel(id: string): Promise<Record<string, unknown>> {
    return request<Record<string, unknown>>(`/requests/${id}/cancel`, { method: 'POST' })
  }

  return { list, available, checkout, checkin, borrow, myRequests, cancel }
}
```

- [ ] **Step 4: Run the meta unit test + typecheck**

Run: `cd frontend && pnpm test -- assignment-meta && pnpm typecheck`
Expected: meta test PASS; typecheck passes (the old `assignment.vue`/`mock/assignment.ts` may still reference removed exports — those are fixed in Task 11/15; if typecheck fails only on those files, proceed and fix them in their task, or run typecheck at the end).

- [ ] **Step 5: Commit**

```bash
git add frontend/app/constants/assignmentMeta.ts frontend/app/composables/api/useAssignment.ts frontend/test/unit/assignment-meta.spec.ts
git commit -m "feat(assignment): real useAssignment composable + meta constants"
```

---

## Task 10: Nav items

**Files:**
- Modify: `frontend/app/utils/nav.ts`, `frontend/test/unit/nav-model.spec.ts`

- [ ] **Step 1: Gate the Manager assignment item + add the Staf peminjaman item**

In `superadminNav`, change the `nav.assignment` item to add a permission gate:
```ts
      {
        labelKey: 'nav.assignment',
        icon: 'i-lucide-clipboard-check',
        to: '/assignment',
        permission: 'assignment.manage'
      },
```

In `staffNav`, replace the disabled `nav.assignment` item with an enabled `nav.peminjaman` item:
```ts
      {
        labelKey: 'nav.peminjaman',
        icon: 'i-lucide-hand',
        to: '/peminjaman',
        permission: 'request.create'
      },
```

- [ ] **Step 2: Update the nav model test**

In `frontend/test/unit/nav-model.spec.ts`, add/adjust assertions: the superadmin `assignment` item now carries `permission: 'assignment.manage'`; `staffNav` contains a `nav.peminjaman` item with `to: '/peminjaman'` and `permission: 'request.create'`. Replace any stale assertion about a disabled staff assignment item.

- [ ] **Step 3: Run the nav test**

Run: `cd frontend && pnpm test -- nav-model`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/utils/nav.ts frontend/test/unit/nav-model.spec.ts
git commit -m "feat(assignment): nav items (Penugasan gate + Peminjaman staff item)"
```

---

## Task 11: Wire the Manager Penugasan screen

**Files:**
- Modify: `frontend/app/pages/assignment.vue`
- Create: `frontend/test/nuxt/assignment.spec.ts`

**Interfaces:**
- Consumes: `useAssignment()` (Task 9), `useEmployees()` (existing — `GET /employees`), `assignmentMeta`.

- [ ] **Step 1: Rewrite `assignment.vue` to real data**

Rewrite `frontend/app/pages/assignment.vue` keeping the exact template structure/classes from the current mock version (3 tabs: Check-out / Check-in / Riwayat — it is already 1:1 with `docs/design/Penugasan Aset.dc.html`). Changes only to the data layer:
- `definePageMeta({ middleware: 'can', permission: 'assignment.manage' })` (was `masterdata.office.manage`).
- Replace `recipientSeed` with real employees: `const employees = await useEmployees().list({ limit: 100 })` → map to `{ value: id, label: name }` for the recipient `USelect`.
- Replace the mock `api.list()` with `useAssignment().list()`; map `Assignment` fields (`asset_name`/`asset_tag`/`employee_name`/`checkout_date`/`checkin_date`/`status`/`condition_out`) into the table (which previously used `nama`/`tag`/`pemegang`/`pinjam`/`kembali`/`kondisi`). Use `formatDateID` for dates and `ASSIGNMENT_STATUS_TONE`/`CONDITION_TONE` for badges.
- Available picker: `useAssignment().available()` → `{ label: `${name} · ${asset_tag}`, value: id }` (the check-out form now submits `asset_id`, not tag).
- `doCheckout`: call `useAssignment().checkout({ asset_id, employee_id, checkout_date, condition_out, notes })`.
- `doCheckin`: call `useAssignment().checkin(id, { checkin_date, condition_in, needs_maintenance })`.
- Condition selects bind to `AssetCondition` keys; send the key string as `condition_out`/`condition_in`.
- Add load-error + retry state around the initial `refresh()` (mirror the pattern in `pages/transfers.vue`).

- [ ] **Step 2: Write the component test**

`frontend/test/nuxt/assignment.spec.ts` (add `// @vitest-environment nuxt` at top). Mock `useAssignment` and `useEmployees` (see `test/nuxt/transfers.spec.ts` for the mocking pattern). Assert:
- Renders the 3 tabs; default tab is Check-out.
- Check-out form: submit disabled until asset + employee + date filled; enabling then clicking calls `checkout` with the chosen `asset_id`/`employee_id`.
- Available picker lists assets from `available()`.
- Check-in tab: empty state when no active assignments; with an active assignment, selecting it + a return date enables submit; `needs_maintenance` checkbox toggles; submit calls `checkin` with `needs_maintenance:true`.
- Riwayat tab: renders rows with resolved `asset_name`/`employee_name`, status badge text (Aktif/Dikembalikan), condition text; search + status filter narrow rows; empty state when none.
- Load-error state renders when `list()` rejects, and retry re-calls it.

- [ ] **Step 3: Run the test**

Run: `cd frontend && pnpm test -- assignment.spec`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/pages/assignment.vue frontend/test/nuxt/assignment.spec.ts
git commit -m "feat(assignment): wire Penugasan Manager screen to real API"
```

---

## Task 12: Ajukan Peminjaman modal + Staf Peminjaman page

**Files:**
- Create: `frontend/app/components/assignment/AjukanPeminjamanModal.vue`, `frontend/app/pages/peminjaman.vue`, `frontend/test/nuxt/peminjaman.spec.ts`, `frontend/test/nuxt/ajukan-peminjaman-modal.spec.ts`

**Interfaces:**
- Consumes: `useAssignment()` (`available`, `borrow`, `myRequests`, `cancel`), `assignmentMeta`.
- Produces: `<AjukanPeminjamanModal>` with props `{ asset?: { id, name, asset_tag, category?, office?, location? } | null }`, `v-model:open`, emits `submitted`.

- [ ] **Step 1: Build the modal component**

`frontend/app/components/assignment/AjukanPeminjamanModal.vue` — a `UModal` matching `docs/design/Modal Ajukan Peminjaman.dc.html`:
- Props: `open` (boolean, `v-model:open`), `asset` (object | null). When `asset` is provided (from Detail), show the locked read-only asset block (name, mono `asset_tag`, category, office, location, lock icon) and hide the asset picker. When `asset` is null (page usage), show a `USelectMenu` asset picker fed by `useAssignment().available()`.
- Fields: Jatuh Tempo (`UInput type="date"`, optional, hint "Boleh dikosongkan bila belum pasti."), Alasan/Keperluan (`UTextarea`, required).
- Green info banner: `t('peminjaman.infoBanner')`.
- Footer: Batal (neutral) + Kirim Pengajuan (primary, loading during submit).
- On submit: call `borrow({ asset_id, due_date, notes })`; on success emit `submitted`, close, `useToast().add({ title: t('peminjaman.toast.sent') })`. Validate Alasan required (inline error).

- [ ] **Step 2: Build the Staf page**

`frontend/app/pages/peminjaman.vue` — 1:1 with `docs/design/Peminjaman Aset.dc.html`:
- `definePageMeta({ middleware: 'can', permission: 'request.create' })`.
- Header (`peminjaman.title` + `peminjaman.subtitle`).
- Card "Ajukan Peminjaman": inline form (reuse the modal's field layout via `<AjukanPeminjamanModal>` with `asset=null` rendered inline is awkward — instead render the form fields directly in the card and call `useAssignment().borrow()`; keep the modal component for the Detail-page usage). Asset picker from `available()`, Jatuh Tempo, Alasan, banner, Batal/Ajukan buttons.
- Section "Pengajuan Peminjaman Saya": status tabs (Menunggu/Disetujui/Ditolak/Semua → maps to `pending`/`approved`/`rejected`/`''`); table columns Aset (name+tag from `payload.asset_id` best-effort — the request row exposes `target_id`; resolve name lazily via `useAssets().get` is optional, else show tag/id), Diajukan (`created_at`), Jatuh Tempo (from `payload.due_date`), Status badge (`REQUEST_STATUS_TONE`), Catatan Keputusan (`decision_note`), Aksi (Batalkan when `status==='pending'` → `cancel(id)` then reload). Expandable row → timeline from `GET /requests/:id` steps (`useApproval().get`). Empty state + skeleton + total count. Data from `myRequests({ status })`.
- After a successful submit, reload the list.

> Asset-name resolution for the list rows: the `mine` request rows carry `target_id` (asset id) + `payload`. To show the asset name, fetch the request detail (`GET /requests/:id`) on row expand, or resolve `target_id` → name via `useAssets().get(tag)` is not possible by id. Simplest faithful approach: show the asset name from the request detail `payload` when a row is expanded, and in the collapsed row show `payload.asset_id` short + the reason. Document this as a minor deviation if the name is not shown collapsed. (The mockup shows the asset name; resolving it requires a per-id asset lookup endpoint — note as a follow-up if `GET /assets/:id` by id is not available; `GET /assets` list is scoped and a Staf may not see it.)

- [ ] **Step 3: Write the modal test**

`frontend/test/nuxt/ajukan-peminjaman-modal.spec.ts` (`// @vitest-environment nuxt`): mock `useAssignment`. Assert:
- With `asset` prop set: renders the locked asset block (name + tag), no asset picker.
- Alasan required: submit disabled/blocked with empty Alasan (inline error shown).
- Filling Alasan + clicking Kirim calls `borrow` with the asset's id and the typed notes; emits `submitted`.
- With `asset=null`: renders the asset picker from `available()`.

- [ ] **Step 4: Write the page test**

`frontend/test/nuxt/peminjaman.spec.ts` (`// @vitest-environment nuxt`): mock `useAssignment` + `useApproval`. Assert:
- Ajukan card: submit blocked until asset + Alasan present; success calls `borrow` and reloads `myRequests`; success toast.
- List: renders rows from `myRequests`; status tabs switch and re-query with the mapped status; status badges use the right tone/text (Menunggu/Disetujui/Ditolak).
- Batalkan action visible only for `pending` rows; clicking calls `cancel` then reloads.
- Empty state when `myRequests` returns `[]`; loading skeleton before resolve; error + retry when it rejects.
- Row expand fetches + renders the approval timeline.

- [ ] **Step 5: Run the tests**

Run: `cd frontend && pnpm test -- peminjaman ajukan-peminjaman-modal`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/components/assignment/AjukanPeminjamanModal.vue frontend/app/pages/peminjaman.vue frontend/test/nuxt/peminjaman.spec.ts frontend/test/nuxt/ajukan-peminjaman-modal.spec.ts
git commit -m "feat(assignment): Staf Peminjaman page + Ajukan Peminjaman modal"
```

---

## Task 13: Ajukan Peminjaman button on Asset Detail

**Files:**
- Modify: `frontend/app/pages/assets/[tag]/index.vue`
- Create: `frontend/test/nuxt/asset-detail-borrow.spec.ts`

**Interfaces:**
- Consumes: `<AjukanPeminjamanModal>` (Task 12), `useCan`.

- [ ] **Step 1: Add the button + modal**

In `frontend/app/pages/assets/[tag]/index.vue`, in the header action button row (where Edit / Cetak Label buttons live), add:
```vue
<UButton
  v-if="can('request.create')"
  icon="i-lucide-hand"
  :label="t('peminjaman.action.borrow')"
  :disabled="asset?.status !== 'available'"
  @click="borrowOpen = true"
/>
```
with a tooltip when disabled (wrap in `UTooltip` with `:text="t('peminjaman.action.borrowDisabled')"` shown when `asset?.status !== 'available'`). Add near the bottom of the template:
```vue
<AjukanPeminjamanModal
  v-model:open="borrowOpen"
  :asset="asset ? { id: asset.id, name: asset.name, asset_tag: asset.asset_tag, category: asset.category_name, office: asset.office_name, location: asset.room_name } : null"
  @submitted="onBorrowSubmitted"
/>
```
In `<script setup>`: `const { can } = useCan()`, `const borrowOpen = ref(false)`, and `function onBorrowSubmitted() { borrowOpen.value = false }` (optionally a toast link "Lihat di Peminjaman Saya" via `navigateTo('/peminjaman')`). Match the exact field names the detail page already uses for the loaded asset (`asset.id`, `asset.name`, `asset.asset_tag`, `asset.category_name`/`asset.office_name`/`asset.room_name` — adjust to the actual property names in this page's asset object).

- [ ] **Step 2: Write the test**

`frontend/test/nuxt/asset-detail-borrow.spec.ts` (`// @vitest-environment nuxt`): mount the detail page (or a minimal harness) with `useCan` granting `request.create`. Assert:
- Button renders and is enabled when the loaded asset status is `available`.
- Button is disabled when status is `assigned`/`under_maintenance` (tooltip text present).
- Clicking opens the modal (asset block shows the asset name/tag).
- With `request.create` denied, the button is absent.

> If mounting the full detail page is heavy, extract the header actions into the existing structure and test the button's `disabled`/visibility logic via a focused mount, mocking `useAssets` detail fetch. Follow the mounting approach already used in `test/nuxt` for detail-page specs.

- [ ] **Step 3: Run the test**

Run: `cd frontend && pnpm test -- asset-detail-borrow`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/pages/assets/[tag]/index.vue frontend/test/nuxt/asset-detail-borrow.spec.ts
git commit -m "feat(assignment): Ajukan Peminjaman button + modal on Asset Detail"
```

---

## Task 14: i18n + remove mock

**Files:**
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`
- Delete: `frontend/app/mock/assignment.ts` (only if no other consumer)

- [ ] **Step 1: Add/extend i18n keys**

Add a `nav.peminjaman` key (id: "Peminjaman Aset", en: "Asset Borrowing") to both locales. Extend `assignment.*` where the wired Manager screen needs new keys (status `active`/`returned` labels, condition labels — many already exist from the mock). Add a `peminjaman.*` namespace covering: `title`, `subtitle`, `ajukan.title`, `ajukan.subtitle`, form labels (`asset`, `assetHint`, `assetPlaceholder`, `dueDate`, `dueDateHint`, `reason`, `reasonPlaceholder`), `infoBanner`, `submit`, `reset`, `toast.sent`, list section (`myRequests`, `tabs.pending/approved/rejected/all`, `col.asset/submitted/dueDate/status/decisionNote/action`, `status.pending/approved/rejected/cancelled`, `cancel`, `empty`, `total`, `timeline.title`), and `action.borrow`/`action.borrowDisabled`. Use natural Indonesian; mirror wording from the two mockups (verified strings: "Ajukan Peminjaman", "Pilih aset tersedia dan sampaikan keperluan Anda.", "Hanya aset berstatus \"Tersedia\" yang dapat dipinjam.", "Jatuh Tempo / Rencana Kembali", "Boleh dikosongkan bila belum pasti.", "Alasan / Keperluan", "Peminjaman akan dikirim ke Manager untuk disetujui. Aset baru berpindah ke Anda setelah disetujui.", "Pengajuan Peminjaman Saya", "Kirim Pengajuan").

- [ ] **Step 2: Check the mock is unused, then delete it**

Run: `cd frontend && grep -rn "mock/assignment" app/ test/ e2e/`
Expected: only `useAssignment.ts` (now rewritten, no longer imports it) and its own tests. If `useGlobalSearch.ts` imports it, keep the file (mirror the retained-mock pattern) and skip deletion; otherwise delete `frontend/app/mock/assignment.ts`. Fix any remaining import.

- [ ] **Step 3: Lint + typecheck + full unit/component test**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test`
Expected: all green. Fix i18n-missing-key or lint (trailing comma / brace) issues.

- [ ] **Step 4: Commit**

```bash
git add frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/app/mock/assignment.ts frontend/app/composables/api/useAssignment.ts
git commit -m "feat(assignment): i18n id/en + remove assignment mock"
```

---

## Task 15: E2E + final gate + PROGRESS

**Files:**
- Create: `frontend/e2e/assignment.spec.ts`
- Modify: `docs/PROGRESS.md`

**Interfaces:**
- Consumes: real backend stack + seeded admin; the API-setup + maker≠checker approve pattern from `frontend/e2e/assets.spec.ts` / `transfers.spec.ts`.

- [ ] **Step 1: Write the e2e spec**

`frontend/e2e/assignment.spec.ts` — real-backend, unique-per-run data (mirror `assets.spec.ts` setup: create office/floor/room/category/asset via API; unique name+code per run per the persistent-data-uniqueness convention). Two flows:
- **Direct (Manager):** log in as the seeded admin (has `assignment.manage`); on `/assignment`, check out the seeded available asset to an employee → assert it appears in Riwayat as Aktif and the asset detail shows status Dipinjam/assigned; check it back in → assert Dikembalikan + asset available.
- **Peminjaman (Staf→approve):** submit a borrow via API as a Staf-type user (or via `/peminjaman` UI) → appears in "Pengajuan Saya" as Menunggu → approve via API as a second SoD-eligible user (maker ≠ checker) → assert an assignment now exists (asset assigned) and the request shows Disetujui.
- Negative: submit borrow with empty Alasan → validation blocks; Detail-page "Ajukan Peminjaman" button disabled when asset not available.

Follow the e2e conventions: unique name+code per run, assert-after-search, wait for modal-closed.

- [ ] **Step 2: Run the e2e spec (needs stack up + seeded admin + RATELIMIT_ENABLED=false)**

Run: `cd frontend && pnpm test:e2e -- assignment`
Expected: PASS (1 flow file). If the dev-DB office picker `limit:100` debris bites the UI setup, drive session/office setup via API (documented convention) and keep UI assertions on the assignment surfaces.

- [ ] **Step 3: Full frontend gate**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`: tick the **Assignment** checkbox under *Backend — Feature modules* (`- [ ]` → `- [x]`) with a one-line note + this branch; add an **Assignment (Penugasan/Peminjaman)** entry under the frontend *wired screens* list; add a new "▶ Next session" item 36 recording completion + the approved deviations (checkbox "perlu maintenance" status-only; Detail button visible to all `request.create` roles; asset-name resolution in "Pengajuan Saya" collapsed rows if deferred) and pointing at the next candidate (Maintenance / global search / Reporting). Refresh the "start here" pointer.

- [ ] **Step 5: Commit**

```bash
git add frontend/e2e/assignment.spec.ts docs/PROGRESS.md
git commit -m "test(assignment): real-backend e2e + update PROGRESS"
```

- [ ] **Step 6: Full-stack gate sweep (task-13)**

Run backend: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./...`
Run spectral: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Run frontend: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green. This is the merge gate.

---

## Self-Review notes (addressed inline)

- **Spec coverage:** bagian 1 migration/catalog → Task 1; bagian 1.2 queries → Task 2; bagian 1.3 service state machine + borrow → Task 3; bagian 1.4 executor/handler/routes → Task 4; bagian 1.5 wiring + `mine` → Tasks 5–6; OpenAPI → Task 8; bagian 2.1 Manager screen → Task 11; bagian 2.2 Staf page → Task 12; bagian 2.3 Detail modal → Tasks 12–13; bagian 2.4 meta/nav/i18n/cleanup → Tasks 9,10,14; bagian 2.5 tests → Tasks 7,9,11,12,13,15. All spec sections map to a task.
- **`mine` scope safety:** Task 6 bypasses office scope (`all=true, ids=nil`) when `mine=true`, so a Staf reliably sees only their own requests regardless of the `requests` scope policy — resolves the spec's "office-scope leaks others' requests" concern.
- **Borrow endpoint refinement:** the spec said "POST /requests type assignment"; the plan uses a dedicated `POST /assignments/borrow` (Task 4) that internally calls `appr.Submit`, because the generic `SubmitRequest.Type` binding excludes `assignment` and requires an `office_id` a Staf wouldn't supply — consistent with how transfer/disposal submit. Frontend `useAssignment.borrow` targets this endpoint. Recorded as an intentional refinement (not a scope change).
- **Available-picker scope trap:** a Staf's `own` scope makes `GET /assets` empty; the plan adds `GET /assignments/available` (office-scoped by the caller's own `office_id`) for the borrow picker (Tasks 4, 9, 12).
- **Type consistency:** `checkoutTx` shared by `Checkout` + executor; enum constants and generated param/row field names flagged for post-`sqlc generate` verification in Tasks 2–4 (`Lim`/`Off`/`OfficeIds`/`AllScope`/embedded row fields).
