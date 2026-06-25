# Dashboard Screen — Design Spec

**Date:** 2026-06-25
**Phase:** Frontend feature screens (mock-first)
**Mockup (source of truth):** `docs/design/Dashboard.dc.html`
**Route:** `/` (replaces the current placeholder `frontend/app/pages/index.vue`)

## 1. Goal

Replace the placeholder dashboard (four all-zero `StatCard`s) with the full Dashboard from
`docs/design/Dashboard.dc.html`, reproduced 1:1: a page header with scope/period controls, a
6-tile KPI row, a charts row (status donut + two bar lists), and a panels row (maintenance-due +
approval queue) — each with its loading skeleton. Data is **mock-first** behind the same interface a
real `$fetch('/dashboard/summary')` will use later.

The app shell (`layouts/default.vue` = `AppSidebar` + `AppTopbar` + `UMain`) is already built and is
what the mockup's outer chrome (sidebar, topbar search/lang/theme/notifications/user) represents. This
work builds **only the `<main>` content** of the dashboard page. The sidebar already wires Dashboard
(`to: '/'`), so no nav change is needed.

## 2. Scope

### In scope
- Rewrite `pages/index.vue` as the dashboard orchestrator (header + three content rows + loading state).
- Five new presentational components under `app/components/dashboard/`.
- Mock data module + `useDashboard` composable + pure derivation helpers.
- Status donut via **Unovis** (`@unovis/vue`); the two bar lists stay pure CSS.
- Full i18n (`id` + `en`) for all static chrome.
- Comprehensive Vitest coverage (unit + Nuxt-runtime component tests).

### Out of scope (later phases / other specs)
- Real backend `/dashboard/summary` endpoint (backend reporting module not built).
- The destinations behind Export and "Lihat semua" (assets/maintenance/approval screens). These render
  as in the mockup but trigger a "belum tersedia" toast (see §6).
- Any change to the app shell, topbar, or sidebar.

## 3. Component architecture

New components live in `app/components/dashboard/` and are auto-imported with a `Dashboard` prefix
(e.g. `dashboard/KpiCard.vue` → `<DashboardKpiCard>`). Each is **presentational** (props in, events out);
all data fetching and derivation happen in the page + composable + helpers.

| Component | Responsibility | Key props / events |
|---|---|---|
| `DashboardKpiCard` | One KPI tile: label, value, icon w/ colored bg, trend (icon+text+color). | `label, value, icon, iconBg, iconColor, trendIcon, trendText, trendColor` |
| `DashboardDonut` | "Aset per Status": Unovis donut ring + central total, plus a 5-row legend (color dot, label, count, %). | `total, totalLabel, segments: StatusSegment[]` |
| `DashboardBarList` | Reusable labeled horizontal bars; used for "per Kategori" (primary) and "per Lokasi" (info). | `title, items: BarItem[], color: 'primary' \| 'info'` |
| `DashboardMaintenancePanel` | "Maintenance Jatuh Tempo" list: icon, asset, task, due-pill (urgent=warning/normal=neutral) + header "Lihat semua". | `items: MaintenanceItem[]`; emits `see-all` |
| `DashboardApprovalPanel` | "Pengajuan Menunggu Approval": rows with approve/reject buttons, count badge, "all handled" empty state, header "Lihat semua". | `items: ApprovalItem[]`; emits `approve(id)`, `reject(id)`, `see-all` |

**Loading skeletons.** The mockup defines exact shimmer skeletons for the KPI row, charts row, and
panels row. Each content row renders its skeleton variant while `loading` is true and the real content
when `loaded`. Skeletons are implemented as small sibling blocks in the page (mirroring the mockup's
`loaded`/`loading` split) reusing the existing shimmer style; the existing `CardSkeleton` is reused
where it fits. The page swaps row-by-row, not all-or-nothing, exactly as the mockup does.

