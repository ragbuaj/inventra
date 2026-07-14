# Depreciation perf + table-UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/depreciation/schedule` fast + single-call + paginated, fix the KPI-card overflow, standardize row actions across action-bearing tables, and set every table page size to 10.

**Architecture:** Backend replaces three unbounded queries + a full-set Go loop with one asset-based paginated row query plus two SQL aggregate queries (filtered totals+count, unfiltered KPIs); the engine only resolves method/life for the ≤10 visible rows. Frontend drops the redundant KPI request, adds server pagination, compact KPI money formatting, and a shared `RowActionsMenu` (kebab dropdown + right-click context menu) reused by `ResourceTable` and the hand-rolled tables.

**Tech Stack:** Go 1.25 + Gin + sqlc/pgx (backend); Nuxt 4 + Nuxt UI (`U*`) + Vitest/Playwright (frontend); PostgreSQL 16.

## Global Constraints

- Branch: `feat/depreciation-perf-and-table-ux`. Conventional Commits per area (`fix(depreciation):`, `feat(ux):`, `fix(ux):`, `feat(db):`). No AI/co-author trailers.
- Money/numeric columns are Go `string` (sqlc override); compute with `math/big` or in SQL, never float.
- Backend gate: `go build ./...`, `go vet ./...`, `go test ./...`, and integration `go test -tags=integration ./...`; Spectral lint `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`.
- Frontend gate: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` (run from `frontend/`). ESLint: no trailing commas, 1tbs braces.
- i18n mandatory: every user-facing string in `i18n/locales/{id,en}.json`, referenced via `$t`. Default locale `id`.
- List endpoints return `{data,total,limit,offset}`-style envelopes; `limit` clamped 1–100 via `clampInt`.
- Don't hand-edit `backend/db/sqlc/` — edit `db/queries/*.sql` + migrations, then `sqlc generate`.
- Integration tests need the dev stack up: `docker compose -f docker-compose.dev.yml up -d` and `DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"`.

---

## File map

**Backend**
- Modify `backend/db/queries/depreciation.sql` — add `ScheduleRows`, `ScheduleTotals`, `ScheduleKpi`; remove `ListAssetsForScheduleUnion`, `SumAmountsThroughPeriodByAsset` (only Schedule used them; keep `ListEntriesForPeriod` — journal still uses it).
- Regenerate `backend/db/sqlc/depreciation.sql.go` + `querier.go` via `sqlc generate`.
- Modify `backend/internal/depreciation/service.go` — rewrite `Schedule`; `ScheduleResult` gains `Total int64`.
- Modify `backend/internal/depreciation/handler.go` — `schedule` parses `limit`/`offset`; add local `clampInt`.
- Modify `backend/internal/depreciation/dto.go` — `scheduleToMap` adds `total`/`limit`/`offset`.
- Modify `backend/api/openapi.yaml` — schedule query params + response fields.
- Modify `backend/internal/depreciation/depreciation_integration_test.go` — parity/pagination/filter/scope tests.

**Frontend**
- Modify `frontend/app/utils/format.ts` — add `formatRupiahCompact`.
- Create `frontend/test/unit/format-compact.spec.ts` — util tests.
- Modify `frontend/app/composables/api/useDepreciation.ts` — `schedule()` `limit`/`offset`, `ScheduleResponse.total`.
- Modify `frontend/app/pages/depreciation.vue` — drop KPI call, pagination, compact KPI cards, Impair-in-menu.
- Create `frontend/app/components/RowActionsMenu.vue` — shared kebab + context menu.
- Modify `frontend/app/components/ResourceTable.vue` — consume `RowActionsMenu`.
- Modify hand-rolled tables: `assets/index.vue`, `transfers.vue`, `disposals.vue`, `peminjaman.vue`, `stock-opname.vue`, `reports.vue`, `maintenance.vue`.
- Modify page sizes: `ResourceTable.vue`, `assets/index.vue`, `settings/audit.vue`, `master/reference.vue`, `master/employees.vue`, `master/categories.vue`; composable fallbacks in `useAssets/useCategories/useEmployees/useOffices/useReference`.
- Tests: `frontend/test/nuxt/*` for the depreciation page + RowActionsMenu; update any page-size assertions.

**Docs:** `docs/PROGRESS.md`, Obsidian vault status/module notes.

---

## Task 1: New schedule SQL queries + sqlc regenerate

**Files:**
- Modify: `backend/db/queries/depreciation.sql`
- Regenerate: `backend/db/sqlc/depreciation.sql.go`, `backend/db/sqlc/querier.go`

**Interfaces:**
- Produces: sqlc methods `ScheduleRows(ctx, ScheduleRowsParams) ([]ScheduleRowsRow, error)`, `ScheduleTotals(ctx, ScheduleTotalsParams) (ScheduleTotalsRow, error)`, `ScheduleKpi(ctx, ScheduleKpiParams) (ScheduleKpiRow, error)`. Params carry `Basis`, `Period pgtype.Date`, `AllScope bool`, `OfficeIds []uuid.UUID`, `IsCommercial bool`; `ScheduleRows`/`ScheduleTotals` additionally carry `Search *string`, `CategoryID *uuid.UUID`, `OfficeID *uuid.UUID`; `ScheduleRows` also `Lim int32`, `Off int32`. `ScheduleRowsRow` embeds `AssetAsset`, `MasterdataCategory`, plus `OfficeName *string`, `HasEntry bool`, `EntryMethod *sqlc.SharedDepreciationMethod`, `Opening/Amount/Accumulated/Closing string`.

- [ ] **Step 1: Remove the two now-unused queries**

In `backend/db/queries/depreciation.sql`, delete the `ListAssetsForScheduleUnion` block (the `-- name: ListAssetsForScheduleUnion :many` comment through its trailing `ORDER BY a.name;`) and the `SumAmountsThroughPeriodByAsset` block. Leave `ListEntriesForPeriod` intact (journal uses it).

- [ ] **Step 2: Append the three new queries**

Add to `backend/db/queries/depreciation.sql`:

```sql
-- name: ScheduleRows :many
-- One asset-based, paginated schedule page. A row is included if it has an
-- entry for this period+basis (entry row) OR the asset is a parameterizable
-- "union" row (fully depreciated, no entry this period). The parameterizable
-- predicate mirrors ResolveCommercial/ResolveFiscal's Skip checks in SQL.
SELECT sqlc.embed(a), sqlc.embed(c),
       o.name AS office_name,
       (e.id IS NOT NULL) AS has_entry,
       e.method AS entry_method,
       CASE WHEN e.id IS NOT NULL THEN e.opening_value
            ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2)::text END AS opening,
       CASE WHEN e.id IS NOT NULL THEN e.depreciation_amount ELSE '0.00' END AS amount,
       acc.accumulated AS accumulated,
       CASE WHEN e.id IS NOT NULL THEN e.closing_value
            ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2)::text END AS closing
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN depreciation.depreciation_entries e
       ON e.asset_id = a.id AND e.basis = sqlc.arg(basis)
      AND e.period = sqlc.arg(period) AND e.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(de.depreciation_amount), 0)::text AS accumulated
  FROM depreciation.depreciation_entries de
  WHERE de.asset_id = a.id AND de.basis = sqlc.arg(basis)
    AND de.period <= sqlc.arg(period) AND de.deleted_at IS NULL
) acc ON true
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    e.id IS NOT NULL
    OR (
      a.capitalized AND a.status <> 'disposed'
      AND a.purchase_cost IS NOT NULL AND a.purchase_cost <> ''
      AND a.purchase_date IS NOT NULL
      AND (
        (sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.depreciation_method, c.default_depreciation_method) IS NOT NULL
         AND COALESCE(a.useful_life_months, c.default_useful_life_months) IS NOT NULL)
        OR (NOT sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) IS NOT NULL
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) <> 'non_susut')
      )
    )
  )
  AND (sqlc.narg(search)::text IS NULL
       OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_id)::uuid IS NULL OR a.office_id = sqlc.narg(office_id))
ORDER BY a.name, a.id
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ScheduleTotals :one
-- Filtered tfoot totals + row count (same FROM/WHERE as ScheduleRows, incl.
-- the search/category/office filters, no pagination).
SELECT
  COUNT(*) AS total,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.opening_value::numeric
                          ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2) END), 2), 0)::text AS opening,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.depreciation_amount::numeric ELSE 0 END), 2), 0)::text AS amount,
  COALESCE(round(SUM(acc.accumulated::numeric), 2), 0)::text AS accumulated,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.closing_value::numeric
                          ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2) END), 2), 0)::text AS closing
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN depreciation.depreciation_entries e
       ON e.asset_id = a.id AND e.basis = sqlc.arg(basis)
      AND e.period = sqlc.arg(period) AND e.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(de.depreciation_amount), 0)::text AS accumulated
  FROM depreciation.depreciation_entries de
  WHERE de.asset_id = a.id AND de.basis = sqlc.arg(basis)
    AND de.period <= sqlc.arg(period) AND de.deleted_at IS NULL
) acc ON true
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    e.id IS NOT NULL
    OR (
      a.capitalized AND a.status <> 'disposed'
      AND a.purchase_cost IS NOT NULL AND a.purchase_cost <> ''
      AND a.purchase_date IS NOT NULL
      AND (
        (sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.depreciation_method, c.default_depreciation_method) IS NOT NULL
         AND COALESCE(a.useful_life_months, c.default_useful_life_months) IS NOT NULL)
        OR (NOT sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) IS NOT NULL
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) <> 'non_susut')
      )
    )
  )
  AND (sqlc.narg(search)::text IS NULL
       OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_id)::uuid IS NULL OR a.office_id = sqlc.narg(office_id));

-- name: ScheduleKpi :one
-- Unfiltered KPI tiles (period + basis + scope only — table filters must never
-- shrink the tiles). Same FROM/WHERE as ScheduleTotals MINUS the search/
-- category/office filters.
SELECT
  COALESCE(round(SUM(a.purchase_cost::numeric), 2), 0)::text AS total_cost,
  COALESCE(round(SUM(acc.accumulated::numeric), 2), 0)::text AS total_accumulated,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.closing_value::numeric
                          ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2) END), 2), 0)::text AS total_book_value,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.depreciation_amount::numeric ELSE 0 END), 2), 0)::text AS period_expense
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN depreciation.depreciation_entries e
       ON e.asset_id = a.id AND e.basis = sqlc.arg(basis)
      AND e.period = sqlc.arg(period) AND e.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(de.depreciation_amount), 0)::text AS accumulated
  FROM depreciation.depreciation_entries de
  WHERE de.asset_id = a.id AND de.basis = sqlc.arg(basis)
    AND de.period <= sqlc.arg(period) AND de.deleted_at IS NULL
) acc ON true
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    e.id IS NOT NULL
    OR (
      a.capitalized AND a.status <> 'disposed'
      AND a.purchase_cost IS NOT NULL AND a.purchase_cost <> ''
      AND a.purchase_date IS NOT NULL
      AND (
        (sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.depreciation_method, c.default_depreciation_method) IS NOT NULL
         AND COALESCE(a.useful_life_months, c.default_useful_life_months) IS NOT NULL)
        OR (NOT sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) IS NOT NULL
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) <> 'non_susut')
      )
    )
  );
```

- [ ] **Step 3: Regenerate sqlc**

Run (from `backend/`): `sqlc generate`
Expected: no errors; `git status` shows `db/sqlc/depreciation.sql.go` and `db/sqlc/querier.go` changed. Confirm the two removed queries no longer appear in `querier.go` and the three new methods do.

- [ ] **Step 4: Verify it compiles (service still references old queries → expected break)**

Run: `go build ./...`
Expected: FAIL — `s.q.ListAssetsForScheduleUnion`/`SumAmountsThroughPeriodByAsset` undefined in `service.go`. That is fixed in Task 2. This step only confirms sqlc generated the new methods (no sqlc-side errors).

- [ ] **Step 5: Commit**

```bash
git add backend/db/queries/depreciation.sql backend/db/sqlc/
git commit -m "feat(db): SQL-aggregated, paginated depreciation schedule queries"
```

---

## Task 2: Rewrite `Service.Schedule` (SQL aggregation + pagination) with parity tests

**Files:**
- Modify: `backend/internal/depreciation/service.go` (`Schedule` ~553-713; `ScheduleResult` ~535-541)
- Test: `backend/internal/depreciation/depreciation_integration_test.go`

**Interfaces:**
- Consumes: `ScheduleRows`/`ScheduleTotals`/`ScheduleKpi` from Task 1; existing `resolveScheduleParams`, `ResolveCommercial`, `ResolveFiscal`, `isImpaired`.
- Produces: `func (s *Service) Schedule(ctx context.Context, period time.Time, basis sqlc.SharedDepreciationBasis, allScope bool, officeIDs []uuid.UUID, search string, categoryID, officeID *uuid.UUID, limit, offset int32) (ScheduleResult, error)`; `ScheduleResult` gains `Total int64`.

- [ ] **Step 1: Write the failing integration test (parity + pagination)**

Add to `depreciation_integration_test.go` (follow the file's existing harness for seeding a period + entries + assets). Key assertions:

```go
func TestSchedulePaginationAndParity(t *testing.T) {
    // ... existing setup: svc, ctx, a computed period P, basis commercial,
    // seeded so there are >2 schedule rows (mix of entry rows + one fully-
    // depreciated union row + one impaired asset where closing != cost-acc).

    // Full page (limit big enough for all rows).
    full, err := svc.Schedule(ctx, P, commercial, true, nil, "", nil, nil, 100, 0)
    require.NoError(t, err)
    require.Equal(t, int64(len(full.Rows)), full.Total)

    // KPI/Totals must equal the pre-rewrite Go-loop reference values captured
    // for this fixture (hard-code the expected decimal strings from the seed).
    require.Equal(t, wantTotalCost, full.KPI.TotalCost)
    require.Equal(t, wantBookValue, full.KPI.TotalBookValue)
    require.Equal(t, wantPeriodExpense, full.KPI.PeriodExpense)
    require.Equal(t, wantTfootAmount, full.Totals.Amount)

    // Page 1 of size 2 + page 2 of size 2 == the full ordered set, no overlap.
    p1, _ := svc.Schedule(ctx, P, commercial, true, nil, "", nil, nil, 2, 0)
    p2, _ := svc.Schedule(ctx, P, commercial, true, nil, "", nil, nil, 2, 2)
    require.Len(t, p1.Rows, 2)
    require.Equal(t, full.Total, p1.Total) // total unaffected by paging
    require.Equal(t, full.Rows[0].AssetID, p1.Rows[0].AssetID)
    require.Equal(t, full.Rows[2].AssetID, p2.Rows[0].AssetID)

    // A search filter shrinks rows+total+tfoot but NOT the kpi tiles.
    filtered, _ := svc.Schedule(ctx, P, commercial, true, nil, full.Rows[0].AssetName, nil, nil, 100, 0)
    require.Less(t, filtered.Total, full.Total)
    require.Equal(t, full.KPI.TotalCost, filtered.KPI.TotalCost)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags=integration ./internal/depreciation/ -run TestSchedulePaginationAndParity -v`
Expected: FAIL to compile (`Schedule` arity changed / `.Total` missing).

- [ ] **Step 3: Rewrite `Schedule` and `ScheduleResult`**

Add `Total int64` to `ScheduleResult`. Replace the body of `Schedule` (new signature above) with:

```go
func (s *Service) Schedule(ctx context.Context, period time.Time, basis sqlc.SharedDepreciationBasis, allScope bool, officeIDs []uuid.UUID, search string, categoryID, officeID *uuid.UUID, limit, offset int32) (ScheduleResult, error) {
    target := pgtype.Date{Time: firstOfMonth(period), Valid: true}
    isCommercial := basis == sqlc.SharedDepreciationBasisCommercial
    var searchArg *string
    if s := strings.TrimSpace(search); s != "" {
        searchArg = &s
    }

    rowsRaw, err := s.q.ScheduleRows(ctx, sqlc.ScheduleRowsParams{
        Basis: basis, Period: target, AllScope: allScope, OfficeIds: officeIDs,
        IsCommercial: isCommercial, Search: searchArg, CategoryID: categoryID,
        OfficeID: officeID, Lim: limit, Off: offset,
    })
    if err != nil {
        return ScheduleResult{}, err
    }
    tot, err := s.q.ScheduleTotals(ctx, sqlc.ScheduleTotalsParams{
        Basis: basis, Period: target, AllScope: allScope, OfficeIds: officeIDs,
        IsCommercial: isCommercial, Search: searchArg, CategoryID: categoryID, OfficeID: officeID,
    })
    if err != nil {
        return ScheduleResult{}, err
    }
    kpi, err := s.q.ScheduleKpi(ctx, sqlc.ScheduleKpiParams{
        Basis: basis, Period: target, AllScope: allScope, OfficeIds: officeIDs, IsCommercial: isCommercial,
    })
    if err != nil {
        return ScheduleResult{}, err
    }

    rows := make([]ScheduleRow, 0, len(rowsRaw))
    for _, r := range rowsRaw {
        a, c := r.AssetAsset, r.MasterdataCategory
        var method sqlc.SharedDepreciationMethod
        var life int32
        if r.HasEntry {
            em := sqlc.SharedDepreciationMethod("")
            if r.EntryMethod != nil {
                em = *r.EntryMethod
            }
            method, life = resolveScheduleParams(a, c, basis, em)
        } else {
            var params *Params
            if isCommercial {
                params, _ = ResolveCommercial(a, c)
            } else {
                params, _ = ResolveFiscal(a, c)
            }
            if params != nil {
                method, life = params.Method, params.LifeMonths
            }
        }
        rows = append(rows, ScheduleRow{
            AssetID: a.ID, AssetName: a.Name, AssetTag: a.AssetTag,
            CategoryName: c.Name, OfficeName: r.OfficeName,
            Method: method, LifeMonths: life,
            Opening: r.Opening, Amount: r.Amount,
            Accumulated: r.Accumulated, Closing: r.Closing,
            Impaired: isImpaired(a), FullyDepreciated: !r.HasEntry,
        })
    }

    return ScheduleResult{
        KPI: ScheduleKPI{
            TotalCost: kpi.TotalCost, TotalAccumulated: kpi.TotalAccumulated,
            TotalBookValue: kpi.TotalBookValue, PeriodExpense: kpi.PeriodExpense,
        },
        Rows:  rows,
        Total: tot.Total,
        Totals: ScheduleTotals{
            Opening: tot.Opening, Amount: tot.Amount,
            Accumulated: tot.Accumulated, Closing: tot.Closing,
        },
    }, nil
}
```

Remove now-dead helpers only if unused elsewhere (`addTotals` was inline; `parseMoney`/`roundHalfUp2` stay — engine uses them). Keep `firstOfMonth`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -tags=integration ./internal/depreciation/ -run TestSchedule -v`
Expected: PASS. Then `go build ./...` and `go vet ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/depreciation/service.go backend/internal/depreciation/depreciation_integration_test.go
git commit -m "fix(depreciation): SQL-aggregate + paginate schedule (kill full-set Go loop)"
```

---

## Task 3: Handler pagination params + DTO + OpenAPI

**Files:**
- Modify: `backend/internal/depreciation/handler.go` (`schedule` ~166-195; add `clampInt`)
- Modify: `backend/internal/depreciation/dto.go` (`scheduleToMap`)
- Modify: `backend/api/openapi.yaml`

**Interfaces:**
- Consumes: `Schedule(..., limit, offset int32)` and `ScheduleResult.Total` from Task 2.

- [ ] **Step 1: Add `clampInt` + wire limit/offset in the handler**

In `handler.go`, add the per-module helper (copy of `internal/user/handler.go:252`):

```go
func clampInt(raw string, def, min, max int32) int32 {
    if raw == "" {
        return def
    }
    n, err := strconv.Atoi(raw)
    if err != nil {
        return def
    }
    v := int32(n)
    if v < min {
        return min
    }
    if v > max {
        return max
    }
    return v
}
```

Add `"strconv"` to imports. In `schedule`, after resolving scope, replace the service call:

```go
limit := clampInt(c.Query("limit"), 10, 1, 100)
offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)
result, err := h.svc.Schedule(c.Request.Context(), period, basis, all, ids, c.Query("search"), categoryID, officeID, limit, offset)
if err != nil {
    h.svcError(c, err)
    return
}
c.JSON(http.StatusOK, scheduleToMap(result, limit, offset))
```

- [ ] **Step 2: Extend `scheduleToMap`**

Change the signature and add the pagination fields:

```go
func scheduleToMap(r ScheduleResult, limit, offset int32) gin.H {
    // ... unchanged rows/kpi/totals build ...
    m := gin.H{ /* kpi, rows, totals as before */ }
    m["total"] = r.Total
    m["limit"] = limit
    m["offset"] = offset
    return m
}
```

- [ ] **Step 3: Update OpenAPI**

In `backend/api/openapi.yaml`, on `GET /depreciation/schedule` add `limit` (integer, 1–100, default 10) and `offset` (integer, min 0, default 0) query params, and add `total`/`limit`/`offset` (integers) to the response schema alongside `kpi`/`rows`/`totals`.

- [ ] **Step 4: Verify**

Run: `go build ./... && go vet ./... && go test ./...`
Run: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: all clean.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/depreciation/handler.go backend/internal/depreciation/dto.go backend/api/openapi.yaml
git commit -m "feat(depreciation): schedule limit/offset params + total in response"
```

