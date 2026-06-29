// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { useConfirm } from '~/composables/useConfirm'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request are intercepted here.
// useEmployees, useReference (depts/positions), and the inline /offices fetch
// all go through useApiClient, so one mock covers everything.
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
import EmployeesPage from '~/pages/master/employees.vue'

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const OFFICES = [
  { id: 'o1', name: 'Kantor Pusat' },
  { id: 'o2', name: 'Kantor Cabang' }
]

const DEPARTMENTS = [
  { id: 'd1', name: 'Umum' },
  { id: 'd2', name: 'Keuangan' }
]

const POSITIONS = [
  { id: 'p1', name: 'Staf' },
  { id: 'p2', name: 'Manajer' }
]

const EMPLOYEES = [
  {
    id: 'emp1',
    code: 'NIP001',
    name: 'Andi Pratama',
    email: 'andi@inventra.go.id',
    phone: '0812-1111-2222',
    department_id: 'd1',
    position_id: 'p1',
    office_id: 'o1',
    status: 'active',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  },
  {
    id: 'emp2',
    code: 'NIP002',
    name: 'Bunga Lestari',
    email: 'bunga@inventra.go.id',
    phone: '0813-3333-4444',
    department_id: 'd2',
    position_id: 'p2',
    office_id: 'o2',
    status: 'active',
    created_at: '2026-01-02T00:00:00Z',
    updated_at: '2026-01-02T00:00:00Z'
  },
  {
    id: 'emp3',
    code: 'NIP003',
    name: 'Citra Dewi',
    email: 'citra@inventra.go.id',
    phone: null,
    department_id: 'd1',
    position_id: 'p1',
    office_id: 'o1',
    status: 'suspended',
    created_at: '2026-01-03T00:00:00Z',
    updated_at: '2026-01-03T00:00:00Z'
  }
]

function makeEmployeesResponse(rows = EMPLOYEES, total = EMPLOYEES.length) {
  return { data: rows, total, limit: 100, offset: 0 }
}

function makeRefResponse(rows: { id: string, name: string }[]) {
  return { data: rows, total: rows.length, limit: 100, offset: 0 }
}

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path.startsWith('/offices')) return { data: OFFICES }
  if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
  if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
  if (path.startsWith('/employees') && (!opts?.method || opts.method === 'GET')) {
    return makeEmployeesResponse()
  }
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
  const wrapper = await mountSuspended(EmployeesPage)
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
// Loaded rows — resolved FK names
// ---------------------------------------------------------------------------

describe('Master Pegawai page — loaded rows with resolved FK names', () => {
  it('renders page title', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Pegawai')
  })

  it('renders employee names', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Andi Pratama')
    expect(text).toContain('Bunga Lestari')
    expect(text).toContain('Citra Dewi')
  })

  it('resolves department_id to department name — not raw UUID', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // d1 → Umum, d2 → Keuangan
    expect(text).toContain('Umum')
    expect(text).toContain('Keuangan')
    // Raw IDs must NOT appear in the table
    expect(text).not.toContain('d1')
    expect(text).not.toContain('d2')
  })

  it('resolves position_id to position name — not raw UUID', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // p1 → Staf, p2 → Manajer
    expect(text).toContain('Staf')
    expect(text).toContain('Manajer')
    // Raw IDs must NOT appear
    expect(text).not.toContain('p1')
    expect(text).not.toContain('p2')
  })

  it('resolves office_id to office name — not raw UUID', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // o1 → Kantor Pusat, o2 → Kantor Cabang
    expect(text).toContain('Kantor Pusat')
    expect(text).toContain('Kantor Cabang')
    // Raw IDs must NOT appear
    expect(text).not.toContain('o1')
    expect(text).not.toContain('o2')
  })

  it('renders NIP codes in the table', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('NIP001')
    expect(text).toContain('NIP002')
  })

  it('renders avatar initials (AP for Andi Pratama, BL for Bunga Lestari)', async () => {
    const wrapper = await mountAndWait()
    const html = wrapper.html()
    expect(html).toContain('AP')
    expect(html).toContain('BL')
  })

  it('renders contact email and phone in the table', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('andi@inventra.go.id')
    expect(text).toContain('0812-1111-2222')
  })

  it('renders Aktif status badge for active employees', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Aktif')
  })

  it('renders Ditangguhkan status label for suspended employee', async () => {
    const wrapper = await mountAndWait()
    // Citra Dewi has status=suspended → i18n key masterdata.employees.status.suspended = "Ditangguhkan"
    expect(wrapper.text()).toContain('Ditangguhkan')
  })

  it('renders filter dropdowns with i18n labels', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Semua Kantor')
    expect(text).toContain('Semua Departemen')
    expect(text).toContain('Semua Jabatan')
    expect(text).toContain('Semua Status')
  })
})

