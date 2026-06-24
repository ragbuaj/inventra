# Frontend Feature Screens — Implementation Roadmap

**Date:** 2026-06-24
**Status:** Approved roadmap (master plan)
**Scope:** Implement the remaining ~18 high-fidelity mockups in `docs/design/` on top of the
already-built Nuxt 4 frontend foundation.

## Context

The frontend **foundation** is complete: SPA shell (`layouts/default.vue` = `AppSidebar` +
`AppTopbar` + `UMain`), `auth` layout, real auth (`useAuthApi`, `useApiClient`, auth/RBAC
middleware), a global component library (`ResourceTable`, `FormModal`, `FormSlideover`,
`PageHeader`, `DataToolbar`, `StatCard`, `TreeView`, `StatusBadge`, `ConfirmDialog`, `EmptyState`,
skeletons, …), i18n (`id`/`en`), and the Vitest + Playwright harness.

**Built screens:** Login (real), a placeholder Dashboard (`index.vue`, zeroed stat cards), and the
dev component gallery (`pages/dev/components.vue`). The *App Shell* and *Component Library* mockups
are therefore already realized and excluded from this roadmap.

**Remaining mockups (~18)** in `docs/design/` map to the phases below. Most have **no backend module
yet** (asset, maintenance, approval, assignment, reporting, authz-admin) — see `docs/PROGRESS.md`.

## Data strategy — mock-first, swap-later

Decision: build **all** screens now against in-memory mock fixtures, behind a stable composable
interface, and swap each composable to real `$fetch` when its backend module ships. The foundation
already anticipates this — `app/mock/index.ts` reserves the slot ("Module fixtures … re-exported
here in later phases"), and `paginate`/`filterBy` helpers + `Paginated<T>`/`ListQuery` types exist.

Pattern (one per entity):

- **`app/composables/api/use<Entity>.ts`** — typed CRUD surface: `list(query): Promise<Paginated<T>>`,
  `get(id)`, `create(input)`, `update(id, input)`, `remove(id)`. **The signature is the contract.**
- **`app/mock/<entity>.ts`** — seeded fixture array (realistic Indonesian data) + the composable's
  current implementation using `paginate`/`filterBy` and a small simulated-latency wrapper; re-export
  from `app/mock/index.ts`.
- **Swap later:** when the backend module exists, only the composable body changes — from reading the
  fixture to `useApiClient().request(...)`. Pages, components, and tests are untouched. Fixtures return
  the same `{data,total,limit,offset}` envelope the backend uses, so the swap is mechanical.

**All phases — including Phase 1 — use mock fixtures first**, for a uniform pattern across the whole
codebase. Even where the backend already exists (offices, employees, reference, users), the screen is
built against a mock fixture behind the composable, then swapped to real `$fetch` later. The
availability table below records which composables can be swapped earliest, not which start real.

### Backend availability (as of 2026-06-24)

Real endpoints present under `/api/v1`: `/offices`, `/offices/{id}`, `/employees`,
`/employees/{id}`, `/categories`, `/categories/{id}`, the reference-engine resources
(`/office-types`, `/departments`, `/positions`, `/units`, `/maintenance-categories`,
`/problem-categories`, `/brands`, `/vendors`, `/provinces`, `/cities`, `/models`), `/floors`,
`/rooms`, and `/users`, `/users/{id}`. Everything else (asset, assignment, maintenance, approval,
reporting/dashboard aggregates, import, audit views, authz-admin CRUD) is **mock-only** today.

## Phase decomposition

Each phase is an independent spec → implementation-plan → build cycle. **This document is the master
roadmap.** The companion Phase 1 spec (`2026-06-24-md-screens-design.md`) is the only phase specced in
full today; later phases get their own spec when reached.

| Phase | Mockup → route | Backend |
|---|---|---|
| **1. Master Data** | Master Data Kantor → `/master/offices` (tree); Master Data Pegawai → `/master/employees`; Master Data Referensi → `/master/reference` | ✅ real API |
| **2. User & Otorisasi** | Manajemen User → `/settings/users`; Peran RBAC → `/settings/roles`; Data Scope → `/settings/data-scope`; Field Permission → `/settings/field-permissions` | User ✅ · authz-admin mock |
| **3. Asset core** | Katalog Aset → `/assets`; Detail Aset → `/assets/[id]`; Form Aset → `/assets/new` + `/assets/[id]/edit`; Import Aset → `/assets/import`; Label Barcode → `/assets/labels` | mock |
| **4. Operasi** | Penugasan Aset → `/assignment`; Maintenance → `/maintenance`; Pengajuan Approval → `/approval` | mock |
| **5. Insight** | Dashboard → `/` (rebuild `index.vue`); Laporan → `/reports`; Audit Trail → `/settings/audit` | mock |

### Navigation changes

`AppSidebar.vue` already defines: `/`, `/assets`, `/assignment`, `/maintenance`, `/approval`,
`/master/{offices,employees,reference}`, `/settings/{users,audit}`. New nav items to add (in the
phase that builds them, permission-gated via `useCan`):

- Settings group: `/settings/roles` (RBAC), `/settings/data-scope`, `/settings/field-permissions`
  (Phase 2).
- Asset group or Main: `/reports` (Phase 5).
- Asset sub-routes (`/assets/new`, `/assets/[id]`, `/assets/import`, `/assets/labels`) are nested and
  reached from the catalog, not top-level nav (Phase 3).

## Per-screen build checklist (uniform)

Applied to every screen, matching the CLAUDE.md frontend order:

1. **Open `docs/design/<Screen>.dc.html`** in a browser. Extract layout, the exact set of components,
   and **every state**: loading (skeletons), empty, error, and populated/success. The mockup is the
   visual source of truth for both light and dark mode.
2. **Reuse global components first** (`ResourceTable`, `FormModal`/`FormSlideover`, `PageHeader`,
   `DataToolbar`, `StatCard`, `TreeView`, `StatusBadge`, `ConfirmDialog`, `EmptyState`, skeletons).
   Extract a new component into `app/components/` only when the same markup repeats across pages.
3. **Back data** with the phase's `composables/api/use<Entity>` service (mock today).
4. **i18n:** every user-facing string into `i18n/locales/{id,en}.json`, referenced via `$t`/`useI18n`.
   Gate role-specific UI with `useCan` / `<Can>`.
5. **Tests:** unit tests for mock services / pure logic; a `mountSuspended` runtime test per screen
   asserting real rendered text and each state; an e2e for the headline flow. Assert real behavior —
   never `expect(html.length).toBeGreaterThan(0)`.
6. **Verify green** before commit: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`. Match the
   mockup in light **and** dark mode.

## Out of scope

- Backend feature modules (asset/maintenance/approval/etc.) — tracked separately in `docs/PROGRESS.md`;
  this roadmap only swaps mock composables to real `$fetch` once those land.
- App Shell and Component Library (already built).
- Real PDF/Excel export and barcode rendering internals beyond a faithful UI mock (the backend owns
  generation; the UI mocks the trigger + preview).

## Deliverables today

1. This roadmap.
2. The Phase 1 (Master Data) detailed spec → `2026-06-24-md-screens-design.md`.