---

## Task 4: `formatRupiahCompact` util + tests (#3 helper)

**Files:**
- Modify: `frontend/app/utils/format.ts`
- Test: `frontend/test/unit/format-compact.spec.ts`

**Interfaces:**
- Produces: `formatRupiahCompact(value: string | number | null | undefined): string` (compact, e.g. `Rp 1,23 M`; `—` when absent/invalid).

- [ ] **Step 1: Write failing unit tests**

Create `frontend/test/unit/format-compact.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { formatRupiahCompact } from '~/utils/format'

describe('formatRupiahCompact', () => {
  it('returns em dash for absent/invalid', () => {
    expect(formatRupiahCompact(null)).toBe('—')
    expect(formatRupiahCompact('')).toBe('—')
    expect(formatRupiahCompact('abc')).toBe('—')
  })
  it('keeps small values ungrouped-compact', () => {
    expect(formatRupiahCompact(500)).toBe('Rp 500')
    expect(formatRupiahCompact('999')).toBe('Rp 999')
  })
  it('scales thousands/millions/billions/trillions', () => {
    expect(formatRupiahCompact(1500)).toBe('Rp 1,5 rb')
    expect(formatRupiahCompact(2_300_000)).toBe('Rp 2,3 jt')
    expect(formatRupiahCompact(1_234_567_890)).toBe('Rp 1,23 M')
    expect(formatRupiahCompact('1234567890000')).toBe('Rp 1,23 T')
  })
  it('handles negatives', () => {
    expect(formatRupiahCompact(-2_300_000)).toBe('-Rp 2,3 jt')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test format-compact`
