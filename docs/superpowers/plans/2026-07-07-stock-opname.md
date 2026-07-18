# Stock Opname (Inventarisasi Fisik) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Stock Opname (physical inventory) feature end-to-end — backend module `internal/stockopname`, a `/stock-opname` frontend screen, and a Berita Acara PDF/Excel export — letting a scoped Manager run per-office count sessions, reconcile variances, and auto-generate disposal/transfer approval requests from findings.

**Architecture:** Modular monolith. New backend module `internal/stockopname` follows the ADR-0008 four-file split (`service.go`/`dto.go`/`handler.go`/`routes.go`) plus a focused `report.go`. It reuses the existing `disposal.Service.Submit` and `transfer.Service.Submit` to auto-generate approval requests (so **no new `request_type` enum and no new approval executor**). Scope is anchored through `stock_opname_sessions.office_id`. The session itself does **not** go through maker-checker — only its value-affecting follow-ups do. Frontend is a single Nuxt page wired via a `useStockOpname` composable through `useApiClient`.

**Tech Stack:** Go 1.25 + Gin + pgx/v5 + sqlc + golang-migrate; gofpdf + excelize for exports; Nuxt 4 SPA + @nuxt/ui (`U*`) + Vitest/@nuxt/test-utils + Playwright; PostgreSQL 16.

## Global Constraints

- Go module path: `github.com/ragbuaj/inventra`. Backend commands run from `backend/`.
- Money/numeric columns are Go `string` (sqlc override `pg_catalog.numeric → string`); parse only when computing.
- Every table has `created_at`/`updated_at`/`deleted_at`; all `UNIQUE` are partial `WHERE deleted_at IS NULL`; every table has a `BEFORE UPDATE` trigger calling `shared.set_updated_at()`. (The stockopname tables already exist with these — migration `000015`.)
- Enforce authorization on **read AND write**: `RequirePermission` for the action + office data-scope threaded through every query. `scopeModule` string must equal the seeded `data_scope_policies.module` value.
- Don't hand-edit `backend/db/sqlc/` (generated). Change `db/queries/*.sql` or migrations, then `sqlc generate`.
- Keep `backend/api/openapi.yaml` in sync (hand-written, Spectral-linted).
- List endpoints return `{data, total, limit, offset}` with `limit` clamped 1–100.
- Frontend: all HTTP goes through `useApiClient().request` (never `$fetch` a hardcoded URL); catch blocks stay empty (errors toasted centrally). Every user-facing string in `i18n/locales/{id,en}.json` (default locale `id`). Theme via semantic tokens/`U*` color props. ESLint: **no trailing commas**, 1tbs. `USelect` items need `value-key="value"` and cannot use empty-string values — use a `NONE = '__none__'` sentinel translated to `null`/`undefined` at the API boundary.
- Build exactly what `docs/design/Stock Opname.dc.html` shows (1:1, light + dark). Only the approved deviations in the spec (bagian 7) are allowed; any new deviation needs user approval first, then gets recorded in the spec + PROGRESS.md.
- No `Co-Authored-By`/AI attribution in commits.
- Verification gates (must be green before "done"): `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./...`, Spectral lint, `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`, and the e2e spec.

**Reference spec:** `docs/superpowers/specs/2026-07-07-stock-opname-module-design.md`. **Mockup:** `docs/design/Stock Opname.dc.html`.

---

## Design facts locked from the codebase (read before starting)

- `disposal.SubmitInput{ AssetID uuid.UUID; Method string; DisposalDate string; Proceeds *string; BookValue *string; BastNo *string; Reason *string }` → `disposal.Service.Submit(ctx, caller approval.Caller, in disposal.SubmitInput) (sqlc.ApprovalRequest, error)`. `Method="write_off"`; leave `Proceeds`/`BookValue` nil (server computes book value via depreciation). Returns request with `.ID`, `.Status`, `.OfficeID`.
- `transfer.SubmitInput{ AssetID uuid.UUID; ToOfficeID uuid.UUID; ToRoomID *uuid.UUID; Reason *string; ConditionSent *string; TransferDate *string }` → `transfer.Service.Submit(ctx, caller approval.Caller, in transfer.SubmitInput) (sqlc.ApprovalRequest, error)`.
- `approval.Caller{ UserID, RoleID uuid.UUID; AllScope bool; OfficeIDs []uuid.UUID }`.
- `common.ScopedDeps{ Q *sqlc.Queries; Scope *authz.ScopeService }`; `(d ScopedDeps) CallerOfficeScope(c *gin.Context, module string) (bool, []uuid.UUID, error)`; `common.InScope(all bool, ids []uuid.UUID, target uuid.UUID) bool`; `common.WriteError(c, err)`; `common.UUIDPtrStr`, `common.TsStr`, `common.DateStr`, `common.ParseUUIDPtr`.
- Identity from Gin ctx: `c.GetString(middleware.CtxUserID)`, `c.GetString(middleware.CtxRoleID)` (set by `middleware.RequireAuth`).
- `RegisterRoutes(rg, h, authMW, requireManage, requireView gin.HandlerFunc)`; middleware order `authMW, require<Perm>, handler`.
- sqlc scope filter idiom: `AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]))`. Nullable args = `sqlc.narg`. `sqlc.embed(alias)` for joined structs.
- `clampInt(raw string, def, min, max int32) int32` is copied per-package (not shared) — duplicate it in `handler.go`.
- Router shared vars in `NewRouter` scope: `queries`, `d.Pool`, `permSvc`, `scopeSvc`, `fieldSvc`, `auditSvc`, `assetSvc`, `approvalSvc`, `depreciationSvc`, `disposalSvc`, `transferSvc`, `requireAuth`, `middleware.RequirePermission`.
- Frontend: `useApiClient().request<T>(path, opts)`; `Paginated<T> = { data, total, limit, offset }`; `BadgeColor = 'primary'|'success'|'warning'|'error'|'neutral'|'info'` (`~/types`). Nav is `superadminNav` in `frontend/app/utils/nav.ts`; `NavItem = { labelKey, icon?, to?, permission?, badgeCount?, disabled?, children? }`.

---

## Task 1: Migration `000025_stockopname_followup`

**Files:**
- Create: `backend/db/migrations/000025_stockopname_followup.up.sql`
- Create: `backend/db/migrations/000025_stockopname_followup.down.sql`

**Interfaces:**
- Produces: column `stockopname.stock_opname_items.followup_request_id uuid` (nullable FK → `approval.requests(id)`); permission keys `stockopname.view`/`stockopname.manage` in `identity.role_permissions`; `data_scope_policies` rows for module `'stockopname'`.

- [ ] **Step 1: Write the up migration**

