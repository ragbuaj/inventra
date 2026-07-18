# Approval Engine + Asset Core — Backend Design

Date: 2026-06-28
Status: Approved (decisions confirmed with user)

## Goal

Build the **value-tiered maker-checker approval engine** (PRD bagian 2.4, bagian 3.6 / FR-6.x) **together with
the Asset core module** (PRD bagian 3.2 / FR-2.x), integrated end-to-end so the full maker-checker flow
actually executes: a maker submits a request, a configurable multi-step chain routes it by value and
office tier, and final approval **executes the real side effect** (an asset row is created, disposed,
or flagged). The approval schema (`000010_approval`) and asset schema (`000008_asset`, `000015_fam_tables`)
already exist; this is a pure Go-module + queries + seed build, plus one small migration to add an
explicit office tier.

Two new modules: `internal/approval/` and `internal/asset/`, each following the four-file split
(ADR-0008): `service.go` / `dto.go` / `handler.go` / `routes.go`.

## Scope

**In scope (this slice):**
- Full generic approval engine: request CRUD, threshold-driven chain construction, per-step decision
  flow, segregation-of-duty (SoD), eligibility (pull model), approver inbox/queue, cancel, audit.
- Pluggable **executor registry** with three real executors: `asset_create`, `asset_disposal`
  (status transition only), `valuation_exclusion`.
- Asset core: read (list/get with scope + field-permission masking), direct update, the `asset_tag`
  generator, and the status state machine.
- Migration `000016`: add `office_types.tier` + seed; thresholds seed; permission/scope seeds.
- OpenAPI sync, unit + integration tests, router wiring.

**Out of scope (separate PROGRESS.md modules — do NOT build here):**
- Attachments / MinIO, barcode / QR.
- Asset transfer (mutasi) full flow, stock opname.
- **Disposal accounting** (gain/loss, `disposal.disposals` table) — `asset_disposal` executor here
  only transitions status; the accounting lands in the dedicated Disposal module.
- Dual-basis depreciation, journal export, bulk import, notifications/scheduler.

## Decisions (confirmed)

1. **Build approval engine + asset core together**, integrated, one spec. The implementation plan
   will phase it (migration+seed → approval engine → asset read/update → executors+integration →
   endpoints+OpenAPI+tests), but it is a single design.
2. **Asset write paths** (PRD FR-6.1): `asset_create`, `asset_disposal`, `valuation_exclusion` go
   **through approval**; they have **no direct write endpoint**. Asset **update** (non-sensitive
   attributes) is **direct** (gated `asset.manage` + scope). Asset **read** is direct (scope +
   field masking).
3. **Approver model = pull.** `request_approvals` rows are created with `approver_id = NULL`. Any
   *eligible* user (has `request.decide`, scope/tier covers the request office, not the maker, not a
   prior approver) sees the request in their inbox and acts. No specific approver is pre-assigned.
4. **Tier resolution = explicit office tier (reuse existing enum).** Add `masterdata.office_types.tier`
   typed `shared.approver_level` (the existing enum). Office hierarchy is **pusat → wilayah → cabang →
   outlet** (4 levels), but approval only needs 3 tiers, so **cabang & outlet both seed `tier = office`**.
   `office_subtree` is unused on office types (reserved for future mutasi routing). Because `tier` and
   `approval_thresholds.required_level` share the same vocabulary, matching needs no mapping table.
5. **Chain model = cumulative, one `approval_thresholds` row per step.** A value band is represented
   by multiple rows sharing `amount_from`/`amount_to`, distinguished by `step_order`, each carrying
   the `required_level` for that step. At request time the engine selects all active rows whose band
   contains `amount`, ordered by `step_order` — that ordered list *is* the chain.

---

## Architecture

### 1. Migration `000016_office_tier`

```sql
-- up
ALTER TABLE masterdata.office_types ADD COLUMN tier shared.approver_level;  -- nullable
-- (no FK / no enum creation — reuses shared.approver_level from 000002)

-- down
ALTER TABLE masterdata.office_types DROP COLUMN tier;
```

