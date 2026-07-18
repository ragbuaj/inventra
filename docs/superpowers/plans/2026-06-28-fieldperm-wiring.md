# Wire Field Permission screen to `/authz` API — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mock-backed `useFieldPermission` composable + Field Permission settings screen with the real `/api/v1/authz` backend (per-role field policies), using a frontend field catalog of the real backend-enforced `(entity, field)` keys and UUID role identity.

**Architecture:** `useFieldPermission` is rewritten to call `useApiClient` against `/authz/roles` + `/authz/roles/:id/fields`. A frontend `FIELD_CATALOG` constant lists the real maskable entities (`assets`, `users`) and their real serialization field keys. The composable holds each role's full field rows; the grid for the selected entity is derived (restriction cells only; absence = default-allow), and saving an entity replaces each changed role's full field set while preserving other entities' rules.

**Tech Stack:** Nuxt 4 (SPA), Nuxt UI (`U*`), `@nuxtjs/i18n` (id default + en), Vitest + `@nuxt/test-utils`, Playwright e2e.

## Global Constraints

- Wire ONLY the Field Permission screen: `pages/settings/field-permission.vue`, `composables/api/useFieldPermission.ts`, new `constants/fieldCatalog.ts`. `components/fieldperm/FieldPermToggle.vue` needs NO change (already mock-free). Do NOT touch other screens.
- Entities = `assets` + `users` ONLY (the only entities the backend `FilterView`-enforces today). Fields = real serialization keys (English), per the catalog in Task 1.
- Role columns are dynamic from `GET /authz/roles` (UUID `id` + `name`).
- Default-allow: a cell with no stored policy = `{view:true, edit:true}`. Persist ONLY restriction cells (`can_view=false` OR `can_edit=false`); full-allow cells are omitted.
- Save = per-role replace-set across ALL entities: preserve a role's other-entity rows verbatim; replace only the selected entity's rows with its restriction cells. PUT only roles whose selected-entity rows changed.
- All API calls via `useApiClient().request<T>`; never hardcode the backend URL.
- i18n mandatory: entity labels (`settings.fieldPermission.entity.<key>`) + field labels (`settings.fieldPermission.field.<field>`, flat) resolve via `te()/t()` with fallback to the raw key.
- Match `docs/design/Field Permission.dc.html` (layout/grid/toggle/states); the entity + field SET intentionally differs (real backend keys) per the approved decision.
- PROGRESS.md MUST record the enforcement TODO (extend `FilterView` to `requests`/`employees`/other modules — only `assets`+`users` are field-masked today).
- Gates: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green. Run from `frontend/`.

---

### Task 1: Field catalog constant + i18n labels

**Files:**
- Create: `frontend/app/constants/fieldCatalog.ts`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`
- Test: `frontend/test/unit/field-catalog.spec.ts`

**Interfaces:**
- Produces: `interface CellRule { view: boolean; edit: boolean }`; `interface CatalogEntity { entity: string; fields: string[] }`; `const FIELD_CATALOG: CatalogEntity[]`.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/field-catalog.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { FIELD_CATALOG } from '~/constants/fieldCatalog'

describe('FIELD_CATALOG', () => {
  it('lists the real backend-enforced entities', () => {
    expect(FIELD_CATALOG.map(e => e.entity)).toEqual(['assets', 'users'])
  })
  it('uses real serialization field keys (no Indonesian mock codes)', () => {
    const assets = FIELD_CATALOG.find(e => e.entity === 'assets')!
    expect(assets.fields).toContain('purchase_cost')
    expect(assets.fields).toContain('book_value')
    expect(assets.fields).not.toContain('harga_beli')
    const users = FIELD_CATALOG.find(e => e.entity === 'users')!
    expect(users.fields).toContain('email')
  })
  it('has no duplicate fields within an entity', () => {
    for (const e of FIELD_CATALOG) {
      expect(new Set(e.fields).size).toBe(e.fields.length)
    }
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- field-catalog`
Expected: FAIL — cannot resolve `~/constants/fieldCatalog`.