Expected: FAIL (`formatRupiahCompact` not exported).

- [ ] **Step 3: Implement the util**

Append to `frontend/app/utils/format.ts`:

```ts
// Compact IDR for tight KPI tiles: 'Rp 1,23 M', 'Rp 3,4 T'. Full precision
// belongs in tables — pair this with a title tooltip carrying formatRupiah().
export function formatRupiahCompact(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—'
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return '—'
  const sign = n < 0 ? '-' : ''
  const abs = Math.abs(n)
  const scales: Array<{ v: number, s: string }> = [
    { v: 1e12, s: 'T' }, { v: 1e9, s: 'M' }, { v: 1e6, s: 'jt' }, { v: 1e3, s: 'rb' }
  ]
  for (const { v, s } of scales) {
    if (abs >= v) {
      const scaled = abs / v
      const digits = scaled >= 100 ? 0 : scaled >= 10 ? 1 : 2
      const num = scaled.toLocaleString('id-ID', { maximumFractionDigits: digits })
      return `${sign}Rp ${num} ${s}`
    }
  }
  return `${sign}Rp ${abs.toLocaleString('id-ID')}`
}
```

- [ ] **Step 4: Run to verify pass**

Run: `pnpm test format-compact`
Expected: PASS. (If a boundary assertion disagrees with the rounding, fix the expected string in the test to match the documented behavior — digits rule above — not the implementation.)

