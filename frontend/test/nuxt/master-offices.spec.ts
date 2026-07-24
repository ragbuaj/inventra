// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { useConfirm } from '~/composables/useConfirm'
import OfficesPage from '~/pages/master/offices.vue'

// ---------------------------------------------------------------------------
// Stub API client — all useApiClient().request calls route through _handler.
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
  { id: 'ot1', name: 'Pusat', tier: 'pusat' },
  { id: 'ot2', name: 'Cabang', tier: 'office' },
  { id: 'ot3', name: 'Wilayah', tier: 'wilayah' }
]
const PROVINCES = [
  { id: 'pr1', name: 'DKI Jakarta' },
  { id: 'pr2', name: 'Jawa Barat' }
]
const CITIES = [
  { id: 'c1', name: 'Jakarta Pusat', province_id: 'pr1' },
  { id: 'c2', name: 'Bandung', province_id: 'pr2' }
]

// Legacy-parity Fase 5 masters the office form loads for its two new selects.
const OFFICE_CLASSES = [
  { id: 'oc1', name: 'Kelas A', code: 'A', is_active: true }
]
const BUILDING_CLASSIFICATIONS = [
  { id: 'bc1', name: 'Gedung Rendah', code: 'LOW', min_floors: 1, max_floors: 4, is_active: true },
  { id: 'bc2', name: 'Gedung Tinggi', code: 'HIGH', min_floors: 25, max_floors: null, is_active: true }
]
const EMPLOYEES: Record<string, unknown> = {
  e1: { id: 'e1', name: 'Budi Santoso', code: 'EMP-001' }
}
// o1 carries the full set of Fase 5 legacy-parity columns so the detail view can
// be asserted end-to-end; o2 leaves them null/empty to exercise the "—" fallback.
const OFFICES = [
  { id: 'o1', parent_id: null, office_type_id: 'ot1', province_id: 'pr1', city_id: 'c1', name: 'Kantor Pusat', code: 'PST', address: 'Jl. Merdeka No. 1', is_active: true, latitude: -6.2, longitude: 106.8166, ownership_status: 'milik', office_class_id: 'oc1', building_classification_id: 'bc2', floor_count: 25, building_area: '1500.50', office_kind: 'syariah', description: 'Kantor pusat utama.', head_employee_id: 'e1', contact: '021-5551234', created_at: null, updated_at: null },
  { id: 'o2', parent_id: 'o1', office_type_id: 'ot2', province_id: 'pr2', city_id: 'c2', name: 'Cabang Bandung', code: 'BDG', address: null, is_active: false, latitude: null, longitude: null, ownership_status: null, office_class_id: null, building_classification_id: null, floor_count: null, building_area: null, office_kind: 'konvensional', description: null, head_employee_id: null, contact: null, created_at: null, updated_at: null }
]
const FLOORS: Record<string, unknown[]> = {
  o1: [{ id: 'f1', office_id: 'o1', name: 'Lantai 1', level: 1, created_at: null, updated_at: null }]
}
const ROOMS: Record<string, unknown[]> = {
  f1: [{ id: 'rm1', floor_id: 'f1', name: 'Lobi', code: 'L1-LOB', created_at: null, updated_at: null }]
}