- [ ] **Step 3: Create the constants file**

Create `frontend/app/constants/fieldCatalog.ts`:

```ts
// Frontend catalog of the (entity, field) pairs the backend actually field-masks
// via authz FilterView. Field keys are the real serialization keys from the
// backend's response maps (assetToMap / userToMap) — rules on any other key would
// have no effect. Entity-agnostic: the screen renders whatever is listed here, so
// adding an entity later is a constant edit + a one-line FilterView call in that
// entity's handler.
export interface CellRule { view: boolean; edit: boolean }
export interface CatalogEntity { entity: string; fields: string[] }

export const FIELD_CATALOG: CatalogEntity[] = [
  {
    entity: 'assets',
    fields: [
      'name', 'category_id', 'office_id', 'serial_number', 'purchase_date',
      'purchase_cost', 'book_value', 'accumulated_depreciation', 'salvage_value', 'impairment_loss',
      'depreciation_method', 'po_number', 'funding_source', 'warranty_expiry', 'status', 'notes'
    ]
  },
  {
    entity: 'users',
    fields: ['name', 'email', 'role_id', 'office_id', 'employee_id', 'status']
  }
]
```

- [ ] **Step 4: Add i18n keys**

In `frontend/i18n/locales/id.json` and `en.json`, under the existing `settings.fieldPermission` object, add `entity`, `field`, `loadError`, `retry`. Read the `settings.fieldPermission` section of each file first; insert as valid JSON.

**id.json** `settings.fieldPermission`:
```json
"entity": { "assets": "Aset", "users": "User" },
"field": {
  "name": "Nama", "category_id": "Kategori", "office_id": "Kantor", "serial_number": "Nomor seri",
  "purchase_date": "Tanggal beli", "purchase_cost": "Harga beli", "book_value": "Nilai buku",
  "accumulated_depreciation": "Akumulasi penyusutan", "salvage_value": "Nilai residu",
  "impairment_loss": "Rugi penurunan nilai", "depreciation_method": "Metode penyusutan",
  "po_number": "Nomor PO", "funding_source": "Sumber dana", "warranty_expiry": "Akhir garansi",
  "status": "Status", "notes": "Catatan", "email": "Email", "role_id": "Peran", "employee_id": "Pegawai"
},
"loadError": "Gagal memuat field permission.",
"retry": "Coba lagi"
```

**en.json** `settings.fieldPermission`:
```json
"entity": { "assets": "Assets", "users": "User" },
"field": {
  "name": "Name", "category_id": "Category", "office_id": "Office", "serial_number": "Serial number",
  "purchase_date": "Purchase date", "purchase_cost": "Purchase cost", "book_value": "Book value",
  "accumulated_depreciation": "Accumulated depreciation", "salvage_value": "Salvage value",
  "impairment_loss": "Impairment loss", "depreciation_method": "Depreciation method",
  "po_number": "PO number", "funding_source": "Funding source", "warranty_expiry": "Warranty expiry",
  "status": "Status", "notes": "Notes", "email": "Email", "role_id": "Role", "employee_id": "Employee"
},
"loadError": "Failed to load field permissions.",
"retry": "Retry"
```

- [ ] **Step 5: Run test + lint**

Run (from `frontend/`): `pnpm test -- field-catalog && pnpm lint`
Expected: PASS, lint clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/constants/fieldCatalog.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/unit/field-catalog.spec.ts
git commit -m "feat(fieldperm): field catalog constant + i18n labels"
```

---

### Task 2: Rewrite `useFieldPermission` to the real API

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useFieldPermission.ts`
- Delete: `frontend/test/unit/field-permission-mock.spec.ts`
- Test: `frontend/test/unit/use-field-permission.spec.ts`