- [ ] **Step 5: Commit**

```bash
git add frontend/app/utils/format.ts frontend/test/unit/format-compact.spec.ts
git commit -m "feat(ux): formatRupiahCompact for compact money display"
```

---

## Task 5: Depreciation page — single call, pagination, compact KPI cards (#2/#3/#4)

**Files:**
- Modify: `frontend/app/composables/api/useDepreciation.ts` (`ScheduleResponse`, `ScheduleQuery`, `schedule()`)
- Modify: `frontend/app/pages/depreciation.vue`
- Test: `frontend/test/nuxt/use-depreciation.spec.ts`, and a page test `frontend/test/nuxt/depreciation-page.spec.ts` (create if absent)

**Interfaces:**
- Consumes: `formatRupiahCompact` (Task 4); backend `total/limit/offset` (Task 3).
- Produces: `ScheduleQuery` gains `limit?: number`, `offset?: number`; `ScheduleResponse` gains `total: number`.

- [ ] **Step 1: Extend the composable + type**

In `useDepreciation.ts`: add `total: number` to `ScheduleResponse`; add `limit?`/`offset?` to `ScheduleQuery`; in `schedule()` forward them:

```ts
async function schedule(q: ScheduleQuery): Promise<ScheduleResponse> {
  const query: Record<string, string> = { period: q.period, basis: q.basis }
  if (q.search !== undefined) query.search = q.search
  if (q.category_id !== undefined) query.category_id = q.category_id
  if (q.office_id !== undefined) query.office_id = q.office_id
  if (q.limit !== undefined) query.limit = String(q.limit)
  if (q.offset !== undefined) query.offset = String(q.offset)
  return request<ScheduleResponse>('/depreciation/schedule', { query })
}
```

