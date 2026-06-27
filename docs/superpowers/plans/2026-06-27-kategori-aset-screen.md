# Kategori Aset Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Master Data → Kategori Aset screen (list + filter + 4-section slideover form) 1:1 with `docs/design/Kategori Aset.dc.html`, mock-first behind the existing data seam.

**Architecture:** A single data seam (`useCategories` over a mock store) feeds a thin page (`pages/master/categories.vue`) that composes the shared `PageHeader` + `ResourceTable` + a new `CategoryFormSlideover` component (the only extracted piece, because the form is complex). Field contract mirrors the backend exactly in English `snake_case` (ADR-0007).

**Tech Stack:** Nuxt 4 (SPA, `ssr: false`), Nuxt UI (`U*`), Vitest + `@nuxt/test-utils`, Playwright. Spec: `docs/superpowers/specs/2026-06-27-kategori-aset-screen-design.md`.

## Global Constraints

- **Field naming:** English `snake_case` matching backend (`name, code, parent_id, default_depreciation_method, default_useful_life_months, default_salvage_rate, asset_class, default_fiscal_group, default_fiscal_life_months, gl_account_code, capitalization_threshold, is_active`). Do NOT touch existing Indonesian-keyed composables.
- **Mock-first:** all data via `composables/api/useCategories.ts` over `mock/categories.ts`. Never call backend URL directly.
- **Money/rate as string:** `default_salvage_rate` & `capitalization_threshold` are `string | null` (backend numeric→string convention).
- **i18n mandatory:** every user-facing string in `i18n/locales/{id,en}.json`; default locale `id`. No hardcoded UI text.
- **Theme tokens only:** semantic Nuxt UI colors / CSS vars; no literal Tailwind colors.
- **Lint:** ESLint stylistic — **no trailing commas** (`commaDangle: 'never'`), 1tbs braces. `pnpm lint` + `pnpm typecheck` must pass.
- **Permission gate:** category writes require `masterdata.global.manage`.
- **Fiscal group options in the form:** only the 6 from the mockup (`kelompok_1..4`, `bangunan_permanen`, `bangunan_non_permanen`); `non_susut` stays in the type (backend parity) but is NOT offered in the select. Intangible class hides the two `bangunan_*` options.
- **Page size:** 7 (mockup).
- All commands run from `frontend/` unless stated.

---

### Task 1: Data layer — types, mock store, composable

**Files:**
- Modify: `frontend/app/types/index.ts` (append Category types after the `Employee` interface, ~line 84)
- Create: `frontend/app/mock/categories.ts`
- Create: `frontend/app/composables/api/useCategories.ts`
- Test: `frontend/test/unit/categories-mock.spec.ts`

**Interfaces:**
- Produces: `Category`, `CategoryInput`, `AssetClass`, `DepreciationMethod`, `FiscalGroup` types; `categorySeed`, `categoryStore`, `FISCAL_GROUPS`, `isBuildingGroup()`, `formatThousands()`, `parseThousands()`; `useCategories()` → `{ list, get, create, update, remove }`.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/categories-mock.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { categorySeed, categoryStore, isBuildingGroup, formatThousands, parseThousands } from '~/mock/categories'
import { filterBy, paginate } from '~/mock/helpers'