**Interfaces:**
- Consumes: `useApiClient().request`; `FIELD_CATALOG`, `CellRule` (Task 1).
- Produces: types `EntityView{key,fields:string[]}`, `RoleColumn{key,label}`, `FieldRow{entity,field,can_view,can_edit}`, `EntityRules = Record<string, Record<string, CellRule>>`; pure helpers `deriveEntityRules(roleFields, entity)`, `buildRoleRows(existing, entity, roleId, rules)`, `entityRowsEqual(rows, entity, rules, roleId)`; `useFieldPermission()` → `{ getEntities, load, getRules, saveRules }`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/test/unit/use-field-permission.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

import {
  useFieldPermission, deriveEntityRules, buildRoleRows, entityRowsEqual
} from '~/composables/api/useFieldPermission'

beforeEach(() => request.mockReset())

describe('pure helpers', () => {
  const roleFields = {
    r1: [
      { entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false },
      { entity: 'users', field: 'email', can_view: true, can_edit: false }
    ],
    r2: []
  }

  it('deriveEntityRules keeps only the entity, as field→role→rule', () => {
    expect(deriveEntityRules(roleFields, 'assets')).toEqual({
      purchase_cost: { r1: { view: false, edit: false } }
    })
    expect(deriveEntityRules(roleFields, 'users')).toEqual({
      email: { r1: { view: true, edit: false } }
    })
  })

  it('buildRoleRows preserves other entities + keeps only restriction cells of the target entity', () => {
    const rules = { purchase_cost: { r1: { view: true, edit: true } }, book_value: { r1: { view: false, edit: false } } }
    const rows = buildRoleRows(roleFields.r1, 'assets', 'r1', rules)
    // users/email (other entity) preserved; purchase_cost is full-allow → dropped; book_value restriction kept
    expect(rows).toContainEqual({ entity: 'users', field: 'email', can_view: true, can_edit: false })
    expect(rows).toContainEqual({ entity: 'assets', field: 'book_value', can_view: false, can_edit: false })
    expect(rows.find(r => r.entity === 'assets' && r.field === 'purchase_cost')).toBeUndefined()
  })

  it('entityRowsEqual detects changes for the target entity only', () => {
    const same = { purchase_cost: { r1: { view: false, edit: false } } }
    expect(entityRowsEqual(roleFields.r1, 'assets', same, 'r1')).toBe(true)
    const changed = { purchase_cost: { r1: { view: true, edit: false } } }
    expect(entityRowsEqual(roleFields.r1, 'assets', changed, 'r1')).toBe(false)
  })
})

describe('useFieldPermission', () => {
  it('getEntities comes from the catalog', () => {
    const ents = useFieldPermission().getEntities()
    expect(ents.map(e => e.key)).toEqual(['assets', 'users'])
  })

  it('load fetches roles then each role fields; getRules derives restrictions', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }], total: 1 })
      .mockResolvedValueOnce({ fields: [{ entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false }] })
    const fp = useFieldPermission()
    const cols = await fp.load()
    expect(request).toHaveBeenNthCalledWith(1, '/authz/roles')
    expect(request).toHaveBeenNthCalledWith(2, '/authz/roles/r1/fields')
    expect(cols).toEqual([{ key: 'r1', label: 'Manager' }])
    expect(fp.getRules('assets')).toEqual({ purchase_cost: { r1: { view: false, edit: false } } })
  })

  it('saveRules PUTs only changed roles with reconstructed full rows', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }], total: 1 })
      .mockResolvedValueOnce({ fields: [{ entity: 'users', field: 'email', can_view: false, can_edit: false }] })
    const fp = useFieldPermission()
    await fp.load()
    request.mockClear()
    request.mockResolvedValueOnce({ fields: [] })
    // add an assets restriction; users/email must be preserved in the PUT body
    await fp.saveRules('assets', { book_value: { r1: { view: false, edit: false } } }, ['r1'])
    expect(request).toHaveBeenCalledTimes(1)
    const [path, opts] = request.mock.calls[0]
    expect(path).toBe('/authz/roles/r1/fields')
    expect(opts.method).toBe('PUT')
    expect(opts.body.fields).toContainEqual({ entity: 'users', field: 'email', can_view: false, can_edit: false })
    expect(opts.body.fields).toContainEqual({ entity: 'assets', field: 'book_value', can_view: false, can_edit: false })
  })

  it('saveRules PUTs nothing when the entity is unchanged', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }], total: 1 })
      .mockResolvedValueOnce({ fields: [{ entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false }] })
    const fp = useFieldPermission()
    await fp.load()
    request.mockClear()
    await fp.saveRules('assets', { purchase_cost: { r1: { view: false, edit: false } } }, ['r1'])
    expect(request).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-field-permission`
Expected: FAIL — new `useFieldPermission` shape / helpers undefined.

- [ ] **Step 3: Rewrite `useFieldPermission.ts`**

Replace `frontend/app/composables/api/useFieldPermission.ts` entirely with:

```ts
import { FIELD_CATALOG } from '~/constants/fieldCatalog'
import type { CellRule } from '~/constants/fieldCatalog'

