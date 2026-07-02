# Asset Transfer (Mutasi) — Backend Module Design

| | |
|---|---|
| **Feature** | Inter-office asset transfer (mutasi) — PRD v1.1 §3.8 |
| **Date** | 2026-07-02 |
| **Scope** | **Backend-only** (no frontend mockup exists yet) |
| **Status** | Approved — ready for implementation plan |
| **Module** | `backend/internal/transfer/` (new, four-file split per ADR-0008) |

> First of the three Bank-FAM operational subsystems (transfer / stock opname / disposal).
> Each is an independent spec → plan → implementation cycle. This spec covers **transfer only**.

## 1. Goal

Move a fixed asset from one office to another through the bank's **value-tiered
maker-checker** approval, then a physical **ship → receive** handover that records a
**BAST** (Berita Acara Serah Terima) and relocates the asset. The `transfer.asset_transfers`
table + enums already exist (migration `000015_fam_tables`); this cycle builds the **backend
module, executor, queries, seed, and OpenAPI**.

## 2. Non-goals (this cycle)

- **Frontend / mutasi screen** — no `docs/design/*.dc.html` mockup exists; built in a later
  cycle once a mockup lands. Pending mutasi already surface in the existing Pengajuan inbox.
- **Gain/loss & book-value basis** — threshold amount uses `purchase_cost` (book value needs
  the depreciation module, not yet built). Documented; revisit when depreciation lands.
- **Strict SoD on ship/receive** beyond data-scope (the approval decision already enforces
  maker ≠ checker). Custodian ship/receive is gated by office scope only.

## 3. Lifecycle & state model

The **approval request** owns the pre-approval state; the **transfer row** owns the physical
lifecycle. The transfer row is created only on final approval, so rejected/cancelled mutasi
never create a transfer row (no dual-status drift).

| Step | Endpoint | Actor | Effect |
|---|---|---|---|
| **Submit** | `POST /transfers` | maker (mgr / kepala) | Creates `approval.requests` row (type `asset_transfer`, `pending`) + its approval chain. **No transfer row yet.** Visible in the existing Pengajuan inbox/list. |
| **Approve** (final step) | `POST /requests/:id/approve` (existing) | checker | `asset_transfer` **executor** creates the `transfer.asset_transfers` row (`status=approved`) inside the approval-commit tx. |
| **Reject / Cancel** | `POST /requests/:id/reject` / `/cancel` (existing) | checker / maker | Request → `rejected`/`cancelled`. **No transfer row created** (executor runs only on approval). |
| **Ship** | `POST /transfers/:id/ship` | from-office mgr | `approved → in_transit`; sets `shipped_date`. |
| **Receive** | `POST /transfers/:id/receive` | to-office mgr | `in_transit → received`; sets `received_date`, `received_by_id`, `bast_no`, `to_room_id`; **moves the asset** (`assets.office_id → to_office_id`, `assets.room_id → to_room_id`); creates a `asset.asset_documents` BAST row (+ optional MinIO file). |

Enum `shared.transfer_status = ('pending','approved','in_transit','received','rejected','cancelled')`.
The transfer row uses `approved → in_transit → received`. `pending`/`rejected`/`cancelled`
are represented by the linked approval request, not a transfer row.

Invalid transitions (e.g. ship a non-`approved` row, receive a non-`in_transit` row) return a
sentinel → HTTP 409.

## 4. Module layout — `internal/transfer/` (four-file split)

- **`service.go`** — business rules + sentinel errors (`ErrNotFound`, `ErrInvalidState`,
  `ErrAssetInTransit`, `ErrOutOfScope`, …) + `mapDBError`. Gin-free. Holds `*sqlc.Queries`
  and (for BAST on receive) a reference to the asset document service. Methods:
  `Submit` (validate + delegate to `approval.Submit`), `Ship`, `Receive`, `Get`, `List`,
  `ListByAsset`. Plus `Executor()` returning the `asset_transfer` approval executor.
- **`dto.go`** — `SubmitRequest` (`asset_id`, `to_office_id`, `to_room_id?`, `reason?`),
  `ShipRequest` (optional `shipped_date`), `ReceiveRequest` (`bast_no`, `received_date?`,
  `to_room_id?`; the file is a multipart part, not JSON), response serializers
  (`toResponse`). No sensitive columns.
- **`handler.go`** — `Handler` (service + `common.ScopedDeps` + `*audit.Service`). Each
  method: bind → resolve scope → call service → serialize → respond; `svcError` maps
  sentinels to HTTP status; records audit entries (`transfers` entity).
