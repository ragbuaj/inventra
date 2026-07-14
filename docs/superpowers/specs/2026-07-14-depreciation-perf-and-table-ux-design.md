# Depreciation performance + table-UX fixes — Design

**Date:** 2026-07-14
**Branch:** `feat/depreciation-perf-and-table-ux`
**Status:** Approved (design), pending implementation plan

Batch of five fixes reported against the live app
(`https://inventra.ragilbuaj.web.id`). Items #2/#4 share a root cause (the
`/depreciation/schedule` endpoint) and are the bulk of the work; #1/#3/#5 are
smaller, mostly-frontend changes.

---

## 1. Standardize row actions across all action-bearing tables (#1)

**Requirement:** every table that has per-row actions must expose them via the
shared pattern — a kebab (`⋮`) dropdown **and** a right-click context menu.

**Current state.** `app/components/ResourceTable.vue` already renders both a
per-row `UDropdownMenu` (kebab) and a tbody-wide `UContextMenu`, driven by a
`RowActions` function prop (`buildItems()` groups actions; a `separator: true`
action starts a new divider group). Four screens already use it (users,
categories, reference, employees). The rest are hand-rolled `<table>`s with
inline icon buttons or full-row clicks.

**Scope (approved: "action-bearing tables only").** Convert the hand-rolled
tables that have real per-row actions to the standard dropdown + context-menu
pattern. Leave read-only tables (journal, assignment history, audit log, asset
detail sub-tables) and matrix/segmented UIs (data-scope, field-permission
matrices; stock-opname segmented result cells) untouched — an empty dropdown
there is noise.

Tables to convert, with the actions each must expose:

| Screen | Actions (order; `—` = divider before) |
| --- | --- |
| `pages/depreciation.vue` (schedule) | Impair (disabled per permission/basis) |
| `pages/assets/index.vue` (table view) | View · Edit · Print label |
| `pages/transfers.vue` (history) | Ship (only when `row.canShip`) |
| `pages/disposals.vue` (history) | Attach BAST (only when `row.canAttach`) |
| `pages/peminjaman.vue` | Cancel (only when `row.canCancel`); keep row-expand timeline |
| `pages/stock-opname.vue` (items) | Set result: Found / Missing / Moved (when editable) |
| `pages/reports.vue` (opname) | Download Berita Acara PDF · Download Excel (when `canExport`) |
| `pages/maintenance.vue` (records) | Edit record (currently a full-row click) |

**Approach — introduce a shared `RowActionsMenu` component.** Several of these
tables cannot simply become `ResourceTable` (bulk-select checkboxes + grid
toggle on assets; expandable timeline on peminjaman; segmented result cells on
stock-opname; custom column layouts). Rather than force `ResourceTable`
everywhere, extract the dropdown+context-menu affordance into a small
auto-imported component:

- `RowActionsMenu` — props: `items: RowAction[]` (same `RowAction` shape
  `ResourceTable` already consumes: `{ label, icon, color?, disabled?,
  separator?, onSelect }`). Renders the kebab `UDropdownMenu` (reusing the
  existing `buildItems` grouping logic) **and** a `#context` slot / wrapper so a
  page can wire the same items into a right-click menu on its row.
- `ResourceTable` is refactored to consume `RowActionsMenu` internally (single
  source of truth for the grouping + rendering), so its behavior is unchanged.
- Hand-rolled tables gain an actions `<td>` that drops in `RowActionsMenu` with
  that row's `items`, and wrap their `<tbody>`/rows so the right-click context
  menu resolves the hovered row (mirroring `ResourceTable.onContextMenu`).

Where a table today has a **conditional single action** (transfers, disposals,
peminjaman, reports), the dropdown shows only the applicable item(s); when none
apply for a row, render no kebab (matching `ResourceTable`, which omits the
button when `buildItems` is empty). Disabled-but-shown actions (depreciation
Impair) keep the `disabled` + `title` semantics.

**i18n / a11y:** all action labels go through `$t`; kebab button keeps
`aria-label="{{ common.actions }}"`.

---