export interface EntityView { key: string; fields: string[] }
export interface RoleColumn { key: string; label: string }
export interface FieldRow { entity: string; field: string; can_view: boolean; can_edit: boolean }
export type EntityRules = Record<string, Record<string, CellRule>>

interface RoleDTO { id: string; name: string }

// Derive an entity's restriction cells (field → roleId → rule) from all roles' rows.
export function deriveEntityRules(roleFields: Record<string, FieldRow[]>, entity: string): EntityRules {
  const out: EntityRules = {}
  for (const [roleId, rows] of Object.entries(roleFields)) {
    for (const r of rows) {
      if (r.entity !== entity) continue
      ;(out[r.field] ??= {})[roleId] = { view: r.can_view, edit: r.can_edit }
    }
  }
  return out
}

// Build a role's full field rows for a save: keep other-entity rows verbatim, then
// append only the RESTRICTION cells (not full-allow) of the target entity from `rules`.
export function buildRoleRows(existing: FieldRow[], entity: string, roleId: string, rules: EntityRules): FieldRow[] {
  const others = existing.filter(r => r.entity !== entity)
  const eRows: FieldRow[] = []
  for (const [field, perRole] of Object.entries(rules)) {
    const cr = perRole[roleId]
    if (cr && !(cr.view && cr.edit)) eRows.push({ entity, field, can_view: cr.view, can_edit: cr.edit })
  }
  return [...others, ...eRows]
}

// Order-insensitive comparison of a role's target-entity rows vs the edited `rules`.
export function entityRowsEqual(rows: FieldRow[], entity: string, rules: EntityRules, roleId: string): boolean {
  const cur = rows.filter(r => r.entity === entity)
  const next = buildRoleRows([], entity, roleId, rules)
  if (cur.length !== next.length) return false
  const key = (r: FieldRow) => `${r.field}:${r.can_view}:${r.can_edit}`
  const cs = new Set(cur.map(key))
  return next.every(r => cs.has(key(r)))
}

/**
 * Field-permission rules, wired to /api/v1/authz. The catalog supplies the
 * maskable (entity, field) keys; each role's policies come from
 * /authz/roles/:id/fields. Default-allow: a cell with no stored policy is
 * view+edit; only restriction cells are persisted.
 */
