// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request/requestBlob are
// intercepted here. useAssets, useAssetAttachments, useCategories, useOffices,
// useFloors and useReference all go through useApiClient, so one dispatcher
// covers everything the page needs (same stubbing style as
// assets-catalog.spec.ts).
// ---------------------------------------------------------------------------

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

let _handler: RequestHandler = () => {
  throw new Error('No handler set')
}
let _blobHandler: RequestHandler = () => new Blob(['x'], { type: 'image/jpeg' })

function setHandler(fn: RequestHandler) {
  _handler = fn
}
function setBlobHandler(fn: RequestHandler) {
  _blobHandler = fn
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

// eslint-disable-next-line import/first
import DetailPage from '~/pages/assets/[tag]/index.vue'

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const CATEGORIES = [{ id: 'c1', name: 'Elektronik' }]
const OFFICES = [{ id: 'o1', name: 'Cabang Jakarta Selatan' }]
const BRANDS = [{ id: 'b1', name: 'Dell' }]
const MODELS = [{ id: 'm1', name: 'Latitude 5440' }]
const VENDORS = [{ id: 'v1', name: 'PT Sinar Komputindo' }]
const UNITS = [{ id: 'u1', name: 'Unit' }]
const FLOORS = [{ id: 'f1', office_id: 'o1', name: 'Lantai 3', level: 3 }]
const ROOMS = [{ id: 'r1', floor_id: 'f1', name: 'Ruang IT', code: null }]

const FULL_ASSET = {
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

// Asset with the masked sensitive money fields absent (field-permission strips
// them) and no room assigned.
const MASKED_ASSET = {
  ...FULL_ASSET,
  room_id: null,
  purchase_cost: undefined,
  book_value: undefined,
  accumulated_depreciation: undefined
}
delete (MASKED_ASSET as Record<string, unknown>).purchase_cost
delete (MASKED_ASSET as Record<string, unknown>).book_value
delete (MASKED_ASSET as Record<string, unknown>).accumulated_depreciation

const PHOTO_ATTACHMENTS = [
  { id: 'att1', asset_id: 'a1', kind: 'photo', original_filename: 'depan.jpg', size_bytes: 1024, mime_type: 'image/jpeg', has_thumbnail: true, created_at: '2026-01-01T00:00:00Z' },
  { id: 'att2', asset_id: 'a1', kind: 'document', original_filename: 'bast.pdf', size_bytes: 2048, mime_type: 'application/pdf', has_thumbnail: false, created_at: '2026-01-01T00:00:00Z' }
]

function defaultHandler(asset: Record<string, unknown>): RequestHandler {
  return (path: string) => {
    if (path.startsWith('/assets/by-tag/')) {
      const requestedTag = decodeURIComponent(path.split('/assets/by-tag/')[1] ?? '')
      if (requestedTag !== asset.asset_tag) {
        throw Object.assign(new Error('not found'), { statusCode: 404 })
      }
      return asset
    }
    if (path.match(/\/assets\/[^/]+\/attachments$/)) return { data: PHOTO_ATTACHMENTS, total: PHOTO_ATTACHMENTS.length, limit: 20, offset: 0 }
    if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
    if (path.startsWith('/offices')) return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
    if (path.startsWith('/brands')) return { data: BRANDS, total: BRANDS.length, limit: 100, offset: 0 }
    if (path.startsWith('/models')) return { data: MODELS, total: MODELS.length, limit: 100, offset: 0 }
    if (path.startsWith('/vendors')) return { data: VENDORS, total: VENDORS.length, limit: 100, offset: 0 }
    if (path.startsWith('/units')) return { data: UNITS, total: UNITS.length, limit: 100, offset: 0 }
    if (path.startsWith('/floors')) return { data: FLOORS, total: FLOORS.length, limit: 100, offset: 0 }
    if (path.startsWith('/rooms')) return { data: ROOMS, total: ROOMS.length, limit: 100, offset: 0 }
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
  setHandler(defaultHandler(FULL_ASSET))
  setBlobHandler(() => new Blob(['thumb'], { type: 'image/jpeg' }))
  grantAdmin()
  // jsdom doesn't implement these — stub them for the gallery's object-URL flow.
  URL.createObjectURL = vi.fn(() => 'blob:mock-url')
  URL.revokeObjectURL = vi.fn()
})

async function mountTag(tag: string) {
  const wrapper = await mountSuspended(DetailPage, { route: `/assets/${tag}` })
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

// ---------------------------------------------------------------------------
// Header + Info tab
// ---------------------------------------------------------------------------

describe('Asset Detail page — header and Info tab', () => {
  it('renders the asset name, tag and status badge', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).toContain('Laptop Dell Latitude 5440')
    expect(text).toContain('JKT01-ELK-2026-00001')
    expect(text).toContain('Tersedia')
  })

  it('renders Info fields with FK ids resolved to names', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).toContain('Identitas')
    expect(text).toContain('Nomor Seri')
    expect(text).toContain('SN-DL5440-2026-0312')
    expect(text).toContain('Elektronik') // category
    expect(text).toContain('Dell') // brand
    expect(text).toContain('Latitude 5440') // model
    expect(text).toContain('Unit') // unit
    expect(text).toContain('Cabang Jakarta Selatan') // office
    expect(text).toContain('PT Sinar Komputindo') // vendor
    expect(text).toContain('Lantai 3 — Ruang IT') // resolved room via floors+rooms
    expect(text).not.toContain('c1')
    expect(text).not.toContain('o1')
    expect(text).not.toContain('r1')
  })

  it('renders the newly-remapped real fields (PO, funding, warranty, notes, class, capitalized, BAST, exclusion)', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).toContain('PO/2026/0112')
    expect(text).toContain('APBN')
    expect(text).toContain('BAST/2026/0112')
    expect(text).toContain('Catatan pengadaan awal.')
    expect(text).toContain('Berwujud') // asset_class tangible
    expect(text).toContain('Ya') // capitalized true
    expect(text).toContain('Tidak') // excluded_from_valuation false
    expect(text).toContain('Garis Lurus') // depreciation method
    expect(text).toContain('48 bulan') // useful_life_months
  })

  it('shows "—" for a null room (no location assigned)', async () => {
    setHandler(defaultHandler(MASKED_ASSET))
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).toContain('—')
  })

  it('still resolves the room when one floor\'s rooms fetch rejects but another floor has the room', async () => {
    // Two floors on the asset's office: f1's rooms fetch fails outright, f2's
    // succeeds and contains the asset's room. Resolution must not be
    // all-or-nothing — the successful floor's room should still be found.
    const floorsTwo = [
      { id: 'f1', office_id: 'o1', name: 'Lantai 1', level: 1 },
      { id: 'f2', office_id: 'o1', name: 'Lantai 2', level: 2 }
    ]
    const roomsF2 = [{ id: 'r2', floor_id: 'f2', name: 'Ruang Server', code: null }]
    const assetOnF2 = { ...FULL_ASSET, room_id: 'r2' }
    setHandler((path: string) => {
      if (path.startsWith('/rooms?floor_id=f1')) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      if (path.startsWith('/rooms?floor_id=f2')) return { data: roomsF2, total: roomsF2.length, limit: 100, offset: 0 }
      if (path.startsWith('/floors')) return { data: floorsTwo, total: floorsTwo.length, limit: 100, offset: 0 }
      return defaultHandler(assetOnF2)(path)
    })
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    expect(wrapper.text()).toContain('Lantai 2 — Ruang Server')
  })
})