function parseQuery(path: string): Record<string, string> {
  const i = path.indexOf('?')
  if (i === -1) return {}
  const out: Record<string, string> = {}
  new URLSearchParams(path.slice(i + 1)).forEach((v, k) => {
    out[k] = v
  })
  return out
}

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  const method = (opts?.method as string) ?? 'GET'
  // NOTE: /office-classes must be matched BEFORE /offices — startsWith('/offices')
  // would otherwise swallow it and return the office list shape.
  if (path.startsWith('/office-classes')) return { data: OFFICE_CLASSES, total: OFFICE_CLASSES.length, limit: 100, offset: 0 }
  if (path.startsWith('/building-classifications')) return { data: BUILDING_CLASSIFICATIONS, total: BUILDING_CLASSIFICATIONS.length, limit: 100, offset: 0 }
  if (path.startsWith('/office-types')) return { data: OFFICE_TYPES }
  if (path.startsWith('/provinces')) return { data: PROVINCES }
  if (path.startsWith('/cities')) return { data: CITIES }
  if (path.startsWith('/employees/')) {
    const id = path.slice('/employees/'.length).split('?')[0]!
    return EMPLOYEES[id] ?? { id, name: '', code: '' }
  }
  if (path.startsWith('/employees')) return { data: [], total: 0, limit: 20, offset: 0 }
  if (path.startsWith('/floors')) {
    if (method === 'GET') return { data: FLOORS[parseQuery(path)['office_id'] ?? ''] ?? [], total: 0, limit: 100, offset: 0 }
    if (method === 'DELETE') return undefined
    return { id: 'f-new', office_id: 'o1', name: 'Lantai 2', level: 2 }
  }
  if (path.startsWith('/rooms')) {
    if (method === 'GET') return { data: ROOMS[parseQuery(path)['floor_id'] ?? ''] ?? [], total: 0, limit: 100, offset: 0 }
    if (method === 'DELETE') return undefined
    return { id: 'r-new', floor_id: 'f1', name: 'Ruang Baru', code: null }
  }
  if (path.startsWith('/offices')) {
    if (method === 'GET') return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
    if (method === 'POST') return { ...OFFICES[0], id: 'o-new' }
    if (method === 'PUT') return { ...OFFICES[0] }
    if (method === 'DELETE') return undefined
  }
  throw new Error(`Unhandled request: ${path} ${method}`)
}

// ---------------------------------------------------------------------------
// Setup
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
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(OfficesPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

type Vm = Record<string, unknown>
async function select(wrapper: Awaited<ReturnType<typeof mountAndWait>>, id: string) {
  await (wrapper.vm as unknown as { onSelect: (id: string) => Promise<void> }).onSelect(id)
  await new Promise(r => setTimeout(r, 100))
  await wrapper.vm.$nextTick()
}

// ---------------------------------------------------------------------------
// Tree + detail rendering (resolved FK names)
// ---------------------------------------------------------------------------

describe('Master Data Kantor — tree & detail', () => {
  it('renders the tree header and seeded office names', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Hierarki Kantor')
    expect(text).toContain('Kantor Pusat')
    expect(text).toContain('Cabang Bandung')
  })

  it('shows the placeholder state until an office is selected', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Pilih kantor untuk melihat detail')
  })

  it('selecting an office resolves office-type/province/city FK ids to names (not UUIDs)', async () => {
    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')
    const text = wrapper.text()
    expect(text).toContain('PST') // code
    expect(text).toContain('Pusat') // office-type name (ot1)
    expect(text).toContain('DKI Jakarta') // province name (pr1)
    expect(text).toContain('Jakarta Pusat') // city name (c1)
    // raw FK UUIDs must not leak into the rendered detail
    expect(text).not.toContain('ot1')
    expect(text).not.toContain('pr1')
    expect(text).not.toContain('c1')
  })

  it('renders every Fase 5 detail field with resolved names for the selected office', async () => {
    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')
    const text = wrapper.text()
    // Tipe kantor (office_kind) + status kepemilikan → localized labels, not raw enum values
    expect(text).toContain('Syariah')
    expect(text).toContain('Milik')
    expect(text).not.toContain('syariah')
    // Kelas kantor + klasifikasi gedung → resolved reference names, not UUIDs
    expect(text).toContain('Kelas A')
    expect(text).toContain('Gedung Tinggi')
    expect(text).not.toContain('oc1')
    expect(text).not.toContain('bc2')
    // Jumlah lantai, luas bangunan (with m2 unit), kontak
    expect(text).toContain('25')
    expect(text).toContain('1500.50 m2')
    expect(text).toContain('021-5551234')
    // Koordinat + deskripsi
    expect(text).toContain('-6.2, 106.8166')
    expect(text).toContain('Kantor pusat utama.')
  })

  it('resolves the head-of-office id to the employee name in the detail view', async () => {
    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')
    // watcher resolves head_employee_id → GET /employees/e1 asynchronously
    await new Promise(r => setTimeout(r, 150))
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Budi Santoso')
  })

  it('falls back to em-dash for offices with empty Fase 5 fields', async () => {
    const wrapper = await mountAndWait()
    await select(wrapper, 'o2') // all Fase 5 fields null/empty
    const vm = wrapper.vm as unknown as Record<string, unknown>
    expect(vm['detailOwnership']).toBe('—')
    expect(vm['detailOfficeClass']).toBe('—')
    expect(vm['detailBuildingClass']).toBe('—')
    expect(vm['detailFloorCount']).toBe('—')
    expect(vm['detailBuildingArea']).toBe('—')
    expect(vm['detailContact']).toBe('—')
    expect(vm['detailCoord']).toBe('—')
    expect(vm['detailDescription']).toBe('—')
    expect(vm['detailHead']).toBe('—')
    // office_kind defaults to konvensional even when unset
    expect(vm['detailKind']).toBe('Konvensional')
  })

  it('resolves the parent office name for a child office', async () => {
    const wrapper = await mountAndWait()
    await select(wrapper, 'o2')
    const text = wrapper.text()
    // parent o1 → "Kantor Pusat"; inactive status label
    expect(text).toContain('Kantor Pusat')
    expect(text).toContain('Nonaktif')
  })

  it('loads floors and rooms for the selected office', async () => {
    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')
    const text = wrapper.text()
    expect(text).toContain('Lantai & Ruangan')
    expect(text).toContain('Lantai 1')
    expect(text).toContain('Lobi')
    expect(text).toContain('L1-LOB')
  })

  it('shows the empty-floors CTA for an office without floors', async () => {
    const wrapper = await mountAndWait()
    await select(wrapper, 'o2') // o2 has no floors in fixtures
    expect(wrapper.text()).toContain('Belum ada lantai')
  })
})