export function useFieldPermission() {
  const { request } = useApiClient()
  let roleFields: Record<string, FieldRow[]> = {}

  function getEntities(): EntityView[] {
    return FIELD_CATALOG.map(e => ({ key: e.entity, fields: [...e.fields] }))
  }

  async function load(): Promise<RoleColumn[]> {
    const res = await request<{ data: RoleDTO[]; total: number }>('/authz/roles')
    const cols = res.data.map(r => ({ key: r.id, label: r.name }))
    const entries = await Promise.all(cols.map(async (c) => {
      const r = await request<{ fields: FieldRow[] }>(`/authz/roles/${c.key}/fields`)
      return [c.key, r.fields] as const
    }))
    roleFields = Object.fromEntries(entries)
    return cols
  }

  function getRules(entity: string): EntityRules {
    return deriveEntityRules(roleFields, entity)
  }

  async function saveRules(entity: string, rules: EntityRules, roleIds: string[]): Promise<void> {
    const changed = roleIds.filter(id => !entityRowsEqual(roleFields[id] ?? [], entity, rules, id))
    await Promise.all(changed.map((id) => {
      const next = buildRoleRows(roleFields[id] ?? [], entity, id, rules)
      return request(`/authz/roles/${id}/fields`, { method: 'PUT', body: { fields: next } })
        .then(() => { roleFields[id] = next })
    }))
  }

  return { getEntities, load, getRules, saveRules }
}
```

- [ ] **Step 4: Run tests + lint**

Run (from `frontend/`): `pnpm test -- use-field-permission && pnpm lint`
Expected: PASS, lint clean. NOTE: `pnpm typecheck` will fail ONLY in `pages/settings/field-permission.vue` (old shape / `~/mock/fieldPermission`) — EXPECTED, fixed in Task 3. Do NOT edit the page here.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useFieldPermission.ts frontend/test/unit/use-field-permission.spec.ts
git rm frontend/test/unit/field-permission-mock.spec.ts
git commit -m "feat(fieldperm): wire useFieldPermission to /authz API (catalog + per-role fields)"
```

---

### Task 3: Update the page

**Files:**
- Modify (script + template): `frontend/app/pages/settings/field-permission.vue`
- (`FieldPermToggle.vue` needs NO change.)

**Interfaces:**
- Consumes: `useFieldPermission()` (`getEntities/load/getRules/saveRules`), `RoleColumn`, `EntityRules` (Task 2); `CellRule` (Task 1).

- [ ] **Step 1: Rewrite the page `<script setup>`**

Replace the `<script setup>` block of `frontend/app/pages/settings/field-permission.vue` with:

```ts
import type { RoleColumn, EntityRules } from '~/composables/api/useFieldPermission'
import { useFieldPermission } from '~/composables/api/useFieldPermission'
import type { CellRule } from '~/constants/fieldCatalog'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

const { t, te } = useI18n()
const toast = useToast()
const { getEntities, load, getRules, saveRules } = useFieldPermission()

const entities = getEntities()                 // [{ key, fields }]
const roleCols = ref<RoleColumn[]>([])
const entityKey = ref(entities[0]?.key ?? 'assets')
const rules = ref<EntityRules>({})
const search = ref('')
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const dirty = ref(false)

function entityLabel(key: string): string {
  const k = `settings.fieldPermission.entity.${key}`
  return te(k) ? t(k) : key
}
function fieldLabel(field: string): string {
  const k = `settings.fieldPermission.field.${field}`
  return te(k) ? t(k) : field
}

const entityOptions = computed(() => entities.map(e => ({ value: e.key, label: entityLabel(e.key) })))
const currentEntity = computed(() => entities.find(e => e.key === entityKey.value))

const filteredFields = computed(() => {
  const q = search.value.trim().toLowerCase()
  return (currentEntity.value?.fields ?? []).filter(f => !q || f.toLowerCase().includes(q) || fieldLabel(f).toLowerCase().includes(q))
})

function isExplicit(field: string): boolean {
  return !!rules.value[field]
}
// Default-allow: a cell with no explicit restriction is view+edit.
function cell(field: string, roleId: string): CellRule {
  const fr = rules.value[field]
  if (fr && fr[roleId]) return fr[roleId]
  return { view: true, edit: true }
}
function ensure(field: string) {
  if (rules.value[field]) return
  const fr: Record<string, CellRule> = {}
  for (const c of roleCols.value) fr[c.key] = { view: true, edit: true }
  rules.value = { ...rules.value, [field]: fr }
}
function toggleView(field: string, roleId: string) {
  ensure(field)
  const fr = rules.value[field]
  if (!fr) return
  const cur: CellRule = { ...(fr[roleId] ?? { view: false, edit: false }) }
  cur.view = !cur.view
  if (!cur.view) cur.edit = false
  fr[roleId] = cur
  dirty.value = true
}
function toggleEdit(field: string, roleId: string) {
  ensure(field)
  const fr = rules.value[field]
  if (!fr) return
  const cur: CellRule = { ...(fr[roleId] ?? { view: false, edit: false }) }
  cur.edit = !cur.edit
  if (cur.edit) cur.view = true
  fr[roleId] = cur
  dirty.value = true
}
function resetField(field: string) {
  const { [field]: _omit, ...rest } = rules.value
  rules.value = rest
  dirty.value = true
}

function refreshRules() {
  rules.value = getRules(entityKey.value)
  dirty.value = false
}

async function load_() {
  loading.value = true
  loadFailed.value = false
  try {
    roleCols.value = await load()
    refreshRules()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function onEntityChange() {
  search.value = ''
  refreshRules()
}

async function save() {
  if (!dirty.value) return
  saving.value = true
  try {
    await saveRules(entityKey.value, rules.value, roleCols.value.map(c => c.key))
    refreshRules()
    toast.add({ title: t('settings.fieldPermission.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

onMounted(() => load_())
```