- **`routes.go`** — `RegisterRoutes(rg, h, authMW, requireManage, requireView)` mounting the
  endpoints with per-endpoint permission + scope.

The `asset_transfer` **executor** lives in `internal/transfer/` (a small type implementing
`approval.Executor`) and is registered in `NewRouter` via
`approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetTransfer, transferSvc.Executor())`.

## 5. Endpoints (under `/api/v1`)

| Method | Path | Permission | Scope check |
|---|---|---|---|
| `POST` | `/transfers` | `transfer.manage` | asset's `office_id` (from-office) in caller scope |
| `POST` | `/transfers/:id/ship` | `transfer.manage` | `from_office_id` in caller scope |
| `POST` | `/transfers/:id/receive` | `transfer.manage` | `to_office_id` in caller scope |
| `GET` | `/transfers` | `transfer.view` | list filtered: `from_office_id` **or** `to_office_id` in scope |
| `GET` | `/transfers/:id` | `transfer.view` | from **or** to office in scope |
| `GET` | `/assets/:id/transfers` | `transfer.view` | asset's office in scope (history) |

List envelope `{data, total, limit, offset}` (`limit` clamped 1–100); single-row flat;
ship/receive return the updated transfer. Cancel/reject **pre-approval** use the existing
`/requests/:id/cancel|reject`.

## 6. Submit flow (validation + delegation)

`POST /transfers` (`transfer.manage`):
1. Bind `SubmitRequest`; parse UUIDs.
2. Load the asset (scoped `GetAsset`). 404 if outside scope / missing.
3. Validate: asset `status = available` (not already `in_transit`/`disposed`/…); **no open
   transfer** for this asset (a pending `asset_transfer` request or a non-`received` transfer
   row → `ErrAssetInTransit`); `to_office_id` exists and differs from the asset's office;
   `to_room_id` (if given) belongs to `to_office_id`.
4. `from_office_id = asset.office_id`; `amount = asset.purchase_cost` (empty → `"0"`).
5. Call `approval.Submit`:
   ```
   Type:         asset_transfer
   OfficeID:     from_office_id          // drives tier resolution + inbox scope
   Amount:       purchase_cost
   TargetEntity: "asset"
   TargetID:     asset_id
   Payload:      {to_office_id, to_room_id?, reason?, from_office_id}
   Maker:        caller
   ```
6. Return the created approval request (so the client can track it in the inbox).

Snapshotting `from_office_id`/`to_office_id`/`to_room_id`/`reason` in the payload means the
executor does not re-read a possibly-changed `asset.office_id` at approval time.

## 7. Executor (`asset_transfer`)

Runs inside the approval-commit tx on **final approval**, receiving the `sqlc.ApprovalRequest`:
1. Unmarshal `req.Payload` → `{to_office_id, to_room_id?, reason?, from_office_id}`.
2. `asset_id = *req.TargetID`.
3. Re-validate defensively: asset exists; still `available`; `to_office ≠ from_office`
   (cross-office / stale state → `approval.ErrInvalidRef`, mirroring the disposal executor).
4. `CreateTransfer` row: `status=approved`, `request_id=req.ID`,
   `requested_by_id=req.RequestedByID`, `approved_by_id=req.DecidedByID`, `from/to_office`,
   `to_room_id`, `reason`.

The executor does **not** move the asset — that happens at receive.

## 8. Receive flow + BAST

`POST /transfers/:id/receive` (`transfer.manage`, multipart):
1. Load transfer (scoped by `to_office_id`). Must be `in_transit` → else 409.
2. Validate `to_room_id` (if overriding) belongs to `to_office_id`.
3. In a tx: `SetTransferReceived` (`status=received`, `received_date`, `received_by_id`,
   `bast_no`, `to_room_id`); `SetAssetOffice` (`assets.office_id=to_office_id`,
   `room_id=to_room_id`).
