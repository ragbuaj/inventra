# Asset Disposal (Penghapusan/Pelepasan) — Backend Module Design

| | |
|---|---|
| **Feature** | Asset disposal — PRD v1.1 §3.6/§5 |
| **Date** | 2026-07-02 |
| **Scope** | **Backend-only** (no frontend mockup exists yet) |
| **Status** | Approved — ready for implementation plan |
| **Module** | `backend/internal/disposal/` (new, four-file split per ADR-0008) |

> Second of the three Bank-FAM operational subsystems (transfer done; disposal here; stock
> opname next). Each is an independent spec → plan → implementation cycle. This spec covers
> **disposal only** and follows the just-shipped transfer module (`internal/transfer/`) as its
> structural template.

## 1. Goal

Complete asset disposal. Today disposal is minimal: submitted via the generic `POST /requests`
(type `asset_disposal`, maker-supplied `amount`), and the executor (`disposalExec` in
`internal/asset/`) only flips `asset.status → disposed`. This cycle builds a dedicated
`internal/disposal/` module that **records the `disposal.disposals` row** (method, proceeds,
book value, computed gain/loss, BAST) on approval, adds **read endpoints** and a **BAST
document** endpoint, and **migrates the executor out of the asset package**.

## 2. Non-goals (this cycle)

- **Frontend / disposal screen** — no `docs/design/*.dc.html` mockup; built later. Pending
  disposals already surface in the existing Pengajuan inbox as `asset_disposal` requests.
- **GL journal export** for gain/loss — deferred (PRD "journal-ready export" backlog item).
- **Server-derived book value** — needs the depreciation module (not built). `book_value_at_disposal`
  is maker-supplied (optional) this cycle; `gain_loss` is computed from it when present.

## 3. Current state being replaced

- `internal/asset/executor.go`: `disposalExec` + `(*Service).DisposalExecutor()` — flips status
  only, no `disposals` row; uses the asset-package-private `validTransition(from,to)`.
- `internal/server/router.go`: registers `approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, assetSvc.DisposalExecutor())`.
- `internal/approval/integration_test.go`: has `TestApproval_AssetDisposal_*` cases that register
  `assetSvc.DisposalExecutor()` and submit with no disposal payload.
- No `db/queries/disposals.sql`. The `disposal.disposals` table + `sqlc.DisposalDisposal` row type
  + enums (`SharedDisposalMethod{Sale,Auction,Donation,WriteOff}`, `SharedAssetDocumentTypeBastDisposal`)
  already exist (migrations `000015`/`000002`). Unique index `uq_disposals_asset (asset_id) WHERE deleted_at IS NULL`.
- `asset_disposal` `approval_thresholds` bands are already seeded (migration `000016`) — **no new
  threshold seed needed**.

## 4. Lifecycle & state model

The approval request owns pre-approval state; the `disposal.disposals` row is created only by the
executor on final approval, so rejected/cancelled disposals never create a row (no drift).

| Step | Endpoint | Actor | Effect |
|---|---|---|---|
| **Submit** | `POST /disposals` | maker (`disposal.manage`) | Validates asset; captures disposal detail; opens an `approval.requests` row (type `asset_disposal`, `pending`). **No disposal row yet.** |
| **Approve** (final) | `POST /requests/:id/approve` (existing) | checker | `asset_disposal` **executor** (now in `internal/disposal/`) writes the `disposal.disposals` row (`gain_loss` computed in SQL) and flips `asset.status → disposed`, inside the approval-commit tx. |
| **Reject / Cancel** | `POST /requests/:id/reject` / `/cancel` (existing) | checker / maker | Request → `rejected`/`cancelled`. **No disposal row.** |
| **BAST** (optional) | `POST /disposals/:id/document` | `disposal.manage` | Creates `asset.asset_documents` (`doc_type=bast_disposal`, `related_disposal_id`, `related_request_id`) + optional MinIO file; updates `disposals.bast_no` if provided. Best-effort file. |
| **Read** | `GET /disposals`, `/disposals/:id`, `/assets/:id/disposal` | `disposal.view` | Scope-filtered. |

`asset.status → disposed` is enforced via the asset package's transition matrix (see §7).

## 5. Module `internal/disposal/` (four-file split)

- **`service.go`** — business rules + sentinels (`ErrNotFound`, `ErrInvalidState`,
  `ErrAlreadyDisposed`, `ErrDisposalExists`, `ErrOutOfScope`, `ErrInvalidRef`) + `mapDBError`.
  Holds `*sqlc.Queries`, `*pgxpool.Pool`, `*approval.Service`. Methods: `Submit`, `Get`, `List`,
  `ListByAsset`, and `Executor() approval.Executor`. Gin-free.
