// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { useConfirm } from '~/composables/useConfirm'
import type { Category } from '~/types'
import type { CategoryInput } from '~/composables/api/useCategories'
import CategoriesPage from '~/pages/master/categories.vue'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request are intercepted here.
// Individual tests call setHandler() to configure per-request behaviour.
// ---------------------------------------------------------------------------

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

let _handler: RequestHandler = () => {
  throw new Error('No handler set')
}

function setHandler(fn: RequestHandler) {
  _handler = fn
}

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => Promise.resolve(_handler(path, opts))
  })
}))

// ---------------------------------------------------------------------------
// Fixtures — parent + child (child has parent_id = parent.id) + extras
// covering tangible/intangible + a building group + an inactive row.
// ---------------------------------------------------------------------------

const CAT_IT: Category = {
  id: 'c-it',
  name: 'Perangkat IT',
  code: 'ITX',
  parent_id: null,
  default_depreciation_method: 'straight_line',
  default_useful_life_months: 48,
  default_salvage_rate: '0',
  asset_class: 'tangible',
  default_fiscal_group: 'kelompok_1',
  default_fiscal_life_months: 48,
  gl_account_code: '1.2.3.00',
  capitalization_threshold: '1000000',
  is_active: true,
  created_at: '2026-01-05'
}

const CAT_LAPTOP: Category = {
  id: 'c-laptop',
  name: 'Komputer & Laptop',
  code: 'ELK',
  parent_id: 'c-it',
  default_depreciation_method: 'straight_line',
  default_useful_life_months: 48,
  default_salvage_rate: '0',
  asset_class: 'tangible',
  default_fiscal_group: 'kelompok_1',
  default_fiscal_life_months: 48,
  gl_account_code: '1.2.3.01',
  capitalization_threshold: '1000000',
  is_active: true,
  created_at: '2026-01-06'
}

const CAT_VEHICLE: Category = {
  id: 'c-vehicle',
  name: 'Kendaraan Bermotor',
  code: 'KEN',
  parent_id: null,
  default_depreciation_method: 'declining_balance',
  default_useful_life_months: 96,
  default_salvage_rate: '10',
  asset_class: 'tangible',
  default_fiscal_group: 'kelompok_2',
  default_fiscal_life_months: 96,
  gl_account_code: '1.2.4.00',
  capitalization_threshold: '10000000',
  is_active: true,
  created_at: '2026-01-07'
}

const CAT_BUILDING: Category = {
  id: 'c-building',
  name: 'Bangunan Kantor',
  code: 'BGN',
  parent_id: null,
  default_depreciation_method: 'straight_line',
  default_useful_life_months: 240,
  default_salvage_rate: '0',
  asset_class: 'tangible',
  default_fiscal_group: 'bangunan_permanen',
  default_fiscal_life_months: 240,
  gl_account_code: '1.2.1.00',
  capitalization_threshold: '50000000',
  is_active: true,
  created_at: '2026-01-08'
}

const CAT_SOFTWARE: Category = {
  id: 'c-software',
  name: 'Software / Lisensi',
  code: 'SFT',
  parent_id: null,
  default_depreciation_method: 'straight_line',
  default_useful_life_months: 48,
  default_salvage_rate: '0',
  asset_class: 'intangible',
  default_fiscal_group: 'kelompok_1',
  default_fiscal_life_months: 48,
  gl_account_code: '1.2.6.00',
  capitalization_threshold: '5000000',
  is_active: true,
  created_at: '2026-01-11'
}

const CAT_LEGACY: Category = {
  id: 'c-legacy',
  name: 'Peralatan Jaringan (Legacy)',
  code: 'NET',
  parent_id: null,
  default_depreciation_method: 'straight_line',
  default_useful_life_months: 48,
  default_salvage_rate: '0',
  asset_class: 'tangible',
  default_fiscal_group: 'kelompok_1',
  default_fiscal_life_months: 48,
  gl_account_code: '1.2.3.09',
  capitalization_threshold: '1000000',
  is_active: false,
  created_at: '2026-01-12'
}