4. BAST document: create an `asset.asset_documents` row (`doc_type=bast_transfer`,
   `related_transfer_id=id`, `doc_no=bast_no`, `doc_date=received_date`,
   `counterparty=<from-office name>`) via the **existing** `asset` document service
   (`CreateDocument`); if a file part is present, `AttachFile` stores it in MinIO
   (reusing the asset module's `Storage` + validation). On MinIO failure the metadata row
   still commits (best-effort file, matching the asset-documents pattern) — OR the whole
   receive rolls back if the file is mandatory. **Decision: file is optional**; a failed
   upload does not block the physical receive (bast_no already recorded); surface a soft error.

Ship (`POST /transfers/:id/ship`): load (scoped by `from_office_id`), must be `approved`,
set `status=in_transit` + `shipped_date` (body or today).

## 9. Authorization

- **Permissions (new):** `transfer.manage` (submit/ship/receive) and `transfer.view` (read).
  Added to the `authzadmin` permission **catalog** and seeded into `role_permissions` for the
  operational roles (Manager, Kepala Unit, Kepala Kanwil; Superadmin via `*`). Seeded in a new
  migration.
- **Data scope:** module string `"transfers"`. Resolved via
  `common.ScopedDeps.CallerOfficeScope(c, "transfers")` → `(allScope, officeIDs)`. Falls back
  to the per-role default policy (`module='*'`) — no dedicated seed row required, though the
  migration may seed sensible `transfers` defaults. Enforced on **read and write**:
  submit/ship → from-office in scope; receive → to-office in scope; list/get/history → from
  **or** to office in scope (new scope-aware queries).
- **SoD:** enforced by the approval engine (maker ≠ checker, no repeat approver). Ship/receive
  are physical custodian actions gated by office scope only.

## 10. Value / threshold basis

`amount = asset.purchase_cost`. A new migration seeds an `asset_transfer`
`approval.approval_thresholds` band (a single `office`-level step for any amount, mirroring the
placeholder bands in `000016`) so `approval.Submit` resolves a non-empty chain; otherwise
`ErrNoThreshold`. Bands are reconfigurable later via the existing authz-admin thresholds API.
Book-value basis is deferred to the depreciation cycle.

## 11. Database / infra work

- **No new tables** (`000015` already created them). New migration `000020_transfer_seed`
  (highest existing is `000019_employee_phone`; re-verify at implementation time):
  - seed `asset_transfer` `approval_thresholds` band(s);
  - insert `transfer.manage` / `transfer.view` into `identity.role_permissions` for the
    operational roles;
  - (optional) seed `data_scope_policies` `transfers` defaults.
- **Queries** `db/queries/transfers.sql`: `CreateTransfer`, `GetTransfer` (scoped: from OR to
  in scope), `ListTransfers` (scoped + status filter + pagination), `CountTransfers`,
  `ListTransfersByAsset`, `SetTransferShipped`, `SetTransferReceived`,
  `GetOpenTransferForAsset` (guard). Add `SetAssetOffice` to `db/queries/assets.sql`. Then
  `sqlc generate`.
- **Permission catalog:** add the two keys to `authzadmin`'s canonical `permissionCatalog`.
- **OpenAPI** `backend/api/openapi.yaml`: add the `/transfers*` paths + schemas; Spectral green.
- **Wiring:** construct `transfer` handler + register routes in `NewRouter`; register the
  executor on the approval service.

## 12. Testing

Integration (`//go:build integration`, real Postgres/Redis/MinIO via `internal/testsupport`):
- **Happy path:** submit → approve (executor creates the `approved` transfer row) → ship
  (`in_transit`) → receive (`received`, asset `office_id`/`room_id` moved, BAST doc row +
  MinIO object present).
- **Reject:** submit → reject → **no transfer row**; asset unchanged.
- **Guards:** submit when asset already in an open transfer → `ErrAssetInTransit`; submit with
  `to_office == from_office` → rejected; cross-office/stale at executor → `ErrInvalidRef`.
- **Scope:** ship by a caller without `from_office` in scope → 403/404; receive without
  `to_office` in scope → 403/404; list filtered to from-or-to scope.
- **State machine:** ship a non-`approved` row → 409; receive a non-`in_transit` row → 409.
- **History:** `GET /assets/:id/transfers` returns the asset's transfers, scope-filtered.
- **BAST:** receive with a file → `asset_documents(bast_transfer)` row + object stored;
  receive without a file → metadata row only.

Unit (default build): `dto` validation (UUID parsing, required fields); service state-machine
transitions where Gin-free.

## 13. Interfaces / contracts summary

- `transfer.Service`: `Submit(ctx, caller, in) (request, error)`, `Ship(ctx, caller, id, in)`,
  `Receive(ctx, caller, id, in, file) (transfer, error)`, `Get`, `List`, `ListByAsset`,
  `Executor() approval.Executor`.
- Approval submit payload JSON: `{ "from_office_id", "to_office_id", "to_room_id"?, "reason"? }`
  with `target_entity="asset"`, `target_id=asset_id`, `type="asset_transfer"`.
- Transfer response JSON: `id, asset_id, from_office_id, to_office_id, to_room_id, status,
  reason, requested_by_id, approved_by_id, shipped_date, received_date, received_by_id,
  bast_no, request_id, notes, created_at, updated_at`.
