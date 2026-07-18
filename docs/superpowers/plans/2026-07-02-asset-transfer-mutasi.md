# Asset Transfer (Mutasi) Backend Module — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the backend inter-office asset transfer (mutasi) module — submit via the generic maker-checker approval engine, then ship → receive with a BAST document and asset relocation.

**Architecture:** New `internal/transfer/` module (four-file split per ADR-0008) plus an `asset_transfer` approval executor. The approval request owns pre-approval state; the executor creates the `transfer.asset_transfers` row on final approval; the transfer module owns `ship`/`receive`. Receive moves the asset (`assets.office_id/room_id`) and records a BAST via the existing `internal/asset` document service (metadata + optional MinIO file).

**Tech Stack:** Go 1.25 + Gin + sqlc (pgx/v5) + golang-migrate; Postgres 16, Redis 7, MinIO. Tests: testify + testcontainers-go (`//go:build integration`).

## Global Constraints

- **No new tables** — `transfer.asset_transfers`, `asset.asset_documents`, and all enums already exist (migration `000015_fam_tables`, enums `000002`). This cycle adds a seed migration, queries, module code, wiring, OpenAPI, tests.
- **Highest existing migration is `000019_employee_phone`** → new migration is `000020_transfer_seed` (re-verify with `ls backend/db/migrations` before writing).
- **Reuse the approval engine.** Submit calls `approval.Service.Submit`; the `asset_transfer` executor implements `approval.Executor` (`Execute(ctx, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error`) and is registered in `NewRouter`. Reject/cancel run **no** executor — never create a transfer row outside the executor.
- **Enum/type names (verbatim):** `sqlc.SharedRequestTypeAssetTransfer` (`"asset_transfer"`), `sqlc.SharedTransferStatus{Approved,InTransit,Received}` (`approved`/`in_transit`/`received`), `sqlc.SharedAssetDocumentTypeBastTransfer` (`bast_transfer`). Money columns map to Go `*string`.
- **Authorization on read AND write.** Writes gated by `transfer.manage`, reads by `transfer.view` (both new permission keys). Data-scope module string is `"transfers"`, resolved via `common.ScopedDeps.CallerOfficeScope(c, "transfers")` → `(allScope bool, officeIDs []uuid.UUID)`. Submit/ship require the asset's/transfer's `from_office_id` in scope; receive requires `to_office_id` in scope; list/get/history filter `from_office_id` **or** `to_office_id` in scope.
- **List envelope** `{data, total, limit, offset}` with `limit` clamped 1–100 via `common.ClampInt(c.Query("limit"), 20, 1, 100)`. Single-row responses are flat. Ship/receive return the updated transfer.
- **Value basis:** threshold `amount = asset.purchase_cost` (nil/empty → `"0"`). Book-value basis deferred to the depreciation cycle (documented in code comment).
- **Never hand-edit `backend/db/sqlc/`** — change `db/queries/*.sql` / migrations then run `sqlc generate` from `backend/`.
- **Gates (from `backend/`):** `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./...`, and Spectral: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` (from repo root). Integration needs dev infra up: `docker compose -f docker-compose.dev.yml up -d`.
- **Canonical patterns to mirror:** module four-file split → `internal/masterdata/office/` (handler scope resolution, `svcError`, audit). Executor → `internal/asset/executor.go` (`disposalExec`). Seed migration → `backend/db/migrations/000016_office_tier.up.sql`.

---

### Task 1: Seed migration — thresholds + permissions + scope for transfer

**Files:**
- Create: `backend/db/migrations/000020_transfer_seed.up.sql`, `backend/db/migrations/000020_transfer_seed.down.sql`

**Interfaces:**
- Produces: `asset_transfer` rows in `approval.approval_thresholds` (so `approval.Submit` resolves a chain); `transfer.manage` / `transfer.view` rows in `identity.role_permissions`; `transfers` rows in `identity.data_scope_policies`. Consumed at runtime by the approval engine + `RequirePermission` + scope resolver.

- [ ] **Step 1: Confirm the next migration number**

Run (from repo root): `ls backend/db/migrations | grep -oE '^[0-9]{6}' | sort -u | tail -1`
Expected: `000019`. If higher, bump the new file number accordingly and adjust all references.

- [ ] **Step 2: Write the up migration**

`backend/db/migrations/000020_transfer_seed.up.sql`:
```sql
-- Migration 000020: seed data for the asset transfer (mutasi) module.
-- Tables already exist (000015_fam_tables); this seeds approval bands, permissions,
-- and data-scope so the transfer endpoints are usable. See
-- docs/superpowers/specs/2026-07-02-asset-transfer-mutasi-design.md.

-- Approval thresholds for asset_transfer (placeholder bands, mirror asset_disposal).
-- Unique constraint: (request_type, amount_from, step_order).
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order) VALUES
  ('asset_transfer', 0,         50000000, 'office',  1),
  ('asset_transfer', 50000000,  NULL,     'office',  1),
  ('asset_transfer', 50000000,  NULL,     'wilayah', 2)
ON CONFLICT DO NOTHING;

