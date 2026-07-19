// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request/requestBlob are
// intercepted here. useAssets, useOffices and useCategories all go through
// useApiClient, so one dispatcher covers everything the page needs (same
// stubbing style as assets-detail.spec.ts / assets-catalog.spec.ts).
// ---------------------------------------------------------------------------

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown
interface BlobCall { path: string, opts?: Record<string, unknown> }

let _handler: RequestHandler = () => {
  throw new Error('No handler set')
}
let _blobHandler: RequestHandler = () => new Blob(['x'], { type: 'image/png' })
let blobCalls: BlobCall[] = []
let assetListPaths: string[] = []

function setHandler(fn: RequestHandler) {
  _handler = fn
}
function setBlobHandler(fn: RequestHandler) {
  _blobHandler = fn
}

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => {
      if (path.startsWith('/assets?')) assetListPaths.push(path)
      const res = _handler(path, opts)
      return res instanceof Promise ? res : Promise.resolve(res)
    },
    requestBlob: (path: string, opts?: Record<string, unknown>) => {
      blobCalls.push({ path, opts })
      const res = _blobHandler(path, opts)
      return res instanceof Promise ? res : Promise.resolve(res)
    }
  })
}))

// eslint-disable-next-line import/first
import LabelPage from '~/pages/assets/label.vue'

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const OFFICES = [
  { id: 'o1', name: 'Kantor Pusat', code: 'KP01' },
  { id: 'o2', name: 'Kantor Cabang', code: 'KC02' }
]
const CATEGORIES = [
  { id: 'c1', name: 'Elektronik', code: 'ELK' },
  { id: 'c2', name: 'Furnitur', code: 'FUR' }
]

// GET /offices/:id (resolve-cache) must be matched before the plain list route.
function officesHandler(path: string): unknown {
  const m = /^\/offices\/([^/?]+)$/.exec(path)
  if (m) return OFFICES.find(o => o.id === m[1]) ?? null
  return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
}

function categoriesHandler(path: string): unknown {
  const m = /^\/categories\/([^/?]+)$/.exec(path)
  if (m) return CATEGORIES.find(c => c.id === m[1]) ?? null
  return { data: CATEGORIES, total: CATEGORIES.length, limit: 100, offset: 0 }
}

const ASSET_A = { id: 'a1', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440', category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible', purchase_date: '2026-01-15' }
const ASSET_B = { id: 'a2', asset_tag: 'JKT01-ELK-2026-00002', name: 'Proyektor Epson EB-X51', category_id: 'c1', office_id: 'o2', status: 'available', asset_class: 'tangible', purchase_date: '2025-11-03' }
const ASSET_C = { id: 'a3', asset_tag: 'JKT01-FUR-2025-00011', name: 'Meja Kerja Ergonomis', category_id: 'c2', office_id: 'o1', status: 'available', asset_class: 'tangible' }
const PICKER_ASSETS = [ASSET_A, ASSET_B, ASSET_C]

function defaultRequestHandler(assets: typeof PICKER_ASSETS = PICKER_ASSETS): RequestHandler {
  return (path: string) => {
    if (path.startsWith('/assets/by-tag/')) {
      const tag = decodeURIComponent(path.split('/assets/by-tag/')[1] ?? '')
      const found = assets.find(a => a.asset_tag === tag)
      if (!found) throw Object.assign(new Error('not found'), { statusCode: 404 })
      return found
    }
    if (path.startsWith('/assets?')) {
      const qs = new URLSearchParams(path.split('?')[1])
      const search = qs.get('search')
      const rows = search
        ? assets.filter(a => a.name.toLowerCase().includes(search.toLowerCase()) || a.asset_tag.toLowerCase().includes(search.toLowerCase()))
        : assets
      return { data: rows, total: rows.length, limit: 50, offset: 0 }
    }
    if (path.startsWith('/offices')) return officesHandler(path)
    if (path.startsWith('/categories')) return categoriesHandler(path)
    throw new Error(`Unhandled request: ${path}`)
  }
}

// ---------------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------------

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

beforeEach(() => {
  blobCalls = []
  assetListPaths = []
  setHandler(defaultRequestHandler())
  setBlobHandler(() => new Blob(['barcode'], { type: 'image/png' }))
  grantAdmin()
  // jsdom doesn't implement these — stub them for the barcode/PDF object-URL flow.
  URL.createObjectURL = vi.fn(() => 'blob:mock-url')
  URL.revokeObjectURL = vi.fn()
})

async function mountAndWait(route = '/assets/label') {
  const wrapper = await mountSuspended(LabelPage, { route })
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  return wrapper
}

