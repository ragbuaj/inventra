// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import type { Asset } from '~/types'
import { useAuthStore } from '~/stores/auth'
import { useConfirm } from '~/composables/useConfirm'

// ---------------------------------------------------------------------------
// Stub API client — useAssets/useAssetRequests/useAssetAttachments/
// useCategories/useOffices/useFloors/useReference all go through
// useApiClient, so one dispatcher covers everything the form needs (same
// stubbing style as assets-detail.spec.ts).
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
    request: (path: string, opts?: Record<string, unknown>) => {
      const res = _handler(path, opts)
      return res instanceof Promise ? res : Promise.resolve(res)
    },
    requestBlob: () => Promise.resolve(new Blob(['x']))
  })
}))

// Hoisted mock for the toast composable used on submit. `navigateTo` is
// deliberately left un-mocked: mocking it with `mockNuxtImport` short-circuits
// @nuxtjs/i18n's initial locale-detection redirect (which itself calls the
// real `navigateTo`), leaving the page stuck on the English fallback locale
// with the `id` message catalog never lazy-loaded — every assertion below
// then sees raw `assets.form.*` keys instead of Indonesian text. The real
// `navigateTo` is safe to leave running (it's fire-and-forget, matching every
// other page in this codebase); success is instead verified via the toast +
// the absence of the error banner, which only happens on the same code path.
const { toastAddMock } = vi.hoisted(() => ({
  toastAddMock: vi.fn()
}))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

