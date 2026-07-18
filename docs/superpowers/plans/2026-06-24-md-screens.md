# Master Data Screens (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the three Master Data screens (Kantor/Offices, Pegawai/Employees, Referensi/Reference) faithfully to their `docs/design` mockups, backed by mock fixtures behind a stable `composables/api/` interface.

**Architecture:** Each entity gets a mock fixture module in `app/mock/<entity>.ts` (seeded data + an in-memory store) and a typed CRUD composable in `app/composables/api/use<Entity>.ts`. Pages compose existing global components (`ResourceTable`, `FormModal`, `FormSlideover`, `PageHeader`, `DataToolbar`, `TreeView`, `StatusBadge`, `useConfirm`). When the backend is wired later, only the composable bodies change.

**Tech Stack:** Nuxt 4 (SPA, `ssr: false`), `@nuxt/ui` (`U*` components), TypeScript, Vitest + `@nuxt/test-utils` (`mountSuspended`), Playwright, vue-i18n (`id` default / `en`).

## Global Constraints

- **Spec:** `docs/superpowers/specs/2026-06-24-md-screens-design.md`; parent roadmap `2026-06-24-frontend-feature-screens-roadmap-design.md`.
- **Mock-first:** all data comes from `app/mock/*` via the composable; no real `$fetch` in this phase. Composable signatures return the backend envelope `Paginated<T>` so the later swap is mechanical.
- **Reuse before authoring:** compose existing `app/components/` primitives; only extract a new component when markup repeats across pages.
- **i18n mandatory:** every user-facing string in `i18n/locales/id.json` AND `i18n/locales/en.json`, referenced via `$t`/`useI18n`. Default locale `id`. Never hardcode UI text.
- **Theme via tokens:** semantic color props (`color="primary"`, `text-muted`) and CSS vars; no literal Tailwind colors.
- **Lint rules (CI-gated):** ESLint stylistic — **no trailing commas** (`commaDangle: 'never'`), 1tbs brace style. `pnpm lint` and `pnpm typecheck` must pass.
- **Permissions:** offices/employees pages gated by `masterdata.office.manage`; reference by `masterdata.global.manage`. Use `definePageMeta({ middleware: 'can', permission: '...' })` for route gating and `<Can>`/`useCan` for action buttons.
- **Tests assert real behavior** (rendered text, resolved i18n, state transitions) — never `expect(html.length).toBeGreaterThan(0)`.
- **Verify green before "done":** `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`. Match mockups in light **and** dark mode.
- Run a single Vitest file with a path filter: `pnpm test <fragment>` (e.g. `pnpm test offices-mock`).
- All commands run from `frontend/`.

---

### Task 1: Shared mock store + master-data types

**Files:**
- Modify: `frontend/app/mock/helpers.ts` (add `generateId`, `createStore`)
- Modify: `frontend/app/types/index.ts` (add `Office`, `Employee`, `ReferenceRow`)
- Test: `frontend/test/unit/mock-store.spec.ts`

**Interfaces:**
- Consumes: existing `Paginated<T>`, `ListQuery` from `~/types`; `paginate`/`filterBy`/`fakeLatency` from `~/mock/helpers`.
- Produces:
  - `generateId(): string`
  - `interface MockStore<T extends { id: string }>` with `all(): T[]`, `find(id: string): T | undefined`, `insert(row: T): T`, `patch(id: string, changes: Partial<T>): T | undefined`, `remove(id: string): boolean`
  - `createStore<T extends { id: string }>(seed: T[]): MockStore<T>`
  - `interface Office { id, nama, kode, tipe: 'pusat'|'kanwil'|'cabang'|'unit', parent_id: string|null, provinsi, kota, alamat, created_at }`
  - `interface Employee { id, nip, nama, email, telepon, jabatan, departemen, office_id, status: 'active'|'inactive', created_at }`
  - `interface ReferenceRow { id: string, name: string, code?: string, [key: string]: unknown }`
  - `interface TreeNode { id: string, label: string, icon?: string, childCount?: number, children?: TreeNode[] }` — moved here from `TreeView.vue` so node-env modules (`app/mock/offices.ts`) can import it without pulling a `.vue` file.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/mock-store.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { createStore, generateId } from '~/mock/helpers'

interface Row { id: string, name: string }

describe('generateId', () => {
  it('returns unique non-empty strings', () => {
    const a = generateId()
    const b = generateId()
    expect(a).not.toBe('')
    expect(a).not.toBe(b)
  })
})

describe('createStore', () => {
  it('returns all seeded rows', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    expect(store.all()).toHaveLength(1)
  })

  it('finds a row by id', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    expect(store.find('1')?.name).toBe('A')
    expect(store.find('nope')).toBeUndefined()
  })

  it('inserts a row at the front', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    store.insert({ id: '2', name: 'B' })
    expect(store.all()[0].id).toBe('2')
    expect(store.all()).toHaveLength(2)
  })

  it('patches an existing row and returns it', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    const updated = store.patch('1', { name: 'Z' })
    expect(updated?.name).toBe('Z')
    expect(store.find('1')?.name).toBe('Z')
  })

  it('returns undefined when patching a missing row', () => {
    const store = createStore<Row>([])
    expect(store.patch('x', { name: 'Z' })).toBeUndefined()
  })

  it('removes a row and reports success', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    expect(store.remove('1')).toBe(true)
    expect(store.all()).toHaveLength(0)
    expect(store.remove('1')).toBe(false)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test mock-store`
Expected: FAIL — `createStore`/`generateId` not exported.

- [ ] **Step 3: Add the helpers**

Append to `frontend/app/mock/helpers.ts`:

```ts
export function generateId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `id-${Math.random().toString(36).slice(2)}-${performance.now()}`
}

export interface MockStore<T extends { id: string }> {
  all(): T[]
  find(id: string): T | undefined
  insert(row: T): T
  patch(id: string, changes: Partial<T>): T | undefined
  remove(id: string): boolean
}

export function createStore<T extends { id: string }>(seed: T[]): MockStore<T> {
  const rows: T[] = [...seed]
  return {
    all: () => rows,
    find: id => rows.find(r => r.id === id),
    insert(row) {
      rows.unshift(row)
      return row
    },
    patch(id, changes) {
      const row = rows.find(r => r.id === id)
      if (!row) return undefined
      Object.assign(row, changes)
      return row
    },
    remove(id) {
      const i = rows.findIndex(r => r.id === id)
      if (i === -1) return false
      rows.splice(i, 1)
      return true
    }
  }
}
```

- [ ] **Step 4: Add the types**

Append to `frontend/app/types/index.ts`:

```ts
export interface Office {
  id: string
  nama: string
  kode: string
  tipe: 'pusat' | 'kanwil' | 'cabang' | 'unit'
  parent_id: string | null
  provinsi: string
  kota: string
  alamat: string
  created_at: string
}

export interface Employee {
  id: string
  nip: string
  nama: string
  email: string
  telepon: string
  jabatan: string
  departemen: string
  office_id: string
  status: 'active' | 'inactive'
  created_at: string
}