- [ ] **Step 2: Write failing test — one call per change + pagination forwards offset**

In `use-depreciation.spec.ts` add a test that `schedule({period,basis,limit:10,offset:10})` issues a request whose query has `limit=10&offset=10`. In `depreciation-page.spec.ts` (mountSuspended, `// @vitest-environment nuxt`) stub `useDepreciation` and assert: on mount `schedule` is called **once** (not twice) and KPI tiles render from that single response; clicking next page refetches with `offset=10`; changing the category filter resets to `offset=0`.

- [ ] **Step 3: Run to verify fail**

Run: `pnpm test use-depreciation depreciation-page`
Expected: FAIL (offset not forwarded / two calls / no pagination control).

- [ ] **Step 4: Rewire the page**

In `depreciation.vue`:
- Delete `kpiResp`, `kpiLoading`, `kpiSeq`, `loadKpis()`, and the `loadKpis()` calls in `computePeriod`/`closePeriod`/the `watch([period,basis])`.
- Add `const PAGE_SIZE = 10`, `const offset = ref(0)`.
- In `loadSchedule()` pass `limit: PAGE_SIZE, offset: offset.value` to `depApi.schedule(...)`.
- `kpiLoading` → reuse `scheduleLoading`; `kpiItems` reads `scheduleResp.value?.kpi` and `kpiAssetCount = scheduleResp.value?.total ?? 0`.
- Reset paging: `watch([period, basis], () => { offset.value = 0; loadSchedule(); loadJournal() })` and `watch([debouncedSearch, categoryId, officeId], () => { offset.value = 0; loadSchedule() })`.
- Under the schedule table card, add pagination:

```vue
<TablePagination
  v-if="(scheduleResp?.total ?? 0) > 0"
  :total="scheduleResp?.total ?? 0"
  :limit="PAGE_SIZE"
  :offset="offset"
  @update:offset="(v) => { offset = v; loadSchedule() }"
/>
```

- KPI card value → compact + tooltip. Replace the tile value block:

```vue
<div
  v-else
  class="text-[22px] font-bold tracking-tight mt-2 min-w-0 truncate"
  :class="k.valueClass"
  :title="k.exact"
>
  {{ k.value }}
</div>
```

and in `kpiItems`, set `value: formatRupiahCompact(kpi?.…)` and `exact: formatRupiah(kpi?.…)` per tile (add `import { formatRupiahCompact } from '~/utils/format'`).

- [ ] **Step 5: Run tests + typecheck**

Run: `pnpm test use-depreciation depreciation-page && pnpm typecheck`
Expected: PASS / clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/composables/api/useDepreciation.ts frontend/app/pages/depreciation.vue frontend/test/nuxt/
git commit -m "fix(depreciation): single schedule fetch, server pagination, compact KPI cards"
```

---

## Task 6: Extract `RowActionsMenu`; refactor `ResourceTable` to use it (#1 foundation)

**Files:**
- Create: `frontend/app/components/RowActionsMenu.vue`
- Modify: `frontend/app/components/ResourceTable.vue`
- Test: `frontend/test/nuxt/row-actions-menu.spec.ts` (create); keep existing `ResourceTable`/consumer tests green.

**Interfaces:**
- Produces: `RowActionsMenu` — props `items: RowAction[]` (the existing `RowAction` type from `~/types`). Renders the kebab `UDropdownMenu` (grouped via the existing `buildItems` logic, moved here). Exposes a helper `useRowActionGroups(items)` OR keeps grouping internal. Also exports (for hand-rolled tables) a `buildActionGroups(items: RowAction[]): DropdownMenuItem[][]` from a new `~/utils/rowActions.ts` so both the menu and page-level `UContextMenu` share grouping.

- [ ] **Step 1: Write failing test**

`row-actions-menu.spec.ts` (mountSuspended): given `items=[{label:'Edit',icon:'i-lucide-pencil',onSelect},{label:'Delete',color:'error',separator:true,onSelect}]`, the component renders a kebab button with `aria-label`, opening it shows both labels, and selecting "Edit" invokes its `onSelect`. Given `items=[]`, no kebab button renders.

- [ ] **Step 2: Run to verify fail**

Run: `pnpm test row-actions-menu`
Expected: FAIL (component missing).

- [ ] **Step 3: Implement + refactor**

Move `buildItems` grouping into `frontend/app/utils/rowActions.ts` as `buildActionGroups(items)`. Create `RowActionsMenu.vue` rendering the `UDropdownMenu` + kebab `UButton` (copy the markup from `ResourceTable.vue:189-201`), returning nothing when `buildActionGroups(items).length === 0`. Refactor `ResourceTable.vue` to import `buildActionGroups` (replace its local `buildItems`) and use `<RowActionsMenu :items="props.actions(row.original)" />` in the `__actions-cell`; keep the existing `UContextMenu` using `buildActionGroups`.

- [ ] **Step 4: Run tests**

Run: `pnpm test row-actions-menu resource-table && pnpm typecheck`
Expected: PASS (existing ResourceTable consumer tests unchanged).

- [ ] **Step 5: Commit**

```bash
git add frontend/app/components/RowActionsMenu.vue frontend/app/utils/rowActions.ts frontend/app/components/ResourceTable.vue frontend/test/nuxt/row-actions-menu.spec.ts
git commit -m "feat(ux): shared RowActionsMenu (kebab + context menu grouping)"
```

---

## Task 7: Convert `assets/index.vue` table to RowActionsMenu (#1)

**Files:**
- Modify: `frontend/app/pages/assets/index.vue` (table view actions ~561-587; add `@contextmenu` on tbody)
- Test: `frontend/test/nuxt/assets-index.spec.ts` (add/extend)

**Interfaces:**
- Consumes: `RowActionsMenu`, `buildActionGroups` (Task 6).

- [ ] **Step 1: Write failing test**

Assert the assets table row exposes a kebab menu with **View / Edit / Print label**, each firing its handler (navigate to detail, open edit, print label), and a right-click on a row opens the same items.

- [ ] **Step 2: Run to verify fail**

Run: `pnpm test assets-index`
Expected: FAIL (inline icon buttons only).

- [ ] **Step 3: Implement**

Replace the inline icon-button cell with an actions `<td>` holding `<RowActionsMenu :items="rowActions(row)" />`, where `rowActions(row)` returns `[{label:$t('common.view'),icon:'i-lucide-eye',onSelect:()=>goDetail(row)},{label:$t('common.edit'),icon:'i-lucide-pencil',onSelect:()=>openEdit(row)},{label:$t('assets.printLabel'),icon:'i-lucide-printer',onSelect:()=>printLabel(row)}]`. Add a tbody `@contextmenu` handler + `UContextMenu` mirroring `ResourceTable.onContextMenu`, or wrap rows so right-click resolves the row. Preserve bulk-select checkboxes + grid toggle.

- [ ] **Step 4: Run tests**

Run: `pnpm test assets-index && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/assets/index.vue frontend/test/nuxt/assets-index.spec.ts
git commit -m "feat(ux): assets table row actions via RowActionsMenu"
```

---

## Task 8: Convert conditional-action tables — transfers, disposals, peminjaman (#1)

**Files:**
- Modify: `frontend/app/pages/transfers.vue` (history ~1034-1041), `frontend/app/pages/disposals.vue` (~1264-1271), `frontend/app/pages/peminjaman.vue` (~502-513)
- Test: extend each page's nuxt test (create if absent).

**Interfaces:** Consumes `RowActionsMenu`/`buildActionGroups`.

- [ ] **Step 1: Write failing tests**

For each page assert: a row where the action applies shows the kebab with the single action (transfers → **Ship** when `row.canShip`; disposals → **Attach BAST** when `row.canAttach`; peminjaman → **Cancel** when `row.canCancel`), firing its handler; a row where it does not apply shows **no** kebab. Peminjaman: row-expand timeline still toggles.

- [ ] **Step 2: Run to verify fail**

Run: `pnpm test transfers disposals peminjaman`
Expected: FAIL.

- [ ] **Step 3: Implement (per file)**

Replace each conditional inline button with `<RowActionsMenu :items="rowActions(row)" />` where `rowActions` returns `[]` when the condition is false (menu renders nothing) else the single action:
- transfers: `row.canShip ? [{label:$t('transfers.ship'),icon:'i-lucide-send',onSelect:()=>openShip(row)}] : []`
- disposals: `row.canAttach ? [{label:$t('disposals.attachBast'),icon:'i-lucide-paperclip',onSelect:()=>openAttach(row)}] : []`
- peminjaman: `row.canCancel ? [{label:$t('peminjaman.cancel'),icon:'i-lucide-x',color:'error',onSelect:()=>cancel(row)}] : []` (keep the separate row-click that toggles the timeline; put the menu in its own cell so it doesn't trigger expand — stop propagation on the actions cell).

Add the tbody right-click `UContextMenu` wiring per page (same pattern as Task 7).

- [ ] **Step 4: Run tests**

Run: `pnpm test transfers disposals peminjaman && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/transfers.vue frontend/app/pages/disposals.vue frontend/app/pages/peminjaman.vue frontend/test/nuxt/
git commit -m "feat(ux): row-action menus on transfers/disposals/peminjaman"
```

---

## Task 9: Convert stock-opname items, reports opname, maintenance records (#1)

**Files:**
- Modify: `frontend/app/pages/stock-opname.vue` (items ~853-867), `frontend/app/pages/reports.vue` (opname ~720-737), `frontend/app/pages/maintenance.vue` (records ~788)
- Test: extend each page's nuxt test.

**Interfaces:** Consumes `RowActionsMenu`.

- [ ] **Step 1: Write failing tests**

- stock-opname: an editable item row exposes a kebab with **Set: Found / Missing / Moved**, each setting the item result; a locked session shows the status badge (no menu).
- reports opname: a row with `canExport` exposes **Download PDF** + **Download Excel** in the menu, each firing its export.
- maintenance: a record row exposes **Edit record** in the menu (when `canManage`), firing `openRecordEdit(row)`; keep whole-row click behavior OR move it into the menu — assert Edit is reachable via the menu.

- [ ] **Step 2: Run to verify fail**

Run: `pnpm test stock-opname reports maintenance`
Expected: FAIL.

- [ ] **Step 3: Implement**

- stock-opname: keep the segmented control if desired **and** add a `RowActionsMenu` whose items map to `SEG_ORDER` (`found/missing/moved`) via the existing setter, gated on `isEditable`. (Menu is the standardized affordance; the segmented buttons may remain as a fast-path.)
- reports: replace the two inline export icon buttons with a `RowActionsMenu` (`Download Berita Acara PDF`, `Download Excel`) gated on `canExport`.
- maintenance: add an actions column whose `RowActionsMenu` has `Edit record` → `openRecordEdit(row)` when `canManage`; keep the row click as a convenience.
- Add per-page right-click `UContextMenu` wiring.

- [ ] **Step 4: Run tests**

Run: `pnpm test stock-opname reports maintenance && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/stock-opname.vue frontend/app/pages/reports.vue frontend/app/pages/maintenance.vue frontend/test/nuxt/
git commit -m "feat(ux): row-action menus on stock-opname/reports/maintenance"
```

---

## Task 10: Pagination limit = 10 everywhere (#5)

**Files:**
- Modify: `frontend/app/components/ResourceTable.vue:20`; `frontend/app/pages/assets/index.vue:8`; `frontend/app/pages/settings/audit.vue:8`; `frontend/app/pages/master/reference.vue:30`; `frontend/app/pages/master/employees.vue:29`; `frontend/app/pages/master/categories.vue:15`.
- Modify composable fallbacks: `useAssets.ts:19`, `useCategories.ts:11`, `useEmployees.ts:20`, `useOffices.ts:22`, `useReference.ts:13` (`?? 20 → ?? 10`).
- Test: update any spec asserting a page size / page-count derived from 20 or 7.

**Interfaces:** none new.

- [ ] **Step 1: Update any failing-by-design tests first**

Grep specs for hardcoded page sizes: `pnpm exec grep -rnE "PAGE_SIZE|limit.*20|slice\(0, ?20\)|20 per|per page" test/`. Update expectations to 10 (and categories 7→10). Run the suite to see which fail now: `pnpm test`.

- [ ] **Step 2: Apply the constant changes**

`ResourceTable.vue:20` `limit: 20 → 10`; `assets/index.vue:8` `PAGE_SIZE = 20 → 10`; `settings/audit.vue:8` `PAGE_SIZE = 20 → 10`; `master/reference.vue:30` `ref(20) → ref(10)`; `master/employees.vue:29` `ref(20) → ref(10)`; `master/categories.vue:15` `PAGE_SIZE = 7 → 10`. Composable fallbacks `?? 20 → ?? 10` in the five composables listed.

- [ ] **Step 3: Verify**

Run: `pnpm test && pnpm typecheck && pnpm lint`
Expected: PASS/clean. Manually confirm each list screen still paginates (page buttons appear when total > 10).

- [ ] **Step 4: Commit**

```bash
git add frontend/app/components/ResourceTable.vue frontend/app/pages/ frontend/app/composables/api/ frontend/test/
git commit -m "fix(ux): standardize table page size to 10"
```

---

## Task 11: Full verification + docs

**Files:**
- Modify: `docs/PROGRESS.md`; Obsidian vault (`Proyek/Status & Roadmap.md`, `Modul/Peta Modul.md`, a `Catatan/2026-07-14-*.md` session note).

- [ ] **Step 1: Backend gates**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./... && go test -tags=integration ./...`
Run: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: all green.