// ---------------------------------------------------------------------------
// Filter: office
// ---------------------------------------------------------------------------

describe('Master Pegawai page — office filter', () => {
  it('filterOffice=o1 shows only o1 employees and hides o2 employees', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'filterOffice', 'o1')

    const text = wrapper.text()
    // Andi Pratama and Citra Dewi belong to o1
    expect(text).toContain('Andi Pratama')
    expect(text).toContain('Citra Dewi')
    // Bunga Lestari belongs to o2 — must be absent
    expect(text).not.toContain('Bunga Lestari')
  })

  it('filterOffice=o2 shows only o2 employees and hides o1 employees', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'filterOffice', 'o2')

    const text = wrapper.text()
    expect(text).toContain('Bunga Lestari')
    expect(text).not.toContain('Andi Pratama')
    expect(text).not.toContain('Citra Dewi')
  })
})

// ---------------------------------------------------------------------------
// Filter: department
// ---------------------------------------------------------------------------

describe('Master Pegawai page — department filter', () => {
  it('filterDept=d2 (Keuangan) shows only Keuangan employees', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'filterDept', 'd2')

    const text = wrapper.text()
    // Only Bunga Lestari is in Keuangan (d2)
    expect(text).toContain('Bunga Lestari')
    // Andi Pratama (d1) and Citra Dewi (d1) must be absent
    expect(text).not.toContain('Andi Pratama')
    expect(text).not.toContain('Citra Dewi')
  })

  it('filterDept=d1 (Umum) shows only Umum employees', async () => {
    const wrapper = await mountAndWait()
    await setVmRef(wrapper, 'filterDept', 'd1')

    const text = wrapper.text()
    expect(text).toContain('Andi Pratama')
    expect(text).toContain('Citra Dewi')
    expect(text).not.toContain('Bunga Lestari')
  })
})

// ---------------------------------------------------------------------------
// Load error state
// ---------------------------------------------------------------------------

