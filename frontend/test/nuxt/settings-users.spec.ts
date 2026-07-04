// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { useConfirm } from '~/composables/useConfirm'
import type { UserView } from '~/composables/api/useUsers'
import UsersPage from '~/pages/settings/users.vue'

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
// Shared fixtures
// ---------------------------------------------------------------------------

const ROLES = [
  { id: 'r1', name: 'Manager' },
  { id: 'r2', name: 'Operator' }
]

const OFFICES = [
  { id: 'o1', name: 'Pusat' },
  { id: 'o2', name: 'Cabang' }
]

const EMPLOYEES = [
  { id: 'e1', name: 'Budi', office_id: 'o1' },
  { id: 'e2', name: 'Sari', office_id: 'o2' }
]

const USERS: UserView[] = [
  {
    id: 'u1',
    name: 'Andi Saputra',
    email: 'andi@inventra.go.id',
    role_id: 'r1',
    office_id: 'o1',
    employee_id: 'e1',
    status: 'active',
    avatar_url: null,
    google_linked: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  },
  {
    id: 'u2',
    name: 'Dewi Rahayu',
    email: 'dewi@inventra.go.id',
    role_id: 'r2',
    office_id: 'o2',
    employee_id: 'e2',
    status: 'suspended',
    avatar_url: null,
    google_linked: true,
    created_at: '2026-01-02T00:00:00Z',
    updated_at: '2026-01-02T00:00:00Z'
  }
]

function makeUsersResponse(rows: UserView[] = USERS, total: number = USERS.length) {
  return { data: rows, total }
}

// Parse query parameters from a path string like /users?limit=10&offset=0
function parseQuery(path: string): Record<string, string> {
  const qIdx = path.indexOf('?')
  if (qIdx === -1) return {}
  const params = new URLSearchParams(path.slice(qIdx + 1))
  const result: Record<string, string> = {}
  params.forEach((val, key) => {
    result[key] = val
  })
  return result
}

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path === '/authz/roles') return { data: ROLES }
  if (path.startsWith('/offices')) return { data: OFFICES }
  if (path.startsWith('/employees')) return { data: EMPLOYEES }
  if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
    return makeUsersResponse()
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
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

beforeEach(() => {
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(UsersPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

// Helper — set a page-level reactive ref exposed via vm and wait for watchers to settle.
async function setVmRef(wrapper: Awaited<ReturnType<typeof mountAndWait>>, key: string, value: unknown) {
  ;(wrapper.vm as unknown as Record<string, unknown>)[key] = value
  await wrapper.vm.$nextTick()
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
}

// ---------------------------------------------------------------------------
// Loaded rows — resolved names and badges
// ---------------------------------------------------------------------------

describe('User Management page — loaded rows', () => {
  it('renders page title and subtitle', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Pengguna')
    expect(text).toContain('Kelola akun login')
  })

  it('renders user names and emails', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Andi Saputra')
    expect(text).toContain('andi@inventra.go.id')
    expect(text).toContain('Dewi Rahayu')
    expect(text).toContain('dewi@inventra.go.id')
  })

  it('resolves role_id to role name (Manager not UUID)', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // r1 → Manager, r2 → Operator (resolved from lookups)
    expect(text).toContain('Manager')
    expect(text).toContain('Operator')
    expect(text).not.toContain('r1')
    expect(text).not.toContain('r2')
  })

  it('resolves office_id to office name (Pusat not UUID)', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Pusat')
    expect(text).toContain('Cabang')
    // UUIDs must not appear as rendered text (resolved to names in table cells)
    expect(text).not.toContain('o1')
    expect(text).not.toContain('o2')
  })

  it('resolves employee_id to employee name (Budi not UUID)', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Budi')
    expect(text).toContain('Sari')
    // UUIDs must not appear as rendered text (resolved to names in table cells)
    expect(text).not.toContain('e1')
    expect(text).not.toContain('e2')
  })

  it('renders login badge: Email for non-google, Google for google_linked', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // Andi is not google_linked → "Email"
    expect(text).toContain('Email')
    // Dewi is google_linked → "Google"
    expect(text).toContain('Google')
  })

  it('renders status badges: Aktif and Disuspend', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Aktif') // Andi active
    expect(text).toContain('Disuspend') // Dewi suspended
  })

  it('shows em-dash for users with no employee linked', async () => {
    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse([{ ...USERS[0]!, employee_id: null }], 1)
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('—')
  })

  it('initial GET /users uses limit=10 and offset=0', async () => {
    const capturedQueries: Array<Record<string, string>> = []
    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        capturedQueries.push(parseQuery(path))
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    await mountAndWait()
    const initial = capturedQueries[0]
    expect(initial?.['limit']).toBe('10')
    expect(initial?.['offset']).toBe('0')
  })
})

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