Notes vs the old script: entities come from the catalog (sync); `roleCols` is loaded async from `/authz/roles`; `entityKey`/`field` are real keys; `cell()` is default-allow (returns view+edit when no explicit restriction for that role — the critical fix vs the old mock which assumed a full per-role map); `ensure` iterates real role ids; `save()` passes all role ids and the composable PUTs only changed roles; `loadFailed` + retry added; the `locale` watch is gone (labels come from i18n functions, reactive in the template). After save, `refreshRules()` re-derives from the updated backend state.

- [ ] **Step 2: Update the page template**

In the `<template>` of `field-permission.vue`:
- Entity `USelect` (`@update:model-value="onEntityChange"`) is unchanged.
- Field rows: change `v-for="fl in filteredFields"` (now `fl` is a string field key):
  - `:key="fl.code"` → `:key="fl"`
  - `{{ fl.code }}` → `{{ fl }}`
  - `{{ fl.label }}` → `{{ fieldLabel(fl) }}`
  - `v-if="!isExplicit(fl.code)"` → `v-if="!isExplicit(fl)"`
  - `@click="resetField(fl.code)"` → `resetField(fl)`
  - In the role cells loop `v-for="c in roleCols"`: `:key="c.key"` stays; the `FieldpermFieldPermToggle` bindings change `cell(fl.code, c.key)` → `cell(fl, c.key)`, `isExplicit(fl.code)` → `isExplicit(fl)`, `toggleView(fl.code, c.key)` → `toggleView(fl, c.key)`, `toggleEdit(fl.code, c.key)` → `toggleEdit(fl, c.key)`.
- Add a load-error state. After the matrix card block (or wherever `loading` is checked), wrap so the matrix shows only when `!loading && !loadFailed`, and add:

```vue
    <div
      v-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-20 text-muted"
    >
      <UIcon name="i-lucide-circle-alert" class="size-6" />
      <span class="text-sm">{{ t('settings.fieldPermission.loadError') }}</span>
      <UButton color="neutral" variant="subtle" @click="load_">
        {{ t('settings.fieldPermission.retry') }}
      </UButton>
    </div>
```