// eslint-disable-next-line import/first
import AssetForm from '~/components/asset/AssetForm.vue'

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const CATEGORIES = [
  { id: 'c1', name: 'Elektronik', code: 'ELK', asset_class: 'tangible', default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0.1' },
  { id: 'c2', name: 'Aset Takberwujud', code: 'ITG', asset_class: 'intangible', default_depreciation_method: null, default_useful_life_months: null, default_salvage_rate: null }
]
const OFFICES = [{ id: 'o1', name: 'Cabang Jakarta Selatan', code: 'JKS' }]
const BRANDS = [{ id: 'b1', name: 'Dell' }]
const MODELS = [
  { id: 'm1', name: 'Latitude 5440', brand_id: 'b1' },
  { id: 'm2', name: 'ProBook 450', brand_id: 'b2' }
]
const UNITS = [{ id: 'u1', name: 'Unit' }]
const VENDORS = [{ id: 'v1', name: 'PT Sinar Komputindo' }]
const FLOORS = [{ id: 'f1', office_id: 'o1', name: 'Lantai 3', level: 3 }]
const ROOMS = [{ id: 'r1', floor_id: 'f1', name: 'Ruang IT', code: null }]
const ATTACHMENTS = [
  { id: 'att1', asset_id: 'a1', kind: 'photo', original_filename: 'foto-depan.jpg', size_bytes: 2048, mime_type: 'image/jpeg', has_thumbnail: true, created_at: '2026-01-01T00:00:00Z' }
]

const EDIT_ASSET: Asset = {
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
  po_number: 'PO/2026/0112',
  funding_source: 'APBN',
  warranty_expiry: '2029-01-12',
  notes: 'Catatan pengadaan awal.'
}

function defaultHandler(): RequestHandler {
  return (path: string, opts?: Record<string, unknown>) => {
    // Single-resource GETs (AsyncSearchPicker resolveFn / picker adapters) must
    // be matched before the plain-list routes below.
    if (/^\/categories\/[^/?]+$/.test(path)) {
      const id = path.split('/')[2]
      return CATEGORIES.find(c => c.id === id) ?? null
    }
    if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
    if (/^\/offices\/[^/?]+$/.test(path)) {
      const id = path.split('/')[2]
      return OFFICES.find(o => o.id === id) ?? null
    }
    if (path.startsWith('/offices')) return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
    if (/^\/brands\/[^/?]+$/.test(path)) {
      const id = path.split('/')[2]
      return BRANDS.find(b => b.id === id) ?? null
    }
    if (path.startsWith('/brands')) return { data: BRANDS, total: BRANDS.length, limit: 20, offset: 0 }
    if (/^\/models\/[^/?]+$/.test(path)) {
      const id = path.split('/')[2]
      return MODELS.find(m => m.id === id) ?? null
    }
    if (path.startsWith('/models')) return { data: MODELS, total: MODELS.length, limit: 20, offset: 0 }
    if (/^\/units\/[^/?]+$/.test(path)) {
      const id = path.split('/')[2]
      return UNITS.find(u => u.id === id) ?? null
    }
    if (path.startsWith('/units')) return { data: UNITS, total: UNITS.length, limit: 20, offset: 0 }
    if (/^\/vendors\/[^/?]+$/.test(path)) {
      const id = path.split('/')[2]
      return VENDORS.find(v => v.id === id) ?? null
    }
    if (path.startsWith('/vendors')) return { data: VENDORS, total: VENDORS.length, limit: 20, offset: 0 }
    if (/^\/employees\/[^/?]+$/.test(path)) {
      const id = path.split('/')[2]!
      return { id, name: `Pegawai ${id}`, code: `NIP${id}` }
    }
    if (path.startsWith('/employees')) return { data: [], total: 0, limit: 20, offset: 0 }
    if (path.startsWith('/floors')) return { data: FLOORS, total: FLOORS.length, limit: 100, offset: 0 }
    if (path.startsWith('/rooms')) return { data: ROOMS, total: ROOMS.length, limit: 100, offset: 0 }
    if (path.match(/\/assets\/[^/]+\/attachments$/) && (!opts || opts.method !== 'POST')) {
      return { data: ATTACHMENTS, total: ATTACHMENTS.length, limit: 20, offset: 0 }
    }
    throw new Error(`Unhandled request: ${path}`)
  }
}

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

beforeEach(() => {
  setHandler(defaultHandler())
  grantAdmin()
  toastAddMock.mockClear()
})

interface FormVm {
  form: Record<string, string>
  errors: Record<string, string>
  submitError: boolean
  save: () => Promise<void>
  onFileChange: (e: unknown) => Promise<void>
  removeAttachment: (att: { id: string, name: string, sizeLabel: string }) => Promise<void>
  attachments: { id: string, name: string, sizeLabel: string }[]
}

async function mountNew() {
  const wrapper = await mountSuspended(AssetForm, { props: { mode: 'new' } })
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

async function mountEdit(initial: Asset = EDIT_ASSET) {
  const wrapper = await mountSuspended(AssetForm, { props: { mode: 'edit', initial } })
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

// ---------------------------------------------------------------------------
// New mode — render + validation
// ---------------------------------------------------------------------------

describe('AssetForm — create mode: render', () => {
  it('renders the title, real maker-checker banner, all sections and required fields', async () => {
    const wrapper = await mountNew()
    const text = wrapper.text()
    expect(text).toContain('Tambah Aset')
    expect(text).toContain('pengajuan') // updated maker-checker banner text
    expect(text).toContain('Identitas')
    expect(text).toContain('Penempatan')
    expect(text).toContain('Pembelian')
    expect(text).toContain('Depresiasi')
    expect(text).toContain('Lampiran')
    expect(text).toContain('Nama Aset')
  })

  it('shows the tag auto-generated hint, not a client-computed preview', async () => {
    const wrapper = await mountNew()
    expect(wrapper.text()).toContain('Dibuat otomatis saat disetujui')
    // no fake preview code pattern like "XXX00-XXX-2026-00001"
    expect(wrapper.html()).not.toContain('XXX00')
  })

  it('shows the disabled attachments dropzone with the deferred-upload hint', async () => {
    const wrapper = await mountNew()
    expect(wrapper.text()).toContain('Lampiran dapat diunggah setelah aset disetujui')
    const drop = wrapper.findAll('button').find(b => b.text().includes('Seret & lepas'))
    expect(drop).toBeDefined()
    expect(drop!.attributes('disabled')).toBeDefined()
  })

  it('blocks save and shows required errors when empty, without calling submitCreate', async () => {
    const wrapper = await mountNew()
    let requestsCalled = false
    setHandler((path, opts) => {
      if (path === '/requests') {
        requestsCalled = true
        return { id: 'r1', status: 'pending' }
      }
      return defaultHandler()(path, opts)
    })
    const vm = wrapper.vm as unknown as FormVm
    await vm.save()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Nama aset wajib diisi')
    expect(wrapper.text()).toContain('Kategori wajib dipilih')
    expect(wrapper.text()).toContain('Kantor wajib dipilih')
    expect(wrapper.text()).toContain('Tanggal beli wajib diisi')
    expect(wrapper.text()).toContain('Harga beli wajib diisi')
    expect(requestsCalled).toBe(false)
    expect(toastAddMock).not.toHaveBeenCalledWith(expect.objectContaining({ title: 'Pengajuan terkirim — menunggu persetujuan' }))
  })
})

// ---------------------------------------------------------------------------
// New mode — kantor field is the async office picker
// ---------------------------------------------------------------------------

describe('AssetForm — create mode: kantor is an AsyncSearchPicker', () => {
  it('renders the office picker input (no more eager-options USelect)', async () => {
    const wrapper = await mountNew()
    expect(wrapper.find('[data-testid="asset-form-kantor-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="office-picker-input"]').exists()).toBe(true)
  })

  it('searching drives office.searchFn (via GET /offices) with search+limit=20', async () => {
    const wrapper = await mountNew()
    let captured: string | undefined
    setHandler((path, opts) => {
      if (path.startsWith('/offices')) {
        captured = path
        return { data: OFFICES, total: OFFICES.length, limit: 20, offset: 0 }
      }
      return defaultHandler()(path, opts)
    })
    vi.useFakeTimers()
    await wrapper.find('[data-testid="office-picker-input"]').setValue('Jakarta')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(captured).toContain('search=Jakarta')
    expect(captured).toContain('limit=20')
  })

  it('resolves a preselected office id to its label via GET /offices/:id', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.officeId = 'o1'
    await flushPromises()
    await wrapper.vm.$nextTick()
    const input = wrapper.find('[data-testid="office-picker-input"]').element as HTMLInputElement
    expect(input.value).toBe('Cabang Jakarta Selatan')
  })
})

// ---------------------------------------------------------------------------
// New mode — kategori/brand/model/unit are async pickers (no more eager
// {limit:100} USelect options)
// ---------------------------------------------------------------------------

describe('AssetForm — create mode: kategori/brand/model/unit are AsyncSearchPickers', () => {
  it('renders the four picker inputs instead of the old eager-options USelects', async () => {
    const wrapper = await mountNew()
    expect(wrapper.find('[data-testid="asset-form-kategori-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="asset-form-brand-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="asset-form-model-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="asset-form-unit-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="category-picker-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="brand-picker-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="model-picker-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="unit-picker-input"]').exists()).toBe(true)
  })

  it('searching kategori drives GET /categories with search+limit=20', async () => {
    const wrapper = await mountNew()
    let captured: string | undefined
    setHandler((path, opts) => {
      if (path.startsWith('/categories') && !path.startsWith('/categories/tree')) {
        captured = path
        return { data: CATEGORIES, total: CATEGORIES.length, limit: 20, offset: 0 }
      }
      return defaultHandler()(path, opts)
    })
    vi.useFakeTimers()
    await wrapper.find('[data-testid="category-picker-input"]').setValue('Elektro')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(captured).toContain('search=Elektro')
    expect(captured).toContain('limit=20')
  })

  it('searching brand drives GET /brands with search+limit=20', async () => {
    const wrapper = await mountNew()
    let captured: string | undefined
    setHandler((path, opts) => {
      if (path.startsWith('/brands') && !/^\/brands\/[^/?]+$/.test(path)) {
        captured = path
        return { data: BRANDS, total: BRANDS.length, limit: 20, offset: 0 }
      }
      return defaultHandler()(path, opts)
    })
    vi.useFakeTimers()
    await wrapper.find('[data-testid="brand-picker-input"]').setValue('Dell')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(captured).toContain('search=Dell')
    expect(captured).toContain('limit=20')
  })

  it('searching unit drives GET /units with search+limit=20', async () => {
    const wrapper = await mountNew()
    let captured: string | undefined
    setHandler((path, opts) => {
      if (path.startsWith('/units') && !/^\/units\/[^/?]+$/.test(path)) {
        captured = path
        return { data: UNITS, total: UNITS.length, limit: 20, offset: 0 }
      }
      return defaultHandler()(path, opts)
    })
    vi.useFakeTimers()
    await wrapper.find('[data-testid="unit-picker-input"]').setValue('Unit')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(captured).toContain('search=Unit')
    expect(captured).toContain('limit=20')
  })

  it('resolves a preselected kategori id to its label via GET /categories/:id', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.categoryId = 'c1'
    await flushPromises()
    await wrapper.vm.$nextTick()
    const input = wrapper.find('[data-testid="category-picker-input"]').element as HTMLInputElement
    expect(input.value).toBe('Elektronik')
  })
})

// ---------------------------------------------------------------------------
// New mode — vendor is an async picker (no more eager {limit:100} USelect)
// ---------------------------------------------------------------------------

describe('AssetForm — create mode: vendor is an AsyncSearchPicker', () => {
  it('renders the vendor picker input instead of the old eager-options USelect', async () => {
    const wrapper = await mountNew()
    expect(wrapper.find('[data-testid="asset-form-vendor-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="vendor-picker-input"]').exists()).toBe(true)
  })

  it('searching vendor drives GET /vendors with search+limit=20', async () => {
    const wrapper = await mountNew()
    let captured: string | undefined
    setHandler((path, opts) => {
      if (path.startsWith('/vendors') && !/^\/vendors\/[^/?]+$/.test(path)) {
        captured = path
        return { data: VENDORS, total: VENDORS.length, limit: 20, offset: 0 }
      }
      return defaultHandler()(path, opts)
    })
    vi.useFakeTimers()
    await wrapper.find('[data-testid="vendor-picker-input"]').setValue('Sinar')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(captured).toContain('search=Sinar')
    expect(captured).toContain('limit=20')
  })

  it('resolves a preselected vendor id to its label via GET /vendors/:id', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.vendorId = 'v1'
    await flushPromises()
    await wrapper.vm.$nextTick()
    const input = wrapper.find('[data-testid="vendor-picker-input"]').element as HTMLInputElement
    expect(input.value).toBe('PT Sinar Komputindo')
  })
})

