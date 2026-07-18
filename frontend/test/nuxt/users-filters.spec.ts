// @vitest-environment nuxt
//
// Task 7 — User Management filter bar (role / office / status). Covers the
// frontend half of Task 5's server-side GET /users?role_id&office_id&status
// filters: selecting a value in the role/office/status filter controls must
// issue a GET /users call carrying the matching query param and must reset
// offset (pagination) to 0. Mirrors the pattern in settings-audit.spec.ts —
// USelect is a reka-ui portal-rendered listbox that JSDOM can't drive via
// PointerEvents reliably, so filter selection is simulated by writing the
// page's reactive filter ref directly via wrapper.vm (script-setup refs are
// exposed as plain properties on the component proxy), which is exactly what
// a real pick does under the hood (fires the same watcher synchronously).
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import type { UserView } from '~/composables/api/useUsers'
import UsersPage from '~/pages/settings/users.vue'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request are intercepted here.
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
  { id: 'e1', name: 'Budi', office_id: 'o1' }
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
    has_avatar: false,
    google_linked: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
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
  if (/^\/offices\/[^/?]+$/.test(path)) return OFFICES.find(o => o.id === path.split('/')[2]) ?? null
  if (path.startsWith('/offices')) return { data: OFFICES }
  if (/^\/employees\/[^/?]+$/.test(path)) return EMPLOYEES.find(e => e.id === path.split('/')[2]) ?? null
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
afterEach(() => {
  vi.useRealTimers()
})

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

async function setVmRef(wrapper: Awaited<ReturnType<typeof mountAndWait>>, key: string, value: unknown) {
  ;(wrapper.vm as unknown as Record<string, unknown>)[key] = value
  await wrapper.vm.$nextTick()
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
}

function captureUserQueries(): Array<Record<string, string>> {
  const captured: Array<Record<string, string>> = []
  setHandler((path, opts) => {
    if (path === '/authz/roles') return { data: ROLES }
    if (/^\/offices\/[^/?]+$/.test(path)) return OFFICES.find(o => o.id === path.split('/')[2]) ?? null
    if (path.startsWith('/offices')) return { data: OFFICES }
    if (/^\/employees\/[^/?]+$/.test(path)) return EMPLOYEES.find(e => e.id === path.split('/')[2]) ?? null
    if (path.startsWith('/employees')) return { data: EMPLOYEES }
    if (path.startsWith('/users') && (!opts?.method || opts.method === 'GET')) {
      captured.push(parseQuery(path))
      return makeUsersResponse()
    }
    throw new Error(`Unhandled: ${path}`)
  })
  return captured
}

// ---------------------------------------------------------------------------
// Role filter
// ---------------------------------------------------------------------------

describe('User Management page — role filter', () => {
  it('selecting a role triggers GET /users with role_id and resets offset to 0', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()
    captured.length = 0

    await setVmRef(wrapper, 'fRole', 'r1')

    const filtered = captured.find(q => q['role_id'] === 'r1')
    expect(filtered).toBeDefined()
    expect(filtered!['offset']).toBe('0')
  })

  it('clearing the role filter (back to __all__) omits role_id', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()

    await setVmRef(wrapper, 'fRole', 'r1')
    captured.length = 0

    await setVmRef(wrapper, 'fRole', '__all__')

    const cleared = captured[captured.length - 1]
    expect(cleared).toBeDefined()
    expect(cleared!['role_id']).toBeUndefined()
  })

  it('role filter options are built from lookups() role names', async () => {
    const wrapper = await mountAndWait()
    const options = (wrapper.vm as unknown as Record<string, unknown>)['roleFilterOptions'] as Array<{ value: string, label: string }>
    expect(options.some(o => o.label === 'Manager' && o.value === 'r1')).toBe(true)
    expect(options.some(o => o.label === 'Operator' && o.value === 'r2')).toBe(true)
    // First option is the "all roles" clear option
    expect(options[0]?.value).toBe('__all__')
  })
})

// ---------------------------------------------------------------------------
// Status filter
// ---------------------------------------------------------------------------

