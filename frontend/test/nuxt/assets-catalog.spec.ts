// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request are intercepted here.
// useAssets, useCategories (tree) and useOffices (list) all go through
// useApiClient, so one mock covers everything the page needs.
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

// eslint-disable-next-line import/first
import CatalogPage from '~/pages/assets/index.vue'

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const CATEGORIES = [
  { id: 'c1', name: 'Elektronik' },
  { id: 'c2', name: 'Furnitur' }
]

const OFFICES = [
  { id: 'o1', name: 'Kantor Pusat' },
  { id: 'o2', name: 'Kantor Cabang' }
]

const BRANDS = [
  { id: 'b1', name: 'Dell' },
  { id: 'b2', name: 'Epson' }
]

const MODELS = [
  { id: 'm1', name: 'Latitude 5440', brand_id: 'b1' },
  { id: 'm2', name: 'EB-X51', brand_id: 'b2' }
]

const ASSETS = [
  {
    id: 'a1',
    asset_tag: 'JKT01-ELK-2026-00001',
    name: 'Laptop Dell Latitude 5440',
    category_id: 'c1',
    office_id: 'o1',
    brand_id: 'b1',
    model_id: 'm1',
    status: 'available',
    asset_class: 'tangible',
    purchase_date: '2026-01-12',
    purchase_cost: '18500000',
    book_value: '16200000'
  },
  {
    id: 'a2',
    asset_tag: 'JKT01-ELK-2026-00002',
    name: 'Proyektor Epson EB-X51',
    category_id: 'c1',
    office_id: 'o2',
    brand_id: 'b2',
    model_id: 'm2',
    status: 'assigned',
    asset_class: 'tangible',
    purchase_date: '2026-01-20',
    purchase_cost: '7200000',
    book_value: '6500000'
  },
  {
    id: 'a3',
    asset_tag: 'JKT01-FUR-2025-00011',
    name: 'Meja Kerja Ergonomis',
    category_id: 'c2',
    office_id: 'o1',
    brand_id: null,
    model_id: null,
    status: 'available',
    asset_class: 'tangible',
    purchase_date: '2025-06-18'
    // purchase_cost / book_value deliberately absent → masked by field permission
  }
]

function makeAssetsResponse(rows = ASSETS, total = ASSETS.length, limit = 20, offset = 0) {
  return { data: rows, total, limit, offset }
}

function makeOfficesResponse(rows = OFFICES) {
  return { data: rows, total: rows.length, limit: 100, offset: 0 }
}

function makeBrandsResponse(rows = BRANDS) {
  return { data: rows, total: rows.length, limit: 100, offset: 0 }
}

function makeModelsResponse(rows = MODELS) {
  return { data: rows, total: rows.length, limit: 100, offset: 0 }
}

interface Call { path: string, opts?: Record<string, unknown> }

const assetCalls: Call[] = []

function lastAssetQuery(): URLSearchParams {
  const call = assetCalls[assetCalls.length - 1]
  if (!call) throw new Error('No /assets call recorded')
  return new URLSearchParams(call.path.split('?')[1] ?? '')
}

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path.startsWith('/assets')) {
    assetCalls.push({ path, opts })
    return makeAssetsResponse()
  }
  if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
  if (path.startsWith('/brands')) return makeBrandsResponse()
  if (path.startsWith('/models')) return makeModelsResponse()
  if (path.startsWith('/offices')) return makeOfficesResponse()
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
  assetCalls.length = 0
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(CatalogPage)
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

async function setVmRef(wrapper: Awaited<ReturnType<typeof mountAndWait>>, key: string, value: unknown) {
  ;(wrapper.vm as unknown as Record<string, unknown>)[key] = value
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
}

// ---------------------------------------------------------------------------
// Loaded rows
// ---------------------------------------------------------------------------

