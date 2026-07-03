// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request/requestBlob are
// intercepted here. useAssets and useOffices both go through useApiClient, so
// one dispatcher covers everything the page needs (same stubbing style as
// assets-detail.spec.ts / assets-catalog.spec.ts).
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
  { id: 'o1', name: 'Kantor Pusat' },
  { id: 'o2', name: 'Kantor Cabang' }
]

const ASSET_A = { id: 'a1', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440', category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible' }
const ASSET_B = { id: 'a2', asset_tag: 'JKT01-ELK-2026-00002', name: 'Proyektor Epson EB-X51', category_id: 'c1', office_id: 'o2', status: 'available', asset_class: 'tangible' }
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
    if (path.startsWith('/offices')) {
      return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
    }
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
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
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
    expect(text).toContain('Keduanya')
    expect(text).toContain('Laptop Dell Latitude 5440')
    expect(text).toContain('Belum ada aset dipilih')
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
      if (path.startsWith('/offices')) {
        return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
      }
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
// Barcode/QR previews
// ---------------------------------------------------------------------------

describe('Asset Label/Barcode page — barcode/QR previews', () => {
  it('selecting an asset renders a barcode/QR preview <img> from the stubbed blob URL', async () => {
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

  it('caches barcode/QR images per asset+type — a later re-render does not refetch', async () => {
    const wrapper = await mountAndWait()
    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click')
    await flushPromises()

    const barcodeCalls = () => blobCalls.filter(c => c.path.includes('/a1/barcode?type='))
    expect(barcodeCalls().length).toBe(2) // default mode is 'both' → code128 + qr

    // Force additional reactivity/re-renders (toggling an unrelated field checkbox).
    const fieldBoxes = checkboxes(wrapper)
    await fieldBoxes[fieldBoxes.length - 1]!.trigger('click')
    await wrapper.vm.$nextTick()
    await flushPromises()

    expect(barcodeCalls().length).toBe(2)
  })

  it('switching mode from barcode to qr fetches the other type without refetching the first', async () => {
    const wrapper = await mountAndWait()
    const barcodeModeBtn = wrapper.findAll('button').find(b => b.text().trim() === 'Barcode')
    await barcodeModeBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    const boxes = checkboxes(wrapper)
    await boxes[1]!.trigger('click')
    await flushPromises()

    const callsOfType = (type: string) => blobCalls.filter(c => c.path === '/assets/a1/barcode?type=' + type).length
    expect(callsOfType('code128')).toBe(1)
    expect(callsOfType('qr')).toBe(0)

    const qrModeBtn = wrapper.findAll('button').find(b => b.text().trim() === 'QR')
    await qrModeBtn!.trigger('click')
    await flushPromises()

    expect(callsOfType('qr')).toBe(1)
    expect(callsOfType('code128')).toBe(1)
  })

  it('revokes all barcode object URLs on unmount', async () => {
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
  it('Cetak posts the exact body from the current controls and triggers a labels.pdf download', async () => {
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
      expect(labelCall!.opts).toEqual({
        method: 'POST',
        body: {
          asset_ids: ['a1', 'a2'],
          template: 'btn',
          layout: 'sheet',
          size: '70x40',
          columns: 3,
          mode: 'both',
          fields: { name: true, office: true }
        }
      })
      expect(captured?.download).toBe('labels.pdf')
      expect(captured?.href).toBe('blob:mock-url')
    } finally {
      spy.mockRestore()
    }
  })

  it('transmits a non-default size preset and column count for a batch print', async () => {
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
    const colBtn = wrapper.findAll('button').find(b => b.text().trim() === '4')
    await colBtn!.trigger('click')
    await wrapper.vm.$nextTick()

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
        layout: 'sheet',
        size: '100x50',
        columns: 4,
        mode: 'both',
        fields: { name: true, office: true }
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
    const barcodeModeBtn = wrapper.findAll('button').find(b => b.text().trim() === 'Barcode')
    await barcodeModeBtn!.trigger('click')
    await wrapper.vm.$nextTick()

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