// All categories served by the default handler
const CATEGORIES: Category[] = [
  CAT_IT,
  CAT_LAPTOP,
  CAT_VEHICLE,
  CAT_BUILDING,
  CAT_SOFTWARE,
  CAT_LEGACY
]

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path === '/categories/tree') return { data: CATEGORIES }
  throw new Error(`Unhandled request: ${path} ${JSON.stringify(opts)}`)
}

// ---------------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------------

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => {
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(CategoriesPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

// ---------------------------------------------------------------------------
// Loaded rows — rendered text and indentation
// ---------------------------------------------------------------------------

describe('Kategori Aset page — loaded rows', () => {
  it('renders page title after load', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Kategori Aset')
  })

  it('renders parent category names', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Perangkat IT')
    expect(text).toContain('Kendaraan Bermotor')
    expect(text).toContain('Bangunan Kantor')
    expect(text).toContain('Software / Lisensi')
  })

  it('renders the child row (Komputer & Laptop)', async () => {
    const wrapper = await mountAndWait()
    // '&' is encoded as '&amp;' in innerHTML — use text()
    expect(wrapper.text()).toContain('Komputer & Laptop')
  })

  it('orderedRows places the child immediately after its parent', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { orderedRows: Category[] }
    const rows = vm.orderedRows
    const parentIdx = rows.findIndex(r => r.id === 'c-it')
    const childIdx = rows.findIndex(r => r.id === 'c-laptop')
    expect(parentIdx).toBeGreaterThanOrEqual(0)
    expect(childIdx).toBe(parentIdx + 1)
  })

  it('child name-cell carries the ps-6 indent class', async () => {
    const wrapper = await mountAndWait()
    // The child row's name-cell div gets class ps-6 when parent_id is set
    expect(wrapper.html()).toContain('ps-6')
  })

  it('child name-cell ps-6 indent is present (parent row has none)', async () => {
    const wrapper = await mountAndWait()
    const html = wrapper.html()
    // ps-6 is applied only on child rows (parent_id is set). It must appear at least once.
    expect(html).toContain('ps-6')
    // The parent row (Perangkat IT, no parent_id) must NOT carry ps-6 in its name-cell.
    // orderedRows confirms the child follows the parent.
    const vm = wrapper.vm as unknown as { orderedRows: Category[] }
    const parent = vm.orderedRows.find(r => r.id === 'c-it')
    const child = vm.orderedRows.find(r => r.id === 'c-laptop')
    expect(parent?.parent_id).toBeNull()
    expect(child?.parent_id).toBe('c-it')
  })

  it('renders class badges Berwujud and Takberwujud', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Berwujud')
    expect(text).toContain('Takberwujud')
  })

  it('renders translated depreciation method labels', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Garis Lurus')
    expect(text).toContain('Saldo Menurun')
  })

  it('renders translated fiscal group label (Bangunan Permanen)', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Bangunan Permanen')
  })

  it('renders the GL account code for the child row', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.html()).toContain('1.2.3.01')
  })

  it('renders filter controls', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Semua Kelas')
    expect(text).toContain('Semua Golongan')
    expect(text).toContain('Hanya aktif')
  })
})

// ---------------------------------------------------------------------------
// Class filter
// ---------------------------------------------------------------------------