- `tier` is nullable: office types not relevant to approval routing may leave it unset.
- Seeds live **in the migration** after the `ALTER` (repo convention bakes seeds into migrations; see
  the "5 roles, 45 RBAC perms" seed in earlier migrations). The column add plus all data seeds for this
  feature (office-type tiers, thresholds, permissions, scope policies, field permissions — bagian Seeds) go
  in `000016`, with the `.down.sql` reversing both DDL and seed rows. Office-type tier backfill is an
  idempotent `UPDATE masterdata.office_types SET tier=... WHERE name=...`.
- `office_type` service/dto + OpenAPI extended so admins can set `tier` going forward.

### 2. New queries → `sqlc generate`

`db/queries/assets.sql`:
- `ListAssets` / `CountAssets` — search (`name`/`asset_tag`/`serial_number`), filters
  (`category_id`, `office_id`, `status`, `asset_class`), scope (`all_scope` bool + `office_ids`),
  sort, `LIMIT/OFFSET`. Returns `{data,total,limit,offset}` shape via handler.
- `GetAsset` — by id, `deleted_at IS NULL`.
- `CreateAsset` — full insert (called inside a tx by the `asset_create` executor).
- `UpdateAsset` — non-sensitive attributes (direct update path).
- `SetAssetStatus` — status transition (used by executors / state machine).
- `SetAssetValuationExclusion` — set `excluded_from_valuation` + `valuation_exclusion_reason`.
- `BumpAssetTagCounter` — `INSERT INTO asset.asset_tag_counters (office_id, category_id, year, last_seq)
  VALUES (...,1) ON CONFLICT (office_id, category_id, year) DO UPDATE SET last_seq =
  asset_tag_counters.last_seq + 1 RETURNING last_seq;` (atomic).

`db/queries/approval.sql`:
- `MatchThresholdSteps` — `WHERE request_type=$1 AND amount_from <= $2 AND (amount_to IS NULL OR
  $2 < amount_to) AND is_active AND deleted_at IS NULL ORDER BY step_order`.
- `ListThresholds` / threshold CRUD (`CreateThreshold`/`UpdateThreshold`/`SoftDeleteThreshold`).
- `CreateRequest`, `GetRequest`, `ListRequests` (status/type filter + scope), `SetRequestDecision`,
  `AdvanceRequestStep`, `CancelRequest`.
- `CreateRequestApproval` (per step), `ListRequestApprovals` (by request), `GetCurrentStepApproval`,
  `DecideRequestApproval` (set `approver_id`/`decision`/`note`/`decided_at`).
