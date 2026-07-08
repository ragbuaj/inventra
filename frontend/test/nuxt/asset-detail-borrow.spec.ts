// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Task 13 — "Ajukan Peminjaman" trigger button + modal on the Asset Detail
// page. Mounts the real detail page (same stubbing approach as
// assets-detail.spec.ts) and additionally mocks useAssignment (the modal's
// borrow()/available() calls) the same way ajukan-peminjaman-modal.spec.ts
// does, since AssignmentAjukanPeminjamanModal (the auto-import name for
// components/assignment/AjukanPeminjamanModal.vue) is rendered live inside
// the page.
// ---------------------------------------------------------------------------

const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

let _handler: RequestHandler = () => {
  throw new Error('No handler set')
}
const _blobHandler: RequestHandler = () => new Blob(['x'], { type: 'image/jpeg' })

function setHandler(fn: RequestHandler) {
  _handler = fn
}

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => {
      const res = _handler(path, opts)
      return res instanceof Promise ? res : Promise.resolve(res)
    },
    requestBlob: (path: string, opts?: Record<string, unknown>) => {
      const res = _blobHandler(path, opts)
      return res instanceof Promise ? res : Promise.resolve(res)
    }
  })
}))

const borrowMock = vi.fn()
const availableMock = vi.fn()

vi.mock('~/composables/api/useAssignment', () => ({
  useAssignment: () => ({
    list: vi.fn(),
    available: availableMock,
    checkout: vi.fn(),
    checkin: vi.fn(),
    borrow: borrowMock,
    myRequests: vi.fn(),
    cancel: vi.fn()
  })
}))

// eslint-disable-next-line import/first
import DetailPage from '~/pages/assets/[tag]/index.vue'

// ---------------------------------------------------------------------------
// Fixtures — mirrors assets-detail.spec.ts's FULL_ASSET / lookup tables.
// ---------------------------------------------------------------------------

const CATEGORIES = [{ id: 'c1', name: 'Elektronik' }]
const OFFICES = [{ id: 'o1', name: 'Cabang Jakarta Selatan' }]
const BRANDS = [{ id: 'b1', name: 'Dell' }]
const MODELS = [{ id: 'm1', name: 'Latitude 5440' }]
const VENDORS = [{ id: 'v1', name: 'PT Sinar Komputindo' }]
const UNITS = [{ id: 'u1', name: 'Unit' }]
const FLOORS = [{ id: 'f1', office_id: 'o1', name: 'Lantai 3', level: 3 }]
const ROOMS = [{ id: 'r1', floor_id: 'f1', name: 'Ruang IT', code: null }]

const BASE_ASSET = {
  id: 'a1',
  asset_tag: 'JKT01-ELK-2026-00001',
  name: 'Laptop Dell Latitude 5440',
  category_id: 'c1',
  office_id: 'o1',
  brand_id: 'b1',
  model_id: 'm1',
  room_id: 'r1',
  unit_id: 'u1',
  vendor_id: 'v1',
  status: 'available',
  asset_class: 'tangible',
  serial_number: 'SN-DL5440-2026-0312',
  purchase_date: '2026-01-12',
  purchase_cost: '18500000',
  book_value: '16200000',
  accumulated_depreciation: '2300000',
  po_number: 'PO/2026/0112',
  funding_source: 'APBN',
  warranty_expiry: '2029-01-12',
  acquisition_bast_no: 'BAST/2026/0112',
  capitalized: true,
  excluded_from_valuation: false,
  depreciation_method: 'straight_line',
  useful_life_months: 48,
  notes: 'Catatan pengadaan awal.'
}

function defaultHandler(asset: Record<string, unknown>): RequestHandler {
  return (path: string) => {
    if (path.startsWith('/assets/by-tag/')) {
      const requestedTag = decodeURIComponent(path.split('/assets/by-tag/')[1] ?? '')
      if (requestedTag !== asset.asset_tag) {
        throw Object.assign(new Error('not found'), { statusCode: 404 })
      }
      return asset
    }
    if (path.match(/\/assets\/[^/]+\/attachments$/)) return { data: [], total: 0, limit: 20, offset: 0 }
    if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
    if (path.startsWith('/offices')) return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
    if (path.startsWith('/brands')) return { data: BRANDS, total: BRANDS.length, limit: 100, offset: 0 }
    if (path.startsWith('/models')) return { data: MODELS, total: MODELS.length, limit: 100, offset: 0 }
    if (path.startsWith('/vendors')) return { data: VENDORS, total: VENDORS.length, limit: 100, offset: 0 }
    if (path.startsWith('/units')) return { data: UNITS, total: UNITS.length, limit: 100, offset: 0 }
    if (path.startsWith('/floors')) return { data: FLOORS, total: FLOORS.length, limit: 100, offset: 0 }
    if (path.startsWith('/rooms')) return { data: ROOMS, total: ROOMS.length, limit: 100, offset: 0 }
    if (path.match(/\/assets\/[^/]+\/depreciation$/)) return { masked: false, computed_book_value: null, entries: [] }
    throw new Error(`Unhandled request: ${path}`)
  }
}