describe('categories mock', () => {
  it('seeds more than one category including a parent/child pair', () => {
    expect(categorySeed.length).toBeGreaterThan(1)
    const child = categorySeed.find(c => c.parent_id)
    expect(child).toBeTruthy()
    expect(categorySeed.some(c => c.id === child!.parent_id)).toBe(true)
  })

  it('seeds at least one intangible and one inactive category', () => {
    expect(categorySeed.some(c => c.asset_class === 'intangible')).toBe(true)
    expect(categorySeed.some(c => !c.is_active)).toBe(true)
  })

  it('filterBy matches by name and code', () => {
    const all = categoryStore.all()
    expect(filterBy(all, { search: 'Kendaraan' }, ['name', 'code'])).toHaveLength(1)
    expect(filterBy(all, { search: 'ELK' }, ['name', 'code'])[0].code).toBe('ELK')
  })

  it('paginate slices to page size 7', () => {
    const page = paginate(categoryStore.all(), { limit: 7, offset: 0 })
    expect(page.data.length).toBeLessThanOrEqual(7)
    expect(page.total).toBe(categorySeed.length)
  })

  it('isBuildingGroup is true only for bangunan_* groups', () => {
    expect(isBuildingGroup('bangunan_permanen')).toBe(true)
    expect(isBuildingGroup('bangunan_non_permanen')).toBe(true)
    expect(isBuildingGroup('kelompok_1')).toBe(false)
    expect(isBuildingGroup(null)).toBe(false)
  })

  it('formatThousands / parseThousands round-trip with id-ID grouping', () => {
    expect(formatThousands('1000000')).toBe('1.000.000')
    expect(formatThousands('')).toBe('')
    expect(parseThousands('1.000.000')).toBe('1000000')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test -- categories-mock`
Expected: FAIL — cannot resolve `~/mock/categories`.

- [ ] **Step 3: Add the types**

In `frontend/app/types/index.ts`, immediately after the `Employee` interface (the block ending at the line `}` before `export interface User {`), insert:

```ts
export type AssetClass = 'tangible' | 'intangible'
export type DepreciationMethod = 'straight_line' | 'declining_balance'
export type FiscalGroup =
  | 'kelompok_1' | 'kelompok_2' | 'kelompok_3' | 'kelompok_4'
  | 'bangunan_permanen' | 'bangunan_non_permanen' | 'non_susut'

export interface Category {
  id: string
  name: string
  code: string | null
  parent_id: string | null
  default_depreciation_method: DepreciationMethod | null
  default_useful_life_months: number | null
  default_salvage_rate: string | null
  asset_class: AssetClass
  default_fiscal_group: FiscalGroup | null
  default_fiscal_life_months: number | null
  gl_account_code: string | null
  capitalization_threshold: string | null
  is_active: boolean
  created_at: string
}
```

- [ ] **Step 4: Create the mock store**

Create `frontend/app/mock/categories.ts`:

```ts
import type { Category, FiscalGroup } from '~/types'
import { createStore } from './helpers'

export const categorySeed: Category[] = [
  { id: 'c-it', name: 'Perangkat IT', code: 'ITX', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3.00', capitalization_threshold: '1000000', is_active: true, created_at: '2026-01-05' },
  { id: 'c-laptop', name: 'Komputer & Laptop', code: 'ELK', parent_id: 'c-it', default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3.01', capitalization_threshold: '1000000', is_active: true, created_at: '2026-01-06' },
  { id: 'c-vehicle', name: 'Kendaraan Bermotor', code: 'KEN', parent_id: null, default_depreciation_method: 'declining_balance', default_useful_life_months: 96, default_salvage_rate: '10', asset_class: 'tangible', default_fiscal_group: 'kelompok_2', default_fiscal_life_months: 96, gl_account_code: '1.2.4.00', capitalization_threshold: '10000000', is_active: true, created_at: '2026-01-07' },
  { id: 'c-building', name: 'Bangunan Kantor', code: 'BGN', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 240, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'bangunan_permanen', default_fiscal_life_months: 240, gl_account_code: '1.2.1.00', capitalization_threshold: '50000000', is_active: true, created_at: '2026-01-08' },
  { id: 'c-atm', name: 'Mesin ATM', code: 'ATM', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 96, default_salvage_rate: '5', asset_class: 'tangible', default_fiscal_group: 'kelompok_2', default_fiscal_life_months: 96, gl_account_code: '1.2.3.05', capitalization_threshold: '25000000', is_active: true, created_at: '2026-01-09' },
  { id: 'c-furniture', name: 'Mebel & Inventaris Kantor', code: 'MBL', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.5.00', capitalization_threshold: '1000000', is_active: true, created_at: '2026-01-10' },
  { id: 'c-software', name: 'Software / Lisensi', code: 'SFT', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'intangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.6.00', capitalization_threshold: '5000000', is_active: true, created_at: '2026-01-11' },
  { id: 'c-network', name: 'Peralatan Jaringan (Legacy)', code: 'NET', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3.09', capitalization_threshold: '1000000', is_active: false, created_at: '2026-01-12' }
]

export const categoryStore = createStore<Category>(categorySeed)

// Form-select order (mockup); excludes non_susut.
export const FISCAL_GROUPS: FiscalGroup[] = [
  'kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen'
]

export function isBuildingGroup(g: FiscalGroup | null | undefined): boolean {
  return g === 'bangunan_permanen' || g === 'bangunan_non_permanen'
}

// Display a numeric string with id-ID thousands grouping ('1000000' → '1.000.000').
export function formatThousands(v: string | number | null | undefined): string {
  const n = Number(String(v ?? '').replace(/\D/g, ''))
  return n ? n.toLocaleString('id-ID') : ''
}

// Strip grouping back to a bare digit string ('1.000.000' → '1000000').
export function parseThousands(v: string | null | undefined): string {
  return String(v ?? '').replace(/\D/g, '')
}
```

- [ ] **Step 5: Create the composable**

Create `frontend/app/composables/api/useCategories.ts`:

```ts
import type { Category, ListQuery, Paginated } from '~/types'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { categoryStore } from '~/mock/categories'

export type CategoryInput = Omit<Category, 'id' | 'created_at'>

export function useCategories() {
  async function list(query: ListQuery = {}): Promise<Paginated<Category>> {
    await fakeLatency()
    return paginate(filterBy(categoryStore.all(), query, ['name', 'code']), query)
  }

  async function get(id: string): Promise<Category | undefined> {
    await fakeLatency()
    return categoryStore.find(id)
  }

  async function create(input: CategoryInput): Promise<Category> {
    await fakeLatency()
    return categoryStore.insert({ id: generateId(), created_at: new Date().toISOString(), ...input })
  }

  async function update(id: string, input: CategoryInput): Promise<Category> {
    await fakeLatency()
    const row = categoryStore.patch(id, input)
    if (!row) throw new Error('masterdata.categories.errNotFound')
    return row
  }

  async function remove(id: string): Promise<void> {
    await fakeLatency()
    categoryStore.remove(id)
  }

  return { list, get, create, update, remove }
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `pnpm test -- categories-mock`
Expected: PASS (6 tests).

- [ ] **Step 7: Typecheck + commit**

Run: `pnpm typecheck`
Expected: no errors.

```bash
git add frontend/app/types/index.ts frontend/app/mock/categories.ts frontend/app/composables/api/useCategories.ts frontend/test/unit/categories-mock.spec.ts
git commit -m "feat(masterdata): categories data seam — types, mock store, composable"
```

---

### Task 2: i18n strings + nav entry

**Files:**
- Modify: `frontend/i18n/locales/id.json` (add `nav.categories`; add `masterdata.categories` block)
- Modify: `frontend/i18n/locales/en.json` (same keys, English)
- Modify: `frontend/app/utils/nav.ts:72` (insert Kategori item after `nav.employees`)
- Test: `frontend/test/unit/nav-model.spec.ts` (add one assertion)

**Interfaces:**
- Produces: i18n keys under `masterdata.categories.*` + `nav.categories`; nav item `{ labelKey: 'nav.categories', to: '/master/categories' }`.

- [ ] **Step 1: Write the failing test**

Append to `frontend/test/unit/nav-model.spec.ts` (inside the existing top-level `describe`, add a new `it`):

```ts
  it('includes a Kategori entry under Master Data linking to /master/categories', () => {
    const master = superadminNav
      .flatMap(g => g.items)
      .find(i => i.labelKey === 'nav.masterData')
    expect(master?.children?.some(c => c.to === '/master/categories' && c.labelKey === 'nav.categories')).toBe(true)
  })
```

If `superadminNav` is not already imported at the top of the file, add `import { superadminNav } from '~/utils/nav'` (check the file's existing imports first and reuse them).

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test -- nav-model`
Expected: FAIL — no `/master/categories` child.

- [ ] **Step 3: Add the nav item**

In `frontend/app/utils/nav.ts`, inside the Master Data `children` array, insert the Kategori entry directly after the `nav.employees` item (between the employees block ending `},` at line 72 and the `nav.officeMap` block):

```ts
          {
            labelKey: 'nav.categories',
            to: '/master/categories'
          },
```

Final order: Kantor → Pegawai → **Kategori** → Peta Lokasi → Referensi.

- [ ] **Step 4: Add i18n keys (id)**

In `frontend/i18n/locales/id.json`: add `"categories": "Kategori Aset"` to the `nav` object, and add this block inside the `masterdata` object (sibling of `employees`):

```json
    "categories": {
      "title": "Kategori Aset",
      "subtitle": "Kelola golongan aset tetap beserta penyusutan komersial (PSAK 16), fiskal (PMK 72/2023), dan akun GL.",
      "add": "Tambah Kategori",
      "empty": "Belum ada kategori",
      "emptyFilter": "Belum ada kategori aset yang cocok dengan pencarian atau filter.",
      "searchPlaceholder": "Cari nama atau kode…",
      "createTitle": "Tambah Kategori Aset",
      "editTitle": "Edit Kategori Aset",
      "createSub": "Buat golongan aset tetap baru.",
      "editSub": "Perbarui parameter kategori.",
      "deleteConfirm": "“{name}” akan dihapus dari master kategori. Pastikan tidak ada aset yang masih memakai kategori ini.",
      "errNotFound": "Kategori tidak ditemukan.",
      "req": "Wajib diisi.",
      "months": "bulan",
      "columns": {
        "name": "Nama",
        "code": "Kode",
        "class": "Kelas Aset",
        "method": "Metode Susut",
        "life": "Masa (bln)",
        "fiscalGroup": "Golongan Pajak",
        "gl": "Akun GL",
        "status": "Status"
      },
      "filter": {
        "allClass": "Semua Kelas",
        "allGroup": "Semua Golongan",
        "activeOnly": "Hanya aktif"
      },
      "class": {
        "tangible": "Berwujud",
        "intangible": "Takberwujud"
      },
      "method": {
        "straight_line": "Garis Lurus",
        "declining_balance": "Saldo Menurun"
      },
      "fiscalGroup": {
        "kelompok_1": "Kelompok 1",
        "kelompok_2": "Kelompok 2",
        "kelompok_3": "Kelompok 3",
        "kelompok_4": "Kelompok 4",
        "bangunan_permanen": "Bangunan Permanen",
        "bangunan_non_permanen": "Bangunan Non-Permanen"
      },
      "section": {
        "general": "Umum",
        "deprCommercial": "Penyusutan Komersial",
        "amortCommercial": "Amortisasi Komersial",
        "deprRef": "PSAK 16 — Aset Tetap",
        "amortRef": "PSAK 19 — Aset Takberwujud",
        "tax": "Pajak / Fiskal",
        "taxRef": "PMK 72/2023 — Penyusutan & Amortisasi Fiskal",
        "accounting": "Akuntansi"
      },
      "fields": {
        "name": "Nama Kategori",
        "code": "Kode",
        "parent": "Kategori Induk",
        "class": "Kelas Aset",
        "active": "Aktif",
        "method": "Metode",
        "life": "Masa Manfaat",
        "salvage": "Nilai Residu",
        "fiscalGroup": "Golongan / Kelompok Harta",
        "fiscalLife": "Masa Manfaat Fiskal",
        "gl": "Akun GL (COA)",
        "capitalization": "Batas Kapitalisasi"
      },
      "placeholders": {
        "name": "mis. Komputer & Laptop",
        "select": "Pilih…",
        "parentNone": "— (Tanpa induk / kategori utama)"
      },
      "hint": {
        "parent": "Kosongkan untuk kategori utama; pilih induk untuk sub-kategori.",
        "gl": "Kode akun buku besar aset.",
        "capitalization": "Minimum nilai untuk dikapitalisasi sebagai aset tetap.",
        "buildingLock": "Aset bangunan wajib memakai Garis Lurus."
      }
    },
```

- [ ] **Step 5: Add i18n keys (en)**

In `frontend/i18n/locales/en.json`: add `"categories": "Asset Categories"` to `nav`, and the parallel block under `masterdata`:

```json
    "categories": {
      "title": "Asset Categories",
      "subtitle": "Manage fixed-asset classes with commercial (IAS 16), fiscal (PMK 72/2023) depreciation, and GL accounts.",
      "add": "Add Category",
      "empty": "No categories yet",
      "emptyFilter": "No asset category matches the search or filter.",
      "searchPlaceholder": "Search name or code…",
      "createTitle": "Add Asset Category",
      "editTitle": "Edit Asset Category",
      "createSub": "Create a new fixed-asset class.",
      "editSub": "Update category parameters.",
      "deleteConfirm": "“{name}” will be removed from the category master. Ensure no asset still uses this category.",
      "errNotFound": "Category not found.",
      "req": "Required.",
      "months": "months",
      "columns": {
        "name": "Name",
        "code": "Code",
        "class": "Asset Class",
        "method": "Deprec. Method",
        "life": "Life (mo)",
        "fiscalGroup": "Tax Group",
        "gl": "GL Account",
        "status": "Status"
      },
      "filter": {
        "allClass": "All Classes",
        "allGroup": "All Groups",
        "activeOnly": "Active only"
      },
      "class": {
        "tangible": "Tangible",
        "intangible": "Intangible"
      },
      "method": {
        "straight_line": "Straight Line",
        "declining_balance": "Declining Balance"
      },
      "fiscalGroup": {
        "kelompok_1": "Group 1",
        "kelompok_2": "Group 2",
        "kelompok_3": "Group 3",
        "kelompok_4": "Group 4",
        "bangunan_permanen": "Permanent Building",
        "bangunan_non_permanen": "Non-Permanent Building"
      },
      "section": {
        "general": "General",
        "deprCommercial": "Commercial Depreciation",
        "amortCommercial": "Commercial Amortization",
        "deprRef": "IAS 16 — Fixed Assets",
        "amortRef": "IAS 38 — Intangible Assets",
        "tax": "Tax / Fiscal",
        "taxRef": "PMK 72/2023 — Fiscal Depreciation & Amortization",
        "accounting": "Accounting"
      },
      "fields": {
        "name": "Category Name",
        "code": "Code",
        "parent": "Parent Category",
        "class": "Asset Class",
        "active": "Active",
        "method": "Method",
        "life": "Useful Life",
        "salvage": "Residual Value",
        "fiscalGroup": "Tax Asset Group",
        "fiscalLife": "Fiscal Useful Life",
        "gl": "GL Account (COA)",
        "capitalization": "Capitalization Threshold"
      },
      "placeholders": {
        "name": "e.g. Computers & Laptops",
        "select": "Select…",
        "parentNone": "— (No parent / top category)"
      },
      "hint": {
        "parent": "Leave empty for a top category; pick a parent for a sub-category.",
        "gl": "Asset general-ledger account code.",
        "capitalization": "Minimum value to capitalize as a fixed asset.",
        "buildingLock": "Building assets must use Straight Line."
      }
    },
```

- [ ] **Step 6: Run tests + validate JSON**

Run: `pnpm test -- nav-model`
Expected: PASS.
Run: `pnpm typecheck`
Expected: no errors (also confirms both JSON files parse).

- [ ] **Step 7: Commit**

```bash
git add frontend/app/utils/nav.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/unit/nav-model.spec.ts
git commit -m "feat(masterdata): categories i18n strings + nav entry"
```

---

### Task 3: CategoryFormSlideover component

**Files:**
- Create: `frontend/app/components/category/CategoryFormSlideover.vue`
- Test: `frontend/test/nuxt/CategoryFormSlideover.spec.ts`

**Interfaces:**
- Consumes: `FormSlideover` (shared), `useCategories`'s `CategoryInput`, `mock/categories` helpers (`FISCAL_GROUPS`, `isBuildingGroup`, `formatThousands`, `parseThousands`).
- Produces: component with `v-model:open` (boolean), props `category: Category | null`, `parentOptions: { value: string, label: string }[]`, `loading?: boolean`; emits `submit` with a fully-built `CategoryInput`. Auto-exposed setup bindings used by tests: `form`, `isIntangible`, `isBuilding`, `onSubmit`.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/nuxt/CategoryFormSlideover.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import CategoryFormSlideover from '~/components/category/CategoryFormSlideover.vue'

type Vm = {
  form: Record<string, unknown>
  isIntangible: boolean
  isBuilding: boolean
  onSubmit: () => void
}

async function mountOpen() {
  const wrapper = await mountSuspended(CategoryFormSlideover, {
    props: { open: true, category: null, parentOptions: [{ value: 'c-it', label: 'Perangkat IT' }] }
  })
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('CategoryFormSlideover', () => {
  it('renders the four numbered sections', async () => {
    const wrapper = await mountOpen()
    const html = document.body.innerHTML
    expect(html).toContain('Umum')
    expect(html).toContain('Penyusutan Komersial')
    expect(html).toContain('Pajak / Fiskal')
    expect(html).toContain('Akuntansi')
  })

  it('switches the depreciation section to Amortisasi/PSAK 19 when class is intangible', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    vm.form.asset_class = 'intangible'
    await wrapper.vm.$nextTick()
    const html = document.body.innerHTML
    expect(html).toContain('Amortisasi Komersial')
    expect(html).toContain('PSAK 19')
    expect(html).not.toContain('Bangunan Permanen')
  })

  it('locks method to Garis Lurus when a building fiscal group is selected', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    vm.form.default_fiscal_group = 'bangunan_permanen'
    await wrapper.vm.$nextTick()
    expect(vm.isBuilding).toBe(true)
    expect(vm.form.default_depreciation_method).toBe('straight_line')
    expect(document.body.innerHTML).toContain('wajib memakai Garis Lurus')
  })

  it('blocks submit and flags errors when name and code are empty', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    vm.onSubmit()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('submit')).toBeFalsy()
    expect(document.body.innerHTML).toContain('Wajib diisi')
  })

  it('emits submit with a snake_case CategoryInput payload', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    Object.assign(vm.form, {
      name: 'Genset',
      code: 'GEN',
      asset_class: 'tangible',
      default_depreciation_method: 'declining_balance',
      default_useful_life_months: '96',
      default_salvage_rate: '10',
      default_fiscal_group: 'kelompok_2',
      default_fiscal_life_months: '96',
      gl_account_code: '1.2.7.00',
      capitalization_threshold: '10.000.000',
      parent_id: 'c-it',
      is_active: true
    })
    vm.onSubmit()
    await wrapper.vm.$nextTick()
    const payload = wrapper.emitted('submit')?.[0]?.[0]
    expect(payload).toEqual({
      name: 'Genset',
      code: 'GEN',
      parent_id: 'c-it',
      default_depreciation_method: 'declining_balance',
      default_useful_life_months: 96,
      default_salvage_rate: '10',
      asset_class: 'tangible',
      default_fiscal_group: 'kelompok_2',
      default_fiscal_life_months: 96,
      gl_account_code: '1.2.7.00',
      capitalization_threshold: '10000000',
      is_active: true
    })
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test -- CategoryFormSlideover`
Expected: FAIL — cannot resolve `~/components/category/CategoryFormSlideover.vue`.

- [ ] **Step 3: Create the component**

Create `frontend/app/components/category/CategoryFormSlideover.vue`:

```vue
<script setup lang="ts">
import type { Category, FiscalGroup } from '~/types'
import type { CategoryInput } from '~/composables/api/useCategories'
import { FISCAL_GROUPS, isBuildingGroup, formatThousands, parseThousands } from '~/mock/categories'

const props = defineProps<{
  category: Category | null
  parentOptions: { value: string, label: string }[]
  loading?: boolean
}>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ submit: [CategoryInput] }>()

const { t } = useI18n()

interface FormState {
  name: string
  code: string
  parent_id: string
  asset_class: Category['asset_class']
  default_depreciation_method: 'straight_line' | 'declining_balance'
  default_useful_life_months: string
  default_salvage_rate: string
  default_fiscal_group: string
  default_fiscal_life_months: string
  gl_account_code: string
  capitalization_threshold: string
  is_active: boolean
}

function emptyForm(): FormState {
  return {
    name: '', code: '', parent_id: '', asset_class: 'tangible',
    default_depreciation_method: 'straight_line', default_useful_life_months: '',
    default_salvage_rate: '', default_fiscal_group: '', default_fiscal_life_months: '',
    gl_account_code: '', capitalization_threshold: '', is_active: true
  }
}

const form = reactive<FormState>(emptyForm())
const errors = reactive<{ name: boolean, code: boolean }>({ name: false, code: false })

function hydrate() {
  const c = props.category
  errors.name = false
  errors.code = false
  if (!c) {
    Object.assign(form, emptyForm())
    return
  }
  Object.assign(form, {
    name: c.name,
    code: c.code ?? '',
    parent_id: c.parent_id ?? '',
    asset_class: c.asset_class,
    default_depreciation_method: c.default_depreciation_method ?? 'straight_line',
    default_useful_life_months: c.default_useful_life_months != null ? String(c.default_useful_life_months) : '',
    default_salvage_rate: c.default_salvage_rate ?? '',
    default_fiscal_group: c.default_fiscal_group ?? '',
    default_fiscal_life_months: c.default_fiscal_life_months != null ? String(c.default_fiscal_life_months) : '',
    gl_account_code: c.gl_account_code ?? '',
    capitalization_threshold: formatThousands(c.capitalization_threshold),
    is_active: c.is_active
  })
}

watch(open, (v) => {
  if (v) hydrate()
}, { immediate: true })

const isIntangible = computed(() => form.asset_class === 'intangible')
const isBuilding = computed(() => isBuildingGroup(form.default_fiscal_group as FiscalGroup))
const metodeLocked = computed(() => isBuilding.value)

// Building assets must use straight line.
watch(isBuilding, (b) => {
  if (b) form.default_depreciation_method = 'straight_line'
})

// Intangible classes can't be buildings; drop the bangunan_* options.
const fiscalGroupOptions = computed(() =>
  FISCAL_GROUPS
    .filter(g => !(isIntangible.value && isBuildingGroup(g)))
    .map(g => ({ value: g as string, label: t(`masterdata.categories.fiscalGroup.${g}`) }))
)

const methodOptions = computed(() => [
  { value: 'straight_line', label: t('masterdata.categories.method.straight_line') },
  { value: 'declining_balance', label: t('masterdata.categories.method.declining_balance') }
])

const susutTitle = computed(() =>
  isIntangible.value ? t('masterdata.categories.section.amortCommercial') : t('masterdata.categories.section.deprCommercial')
)
const susutRef = computed(() =>
  isIntangible.value ? t('masterdata.categories.section.amortRef') : t('masterdata.categories.section.deprRef')
)

const formTitle = computed(() =>
  props.category ? t('masterdata.categories.editTitle') : t('masterdata.categories.createTitle')
)
const formSub = computed(() =>
  props.category ? t('masterdata.categories.editSub') : t('masterdata.categories.createSub')
)

function onCapitalInput(e: Event) {
  form.capitalization_threshold = formatThousands((e.target as HTMLInputElement).value)
}

function toInput(): CategoryInput {
  const numOrNull = (s: string): number | null => {
    const n = Number(s)
    return s.trim() !== '' && Number.isFinite(n) ? Math.trunc(n) : null
  }
  const strOrNull = (s: string): string | null => (s.trim() !== '' ? s.trim() : null)
  const cap = parseThousands(form.capitalization_threshold)
  return {
    name: form.name.trim(),
    code: strOrNull(form.code),
    parent_id: form.parent_id || null,
    default_depreciation_method: form.default_depreciation_method,
    default_useful_life_months: numOrNull(form.default_useful_life_months),
    default_salvage_rate: strOrNull(form.default_salvage_rate),
    asset_class: form.asset_class,
    default_fiscal_group: (form.default_fiscal_group || null) as Category['default_fiscal_group'],
    default_fiscal_life_months: numOrNull(form.default_fiscal_life_months),
    gl_account_code: strOrNull(form.gl_account_code),
    capitalization_threshold: cap !== '' ? cap : null,
    is_active: form.is_active
  }
}

function onSubmit() {
  errors.name = form.name.trim() === ''
  errors.code = form.code.trim() === ''
  if (errors.name || errors.code) return
  emit('submit', toInput())
}

defineExpose({ form, isIntangible, isBuilding, onSubmit })
</script>

<template>
  <FormSlideover
    v-model:open="open"
    :title="formTitle"
    :subtitle="formSub"
    :loading="props.loading"
    @submit="onSubmit"
  >
    <div class="space-y-6">
      <!-- Section 1: Umum -->
      <section>
        <div class="flex items-center gap-2 mb-3">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">1</span>
          <span class="font-semibold text-sm">{{ t('masterdata.categories.section.general') }}</span>
        </div>
        <div class="space-y-3">
          <div class="grid grid-cols-[1fr_140px] gap-3">
            <UFormField
              :label="t('masterdata.categories.fields.name')"
              required
              :error="errors.name ? t('masterdata.categories.req') : undefined"
            >
              <UInput
                v-model="form.name"
                :placeholder="t('masterdata.categories.placeholders.name')"
                class="w-full"
              />
            </UFormField>
            <UFormField
              :label="t('masterdata.categories.fields.code')"
              required
              :error="errors.code ? t('masterdata.categories.req') : undefined"
            >
              <UInput
                v-model="form.code"
                placeholder="ELK"
                class="w-full font-mono"
              />
            </UFormField>
          </div>

          <UFormField
            :label="t('masterdata.categories.fields.parent')"
            :hint="t('masterdata.categories.hint.parent')"
          >
            <USelect
              v-model="form.parent_id"
              :items="[{ value: '', label: t('masterdata.categories.placeholders.parentNone') }, ...props.parentOptions]"
              class="w-full"
            />
          </UFormField>

          <UFormField :label="t('masterdata.categories.fields.class')">
            <div class="flex gap-2">
              <UButton
                :color="form.asset_class === 'tangible' ? 'primary' : 'neutral'"
                :variant="form.asset_class === 'tangible' ? 'solid' : 'outline'"
                icon="i-lucide-box"
                class="flex-1 justify-center"
                @click="form.asset_class = 'tangible'"
              >
                {{ t('masterdata.categories.class.tangible') }}
              </UButton>
              <UButton
                :color="form.asset_class === 'intangible' ? 'primary' : 'neutral'"
                :variant="form.asset_class === 'intangible' ? 'solid' : 'outline'"
                icon="i-lucide-sparkles"
                class="flex-1 justify-center"
                @click="form.asset_class = 'intangible'"
              >
                {{ t('masterdata.categories.class.intangible') }}
              </UButton>
            </div>
          </UFormField>

          <label class="flex items-center justify-between gap-2 rounded-[10px] bg-muted px-3 h-11 cursor-pointer">
            <span class="text-sm font-semibold">{{ t('masterdata.categories.fields.active') }}</span>
            <USwitch v-model="form.is_active" />
          </label>
        </div>
      </section>

      <!-- Section 2: Penyusutan / Amortisasi -->
      <section class="border-t border-default pt-5">
        <div class="flex items-center gap-2 mb-1">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">2</span>
          <span class="font-semibold text-sm">{{ susutTitle }}</span>
        </div>
        <div class="text-[11.5px] text-dimmed mb-3 ms-8">{{ susutRef }}</div>
        <div class="space-y-3">
          <UFormField :label="t('masterdata.categories.fields.method')">
            <USelect
              v-model="form.default_depreciation_method"
              :items="methodOptions"
              :disabled="metodeLocked"
              class="w-full"
            />
            <template
              v-if="metodeLocked"
              #hint
            >
              <span class="flex items-center gap-1 text-xs text-warning mt-1">
                <UIcon
                  name="i-lucide-lock"
                  class="size-3"
                />
                {{ t('masterdata.categories.hint.buildingLock') }}
              </span>
            </template>
          </UFormField>
          <div class="grid grid-cols-2 gap-3">
            <UFormField :label="t('masterdata.categories.fields.life')">
              <UInput
                v-model="form.default_useful_life_months"
                inputmode="numeric"
                placeholder="48"
                :trailing="false"
                class="w-full"
              >
                <template #trailing>
                  <span class="text-xs text-dimmed">{{ t('masterdata.categories.months') }}</span>
                </template>
              </UInput>
            </UFormField>
            <UFormField :label="t('masterdata.categories.fields.salvage')">
              <UInput
                v-model="form.default_salvage_rate"
                inputmode="numeric"
                placeholder="0"
                class="w-full"
              >
                <template #trailing>
                  <span class="text-xs text-dimmed">%</span>
                </template>
              </UInput>
            </UFormField>
          </div>
        </div>
      </section>

      <!-- Section 3: Pajak / Fiskal -->
      <section class="border-t border-default pt-5">
        <div class="flex items-center gap-2 mb-1">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">3</span>
          <span class="font-semibold text-sm">{{ t('masterdata.categories.section.tax') }}</span>
        </div>
        <div class="text-[11.5px] text-dimmed mb-3 ms-8">{{ t('masterdata.categories.section.taxRef') }}</div>
        <div class="space-y-3">
          <UFormField :label="t('masterdata.categories.fields.fiscalGroup')">
            <USelect
              v-model="form.default_fiscal_group"
              :items="[{ value: '', label: t('masterdata.categories.placeholders.select') }, ...fiscalGroupOptions]"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('masterdata.categories.fields.fiscalLife')">
            <UInput
              v-model="form.default_fiscal_life_months"
              inputmode="numeric"
              placeholder="48"
              class="w-full"
            >
              <template #trailing>
                <span class="text-xs text-dimmed">{{ t('masterdata.categories.months') }}</span>
              </template>
            </UInput>
          </UFormField>
        </div>
      </section>

      <!-- Section 4: Akuntansi -->
      <section class="border-t border-default pt-5">
        <div class="flex items-center gap-2 mb-3">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">4</span>
          <span class="font-semibold text-sm">{{ t('masterdata.categories.section.accounting') }}</span>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <UFormField
            :label="t('masterdata.categories.fields.gl')"
            :hint="t('masterdata.categories.hint.gl')"
          >
            <UInput
              v-model="form.gl_account_code"
              placeholder="1.2.3.01"
              class="w-full font-mono"
            />
          </UFormField>
          <UFormField
            :label="t('masterdata.categories.fields.capitalization')"
            :hint="t('masterdata.categories.hint.capitalization')"
          >
            <UInput
              :model-value="form.capitalization_threshold"
              inputmode="numeric"
              placeholder="1.000.000"
              class="w-full"
              @input="onCapitalInput"
            >
              <template #leading>
                <span class="text-xs text-dimmed">Rp</span>
              </template>
            </UInput>
          </UFormField>
        </div>
      </section>
    </div>
  </FormSlideover>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm test -- CategoryFormSlideover`
Expected: PASS (5 tests). If the trailing-slot `<template #trailing>` interferes with `inputmode`, that is cosmetic — the assertions target labels/payload, not slot internals.

- [ ] **Step 5: Lint + typecheck + commit**

Run: `pnpm lint` then `pnpm typecheck`
Expected: clean (watch for trailing commas).

```bash
git add frontend/app/components/category/CategoryFormSlideover.vue frontend/test/nuxt/CategoryFormSlideover.spec.ts
git commit -m "feat(masterdata): CategoryFormSlideover with conditional depreciation/fiscal logic"
```

---

### Task 4: categories.vue page

**Files:**
- Create: `frontend/app/pages/master/categories.vue`
- Test: `frontend/test/nuxt/master-categories.spec.ts`

**Interfaces:**
- Consumes: `useCategories`, `CategoryFormSlideover`, shared `PageHeader`, `ResourceTable`, `<Can>`, `useCan`, `useConfirm`, `useToast`. The page does not import `mock/categories` directly — all data flows through `useCategories`. Auto-exposed bindings used by tests: `openCreate`, `filterClass`, `filterGroup`, `activeOnly`, `formOpen`.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/nuxt/master-categories.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import CategoriesPage from '~/pages/master/categories.vue'

async function mountLoaded() {
  const wrapper = await mountSuspended(CategoriesPage)
  await new Promise(r => setTimeout(r, 350))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Master Data Kategori Aset page', () => {
  it('renders the title and seeded categories after load', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Kategori Aset')
    expect(html).toContain('Komputer & Laptop')
    expect(html).toContain('Kendaraan Bermotor')
  })

  it('renders class badges (Berwujud / Takberwujud)', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Berwujud')
    expect(html).toContain('Takberwujud')
  })

  it('renders translated method and fiscal-group labels', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Garis Lurus')
    expect(html).toContain('Saldo Menurun')
    expect(html).toContain('Bangunan Permanen')
  })

  it('renders the GL account codes', async () => {
    const wrapper = await mountLoaded()
    expect(wrapper.html()).toContain('1.2.3.01')
  })

  it('renders the filter controls', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Semua Kelas')
    expect(html).toContain('Semua Golongan')
    expect(html).toContain('Hanya aktif')
  })

  it('class filter narrows results — Takberwujud shows only intangible rows', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { filterClass: string }
    vm.filterClass = 'intangible'
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Software / Lisensi')
    expect(html).not.toContain('Kendaraan Bermotor')
  })

  it('active-only filter hides the inactive (Legacy) row', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { activeOnly: boolean }
    expect(wrapper.html()).toContain('Peralatan Jaringan (Legacy)')
    vm.activeOnly = true
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).not.toContain('Peralatan Jaringan (Legacy)')
  })

  it('opens the slideover with form labels when Add is triggered', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { formOpen: boolean, openCreate: () => void }
    vm.openCreate()
    await wrapper.vm.$nextTick()
    expect(vm.formOpen).toBe(true)
    const body = document.body.innerHTML
    expect(body).toContain('Nama Kategori')
    expect(body).toContain('Golongan / Kelompok Harta')
    expect(body).toContain('Akun GL (COA)')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test -- master-categories`
Expected: FAIL — cannot resolve `~/pages/master/categories.vue`.

- [ ] **Step 3: Create the page**

Create `frontend/app/pages/master/categories.vue`:

```vue
<script setup lang="ts">
import type { Category, RowAction } from '~/types'
import type { CategoryInput } from '~/composables/api/useCategories'

definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const { open: confirm } = useConfirm()
const api = useCategories()

const PERM = 'masterdata.global.manage'
const ALL = '__all__'
const PAGE_SIZE = 7

const allRows = ref<Category[]>([])
const search = ref('')
const filterClass = ref(ALL)
const filterGroup = ref(ALL)
const activeOnly = ref(false)
const offset = ref(0)
const loading = ref(true)

const formOpen = ref(false)
const saving = ref(false)
const editing = ref<Category | null>(null)

const columns = [
  { accessorKey: 'name', header: t('masterdata.categories.columns.name'), sortable: true },
  { accessorKey: 'code', header: t('masterdata.categories.columns.code'), sortable: true },
  { accessorKey: 'class', header: t('masterdata.categories.columns.class') },
  { accessorKey: 'method', header: t('masterdata.categories.columns.method') },
  { accessorKey: 'life', header: t('masterdata.categories.columns.life') },
  { accessorKey: 'fiscalGroup', header: t('masterdata.categories.columns.fiscalGroup') },
  { accessorKey: 'gl', header: t('masterdata.categories.columns.gl') },
  { accessorKey: 'status', header: t('masterdata.categories.columns.status') }
]

const groupOptions = computed(() =>
  (['kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen'] as const)
    .map(g => ({ value: g, label: t(`masterdata.categories.fiscalGroup.${g}`) }))
)

const anyFilterActive = computed(() =>
  !!(search.value.trim() || filterClass.value !== ALL || filterGroup.value !== ALL || activeOnly.value)
)

const nameById = computed<Record<string, string>>(() => {
  const m: Record<string, string> = {}
  for (const c of allRows.value) m[c.id] = c.name
  return m
})

const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  return allRows.value.filter((r) => {
    if (q && !r.name.toLowerCase().includes(q) && !(r.code ?? '').toLowerCase().includes(q)) return false
    if (filterClass.value !== ALL && r.asset_class !== filterClass.value) return false
    if (filterGroup.value !== ALL && r.default_fiscal_group !== filterGroup.value) return false
    if (activeOnly.value && !r.is_active) return false
    return true
  })
})

// Keep children directly after their parent for indented display.
const orderedRows = computed(() => {
  const rows = filteredRows.value
  const byParent = new Map<string | null, Category[]>()
  for (const r of rows) {
    const key = r.parent_id
    if (!byParent.has(key)) byParent.set(key, [])
    byParent.get(key)!.push(r)
  }
  const present = new Set(rows.map(r => r.id))
  const out: Category[] = []
  const pushTree = (parent: string | null) => {
    for (const r of byParent.get(parent) ?? []) {
      out.push(r)
      if (byParent.has(r.id)) pushTree(r.id)
    }
  }
  // Roots = rows whose parent isn't in the current filtered set.
  for (const r of rows) {
    if (!r.parent_id || !present.has(r.parent_id)) {
      if (!out.includes(r)) {
        out.push(r)
        if (byParent.has(r.id)) pushTree(r.id)
      }
    }
  }
  return out
})

const pagedRows = computed(() =>
  orderedRows.value.slice(offset.value, offset.value + PAGE_SIZE)
    .map(r => ({ ...r })) as unknown as Record<string, unknown>[]
)

// Parent options exclude self and the editing row's descendants.
function descendantIds(id: string): Set<string> {
  const ids = new Set<string>()
  const walk = (pid: string) => {
    for (const c of allRows.value) {
      if (c.parent_id === pid && !ids.has(c.id)) {
        ids.add(c.id)
        walk(c.id)
      }
    }
  }
  walk(id)
  return ids
}

const parentOptions = computed(() => {
  const exclude = new Set<string>()
  if (editing.value) {
    exclude.add(editing.value.id)
    for (const d of descendantIds(editing.value.id)) exclude.add(d)
  }
  return allRows.value
    .filter(c => !exclude.has(c.id))
    .map(c => ({ value: c.id, label: c.name }))
})

async function refresh() {
  loading.value = true
  const res = await api.list({ limit: 100 })
  allRows.value = res.data
  loading.value = false
}

function openCreate() {
  editing.value = null
  formOpen.value = true
}

function openEdit(row: Category) {
  editing.value = row
  formOpen.value = true
}

async function onSubmit(input: CategoryInput) {
  saving.value = true
  try {
    if (editing.value) await api.update(editing.value.id, input)
    else await api.create(input)
    formOpen.value = false
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete(row: Category) {
  const ok = await confirm({
    title: t('common.delete'),
    description: t('masterdata.categories.deleteConfirm', { name: row.name })
  })
  if (!ok) return
  await api.remove(row.id)
  await refresh()
}

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can(PERM)) return []
  const r = row as unknown as Category
  return [
    { label: t('common.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    { label: t('common.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

function resetFilters() {
  search.value = ''
  filterClass.value = ALL
  filterGroup.value = ALL
  activeOnly.value = false
  offset.value = 0
}

watch([search, filterClass, filterGroup, activeOnly], () => {
  offset.value = 0
})

onMounted(refresh)
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.categories.title')"
      :subtitle="t('masterdata.categories.subtitle')"
    >
      <template #actions>
        <Can :permission="PERM">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.categories.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <!-- Filter bar -->
    <div class="bg-default border border-default rounded-[13px] shadow p-[14px] mb-4 flex flex-wrap items-center gap-[10px]">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('masterdata.categories.searchPlaceholder')"
        class="flex-1 min-w-[200px]"
      />
      <USelect
        v-model="filterClass"
        :items="[
          { value: ALL, label: t('masterdata.categories.filter.allClass') },
          { value: 'tangible', label: t('masterdata.categories.class.tangible') },
          { value: 'intangible', label: t('masterdata.categories.class.intangible') }
        ]"
        class="min-w-[150px]"
      />
      <USelect
        v-model="filterGroup"
        :items="[{ value: ALL, label: t('masterdata.categories.filter.allGroup') }, ...groupOptions]"
        class="min-w-[170px]"
      />
      <label class="flex items-center gap-2 px-3 h-9 rounded-[9px] border border-default cursor-pointer">
        <USwitch v-model="activeOnly" />
        <span class="text-sm text-muted">{{ t('masterdata.categories.filter.activeOnly') }}</span>
      </label>
      <UButton
        v-if="anyFilterActive"
        color="error"
        variant="ghost"
        icon="i-lucide-x"
        @click="resetFilters"
      >
        {{ t('common.reset') }}
      </UButton>
    </div>

    <ResourceTable
      :rows="pagedRows"
      :columns="columns"
      :loading="loading"
      :total="orderedRows.length"
      :limit="PAGE_SIZE"
      :offset="offset"
      :empty-title="anyFilterActive ? t('masterdata.categories.emptyFilter') : t('masterdata.categories.empty')"
      :actions="rowActions"
      @update:offset="offset = $event"
    >
      <template #name-cell="{ row }">
        <div
          class="flex items-center gap-2"
          :class="(row as unknown as Category).parent_id ? 'ps-6' : ''"
        >
          <UIcon
            v-if="(row as unknown as Category).parent_id"
            name="i-lucide-corner-down-right"
            class="size-3.5 text-dimmed flex-none"
          />
          <span class="font-medium">{{ (row as unknown as Category).name }}</span>
        </div>
      </template>

      <template #code-cell="{ row }">
        <UBadge
          color="neutral"
          variant="subtle"
          class="font-mono"
        >
          {{ (row as unknown as Category).code ?? '—' }}
        </UBadge>
      </template>

      <template #class-cell="{ row }">
        <UBadge
          :color="(row as unknown as Category).asset_class === 'intangible' ? 'info' : 'success'"
          variant="subtle"
        >
          {{ t(`masterdata.categories.class.${(row as unknown as Category).asset_class}`) }}
        </UBadge>
      </template>

      <template #method-cell="{ row }">
        <span class="text-muted">
          {{ (row as unknown as Category).default_depreciation_method
            ? t(`masterdata.categories.method.${(row as unknown as Category).default_depreciation_method}`)
            : '—' }}
        </span>
      </template>

      <template #life-cell="{ row }">
        <span class="tabular-nums">{{ (row as unknown as Category).default_useful_life_months ?? '—' }}</span>
      </template>

      <template #fiscalGroup-cell="{ row }">
        <span class="text-muted">
          {{ (row as unknown as Category).default_fiscal_group
            ? t(`masterdata.categories.fiscalGroup.${(row as unknown as Category).default_fiscal_group}`)
            : '—' }}
        </span>
      </template>

      <template #gl-cell="{ row }">
        <span class="font-mono text-sm text-muted">{{ (row as unknown as Category).gl_account_code ?? '—' }}</span>
      </template>

      <template #status-cell="{ row }">
        <UBadge
          :color="(row as unknown as Category).is_active ? 'success' : 'neutral'"
          variant="subtle"
        >
          {{ (row as unknown as Category).is_active ? t('common.active') : t('common.inactive') }}
        </UBadge>
      </template>
    </ResourceTable>

    <CategoryFormSlideover
      v-model:open="formOpen"
      :category="editing"
      :parent-options="parentOptions"
      :loading="saving"
      @submit="onSubmit"
    />
  </div>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm test -- master-categories`
Expected: PASS (8 tests).

- [ ] **Step 5: Full unit/runtime suite + lint + typecheck**

Run: `pnpm test`
Expected: all green (existing + new categories specs).
Run: `pnpm lint` then `pnpm typecheck`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/pages/master/categories.vue frontend/test/nuxt/master-categories.spec.ts
git commit -m "feat(masterdata): Kategori Aset page (list, filters, slideover wiring)"
```

---

### Task 5: E2E (Playwright)

**Files:**
- Create: `frontend/e2e/categories.spec.ts`

**Interfaces:**
- Consumes: real backend login (`admin@inventra.local` / `admin12345`), the built mock-backed page at `/master/categories`.

- [ ] **Step 1: Write the e2e spec**

Create `frontend/e2e/categories.spec.ts` (pattern copied from `master-offices.spec.ts`):

```ts
import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'

const EMAIL = process.env.E2E_EMAIL || 'admin@inventra.local'
const PASSWORD = process.env.E2E_PASSWORD || 'admin12345'

async function login(page: Page) {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

test.describe('Master Data Kategori Aset (mock-backed)', () => {
  test('lists seeded categories and creates a new one', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')

    // Seeded categories are visible.
    await expect(page.getByText('Komputer & Laptop')).toBeVisible()
    await expect(page.getByText('Kendaraan Bermotor')).toBeVisible()

    // Open the create slideover and add a category.
    await page.getByRole('button', { name: 'Tambah Kategori' }).click()
    await page.getByLabel('Nama Kategori').fill('Genset E2E')
    await page.getByLabel('Kode').fill('GEN')
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // New category appears in the table.
    await expect(page.getByText('Genset E2E')).toBeVisible()
  })

  test('search narrows the list', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')
    await page.getByPlaceholder('Cari nama atau kode…').fill('Kendaraan')
    await expect(page.getByText('Kendaraan Bermotor')).toBeVisible()
    await expect(page.getByText('Komputer & Laptop')).toHaveCount(0)
  })
})
```

- [ ] **Step 2: Run e2e (requires the stack up + seeded admin)**

Ensure the dev stack is running and admin is seeded (already true this session). Run:
`pnpm test:e2e -- categories`
Expected: PASS (2 tests). If the dev server isn't the Playwright `webServer`, follow the repo's existing `pnpm test:e2e` setup (same as `master-offices`).

- [ ] **Step 3: Commit**

```bash
git add frontend/e2e/categories.spec.ts
git commit -m "test(e2e): Kategori Aset list, create, and search flows"
```

---

### Task 6: Visual parity pass + finalize

**Files:** none (verification + any fixes uncovered).

- [ ] **Step 1: Build**

Run: `pnpm build`
Expected: success.

- [ ] **Step 2: Side-by-side visual comparison**

Open the built screen at `/master/categories` and `docs/design/Kategori Aset.dc.html` side by side, in **light and dark**. Verify 1:1: header + Add button, filter bar (search, 2 selects, active-only toggle), table columns & badges, child indentation, pagination (page size 7), empty state, and the 4-section slideover (including the Takberwujud→Amortisasi/PSAK 19 relabel and the Bangunan→Garis Lurus lock). Fix any deviation found; do not redesign or defer any part of the mockup.

- [ ] **Step 3: Final green gate**

Run: `pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green.

- [ ] **Step 4: Commit any parity fixes**

```bash
git add -A
git commit -m "fix(masterdata): Kategori Aset visual parity with mockup (light/dark)"
```

---

## Self-Review

**Spec coverage** (spec §-by-§):
- §3 berkas: types/mock/composable (Task 1), i18n/nav (Task 2), CategoryFormSlideover (Task 3), page (Task 4) — covered. Note: spec §3 listed a separate `CategoryTable.vue`; this plan instead reuses the shared `ResourceTable` inline (matching `employees.vue`), which satisfies the "pages tipis" intent with less duplication. **This is a deliberate refinement of the spec** — only `CategoryFormSlideover` is extracted.
- §4 kontrak data: Task 1 types + `CategoryInput` — covered.
- §5 tata letak: Task 4 page (header/filter/table/empty/pagination) — covered.
- §6 form 4 section + perilaku kondisional + validasi: Task 3 — covered (intangible relabel, building lock, name/code required, snake_case payload).
- §7 i18n: Task 2 (id+en, no hardcoded) — covered.
- §8 nav: Task 2 — covered.
- §9 testing: unit (Task 1), nav (Task 2), runtime form (Task 3), runtime page (Task 4), e2e (Task 5), parity (Task 6) — covered.

**Placeholder scan:** no TBD/TODO; every code step shows full code; commands have expected output. Clean.

**Type consistency:** `Category`/`CategoryInput` field names identical across Tasks 1/3/4. `useCategories` returns `{ list, get, create, update, remove }` used consistently. `formatThousands`/`parseThousands`/`isBuildingGroup`/`FISCAL_GROUPS` signatures match between `mock/categories.ts` (Task 1) and `CategoryFormSlideover.vue` (Task 3). i18n keys referenced in Tasks 3/4 all defined in Task 2. Nav `to: '/master/categories'` matches the page path. Consistent.