-- Permissions: transfer.manage (submit/ship/receive) + transfer.view (read).
-- Superadmin via '*'; operational roles get both; Staf gets neither (cannot mutate/see).
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('transfer.manage'), ('transfer.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Data-scope for the 'transfers' module (mirror 'assets' from 000016).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'transfers', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
```

- [ ] **Step 3: Write the down migration**

`backend/db/migrations/000020_transfer_seed.down.sql`:
```sql
DELETE FROM identity.data_scope_policies WHERE module = 'transfers';
DELETE FROM identity.role_permissions WHERE permission_key IN ('transfer.manage', 'transfer.view');
DELETE FROM approval.approval_thresholds WHERE request_type = 'asset_transfer';
```

- [ ] **Step 4: Apply the migration**

Run (from `backend/`, dev DB on :5433):
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: `000020` applied, no error. Verify: `psql "$DATABASE_URL" -c "select count(*) from approval.approval_thresholds where request_type='asset_transfer';"` → `3`.

- [ ] **Step 5: Commit**

```bash
git add backend/db/migrations/000020_transfer_seed.up.sql backend/db/migrations/000020_transfer_seed.down.sql
git commit -m "feat(db): seed asset_transfer thresholds + transfer permissions/scope"
```

---

### Task 2: Queries + sqlc generate

**Files:**
- Create: `backend/db/queries/transfers.sql`
- Modify: `backend/db/queries/assets.sql` (add `SetAssetOffice`)
- Regenerate: `backend/db/sqlc/`

**Interfaces:**
- Produces (sqlc-generated, consumed by Tasks 4–6): `CreateTransfer`, `GetTransfer`, `ListTransfers`, `CountTransfers`, `ListTransfersByAsset`, `SetTransferShipped`, `SetTransferReceived`, `GetOpenTransferForAsset`, `CountPendingTransferRequestsForAsset`, `SetAssetOffice`. Row type `sqlc.TransferAssetTransfer` already exists in `models.go`.

- [ ] **Step 1: Write the transfer queries**

`backend/db/queries/transfers.sql`:
```sql
-- name: CreateTransfer :one
INSERT INTO transfer.asset_transfers (
  asset_id, from_office_id, to_office_id, to_room_id, status,
  reason, requested_by_id, approved_by_id, request_id
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(from_office_id), sqlc.arg(to_office_id), sqlc.narg(to_room_id),
  'approved', sqlc.narg(reason), sqlc.arg(requested_by_id), sqlc.narg(approved_by_id), sqlc.narg(request_id)
)
RETURNING *;

-- name: GetTransfer :one
-- Scoped: caller must have the from- or to-office in scope.
SELECT * FROM transfer.asset_transfers
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListTransfers :many
SELECT * FROM transfer.asset_transfers
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.transfer_status IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountTransfers :one
SELECT count(*) FROM transfer.asset_transfers
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.transfer_status IS NULL OR status = sqlc.narg(status));

-- name: ListTransfersByAsset :many
-- Per-asset history, scoped by from- or to-office.
SELECT * FROM transfer.asset_transfers
WHERE asset_id = sqlc.arg(asset_id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY created_at DESC;

-- name: SetTransferShipped :one
UPDATE transfer.asset_transfers
SET status = 'in_transit', shipped_date = sqlc.arg(shipped_date)
WHERE id = sqlc.arg(id) AND status = 'approved' AND deleted_at IS NULL
RETURNING *;

-- name: SetTransferReceived :one
UPDATE transfer.asset_transfers
SET status = 'received',
    received_date = sqlc.arg(received_date),
    received_by_id = sqlc.arg(received_by_id),
    bast_no = sqlc.narg(bast_no),
    to_room_id = COALESCE(sqlc.narg(to_room_id), to_room_id)
WHERE id = sqlc.arg(id) AND status = 'in_transit' AND deleted_at IS NULL
RETURNING *;

-- name: GetOpenTransferForAsset :one
-- Guard: an asset may have at most one non-terminal transfer row.
SELECT * FROM transfer.asset_transfers
WHERE asset_id = sqlc.arg(asset_id) AND deleted_at IS NULL
  AND status IN ('approved', 'in_transit')
LIMIT 1;

-- name: CountPendingTransferRequestsForAsset :one
-- Guard: an asset may have at most one pending asset_transfer approval request.
SELECT count(*) FROM approval.requests
WHERE type = 'asset_transfer' AND target_id = sqlc.arg(asset_id)
  AND status = 'pending' AND deleted_at IS NULL;
```

- [ ] **Step 2: Add `SetAssetOffice` to assets.sql**

Append to `backend/db/queries/assets.sql`:
```sql
-- name: SetAssetOffice :one
-- Relocate an asset to a new office/room (used by the transfer receive step).
UPDATE asset.assets
SET office_id = sqlc.arg(office_id), room_id = sqlc.narg(room_id)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc**

Run (from `backend/`): `sqlc generate`
Expected: new methods on `*sqlc.Queries` in `db/sqlc/transfers.sql.go`; `SetAssetOffice` in `assets.sql.go`. No diff to hand-written files.

- [ ] **Step 4: Build**

Run (from `backend/`): `go build ./...`
Expected: clean (generated code compiles).

- [ ] **Step 5: Commit**

```bash
git add backend/db/queries/transfers.sql backend/db/queries/assets.sql backend/db/sqlc/
git commit -m "feat(db): transfer queries + SetAssetOffice (sqlc)"
```

---

### Task 3: Permission catalog entries

**Files:**
- Modify: `backend/internal/authzadmin/catalog.go`

**Interfaces:**
- Consumes: `PermissionGroup` / entry shape already in `catalog.go` (`{key, label}` pairs grouped).
- Produces: `transfer.view` / `transfer.manage` visible in `GET /authz/catalog` and assignable via authz-admin.

- [ ] **Step 1: Read the catalog shape**

Read `backend/internal/authzadmin/catalog.go` lines 22–55 to see the `PermissionGroup` literal (groups with `{ "key", "Label" }` entries, e.g. the asset group holding `{"asset.view", "Lihat aset"}`).

- [ ] **Step 2: Add the transfer group**

In `permissionCatalog` (after the asset/approval group, following the existing struct literal style), add:
```go
{
    Label: "Mutasi Aset",
    Permissions: []Permission{
        {"transfer.view", "Lihat mutasi aset"},
        {"transfer.manage", "Kelola mutasi aset (ajukan/kirim/terima)"},
    },
},
```
(Match the exact field names used by the surrounding entries — if the struct uses positional/other field names, mirror them precisely. Do not invent new fields.)

- [ ] **Step 3: Build + verify no test breaks the catalog count**

Run (from `backend/`): `go build ./... && go test ./internal/authzadmin/...`
Expected: build clean; if a test asserts the catalog’s total permission count, update that expected number to include the two new keys.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/authzadmin/catalog.go
git commit -m "feat(authz): add transfer.view/manage to permission catalog"
```

---

### Task 4: Transfer service + DTO (with unit tests)

**Files:**
- Create: `backend/internal/transfer/service.go`
- Create: `backend/internal/transfer/dto.go`
- Test: `backend/internal/transfer/dto_test.go`

**Interfaces:**
- Consumes: `sqlc.Queries` (Task 2 methods), `*pgxpool.Pool`, `*approval.Service`, `approval.SubmitInput`, `approval.Caller`, `common.InScope`.
- Produces (consumed by Tasks 5–6):
  - Sentinels: `ErrNotFound`, `ErrInvalidState`, `ErrAssetInTransit`, `ErrOutOfScope`, `ErrSameOffice`, `ErrInvalidRef`.
  - `type SubmitInput struct { AssetID uuid.UUID; ToOfficeID uuid.UUID; ToRoomID *uuid.UUID; Reason *string }`
  - `type ShipInput struct { ShippedDate pgtype.Date }`
  - `type ReceiveInput struct { BastNo *string; ReceivedDate pgtype.Date; ToRoomID *uuid.UUID }`
  - `func NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service) *Service`
  - `func (s *Service) Submit(ctx, caller approval.Caller, in SubmitInput) (sqlc.ApprovalRequest, error)`
  - `func (s *Service) Ship(ctx, all bool, ids []uuid.UUID, id uuid.UUID, in ShipInput) (sqlc.TransferAssetTransfer, error)`
  - `func (s *Service) Receive(ctx, all bool, ids []uuid.UUID, receiver uuid.UUID, id uuid.UUID, in ReceiveInput) (before, after sqlc.TransferAssetTransfer, err error)`
  - `func (s *Service) Get(ctx, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.TransferAssetTransfer, error)`
  - `func (s *Service) List(ctx, all bool, ids []uuid.UUID, status string, limit, offset int32) ([]sqlc.TransferAssetTransfer, int64, error)`
  - `func (s *Service) ListByAsset(ctx, assetID uuid.UUID, all bool, ids []uuid.UUID) ([]sqlc.TransferAssetTransfer, error)`
  - `TransferPayload` struct (JSON stored in the approval request) — also consumed by Task 5.

- [ ] **Step 1: Write `dto.go`**

`backend/internal/transfer/dto.go`:
```go
package transfer

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// SubmitRequest is the POST /transfers body.
type SubmitRequest struct {
	AssetID    string  `json:"asset_id" binding:"required,uuid"`
	ToOfficeID string  `json:"to_office_id" binding:"required,uuid"`
	ToRoomID   *string `json:"to_room_id" binding:"omitempty,uuid"`
	Reason     *string `json:"reason"`
}