// ---------------------------------------------------------------------------
// Deep-link from Peta Lokasi ("Lihat Kantor")
// ---------------------------------------------------------------------------

describe('Master Data Kantor — deep-link from peta lokasi', () => {
  it('auto-selects the office named in the ?office= query on mount', async () => {
    const wrapper = await mountSuspended(OfficesPage, { route: '/master/offices?office=o1' })
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()
    expect((wrapper.vm as unknown as { selectedId: string | undefined }).selectedId).toBe('o1')
    const text = wrapper.text()
    expect(text).toContain('PST') // detail opened straight away
    expect(text).not.toContain('Pilih kantor untuk melihat detail')
  })

  it('ignores an unknown ?office= id and stays on the placeholder', async () => {
    const wrapper = await mountSuspended(OfficesPage, { route: '/master/offices?office=missing' })
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()
    expect((wrapper.vm as unknown as { selectedId: string | undefined }).selectedId).toBeUndefined()
    expect(wrapper.text()).toContain('Pilih kantor untuk melihat detail')
  })
})

// ---------------------------------------------------------------------------
// FK pickers — city filtered by province
// ---------------------------------------------------------------------------

describe('Master Data Kantor — city picker', () => {
  it('cityOptions is empty until a province is chosen, then filters by province', async () => {
    const wrapper = await mountAndWait()
    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as Vm
    expect((vm['cityOptions'] as unknown[]).length).toBe(0)

    const form = vm['form'] as Record<string, unknown>
    form['province_id'] = 'pr1'
    await wrapper.vm.$nextTick()
    const opts = vm['cityOptions'] as Array<{ value: string, label: string }>
    expect(opts.some(o => o.label === 'Jakarta Pusat')).toBe(true)
    expect(opts.some(o => o.label === 'Bandung')).toBe(false)
  })

  it('changing province clears a city that no longer belongs to it', async () => {
    const wrapper = await mountAndWait()
    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as Vm
    // set province via the bridge setter (mirrors the USelect binding)
    ;(vm as Record<string, unknown>)['formProvinceId'] = 'pr1'
    await wrapper.vm.$nextTick()
    ;(vm as Record<string, unknown>)['formCityId'] = 'c1'
    await wrapper.vm.$nextTick()
    expect((vm['form'] as Record<string, unknown>)['city_id']).toBe('c1')

    ;(vm as Record<string, unknown>)['formProvinceId'] = 'pr2'
    await wrapper.vm.$nextTick()
    expect((vm['form'] as Record<string, unknown>)['city_id']).toBe(null)
  })
})