The current page has no top-level `loading` spinner block guarding the matrix (the matrix renders immediately; the empty-state checks `!loading`). Add a `loading` spinner + the `loadFailed` block guarding the controls+matrix so the page shows loading → error → content. Concretely: wrap the Controls + Matrix sections in `<template v-else>` after a `v-if="loading"` spinner and the `v-else-if="loadFailed"` error block (mirror the Data Scope page's three-state structure), keeping the header always visible.

- [ ] **Step 3: Verify build/lint/typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: exit 0. NOTE: `pnpm test` will still FAIL on `test/nuxt/settings-field-permission.spec.ts` (old mock stub) — fixed in Task 4. Run `pnpm test -- use-field-permission field-catalog` to confirm Task 1/2 units still pass.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/pages/settings/field-permission.vue
git commit -m "feat(fieldperm): page on real API (catalog entities, id roles, default-allow, load error)"
```

---

### Task 4: Nuxt component test for the wired page

**Files:**
- Modify (rewrite): `frontend/test/nuxt/settings-field-permission.spec.ts`

**Interfaces:**
- Consumes: the wired page; mock the HTTP layer the way the RBAC/Data Scope component tests do.

- [ ] **Step 1: Study the patterns**

Read the CURRENT `frontend/test/nuxt/settings-field-permission.spec.ts` AND `frontend/test/nuxt/settings-data-scope.spec.ts` (the just-wired Data Scope test — it uses `vi.mock('~/composables/useApiClient', ...)` + a per-test `setHandler` that routes by method+path and captures request bodies, plus `useAuthStore().setSession(token, user, ['*'])` + `mountSuspended`). Mirror that stub approach for `GET /authz/roles`, `GET /authz/roles/:id/fields`, `PUT /authz/roles/:id/fields`.

- [ ] **Step 2: Write the rewritten test**

Rewrite `frontend/test/nuxt/settings-field-permission.spec.ts` to stub the real endpoints and assert real behavior. Fixtures: roles e.g. `[{id:'r-super',name:'Superadmin'},{id:'r-manager',name:'Manager'}]`; per-role fields e.g. Manager has `[{entity:'assets',field:'purchase_cost',can_view:false,can_edit:false},{entity:'users',field:'email',can_view:true,can_edit:false}]`, Superadmin `[]`. Cover:
- Loaded grid (default entity `assets`): role column headers show seeded role names ("Superadmin","Manager"); field rows show real keys (e.g. `purchase_cost`) with their i18n label ("Harga beli"); a field with no restriction shows the "Default" badge; `purchase_cost` for Manager shows view/edit off.
- Switching the entity select to `users` shows the users fields (e.g. `email`).
- Toggling a cell marks dirty + enables Save; clicking Save issues `PUT /authz/roles/:id/fields` whose body.fields (a) PRESERVES the other entity's rows (e.g. Manager's `users/email` row still present when saving `assets`), and (b) contains ONLY restriction cells for the edited entity (assert the captured body).
- Only changed roles are PUT (toggle one role's cell → exactly one PUT).
- Load-error: `GET /authz/roles` 500 → error block + retry.

Assert real rendered text + captured request bodies — no hollow checks. Use the harness default locale for i18n strings.

- [ ] **Step 3: Run the test + whole suite**

Run (from `frontend/`): `pnpm test -- settings-field-permission` then `pnpm test`
Expected: target PASS; whole suite green.

- [ ] **Step 4: Commit**

```bash
git add frontend/test/nuxt/settings-field-permission.spec.ts
git commit -m "test(fieldperm): component test against stubbed /authz endpoints"
```

---

### Task 5: E2E + delete mock + mockup + PROGRESS (+ enforcement TODO) + gate

**Files:**
- Modify: `frontend/e2e/settings.spec.ts`
- Delete (if orphaned): `frontend/app/mock/fieldPermission.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Update the e2e Field Permission assertions**

Read `frontend/e2e/settings.spec.ts` + `frontend/e2e/helpers.ts` (`login()`). Add/update a `/settings/field-permission` spec against the real backend + seeded admin: the grid renders with seeded role columns + real field rows (e.g. `purchase_cost`); toggle one cell + Save; reload → the change persists. Keep the RBAC + Data Scope + audit specs untouched. Use robust text/role locators (e.g. locate the field row by its `purchase_cost` text), NOT brittle Tailwind class selectors. You likely cannot RUN `pnpm test:e2e` here (needs the full backend stack); ensure the spec compiles + lints; it runs in CI. State that in the report.

- [ ] **Step 2: Delete the orphaned mock**

Run `grep -rn "mock/fieldPermission" frontend/app frontend/test` (exclude the file itself). After Tasks 2–4 the importers (composable, page, both old tests) no longer reference it. If ZERO importers remain, `git rm frontend/app/mock/fieldPermission.ts`. If something still imports it, do NOT delete — report what does.

- [ ] **Step 3: Mockup fidelity comparison**

Reference `docs/design/Field Permission.dc.html`. Structural comparison (read the `.dc.html` + the built `pages/settings/field-permission.vue` + `components/fieldperm/FieldPermToggle.vue`): verify the header/controls/entity-select/search/legend/matrix/sticky-column/default-badge/reset/toggle/states match. The entity + field SET intentionally differs (real backend keys: `assets`/`users` with English field keys) — approved decision, not a regression. Fix any other genuine deviation; report the result.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`:
- Under the frontend "Wire screens to real backend APIs" sub-list (RBAC ✅, Data Scope ✅), mark **Field Permission ✅ wired to `/authz`** (catalog `assets`+`users`; per-role fields; English DTO; id identity; default-allow; save preserves other entities).
- Add an explicit **TODO**: extend field-permission ENFORCEMENT (`FilterView`) beyond `assets`+`users` — `requests` (approval handler already injects `fieldSvc` + has `requestToMap`; just add the `ForEntity`/`FilterView` calls), `employees` (needs `fieldSvc`+map wiring), and other master-data modules. Until then the Field Permission screen only affects `assets`+`users`. Note the frontend catalog (`constants/fieldCatalog.ts`) is the single place to add an entity once its backend enforcement lands.
- Refresh "▶ Next session — start here": the authz-screen wiring trio is complete; point at backend bank-FAM next (e.g. asset transfer/mutasi) or the field-permission enforcement extension above.

- [ ] **Step 5: Full frontend gate**

Run (from `frontend/`):
```
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```
Expected: all green. (E2E runs in CI's e2e job.)

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/settings.spec.ts docs/PROGRESS.md
git commit -m "test(fieldperm): e2e against real backend + progress (with enforcement TODO); drop orphaned mock"
```
(If `mock/fieldPermission.ts` was deleted, the `git rm` is already staged — include it.)

---

## Self-Review

**Spec coverage:**
- bagian 2 catalog (real entities/fields, entity-agnostic) → Task 1 + Task 2 (`getEntities`). ✓
- bagian 3 composable rewrite (load/getEntities/getRules/saveRules) → Task 2. ✓
- bagian 4 pivot + cross-entity-preserving save (only restrictions; only changed roles) → Task 2 pure helpers (`deriveEntityRules`/`buildRoleRows`/`entityRowsEqual`) + `saveRules`. ✓
- bagian 5 page + FieldPermToggle (unchanged) + constants/i18n (default-allow cell fix, id roles, labels) → Task 1 (i18n) + Task 3 (page). ✓
- bagian 6 tests (unit/component/e2e) → Tasks 2, 4, 5. ✓
- bagian 7 done (delete mock, mockup, PROGRESS + enforcement TODO, gate) → Task 5. ✓

**Placeholder scan:** Tasks 4 & 5 give explicit assertion lists / steps (read the existing stub pattern first, since the FieldPermToggle interaction + stub helper match the Data Scope test). Concrete checklists, not "TODO"s.

**Type consistency:** `EntityView{key,fields:string[]}`, `RoleColumn{key,label}`, `FieldRow{entity,field,can_view,can_edit}`, `EntityRules = Record<string,Record<string,CellRule>>`, `CellRule{view,edit}` consistent across Tasks 1/2/3/4. Page `cell(field,roleId)`/`toggleView/Edit(field,roleId)`/`ensure(field)`/`saveRules(entity,rules,roleIds)` consistent with the composable + template. `getEntities` returns `{key,fields}`; the page builds localized labels via i18n functions — no label field on the composable's EntityView (page resolves `entityLabel`/`fieldLabel`).