// ShipRequest is the POST /transfers/:id/ship body (all optional).
type ShipRequest struct {
	ShippedDate *string `json:"shipped_date"` // "2006-01-02"
}

// ReceiveRequest is the POST /transfers/:id/receive body (multipart or JSON).
// The optional BAST file is read from the multipart form, not this struct.
type ReceiveRequest struct {
	BastNo       *string `json:"bast_no" form:"bast_no"`
	ReceivedDate *string `json:"received_date" form:"received_date"`
	ToRoomID     *string `json:"to_room_id" form:"to_room_id" binding:"omitempty,uuid"`
}

// TransferPayload is the JSON stored in approval.requests.payload for asset_transfer.
type TransferPayload struct {
	FromOfficeID string  `json:"from_office_id"`
	ToOfficeID   string  `json:"to_office_id"`
	ToRoomID     *string `json:"to_room_id"`
	Reason       *string `json:"reason"`
}

// toResponse serializes a transfer row for API responses (no sensitive columns).
func toResponse(t sqlc.TransferAssetTransfer) map[string]any {
	return map[string]any{
		"id":              t.ID.String(),
		"asset_id":        t.AssetID.String(),
		"from_office_id":  t.FromOfficeID.String(),
		"to_office_id":    t.ToOfficeID.String(),
		"to_room_id":      common.UUIDPtrStr(t.ToRoomID),
		"status":          string(t.Status),
		"reason":          t.Reason,
		"requested_by_id": t.RequestedByID.String(),
		"approved_by_id":  common.UUIDPtrStr(t.ApprovedByID),
		"shipped_date":    common.DateStr(t.ShippedDate),
		"received_date":   common.DateStr(t.ReceivedDate),
		"received_by_id":  common.UUIDPtrStr(t.ReceivedByID),
		"bast_no":         t.BastNo,
		"request_id":      common.UUIDPtrStr(t.RequestID),
		"created_at":      common.TsStr(t.CreatedAt),
		"updated_at":      common.TsStr(t.UpdatedAt),
	}
}

// marshalPayload builds the approval payload JSON for a submit.
func marshalPayload(fromOffice, toOffice uuid.UUID, toRoom *uuid.UUID, reason *string) ([]byte, error) {
	p := TransferPayload{FromOfficeID: fromOffice.String(), ToOfficeID: toOffice.String(), Reason: reason}
	if toRoom != nil {
		s := toRoom.String()
		p.ToRoomID = &s
	}
	return json.Marshal(p)
}
```

Note: verify `common.DateStr` exists (used to serialize `pgtype.Date`). If the helper is named differently (e.g. `common.DateOrNil`), use that name — grep `internal/masterdata/common` for the `pgtype.Date` → `*string` helper and use the real name. If none exists, add a small `DateStr(d pgtype.Date) *string` to `common` in this task.

- [ ] **Step 2: Write the failing DTO unit test**

`backend/internal/transfer/dto_test.go`:
```go
package transfer

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalPayload_RoundTrip(t *testing.T) {
	from, to, room := uuid.New(), uuid.New(), uuid.New()
	reason := "relokasi cabang"
	raw, err := marshalPayload(from, to, &room, &reason)
	require.NoError(t, err)

	var p TransferPayload
	require.NoError(t, json.Unmarshal(raw, &p))
	assert.Equal(t, from.String(), p.FromOfficeID)
	assert.Equal(t, to.String(), p.ToOfficeID)
	require.NotNil(t, p.ToRoomID)
	assert.Equal(t, room.String(), *p.ToRoomID)
	assert.Equal(t, "relokasi cabang", *p.Reason)
}

func TestMarshalPayload_NilRoom(t *testing.T) {
	raw, err := marshalPayload(uuid.New(), uuid.New(), nil, nil)
	require.NoError(t, err)
	var p TransferPayload
	require.NoError(t, json.Unmarshal(raw, &p))
	assert.Nil(t, p.ToRoomID)
	assert.Nil(t, p.Reason)
}
```

- [ ] **Step 3: Run to verify it fails**

Run (from `backend/`): `go test ./internal/transfer/`
Expected: FAIL — package/`marshalPayload` not yet compiling (service.go missing). (After Step 4 it passes.)

- [ ] **Step 4: Write `service.go`**

`backend/internal/transfer/service.go`:
```go
// Package transfer implements inter-office asset transfer (mutasi): submit via the
// generic approval engine, then ship/receive with BAST + asset relocation. Split
// into dto / service / handler / routes (ADR-0008). The service holds business
// rules + data-scope enforcement (Gin-free); the handler maps HTTP ↔ service.
package transfer

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

