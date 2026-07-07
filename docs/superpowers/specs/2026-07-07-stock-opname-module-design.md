# Stock Opname (Inventarisasi Fisik) — Module Design

**Date:** 2026-07-07
**Status:** Approved (brainstorm)
**Scope:** Full-stack — backend module `internal/stockopname` + frontend screen `/stock-opname` + Berita Acara export.
**PRD:** §3.9 (FR-9.1–FR-9.6); role matrix "Kelola stock opname".
**Mockup:** `docs/design/Stock Opname.dc.html`.
**Pattern references:** `internal/transfer`, `internal/disposal` (ADR-0008 four-file split), `internal/depreciation` (engine + report export).

---

## 1. Product decisions (from brainstorm)

| # | Decision | Choice |
|---|----------|--------|
| 1 | Task scope | **Full-stack** (backend + frontend + e2e), same as Mutasi/Disposal/Depresiasi. |
| 2 | Session state machine | **Full 4-state DB enum**: `open → counting → reconciling → closed`. |
| 3 | Variance follow-up (FR-9.4) | **Auto-generate approval requests from backend** (linked back to the opname item). |
| 4 | Approval on the session itself | **No maker-checker** — direct, scoped, permission-gated, audit-logged, immutable after close. SoD lives on the value-affecting follow-ups (disposal/transfer), which go through the approval engine via their own executors. |
| 5 | Berita Acara (FR-9.5) | **Included** — PDF (gofpdf) + Excel (excelize), per session. |

**Rationale for #4 (no maker-checker on the session):** PRD FR-9.x does not require approval for the session; it is an operational Manager capability gated by permission + office data-scope. Maker-checker is value-tiered (`approval_thresholds` by amount) and the session has no monetary amount to tier. The value-affecting outcomes (write-off of lost assets, transfer of misplaced ones) DO go through approval as auto-generated requests, so segregation of duties is preserved exactly where value changes. Count integrity is controlled via `counted_by_id`/`counted_at` per item + an immutable Berita Acara + audit trail — the standard stock-take control.

---

## 2. Existing schema (already built, migration `000015_fam_tables`)

```
stockopname.stock_opname_sessions (
  id, office_id → masterdata.offices, name, period date,
  status shared.opname_session_status DEFAULT 'open',
  started_by_id → identity.users, started_at,
  closed_by_id?, closed_at?, created_at, updated_at, deleted_at )

stockopname.stock_opname_items (
  id, session_id → sessions ON DELETE CASCADE, asset_id → asset.assets,
  expected bool DEFAULT true, result shared.opname_item_result DEFAULT 'pending',
  counted_by_id?, counted_at?, note, created_at, updated_at, deleted_at )
  UNIQUE (session_id, asset_id) WHERE deleted_at IS NULL
```

Enums (migration `000002`):
- `shared.opname_session_status = ('open','counting','reconciling','closed')`
- `shared.opname_item_result = ('pending','found','not_found','damaged','misplaced')`

---

## 3. New migration — `000025_stockopname_followup`

1. **Column** `stockopname.stock_opname_items.followup_request_id uuid` (nullable, FK → `approval.requests(id)`) — traceability: which approval request was generated from a variance item; lets the variance panel show "sudah diajukan" and prevents duplicate follow-ups.
2. **Seed permissions** `stockopname.view` / `stockopname.manage` into `identity.role_permissions` (Superadmin full; Kepala/Manager scoped; Staf none) — matching the depreciation seed pattern in `000023` and the PRD "Kelola stock opname" role row.
3. Down migration drops the column + seeded rows.
4. Reuse existing `app_settings` `company_name`/`disclaimer` for the Berita Acara header — **no new settings**.

