// @vitest-environment nuxt
// Page-level contract for the compact (mobile) user list — the mirror of
// assets-index-compact.spec.ts. This page received a rewiring as large as the
// catalog's but had no page-level spec of its own, so its three-branch mutation
// reload and its fetch accounting went unverified.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { ref } from 'vue'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

let _handler: RequestHandler = () => {
  throw new Error('No handler set')
}

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => Promise.resolve(_handler(path, opts))
  })
}))

const compact = ref(true)
mockNuxtImport('useIsCompact', () => () => compact)

const confirmMock = vi.fn(async () => true)
mockNuxtImport('useConfirm', () => () => ({ open: confirmMock }))

// eslint-disable-next-line import/first
import UsersPage from '~/pages/settings/users.vue'

// Big enough that the list can run past the 100-row server page cap.
let total = 450

function makeUsers(offset: number, limit: number) {
  const count = Math.max(0, Math.min(limit, total - offset))
  return {
    data: Array.from({ length: count }, (_, i) => ({
      id: `u${offset + i}`,
      name: `User ${offset + i}`,
      email: `user${offset + i}@test.local`,
      role_id: 'r1',
      office_id: null,
      employee_id: null,
      status: 'active',
      login_method: 'email'
    })),
    total
  }
}

const userCalls: string[] = []
const writeCalls: { path: string, method: string }[] = []

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  const method = String((opts as { method?: string })?.method ?? 'GET').toUpperCase()
  if (path.startsWith('/users')) {
    if (method !== 'GET') {
      writeCalls.push({ path, method })
      return { id: 'u0', name: 'User 0', email: 'user0@test.local', role_id: 'r1', office_id: null, employee_id: null, status: 'inactive', login_method: 'email' }
    }
    userCalls.push(path)
    const q = new URLSearchParams(path.split('?')[1] ?? '')
    return makeUsers(Number(q.get('offset') ?? '0'), Number(q.get('limit') ?? '10'))
  }
  if (path.startsWith('/roles')) return { data: [{ id: 'r1', name: 'Staf' }], total: 1, limit: 100, offset: 0 }
  if (path.startsWith('/offices')) return { data: [], total: 0, limit: 100, offset: 0 }
  if (path.startsWith('/employees')) return { data: [], total: 0, limit: 100, offset: 0 }
  throw new Error(`Unhandled request: ${path} ${JSON.stringify(opts)}`)
}

enableAutoUnmount(afterEach)

beforeEach(() => {
  userCalls.length = 0
  writeCalls.length = 0
  total = 450
  compact.value = true
  _handler = defaultHandler
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
})

async function mountUsers() {
  const wrapper = await mountSuspended(UsersPage)
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  return wrapper
}

async function settle(wrapper: Awaited<ReturnType<typeof mountUsers>>) {
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
}

type Vm = {
  fStatus: string
  search: string
  offset: number
  rows: unknown[]
  reloadAfterMutation: () => Promise<void> | void
}

/** Seeds the list with `n` rows without going through the network. */
function hydrate(wrapper: Awaited<ReturnType<typeof mountUsers>>, n: number) {
  const list = (wrapper.vm as unknown as {
    list: { hydrate: (rows: unknown[], total: number) => void }
  }).list
  list.hydrate(makeUsers(0, n).data, total)
}

function limitOf(path: string): number {
  return Number(new URLSearchParams(path.split('?')[1] ?? '').get('limit') ?? '0')
}

describe('Users (compact) — fetch accounting', () => {
  it('loads exactly once on mount', async () => {
    await mountUsers()
    expect(userCalls).toHaveLength(1)
    expect(userCalls[0]).toContain('offset=0')
  })

  it('issues exactly ONE fetch when a filter changes', async () => {
    const w = await mountUsers()
    userCalls.length = 0
    ;(w.vm as unknown as Vm).fStatus = 'inactive'
    await settle(w)
    expect(userCalls).toHaveLength(1)
  })

  it('issues exactly ONE fetch when a filter changes from a non-zero offset', async () => {
    compact.value = false
    const w = await mountUsers()
    ;(w.vm as unknown as Vm).offset = 30
    await settle(w)

    userCalls.length = 0
    ;(w.vm as unknown as Vm).fStatus = 'inactive'
    await settle(w)
    expect(userCalls).toHaveLength(1)
    expect(userCalls[0]).toContain('offset=0')
  })

  it('issues exactly ONE fetch when the breakpoint is crossed from a non-zero offset', async () => {
    compact.value = false
    const w = await mountUsers()
    ;(w.vm as unknown as Vm).offset = 30
    await settle(w)

    userCalls.length = 0
    compact.value = true
    await settle(w)
    expect(userCalls).toHaveLength(1)
    expect(userCalls[0]).toContain('offset=0')
  })

  it('issues exactly ONE fetch when the search term changes', async () => {
    const w = await mountUsers()
    userCalls.length = 0
    ;(w.vm as unknown as Vm).search = 'budi'
    await settle(w)
    expect(userCalls).toHaveLength(1)
    expect(userCalls[0]).toContain('search=budi')
  })
})