describe('Kategori Aset page — class filter', () => {
  it('filterClass=intangible narrows to intangible rows only', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { filterClass: string, orderedRows: Category[] }
    vm.filterClass = 'intangible'
    await wrapper.vm.$nextTick()
    const rows = vm.orderedRows
    // Only Software / Lisensi is intangible in fixtures
    expect(rows.every(r => r.asset_class === 'intangible')).toBe(true)
    expect(rows.some(r => r.name.includes('Software'))).toBe(true)
    expect(rows.some(r => r.name.includes('Kendaraan'))).toBe(false)
    expect(rows.some(r => r.name === 'Perangkat IT')).toBe(false)
  })

  it('filterClass=tangible excludes intangible rows', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { filterClass: string, orderedRows: Category[] }
    vm.filterClass = 'tangible'
    await wrapper.vm.$nextTick()
    expect(vm.orderedRows.every(r => r.asset_class === 'tangible')).toBe(true)
    expect(vm.orderedRows.some(r => r.name.includes('Software'))).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Fiscal-group filter
// ---------------------------------------------------------------------------

describe('Kategori Aset page — fiscal group filter', () => {
  it('filterGroup=bangunan_permanen shows only building rows', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { filterGroup: string, orderedRows: Category[] }
    vm.filterGroup = 'bangunan_permanen'
    await wrapper.vm.$nextTick()
    const rows = vm.orderedRows
    expect(rows.every(r => r.default_fiscal_group === 'bangunan_permanen')).toBe(true)
    expect(rows.some(r => r.name === 'Bangunan Kantor')).toBe(true)
    expect(rows.some(r => r.name === 'Kendaraan Bermotor')).toBe(false)
  })

  it('filterGroup=kelompok_2 narrows to kelompok_2 rows', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { filterGroup: string, orderedRows: Category[] }
    vm.filterGroup = 'kelompok_2'
    await wrapper.vm.$nextTick()
    const rows = vm.orderedRows
    expect(rows.every(r => r.default_fiscal_group === 'kelompok_2')).toBe(true)
    expect(rows.some(r => r.name === 'Kendaraan Bermotor')).toBe(true)
    expect(rows.some(r => r.name === 'Bangunan Kantor')).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Active-only filter
// ---------------------------------------------------------------------------

describe('Kategori Aset page — activeOnly filter', () => {
  it('activeOnly hides the inactive Legacy row', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { activeOnly: boolean, orderedRows: Category[] }
    // Legacy row should be present before filter
    expect(vm.orderedRows.some(r => r.id === 'c-legacy')).toBe(true)
    vm.activeOnly = true
    await wrapper.vm.$nextTick()
    expect(vm.orderedRows.some(r => r.id === 'c-legacy')).toBe(false)
  })

  it('activeOnly keeps all active rows', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { activeOnly: boolean, orderedRows: Category[] }
    vm.activeOnly = true
    await wrapper.vm.$nextTick()
    expect(vm.orderedRows.every(r => r.is_active)).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Parent options
// ---------------------------------------------------------------------------

describe('Kategori Aset page — parentOptions', () => {
  it('parentOptions excludes the editing row itself', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as {
      openEdit: (row: Category) => void
      parentOptions: { value: string, label: string }[]
    }
    vm.openEdit(CAT_IT)
    await wrapper.vm.$nextTick()
    const ids = vm.parentOptions.map(o => o.value)
    expect(ids).not.toContain('c-it')
  })

  it('parentOptions excludes descendants of the editing row', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as {
      openEdit: (row: Category) => void
      parentOptions: { value: string, label: string }[]
    }
    // c-laptop is a descendant of c-it
    vm.openEdit(CAT_IT)
    await wrapper.vm.$nextTick()
    const ids = vm.parentOptions.map(o => o.value)
    expect(ids).not.toContain('c-laptop')
  })

  it('parentOptions includes unrelated rows when editing a parent', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as {
      openEdit: (row: Category) => void
      parentOptions: { value: string, label: string }[]
    }
    vm.openEdit(CAT_IT)
    await wrapper.vm.$nextTick()
    const ids = vm.parentOptions.map(o => o.value)
    expect(ids).toContain('c-vehicle')
    expect(ids).toContain('c-building')
    expect(ids).toContain('c-software')
  })
})

// ---------------------------------------------------------------------------
// Create mutation
// ---------------------------------------------------------------------------

describe('Kategori Aset page — create', () => {
  it('POST /categories body has the expected fields', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path === '/categories/tree') return { data: CATEGORIES }
      if (path === '/categories' && opts?.method === 'POST') {
        capturedPath = path
        capturedOpts = opts
        return { ...CAT_SOFTWARE, id: 'c-new' }
      }
      throw new Error(`Unhandled: ${path} ${JSON.stringify(opts)}`)
    })

    const wrapper = await mountAndWait()

    const input: CategoryInput = {
      name: 'Lisensi Antivirus',
      code: 'AV',
      parent_id: null,
      asset_class: 'intangible',
      default_depreciation_method: 'straight_line',
      default_useful_life_months: 12,
      default_salvage_rate: '0',
      default_fiscal_group: 'kelompok_1',
      default_fiscal_life_months: 12,
      gl_account_code: '1.2.6.01',
      capitalization_threshold: '1000000',
      is_active: true
    }

    await (wrapper.vm as unknown as { onSubmit: (i: CategoryInput) => Promise<void> }).onSubmit(input)
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/categories')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['name']).toBe('Lisensi Antivirus')
    expect(body['code']).toBe('AV')
    expect(body['asset_class']).toBe('intangible')
    expect(body['parent_id']).toBeNull()
    expect(body['is_active']).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Edit mutation
// ---------------------------------------------------------------------------

describe('Kategori Aset page — edit', () => {
  it('PUT /categories/:id is called with the correct id and body', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path === '/categories/tree') return { data: CATEGORIES }
      if (path.startsWith('/categories/') && opts?.method === 'PUT') {
        capturedPath = path
        capturedOpts = opts
        return { ...CAT_VEHICLE, name: 'Kendaraan Dinas' }
      }
      throw new Error(`Unhandled: ${path} ${JSON.stringify(opts)}`)
    })

    const wrapper = await mountAndWait()

    // Open edit for the vehicle row
    ;(wrapper.vm as unknown as { openEdit: (row: Category) => void }).openEdit(CAT_VEHICLE)
    await wrapper.vm.$nextTick()

    const input: CategoryInput = {
      name: 'Kendaraan Dinas',
      code: 'KEN',
      parent_id: null,
      asset_class: 'tangible',
      default_depreciation_method: 'declining_balance',
      default_useful_life_months: 96,
      default_salvage_rate: '10',
      default_fiscal_group: 'kelompok_2',
      default_fiscal_life_months: 96,
      gl_account_code: '1.2.4.00',
      capitalization_threshold: '10000000',
      is_active: true
    }

    await (wrapper.vm as unknown as { onSubmit: (i: CategoryInput) => Promise<void> }).onSubmit(input)
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/categories/c-vehicle')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['name']).toBe('Kendaraan Dinas')
    expect(body['asset_class']).toBe('tangible')
  })
})