export interface ReferenceRow {
  id: string
  name: string
  code?: string
  [key: string]: unknown
}

export interface TreeNode {
  id: string
  label: string
  icon?: string
  childCount?: number
  children?: TreeNode[]
}
```

- [ ] **Step 4b: Point `TreeView.vue` at the shared type (single source of truth)**

Edit `frontend/app/components/TreeView.vue` — replace its local `export interface TreeNode { … }` block with a re-export so existing/new consumers resolve the same type:

```ts
import type { TreeNode } from '~/types'
export type { TreeNode }
```

Leave the rest of `TreeView.vue` unchanged (it still uses `TreeNode` for its `nodes` prop).

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm test mock-store`
Expected: PASS (all cases).

- [ ] **Step 6: Typecheck + lint**

Run: `pnpm typecheck && pnpm lint`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add app/mock/helpers.ts app/types/index.ts app/components/TreeView.vue test/unit/mock-store.spec.ts
git commit -m "feat(frontend): add mock store + master-data types"
```

---

### Task 2: Offices mock module + useOffices composable

**Files:**
- Create: `frontend/app/mock/offices.ts`
- Create: `frontend/app/composables/api/useOffices.ts`
- Modify: `frontend/app/mock/index.ts` (re-export offices)
- Test: `frontend/test/unit/offices-mock.spec.ts`

**Interfaces:**
- Consumes: `createStore`, `generateId`, `paginate`, `filterBy`, `fakeLatency` from `~/mock/helpers`; `Office`, `Paginated`, `ListQuery`, `TreeNode` from `~/types`.
- Produces:
  - `app/mock/offices.ts`: `officeStore: MockStore<Office>`, `buildOfficeTree(offices: Office[]): TreeNode[]`, `officeSeed: Office[]`
  - `app/composables/api/useOffices.ts`: `interface OfficeInput { nama, kode, tipe, parent_id, provinsi, kota, alamat }` and `useOffices()` returning `{ list(query?): Promise<Paginated<Office>>, get(id): Promise<Office|undefined>, tree(): Promise<TreeNode[]>, create(input: OfficeInput): Promise<Office>, update(id, input: OfficeInput): Promise<Office>, remove(id): Promise<void> }`
  - Sentinel: `useOffices().create`/`update` throw `Error` with message key `masterdata.offices.errInvalidParent` when `parent_id` references a non-existent office (mock scope rule).

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/offices-mock.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { buildOfficeTree } from '~/mock/offices'
import type { Office } from '~/types'

function office(id: string, nama: string, parent_id: string | null): Office {
  return { id, nama, kode: id, tipe: 'cabang', parent_id, provinsi: 'X', kota: 'Y', alamat: 'Z', created_at: '2026-01-01' }
}

describe('buildOfficeTree', () => {
  it('nests children under their parent', () => {
    const tree = buildOfficeTree([
      office('1', 'Pusat', null),
      office('2', 'Kanwil A', '1'),
      office('3', 'Cabang A1', '2')
    ])
    expect(tree).toHaveLength(1)
    expect(tree[0].label).toBe('Pusat')
    expect(tree[0].children?.[0].label).toBe('Kanwil A')
    expect(tree[0].children?.[0].children?.[0].label).toBe('Cabang A1')
  })

  it('reports child counts and leaves children undefined for leaves', () => {
    const tree = buildOfficeTree([
      office('1', 'Pusat', null),
      office('2', 'Kanwil A', '1')
    ])
    expect(tree[0].childCount).toBe(1)
    expect(tree[0].children?.[0].children).toBeUndefined()
  })

  it('returns multiple roots when several offices have no parent', () => {
    const tree = buildOfficeTree([office('1', 'A', null), office('2', 'B', null)])
    expect(tree).toHaveLength(2)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test offices-mock`
Expected: FAIL — cannot resolve `~/mock/offices`.

- [ ] **Step 3: Create the offices mock module**

Create `frontend/app/mock/offices.ts`:

```ts
import type { Office, TreeNode } from '~/types'
import { createStore } from './helpers'

export const officeSeed: Office[] = [
  { id: 'o-pusat', nama: 'Kantor Pusat', kode: 'PST', tipe: 'pusat', parent_id: null, provinsi: 'DKI Jakarta', kota: 'Jakarta Pusat', alamat: 'Jl. Merdeka No. 1', created_at: '2026-01-02' },
  { id: 'o-jkt', nama: 'Kanwil Jakarta', kode: 'JKT01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Sudirman No. 10', created_at: '2026-01-03' },
  { id: 'o-jkt-a', nama: 'Cabang Kebayoran', kode: 'JKT01-A', tipe: 'cabang', parent_id: 'o-jkt', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Kebayoran No. 5', created_at: '2026-01-04' },
  { id: 'o-bdg', nama: 'Kanwil Bandung', kode: 'BDG01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'Jawa Barat', kota: 'Bandung', alamat: 'Jl. Asia Afrika No. 8', created_at: '2026-01-05' }
]

export const officeStore = createStore<Office>(officeSeed)

export function buildOfficeTree(offices: Office[]): TreeNode[] {
  const byParent = new Map<string | null, Office[]>()
  for (const o of offices) {
    const list = byParent.get(o.parent_id) ?? []
    list.push(o)
    byParent.set(o.parent_id, list)
  }
  function build(parentId: string | null): TreeNode[] {
    return (byParent.get(parentId) ?? []).map((o) => {
      const children = build(o.id)
      return {
        id: o.id,
        label: o.nama,
        icon: 'i-lucide-building-2',
        childCount: children.length || undefined,
        children: children.length ? children : undefined
      }
    })
  }
  return build(null)
}
```

- [ ] **Step 4: Create the composable**

Create `frontend/app/composables/api/useOffices.ts`:

```ts
import type { ListQuery, Office, Paginated, TreeNode } from '~/types'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { buildOfficeTree, officeStore } from '~/mock/offices'

export interface OfficeInput {
  nama: string
  kode: string
  tipe: Office['tipe']
  parent_id: string | null
  provinsi: string
  kota: string
  alamat: string
}

function assertValidParent(parentId: string | null) {
  if (parentId && !officeStore.find(parentId)) {
    throw new Error('masterdata.offices.errInvalidParent')
  }
}

export function useOffices() {
  async function list(query: ListQuery = {}): Promise<Paginated<Office>> {
    await fakeLatency()
    return paginate(filterBy(officeStore.all(), query, ['nama', 'kode', 'kota']), query)
  }

  async function get(id: string): Promise<Office | undefined> {
    await fakeLatency()
    return officeStore.find(id)
  }

  async function tree(): Promise<TreeNode[]> {
    await fakeLatency()
    return buildOfficeTree(officeStore.all())
  }

  async function create(input: OfficeInput): Promise<Office> {
    await fakeLatency()
    assertValidParent(input.parent_id)
    return officeStore.insert({ id: generateId(), created_at: new Date().toISOString(), ...input })
  }

  async function update(id: string, input: OfficeInput): Promise<Office> {
    await fakeLatency()
    assertValidParent(input.parent_id)
    const row = officeStore.patch(id, input)
    if (!row) throw new Error('masterdata.offices.errNotFound')
    return row
  }

  async function remove(id: string): Promise<void> {
    await fakeLatency()
    officeStore.remove(id)
  }

  return { list, get, tree, create, update, remove }
}
```

