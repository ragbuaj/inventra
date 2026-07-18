# Wire RBAC screen to `/authz` API — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mock-backed `useRbac` composable + RBAC settings screen with the real `/api/v1/authz` backend (catalog, roles, role-permissions), adopting English snake_case DTO keys and id-based role identity.

**Architecture:** `useRbac` is rewritten to call `useApiClient().request` against `/authz/catalog`, `/authz/roles`, `/authz/roles/:id/permissions`. Permission/group display labels come from a frontend i18n map (keys + grouping come from the API catalog; icons from a frontend constant); role `code` is auto-derived from the name on create. The page loads each role's permissions eagerly (parallel) so the role-list count from the mockup is preserved.

**Tech Stack:** Nuxt 4 (SPA), Nuxt UI (`U*`), `@nuxtjs/i18n` (id default + en), Vitest + `@nuxt/test-utils` (`mountSuspended`, `registerEndpoint`), Playwright e2e.

## Global Constraints

- Wire ONLY the RBAC screen: `pages/settings/rbac.vue`, `composables/api/useRbac.ts`, `components/rbac/*`. Do NOT touch other composables or regroup folders (rest of ADR-0007 deferred).
- Adopt English keys in `useRbac`: `RoleView { id, code, name, is_system, description?, perms }` (drop `nama`/`system`/`desc`/`key`). `ModuleView` keeps its shape `{ key, label, icon, perms: [{code,label}] }` (so `RbacPermissionCard` needs no change) but is now built from the API catalog (`group`→`key`, `items[].key`→`perms[].code`).
- Endpoints: `GET /authz/catalog`, `GET /authz/roles` (`{data,total}`), `POST /authz/roles` (`{code,name,description?}`→201), `GET /authz/roles/:id/permissions` (`{permissions:[]}`), `PUT /authz/roles/:id/permissions` (`{permissions:[]}`).
- All API calls go through `useApiClient().request<T>(path, opts)` (Bearer + refresh-on-401 + error toast already handled there). Never hardcode the backend URL.
- Role identity for selection + CRUD is the UUID `id` (not the human `code`).
- Create role: derive `code = slugifyRoleCode(name)`; on backend `409` show an inline form error, not a generic toast. Copy-from = client-side (create → get source perms → put new perms). New roles always `is_system=false`.
- System roles: permissions remain editable & savable; only delete/code are locked (no delete in this screen). Keep the lock badge/note.
- i18n mandatory: every new user-facing string in `i18n/locales/{id,en}.json`. Permission/group labels resolve via i18n with **fallback to the catalog API label** when a key is missing.
- Match `docs/design/Peran RBAC.dc.html` 1:1 (layout + every state) after wiring.
- Gates: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green. Run from `frontend/`.

---

### Task 1: Catalog presentation constants + i18n labels

**Files:**
- Create: `frontend/app/constants/authzCatalog.ts`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`
- Test: `frontend/test/unit/authz-catalog.spec.ts`

**Interfaces:**
- Produces: `GROUP_ICON: Record<string,string>`; `iconForGroup(group: string): string` (fallback `i-lucide-key`).

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/authz-catalog.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { iconForGroup } from '~/constants/authzCatalog'

describe('iconForGroup', () => {
  it('maps known groups to icons', () => {
    expect(iconForGroup('Sistem')).toBe('i-lucide-shield')
    expect(iconForGroup('Aset')).toBe('i-lucide-box')
  })
  it('falls back for unknown groups', () => {
    expect(iconForGroup('Tak Dikenal')).toBe('i-lucide-key')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- authz-catalog`
Expected: FAIL — cannot resolve `~/constants/authzCatalog`.

- [ ] **Step 3: Create the constants file**

Create `frontend/app/constants/authzCatalog.ts`:

```ts
// Presentation metadata for the authz permission catalog. The API catalog
// supplies the authoritative permission keys + grouping; icons (and i18n
// labels, see locale files) are a frontend concern.
export const GROUP_ICON: Record<string, string> = {
  'Sistem': 'i-lucide-shield',
  'Master Data': 'i-lucide-database',
  'Aset': 'i-lucide-box',
  'Persetujuan': 'i-lucide-git-pull-request',
  'Cadangan': 'i-lucide-layers'
}

export function iconForGroup(group: string): string {
  return GROUP_ICON[group] ?? 'i-lucide-key'
}
```

- [ ] **Step 4: Add i18n catalog labels**

In `frontend/i18n/locales/id.json` and `en.json`, under the existing `settings.rbac` object, add a `catalog` block. Keys are slugged group names and the raw permission keys (dots are valid in JSON keys but use a nested `perm` object keyed by the full permission string).

Add to **id.json** `settings.rbac`:

```json
"catalog": {
  "group": {
    "Sistem": "Sistem",
    "Master Data": "Master Data",
    "Aset": "Aset",
    "Persetujuan": "Persetujuan",
    "Cadangan": "Cadangan"
  },
  "perm": {
    "user.manage": "Kelola user",
    "role.manage": "Kelola peran & RBAC",
    "scope.manage": "Kelola data scope",
    "fieldperm.manage": "Kelola field permission",
    "audit.view": "Lihat audit trail",
    "masterdata.global.manage": "Kelola master data global",
    "masterdata.office.manage": "Kelola kantor & pegawai",
    "asset.view": "Lihat aset",
    "asset.manage": "Kelola aset",
    "request.create": "Buat pengajuan",
    "request.decide": "Setujui/tolak pengajuan",
    "approval.config.manage": "Kelola ambang persetujuan",
    "report.view": "Lihat laporan",
    "report.export": "Ekspor laporan",
    "maintenance.manage": "Kelola maintenance",
    "depreciation.manage": "Kelola penyusutan",
    "valuation.exclude.approve": "Setujui pengecualian valuasi",
    "assignment.manage": "Kelola penugasan aset"
  }
}
```

Add to **en.json** `settings.rbac`:

```json
"catalog": {
  "group": {
    "Sistem": "System",
    "Master Data": "Master Data",
    "Aset": "Asset",
    "Persetujuan": "Approval",
    "Cadangan": "Reserved"
  },
  "perm": {
    "user.manage": "Manage users",
    "role.manage": "Manage roles & RBAC",
    "scope.manage": "Manage data scope",
    "fieldperm.manage": "Manage field permissions",
    "audit.view": "View audit trail",
    "masterdata.global.manage": "Manage global master data",
    "masterdata.office.manage": "Manage offices & employees",
    "asset.view": "View assets",
    "asset.manage": "Manage assets",
    "request.create": "Create requests",
    "request.decide": "Approve/reject requests",
    "approval.config.manage": "Manage approval thresholds",
    "report.view": "View reports",
    "report.export": "Export reports",
    "maintenance.manage": "Manage maintenance",
    "depreciation.manage": "Manage depreciation",
    "valuation.exclude.approve": "Approve valuation exclusion",
    "assignment.manage": "Manage asset assignment"
  }
}
```

Also add these RBAC error/conflict strings to **both** files under `settings.rbac` (id shown; provide en equivalents):

```json
"loadError": "Gagal memuat data peran.",
"retry": "Coba lagi"
```
(en: `"loadError": "Failed to load roles.", "retry": "Retry"`)

And under `settings.rbac.add`, add the conflict message:
```json
"conflict": "Nama peran sudah dipakai."
```
(en: `"conflict": "Role name already in use."`)

- [ ] **Step 5: Run the test + lint**

Run (from `frontend/`): `pnpm test -- authz-catalog && pnpm lint`
Expected: PASS, lint clean (no trailing commas; 1tbs).

- [ ] **Step 6: Commit**

```bash
git add frontend/app/constants/authzCatalog.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/unit/authz-catalog.spec.ts
git commit -m "feat(rbac): catalog presentation constants + i18n labels"
```

---