// ---------------------------------------------------------------------------
// reloadAfterMutation has three branches and three call sites. Before this
// spec, none of them were verified at any level.
// ---------------------------------------------------------------------------
describe('Users (compact) — reload after a mutation', () => {
  it('refetches one page at the paged layout', async () => {
    compact.value = false
    const w = await mountUsers()
    ;(w.vm as unknown as Vm).offset = 20
    await settle(w)

    userCalls.length = 0
    await (w.vm as unknown as Vm).reloadAfterMutation()
    await settle(w)
    expect(userCalls).toHaveLength(1)
    expect(userCalls[0]).toContain('offset=20')
    expect(limitOf(userCalls[0]!)).toBe(10)
  })

  it('refetches every accumulated row while the list still fits one server page', async () => {
    const w = await mountUsers()
    const vm = w.vm as unknown as Vm
    // 10 -> 30 rows.
    await (w.vm as unknown as { list: { loadMore: () => Promise<void> } }).list.loadMore()
    await (w.vm as unknown as { list: { loadMore: () => Promise<void> } }).list.loadMore()
    await settle(w)
    expect(vm.rows).toHaveLength(30)

    userCalls.length = 0
    await vm.reloadAfterMutation()
    await settle(w)
    expect(userCalls).toHaveLength(1)
    expect(limitOf(userCalls[0]!)).toBe(30)
    expect(vm.rows).toHaveLength(30)
  })

  // Past the server's 100-row page cap the list cannot be preserved, so it
  // deliberately reloads from the top rather than silently losing the overflow.
  it('reloads from the top once the list outgrows one server page', async () => {
    const w = await mountUsers()
    const vm = w.vm as unknown as Vm
    // Seed 120 rows directly rather than issuing 12 appends: this asserts the
    // branch chosen at that list length, not the accumulation that got there,
    // and 120 rendered table rows per append makes the loop needlessly slow.
    hydrate(w, 120)
    await settle(w)
    expect(vm.rows).toHaveLength(120)

    userCalls.length = 0
    await vm.reloadAfterMutation()
    await settle(w)
    expect(userCalls).toHaveLength(1)
    expect(limitOf(userCalls[0]!)).toBe(10)
    expect(vm.rows).toHaveLength(10)
  })

  // Regression: the >100 branch used to return void, so `await` resumed before
  // the rows were fresh.
  it('is awaitable on every branch', async () => {
    const w = await mountUsers()
    const vm = w.vm as unknown as Vm
    hydrate(w, 120)
    await settle(w)

    total = 40
    userCalls.length = 0
    await vm.reloadAfterMutation()
    // Rows already reflect the reload the moment the await resolves.
    expect(userCalls).toHaveLength(1)
    expect(vm.rows).toHaveLength(10)
  })
})

describe('Users (compact) — first-load skeleton', () => {
  // Regression: `loading` used to start false once the page moved to
  // useInfiniteRows, and ResourceTable locks `everLoaded` from an `immediate`
  // watcher during ITS setup — so the first-load skeleton could never render.
  it('shows the table skeleton before the first page arrives', async () => {
    let release: (() => void) | undefined
    _handler = (path, opts) => {
      if (path.startsWith('/users') && String((opts as { method?: string })?.method ?? 'GET') === 'GET') {
        return new Promise((resolve) => {
          release = () => resolve(makeUsers(0, 10))
        })
      }
      return defaultHandler(path, opts)
    }

    const wrapper = await mountSuspended(UsersPage)
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('.animate-pulse').length).toBeGreaterThan(0)

    release?.()
    await flushPromises()
    await wrapper.vm.$nextTick()
    await flushPromises()
    expect(wrapper.findAll('.animate-pulse').length).toBe(0)
    expect(wrapper.html()).toContain('User 0')
  })
})