`backend/db/migrations/000025_stockopname_followup.up.sql`:
```sql
-- Traceability: link a variance item to the approval request generated from it.
ALTER TABLE stockopname.stock_opname_items
  ADD COLUMN followup_request_id uuid REFERENCES approval.requests (id);
CREATE INDEX idx_opnitem_followup ON stockopname.stock_opname_items (followup_request_id);

-- Permissions: stockopname.manage (create/count/reconcile/close/follow-up) + stockopname.view (read).
-- Operational roles get both; Staf gets neither (PRD bagian 2.1 "Kelola stock opname").
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('stockopname.manage'), ('stockopname.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Data-scope for the 'stockopname' module (mirror 000023 depreciation pattern).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'stockopname', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Write the down migration**

`backend/db/migrations/000025_stockopname_followup.down.sql`:
```sql
DELETE FROM identity.data_scope_policies WHERE module = 'stockopname';
DELETE FROM identity.role_permissions WHERE permission_key IN ('stockopname.view', 'stockopname.manage');
DROP INDEX IF EXISTS stockopname.idx_opnitem_followup;
ALTER TABLE stockopname.stock_opname_items DROP COLUMN IF EXISTS followup_request_id;
```

- [ ] **Step 3: Apply up, then down, then up again to verify reversibility**

Run (from `backend/`, dev Postgres on :5433):
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
migrate -path db/migrations -database "$DATABASE_URL" down 1
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: all three succeed with no error; final state has the column + seeds present.

- [ ] **Step 4: Verify the column and seeds exist**

Run:
```bash
psql "$DATABASE_URL" -c "\d stockopname.stock_opname_items" -c "SELECT permission_key FROM identity.role_permissions WHERE permission_key LIKE 'stockopname%' GROUP BY permission_key;" -c "SELECT DISTINCT module FROM identity.data_scope_policies WHERE module='stockopname';"
```
Expected: `followup_request_id` listed; both permission keys returned; module `stockopname` returned.

- [ ] **Step 5: Commit**

```bash
git add backend/db/migrations/000025_stockopname_followup.up.sql backend/db/migrations/000025_stockopname_followup.down.sql
git commit -m "feat(db): stock opname follow-up link + permissions + data-scope (000025)"
```

---

## Task 2: sqlc queries for stockopname

**Files:**
- Create: `backend/db/queries/stockopname.sql`
- Modify (generated): `backend/db/sqlc/*` via `sqlc generate`

**Interfaces:**
- Produces (generated `sqlc.Queries` methods): `CreateOpnameSession`, `SnapshotSessionItems`, `GetOpnameSession`, `ListOpnameSessions`, `CountOpnameSessions`, `SetSessionStatus`, `SessionKpis`, `ListOpnameItemsEnriched`, `GetOpnameItem`, `SetOpnameItemResult`, `SetItemFollowup`, `GetOpnameItemByTag`, `InsertUnexpectedItem`, `ListAssetsForSnapshot` (or a single snapshot insert), `SessionVariance`.

- [ ] **Step 1: Write the query file**

`backend/db/queries/stockopname.sql` — scope is anchored through `stock_opname_sessions.office_id` for sessions and via a JOIN to the session for items:
```sql
-- name: CreateOpnameSession :one
INSERT INTO stockopname.stock_opname_sessions (office_id, name, period, started_by_id)
VALUES (sqlc.arg(office_id), sqlc.narg(name), sqlc.arg(period), sqlc.arg(started_by_id))
RETURNING *;

-- name: SnapshotSessionItems :exec
INSERT INTO stockopname.stock_opname_items (session_id, asset_id, expected, result)
SELECT sqlc.arg(session_id), a.id, true, 'pending'
FROM asset.assets a
WHERE a.office_id = sqlc.arg(office_id)
  AND a.status <> 'disposed'
  AND a.deleted_at IS NULL;

-- name: GetOpnameSession :one
SELECT sqlc.embed(s), o.name AS office_name,
       su.name AS started_by_name, cu.name AS closed_by_name
FROM stockopname.stock_opname_sessions s
LEFT JOIN masterdata.offices o ON o.id = s.office_id
LEFT JOIN identity.users su ON su.id = s.started_by_id
LEFT JOIN identity.users cu ON cu.id = s.closed_by_id
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListOpnameSessions :many
SELECT sqlc.embed(s), o.name AS office_name
FROM stockopname.stock_opname_sessions s
LEFT JOIN masterdata.offices o ON o.id = s.office_id
WHERE s.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.opname_session_status IS NULL OR s.status = sqlc.narg(status))
ORDER BY s.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountOpnameSessions :one
SELECT count(*)
FROM stockopname.stock_opname_sessions s
WHERE s.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.opname_session_status IS NULL OR s.status = sqlc.narg(status));

-- name: SetSessionStatus :one
UPDATE stockopname.stock_opname_sessions
SET status = sqlc.arg(status),
    closed_by_id = sqlc.narg(closed_by_id),
    closed_at = sqlc.narg(closed_at)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SessionKpis :one
SELECT
  count(*)::bigint AS total,
  count(*) FILTER (WHERE result = 'found')::bigint AS found,
  count(*) FILTER (WHERE result = 'pending')::bigint AS pending,
  count(*) FILTER (WHERE result IN ('not_found','damaged','misplaced'))::bigint AS variance
FROM stockopname.stock_opname_items
WHERE session_id = sqlc.arg(session_id) AND deleted_at IS NULL;

-- name: ListOpnameItemsEnriched :many
SELECT sqlc.embed(it), a.name AS asset_name, a.asset_tag AS asset_tag,
       o.name AS office_name, rm.name AS room_name, fl.name AS floor_name,
       cu.name AS counted_by_name
FROM stockopname.stock_opname_items it
LEFT JOIN asset.assets a ON a.id = it.asset_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id
LEFT JOIN masterdata.rooms rm ON rm.id = a.room_id
LEFT JOIN masterdata.floors fl ON fl.id = rm.floor_id
LEFT JOIN identity.users cu ON cu.id = it.counted_by_id
WHERE it.session_id = sqlc.arg(session_id) AND it.deleted_at IS NULL
  AND (sqlc.narg(result)::shared.opname_item_result IS NULL OR it.result = sqlc.narg(result))
ORDER BY a.name;

-- name: GetOpnameItem :one
SELECT * FROM stockopname.stock_opname_items
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id) AND deleted_at IS NULL;

-- name: SetOpnameItemResult :one
UPDATE stockopname.stock_opname_items
SET result = sqlc.arg(result), note = sqlc.narg(note),
    counted_by_id = sqlc.arg(counted_by_id), counted_at = now()
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id) AND deleted_at IS NULL
RETURNING *;

-- name: SetItemFollowup :one
UPDATE stockopname.stock_opname_items
SET followup_request_id = sqlc.arg(followup_request_id)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id) AND deleted_at IS NULL
RETURNING *;

-- name: GetOpnameItemByTag :one
SELECT it.* FROM stockopname.stock_opname_items it
JOIN asset.assets a ON a.id = it.asset_id
WHERE it.session_id = sqlc.arg(session_id) AND it.deleted_at IS NULL
  AND a.asset_tag = sqlc.arg(asset_tag);

-- (scan's asset-by-tag lookup reuses the EXISTING assets.sql `GetAssetByTag`;
--  do NOT add a duplicate here — scope is enforced in the service via common.InScope)

-- name: InsertUnexpectedItem :one
INSERT INTO stockopname.stock_opname_items (session_id, asset_id, expected, result)
VALUES (sqlc.arg(session_id), sqlc.arg(asset_id), false, 'pending')
ON CONFLICT (session_id, asset_id) WHERE deleted_at IS NULL DO NOTHING
RETURNING *;
```

- [ ] **Step 2: Regenerate sqlc**

Run (from `backend/`):
```bash
sqlc generate
```
Expected: no error; new methods appear in `backend/db/sqlc/`.

- [ ] **Step 3: Verify it compiles**

Run:
```bash
go build ./...
```
Expected: success (generated code compiles).

- [ ] **Step 4: Commit**

```bash
git add backend/db/queries/stockopname.sql backend/db/sqlc/
git commit -m "feat(db): stock opname sqlc queries"
```

---

## Task 3: Service — session lifecycle, snapshot, KPI, scoped list/get

**Files:**
- Create: `backend/internal/stockopname/service.go`
- Test: `backend/internal/stockopname/stockopname_integration_test.go`

**Interfaces:**
- Produces:
  - `type Service struct` + `func NewService(q *sqlc.Queries, pool *pgxpool.Pool, disp *disposal.Service, tr *transfer.Service) *Service`.
  - Sentinels: `ErrNotFound`, `ErrOutOfScope`, `ErrInvalidState`, `ErrInvalidRef`, `ErrAlreadyFollowedUp`, `ErrNoItem`.
  - `func mapDBError(err error) error`.
  - `type CreateInput struct { OfficeID uuid.UUID; Name *string; Period time.Time }`.
  - `CreateSession(ctx, caller approval.Caller, in CreateInput) (sqlc.StockopnameStockOpnameSession, error)` — inserts session (`open`) then snapshots items; scope-checked via `common.InScope`.
  - `GetSession(ctx, caller, id uuid.UUID) (session row, kpi struct, error)`.
  - `ListSessions(ctx, caller, status *string, limit, offset int32) (rows, total int64, error)`.
  - `Transition(ctx, caller, id uuid.UUID, to sqlc.SharedOpnameSessionStatus) (session, error)` — enforces the legal state graph; `close` stamps `closed_by/at`.
- Consumes: Task 2 sqlc methods; `disposal`/`transfer` services (stored for Tasks 4–5).

- [ ] **Step 1: Write failing integration test for create+snapshot and scope**

`backend/internal/stockopname/stockopname_integration_test.go` (header `//go:build integration`). Use the existing `internal/testsupport` harness (mirror `transfer_integration_test.go` setup — container, migrate, seed). Test cases:
```go
//go:build integration

package stockopname_test

// Uses testsupport to bring up Postgres, migrate, seed roles + an office tree + assets.
// See internal/transfer/transfer_integration_test.go for the exact harness bootstrapping.

func TestCreateSessionSnapshotsInScopeAssets(t *testing.T) {
	// Arrange: office A with 3 non-deleted, non-disposed assets + 1 disposed asset.
	// caller scoped to office A.
	// Act: svc.CreateSession(ctx, caller, CreateInput{OfficeID: officeA, Period: month})
	// Assert: session.Status == "open"; ListItems returns exactly 3 items, all result="pending", expected=true;
	//         the disposed asset is NOT snapshotted.
}

func TestCreateSessionOutOfScopeRejected(t *testing.T) {
	// caller scoped only to office B; CreateInput.OfficeID = office A → ErrOutOfScope.
}

func TestSessionStateMachineLegalAndIllegal(t *testing.T) {
	// open→counting ok; counting→reconciling ok; reconciling→closed ok (stamps closed_by/at).
	// Illegal: open→closed → ErrInvalidState; closed→counting → ErrInvalidState.
}

func TestKpisCountByResult(t *testing.T) {
	// After setting some item results, GetSession KPIs: total/found/pending/variance match.
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -tags=integration ./internal/stockopname/ -run TestCreateSessionSnapshotsInScopeAssets -v`
Expected: FAIL — package/`Service` not defined.

- [ ] **Step 3: Implement `service.go`**

`backend/internal/stockopname/service.go` — Gin-free. Key logic:
```go
package stockopname

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/disposal"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/transfer"
)

var (
	ErrNotFound          = errors.New("stockopname: not found")
	ErrOutOfScope        = errors.New("stockopname: office out of scope")
	ErrInvalidState      = errors.New("stockopname: not in a state that allows this action")
	ErrInvalidRef        = errors.New("stockopname: invalid reference")
	ErrAlreadyFollowedUp = errors.New("stockopname: item already has a follow-up request")
	ErrNoItem            = errors.New("stockopname: asset not found in this session")
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrInvalidRef
	}
	return err
}

type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
	disp *disposal.Service
	tr   *transfer.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, disp *disposal.Service, tr *transfer.Service) *Service {
	return &Service{q: q, pool: pool, disp: disp, tr: tr}
}

type CreateInput struct {
	OfficeID uuid.UUID
	Name     *string
	Period   time.Time
}

// CreateSession opens a session and snapshots every in-scope, non-disposed asset of the office.
func (s *Service) CreateSession(ctx context.Context, caller approval.Caller, in CreateInput) (sqlc.StockopnameStockOpnameSession, error) {
	if !common.InScope(caller.AllScope, caller.OfficeIDs, in.OfficeID) {
		return sqlc.StockopnameStockOpnameSession{}, ErrOutOfScope
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.StockopnameStockOpnameSession{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	sess, err := qtx.CreateOpnameSession(ctx, sqlc.CreateOpnameSessionParams{
		OfficeID:    in.OfficeID,
		Name:        in.Name,
		Period:      pgtype.Date{Time: in.Period, Valid: true},
		StartedByID: caller.UserID,
	})
	if err != nil {
		return sqlc.StockopnameStockOpnameSession{}, mapDBError(err)
	}
	if err := qtx.SnapshotSessionItems(ctx, sqlc.SnapshotSessionItemsParams{
		SessionID: sess.ID,
		OfficeID:  in.OfficeID,
	}); err != nil {
		return sqlc.StockopnameStockOpnameSession{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.StockopnameStockOpnameSession{}, err
	}
	return sess, nil
}
```
Add:
- `GetSession(ctx, caller, id)` → `q.GetOpnameSession(...)` with `AllScope`/`OfficeIds` (nil-guard `[]uuid.UUID{}`), then `q.SessionKpis(id)`; map no-rows → `ErrNotFound`.
- `ListSessions(...)` → `q.ListOpnameSessions` + `q.CountOpnameSessions` with the same scope args + optional `*string` status parsed into `sqlc.NullSharedOpnameSessionStatus`.
- `Transition(ctx, caller, id, to)`: first `GetSession` (enforces scope + existence), then validate the edge with a package helper:
```go
func canTransition(from, to sqlc.SharedOpnameSessionStatus) bool {
	switch from {
	case sqlc.SharedOpnameSessionStatusOpen:
		return to == sqlc.SharedOpnameSessionStatusCounting
	case sqlc.SharedOpnameSessionStatusCounting:
		return to == sqlc.SharedOpnameSessionStatusReconciling
	case sqlc.SharedOpnameSessionStatusReconciling:
		return to == sqlc.SharedOpnameSessionStatusClosed
	default:
		return false
	}
}
```
On `close`, pass `closed_by_id = caller.UserID` and `closed_at = now()` into `SetSessionStatus`; otherwise leave those null. Return `ErrInvalidState` when `!canTransition`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags=integration ./internal/stockopname/ -v`
Expected: PASS (all Task-3 cases).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/stockopname/service.go backend/internal/stockopname/stockopname_integration_test.go
git commit -m "feat(stockopname): session lifecycle, snapshot, KPI, scoped list/get"
```

---

## Task 4: Service — set item result + scan

**Files:**
- Modify: `backend/internal/stockopname/service.go`
- Modify: `backend/internal/stockopname/stockopname_integration_test.go`

**Interfaces:**
- Produces:
  - `SetItemResult(ctx, caller, sessionID, itemID uuid.UUID, result sqlc.SharedOpnameItemResult, note *string) (sqlc.StockopnameStockOpnameItem, error)` — only when session is `counting`; stamps `counted_by/at`.
  - `Scan(ctx, caller, sessionID uuid.UUID, tag string) (sqlc.StockopnameStockOpnameItem, error)` — returns the matching item; if the tag resolves to an in-scope asset not yet in the session, inserts an `expected=false` item and returns it; unknown/out-of-scope tag → `ErrNoItem`/`ErrOutOfScope`.
  - `ListItems(ctx, caller, sessionID uuid.UUID, result *string) (rows, error)`.

- [ ] **Step 1: Write failing tests**

Add to the integration test:
```go
func TestSetItemResultOnlyWhenCounting(t *testing.T) {
	// session still 'open' → SetItemResult → ErrInvalidState.
	// after start (counting): SetItemResult(found) succeeds; row.Result=='found', counted_by/at set.
	// after reconcile (reconciling): SetItemResult → ErrInvalidState (locked).
}

func TestScanAddsUnexpectedInScopeAsset(t *testing.T) {
	// Session in 'counting'. Scan a tag of an in-scope asset NOT in the snapshot
	// (e.g. an asset moved in) → returns a new item with expected=false, result='pending'.
	// Scanning a tag already in the session returns the existing item (no duplicate).
	// Scanning an out-of-scope asset's tag → ErrOutOfScope.
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test -tags=integration ./internal/stockopname/ -run TestSetItemResultOnlyWhenCounting -v`
Expected: FAIL — method undefined.

- [ ] **Step 3: Implement**

Add to `service.go`. `SetItemResult` loads the session (scope + existence via `GetSession`), rejects unless `status == counting` (`ErrInvalidState`), then:
```go
row, err := s.q.SetOpnameItemResult(ctx, sqlc.SetOpnameItemResultParams{
	ID: itemID, SessionID: sessionID, Result: result, Note: note, CountedByID: &caller.UserID,
})
if err != nil { return sqlc.StockopnameStockOpnameItem{}, mapDBError(err) }
return row, nil
```
`Scan`: load session (scope + must be `counting`); `GetOpnameItemByTag(session, tag)` → if found, return it; if no-rows, `GetAssetByTag(tag)` (the existing `assets.sql` query) → if no-rows `ErrNoItem`; check `common.InScope(caller..., asset.OfficeID)` else `ErrOutOfScope`; `InsertUnexpectedItem(session, asset.ID)` and return — `InsertUnexpectedItem` is `:one` with `ON CONFLICT DO NOTHING`, so a conflict (item already exists) surfaces as `pgx.ErrNoRows`: treat that as "already present" and re-`GetOpnameItemByTag` rather than a real not-found. `ListItems` calls `ListOpnameItemsEnriched` with optional result filter after a scope check via `GetSession`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -tags=integration ./internal/stockopname/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/stockopname/service.go backend/internal/stockopname/stockopname_integration_test.go
git commit -m "feat(stockopname): set item result + scan (add unexpected in-scope asset)"
```

---

## Task 5: Service — variance follow-up (reuse disposal/transfer Submit)

**Files:**
- Modify: `backend/internal/stockopname/service.go`
- Modify: `backend/internal/stockopname/stockopname_integration_test.go`

**Interfaces:**
- Produces:
  - `type FollowupInput struct { ToOfficeID *uuid.UUID; ToRoomID *uuid.UUID; Reason *string }`.
  - `GenerateFollowup(ctx, caller approval.Caller, sessionID, itemID uuid.UUID, in FollowupInput) (requestID uuid.UUID, requestType string, error)` — maps item result → a disposal (`not_found`) or transfer (`misplaced`) request via the reused services, then `SetItemFollowup`. `damaged` → `ErrInvalidState` (maintenance module not built). Second call on an already-linked item → `ErrAlreadyFollowedUp`.

- [ ] **Step 1: Write failing tests**

```go
func TestFollowupNotFoundCreatesDisposalWriteOff(t *testing.T) {
	// Item result 'not_found' on an available asset. GenerateFollowup (no office needed).
	// Assert: an approval.requests row of type 'asset_disposal' exists targeting the asset;
	//         requestType=="asset_disposal"; the item's followup_request_id == that request id.
}

func TestFollowupMisplacedCreatesTransfer(t *testing.T) {
	// Item result 'misplaced'. FollowupInput.ToOfficeID = a different in-scope office.
	// Assert: an 'asset_transfer' request exists; item linked.
}

func TestFollowupDamagedRejected(t *testing.T) {
	// Item result 'damaged' → ErrInvalidState (maintenance deferred).
}

func TestFollowupDuplicateRejected(t *testing.T) {
	// After a successful follow-up, a second call on the same item → ErrAlreadyFollowedUp.
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test -tags=integration ./internal/stockopname/ -run TestFollowupNotFoundCreatesDisposalWriteOff -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `service.go`:
```go
type FollowupInput struct {
	ToOfficeID *uuid.UUID
	ToRoomID   *uuid.UUID
	Reason     *string
}

func (s *Service) GenerateFollowup(ctx context.Context, caller approval.Caller, sessionID, itemID uuid.UUID, in FollowupInput) (uuid.UUID, string, error) {
	// session scope + existence
	if _, _, err := s.GetSession(ctx, caller, sessionID); err != nil {
		return uuid.Nil, "", err
	}
	item, err := s.q.GetOpnameItem(ctx, sqlc.GetOpnameItemParams{ID: itemID, SessionID: sessionID})
	if err != nil {
		return uuid.Nil, "", mapDBError(err)
	}
	if item.FollowupRequestID != nil {
		return uuid.Nil, "", ErrAlreadyFollowedUp
	}
	writeOff := "write_off"
	var reqID uuid.UUID
	var reqType string
	switch item.Result {
	case sqlc.SharedOpnameItemResultNotFound:
		req, err := s.disp.Submit(ctx, caller, disposal.SubmitInput{
			AssetID: item.AssetID, Method: writeOff,
			DisposalDate: time.Now().Format("2006-01-02"), Reason: in.Reason,
		})
		if err != nil {
			return uuid.Nil, "", err
		}
		reqID, reqType = req.ID, "asset_disposal"
	case sqlc.SharedOpnameItemResultMisplaced:
		if in.ToOfficeID == nil {
			return uuid.Nil, "", ErrInvalidRef
		}
		req, err := s.tr.Submit(ctx, caller, transfer.SubmitInput{
			AssetID: item.AssetID, ToOfficeID: *in.ToOfficeID, ToRoomID: in.ToRoomID, Reason: in.Reason,
		})
		if err != nil {
			return uuid.Nil, "", err
		}
		reqID, reqType = req.ID, "asset_transfer"
	default: // pending / found / damaged
		return uuid.Nil, "", ErrInvalidState
	}
	if _, err := s.q.SetItemFollowup(ctx, sqlc.SetItemFollowupParams{
		ID: itemID, SessionID: sessionID, FollowupRequestID: &reqID,
	}); err != nil {
		return uuid.Nil, "", mapDBError(err)
	}
	return reqID, reqType, nil
}
```
(Note: `disposal.Submit`/`transfer.Submit` enforce their own asset-scope + state guards, so an out-of-scope asset or bad target surfaces their sentinels — surface them as-is; the handler maps to 4xx.)

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -tags=integration ./internal/stockopname/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/stockopname/service.go backend/internal/stockopname/stockopname_integration_test.go
git commit -m "feat(stockopname): auto-generate disposal/transfer follow-up from variance"
```

---

## Task 6: Berita Acara export (PDF + Excel)

**Files:**
- Create: `backend/internal/stockopname/report.go`
- Test: `backend/internal/stockopname/report_test.go`

**Interfaces:**
- Produces:
  - `type ReportData struct { SessionName, OfficeName, Period, ClosedByName string; Kpi KpiCounts; Items []ReportItem }` (define `KpiCounts`/`ReportItem` with the display fields).
  - `RenderPDF(d ReportData) ([]byte, error)` and `RenderXLSX(d ReportData) ([]byte, error)`.
  - Service method `ReportData(ctx, caller, sessionID uuid.UUID) (ReportData, error)` assembling the struct from `GetOpnameSession` + `SessionKpis` + `ListOpnameItemsEnriched`.

- [ ] **Step 1: Write failing unit test**

`backend/internal/stockopname/report_test.go` (plain unit test, no DB):
```go
package stockopname

import "testing"

func sampleReport() ReportData {
	return ReportData{
		SessionName: "Opname Semester I 2026", OfficeName: "KC Jakarta Selatan",
		Period: "Juni 2026", ClosedByName: "Dewi Lestari",
		Kpi:   KpiCounts{Total: 4, Found: 2, Pending: 0, Variance: 2},
		Items: []ReportItem{{AssetName: "Laptop", AssetTag: "JKT01-ELK-2026-00001", Result: "found"}},
	}
}

func TestRenderPDFNonEmpty(t *testing.T) {
	b, err := RenderPDF(sampleReport())
	if err != nil { t.Fatal(err) }
	if len(b) < 100 || string(b[:4]) != "%PDF" { t.Fatalf("not a PDF: %d bytes", len(b)) }
}

func TestRenderXLSXNonEmpty(t *testing.T) {
	b, err := RenderXLSX(sampleReport())
	if err != nil { t.Fatal(err) }
	if len(b) < 100 || string(b[:2]) != "PK" { t.Fatalf("not an xlsx zip: %d bytes", len(b)) }
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/stockopname/ -run TestRenderPDFNonEmpty -v`
Expected: FAIL — `RenderPDF` undefined.

- [ ] **Step 3: Implement `report.go`**

Use `github.com/jung-kurt/gofpdf` (already a dep — see depreciation export) for PDF and `github.com/xuri/excelize/v2` for XLSX. PDF: bank header (reuse `app_settings` company_name is done at the handler/service layer — pass strings in via `ReportData`; keep `report.go` pure), title "BERITA ACARA STOCK OPNAME", session/office/period lines, a KPI summary block, then a table of items (asset name, tag, result), and a signatory line. XLSX: a "Ringkasan" sheet (KPI rows) + an "Item" sheet (columns: Aset, Kode, Hasil, Catatan). Return `buf.Bytes()`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/stockopname/ -run 'TestRender' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/stockopname/report.go backend/internal/stockopname/report_test.go
git commit -m "feat(stockopname): Berita Acara PDF + Excel render"
```

---

## Task 7: DTO + handler + routes + router wiring

**Files:**
- Create: `backend/internal/stockopname/dto.go`
- Create: `backend/internal/stockopname/handler.go`
- Create: `backend/internal/stockopname/routes.go`
- Modify: `backend/internal/server/router.go`

**Interfaces:**
- Produces: `func NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler`; `func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView gin.HandlerFunc)`.
- Consumes: Service methods (Tasks 3–6); router shared vars.

- [ ] **Step 1: Write `dto.go`**

Request structs with binding + `toSessionResponse`/`toItemResponse` maps:
```go
type CreateSessionRequest struct {
	OfficeID string  `json:"office_id" binding:"required,uuid"`
	Name     *string `json:"name"`
	Period   string  `json:"period" binding:"required"` // "2006-01" or "2006-01-02"
}
type SetResultRequest struct {
	Result string  `json:"result" binding:"required,oneof=found not_found damaged misplaced pending"`
	Note   *string `json:"note"`
}
type ScanRequest struct {
	AssetTag string `json:"asset_tag" binding:"required"`
}
type FollowupRequest struct {
	ToOfficeID *string `json:"to_office_id" binding:"omitempty,uuid"`
	ToRoomID   *string `json:"to_room_id" binding:"omitempty,uuid"`
	Reason     *string `json:"reason"`
}
```
`toSessionResponse(s, officeName, startedByName, closedByName *string, kpi)` and `toItemResponse(enriched row)` return `map[string]any` with English snake_case keys (`id`, `office_id`, `status`, `period`, `office_name`, `started_by_name`, `total`/`found`/`pending`/`variance`, and for items `asset_id`, `asset_name`, `asset_tag`, `room_name`, `result`, `expected`, `note`, `counted_by_name`, `counted_at`, `followup_request_id`). Add a `parsePeriod(string) (time.Time, error)` accepting `"2006-01"`→first-of-month or `"2006-01-02"`.

- [ ] **Step 2: Write `handler.go`**

```go
const scopeModule = "stockopname"

type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
	aud    *audit.Service
}
func NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler {
	return &Handler{svc: svc, scoped: common.ScopedDeps{Q: q, Scope: scope}, aud: aud}
}
```
Add `svcError(c, err)` mapping: `ErrNotFound`/`ErrNoItem`→404, `ErrOutOfScope`→403, `ErrInvalidState`/`ErrAlreadyFollowedUp`→409, `ErrInvalidRef`→422, plus fall-through to disposal/transfer sentinels (import and check `disposal.ErrOutOfScope`, `transfer.ErrSameOffice`, etc. OR just `common.WriteError` default → these already print sensible messages; map their obvious ones to 409/422). Add `caller(c)` (copy the transfer pattern: parse `CtxUserID`/`CtxRoleID`, `scoped.CallerOfficeScope(c, scopeModule)` → `approval.Caller`). Copy `clampInt`. Implement one handler per endpoint (bind → caller → parse uuids → service → `audit.Record` on mutations → respond). List returns `{data,total,limit,offset}`. The report handler streams bytes: `c.Data(200, "application/pdf"|xlsx-mime, bytes)` with a `Content-Disposition` filename.

- [ ] **Step 3: Write `routes.go`**

```go
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView gin.HandlerFunc) {
	g := rg.Group("/stock-opname/sessions")
	g.GET("", authMW, requireView, h.list)
	g.POST("", authMW, requireManage, h.create)
	g.GET("/:id", authMW, requireView, h.get)
	g.GET("/:id/items", authMW, requireView, h.listItems)
	g.POST("/:id/start", authMW, requireManage, h.start)
	g.POST("/:id/scan", authMW, requireManage, h.scan)
	g.PATCH("/:id/items/:itemId", authMW, requireManage, h.setResult)
	g.POST("/:id/reconcile", authMW, requireManage, h.reconcile)
	g.POST("/:id/items/:itemId/follow-up", authMW, requireManage, h.followup)
	g.POST("/:id/close", authMW, requireManage, h.close)
	g.GET("/:id/report", authMW, requireView, h.report)
}
```

- [ ] **Step 4: Wire into `NewRouter`**

In `backend/internal/server/router.go`, after the `disposalSvc`/`transferSvc` block (both are already constructed there), add:
```go
stockopnameSvc := stockopname.NewService(queries, d.Pool, disposalSvc, transferSvc)
stockopnameHandler := stockopname.NewHandler(stockopnameSvc, scopeSvc, queries, auditSvc)
stockopname.RegisterRoutes(api, stockopnameHandler,
	requireAuth,
	middleware.RequirePermission(permSvc, "stockopname.manage"),
	middleware.RequirePermission(permSvc, "stockopname.view"),
)
```
Add the import `"github.com/ragbuaj/inventra/internal/stockopname"`.

- [ ] **Step 5: Build, vet, and run all backend tests**

Run:
```bash
go build ./... && go vet ./... && go test ./... && go test -tags=integration ./internal/stockopname/ -v
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/stockopname/dto.go backend/internal/stockopname/handler.go backend/internal/stockopname/routes.go backend/internal/server/router.go
git commit -m "feat(stockopname): HTTP handlers, routes, router wiring"
```

---

## Task 8: OpenAPI spec

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add schemas + paths**

Add a `StockOpnameSession`, `StockOpnameItem`, and request-body schemas, plus the 11 paths from Task 7 under a `stock-opname` tag. Mirror the shape of the existing `Transfer`/`Disposal` entries (auth security, `{data,total,limit,offset}` list envelope, 400/403/404/409/422 responses, the report endpoint producing `application/pdf` and the xlsx mime with a `format` query param).

- [ ] **Step 2: Lint**

Run: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(openapi): stock opname endpoints"
```

---

## Task 9: Frontend — meta constants, i18n, nav item

**Files:**
- Create: `frontend/app/constants/stockOpnameMeta.ts`
- Modify: `frontend/app/utils/nav.ts`
- Modify: `frontend/i18n/locales/id.json`
- Modify: `frontend/i18n/locales/en.json`
- Test: `frontend/test/unit/stock-opname-meta.spec.ts`

**Interfaces:**
- Produces: `SESSION_STATUS_TONE`, `ITEM_RESULT_TONE`, `SESSION_STATUS_KEYS`, `ITEM_RESULT_KEYS`, `RESULT_ACTION` (maps result→follow-up kind), types `SessionStatus`, `ItemResult`.

- [ ] **Step 1: Write failing unit test**

`frontend/test/unit/stock-opname-meta.spec.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { SESSION_STATUS_TONE, ITEM_RESULT_TONE, ITEM_RESULT_KEYS, RESULT_ACTION } from '~/constants/stockOpnameMeta'

describe('stockOpnameMeta', () => {
  it('has a tone for every session status incl. reconciling', () => {
    expect(SESSION_STATUS_TONE.open).toBe('neutral')
    expect(SESSION_STATUS_TONE.counting).toBe('info')
    expect(SESSION_STATUS_TONE.reconciling).toBe('warning')
    expect(SESSION_STATUS_TONE.closed).toBe('success')
  })
  it('maps each variance result to a follow-up action', () => {
    expect(RESULT_ACTION.not_found).toBe('disposal')
    expect(RESULT_ACTION.misplaced).toBe('transfer')
    expect(RESULT_ACTION.damaged).toBe('maintenance') // disabled in UI
  })
  it('lists all five item results', () => {
    expect(ITEM_RESULT_KEYS).toEqual(['pending', 'found', 'not_found', 'damaged', 'misplaced'])
    expect(ITEM_RESULT_TONE.found).toBe('success')
  })
})
```

- [ ] **Step 2: Run to verify fail**

Run (from `frontend/`): `pnpm test stock-opname-meta`
Expected: FAIL — module missing.

- [ ] **Step 3: Implement meta + i18n + nav**

`frontend/app/constants/stockOpnameMeta.ts`:
```ts
import type { BadgeColor } from '~/types'

export type SessionStatus = 'open' | 'counting' | 'reconciling' | 'closed'
export type ItemResult = 'pending' | 'found' | 'not_found' | 'damaged' | 'misplaced'

export const SESSION_STATUS_KEYS: SessionStatus[] = ['open', 'counting', 'reconciling', 'closed']
export const ITEM_RESULT_KEYS: ItemResult[] = ['pending', 'found', 'not_found', 'damaged', 'misplaced']

export const SESSION_STATUS_TONE: Record<SessionStatus, BadgeColor> = {
  open: 'neutral',
  counting: 'info',
  reconciling: 'warning',
  closed: 'success'
}

export const ITEM_RESULT_TONE: Record<ItemResult, BadgeColor> = {
  pending: 'neutral',
  found: 'success',
  not_found: 'error',
  damaged: 'warning',
  misplaced: 'primary'
}

export const RESULT_ACTION: Record<'not_found' | 'damaged' | 'misplaced', 'disposal' | 'maintenance' | 'transfer'> = {
  not_found: 'disposal',
  misplaced: 'transfer',
  damaged: 'maintenance'
}
```
Add a `stockOpname` block to `frontend/i18n/locales/id.json` and the 1:1 mirror in `en.json` (keys: `pageTitle`, `pageSub`, `create.*`, `status.{open,counting,reconciling,closed}`, `result.{pending,found,not_found,damaged,misplaced}`, `kpi.{total,found,pending,variance}`, `scan.*`, `variance.*`, `followup.*`, `finish.*`, `empty.*`, plus `submitSuccess`/error strings). Add `"stockOpname": "Stock Opname"` to the `nav` object in both locales. In `frontend/app/utils/nav.ts`, add to the Operasional group of `superadminNav`:
```ts
{ labelKey: 'nav.stockOpname', icon: 'i-lucide-clipboard-list', to: '/stock-opname', permission: 'stockopname.view' }
```

- [ ] **Step 4: Run tests + lint + typecheck**

Run: `pnpm test stock-opname-meta && pnpm lint && pnpm typecheck`
Expected: PASS (mind: no trailing commas).

- [ ] **Step 5: Commit**

```bash
git add frontend/app/constants/stockOpnameMeta.ts frontend/app/utils/nav.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/unit/stock-opname-meta.spec.ts
git commit -m "feat(stockopname-ui): meta constants, i18n, nav item"
```

---

## Task 10: Frontend — `useStockOpname` composable

**Files:**
- Create: `frontend/app/composables/api/useStockOpname.ts`
- Test: `frontend/test/unit/use-stock-opname.spec.ts`

**Interfaces:**
- Produces `useStockOpname()` returning `{ list, get, items, create, start, scan, setResult, reconcile, followup, close, reportUrl }` with typed DTOs `OpnameSession`, `OpnameItem`, `OpnameSessionDetail` (session + `kpi`), `CreateSessionInput`, `FollowupInput`. All calls go through `useApiClient().request`. `reportUrl(id, format)` returns the path string for a blob download via `requestBlob`.

- [ ] **Step 1: Write failing unit test**

`frontend/test/unit/use-stock-opname.spec.ts` — mock `useApiClient` and assert the composable calls the right paths/bodies:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const requestMock = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request: requestMock, requestBlob: vi.fn() }) }))
// eslint-disable-next-line import/first
import { useStockOpname } from '~/composables/api/useStockOpname'

describe('useStockOpname', () => {
  beforeEach(() => { vi.clearAllMocks(); requestMock.mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0 }) })

  it('lists sessions with status filter', async () => {
    await useStockOpname().list({ status: 'counting', limit: 20, offset: 0 })
    expect(requestMock).toHaveBeenCalledWith('/stock-opname/sessions', { query: { status: 'counting', limit: 20, offset: 0 } })
  })
  it('creates a session with the exact body', async () => {
    requestMock.mockResolvedValue({ id: 's1' })
    await useStockOpname().create({ office_id: 'o1', name: 'Opname', period: '2026-07' })
    expect(requestMock).toHaveBeenCalledWith('/stock-opname/sessions', { method: 'POST', body: { office_id: 'o1', name: 'Opname', period: '2026-07' } })
  })
  it('sets an item result', async () => {
    requestMock.mockResolvedValue({ id: 'i1', result: 'found' })
    await useStockOpname().setResult('s1', 'i1', { result: 'found', note: null })
    expect(requestMock).toHaveBeenCalledWith('/stock-opname/sessions/s1/items/i1', { method: 'PATCH', body: { result: 'found', note: null } })
  })
  it('posts a follow-up', async () => {
    requestMock.mockResolvedValue({ request_id: 'r1', request_type: 'asset_disposal' })
    await useStockOpname().followup('s1', 'i1', { to_office_id: null, to_room_id: null, reason: null })
    expect(requestMock).toHaveBeenCalledWith('/stock-opname/sessions/s1/items/i1/follow-up', { method: 'POST', body: { to_office_id: null, to_room_id: null, reason: null } })
  })
})
```

- [ ] **Step 2: Run to verify fail**

Run: `pnpm test use-stock-opname`
Expected: FAIL.

- [ ] **Step 3: Implement the composable**

Mirror `useTransfers.ts` structure. Typed interfaces (English snake_case matching the backend `toSessionResponse`/`toItemResponse` keys), `useApiClient().request` for every call, `undefined`-stripped query building. `reportUrl` uses `requestBlob('/stock-opname/sessions/${id}/report', { query: { format } })`.

- [ ] **Step 4: Run tests to verify pass**

Run: `pnpm test use-stock-opname && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useStockOpname.ts frontend/test/unit/use-stock-opname.spec.ts
git commit -m "feat(stockopname-ui): useStockOpname composable"
```

---

## Task 11: Frontend — `/stock-opname` page (list + detail + modals)

**Files:**
- Create: `frontend/app/pages/stock-opname.vue`
- Create (if extracted): `frontend/app/components/stockopname/*.vue` (e.g. `SessionCard.vue`, `CreateSessionModal.vue`, `FinishModal.vue`, `FollowupModal.vue`)
- Test: `frontend/test/nuxt/stock-opname.spec.ts`

**Interfaces:**
- Consumes: `useStockOpname`, `useOffices`, `useFloors`/`useReference` (destination pickers), `stockOpnameMeta`, `useCan`, `useI18n`. Reuse `AssetSearchPicker` where an asset picker is needed.
- Page contract (build 1:1 against `docs/design/Stock Opname.dc.html`):
  - `definePageMeta({ middleware: 'can', permission: 'stockopname.view' })`.
  - **List view** (`isList`): header (`pageTitle`/`pageSub`) + "Buat Sesi" button (gated `stockopname.manage`); session cards (`data-testid="opname-session-row"`) showing name, status `UBadge` (`SESSION_STATUS_TONE`), scope (office_name), period, and a progress bar = `(found)/(total)`; empty state (`data-testid="opname-empty"`). Create modal (`data-testid="opname-create-*"`: name input, office `USelect` from `useOffices().list({limit:100})`, period `type="month"`, snapshot info note, confirm).
  - **Detail view** (`isDetail`): header with status badge + "Berita Acara" export button + "Selesaikan" button (running only); 4 KPI tiles (`data-testid="opname-kpi-{total,found,pending,variance}"`); scan bar when `counting` (scan-next stub + manual code `UInput` `data-testid="opname-scan-input"` + check button calling `scan`); item table (`data-testid="opname-item-row"`) with asset name+tag, location, and a result control — segmented buttons (found/damaged/misplaced/not_found) when `counting` (`data-testid="opname-result-{result}"`), read-only `UBadge` otherwise; a variance panel listing `not_found`/`damaged`/`misplaced` items each with a follow-up button (`data-testid="opname-followup-{result}"`) — the `damaged` button is `disabled` with a "segera hadir" tooltip. Follow-up modal for `misplaced` collects destination office/room. Finish modal shows a Berita Acara preview + PDF/Excel download + confirm (calls `close` then triggers report).
  - State transitions call `start`/`reconcile`/`close`; four render states (loading skeleton / error+retry / empty / populated) on every fetch.

- [ ] **Step 1: Write failing component tests**

`frontend/test/nuxt/stock-opname.spec.ts` (`// @vitest-environment nuxt`). Mock `useStockOpname`, `useOffices` (and any other composables the page calls) with `vi.fn()`s; use `mountSuspended`, `useAuthStore().setSession(...)` to grant permissions; teleported modal buttons via `document.body.querySelector`. Cover (assert real rendered text/behavior, resolved id-locale strings, exact call args):
```ts
// Representative cases (write all of them):
// 1. list loads: sessions render, status badge text resolves ('Berjalan'/'Rekonsiliasi'/'Selesai'/'Terbuka'), empty-state when none.
// 2. create: fill name+office+period, confirm → create() called with { office_id, name, period }; success toast text.
// 3. create hidden without stockopname.manage (grant only 'stockopname.view') → create button absent/disabled.
// 4. detail open state: KPI tiles show counts; no scan bar.
// 5. detail counting state: scan bar visible; clicking a result segment → setResult() called with the item id + result.
// 6. detail reconciling state: result segments read-only (badges), variance panel shows follow-up buttons.
// 7. variance follow-up not_found → followup() called with disposal (no office); misplaced opens modal, requires office; damaged button disabled.
// 8. detail closed state: "Selesaikan" hidden; Berita Acara export button present.
// 9. loading skeleton + error+retry states on the list and detail fetch.
```

- [ ] **Step 2: Run to verify fail**

Run: `pnpm test stock-opname` (nuxt spec)
Expected: FAIL — page missing.

- [ ] **Step 3: Build the page to match the mockup**

Open `docs/design/Stock Opname.dc.html` in a browser and reproduce it with `U*` components per the page contract above. Keep pages thin — extract `SessionCard`, `CreateSessionModal`, `FinishModal`, `FollowupModal` into `frontend/app/components/stockopname/`. Semantic tokens only; `data-testid` on every asserted element; `USelect` uses `value-key="value"` + `NONE='__none__'` sentinel. Reuse `AssetSearchPicker` if the scan/manual flow benefits from it (otherwise a plain code input is fine — the mockup uses a mono code input).

- [ ] **Step 4: Run component tests to verify pass**

Run: `pnpm test stock-opname && pnpm lint && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/stock-opname.vue frontend/app/components/stockopname/ frontend/test/nuxt/stock-opname.spec.ts
git commit -m "feat(stockopname-ui): /stock-opname screen (list, detail, modals)"
```

---

## Task 12: Frontend — real-backend e2e

**Files:**
- Create: `frontend/e2e/stock-opname.spec.ts`

**Interfaces:**
- Consumes: the running backend stack + seeded admin; `frontend/e2e/helpers.ts` (`login`, `EMAIL`, `PASSWORD`).

- [ ] **Step 1: Write the e2e spec**

`frontend/e2e/stock-opname.spec.ts` — `test.describe.configure({ mode: 'serial' })`, `RUN = ${Date.now()}` unique suffix, `APIRequestContext` for prereqs. Flow:
```
beforeAll (via API, unique per run):
  - login admin; create office-type, office A (in admin scope), a category (asset_class 'intangible' to avoid room CHECK), and 2 assets in office A via POST /assets → approve each with a second SoD Superadmin checker (reuse the transfers.spec.ts checker pattern) so they reach status 'available'.
  - create a second in-scope office B (transfer destination).
Test 1 — full lifecycle:
  - UI login; navigate /stock-opname; "Buat Sesi" with office A + current month → session appears (status Terbuka).
  - Open detail; "Mulai" (start) → status Berjalan; scan/mark asset1 'found', asset2 'not_found' via the result segments (assert KPI updates).
  - "Rekonsiliasi" (reconcile) → status Rekonsiliasi; variance panel shows asset2.
  - Follow-up asset2 (not_found → disposal): click → API-verify an 'asset_disposal' pending request exists targeting asset2 (GET /requests?type=asset_disposal&status=pending), and the item shows "sudah diajukan".
  - "Selesaikan" (close) → status Selesai; export button visible.
  - GET /stock-opname/sessions/:id/report?format=pdf returns 200 application/pdf (API assert).
afterAll: delete the checker user (best-effort).
```
Use row-scoped assertions (`page.locator('[data-testid="opname-session-row"]', { hasText: sessionName })`), never bare `getByText` for rows. Approve via `POST /requests/:id/approve` with `{ decision: 'approve' }` as the checker token.

- [ ] **Step 2: Run the e2e (backend stack must be up + seeded admin)**

Run (from `frontend/`, with the stack up and `RATELIMIT_ENABLED=false` on the backend):
```bash
pnpm test:e2e stock-opname
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/e2e/stock-opname.spec.ts
git commit -m "test(stockopname): real-backend e2e (lifecycle + follow-up + report)"
```

---

## Task 13: PROGRESS.md + full gate sweep + mockup side-by-side

**Files:**
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Run the full gate sweep**

Backend (from `backend/`):
```bash
go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
```
Frontend (from `frontend/`):
```bash
pnpm lint && pnpm typecheck && pnpm test && pnpm build
```
Expected: all green. (Per the full-integration-gate memory, run integration across **all** packages after the shared router change.)

- [ ] **Step 2: Side-by-side mockup comparison**

Open the built `/stock-opname` and `docs/design/Stock Opname.dc.html` together (light + dark). Verify 1:1 for layout/spacing/hierarchy and every state (list empty/populated; detail open/counting/reconciling/closed; scan bar; variance panel; create/finish/follow-up modals). Confirm only the spec's approved deviations (a)/(b)/(c) are present. Fix any gap before proceeding.

- [ ] **Step 3: Update PROGRESS.md**

Tick the Stock Opname checkbox in the "⛔ Remaining → Bank-FAM" section (`- [ ] Stock opname` → `- [x]` with a one-line note + this feature's PR number), add a "Frontend Stock Opname" done line in the frontend-wiring section, record the three approved deviations, and refresh the "▶ Next session — start here" block to point at the next real step (remaining candidates: Assignment/Maintenance, global search backend, Reporting & Dashboard).

- [ ] **Step 4: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): stock opname module complete"
```

---

## Self-review notes (coverage against the spec)

- Spec bagian 3 migration → Task 1. bagian 4 module files → Tasks 3–7 (`service`/`dto`/`handler`/`routes`) + Task 6 (`report.go`). bagian 5 endpoints → Tasks 7–8 (all 11 paths). bagian 5 follow-up mapping → Task 5 (not_found→disposal, misplaced→transfer, damaged deferred/disabled). bagian 6 frontend → Tasks 9–11. bagian 7 deviations → recorded in Task 13. bagian 8 testing → Tasks 3–6 (backend integration/unit), 9–11 (frontend unit/component), 12 (e2e). bagian 9 out-of-scope (maintenance follow-up, MinIO storage, camera scan) honored (damaged disabled; report on-the-fly; manual code entry).
- Scope enforced read+write: `common.InScope` in service single-row ops; `AllScope`/`OfficeIds` in list/get queries; every route gated by `RequirePermission`.
- No new `request_type` enum / executor (reuses `disposal.Submit`/`transfer.Submit`) — consistent across Tasks 5 and 7.