### Task 2: Rewrite `useRbac` to the real API

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useRbac.ts`
- Delete: `frontend/test/unit/rbac-mock.spec.ts`
- Test: `frontend/test/unit/use-rbac.spec.ts`

**Interfaces:**
- Consumes: `useApiClient().request` (from Task 0 codebase); `iconForGroup` (Task 1).
- Produces: types `PermissionView{code,label}`, `ModuleView{key,label,icon,perms:PermissionView[]}`, `RoleView{id,code,name,is_system,description?,perms:string[]}`, `CreateRoleInput{name,description?,copyFromId?}`; `slugifyRoleCode(name:string):string`; `useRbac()` returning `{ getCatalog, listRoles, getRolePermissions, createRole, updateRolePermissions }`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/test/unit/use-rbac.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

import { useRbac, slugifyRoleCode } from '~/composables/api/useRbac'

beforeEach(() => request.mockReset())

describe('slugifyRoleCode', () => {
  it('lowercases and underscores non-alphanumerics', () => {
    expect(slugifyRoleCode('Auditor Cabang')).toBe('auditor_cabang')
    expect(slugifyRoleCode('  Kepala  Unit!! ')).toBe('kepala_unit')
    expect(slugifyRoleCode('Tim A/B')).toBe('tim_a_b')
  })
})

describe('useRbac', () => {
  it('getCatalog maps groups to modules with icon + perms', async () => {
    request.mockResolvedValueOnce({
      permissions: [{ group: 'Aset', items: [{ key: 'asset.view', label: 'Lihat aset' }] }],
      scope_levels: [], scope_modules: []
    })
    const mods = await useRbac().getCatalog()
    expect(request).toHaveBeenCalledWith('/authz/catalog')
    expect(mods[0]).toMatchObject({ key: 'Aset', icon: 'i-lucide-box' })
    expect(mods[0].perms[0]).toEqual({ code: 'asset.view', label: 'Lihat aset' })
  })

  it('listRoles returns data array', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'u1', code: 'manager', name: 'Manager', is_system: true }], total: 1 })
    const roles = await useRbac().listRoles()
    expect(request).toHaveBeenCalledWith('/authz/roles')
    expect(roles).toHaveLength(1)
    expect(roles[0]).toMatchObject({ id: 'u1', code: 'manager', is_system: true })
  })

  it('getRolePermissions unwraps permissions', async () => {
    request.mockResolvedValueOnce({ permissions: ['asset.view', 'asset.manage'] })
    expect(await useRbac().getRolePermissions('u1')).toEqual(['asset.view', 'asset.manage'])
    expect(request).toHaveBeenCalledWith('/authz/roles/u1/permissions')
  })

  it('updateRolePermissions PUTs the permission set', async () => {
    request.mockResolvedValueOnce({ permissions: ['asset.view'] })
    await useRbac().updateRolePermissions('u1', ['asset.view'])
    expect(request).toHaveBeenCalledWith('/authz/roles/u1/permissions', {
      method: 'PUT', body: { permissions: ['asset.view'] }
    })
  })

  it('createRole derives code, posts, and copies perms when copyFromId set', async () => {
    request
      .mockResolvedValueOnce({ permissions: ['asset.view', 'asset.manage'] }) // get source perms
      .mockResolvedValueOnce({ id: 'new1', code: 'auditor', name: 'Auditor', is_system: false }) // post
      .mockResolvedValueOnce({ permissions: ['asset.view', 'asset.manage'] }) // put new perms
    const role = await useRbac().createRole({ name: 'Auditor', copyFromId: 'src1' })
    expect(request).toHaveBeenNthCalledWith(1, '/authz/roles/src1/permissions')
    expect(request).toHaveBeenNthCalledWith(2, '/authz/roles', { method: 'POST', body: { code: 'auditor', name: 'Auditor', description: undefined } })
    expect(request).toHaveBeenNthCalledWith(3, '/authz/roles/new1/permissions', { method: 'PUT', body: { permissions: ['asset.view', 'asset.manage'] } })
    expect(role.id).toBe('new1')
  })

  it('createRole without copyFromId only posts', async () => {
    request.mockResolvedValueOnce({ id: 'new2', code: 'gudang', name: 'Gudang', is_system: false })
    const role = await useRbac().createRole({ name: 'Gudang' })
    expect(request).toHaveBeenCalledTimes(1)
    expect(role.code).toBe('gudang')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-rbac`
