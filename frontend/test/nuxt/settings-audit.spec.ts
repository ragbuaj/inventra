// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import AuditPage from '~/pages/settings/audit.vue'

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

const AUDIT_ROWS = [
  {
    id: 'a1',
    entity_type: 'assets',
    entity_id: 'e9',
    action: 'update',
    ip: '10.0.0.1',
    changes: { purchase_cost: { before: '1000', after: '1200' } },
    actor: { id: 'u1', name: 'Bambang Sukasno', email: 'b@x.id' },
    office_id: 'o1',
    created_at: '2026-06-24T08:30:00Z'
  },
  {
    id: 'a2',
    entity_type: 'users',
    entity_id: 'u5',
    action: 'create',
    ip: '192.168.1.5',
    changes: null,
    actor: { id: 'u2', name: 'Siti Rahayu', email: 's@x.id' },
    office_id: 'o1',
    created_at: '2026-06-24T09:00:00Z'
  },
  {
    id: 'a3',
    entity_type: 'roles',
    entity_id: 'r3',
    action: 'delete',
    ip: '172.16.0.2',
    changes: null,
    actor: { id: 'u3', name: 'Agus Prasetyo', email: 'a@x.id' },
    office_id: 'o2',
    created_at: '2026-06-24T10:15:00Z'
  }
]

// Default success response — total 25 to exercise multi-page pagination
function makeAuditResponse(rows = AUDIT_ROWS, total = 25) {
  return { data: rows, total, limit: 20, offset: 0 }
}

// Parse query parameters from a /audit?... path string
function parseAuditQuery(path: string): Record<string, string> {
  const qIdx = path.indexOf('?')
  if (qIdx === -1) return {}
  const params = new URLSearchParams(path.slice(qIdx + 1))
  const result: Record<string, string> = {}
  params.forEach((val, key) => {
    result[key] = val
  })
  return result
}

// Last captured query from the most recent /audit request
let _lastQuery: Record<string, string> = {}

function defaultHandler(path: string, _opts?: Record<string, unknown>): unknown {
  if (path.startsWith('/audit')) {
    _lastQuery = parseAuditQuery(path)
    return makeAuditResponse()
  }
  throw new Error(`Unhandled request: ${path}`)
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
  _lastQuery = {}
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(AuditPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

// Helper — set a page-level reactive ref (exposed via vm) and wait for the
// resulting watcher → load() cycle to complete.
async function setVmRef(wrapper: Awaited<ReturnType<typeof mountAndWait>>, key: string, value: unknown) {
  ;(wrapper.vm as unknown as Record<string, unknown>)[key] = value
  await wrapper.vm.$nextTick()
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
}

// ---------------------------------------------------------------------------
// Loaded rows
// ---------------------------------------------------------------------------

describe('Audit Trail page — loaded rows', () => {
  it('renders page title', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Audit Trail')
  })

  it('renders actor names for each row', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Bambang Sukasno')
    expect(text).toContain('Siti Rahayu')
    expect(text).toContain('Agus Prasetyo')
  })

  it('renders action badge with i18n label (Ubah/Buat/Hapus)', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Ubah') // update → settings.audit.action.update
    expect(text).toContain('Buat') // create → settings.audit.action.create
    expect(text).toContain('Hapus') // delete → settings.audit.action.delete
  })

  it('renders entity type with i18n label (Aset/User/Peran)', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Aset') // assets → settings.audit.entity.assets
    expect(text).toContain('User') // users  → settings.audit.entity.users
    expect(text).toContain('Peran') // roles  → settings.audit.entity.roles
  })

  it('renders IP addresses for rows', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('10.0.0.1')
    expect(text).toContain('192.168.1.5')
  })

  it('initial GET /audit uses offset=0 and limit=20', async () => {
    await mountAndWait()
    expect(_lastQuery['offset']).toBe('0')
    expect(_lastQuery['limit']).toBe('20')
  })
})

// ---------------------------------------------------------------------------
// Filter — entity type
//
// USelect in Nuxt UI v4 is built on reka-ui's SelectRoot and renders a fully
// custom listbox via portal into document.body (no native <select>). In JSDOM,
// reka-ui's PointerEvent-based item selection does not reliably update the
// v-model. The correct approach is to set the page's reactive ref directly via
// wrapper.vm — script-setup refs are accessible as plain properties on the
// component proxy and Vue's reactivity fires the watcher synchronously, so the
// resulting load() is observed exactly as it would be after a real user pick.
// ---------------------------------------------------------------------------

