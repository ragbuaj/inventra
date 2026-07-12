# Tech-Debt Sweep — Design

**Date:** 2026-07-12
**Status:** Approved (design)
**Scope:** Three independent tech-debt items from `docs/PROGRESS.md` item 49(c):
field-permission enforcement, enriched audit response, async searchable pickers.

This spec covers all three as one cohesive sweep. They are independent and may be
implemented in parallel, with one synergy: the audit actor filter (Part 2) reuses the
generic async picker built in Part 3.

---

## Part 1 — Field-permission enforcement

### Problem

The `field_permissions` infrastructure (`internal/authz/fields.go`, table
`identity.field_permissions`, `FilterView` default-allow) is only enforced on `assets`,
`users`, and `requests`. The Field Permission admin screen lets operators configure rules
for other entities, but they take no effect. Three separate, inconsistent enforcement
helpers exist, and two response paths leak fields that the main asset path masks.

Current state (from scoping):
- **Enforced today:** `user` (`filterMaps`, **fail-open** on error), `asset` (`filterMap`,
  fail-closed), `approval`/requests (`filterMap`, fail-closed), `depreciation`
  (hand-rolled single-field `book_value` check).
- **`employee`** serializes a typed struct `Response` (not a map), has **no** `fieldSvc`,
  and is not wired to receive one (`masterdata.RegisterRoutes` does not pass `fieldSvc`).
- **Known leaks:** `depreciation.impairmentResultToMap` returns `book_value` /
  `accumulated_depreciation` unmasked (flagged in a code comment); asset attachment /
  document maps are unfiltered (to be verified — see below).
- Entity keys in `field_permissions` are **free-form** — no schema or `validateFieldPerms`
  change is needed to add an entity.

### Design

**1a. Canonical fail-closed helper.** Add a method on `FieldService`:

```
func (s *FieldService) FilterEntity(ctx, roleID uuid.UUID, entity string, data map[string]any) error
```

It calls `ForEntity` then the package-level `FilterView`, returning the error (fail-closed)
so callers stop on lookup failure. Refactor `user`, `asset`, and `approval` to use it,
replacing their per-handler `filterMap`/`filterMaps`. This changes `user` from **fail-open**
to **fail-closed** — a deliberate security improvement. `FilterView` (pure function) stays
as-is; `FilterEntity` is the single combined entry point going forward.

**1b. Employees enforcement.**
- Convert the employee response from typed `toResponse(e) Response` to an `employeeToMap(e)
  map[string]any` DTO (keys matching the frontend catalog).
- Inject `*authz.FieldService` into the employee `Handler`; update `NewHandler` signature.
- Thread `fieldSvc` through `masterdata.RegisterRoutes(...)` → the employee sub-module ctor.
  (`masterdata.go` aggregator + `employee/routes.go`.)
- Apply `FilterEntity(ctx, roleID, "employees", m)` on the create/update/get/list response
  paths. Entity key is **`"employees"`** (matches the scope-module key); fix the authz
  integration test that uses the singular `"employee"`.

**1c. Close known leaks.**
- `depreciation.impairmentResultToMap`: route `book_value` / `accumulated_depreciation`
  through the same field policy the asset schedule path uses (mask when
  `policies["book_value"].CanView == false`), consistent with `maskedAssetScheduleMap`.
- Asset attachment/document maps (`attachmentToMap`, `documentToMap`): **verify first** — if
  they only serialize file metadata (filename, size, uploader) and do not echo masked
  asset money fields, they are **not** a leak and are left untouched (honest scoping). Only
  apply filtering if they actually surface a field the `assets` policy masks.

**Frontend.** Add an `employees` entity to `frontend/app/constants/fieldCatalog.ts` with
fields `name, nip, email, phone, department_id, position_id, office_id, status` (keys must
match `employeeToMap`). Add i18n labels under `settings.fieldPermission.entity.employees`
and `.field.<field>` for id + en (fallback is the raw key).

**Tests.**
- Backend integration: a role with `can_view=false` on an employee field receives a
  response with that field removed; a role with no policy sees all fields (default-allow).
- Backend: fix the `"employee"` → `"employees"` key in the existing authz test.
- Frontend: extend `field-catalog.spec.ts`; the field-permission e2e includes the employees
  entity.

---

## Part 2 — Enriched audit response

### Problem

`GET /api/v1/audit` returns actor `{id, name, email}` and a raw `office_id` UUID. Missing:
the actor's **role**, the **office name**, and a human **summary**. The backend already
accepts an `actor_id` filter, but the frontend never wired it (actor filter dropped).

Scoping facts:
- Actor **name** is already resolved via `LEFT JOIN identity.users` in `ListAuditLogs`.
- Audit rows do **not** store role or a summary; `office_id` is stored (nullable, **no FK** —
  audit is append-only and outlives office deletion).
- Name resolution done as **SQL joins inside the audit query** does not invoke the
  user/masterdata service layers, so it does **not** trip their `user.manage` / masterdata
  permission checks — an `audit.view`-only viewer can safely receive resolved names.

### Design

**2a. Backend joins.** Extend `ListAuditLogs` and `CountAuditLogs` (in
`db/queries/audit.sql`; `CountAuditLogs` only if it needs the same filter columns — joins
are for the list) with:
- `LEFT JOIN identity.roles ro ON ro.id = u.role_id` → `actor_role` (the actor's **current**
  role; not snapshotted at action time — accepted limitation, documented).
- `LEFT JOIN masterdata.offices o ON o.id = a.office_id` → `office_name` (nullable; tolerate
  soft-deleted / missing office rows).