// ---------------------------------------------------------------------------
// Masked money fields
// ---------------------------------------------------------------------------

describe('Asset Detail page — masked vs visible money rows', () => {
  it('shows a formatted Rupiah value when purchase_cost/accumulated_depreciation/book_value are present', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).toContain('Rp 18.500.000')
    expect(text).toContain('Rp 2.300.000')
    expect(text).toContain('Rp 16.200.000')
  })

  it('shows the masked lock indicator (not "Rp 0") when the money keys are absent', async () => {
    setHandler(defaultHandler(MASKED_ASSET))
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).not.toContain('Rp 0')
    expect(wrapper.html()).toContain('i-lucide:lock')
  })

  it('renders an explicit negative purchase_cost as a formatted value, not masked', async () => {
    setHandler(defaultHandler({ ...FULL_ASSET, purchase_cost: '-500000' }))
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).not.toContain('Rp 0')
    expect(text).toMatch(/Rp\s*-?500\.000/)
  })
})

// ---------------------------------------------------------------------------
// History tabs — empty-state deviation (approved)
// ---------------------------------------------------------------------------

describe('Asset Detail page — history tabs show empty-state, not sample data', () => {
  it('Penugasan tab shows the module-not-available empty state', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const assignTab = wrapper.findAll('button').find(b => b.text().trim() === 'Riwayat Penugasan')
    await assignTab!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Belum ada data — modul belum tersedia')
    expect(wrapper.text()).not.toContain('Rina Putri')
  })

  it('Maintenance tab shows the module-not-available empty state', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const maintTab = wrapper.findAll('button').find(b => b.text().trim() === 'Riwayat Maintenance')
    await maintTab!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Belum ada data — modul belum tersedia')
    expect(wrapper.text()).not.toContain('Preventive')
  })

  it('Depreciation tab shows the module-not-available empty state', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const deprTab = wrapper.findAll('button').find(b => b.text().trim() === 'Jadwal Depresiasi')
    await deprTab!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Belum ada data — modul belum tersedia')
    expect(wrapper.text()).not.toContain('Berjalan')
  })
})

// ---------------------------------------------------------------------------
// Gallery
// ---------------------------------------------------------------------------