Data-scope module string: **`stock_opname`** (falls back to the role's `*` default policy; conservative fallback `own`). Follow the transfer/disposal wiring; no module-specific `data_scope_policies` seed row required.

---

## 4. Backend module `internal/stockopname/`

Four-file split (ADR-0008) + a focused `report.go`:

- **`service.go`** — holds `*sqlc.Queries` (+ deps: approval submit service, depreciation book-value calculator). The Berita Acara is exported on-the-fly (no MinIO storage — see §9). Business logic:
  - `CreateSession(input)` → insert session (`open`) + **snapshot**: select every non-deleted asset in the caller's office scope (respecting `expected`), bulk-insert `stock_opname_items` with `result='pending'`. Empty scope → session with zero items is allowed.
  - `StartCounting`, `Reconcile`, `Close` — state-machine transitions with explicit illegal-transition sentinel errors. `Close` stamps `closed_by/at` and makes the session immutable.
  - `SetItemResult(sessionID, itemID, result, note)` — only when `counting`; stamps `counted_by/at`.
  - `Scan(sessionID, assetTag)` — resolve tag → asset within scope; return the matching item; if the asset is in scope but not in the snapshot, insert an `expected=false` item (handles "misplaced-in"/FR-9.3).
  - `Kpis(sessionID)` / variance computation (`not_found`/`damaged`/`misplaced`, plus expected-but-pending).
  - `GenerateFollowup(sessionID, itemID, params)` — build + submit the appropriate approval request, set `followup_request_id`. Mapping in §5.
  - `ReportData(sessionID)` — assemble Berita Acara payload (summary + variance list + signatory from `started_by`/`closed_by`).
  - `mapDBError` (23505 → conflict, 23503 → invalid ref, no-rows → not found).
- **`dto.go`** — request structs with `binding` tags; `sessionToMap`/`itemToMap` response serialization.
- **`handler.go`** — `Handler` struct; each method binds/validates → resolves scope via `common.ScopedDeps.CallerOfficeScope(c, "stock_opname")` → calls service → serializes → responds; `svcError` maps sentinels to HTTP status.
- **`routes.go`** — `RegisterRoutes(rg, h, authMW, permSvc, ...)` mounts routes with `RequirePermission(permSvc, "stockopname.view"|"stockopname.manage")` per verb.
- **`report.go`** — `RenderPDF` (gofpdf) + `RenderXLSX` (excelize) of the Berita Acara.

Queries: `db/queries/stockopname.sql` → `sqlc generate` (scope-aware list/get with `AllScope`/`OfficeIds` params, snapshot insert, item CRUD, KPI aggregates).

Wire into `internal/server/router.go` (`NewRouter`).

---

## 5. Endpoints (`/api/v1`, scope + permission enforced on read AND write)

| Method + Path | Perm | Function |
|---|---|---|
| `GET /stock-opname/sessions` | view | List (filter `status`,`period`; `{data,total,limit,offset}`) |
| `POST /stock-opname/sessions` | manage | Create + snapshot → `open` |
| `GET /stock-opname/sessions/:id` | view | Detail + KPI (total/found/pending/variance) |
| `GET /stock-opname/sessions/:id/items` | view | Items (filter `result`,`room`, `search`) |
| `POST /stock-opname/sessions/:id/start` | manage | `open → counting` |
| `POST /stock-opname/sessions/:id/scan` | manage | Lookup by `asset_tag`; auto-add out-of-snapshot in-scope asset as `expected=false` |
| `PATCH /stock-opname/sessions/:id/items/:itemId` | manage | Set `result` + `note` (only in `counting`) |
| `POST /stock-opname/sessions/:id/reconcile` | manage | `counting → reconciling` (locks items) |
| `POST /stock-opname/sessions/:id/items/:itemId/follow-up` | manage | Generate approval request from variance item |
| `POST /stock-opname/sessions/:id/close` | manage | `reconciling → closed` (immutable) |
| `GET /stock-opname/sessions/:id/report?format=pdf\|xlsx` | view | Berita Acara export |

### Follow-up mapping
- **`not_found` → `asset_disposal`** (`method=write_off`); `amount` = **server-computed commercial book value** as of the current month (reuse `depreciation.ComputeBookValue`, same path disposal already uses; falls back to `purchase_cost` when no entries). Fully server-side, no client params.
- **`misplaced` → `asset_transfer`**; requires **destination office/room** supplied by the client (small modal in the variance panel). Server assembles + submits the transfer request and links it back.
- **`damaged` → maintenance**: Maintenance module does not exist yet → button **disabled / "coming soon"**, deferred and documented.
- Guard: an item that already has `followup_request_id` cannot generate a second request (409).

---

## 6. Frontend `/stock-opname`

- `app/pages/stock-opname.vue` — list view (session cards: status chip, scope, period, progress bar, empty state, create button) + detail view (header with status + Berita Acara + Selesaikan buttons; 4 KPI tiles; scan bar with scan-next + manual code entry when `counting`; item table with segmented result buttons when editable / read-only badge otherwise; variance panel with follow-up action buttons). 1:1 with the mockup.
- `composables/api/useStockOpname.ts` — real `$fetch` behind the standard `use*` interface.
- `constants/stockOpnameMeta.ts` — session-status meta (incl. new `reconciling` chip "Rekonsiliasi") + item-result meta.
- Components (auto-imported): session card, create-session modal, finish/Berita-Acara modal (preview + PDF/Excel), variance follow-up modal (reuse `AssetSearchPicker` where relevant).
- i18n: every string in `i18n/locales/{id,en}.json`.
- Nav: "Stock Opname" item in the Operasional group, gated on `stockopname.view`.
- Theme via tokens; light + dark parity; final 1:1 side-by-side against the mockup.

---

## 7. Approved mockup deviations (catat-deviasi convention)

- **(a)** Mockup shows 3 session statuses (draft/berjalan/selesai); backend uses **4** → `reconciling` surfaces as a new "Rekonsiliasi" chip (direct consequence of decision #2).
- **(b)** The "damaged → maintenance" follow-up button is **disabled** (Maintenance module not built yet).
- **(c)** Mockup labels `not_found`="Belum dicek" and `hilang`="Tidak ditemukan"; the DB enum uses `pending`=unchecked and `not_found`=not-found, so the UI labels follow the correct enum semantics (`pending`→"Belum dicek", `not_found`→"Tidak ditemukan").

Any further deviation found during the final side-by-side gets user approval first, then is recorded here.

---

## 8. Testing

**Backend integration (`//go:build integration`, testcontainers):**
- Snapshot creates one item per in-scope asset; out-of-scope assets excluded.
- Scope enforced on every verb (read + write) — cross-office access denied.
- State machine: legal transitions succeed; illegal ones (e.g. `SetItemResult` outside `counting`, `close` from `open`) rejected.
- `SetItemResult` stamps `counted_by/at`.
- `scan` adds an out-of-snapshot in-scope asset as `expected=false`.
- Follow-up: `not_found` creates an `asset_disposal` write-off request with server-computed amount; `misplaced` creates an `asset_transfer` with client destination; duplicate follow-up rejected (409).
- Immutability: no item mutation after `closed`.
- Report renders PDF + XLSX without error for a closed session.

**Frontend:**
- Unit: meta maps, KPI/variance helpers (pure).
- `mountSuspended` component specs: list empty/populated, detail in all 4 statuses, scan interaction, variance panel (each result kind + disabled maintenance), create/finish/follow-up modals, loading/error/empty states.
- e2e real-backend (`frontend/e2e/stock-opname.spec.ts`): create → snapshot → start → count → reconcile → follow-up (disposal) → close → report. Unique per run; assert-after-search.

**Gates (task-13):** backend build/vet/test + `-tags=integration`; Spectral; frontend lint/typecheck/test/build; full e2e. OpenAPI synced. PROGRESS.md updated.

---

## 9. Out of scope / deferred

- `damaged → maintenance` follow-up (waits on the Maintenance module).
- Storing the Berita Acara file in MinIO (export is on-the-fly; add storage later if an archived-document requirement appears).
- Real-time barcode camera scanning (the scan bar uses tag lookup / manual entry; hardware camera integration is a frontend follow-up).
- Global inbox badge counts (shared deferral with Approval/Mutasi screens).