describe('Master Pegawai page — load error', () => {
  it('shows error message when GET /employees fails', async () => {
    setHandler((path) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path.startsWith('/employees')) {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // i18n: masterdata.employees.loadError = "Gagal memuat pegawai."
    expect(text).toContain('Gagal memuat pegawai.')
    expect(text).not.toContain('Andi Pratama')
  })

  it('shows retry button on load error', async () => {
    setHandler((path) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path.startsWith('/employees')) {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    // i18n: common.retry = "Coba lagi"
    expect(wrapper.text()).toContain('Coba lagi')
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
  })

  it('retry button re-fetches and recovers when second call succeeds', async () => {
    let callCount = 0
    setHandler((path) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path.startsWith('/employees')) {
        callCount++
        if (callCount === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return makeEmployeesResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat pegawai.')

    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Andi Pratama')
    expect(wrapper.text()).not.toContain('Gagal memuat pegawai.')
  })
})

// ---------------------------------------------------------------------------
// Create form
// ---------------------------------------------------------------------------

describe('Master Pegawai page — create form', () => {
  it('openCreate sets formOpen=true', async () => {
    const wrapper = await mountAndWait()
    const vm = wrapper.vm as unknown as { formOpen: boolean, openCreate: () => void }
    vm.openCreate()
    await wrapper.vm.$nextTick()
    expect(vm.formOpen).toBe(true)
  })

  it('POST /employees body contains code, name, office_id, department_id, position_id with UUID values', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path === '/employees' && opts?.method === 'POST') {
        capturedPath = path
        capturedOpts = opts
        return { ...EMPLOYEES[0]!, id: 'emp-new' }
      }
      if (path.startsWith('/employees') && (!opts?.method || opts.method === 'GET')) {
        return makeEmployeesResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    const vm = wrapper.vm as unknown as Record<string, unknown>
    const form = vm['form'] as Record<string, unknown>
    form['code'] = 'NIP099'
    form['name'] = 'Dono Saputra'
    form['office_id'] = 'o1'
    form['department_id'] = 'd1'
    form['position_id'] = 'p2'
    form['status'] = 'active'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/employees')
    const body = capturedOpts['body'] as Record<string, unknown>
    // UUID values sent — not resolved names
    expect(body['code']).toBe('NIP099')
    expect(body['name']).toBe('Dono Saputra')
    expect(body['office_id']).toBe('o1')
    expect(body['department_id']).toBe('d1')
    expect(body['position_id']).toBe('p2')
    expect(body['status']).toBe('active')
    // Names must NOT appear in body keys
    expect(body['office_id']).not.toBe('Kantor Pusat')
    expect(body['department_id']).not.toBe('Umum')
    expect(body['position_id']).not.toBe('Manajer')
  })

  it('required guard: empty code → no POST sent', async () => {
    let postCalled = false

    setHandler((path, opts) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path === '/employees' && opts?.method === 'POST') {
        postCalled = true
        return EMPLOYEES[0]
      }
      if (path.startsWith('/employees') && (!opts?.method || opts.method === 'GET')) {
        return makeEmployeesResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    // Leave form.code empty, but set other required fields
    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    form['code'] = '' // deliberately empty
    form['name'] = 'Someone'
    form['office_id'] = 'o1'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))

    expect(postCalled).toBe(false)
  })

  it('required guard: empty name → no POST sent', async () => {
    let postCalled = false

    setHandler((path, opts) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path === '/employees' && opts?.method === 'POST') {
        postCalled = true
        return EMPLOYEES[0]
      }
      if (path.startsWith('/employees') && (!opts?.method || opts.method === 'GET')) {
        return makeEmployeesResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    form['code'] = 'NIP099'
    form['name'] = '' // deliberately empty
    form['office_id'] = 'o1'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))

    expect(postCalled).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Edit form
// ---------------------------------------------------------------------------

describe('Master Pegawai page — edit form', () => {
  it('openEdit pre-fills form with row values', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openEdit: (row: unknown) => void }).openEdit(EMPLOYEES[0])
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    expect(form['code']).toBe('NIP001')
    expect(form['name']).toBe('Andi Pratama')
    expect(form['office_id']).toBe('o1')
    expect(form['department_id']).toBe('d1')
    expect(form['position_id']).toBe('p1')
    expect(form['status']).toBe('active')
  })

  it('PUT /employees/:id body on submit with updated values', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path.startsWith('/employees/') && opts?.method === 'PUT') {
        capturedPath = path
        capturedOpts = opts
        return { ...EMPLOYEES[0]!, name: 'Andi Updated' }
      }
      if (path.startsWith('/employees') && (!opts?.method || opts.method === 'GET')) {
        return makeEmployeesResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openEdit: (row: unknown) => void }).openEdit(EMPLOYEES[0])
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    form['name'] = 'Andi Updated'
    form['status'] = 'inactive'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/employees/emp1')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['name']).toBe('Andi Updated')
    expect(body['code']).toBe('NIP001')
    expect(body['office_id']).toBe('o1')
    expect(body['status']).toBe('inactive')
  })
})

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

describe('Master Pegawai page — delete', () => {
  it('DELETE /employees/:id issued after confirmation', async () => {
    let deletedPath = ''

    setHandler((path, opts) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path.startsWith('/employees/') && opts?.method === 'DELETE') {
        deletedPath = path
        return undefined
      }
      if (path.startsWith('/employees') && (!opts?.method || opts.method === 'GET')) {
        return makeEmployeesResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: unknown) => Promise<void> }).onDelete(EMPLOYEES[1])
    await wrapper.vm.$nextTick()

    useConfirm().resolve(true)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(deletedPath).toBe('/employees/emp2')
  })

  it('no DELETE issued when confirm is cancelled', async () => {
    let deleteCalled = false

    setHandler((path, opts) => {
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/departments')) return makeRefResponse(DEPARTMENTS)
      if (path.startsWith('/positions')) return makeRefResponse(POSITIONS)
      if (path.startsWith('/employees/') && opts?.method === 'DELETE') {
        deleteCalled = true
        return undefined
      }
      if (path.startsWith('/employees') && (!opts?.method || opts.method === 'GET')) {
        return makeEmployeesResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: unknown) => Promise<void> }).onDelete(EMPLOYEES[0])
    await wrapper.vm.$nextTick()

    useConfirm().resolve(false)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))

    expect(deleteCalled).toBe(false)
  })
})