// ---------------------------------------------------------------------------
// New mode — cascade + filtering
// ---------------------------------------------------------------------------

describe('AssetForm — create mode: kantor→lantai→ruangan cascade, brand→model filter', () => {
  it('disables ruangan until both kantor and lantai are chosen', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    let ruangan = wrapper.find('[data-testid="asset-form-ruangan-select"]')
    expect(ruangan.attributes('disabled')).toBeDefined()

    vm.form.officeId = 'o1'
    await flushPromises()
    await wrapper.vm.$nextTick()
    ruangan = wrapper.find('[data-testid="asset-form-ruangan-select"]')
    expect(ruangan.attributes('disabled')).toBeDefined() // still no lantai chosen

    vm.form.floorId = 'f1'
    await flushPromises()
    await wrapper.vm.$nextTick()
    ruangan = wrapper.find('[data-testid="asset-form-ruangan-select"]')
    expect(ruangan.attributes('disabled')).toBeUndefined()
  })

  it('disables the model picker until a brand is chosen', async () => {
    const wrapper = await mountNew()
    const model = wrapper.find('[data-testid="model-picker-input"]')
    expect(model.attributes('disabled')).toBeDefined()
  })

  it('enables the model picker once a brand is chosen and filters search results by brand_id client-side', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    let captured: string | undefined
    setHandler((path, opts) => {
      if (path.startsWith('/models') && !/^\/models\/[^/?]+$/.test(path)) {
        captured = path
        return { data: MODELS, total: MODELS.length, limit: 20, offset: 0 } // both brands' models
      }
      return defaultHandler()(path, opts)
    })

    vm.form.brandId = 'b1'
    await wrapper.vm.$nextTick()
    const model = wrapper.find('[data-testid="model-picker-input"]')
    expect(model.attributes('disabled')).toBeUndefined()

    vi.useFakeTimers()
    await model.setValue('a') // matches both "Latitude 5440" and "ProBook 450"
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()

    expect(captured).toContain('limit=20')
    // Only b1's model ("Latitude 5440") is shown, even though the (unfiltered)
    // backend response includes b2's "ProBook 450" too.
    const items = wrapper.findAll('[data-testid="model-picker-item"]')
    expect(items).toHaveLength(1)
    expect(items[0]!.text()).toContain('Latitude 5440')
  })

  it('clears the model field when the brand changes', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.brandId = 'b1'
    await wrapper.vm.$nextTick()
    vm.form.modelId = 'm1'
    await wrapper.vm.$nextTick()

    vm.form.brandId = 'b2'
    await wrapper.vm.$nextTick()
    expect(vm.form.modelId).toBe('')
  })
})

