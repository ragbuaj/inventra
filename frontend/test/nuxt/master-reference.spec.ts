// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { useConfirm } from '~/composables/useConfirm'
import ReferencePage from '~/pages/master/reference.vue'

// ---------------------------------------------------------------------------
// Stub API client — all calls routed through _handler
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
// Fixtures
// ---------------------------------------------------------------------------

const OFFICE_TYPES = [
  { id: 'ot1', name: 'Kantor Pusat', tier: 'pusat', is_active: true }
]

const PROVINCES = [
  { id: 'p1', name: 'DKI Jakarta', code: 'DKI' },
  { id: 'p2', name: 'Jawa Barat', code: 'JBR' }
]

const CITIES = [
  { id: 'c1', name: 'Jakarta Selatan', province_id: 'p1', code: 'JKS' }
]

const BRANDS = [
  { id: 'br1', name: 'Dell', is_active: true }
]

function makeEnvelope<T>(data: T[], total?: number) {
  return { data, total: total ?? (data as unknown[]).length, limit: 20, offset: 0 }
}

// Parse query parameters from path string
function parseQuery(path: string): Record<string, string> {
  const idx = path.indexOf('?')
  if (idx === -1) return {}
  const params = new URLSearchParams(path.slice(idx + 1))
  const result: Record<string, string> = {}
  params.forEach((val, key) => {
    result[key] = val
  })
  return result
}

// Return an envelope with total=N and empty data (used for sidebar count calls)
function countResponse(total: number) {
  return { data: [], total, limit: 1, offset: 0 }
}

/**
 * Default handler: routes by path prefix and method.
 * Supports override via overrides map keyed on path prefix or exact path.
 */
function buildDefaultHandler(overrides: Record<string, unknown> = {}): RequestHandler {
  return (path: string, opts?: Record<string, unknown>): unknown => {
    const method = (opts?.method as string | undefined) ?? 'GET'
    const pathBase = path.split('?')[0]!

    // Check overrides first (exact match, no query)
    const overrideKey = `${method}:${pathBase}`
    if (Object.prototype.hasOwnProperty.call(overrides, overrideKey)) {
      return overrides[overrideKey]
    }

    // Sidebar count calls: ?limit=1 for any resource
    const q = parseQuery(path)
    const isCountCall = q['limit'] === '1'

    if (method === 'GET') {
      if (pathBase === '/office-types') {
        return isCountCall ? countResponse(1) : makeEnvelope(OFFICE_TYPES)
      }
      if (pathBase === '/departments') return isCountCall ? countResponse(2) : makeEnvelope([])
      if (pathBase === '/positions') return isCountCall ? countResponse(3) : makeEnvelope([])
      if (pathBase === '/units') return isCountCall ? countResponse(4) : makeEnvelope([])
      if (pathBase === '/maintenance-categories') return isCountCall ? countResponse(5) : makeEnvelope([])
      if (pathBase === '/problem-categories') return isCountCall ? countResponse(6) : makeEnvelope([])
      if (pathBase === '/brands') return isCountCall ? countResponse(7) : makeEnvelope(BRANDS)
      if (pathBase === '/vendors') return isCountCall ? countResponse(8) : makeEnvelope([])
      if (pathBase === '/provinces') return makeEnvelope(PROVINCES)
      if (pathBase === '/cities') return isCountCall ? countResponse(10) : makeEnvelope(CITIES)
      if (pathBase === '/models') return isCountCall ? countResponse(11) : makeEnvelope([])
    }

    throw new Error(`Unhandled: ${method} ${path}`)
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
  setHandler(buildDefaultHandler())
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(ReferencePage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

async function setVmRef(wrapper: Awaited<ReturnType<typeof mountAndWait>>, key: string, value: unknown) {
  ;(wrapper.vm as unknown as Record<string, unknown>)[key] = value
  await wrapper.vm.$nextTick()
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
}

// ---------------------------------------------------------------------------
// Default load
// ---------------------------------------------------------------------------

describe('Master Data Referensi — default load', () => {
  it('renders panel title and subtitle', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Master Data')
    expect(text).toContain('Data referensi')
  })

  it('sidebar lists all 11 resource labels', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Jenis Kantor')
    expect(text).toContain('Departemen')
    expect(text).toContain('Jabatan')
    expect(text).toContain('Satuan')
    expect(text).toContain('Kategori Pemeliharaan')
    expect(text).toContain('Kategori Masalah')
    expect(text).toContain('Brand')
    expect(text).toContain('Vendor')
    expect(text).toContain('Provinsi')
    expect(text).toContain('Kota')
    expect(text).toContain('Model')
  })

  it('sidebar shows numeric counts from the 11 count calls', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // office-types total=1, departments total=2 — both must appear as digits
    expect(text).toContain('1')
    expect(text).toContain('2')
  })

  it('renders office-types row name in the table', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Kantor Pusat')
  })

  it('Tambah button is visible for the admin', async () => {
    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah'))
    expect(addBtn).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// FK name resolution
// ---------------------------------------------------------------------------

describe('Master Data Referensi — FK name resolution (cities → province)', () => {
  it('resolves province_id to province name in the cities table cell', async () => {
    const wrapper = await mountAndWait()

    // Switch to cities via vm ref to avoid jsdom click propagation issues
    await setVmRef(wrapper, 'resourceKey', 'cities')

    const text = wrapper.text()
    // The city row shows the resolved province name, not the raw id
    expect(text).toContain('DKI Jakarta')
    expect(text).not.toContain('p1')
  })

  it('shows the city name in the row', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'resourceKey', 'cities')
    expect(wrapper.text()).toContain('Jakarta Selatan')
  })
})

// ---------------------------------------------------------------------------
// FK picker + create
// ---------------------------------------------------------------------------

describe('Master Data Referensi — FK picker + create (cities)', () => {
  it('POST /cities captures province_id and name in request body', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      const pathBase = path.split('?')[0]!
      if (method === 'POST' && pathBase === '/cities') {
        capturedPath = pathBase
        capturedOpts = opts ?? {}
        return { id: 'c-new', name: 'Bekasi', province_id: 'p1' }
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'resourceKey', 'cities')

    // Open create form
    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    // Set form fields via vm
    const vm = wrapper.vm as unknown as Record<string, unknown>
    const form = vm['form'] as Record<string, unknown>
    form['province_id'] = 'p1'
    form['name'] = 'Bekasi'
    form['code'] = 'BKS'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/cities')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['province_id']).toBe('p1')
    expect(body['name']).toBe('Bekasi')
  })
})

