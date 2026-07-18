// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import DataScopePage from '~/pages/settings/data-scope.vue'

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
// Shared fixtures — matches the catalog + roles + scope shape the backend returns.
// ---------------------------------------------------------------------------

const CATALOG_RESPONSE = {
  scope_modules: ['*', 'offices', 'employees', 'assets', 'requests', 'audit'],
  permissions: [],
  scope_levels: []
}

const ROLES = [
  { id: 'r-superadmin', code: 'superadmin', name: 'Superadmin', description: 'Akses penuh' },
  { id: 'r-manager', code: 'manager', name: 'Manager', description: 'Manajer aset' }
]

// Superadmin: default=global, no module overrides
const SCOPE_SUPERADMIN = {
  policies: [
    { module: '*', scope_level: 'global' }
  ]
}

// Manager: default=office, assets override=office_subtree
const SCOPE_MANAGER = {
  policies: [
    { module: '*', scope_level: 'office' },
    { module: 'assets', scope_level: 'office_subtree' }
  ]
}

const SCOPE_RESPONSES: Record<string, typeof SCOPE_SUPERADMIN> = {
  'r-superadmin': SCOPE_SUPERADMIN,
  'r-manager': SCOPE_MANAGER
}

/**
 * Default handler: serves catalog, roles list, per-role scope, and handles PUTs.
 */
function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path === '/authz/catalog') return CATALOG_RESPONSE

  if (path === '/authz/roles') return { data: ROLES, total: ROLES.length }

  const scopeGetMatch = path.match(/^\/authz\/roles\/([\w-]+)\/scope$/)
  if (scopeGetMatch) {
    const id = scopeGetMatch[1]!
    if (opts?.method === 'PUT') return {}
    return SCOPE_RESPONSES[id] ?? { policies: [{ module: '*', scope_level: 'own' }] }
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
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(DataScopePage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

// Master-detail UI: the first role (Superadmin per fixture order) is
// auto-selected on load; other roles are selected via their list item.
async function selectRole(wrapper: Awaited<ReturnType<typeof mountAndWait>>, code: string) {
  const item = wrapper.find(`[data-testid="scope-role-item-${code}"]`)
  expect(item.exists()).toBe(true)
  await item.trigger('click')
  await new Promise(r => setTimeout(r, 50))
  await wrapper.vm.$nextTick()
}

// ---------------------------------------------------------------------------
// Loaded editor
// ---------------------------------------------------------------------------

describe('Data Scope page — loaded editor', () => {
  it('renders title and legend with all 4 scope-level keys', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // Page title (role-list pane header)
    expect(text).toContain('Data Scope')
    // Legend section header
    expect(text).toContain('Level lingkup data')
    // All 4 scope level keys
    expect(text).toContain('global')
    expect(text).toContain('office_subtree')
    expect(text).toContain('office')
    expect(text).toContain('own')
  })

  it('renders i18n descriptions for all 4 scope levels in the legend', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // i18n resolved descriptions from settings.dataScope.level.*
    expect(text).toContain('Semua data lintas kantor')
    expect(text).toContain('Kantor sendiri + seluruh turunannya')
    expect(text).toContain('Hanya kantor sendiri')
    expect(text).toContain('Hanya data miliknya')
  })

  it('renders module rows with i18n labels from catalog for the selected role', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // Module labels from settings.dataScope.module.*
    expect(text).toContain('Kantor') // offices
    expect(text).toContain('Pegawai') // employees
    expect(text).toContain('Aset') // assets
    expect(text).toContain('Pengajuan') // requests
    expect(text).toContain('Audit') // audit
    // The "Default" card label
    expect(text).toContain('Default')
    // One row per module
    expect(wrapper.find('[data-testid="scope-module-row-assets"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="scope-module-row-audit"]').exists()).toBe(true)
  })

  it('renders seeded role names in the role list', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Superadmin')
    expect(text).toContain('Manager')
    expect(wrapper.find('[data-testid="scope-role-item-superadmin"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="scope-role-item-manager"]').exists()).toBe(true)
  })

  it('lazy-loads scope: only the auto-selected role is fetched on mount', async () => {
    const scopeGets: string[] = []
    setHandler((path, opts = {}) => {
      if (/\/scope$/.test(path) && opts?.method !== 'PUT') scopeGets.push(path)
      return defaultHandler(path, opts)
    })
    await mountAndWait()
    expect(scopeGets).toEqual(['/authz/roles/r-superadmin/scope'])
  })

  it('Save is disabled when first loaded (no dirty changes)', async () => {
    const wrapper = await mountAndWait()
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save).toBeDefined()
    expect(save!.attributes('disabled')).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// Changing the selected role's DEFAULT via ScopeCell
// ---------------------------------------------------------------------------

describe('Data Scope page — change role default', () => {
  it('changing Superadmin default marks dirty and enables Save', async () => {
    const wrapper = await mountAndWait()
    // Superadmin is auto-selected; its Default cell pill shows "global"
    const pill = wrapper.find('[data-testid="scope-default-cell"] button')
    expect(pill.exists()).toBe(true)
    expect(pill.text()).toContain('global')
    await pill.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 20))

    // Pick "own" from the teleported popover (look for its description)
    const ownOpt = Array.from(document.body.querySelectorAll('button')).find(b =>
      b.textContent?.includes('Hanya data miliknya')
    )
    expect(ownOpt).toBeDefined()
    ownOpt!.click()
    await wrapper.vm.$nextTick()

    // Dirty indicator visible
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    // Save enabled
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save!.attributes('disabled')).toBeUndefined()
  })

  it('Save issues PUT /authz/roles/:id/scope with {module:"*", scope_level:<new>} and clears dirty', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // Superadmin auto-selected — open its default cell (shows "global")
    const pill = wrapper.find('[data-testid="scope-default-cell"] button')
    await pill.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 20))

    // Pick "own" from the popover
    const ownOpt = Array.from(document.body.querySelectorAll('button')).find(b =>
      b.textContent?.includes('Hanya data miliknya')
    )
    expect(ownOpt).toBeDefined()
    ownOpt!.click()
    await wrapper.vm.$nextTick()

    // Click Save
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Dirty clears
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')

    // Assert the PUT was issued for Superadmin's scope
    const putReq = capturedRequests.find(r =>
      r.path === '/authz/roles/r-superadmin/scope' && r.opts.method === 'PUT'
    )
    expect(putReq).toBeDefined()
    const body = putReq!.opts.body as { policies: Array<{ module: string, scope_level: string }> }
    const starPolicy = body.policies.find(p => p.module === '*')
    expect(starPolicy).toBeDefined()
    expect(starPolicy!.scope_level).toBe('own')
  })
})