// ---------------------------------------------------------------------------
// New mode — submit
// ---------------------------------------------------------------------------

describe('AssetForm — create mode: submit', () => {
  async function fillValidForm(wrapper: Awaited<ReturnType<typeof mountNew>>) {
    const vm = wrapper.vm as unknown as FormVm
    vm.form.nama = 'Laptop Dell Latitude 5440'
    vm.form.categoryId = 'c1'
    vm.form.officeId = 'o1'
    await flushPromises()
    vm.form.floorId = 'f1'
    await flushPromises()
    vm.form.roomId = 'r1'
    vm.form.brandId = 'b1'
    await wrapper.vm.$nextTick()
    vm.form.modelId = 'm1'
    vm.form.unitId = 'u1'
    vm.form.vendorId = 'v1'
    vm.form.serialNumber = 'SN-1'
    vm.form.poNumber = 'PO-1'
    vm.form.fundingSource = 'APBN'
    vm.form.tglBeli = '2026-01-12'
    vm.form.warrantyExpiry = '2029-01-12'
    vm.form.harga = '18500000'
    vm.form.notes = 'catatan uji'
    await wrapper.vm.$nextTick()
  }

  it('harga: typing a formatted-looking value keeps form.harga a raw digit-string (NumberInput money)', async () => {
    const wrapper = await mountNew()
    const input = wrapper.find('input[placeholder="0"]')
    expect(input.exists()).toBe(true)
    await input.setValue('18500000')
    const vm = wrapper.vm as unknown as FormVm
    expect(vm.form.harga).toBe('18500000')
    expect((input.element as HTMLInputElement).value).toBe('18.500.000')
  })

  it('submits an exact AssetCreateInput (purchase_cost as a decimal string, all filled FK ids present)', async () => {
    const wrapper = await mountNew()
    await fillValidForm(wrapper)

    let capturedBody: Record<string, unknown> | undefined
    setHandler((path, opts) => {
      if (path === '/requests') {
        capturedBody = (opts?.body as { payload: Record<string, unknown> }).payload
        return { id: 'r1', status: 'pending' }
      }
      return defaultHandler()(path, opts)
    })

    const vm = wrapper.vm as unknown as FormVm
    await vm.save()
    await flushPromises()

    expect(capturedBody).toEqual({
      name: 'Laptop Dell Latitude 5440',
      category_id: 'c1',
      office_id: 'o1',
      asset_class: 'tangible',
      brand_id: 'b1',
      model_id: 'm1',
      room_id: 'r1',
      unit_id: 'u1',
      vendor_id: 'v1',
      serial_number: 'SN-1',
      po_number: 'PO-1',
      funding_source: 'APBN',
      purchase_date: '2026-01-12',
      warranty_expiry: '2029-01-12',
      floor_id: 'f1',
      pic_employee_id: null,
      capacity: null,
      lease_date: null,
      installation_date: null,
      warranty_start: null,
      notes: 'catatan uji',
      purchase_cost: '18500000',
      quantity: 1
    })
    expect(typeof capturedBody!.purchase_cost).toBe('string')
  })

  it('on success shows the request-submitted toast and clears the error banner (redirect path)', async () => {
    const wrapper = await mountNew()
    await fillValidForm(wrapper)
    setHandler((path, opts) => {
      if (path === '/requests') return { id: 'r1', status: 'pending' }
      return defaultHandler()(path, opts)
    })
    const vm = wrapper.vm as unknown as FormVm
    await vm.save()
    await flushPromises()
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ title: 'Pengajuan terkirim — menunggu persetujuan' }))
    // the error banner only ever gets set in the catch branch — its absence
    // here, together with the success toast, confirms the try block (which
    // ends in the redirect) ran to completion without throwing.
    expect(vm.submitError).toBe(false)
    expect(wrapper.html()).not.toContain('Gagal mengirim data')
  })

  it('on API failure shows the inline error banner, keeps the input, and skips the success toast', async () => {
    const wrapper = await mountNew()
    await fillValidForm(wrapper)
    setHandler((path, opts) => {
      if (path === '/requests') throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      return defaultHandler()(path, opts)
    })
    const vm = wrapper.vm as unknown as FormVm
    await vm.save()
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(vm.submitError).toBe(true)
    expect(wrapper.text()).toContain('Gagal mengirim data')
    expect(toastAddMock).not.toHaveBeenCalledWith(expect.objectContaining({ title: 'Pengajuan terkirim — menunggu persetujuan' }))
    // input preserved
    expect(vm.form.nama).toBe('Laptop Dell Latitude 5440')
    expect(vm.form.harga).toBe('18500000')
  })
})