describe('Audit Trail page — entity filter', () => {
  it('setting fEntity triggers GET /audit with entity_type query param', async () => {
    const capturedQueries: Array<Record<string, string>> = []
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        const q = parseAuditQuery(path)
        capturedQueries.push(q)
        return makeAuditResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    capturedQueries.length = 0 // discard the initial load

    // Simulate the user picking "Aset" from the entity USelect
    await setVmRef(wrapper, 'fEntity', 'assets')

    const filtered = capturedQueries.find(q => q['entity_type'] === 'assets')
    expect(filtered).toBeDefined()
    // Filter change must reset to page 1
    expect(filtered!['offset']).toBe('0')
  })

  it('setting fEntity to __all__ omits entity_type from the request', async () => {
    const capturedQueries: Array<Record<string, string>> = []
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        capturedQueries.push(parseAuditQuery(path))
        return makeAuditResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    capturedQueries.length = 0

    // First set to filter
    await setVmRef(wrapper, 'fEntity', 'assets')
    capturedQueries.length = 0

    // Then clear (USelect picks "Semua Entitas" → __all__)
    await setVmRef(wrapper, 'fEntity', '__all__')

    const cleared = capturedQueries[capturedQueries.length - 1]
    expect(cleared).toBeDefined()
    expect(cleared!['entity_type']).toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// Filter — action
// ---------------------------------------------------------------------------

describe('Audit Trail page — action filter', () => {
  it('setting fAction triggers GET /audit with action query param', async () => {
    const capturedQueries: Array<Record<string, string>> = []
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        const q = parseAuditQuery(path)
        capturedQueries.push(q)
        return makeAuditResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    capturedQueries.length = 0

    // Simulate user picking "Ubah" (update) from the action USelect
    await setVmRef(wrapper, 'fAction', 'update')

    const filtered = capturedQueries.find(q => q['action'] === 'update')
    expect(filtered).toBeDefined()
    // Filter change must reset to page 1
    expect(filtered!['offset']).toBe('0')
  })

  it('setting fAction to delete sends action=delete', async () => {
    const capturedQueries: Array<Record<string, string>> = []
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        capturedQueries.push(parseAuditQuery(path))
        return makeAuditResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    capturedQueries.length = 0

    await setVmRef(wrapper, 'fAction', 'delete')

    const filtered = capturedQueries.find(q => q['action'] === 'delete')
    expect(filtered).toBeDefined()
    // Filter change must reset to page 1
    expect(filtered!['offset']).toBe('0')
  })
})

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

describe('Audit Trail page — pagination', () => {
  it('clicking next page button sends offset=20', async () => {
    const capturedQueries: Array<Record<string, string>> = []
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        const q = parseAuditQuery(path)
        capturedQueries.push(q)
        return makeAuditResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    capturedQueries.length = 0

    // With total=25 and PAGE_SIZE=20, there are 2 pages. Click the "2" button.
    const page2Btn = wrapper.findAll('button').find(b => b.text().trim() === '2')
    expect(page2Btn).toBeDefined()
    await page2Btn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    const page2Query = capturedQueries.find(q => q['offset'] === '20')
    expect(page2Query).toBeDefined()
    expect(page2Query!['limit']).toBe('20')
  })

  it('filter change resets page to 1 (offset=0) even when on page 2', async () => {
    const capturedQueries: Array<Record<string, string>> = []
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        const q = parseAuditQuery(path)
        capturedQueries.push(q)
        return makeAuditResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    capturedQueries.length = 0

    // Navigate to page 2
    const page2Btn = wrapper.findAll('button').find(b => b.text().trim() === '2')
    expect(page2Btn).toBeDefined()
    await page2Btn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    // Confirm we are on page 2 (offset=20 was sent)
    expect(capturedQueries.some(q => q['offset'] === '20')).toBe(true)
    capturedQueries.length = 0

    // Now change the entity filter — page must reset to 1
    await setVmRef(wrapper, 'fEntity', 'assets')

    const resetQuery = capturedQueries.find(q => q['entity_type'] === 'assets')
    expect(resetQuery).toBeDefined()
    expect(resetQuery!['offset']).toBe('0')
  })

  it('next-page button is disabled when on last page', async () => {
    // total=25, PAGE_SIZE=20 → 2 pages; on page 2 the next button is disabled
    const wrapper = await mountAndWait()

    // On page 1 (not the last page) the next button must be ENABLED
    const nextBtnPage1 = wrapper.find('[data-testid="audit-next-page"]')
    expect(nextBtnPage1.exists()).toBe(true)
    expect(nextBtnPage1.attributes('disabled')).toBeUndefined()

    // Go to page 2 (the last page)
    const page2Btn = wrapper.findAll('button').find(b => b.text().trim() === '2')
    expect(page2Btn).toBeDefined()
    await page2Btn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    // On the last page the next (chevron-right) button must be disabled
    const nextBtnPage2 = wrapper.find('[data-testid="audit-next-page"]')
    expect(nextBtnPage2.exists()).toBe(true)
    expect(nextBtnPage2.attributes('disabled')).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// Expandable diff (changes)
// ---------------------------------------------------------------------------

describe('Audit Trail page — expandable diff', () => {
  it('clicking a row reveals the changes diff with field and before/after values', async () => {
    const wrapper = await mountAndWait()

    // Click the first table row (Bambang Sukasno: update on assets with purchase_cost 1000→1200)
    const firstRow = wrapper.find('tbody tr')
    expect(firstRow.exists()).toBe(true)
    await firstRow.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))
    await wrapper.vm.$nextTick()

    const text = wrapper.text()
    // Field name from changes object
    expect(text).toContain('purchase_cost')
    // Before value
    expect(text).toContain('1000')
    // After value
    expect(text).toContain('1200')
  })

  it('expanded row also renders the entity_id in the diff header', async () => {
    const wrapper = await mountAndWait()
    const firstRow = wrapper.find('tbody tr')
    await firstRow.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 100))
    await wrapper.vm.$nextTick()

    // entity_id 'e9' should appear in the diff panel header
    expect(wrapper.text()).toContain('e9')
  })

  it('clicking an expanded row again collapses the diff', async () => {
    const wrapper = await mountAndWait()
    const firstRow = wrapper.find('tbody tr')

    // Expand
    await firstRow.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 50))
    expect(wrapper.text()).toContain('purchase_cost')

    // Collapse
    await firstRow.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 50))
    expect(wrapper.text()).not.toContain('purchase_cost')
  })
})

