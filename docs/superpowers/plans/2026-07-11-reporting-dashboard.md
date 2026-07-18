# Reporting & Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the read-only `internal/report` backend module (dashboard aggregates + 7 report types + PDF/xlsx exports, all office-scoped) and wire the mock-backed Dashboard (`/`) and Laporan (`/reports`) pages to it, deleting `mock/dashboard.ts` + `mock/reports.ts`.

**Architecture:** Direct OLTP aggregate queries (sqlc, standard `all_scope OR office_id = ANY(office_ids)` scope clause) — no new tables, no materialized views (CQRS level 2 stays deferred per PROGRESS.md). Redis caches ONLY `GET /dashboard/summary` (TTL 90s, self-healing, no invalidation logic); reports and every export are always computed fresh. Exports reuse the excelize/gofpdf patterns from `internal/depreciation/export.go` + `internal/stockopname/report.go`. Frontend keeps existing page view-model structure; the two composables are rewritten onto `useApiClient()`.

**Tech Stack:** Go 1.25 + Gin, pgx/v5, sqlc, golang-migrate, excelize/v2, go-pdf/fpdf, testify + testcontainers (`-tags=integration`); Nuxt 4 SPA + Nuxt UI v4 (UCalendar range picker — net-new), Vitest + @nuxt/test-utils, Playwright.

**Spec:** `docs/superpowers/specs/2026-07-11-reporting-dashboard-design.md`

## Global Constraints

- Go module `github.com/ragbuaj/inventra`; never hand-edit `db/sqlc` — edit `db/queries/*.sql` / migrations then `cd backend && sqlc generate`.
- Scope module string is **`"report"`** (`data_scope_policies.module`), resolved via `common.ScopedDeps.CallerOfficeScope(c, "report")`. Every aggregate query takes `all_scope bool` + `office_ids uuid[]`. `office_id` query-param filter must pass `common.InScope(all, ids, officeID)` → 403 otherwise.
- Permissions already seeded in `000005`: `report.view` (all 5 roles), `report.export` (all except Staf). **No new permission keys.**
- Money: `numeric` → Go `string` (sqlc override); money aggregates use `COALESCE(SUM(col), 0)::text`; never float for money. Percentage trends may use float64 (display-only).
- Valuation rule: `excluded_from_valuation = true` assets are **excluded from every money total** but **included in unit counts** (FR-2.10/FR-7.6).
- Every query filters `deleted_at IS NULL` on every table touched.
- Reports/exports: NEVER cached. Dashboard summary: Redis TTL **90s**, key prefix `report:dash:`, `cacheGetJSON`/`cacheSetJSON` pattern mirrored locally (Redis never source of truth).
- Frontend: i18n mandatory (`i18n/locales/{id,en}.json`); semantic tokens only; `U*` components; ESLint `commaDangle: 'never'` + 1tbs; API via `useApiClient()` (`request`/`requestBlob`).
- Conventional Commits, lowercase scope `feat(report): …` / `feat(dashboard): …`. **No Claude/AI attribution in commits.**
- Branch: `feat/reporting-dashboard` (already created; spec committed there).
- Backend period contract: `period=last30|this_month|this_quarter|ytd` **or** `date_from=YYYY-MM-DD&date_to=YYYY-MM-DD` (both required together, `from ≤ to`); giving both `period` and dates is a 400.
- Report `:type` whitelist: `assets|depreciation|utilization|maintenance|transfers|disposals|opname`.
- Approved mockup deviations (a)–(h) are in spec bagian 6. Two plan-time refinements to record in PROGRESS.md as deviations **(i)** and **(j)** (flag to the user at final review): **(i)** the status donut renders **all 7 real enum statuses** (`in_transfer`, `retired` appended after the mockup's 5) — hiding nonzero statuses would make the donut lie; **(j)** the "Maintenance Jatuh Tempo" KPI uses a fixed **`next_due_date ≤ today+7d`** window (matches the mockup's own "dalam 7 hari" trend text), independent of the period filter — a past-looking period makes no sense for future due dates.

---

## File Structure

**Backend — create:**
- `backend/db/migrations/000029_report_scope_seed.up.sql` / `.down.sql` — `report` data-scope rows only.
- `backend/db/queries/report.sql` — all aggregate queries.
- `backend/internal/report/dto.go` — period resolver, request parsing helpers, response DTO structs, sentinel errors.
- `backend/internal/report/service.go` — aggregate assembly + dashboard cache.
- `backend/internal/report/export.go` — pure xlsx/pdf builders (reports + dashboard + GL recap).
- `backend/internal/report/handler.go`, `backend/internal/report/routes.go`.
- `backend/internal/report/dto_test.go`, `backend/internal/report/export_test.go` — unit.
- `backend/internal/report/report_integration_test.go` — integration (`//go:build integration`).

**Backend — modify:**
- `backend/internal/authzadmin/catalog.go` — add `"report"` to `ScopeModules()`.
- `backend/internal/server/router.go` — construct + register the module.
- `backend/api/openapi.yaml` — Report tag, 4 paths, schemas.

**Frontend — create:**
- `frontend/app/constants/reportMeta.ts` — report keys/icons, period presets, `formatMoneyShort`, `formatTrendPct`, `periodToQuery`.
- `frontend/app/components/PeriodFilter.vue` — preset select + "Rentang kustom…" UCalendar range popover (shared by both pages).
- `frontend/app/components/dashboard/RejectModal.vue` — rejection-note modal.
- `frontend/test/unit/report-meta.spec.ts`, `frontend/test/nuxt/period-filter.spec.ts`, `frontend/test/nuxt/use-dashboard.spec.ts`, `frontend/test/nuxt/use-reports.spec.ts`.
- `frontend/e2e/reports.spec.ts`.

**Frontend — modify:**
- `frontend/app/composables/api/useDashboard.ts`, `useReports.ts` — full rewrite onto `useApiClient`.
- `frontend/app/pages/index.vue`, `frontend/app/pages/reports.vue` — rewire.
- `frontend/app/components/dashboard/MaintenancePanel.vue`, `ApprovalPanel.vue` — item types move to the new composable DTOs.
- `frontend/app/utils/dashboard.ts` — `STATUS_KEYS`/`STATUS_COLORS` extended to 7 statuses.
- `frontend/app/utils/nav.ts` — reports item gains `permission: 'report.view'`.
- `frontend/i18n/locales/id.json`, `en.json` — new keys.
- `frontend/test/nuxt/dashboard-page.spec.ts`, `reports.spec.ts` — rewritten against mocked composables.
- `frontend/e2e/dashboard.spec.ts` — extended.
- `frontend/test/unit/dashboard-utils.spec.ts` — 7-status update.

**Delete:** `frontend/app/mock/dashboard.ts`, `frontend/app/mock/reports.ts`, `frontend/test/unit/dashboard-mock.spec.ts`, `frontend/test/unit/reports-mock.spec.ts`.

---

## Task 1: Migration `000029_report_scope_seed` + scope-module catalog

**Files:**
- Create: `backend/db/migrations/000029_report_scope_seed.up.sql`, `.down.sql`
- Modify: `backend/internal/authzadmin/catalog.go` (ScopeModules), `backend/internal/authzadmin/catalog_test.go` (if it asserts counts)

**Interfaces:**
- Produces: `data_scope_policies` rows for module `report`; `"report"` listed in `ScopeModules()` so the Data Scope settings screen can configure it.

- [ ] **Step 1: Write the up migration**

`backend/db/migrations/000029_report_scope_seed.up.sql`:
```sql
-- Reporting & Dashboard module: data-scope seed only.
-- Permissions report.view / report.export were already seeded in 000005
-- (report.view: all roles; report.export: all except Staf).

-- Data-scope for the 'report' module (mirror 'maintenance', 000027).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'report', (CASE
    WHEN r.name = 'Superadmin'                    THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit') THEN 'office_subtree'
    WHEN r.name = 'Manager'                       THEN 'office'
    ELSE 'own'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
```

`.down.sql`:
```sql
DELETE FROM identity.data_scope_policies WHERE module = 'report';
```