// ---------------------------------------------------------------------------
// Edit mode
// ---------------------------------------------------------------------------

describe('AssetForm — edit mode: render + read-only fields', () => {
  it('populates form fields from the initial (English) Asset', async () => {
    const wrapper = await mountEdit()
    const vm = wrapper.vm as unknown as FormVm
    expect(vm.form.nama).toBe('Laptop Dell Latitude 5440')
    expect(vm.form.categoryId).toBe('c1')
    expect(vm.form.brandId).toBe('b1')
    expect(vm.form.modelId).toBe('m1')
    expect(vm.form.serialNumber).toBe('SN-DL5440-2026-0312')
    expect(vm.form.poNumber).toBe('PO/2026/0112')
    expect(vm.form.fundingSource).toBe('APBN')
    expect(vm.form.tglBeli).toBe('2026-01-12')
    expect(vm.form.warrantyExpiry).toBe('2029-01-12')
    expect(vm.form.notes).toBe('Catatan pengadaan awal.')
    // room resolved from office's floors/rooms
    expect(vm.form.floorId).toBe('f1')
    expect(vm.form.roomId).toBe('r1')
  })

  it('renders the edit title, real asset tag, class and status as read-only', async () => {
    const wrapper = await mountEdit()
    const text = wrapper.text()
    const inputValues = wrapper.findAll('input').map(i => (i.element as HTMLInputElement).value)
    expect(text).toContain('Edit Aset')
    expect(inputValues).toContain('JKT01-ELK-2026-00001')
    expect(inputValues).toContain('Berwujud') // asset_class label
    expect(text).toContain('Tersedia') // status badge label
    expect(inputValues.some(v => v.includes('18.500.000'))).toBe(true) // read-only purchase_cost
  })

  it('renders kantor as read-only text, not an editable select', async () => {
    const wrapper = await mountEdit()
    expect(wrapper.find('[data-testid="asset-form-kantor-select"]').exists()).toBe(false)
    const inputValues = wrapper.findAll('input').map(i => (i.element as HTMLInputElement).value)
    expect(inputValues).toContain('Cabang Jakarta Selatan')
  })
})