// ---------------------------------------------------------------------------
// Module override — set and clear (on the Manager role via the role list)
// ---------------------------------------------------------------------------

describe('Data Scope page — module overrides', () => {
  it('selecting Manager shows its assets override (office_subtree)', async () => {
    const wrapper = await mountAndWait()
    await selectRole(wrapper, 'manager')
    // Manager has assets=office_subtree override — its pill shows "office_subtree"
    const assetsRow = wrapper.find('[data-testid="scope-module-row-assets"]')
    expect(assetsRow.exists()).toBe(true)
    expect(assetsRow.text()).toContain('office_subtree')
  })

  it('PUT for Manager includes both the * default and the assets override', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await selectRole(wrapper, 'manager')

    // Change Manager's DEFAULT (currently "office") → pick "global"
    const pill = wrapper.find('[data-testid="scope-default-cell"] button')
    expect(pill.text()).toContain('office')
    await pill.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 20))

    // Pick "global" from the popover (button contains both the mono key and the description)
    const globalOpt = Array.from(document.body.querySelectorAll('button')).find(b =>
      b.textContent?.includes('global') && b.textContent?.includes('Semua data lintas kantor')
    )
    expect(globalOpt).toBeDefined()
    globalOpt!.click()
    await wrapper.vm.$nextTick()

    // Save
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // PUT for Manager should include * and the assets override
    const putReq = capturedRequests.find(r =>
      r.path === '/authz/roles/r-manager/scope' && r.opts.method === 'PUT'
    )
    expect(putReq).toBeDefined()
    const body = putReq!.opts.body as { policies: Array<{ module: string, scope_level: string }> }

    const starPolicy = body.policies.find(p => p.module === '*')
    expect(starPolicy).toBeDefined()
    expect(starPolicy!.scope_level).toBe('global')

    const assetsPolicy = body.policies.find(p => p.module === 'assets')
    expect(assetsPolicy).toBeDefined()
    expect(assetsPolicy!.scope_level).toBe('office_subtree')
  })

  it('clearing Manager assets override removes that module from the PUT body', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await selectRole(wrapper, 'manager')

    // Manager's assets row shows "office_subtree" (the override) — open its popover.
    const overridePill = wrapper.find('[data-testid="scope-module-row-assets"]').findAll('button')
      .find(b => b.text().includes('office_subtree'))
    expect(overridePill).toBeDefined()
    await overridePill!.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 20))

    // Click "Ikuti Default" (follow default / clear override)
    const followBtn = Array.from(document.body.querySelectorAll('button')).find(b =>
      b.textContent?.includes('Ikuti Default')
    )
    expect(followBtn).toBeDefined()
    followBtn!.click()
    await wrapper.vm.$nextTick()

    // Dirty is set
    expect(wrapper.text()).toContain('Perubahan belum disimpan')

    // Save
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    const putReq = capturedRequests.find(r =>
      r.path === '/authz/roles/r-manager/scope' && r.opts.method === 'PUT'
    )
    expect(putReq).toBeDefined()
    const body = putReq!.opts.body as { policies: Array<{ module: string, scope_level: string }> }

    // "assets" module should NOT be in the PUT body (override was cleared)
    const assetsPolicy = body.policies.find(p => p.module === 'assets')
    expect(assetsPolicy).toBeUndefined()

    // "*" default should still be present
    const starPolicy = body.policies.find(p => p.module === '*')
    expect(starPolicy).toBeDefined()
    expect(starPolicy!.scope_level).toBe('office')
  })
})