> Note the per-spec role mapping (PRD bagian 2.2): Kepala Kanwil/Unit see their subtree, Manager their office, Staf `own` (a Staf's `own` resolves to their office in `CallerOfficeScope`, which for aggregate dashboards means "their office's numbers" — matching "🔵 (miliknya)" pragmatically since assets are office-owned, not user-owned).

- [ ] **Step 2: Add `"report"` to `ScopeModules()`** in `backend/internal/authzadmin/catalog.go` (alphabetical/consistent position with existing entries like `"maintenance"`, `"stockopname"`). If `catalog_test.go` asserts the module list/length, update it.

- [ ] **Step 3: Apply + verify**

Run (infra stack up): `cd backend && migrate -path db/migrations -database "$DATABASE_URL" up`
Then `go build ./... && go test ./internal/authzadmin/`
Expected: migration applies; tests pass.

- [ ] **Step 4: Commit** — `feat(report): seed report data-scope module + catalog entry`

---

## Task 2: `internal/report/dto.go` — period resolver, DTOs, sentinel errors (+ unit tests)

**Files:**
- Create: `backend/internal/report/dto.go`, `backend/internal/report/dto_test.go`

**Interfaces:**
- Produces (used by every later task):
  - `type DateRange struct { From, To time.Time }` (inclusive dates, normalized to midnight UTC)
  - `func ResolvePeriod(preset, fromStr, toStr string, now time.Time) (cur, prev DateRange, err error)`
  - `func ParseReportType(raw string) (string, error)` — whitelist `assets|depreciation|utilization|maintenance|transfers|disposals|opname`
  - `func parseExportFormat(raw string) (string, error)` — `xlsx|pdf`
  - Sentinel errors: `ErrInvalidPeriod`, `ErrInvalidReportType`, `ErrInvalidExportFormat`, `ErrInvalidVariant`, `ErrOfficeOutOfScope`
  - Response structs (JSON tags exactly as below) — `DashboardSummary`, `DashboardKpi`, `Trends`, `StatusCount`, `NamedCount`, `MaintenanceDueItem`, `ReportResult`, `ReportKpi`, `ChartBar`, `GlRecapResult`, `GlRow`

- [ ] **Step 1: Write failing unit tests** (`dto_test.go`, plain `package report` internal test):

```go
package report

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestResolvePeriodPresets(t *testing.T) {
	now := date("2026-07-11")

	cur, prev, err := ResolvePeriod("last30", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-06-12"), cur.From)
	assert.Equal(t, date("2026-07-11"), cur.To)
	assert.Equal(t, date("2026-05-13"), prev.From) // same 30-day length, ends day before cur.From
	assert.Equal(t, date("2026-06-11"), prev.To)

	cur, _, err = ResolvePeriod("this_month", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-07-01"), cur.From)
	assert.Equal(t, date("2026-07-11"), cur.To)

	cur, _, err = ResolvePeriod("this_quarter", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-07-01"), cur.From) // Q3 starts July

	cur, _, err = ResolvePeriod("ytd", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-01-01"), cur.From)
}

func TestResolvePeriodCustom(t *testing.T) {
	now := date("2026-07-11")
	cur, prev, err := ResolvePeriod("", "2026-03-01", "2026-03-31", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-03-01"), cur.From)
	assert.Equal(t, date("2026-03-31"), cur.To)
	assert.Equal(t, date("2026-01-29"), prev.From) // 31 days ending 2026-02-28
	assert.Equal(t, date("2026-02-28"), prev.To)
}

func TestResolvePeriodErrors(t *testing.T) {
	now := date("2026-07-11")
	for _, tc := range []struct{ preset, from, to string }{
		{"bogus", "", ""},              // unknown preset
		{"", "", ""},                   // nothing given
		{"", "2026-01-01", ""},         // half a custom range
		{"", "2026-02-01", "2026-01-01"}, // from > to
		{"", "01-02-2026", "2026-03-01"}, // bad format
		{"last30", "2026-01-01", "2026-02-01"}, // both preset and custom
	} {
		_, _, err := ResolvePeriod(tc.preset, tc.from, tc.to, now)
		assert.ErrorIs(t, err, ErrInvalidPeriod, "preset=%q from=%q to=%q", tc.preset, tc.from, tc.to)
	}
}

func TestParseReportType(t *testing.T) {
	for _, ok := range []string{"assets", "depreciation", "utilization", "maintenance", "transfers", "disposals", "opname"} {
		got, err := ParseReportType(ok)
		require.NoError(t, err)
		assert.Equal(t, ok, got)
	}
	_, err := ParseReportType("aset; DROP TABLE")
	assert.ErrorIs(t, err, ErrInvalidReportType)
}
```

- [ ] **Step 2: Run to verify failure** — `cd backend && go test ./internal/report/` → FAIL (package doesn't exist).

- [ ] **Step 3: Implement `dto.go`**

```go
// Package report serves the dashboard aggregates and the report builder
// (7 report types) read-only, over the OLTP tables, office-scoped.
package report

import (
	"errors"
	"time"
)

var (
	ErrInvalidPeriod       = errors.New("report: invalid period")
	ErrInvalidReportType   = errors.New("report: invalid report type")
	ErrInvalidExportFormat = errors.New("report: invalid export format")
	ErrInvalidVariant      = errors.New("report: invalid export variant")
	ErrOfficeOutOfScope    = errors.New("report: office outside caller scope")
)

// scopeModule is the data_scope_policies module (seeded in 000029).
const scopeModule = "report"

// DateRange is an inclusive [From, To] date window (midnight-normalized).
type DateRange struct{ From, To time.Time }

func (r DateRange) Days() int { return int(r.To.Sub(r.From).Hours()/24) + 1 }

// ResolvePeriod turns either a preset or a custom from/to pair into the
// current window plus the equal-length window immediately preceding it
// (used for trend comparison). Supplying both or neither is an error.
func ResolvePeriod(preset, fromStr, toStr string, now time.Time) (DateRange, DateRange, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var cur DateRange
	custom := fromStr != "" || toStr != ""
	switch {
	case preset != "" && custom:
		return DateRange{}, DateRange{}, ErrInvalidPeriod
	case custom:
		if fromStr == "" || toStr == "" {
			return DateRange{}, DateRange{}, ErrInvalidPeriod
		}
		from, err1 := time.Parse("2006-01-02", fromStr)
		to, err2 := time.Parse("2006-01-02", toStr)
		if err1 != nil || err2 != nil || from.After(to) {
			return DateRange{}, DateRange{}, ErrInvalidPeriod
		}
		cur = DateRange{From: from, To: to}
	case preset == "last30":
		cur = DateRange{From: today.AddDate(0, 0, -29), To: today}
	case preset == "this_month":
		cur = DateRange{From: time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC), To: today}
	case preset == "this_quarter":
		qm := time.Month((int(today.Month())-1)/3*3 + 1)
		cur = DateRange{From: time.Date(today.Year(), qm, 1, 0, 0, 0, 0, time.UTC), To: today}
	case preset == "ytd":
		cur = DateRange{From: time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC), To: today}
	default:
		return DateRange{}, DateRange{}, ErrInvalidPeriod
	}
	days := cur.Days()
	prevTo := cur.From.AddDate(0, 0, -1)
	prev := DateRange{From: prevTo.AddDate(0, 0, -(days - 1)), To: prevTo}
	return cur, prev, nil
}

var reportTypes = map[string]bool{
	"assets": true, "depreciation": true, "utilization": true,
	"maintenance": true, "transfers": true, "disposals": true, "opname": true,
}

// ParseReportType validates :type against the whitelist (never used raw in SQL).
func ParseReportType(raw string) (string, error) {
	if reportTypes[raw] {
		return raw, nil
	}
	return "", ErrInvalidReportType
}

func parseExportFormat(raw string) (string, error) {
	switch raw {
	case "xlsx", "pdf":
		return raw, nil
	default:
		return "", ErrInvalidExportFormat
	}
}

// ---- Dashboard response ----

type Trends struct {
	AcquisitionPct     *float64 `json:"acquisition_pct"`
	BookValuePct       *float64 `json:"book_value_pct"`
	MaintenanceCostPct *float64 `json:"maintenance_cost_pct"`
}

type DashboardKpi struct {
	TotalAssets      int64  `json:"total_assets"`
	AcquisitionValue string `json:"acquisition_value"`
	BookValue        string `json:"book_value"`
	OverdueAssets    int64  `json:"overdue_assets"`
	MaintenanceDue   int64  `json:"maintenance_due"`
	MaintenanceCost  string `json:"maintenance_cost"`
	Trends           Trends `json:"trends"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type NamedCount struct {
	Name  *string `json:"name"` // nil = "no room" bucket; frontend localizes
	Count int64   `json:"count"`
}

type MaintenanceDueItem struct {
	ID           string  `json:"id"`
	AssetName    string  `json:"asset_name"`
	AssetTag     string  `json:"asset_tag"`
	CategoryName *string `json:"category_name"`
	NextDueDate  string  `json:"next_due_date"` // YYYY-MM-DD
}

type DashboardSummary struct {
	OfficeName         *string              `json:"office_name"`
	Kpi                DashboardKpi         `json:"kpi"`
	ByStatus           []StatusCount        `json:"by_status"`
	ByCategory         []NamedCount         `json:"by_category"`
	LocationKind       string               `json:"location_kind"` // "office" | "room"
	ByLocation         []NamedCount         `json:"by_location"`
	MaintenanceDueList []MaintenanceDueItem `json:"maintenance_due_list"`
	ExcludedCount      int64                `json:"excluded_count"`
}

// ---- Report responses (generic envelope, per-type rows) ----

type ReportKpi struct {
	Key   string `json:"key"`   // stable per-type key, e.g. "total_assets"
	Value string `json:"value"` // pre-stringified (count or decimal money string)
}

type ChartBar struct {
	Label string `json:"label"`
	Value string `json:"value"` // decimal string (money) or plain number string
}

// ReportResult is the JSON body of GET /reports/:type. Rows is a slice of
// per-type row structs (each defined in service.go next to its query);
// Totals mirrors the tfoot TOTAL row keyed by column.
type ReportResult struct {
	Type      string            `json:"type"`
	Kpis      []ReportKpi       `json:"kpis"`
	Chart     []ChartBar        `json:"chart"`
	Rows      any               `json:"rows"`
	Totals    map[string]string `json:"totals"`
	RowCount  int64             `json:"row_count"`
	Truncated bool              `json:"truncated"`
}

// ---- Disposal GL recap ----

type GlRow struct {
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
}

type GlRecapResult struct {
	Rows        []GlRow `json:"rows"`
	TotalDebit  string  `json:"total_debit"`
	TotalCredit string  `json:"total_credit"`
	Balanced    bool    `json:"balanced"`
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/report/ -v` → all PASS. `go vet ./...`.

- [ ] **Step 5: Commit** — `feat(report): period resolver, report-type whitelist, response DTOs`

---

## Task 3: Dashboard aggregate queries + `Service.DashboardSummary` (no cache yet)

**Files:**
- Create: `backend/db/queries/report.sql` (dashboard section), `backend/internal/report/service.go`, `backend/internal/report/report_integration_test.go`
- Regenerate: `cd backend && sqlc generate`

**Interfaces:**
- Consumes: Task 2 DTOs, `ResolvePeriod`.
- Produces: `func NewService(q *sqlc.Queries, rdb *redis.Client) *Service` (rdb may be nil in unit contexts); `func (s *Service) DashboardSummary(ctx, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur, prev DateRange) (DashboardSummary, error)`. Service field `now func() time.Time` (defaults `time.Now`, overridable in tests).

- [ ] **Step 1: Add dashboard queries to `backend/db/queries/report.sql`**

```sql
-- Reporting & Dashboard module — read-only aggregates.
-- Every query: deleted_at IS NULL + the standard scope clause
--   (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
-- plus an optional narg(office_filter) drill-down (validated against scope in the handler).
-- Money aggregates: COALESCE(SUM(x), 0)::text — never float.
-- Valuation rule: excluded_from_valuation is excluded from money sums, included in counts.

-- name: DashboardAssetKpis :one
SELECT
  count(*)::bigint AS total_assets,
  COALESCE(SUM(purchase_cost) FILTER (WHERE NOT excluded_from_valuation), 0)::text AS acquisition_value,
  COALESCE(SUM(book_value) FILTER (WHERE NOT excluded_from_valuation), 0)::text AS book_value,
  count(*) FILTER (WHERE excluded_from_valuation)::bigint AS excluded_count,
  count(*) FILTER (WHERE status = 'available')::bigint AS st_available,
  count(*) FILTER (WHERE status = 'assigned')::bigint AS st_assigned,
  count(*) FILTER (WHERE status = 'under_maintenance')::bigint AS st_under_maintenance,
  count(*) FILTER (WHERE status = 'in_transfer')::bigint AS st_in_transfer,
  count(*) FILTER (WHERE status = 'retired')::bigint AS st_retired,
  count(*) FILTER (WHERE status = 'disposed')::bigint AS st_disposed,
  count(*) FILTER (WHERE status = 'lost')::bigint AS st_lost,
  COALESCE(SUM(purchase_cost) FILTER (
    WHERE NOT excluded_from_valuation
      AND purchase_date BETWEEN sqlc.arg(period_from)::date AND sqlc.arg(period_to)::date), 0)::text AS acquired_in_period
FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR office_id = sqlc.narg(office_filter));

-- name: DashboardAssetsByCategory :many
SELECT c.name, count(*)::bigint AS cnt
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
GROUP BY c.name
ORDER BY cnt DESC, c.name
LIMIT 5;

-- name: DashboardAssetsByOffice :many
SELECT o.name, count(*)::bigint AS cnt
FROM asset.assets a
JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
GROUP BY o.name
ORDER BY cnt DESC, o.name
LIMIT 5;

-- name: DashboardAssetsByRoom :many
SELECT r.name, count(*)::bigint AS cnt
FROM asset.assets a
LEFT JOIN masterdata.rooms r ON r.id = a.room_id AND r.deleted_at IS NULL
WHERE a.deleted_at IS NULL AND a.office_id = sqlc.arg(office_id)
GROUP BY r.name
ORDER BY cnt DESC, r.name NULLS LAST
LIMIT 5;

-- name: DashboardOverdueCount :one
SELECT count(*)::bigint
FROM assignment.assignments ag
JOIN asset.assets a ON a.id = ag.asset_id AND a.deleted_at IS NULL
WHERE ag.deleted_at IS NULL AND ag.status = 'active'
  AND ag.due_date IS NOT NULL AND ag.due_date < sqlc.arg(today)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));

-- name: DashboardMaintenanceDueCount :one
SELECT count(*)::bigint
FROM maintenance.maintenance_schedules s
JOIN asset.assets a ON a.id = s.asset_id AND a.deleted_at IS NULL
WHERE s.deleted_at IS NULL AND s.is_active AND s.next_due_date <= sqlc.arg(window_end)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));