function checkboxes(wrapper: Awaited<ReturnType<typeof mountAndWait>>) {
  return wrapper.findAll('button[role="checkbox"]')
}

// ---------------------------------------------------------------------------
// Base rendering
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — base rendering', () => {
  it('renders the select panel + layout controls and an empty preview by default', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Label & Barcode')
    expect(text).toContain('Pilih Aset')
    expect(text).toContain('Tata Letak')
    expect(text).toContain('format standar bank')
    expect(text).toContain('Laptop Dell Latitude 5440')
    expect(text).toContain('Belum ada aset dipilih')
    // The BTN template is fixed — the old mode/field controls must be gone.
    expect(text).not.toContain('Tampilkan')
    expect(text).not.toContain('Field dicetak')
  })

  it('defaults to the 60x24 BTN size preset with 3 columns available', async () => {
    const wrapper = await mountAndWait()
    expect((wrapper.vm as unknown as { size: string }).size).toBe('60x24')
    expect((wrapper.vm as unknown as { cols: number }).cols).toBe(3)
    expect(wrapper.text()).toContain('Maks. 3 kolom')
  })

  it('shows a loading skeleton while the picker fetch is pending, then the list', async () => {
    let resolveList!: (v: unknown) => void
    const pending = new Promise((resolve) => {
      resolveList = resolve
    })
    setHandler((path: string) => {
      if (path.startsWith('/assets?')) return pending
      return defaultRequestHandler()(path)
    })
    const wrapper = await mountSuspended(LabelPage, { route: '/assets/label' })
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).not.toContain('Laptop Dell Latitude 5440')

    resolveList({ data: PICKER_ASSETS, total: PICKER_ASSETS.length, limit: 50, offset: 0 })
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')
  })

  it('shows the picker error state with a retry that recovers', async () => {
    setHandler(() => {
      throw Object.assign(new Error('Server Error'), { statusCode: 500 })
    })
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat data.')

    setHandler(defaultRequestHandler())
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).not.toContain('Gagal memuat data.')
    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')
  })

  it('shows the picker empty state when a search matches nothing', async () => {
    setHandler((path: string) => {
      if (path.startsWith('/assets?')) return { data: [], total: 0, limit: 50, offset: 0 }
      return defaultRequestHandler()(path)
    })
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Tidak ada aset yang cocok.')
  })

  it('pre-selects assets from the ?tags query and renders the label preview', async () => {
    const wrapper = await mountAndWait('/assets/label?tags=JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).not.toContain('Belum ada aset dipilih')
    expect(text).toContain('Label Tunggal')
    expect(text).toContain('1 label')
    expect(wrapper.html()).toContain('JKT01-ELK-2026-00001')
  })
})

// ---------------------------------------------------------------------------
// BTN label preview content
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — BTN label preview content', () => {
  it('renders the fixed BTN fields: company, tag, office code, TP year, category, name, disclaimer', async () => {
    const wrapper = await mountAndWait('/assets/label?tags=JKT01-ELK-2026-00001')
    // The resolve-caches' resolveFn(id) calls are async (GET /offices/:id,
    // GET /categories/:id) — one more flush+tick beyond mountAndWait settles them.
    await flushPromises()
    await wrapper.vm.$nextTick()

    const text = wrapper.text()
    expect(text).toContain('PT Bank Tabungan Negara (Persero) Tbk')
    expect(text).toContain('JKT01-ELK-2026-00001')
    expect(text).toContain('TP: 2026')
    expect(text).toContain('Elektronik')
    expect(text).toContain('Laptop Dell Latitude 5440')
    expect(text).toContain('Tidak Untuk Diperjualbelikan')
  })

  it('resolves office_id to the office CODE (what the printed label shows) — not the name or raw id', async () => {
    const wrapper = await mountAndWait('/assets/label?tags=JKT01-ELK-2026-00001')
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('KP01')
    expect(wrapper.text()).not.toContain('o1')
  })

  it('renders an empty TP year when the asset has no purchase_date', async () => {
    const wrapper = await mountAndWait('/assets/label?tags=JKT01-FUR-2025-00011')
    await flushPromises()
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Meja Kerja Ergonomis')
    expect(text).toContain('TP:')
    expect(text).not.toContain('TP: 20')
  })
})