describe('Asset Detail page — photo gallery', () => {
  it('renders an <img> per photo-kind attachment with a thumbnail', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const imgs = wrapper.findAll('img')
    // Only the photo-kind attachment (att1) has has_thumbnail; the PDF (att2) is excluded.
    expect(imgs.length).toBeGreaterThan(0)
    expect(imgs.every(img => img.attributes('src') === 'blob:mock-url')).toBe(true)
  })

  it('shows the empty-state text when there are no photo attachments', async () => {
    setHandler((path: string) => {
      if (path.match(/\/assets\/[^/]+\/attachments$/)) return { data: [], total: 0, limit: 20, offset: 0 }
      return defaultHandler(FULL_ASSET)(path)
    })
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    expect(wrapper.text()).toContain('Belum ada foto aset.')
    expect(wrapper.findAll('img').length).toBe(0)
  })

  it('discards a late-resolving stale gallery load and revokes its object URLs, keeping the newer photos', async () => {
    // Two overlapping loads of the same mounted page (e.g. a fast route-param
    // change re-triggering `load()`): the older load's attachments *list*
    // resolves promptly (so it gets past the first staleness check and into
    // the per-photo thumbnail fetch, creating an object URL), but its
    // thumbnail blob resolves late — AFTER the newer load has already
    // populated `photos.value`. The stale response must not overwrite the
    // fresher photos, and the object URL it created must be revoked.
    let thumbnailCall = 0
    let resolveOldThumbnail!: (v: Blob) => void
    const oldThumbnailPromise = new Promise<Blob>((resolve) => {
      resolveOldThumbnail = resolve
    })

    setHandler(defaultHandler(FULL_ASSET))
    setBlobHandler(() => {
      thumbnailCall++
      if (thumbnailCall === 1) return oldThumbnailPromise
      return new Blob(['new-thumb'], { type: 'image/jpeg' })
    })

    // Only one row (att1) is a photo with a thumbnail, so each load's gallery
    // creates exactly one object URL: the first call chronologically belongs
    // to the newer (second) load — since the old load's thumbnail fetch is
    // still pending — the second call belongs to the stale load once its
    // thumbnail resolves late.
    const createObjectURLMock = vi.fn()
      .mockReturnValueOnce('blob:new-1')
      .mockReturnValue('blob:old-1')
    URL.createObjectURL = createObjectURLMock
    const revokeObjectURLMock = vi.fn()
    URL.revokeObjectURL = revokeObjectURLMock

    const wrapper = await mountSuspended(DetailPage, { route: '/assets/JKT01-ELK-2026-00001' })
    // Let the initial (stale) load's attachments list resolve, then hang on
    // its thumbnail-blob fetch.
    await flushPromises()

    // Trigger a second, newer load whose thumbnail fetch resolves immediately
    // — it should win and populate photos.value first.
    const vm = wrapper.vm as unknown as { load: () => Promise<void> }
    await vm.load()
    await flushPromises()
    await wrapper.vm.$nextTick()
    await flushPromises()

    expect(wrapper.findAll('img').every(img => img.attributes('src') === 'blob:new-1')).toBe(true)

    // Now let the stale first load's thumbnail resolve late.
    resolveOldThumbnail(new Blob(['old-thumb'], { type: 'image/jpeg' }))
    await flushPromises()
    await wrapper.vm.$nextTick()
    await flushPromises()

    // Newer photos must survive — the stale response must not overwrite them.
    expect(wrapper.findAll('img').every(img => img.attributes('src') === 'blob:new-1')).toBe(true)
    // The stale load's own object URL must have been revoked, not leaked.
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:old-1')
  })
})

// ---------------------------------------------------------------------------
// Not found / load error
// ---------------------------------------------------------------------------

describe('Asset Detail page — not-found and load-error', () => {
  it('shows the not-found card for an unknown tag (404 from getByTag)', async () => {
    const wrapper = await mountTag('NOPE-0000')
    expect(wrapper.text()).toContain('Aset tidak ditemukan')
  })

  it('shows the load-error state with a retry button on a non-404 failure, then recovers', async () => {
    let callCount = 0
    setHandler((path: string) => {
      if (path.startsWith('/assets/by-tag/')) {
        callCount++
        if (callCount === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return FULL_ASSET
      }
      return defaultHandler(FULL_ASSET)(path)
    })
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    expect(wrapper.text()).toContain('Gagal memuat data.')

    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()
    await flushPromises()

    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')
    expect(wrapper.text()).not.toContain('Gagal memuat data.')
  })
})

// ---------------------------------------------------------------------------
// Delete action fully removed
// ---------------------------------------------------------------------------

describe('Asset Detail page — no delete action', () => {
  it('renders no delete/trash affordance anywhere on the page', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    expect(wrapper.html()).not.toContain('i-lucide-trash-2')
    expect(wrapper.text()).not.toContain('Hapus Aset')
  })

  it('does not expose an onDelete handler', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const vm = wrapper.vm as unknown as Record<string, unknown>
    expect(vm['onDelete']).toBeUndefined()
  })
})