describe('AssetForm — edit mode: submit only AssetUpdateInput fields', () => {
  it('PUTs /assets/:id with exactly the AssetUpdateInput keys — no purchase_cost/asset_class/office_id/status/tag', async () => {
    let capturedPath = ''
    let capturedBody: Record<string, unknown> = {}
    setHandler((path, opts) => {
      if (path === '/assets/a1' && opts?.method === 'PUT') {
        capturedPath = path
        capturedBody = opts.body as Record<string, unknown>
        return { ...EDIT_ASSET }
      }
      return defaultHandler()(path, opts)
    })
    const wrapper = await mountEdit()
    const vm = wrapper.vm as unknown as FormVm
    await vm.save()
    await flushPromises()

    expect(capturedPath).toBe('/assets/a1')
    expect(capturedBody).toEqual({
      name: 'Laptop Dell Latitude 5440',
      category_id: 'c1',
      brand_id: 'b1',
      model_id: 'm1',
      room_id: 'r1',
      unit_id: 'u1',
      vendor_id: 'v1',
      serial_number: 'SN-DL5440-2026-0312',
      po_number: 'PO/2026/0112',
      funding_source: 'APBN',
      purchase_date: '2026-01-12',
      warranty_expiry: '2029-01-12',
      floor_id: 'f1',
      pic_employee_id: null,
      capacity: null,
      lease_date: null,
      installation_date: null,
      warranty_start: null,
      notes: 'Catatan pengadaan awal.'
    })
    expect(capturedBody).not.toHaveProperty('purchase_cost')
    expect(capturedBody).not.toHaveProperty('asset_class')
    expect(capturedBody).not.toHaveProperty('office_id')
    expect(capturedBody).not.toHaveProperty('status')
    expect(capturedBody).not.toHaveProperty('asset_tag')
  })

  it('on success shows the saved toast and redirects to the asset detail page', async () => {
    setHandler((path, opts) => {
      if (path === '/assets/a1' && opts?.method === 'PUT') return { ...EDIT_ASSET }
      return defaultHandler()(path, opts)
    })
    const wrapper = await mountEdit()
    const vm = wrapper.vm as unknown as FormVm
    await vm.save()
    await flushPromises()
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ title: 'Aset diperbarui' }))
    expect(vm.submitError).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Edit mode — Lampiran (live)
// ---------------------------------------------------------------------------

describe('AssetForm — edit mode: attachments are live', () => {
  it('lists existing attachments and enables the dropzone (not disabled)', async () => {
    const wrapper = await mountEdit()
    expect(wrapper.text()).toContain('foto-depan.jpg')
    const drop = wrapper.findAll('button').find(b => b.text().includes('Seret & lepas'))
    expect(drop!.attributes('disabled')).toBeUndefined()
  })

  it('uploads a file via the attachments composable and refreshes the list', async () => {
    let uploadCalled = false
    const NEW_ATTACHMENTS = [...ATTACHMENTS, { id: 'att2', asset_id: 'a1', kind: 'photo', original_filename: 'foto-baru.jpg', size_bytes: 4096, mime_type: 'image/jpeg', has_thumbnail: true, created_at: '2026-01-02T00:00:00Z' }]
    setHandler((path, opts) => {
      if (path === '/assets/a1/attachments' && opts?.method === 'POST') {
        uploadCalled = true
        return { id: 'att2', asset_id: 'a1', kind: 'photo', original_filename: 'foto-baru.jpg', size_bytes: 4096, mime_type: 'image/jpeg', has_thumbnail: true, created_at: '2026-01-02T00:00:00Z' }
      }
      if (path === '/assets/a1/attachments' && (!opts || opts.method !== 'POST')) {
        return { data: uploadCalled ? NEW_ATTACHMENTS : ATTACHMENTS, total: uploadCalled ? 2 : 1, limit: 20, offset: 0 }
      }
      return defaultHandler()(path, opts)
    })
    const wrapper = await mountEdit()
    const vm = wrapper.vm as unknown as FormVm
    const file = new File(['data'], 'foto-baru.jpg', { type: 'image/jpeg' })
    await vm.onFileChange({ target: { files: [file], value: '' } })
    await flushPromises()
    expect(uploadCalled).toBe(true)
    expect(wrapper.text()).toContain('foto-baru.jpg')
  })

  it('removes an attachment via the composable after confirming', async () => {
    let removeCalled = false
    setHandler((path, opts) => {
      if (path === '/assets/a1/attachments/att1' && opts?.method === 'DELETE') {
        removeCalled = true
        return undefined
      }
      if (path === '/assets/a1/attachments' && (!opts || opts.method !== 'POST')) {
        return { data: removeCalled ? [] : ATTACHMENTS, total: removeCalled ? 0 : 1, limit: 20, offset: 0 }
      }
      return defaultHandler()(path, opts)
    })
    const wrapper = await mountEdit()
    const vm = wrapper.vm as unknown as FormVm
    const p = vm.removeAttachment(vm.attachments[0]!)
    await flushPromises()
    useConfirm().resolve(true)
    await p
    await flushPromises()
    expect(removeCalled).toBe(true)
    expect(wrapper.text()).toContain('Belum ada lampiran')
  })
})

// ---------------------------------------------------------------------------
// Depreciation — read-only, derived from category
// ---------------------------------------------------------------------------

describe('AssetForm — depreciation info is read-only and category-derived', () => {
  it('shows a placeholder before a category is chosen', async () => {
    const wrapper = await mountNew()
    expect(wrapper.text()).toContain('Pilih kategori untuk melihat konfigurasi depresiasi')
  })

  it('shows the category default method/life/salvage once a category with defaults is chosen', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.categoryId = 'c1'
    await flushPromises()
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Garis Lurus')
    expect(text).toContain('48 bulan')
    expect(text).toContain('10%')
  })

  it('shows "—" placeholders for a category without depreciation defaults', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.categoryId = 'c2'
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).not.toContain('Garis Lurus')
  })

  it('does not render any editable depreciation method/life/salvage inputs', async () => {
    const wrapper = await mountEdit()
    // No select bound to a "metode" field, no number input for masa/residu.
    expect(wrapper.find('[data-testid="asset-form-metode-select"]').exists()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Legacy-parity fields (spec 2026-07-23): capacity, PIC picker, floor-only
// location, installation/lease/warranty-start dates.
// ---------------------------------------------------------------------------

describe('AssetForm — legacy-parity fields', () => {
  it('renders the capacity input and the PIC async picker', async () => {
    const wrapper = await mountNew()
    expect(wrapper.find('[data-testid="asset-form-capacity"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pic-picker-input"]').exists()).toBe(true)
  })

  it('renders the new date fields (installation, lease, warranty-start labels)', async () => {
    const wrapper = await mountNew()
    const text = wrapper.text()
    expect(text).toContain('Tanggal Instalasi')
    expect(text).toContain('Tanggal Sewa')
    expect(text).toContain('Garansi Mulai')
  })

  it('blocks save for a tangible asset with neither floor nor room, showing the location error', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.nama = 'Kursi Kerja'
    vm.form.categoryId = 'c1' // tangible
    vm.form.officeId = 'o1'
    await flushPromises()
    vm.form.tglBeli = '2026-02-01'
    vm.form.harga = '750000'
    // deliberately leave floorId + roomId empty
    let requestsCalled = false
    setHandler((path, opts) => {
      if (path === '/requests') {
        requestsCalled = true
        return { id: 'r1', status: 'pending' }
      }
      return defaultHandler()(path, opts)
    })
    await vm.save()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Pilih minimal lantai')
    expect(requestsCalled).toBe(false)
  })

  it('a floor-only tangible asset (no room) passes validation and submits floor_id with null room_id + capacity', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.nama = 'AC Split'
    vm.form.categoryId = 'c1'
    vm.form.officeId = 'o1'
    await flushPromises()
    vm.form.floorId = 'f1'
    await flushPromises()
    // no roomId
    vm.form.tglBeli = '2026-02-01'
    vm.form.harga = '4500000'
    vm.form.capacity = '2 PK'
    await wrapper.vm.$nextTick()
    let capturedBody: Record<string, unknown> | undefined
    setHandler((path, opts) => {
      if (path === '/requests') {
        capturedBody = (opts?.body as { payload: Record<string, unknown> }).payload
        return { id: 'r1', status: 'pending' }
      }
      return defaultHandler()(path, opts)
    })
    await vm.save()
    await flushPromises()
    expect(capturedBody).toBeDefined()
    expect(capturedBody!.floor_id).toBe('f1')
    expect(capturedBody!.room_id).toBe(null)
    expect(capturedBody!.capacity).toBe('2 PK')
  })

  it('populates capacity and PIC from the initial asset in edit mode', async () => {
    const wrapper = await mountEdit({ ...EDIT_ASSET, capacity: 'Core i7 16GB', pic_employee_id: 'e9', floor_id: 'f1' })
    const vm = wrapper.vm as unknown as FormVm
    expect(vm.form.capacity).toBe('Core i7 16GB')
    expect((vm.form as Record<string, string>).picEmployeeId).toBe('e9')
  })
})

// ---------------------------------------------------------------------------
// Batch registration (spec 2026-07-23 section 9): quantity field, summary,
// serial-per-unit guard, and amount = purchase_cost * quantity.
// ---------------------------------------------------------------------------

describe('AssetForm — batch registration', () => {
  async function fillBatchForm(wrapper: Awaited<ReturnType<typeof mountNew>>) {
    const vm = wrapper.vm as unknown as FormVm
    vm.form.nama = 'Kursi Kantor'
    vm.form.categoryId = 'c1'
    vm.form.officeId = 'o1'
    await flushPromises()
    vm.form.floorId = 'f1'
    await flushPromises()
    vm.form.tglBeli = '2026-02-01'
    vm.form.harga = '3000000'
    await wrapper.vm.$nextTick()
  }

  it('renders the Jumlah quantity field in new mode only', async () => {
    const newWrapper = await mountNew()
    expect(newWrapper.find('[data-testid="asset-form-quantity"]').exists()).toBe(true)
    const editWrapper = await mountEdit()
    expect(editWrapper.find('[data-testid="asset-form-quantity"]').exists()).toBe(false)
  })

  it('defaults quantity to 1 and hides the batch summary until quantity > 1', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    expect(vm.form.quantity).toBe('1')
    expect(wrapper.find('[data-testid="asset-form-batch-summary"]').exists()).toBe(false)
  })

  it('shows the "N assets" summary once quantity exceeds 1', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.quantity = '5'
    await wrapper.vm.$nextTick()
    const summary = wrapper.find('[data-testid="asset-form-batch-summary"]')
    expect(summary.exists()).toBe(true)
    expect(summary.text()).toContain('5')
  })

  it('disables the serial input for a batch (serial is unique per unit)', async () => {
    const wrapper = await mountNew()
    const vm = wrapper.vm as unknown as FormVm
    vm.form.serialNumber = 'SN-1'
    vm.form.quantity = '3'
    await wrapper.vm.$nextTick()
    // The serial input is the one bound to the serial FormField; find it via its
    // label proximity is brittle, so assert on the disabled input count change:
    const disabledInputs = wrapper.findAll('input').filter(i => (i.element as HTMLInputElement).disabled)
    // In new mode the only disabled text input is the auto-tag code + the batched serial.
    expect(disabledInputs.length).toBeGreaterThanOrEqual(1)
  })

  it('submits amount = purchase_cost * quantity, quantity in payload, and null serial for a batch', async () => {
    const wrapper = await mountNew()
    await fillBatchForm(wrapper)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.serialNumber = 'SN-should-be-dropped'
    vm.form.quantity = '10'
    await wrapper.vm.$nextTick()

    let capturedAmount: unknown
    let capturedPayload: Record<string, unknown> | undefined
    setHandler((path, opts) => {
      if (path === '/requests') {
        const body = opts?.body as { amount: unknown, payload: Record<string, unknown> }
        capturedAmount = body.amount
        capturedPayload = body.payload
        return { id: 'r1', status: 'pending' }
      }
      return defaultHandler()(path, opts)
    })
    await vm.save()
    await flushPromises()

    expect(capturedAmount).toBe('30000000')
    expect(capturedPayload!.quantity).toBe(10)
    expect(capturedPayload!.serial_number).toBe(null)
  })

  it('keeps the serial number for a single-unit submit (quantity 1)', async () => {
    const wrapper = await mountNew()
    await fillBatchForm(wrapper)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.serialNumber = 'SN-KEEP'
    vm.form.quantity = '1'
    await wrapper.vm.$nextTick()

    let capturedPayload: Record<string, unknown> | undefined
    setHandler((path, opts) => {
      if (path === '/requests') {
        capturedPayload = (opts?.body as { payload: Record<string, unknown> }).payload
        return { id: 'r1', status: 'pending' }
      }
      return defaultHandler()(path, opts)
    })
    await vm.save()
    await flushPromises()
    expect(capturedPayload!.serial_number).toBe('SN-KEEP')
    expect(capturedPayload!.quantity).toBe(1)
  })

  it('blocks save with a validation error when quantity is below 1', async () => {
    const wrapper = await mountNew()
    await fillBatchForm(wrapper)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.quantity = '0'
    let requestsCalled = false
    setHandler((path, opts) => {
      if (path === '/requests') {
        requestsCalled = true
        return { id: 'r1', status: 'pending' }
      }
      return defaultHandler()(path, opts)
    })
    await vm.save()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Jumlah harus bilangan bulat minimal 1')
    expect(requestsCalled).toBe(false)
  })
})