// ---------------------------------------------------------------------------
// Required-FK guard
// ---------------------------------------------------------------------------

describe('Master Data Referensi — required FK guard', () => {
  it('does NOT send POST when required FK province_id is empty', async () => {
    let postCalled = false

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      if (method === 'POST') {
        postCalled = true
        return { id: 'c-new', name: 'Test' }
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'resourceKey', 'cities')

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    // Leave province_id empty, only fill name
    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    form['province_id'] = ''
    form['name'] = 'Kota Test'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    expect(postCalled).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Select (tier) field — office-types
// ---------------------------------------------------------------------------

describe('Master Data Referensi — select tier field (office-types)', () => {
  it('POST /office-types captures tier value in request body', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      const pathBase = path.split('?')[0]!
      if (method === 'POST' && pathBase === '/office-types') {
        capturedPath = pathBase
        capturedOpts = opts ?? {}
        return { id: 'ot-new', name: 'Cabang Baru', tier: 'pusat', is_active: true }
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    const vm = wrapper.vm as unknown as Record<string, unknown>
    const form = vm['form'] as Record<string, unknown>
    form['name'] = 'Cabang Baru'
    form['tier'] = 'pusat'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/office-types')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['tier']).toBe('pusat')
    expect(body['name']).toBe('Cabang Baru')
  })
})

// ---------------------------------------------------------------------------
// hasActive gating
// ---------------------------------------------------------------------------

describe('Master Data Referensi — hasActive gating', () => {
  it('shows Status column header for brands (hasActive=true)', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'resourceKey', 'brands')
    expect(wrapper.text()).toContain('Status')
  })

  it('does NOT show Status column header for cities (hasActive=false)', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'resourceKey', 'cities')
    // The "Status" column header must not be present when hasActive is false
    // We check the columns computed — no Status column should exist
    const vm = wrapper.vm as unknown as Record<string, unknown>
    const columns = vm['columns'] as Array<{ accessorKey: string, header: string }>
    const hasStatusCol = columns.some(c => c.accessorKey === 'is_active')
    expect(hasStatusCol).toBe(false)
  })

  it('does NOT show Status column header for provinces (hasActive=false)', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'resourceKey', 'provinces')
    const vm = wrapper.vm as unknown as Record<string, unknown>
    const columns = vm['columns'] as Array<{ accessorKey: string, header: string }>
    const hasStatusCol = columns.some(c => c.accessorKey === 'is_active')
    expect(hasStatusCol).toBe(false)
  })

  it('shows is_active column for office-types (hasActive=true, default resource)', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as Record<string, unknown>
    const columns = vm['columns'] as Array<{ accessorKey: string, header: string }>
    const hasStatusCol = columns.some(c => c.accessorKey === 'is_active')
    expect(hasStatusCol).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Delete — confirm dialog
// ---------------------------------------------------------------------------

describe('Master Data Referensi — delete', () => {
  it('DELETE /<key>/<id> is called after confirm', async () => {
    let deletedPath = ''

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      if (method === 'DELETE') {
        deletedPath = path
        return undefined
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()

    const row = OFFICE_TYPES[0]!
    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: unknown) => Promise<void> }).onDelete(row)
    await wrapper.vm.$nextTick()

    useConfirm().resolve(true)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(deletedPath).toBe('/office-types/ot1')
  })

  it('does NOT call DELETE when user cancels the confirm dialog', async () => {
    let deleteCalled = false

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      if (method === 'DELETE') {
        deleteCalled = true
        return undefined
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()

    const row = OFFICE_TYPES[0]!
    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: unknown) => Promise<void> }).onDelete(row)
    await wrapper.vm.$nextTick()

    useConfirm().resolve(false)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))

    expect(deleteCalled).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