describe('Asset Catalog page — loaded rows', () => {
  it('renders page title', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Katalog Aset')
  })

  it('renders asset rows from the stubbed list response', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Laptop Dell Latitude 5440')
    expect(text).toContain('Proyektor Epson EB-X51')
    expect(text).toContain('JKT01-ELK-2026-00001')
  })

  it('resolves category_id to category name — not raw id', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Elektronik')
    expect(text).toContain('Furnitur')
    expect(text).not.toContain('c1')
    expect(text).not.toContain('c2')
  })

  it('resolves office_id to office name — not raw id', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Kantor Pusat')
    expect(text).toContain('Kantor Cabang')
    expect(text).not.toContain('o1')
    expect(text).not.toContain('o2')
  })

  it('resolves brand_id/model_id to a combined brand + model name — not raw ids', async () => {
    const wrapper = await mountAndWait()
    const rows = wrapper.findAll('tr')
    // a1 (Laptop Dell) should have brand cell with 'Dell Latitude 5440'
    const a1Row = rows.find(tr => tr.text().includes('Laptop Dell Latitude 5440'))
    const a1BrandCell = a1Row?.find('[data-testid="asset-brand-cell"]')
    expect(a1BrandCell?.text()).toContain('Dell Latitude 5440')
    // a2 (Proyektor Epson) should have brand cell with 'Epson EB-X51'
    const a2Row = rows.find(tr => tr.text().includes('Proyektor Epson EB-X51'))
    const a2BrandCell = a2Row?.find('[data-testid="asset-brand-cell"]')
    expect(a2BrandCell?.text()).toContain('Epson EB-X51')
    // Raw ids should not appear
    expect(wrapper.text()).not.toContain('b1')
    expect(wrapper.text()).not.toContain('m1')
  })

  it('shows — for a row whose brand_id/model_id are null', async () => {
    const wrapper = await mountAndWait()
    // a3 (Meja Kerja Ergonomis) has brand_id/model_id null.
    const rows = wrapper.findAll('tr')
    const a3Row = rows.find(tr => tr.text().includes('Meja Kerja Ergonomis'))
    const a3BrandCell = a3Row?.find('[data-testid="asset-brand-cell"]')
    expect(a3BrandCell?.text()).toBe('—')
    // Ensure it doesn't contain any stubbed brand name
    expect(a3BrandCell?.text()).not.toContain('Dell')
    expect(a3BrandCell?.text()).not.toContain('Epson')
  })

  it('renders a resolved status badge label (English status → i18n)', async () => {
    const wrapper = await mountAndWait()
    // available → "Tersedia" (id)
    expect(wrapper.text()).toContain('Tersedia')
    // assigned → "Digunakan" (id)
    expect(wrapper.text()).toContain('Digunakan')
  })

  it('renders holder column as — (assignment module not built)', async () => {
    const wrapper = await mountAndWait()
    const cells = wrapper.findAll('td')
    // At least one holder cell renders the placeholder dash.
    expect(cells.some(c => c.text().trim() === '—')).toBe(true)
  })

  it('formats a present purchase_cost as Rupiah', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Rp 18.500.000')
  })

  it('shows the initial GET /assets call with default paging', async () => {
    await mountAndWait()
    const q = lastAssetQuery()
    expect(q.get('limit')).toBe('20')
    expect(q.get('offset')).toBe('0')
  })
})

// ---------------------------------------------------------------------------
// Masked money fields
// ---------------------------------------------------------------------------

describe('Asset Catalog page — masked money fields', () => {
  it('renders a masked indicator (not "Rp 0") when purchase_cost is absent', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).not.toContain('Rp 0')
    // The masked row (a3) has no purchase_cost/book_value keys at all.
    expect(text).toContain('Meja Kerja Ergonomis')
  })

  it('renders the masked lock affordance for the row missing purchase_cost', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.html()).toContain('Tersembunyi (izin)')
  })
})

// ---------------------------------------------------------------------------
// Status filter — 7 statuses via i18n
// ---------------------------------------------------------------------------

describe('Asset Catalog page — status filter options', () => {
  it('exposes exactly the 7 AssetStatus values plus "all", resolved via i18n', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { statusOptions: { value: string, label: string }[] }
    const values = vm.statusOptions.map(o => o.value)
    expect(values).toEqual([
      '__all__', 'available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost'
    ])
    const labels = vm.statusOptions.map(o => o.label)
    expect(labels).toContain('Tersedia')
    expect(labels).toContain('Digunakan')
    expect(labels).toContain('Dalam Mutasi')
    expect(labels).toContain('Nonaktif')
    expect(labels).toContain('Dilepas')
    expect(labels).toContain('Hilang')
  })
})

// ---------------------------------------------------------------------------
// Resilient filter-option loading — one lookup failing must not blank the
// others or leave an unhandled rejection (each lookup is guarded with its
// own .catch(), same pattern as the Detail page's loadLookups).
// ---------------------------------------------------------------------------