-- name: DashboardMaintenanceDueList :many
SELECT s.id, a.name AS asset_name, a.asset_tag, mc.name AS category_name, s.next_due_date
FROM maintenance.maintenance_schedules s
JOIN asset.assets a ON a.id = s.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = s.maintenance_category_id AND mc.deleted_at IS NULL
WHERE s.deleted_at IS NULL AND s.is_active AND s.next_due_date <= sqlc.arg(window_end)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
ORDER BY s.next_due_date ASC
LIMIT 3;

-- name: DashboardMaintenanceCost :one
SELECT
  COALESCE(SUM(r.cost) FILTER (WHERE r.completed_date BETWEEN sqlc.arg(cur_from)::date AND sqlc.arg(cur_to)::date), 0)::text AS current_cost,
  COALESCE(SUM(r.cost) FILTER (WHERE r.completed_date BETWEEN sqlc.arg(prev_from)::date AND sqlc.arg(prev_to)::date), 0)::text AS previous_cost
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));

-- name: DashboardDepreciationInPeriod :one
SELECT COALESCE(SUM(e.depreciation_amount), 0)::text
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.basis = 'commercial'
  AND e.period BETWEEN sqlc.arg(period_from)::date AND sqlc.arg(period_to)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));
```

> **Check the real table name for maintenance categories** before finalizing `DashboardMaintenanceDueList` — grep `maintenance_categories` in `db/migrations/` (it is a masterdata reference table; confirm schema + soft-delete column). If the reference table has no `deleted_at`, drop that predicate.

- [ ] **Step 2: `sqlc generate`** — `cd backend && sqlc generate && go build ./...` → compiles; new methods exist in `db/sqlc`.

- [ ] **Step 3: Write `service.go` with `DashboardSummary`**

```go
package report

