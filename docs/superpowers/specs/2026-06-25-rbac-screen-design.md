# Peran & RBAC Screen — Design Spec

**Date:** 2026-06-25
**Phase:** Frontend feature screens (mock-first)
**Mockup (source of truth):** `docs/design/Peran RBAC.dc.html`
**Route:** `/settings/rbac` (sidebar item `nav.rbac` is currently disabled — this wires it up)

## 1. Goal

Build the **Peran & RBAC** screen 1:1 with the mockup: a two-pane role/permission editor.
- **Left pane** — list of roles (system + custom), each with a shield icon, name, permission count, and a
  lock icon for system roles; an "Add Role" button at the bottom.
- **Right pane** — header (selected role name + System/Custom badge, description, unsaved-changes
  indicator, Save button) and a 2-column grid of **module cards**. Each card shows the module icon,
  label, granted/total count, a "Select all / Clear" toggle (custom roles only), and a list of permission
  switches (label + mono code).
- **Add Role modal** — name, optional "copy permissions from" role, description.

**System roles are read-only** (switches disabled, no select-all, Save disabled, a lock note banner).
**Custom roles** are editable: toggling permissions or select-all marks the draft dirty; Save commits it.

Mock-first behind a composable seam (the backend RBAC-admin endpoints aren't built yet — see
`docs/PROGRESS.md`). The module/permission catalog and seed roles come from the mockup verbatim; a later
pass maps them to the real `identity.role_permissions` keys.

## 2. Scope

### In scope
- `pages/settings/rbac.vue`, gated by `definePageMeta({ middleware: 'can', permission: 'user.manage' })`.
- `mock/rbac.ts` (modules + 7 seed roles) and `useRbac` composable.
- Two components: `RbacRoleList` (left), `RbacPermissionCard` (one module card). The matrix grid and the
  Add Role modal live in the page.
- Wire the sidebar nav item to `/settings/rbac`.
- Full `id`/`en` i18n for the chrome; Vitest unit + component coverage.