- [ ] **Step 2: Frontend gates**

Run (from `frontend/`): `pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green.

- [ ] **Step 3: Browser verification (perf + single call)**

Bring up the stack; open the depreciation page in the Browser pane. In the network panel confirm `GET /depreciation/schedule` is called **once** on load and once per filter/period/basis change (not twice), returns `total/limit/offset`, and responds well under the old ~2 s. Confirm KPI cards no longer overflow (compact + tooltip) and the schedule table paginates at 10. Screenshot for the user.

- [ ] **Step 4: e2e**

Run depreciation e2e (needs backend stack + seeded admin): `pnpm test:e2e` (or the depreciation spec). Add a pagination assertion if missing. Expected: green.

- [ ] **Step 5: Docs**

Tick the relevant `docs/PROGRESS.md` items with PR note; record the deliberate deviation (schedule now applies `a.deleted_at IS NULL` uniformly, so entries for soft-deleted assets no longer appear). Update the Obsidian vault status/module notes + a dated session note.

- [ ] **Step 6: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(depreciation): record schedule perf/UX batch + deleted-asset tightening"
```

---

## Self-review

- **Spec coverage:** #1 → Tasks 6–9 (+ Impair menu in Task 5). #2 → Tasks 1–3 (backend single-source + SQL) + Task 5 (drop KPI call). #3 → Tasks 4–5. #4 → Tasks 1–3, 5 (server pagination). #5 → Task 10 (+ depreciation via Task 5). Deviation + docs → Task 11. All covered.
- **Type consistency:** `Schedule(..., limit, offset int32)` and `ScheduleResult.Total int64` used identically in Tasks 2–3; `scheduleToMap(r, limit, offset)` matches its Task-3 caller; `ScheduleResponse.total`/`ScheduleQuery.limit|offset` consistent across Tasks 5. `RowActionsMenu`/`buildActionGroups`/`RowAction` names consistent across Tasks 6–9.
- **Placeholder scan:** no TBD/TODO; each code step carries real code. SQL parameter names (`basis`, `period`, `all_scope`, `office_ids`, `is_commercial`, `search`, `category_id`, `office_id`, `lim`, `off`) map to the Task-2 `*Params` field names sqlc generates (`Basis`, `Period`, `AllScope`, `OfficeIds`, `IsCommercial`, `Search`, `CategoryID`, `OfficeID`, `Lim`, `Off`).
- **Risk note:** the LATERAL accumulated-sum benefits from an index on `depreciation.depreciation_entries (asset_id, basis, period)`; the table's `(asset_id, basis, period)` uniqueness (one entry per asset/basis/period) already provides it — verify during Task 2; if absent, add a migration before claiming the perf win.
