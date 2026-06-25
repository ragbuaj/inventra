# Remaining Settings Screens — Design Spec (Data Scope · Field Permission · Audit Trail)

**Date:** 2026-06-25
**Phase:** Frontend feature screens (mock-first)
**Mockups (source of truth):** `docs/design/{Data Scope,Field Permission,Audit Trail}.dc.html`
**Routes:** `/settings/data-scope`, `/settings/field-permission`, `/settings/audit` — all three sidebar
items are currently disabled; this wires them. All gated by `middleware can(user.manage)`.

Builds the last three Settings screens 1:1 with their mockups. Mock-first (the backend authz-admin and
audit endpoints aren't built yet), consistent with the other built screens. Each screen is self-contained
(its own `mock/*.ts` + `composables/api/use*.ts`), ported verbatim from its mockup; bilingual catalog
data lives in the fixture (resolved by locale), page chrome lives in `settings.*` i18n.

## 1. Data Scope (`/settings/data-scope`)

A roles × (Default + modules) matrix of **data-scope levels** (`global` / `office_subtree` / `office` /
`own`). The Default column applies to all modules; each module cell either **inherits** the role default
(dashed pill) or **overrides** it (solid pill + warning dot). A legend explains the four levels.
Save/dirty state in the page header; footnote below the table.

- `mock/dataScope.ts` — `SCOPE_LEVELS` (key → color tokens + bilingual desc), `SCOPE_LEVEL_KEYS`,
  `DATA_SCOPE_MODULES` (5: aset/pengajuan/maintenance/master/laporan), `dataScopeRoleSeed` (6 roles with
  `def` + `ov` override map), and a `dataScopeStore` (mutable, with `reset()` for tests).
- `composables/api/useDataScope.ts` — `listRoles(locale)`, `getModules(locale)`,
  `saveScopes(roles)` (persists the whole matrix). Resolved view types carry localized role names + descs.
- `components/scope/ScopeCell.vue` — a pill button + `UPopover`/dropdown listing the four levels
  (+ "Follow Default" for module cells). Emits `select(level)` / `clear` (clear = inherit). Colored per
  level via tokens.
- `pages/settings/data-scope.vue` — header (title/subtitle + unsaved indicator + Save), legend card,
  the matrix table (sticky role column + highlighted Default column), footnote.

## 2. Field Permission (`/settings/field-permission`)

Per-entity field × role view/edit matrix. An **entity selector** + **field search** above a table whose
rows are fields and columns are the 5 roles; each cell has two toggle pills: **View (L)** and **Edit
(E)**. Fields with no explicit rule render as "default" (dimmed, opacity .5) and follow system default
(view+edit allowed); a per-row reset reverts to default. Toggle rules: turning View off also turns Edit
off; turning Edit on also turns View on. Save/dirty; empty state when search matches nothing; footnote.

- `mock/fieldPermission.ts` — `FIELD_ENTITIES` (4: aset/pegawai/user/pengajuan, each with bilingual
  field list + seed `rules`), `FIELD_ROLE_KEYS` + labels, and a `fieldPermStore` (mutable + `reset()`).
- `composables/api/useFieldPermission.ts` — `getEntities(locale)`, `getRules(entityKey)`,
  `saveRules(entityKey, rules)`.
- `components/fieldperm/FieldPermToggle.vue` — a single View/Edit pill pair for one (field, role) cell.
- `pages/settings/field-permission.vue` — header + Save, entity `USelect` + search `UInput`, the table,
  per-row reset, footnote, empty state.

## 3. Audit Trail (`/settings/audit`)

A **read-only** activity log. Toolbar: search (summary/actor/ref) + actor/action/entity `USelect`
filters + a date-from/date-to range + reset + an Export button (coming-soon toast). Table columns:
Time (date+time), Actor (avatar + name + role), Action badge (create=success / update=info /
delete=error), Entity, Change summary (+ ref code), Office/IP — each row **expands** to show the
before→after diff. Pagination (8/page); empty state.

- `mock/audit.ts` — `auditSeed` (14 logs with `diff[]`), `AUDIT_ENTITIES`, `AUDIT_ACTIONS` (meta:
  label/color/icon).
- `composables/api/useAudit.ts` — `list(query, locale)` returning resolved rows (localized summary/role,
  formatted date) — filtering (search/actor/action/entity/date range) + pagination handled in the page
  over the full set (mock), latency on initial load.
- `pages/settings/audit.vue` — toolbar, the expandable table (custom, since `ResourceTable` has no
  expandable rows), pagination, empty state. Read-only (no row actions).

## 4. i18n

New keys under `settings.dataScope.*`, `settings.fieldPermission.*`, `settings.audit.*` (titles,
subtitles, column/filter labels, legend, save/unsaved, empty states, footnotes, level/action labels that
are chrome). Entity/field/level descriptions and log content come from the fixtures.

## 5. Nav

Wire the three disabled sidebar items in `utils/nav.ts` to their routes; extend the nav-model test's
`BUILT_ROUTES` allowlist.

## 6. Testing

For each screen: **unit** (mock + composable — seed integrity, locale resolution, save/persist, filter
logic) and a **`mountSuspended` page test** asserting real rendered text/state:
- Data Scope: matrix renders roles+levels; changing a cell marks dirty + enables Save; module override
  vs inherit; Save clears dirty.
- Field Permission: entity switch changes fields; toggling View/Edit marks dirty (and the view→edit
  coupling); reset reverts a field; search filters; empty state.
- Audit Trail: rows render with action badges; search + each filter narrows; date range filters; row
  expands to show diff; pagination; empty state; export toast.
Use `enableAutoUnmount(afterEach)` where modals/teleports are involved.

## 7. Files (per screen: mock + composable + page + tests; shared: nav, i18n, mock barrel where safe)

## 8. Verification (DoD)

`pnpm lint` · `pnpm typecheck` · `pnpm test` · `pnpm build` green; live 1:1 comparison of each screen vs
its mockup in light **and** dark before claiming done. Built incrementally (one commit per screen).