- `ListInboxCandidates` — pending requests whose **current step** is undecided (eligibility is then
  filtered in Go against the caller's scope/tier + SoD; see bagian 6).

`db/queries/offices.sql` (extend):
- `GetOfficeAncestors` — recursive CTE walking **up** via `parent_id` from a given office, returning
  each ancestor with its `office_type.tier` (joined). Used by `resolveTierOffice`.

### 3. `internal/approval/` module

**`service.go`** — engine + sentinel errors (`ErrNotFound`, `ErrForbidden`, `ErrInvalidState`,
`ErrSelfApproval`, `ErrNotEligible`, `ErrNoThreshold`) + `mapDBError`. Holds `*sqlc.Queries`, the
`*pgxpool.Pool` (for transactions), `*authz.ScopeService`, the threshold cache, and the executor
registry. Gin-free.

Key methods:
- `Submit(ctx, in SubmitInput) (Request, error)` — validate type/amount/office; `MatchThresholdSteps`;
  if no steps → `ErrNoThreshold`; in a tx: insert `requests` (`status=pending`, `current_step=1`,
  `payload` JSON), insert one `request_approvals` row per step. **SoD pre-check**: maker recorded as
  `requested_by_id`.
- `Decide(ctx, requestID, approver Caller, decision, note) (Request, error)` — load request + current
  step; enforce `eligibleToDecide` (bagian 6); on **approve**: mark step approved; if more steps remain →
  `AdvanceRequestStep` (`current_step++`); if last step → set request `approved` + run executor in the
  **same tx** (atomic). On **reject**: mark step rejected, request `rejected`, stop. Records audit on
  every decision.
- `Cancel(ctx, requestID, maker Caller)` — only `requested_by_id == maker` and `status==pending`.
- `Inbox(ctx, caller Caller) ([]Request, error)` — `ListInboxCandidates` then filter by
  `eligibleToDecide`.
- Threshold CRUD with Redis cache invalidation (mirrors authz cache pattern).

**Executor registry:**
```go
type Executor interface {
    Execute(ctx context.Context, tx pgx.Tx, req sqlc.ApprovalRequest) error
}
// registry: map[sqlc.SharedRequestType]Executor
```
Registered at construction. The engine looks up the executor by `req.Type` and calls it **inside the
approval-commit transaction**; an executor error rolls the whole decision back (no partial state).
Asset executors live in `internal/asset` and are injected into the approval service at wiring time to
avoid an import cycle (approval depends on the `Executor` interface only, not on the asset package).

**`dto.go`** — `SubmitRequest` (binding tags: `type` oneof, `amount` numeric string, `office_id` uuid,
`payload`, `reason`), `DecideRequest` (`decision` oneof approve/reject, `note`), threshold DTOs.
Request/threshold response serializers. Request responses are **field-permission-filtered** via map
form where they expose asset payloads.

**`handler.go`** — binds, resolves `Caller` from context (`CtxUserID`/`CtxRoleID` + office), calls
service, maps sentinel → HTTP via `svcError`, serializes, records audit. SoD/eligibility violations →
`403`; invalid state (e.g. deciding a non-current step) → `409`.

**`routes.go`** — see bagian 6 endpoint table.

### 4. `internal/asset/` module

**`service.go`** — asset business logic + sentinel errors + `mapDBError`. Holds `*sqlc.Queries` +
`*pgxpool.Pool`.
- `List` / `Get` — scope-aware (take `allScope bool, officeIDs []uuid`), return sqlc rows.
- `Update` — non-sensitive attribute update; validates referenced FKs; enforces the
  `room_id` CHECK (tangible requires a room) at the app layer with a friendly error.
- `GenerateAssetTag(ctx, tx, officeID, categoryID, year) (string, error)` — fetches office `code` +
  category `code`, `BumpAssetTagCounter`, formats `<officeCode>-<categoryCode>-<year>-<seq:%05d>`.
- `StateMachine` — `validTransition(from, to) bool`. Allowed here:
  `available→assigned`, `assigned→available`, `available→under_maintenance`,
  `under_maintenance→available`, `available→lost`, `assigned→lost`, `*(non-terminal)→disposed`.
  Transitions owned by other modules (`in_transfer`, `retired`) are **rejected** from this module.
- **Executors** (implement `approval.Executor`):
  - `assetCreateExecutor` — parse `payload` → generate tag (in tx) → `CreateAsset` (`created_by_id`
    = maker, `status='available'`).
  - `assetDisposalExecutor` — `target_id` → `SetAssetStatus(disposed)` (validates current status is
    disposable). Accounting deferred.
  - `valuationExclusionExecutor` — `target_id` → `SetAssetValuationExclusion(true, reason)`.

**`dto.go`** — `assetToMap(...)` **map form** (so `authz.FilterView` can drop fields); never serialize
nothing sensitive beyond policy. The create/disposal/exclusion payload DTOs are validated **at submit
time** in the approval handler path (so bad payloads are rejected before a request is created),
re-validated defensively in the executor.

**`handler.go`** — `list`, `get`, `update`. Read handlers apply field masking:
`fieldSvc.FilterView(ctx, roleID, "assets", assetMap)` masking `purchase_cost`, `book_value`,
`accumulated_depreciation` (default-allow for unlisted fields, per existing FieldService). Update
handler rejects edits to fields the role can't edit (`field_permissions` edit flag).

**`routes.go`** — see bagian 6.

### 5. Field-permission masking (FR-2.3)

Seed `field_permissions` for entity `assets`:
- `purchase_cost`, `book_value` → `can_view` only Superadmin & Manager.
- `accumulated_depreciation` → `can_view` only Superadmin.
- Others default-allow. Masking is applied in the asset read handlers via the map form; edit flags are
  enforced in the update handler.

### 6. Authorization, eligibility, endpoints

**New permission keys** (seed `role_permissions`): `request.create`, `request.decide`,
`approval.config.manage`, `asset.view`, `asset.manage`.

**New data-scope modules** (seed `data_scope_policies` per-role defaults + `module='*'` fallback):
`"assets"`, `"requests"`. Read/list and the direct update path resolve caller scope via
`common.ScopedDeps.CallerOfficeScope(c, module)` and pass `(allScope, officeIDs)` into queries.

**Eligibility — `eligibleToDecide(caller, request, step)`** (pull model, bagian Decision 3/4):
1. caller has `request.decide` permission, AND
2. caller is **not** the maker (`requested_by_id`) and **not** an `approver_id` on any prior step
   (SoD; DB also enforces `decided_by_id <> requested_by_id`), AND
3. `resolveTierOffice(request.office_id, step.required_level)` returns an office `T` such that the
   caller can act for `T`:
   - `required_level=office` → `T = request.office_id`; caller eligible if their office scope over
     `"requests"` covers `T` (typically caller's own office == T).
   - `required_level=wilayah` → `T` = nearest ancestor with `tier=wilayah`; caller eligible if their
     office == `T` (or scope covers `T`).
   - `required_level=pusat` → `T` = ancestor with `tier=pusat`; caller eligible likewise.
   - `required_level=office_subtree` → reserved; treated as `office` for now.
   `resolveTierOffice` uses `GetOfficeAncestors` (cached subtree/ancestors per office in Redis, mirroring
   `ScopeService`). If no ancestor of the required tier exists, the step is unsatisfiable → surfaced as
   a config error (logged), not a silent pass.

**Endpoints** (all under `/api/v1`, after `RequireAuth`):

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/requests` | `request.create` | maker submits; builds chain |
| GET | `/requests` | `request.create` or `request.decide` | scope-filtered list; `?status=&type=` |
| GET | `/requests/:id` | (scope) | detail incl. step chain |
| GET | `/requests/inbox` | `request.decide` | eligible-pending queue for caller |
| POST | `/requests/:id/approve` | `request.decide` | eligibility + SoD enforced |
| POST | `/requests/:id/reject` | `request.decide` | |
| POST | `/requests/:id/cancel` | (maker only) | only while `pending` |
| GET | `/approval-thresholds` | `approval.config.manage` | |
| POST/PUT/DELETE | `/approval-thresholds[/:id]` | `approval.config.manage` | invalidates cache |
| GET | `/assets` | `asset.view` | scope + field masking |
| GET | `/assets/:id` | `asset.view` | scope + field masking |
| PUT | `/assets/:id` | `asset.manage` | direct update; scope + edit-flag |

Asset create/disposal/exclusion are performed **only** via `POST /requests` with the matching `type`.

### 7. Router wiring (`internal/server/router.go`)

```go
assetSvc := asset.NewService(queries, d.Pool)
approvalSvc := approval.NewService(queries, d.Pool, scopeSvc, d.Redis)
approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())
approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, assetSvc.DisposalExecutor())
approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeValuationExclusion, assetSvc.ExclusionExecutor())