describe('Asset Catalog page — resilient filter option loading', () => {
  it('one failing lookup (offices) still renders rows and populates the other dropdowns, without an unhandled rejection', async () => {
    const unhandled: unknown[] = []
    const onUnhandledRejection = (reason: unknown) => unhandled.push(reason)
    process.on('unhandledRejection', onUnhandledRejection)
    try {
      setHandler((path, opts) => {
        if (path.startsWith('/offices')) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return defaultHandler(path, opts)
      })

      const wrapper = await mountAndWait()

      // Rows still render — the catalog's own list() load() is independent
      // of the filter-option lookups.
      expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')

      const vm = wrapper.vm as unknown as {
        categoryOptions: { value: string, label: string }[]
        officeOptions: { value: string, label: string }[]
        brandOptions: { value: string, label: string }[]
        modelOptions: { value: string, label: string }[]
      }
      // The failing lookup's options stay empty...
      expect(vm.officeOptions.length).toBe(0)
      // ...but the other three still populate from their own successful calls.
      expect(vm.categoryOptions.length).toBeGreaterThan(0)
      expect(vm.brandOptions.length).toBeGreaterThan(0)
      expect(vm.modelOptions.length).toBeGreaterThan(0)

      expect(unhandled).toEqual([])
    } finally {
      process.off('unhandledRejection', onUnhandledRejection)
    }
  })
})

// ---------------------------------------------------------------------------
// Filters re-fetch with matching query params
// ---------------------------------------------------------------------------

describe('Asset Catalog page — filters refetch via list()', () => {
  it('status filter sends status= and resets offset to 0', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'fStatus', 'available')
    const q = lastAssetQuery()
    expect(q.get('status')).toBe('available')
    expect(q.get('offset')).toBe('0')
  })

  it('kategori filter sends category_id=', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'fKat', 'c2')
    const q = lastAssetQuery()
    expect(q.get('category_id')).toBe('c2')
  })

  it('kantor filter sends office_id=', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'fKantor', 'o2')
    const q = lastAssetQuery()
    expect(q.get('office_id')).toBe('o2')
  })

  it('asset_class filter sends asset_class=', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'fClass', 'intangible')
    const q = lastAssetQuery()
    expect(q.get('asset_class')).toBe('intangible')
  })

  it('resetting a filter back to "all" omits the param entirely', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'fStatus', 'available')
    await setVmRef(wrapper, 'fStatus', '__all__')
    const q = lastAssetQuery()
    expect(q.get('status')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Debounced search
// ---------------------------------------------------------------------------

describe('Asset Catalog page — debounced search', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('does not refetch immediately on keystroke, then refetches ~300ms later with search=', async () => {
    const wrapper = await mountSuspended(CatalogPage)
    await vi.advanceTimersByTimeAsync(0)
    await wrapper.vm.$nextTick()
    const callsBefore = assetCalls.length

    ;(wrapper.vm as unknown as { search: string }).search = 'Toyota'
    await wrapper.vm.$nextTick()
    // Immediately after typing, no new call yet.
    expect(assetCalls.length).toBe(callsBefore)

    await vi.advanceTimersByTimeAsync(300)
    await wrapper.vm.$nextTick()

    expect(assetCalls.length).toBeGreaterThan(callsBefore)
    const q = lastAssetQuery()
    expect(q.get('search')).toBe('Toyota')
    expect(q.get('offset')).toBe('0')
  })
})

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