- [ ] **Step 5: Re-export from the mock barrel**

Edit `frontend/app/mock/index.ts` — replace the placeholder comment line with:

```ts
// Module fixtures (assets, employees, …) are re-exported here in later phases.
export * from './helpers'
export * from './offices'
```

- [ ] **Step 6: Run test to verify it passes**

Run: `pnpm test offices-mock`
Expected: PASS.

- [ ] **Step 7: Typecheck + lint**

Run: `pnpm typecheck && pnpm lint`
Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add app/mock/offices.ts app/mock/index.ts app/composables/api/useOffices.ts test/unit/offices-mock.spec.ts
git commit -m "feat(frontend): add offices mock store + useOffices composable"
```

---

### Task 3: Master Data Kantor page (`/master/offices`)

**Files:**
- Create: `frontend/app/pages/master/offices.vue`
- Modify: `frontend/i18n/locales/id.json` (add `masterdata.offices.*`)
- Modify: `frontend/i18n/locales/en.json` (add `masterdata.offices.*`)
- Test: `frontend/test/nuxt/master-offices.spec.ts`

**Interfaces:**
- Consumes: `useOffices()` (`list`/`tree`/`get`/`create`/`update`/`remove`, `OfficeInput`) from Task 2; existing `PageHeader`, `DataToolbar`, `TreeView`, `FormSlideover`, `Can`, `useConfirm`, `EmptyState`.
- Produces: route `/master/offices` (permission `masterdata.office.manage`).

> **Mockup:** open `docs/design/Master Data Kantor.dc.html`. Layout = master/detail: left a `TreeView` of offices; selecting a node fills a right-side detail panel (nama, kode, tipe, kota, provinsi, alamat, parent). Toolbar with search. Add/Edit via `FormSlideover`; delete via `useConfirm`. Match light and dark mode.

- [ ] **Step 1: Add i18n keys**

In `frontend/i18n/locales/id.json`, add a top-level `"masterdata"` object (sibling of `"status"`):

```json
"masterdata": {
  "offices": {
    "title": "Master Data Kantor",
    "subtitle": "Kelola hierarki kantor 4 jenjang",
    "add": "Tambah Kantor",
    "searchPlaceholder": "Cari nama / kode / kota",
    "selectHint": "Pilih kantor pada pohon untuk melihat detail",
    "empty": "Belum ada kantor",
    "fields": {
      "nama": "Nama Kantor",
      "kode": "Kode",
      "tipe": "Jenjang",
      "parent": "Induk",
      "provinsi": "Provinsi",
      "kota": "Kota",
      "alamat": "Alamat"
    },
    "tipe": { "pusat": "Pusat", "kanwil": "Kanwil", "cabang": "Cabang", "unit": "Unit" },
    "noParent": "Tanpa induk (Pusat)",
    "createTitle": "Tambah Kantor",
    "editTitle": "Ubah Kantor",
    "deleteConfirm": "Hapus kantor ini? Tindakan ini tidak dapat dibatalkan.",
    "errInvalidParent": "Induk kantor tidak valid.",
    "errNotFound": "Kantor tidak ditemukan."
  }
}
```

In `frontend/i18n/locales/en.json`, add the matching object:

```json
"masterdata": {
  "offices": {
    "title": "Office Master Data",
    "subtitle": "Manage the 4-tier office hierarchy",
    "add": "Add Office",
    "searchPlaceholder": "Search name / code / city",
    "selectHint": "Select an office in the tree to view details",
    "empty": "No offices yet",
    "fields": {
      "nama": "Office Name",
      "kode": "Code",
      "tipe": "Tier",
      "parent": "Parent",
      "provinsi": "Province",
      "kota": "City",
      "alamat": "Address"
    },
    "tipe": { "pusat": "Head Office", "kanwil": "Regional", "cabang": "Branch", "unit": "Unit" },
    "noParent": "No parent (Head Office)",
    "createTitle": "Add Office",
    "editTitle": "Edit Office",
    "deleteConfirm": "Delete this office? This action cannot be undone.",
    "errInvalidParent": "Invalid parent office.",
    "errNotFound": "Office not found."
  }
}
```

- [ ] **Step 2: Write the failing runtime test**

Create `frontend/test/nuxt/master-offices.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import OfficesPage from '~/pages/master/offices.vue'