asset.RegisterRoutes(api, asset.NewHandler(assetSvc, fieldSvc, scopeSvc, auditSvc),
    requireAuth, middleware.RequirePermission(permSvc, "asset.view"),
    middleware.RequirePermission(permSvc, "asset.manage"))
approval.RegisterRoutes(api, approval.NewHandler(approvalSvc, fieldSvc, scopeSvc, auditSvc),
    requireAuth, permSvc)
```

## Seeds

Seeded in migration `000016` (idempotent; reversed in `.down.sql`):
- `office_types.tier`: Pusat→`pusat`, Wilayah→`wilayah`, Cabang→`office`, Outlet→`office`.
- `approval_thresholds` (cumulative, per PRD bagian 2.4 placeholders — flagged "confirm with bank policy"):
  - `asset_create`: ≤10jt → [office]; 10–100jt → [office, wilayah]; >100jt → [office, wilayah, pusat].
  - `asset_disposal`: ≤5jt → [office]; 5–50jt → [office, wilayah]; >50jt → [office, wilayah, pusat].
  - `valuation_exclusion`: [wilayah].
- `role_permissions`: grant the 5 new keys to roles per PRD role matrix (Superadmin all; Manager/Kepala
  Unit/Kanwil get `request.decide` + `asset.view`/`asset.manage`; Staf gets `request.create` +
  `asset.view`).
- `data_scope_policies`: per-role defaults for `"assets"` and `"requests"` (Kanwil→office_subtree,
  Unit→office_subtree, Manager→office_subtree, Staf→own/office), conservative `own` fallback.
- `field_permissions`: the asset masking rows above.

## Error handling

- Reuse `common.MapDBError` / `common.WriteError` patterns (404/409/400/403/500). Approval adds:
  `ErrSelfApproval`/`ErrNotEligible`→403, `ErrInvalidState`→409, `ErrNoThreshold`→422
  (`{error: "no approval threshold configured for this type/amount"}`).
- Executor failure inside the decide tx → whole decision rolls back; request stays at its prior step;
  surface `500` with a correlation id (the engine logs the executor error via `slog`).
- Audit writes are best-effort (never fail the request), matching the audit module.

## Testing

Proactive and expansive (repo convention). Default `go test ./...` stays unit-only; integration suites
behind `//go:build integration` using `internal/testsupport` (real Postgres/Redis).

