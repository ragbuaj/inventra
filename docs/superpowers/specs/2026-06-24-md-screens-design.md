# Phase 1 — Master Data Screens (Design Spec)

**Date:** 2026-06-24
**Parent:** `2026-06-24-frontend-feature-screens-roadmap-design.md`
**Status:** Approved spec, ready for writing-plans
**Backend:** Real API exists for all three entities — these screens call the live backend through
their `composables/api/` service (no mock fixtures needed), behind the same interface used by later
mock-backed phases.

## Goal

Implement the three Master Data screens faithfully to their `docs/design` mockups, on top of the
existing component library, with full CRUD, office-subtree data scope respected by the backend, i18n,
and tests.

| Screen | Mockup | Route | Backend endpoints |
|---|---|---|---|
| Kantor (offices) | `Master Data Kantor.dc.html` | `/master/offices` | `/offices`, `/offices/{id}` (+ `/floors`, `/rooms` nested) |
| Pegawai (employees) | `Master Data Pegawai.dc.html` | `/master/employees` | `/employees`, `/employees/{id}` |
| Referensi (reference) | `Master Data Referensi.dc.html` | `/master/reference` | reference-engine resources (`/office-types`, `/departments`, `/positions`, `/units`, `/maintenance-categories`, `/problem-categories`, `/brands`, `/vendors`, `/provinces`, `/cities`, `/models`) |

All three nav items already exist in `AppSidebar.vue` (`masterdata.office.manage` /
`masterdata.global.manage` permissions). No sidebar changes in this phase.

## Shared API layer

`app/composables/api/` (new folder). One composable per concern; each returns the typed CRUD surface
and calls `useApiClient().request(...)`. Responses use the backend envelope `{data,total,limit,offset}`
→ map to `Paginated<T>` (already in `app/types`). List inputs use `ListQuery`.

- **`useOffices.ts`** — `list(query)`, `get(id)`, `create(input)`, `update(id,input)`, `remove(id)`,
  plus `tree()` (offices arranged into the hierarchy for `TreeView`). Floors/rooms accessed via
  `useFloors.ts` / `useRooms.ts` (scoped to a selected office) if the mockup exposes them inline;
  otherwise deferred.
- **`useEmployees.ts`** — standard CRUD over `/employees`.
- **`useReference.ts`** — generic: `list(resource, query)` / `create(resource, input)` / … where
  `resource` is one of the 11 reference keys. Mirrors the backend's declarative engine: a single
  composable parameterized by resource string, with a typed `referenceResources` descriptor table
  (key, i18n label, field schema) living in `app/composables/api/referenceResources.ts`.

Types for `Office`, `Employee`, and the reference row shapes go in `app/types/index.ts` (or a
`app/types/masterdata.ts` if it grows), matching the OpenAPI schemas.

## Screen designs

### 1. Master Data Kantor (`/master/offices`)

Office hierarchy. Per the mockup: a master/detail layout — left a **`TreeView`** of the office
subtree the caller can see; selecting a node shows a detail panel (nama, code e.g. `JKT01`, kota,
alamat, parent, type). Toolbar (`DataToolbar`) with search. Create/Edit via `FormSlideover` (or
`FormModal` per mockup) with fields: nama, code, office-type, parent office (select within caller
scope), kota/province, alamat. Delete via `ConfirmDialog`. Scoped callers may only place an office
under a parent within their scope — surface backend `403`/conflict errors as toasts.

**States:** tree loading skeleton, empty (no offices in scope), detail empty (nothing selected),
form validation errors, delete confirm, API error toast.

### 2. Master Data Pegawai (`/master/employees`)

Flat list of employees. `PageHeader` + `DataToolbar` (search by name/email, optional filters) +
`ResourceTable` with columns from the mockup (nama, email, phone, department/position, office,
status badge) + `TablePagination`. Row actions: edit, delete. Create/Edit via `FormModal` /
`FormSlideover` with fields: nama, email (`nama@inventra.go.id`), phone (`08xx-xxxx-xxxx`), birth
year, department, position, office. `StatusBadge` for active/inactive.

**States:** table skeleton, empty state, form validation (email/phone format), delete confirm,
error toast.

### 3. Master Data Referensi (`/master/reference`)

One screen serving all 11 reference resources. Per the mockup: a resource switcher (tabs or select —
follow the mockup) drives `entityLabel`; the body is a `ResourceTable` whose columns and the
create/edit form fields come from the selected resource's **field schema** (in
`referenceResources.ts`). Flat fields only (name, code, symbol, address, email, phone, parent for
cities→provinces, etc., per resource). Reuse `ResourceTable` + `FormModal` + `ConfirmDialog`.

**States:** per-resource table skeleton/empty, dynamic-form validation, delete confirm, error toast.

## Components

Reuse only, unless markup repeats:

- Likely **new** shared component: `MasterDetailLayout.vue` (tree + detail panel) if the Kantor
  mockup's split is reused elsewhere — otherwise inline in the page.
- Everything else from the existing library (`ResourceTable`, `FormModal`, `FormSlideover`,
  `PageHeader`, `DataToolbar`, `TreeView`, `StatusBadge`, `ConfirmDialog`, `EmptyState`, skeletons,
  `TablePagination`).

## i18n

New keys under namespaces `masterdata.offices.*`, `masterdata.employees.*`, `masterdata.reference.*`
(field labels, placeholders, validation messages, empty/error copy, resource labels) in both
`id.json` and `en.json`. Default locale `id`.

## Permissions

- Offices/employees pages gated by `masterdata.office.manage`; reference by `masterdata.global.manage`
  (matches sidebar). Create/edit/delete buttons gated with `useCan` so a read-only caller sees a
  read-only view.

## Testing

- **Unit:** `useReference` resource-descriptor logic; any list-query mapping helper.
- **Runtime (`mountSuspended`, `// @vitest-environment nuxt`):** one per screen — assert the table
  renders seeded rows with resolved i18n labels, empty state shows when list is empty, and the
  create form opens. Mock the composable's `request` at the `useApiClient` boundary.
- **E2E (`frontend/e2e/`):** offices CRUD happy path against the real backend + seeded admin (create
  an office, see it in the tree, edit, delete). Runs in CI's e2e job.

## Verification

`pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green; screens match mockups in light and
dark mode; backend scope `403`s surface as user-facing errors (not silent failures).

## Out of scope (Phase 1)

- Floors/rooms management UI beyond what the Kantor mockup shows inline (can be a follow-up if the
  mockup omits it).
- Authorization-admin screens (Phase 2).