## 2 + 4. `/depreciation/schedule`: kill the double call, paginate, push
aggregation into SQL (#2, #4)

### Root cause

- **Double call.** `watch([period, basis])` fires both `loadSchedule()`
  (filtered → table + tfoot totals) and `loadKpis()` (unfiltered → the four KPI
  tiles). Both hit `GET /depreciation/schedule`; on first load / period change
  they are two near-identical requests.
- **~2 s latency.** The service runs **three unbounded queries**
  (`ListEntriesForPeriod`, `ListAssetsForScheduleUnion`,
  `SumAmountsThroughPeriodByAsset` — the last over the *entire* entry history,
  not scope/period-bounded), applies `search`/`category_id`/`office_id`
  **in Go after** fetching everything, and accumulates all KPI/Totals
  **row-by-row in Go** with `math/big` (5–7 `big.Rat` parses/adds per asset)
  over an unbounded set. Latency scales with total asset count (demo seed:
  ~1 500 assets × 42 offices).

### Key insight

The KPI tiles and tfoot totals are all **SQL-aggregatable** — no depreciation
math needs to move to SQL. Per-row `opening`/`amount`/`accumulated`/`closing`
are either persisted on the entry or derived as `cost − accumulated`; the
engine (`ResolveCommercial`/`ResolveFiscal`) is only needed to resolve
`method`/`life_months` for **display**, which we now do for the 10 visible rows
only. The engine's `Skip` predicate (the reason union rows get dropped in Go) is
entirely column checks and therefore **expressible in SQL**:

- Common: `a.capitalized AND a.status <> 'disposed' AND a.purchase_cost IS NOT NULL
  AND a.purchase_cost <> '' AND a.purchase_date IS NOT NULL`
- Commercial: `COALESCE(a.depreciation_method, c.default_depreciation_method) IS NOT NULL
  AND COALESCE(a.useful_life_months, c.default_useful_life_months) IS NOT NULL`
- Fiscal: `COALESCE(a.fiscal_group, c.default_fiscal_group) IS NOT NULL
  AND COALESCE(a.fiscal_group, c.default_fiscal_group) <> 'non_susut'`

### New backend design

Replace the two list queries + Go loop with a single **asset-based row set**
(`asset.assets a JOIN categories c LEFT JOIN offices o LEFT JOIN
depreciation_entries e` on this period+basis `LEFT JOIN acc` = per-asset
`SUM(amount) WHERE period ≤ target`), gated by:

```
WHERE a.deleted_at IS NULL
  AND (scope: all_scope OR a.office_id = ANY(office_ids))
  AND ( e.id IS NOT NULL                 -- has an entry this period (entry row)
        OR <parameterizable(basis)> )    -- else a valid "union" row
```

Per-row display values via `CASE WHEN e.id IS NOT NULL`:
- `opening`  = entry ? `e.opening_value`      : `cost − acc`
- `amount`   = entry ? `e.depreciation_amount`: `0.00`
- `accumulated` = `COALESCE(acc.accumulated, 0)`
- `closing`  = entry ? `e.closing_value`      : `cost − acc`
- `fully_depreciated` = `e.id IS NULL`

Three sqlc queries (basis selected via an `is_commercial boolean` param that
switches the parameterizable predicate through a `CASE`, avoiding 6 near-dup
queries):

1. **`ScheduleRows`** — the WHERE above **plus** the table filters
   (`search` ILIKE name/tag, optional `category_id`, optional `office_id`),
   `ORDER BY a.name`, `LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset)`. Returns
   the page's asset+category rows and the four CASE-derived money strings.
   `method`/`life_months` resolved in Go for these ≤10 rows only.
2. **`ScheduleTotals`** — same WHERE **with** filters, aggregate:
   `COUNT(*)`, `SUM(opening_expr)`, `SUM(amount_expr)`, `SUM(acc_expr)`,
   `SUM(closing_expr)`. Drives the tfoot **and** `total_count` for pagination.
3. **`ScheduleKpi`** — same WHERE **without** the table filters
   (period + basis + scope only): `SUM(purchase_cost)` = total_cost,
   `SUM(acc_expr)` = total_accumulated, `SUM(closing_expr)` = total_book_value,
   `SUM(amount_expr)` = period_expense. Matches today's "KPIs ignore table
   filters" behavior.

The old `ListEntriesForPeriod` / `ListAssetsForScheduleUnion` /
`SumAmountsThroughPeriodByAsset` stay if the **journal** path still needs them;
otherwise remove those no longer referenced. (Plan step must check the journal
handler's usage before deleting.)

**Deliberate, documented tightening:** the old entry-row query did not filter
`a.deleted_at IS NULL` (union query did). The unified query applies
`a.deleted_at IS NULL` uniformly — the schedule no longer shows entries for
soft-deleted assets. This is a consistency fix, not a mockup deviation; record
it in `PROGRESS.md`.

### New response shape

`GET /depreciation/schedule?period&basis&search&category_id&office_id&limit&offset`

```jsonc
{
  "kpi":   { "total_cost","total_accumulated","total_book_value","period_expense" },
  "rows":  [ ScheduleRow, ... ],          // one page
  "totals":{ "opening","amount","accumulated","closing" },
  "total": <int>,                          // filtered row count (for pagination)
  "limit": 10,
  "offset": 0
}
```

`limit` clamped 1–100 (default 10, per `clampInt`); `offset ≥ 0`. Handler passes
the new params through to the service; DTO `scheduleToMap` extended with
`total/limit/offset`. `openapi.yaml` updated. Because the KPI now ships inside
this one response, the frontend drops the separate KPI request entirely — one
call replaces two.

### Frontend depreciation page

- `useDepreciation.schedule()` gains `limit`/`offset` params and the new
  response type (`total`, and `kpi` now authoritative from the same call).
- Delete `loadKpis()` / `kpiResp` / `kpiSeq`; derive KPI tiles from
  `scheduleResp.kpi`, and `kpiAssetCount` from `scheduleResp.total`.
- Add `offset` state (page size 10, per #5); render the shared
  `TablePagination` under the schedule table (`total = scheduleResp.total`).
- Reset `offset → 0` whenever `period`, `basis`, `debouncedSearch`,
  `categoryId`, or `officeId` change (the existing watchers gain the reset).
- `scheduleSeq` guard stays (stale-response protection).

### Tests

- **Backend:** parity tests asserting the new SQL-aggregated KPI/Totals equal
  the old Go-loop results across representative fixtures (entry rows, union
  rows, impaired asset where `closing ≠ cost − acc`, disposed-with-entry,
  fully-depreciated, non-parameterizable/non_susut). Pagination tests
  (`limit`/`offset`, `total` correctness, ordering stable by name). Filter tests
  (search/category/office shrink `total` and rows but not `kpi`). Scope tests
  (office-scoped caller sees only their subtree on rows, totals, **and** kpi).
- **Frontend:** unit test that `schedule()` forwards `limit`/`offset`; runtime
  mount test that changing page refetches with new `offset` and that a
  filter/period/basis change resets to page 1; assert KPI tiles read from the
  single response (only one `/schedule` call per change — regression guard for
  the double call).
- **e2e:** existing depreciation flow stays green; add a pagination step
  (next/prev changes rows, KPI tiles unchanged).

---

## 3. Depreciation KPI card overflow (#3)

Large rupiah totals (e.g. `Rp 1.234.567.890.000`) overflow the fixed KPI tiles.

- Add `formatRupiahCompact(value)` to `app/utils/format.ts`: renders
  `Rp 1,23 M` / `Rp 3,4 T` (Indonesian scale: rb / jt / M / T) with 1–2
  significant decimals; falls back to `formatRupiah` for small values. Add
  `en` scale words where the util is locale-aware, matching existing
  `formatRupiah` conventions.
- In `depreciation.vue`, the four KPI tile values use `formatRupiahCompact`,
  wrapped with `min-w-0 truncate` and a `:title="formatRupiah(exact)"` so the
  full-precision figure is available on hover. Table tfoot totals keep
  `formatRupiah` (full precision).
- Unit tests for `formatRupiahCompact` across boundaries (0, < 1 000, thousands,
  millions, billions, trillions, negative, null/empty, rounding at scale edges
  like 999 999 → `Rp 1 jt` vs `Rp 999,99 rb`).

---

## 5. Pagination limit = 10 everywhere (#5)

Set every list-screen page size to 10:

| File | Change |
| --- | --- |
| `components/ResourceTable.vue` | prop default `limit: 20 → 10` |
| `pages/assets/index.vue` | `PAGE_SIZE 20 → 10` |
| `pages/settings/audit.vue` | `PAGE_SIZE 20 → 10` |
| `pages/master/reference.vue` | `limit = ref(20) → ref(10)` |
| `pages/master/employees.vue` | `limit = ref(20) → ref(10)` |
| `pages/master/categories.vue` | `PAGE_SIZE 7 → 10` |
| `pages/depreciation.vue` | new page size = 10 (from #2/#4) |

Align the composable fallbacks `?? 20 → ?? 10` (`useAssets`, `useCategories`,
`useEmployees`, `useOffices`, `useReference`) and the backend list-`limit`
default clamp where it defaults to 20, so an omitted `limit` yields 10 too.

**Do not touch** (not table pagination): async-search-picker fetch limits
(`limit: 20`), eager lookup/id→name fetches (`limit: 100`), existence checks
(`limit: 1`), `maintenance.myReports({limit:50})`, `assets/label` print
`perPage`. `settings/users.vue` is already 10.

Any e2e/unit tests that assert a specific page size or a page-count derived from
20 get updated to 10.

---

## Work order & verification

1. **Backend depreciation** (highest risk): migration-free — new
   `db/queries/depreciation.sql` queries → `sqlc generate` → service rewrite →
   handler params → DTO → `openapi.yaml` → backend tests. Gate: `go build`,
   `go vet`, `go test ./...`, Spectral lint.
2. **Frontend depreciation** (#2/#3/#4): composable + page + `formatRupiahCompact`
   + tests.
3. **#1** `RowActionsMenu` + per-table conversions + tests.
4. **#5** limit sweep + test updates.
5. Frontend gate: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`;
   depreciation e2e. Verify the schedule endpoint is hit **once** per change and
   under the old latency using the browser network panel.
6. Update `docs/PROGRESS.md` (tick the items, note the deleted-asset tightening
   deviation) and the Obsidian vault status/module notes.

All on `feat/depreciation-perf-and-table-ux`; Conventional Commits per area
(`fix(depreciation):`, `feat(ux):`, `fix(ux):`).