// ---------------------------------------------------------------------------
// Debounced picker search
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — debounced picker search', () => {
  // mountSuspended() itself relies on real microtask/timer scheduling to
  // resolve <Suspense>, so fake timers are only enabled *after* the page has
  // finished mounting (with real timers) — not around the mount call itself.
  afterEach(() => {
    vi.useRealTimers()
  })

  it('does not refetch immediately on keystroke, then searches ~300ms later with search=', async () => {
    const wrapper = await mountAndWait()
    const callsBefore = assetListPaths.length

    vi.useFakeTimers()
    const input = wrapper.find('input[placeholder]')
    await input.setValue('Proyektor')
    await wrapper.vm.$nextTick()

    // No new list call yet.
    expect(assetListPaths.length).toBe(callsBefore)
    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')

    await vi.advanceTimersByTimeAsync(300)
    await wrapper.vm.$nextTick()

    expect(assetListPaths.length).toBeGreaterThan(callsBefore)
    const qs = new URLSearchParams(assetListPaths.at(-1)!.split('?')[1])
    expect(qs.get('search')).toBe('Proyektor')

    expect(wrapper.text()).toContain('Proyektor Epson EB-X51')
    expect(wrapper.text()).not.toContain('Laptop Dell Latitude 5440')
  })

  it('discards a late-resolving stale picker response after a newer search completes', async () => {
    let resolveFirstSearch!: (v: unknown) => void
    let resolveSecondSearch!: (v: unknown) => void
    let searchCallCount = 0

    setHandler((path: string) => {
      if (path.startsWith('/assets?')) {
        searchCallCount++
        if (searchCallCount === 1) {
          return { data: PICKER_ASSETS, total: PICKER_ASSETS.length, limit: 50, offset: 0 }
        }
        if (searchCallCount === 2) {
          return new Promise((resolve) => {
            resolveFirstSearch = resolve as (v: unknown) => void
          })
        }
        if (searchCallCount === 3) {
          return new Promise((resolve) => {
            resolveSecondSearch = resolve as (v: unknown) => void
          })
        }
      }
      if (path.startsWith('/assets/by-tag/')) {
        const tag = decodeURIComponent(path.split('/assets/by-tag/')[1] ?? '')
        const found = PICKER_ASSETS.find(a => a.asset_tag === tag)
        if (!found) throw Object.assign(new Error('not found'), { statusCode: 404 })
        return found
      }
      if (path.startsWith('/offices')) return officesHandler(path)
      if (path.startsWith('/categories')) return categoriesHandler(path)
      throw new Error(`Unhandled request: ${path}`)
    })

    const wrapper = await mountSuspended(LabelPage, { route: '/assets/label' })
    await flushPromises()
    await wrapper.vm.$nextTick()

    // Mounted load (call #1) completed. Now trigger first search (call #2)
    // for 'Proyektor' which will be left in-flight.
    vi.useFakeTimers()
    const input = wrapper.find('input[placeholder]')
    await input.setValue('Proyektor')
    await wrapper.vm.$nextTick()
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.vm.$nextTick()

    // Now trigger search #3 (call #3) for 'Meja' while #2 is still pending.
    await input.setValue('Meja')
    await wrapper.vm.$nextTick()
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.vm.$nextTick()

    // Resolve search #3 (newer) first with Meja results.
    resolveSecondSearch({ data: [ASSET_C], total: 1, limit: 50, offset: 0 })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Meja Kerja Ergonomis')
    expect(wrapper.text()).not.toContain('Proyektor Epson EB-X51')

    // Resolve search #2 (older, stale) late with Proyektor results — must NOT
    // overwrite the newer Meja results already rendered.
    resolveFirstSearch({ data: [ASSET_B], total: 1, limit: 50, offset: 0 })
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Meja Kerja Ergonomis')
    expect(wrapper.text()).not.toContain('Proyektor Epson EB-X51')
  })
})

// ---------------------------------------------------------------------------
// QR previews
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — QR previews', () => {
  it('selecting an asset renders a QR preview <img> from the stubbed blob URL', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click') // index 0 = select-all; index 1 = first asset row
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).not.toContain('Belum ada aset dipilih')
    const imgs = wrapper.findAll('img')
    expect(imgs.length).toBeGreaterThan(0)
    expect(imgs.some(img => img.attributes('src') === 'blob:mock-url')).toBe(true)
  })

  it('fetches only type=qr — the BTN label never prints a Code128 barcode', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click')
    await flushPromises()

    const barcodeCalls = blobCalls.filter(c => c.path.includes('/a1/barcode?type='))
    expect(barcodeCalls.length).toBe(1)
    expect(barcodeCalls[0]!.path).toBe('/assets/a1/barcode?type=qr')
  })

  it('caches QR images per asset — a later re-render does not refetch', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click')
    await flushPromises()

    const qrCalls = () => blobCalls.filter(c => c.path === '/assets/a1/barcode?type=qr')
    expect(qrCalls().length).toBe(1)

    // Force additional reactivity/re-renders (switching the size preset).
    ;(wrapper.vm as unknown as { size: string }).size = '70x40'
    await wrapper.vm.$nextTick()
    await flushPromises()

    expect(qrCalls().length).toBe(1)
  })

  it('revokes all QR object URLs on unmount', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click')
    await flushPromises()

    wrapper.unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-url')
  })
})

