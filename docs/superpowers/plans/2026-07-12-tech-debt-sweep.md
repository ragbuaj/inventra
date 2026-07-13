# Tech-Debt Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land three independent tech-debt items — field-permission enforcement for employees (plus helper standardization + leak fixes), an enriched audit response (actor role, office name, derived summary, actor filter), and resource-agnostic async searchable pickers replacing every `limit:100` client-side picker.

**Architecture:** Backend changes are additive (one authz helper, employee map DTO + fieldSvc wiring, two audit SQL joins, one depreciation mask fix). Frontend introduces one reusable `AsyncSearchPicker.vue` that all pickers adopt; audit gains columns + a client-side localized summary. Tasks are ordered by dependency: the picker component is built first because the audit actor filter reuses it.

**Tech Stack:** Go 1.25 + Gin + sqlc/pgx, PostgreSQL 16, Redis. Nuxt 4 (SPA) + Nuxt UI (`U*`) + Vitest/@nuxt/test-utils + Playwright. i18n (id default, en).

## Global Constraints

- **Branch:** `feat/tech-debt-sweep` (already created; spec committed).
- **Commits:** Conventional Commits, lowercase, scoped: `feat(authz):`, `fix(security):`, `feat(audit):`, `feat(frontend):`, `refactor(frontend):`. **No AI/Claude co-author trailers.**
- **Field-permission:** entity keys are free-form strings — no schema/`validateFieldPerms` change. `FilterView` is default-allow (a field with no policy stays visible) and view-only (no edit masking). Employee entity key is exactly **`"employees"`** (matches its scope-module key).
- **Audit:** `office_id` has **no FK** and is nullable — all joins are `LEFT JOIN` and must tolerate NULL / soft-deleted rows. Name resolution stays **inside the audit SQL query** (never via user/masterdata services) so an `audit.view`-only viewer isn't blocked. Actor role is the **current** role (not snapshotted).
- **Pickers:** backend already supports `search` server-side for offices, employees, all reference resources, and categories — **no backend change for Part 3**. Use the hand-rolled dropdown pattern (`UInput` + `<ul>`), **never `USelectMenu`** (focus-trap breaks e2e). Number/date/dropdown component-first rules from CLAUDE.md apply. Dropdowns must render a "No Data" empty state.
- **i18n:** every user-facing string in `i18n/locales/{id,en}.json`; no hardcoded UI text.
- **Verification gate (must be green before claiming done):** backend `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./... -p 1`, Spectral lint; frontend `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`; affected `pnpm test:e2e`.
- **On completion:** tick `docs/PROGRESS.md` item 49(c) sub-items and record approved deviations.

---

## Part 3 — Async searchable pickers (built first: the audit actor filter depends on it)

### Task 1: `AsyncSearchPicker.vue` generic component

**Files:**
- Create: `frontend/app/components/AsyncSearchPicker.vue`
- Test: `frontend/test/nuxt/async-search-picker.spec.ts`

**Interfaces:**
- Produces: a component with props
  `{ modelValue: string | null, searchFn: (term: string) => Promise<PickerItem[]>, resolveFn?: (id: string) => Promise<PickerItem | null>, placeholder: string, disabled?: boolean, testid?: string }`
  and emits `update:modelValue: [id: string | null]`. `PickerItem = { id: string, label: string, sublabel?: string }`.
- Exposes the `PickerItem` type via `frontend/app/types` (add `export interface PickerItem { id: string, label: string, sublabel?: string }` to the existing types barrel).

- [ ] **Step 1: Add the `PickerItem` type**

In `frontend/app/types/index.ts` (the types barrel — confirm the exact file with `ls frontend/app/types`), add:

```ts
export interface PickerItem {
  id: string
  label: string
  sublabel?: string
}
```

- [ ] **Step 2: Write the failing component test**

Create `frontend/test/nuxt/async-search-picker.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import AsyncSearchPicker from '~/components/AsyncSearchPicker.vue'
import type { PickerItem } from '~/types'

const items: PickerItem[] = [
  { id: 'o1', label: 'Kantor Pusat', sublabel: 'KP-001' },
  { id: 'o2', label: 'Kanwil Jakarta', sublabel: 'KW-002' }
]

function picker(overrides: Record<string, unknown> = {}) {
  return mountSuspended(AsyncSearchPicker, {
    props: {
      modelValue: null,
      searchFn: vi.fn(async (term: string) => items.filter(i => i.label.toLowerCase().includes(term.toLowerCase()))),
      placeholder: 'Cari kantor',
      testid: 'office',
      ...overrides
    }
  })
}

describe('AsyncSearchPicker', () => {
  it('renders the input with placeholder and testid', async () => {
    const w = await picker()
    const input = w.find('[data-testid="office-picker-input"]')
    expect(input.exists()).toBe(true)
    expect(input.attributes('placeholder')).toBe('Cari kantor')
  })

  it('searches (debounced) and lists results, then emits the id on select', async () => {
    vi.useFakeTimers()
    const w = await picker()
    await w.find('[data-testid="office-picker-input"]').setValue('kanwil')
    vi.advanceTimersByTime(300)
    await flushPromises()
    vi.useRealTimers()
    const rows = w.findAll('[data-testid="office-picker-item"]')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('Kanwil Jakarta')
    await rows[0]!.trigger('click')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['o2'])
  })

  it('shows a No Data empty state when search yields nothing', async () => {
    vi.useFakeTimers()
    const w = await picker({ searchFn: vi.fn(async () => []) })
    await w.find('[data-testid="office-picker-input"]').setValue('zzz')
    vi.advanceTimersByTime(300)
    await flushPromises()
    vi.useRealTimers()
    expect(w.find('[data-testid="office-picker-empty"]').exists()).toBe(true)
  })

  it('resolves and displays a preselected value via resolveFn', async () => {
    const resolveFn = vi.fn(async (id: string) => items.find(i => i.id === id) ?? null)
    const w = await picker({ modelValue: 'o1', resolveFn })
    await flushPromises()
    expect(resolveFn).toHaveBeenCalledWith('o1')
    expect((w.find('[data-testid="office-picker-input"]').element as HTMLInputElement).value).toBe('Kantor Pusat')
  })

  it('does not search or open when disabled', async () => {
    vi.useFakeTimers()
    const searchFn = vi.fn(async () => items)
    const w = await picker({ disabled: true, searchFn })
    await w.find('[data-testid="office-picker-input"]').setValue('kan')
    vi.advanceTimersByTime(300)
    await flushPromises()
    vi.useRealTimers()
    expect(searchFn).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `cd frontend && pnpm test async-search-picker`
Expected: FAIL — cannot resolve `~/components/AsyncSearchPicker.vue`.

- [ ] **Step 4: Implement the component**

Create `frontend/app/components/AsyncSearchPicker.vue` (generalized from `AssetSearchPicker.vue`, keeping its debounce / seq-guard / suppress / outside-click logic):

```vue
<script setup lang="ts">
import type { PickerItem } from '~/types'