Expected: FAIL — `slugifyRoleCode`/new `useRbac` shape undefined.

- [ ] **Step 3: Rewrite `useRbac.ts`**

Replace `frontend/app/composables/api/useRbac.ts` entirely with:

```ts
import { iconForGroup } from '~/constants/authzCatalog'

export interface PermissionView { code: string; label: string }
export interface ModuleView { key: string; label: string; icon: string; perms: PermissionView[] }
export interface RoleView { id: string; code: string; name: string; is_system: boolean; description?: string; perms: string[] }
export interface CreateRoleInput { name: string; description?: string; copyFromId?: string }

interface CatalogResponse {
  permissions: { group: string; items: { key: string; label: string }[] }[]
}
interface RoleDTO { id: string; code: string; name: string; is_system: boolean; description?: string }

// slugifyRoleCode derives a backend role `code` from a human name:
// lowercase, runs of non-alphanumerics collapse to a single '_', trimmed.
export function slugifyRoleCode(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
}

/**
 * RBAC data source, wired to /api/v1/authz. The catalog supplies the
 * authoritative permission key set + grouping; display labels are resolved by
 * the UI via i18n (with fallback to the catalog label), icons via iconForGroup.
 */
export function useRbac() {
  const { request } = useApiClient()

  async function getCatalog(): Promise<ModuleView[]> {
    const cat = await request<CatalogResponse>('/authz/catalog')
    return cat.permissions.map(g => ({
      key: g.group,
      label: g.group,
      icon: iconForGroup(g.group),
      perms: g.items.map(i => ({ code: i.key, label: i.label }))
    }))
  }

  async function listRoles(): Promise<RoleView[]> {
    const res = await request<{ data: RoleDTO[]; total: number }>('/authz/roles')
    return res.data.map(r => ({ ...r, perms: [] }))
  }

  async function getRolePermissions(id: string): Promise<string[]> {
    const res = await request<{ permissions: string[] }>(`/authz/roles/${id}/permissions`)
    return res.permissions
  }

  async function updateRolePermissions(id: string, perms: string[]): Promise<void> {
    await request(`/authz/roles/${id}/permissions`, { method: 'PUT', body: { permissions: perms } })
  }

  async function createRole(input: CreateRoleInput): Promise<RoleView> {
    let copied: string[] = []
    if (input.copyFromId) copied = await getRolePermissions(input.copyFromId)
    const role = await request<RoleDTO>('/authz/roles', {
      method: 'POST',
      body: { code: slugifyRoleCode(input.name), name: input.name.trim(), description: input.description?.trim() || undefined }
    })
    if (copied.length) await updateRolePermissions(role.id, copied)
    return { ...role, perms: copied }
  }

  return { getCatalog, listRoles, getRolePermissions, createRole, updateRolePermissions }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run (from `frontend/`): `pnpm test -- use-rbac && pnpm typecheck`
Expected: PASS, typecheck clean. (`rbac-mock.spec.ts` was deleted; `pnpm test` should not reference it.)

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useRbac.ts frontend/test/unit/use-rbac.spec.ts
git rm frontend/test/unit/rbac-mock.spec.ts
git commit -m "feat(rbac): wire useRbac to /authz API (English DTO, id identity)"
```

---

### Task 3: Update the RBAC page + role-list component

**Files:**
- Modify (script + template bindings): `frontend/app/pages/settings/rbac.vue`
- Modify: `frontend/app/components/rbac/RbacRoleList.vue`
- Modify: `frontend/app/components/rbac/RbacPermissionCard.vue` (resolve catalog labels via i18n with fallback — Step 3b)