// ---------------------------------------------------------------------------
// Delete mutation
// ---------------------------------------------------------------------------

describe('Kategori Aset page — delete', () => {
  it('DELETE /categories/:id is called after confirmation', async () => {
    let deletedPath = ''

    setHandler((path, opts) => {
      if (path === '/categories/tree') return { data: CATEGORIES }
      if (path.startsWith('/categories/') && opts?.method === 'DELETE') {
        deletedPath = path
        return undefined
      }
      throw new Error(`Unhandled: ${path} ${JSON.stringify(opts)}`)
    })

    const wrapper = await mountAndWait()

    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: Category) => Promise<void> }).onDelete(CAT_LEGACY)
    await wrapper.vm.$nextTick()

    useConfirm().resolve(true)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(deletedPath).toBe('/categories/c-legacy')
  })

  it('DELETE is NOT called when user cancels confirmation', async () => {
    let deleteCalled = false

    setHandler((path, opts) => {
      if (path === '/categories/tree') return { data: CATEGORIES }
      if (path.startsWith('/categories/') && opts?.method === 'DELETE') {
        deleteCalled = true
        return undefined
      }
      throw new Error(`Unhandled: ${path} ${JSON.stringify(opts)}`)
    })

    const wrapper = await mountAndWait()

    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: Category) => Promise<void> }).onDelete(CAT_LEGACY)
    await wrapper.vm.$nextTick()

    useConfirm().resolve(false)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))

    expect(deleteCalled).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Load error state
// ---------------------------------------------------------------------------

describe('Kategori Aset page — load error', () => {
  it('shows load-error text when GET /categories/tree rejects', async () => {
    setHandler((path) => {
      if (path === '/categories/tree') {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    // i18n: masterdata.categories.loadError
    expect(wrapper.text()).toContain('Gagal memuat kategori.')
    expect(wrapper.text()).not.toContain('Perangkat IT')
  })

  it('shows a retry button on load error', async () => {
    setHandler((path) => {
      if (path === '/categories/tree') {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    // i18n: common.retry
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
  })

  it('retry button re-fetches and recovers when second call succeeds', async () => {
    let callCount = 0
    setHandler((path) => {
      if (path === '/categories/tree') {
        callCount++
        if (callCount === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return { data: CATEGORIES }
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat kategori.')

    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Perangkat IT')
    expect(wrapper.text()).not.toContain('Gagal memuat kategori.')
  })
})