describe('User Management page — status filter', () => {
  it('selecting a status triggers GET /users with status and resets offset to 0', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()
    captured.length = 0

    await setVmRef(wrapper, 'fStatus', 'suspended')

    const filtered = captured.find(q => q['status'] === 'suspended')
    expect(filtered).toBeDefined()
    expect(filtered!['offset']).toBe('0')
  })

  it('clearing the status filter (back to __all__) omits status', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()

    await setVmRef(wrapper, 'fStatus', 'inactive')
    captured.length = 0

    await setVmRef(wrapper, 'fStatus', '__all__')

    const cleared = captured[captured.length - 1]
    expect(cleared).toBeDefined()
    expect(cleared!['status']).toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// Office filter
// ---------------------------------------------------------------------------

describe('User Management page — office filter', () => {
  it('selecting an office triggers GET /users with office_id and resets offset to 0', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()
    captured.length = 0

    await setVmRef(wrapper, 'fOffice', 'o2')

    const filtered = captured.find(q => q['office_id'] === 'o2')
    expect(filtered).toBeDefined()
    expect(filtered!['offset']).toBe('0')
  })

  it('clearing the office filter (back to null) omits office_id', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()

    await setVmRef(wrapper, 'fOffice', 'o2')
    captured.length = 0

    await setVmRef(wrapper, 'fOffice', null)

    const cleared = captured[captured.length - 1]
    expect(cleared).toBeDefined()
    expect(cleared!['office_id']).toBeUndefined()
  })

  it('renders the office filter as an AsyncSearchPicker with a clear control', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.find('[data-testid="users-filter-office-picker-input"]').exists()).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Combined filters + offset (pagination interaction)
// ---------------------------------------------------------------------------

describe('User Management page — combined filters', () => {
  it('a filter change while on a later page resets offset back to 0 in the request', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()

    // Move to "page 2" first
    ;(wrapper.vm as unknown as Record<string, unknown>)['offset'] = 10
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 400))
    captured.length = 0

    await setVmRef(wrapper, 'fStatus', 'active')

    const filtered = captured.find(q => q['status'] === 'active')
    expect(filtered).toBeDefined()
    expect(filtered!['offset']).toBe('0')
  })

  it('search + role + status + office can all be present in the same request', async () => {
    const captured = captureUserQueries()
    const wrapper = await mountAndWait()

    await setVmRef(wrapper, 'search', 'andi')
    await setVmRef(wrapper, 'fRole', 'r1')
    await setVmRef(wrapper, 'fStatus', 'active')
    await setVmRef(wrapper, 'fOffice', 'o1')

    const last = captured[captured.length - 1]
    expect(last).toBeDefined()
    expect(last!['search']).toBe('andi')
    expect(last!['role_id']).toBe('r1')
    expect(last!['status']).toBe('active')
    expect(last!['office_id']).toBe('o1')
  })
})

// ---------------------------------------------------------------------------
// Reset
// ---------------------------------------------------------------------------

describe('User Management page — filter reset', () => {
  it('resetFilters clears search/role/office/status back to defaults', async () => {
    const wrapper = await mountAndWait()

    await setVmRef(wrapper, 'search', 'andi')
    await setVmRef(wrapper, 'fRole', 'r1')
    await setVmRef(wrapper, 'fStatus', 'active')
    await setVmRef(wrapper, 'fOffice', 'o1')

    await (wrapper.vm as unknown as { resetFilters: () => void }).resetFilters()
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 400))

    const vm = wrapper.vm as unknown as Record<string, unknown>
    expect(vm['search']).toBe('')
    expect(vm['fRole']).toBe('__all__')
    expect(vm['fStatus']).toBe('__all__')
    expect(vm['fOffice']).toBe(null)
  })

  it('shows a reset button only when a filter is active', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.find('[data-testid="users-filter-reset"]').exists()).toBe(false)

    await setVmRef(wrapper, 'fStatus', 'active')
    expect(wrapper.find('[data-testid="users-filter-reset"]').exists()).toBe(true)
  })
})