import (
	"context"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

const maintenanceDueWindowDays = 7 // mockup: "dalam 7 hari"

// Service assembles the dashboard and report aggregates. Read-only.
type Service struct {
	q   *sqlc.Queries
	rdb *redis.Client
	now func() time.Time
}

func NewService(q *sqlc.Queries, rdb *redis.Client) *Service {
	return &Service{q: q, rdb: rdb, now: time.Now}
}

// pctChange returns (cur-prev)/prev*100 as a display percentage, nil when
// the comparison base is zero/unparseable. Floats are fine here: this is a
// trend indicator, never an accounting figure.
func pctChange(cur, prev string) *float64 {
	c, okc := new(big.Rat).SetString(cur)
	p, okp := new(big.Rat).SetString(prev)
	if !okc || !okp || p.Sign() == 0 {
		return nil
	}
	diff := new(big.Rat).Sub(c, p)
	diff.Quo(diff, p)
	f, _ := diff.Float64()
	v := f * 100
	// one decimal place
	v = float64(int(v*10+copysignHalf(v))) / 10
	return &v
}

func copysignHalf(v float64) float64 {
	if v < 0 {
		return -0.5
	}
	return 0.5
}

// ratioPct returns part/whole*100 (nil when whole is zero) — used for the
// acquisition trend (additions vs prior base) and depreciation trend.
func ratioPct(part, whole string) *float64 { /* same big.Rat shape as pctChange, part/whole*100 */ }

func (s *Service) DashboardSummary(ctx context.Context, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur, prev DateRange) (DashboardSummary, error) {
	today := s.now().UTC().Truncate(24 * time.Hour)
	dueEnd := today.AddDate(0, 0, maintenanceDueWindowDays)

	k, err := s.q.DashboardAssetKpis(ctx, sqlc.DashboardAssetKpisParams{
		PeriodFrom: cur.From, PeriodTo: cur.To,
		AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}
	// ... byCategory, overdue, maintenance due count+list, maintenance cost,
	// depreciation-in-period: one query call each, same params shape.
	// Location granularity: officeFilter set, or a non-all scope with exactly
	// one office → DashboardAssetsByRoom(office); else DashboardAssetsByOffice.
	// Trends:
	//   acquisition: base = acquisition_value - acquired_in_period; ratioPct(acquired_in_period, base)
	//   book value:  ratioPct(depreciation_in_period, book_value + depreciation_in_period), negated
	//   maintenance: pctChange(current_cost, previous_cost)
	// Assemble DashboardSummary literal, mapping the st_* columns into
	// ByStatus entries ordered: available, assigned, under_maintenance,
	// in_transfer, retired, disposed, lost.
	// office_name: when officeFilter != nil, resolve via s.q.GetOffice (existing
	// masterdata query — check name in db/sqlc/querier.go, e.g. GetOfficeByID).
	...
}
```

Write the elided assembly in full (it is mechanical: 7 query calls + struct literal). Keep it a single method; no goroutine fan-out (pgx pool + ms-level queries — measure before parallelizing).

- [ ] **Step 4: Integration tests** (`report_integration_test.go`, `//go:build integration`, external package `report_test`, copy the depreciation harness shape: `testsupport.NewPostgres(t)` + `sqlc.New(pool)` + direct service calls; seed via existing `testsupport` seed helpers / direct inserts):

Cover, with a deterministic seed (2 offices A and B, office B outside scope; 3 assets in A — one `excluded_from_valuation`, one `assigned` with an overdue active assignment; 1 asset in B; a maintenance schedule in A due in 3 days; a completed maintenance record in A inside the period and one in the previous window; commercial depreciation entries in the period):

```go
func TestDashboardSummaryScopeAndExclusion(t *testing.T) {
	// scope = office A only (all=false, ids=[A])
	// total_assets == 3 (B invisible), excluded_count == 1
	// acquisition_value/book_value exclude the excluded asset's money but count it
	// overdue_assets == 1; maintenance_due == 1; maintenance_due_list has the A schedule
	// maintenance_cost == cost of the in-period completed record only
	// trends.maintenance_cost_pct non-nil (prev window has a record)
	// by_status counts match seeded statuses incl. the 7-key order
	// location_kind == "room" (single office scope)
}

func TestDashboardSummaryAllScope(t *testing.T) {
	// all=true: total 4, location_kind == "office", by_location contains A and B
}

func TestDashboardSummaryOfficeFilter(t *testing.T) {
	// all=true + officeFilter=A behaves like the A-scoped call; office_name resolved
}

func TestDashboardSummaryEmptyDB(t *testing.T) {
	// zero rows: money "0", counts 0, trends all nil — no division-by-zero panic
}
```

- [ ] **Step 5: Run** — `go test ./internal/report/ -tags=integration -v` → PASS. Also `go vet ./...`.

- [ ] **Step 6: Commit** — `feat(report): dashboard aggregate queries + DashboardSummary service`

---

## Task 4: Dashboard Redis cache (TTL 90s)

**Files:**
- Modify: `backend/internal/report/service.go`
- Test: extend `backend/internal/report/report_integration_test.go`

**Interfaces:**
- Produces: `func (s *Service) CachedDashboardSummary(ctx, roleID uuid.UUID, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur, prev DateRange) (DashboardSummary, error)` — get-or-compute wrapper; direct `DashboardSummary` stays for tests/exports (exports must be fresh).

- [ ] **Step 1: Failing integration test**

```go
func TestCachedDashboardSummary(t *testing.T) {
	// rdb := testsupport.NewRedis(t); seed one asset; call CachedDashboardSummary → total 1
	// insert a second asset directly via SQL
	// call again with identical args → STILL total 1 (served from cache)
	// call DashboardSummary (uncached) → total 2 (source of truth unaffected)
	// different officeFilter arg → different key → total 2
	// TTL check: rdb.TTL(key) in (0, 90s]
}
```

- [ ] **Step 2: Implement** — mirror the ~15-line `cacheGetJSON`/`cacheSetJSON` pattern from `internal/authz/cache.go` locally in `service.go` (they are package-private to authz; do NOT export them from authz):

```go
const dashboardCacheTTL = 90 * time.Second

func dashboardCacheKey(roleID uuid.UUID, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur DateRange) string {
	h := sha256.New()
	sorted := append([]uuid.UUID(nil), ids...)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	for _, id := range sorted {
		h.Write(id[:])
	}
	filter := "-"
	if officeFilter != nil {
		filter = officeFilter.String()
	}
	return fmt.Sprintf("report:dash:%s:%t:%x:%s:%s:%s",
		roleID, all, h.Sum(nil)[:8], filter,
		cur.From.Format("2006-01-02"), cur.To.Format("2006-01-02"))
}

func (s *Service) CachedDashboardSummary(ctx context.Context, roleID uuid.UUID, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur, prev DateRange) (DashboardSummary, error) {
	key := dashboardCacheKey(roleID, all, ids, officeFilter, cur)
	var cached DashboardSummary
	if s.rdb != nil && cacheGetJSON(ctx, s.rdb, key, &cached) {
		return cached, nil
	}
	out, err := s.DashboardSummary(ctx, all, ids, officeFilter, cur, prev)
	if err != nil {
		return DashboardSummary{}, err
	}
	if s.rdb != nil {
		cacheSetJSON(ctx, s.rdb, key, out, dashboardCacheTTL)
	}
	return out, nil
}
```

- [ ] **Step 3: Run** — integration tests PASS. **Step 4: Commit** — `feat(report): 90s redis cache for dashboard summary`

---

## Task 5: Report queries + service — assets, depreciation, utilization, maintenance

**Files:**
- Modify: `backend/db/queries/report.sql`, `backend/internal/report/service.go`; regenerate sqlc.
- Test: extend `report_integration_test.go`.

**Interfaces:**
- Produces: `func (s *Service) Run(ctx context.Context, typ string, p ReportParams) (ReportResult, error)` where

```go
// ReportParams carries the common + per-type filters, already validated.
type ReportParams struct {
	All          bool
	OfficeIDs    []uuid.UUID
	OfficeFilter *uuid.UUID
	CategoryID   *uuid.UUID
	Status       *string // assets only, one of shared.asset_status values
	Basis        string  // depreciation only: "commercial"|"fiscal" (default commercial)
	Cur, Prev    DateRange
	RowLimit     int64 // 1000 for JSON, effectively-unbounded for export
}
```
and per-type row structs (JSON-tagged, exported for export.go): `AssetRow{Tag, Name, Category, Status, PurchaseCost, AccumDeprec, BookValue string}`, `DeprRow{Period, Opening, Amount, Closing string}`, `UtilRow{Name, Tag, Category string; DaysLoaned, LoanCount int64; UtilizationPct float64}`, `MaintRow{AssetName, Category, Type string; Actions int64; TotalCost string}`.

- [ ] **Step 1: Queries** (append to `report.sql`; all four share the scope + `narg(office_filter)` + `narg(category_id)` clause shape from Task 3):

```sql
-- name: ReportAssetRows :many
SELECT a.asset_tag, a.name, c.name AS category_name, a.status,
  COALESCE(a.purchase_cost, '0')::text AS purchase_cost,
  a.accumulated_depreciation::text AS accum_deprec,
  COALESCE(a.book_value, '0')::text AS book_value
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR a.status = sqlc.narg(status))
ORDER BY a.asset_tag
LIMIT sqlc.arg(lim);

-- name: ReportAssetTotals :one
SELECT count(*)::bigint AS row_count,
  COALESCE(SUM(a.purchase_cost) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_cost,
  COALESCE(SUM(a.accumulated_depreciation) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_accum,
  COALESCE(SUM(a.book_value) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_book
FROM asset.assets a
WHERE a.deleted_at IS NULL
  AND (…same filter block…);

-- name: ReportAssetChart :many  -- book value per category
SELECT c.name, COALESCE(SUM(a.book_value) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_book
FROM asset.assets a JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE a.deleted_at IS NULL AND (…same filter block…)
GROUP BY c.name ORDER BY SUM(a.book_value) DESC NULLS LAST LIMIT 8;

-- name: ReportDepreciationRows :many
SELECT to_char(e.period, 'YYYY-MM') AS period,
  COALESCE(SUM(e.opening_value), 0)::text AS opening,
  COALESCE(SUM(e.depreciation_amount), 0)::text AS amount,
  COALESCE(SUM(e.closing_value), 0)::text AS closing
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.basis = sqlc.arg(basis)
  AND e.period BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (…scope + office_filter + category_id block on a…)
GROUP BY e.period ORDER BY e.period;

-- name: ReportDepreciationKpis :one
SELECT
  COALESCE(SUM(e.depreciation_amount) FILTER (WHERE e.period BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date), 0)::text AS period_expense,
  COALESCE(SUM(e.depreciation_amount) FILTER (WHERE e.period <= sqlc.arg(date_to)::date), 0)::text AS accumulated
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.basis = sqlc.arg(basis)
  AND (…scope block…);

-- name: ReportDepreciationRemaining :one  -- sum of each asset's last closing ≤ date_to
SELECT COALESCE(SUM(last.closing_value), 0)::text
FROM (
  SELECT DISTINCT ON (e.asset_id) e.closing_value
  FROM depreciation.depreciation_entries e
  JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
  WHERE e.deleted_at IS NULL AND e.basis = sqlc.arg(basis) AND e.period <= sqlc.arg(date_to)::date
    AND (…scope block…)
  ORDER BY e.asset_id, e.period DESC
) AS last;

-- name: ReportUtilizationRows :many
SELECT a.name, a.asset_tag, c.name AS category_name,
  COALESCE(SUM(GREATEST(0,
    LEAST(COALESCE(ag.checkin_date::date, sqlc.arg(date_to)::date), sqlc.arg(date_to)::date)
    - GREATEST(ag.checkout_date::date, sqlc.arg(date_from)::date) + 1)), 0)::bigint AS days_loaned,
  count(ag.id)::bigint AS loan_count
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
LEFT JOIN assignment.assignments ag ON ag.asset_id = a.id AND ag.deleted_at IS NULL
  AND ag.checkout_date::date <= sqlc.arg(date_to)::date
  AND COALESCE(ag.checkin_date::date, sqlc.arg(date_to)::date) >= sqlc.arg(date_from)::date
WHERE a.deleted_at IS NULL AND (…scope + office_filter + category_id block…)
GROUP BY a.id, a.name, a.asset_tag, c.name
HAVING count(ag.id) > 0
ORDER BY days_loaned DESC
LIMIT sqlc.arg(lim);

-- name: ReportUtilizationKpis :one
SELECT count(*)::bigint AS active_loans
FROM assignment.assignments ag
JOIN asset.assets a ON a.id = ag.asset_id AND a.deleted_at IS NULL
WHERE ag.deleted_at IS NULL AND ag.status = 'active'
  AND (…scope + office_filter block…);

-- name: ReportMaintenanceRows :many
SELECT a.name AS asset_name, c.name AS category_name, r.type,
  count(*)::bigint AS actions, COALESCE(SUM(r.cost), 0)::text AS total_cost
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND r.completed_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (…scope + office_filter + category_id block…)
GROUP BY a.id, a.name, c.name, r.type
ORDER BY SUM(r.cost) DESC NULLS LAST
LIMIT sqlc.arg(lim);

-- name: ReportMaintenanceKpis :one
SELECT COALESCE(SUM(r.cost), 0)::text AS total,
  COALESCE(SUM(r.cost) FILTER (WHERE r.type = 'preventive'), 0)::text AS preventive,
  COALESCE(SUM(r.cost) FILTER (WHERE r.type = 'corrective'), 0)::text AS corrective
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND r.completed_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (…scope + office_filter + category_id block…);

-- name: ReportMaintenanceChart :many  -- cost per category
SELECT c.name, COALESCE(SUM(r.cost), 0)::text AS total
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND r.completed_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (…scope + office_filter + category_id block…)
GROUP BY c.name ORDER BY SUM(r.cost) DESC NULLS LAST LIMIT 8;
```

Write every `(…block…)` out in full — sqlc has no macros. `sqlc generate && go build ./...`.

- [ ] **Step 2: Service `Run` dispatch + the four builders** in `service.go`:

```go
const jsonRowLimit = 1000

func (s *Service) Run(ctx context.Context, typ string, p ReportParams) (ReportResult, error) {
	switch typ {
	case "assets":
		return s.runAssets(ctx, p)
	case "depreciation":
		return s.runDepreciation(ctx, p)
	case "utilization":
		return s.runUtilization(ctx, p)
	case "maintenance":
		return s.runMaintenance(ctx, p)
	case "transfers":
		return s.runTransfers(ctx, p) // Task 6
	case "disposals":
		return s.runDisposals(ctx, p) // Task 6
	case "opname":
		return s.runOpname(ctx, p) // Task 6
	default:
		return ReportResult{}, ErrInvalidReportType
	}
}
```

Each `runX` calls its rows+kpis+chart queries and assembles `ReportResult{Type, Kpis: []ReportKpi{{key,value}…}, Chart, Rows, Totals, RowCount, Truncated: rowCount > int64(len(rows))}`. KPI keys (stable contract for frontend + export):
- assets: `total_assets`, `total_acquisition`, `total_book`
- depreciation: `period_expense`, `accumulated`, `remaining_book`
- utilization: `avg_utilization` (float pct as string, 1 decimal — computed in Go from total days / (rows × period days)), `active_loans`, `total_days`
- maintenance: `total_cost`, `preventive`, `corrective`
Utilization row `UtilizationPct = days_loaned / cur.Days() * 100` computed in Go (float, 1 decimal).

- [ ] **Step 3: Integration tests** — extend the Task 3 seed; per type assert: rows content + ordering, KPI values, chart, `excluded_from_valuation` money exclusion (assets report), category/status filters, scope invisibility of office B, custom period boundaries (a record exactly on `date_from`/`date_to` is included; one the day before/after is not), utilization clipping (an assignment spanning the whole period counts `cur.Days()`, an open one clips at `date_to`).

- [ ] **Step 4: Run + commit** — `feat(report): asset/depreciation/utilization/maintenance report queries + service`

---

## Task 6: Report queries + service — transfers, disposals (+ GL recap), opname

**Files:** same as Task 5.

**Interfaces:**
- Produces: `runTransfers`, `runDisposals`, `runOpname`; `func (s *Service) DisposalGlRecap(ctx context.Context, p ReportParams) (GlRecapResult, error)`; row structs `TransferRow{AssetName, AssetTag, FromOffice, ToOffice, Status, ShippedDate, ReceivedDate, BastNo string}` (dates/bast may be empty), `DisposalRow{AssetName, AssetTag, Method, Date, BookValue, Proceeds, GainLoss string}`, `OpnameRow{SessionID, Name, OfficeName, Period, Status string; TotalItems, Variance int64}`.

- [ ] **Step 1: Queries**

```sql
-- name: ReportTransferRows :many
SELECT a.name AS asset_name, a.asset_tag, fo.name AS from_office, tofc.name AS to_office,
  t.status, t.shipped_date, t.received_date, t.bast_no
FROM transfer.asset_transfers t
JOIN asset.assets a ON a.id = t.asset_id AND a.deleted_at IS NULL
JOIN masterdata.offices fo ON fo.id = t.from_office_id
JOIN masterdata.offices tofc ON tofc.id = t.to_office_id
WHERE t.deleted_at IS NULL
  AND t.created_at::date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(all_scope)::boolean OR t.from_office_id = ANY(sqlc.arg(office_ids)::uuid[]) OR t.to_office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR t.from_office_id = sqlc.narg(office_filter) OR t.to_office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
ORDER BY t.created_at DESC
LIMIT sqlc.arg(lim);

-- name: ReportTransferKpis :one   -- + chart per destination office
SELECT count(*)::bigint AS total,
  count(*) FILTER (WHERE t.status = 'in_transit')::bigint AS in_transit,
  count(*) FILTER (WHERE t.status = 'received')::bigint AS received
FROM transfer.asset_transfers t
JOIN asset.assets a ON a.id = t.asset_id AND a.deleted_at IS NULL
WHERE t.deleted_at IS NULL AND (…same filter block…);

-- name: ReportTransferChart :many
SELECT tofc.name, count(*)::bigint AS cnt
FROM transfer.asset_transfers t
JOIN asset.assets a ON a.id = t.asset_id AND a.deleted_at IS NULL
JOIN masterdata.offices tofc ON tofc.id = t.to_office_id
WHERE t.deleted_at IS NULL AND (…same filter block…)
GROUP BY tofc.name ORDER BY cnt DESC LIMIT 8;

-- name: ReportDisposalRows :many
SELECT a.name AS asset_name, a.asset_tag, d.method, d.disposal_date,
  COALESCE(d.book_value_at_disposal, '0')::text AS book_value,
  COALESCE(d.proceeds, '0')::text AS proceeds,
  COALESCE(d.gain_loss, '0')::text AS gain_loss
FROM disposal.disposals d
JOIN asset.assets a ON a.id = d.asset_id AND a.deleted_at IS NULL
WHERE d.deleted_at IS NULL
  AND d.disposal_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
ORDER BY d.disposal_date DESC
LIMIT sqlc.arg(lim);

-- name: ReportDisposalKpis :one
SELECT count(*)::bigint AS total,
  COALESCE(SUM(d.proceeds), 0)::text AS total_proceeds,
  COALESCE(SUM(d.gain_loss), 0)::text AS total_gain_loss,
  COALESCE(SUM(d.gain_loss) FILTER (WHERE d.gain_loss > 0), 0)::text AS total_gain,
  COALESCE(SUM(ABS(d.gain_loss)) FILTER (WHERE d.gain_loss < 0), 0)::text AS total_loss,
  COALESCE(SUM(d.book_value_at_disposal), 0)::text AS total_book_value
FROM disposal.disposals d
JOIN asset.assets a ON a.id = d.asset_id AND a.deleted_at IS NULL
WHERE d.deleted_at IS NULL AND (…same filter block…);

-- name: ReportDisposalChart :many  -- gain/loss per method
SELECT d.method, COALESCE(SUM(d.gain_loss), 0)::text AS total
FROM disposal.disposals d
JOIN asset.assets a ON a.id = d.asset_id AND a.deleted_at IS NULL
WHERE d.deleted_at IS NULL AND (…same filter block…)
GROUP BY d.method ORDER BY d.method;

-- name: ReportOpnameSessions :many
SELECT s.id, COALESCE(s.name, '') AS name, o.name AS office_name, s.period, s.status,
  count(i.id)::bigint AS total_items,
  count(i.id) FILTER (WHERE i.result IN ('not_found', 'damaged', 'misplaced'))::bigint AS variance
FROM stockopname.stock_opname_sessions s
JOIN masterdata.offices o ON o.id = s.office_id
LEFT JOIN stockopname.stock_opname_items i ON i.session_id = s.id AND i.deleted_at IS NULL
WHERE s.deleted_at IS NULL AND s.status = 'closed'
  AND s.period BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR s.office_id = sqlc.narg(office_filter))
GROUP BY s.id, s.name, o.name, s.period, s.status
ORDER BY s.period DESC
LIMIT sqlc.arg(lim);
```

> Verify `stock_opname_sessions.name` nullability against migration 000015 before the COALESCE; drop it if NOT NULL. Verify `disposals.gain_loss` type allows `> 0` comparison directly (it's `numeric` — fine).

- [ ] **Step 2: Service builders.** KPI keys: transfers `total`/`in_transit`/`received`; disposals `total_disposals`/`total_proceeds`/`total_gain_loss`; opname `sessions`/`total_items`/`total_variance` (summed in Go over rows — closed sessions are few). `runDisposals` chart labels are the raw method enum values (`sale|auction|donation|write_off`) — frontend localizes.

`DisposalGlRecap` (book-value-basis disposal journal, balanced by construction since `gain_loss = proceeds − book_value`):

```go
// DisposalGlRecap builds the journal-ready recap for disposals in the period:
//   Dr Kas/Bank                  = Σ proceeds
//   Dr Rugi Pelepasan Aset       = Σ |gain_loss| where gain_loss < 0
//   Cr Nilai Buku Aset Dilepas   = Σ book_value_at_disposal
//   Cr Laba Pelepasan Aset       = Σ gain_loss where gain_loss > 0
// Account codes come from app_settings keys report.gl.{cash,loss,asset,gain}_account
// (empty string when unset — configurable mapping is a recorded follow-up).
func (s *Service) DisposalGlRecap(ctx context.Context, p ReportParams) (GlRecapResult, error)
```

Rows with a zero amount are omitted. `Balanced` computed by big.Rat comparison of totals.

- [ ] **Step 3: Integration tests** — seed 1 gain disposal (proceeds > book value) + 1 loss disposal + 1 transfer chain + 1 closed opname session with variance items; assert rows/KPIs/GL recap balance (`total_debit == total_credit`, `Balanced == true`), transfers visible when only the *destination* office is in scope, opname sessions of out-of-scope offices invisible, non-closed sessions excluded.

- [ ] **Step 4: Run + commit** — `feat(report): transfer/disposal/opname reports + disposal GL recap`

---

## Task 7: `export.go` — xlsx/pdf builders (+ unit tests)

**Files:**
- Create: `backend/internal/report/export.go`, `backend/internal/report/export_test.go`

**Interfaces:**
- Produces:
  - `func BuildReportXLSX(res ReportResult, meta ExportMeta) ([]byte, error)`
  - `func (s *Service) BuildReportPDF(ctx context.Context, res ReportResult, meta ExportMeta) ([]byte, error)` (needs `GetAppSetting("label.company_name")`)
  - `func BuildDashboardXLSX(sum DashboardSummary, meta ExportMeta) ([]byte, error)` / `func (s *Service) BuildDashboardPDF(ctx, sum DashboardSummary, meta ExportMeta) ([]byte, error)`
  - `func BuildGlRecapXLSX(r GlRecapResult, meta ExportMeta) ([]byte, error)` / `func (s *Service) BuildGlRecapPDF(ctx, r GlRecapResult, meta ExportMeta) ([]byte, error)`
  - `type ExportMeta struct { Title, PeriodLabel, OfficeLabel, PrintedBy string; PrintedAt time.Time }`
  - `func exportFilename(kind string, cur DateRange) string` → e.g. `laporan-assets-2026-06-12--2026-07-11`

- [ ] **Step 1: Failing unit tests** (`export_test.go`, internal package; construct small `ReportResult` fixtures per type):

```go
func TestBuildReportXLSXAssets(t *testing.T) {
	res := ReportResult{Type: "assets", Rows: []AssetRow{{Tag: "AST-1", Name: "Laptop", Category: "Elektronik", Status: "available", PurchaseCost: "15000000.00", AccumDeprec: "5000000.00", BookValue: "10000000.00"}}, Totals: map[string]string{"purchase_cost": "15000000.00", "accum_deprec": "5000000.00", "book_value": "10000000.00"}}
	body, err := BuildReportXLSX(res, ExportMeta{Title: "Daftar Aset & Nilai Buku"})
	require.NoError(t, err)
	f, err := excelize.OpenReader(bytes.NewReader(body)) // re-open to verify content
	require.NoError(t, err)
	sheet := f.GetSheetName(0)
	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "AST-1", v)
	// header row matches column labels; last row is TOTAL
}
// analogous minimal tests: one per report type's column mapping,
// dashboard xlsx (two sheets: Ringkasan + per-status/kategori),
// GL recap xlsx (Kode Akun|Nama Akun|Debit|Kredit + TOTAL),
// PDF: len(body) > 0 && bytes.HasPrefix(body, []byte("%PDF"))
```

- [ ] **Step 2: Implement** following `depreciation/export.go` verbatim style (excelize `SetCellValue` loops; fpdf A4 with company header via `GetAppSetting("label.company_name")` fallback `"PT Bank Tabungan Negara (Persero) Tbk"`, title, `meta.PeriodLabel · meta.OfficeLabel` subtitle, bordered `CellFormat` table, TOTAL row, italic footer `fmt.Sprintf("Dicetak oleh %s · %s", meta.PrintedBy, meta.PrintedAt.Format("2006-01-02 15:04"))`). One `columnsFor(res)` helper maps each report type → `[]struct{Header string; Width float64; Value func(row) string; Align string}` so xlsx and pdf share the same column definitions (DRY). Type-switch on `res.Rows.(type)` to iterate.

- [ ] **Step 3: Run** — `go test ./internal/report/ -run TestBuild -v` → PASS. **Step 4: Commit** — `feat(report): xlsx/pdf export builders for reports, dashboard, GL recap`

---

## Task 8: Handler + routes + router wiring (+ HTTP integration tests)

**Files:**
- Create: `backend/internal/report/handler.go`, `backend/internal/report/routes.go`
- Modify: `backend/internal/server/router.go`
- Test: extend `report_integration_test.go` (copy the depreciation `httpHarness` shape: fresh `gin.New()` per request, stub auth middleware setting `CtxUserID`/`CtxRoleID`, real `middleware.RequirePermission(permSvc, …)`).

**Interfaces:**
- Produces routes: `GET /dashboard/summary`, `GET /dashboard/export`, `GET /reports/:type`, `GET /reports/:type/export`.

- [ ] **Step 1: `routes.go`**

```go
package report

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the reporting endpoints. Read-only module:
// report.view gates the JSON reads, report.export gates every file download.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireView, requireExport gin.HandlerFunc) {
	d := rg.Group("/dashboard")
	d.GET("/summary", authMW, requireView, h.dashboardSummary)
	d.GET("/export", authMW, requireExport, h.dashboardExport)

	r := rg.Group("/reports")
	r.GET("/:type", authMW, requireView, h.run)
	r.GET("/:type/export", authMW, requireExport, h.runExport)
}
```

- [ ] **Step 2: `handler.go`** — Handler struct `{svc *Service; scoped common.ScopedDeps}`; shared param parsing:

```go
// parseCommon extracts scope + validated filters shared by every endpoint.
// officeFilter outside the caller's scope → ErrOfficeOutOfScope (403).
func (h *Handler) parseCommon(c *gin.Context) (p ReportParams, ok bool) {
	cur, prev, err := ResolvePeriod(c.Query("period"), c.Query("date_from"), c.Query("date_to"), time.Now())
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return p, false }
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil { common.WriteError(c, err); return p, false }
	// narg office_filter / category_id via parseOptionalUUIDQuery-style helpers (copy from depreciation/handler.go)
	if officeFilter != nil && !common.InScope(all, ids, *officeFilter) {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrOfficeOutOfScope.Error()})
		return p, false
	}
	// status: optional, must be a member of the shared.asset_status set (400 otherwise)
	// basis: optional, "commercial"(default)|"fiscal" (400 otherwise)
	p = ReportParams{All: all, OfficeIDs: ids, OfficeFilter: officeFilter, CategoryID: categoryID,
		Status: status, Basis: basis, Cur: cur, Prev: prev, RowLimit: jsonRowLimit}
	return p, true
}
```

Handlers:
- `dashboardSummary` — parseCommon → roleID from `c.GetString(middleware.CtxRoleID)` → `svc.CachedDashboardSummary` → 200 JSON.
- `dashboardExport` — parseCommon + `parseExportFormat` → **uncached** `svc.DashboardSummary` → `BuildDashboardXLSX`/`BuildDashboardPDF` → attachment headers (nosniff + Content-Disposition, exact `journalExport` pattern). `PrintedBy`: resolve caller name via `h.scoped.Q.GetUserByID`.
- `run` — `ParseReportType(c.Param("type"))` (400 on invalid) + parseCommon → `svc.Run` → 200 JSON.
- `runExport` — type + format + `variant := c.DefaultQuery("variant", "table")`; `variant == "gl_recap"` only valid for `disposals` (else **422** `ErrInvalidVariant`); table variant uses `p.RowLimit = 1_000_000`; dispatch to `BuildReportXLSX/PDF` or `BuildGlRecapXLSX/PDF`.
- `svcError` maps: `ErrOfficeOutOfScope` → 403, `ErrInvalidVariant` → 422, `ErrInvalid*` → 400, default `common.WriteError`.

- [ ] **Step 3: Wire in `backend/internal/server/router.go`** (after the depreciation block, inside the api group):

```go
reportSvc := report.NewService(queries, d.Redis)
reportHandler := report.NewHandler(reportSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc})
report.RegisterRoutes(api, reportHandler,
	requireAuth,
	middleware.RequirePermission(permSvc, "report.view"),
	middleware.RequirePermission(permSvc, "report.export"),
)
```

- [ ] **Step 4: HTTP integration tests** — with the seeded users from `testsupport` (superadmin global; a Manager office-scoped; a Staf):

```
- GET /dashboard/summary as superadmin → 200, kpi.total_assets matches seed
- GET /dashboard/summary?office_id=<office outside scope> as Manager → 403
- GET /reports/assets?period=last30 as Manager → 200, only own-office rows
- GET /reports/nope?period=last30 → 400
- GET /reports/assets?date_from=2026-01-01 (half range) → 400
- GET /reports/assets/export?format=xlsx&period=last30 as Manager → 200,
  Content-Type spreadsheet, Content-Disposition attachment, body opens via excelize
- GET /reports/assets/export?...&format=pdf as Staf → 403 (no report.export)
- GET /reports/assets?period=last30 as Staf → 200 (report.view seeded for Staf)
- GET /reports/assets/export?format=docx → 400
- GET /reports/transfers/export?variant=gl_recap&... → 422
- GET /reports/disposals/export?variant=gl_recap&format=xlsx → 200, balanced recap
- no Authorization header → 401
```

- [ ] **Step 5: Run everything** — `go build ./... && go vet ./... && go test ./... && go test ./internal/report/ -tags=integration`. **Step 6: Commit** — `feat(report): http handlers, routes, router wiring`

---

## Task 9: OpenAPI spec

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1:** Add tag `Report`; schemas `DashboardSummary`, `ReportResult` (rows as free-form array + per-type row schemas referenced in the description), `GlRecapResult`; the 4 paths mirroring the depreciation-journal-export style (shown in that file at `/api/v1/depreciation/journal/export`): shared params `period` (enum last30/this_month/this_quarter/ytd), `date_from`/`date_to` (format date, "both or neither; mutually exclusive with period"), `office_id`, `category_id`; `:type` enum of the 7 values; per-type `status` (asset_status enum) and `basis` (commercial/fiscal); export adds `format` (xlsx/pdf, required) and `variant` (table/gl_recap, disposals-only, 422 note). Document 400/401/403/422 responses; note the 90-second cache on `/dashboard/summary` and the "exports always fresh" guarantee in the descriptions.

- [ ] **Step 2: Lint** — `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` → 0 errors (the pre-existing `AssetCreatePayload` warning persists, unrelated).

- [ ] **Step 3: Commit** — `docs(api): openapi spec for report module`

---

## Task 10: Frontend meta + `PeriodFilter` component

**Files:**
- Create: `frontend/app/constants/reportMeta.ts`, `frontend/app/components/PeriodFilter.vue`
- Test: `frontend/test/unit/report-meta.spec.ts`, `frontend/test/nuxt/period-filter.spec.ts`

**Interfaces:**
- Produces:

```ts
// constants/reportMeta.ts
export type ReportKey = 'assets' | 'depreciation' | 'utilization' | 'maintenance' | 'transfers' | 'disposals' | 'opname'
export const REPORT_KEYS: ReportKey[]  // in that order (mockup 4 first, then the 3 new)
export const REPORT_ICON: Record<ReportKey, string>
// assets: i-lucide-package, depreciation: i-lucide-trending-down, utilization: i-lucide-gauge,
// maintenance: i-lucide-receipt, transfers: i-lucide-arrow-left-right,
// disposals: i-lucide-trash-2, opname: i-lucide-clipboard-check
export type PeriodPreset = 'last30' | 'this_month' | 'this_quarter' | 'ytd'
export interface PeriodValue { preset: PeriodPreset | 'custom'; from?: string; to?: string } // from/to ISO YYYY-MM-DD, set iff custom
export function periodToQuery(p: PeriodValue): Record<string, string>
// preset → { period: p.preset }; custom → { date_from: p.from!, date_to: p.to! }
export function formatMoneyShort(v: string): string
// decimal string → "Rp 3,82 M" (≥1e9), "Rp 42,5 Jt" (≥1e6), else "Rp 950.000"; unparseable → the input
export function formatTrendPct(p: number | null | undefined): string | null
// 8.3 → "+8,3%", -6.4 → "−6,4%", null/undefined → null
```

`PeriodFilter.vue`: `defineProps<{ modelValue: PeriodValue }>()`, `defineEmits<{ 'update:modelValue': [PeriodValue] }>()`. A `USelect` with the 4 preset options + `custom` (label `t('common.periodCustom')`); when `custom` is active, a `UPopover` anchored button showing `from – to` opens a **`UCalendar` with `range`** bound to a `CalendarRange` from `@internationalized/date`; converting `CalendarDate ↔ 'YYYY-MM-DD'` via `.toString()` / `parseDate()`. Selecting a complete range emits the new value and closes the popover. `data-testid="period-filter-select"`, `"period-filter-range"`, `"period-filter-calendar"`.

- [ ] **Step 1:** `cd frontend && pnpm ls @internationalized/date` — it ships as a transitive dep of `@nuxt/ui` v4; if not directly resolvable from app code, `pnpm add @internationalized/date`.

- [ ] **Step 2: Failing unit tests** (`report-meta.spec.ts`): table-drive `periodToQuery` (each preset; custom; ensure no stray keys), `formatMoneyShort` boundaries (999999 → full Rp, 1e6, 42.5e6, 1e9, 3.82e9, "abc" → "abc", "0" → "Rp 0"), `formatTrendPct` (positive sign, negative uses −, null). Run → FAIL.

- [ ] **Step 3: Implement `reportMeta.ts`**, run unit tests → PASS.

- [ ] **Step 4: Component test** (`period-filter.spec.ts`, `// @vitest-environment nuxt`, `mountSuspended`): renders 5 options; emits preset change; switching to custom shows the range button; selecting a range via the calendar emits `{preset:'custom', from, to}` (drive `UCalendar` by setting its model directly through the component vm if DOM clicking is brittle — assert the emitted payload). Run → PASS.

- [ ] **Step 5: Lint + commit** — `pnpm lint && pnpm typecheck` → `feat(frontend): report meta constants + PeriodFilter (UCalendar range)`

---

## Task 11: Composables rewrite — `useDashboard` + `useReports`

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useDashboard.ts`, `frontend/app/composables/api/useReports.ts`
- Test: `frontend/test/nuxt/use-dashboard.spec.ts`, `frontend/test/nuxt/use-reports.spec.ts`

**Interfaces:**
- Produces (DTOs exactly mirror backend JSON — Task 2/5/6):

```ts
// useDashboard.ts
export interface DashboardTrends { acquisition_pct: number | null; book_value_pct: number | null; maintenance_cost_pct: number | null }
export interface DashboardKpi { total_assets: number; acquisition_value: string; book_value: string; overdue_assets: number; maintenance_due: number; maintenance_cost: string; trends: DashboardTrends }
export interface StatusCount { status: string; count: number }
export interface NamedCount { name: string | null; count: number }
export interface MaintenanceDueItem { id: string; asset_name: string; asset_tag: string; category_name: string | null; next_due_date: string }
export interface DashboardSummary {
  office_name: string | null
  kpi: DashboardKpi
  by_status: StatusCount[]
  by_category: NamedCount[]
  location_kind: 'office' | 'room'
  by_location: NamedCount[]
  maintenance_due_list: MaintenanceDueItem[]
  excluded_count: number
}
export interface DashboardQuery { officeId?: string; period: PeriodValue }
export function useDashboard(): {
  summary(q: DashboardQuery): Promise<DashboardSummary>       // GET /dashboard/summary
  exportSummary(q: DashboardQuery, format: 'xlsx' | 'pdf'): Promise<Blob>  // GET /dashboard/export
}
```

```ts
// useReports.ts
export interface ReportKpi { key: string; value: string }
export interface ChartBar { label: string; value: string }
export interface AssetReportRow { asset_tag: string; name: string; category_name: string; status: string; purchase_cost: string; accum_deprec: string; book_value: string }
export interface DeprReportRow { period: string; opening: string; amount: string; closing: string }
export interface UtilReportRow { name: string; asset_tag: string; category_name: string; days_loaned: number; loan_count: number; utilization_pct: number }
export interface MaintReportRow { asset_name: string; category_name: string; type: string; actions: number; total_cost: string }
export interface TransferReportRow { asset_name: string; asset_tag: string; from_office: string; to_office: string; status: string; shipped_date: string | null; received_date: string | null; bast_no: string | null }
export interface DisposalReportRow { asset_name: string; asset_tag: string; method: string; disposal_date: string; book_value: string; proceeds: string; gain_loss: string }
export interface OpnameReportRow { session_id: string; name: string; office_name: string; period: string; status: string; total_items: number; variance: number }
export type ReportRow = AssetReportRow | DeprReportRow | UtilReportRow | MaintReportRow | TransferReportRow | DisposalReportRow | OpnameReportRow
export interface ReportResult { type: ReportKey; kpis: ReportKpi[]; chart: ChartBar[]; rows: ReportRow[]; totals: Record<string, string>; row_count: number; truncated: boolean }
export interface ReportFilters { period: PeriodValue; officeId?: string; categoryId?: string; status?: string; basis?: 'commercial' | 'fiscal' }
export function useReports(): {
  run(type: ReportKey, f: ReportFilters): Promise<ReportResult>                       // GET /reports/:type
  exportReport(type: ReportKey, f: ReportFilters, format: 'xlsx' | 'pdf', variant?: 'table' | 'gl_recap'): Promise<Blob>
  opnameBa(sessionId: string, format: 'xlsx' | 'pdf'): Promise<Blob>                  // GET /stock-opname/sessions/:id/report
}
```

Both build `query` via `periodToQuery(f.period)` + conditional `office_id`/`category_id`/`status`/`basis`/`variant` keys, and use `useApiClient()`'s `request`/`requestBlob` (exact `useDepreciation` idiom).

- [ ] **Step 1: Failing composable specs** (`vi.mock('~/composables/useApiClient', …)` pattern from `use-depreciation.spec.ts`): assert exact path + query for each function — preset period, custom period, office/category/status/basis inclusion & omission, gl_recap variant, opname BA path. Run → FAIL (old mock composables).

- [ ] **Step 2: Rewrite both composables.** Delete the `~/mock/*` imports. Run specs → PASS.

> `test/nuxt/dashboard-page.spec.ts` and `test/nuxt/reports.spec.ts` now break (pages still consume the old shapes) — they are rewritten in Tasks 12/13; run only the new specs here (`pnpm vitest run test/nuxt/use-dashboard.spec.ts test/nuxt/use-reports.spec.ts test/unit/report-meta.spec.ts`).

- [ ] **Step 3: Commit** — `feat(frontend): wire useDashboard/useReports composables to /dashboard + /reports`

---

## Task 12: Dashboard page rewiring

**Files:**
- Modify: `frontend/app/pages/index.vue`, `frontend/app/components/dashboard/MaintenancePanel.vue`, `ApprovalPanel.vue`, `frontend/app/utils/dashboard.ts`, `frontend/i18n/locales/id.json`, `en.json`, `frontend/test/unit/dashboard-utils.spec.ts`
- Create: `frontend/app/components/dashboard/RejectModal.vue`
- Delete: `frontend/app/mock/dashboard.ts`, `frontend/test/unit/dashboard-mock.spec.ts`
- Test: rewrite `frontend/test/nuxt/dashboard-page.spec.ts`

**Interfaces:**
- Consumes: Task 11 `useDashboard`, `useApproval` (`inbox`/`approve`/`reject` — existing), `useOffices().list()`, `useCan`, `PeriodFilter`, `formatMoneyShort`/`formatTrendPct`.

- [ ] **Step 1: `utils/dashboard.ts`** — extend to 7 statuses (order matches backend `by_status`):

```ts
export const STATUS_KEYS = ['available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost'] as const
export const STATUS_COLORS = [
  'var(--ui-success)', 'var(--ui-info)', 'var(--ui-warning)', 'var(--ui-secondary)',
  'var(--ui-text-muted)', 'var(--ui-text-dimmed)', 'var(--ui-error)'
] as const
```
`buildDonut` unchanged (input becomes `by_status.map(s => s.count)`). Update `dashboard.status.*` i18n keys to the real enum keys (`available` Tersedia, `assigned` Digunakan, `under_maintenance` Maintenance, `in_transfer` Dalam Mutasi, `retired` Purna Pakai, `disposed` Dilepas, `lost` Hilang) — keep en.json in sync. Update `test/unit/dashboard-utils.spec.ts` for 7 keys.

- [ ] **Step 2: Rewire `index.vue` script:**
  - State: `officeId = ref<string | undefined>()`, `period = ref<PeriodValue>({ preset: 'last30' })`, `data = ref<DashboardSummary | null>()`, `loadError = ref(false)`, `inboxItems = ref<ApprovalRequestRow[]>([])`.
  - `load()`: `Promise.all([summary({ officeId, period }), canDecide ? approvalApi.inbox() : Promise.resolve([])])` in try/catch → `loadError` on failure (retry button per app convention).
  - Office select replaces the scope select: options from `useOffices().list({ limit: 100 })` prepended with `{ value: '', label: t('dashboard.allOfficesInScope') }`; the whole control hidden when `offices.length <= 1`. `data-testid="dashboard-office-select"`.
  - Period control → `<PeriodFilter v-model="period" @update:model-value="load" />`.
  - KPI view-model: values via `formatMoneyShort`; trend rows use `formatTrendPct(data.kpi.trends.…)` when non-null, else the static descriptor keys (`kpiTrend.growing`/`needsAction`/`within7Days`); when a computed trend exists, trend tone: positive acquisition → success, book value → muted, maintenance cost positive → warning.
  - Donut/bars: `by_status` → `buildDonut`, `by_category`/`by_location` → `barWidths(items.map(i => [i.name ?? t('dashboard.noRoom'), i.count]))`; location card title switches on `location_kind` (`chart.locationOffices` / `chart.locationRooms`).
  - Maintenance panel items: `maintenance_due_list` mapped to `{ asset: `${asset_name} · ${asset_tag}`, task: category_name ?? t('dashboard.panel.maintenanceGeneric'), due, urg }` — `urg = 1` when `next_due_date <= today+1d`; `due` text via a small `dueLabel(next_due_date)` helper (Hari ini / Besok / `n hari lagi` / `terlambat n hari` — i18n plural keys). "Lihat semua" → `navigateTo('/maintenance')`.
  - Approval panel: gated `v-if="can('request.decide')"`; items = first 5 inbox rows mapped `{ id, title: t(`approval.type.${row.type}`) + (office_name ? ` — ${office_name}` : ''), meta: `${requested_by_name ?? '—'} · ${requested_by_role ?? '—'}`, icon: TYPE_META[row.type].icon, tone }`; count badge = `inboxItems.length`. `approve(id)` → `approvalApi.approve(id)` → success toast + `load()`. `reject(id)` opens `RejectModal`; its confirm emits `(id, note)` → `approvalApi.reject(id, note)` → toast + `load()`. Errors: `useApiClient` already toasts; just re-enable buttons.
  - Export button → `UDropdownMenu` (PDF / Excel) gated `v-if="can('report.export')"`, calling `doExport(format)` with the exact blob-anchor-download pattern from `depreciation.vue` (`URL.createObjectURL` → temp `<a download>` → revoke), filename `dashboard-${period-label}.${format}`. `data-testid="dashboard-export"`, `"dashboard-export-pdf"`, `"dashboard-export-xlsx"`.
  - Header scope line shows `data.office_name ?? t('dashboard.scopeAll')`; keep the `scopeNote` pill. Show a small muted line `t('dashboard.excludedNote', { n: excluded_count })` under the KPI grid when `excluded_count > 0` (transparency for the valuation rule).

- [ ] **Step 3: `RejectModal.vue`** — `UModal` with required `UTextarea` note (`data-testid="dashboard-reject-note"`), disabled confirm until non-empty, emits `confirm(note: string)` / `cancel`. Reuse the approval screen's copy keys (`approval.decide.rejectNote…`) if present, else add `dashboard.panel.rejectNoteLabel`/`rejectConfirm`.

- [ ] **Step 4: Panels' prop types** — `MaintenancePanel.vue`/`ApprovalPanel.vue` import `MaintenanceItem`/`ApprovalItem` from `useDashboard` today; keep those interface names exported from the rewritten composable (same fields the templates bind: `asset/task/icon/urg/due` and `id/title/meta/icon/tone`) so the panel templates stay untouched — the page does the mapping.

- [ ] **Step 5: Delete `mock/dashboard.ts` + `test/unit/dashboard-mock.spec.ts`;** grep `~/mock/dashboard` → only index.vue references remain removed; `mock/helpers.ts` stays (other mocks may still use it — verify with grep; if nothing imports it, delete it too).

- [ ] **Step 6: i18n additions** (id + en): `dashboard.allOfficesInScope` ("Seluruh kantor dalam scope"), `dashboard.scopeAll` ("Seluruh scope Anda"), `dashboard.noRoom` ("Tanpa ruangan"), `dashboard.excludedNote` ("{n} aset dikecualikan dari total nilai (pengecualian valuasi)"), `dashboard.chart.locationOffices`/`locationRooms`, `dashboard.panel.maintenanceGeneric` ("Maintenance terjadwal"), due-label keys, reject-modal keys, `dashboard.export.pdf`/`xlsx`, `common.periodCustom` ("Rentang kustom…"), status keys from Step 1. Remove now-dead keys `dashboard.kpiTrend.acqUp`/`costUp` only if the static fallback no longer uses them (grep first).

- [ ] **Step 7: Rewrite `test/nuxt/dashboard-page.spec.ts`** — `vi.mock('~/composables/api/useDashboard')` + `vi.mock('~/composables/api/useApproval')` + `vi.mock('~/composables/api/useOffices')` returning controllable fns (follow the maintenance page-spec style: `vi.hoisted` + `mockNuxtImport('useToast', …)` for toast assertions). Auth store seeded with `['*']`. Cases:
  1. loading skeletons while summary pending; 2. KPI values + formatted money (`Rp 3,82 M`) rendered; 3. real trend renders `+8,3%`, null trend renders static descriptor; 4. donut + bars from by_status/by_category; 5. location title switches office/room kinds; 6. maintenance panel rows + urgency badge; 7. approval panel hidden without `request.decide`, shown with items + badge when granted; 8. approve click calls `approve(id)` + reloads; 9. reject opens modal, confirm disabled while empty, confirm calls `reject(id, note)`; 10. export dropdown hidden without `report.export`; with it, clicking PDF calls `exportSummary(..., 'pdf')`; 11. load failure renders error + retry, retry reloads; 12. empty summary (all zeros) renders zero-state donut without NaN; 13. office select hidden with 1 office, shown with 2; 14. excluded_count > 0 renders the note. Run `pnpm test` (full) → green.

- [ ] **Step 8: Commit** — `feat(dashboard): wire dashboard to /dashboard/summary (office filter, period, export, live approvals)`

---

## Task 13: Reports page rewiring (7 cards)

**Files:**
- Modify: `frontend/app/pages/reports.vue`, `frontend/app/utils/nav.ts`, `frontend/test/unit/nav-model.spec.ts` (if it asserts nav items), `frontend/i18n/locales/id.json`, `en.json`
- Delete: `frontend/app/mock/reports.ts`, `frontend/test/unit/reports-mock.spec.ts`
- Test: rewrite `frontend/test/nuxt/reports.spec.ts`

**Interfaces:**
- Consumes: Task 11 `useReports`, `PeriodFilter`, `useCategories().tree()` (existing — for the category filter), `useOffices().list()`, `useCan`, `formatMoneyShort`.

- [ ] **Step 1: Script rewiring:**
  - `definePageMeta({ middleware: 'can', permission: 'report.view' })` (replaces the placeholder).
  - `report = ref<ReportKey>('assets')`; cards from `REPORT_KEYS`/`REPORT_ICON` (reportMeta) + i18n `reports.card.<key>.{label,desc}` — 7 cards, same card markup (grid wraps to a second row).
  - Filters: `period = ref<PeriodValue>({ preset: 'this_quarter' })` via `PeriodFilter`; office (real `useOffices` options + Semua Kantor); category (real `useCategories().tree()` flattened + Semua Kategori); status select only for `assets` (real enum values, labels `t(`dashboard.status.${k}`)`); basis toggle only for `depreciation` (`commercial`/`fiscal`, default commercial). Reset button clears office/category/status/basis.
  - `apply()` → `loading = true; result.value = await api.run(report.value, filters)` in try/catch with `loadError` + retry; `applied = true`.
  - The `view` computed keeps its `{ kpis, chartTitle, chartBars, cols, rows, footer }` contract but now branches on the 7 backend types, reading `result.value` (typed rows) instead of `computeReport`: kpi labels from i18n keyed by the stable KPI `key`s (`reports.kpi.<key>`), money through `formatMoneyShort`, utilization values as `${row.utilization_pct}%`, transfer status via existing `transferMeta` labels, disposal method via `disposalMeta` labels, disposal gain/loss cell tone `error` when negative / `success` when positive, `totals` map → footer cells. When `truncated`, render a muted notice `t('reports.truncated', { n: row_count })`.
  - Export: PDF/Excel buttons gated `<Can permission="report.export">`, `data-testid="reports-export-pdf"/"reports-export-xlsx"`, blob-anchor download (filename from Content-Disposition is browser-handled; anchor `download` attr = `laporan-${report}-${periodLabel}.${format}`). For `disposals`, an extra "Rekap Jurnal GL" `UDropdownMenu` (PDF/Excel) calling `exportReport(type, f, format, 'gl_recap')`, `data-testid="reports-export-gl"`.
  - `opname` rows: last column renders two small download buttons (BA PDF / BA Excel) calling `opnameBa(session_id, format)` — extend the row-rendering `Cell` union with an `actions` cell kind carrying the session id, or special-case the opname table section in the template (pick the smaller diff; the mockup table is hand-rolled `<table>` so a dedicated `<template v-if="report === 'opname'">` block for the action cell is acceptable and keeps `cell()` untouched).
  - `resultMeta` becomes honest: `t('reports.resultMeta', { period: periodLabel, office: officeLabel })` where officeLabel = selected office name or `t('reports.allOffices')` (fixes the hardcoded "Cabang Jakarta Selatan").

- [ ] **Step 2: Nav** — in `utils/nav.ts` add `permission: 'report.view'` to the reports item (staffNav too if present); update `nav-model.spec.ts` accordingly.

- [ ] **Step 3: i18n** — add `reports.card.{assets,depreciation,utilization,maintenance,transfers,disposals,opname}.{label,desc}` (rename old aset/depr/util/biaya keys — grep for stragglers), `reports.kpi.<all stable keys>` (replace the old hardcoded-year `deprCurrent`), `reports.chart.{…7…}`, `reports.col.{fromOffice,toOffice,shipped,received,bast,method,date,proceeds,gainLoss,session,office,totalItems,variance,sessionStatus,downloadBa}`, `reports.filter.basis`, `reports.basis.{commercial,fiscal}`, `reports.exportGl`, `reports.truncated`, `reports.error` (+ retry). Delete `reports.exportSoon`. Mirror in en.json.

- [ ] **Step 4: Delete `mock/reports.ts` + `test/unit/reports-mock.spec.ts`;** grep `~/mock/reports` → zero references. Grep `mock/helpers` — if the dashboard/report mocks were its last consumers, delete it and its spec too.

- [ ] **Step 5: Rewrite `test/nuxt/reports.spec.ts`** — mock `useReports`/`useOffices`/`useCategories`; auth `['*']` (+ a case with only `report.view` to assert export buttons hidden). Cases: 7 cards render + active styling; status filter only on assets; basis only on depreciation; placeholder pre-apply; apply calls `run` with mapped filters (incl. custom period → date_from/date_to); loading state; error + retry; empty rows → empty-state + reset; populated assets table with TOTAL footer + money formatting; disposal gain/loss tones; transfers status labels; opname BA buttons call `opnameBa`; GL recap button only on disposals + calls variant `gl_recap`; truncated notice; export hidden without `report.export`. Run `pnpm test` full → green; `pnpm lint && pnpm typecheck && pnpm build`.

- [ ] **Step 6: Commit** — `feat(reports): wire 7-report builder to /reports (filters, exports, GL recap, opname BA)`

---

## Task 14: E2E (real backend)

**Files:**
- Modify: `frontend/e2e/dashboard.spec.ts`
- Create: `frontend/e2e/reports.spec.ts`

Follow the maintenance.spec.ts conventions: `RUN = Date.now()` unique names, API-context setup via `auth/login`, assert-after-search, `waitForEvent('download')` for exports (depreciation.spec.ts pattern). Stack must be up + seeded admin; `RATELIMIT_ENABLED=false`.

- [ ] **Step 1: `dashboard.spec.ts`** — extend/rewrite:
  1. **KPI + charts populate:** API-setup one office (+floor/room), category, and an approved asset (submit `asset_create` + approve as second SoD user — copy the assets.spec.ts helper) with known `purchase_cost`; login → `/` → assert Total Aset ≥ 1 and Nilai Perolehan renders an `Rp` value; donut legend shows Tersedia count.
  2. **Inline approval:** API-submit a pending request visible to admin's inbox; dashboard panel shows it; click ✓ → toast + row disappears; verify via API the request is `approved`. (Reject path: create a second request, click ✕, modal requires note, fill + confirm → `rejected`.)
  3. **Export:** click Ekspor → PDF → `download.suggestedFilename()` matches `/\.pdf$/`; Excel → `/\.xlsx$/`.
  4. **Period custom range:** switch PeriodFilter to Rentang kustom, pick a range, dashboard reloads without error (assert a KPI still visible).

- [ ] **Step 2: `reports.spec.ts`:**
  1. Setup via API: office/category/asset as above (reuse helper), a completed maintenance record with cost, and (cheap wins) reuse whatever transfers/disposals test data the run creates — otherwise assert those tabs' empty states.
  2. `assets` report: select card → Terapkan → row with the created asset tag appears (search-scoped assertion), TOTAL footer renders; export xlsx + pdf download-events.
  3. `maintenance` report: apply with the seeded record's period → row + `Rp` total.
  4. `depreciation` report: apply → either rows (if entries exist) or empty state — assert no error state; toggle basis fiskal reloads.
  5. Staf permission: create a Staf user via API (or reuse seeded one), `clearCookies`+localStorage relogin (per e2e convention), `/reports` loads (report.view) but export buttons are absent.
  6. `disposals` GL recap button only visible on disposals card.

- [ ] **Step 3: Run** — `pnpm test:e2e -- dashboard.spec.ts reports.spec.ts` (single worker) → green. Known dev-DB fragilities (office-count >100 debris, Data-Scope test cleanup) are documented pre-existing issues — don't chase them if they hit *other* specs.

- [ ] **Step 4: Commit** — `test(e2e): dashboard + reports real-backend flows`

---

## Task 15: Gate sweep, mockup comparison, docs

- [ ] **Step 1: Full gates** — backend: `go build ./... && go vet ./... && go test ./...` + `go test -tags=integration ./...` (all packages — full-integration-gate convention); Spectral; frontend: `pnpm lint && pnpm typecheck && pnpm test && pnpm build`. All green before proceeding.

- [ ] **Step 2: Side-by-side mockup comparison** (mandatory) — open the running app and `docs/design/Dashboard.dc.html` + `docs/design/Laporan.dc.html` (Playwright MCP or browser), light **and** dark: verify 1:1 layout/spacing/states; only the approved deviations (a)–(h) from the spec plus (i)/(j) from Global Constraints may differ. Fix any gap; report the comparison result.

- [ ] **Step 3: `docs/PROGRESS.md`** — tick the Reporting & Dashboard item with a summary note (endpoints, 7 reports, cache policy, deviations (a)–(j), honest limitations: 1000-row JSON cap, GL account codes unconfigured follow-up, historical value snapshots deferred); refresh the "▶ Next session — start here" block.

- [ ] **Step 4: Obsidian vault** — update `Proyek/Status & Roadmap.md` + `Modul/Peta Modul.md`; add a product-decision note (`Keputusan/Produk/`) for the OLTP+cache architecture decision and the period/office-filter behavior; session note in `Catatan/` prefixed `2026-07-11`.

- [ ] **Step 5: Commit** — `docs(progress): reporting & dashboard module landed` — then hand off per superpowers:finishing-a-development-branch (merge/PR decision is the user's).

---

## Self-Review Notes

- **Spec coverage:** bagian Keputusan 1–7 → Tasks 5/6 (7 reports), 2 (period), 12 (office select/export/approval panel), 4 (cache), 3+5 (exclusion rule); bagian Arsitektur → Tasks 3–8; bagian 1 backend table → Tasks 1–9; bagian 2 report table → Tasks 5/6/13; bagian 3 frontend → Tasks 10–13; bagian 4 keamanan → Tasks 1/8 (403 tests), bagian 5 pengujian → every task + 14/15; bagian 6 deviasi → recorded in Global Constraints + Task 15; bagian 7 batasan → Task 15 PROGRESS notes. GL recap endpoint variant (spec bagian 2 jenis 6) → Tasks 6/8/13.
- **Type consistency:** `ReportParams`/`ReportResult`/`GlRecapResult` defined once (Task 2) and consumed in 5–8; frontend DTO names in Task 11 are consumed verbatim in 12/13; `PeriodValue`/`periodToQuery` defined in Task 10, consumed in 11–13. KPI `key` strings listed in Task 5/6 are the same ones i18n'd in Task 13.
- **Two refinements ((i) donut 7 statuses, (j) due-window ≤ today+7) must be surfaced to the user at review** — they are marked in Global Constraints and Task 15.