describe('User Management page — search', () => {
  it('search input triggers GET /users with search param and offset=0', async () => {
    const capturedPaths: string[] = []
    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        capturedPaths.push(path)
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    capturedPaths.length = 0

    await setVmRef(wrapper, 'search', 'andi')

    const searchCall = capturedPaths.find(p => p.includes('search=andi'))
    expect(searchCall).toBeDefined()
    const q = parseQuery(searchCall!)
    expect(q['search']).toBe('andi')
    expect(q['offset']).toBe('0')
  })
})

// ---------------------------------------------------------------------------
// Load error state
// ---------------------------------------------------------------------------

describe('User Management page — load error', () => {
  it('shows error message when GET /users returns 500', async () => {
    setHandler((path) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users')) {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // i18n: settings.users.loadError
    expect(text).toContain('Gagal memuat data user.')
    expect(text).not.toContain('Andi Saputra')
  })

  it('shows retry button on load error', async () => {
    setHandler((path) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users')) {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    // i18n: settings.users.retry
    expect(wrapper.text()).toContain('Coba lagi')
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
  })

  it('retry button re-fetches and recovers when second call succeeds', async () => {
    let callCount = 0
    setHandler((path) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users')) {
        callCount++
        if (callCount === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat data user.')

    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Andi Saputra')
    expect(wrapper.text()).not.toContain('Gagal memuat data user.')
  })
})

// ---------------------------------------------------------------------------
// Create form
// ---------------------------------------------------------------------------

describe('User Management page — create form', () => {
  it('captures POST /users body with name, email, role_id, office_id, employee_id', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path === '/users' && opts?.method === 'POST') {
        capturedPath = path
        capturedOpts = opts
        return { ...USERS[0]!, id: 'u-new' }
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    // Open the create form via wrapper.vm
    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    // Set form fields directly via vm
    const vm = wrapper.vm as unknown as Record<string, unknown>
    const form = vm['form'] as Record<string, unknown>
    form['name'] = 'Citra Dewi'
    form['email'] = 'citra@inventra.go.id'
    form['role_id'] = 'r1'
    form['office_id'] = 'o1'
    form['employee_id'] = 'e1'
    await wrapper.vm.$nextTick()

    // Call onSubmit directly
    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/users')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['name']).toBe('Citra Dewi')
    expect(body['email']).toBe('citra@inventra.go.id')
    expect(body['role_id']).toBe('r1')
    expect(body['office_id']).toBe('o1')
    expect(body['employee_id']).toBe('e1')
    // password blank → omitted from body
    expect(body['password']).toBeUndefined()
  })

  it('409 conflict shows inline email conflict error', async () => {
    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path === '/users' && opts?.method === 'POST') {
        throw Object.assign(new Error('Conflict'), { statusCode: 409 })
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as Record<string, unknown>
    const form = vm['form'] as Record<string, unknown>
    form['name'] = 'Citra Dewi'
    form['email'] = 'andi@inventra.go.id' // duplicate email
    form['role_id'] = 'r1'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    // The inline error is set on errors.email (internal state check)
    const errors = vm['errors'] as Record<string, string | undefined>
    expect(errors['email']).toBe('Email sudah dipakai.')
    // The error must also be rendered in the DOM so the user actually sees it.
    // USlideover teleports to document.body, so query there rather than wrapper.text().
    expect(document.body.textContent).toContain('Email sudah dipakai.')
  })

  it('employee picker filtered by selected office: o1 → only Budi, not Sari', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    // Set office_id to o1 via vm
    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    form['office_id'] = 'o1'
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as Record<string, unknown>
    const employeeFormOptions = vm['employeeFormOptions'] as Array<{ value: string, label: string }>

    // Only Budi (o1) should be in the options, not Sari (o2)
    expect(employeeFormOptions.some(o => o.label === 'Budi')).toBe(true)
    expect(employeeFormOptions.some(o => o.label === 'Sari')).toBe(false)
  })

  it('switching office clears previously selected employee', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>

    // Select office o1 and employee e1
    form['office_id'] = 'o1'
    await wrapper.vm.$nextTick()
    form['employee_id'] = 'e1'
    await wrapper.vm.$nextTick()
    expect(form['employee_id']).toBe('e1')

    // Switch to office o2 — watcher should clear employee_id since e1 is not in o2
    form['office_id'] = 'o2'
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 50))
    await wrapper.vm.$nextTick()

    expect(form['employee_id']).toBe('')
  })

  it('employee picker switches to o2 employees after office change', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    form['office_id'] = 'o2'
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as Record<string, unknown>
    const employeeFormOptions = vm['employeeFormOptions'] as Array<{ value: string, label: string }>

    expect(employeeFormOptions.some(o => o.label === 'Sari')).toBe(true)
    expect(employeeFormOptions.some(o => o.label === 'Budi')).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Edit form
// ---------------------------------------------------------------------------

describe('User Management page — edit form', () => {
  it('captures PUT /users/:id body with name, role_id, status (no email/password)', async () => {
    let capturedPath = ''
    let capturedOpts: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users/') && opts?.method === 'PUT') {
        capturedPath = path
        capturedOpts = opts
        return { ...USERS[0]!, name: 'Andi Updated' }
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    // Open edit for u1
    ;(wrapper.vm as unknown as { openEdit: (row: unknown) => void }).openEdit(USERS[0])
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))

    const vm = wrapper.vm as unknown as Record<string, unknown>
    const form = vm['form'] as Record<string, unknown>
    form['name'] = 'Andi Updated'
    form['status'] = 'inactive'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/users/u1')
    const body = capturedOpts['body'] as Record<string, unknown>
    expect(body['name']).toBe('Andi Updated')
    expect(body['role_id']).toBe('r1')
    expect(body['status']).toBe('inactive')
    // optional FK fields from the pre-filled row must be forwarded
    expect(body['office_id']).toBe('o1')
    expect(body['employee_id']).toBe('e1')
    // no email or password in PUT body
    expect(body['email']).toBeUndefined()
    expect(body['password']).toBeUndefined()
  })

  it('edit form pre-fills name, role, office, employee from the row', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openEdit: (row: unknown) => void }).openEdit(USERS[0])
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    expect(form['name']).toBe('Andi Saputra')
    expect(form['role_id']).toBe('r1')
    expect(form['office_id']).toBe('o1')
    expect(form['employee_id']).toBe('e1')
    expect(form['status']).toBe('active')
  })
})