// ---------------------------------------------------------------------------
// Cetak / Unduh PDF
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — Cetak / Unduh PDF', () => {
  it('Cetak posts the exact BTN body from the current controls and triggers a labels.pdf download', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click') // a1
    await boxes[2]!.trigger('click') // a2
    await flushPromises()

    let captured: { href: string, download: string } | undefined
    const spy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      captured = { href: this.href, download: this.download }
    })
    try {
      setBlobHandler((path: string) => {
        if (path === '/assets/labels') return new Blob(['%PDF'], { type: 'application/pdf' })
        return new Blob(['barcode'], { type: 'image/png' })
      })

      const printBtn = wrapper.findAll('button').find(b => b.text().includes('Cetak'))
      await printBtn!.trigger('click')
      await flushPromises()

      const labelCall = blobCalls.find(c => c.path === '/assets/labels')
      expect(labelCall).toBeDefined()
      // The BTN template's layout is fixed server-side — no mode/fields keys.
      expect(labelCall!.opts).toEqual({
        method: 'POST',
        body: {
          asset_ids: ['a1', 'a2'],
          template: 'btn',
          layout: 'sheet',
          size: '60x24',
          columns: 3
        }
      })
      expect(captured?.download).toBe('labels.pdf')
      expect(captured?.href).toBe('blob:mock-url')
    } finally {
      spy.mockRestore()
    }
  })

  it('falls back to a continuous roll (no columns) for a 100x50 batch print — only 1 column fits an A4 sheet', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click') // a1
    await boxes[2]!.trigger('click') // a2
    await flushPromises()

    // USelect is a custom popover, not a native <select> — drive the page's
    // own reactive state directly, the same access pattern already used
    // elsewhere in this suite (e.g. `addMany`, `toast`) and in the catalog
    // spec (`fStatus`).
    ;(wrapper.vm as unknown as { size: string }).size = '100x50'
    await wrapper.vm.$nextTick()

    // 100mm labels only fit 1 column on an A4 sheet — every offered preset
    // above that is disabled, and clicking one must not select it.
    const colBtn = wrapper.findAll('button').find(b => b.text().trim() === '4')
    expect(colBtn!.attributes('disabled')).toBeDefined()
    await colBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect((wrapper.vm as unknown as { cols: number }).cols).toBe(1)

    const printBtn = wrapper.findAll('button').find(b => b.text().includes('Cetak'))
    await printBtn!.trigger('click')
    await flushPromises()

    const labelCall = blobCalls.find(c => c.path === '/assets/labels')
    expect(labelCall).toBeDefined()
    expect(labelCall!.opts).toEqual({
      method: 'POST',
      body: {
        asset_ids: ['a1', 'a2'],
        template: 'btn',
        layout: 'roll',
        size: '100x50'
      }
    })
  })

  it('Unduh PDF uses the same download flow', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click')
    await flushPromises()

    let captured: { href: string, download: string } | undefined
    const spy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      captured = { href: this.href, download: this.download }
    })
    try {
      const pdfBtn = wrapper.findAll('button').find(b => b.text().includes('Unduh PDF'))
      await pdfBtn!.trigger('click')
      await flushPromises()

      expect(blobCalls.some(c => c.path === '/assets/labels')).toBe(true)
      expect(captured?.download).toBe('labels.pdf')
    } finally {
      spy.mockRestore()
    }
  })

  it('the Cetak/Unduh PDF buttons are disabled while no asset is selected', async () => {
    const wrapper = await mountAndWait()
    const printBtn = wrapper.findAll('button').find(b => b.text().includes('Cetak'))
    const pdfBtn = wrapper.findAll('button').find(b => b.text().includes('Unduh PDF'))
    expect(printBtn!.attributes('disabled')).toBeDefined()
    expect(pdfBtn!.attributes('disabled')).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// A4 sheet-fit clamp (regression — cols*labelW + (cols-1)*3 + 16 <= 210,
// mirrors backend/internal/asset/barcode.go's sheetFits check).
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — A4 sheet-fit clamp (regression)', () => {
  const SIZE_MM: Record<string, number> = { '60x24': 60, '50x30': 50, '70x40': 70, '100x50': 100 }
  const ALL_SIZES = Object.keys(SIZE_MM)
  const COL_OPTIONS = [2, 3, 4]

  it('keeps 3 columns for the default 60x24 size (fits) and clamps to 2 when switching to 70x40', async () => {
    const wrapper = await mountAndWait()
    expect((wrapper.vm as unknown as { cols: number }).cols).toBe(3)
    expect(wrapper.text()).toContain('Maks. 3 kolom')

    ;(wrapper.vm as unknown as { size: string }).size = '70x40'
    await wrapper.vm.$nextTick()
    expect((wrapper.vm as unknown as { cols: number }).cols).toBe(2)
    expect(wrapper.text()).toContain('Maks. 2 kolom')
  })

  it('clamps the column count when switching from a size that fits 3 columns to one that only fits 1', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { size: string }).size = '50x30'
    await wrapper.vm.$nextTick()
    const col3Btn = wrapper.findAll('button').find(b => b.text().trim() === '3')
    await col3Btn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect((wrapper.vm as unknown as { cols: number }).cols).toBe(3)

    ;(wrapper.vm as unknown as { size: string }).size = '100x50'
    await wrapper.vm.$nextTick()
    expect((wrapper.vm as unknown as { cols: number }).cols).toBe(1)
    expect(wrapper.text()).toContain('Maks. 1 kolom')
  })

  it('every UI-reachable size × columns combo prints a body that fits an A4 sheet, or falls back to roll', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click') // a1
    await boxes[2]!.trigger('click') // a2
    await flushPromises()

    for (const sizeKey of ALL_SIZES) {
      ;(wrapper.vm as unknown as { size: string }).size = sizeKey
      await wrapper.vm.$nextTick()

      for (const n of COL_OPTIONS) {
        const colBtn = wrapper.findAll('button').find(b => b.text().trim() === String(n))
        const disabled = colBtn!.attributes('disabled') !== undefined
        if (!disabled) {
          await colBtn!.trigger('click')
          await wrapper.vm.$nextTick()
        }

        blobCalls.length = 0
        const printBtn = wrapper.findAll('button').find(b => b.text().includes('Cetak'))
        await printBtn!.trigger('click')
        await flushPromises()

        const labelCall = blobCalls.find(c => c.path === '/assets/labels')
        expect(labelCall).toBeDefined()
        const body = labelCall!.opts!.body as { layout: string, size: string, columns?: number }
        const w = SIZE_MM[sizeKey]!

        if (body.layout === 'sheet') {
          expect(body.columns).toBeDefined()
          const cols = body.columns!
          // The exact inequality the backend enforces (barcode.go sheetFits) —
          // pins the regression where the UI could send a column count that
          // overflows an A4 page and got a 400 ErrSheetOverflow.
          expect(cols * w + (cols - 1) * 3 + 16).toBeLessThanOrEqual(210)
        } else {
          expect(body.layout).toBe('roll')
          expect(body.columns).toBeUndefined()
        }
      }
    }
  })
})