// ---------------------------------------------------------------------------
// Error state
// ---------------------------------------------------------------------------

describe('Audit Trail page — load error', () => {
  it('shows error message when GET /audit returns 500', async () => {
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // i18n: settings.audit.loadError
    expect(text).toContain('Gagal memuat audit trail.')
    // Should not show data rows
    expect(text).not.toContain('Bambang Sukasno')
  })

  it('shows retry button on error', async () => {
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    // i18n: settings.audit.retry
    expect(wrapper.text()).toContain('Coba lagi')
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
  })

  it('retry button re-fetches and recovers when second call succeeds', async () => {
    let callCount = 0
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        callCount++
        if (callCount === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return makeAuditResponse()
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Gagal memuat audit trail.')

    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    // After recovery, data rows should render
    expect(wrapper.text()).toContain('Bambang Sukasno')
    expect(wrapper.text()).not.toContain('Gagal memuat audit trail.')
  })
})

// ---------------------------------------------------------------------------
// Empty state
// ---------------------------------------------------------------------------

describe('Audit Trail page — empty state', () => {
  it('shows empty-state title when data is an empty array', async () => {
    setHandler((path) => {
      if (path.startsWith('/audit')) return makeAuditResponse([], 0)
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // i18n: settings.audit.emptyTitle
    expect(text).toContain('Tidak ada log')
    expect(text).not.toContain('Bambang Sukasno')
  })

  it('empty state shows reset button when a filter is active', async () => {
    let callCount = 0
    setHandler((path) => {
      if (path.startsWith('/audit')) {
        callCount++
        // First call returns data; subsequent calls (after filter change) return empty
        if (callCount === 1) return makeAuditResponse()
        return makeAuditResponse([], 0)
      }
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()

    // Set an entity filter to make anyFilter=true then trigger an empty response
    await setVmRef(wrapper, 'fEntity', 'assets')

    // Now in empty state with an active filter — the Reset button should appear
    const text = wrapper.text()
    expect(text).toContain('Tidak ada log')
    expect(text).toContain('Reset')
  })

  it('empty state without filters does not show reset button', async () => {
    setHandler((path) => {
      if (path.startsWith('/audit')) return makeAuditResponse([], 0)
      throw new Error(`Unhandled: ${path}`)
    })

    const wrapper = await mountAndWait()
    // No active filter → Reset button inside empty block must not render
    const resetBtns = wrapper.findAll('button').filter(b => b.text().trim() === 'Reset')
    expect(resetBtns.length).toBe(0)
  })
})