**Interfaces:**
- Consumes: `useRbac()` (Task 2) + `RoleView`/`ModuleView` (English keys, id identity).

- [ ] **Step 1: Rewrite the page `<script setup>`**

Replace the `<script setup>` block of `frontend/app/pages/settings/rbac.vue` with:

```ts
import type { RoleView, ModuleView } from '~/composables/api/useRbac'
import { useRbac } from '~/composables/api/useRbac'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

const { t } = useI18n()
const toast = useToast()
const { getCatalog, listRoles, getRolePermissions, createRole, updateRolePermissions } = useRbac()

const roles = ref<RoleView[]>([])
const modules = ref<ModuleView[]>([])
const selectedId = ref('')
const draft = ref<string[]>([])
const dirty = ref(false)
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)

const selectedRole = computed(() => roles.value.find(r => r.id === selectedId.value))
const saveDisabled = computed(() => !dirty.value)

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const [mods, roleList] = await Promise.all([getCatalog(), listRoles()])
    modules.value = mods
    // Eager-load each role's permissions (parallel) so the list count + matrix are populated.
    const permsList = await Promise.all(roleList.map(r => getRolePermissions(r.id)))
    roleList.forEach((r, i) => { r.perms = permsList[i] ?? [] })
    roles.value = roleList
    if (!selectedId.value || !roles.value.some(r => r.id === selectedId.value)) {
      const mgr = roles.value.find(r => r.code === 'manager')
      selectedId.value = mgr?.id ?? roles.value[0]?.id ?? ''
    }
    draft.value = [...(selectedRole.value?.perms ?? [])]
    dirty.value = false
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function selectRole(id: string) {
  selectedId.value = id
  draft.value = [...(roles.value.find(r => r.id === id)?.perms ?? [])]
  dirty.value = false
}

function togglePerm(code: string) {
  draft.value = draft.value.includes(code) ? draft.value.filter(c => c !== code) : [...draft.value, code]
  dirty.value = true
}

function toggleModule(modKey: string) {
  const mod = modules.value.find(m => m.key === modKey)
  if (!mod) return
  const ids = mod.perms.map(p => p.code)
  const allOn = ids.every(id => draft.value.includes(id))
  draft.value = allOn ? draft.value.filter(c => !ids.includes(c)) : [...new Set([...draft.value, ...ids])]
  dirty.value = true
}

async function save() {
  if (saveDisabled.value) return
  saving.value = true
  try {
    await updateRolePermissions(selectedId.value, draft.value)
    const r = roles.value.find(x => x.id === selectedId.value)
    if (r) r.perms = [...draft.value]
    dirty.value = false
    toast.add({ title: t('settings.rbac.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

// Add Role modal. NO_COPY sentinel — Nuxt UI Select rejects empty-string values.
const NO_COPY = '__none__'
const addOpen = ref(false)
const addForm = reactive({ name: '', copyFromId: NO_COPY, desc: '' })
const addError = ref('')
const creating = ref(false)

const copyOptions = computed(() => [
  { value: NO_COPY, label: t('settings.rbac.add.copyNone') },
  ...roles.value.map(r => ({ value: r.id, label: r.name }))
])

function openAdd() {
  addForm.name = ''
  addForm.copyFromId = NO_COPY
  addForm.desc = ''
  addError.value = ''
  addOpen.value = true
}

async function submitAdd() {
  if (!addForm.name.trim()) { addError.value = t('settings.rbac.add.required'); return }
  creating.value = true
  try {
    const created = await createRole({
      name: addForm.name,
      copyFromId: addForm.copyFromId !== NO_COPY ? addForm.copyFromId : undefined,
      description: addForm.desc
    })
    roles.value.push(created)
    selectRole(created.id)
    addOpen.value = false
    toast.add({ title: t('settings.rbac.add.createdToast'), color: 'success', icon: 'i-lucide-plus' })
  } catch (err: unknown) {
    // 409 = duplicate code/name -> inline form error instead of a generic toast.
    if ((err as { statusCode?: number }).statusCode === 409) addError.value = t('settings.rbac.add.conflict')
    else addError.value = t('settings.rbac.loadError')
  } finally {
    creating.value = false
  }
}

onMounted(() => load())
```