Run `sqlc generate`. Add `role` (under `actor`) and `office_name` to `auditToMap` in
`internal/audit/dto.go`. Update `backend/api/openapi.yaml` audit schema.

**2b. Summary — read-time, derived on the frontend.** The frontend already receives
`entity_type`, `action`, and the `changes` diff. It composes a localized, human-readable
summary there (e.g. "Mengubah 3 field pada Aset AST-001", "Membuat Pengguna") via i18n
templates keyed by `action`, with the affected-field count derived from the diff. No new
column, no migration, works retroactively for all existing rows, and honors the i18n mandate.

**2c. Actor filter (frontend).** Add `actor_id` to `AuditListParams` and wire it into the
`useAudit` fetch. The audit filter bar gains an actor picker — an **`AsyncSearchPicker`
(Part 3) bound to `useUsers`** — so a viewer can filter by who performed the action.

**Frontend.** Add **Role** and **Office** columns to the audit table
(`pages/settings/audit.vue`); render the derived **summary** line; add the actor filter
control. Update `useAudit` `AuditDTO`/`AuditRow`/`AuditListParams` and `auditCatalog` as
needed.

**Tests.**
- Backend integration: response includes `actor.role` and `office_name`; office name is
  null-safe when the office was soft-deleted; `actor_id` filter narrows the list.
- Frontend: `use-audit.spec.ts` asserts role/office/summary mapping and the localized
  summary text; `settings-audit.spec.ts` renders the new columns + actor filter; a case
  where role/office are null renders gracefully.

---

## Part 3 — Async searchable pickers (all)

### Problem

Office, employee, and every reference/category picker eagerly fetch `{ limit: 100 }` and
filter client-side. `100` is also the backend's hard ceiling (`ClampInt(..., 1, 100)`), so
lists over 100 rows silently truncate. This repeatedly breaks e2e (a freshly created
office/employee isn't reliably selectable through the dropdown), and `USelectMenu`'s
focus-trap compounds it.

Scoping facts:
- **Backend already fully supports server-side search** for offices, employees, **and all
  reference resources** (`Search: true` columns → `ILIKE`) and categories — all data-scoped,
  paginated. **No backend change is required for Part 3.**
- `AssetSearchPicker.vue` already implements the target pattern (300ms debounce, stale-
  response `seq` guard, outside-click close, skeleton loading, empty state) using a
  hand-rolled dropdown (`UInput` + `<ul>`), deliberately avoiding `USelectMenu` to dodge the
  focus-trap. It is hardcoded to `useAssets()`.

### Design

**3a. Generic component `AsyncSearchPicker.vue`.** Generalize `AssetSearchPicker` into a
resource-agnostic picker in `app/components/`:
- Props: `searchFn(term: string) => Promise<PickerItem[]>`, `resolveFn(id) =>
  Promise<PickerItem | null>` (to display a preselected value even when it's outside the
  latest search page), `modelValue` (selected id), `placeholder`, `disabled`, and an
  optional `sublabel` renderer. `PickerItem = { id, label, sublabel? }`.
- Emits `update:modelValue`.
- Behavior: debounced server search, seq-guard against stale responses, outside-click close,
  skeleton loading, **"No Data" empty state** (per the component-first form-input rule),
  keyboard-navigable. Hand-rolled dropdown — **not** `USelectMenu`.

**3b. Refactor `AssetSearchPicker.vue`** into a thin wrapper over `AsyncSearchPicker`
(passing an asset `searchFn`/`resolveFn` and asset-shaped `label`/`sublabel` with the tag +
status rendering), keeping its public API and existing tests green.

**3c. Swap all `limit:100` pickers** to `AsyncSearchPicker`, in **both** form fields and
filter dropdowns, across every screen:
- Office: asset form/catalog/label, employees, transfers, disposals, stock-opname, users,
  reports, depreciation, approval, dashboard, master/offices.
- Employee: employees master, assignment, users.
- Reference/category: category, brand, model, unit, vendor, problem-category,
  maintenance-category (asset form, maintenance slideovers, master/reference).
- Filter contexts get an explicit "Semua/clear" option (null selection).

**3d. Preselected display.** Read-only id→name maps on detail pages stay as-is. Where a
picker must display a stored value whose row may be outside the current search page, it uses
`resolveFn(id)`.

**Tests.**
- `AsyncSearchPicker` nuxt spec: base render, debounce, results, selection, resolve of a
  preselected value, disabled state, empty "No Data" state, outside-click close, stale-
  response guard.
- Update the ~30 specs that assert `{ limit: 100 }` to the new search-driven call shape.
- e2e: the assignment/office cases that were forced to API-only setup now select through the
  async picker (type → pick); update those specs. Keep the persistent-data-uniqueness and
  fill-text-before-opening-popover conventions.

---

## Non-goals

- Snapshotting the actor's role at action time (Part 2 uses current role).
- Write-time audit summary column / migration (Part 2 derives at read time).
- Field-permission **write**-side (edit) masking — `FilterView` enforces view only, unchanged.
- Adding search to backend endpoints — already present for all in-scope resources.
- The other item-49(c) tech-debt (Users server-side filters + reset-password, badge counts,
  Data-Scope e2e cleanup) — out of scope for this sweep.

## Verification gate (all three parts)

Backend: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration
./... -p 1` (all packages), Spectral lint. Frontend: `pnpm lint`, `pnpm typecheck`,
`pnpm test`, `pnpm build`; affected e2e (`pnpm test:e2e`) for field-permission, audit, and
the picker-touched flows. Update `docs/PROGRESS.md` item 49(c) when each part lands.