describe('Master Data Kantor page', () => {
  it('renders the page title and seeded offices in the tree', async () => {
    const wrapper = await mountSuspended(OfficesPage)
    // wait for fakeLatency-resolved tree()
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Master Data Kantor')
    expect(html).toContain('Kantor Pusat')
    expect(html).toContain('Kanwil Jakarta')
  })

  it('shows the select hint before any node is chosen', async () => {
    const wrapper = await mountSuspended(OfficesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Pilih kantor pada pohon untuk melihat detail')
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `pnpm test master-offices`
Expected: FAIL — cannot resolve `~/pages/master/offices.vue`.

- [ ] **Step 4: Create the page**

Create `frontend/app/pages/master/offices.vue`:

```vue
<script setup lang="ts">
import type { Office, TreeNode } from '~/types'
import type { OfficeInput } from '~/composables/api/useOffices'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useOffices()

const nodes = ref<TreeNode[]>([])
const selectedId = ref<string>()
const selected = ref<Office>()
const search = ref('')
const loading = ref(true)

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<OfficeInput>({
  nama: '', kode: '', tipe: 'cabang', parent_id: null, provinsi: '', kota: '', alamat: ''
})

const tipeOptions = (['pusat', 'kanwil', 'cabang', 'unit'] as const).map(v => ({
  value: v, label: t(`masterdata.offices.tipe.${v}`)
}))

async function refresh() {
  loading.value = true
  nodes.value = await api.tree()
  loading.value = false
}

async function onSelect(id: string) {
  selectedId.value = id
  selected.value = await api.get(id)
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { nama: '', kode: '', tipe: 'cabang', parent_id: selectedId.value ?? null, provinsi: '', kota: '', alamat: '' })
  formOpen.value = true
}

function openEdit() {
  if (!selected.value) return
  editingId.value = selected.value.id
  Object.assign(form, {
    nama: selected.value.nama, kode: selected.value.kode, tipe: selected.value.tipe,
    parent_id: selected.value.parent_id, provinsi: selected.value.provinsi,
    kota: selected.value.kota, alamat: selected.value.alamat
  })
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    const saved = editingId.value
      ? await api.update(editingId.value, { ...form })
      : await api.create({ ...form })
    formOpen.value = false
    await refresh()
    await onSelect(saved.id)
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete() {
  if (!selected.value) return
  const ok = await confirm({ title: t('common.delete'), description: t('masterdata.offices.deleteConfirm') })
  if (!ok) return
  await api.remove(selected.value.id)
  selected.value = undefined
  selectedId.value = undefined
  await refresh()
}

const detailRows = computed(() => {
  const o = selected.value
  if (!o) return []
  return [
    { label: t('masterdata.offices.fields.kode'), value: o.kode },
    { label: t('masterdata.offices.fields.tipe'), value: t(`masterdata.offices.tipe.${o.tipe}`) },
    { label: t('masterdata.offices.fields.provinsi'), value: o.provinsi },
    { label: t('masterdata.offices.fields.kota'), value: o.kota },
    { label: t('masterdata.offices.fields.alamat'), value: o.alamat }
  ]
})

onMounted(refresh)
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.offices.title')"
      :subtitle="t('masterdata.offices.subtitle')"
    >
      <template #actions>
        <Can permission="masterdata.office.manage">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.offices.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <DataToolbar
      v-model:search="search"
      @reset="search = ''"
    />

    <div class="grid grid-cols-1 lg:grid-cols-[20rem_1fr] gap-4">
      <UCard>
        <TableSkeleton
          v-if="loading"
          :cols="1"
        />
        <EmptyState
          v-else-if="nodes.length === 0"
          :title="t('masterdata.offices.empty')"
        />
        <TreeView
          v-else
          :nodes="nodes"
          :selected-id="selectedId"
          @select="onSelect"
        />
      </UCard>

      <UCard>
        <EmptyState
          v-if="!selected"
          :title="t('masterdata.offices.selectHint')"
        />
        <div v-else>
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold">
              {{ selected.nama }}
            </h2>
            <Can permission="masterdata.office.manage">
              <div class="flex gap-2">
                <UButton
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-pencil"
                  @click="openEdit"
                >
                  {{ t('common.edit') }}
                </UButton>
                <UButton
                  color="error"
                  variant="ghost"
                  icon="i-lucide-trash-2"
                  @click="onDelete"
                >
                  {{ t('common.delete') }}
                </UButton>
              </div>
            </Can>
          </div>
          <dl class="grid grid-cols-2 gap-y-3 text-sm">
            <template
              v-for="row in detailRows"
              :key="row.label"
            >
              <dt class="text-muted">
                {{ row.label }}
              </dt>
              <dd>{{ row.value }}</dd>
            </template>
          </dl>
        </div>
      </UCard>
    </div>

    <FormSlideover
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.offices.editTitle') : t('masterdata.offices.createTitle')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField :label="t('masterdata.offices.fields.nama')">
          <UInput
            v-model="form.nama"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.kode')">
          <UInput
            v-model="form.kode"
            placeholder="JKT01"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.tipe')">
          <USelect
            v-model="form.tipe"
            :items="tipeOptions"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.provinsi')">
          <UInput
            v-model="form.provinsi"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.kota')">
          <UInput
            v-model="form.kota"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.alamat')">
          <UTextarea
            v-model="form.alamat"
            class="w-full"
          />
        </UFormField>
      </div>
    </FormSlideover>
  </div>
</template>
```

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm test master-offices`
Expected: PASS (both cases).

- [ ] **Step 6: Verify the i18n JSON is valid + lint + typecheck**

Run: `pnpm typecheck && pnpm lint`
Expected: no errors (JSON parses, no trailing commas).

- [ ] **Step 7: Commit**

```bash
git add app/pages/master/offices.vue i18n/locales/id.json i18n/locales/en.json test/nuxt/master-offices.spec.ts
git commit -m "feat(frontend): build master data kantor (offices) screen"
```

---

### Task 4: Employees mock module + useEmployees composable

**Files:**
- Create: `frontend/app/mock/employees.ts`
- Create: `frontend/app/composables/api/useEmployees.ts`
- Modify: `frontend/app/mock/index.ts` (re-export employees)
- Test: `frontend/test/unit/employees-mock.spec.ts`

**Interfaces:**
- Consumes: `createStore`, `generateId`, `paginate`, `filterBy`, `fakeLatency` from `~/mock/helpers`; `Employee`, `Paginated`, `ListQuery` from `~/types`.
- Produces:
  - `app/mock/employees.ts`: `employeeStore: MockStore<Employee>`, `employeeSeed: Employee[]`
  - `app/composables/api/useEmployees.ts`: `interface EmployeeInput { nip, nama, email, telepon, jabatan, departemen, office_id, status }` and `useEmployees()` returning `{ list(query?): Promise<Paginated<Employee>>, get(id): Promise<Employee|undefined>, create(input: EmployeeInput): Promise<Employee>, update(id, input: EmployeeInput): Promise<Employee>, remove(id): Promise<void> }`

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/employees-mock.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { employeeSeed, employeeStore } from '~/mock/employees'
import { filterBy } from '~/mock/helpers'

describe('employees mock', () => {
  it('seeds more than one employee', () => {
    expect(employeeSeed.length).toBeGreaterThan(1)
  })

  it('every seeded employee has an active or inactive status', () => {
    expect(employeeSeed.every(e => e.status === 'active' || e.status === 'inactive')).toBe(true)
  })

  it('filterBy matches by name and nip', () => {
    const all = employeeStore.all()
    const first = all[0]
    expect(filterBy(all, { search: first.nama }, ['nama', 'nip', 'email'])).toContainEqual(first)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test employees-mock`
Expected: FAIL — cannot resolve `~/mock/employees`.

- [ ] **Step 3: Create the employees mock module**

Create `frontend/app/mock/employees.ts`:

```ts
import type { Employee } from '~/types'
import { createStore } from './helpers'

export const employeeSeed: Employee[] = [
  { id: 'e-1', nip: '199001012015011001', nama: 'Andi Pratama', email: 'andi.pratama@inventra.go.id', telepon: '0812-1111-2222', jabatan: 'Kepala Kantor', departemen: 'Umum', office_id: 'o-jkt', status: 'active', created_at: '2026-01-10' },
  { id: 'e-2', nip: '199203122016012002', nama: 'Bunga Lestari', email: 'bunga.lestari@inventra.go.id', telepon: '0813-3333-4444', jabatan: 'Staf', departemen: 'Keuangan', office_id: 'o-jkt', status: 'active', created_at: '2026-01-11' },
  { id: 'e-3', nip: '198805052012011003', nama: 'Citra Dewi', email: 'citra.dewi@inventra.go.id', telepon: '0814-5555-6666', jabatan: 'Kepala Unit', departemen: 'Aset', office_id: 'o-bdg', status: 'inactive', created_at: '2026-01-12' }
]

export const employeeStore = createStore<Employee>(employeeSeed)
```

- [ ] **Step 4: Create the composable**

Create `frontend/app/composables/api/useEmployees.ts`:

```ts
import type { Employee, ListQuery, Paginated } from '~/types'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { employeeStore } from '~/mock/employees'

export interface EmployeeInput {
  nip: string
  nama: string
  email: string
  telepon: string
  jabatan: string
  departemen: string
  office_id: string
  status: Employee['status']
}

export function useEmployees() {
  async function list(query: ListQuery = {}): Promise<Paginated<Employee>> {
    await fakeLatency()
    return paginate(filterBy(employeeStore.all(), query, ['nama', 'nip', 'email']), query)
  }

  async function get(id: string): Promise<Employee | undefined> {
    await fakeLatency()
    return employeeStore.find(id)
  }

  async function create(input: EmployeeInput): Promise<Employee> {
    await fakeLatency()
    return employeeStore.insert({ id: generateId(), created_at: new Date().toISOString(), ...input })
  }

  async function update(id: string, input: EmployeeInput): Promise<Employee> {
    await fakeLatency()
    const row = employeeStore.patch(id, input)
    if (!row) throw new Error('masterdata.employees.errNotFound')
    return row
  }

  async function remove(id: string): Promise<void> {
    await fakeLatency()
    employeeStore.remove(id)
  }

  return { list, get, create, update, remove }
}
```

- [ ] **Step 5: Re-export from the mock barrel**

Edit `frontend/app/mock/index.ts` — add the line:

```ts
export * from './employees'
```

- [ ] **Step 6: Run test to verify it passes**

Run: `pnpm test employees-mock`
Expected: PASS.

- [ ] **Step 7: Typecheck + lint**

Run: `pnpm typecheck && pnpm lint`
Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add app/mock/employees.ts app/mock/index.ts app/composables/api/useEmployees.ts test/unit/employees-mock.spec.ts
git commit -m "feat(frontend): add employees mock store + useEmployees composable"
```

---

### Task 5: Master Data Pegawai page (`/master/employees`)

**Files:**
- Create: `frontend/app/pages/master/employees.vue`
- Modify: `frontend/i18n/locales/id.json` (add `masterdata.employees.*`)
- Modify: `frontend/i18n/locales/en.json` (add `masterdata.employees.*`)
- Test: `frontend/test/nuxt/master-employees.spec.ts`

**Interfaces:**
- Consumes: `useEmployees()` + `EmployeeInput` from Task 4; existing `PageHeader`, `DataToolbar`, `ResourceTable`, `FormModal`, `StatusBadge`, `Can`, `useConfirm`.
- Produces: route `/master/employees` (permission `masterdata.office.manage`).

> **Mockup:** open `docs/design/Master Data Pegawai.dc.html`. Layout = `PageHeader` + `DataToolbar` (search) + `ResourceTable` (columns: NIP, Nama, Jabatan, Kantor, Status badge) with row actions (edit/delete) + pagination. Create/Edit via `FormModal`. Status shown via a badge (active=success, inactive=neutral).

- [ ] **Step 1: Add i18n keys**

Into the existing `"masterdata"` object in `frontend/i18n/locales/id.json`, add an `"employees"` key:

```json
"employees": {
  "title": "Master Data Pegawai",
  "subtitle": "Kelola data pegawai",
  "add": "Tambah Pegawai",
  "searchPlaceholder": "Cari nama / NIP / email",
  "empty": "Belum ada pegawai",
  "columns": { "nip": "NIP", "nama": "Nama", "jabatan": "Jabatan", "kantor": "Kantor", "status": "Status" },
  "fields": {
    "nip": "NIP", "nama": "Nama", "email": "Email", "telepon": "Telepon",
    "jabatan": "Jabatan", "departemen": "Departemen", "office": "Kantor", "status": "Status"
  },
  "status": { "active": "Aktif", "inactive": "Nonaktif" },
  "createTitle": "Tambah Pegawai",
  "editTitle": "Ubah Pegawai",
  "deleteConfirm": "Hapus pegawai ini?",
  "errNotFound": "Pegawai tidak ditemukan."
}
```

Into the `"masterdata"` object in `frontend/i18n/locales/en.json`, add:

```json
"employees": {
  "title": "Employee Master Data",
  "subtitle": "Manage employee records",
  "add": "Add Employee",
  "searchPlaceholder": "Search name / ID / email",
  "empty": "No employees yet",
  "columns": { "nip": "ID", "nama": "Name", "jabatan": "Position", "kantor": "Office", "status": "Status" },
  "fields": {
    "nip": "Employee ID", "nama": "Name", "email": "Email", "telepon": "Phone",
    "jabatan": "Position", "departemen": "Department", "office": "Office", "status": "Status"
  },
  "status": { "active": "Active", "inactive": "Inactive" },
  "createTitle": "Add Employee",
  "editTitle": "Edit Employee",
  "deleteConfirm": "Delete this employee?",
  "errNotFound": "Employee not found."
}
```

- [ ] **Step 2: Write the failing runtime test**

Create `frontend/test/nuxt/master-employees.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import EmployeesPage from '~/pages/master/employees.vue'

describe('Master Data Pegawai page', () => {
  it('renders the title and seeded employees after load', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Master Data Pegawai')
    expect(html).toContain('Andi Pratama')
    expect(html).toContain('Bunga Lestari')
  })

  it('renders translated status labels', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    // active → "Aktif", inactive → "Nonaktif" (id locale)
    expect(wrapper.html()).toContain('Aktif')
    expect(wrapper.html()).toContain('Nonaktif')
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `pnpm test master-employees`
Expected: FAIL — cannot resolve `~/pages/master/employees.vue`.

- [ ] **Step 4: Create the page**

Create `frontend/app/pages/master/employees.vue`:

```vue
<script setup lang="ts">
import type { Employee } from '~/types'
import type { EmployeeInput } from '~/composables/api/useEmployees'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useEmployees()

const rows = ref<Employee[]>([])
const total = ref(0)
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const loading = ref(true)

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<EmployeeInput>({
  nip: '', nama: '', email: '', telepon: '', jabatan: '', departemen: '', office_id: 'o-jkt', status: 'active'
})

const columns = [
  { accessorKey: 'nip', header: t('masterdata.employees.columns.nip') },
  { accessorKey: 'nama', header: t('masterdata.employees.columns.nama') },
  { accessorKey: 'jabatan', header: t('masterdata.employees.columns.jabatan') },
  { accessorKey: 'status', header: t('masterdata.employees.columns.status') }
]

const statusOptions = (['active', 'inactive'] as const).map(v => ({
  value: v, label: t(`masterdata.employees.status.${v}`)
}))

async function refresh() {
  loading.value = true
  const res = await api.list({ search: search.value, limit: limit.value, offset: offset.value })
  rows.value = res.data
  total.value = res.total
  loading.value = false
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { nip: '', nama: '', email: '', telepon: '', jabatan: '', departemen: '', office_id: 'o-jkt', status: 'active' })
  formOpen.value = true
}

function openEdit(row: Employee) {
  editingId.value = row.id
  Object.assign(form, {
    nip: row.nip, nama: row.nama, email: row.email, telepon: row.telepon,
    jabatan: row.jabatan, departemen: row.departemen, office_id: row.office_id, status: row.status
  })
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    if (editingId.value) await api.update(editingId.value, { ...form })
    else await api.create({ ...form })
    formOpen.value = false
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete(row: Employee) {
  const ok = await confirm({ title: t('common.delete'), description: t('masterdata.employees.deleteConfirm') })
  if (!ok) return
  await api.remove(row.id)
  await refresh()
}

watch([search, offset], refresh)
onMounted(refresh)
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.employees.title')"
      :subtitle="t('masterdata.employees.subtitle')"
    >
      <template #actions>
        <Can permission="masterdata.office.manage">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.employees.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <DataToolbar
      v-model:search="search"
      @reset="search = ''"
    />

    <ResourceTable
      :rows="rows"
      :columns="columns"
      :loading="loading"
      :total="total"
      :limit="limit"
      :offset="offset"
      :empty-title="t('masterdata.employees.empty')"
      @update:offset="offset = $event"
    >
      <template #status-cell="{ row }">
        <UBadge
          :color="(row as Employee).status === 'active' ? 'success' : 'neutral'"
          variant="subtle"
        >
          {{ t(`masterdata.employees.status.${(row as Employee).status}`) }}
        </UBadge>
      </template>
      <template #row-actions="{ row }">
        <Can permission="masterdata.office.manage">
          <div class="flex gap-1">
            <UButton
              color="neutral"
              variant="ghost"
              icon="i-lucide-pencil"
              size="xs"
              @click="openEdit(row as Employee)"
            />
            <UButton
              color="error"
              variant="ghost"
              icon="i-lucide-trash-2"
              size="xs"
              @click="onDelete(row as Employee)"
            />
          </div>
        </Can>
      </template>
    </ResourceTable>

    <FormModal
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.employees.editTitle') : t('masterdata.employees.createTitle')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField :label="t('masterdata.employees.fields.nip')">
          <UInput
            v-model="form.nip"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.nama')">
          <UInput
            v-model="form.nama"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.email')">
          <UInput
            v-model="form.email"
            type="email"
            placeholder="nama@inventra.go.id"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.telepon')">
          <UInput
            v-model="form.telepon"
            placeholder="08xx-xxxx-xxxx"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.jabatan')">
          <UInput
            v-model="form.jabatan"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.departemen')">
          <UInput
            v-model="form.departemen"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.status')">
          <USelect
            v-model="form.status"
            :items="statusOptions"
            class="w-full"
          />
        </UFormField>
      </div>
    </FormModal>
  </div>
</template>
```

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm test master-employees`
Expected: PASS (both cases).

- [ ] **Step 6: Typecheck + lint**

Run: `pnpm typecheck && pnpm lint`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add app/pages/master/employees.vue i18n/locales/id.json i18n/locales/en.json test/nuxt/master-employees.spec.ts
git commit -m "feat(frontend): build master data pegawai (employees) screen"
```

---

### Task 6: Reference descriptors + useReference composable

**Files:**
- Create: `frontend/app/composables/api/referenceResources.ts`
- Create: `frontend/app/mock/reference.ts`
- Create: `frontend/app/composables/api/useReference.ts`
- Modify: `frontend/app/mock/index.ts` (re-export reference)
- Test: `frontend/test/unit/reference-mock.spec.ts`

**Interfaces:**
- Consumes: `createStore`, `generateId`, `paginate`, `filterBy`, `fakeLatency` from `~/mock/helpers`; `ReferenceRow`, `Paginated`, `ListQuery` from `~/types`.
- Produces:
  - `app/composables/api/referenceResources.ts`: `type ReferenceKey` (union of the 11 keys), `interface ReferenceField { key: string, labelKey: string, placeholder?: string }`, `interface ReferenceDescriptor { key: ReferenceKey, labelKey: string, fields: ReferenceField[] }`, `referenceResources: ReferenceDescriptor[]`
  - `app/mock/reference.ts`: `referenceStores: Record<ReferenceKey, MockStore<ReferenceRow>>`
  - `app/composables/api/useReference.ts`: `useReference()` returning `{ list(key, query?): Promise<Paginated<ReferenceRow>>, create(key, input): Promise<ReferenceRow>, update(key, id, input): Promise<ReferenceRow>, remove(key, id): Promise<void> }`

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/reference-mock.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { referenceResources } from '~/composables/api/referenceResources'
import { referenceStores } from '~/mock/reference'

describe('reference resources', () => {
  it('declares all 11 reference resources', () => {
    expect(referenceResources).toHaveLength(11)
  })

  it('every descriptor has at least one field', () => {
    expect(referenceResources.every(r => r.fields.length >= 1)).toBe(true)
  })

  it('has a backing store for every declared resource', () => {
    for (const r of referenceResources) {
      expect(referenceStores[r.key]).toBeDefined()
    }
  })

  it('provinces store is seeded', () => {
    expect(referenceStores.provinces.all().length).toBeGreaterThan(0)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test reference-mock`
Expected: FAIL — cannot resolve the new modules.

- [ ] **Step 3: Create the descriptor table**

Create `frontend/app/composables/api/referenceResources.ts`:

```ts
export type ReferenceKey =
  | 'office-types' | 'departments' | 'positions' | 'units'
  | 'maintenance-categories' | 'problem-categories' | 'brands'
  | 'vendors' | 'provinces' | 'cities' | 'models'

export interface ReferenceField {
  key: string
  labelKey: string
  placeholder?: string
}

export interface ReferenceDescriptor {
  key: ReferenceKey
  labelKey: string
  fields: ReferenceField[]
}

const nameField: ReferenceField = { key: 'name', labelKey: 'masterdata.reference.fields.name' }
const codeField: ReferenceField = { key: 'code', labelKey: 'masterdata.reference.fields.code' }

export const referenceResources: ReferenceDescriptor[] = [
  { key: 'office-types', labelKey: 'masterdata.reference.resources.office-types', fields: [nameField] },
  { key: 'departments', labelKey: 'masterdata.reference.resources.departments', fields: [nameField] },
  { key: 'positions', labelKey: 'masterdata.reference.resources.positions', fields: [nameField] },
  { key: 'units', labelKey: 'masterdata.reference.resources.units', fields: [nameField, { key: 'symbol', labelKey: 'masterdata.reference.fields.symbol' }] },
  { key: 'maintenance-categories', labelKey: 'masterdata.reference.resources.maintenance-categories', fields: [nameField] },
  { key: 'problem-categories', labelKey: 'masterdata.reference.resources.problem-categories', fields: [nameField] },
  { key: 'brands', labelKey: 'masterdata.reference.resources.brands', fields: [nameField] },
  { key: 'vendors', labelKey: 'masterdata.reference.resources.vendors', fields: [nameField, { key: 'email', labelKey: 'masterdata.reference.fields.email' }, { key: 'phone', labelKey: 'masterdata.reference.fields.phone' }] },
  { key: 'provinces', labelKey: 'masterdata.reference.resources.provinces', fields: [nameField, codeField] },
  { key: 'cities', labelKey: 'masterdata.reference.resources.cities', fields: [nameField, codeField] },
  { key: 'models', labelKey: 'masterdata.reference.resources.models', fields: [nameField] }
]
```

- [ ] **Step 4: Create the reference mock stores**

Create `frontend/app/mock/reference.ts`:

```ts
import type { ReferenceRow } from '~/types'
import type { ReferenceKey } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'
import { createStore } from './helpers'

const seeds: Partial<Record<ReferenceKey, ReferenceRow[]>> = {
  provinces: [
    { id: 'p-1', name: 'DKI Jakarta', code: '31' },
    { id: 'p-2', name: 'Jawa Barat', code: '32' }
  ],
  cities: [
    { id: 'c-1', name: 'Jakarta Selatan', code: '3171' },
    { id: 'c-2', name: 'Bandung', code: '3273' }
  ],
  units: [
    { id: 'u-1', name: 'Unit', symbol: 'pcs' },
    { id: 'u-2', name: 'Set', symbol: 'set' }
  ],
  brands: [
    { id: 'b-1', name: 'Dell' },
    { id: 'b-2', name: 'HP' }
  ],
  vendors: [
    { id: 'v-1', name: 'PT Sumber Jaya', email: 'sales@sumberjaya.co.id', phone: '021-5550001' }
  ]
}

function makeStore(key: ReferenceKey) {
  const seed = seeds[key] ?? [{ id: `${key}-1`, name: `${key} contoh` }]
  return createStore<ReferenceRow>(seed)
}

export const referenceStores = Object.fromEntries(
  referenceResources.map(r => [r.key, makeStore(r.key)])
) as Record<ReferenceKey, ReturnType<typeof makeStore>>
```

- [ ] **Step 5: Create the composable**

Create `frontend/app/composables/api/useReference.ts`:

```ts
import type { ListQuery, Paginated, ReferenceRow } from '~/types'
import type { ReferenceKey } from './referenceResources'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { referenceStores } from '~/mock/reference'

export function useReference() {
  async function list(key: ReferenceKey, query: ListQuery = {}): Promise<Paginated<ReferenceRow>> {
    await fakeLatency()
    return paginate(filterBy(referenceStores[key].all(), query, ['name', 'code']), query)
  }

  async function create(key: ReferenceKey, input: Record<string, unknown>): Promise<ReferenceRow> {
    await fakeLatency()
    return referenceStores[key].insert({ id: generateId(), name: '', ...input } as ReferenceRow)
  }

  async function update(key: ReferenceKey, id: string, input: Record<string, unknown>): Promise<ReferenceRow> {
    await fakeLatency()
    const row = referenceStores[key].patch(id, input as Partial<ReferenceRow>)
    if (!row) throw new Error('masterdata.reference.errNotFound')
    return row
  }

  async function remove(key: ReferenceKey, id: string): Promise<void> {
    await fakeLatency()
    referenceStores[key].remove(id)
  }

  return { list, create, update, remove }
}
```

- [ ] **Step 6: Re-export from the mock barrel**

Edit `frontend/app/mock/index.ts` — add the line:

```ts
export * from './reference'
```

- [ ] **Step 7: Run test to verify it passes**

Run: `pnpm test reference-mock`
Expected: PASS.

- [ ] **Step 8: Typecheck + lint**

Run: `pnpm typecheck && pnpm lint`
Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add app/composables/api/referenceResources.ts app/mock/reference.ts app/composables/api/useReference.ts app/mock/index.ts test/unit/reference-mock.spec.ts
git commit -m "feat(frontend): add reference descriptors + mock stores + useReference"
```

---

### Task 7: Master Data Referensi page (`/master/reference`)

**Files:**
- Create: `frontend/app/pages/master/reference.vue`
- Modify: `frontend/i18n/locales/id.json` (add `masterdata.reference.*`)
- Modify: `frontend/i18n/locales/en.json` (add `masterdata.reference.*`)
- Test: `frontend/test/nuxt/master-reference.spec.ts`

**Interfaces:**
- Consumes: `useReference()` + `referenceResources`/`ReferenceKey` from Task 6; existing `PageHeader`, `DataToolbar`, `ResourceTable`, `FormModal`, `Can`, `useConfirm`.
- Produces: route `/master/reference` (permission `masterdata.global.manage`).

> **Mockup:** open `docs/design/Master Data Referensi.dc.html`. Layout = a resource switcher (`USelect`) that drives the page subtitle/`entityLabel`; below it a `ResourceTable` whose columns come from the selected resource's field descriptors, with add/edit (`FormModal`, dynamic fields) and delete (`useConfirm`).

- [ ] **Step 1: Add i18n keys**

Into the `"masterdata"` object in `frontend/i18n/locales/id.json`, add a `"reference"` key:

```json
"reference": {
  "title": "Master Data Referensi",
  "subtitle": "Kelola data referensi",
  "resourceLabel": "Jenis Referensi",
  "add": "Tambah",
  "empty": "Belum ada data",
  "createTitle": "Tambah Data",
  "editTitle": "Ubah Data",
  "deleteConfirm": "Hapus data ini?",
  "errNotFound": "Data tidak ditemukan.",
  "fields": { "name": "Nama", "code": "Kode", "symbol": "Simbol", "email": "Email", "phone": "Telepon" },
  "resources": {
    "office-types": "Jenis Kantor", "departments": "Departemen", "positions": "Jabatan",
    "units": "Satuan", "maintenance-categories": "Kategori Pemeliharaan",
    "problem-categories": "Kategori Masalah", "brands": "Merek", "vendors": "Vendor",
    "provinces": "Provinsi", "cities": "Kota", "models": "Model"
  }
}
```

Into the `"masterdata"` object in `frontend/i18n/locales/en.json`, add:

```json
"reference": {
  "title": "Reference Master Data",
  "subtitle": "Manage reference data",
  "resourceLabel": "Reference Type",
  "add": "Add",
  "empty": "No data yet",
  "createTitle": "Add Entry",
  "editTitle": "Edit Entry",
  "deleteConfirm": "Delete this entry?",
  "errNotFound": "Entry not found.",
  "fields": { "name": "Name", "code": "Code", "symbol": "Symbol", "email": "Email", "phone": "Phone" },
  "resources": {
    "office-types": "Office Types", "departments": "Departments", "positions": "Positions",
    "units": "Units", "maintenance-categories": "Maintenance Categories",
    "problem-categories": "Problem Categories", "brands": "Brands", "vendors": "Vendors",
    "provinces": "Provinces", "cities": "Cities", "models": "Models"
  }
}
```

- [ ] **Step 2: Write the failing runtime test**

Create `frontend/test/nuxt/master-reference.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import ReferencePage from '~/pages/master/reference.vue'

describe('Master Data Referensi page', () => {
  it('renders the title and the first resource rows after load', async () => {
    const wrapper = await mountSuspended(ReferencePage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Master Data Referensi')
    // default resource is the first descriptor (office-types) → seeded "office-types contoh"
    expect(html).toContain('office-types contoh')
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `pnpm test master-reference`
Expected: FAIL — cannot resolve `~/pages/master/reference.vue`.

- [ ] **Step 4: Create the page**

Create `frontend/app/pages/master/reference.vue`:

```vue
<script setup lang="ts">
import type { ReferenceRow } from '~/types'
import type { ReferenceKey } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'

definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useReference()

const resourceKey = ref<ReferenceKey>(referenceResources[0].key)
const descriptor = computed(() => referenceResources.find(r => r.key === resourceKey.value)!)

const resourceOptions = referenceResources.map(r => ({ value: r.key, label: t(r.labelKey) }))

const rows = ref<ReferenceRow[]>([])
const total = ref(0)
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const loading = ref(true)

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<Record<string, unknown>>({})

const columns = computed(() => descriptor.value.fields.map(f => ({
  accessorKey: f.key, header: t(f.labelKey)
})))

async function refresh() {
  loading.value = true
  const res = await api.list(resourceKey.value, { search: search.value, limit: limit.value, offset: offset.value })
  rows.value = res.data
  total.value = res.total
  loading.value = false
}

function resetForm() {
  for (const k of Object.keys(form)) delete form[k]
  for (const f of descriptor.value.fields) form[f.key] = ''
}

function openCreate() {
  editingId.value = undefined
  resetForm()
  formOpen.value = true
}

function openEdit(row: ReferenceRow) {
  editingId.value = row.id
  resetForm()
  for (const f of descriptor.value.fields) form[f.key] = row[f.key] ?? ''
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    if (editingId.value) await api.update(resourceKey.value, editingId.value, { ...form })
    else await api.create(resourceKey.value, { ...form })
    formOpen.value = false
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete(row: ReferenceRow) {
  const ok = await confirm({ title: t('common.delete'), description: t('masterdata.reference.deleteConfirm') })
  if (!ok) return
  await api.remove(resourceKey.value, row.id)
  await refresh()
}

watch(resourceKey, () => {
  offset.value = 0
  search.value = ''
  refresh()
})
watch([search, offset], refresh)
onMounted(refresh)
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.reference.title')"
      :subtitle="t(descriptor.labelKey)"
    >
      <template #actions>
        <Can permission="masterdata.global.manage">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.reference.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <DataToolbar
      v-model:search="search"
      @reset="search = ''"
    >
      <template #filters>
        <USelect
          v-model="resourceKey"
          :items="resourceOptions"
          class="w-56"
          :aria-label="t('masterdata.reference.resourceLabel')"
        />
      </template>
    </DataToolbar>

    <ResourceTable
      :rows="rows"
      :columns="columns"
      :loading="loading"
      :total="total"
      :limit="limit"
      :offset="offset"
      :empty-title="t('masterdata.reference.empty')"
      @update:offset="offset = $event"
    >
      <template #row-actions="{ row }">
        <Can permission="masterdata.global.manage">
          <div class="flex gap-1">
            <UButton
              color="neutral"
              variant="ghost"
              icon="i-lucide-pencil"
              size="xs"
              @click="openEdit(row as ReferenceRow)"
            />
            <UButton
              color="error"
              variant="ghost"
              icon="i-lucide-trash-2"
              size="xs"
              @click="onDelete(row as ReferenceRow)"
            />
          </div>
        </Can>
      </template>
    </ResourceTable>

    <FormModal
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.reference.editTitle') : t('masterdata.reference.createTitle')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField
          v-for="field in descriptor.fields"
          :key="field.key"
          :label="t(field.labelKey)"
        >
          <UInput
            :model-value="form[field.key] as string"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
        </UFormField>
      </div>
    </FormModal>
  </div>
</template>
```

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm test master-reference`
Expected: PASS.

- [ ] **Step 6: Typecheck + lint**

Run: `pnpm typecheck && pnpm lint`
Expected: no errors.

- [ ] **Step 7: Run the full unit + runtime suite**

Run: `pnpm test`
Expected: all suites PASS (mock-store, offices-mock, employees-mock, reference-mock, all three page specs, plus the pre-existing specs).

- [ ] **Step 8: Commit**

```bash
git add app/pages/master/reference.vue i18n/locales/id.json i18n/locales/en.json test/nuxt/master-reference.spec.ts
git commit -m "feat(frontend): build master data referensi (reference) screen"
```

---

### Task 8: Offices CRUD e2e (Playwright)

**Files:**
- Create: `frontend/e2e/master-offices.spec.ts`

**Interfaces:**
- Consumes: the running app (`pnpm preview`) + the `/master/offices` route from Task 3. Login uses the real backend + seeded admin (same as `e2e/login.spec.ts`), then the mock-backed offices page is exercised within one page session (the in-memory store persists across SPA navigation but resets on reload).

> **Note:** this e2e depends on the backend stack + seeded admin only for **login** (CI's `e2e` job provides them). The offices data itself is the frontend mock. When the real offices backend is wired later, this spec keeps working unchanged.

- [ ] **Step 1: Write the e2e spec**

Create `frontend/e2e/master-offices.spec.ts`:

```ts
import { test, expect } from '@playwright/test'

const EMAIL = process.env.E2E_EMAIL || 'admin@inventra.local'
const PASSWORD = process.env.E2E_PASSWORD || 'admin12345'

async function login(page) {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

test.describe('Master Data Kantor (mock-backed)', () => {
  test('creates an office and sees it in the tree', async ({ page }) => {
    await login(page)
    await page.goto('/master/offices')

    await expect(page.getByRole('heading', { name: 'Master Data Kantor' })).toBeVisible()
    // Seeded office is present.
    await expect(page.getByText('Kantor Pusat')).toBeVisible()

    // Open the create form and add a new office.
    await page.getByRole('button', { name: 'Tambah Kantor' }).click()
    await page.getByLabel('Nama Kantor').fill('Cabang E2E')
    await page.getByLabel('Kode').fill('E2E01')
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // New office appears in the tree.
    await expect(page.getByText('Cabang E2E')).toBeVisible()
  })
})
```

- [ ] **Step 2: Run the e2e locally (requires backend stack + seeded admin up)**

Run: `pnpm test:e2e --grep "Master Data Kantor"`
Expected: PASS (the new office appears). If the backend stack is not running locally, this is expected to fail at login — the CI `e2e` job runs it with the stack up.

- [ ] **Step 3: Commit**

```bash
git add e2e/master-offices.spec.ts
git commit -m "test(frontend): add offices CRUD e2e"
```

---

## Final verification

- [ ] Run the full gate from `frontend/`:

```bash
pnpm lint && pnpm typecheck && pnpm test && pnpm build
```

Expected: all green. (E2E runs separately and needs the backend stack + seeded admin — CI's `e2e` job covers it.)

- [ ] Open each page in the browser (`pnpm dev`) and compare against its `docs/design` mockup in **light and dark mode**: `/master/offices`, `/master/employees`, `/master/reference`. Confirm loading skeletons, empty states, create/edit forms, and delete confirm all behave.

## Self-Review notes (coverage vs spec)

- Offices tree + detail + CRUD + mock scope rejection → Tasks 2–3. ✓
- Employees list + CRUD + status badge → Tasks 4–5. ✓
- Reference generic engine (11 resources, descriptor-driven columns/forms) → Tasks 6–7. ✓
- `composables/api/` interface returning `Paginated<T>` for mechanical swap-later → all composables. ✓
- i18n id/en for every string → Tasks 3, 5, 7. ✓
- Permission gating (route + buttons) → `definePageMeta` + `<Can>` in each page. ✓
- Tests: unit (mock stores/pure fns), runtime (`mountSuspended` per screen), e2e (offices) → Tasks 1–8. ✓
- Floors/rooms management → intentionally out of scope this phase (spec bagian Out of scope).