### Out of scope
- Real backend wiring; persistence across reloads (mock store, like the other screens).
- Reordering/deleting roles, renaming system roles (mockup doesn't offer these).

## 3. Data model (`mock/rbac.ts`)

The mockup models module/permission labels and role names as bilingual `{id,en}` — these are catalog
**data** a real API returns already-localized, so they live in the fixture (not i18n), resolved by locale
in `useRbac`. Page chrome is i18n (bagian 6).

```ts
export interface Localized { id: string; en: string }
export interface PermissionDef { code: string; label: Localized }   // code e.g. 'aset.view'
export interface ModuleDef { key: string; label: Localized; icon: string; perms: PermissionDef[] }
export interface Role {
  key: string                 // 'superadmin' | 'custom-1' | …
  nama: Localized
  system: boolean             // system roles are read-only
  desc: Localized
  perms: string[]             // granted permission codes
}
```
- `RBAC_MODULES: ModuleDef[]` — 8 modules (aset, penugasan, maintenance, pengajuan, master, user,
  laporan, audit) with their permissions, ported from the mockup. Icons mapped to `i-lucide-*`.
- `ALL_PERMISSION_CODES` — flat list (used by the Superadmin seed).
- `roleSeed: Role[]` — 7 roles (5 system: superadmin/kakanwil/kaunit/manager/staf; 2 custom:
  auditor/gudang), with the mockup's permission sets.
- `roleStore` — a small mutable store keyed by `key` (`all`, `find`, `insert`, `setPerms`).

### `composables/api/useRbac.ts`
The real-API seam:
- `listRoles(locale)` → `Promise<RoleView[]>` (name/desc resolved to strings; perms kept).
- `getModules(locale)` → `ModuleView[]` (labels resolved).
- `createRole({ nama, copyFromKey?, desc }, locale)` → `Promise<RoleView>` — perms copied from the source
  role (or empty); new role is `system: false`.
- `updateRolePermissions(key, perms)` → `Promise<void>` — commits the draft to the store.
All resolve after `fakeLatency()`. `RoleView`/`ModuleView` are the resolved (string-label) shapes.

## 4. Components

| Component | Responsibility | Props / events |
|---|---|---|
| `RbacRoleList` | Left pane: role buttons (icon, name, perm-count, lock for system) + "Add Role" button. Highlights the selected role. | `roles, selectedKey, permCountLabel(n)`; emits `select(key)`, `add` |
| `RbacPermissionCard` | One module card: icon + label + `granted/total` count, optional Select-all/Clear, and the permission switches (label + mono code). Switches/select-all disabled when `readonly`. | `module, grantedCodes, readonly, countLabel, selectAllLabel, clearLabel`; emits `toggle(code)`, `toggle-all` |

`pages/settings/rbac.vue` owns: `roles`, `modules`, `selectedKey`, `draft` (granted codes), `dirty`; the
right-pane header (badges, unsaved indicator, Save); the module grid (`RbacPermissionCard` per module);
and the Add Role `UModal` (custom footer button "Buat Peran"). Switches use `USwitch`.

### Interactions
- **Select role** → `draft = role.perms`, `dirty = false`.
- **Toggle permission / Select-all** → no-op for system roles; else update `draft`, set `dirty`.
- **Save** → no-op when system or not dirty; else `updateRolePermissions`, sync local role, clear dirty,
  success toast.
- **Add Role** → validates name (required); creates a custom role (optionally copying perms), selects it.

## 5. Permission gating

Route + nav gated by `user.manage` (superadmin-level, same as User Management). The mockup's `user.rbac`
permission is part of the catalog data but the route guard uses the real `user.manage` key.

## 6. i18n (`settings.rbac.*`)

`title`, `rolesTitle`, `rolesSub`, `addRole`, `systemBadge`, `customBadge`, `lockNote`,
`unsavedChanges`, `saveChanges`, `savedToast`, `permCount` ({n}), `moduleCount` ({granted}/{total}),
`selectAll`, `clearAll`, `add.title`, `add.subtitle`, `add.roleName`, `add.copyFrom`, `add.copyNone`,
`add.copyNote`, `add.description`, `add.namePlaceholder`, `add.descPlaceholder`, `add.required`,
`add.create`, `add.createdToast`, `add.defaultDesc`. Module/permission labels come from the fixture.

## 7. Testing

- **Unit (node):** `mock/rbac` — 8 modules; `ALL_PERMISSION_CODES` length = sum of module perms;
  Superadmin seed grants every code; system flags correct. `useRbac` — `createRole` copies the source
  role's perms (and starts empty with no source); `updateRolePermissions` persists; `listRoles` resolves
  localized names.
- **Component (nuxt env, `mountSuspended`):** roles list renders all 7 with counts; selecting a role
  swaps the matrix; a **system** role shows the lock note, disabled switches, and a disabled Save; a
  **custom** role toggle flips the module count, shows "unsaved changes", and enables Save; Save clears
  the unsaved state; Select-all grants every permission in a module; the Add Role modal validates a
  required name and creates a new role that appears in the list. Assert resolved text / counts / state,
  not hollow checks.

## 8. Files

**New:** `pages/settings/rbac.vue`, `components/rbac/RbacRoleList.vue`,
`components/rbac/RbacPermissionCard.vue`, `mock/rbac.ts`, `composables/api/useRbac.ts`,
`test/unit/rbac-mock.spec.ts`, `test/nuxt/settings-rbac.spec.ts`.
**Modified:** `mock/index.ts` (re-export), `utils/nav.ts` (wire `/settings/rbac`),
`i18n/locales/{id,en}.json` (+`settings.rbac.*`).

## 9. Verification (DoD)

- `pnpm lint` · `pnpm typecheck` · `pnpm test` · `pnpm build` green.
- Live 1:1 comparison of `/settings/rbac` vs the mockup in light **and** dark — two-pane layout, role
  list, module cards, switches, system lock state, dirty/save, and the Add Role modal. Fix any deviation
  before done.