**Unit:**
- Chain construction from thresholds for every band of each type (1/2/3 steps), boundary amounts
  (exactly `amount_from`, just below `amount_to`), and the no-threshold case.
- `resolveTierOffice` for office/wilayah/pusat across the 4-level hierarchy, including outlet→office,
  outlet→wilayah, missing-tier (unsatisfiable) cases.
- `eligibleToDecide`: maker self-approval blocked, prior-approver-as-next-approver blocked (SoD),
  wrong-tier blocked, out-of-scope blocked, happy eligible.
- State machine `validTransition` (all allowed + representative rejected transitions).
- `asset_tag` format + counter atomicity (concurrent bumps → no duplicate seq) + per-year reset.
- Field masking: Staf sees no `purchase_cost`/`book_value`; Manager sees cost not
  `accumulated_depreciation`; Superadmin sees all.

**Integration (real DB):**
- End-to-end `asset_create` >100jt: maker submit → 3 sequential approvals by distinct eligible
  approvers → asset row exists with generated tag; SoD violation at each step rejected (403).
- `asset_disposal`: approve chain → asset `status=disposed`; reject mid-chain → no status change.
- `valuation_exclusion`: approve → flag + reason set.
- Cancel by maker while pending; cancel by non-maker rejected.
- Scope enforcement on read **and** the direct update path (out-of-scope office → 403/empty).
- Threshold CRUD invalidates cache (decision after edit uses new bands).
- **Executor atomicity**: force an executor error → request not advanced, no asset row (rollback).

## Verification gates (before claiming done)

`sqlc generate` clean · `go build ./...` · `go vet ./...` · `go test ./...` (+ integration job) ·
Spectral lint of `backend/api/openapi.yaml` · update `docs/PROGRESS.md` (tick Approval + Asset-core
items, refresh "Next session").

## Open items (flagged, not blocking)

- Threshold band numbers, capitalization threshold, and office-tier naming are **placeholders pending
  bank policy** (PRD ⚠️ / DATABASE DB-Q6–Q8).
- `office_subtree` as an `approver_level` value stays unused until the mutasi (transfer) module needs
  subtree-relative routing.