- **`dto.go`** — `SubmitRequest` (`asset_id*`, `method*`, `disposal_date*`, `proceeds?`,
  `book_value_at_disposal?`, `bast_no?`, `reason?`), `DocumentRequest` (BAST metadata; file is a
  multipart part), `DisposalPayload` (JSON stored in the approval request), `toResponse`
  serializer, `marshalPayload`. No sensitive columns.
- **`handler.go`** — `Handler` (service + `*asset.Service` for BAST + `common.ScopedDeps` +
  `*audit.Service`). Methods: submit / get / list / listByAsset / attachDocument. `svcError` maps
  sentinels to HTTP status; `caller()` mirrors `internal/transfer/handler.go` (which mirrors
  `approval/handler.go` — `c.GetString(...)` + `uuid.Parse`, never `MustParse` on unbound input).
- **`routes.go`** — `RegisterRoutes(rg, h, authMW, requireManage, requireView)`.

The `asset_disposal` **executor** lives here; registered in `NewRouter` via
`approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, disposalSvc.Executor())`,
**replacing** `assetSvc.DisposalExecutor()`.

## 6. Endpoints (under `/api/v1`)

| Method | Path | Permission | Scope check |
|---|---|---|---|
| `POST` | `/disposals` | `disposal.manage` | asset's `office_id` in caller scope |
| `POST` | `/disposals/:id/document` | `disposal.manage` | disposal's asset office in caller scope |
| `GET` | `/disposals` | `disposal.view` | list filtered by asset office in scope |
| `GET` | `/disposals/:id` | `disposal.view` | asset office in scope |
| `GET` | `/assets/:id/disposal` | `disposal.view` | asset office in scope (0 or 1) |

List envelope `{data, total, limit, offset}` (limit clamped 1–100); single-row flat.
Reject/cancel pre-approval use the existing `/requests/:id/cancel|reject`.

Scoping: `disposal.disposals` has no `office_id` column, so scoped queries **join
`asset.assets`** on `asset_id` and filter `assets.office_id` against `all_scope`/`office_ids`.

## 7. Submit flow + executor

`POST /disposals` (`disposal.manage`):
1. Bind `SubmitRequest`; parse UUIDs (`uuid.Parse` → 400 on bad).
2. Load asset (`GetAsset`); scope-check `common.InScope(all, ids, asset.OfficeID)` → 404/403 if out.
3. Validate: `asset.ValidTransition(asset.Status, disposed)` is true (else `ErrAlreadyDisposed`/`ErrInvalidState`);
   no existing non-deleted disposal for the asset (`GetDisposalByAsset` → `ErrDisposalExists`); no
   pending `asset_disposal` request for the asset (`CountPendingDisposalRequestsForAsset`).
4. `amount = asset.purchase_cost` (nil → `"0"`) — **server-derived**, improving on the current
   maker-supplied amount (closes the `disposalExec` SECURITY/TODO for the tiering basis).
5. `approval.Submit(Type=asset_disposal, OfficeID=asset.OfficeID, TargetEntity="asset",
   TargetID=asset_id, Amount, Payload={method, disposal_date, proceeds, book_value_at_disposal,
   bast_no, reason}, Maker=caller)`.
6. Return the created request `{request_id, status}`.

**Executor** (`asset_disposal`, in `internal/disposal/`, runs in the approval-commit tx):
1. nil `TargetID` guard → `approval.ErrInvalidRef`.
2. Unmarshal `req.Payload`.
3. Load asset (`qtx.GetAsset`); defense-in-depth: `req.OfficeID != nil && asset.OfficeID == *req.OfficeID`
   (else `approval.ErrInvalidRef`); `asset.ValidTransition(asset.Status, disposed)` true.
4. Guard: no existing disposal (`qtx.GetDisposalByAsset` → `approval.ErrConflict` if present).
5. `qtx.CreateDisposal(...)` — `gain_loss` computed **in SQL** as `proceeds - book_value_at_disposal`
   (null-propagating: null when either is null); `approved_by_id=req.DecidedByID`,
   `request_id=req.ID`, `created_by_id=req.RequestedByID`, plus method/date/proceeds/book_value/bast_no
   from the payload.
6. `qtx.SetAssetStatus(asset_id, disposed)`.

The asset package **exports** `ValidTransition(from, to sqlc.SharedAssetStatus) bool` (was the
private `validTransition`) so the disposal executor reuses the canonical transition matrix.

## 8. BAST document

`POST /disposals/:id/document` (`disposal.manage`, multipart):
1. Load disposal (scoped by asset office); 404 if out of scope/missing.
2. Create `asset.asset_documents` via `asset.Service.CreateDocument` with `doc_type=bast_disposal`,
   `related_disposal_id=disposal.id`, `related_request_id=disposal.request_id`, plus
   `doc_no`/`doc_date`/`counterparty` from the body.
3. If a `file` part is present, `asset.Service.AttachFile` stores it in MinIO (best-effort — a file
   failure does not fail the metadata creation).