Note the deliberate changes vs the old script: `selectedKey`→`selectedId`; role identity via `id`; default selection by `code === 'manager'`; eager parallel permission load (preserves the per-role count in the list); the `locale` watch is removed because labels now come from i18n reactively in components (re-render on locale change happens via `t()` in templates — the role/permission DATA no longer depends on locale); `saveDisabled` no longer blocks system roles (their perms are editable); add-role uses `name`/`copyFromId`/`description` and maps `409`→inline `conflict` error.

- [ ] **Step 2: Update the page template bindings**

In the same file's `<template>`, change the role-name/badge/desc bindings and the role-list props to the new keys:
- `selectedRole?.nama` → `selectedRole?.name`
- `selectedRole?.system` → `selectedRole?.is_system` (all 3 occurrences: badge `v-if`, lock-note `v-if`, and the `:readonly` on `RbacPermissionCard`)
- `selectedRole?.desc` → `selectedRole?.description`
- `<RbacRoleList :selected-key="selectedKey" ...>` → `:selected-id="selectedId"`
- Add a load-error state: directly after the `v-if="loading"` block, add an error block shown when `!loading && loadFailed`:

```vue
    <div
      v-else-if="loadFailed"
      class="flex-1 flex flex-col items-center justify-center gap-3 text-muted"
    >
      <UIcon name="i-lucide-circle-alert" class="size-6" />
      <span class="text-sm">{{ t('settings.rbac.loadError') }}</span>
      <UButton color="neutral" variant="subtle" @click="load">
        {{ t('settings.rbac.retry') }}
      </UButton>
    </div>

    <template v-else>
```
(Change the existing `<template v-else>` so the populated UI shows only when not loading AND not failed — i.e. it now follows the `v-else-if="loadFailed"` block.)

- [ ] **Step 3: Update `RbacRoleList.vue`**

In `frontend/app/components/rbac/RbacRoleList.vue`:
- Props: `selectedKey: string` → `selectedId: string`.
- Emit: `select: [key: string]` → `select: [id: string]`.
- Template: `:key="r.key"` → `:key="r.id"`; every `r.key === selectedKey` → `r.id === selectedId`; `@click="$emit('select', r.key)"` → `$emit('select', r.id)`; `{{ r.nama }}` → `{{ r.name }}`; `v-if="r.system"` → `v-if="r.is_system"`. (`r.perms.length` stays — perms are eager-loaded.)

- [ ] **Step 3b: Resolve catalog labels via i18n (with fallback) in `RbacPermissionCard.vue`**

The catalog API supplies labels in Indonesian only; the card must show the i18n label for the active locale and fall back to the catalog label for unknown keys. In `frontend/app/components/rbac/RbacPermissionCard.vue`, replace the `useI18n()` line and add two helpers in `<script setup>`:

```ts
const { t, te } = useI18n()
function permLabel(code: string, fallback: string) { const k = `settings.rbac.catalog.perm.${code}`; return te(k) ? t(k) : fallback }
function groupLabel(key: string, fallback: string) { const k = `settings.rbac.catalog.group.${key}`; return te(k) ? t(k) : fallback }
```

In the template, change the header label binding `{{ module.label }}` → `{{ groupLabel(module.key, module.label) }}`, and the per-row label `{{ p.label }}` → `{{ permLabel(p.code, p.label) }}`. (`{{ p.code }}` mono line stays as the raw key.) This keeps `RbacPermissionCard` the single place catalog labels are rendered, so the page header card is unaffected.