// ---------------------------------------------------------------------------
// Status toggle row action
// ---------------------------------------------------------------------------

describe('User Management page — status toggle', () => {
  it('onToggleStatus issues PUT with toggled status (active → inactive)', async () => {
    let capturedPath = ''
    let capturedBody: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users/') && opts?.method === 'PUT') {
        capturedPath = path
        capturedBody = (opts['body'] as Record<string, unknown>) ?? {}
        return { ...USERS[0]!, status: 'inactive' }
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    // Call onToggleStatus directly for u1 (active → inactive)
    await (wrapper.vm as unknown as { onToggleStatus: (row: unknown) => Promise<void> }).onToggleStatus(USERS[0])
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedPath).toBe('/users/u1')
    expect(capturedBody['status']).toBe('inactive')
    expect(capturedBody['name']).toBe('Andi Saputra')
    expect(capturedBody['role_id']).toBe('r1')
  })

  it('onToggleStatus issues PUT with active for suspended user', async () => {
    let capturedBody: Record<string, unknown> = {}

    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users/') && opts?.method === 'PUT') {
        capturedBody = (opts['body'] as Record<string, unknown>) ?? {}
        return { ...USERS[1]!, status: 'active' }
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    // u2 is suspended → toggling should flip to active
    await (wrapper.vm as unknown as { onToggleStatus: (row: unknown) => Promise<void> }).onToggleStatus(USERS[1])
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(capturedBody['status']).toBe('active')
  })
})

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