// ---------------------------------------------------------------------------
// Only changed roles are PUT
// ---------------------------------------------------------------------------

describe('Data Scope page — only dirty roles PUT', () => {
  it('changing exactly one role fires exactly one PUT', async () => {
    const putPaths: string[] = []
    setHandler((path, opts = {}) => {
      if (opts?.method === 'PUT') putPaths.push(path)
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // Change ONLY Superadmin's default (auto-selected)
    const pill = wrapper.find('[data-testid="scope-default-cell"] button')
    await pill.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 20))

    const ownOpt = Array.from(document.body.querySelectorAll('button')).find(b =>
      b.textContent?.includes('Hanya data miliknya')
    )
    expect(ownOpt).toBeDefined()
    ownOpt!.click()
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Exactly one PUT, for Superadmin only
    expect(putPaths).toHaveLength(1)
    expect(putPaths[0]).toBe('/authz/roles/r-superadmin/scope')
  })

  it('edits on two roles survive switching and both PUT on save', async () => {
    const putPaths: string[] = []
    setHandler((path, opts = {}) => {
      if (opts?.method === 'PUT') putPaths.push(path)
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // Dirty Superadmin (auto-selected): default → own
    await wrapper.find('[data-testid="scope-default-cell"] button').trigger('click')
    await new Promise(r => setTimeout(r, 20))
    Array.from(document.body.querySelectorAll('button')).find(b =>
      b.textContent?.includes('Hanya data miliknya')
    )!.click()
    await wrapper.vm.$nextTick()

    // Switch to Manager and dirty it too: default → global
    await selectRole(wrapper, 'manager')
    await wrapper.find('[data-testid="scope-default-cell"] button').trigger('click')
    await new Promise(r => setTimeout(r, 20))
    Array.from(document.body.querySelectorAll('button')).find(b =>
      b.textContent?.includes('global') && b.textContent?.includes('Semua data lintas kantor')
    )!.click()
    await wrapper.vm.$nextTick()

    // Save flushes BOTH dirty roles
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    expect(putPaths).toHaveLength(2)
    expect(putPaths).toContain('/authz/roles/r-superadmin/scope')
    expect(putPaths).toContain('/authz/roles/r-manager/scope')
  })
})

// ---------------------------------------------------------------------------
// Load-error state
// ---------------------------------------------------------------------------

describe('Data Scope page — load error', () => {
  it('shows error block and retry button when GET /authz/roles returns 500', async () => {
    setHandler((path) => {
      if (path === '/authz/roles') throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      if (path === '/authz/catalog') return CATALOG_RESPONSE
      return defaultHandler(path)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // i18n: settings.dataScope.loadError
    expect(text).toContain('Gagal memuat kebijakan data scope.')
    // i18n: settings.dataScope.retry
    expect(text).toContain('Coba lagi')
    // Editor should NOT be visible
    expect(text).not.toContain('Superadmin')
  })

  it('shows error block when GET /authz/catalog returns 500', async () => {
    setHandler((path) => {
      if (path === '/authz/catalog') throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      return defaultHandler(path)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Gagal memuat kebijakan data scope.')
    expect(text).toContain('Coba lagi')
  })

  it('recovers when retry succeeds after initial roles failure', async () => {
    let callCount = 0
    setHandler((path, opts) => {
      if (path === '/authz/roles') {
        callCount++
        if (callCount === 1) throw Object.assign(new Error('Server Error'), { statusCode: 500 })
        return defaultHandler(path, opts)
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    // Error state shown
    expect(wrapper.text()).toContain('Gagal memuat kebijakan data scope.')

    // Click retry
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    // Should now show the role list + editor
    expect(wrapper.text()).toContain('Superadmin')
    expect(wrapper.text()).toContain('Manager')
    expect(wrapper.text()).not.toContain('Gagal memuat kebijakan data scope.')
  })
})