const props = withDefaults(defineProps<{
  modelValue: string | null
  searchFn: (term: string) => Promise<PickerItem[]>
  resolveFn?: (id: string) => Promise<PickerItem | null>
  placeholder: string
  disabled?: boolean
  testid?: string
}>(), {
  resolveFn: undefined,
  disabled: false,
  testid: 'async'
})

const emit = defineEmits<{ 'update:modelValue': [id: string | null] }>()

const DEBOUNCE_MS = 300
const { t } = useI18n()

const query = ref('')
const results = ref<PickerItem[]>([])
const loading = ref(false)
const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)

let debounceTimer: ReturnType<typeof setTimeout> | undefined
let seq = 0
let suppressNextSearch = false

async function runSearch(term: string) {
  const mine = ++seq
  loading.value = true
  try {
    const found = await props.searchFn(term)
    if (mine !== seq) return
    results.value = found
  } catch {
    if (mine === seq) results.value = []
  } finally {
    if (mine === seq) loading.value = false
  }
}

watch(query, (value) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (suppressNextSearch) {
    suppressNextSearch = false
    return
  }
  if (props.disabled) return
  const term = value.trim()
  if (!term) {
    results.value = []
    loading.value = false
    isOpen.value = false
    return
  }
  isOpen.value = true
  debounceTimer = setTimeout(() => runSearch(term), DEBOUNCE_MS)
})

// Resolve a preselected id into its display label (may be outside the search page).
watch(() => props.modelValue, async (id) => {
  if (!id) {
    if (!isOpen.value) query.value = ''
    return
  }
  if (props.resolveFn) {
    const item = await props.resolveFn(id)
    if (item && props.modelValue === id) {
      suppressNextSearch = query.value !== item.label
      query.value = item.label
    }
  }
}, { immediate: true })

function select(item: PickerItem) {
  if (debounceTimer) clearTimeout(debounceTimer)
  suppressNextSearch = query.value !== item.label
  query.value = item.label
  isOpen.value = false
  results.value = []
  emit('update:modelValue', item.id)
}