// ---------------------------------------------------------------------------
// Coordinate bridge
// ---------------------------------------------------------------------------

describe('Master Data Kantor — coordinate inputs', () => {
  it('formLat/formLng coerce strings to numbers and empty to null', async () => {
    const wrapper = await mountAndWait()
    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as Record<string, unknown>
    vm['formLat'] = '-6.2000'
    vm['formLng'] = '106.8166'
    await wrapper.vm.$nextTick()
    const form = vm['form'] as Record<string, unknown>
    expect(form['latitude']).toBe(-6.2)
    expect(form['longitude']).toBe(106.8166)

    vm['formLat'] = ''
    await wrapper.vm.$nextTick()
    expect(form['latitude']).toBe(null)
  })
})

// ---------------------------------------------------------------------------
// Create / edit / delete
// ---------------------------------------------------------------------------

describe('Master Data Kantor — create', () => {
  it('POSTs /offices with office_type_id + name + code + parent_id, omitting empty optionals', async () => {
    let capturedPath = ''
    let capturedBody: Record<string, unknown> = {}
    setHandler((path, opts) => {
      if (path === '/offices' && opts?.method === 'POST') {
        capturedPath = path
        capturedBody = (opts['body'] as Record<string, unknown>) ?? {}
        return { ...OFFICES[0], id: 'o-new' }
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Vm)['form'] as Record<string, unknown>
    form['name'] = 'Cabang Baru'
    form['code'] = 'CB99'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/offices')
    expect(capturedBody['office_type_id']).toBe('ot1') // first option pre-selected
    expect(capturedBody['name']).toBe('Cabang Baru')
    expect(capturedBody['code']).toBe('CB99')
    expect(capturedBody['is_active']).toBe(true)
    expect('parent_id' in capturedBody).toBe(true)
    // empty optionals are not sent
    expect(capturedBody['province_id']).toBeUndefined()
    expect(capturedBody['city_id']).toBeUndefined()
    expect(capturedBody['latitude']).toBeUndefined()
  })

  it('does not POST when name/code are empty (required guard)', async () => {
    let postCalled = false
    setHandler((path, opts) => {
      if (path === '/offices' && opts?.method === 'POST') {
        postCalled = true
        return { id: 'x' }
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 150))
    expect(postCalled).toBe(false)
  })
})

describe('Master Data Kantor — edit & delete', () => {
  it('PUTs /offices/:id when editing the selected office', async () => {
    let capturedPath = ''
    let capturedBody: Record<string, unknown> = {}
    setHandler((path, opts) => {
      if (path.startsWith('/offices/') && opts?.method === 'PUT') {
        capturedPath = path
        capturedBody = (opts['body'] as Record<string, unknown>) ?? {}
        return { ...OFFICES[0], name: 'Kantor Pusat Diubah' }
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')
    ;(wrapper.vm as unknown as { openEdit: () => void }).openEdit()
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Vm)['form'] as Record<string, unknown>
    expect(form['name']).toBe('Kantor Pusat') // pre-filled
    form['name'] = 'Kantor Pusat Diubah'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/offices/o1')
    expect(capturedBody['name']).toBe('Kantor Pusat Diubah')
    expect(capturedBody['office_type_id']).toBe('ot1')
  })

  it('DELETEs /offices/:id after confirmation', async () => {
    let deletedPath = ''
    setHandler((path, opts) => {
      if (path.startsWith('/offices/') && opts?.method === 'DELETE') {
        deletedPath = path
        return undefined
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')

    const p = (wrapper.vm as unknown as { onDelete: () => Promise<void> }).onDelete()
    await wrapper.vm.$nextTick()
    useConfirm().resolve(true)
    await p
    await new Promise(r => setTimeout(r, 200))

    expect(deletedPath).toBe('/offices/o1')
  })

  it('does not DELETE when the confirm dialog is cancelled', async () => {
    let deleteCalled = false
    setHandler((path, opts) => {
      if (path.startsWith('/offices/') && opts?.method === 'DELETE') {
        deleteCalled = true
        return undefined
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')

    const p = (wrapper.vm as unknown as { onDelete: () => Promise<void> }).onDelete()
    await wrapper.vm.$nextTick()
    useConfirm().resolve(false)
    await p
    await new Promise(r => setTimeout(r, 150))

    expect(deleteCalled).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Floors & rooms mutations
// ---------------------------------------------------------------------------

describe('Master Data Kantor — floors & rooms', () => {
  it('addFloor POSTs /floors with the selected office_id + level', async () => {
    let capturedBody: Record<string, unknown> = {}
    setHandler((path, opts) => {
      if (path === '/floors' && opts?.method === 'POST') {
        capturedBody = (opts['body'] as Record<string, unknown>) ?? {}
        return { id: 'f-new', office_id: 'o1', name: 'Lantai 2', level: 2 }
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')

    await (wrapper.vm as unknown as { addFloor: () => Promise<void> }).addFloor()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedBody['office_id']).toBe('o1')
    expect(capturedBody['name']).toBe('Lantai 2') // 1 existing floor → next is 2
    expect(capturedBody['level']).toBe(2)
  })

  it('renaming a floor PUTs /floors/:id and resends the required office_id', async () => {
    let capturedPath = ''
    let capturedBody: Record<string, unknown> = {}
    setHandler((path, opts) => {
      if (path.startsWith('/floors/') && opts?.method === 'PUT') {
        capturedPath = path
        capturedBody = (opts['body'] as Record<string, unknown>) ?? {}
        return { id: 'f1', office_id: 'o1', name: 'Lantai Dasar', level: 1 }
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')

    const vm = wrapper.vm as unknown as Record<string, unknown>
    ;(vm['startEditFloor'] as (f: unknown) => void)({ id: 'f1', office_id: 'o1', name: 'Lantai 1', level: 1 })
    vm['editingFloorName'] = 'Lantai Dasar'
    await wrapper.vm.$nextTick()
    await (vm['commitEditFloor'] as () => Promise<void>)()
    await new Promise(r => setTimeout(r, 200))

    expect(capturedPath).toBe('/floors/f1')
    expect(capturedBody['office_id']).toBe('o1')
    expect(capturedBody['name']).toBe('Lantai Dasar')
  })

  it('addRoom POSTs /rooms with the floor_id', async () => {
    let capturedBody: Record<string, unknown> = {}
    setHandler((path, opts) => {
      if (path === '/rooms' && opts?.method === 'POST') {
        capturedBody = (opts['body'] as Record<string, unknown>) ?? {}
        return { id: 'r-new', floor_id: 'f1', name: 'Ruang Baru', code: null }
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await select(wrapper, 'o1')

    await (wrapper.vm as unknown as { addRoom: (id: string) => Promise<void> }).addRoom('f1')
    await new Promise(r => setTimeout(r, 200))

    expect(capturedBody['floor_id']).toBe('f1')
    expect(capturedBody['name']).toBe('Ruang Baru')
  })
})

// ---------------------------------------------------------------------------
// Load error
// ---------------------------------------------------------------------------

describe('Master Data Kantor — load error', () => {
  it('shows the load-error panel + retry when GET /offices fails', async () => {
    setHandler((path, opts) => {
      if (path.startsWith('/offices') && (!opts?.method || opts.method === 'GET')) {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Gagal memuat kantor.')
    expect(text).toContain('Coba lagi')
    expect(text).not.toContain('Kantor Pusat')
  })

  it('retry re-fetches and recovers when the second call succeeds', async () => {
    let n = 0
    setHandler((path, opts) => {
      if (path.startsWith('/offices') && (!opts?.method || opts.method === 'GET')) {
        n++
        if (n === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat kantor.')
    const retry = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retry).toBeDefined()
    await retry!.trigger('click')
    await new Promise(r => setTimeout(r, 300))
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Kantor Pusat')
    expect(wrapper.text()).not.toContain('Gagal memuat kantor.')
  })
})