// Sentinel errors (mapped to HTTP status by the handler).
var (
	ErrNotFound       = errors.New("transfer: not found")
	ErrInvalidState   = errors.New("transfer: not in a state that allows this action")
	ErrAssetInTransit = errors.New("transfer: asset already has an open transfer")
	ErrOutOfScope     = errors.New("transfer: office out of scope")
	ErrSameOffice     = errors.New("transfer: destination office must differ from origin")
	ErrInvalidRef     = errors.New("transfer: invalid reference")
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

// Service holds data access + business rules for transfers.
type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
	appr *approval.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service) *Service {
	return &Service{q: q, pool: pool, appr: appr}
}

// Input structs.
type SubmitInput struct {
	AssetID    uuid.UUID
	ToOfficeID uuid.UUID
	ToRoomID   *uuid.UUID
	Reason     *string
}
type ShipInput struct{ ShippedDate pgtype.Date }
type ReceiveInput struct {
	BastNo       *string
	ReceivedDate pgtype.Date
	ToRoomID     *uuid.UUID
}

// Submit validates the asset + destination and opens an approval request. No transfer
// row is created here — the asset_transfer executor creates it on final approval.
func (s *Service) Submit(ctx context.Context, caller approval.Caller, in SubmitInput) (sqlc.ApprovalRequest, error) {
	asset, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	// Scope: caller must have the asset's home (from) office in scope.
	if !common.InScope(caller.AllScope, caller.OfficeIDs, asset.OfficeID) {
		return sqlc.ApprovalRequest{}, ErrOutOfScope
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return sqlc.ApprovalRequest{}, ErrInvalidState
	}
	if in.ToOfficeID == asset.OfficeID {
		return sqlc.ApprovalRequest{}, ErrSameOffice
	}
	// Guard: at most one open transfer row + one pending transfer request per asset.
	if _, err := s.q.GetOpenTransferForAsset(ctx, in.AssetID); err == nil {
		return sqlc.ApprovalRequest{}, ErrAssetInTransit
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ApprovalRequest{}, err
	}
	pending, err := s.q.CountPendingTransferRequestsForAsset(ctx, &in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	if pending > 0 {
		return sqlc.ApprovalRequest{}, ErrAssetInTransit
	}

	payload, err := marshalPayload(asset.OfficeID, in.ToOfficeID, in.ToRoomID, in.Reason)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	// Value basis: purchase_cost (book value needs depreciation — deferred).
	amount := "0"
	if asset.PurchaseCost != nil {
		amount = *asset.PurchaseCost
	}
	entity := "asset"
	targetID := in.AssetID
	return s.appr.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetTransfer,
		Amount:       amount,
		OfficeID:     asset.OfficeID,
		TargetEntity: &entity,
		TargetID:     &targetID,
		Payload:      payload,
		Reason:       in.Reason,
		Maker:        caller.UserID,
	})
}

// Ship marks an approved transfer as in_transit. Caller must have from_office in scope.
func (s *Service) Ship(ctx context.Context, all bool, ids []uuid.UUID, id uuid.UUID, in ShipInput) (sqlc.TransferAssetTransfer, error) {
	cur, err := s.q.GetTransfer(ctx, sqlc.GetTransferParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, mapDBError(err)
	}
	if !common.InScope(all, ids, cur.FromOfficeID) {
		return cur, ErrOutOfScope
	}
	if cur.Status != sqlc.SharedTransferStatusApproved {
		return cur, ErrInvalidState
	}
	shipped := in.ShippedDate
	if !shipped.Valid {
		shipped = pgtype.Date{Time: time.Now(), Valid: true}
	}
	out, err := s.q.SetTransferShipped(ctx, sqlc.SetTransferShippedParams{ID: id, ShippedDate: shipped})
	if err != nil {
		return cur, mapDBError(err)
	}
	return out, nil
}

// Receive marks an in_transit transfer as received and relocates the asset, atomically.
// Returns (before, after) for audit diffing. BAST document creation is done by the handler.
func (s *Service) Receive(ctx context.Context, all bool, ids []uuid.UUID, receiver, id uuid.UUID, in ReceiveInput) (before, after sqlc.TransferAssetTransfer, err error) {
	before, err = s.q.GetTransfer(ctx, sqlc.GetTransferParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, before, mapDBError(err)
	}
	if !common.InScope(all, ids, before.ToOfficeID) {
		return before, before, ErrOutOfScope
	}
	if before.Status != sqlc.SharedTransferStatusInTransit {
		return before, before, ErrInvalidState
	}
	recvDate := in.ReceivedDate
	if !recvDate.Valid {
		recvDate = pgtype.Date{Time: time.Now(), Valid: true}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return before, before, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	after, err = qtx.SetTransferReceived(ctx, sqlc.SetTransferReceivedParams{
		ID:           id,
		ReceivedDate: recvDate,
		ReceivedByID: &receiver,
		BastNo:       in.BastNo,
		ToRoomID:     in.ToRoomID,
	})
	if err != nil {
		return before, before, mapDBError(err)
	}
	// Relocate the asset to the destination office/room.
	if _, err = qtx.SetAssetOffice(ctx, sqlc.SetAssetOfficeParams{
		ID:       before.AssetID,
		OfficeID: before.ToOfficeID,
		RoomID:   after.ToRoomID,
	}); err != nil {
		return before, before, mapDBError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return before, before, err
	}
	return before, after, nil
}

// Get returns one scoped transfer.
func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.TransferAssetTransfer, error) {
	t, err := s.q.GetTransfer(ctx, sqlc.GetTransferParams{ID: id, AllScope: all, OfficeIds: ids})
	return t, mapDBError(err)
}

