# Operasional Cluster — Frontend Design (mock-first)

Date: 2026-06-25
Status: Approved (mockups are the source of truth; built 1:1 per CLAUDE.md)

## Scope

Build the four remaining **Operasional** menu screens on top of the existing Nuxt 4 foundation,
matching their high-fidelity mockups in `docs/design/` exactly:

| Screen | Route | Mockup | Permission gate |
| --- | --- | --- | --- |
| Penugasan (Assignment) | `/assignment` | `Penugasan Aset.dc.html` | `masterdata.office.manage` |
| Maintenance | `/maintenance` | `Maintenance.dc.html` | `masterdata.office.manage` |
| Pengajuan & Approval | `/approval` | `Pengajuan Approval.dc.html` | `masterdata.office.manage` |
| Laporan (Reports) | `/reports` | `Laporan.dc.html` | `masterdata.office.manage` |

These complete the Operasional sidebar group (Dashboard + Aset are already built). The four
`nav.ts` items are currently `disabled: true`; this cluster wires them to the routes above and
extends `BUILT_ROUTES` in `test/unit/nav-model.spec.ts`.

## Architecture (reuses the established mock-first seam)

Each screen follows the same pattern as the Assets cluster:

- **`app/mock/<feature>.ts`** — fixtures ported verbatim from the mockup's seed arrays, plus a
  small mutable store with `reset()` (mirrors `mock/assets.ts`). Pure data + tone/label maps.
- **`app/composables/api/use<Feature>.ts`** — the real-API seam: async functions wrapping the
  store behind `fakeLatency()`, so the page never touches the store directly. Sentinel errors as
  `'<feature>.errX'` i18n keys (mirrors `useAssets`).
- **`app/pages/<route>.vue`** — thin page gated with
  `definePageMeta({ middleware: 'can', permission: '…' })`, composing `U*` components.
- **`app/components/<feature>/…`** — extract any repeated block (status badge, request card,
  timeline, report table) into an auto-imported component.
- **i18n** — every string in `i18n/locales/{id,en}.json` under a new top-level key per feature
  (`assignment`, `maintenance`, `approval`, `reports`). `nav.*` labels already exist.

### Per-screen notes

**Penugasan** — 3 tabs (Check-out / Check-in / Riwayat). Check-out form (asset combobox limited to
"tersedia", recipient, borrow date, outgoing condition, note) creates an active assignment and
removes the asset from the available pool. Check-in selects an active assignment, sets return date +
incoming condition + optional "needs maintenance" flag. History is a filterable/searchable table
with status + condition. Inline success/error banners (auto-dismiss).

**Maintenance** — 3 tabs (Jadwal / Catatan / Laporan Kerusakan). Jadwal: due-banner (items ≤3 days)
+ schedule cards with overdue/soon/later due badges (relative to a fixed "today" = 2026-06-24).
Catatan: searchable records table + an "Add Note" slideover (`USlideover`) that prepends a record.
Laporan Kerusakan (staff view): a damage-report form (asset you hold, problem category, description,
optional photo) that queues a report into "My Report History".

**Pengajuan & Approval** — master/detail inbox. Left: status filter tabs (Pending/Approved/Rejected/All)
+ type filter + request list. Right: detail panel with summary or before/after diff table, reason,
attachments, approval timeline, and (when pending) a note + Approve/Reject action. Deciding appends a
timeline entry and flips status. Sensitive types (disposal, valuation) show a warning. The pending
count drives the page subtitle; nav badge stays static.

**Laporan** — 4 report-type cards (Aset / Depresiasi / Utilisasi / Biaya) + filter bar
(period/office/category/status) + Apply. Before Apply → placeholder; after → KPI cards, a horizontal
bar chart (CSS bars, no chart lib), and a totaled data table. Export PDF/Excel are mock (toast).
Computations (totals, category grouping, averages) ported from the mockup's `build()`.

## Deviations from the mockups (intentional, per project conventions)

- The mockup chrome (sidebar, topbar, theme/lang toggles, collapse, "preview role" demo widgets) is
  **omitted** — the app shell (`layouts/default.vue`) already provides it. Only the page main content
  is reproduced.
- Theme/lang come from the app (`useI18n`, color-mode), not local component state.
- Literal hex/`var(--…)` colors in the mockup map to the project's semantic Nuxt UI tokens
  (`text-muted`, `bg-default`, `color="primary"`, status badge tones), keeping light/dark automatic.
- Mockup slideovers/modals use `USlideover`/`UModal`; the date "today" is fixed at **2026-06-24** to
  match the mockup's relative due-date math deterministically (also keeps tests stable).

## Testing

Per screen, proactively and broadly (CLAUDE.md):

- **Unit** (`test/unit/`) — mock store + composable: seed counts, list/filter/get, create/update,
  sentinel errors, and any pure computation (due-date bucketing, report totals/averages).
- **Runtime** (`test/nuxt/`, `mountSuspended`) — render each tab/state: populated, empty, loading,
  the create/decide flows (emitted result, appended row, status flip), filter narrowing, and
  permission-gated render. Assert real rendered text / resolved i18n, never hollow length checks.
- Extend `test/unit/nav-model.spec.ts` `BUILT_ROUTES` with the four new routes.

## Gates

`pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green before the PR. One branch
(`feat/operasional`), one commit per screen, one PR for the cluster. Final 1:1 visual comparison of
each built screen against its `docs/design/<Screen>.dc.html` in light + dark before claiming done.