describe('User Management page — delete', () => {
  it('onDelete issues DELETE /users/:id after confirmation', async () => {
    let deletedPath = ''

    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users/') && opts?.method === 'DELETE') {
        deletedPath = path
        return undefined
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    // Trigger onDelete — it opens the confirm dialog and awaits resolution
    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: unknown) => Promise<void> }).onDelete(USERS[0])
    await wrapper.vm.$nextTick()

    // The confirm dialog is now open; resolve it affirmatively via useConfirm().resolve()
    useConfirm().resolve(true)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    expect(deletedPath).toBe('/users/u1')
  })

  it('onDelete does not call DELETE when user cancels confirm', async () => {
    let deleteCalled = false

    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users/') && opts?.method === 'DELETE') {
        deleteCalled = true
        return undefined
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    const deletePromise = (wrapper.vm as unknown as { onDelete: (row: unknown) => Promise<void> }).onDelete(USERS[0])
    await wrapper.vm.$nextTick()

    // Cancel the confirm dialog
    useConfirm().resolve(false)
    await deletePromise
    await new Promise(r => setTimeout(r, 200))

    expect(deleteCalled).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

describe('User Management page — form validation', () => {
  it('validate() requires name, email (create), and role_id', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    // Submit with empty form
    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await wrapper.vm.$nextTick()

    const errors = (wrapper.vm as unknown as Record<string, unknown>)['errors'] as Record<string, string | undefined>
    expect(errors['name']).toBe('Wajib diisi.')
    expect(errors['email']).toBe('Wajib diisi.')
    expect(errors['role_id']).toBe('Wajib diisi.')
  })

  it('validate() rejects invalid email format', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openCreate: () => void }).openCreate()
    await wrapper.vm.$nextTick()

    const form = (wrapper.vm as unknown as Record<string, unknown>)['form'] as Record<string, unknown>
    form['name'] = 'Someone'
    form['email'] = 'not-valid'
    form['role_id'] = 'r1'
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await wrapper.vm.$nextTick()

    const errors = (wrapper.vm as unknown as Record<string, unknown>)['errors'] as Record<string, string | undefined>
    expect(errors['email']).toBe('Format email tidak valid.')
  })

  it('validate() does not require email in edit mode', async () => {
    let postCalled = false
    let putCalled = false

    setHandler((path, opts) => {
      if (path === '/authz/roles') return { data: ROLES }
      if (path.startsWith('/offices')) return { data: OFFICES }
      if (path.startsWith('/employees')) return { data: EMPLOYEES }
      if (path.startsWith('/users/') && opts?.method === 'PUT') {
        putCalled = true
        return { ...USERS[0]! }
      }
      if (path === '/users' && opts?.method === 'POST') {
        postCalled = true
        return { ...USERS[0]! }
      }
      if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
        return makeUsersResponse()
      }
      throw new Error(`Unhandled: ${path} ${opts?.method}`)
    })

    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as { openEdit: (row: unknown) => void }).openEdit(USERS[0])
    await wrapper.vm.$nextTick()

    await (wrapper.vm as unknown as { onSubmit: () => Promise<void> }).onSubmit()
    await new Promise(r => setTimeout(r, 200))
    await wrapper.vm.$nextTick()

    // PUT should have been called, not POST
    expect(putCalled).toBe(true)
    expect(postCalled).toBe(false)
  })
})