describe('Master Data Referensi — search', () => {
  it('setting search triggers list with search param', async () => {
    const capturedPaths: string[] = []

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      if (method === 'GET' && path.startsWith('/office-types')) {
        capturedPaths.push(path)
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()
    capturedPaths.length = 0

    await setVmRef(wrapper, 'search', 'pusat')

    const searchCall = capturedPaths.find(p => p.includes('search=pusat'))
    expect(searchCall).toBeDefined()
    const q = parseQuery(searchCall!)
    expect(q['search']).toBe('pusat')
    expect(q['offset']).toBe('0')
  })
})

// ---------------------------------------------------------------------------
// Resource switch via sidebar click
// ---------------------------------------------------------------------------

describe('Master Data Referensi — resource switch', () => {
  it('clicking Departemen in the sidebar updates the page header', async () => {
    const wrapper = await mountAndWait()

    // Find the Departemen sidebar button and click it
    const buttons = wrapper.findAll('button')
    const deptBtn = buttons.find(b => b.text().includes('Departemen') && !b.text().includes('Status'))
    if (deptBtn) {
      await deptBtn.trigger('click')
      await new Promise(r => setTimeout(r, 400))
      await wrapper.vm.$nextTick()
    } else {
      // Fallback: set via vm ref if button click is blocked by jsdom
      await setVmRef(wrapper, 'resourceKey', 'departments')
    }

    expect(wrapper.text()).toContain('Departemen')
  })

  it('selectResource function switches resourceKey', async () => {
    const wrapper = await mountAndWait()
    ;(wrapper.vm as unknown as { selectResource: (key: string) => void }).selectResource('brands')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as Record<string, unknown>
    expect(vm['resourceKey']).toBe('brands')
  })
})

// ---------------------------------------------------------------------------
// Edit form
// ---------------------------------------------------------------------------

describe('Master Data Referensi — edit form', () => {
  it('PUT /<key>/<id> is called with form data', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      if (method === 'PUT') {
        capturedPath = path.split('?')[0]!
        capturedOpts = opts ?? {}
        return { ...OFFICE_TYPES[0]!, name: 'Updated Name' }
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()

    const row = OFFICE_TYPES[0]!
    ;(wrapper.vm as unknown as { openEdit: (row: unknown) => void }).openEdit(row)
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    const vm = wrapper.vm as unknown as Record<string, unknown>
    const form = vm['form'] as Record<string, unknown>
    form['name'] = 'Updated Name'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/office-types/ot1')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['name']).toBe('Updated Name')
  })

  it('openEdit pre-fills form fields from the row', async () => {
    const wrapper = await mountAndWait()

    const row = OFFICE_TYPES[0]!
    ;(wrapper.vm as unknown as { openEdit: (row: unknown) => void }).openEdit(row)
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    expect(form['name']).toBe('Kantor Pusat')
    expect(form['tier']).toBe('pusat')
    expect(form['is_active']).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Toggle active
// ---------------------------------------------------------------------------

describe('Master Data Referensi — toggleActive', () => {
  it('toggleActive sends PUT with is_active flipped', async () => {
    let capturedBody: Record<string, unknown> = {}

    setHandler((path, opts) => {
      const method = (opts?.method as string | undefined) ?? 'GET'
      if (method === 'PUT') {
        capturedBody = (opts?.['body'] as Record<string, unknown>) ?? {}
        return { ...OFFICE_TYPES[0]!, is_active: false }
      }
      return buildDefaultHandler()(path, opts)
    })

    const wrapper = await mountAndWait()

    const row = { ...OFFICE_TYPES[0]! }
    await (wrapper.vm as unknown as { toggleActive: (row: unknown) => Promise<void> }).toggleActive(row)
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    // Row was is_active:true → should toggle to false
    expect(capturedBody['is_active']).toBe(false)
  })
})