function onOutsideClick(event: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onOutsideClick))
onUnmounted(() => {
  document.removeEventListener('mousedown', onOutsideClick)
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>

<template>
  <div
    ref="containerRef"
    class="relative"
  >
    <UInput
      v-model="query"
      :data-testid="`${testid}-picker-input`"
      :placeholder="placeholder"
      :disabled="disabled"
      icon="i-lucide-search"
      class="w-full"
    />
    <div
      v-if="isOpen"
      class="absolute z-10 mt-1 w-full bg-default border border-default rounded-lg shadow-lg overflow-hidden"
    >
      <div
        v-if="loading"
        class="p-3 space-y-2"
      >
        <USkeleton
          v-for="n in 3"
          :key="n"
          class="h-[34px] w-full rounded-lg"
        />
      </div>
      <div
        v-else-if="results.length === 0"
        :data-testid="`${testid}-picker-empty`"
        class="py-6 px-4 text-center text-xs text-muted"
      >
        {{ t('common.pickerEmpty') }}
      </div>
      <ul
        v-else
        class="max-h-[260px] overflow-y-auto py-1"
      >
        <li
          v-for="item in results"
          :key="item.id"
          :data-testid="`${testid}-picker-item`"
          class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-muted"
          @click="select(item)"
        >
          <span class="min-w-0 flex-1">
            <span class="block text-[13px] font-medium truncate">{{ item.label }}</span>
            <span
              v-if="item.sublabel"
              class="block text-[11px] text-dimmed truncate"
            >{{ item.sublabel }}</span>
          </span>
        </li>
      </ul>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Add the i18n key**

In `frontend/i18n/locales/id.json` add `"common": { ..., "pickerEmpty": "Tidak ada data" }`; in `en.json` add `"pickerEmpty": "No data"`. (Keep the existing `common.assetPickerEmpty` for now — Task 2 migrates it.)

- [ ] **Step 6: Run the test to verify it passes**

Run: `cd frontend && pnpm test async-search-picker`
Expected: PASS (5 tests).

- [ ] **Step 7: Lint + typecheck**

Run: `cd frontend && pnpm lint && pnpm typecheck`
Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add frontend/app/components/AsyncSearchPicker.vue frontend/test/nuxt/async-search-picker.spec.ts frontend/app/types frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "feat(frontend): add resource-agnostic AsyncSearchPicker component"
```

---

### Task 2: Refactor `AssetSearchPicker.vue` to wrap `AsyncSearchPicker`

**Files:**
- Modify: `frontend/app/components/AssetSearchPicker.vue`
- Test: `frontend/test/nuxt/asset-search-picker.spec.ts` (must stay green — do not weaken)

**Interfaces:**
- Consumes: `AsyncSearchPicker` from Task 1.
- Produces: unchanged public API — props `{ statuses, placeholder, hint?, disabled?, officeNames? }`, emits `select: [asset: Asset]`, testids `asset-picker-input` / `asset-picker-item` / `asset-picker-hint`.

- [ ] **Step 1: Run the existing spec to establish the green baseline**

Run: `cd frontend && pnpm test asset-search-picker`
Expected: PASS (baseline). Record the passing test names.

- [ ] **Step 2: Rewrite `AssetSearchPicker.vue` to compose the generic picker**

Keep the asset-specific search (multi-status merge) and label rendering by delegating the dropdown to `AsyncSearchPicker`. The wrapper maps `Asset` → `PickerItem` and re-emits the full asset on select by keeping a local id→asset map:

```vue
<script setup lang="ts">
import type { Asset, AssetStatus, PickerItem } from '~/types'

const props = withDefaults(defineProps<{
  statuses: AssetStatus[]
  placeholder: string
  hint?: string
  disabled?: boolean
  officeNames?: Map<string, string>
}>(), { hint: undefined, disabled: false, officeNames: () => new Map() })

const emit = defineEmits<{ select: [asset: Asset] }>()

const assetsApi = useAssets()
const byId = new Map<string, Asset>()
const selected = ref<string | null>(null)

async function searchFn(term: string): Promise<PickerItem[]> {
  const pages = await Promise.all(
    props.statuses.map(status => assetsApi.list({ search: term, status, limit: 20 }))
  )
  const merged = new Map<string, Asset>()
  for (const page of pages) for (const a of page.data) merged.set(a.id, a)
  byId.clear()
  const items: PickerItem[] = []
  for (const a of merged.values()) {
    byId.set(a.id, a)
    items.push({ id: a.id, label: a.name, sublabel: `${a.asset_tag} · ${props.officeNames?.get(a.office_id) ?? '—'}` })
  }
  return items
}

function onUpdate(id: string | null) {
  selected.value = id
  const asset = id ? byId.get(id) : undefined
  if (asset) emit('select', asset)
}
</script>

<template>
  <div>
    <AsyncSearchPicker
      v-model="selected"
      testid="asset"
      :search-fn="searchFn"
      :placeholder="placeholder"
      :disabled="disabled"
      @update:model-value="onUpdate"
    />
    <p
      v-if="hint"
      data-testid="asset-picker-hint"
      class="text-xs text-muted mt-1"
    >
      {{ hint }}
    </p>
  </div>
</template>
```

- [ ] **Step 3: Reconcile the empty-state key**

The asset spec may assert `common.assetPickerEmpty`. Since the dropdown now renders `common.pickerEmpty`, update `asset-search-picker.spec.ts` only where it asserts the empty **text** (change the expected key/text to `pickerEmpty`). Do not change behavioral assertions (debounce, selection, disabled). Then remove the now-unused `common.assetPickerEmpty` from both locale files.

- [ ] **Step 4: Run the asset spec to verify still green**

Run: `cd frontend && pnpm test asset-search-picker`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/components/AssetSearchPicker.vue frontend/test/nuxt/asset-search-picker.spec.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "refactor(frontend): make AssetSearchPicker wrap AsyncSearchPicker"
```

---

### Task 3: Picker source adapters (`useOfficePicker`, `useEmployeePicker`, `useReferencePicker`)

**Files:**
- Create: `frontend/app/composables/usePickerSource.ts`
- Test: `frontend/test/unit/use-picker-source.spec.ts`

**Interfaces:**
- Consumes: `useOffices().list/get`, `useEmployees().list/get`, `useReference(resource).list/get` (verify the reference composable's exact list/get signature in `frontend/app/composables/api/useReference.ts`).
- Produces: three factories, each returning `{ searchFn: (term) => Promise<PickerItem[]>, resolveFn: (id) => Promise<PickerItem | null> }`:
  - `useOfficePicker()` — label `office.name`, sublabel `office.code`.
  - `useEmployeePicker()` — label `employee.name`, sublabel `employee.code`.
  - `useReferencePicker(resource: string)` — label `row.name`, sublabel `row.code` if present.

- [ ] **Step 1: Write the failing unit test**

Create `frontend/test/unit/use-picker-source.spec.ts` (node env — mock the api composables):

```ts
import { describe, it, expect, vi } from 'vitest'

const listOffices = vi.fn(async () => ({ data: [{ id: 'o1', name: 'Pusat', code: 'KP-001' }], total: 1 }))
const getOffice = vi.fn(async (id: string) => ({ id, name: 'Pusat', code: 'KP-001' }))
vi.stubGlobal('useOffices', () => ({ list: listOffices, get: getOffice }))

const { useOfficePicker } = await import('~/composables/usePickerSource')

describe('useOfficePicker', () => {
  it('searchFn maps offices to picker items (label=name, sublabel=code)', async () => {
    const { searchFn } = useOfficePicker()
    const items = await searchFn('pus')
    expect(listOffices).toHaveBeenCalledWith({ search: 'pus', limit: 20 })
    expect(items).toEqual([{ id: 'o1', label: 'Pusat', sublabel: 'KP-001' }])
  })

  it('resolveFn maps a single office by id', async () => {
    const { resolveFn } = useOfficePicker()
    expect(await resolveFn('o1')).toEqual({ id: 'o1', label: 'Pusat', sublabel: 'KP-001' })
  })
})
```

(Confirm the test harness's alias/auto-import strategy — mirror how existing `frontend/test/unit/*.spec.ts` stub Nuxt auto-imports; if they use `mockNuxtImport`, use that instead of `stubGlobal`.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd frontend && pnpm test use-picker-source`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the composable**

Create `frontend/app/composables/usePickerSource.ts`:

```ts
import type { PickerItem } from '~/types'

export function useOfficePicker() {
  const api = useOffices()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20 })
      return res.data.map(o => ({ id: o.id, label: o.name, sublabel: o.code }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      const o = await api.get(id)
      return o ? { id: o.id, label: o.name, sublabel: o.code } : null
    }
  }
}

export function useEmployeePicker() {
  const api = useEmployees()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20 })
      return res.data.map(e => ({ id: e.id, label: e.name, sublabel: e.code }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      const e = await api.get(id)
      return e ? { id: e.id, label: e.name, sublabel: e.code } : null
    }
  }
}

export function useReferencePicker(resource: string) {
  const api = useReference(resource)
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20 })
      return res.data.map((r: Record<string, unknown>) => ({
        id: String(r.id), label: String(r.name), sublabel: r.code ? String(r.code) : undefined
      }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      const r = await api.get(id)
      return r ? { id: String(r.id), label: String(r.name), sublabel: r.code ? String(r.code) : undefined } : null
    }
  }
}
```

Adjust `useReference(resource)`'s method names to the real ones after reading `useReference.ts` (it may be `list(resource, query)` rather than `useReference(resource).list(query)` — match the actual signature and update the test accordingly).

- [ ] **Step 4: Run to verify it passes**

Run: `cd frontend && pnpm test use-picker-source`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/usePickerSource.ts frontend/test/unit/use-picker-source.spec.ts
git commit -m "feat(frontend): picker source adapters for office/employee/reference"
```

---

### Task 4: Swap the office & employee **form** pickers

**Files (modify — each currently binds a `USelect` to a `{limit:100}` options array):**
- `frontend/app/components/asset/AssetForm.vue:297,473-476` (office)
- `frontend/app/pages/master/employees.vue:114,405-432` (office; dept/position stay reference-backed via Task 5's reference picker)
- `frontend/app/pages/assignment.vue:91,271-277` (employee recipient)
- `frontend/app/pages/transfers.vue:122` (source/dest office)
- `frontend/app/pages/disposals.vue:68` (office)
- `frontend/app/pages/stock-opname.vue:57` (office)
- `frontend/app/pages/settings/users.vue` + `frontend/app/composables/api/useUsers.ts:81-82` (office + employee lookups)
- Tests: the matching `frontend/test/nuxt/*.spec.ts` for each screen.

**Interfaces:**
- Consumes: `AsyncSearchPicker` (Task 1), `useOfficePicker` / `useEmployeePicker` (Task 3).

**Transform recipe (apply to each office field; employee is identical with `useEmployeePicker`):**

Before:
```vue
<USelect v-model="officeId" :items="officeOptions" ... />
```
```ts
const officeOptions = ref<{ value: string, label: string }[]>([])
onMounted(async () => {
  const res = await officesApi.list({ limit: 100 })
  officeOptions.value = res.data.map(o => ({ value: o.id, label: o.name }))
})
```

After:
```vue
<AsyncSearchPicker
  v-model="officeId"
  testid="office"
  :search-fn="office.searchFn"
  :resolve-fn="office.resolveFn"
  :placeholder="t('common.searchOffice')"
/>
```
```ts
const office = useOfficePicker()
// remove officeOptions + its onMounted eager fetch
```

- [ ] **Step 1: Update the specs first (they currently assert `{ limit: 100 }`)**

For each screen spec that asserts `expect(list).toHaveBeenCalledWith({ limit: 100 })` on offices/employees, change the assertion to reflect the async picker: assert the picker renders (`[data-testid="office-picker-input"]`) and that selecting drives the model. Where a test pre-set a selected office by seeding options, switch to stubbing `resolveFn` to return the label. Add a test that typing a term calls `searchFn` with `{ search, limit: 20 }`.

- [ ] **Step 2: Run the specs to verify they now fail against the old markup**

Run: `cd frontend && pnpm test master-employees assignment transfers disposals stock-opname users assets-form`
Expected: FAIL (old `USelect`/options markup still present).

- [ ] **Step 3: Apply the transform recipe to each file listed above**

Replace each office/employee `USelect`+options block with the `AsyncSearchPicker` + `usePickerSource` adapter. Remove now-dead `officeOptions`/`employees` refs and their `{limit:100}` fetches. For `useUsers.ts`, replace the `/offices?limit=100` + `/employees?limit=100` lookups: the user form's office/employee fields become `AsyncSearchPicker`s in `settings/users.vue`, and the id→name display for the users table uses `resolveFn` on demand (or keep a small resolve cache).

- [ ] **Step 4: Add i18n keys**

`common.searchOffice` / `common.searchEmployee` (id: "Cari kantor…" / "Cari pegawai…"; en: "Search office…" / "Search employee…").

- [ ] **Step 5: Run the specs + typecheck**

Run: `cd frontend && pnpm test master-employees assignment transfers disposals stock-opname users assets-form && pnpm typecheck`
Expected: PASS + no type errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/components/asset/AssetForm.vue frontend/app/pages/master/employees.vue frontend/app/pages/assignment.vue frontend/app/pages/transfers.vue frontend/app/pages/disposals.vue frontend/app/pages/stock-opname.vue frontend/app/pages/settings/users.vue frontend/app/composables/api/useUsers.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/nuxt
git commit -m "refactor(frontend): async office/employee form pickers replace limit:100"
```

---

### Task 5: Swap reference/category form pickers (brand/model/unit/vendor/category/problem/maintenance-category)

**Files (modify):**
- `frontend/app/components/asset/AssetForm.vue:298-301` (category, brand, model, unit)
- `frontend/app/pages/maintenance/*Slideover.vue` (maintenance-category, problem-category, vendor)
- `frontend/app/pages/master/reference.vue:88` (FK pickers cities→provinces, models→brands)
- `frontend/app/pages/master/employees.vue` dept/position selects (reference: departments, positions)
- Tests: matching specs.

**Interfaces:**
- Consumes: `AsyncSearchPicker` (Task 1), `useReferencePicker(resource)` (Task 3). Category uses `useCategories` — add a `useCategoryPicker()` to `usePickerSource.ts` mirroring the office one (label `name`, sublabel `code`) against `useCategories().list({search,limit})`.

- [ ] **Step 1: Extend `usePickerSource.ts` with `useCategoryPicker`**

Add (and a unit test case in `use-picker-source.spec.ts`):

```ts
export function useCategoryPicker() {
  const api = useCategories()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20 })
      return res.data.map(c => ({ id: c.id, label: c.name, sublabel: c.code ?? undefined }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      const c = await api.get(id)
      return c ? { id: c.id, label: c.name, sublabel: c.code ?? undefined } : null
    }
  }
}
```

Confirm `useCategories` exposes `get(id)`; if not, add it (mirrors `useOffices.get`).

- [ ] **Step 2: Update the affected specs to expect the picker**

Same pattern as Task 4 Step 1 for each reference/category field.

- [ ] **Step 3: Run to verify fail, then apply the transform**

Run: `cd frontend && pnpm test assets-form maintenance master-reference master-employees`
Expected: FAIL first; then replace each reference/category `USelect`+`{limit:100}` block with `AsyncSearchPicker` + `useReferencePicker('brands'|'models'|'units'|'vendors'|'problem-categories'|'maintenance-categories'|'departments'|'positions')` or `useCategoryPicker()`. For dependent pickers (models filtered by brand, cities by province), pass the parent id into the `searchFn` closure (wrap: `(term) => useReferencePicker('models').searchFn(term)` then filter client-side by `brand_id`, OR extend the adapter to accept a `parentFilter`). Keep the existing FK-dependency behavior.

- [ ] **Step 4: i18n keys** for each `common.search<Resource>` placeholder (id + en).

- [ ] **Step 5: Run specs + typecheck**

Run: `cd frontend && pnpm test assets-form maintenance master-reference master-employees use-picker-source && pnpm typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/components/asset/AssetForm.vue frontend/app/pages/maintenance frontend/app/pages/master/reference.vue frontend/app/pages/master/employees.vue frontend/app/composables/usePickerSource.ts frontend/i18n/locales frontend/test
git commit -m "refactor(frontend): async reference/category form pickers replace limit:100"
```

---

### Task 6: Swap the **filter** dropdowns (office/employee) + add clear option

**Files (modify):** `frontend/app/pages/assets/index.vue:194`, `frontend/app/pages/reports.vue:310`, `frontend/app/pages/depreciation.vue:142`, `frontend/app/pages/approval.vue:288`, `frontend/app/pages/index.vue:83`, `frontend/app/pages/master/offices.vue:209` (table list stays a real list, not a picker — see note), plus their specs.

**Note:** `master/offices.vue:209` and `master/employees.vue:103` load the **table** with `{limit:100}` — that's a paginated list view, not a picker. Convert those to real server-side pagination (limit 20 + offset + a search box) rather than an `AsyncSearchPicker`. The `assets/[tag]/index.vue:321`, `assets/label.vue:165` id→name **display maps** stay as maps but switch to `get(id)`-on-demand where the id may be outside the first page.

- [ ] **Step 1: Update specs** for each filter page to expect the async picker with a null/"Semua" clear option (assert clearing emits `null`).

- [ ] **Step 2: Run to verify fail; apply transform**

Filter pickers use `AsyncSearchPicker` bound to `useOfficePicker`/`useEmployeePicker`; a "Semua" clear is a small button/`x` that emits `update:modelValue = null`. For the two master **tables**, wire a `UInput` search box → `list({ search, limit: 20, offset })` with pagination controls (follow the existing paginated-table pattern used elsewhere, e.g. the katalog aset list).

- [ ] **Step 3: Run specs + typecheck**

Run: `cd frontend && pnpm test assets-list reports depreciation approval dashboard master-offices master-employees && pnpm typecheck`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/pages frontend/test frontend/i18n/locales
git commit -m "refactor(frontend): async office/employee filter pickers + paginated master tables"
```

---

### Task 7: Update e2e specs that were forced to API-only picker setup

**Files (modify):** `frontend/e2e/assignment.spec.ts`, `frontend/e2e/import-masterdata.spec.ts` (office case), and any e2e that commented "cannot reliably be selected through a UI dropdown". Add a small helper in the e2e support to drive `AsyncSearchPicker` (fill `[data-testid="<x>-picker-input"]`, wait for `[data-testid="<x>-picker-item"]`, click the match).

- [ ] **Step 1: Add the picker helper** in the e2e helpers module (mirror existing helpers). Signature: `async function pickAsync(page, testid, term, matchText)`.

- [ ] **Step 2: Rewrite the assignment e2e recipient selection** to use `pickAsync(page, 'employee', code, name)` instead of API-only setup, honoring the "fill text fields before opening a picker popover" and persistent-data-uniqueness conventions.

- [ ] **Step 3: Run the affected e2e** (needs stack up + seeded admin)

Run: `cd frontend && pnpm test:e2e assignment import-masterdata`
Expected: PASS. If a fresh row still isn't reliably visible, keep API creation but assert selection via the picker.

- [ ] **Step 4: Commit**

```bash
git add frontend/e2e
git commit -m "test(e2e): drive async pickers instead of API-only setup"
```

---

## Part 2 — Enriched audit response

### Task 8: Audit SQL joins — actor role + office name

**Files:**
- Modify: `backend/db/queries/audit.sql:19-38` (`ListAuditLogs` only — the count needs no descriptive columns)
- Regenerate: `backend/db/sqlc/` via `sqlc generate`
- Modify: `backend/internal/audit/dto.go:19-48` (`auditToMap`)
- Modify: `backend/api/openapi.yaml` (audit row schema)
- Test: `backend/internal/audit/audit_integration_test.go`

**Interfaces:**
- Produces: `ListAuditLogsRow` gains `ActorRole *string` (or the sqlc-generated nullable type) and `OfficeName *string`. `auditToMap` adds `actor.role` and top-level `office_name`.

- [ ] **Step 1: Write the failing integration test**

In `backend/internal/audit/audit_integration_test.go` add a test asserting the enriched response. Follow the file's existing setup (seed a user with a known role + office, record one audit row, list it):

```go
func TestListAudit_IncludesRoleAndOfficeName(t *testing.T) {
    // ... existing harness setup: seed office "KP Test", role "auditor-role",
    // a user in that office/role, and record one audit row with that office_id ...
    rows := listAudit(t, /* as the seeded viewer */)
    require.Len(t, rows, 1)
    actor := rows[0]["actor"].(map[string]any)
    require.Equal(t, "auditor-role", actor["role"])
    require.Equal(t, "KP Test", rows[0]["office_name"])
}

func TestListAudit_OfficeNameNullWhenOfficeMissing(t *testing.T) {
    // record an audit row with office_id = a random UUID not in masterdata.offices
    rows := listAudit(t, /* all-scope viewer */)
    require.Nil(t, findByEntity(rows, "orphan")["office_name"])
}
```

(Match the actual helper names in the existing integration test; if there's no `listAudit` helper, call the handler through the test server the file already stands up.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test -tags=integration ./internal/audit/ -run TestListAudit_Includes`
Expected: FAIL — `actor["role"]` / `office_name` absent.

- [ ] **Step 3: Extend `ListAuditLogs` SQL**

Edit `backend/db/queries/audit.sql` `ListAuditLogs` SELECT + JOINs:

```sql
-- name: ListAuditLogs :many
SELECT
  a.*,
  u.name  AS actor_name,
  u.email AS actor_email,
  ro.name AS actor_role,
  o.name  AS office_name
FROM audit.audit_logs a
LEFT JOIN identity.users u ON u.id = a.actor_id
LEFT JOIN identity.roles ro ON ro.id = u.role_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id
WHERE (sqlc.arg(all_scope)::bool OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR a.actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(entity_type)::text IS NULL OR a.entity_type = sqlc.narg(entity_type))
  AND (sqlc.narg(action)::shared.audit_action IS NULL OR a.action = sqlc.narg(action))
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR a.created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR a.created_at <= sqlc.narg(to_ts))
  AND (
    sqlc.arg(search)::text = ''
    OR a.entity_type ILIKE '%' || sqlc.arg(search) || '%'
    OR a.entity_id::text ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY a.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);
```

- [ ] **Step 4: Regenerate sqlc**

Run: `cd backend && sqlc generate`
Expected: `ListAuditLogsRow` now has `ActorRole` + `OfficeName` (nullable pointer types). Do not hand-edit generated files.

- [ ] **Step 5: Extend `auditToMap`**

In `backend/internal/audit/dto.go`, inside the `if r.ActorID != nil` block add `"role": r.ActorRole` to the actor map, and after the office_id block add:

```go
if r.OfficeName != nil {
    m["office_name"] = *r.OfficeName
} else {
    m["office_name"] = nil
}
```

- [ ] **Step 6: Update OpenAPI**

In `backend/api/openapi.yaml`, add `role` (nullable string) under the audit actor object and a nullable `office_name` string to the audit row schema.

- [ ] **Step 7: Run the integration tests + Spectral**

Run: `cd backend && go test -tags=integration ./internal/audit/ -run TestListAudit && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: tests PASS; Spectral 0 new errors.

- [ ] **Step 8: Commit**

```bash
git add backend/db/queries/audit.sql backend/db/sqlc backend/internal/audit/dto.go backend/api/openapi.yaml backend/internal/audit/audit_integration_test.go
git commit -m "feat(audit): resolve actor role and office name in audit list"
```

---

### Task 9: Frontend audit — DTO, derived summary, columns, actor filter

**Files:**
- Modify: `frontend/app/composables/api/useAudit.ts`
- Modify: `frontend/app/pages/settings/audit.vue`
- Modify: `frontend/i18n/locales/{id,en}.json`
- Test: `frontend/test/unit/use-audit.spec.ts`, `frontend/test/nuxt/settings-audit.spec.ts`

**Interfaces:**
- Consumes: `AsyncSearchPicker` + `useUsers`-backed source. Add `useUserPicker()` to `usePickerSource.ts` (label `user.name`, sublabel `user.email`) against `useUsers().list({search,limit})`.
- Produces: `AuditRow` gains `role: string`, `office_name: string`, `summary: string`; `AuditListParams` gains `actor_id?: string`; `AuditDTO` gains `actor.role` + `office_name`.

- [ ] **Step 1: Write failing unit tests for the summary + mapping**

In `frontend/test/unit/use-audit.spec.ts`:

```ts
it('maps role, office_name, and a derived localized summary', () => {
  const row = toRow({
    id: '1', entity_type: 'assets', entity_id: 'AST-001', action: 'update', ip: '', created_at: '2026-07-12T03:04:05Z',
    changes: { name: { before: 'A', after: 'B' }, status: { before: 'x', after: 'y' } },
    actor: { id: 'u1', name: 'Budi', email: 'b@x.id', role: 'admin' }, office_name: 'KP Test'
  }, tMock)
  expect(row.role).toBe('admin')
  expect(row.office_name).toBe('KP Test')
  expect(row.summary).toBe('Mengubah 2 field pada assets AST-001')
})
```

Where `tMock` is a fake `t()` implementing the summary keys. Define expected key contract: `audit.summary.create` = "Membuat {entity} {id}", `audit.summary.update` = "Mengubah {count} field pada {entity} {id}", `audit.summary.delete` = "Menghapus {entity} {id}".

- [ ] **Step 2: Run to verify fail**

Run: `cd frontend && pnpm test use-audit`
Expected: FAIL — `toRow` doesn't accept `t`, no `role`/`summary`.

- [ ] **Step 3: Implement the DTO + summary in `useAudit.ts`**

- Extend `AuditDTO` with `actor: { ..., role: string }` and `office_name: string | null`.
- Extend `AuditRow` with `role`, `office_name`, `summary`.
- Add `actor_id?: string` to `AuditListParams` and wire `if (params.actor_id) q.set('actor_id', params.actor_id)` in `list`.
- Add a summary builder that takes `t`:

```ts
function toSummary(d: AuditDTO, t: (k: string, p?: Record<string, unknown>) => string): string {
  const count = d.changes ? Object.keys(d.changes).length : 0
  const base = { entity: d.entity_type, id: d.entity_id, count }
  if (d.action === 'create') return t('audit.summary.create', base)
  if (d.action === 'delete') return t('audit.summary.delete', base)
  return t('audit.summary.update', base)
}
```

- Thread `t` from the component into `list`/`toRow` (pass `useI18n().t` into `useAudit`, or map rows in the page). Simplest: `toRow(d, t)` and `list` receives `t`. Update the page call sites accordingly.

- [ ] **Step 4: Add i18n keys**

`audit.summary.{create,update,delete}` + column labels `audit.column.{role,office}` + `audit.filter.actor` in both locales.

- [ ] **Step 5: Write the failing component test for the new columns + actor filter**

In `frontend/test/nuxt/settings-audit.spec.ts`: assert the table renders a Role and Office column with the mocked values, the summary text appears, and the actor filter (`[data-testid="audit-actor-picker-input"]`) exists; selecting an actor calls `list` with `actor_id`.

- [ ] **Step 6: Implement the page changes**

In `settings/audit.vue`: add Role + Office columns (`role`, `office_name`), render `summary` (e.g. under the actor or as the row's primary line), and add the actor filter using `<AsyncSearchPicker testid="audit-actor" :search-fn resolve-fn :placeholder>` bound to `useUserPicker()`, feeding `actor_id` into the list params.

- [ ] **Step 7: Run tests + typecheck**

Run: `cd frontend && pnpm test use-audit settings-audit use-picker-source && pnpm typecheck`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add frontend/app/composables/api/useAudit.ts frontend/app/composables/usePickerSource.ts frontend/app/pages/settings/audit.vue frontend/i18n/locales frontend/test
git commit -m "feat(frontend): audit role/office columns, derived summary, actor filter"
```

---

## Part 1 — Field-permission enforcement

### Task 10: Canonical `FilterEntity` helper + standardize consumers

**Files:**
- Modify: `backend/internal/authz/fields.go`
- Modify: `backend/internal/user/handler.go` (replace `filterMaps`), `backend/internal/asset/handler.go` (replace `filterMap`), `backend/internal/approval/handler.go` (replace `filterMap`)
- Test: `backend/internal/authz/fields_test.go` (or the existing fields test file)

**Interfaces:**
- Produces: `func (s *FieldService) FilterEntity(ctx context.Context, roleID uuid.UUID, entity string, data map[string]any) error` — loads policies via `ForEntity`, applies `FilterView`, returns any lookup error (fail-closed). Consumers call it and, on error, respond 500 (matching approval's existing fail-closed behavior).

- [ ] **Step 1: Write the failing unit test**

In `backend/internal/authz/fields_test.go` (create if absent; unit-testable if `ForEntity` can be exercised — otherwise make this a small integration test alongside the existing field tests):

```go
func TestFilterEntity_RemovesNonViewableAndFailsClosed(t *testing.T) {
    // With a fake/queries-backed FieldService returning {book_value: {CanView:false}}
    m := map[string]any{"name": "x", "book_value": "100"}
    err := svc.FilterEntity(ctx, roleID, "assets", m)
    require.NoError(t, err)
    _, ok := m["book_value"]
    require.False(t, ok)
    require.Contains(t, m, "name")
}
```

(If the existing tests already cover `ForEntity`+`FilterView`, model this test on them; the goal is proving the combined helper deletes masked fields and propagates errors.)

- [ ] **Step 2: Run to verify fail**

Run: `cd backend && go test ./internal/authz/ -run TestFilterEntity`
Expected: FAIL — `FilterEntity` undefined.

- [ ] **Step 3: Implement `FilterEntity`**

Append to `backend/internal/authz/fields.go`:

```go
// FilterEntity strips fields the role may not view from a serialized record.
// Fail-closed: a policy-lookup error is returned so callers can refuse to leak.
func (s *FieldService) FilterEntity(ctx context.Context, roleID uuid.UUID, entity string, data map[string]any) error {
    policies, err := s.ForEntity(ctx, roleID, entity)
    if err != nil {
        return err
    }
    FilterView(policies, data)
    return nil
}
```

- [ ] **Step 4: Refactor the three consumers to use it**

Replace `user.filterMaps`/`filterMap`, `asset.filterMap`, and `approval.filterMap` bodies to delegate to `FilterEntity` (keep each handler's existing wrapper name/signature if other call sites use it, but route the logic through `FilterEntity`). Ensure `user` now returns/handles the error (fail-closed) — on error respond `500` instead of silently serving unfiltered data. Verify each call site's error handling compiles.

- [ ] **Step 5: Run the authz + consumer tests**

Run: `cd backend && go test ./internal/authz/... ./internal/user/... ./internal/asset/... ./internal/approval/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/authz/fields.go backend/internal/user backend/internal/asset backend/internal/approval
git commit -m "refactor(authz): single fail-closed FilterEntity helper for field masking"
```

---

### Task 11: Employee field-permission enforcement

**Files:**
- Modify: `backend/internal/masterdata/employee/dto.go` (add `employeeToMap`)
- Modify: `backend/internal/masterdata/employee/handler.go` (inject `fieldSvc`, filter responses)
- Modify: `backend/internal/masterdata/masterdata.go` (thread `fieldSvc` through `RegisterRoutes`)
- Modify: `backend/internal/server/router.go` (pass `fieldSvc` into `masterdata.RegisterRoutes`)
- Modify: the authz field integration test that uses key `"employee"` → `"employees"`
- Test: `backend/internal/masterdata/employee/*_integration_test.go` (add a field-masking case)

**Interfaces:**
- Consumes: `authz.FieldService.FilterEntity` (Task 10).
- Produces: `employeeToMap(e sqlc.MasterdataEmployee) map[string]any` with keys `id, code, name, email, phone, avatar_key, department_id, position_id, office_id, status, created_at, updated_at`. `NewHandler(q, scope, aud, fieldSvc)` signature. `RegisterRoutes(rg, q, pool, permSvc, scopeSvc, fieldSvc, aud, authMW)`.

- [ ] **Step 1: Write the failing integration test**

Add to the employee integration test: seed a role with `field_permissions` row `(entity="employees", field="email", can_view=false)`, then GET an employee as that role and assert `email` is absent while `name` is present.

```go
func TestEmployee_FieldMasking_HidesEmail(t *testing.T) {
    // seed employee + a role with can_view=false on employees.email
    body := getEmployee(t, empID, /* as masked role */)
    require.NotContains(t, body, "email")
    require.Contains(t, body, "name")
}
```

- [ ] **Step 2: Run to verify fail**

Run: `cd backend && go test -tags=integration ./internal/masterdata/employee/ -run TestEmployee_FieldMasking`
Expected: FAIL — response is a typed struct with `email` always present.

- [ ] **Step 3: Add `employeeToMap` to `dto.go`**

```go
func employeeToMap(e sqlc.MasterdataEmployee) map[string]any {
    return map[string]any{
        "id":            e.ID.String(),
        "code":          e.Code,
        "name":          e.Name,
        "email":         e.Email,
        "phone":         e.Phone,
        "avatar_key":    e.AvatarKey,
        "department_id": common.UUIDPtrStr(e.DepartmentID),
        "position_id":   common.UUIDPtrStr(e.PositionID),
        "office_id":     e.OfficeID.String(),
        "status":        string(e.Status),
        "created_at":    common.TsStr(e.CreatedAt),
        "updated_at":    common.TsStr(e.UpdatedAt),
    }
}
```

- [ ] **Step 4: Inject `fieldSvc` + filter in `handler.go`**

- Add `fields *authz.FieldService` to `Handler`; extend `NewHandler(q, scope, aud, fieldSvc)`.
- Add a helper:

```go
func (h *Handler) roleID(c *gin.Context) uuid.UUID { return c.MustGet(middleware.CtxRoleID).(uuid.UUID) }
```

(match how `asset`/`approval` read the role id from context — copy their exact accessor).
- In `get`/`list`/`create`/`update` responses, build `m := employeeToMap(e)`, call `if err := h.fields.FilterEntity(c.Request.Context(), h.roleID(c), "employees", m); err != nil { c.JSON(500, gin.H{"error":"..."}); return }`, and return `m` (for list, filter each row map). Keep `audit.Diff` using `toResponse` (audit diffs are internal, not the masked view) — leave the `Record(...)` calls as-is.

- [ ] **Step 5: Thread `fieldSvc` through masterdata wiring**

- `masterdata.go`: add `fieldSvc *authz.FieldService` param to `RegisterRoutes`; pass into `employee.NewHandler(q, scopeSvc, aud, fieldSvc)`.
- `router.go`: change the `masterdata.RegisterRoutes(api, queries, d.Pool, permSvc, scopeSvc, auditSvc, requireAuth)` call to include `fieldSvc` in the right position.

- [ ] **Step 6: Fix the `"employee"` → `"employees"` test key**

Update `backend/internal/authz/fields_integration_test.go` (the `"employee"` singular usage) to `"employees"`.

- [ ] **Step 7: Run backend build + the integration test**

Run: `cd backend && go build ./... && go test -tags=integration ./internal/masterdata/employee/ ./internal/authz/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/masterdata/employee backend/internal/masterdata/masterdata.go backend/internal/server/router.go backend/internal/authz
git commit -m "feat(security): enforce field permissions on employees"
```

---

### Task 12: Close the depreciation impairment leak (+ verify attachment/document)

**Files:**
- Modify: `backend/internal/depreciation/handler.go` (impairment path), `backend/internal/depreciation/dto.go` (`impairmentResultToMap`)
- Investigate: `backend/internal/asset/dto.go` (`attachmentToMap`), `backend/internal/asset/document_dto.go` (`documentToMap`)
- Test: `backend/internal/depreciation/*_integration_test.go`

- [ ] **Step 1: Write the failing test**

Assert that a role with `field_permissions (entity="assets", field="book_value", can_view=false)` receives an impairment result **without** `book_value`/`accumulated_depreciation`:

```go
func TestImpairment_MasksBookValueForMaskedRole(t *testing.T) {
    body := getImpairment(t, /* masked role */)
    require.NotContains(t, body, "book_value")
    require.NotContains(t, body, "accumulated_depreciation")
}
```

- [ ] **Step 2: Run to verify fail**

Run: `cd backend && go test -tags=integration ./internal/depreciation/ -run TestImpairment_MasksBookValue`
Expected: FAIL — `impairmentResultToMap` returns them unmasked.

- [ ] **Step 3: Route impairment through the field policy**

Mirror the existing `maskedAssetScheduleMap` guard (the `policies["book_value"].CanView` check at handler.go:301-310): in the impairment handler, load the `assets` policies via `fieldSvc.ForEntity`, and when `book_value` is not viewable, omit `book_value` and `accumulated_depreciation` from `impairmentResultToMap` output (or delete the keys before responding). Reuse `FilterEntity` where the keys match the `assets` field names.

- [ ] **Step 4: Verify attachment/document maps (honest scoping)**

Read `attachmentToMap` and `documentToMap`. If they only serialize file metadata (filename, size, key, uploader, timestamps) and do **not** echo any field the `assets` policy masks (money fields), record in the commit message that they are **not** a leak and leave them unchanged. If either does echo a masked field, apply `FilterEntity(..., "assets", m)` to it and add a test.

- [ ] **Step 5: Run the depreciation tests**

Run: `cd backend && go test -tags=integration ./internal/depreciation/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/depreciation
git commit -m "fix(security): mask book_value/accumulated_depreciation in impairment result"
```

---

### Task 13: Frontend field catalog — employees entity

**Files:**
- Modify: `frontend/app/constants/fieldCatalog.ts`
- Modify: `frontend/i18n/locales/{id,en}.json` (entity + field labels)
- Test: `frontend/test/unit/field-catalog.spec.ts`

- [ ] **Step 1: Write the failing test**

In `frontend/test/unit/field-catalog.spec.ts` add:

```ts
it('includes the employees entity with its maskable fields', () => {
  const emp = FIELD_CATALOG.find(e => e.entity === 'employees')
  expect(emp).toBeTruthy()
  expect(emp!.fields).toEqual(['name', 'email', 'phone', 'department_id', 'position_id', 'office_id', 'status'])
})
```

- [ ] **Step 2: Run to verify fail**

Run: `cd frontend && pnpm test field-catalog`
Expected: FAIL — no `employees` entity.

- [ ] **Step 3: Add the entity**

In `fieldCatalog.ts` append to `FIELD_CATALOG`:

```ts
{
  entity: 'employees',
  fields: ['name', 'email', 'phone', 'department_id', 'position_id', 'office_id', 'status']
}
```

(`code` is the employee's identifier and is intentionally not maskable — mirrors `users` omitting its id-like keys.)

- [ ] **Step 4: Add i18n labels**

`settings.fieldPermission.entity.employees` (id "Pegawai" / en "Employees") and `.field.{name,email,phone,department_id,position_id,office_id,status}` in both locales.

- [ ] **Step 5: Run tests + typecheck + lint**

Run: `cd frontend && pnpm test field-catalog && pnpm typecheck && pnpm lint`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/constants/fieldCatalog.ts frontend/i18n/locales frontend/test/unit/field-catalog.spec.ts
git commit -m "feat(frontend): add employees entity to field-permission catalog"
```

---

## Final task: Full verification gate + PROGRESS.md

- [ ] **Step 1: Backend full gate**

Run: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... -p 1 && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: all green; Spectral 0 errors (known warnings OK).

- [ ] **Step 2: Frontend full gate**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green.

- [ ] **Step 3: Affected e2e** (stack up + seeded admin)

Run: `cd frontend && pnpm test:e2e settings assignment import-masterdata`
Expected: PASS (field-permission/audit screens + picker flows).

- [ ] **Step 4: Update `docs/PROGRESS.md`**

Tick item 49(c) sub-items (field-permission enforcement beyond assets+users; async searchable pickers) with a one-line note + this branch; note audit enrichment landed; record approved deviations (audit summary derived client-side + current-role join; pickers standardized on the hand-rolled dropdown; master tables converted to server-side pagination). Refresh the "Next session — start here" pointer.

- [ ] **Step 5: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): tech-debt sweep — field-perm, audit enrich, async pickers"
```

---

## Self-review notes (coverage map)

- Spec Part 1 → Tasks 10 (helper standardize), 11 (employees), 12 (leaks), 13 (frontend catalog). ✓
- Spec Part 2 → Tasks 8 (backend joins), 9 (frontend DTO/summary/columns/actor filter). ✓
- Spec Part 3 → Tasks 1 (component), 2 (AssetSearchPicker refactor), 3 (adapters), 4 (office/employee forms), 5 (reference/category forms), 6 (filters + master tables), 7 (e2e). ✓
- Non-goals respected: no write-side field masking, no audit summary column/migration, no backend search additions, no role snapshot.
- Cross-task type consistency: `PickerItem` (Task 1) used by Tasks 2–6, 9; `FilterEntity` (Task 10) used by Tasks 11–12; `employeeToMap` keys (Task 11) match `fieldCatalog` employees fields (Task 13); `useUserPicker` (Task 9) added to the same `usePickerSource.ts` from Task 3.
</content>