describe('Asset Catalog page — server-side pagination', () => {
  it('clicking page 2 sends offset=20', async () => {
    setHandler((path, opts) => {
      if (path.startsWith('/assets')) {
        assetCalls.push({ path, opts })
        return makeAssetsResponse(ASSETS, 45)
      }
      if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
      if (path.startsWith('/brands')) return makeBrandsResponse()
      if (path.startsWith('/models')) return makeModelsResponse()
      if (path.startsWith('/offices')) return makeOfficesResponse()
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    const page2 = wrapper.findAll('button').find(b => b.text().trim() === '2')
    expect(page2).toBeDefined()
    await page2!.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()

    const q = lastAssetQuery()
    expect(q.get('offset')).toBe('20')
    expect(q.get('limit')).toBe('20')
  })
})

// ---------------------------------------------------------------------------
// Load error + retry
// ---------------------------------------------------------------------------

describe('Asset Catalog page — load error', () => {
  it('shows the load-error state when GET /assets fails', async () => {
    setHandler((path) => {
      if (path.startsWith('/assets')) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
      if (path.startsWith('/brands')) return makeBrandsResponse()
      if (path.startsWith('/models')) return makeModelsResponse()
      if (path.startsWith('/offices')) return makeOfficesResponse()
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat data.')
    expect(wrapper.text()).not.toContain('Laptop Dell Latitude 5440')
  })

  it('retry button re-fetches and recovers when the second call succeeds', async () => {
    let callCount = 0
    setHandler((path) => {
      if (path.startsWith('/assets')) {
        callCount++
        if (callCount === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return makeAssetsResponse()
      }
      if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
      if (path.startsWith('/brands')) return makeBrandsResponse()
      if (path.startsWith('/models')) return makeModelsResponse()
      if (path.startsWith('/offices')) return makeOfficesResponse()
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat data.')

    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')
    expect(wrapper.text()).not.toContain('Gagal memuat data.')
  })
})

// ---------------------------------------------------------------------------
// Empty states
// ---------------------------------------------------------------------------

describe('Asset Catalog page — empty states', () => {
  it('shows the no-data empty state when there are no assets and no filter is active', async () => {
    setHandler((path) => {
      if (path.startsWith('/assets')) return makeAssetsResponse([], 0)
      if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
      if (path.startsWith('/brands')) return makeBrandsResponse()
      if (path.startsWith('/models')) return makeModelsResponse()
      if (path.startsWith('/offices')) return makeOfficesResponse()
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Belum ada aset')
  })

  it('shows the filtered-empty state when a filter is active and the server returns nothing', async () => {
    setHandler((path) => {
      if (path.startsWith('/assets')) return makeAssetsResponse([], 0)
      if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
      if (path.startsWith('/brands')) return makeBrandsResponse()
      if (path.startsWith('/models')) return makeModelsResponse()
      if (path.startsWith('/offices')) return makeOfficesResponse()
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'fStatus', 'lost')

    expect(wrapper.text()).toContain('Tidak ada aset yang cocok')
  })
})

// ---------------------------------------------------------------------------
// Grid view
// ---------------------------------------------------------------------------

describe('Asset Catalog page — grid view', () => {
  it('switches to grid view and still shows asset names, no table header', async () => {
    const wrapper = await mountAndWait()
    const gridBtn = wrapper.find('button[aria-label="Tampilan grid"]')
    expect(gridBtn.exists()).toBe(true)
    await gridBtn.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')
    expect(wrapper.find('thead').exists()).toBe(false)
  })

  it('grid cards also show the resolved brand/model name', async () => {
    const wrapper = await mountAndWait()
    const gridBtn = wrapper.find('button[aria-label="Tampilan grid"]')
    await gridBtn.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Dell Latitude 5440')
  })
})

// ---------------------------------------------------------------------------
// Stale-response race guard
// ---------------------------------------------------------------------------

describe('Asset Catalog page — stale response race guard', () => {
  it('ignores a late-resolving older /assets response after a newer request has started', async () => {
    let resolveFirst!: (v: unknown) => void
    let resolveSecond!: (v: unknown) => void
    let assetCallCount = 0

    setHandler((path) => {
      if (path.startsWith('/assets')) {
        assetCallCount++
        if (assetCallCount === 1) {
          return new Promise((resolve) => {
            resolveFirst = resolve as (v: unknown) => void
          })
        }
        return new Promise((resolve) => {
          resolveSecond = resolve as (v: unknown) => void
        })
      }
      if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
      if (path.startsWith('/brands')) return makeBrandsResponse()
      if (path.startsWith('/models')) return makeModelsResponse()
      if (path.startsWith('/offices')) return makeOfficesResponse()
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountSuspended(CatalogPage)
    await flushPromises()
    await wrapper.vm.$nextTick()

    // Mounted load() (call #1) is now in-flight. Trigger a second, newer
    // load() (call #2) before the first resolves — e.g. a fast filter change.
    ;(wrapper.vm as unknown as { fStatus: string }).fStatus = 'available'
    await wrapper.vm.$nextTick()
    await flushPromises()

    // Newer request (#2) resolves first with its own rows.
    resolveSecond(makeAssetsResponse([ASSETS[1]!], 1))
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Proyektor Epson EB-X51')
    expect(wrapper.text()).not.toContain('Laptop Dell Latitude 5440')

    // Older, stale request (#1) resolves late with different rows — must be
    // discarded, not overwrite the newer result already rendered.
    resolveFirst(makeAssetsResponse([ASSETS[0]!], 1))
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Proyektor Epson EB-X51')
    expect(wrapper.text()).not.toContain('Laptop Dell Latitude 5440')
  })
})

// ---------------------------------------------------------------------------
// Delete action fully removed
// ---------------------------------------------------------------------------

describe('Asset Catalog page — no delete action', () => {
  it('renders no button with the delete/trash affordance', async () => {
    const wrapper = await mountAndWait()
    const trashBtn = wrapper.findAll('button').find(b => b.attributes('aria-label') === 'Hapus')
    expect(trashBtn).toBeUndefined()
    expect(wrapper.html()).not.toContain('i-lucide-trash-2')
  })

  it('does not expose an onDelete handler or confirm-modal wiring', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as Record<string, unknown>
    expect(vm['onDelete']).toBeUndefined()
  })
})