4. If `bast_no` is provided, update `disposals.bast_no` (`SetDisposalBastNo`).

**Asset-package change:** `asset.DocumentInput` gains an optional `RelatedDisposalID *uuid.UUID`,
and the `CreateAssetDocument` query + params gain `related_disposal_id` (currently only
`related_request_id` is wired, though the column exists). Backward-compatible — existing callers
(transfer BAST, asset document handler) pass nil.

## 9. Authorization

- **Permissions (new):** `disposal.manage` (submit/BAST) + `disposal.view` (read) — added to the
  `authzadmin` catalog + seeded into `role_permissions` for operational roles (Superadmin via `*`;
  Manager/Kepala Kanwil/Kepala Unit both). Seeded in the new migration.
- **Data scope:** module string `"disposals"`, resolved via `common.ScopedDeps.CallerOfficeScope(c,
  "disposals")`. Enforced on **read and write**; scoped reads join `asset.assets` for the office.
- **SoD:** the approval engine enforces maker ≠ checker.

## 10. Value / threshold basis

`amount = asset.purchase_cost` (interim proxy for book value; documented, revisit with the
depreciation module). `asset_disposal` threshold bands already seeded (`000016`).

## 11. Database / infra work

- **No new tables.** New migration `000021_disposal_seed` (highest existing is `000020_transfer_seed`;
  re-verify at implementation time):
  - insert `disposal.manage` / `disposal.view` into `identity.role_permissions` (operational roles);
  - seed `data_scope_policies` `disposals` defaults (global/office_subtree/office, mirror
    `transfers` from `000020`).
- **Queries** `db/queries/disposals.sql`: `CreateDisposal` (with `gain_loss = proceeds -
  book_value_at_disposal` computed in SQL), `GetDisposal` (scoped via asset-office join),
  `ListDisposals` (scoped, pagination only — no method filter this cycle), `CountDisposals`,
  `ListDisposalsByAsset` (0/1, scoped), `GetDisposalByAsset` (guard, unscoped by office — used in
  submit/executor), `CountPendingDisposalRequestsForAsset` (guard, joins `approval.requests`),
  `SetDisposalBastNo`. Extend `db/queries/assets.sql` `CreateAssetDocument` with `related_disposal_id`.
  Then `sqlc generate`.
- **Asset package:** export `ValidTransition`; remove `disposalExec` + `DisposalExecutor()`; add
  `RelatedDisposalID` to `DocumentInput` + thread through `CreateDocument`.
- **Permission catalog:** add the two keys to `authzadmin`'s `permissionCatalog`.
- **OpenAPI:** add `/disposals*` paths + `Disposal` schema.
- **Wiring:** construct disposal handler + register routes in `NewRouter`; register the executor on
  the approval service (replacing the asset disposal executor).
- **Migrate tests:** update `approval/integration_test.go` disposal cases to register the new
  disposal executor and submit with a disposal payload (or relocate those scenarios into the
  disposal integration suite; keep approval-engine coverage intact).

## 12. Testing

Integration (`//go:build integration`, real Postgres/Redis/MinIO):
- **Happy path:** submit → approve (executor writes the `disposals` row with correct method/proceeds/
  book_value; `gain_loss` correct; asset `status=disposed`).
- **Gain/loss:** proceeds=`120M`, book_value=`100M` → `gain_loss=20M`; book_value nil → `gain_loss` nil.
- **Reject:** submit → reject → no `disposals` row; asset status unchanged.
- **Guards:** submit for an already-`disposed` asset → `ErrInvalidState`/`ErrAlreadyDisposed`; submit
  when a disposal already exists → `ErrDisposalExists`; out-of-scope asset → `ErrOutOfScope`;
  executor cross-office/stale → `approval.ErrInvalidRef`.
- **Scope:** list/get filtered by asset office; caller scoped to another office sees empty/404.
- **BAST:** `POST /disposals/:id/document` → `asset_documents(bast_disposal)` row with
  `related_disposal_id`; with a file → MinIO object present; `disposals.bast_no` updated.
- **Executor migration:** the disposal-through-approval path still works end-to-end after moving the
  executor.

Unit (default build): `dto` validation + `marshalPayload` round-trip.

## 13. Interfaces / contracts summary

- `disposal.Service`: `Submit(ctx, caller, in) (request, error)`, `Get`, `List`, `ListByAsset`,
  `Executor() approval.Executor`.
- Approval payload JSON: `{ "method", "disposal_date", "proceeds"?, "book_value_at_disposal"?,
  "bast_no"?, "reason"? }`; `type="asset_disposal"`, `target_entity="asset"`, `target_id=asset_id`.
- Disposal response JSON: `id, asset_id, method, disposal_date, proceeds, book_value_at_disposal,
  gain_loss, bast_no, approved_by_id, request_id, created_by_id, created_at, updated_at`.