// List returns a scoped, paginated page + total. Empty status = no filter.
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, status string, limit, offset int32) ([]sqlc.TransferAssetTransfer, int64, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	var st *sqlc.SharedTransferStatus
	if status != "" {
		v := sqlc.SharedTransferStatus(status)
		st = &v
	}
	rows, err := s.q.ListTransfers(ctx, sqlc.ListTransfersParams{AllScope: all, OfficeIds: ids, Status: st, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountTransfers(ctx, sqlc.CountTransfersParams{AllScope: all, OfficeIds: ids, Status: st})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// ListByAsset returns a scoped transfer history for one asset.
func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, all bool, ids []uuid.UUID) ([]sqlc.TransferAssetTransfer, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListTransfersByAsset(ctx, sqlc.ListTransfersByAssetParams{AssetID: assetID, AllScope: all, OfficeIds: ids})
	return rows, mapDBError(err)
}
```

Notes for the implementer:
- Verify the exact sqlc param struct field names after `sqlc generate` (e.g. `ListTransfersParams{AllScope, OfficeIds, Status, Lim, Off}`) and adjust if sqlc emits different casing.
- `common.InScope`, `common.UUIDPtrStr`, `common.TsStr` exist (used across masterdata). Confirm `common.DateStr` — see the Step-1 note.
- `sqlc.SharedAssetStatusAvailable` — confirm the constant name via `grep SharedAssetStatus db/sqlc/models.go`; use the real constant for `available`.

- [ ] **Step 5: Run the unit tests + build**

Run (from `backend/`): `go build ./... && go test ./internal/transfer/`
Expected: build clean; `TestMarshalPayload_*` PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/transfer/service.go backend/internal/transfer/dto.go backend/internal/transfer/dto_test.go
git commit -m "feat(transfer): service + dto for mutasi (submit/ship/receive/list)"
```

---

### Task 5: `asset_transfer` approval executor

**Files:**
- Create: `backend/internal/transfer/executor.go`

**Interfaces:**
- Consumes: `TransferPayload` (Task 4), `approval.Executor`, `sqlc.Queries.{GetAsset,GetOpenTransferForAsset,CreateTransfer}`.
- Produces: `func (s *Service) Executor() approval.Executor` — registered in Task 7 via `approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetTransfer, transferSvc.Executor())`. Creates the `transfer.asset_transfers` row (`status=approved`) inside the approval-commit tx.

- [ ] **Step 1: Write `executor.go`**

`backend/internal/transfer/executor.go`:
```go
package transfer

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// transferExec creates the transfer row on final approval, inside the commit tx.
type transferExec struct{ s *Service }

func (e transferExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return approval.ErrInvalidRef
	}
	var p TransferPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return approval.ErrInvalidRef
	}
	fromOffice, err := uuid.Parse(p.FromOfficeID)
	if err != nil {
		return approval.ErrInvalidRef
	}
	toOffice, err := uuid.Parse(p.ToOfficeID)
	if err != nil {
		return approval.ErrInvalidRef
	}

	// Defense-in-depth: the payload's from-office must match the request office (set at
	// submit from the asset's home office), and the asset must still live there.
	if req.OfficeID == nil || fromOffice != *req.OfficeID {
		return approval.ErrInvalidRef
	}
	asset, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		return approval.ErrInvalidRef
	}
	if asset.OfficeID != fromOffice || toOffice == fromOffice {
		return approval.ErrInvalidRef
	}
	// Guard: refuse a second open transfer for the same asset.
	if _, err := qtx.GetOpenTransferForAsset(ctx, *req.TargetID); err == nil {
		return approval.ErrConflict
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var toRoom *uuid.UUID
	if p.ToRoomID != nil {
		r, perr := uuid.Parse(*p.ToRoomID)
		if perr != nil {
			return approval.ErrInvalidRef
		}
		toRoom = &r
	}
	reqID := req.ID
	approver := req.DecidedByID
	_, err = qtx.CreateTransfer(ctx, sqlc.CreateTransferParams{
		AssetID:       *req.TargetID,
		FromOfficeID:  fromOffice,
		ToOfficeID:    toOffice,
		ToRoomID:      toRoom,
		Reason:        p.Reason,
		RequestedByID: req.RequestedByID,
		ApprovedByID:  approver,
		RequestID:     &reqID,
	})
	return err
}

// Executor returns the asset_transfer approval executor.
func (s *Service) Executor() approval.Executor { return transferExec{s} }
```

Notes: confirm `approval.ErrConflict` / `approval.ErrInvalidRef` are exported (they are — see `approval/service.go`). Confirm `CreateTransferParams` field names after `sqlc generate` (`ApprovedByID *uuid.UUID`, `RequestID *uuid.UUID`, `ToRoomID *uuid.UUID`, `Reason *string`).

- [ ] **Step 2: Build**

Run (from `backend/`): `go build ./... && go vet ./...`
Expected: clean. (Behavioral coverage lands in Task 8 integration tests.)

- [ ] **Step 3: Commit**

```bash
git add backend/internal/transfer/executor.go
git commit -m "feat(transfer): asset_transfer approval executor"
```

---

### Task 6: Handler + routes

**Files:**
- Create: `backend/internal/transfer/handler.go`
- Create: `backend/internal/transfer/routes.go`

**Interfaces:**
- Consumes: `*transfer.Service` (Task 4), `common.ScopedDeps`, `*audit.Service`, `*asset.Service` (for BAST on receive), `approval.Caller`, middleware `authMW`/`requireManage`/`requireView`.
- Produces: `NewHandler(...)`, `RegisterRoutes(rg, h, authMW, requireManage, requireView)` — called in Task 7.

- [ ] **Step 1: Write `handler.go`**

`backend/internal/transfer/handler.go`:
```go
package transfer

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

const scopeModule = "transfers"

// Handler maps HTTP ↔ the transfer service, orchestrating BAST document creation on receive.
type Handler struct {
	svc      *Service
	assetSvc *asset.Service
	scoped   common.ScopedDeps
	aud      *audit.Service
}

func NewHandler(svc *Service, assetSvc *asset.Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler {
	return &Handler{svc: svc, assetSvc: assetSvc, scoped: common.ScopedDeps{Q: q, Scope: scope}, aud: aud}
}

func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidState):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAssetInTransit), errors.Is(err, ErrSameOffice), errors.Is(err, ErrInvalidRef):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

func parseDate(s *string) (pgtype.Date, error) {
	if s == nil || *s == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func (h *Handler) caller(c *gin.Context) (approval.Caller, bool, []uuid.UUID, error) {
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		return approval.Caller{}, false, nil, err
	}
	uid := c.MustGet(string(middleware.CtxUserID)).(uuid.UUID)
	rid := c.MustGet(string(middleware.CtxRoleID)).(uuid.UUID)
	return approval.Caller{UserID: uid, RoleID: rid, AllScope: all, OfficeIDs: ids}, all, ids, nil
}
```
Note: the exact way to read the user/role id from the Gin context — mirror how `internal/approval/handler.go` builds its `approval.Caller` (it already resolves `CtxUserID`/`CtxRoleID` + scope). Copy that exact code rather than guessing the context-key accessors; adjust the helper above to match.

Continue `handler.go` with the endpoint methods:
```go
func (h *Handler) submit(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, _, _, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	in := SubmitInput{
		AssetID:    uuid.MustParse(req.AssetID),
		ToOfficeID: uuid.MustParse(req.ToOfficeID),
		Reason:     req.Reason,
	}
	if req.ToRoomID != nil {
		r := uuid.MustParse(*req.ToRoomID)
		in.ToRoomID = &r
	}
	out, err := h.svc.Submit(c.Request.Context(), caller, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "transfers", out.ID, out.OfficeID, audit.Diff(nil, map[string]any{"request_id": out.ID.String(), "type": "asset_transfer", "asset_id": req.AssetID}))
	c.JSON(http.StatusCreated, gin.H{"request_id": out.ID.String(), "status": string(out.Status)})
}

func (h *Handler) ship(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body ShipRequest
	_ = c.ShouldBindJSON(&body) // body optional
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	shipped, derr := parseDate(body.ShippedDate)
	if derr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid shipped_date"})
		return
	}
	out, err := h.svc.Ship(c.Request.Context(), all, ids, id, ShipInput{ShippedDate: shipped})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "transfers", out.ID, &out.FromOfficeID, audit.Diff(map[string]any{"status": "approved"}, map[string]any{"status": "in_transit"}))
	c.JSON(http.StatusOK, toResponse(out))
}

func (h *Handler) receive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body ReceiveRequest
	_ = c.ShouldBind(&body) // multipart or JSON; file read separately
	caller, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	recvDate, derr := parseDate(body.ReceivedDate)
	if derr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid received_date"})
		return
	}
	in := ReceiveInput{BastNo: body.BastNo, ReceivedDate: recvDate}
	if body.ToRoomID != nil {
		r := uuid.MustParse(*body.ToRoomID)
		in.ToRoomID = &r
	}
	before, after, err := h.svc.Receive(c.Request.Context(), all, ids, caller.UserID, id, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "transfers", after.ID, &after.ToOfficeID, audit.Diff(toResponse(before), toResponse(after)))

	// BAST document (best-effort): metadata row + optional MinIO file. Failures here do
	// not roll back the physical receive (asset already relocated + bast_no recorded).
	h.recordBAST(c, after)
	c.JSON(http.StatusOK, toResponse(after))
}

// recordBAST creates an asset_documents(bast_transfer) row and, if a file part is present,
// stores it in MinIO via the asset document service.
func (h *Handler) recordBAST(c *gin.Context, t sqlc.TransferAssetTransfer) {
	uid := t.ReceivedByID
	doc, err := h.assetSvc.CreateDocument(c.Request.Context(), asset.DocumentInput{
		AssetID:          t.AssetID,
		DocType:          sqlc.SharedAssetDocumentTypeBastTransfer,
		DocNo:            t.BastNo,
		DocDate:          t.ReceivedDate,
		RelatedRequestID: t.RequestID,
		CreatedBy:        deref(uid),
	})
	if err != nil {
		return // soft-fail; the transfer already succeeded
	}
	fh, ferr := c.FormFile("file")
	if ferr != nil || fh == nil {
		return // no file uploaded
	}
	f, oerr := fh.Open()
	if oerr != nil {
		return
	}
	defer f.Close()
	data, rerr := io.ReadAll(f)
	if rerr != nil {
		return
	}
	_, _ = h.assetSvc.AttachFile(c.Request.Context(), doc, asset.DocumentFileInput{
		ContentType: fh.Header.Get("Content-Type"),
		Data:        data,
	})
}

func deref(u *uuid.UUID) uuid.UUID {
	if u == nil {
		return uuid.Nil
	}
	return *u
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
	t, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(t))
}

func (h *Handler) list(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	status := c.Query("status")
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.List(c.Request.Context(), all, ids, status, limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		data = append(data, toResponse(t))
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
	for _, t := range rows {
		data = append(data, toResponse(t))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
```
Notes: verify `audit.Record`/`audit.Diff`/`audit.ActionCreate|Update` signatures against `internal/audit` (the office/asset handlers call them — copy the exact call shape; the 5th arg office pointer type may be `*uuid.UUID`). `out.OfficeID` on the approval request is `*uuid.UUID` — pass it directly where an office pointer is expected.

- [ ] **Step 2: Write `routes.go`**

`backend/internal/transfer/routes.go`:
```go
package transfer

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the transfer endpoints. Reads require transfer.view; writes
// require transfer.manage. Per-asset history is mounted under /assets/:id/transfers.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView gin.HandlerFunc) {
	g := rg.Group("/transfers")
	g.GET("", authMW, requireView, h.list)
	g.GET("/:id", authMW, requireView, h.get)
	g.POST("", authMW, requireManage, h.submit)
	g.POST("/:id/ship", authMW, requireManage, h.ship)
	g.POST("/:id/receive", authMW, requireManage, h.receive)

	rg.GET("/assets/:id/transfers", authMW, requireView, h.listByAsset)
}
```
Note: confirm no route conflict on `/assets/:id/...` with the asset module’s existing `/assets/:id` group registration in Gin (Gin allows sibling routes but not conflicting wildcards on the same path segment). If `asset.RegisterRoutes` already defines `/assets/:id/<sub>` groups, mounting one more sibling `GET /assets/:id/transfers` is fine. If Gin panics on a wildcard conflict, mount history as `GET /transfers/by-asset/:id` instead and note the change.

- [ ] **Step 3: Build**

Run (from `backend/`): `go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/transfer/handler.go backend/internal/transfer/routes.go
git commit -m "feat(transfer): HTTP handler + routes (submit/ship/receive/list/history)"
```

---

### Task 7: Wire into NewRouter + register executor

**Files:**
- Modify: `backend/internal/server/router.go`

**Interfaces:**
- Consumes: `transfer.NewService`, `transfer.NewHandler`, `transfer.RegisterRoutes`, `transferSvc.Executor()`, existing `assetSvc`, `approvalSvc`, `permSvc`, `scopeSvc`, `auditSvc`, `queries`, `d.Pool`.

- [ ] **Step 1: Add the import**

In `backend/internal/server/router.go` imports, add:
```go
	"github.com/ragbuaj/inventra/internal/transfer"
```

- [ ] **Step 2: Construct + register (after the approval block, before authzAdmin)**

The approval service must exist first (transfer.Submit needs it) and the executor must be registered on it. Replace the approval construction block so the transfer service is built from `approvalSvc`, then register its executor, then mount routes. Insert after line `approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeValuationExclusion, assetSvc.ExclusionExecutor())` and before `approvalHandler := ...`:
```go
			transferSvc := transfer.NewService(queries, d.Pool, approvalSvc)
			approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetTransfer, transferSvc.Executor())
```
Then after the `approval.RegisterRoutes(...)` line, add:
```go
			transferHandler := transfer.NewHandler(transferSvc, assetSvc, scopeSvc, queries, auditSvc)
			transfer.RegisterRoutes(api, transferHandler,
				requireAuth,
				middleware.RequirePermission(permSvc, "transfer.manage"),
				middleware.RequirePermission(permSvc, "transfer.view"),
			)
```

- [ ] **Step 3: Build + vet + unit tests**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./...`
Expected: clean; existing unit suite green.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/server/router.go
git commit -m "feat(transfer): wire module + register asset_transfer executor in NewRouter"
```

---

### Task 8: Integration tests

**Files:**
- Create: `backend/internal/transfer/transfer_integration_test.go`

**Interfaces:**
- Consumes: `internal/testsupport` (Postgres/Redis/MinIO containers, `Reset`, `SeedOfficeTree`, seed helpers), `transfer.NewService`, `transfer.transferExec` via `Executor()`, `approval.NewService`, `asset.NewService`, real `sqlc.Queries`.

- [ ] **Step 1: Study the harness**

Read `backend/internal/approval/integration_test.go` (how it builds `approval.Service`, seeds thresholds, submits, and drives `Decide` through the chain + registers executors) and `backend/internal/asset/document_integration_test.go` (MinIO testcontainer + `asset.NewService` with storage). Reuse their setup helpers verbatim. Read `backend/internal/testsupport/` for `SeedOfficeTree`, `Reset`, and any asset/seed helpers.

- [ ] **Step 2: Write the happy-path test (submit → approve → ship → receive)**

Create `backend/internal/transfer/transfer_integration_test.go` (`//go:build integration`). Build a `transfer.Service` + registered `asset_transfer` executor + `asset.Service` (MinIO). Seed an office tree (`from`, `to` under the same subtree so a single approver is eligible), a category, and an `available` asset in `from`. Assert:
```
// submit
req, err := tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: toOffice, Reason: strptr("relok")})
require.NoError(t, err)
// no transfer row yet
_, err = q.GetOpenTransferForAsset(ctx, assetID)
require.ErrorIs(t, err, pgx.ErrNoRows)
// approve (through the chain) — mirror approval integration_test's Decide loop
finalReq := approveThroughChain(t, apprSvc, req.ID, checkerCaller)
require.Equal(t, sqlc.SharedRequestStatusApproved, finalReq.Status)
// executor created the transfer row, status=approved
row, err := q.GetOpenTransferForAsset(ctx, assetID)
require.NoError(t, err)
assert.Equal(t, sqlc.SharedTransferStatusApproved, row.Status)
// ship
shipped, err := tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
require.NoError(t, err)
assert.Equal(t, sqlc.SharedTransferStatusInTransit, shipped.Status)
require.True(t, shipped.ShippedDate.Valid)
// receive → asset moved
_, after, err := tsvc.Receive(ctx, true, nil, receiverID, row.ID, transfer.ReceiveInput{BastNo: strptr("BAST-001")})
require.NoError(t, err)
assert.Equal(t, sqlc.SharedTransferStatusReceived, after.Status)
movedAsset, err := q.GetAsset(ctx, assetID)
require.NoError(t, err)
assert.Equal(t, toOffice, movedAsset.OfficeID)
```
(Use `true, nil` for `allScope` in service calls that don't test scope; use explicit `officeIDs` in the scope tests below.)

- [ ] **Step 3: Write the reject test (no transfer row)**

```
req, _ := tsvc.Submit(ctx, makerCaller, in)
rejectFinalStep(t, apprSvc, req.ID, checkerCaller) // Decide(approve=false)
_, err := q.GetOpenTransferForAsset(ctx, assetID)
require.ErrorIs(t, err, pgx.ErrNoRows)
a, _ := q.GetAsset(ctx, assetID)
assert.Equal(t, fromOffice, a.OfficeID) // unchanged
```

- [ ] **Step 4: Write the guard + validation tests**

- Submit when an open transfer exists → `ErrAssetInTransit`:
```
// after a first submit is approved (open row exists), a second submit fails
_, err := tsvc.Submit(ctx, makerCaller, in)
require.ErrorIs(t, err, transfer.ErrAssetInTransit)
```
- Submit with `to == from` → `ErrSameOffice`.
- Submit for an out-of-scope asset (caller scope = `[otherOffice]`) → `ErrOutOfScope`.

- [ ] **Step 5: Write the scope + state-machine tests**

- Ship with caller scope not covering `from_office` → `ErrOutOfScope`.
- Receive with caller scope not covering `to_office` → `ErrOutOfScope`.
- Ship a non-`approved` row (e.g. already `in_transit`) → `ErrInvalidState`.
- Receive a non-`in_transit` row (e.g. still `approved`) → `ErrInvalidState`.

- [ ] **Step 6: Write the BAST test**

After a successful receive, assert an `asset.asset_documents` row exists with `doc_type='bast_transfer'` and `related_request_id` set:
```
docs, err := q.ListAssetDocuments(ctx, assetID)
require.NoError(t, err)
require.Len(t, docs, 1)
assert.Equal(t, sqlc.SharedAssetDocumentTypeBastTransfer, docs[0].DocType)
```
(BAST metadata is created by the handler; for a service-level integration test, call the asset service `CreateDocument` in the test to mirror the handler, OR add a thin `Service`-level BAST hook. Prefer testing metadata creation by invoking the same `assetSvc.CreateDocument` the handler uses, asserting the row + type.)

- [ ] **Step 7: Write the history test**

`ListByAsset` returns the asset's transfers newest-first, scoped:
```
rows, err := tsvc.ListByAsset(ctx, assetID, true, nil)
require.NoError(t, err)
require.GreaterOrEqual(t, len(rows), 1)
```

- [ ] **Step 8: Run the integration suite**

Ensure infra is up: `docker compose -f docker-compose.dev.yml up -d`.
Run (from `backend/`): `go test -tags=integration ./internal/transfer/ -v`
Expected: all transfer integration tests PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/transfer/transfer_integration_test.go
git commit -m "test(transfer): integration coverage for mutasi lifecycle + scope + BAST"
```

---

### Task 9: OpenAPI + full gate

**Files:**
- Modify: `backend/api/openapi.yaml`

**Interfaces:**
- Produces: documented `/transfers`, `/transfers/{id}`, `/transfers/{id}/ship`, `/transfers/{id}/receive`, `/assets/{id}/transfers` paths + `Transfer` schema. No new consumers.

- [ ] **Step 1: Add the schema + paths**

In `backend/api/openapi.yaml`, add a `Transfer` schema under `components/schemas` mirroring `toResponse` (fields: `id, asset_id, from_office_id, to_office_id, to_room_id, status, reason, requested_by_id, approved_by_id, shipped_date, received_date, received_by_id, bast_no, request_id, created_at, updated_at`; `status` enum `approved|in_transit|received`). Add paths:
- `POST /transfers` — body `{asset_id*(uuid), to_office_id*(uuid), to_room_id?(uuid), reason?}`; `201` → `{request_id, status}`.
- `GET /transfers` — query `status?`, `limit?`, `offset?`; `200` → `{data: [Transfer], total, limit, offset}`.
- `GET /transfers/{id}` — `200` → `Transfer`; `404`.
- `POST /transfers/{id}/ship` — body `{shipped_date?}`; `200` → `Transfer`; `409`.
- `POST /transfers/{id}/receive` — `multipart/form-data` `{bast_no?, received_date?, to_room_id?, file?(binary)}`; `200` → `Transfer`; `409`.
- `GET /assets/{id}/transfers` — `200` → `{data: [Transfer]}`.
Mark writes with the `transfer.manage` and reads with `transfer.view` scopes in the description; secure with the existing bearer `securityScheme`. Follow the file’s existing style for parameters/responses (copy an existing list endpoint like `/offices` as a template).

- [ ] **Step 2: Spectral lint**

Run (from repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors.

- [ ] **Step 3: Full backend gate**

Run (from `backend/`):
```bash
go build ./... && go vet ./... && go test ./...
go test -tags=integration ./...
```
Expected: all green. (Integration needs dev infra up.)

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`: tick **Asset transfer (mutasi)** under Bank-FAM as ✅ (backend: module + `asset_transfer` executor + BAST via asset documents + integration tests; frontend pending a mockup). Refresh the "▶ Next session — start here" block to point at the next FAM subsystem (stock opname or disposal) or a frontend mutasi mockup.

- [ ] **Step 5: Commit**

```bash
git add backend/api/openapi.yaml docs/PROGRESS.md
git commit -m "docs(transfer): OpenAPI paths + PROGRESS for mutasi module"
```

---

## Self-Review

**Spec coverage:**
- bagian 3 lifecycle (submit→approve→ship→receive; no row on reject/cancel) → Tasks 4 (Submit), 5 (executor creates row), 6 (ship/receive). ✓
- bagian 4 module four-file split → Tasks 4 (service/dto), 5 (executor), 6 (handler/routes). ✓
- bagian 5 endpoints + permission/scope table → Task 6 (handlers) + Task 7 (RequirePermission wiring) + Task 1 (perms/scope seed). ✓
- bagian 6 submit validation (scope, available, no open transfer, to≠from, room belongs) → Task 4 Submit (room-belongs is enforced by FK at executor/`SetAssetOffice`; explicit room-office check is optional — noted). ✓
- bagian 7 executor (payload snapshot, defense-in-depth, create approved row) → Task 5. ✓
- bagian 8 receive + BAST (asset move tx, asset_documents + MinIO, file optional) → Task 6 (`receive`+`recordBAST`), Task 8 Step 6. ✓
- bagian 9 authz (transfer.manage/view, scope module "transfers", read+write) → Tasks 1, 6, 7. ✓
- bagian 10 value basis (purchase_cost + seeded band) → Task 1 (band), Task 4 (amount). ✓
- bagian 11 DB/infra (seed migration, queries, SetAssetOffice, catalog, OpenAPI, wiring) → Tasks 1,2,3,7,9. ✓
- bagian 12 testing (happy path, reject, guards, scope, state machine, history, BAST) → Task 8. ✓

**Placeholder scan:** Each code step shows complete code. Notes that say "confirm the exact sqlc field name / audit signature / context accessor" point to a named canonical file to copy from (`approval/handler.go`, `masterdata/office/handler.go`, generated `db/sqlc`) — these are verification steps against real code, not deferred logic. No "TODO/TBD/add validation".

**Type consistency:** `Service` methods, `SubmitInput`/`ShipInput`/`ReceiveInput`, `TransferPayload`, and sentinel names are identical across Tasks 4/5/6/7/8. `Executor()` (Task 5) matches the registration call (Task 7). `toResponse` used by handler (Task 6) is defined in dto (Task 4). Query/param names (`GetTransferParams`, `ListTransfersParams{...,Lim,Off}`, `CreateTransferParams`, `SetAssetOfficeParams`) are used consistently and flagged for post-`sqlc generate` verification. `sqlc.SharedTransferStatus{Approved,InTransit,Received}` and `SharedRequestTypeAssetTransfer` / `SharedAssetDocumentTypeBastTransfer` match `models.go`.

**Ambiguity resolved:** `/assets/:id/transfers` route-conflict fallback documented (Task 6 Step 2). BAST file-optional + best-effort documented (Task 6). `common.DateStr` existence flagged with a fallback (Task 4 Step 1).