- [ ] **Step 4: Verify build/lint/typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: exit 0. (Component tests are updated in Task 4.)

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/settings/rbac.vue frontend/app/components/rbac/RbacRoleList.vue frontend/app/components/rbac/RbacPermissionCard.vue
git commit -m "feat(rbac): page + role list on real API (id identity, eager perms, load error)"
```

---

### Task 4: Nuxt component test for the wired page

**Files:**
- Modify (rewrite): `frontend/test/nuxt/settings-rbac.spec.ts`

**Interfaces:**
- Consumes: `registerEndpoint` from `@nuxt/test-utils/runtime` to stub `/authz/*`; `mountSuspended`; `useAuthStore` to grant `['*']`.

- [ ] **Step 1: Study the existing test + the auth/endpoint pattern**

Read the current `frontend/test/nuxt/settings-rbac.spec.ts` (store-reset + `setSession(['*'])` + `mountSuspended` pattern) and find one existing Nuxt test that stubs HTTP via `registerEndpoint` (search `frontend/test/` for `registerEndpoint`). The wired page calls real endpoints, so the test must register handlers for: `GET /api/v1/authz/catalog`, `GET /api/v1/authz/roles`, `GET /api/v1/authz/roles/:id/permissions`, `PUT /api/v1/authz/roles/:id/permissions`, `POST /api/v1/authz/roles`. The base path is `runtimeConfig.public.apiBase` (`http://localhost:8080/api/v1`); `registerEndpoint` matches on the path — confirm whether to register `/api/v1/authz/...` or the full URL by following the existing example.

- [ ] **Step 2: Write the rewritten test**

Rewrite `frontend/test/nuxt/settings-rbac.spec.ts` to register stub endpoints and assert real behavior. Cover:
- Loaded state: catalog renders module cards + permission labels (resolved via i18n, e.g. "Lihat aset" / "View assets"); role list renders seeded roles ("Superadmin", "Manager") with per-role permission counts.
- Default selection is the role with `code === 'manager'`.
- Toggling a permission marks dirty and enables Save; Save issues `PUT /authz/roles/:id/permissions` with the updated set (assert via a spy/among registered handler calls) and clears dirty.
- System role: lock badge + note shown, but toggling a permission still works (perms editable).
- Add Role: open modal, submit name → `POST /authz/roles` with derived `code`; on a stubbed `409` the inline `add.conflict` error renders.
- Load-error state: when `GET /authz/roles` stub returns a 500, the error block + retry button render.

Assert real rendered text and emitted requests — no hollow `html.length` checks. Use the locale the harness defaults to and assert the resolved i18n string accordingly.

- [ ] **Step 3: Run the test**

Run (from `frontend/`): `pnpm test -- settings-rbac`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/test/nuxt/settings-rbac.spec.ts
git commit -m "test(rbac): component test against stubbed /authz endpoints"
```

---

### Task 5: E2E + mockup fidelity + PROGRESS + full gate

**Files:**
- Modify: `frontend/e2e/settings.spec.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Update the e2e RBAC assertions**

Read `frontend/e2e/settings.spec.ts` + `frontend/e2e/helpers.ts` (`login()`). The RBAC e2e runs against the real backend with a seeded admin. Update the `/settings/rbac` spec to assert real seeded data: the role list shows built-in roles (e.g. "Superadmin", "Manager"), selecting a role shows its permission matrix with resolved labels, and toggling a permission + Save persists (reload the page → the toggle state remains). Keep the existing audit-trail spec untouched.

- [ ] **Step 2: Mockup 1:1 comparison**

Open the built `/settings/rbac` screen and `docs/design/Peran RBAC.dc.html` side by side (the `.dc.html` renders standalone in a browser). Verify layout, spacing, the role list, permission cards, system lock note, dirty indicator, Add Role modal, and every state (loading/error/populated) match. Fix any deviation in `rbac.vue`/`RbacRoleList.vue`/`RbacPermissionCard.vue`. Report the comparison result.

- [ ] **Step 3: Update PROGRESS.md**

In `docs/PROGRESS.md`, under the frontend "Wire screens to real backend APIs" remaining item, add a sub-note that **RBAC (Peran & RBAC) is now wired to `/authz`** (catalog + roles + role-permissions; English DTO; id identity). Refresh the "▶ Next session — start here" block to point at **Data Scope** as the next screen to wire. Do not invent status for the other unscoped screens.

- [ ] **Step 4: Full frontend gate**

Run (from `frontend/`):
```
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```
Expected: all exit 0 / green. (E2E `pnpm test:e2e` needs the backend stack up + seeded admin; run if available, else note it runs in CI's e2e job.)

- [ ] **Step 5: Commit**

```bash
git add frontend/e2e/settings.spec.ts docs/PROGRESS.md
git commit -m "test(rbac): e2e against real backend + progress update"
```

---

## Self-Review

**Spec coverage:**
- bagian 2 composable rewrite (English keys, id, getCatalog/listRoles/getRolePermissions/createRole/updateRolePermissions) → Task 2. ✓
- bagian 3 labels/icons (frontend i18n + icon map, fallback to catalog label) → Task 1 (constants+i18n) + Task 2 (getCatalog uses catalog label; component resolves i18n with fallback — note: components display `p.label`/`m.label` which carry the catalog label; the i18n override is applied in the card/label resolution). ⚠️ See note below.
- bagian 4 create role (auto-derive code via slugify; 409 inline; client-side copy-from) → Task 2 (slugify+copy) + Task 3 (409 inline). ✓
- bagian 5 permissions load/save (eager parallel; PUT replace; system roles editable) → Task 3. ✓
- bagian 6 states/i18n/error → Task 1 (i18n) + Task 3 (loading/error/retry). ✓
- bagian 7 tests (unit/component/e2e) → Tasks 2,4,5. ✓
- bagian 8 done (gate, mockup compare, PROGRESS) → Task 5. ✓

**Resolved ambiguity (label i18n vs catalog label):** The composable's `getCatalog` returns `perms[].label` = the catalog API label (Indonesian) and `m.label` = the group string. To honor i18n with fallback (bagian 3), the DISPLAY label must be resolved in the components via `t('settings.rbac.catalog.perm.<code>')` with fallback to the carried catalog label, and `t('settings.rbac.catalog.group.<key>')` for the group. **Add to Task 3** a small change in `RbacPermissionCard.vue` and `rbac.vue`'s card header: resolve labels through i18n with fallback. Concretely, in `RbacPermissionCard.vue` use a helper:
```ts
const { t, te } = useI18n()
function permLabel(code: string, fallback: string) { const k = `settings.rbac.catalog.perm.${code}`; return te(k) ? t(k) : fallback }
function groupLabel(key: string, fallback: string) { const k = `settings.rbac.catalog.group.${key}`; return te(k) ? t(k) : fallback }
```
and bind `{{ groupLabel(module.key, module.label) }}` for the header and `{{ permLabel(p.code, p.label) }}` per row. This keeps `RbacPermissionCard` as the single place that renders catalog labels. **This makes RbacPermissionCard.vue an edited file in Task 3** (correcting the Task-3 "no change" note). Add it to Task 3's files + commit.

**Placeholder scan:** Tasks 4 & 5 give explicit assertion lists / comparison steps rather than full literal test code (the endpoint-stub API differs across the repo's test helpers, so Step 1 of each says to read the existing pattern first). These are concrete checklists, not "TODO"s.

**Type consistency:** `RoleView{id,code,name,is_system,description?,perms}` and `ModuleView{key,label,icon,perms:[{code,label}]}` are used identically across Tasks 2/3/4. `selectedId`, `selectRole(id)`, `select:[id]` consistent page↔RbacRoleList. `slugifyRoleCode`, `getCatalog`, `getRolePermissions`, `createRole({name,description,copyFromId})`, `updateRolePermissions(id,perms)` consistent Tasks 2↔3↔4.