The page (`pages/index.vue`) owns: the bespoke header (H1 + scope name w/ building icon + info pill +
period select + scope select + reload button + export button — matching the mockup precisely, *not*
the generic `PageHeader`, whose subtitle slot can't express the icon+pill), the `loading` state, the
fetch call, and wiring section components to derived data.

## 4. Data layer (mock-first)

### `app/mock/dashboard.ts`
Ports the mockup's `DATA` (3 scopes: `jaksel`, `kanwil`, `pusat`) and `STAT_LABELS`/`STAT_COLORS`.

```ts
export type Scope = 'jaksel' | 'kanwil' | 'pusat'
export interface Localized { id: string; en: string }      // for dynamic record text the mockup localizes

export interface DashboardData {
  scope: Scope
  name: Localized                 // office/scope display name
  total: number
  perolehan: string               // pre-formatted money, e.g. "Rp 3,82 M"
  buku: string
  overdue: number
  due: number
  biaya: string
  status: number[]                // 5 counts: tersedia, dipinjam, maintenance, dilepas, hilang
  kategori: [string, number][]    // label, count
  lokasi: [string, number][]
  maint: MaintenanceSeed[]
  appr: ApprovalSeed[]
}
export interface MaintenanceSeed { asset: string; task: Localized; icon: string; urg: 0 | 1; due: Localized }
export interface ApprovalSeed { id: string; title: Localized; meta: Localized; icon: string; tone: 'info' | 'primary' | 'neutral' }
```

- **Static UI labels** (KPI labels, status labels, chart/panel titles, trend phrases, scope-note, export,
  period options, empty-state copy) are **not** in the fixture — they are i18n keys (§5).
- **Dynamic record text** the mockup localizes (`maint.task`, `maint.due`, `appr.title`, `appr.meta`,
  scope `name`) is stored as `{id, en}` and picked by current locale in the composable — this is
  mock-only data a real API would return already-resolved.
- Money values stay pre-formatted strings (as in the mockup); counts are numbers.
- `tone` replaces the mockup's raw `iconBg`/`iconFg` CSS vars so the component maps tone → token classes.

Approval "handled" state (approve/reject removing a row) is **page-local** UI state, not stored in the
mock (matches the mockup, which keeps a `handled` array in component state). The mock is read-only here.

### `app/composables/api/useDashboard.ts`
The single seam a real implementation swaps behind:

```ts
export function useDashboard() {
  async function summary(scope: Scope, _period: string): Promise<DashboardData> {
    await fakeLatency(700)                 // mirrors the mockup's 700ms load
    return dashboardData[scope] ?? dashboardData.jaksel   // unknown scope → jaksel fallback
  }
  return { summary }
}
```
`period` is accepted but cosmetic (the mockup shows the same data for every period); it only triggers a
reload. Later, `summary` becomes `$fetch('/dashboard/summary', { query: { scope, period } })`.

### `app/utils/dashboard.ts` (pure, unit-tested)
Extracted so the math is testable without mounting:
- `buildDonut(status: number[])` → `{ total, segments: StatusSegment[] }` where each segment has
  `{ key, count, pct }` (`pct` rounded to integer %). Used to feed both Unovis and the legend.
- `barWidths(items: [string, number][])` → `BarItem[]` with `w` = `round(count / max * 100)` (%),
  guarding `max === 0`.
- `formatCount(n)` → `n.toLocaleString('id-ID')` (matches the mockup's `fmt`).

Status segment colors map to our semantic tokens (success / info / warning / dimmed / error), defined as
a single palette constant reused by `buildDonut` consumers and the donut's Unovis `color` accessor.

## 5. i18n

New `dashboard.*` keys in `i18n/locales/{id,en}.json`. Inventory (id → en):

- `dashboard.title` ("Dashboard"), `dashboard.scopeNote` ("Angka mengikuti data scope Anda" / "Figures
  follow your data scope"), `dashboard.export` ("Ekspor" / "Export"), `dashboard.reload`, `dashboard.totalLabel`.
- `dashboard.period.*`: `last30`, `thisMonth`, `thisQuarter`, `ytd`.
- `dashboard.kpi.*`: `total`, `acquisition`, `bookValue`, `overdue`, `maintenanceDue`, `maintenanceCost`.
- `dashboard.kpiTrend.*`: `growing`, `acqUp` ("+1,2%"), `depreciation` ("−6,4% depresiasi"),
  `needsAction`, `within7Days`, `costUp` ("+8,3%"). (Trend phrases are chrome, not data.)
- `dashboard.status.*`: `available`, `inUse`, `maintenance`, `disposed`, `lost`.
- `dashboard.chart.*`: `statusTitle`, `categoryTitle`, `locationTitle`.
- `dashboard.panel.*`: `maintenanceTitle`, `approvalTitle`, `seeAll`, `approvedToast`, `rejectedToast`,
  `allHandledTitle`, `allHandledSub`.
- `dashboard.comingSoon` ("Fitur ini belum tersedia." — may reuse existing `auth.featureComingSoon`;
  decide during implementation, prefer a shared key).

All user-facing strings resolve via `$t`/`useI18n`; nothing hardcoded.

## 6. Interactions

- **Scope select / Period select / Reload button** → call `summary(scope, period)` again; the row
  skeletons show during the ~700ms latency, then content re-renders. Switching scope also clears the
  page-local `handled` set (fresh approval list), matching the mockup.
- **Approve / Reject** (per approval row) → optimistically add the row id to the page-local `handled`
  set so it disappears; the count badge decrements; when the list empties, the "all handled" empty
  state shows. A success toast fires (`dashboard.panel.approvedToast` / `rejectedToast`).
- **Export button** and **"Lihat semua"** → targets not built yet → fire a neutral "belum tersedia"
  toast (`dashboard.comingSoon`). Rendered exactly as the mockup; no invented destination. *(Decision:
  toast over disabling, per user — keeps the mockup's visual intent intact.)*

## 7. Charts — Unovis

Dependencies added: `@unovis/vue` and `@unovis/ts`.

- **Donut** (`DashboardDonut`): `VisSingleContainer` + `VisDonut`. Data = the 5 status segments;
  `value` = count; `color` accessor returns the segment's token color (passed as CSS color strings /
  `var(--ui-*)` so light/dark theming follows the tokens automatically); `arcWidth` and `cornerRadius`
  tuned to match the mockup's ring (≈128px ring, thick arc, small gaps via `padAngle`); `centralLabel`
  = formatted total, `centralSubLabel` = `dashboard.totalLabel`. The 5-row **legend** beside the ring is
  our own markup (color dot + label + count + %), not Unovis, matching the mockup layout.
- **Bar lists** (`DashboardBarList`): pure CSS — a labeled track with an inner bar at `w%` and the
  mockup's `growBar` grow animation. No Unovis (a lib adds nothing for simple progress bars).
- Exact Unovis arc dimensions/CSS-var names are finalized during implementation against the running
  screen; the donut is verified side-by-side with the mockup in light **and** dark mode.

## 8. Testing

Per project conventions: pure logic gets node-env unit tests; components get `mountSuspended` runtime
tests (`// @vitest-environment nuxt`); assert real rendered text / resolved i18n / emitted events — no
hollow assertions. Be expansive: cover empty/loading/error and edge cases.

### Unit (node env)
- `utils/dashboard.ts`:
  - `buildDonut`: segment counts/pcts sum correctly; pct rounding; `total` = sum; all-zero status → no
    division-by-zero, pcts = 0.
  - `barWidths`: width = count/max·100; the max item = 100%; empty array and `max === 0` guarded.
  - `formatCount`: thousands grouping in `id-ID`.
- `useDashboard.summary`: returns the right dataset per scope; unknown scope → `jaksel`; resolves after
  latency.

### Component (nuxt env, `mountSuspended`)
- `pages/index.vue`: shows skeletons first, then loaded content after latency resolves; renders 6
  `DashboardKpiCard`s with the default-scope values; donut central total present; both bar lists render;
  both panels render.
- `DashboardKpiCard`: renders label/value/trend; applies trend color class.
- `DashboardDonut`: central total + `totalLabel`; legend has exactly 5 rows with correct counts/pcts.
  *(jsdom lacks `SVGElement.getBBox`; add a stub in the Nuxt test setup if Unovis needs it, and assert
  our legend/central markup rather than Unovis SVG internals.)*
- `DashboardBarList`: one row per item; bar width style reflects `w`; renders for both color variants.
- `DashboardMaintenancePanel`: one row per item; urgent vs normal due-pill tone; empty list renders no
  rows; `see-all` emits.
- `DashboardApprovalPanel`: approve emits `approve(id)` and (when the parent removes it) the row
  disappears and count drops; rejecting the last item shows the "all handled" empty state; `see-all` emits.
- Page interactions: switching scope re-fetches and changes totals + resets handled approvals; reload
  re-triggers loading; export and see-all fire the coming-soon toast; locale switch changes resolved
  labels (assert text in `id` then `en`).
- Edge cases: a scope/fixture with an empty maintenance list; a scope where all approvals are handled
  (empty state); unknown-scope fallback.

## 9. Files touched

**New**
- `app/components/dashboard/KpiCard.vue`
- `app/components/dashboard/Donut.vue`
- `app/components/dashboard/BarList.vue`
- `app/components/dashboard/MaintenancePanel.vue`
- `app/components/dashboard/ApprovalPanel.vue`
- `app/mock/dashboard.ts`
- `app/composables/api/useDashboard.ts`
- `app/utils/dashboard.ts`
- `test/...` unit + component specs (mirroring existing `test/` layout)

**Modified**
- `app/pages/index.vue` (full rewrite)
- `app/mock/index.ts` (re-export `./dashboard`)
- `i18n/locales/id.json`, `i18n/locales/en.json` (add `dashboard.*`)
- `package.json` (add `@unovis/vue`, `@unovis/ts`)
- `docs/PROGRESS.md` (mark Dashboard built)

## 10. Verification (Definition of Done)

- `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` all green.
- Side-by-side 1:1 comparison of the built `/` against `docs/design/Dashboard.dc.html` in light **and**
  dark mode — layout, spacing, hierarchy, and every state (loading, loaded, approval empty) match.
- Test suite re-checked for completeness across every state/branch/edge case before claiming done.