// ---------------------------------------------------------------------------
// Selection cap
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — 500-asset selection cap', () => {
  it('blocks selecting more than 500 assets at once and warns', async () => {
    // 501 rendered picker checkboxes / preview label cards is real DOM weight
    // the UI would only ever hit at the documented cap boundary — driving it
    // through 501 real clicks is unnecessarily heavy for a unit test, so the
    // selection is constructed directly via the page's own `addMany` (exposed
    // through setupState, same access pattern already used elsewhere in this
    // suite for internal methods like `load`).
    const wrapper = await mountAndWait()

    const many = Array.from({ length: 501 }, (_, i) => ({
      id: `bulk-${i}`,
      asset_tag: `BULK-${i}`,
      name: `Bulk Asset ${i}`,
      category_id: 'c1',
      office_id: 'o1',
      status: 'available',
      asset_class: 'tangible'
    }))
    ;(wrapper.vm as unknown as { addMany: (assets: typeof many) => void }).addMany(many)
    await wrapper.vm.$nextTick()
    await flushPromises()

    expect(wrapper.text()).toContain('500 dipilih')

    // Read the toast state off the mounted component's own `toast` binding
    // (exposed via setupState) rather than calling useToast() bare in the
    // test body — the latter calls inject() outside any component instance,
    // which only warns under a quick/serial run but has caused a hang under
    // full-suite parallel load (the very flake this rewrite must eliminate).
    const toasts = (wrapper.vm as unknown as { toast: { toasts: { value: { title?: string }[] } } }).toast.toasts.value
    expect(toasts.some(tst => String(tst.title).includes('Maksimum 500'))).toBe(true)
  })
})