enableAutoUnmount(afterEach)

function grantPermissions(perms: string[]) {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Staff', email: 'staff@test.com', role_id: 'r1', role_name: 'Staff', office_id: null },
    perms
  )
}

beforeEach(() => {
  setHandler(defaultHandler(BASE_ASSET))
  borrowMock.mockReset()
  availableMock.mockReset()
  availableMock.mockResolvedValue({ data: [] })
  toastAddMock.mockReset()
  URL.createObjectURL = vi.fn(() => 'blob:mock-url')
  URL.revokeObjectURL = vi.fn()
})

async function mountTag(tag = 'JKT01-ELK-2026-00001') {
  const wrapper = await mountSuspended(DetailPage, { route: `/assets/${tag}` })
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

function findBorrowButton(wrapper: Awaited<ReturnType<typeof mountTag>>) {
  return wrapper.findAll('button').find(b => b.text().includes('Ajukan Peminjaman'))
}

// The trigger button is wrapped in a <span :title="..."> (native tooltip) —
// UTooltip itself needs a TooltipProvider (supplied by the root <UApp> in the
// real app), which isn't present in these isolated component-level mounts.
function findBorrowWrapperSpan(wrapper: Awaited<ReturnType<typeof mountTag>>): Element | undefined {
  return wrapper.findAll('span').map(w => w.element).find(el => el.hasAttribute('title') && el.getAttribute('title') === 'Hanya aset tersedia yang bisa dipinjam')
}

describe('Asset Detail page — "Ajukan Peminjaman" trigger button', () => {
  it('renders the button, enabled, when status is available and request.create is granted', async () => {
    grantPermissions(['asset.view', 'request.create'])
    setHandler(defaultHandler({ ...BASE_ASSET, status: 'available' }))
    const wrapper = await mountTag()

    const btn = findBorrowButton(wrapper)
    expect(btn).toBeDefined()
    expect(btn!.attributes('disabled')).toBeUndefined()
    // No disabled-tooltip text when the asset is available.
    expect(findBorrowWrapperSpan(wrapper)).toBeUndefined()
  })

  it('renders the button DISABLED with a tooltip when status is assigned', async () => {
    grantPermissions(['asset.view', 'request.create'])
    setHandler(defaultHandler({ ...BASE_ASSET, status: 'assigned' }))
    const wrapper = await mountTag()

    const btn = findBorrowButton(wrapper)
    expect(btn).toBeDefined()
    expect(btn!.attributes('disabled')).toBeDefined()
    expect(findBorrowWrapperSpan(wrapper)).toBeTruthy()
  })

  it('renders the button DISABLED with a tooltip when status is under_maintenance', async () => {
    grantPermissions(['asset.view', 'request.create'])
    setHandler(defaultHandler({ ...BASE_ASSET, status: 'under_maintenance' }))
    const wrapper = await mountTag()

    const btn = findBorrowButton(wrapper)
    expect(btn).toBeDefined()
    expect(btn!.attributes('disabled')).toBeDefined()
    expect(findBorrowWrapperSpan(wrapper)).toBeTruthy()
  })

  it('is ABSENT when request.create is denied', async () => {
    grantPermissions(['asset.view'])
    setHandler(defaultHandler({ ...BASE_ASSET, status: 'available' }))
    const wrapper = await mountTag()

    expect(findBorrowButton(wrapper)).toBeUndefined()
  })

  it('clicking the enabled button opens the modal, showing the locked asset name + tag', async () => {
    grantPermissions(['asset.view', 'request.create'])
    setHandler(defaultHandler({ ...BASE_ASSET, status: 'available' }))
    const wrapper = await mountTag()

    const btn = findBorrowButton(wrapper)!
    await btn.trigger('click')
    await flushPromises()
    // UModal teleports its content to document.body via a Portal, and needs
    // the enter-transition to settle before it appears (same pattern as
    // ajukan-peminjaman-modal.spec.ts).
    await new Promise(resolve => setTimeout(resolve, 400))
    await wrapper.vm.$nextTick()
    await flushPromises()

    const locked = document.body.querySelector('[data-testid="peminjaman-modal-locked-asset"]')
    expect(locked).toBeTruthy()
    expect(locked!.textContent).toContain('Laptop Dell Latitude 5440')
    expect(locked!.textContent).toContain('JKT01-ELK-2026-00001')
    // Category/office/location resolved from the page's real lookup maps.
    expect(locked!.textContent).toContain('Elektronik')
    expect(locked!.textContent).toContain('Cabang Jakarta Selatan')
    expect(locked!.textContent).toContain('Lantai 3 — Ruang IT')
  })
})
