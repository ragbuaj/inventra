// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import FieldPermissionPage from '~/pages/settings/field-permission.vue'
import type { FieldRow } from '~/composables/api/useFieldPermission'

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
  { id: 'r-super', name: 'Superadmin' },
  { id: 'r-manager', name: 'Manager' }
]

// Manager has explicit restrictions: purchase_cost fully off, users/email view-only (no edit)
const FIELDS_MANAGER: FieldRow[] = [
  { entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false },
  { entity: 'users', field: 'email', can_view: true, can_edit: false }
]

// Superadmin has no restrictions
const FIELDS_SUPERADMIN: FieldRow[] = []

const FIELDS_RESPONSES: Record<string, { fields: FieldRow[] }> = {
  'r-super': { fields: FIELDS_SUPERADMIN },
  'r-manager': { fields: FIELDS_MANAGER }
}

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path === '/authz/roles') return { data: ROLES, total: ROLES.length }

  const fieldsGetMatch = path.match(/^\/authz\/roles\/([\w-]+)\/fields$/)
  if (fieldsGetMatch) {
    const id = fieldsGetMatch[1]!
    if (opts?.method === 'PUT') return {}
    return FIELDS_RESPONSES[id] ?? { fields: [] }
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
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => {
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(FieldPermissionPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

// ---------------------------------------------------------------------------
// Loaded grid — assets entity (default)
// ---------------------------------------------------------------------------

describe('Field Permission page — loaded grid (assets)', () => {
  it('renders page title', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Field-Permission')
  })

  it('renders role column headers for both seeded roles', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Superadmin')
    expect(text).toContain('Manager')
  })

  it('shows purchase_cost field row with i18n label "Harga beli"', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('purchase_cost')
    expect(text).toContain('Harga beli')
  })

  it('shows Default badge for fields with no explicit restriction (e.g. name for all roles)', async () => {
    const wrapper = await mountAndWait()
    // "name" field has no restriction for either role — should show Default badge
    expect(wrapper.text()).toContain('Default')
  })

  it('Save is disabled when first loaded (no dirty changes)', async () => {
    const wrapper = await mountAndWait()
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save).toBeDefined()
    expect(save!.attributes('disabled')).toBeDefined()
  })

  it('purchase_cost for Manager shows view+edit OFF (explicit restriction)', async () => {
    const wrapper = await mountAndWait()
    const rows = wrapper.findAll('tr')
    const purchaseCostRow = rows.find(r => r.text().includes('purchase_cost'))
    expect(purchaseCostRow).toBeDefined()
    // purchase_cost has explicit rules → no "Default" badge on that row
    expect(purchaseCostRow!.text()).not.toContain('Default')
    // L buttons: index 0 = Superadmin, index 1 = Manager (fixture order)
    const lBtns = purchaseCostRow!.findAll('button').filter(b => b.text().includes('L'))
    expect(lBtns.length).toBeGreaterThanOrEqual(2)
    // Manager's L (index 1) must be in the OFF/restricted visual state:
    // offPill class includes 'border-dashed'; view-ON class includes 'text-info'
    const managerL = lBtns[1]!
    expect(managerL.classes().join(' ')).toContain('border-dashed')
    expect(managerL.classes().join(' ')).not.toContain('text-info')
  })
})

// ---------------------------------------------------------------------------
// Switch entity to users
// ---------------------------------------------------------------------------

describe('Field Permission page — entity switch', () => {
  it('switching entity to users shows email row', async () => {
    const wrapper = await mountAndWait()
    // Default is assets — email should not be visible
    // Switch to users via USelect
    const select = wrapper.find('select')
    if (select.exists()) {
      await select.setValue('users')
      await wrapper.vm.$nextTick()
      await new Promise(r => setTimeout(r, 50))
    } else {
      // USelect may not render as a native <select>; trigger onEntityChange via the USelect component
      // Find the entity select by looking for the USelect near "Entitas" label
      const selects = wrapper.findAllComponents({ name: 'USelect' })
      expect(selects.length).toBeGreaterThan(0)
      await selects[0]!.setValue('users')
      await wrapper.vm.$nextTick()
      await new Promise(r => setTimeout(r, 50))
    }
    expect(wrapper.text()).toContain('email')
    expect(wrapper.text()).toContain('Email')
  })
})

// ---------------------------------------------------------------------------
// Toggle a cell → dirty + Save enabled; Save issues PUT with correct body
// ---------------------------------------------------------------------------

describe('Field Permission page — toggle and save', () => {
  it('toggling a cell marks dirty and enables Save', async () => {
    const wrapper = await mountAndWait()
    // Find an L button (view toggle) for a field; use the first one found (name/Superadmin default-allow)
    const lBtn = wrapper.findAll('button').find(b => b.text().trim() === 'L')
    expect(lBtn).toBeDefined()
    await lBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save!.attributes('disabled')).toBeUndefined()
  })

  it('Save PUT body preserves other-entity rows and contains only restriction cells for the edited entity', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // We're on "assets" entity. Toggle Manager's purchase_cost view-L button (index 1
    // in the row — Superadmin is index 0, Manager is index 1 per fixture order).
    // Manager's purchase_cost starts as {can_view:false, can_edit:false}; clicking L
    // sets view=true (edit stays false since toggleView only flips view).
    // That makes r-manager dirty → saveRules issues a PUT for r-manager.
    const rows = wrapper.findAll('tr')
    const purchaseCostRow = rows.find(r => r.text().includes('purchase_cost'))
    expect(purchaseCostRow).toBeDefined()

    // L buttons in purchase_cost row: [0]=Superadmin, [1]=Manager
    const lBtns = purchaseCostRow!.findAll('button').filter(b => b.text().includes('L'))
    expect(lBtns.length).toBeGreaterThanOrEqual(2)
    // Toggle Manager's L (index 1) to make r-manager dirty
    await lBtns[1]!.trigger('click')
    await wrapper.vm.$nextTick()

    // Click Save
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Dirty clears
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')

    // Find any PUT to /authz/roles/*/fields
    const putReqs = capturedRequests.filter(r =>
      /^\/authz\/roles\/.+\/fields$/.test(r.path) && r.opts.method === 'PUT'
    )
    expect(putReqs.length).toBeGreaterThan(0)

    // r-manager MUST have been PUT — hard assertion (no if-guard)
    const managerPut = putReqs.find(r => r.path === '/authz/roles/r-manager/fields')
    expect(managerPut).toBeTruthy()

    const body = managerPut!.opts.body as { fields: FieldRow[] }
    // Manager's users/email restriction must be PRESERVED (cross-entity row)
    const emailRow = body.fields.find(f => f.entity === 'users' && f.field === 'email')
    expect(emailRow).toBeDefined()
    expect(emailRow!.can_view).toBe(true)
    expect(emailRow!.can_edit).toBe(false)
    // Body.fields for assets must contain only restriction cells (not full-allow rows)
    const assetRows = body.fields.filter(f => f.entity === 'assets')
    for (const row of assetRows) {
      const isFullAllow = row.can_view && row.can_edit
      expect(isFullAllow).toBe(false)
    }
  })

  it('dirty clears after save', async () => {
    const wrapper = await mountAndWait()
    const lBtn = wrapper.findAll('button').find(b => b.text().trim() === 'L')
    expect(lBtn).toBeDefined()
    await lBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Perubahan belum disimpan')

    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')
    const saveBtn = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(saveBtn!.attributes('disabled')).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// Only changed roles are PUT
// ---------------------------------------------------------------------------

describe('Field Permission page — only dirty roles PUT', () => {
  it('toggling exactly one role cell fires exactly one PUT on save', async () => {
    const putPaths: string[] = []
    setHandler((path, opts = {}) => {
      if ((opts as { method?: string }).method === 'PUT') putPaths.push(path)
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // Find purchase_cost row — Manager has explicit rules there, Superadmin has default-allow
    // Toggle purchase_cost's L for Superadmin (first L button in the row)
    const rows = wrapper.findAll('tr')
    const purchaseCostRow = rows.find(r => r.text().includes('purchase_cost'))
    expect(purchaseCostRow).toBeDefined()
    const lBtns = purchaseCostRow!.findAll('button').filter(b => b.text().includes('L'))
    expect(lBtns.length).toBeGreaterThanOrEqual(2)
    // Click the first L button (Superadmin column — default-allow, so this makes it explicit)
    await lBtns[0]!.trigger('click')
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Only one PUT should be issued (only Superadmin's rules changed for assets)
    expect(putPaths).toHaveLength(1)
    expect(putPaths[0]).toBe('/authz/roles/r-super/fields')
  })
})

// ---------------------------------------------------------------------------
// Default-allow toggle baseline (covers Task-3 fix)
// ---------------------------------------------------------------------------

describe('Field Permission page — default-allow toggle baseline', () => {
  it('toggling view OFF for a field on Superadmin (who has no restriction) creates a restriction PUT', async () => {
    // Superadmin has ZERO stored restrictions (FIELDS_SUPERADMIN = []).
    // purchase_cost appears as default-allow (view+edit ON, dimmed) for Superadmin.
    // Clicking Superadmin's L button on purchase_cost should turn view OFF (not leave it ON).
    // The resulting PUT for r-super must include {entity:'assets', field:'purchase_cost', can_view:false, can_edit:false}.

    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    const rows = wrapper.findAll('tr')
    const purchaseCostRow = rows.find(r => r.text().includes('purchase_cost'))
    expect(purchaseCostRow).toBeDefined()

    // Buttons in purchase_cost row: each role column has L then E buttons
    // roleCols order: [Superadmin, Manager] — so buttons[0]=Superadmin-L, buttons[1]=Superadmin-E,
    //                                              buttons[2]=Manager-L, buttons[3]=Manager-E
    // (header row reset button is NOT in the data row, so no interference)
    const lBtns = purchaseCostRow!.findAll('button').filter(b => b.text().includes('L'))
    // Superadmin's L is the first L button in the row
    expect(lBtns.length).toBeGreaterThanOrEqual(2)
    const superadminL = lBtns[0]!
    await superadminL.trigger('click')
    await wrapper.vm.$nextTick()

    // Should be dirty
    expect(wrapper.text()).toContain('Perubahan belum disimpan')

    // Save
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Dirty clears
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')

    // Find PUT for r-super
    const superPut = capturedRequests.find(r =>
      r.path === '/authz/roles/r-super/fields' && (r.opts as { method?: string }).method === 'PUT'
    )
    expect(superPut).toBeDefined()

    const body = superPut!.opts.body as { fields: FieldRow[] }
    const restriction = body.fields.find(f => f.entity === 'assets' && f.field === 'purchase_cost')
    expect(restriction).toBeDefined()
    // Toggling view OFF also forces edit OFF (see toggleView logic: if !cur.view => cur.edit = false)
    expect(restriction!.can_view).toBe(false)
    expect(restriction!.can_edit).toBe(false)
  })

  it('Manager PUT for purchase_cost is NOT issued when only Superadmin cell was toggled', async () => {
    const putPaths: string[] = []
    setHandler((path, opts = {}) => {
      if ((opts as { method?: string }).method === 'PUT') putPaths.push(path)
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // Toggle only Superadmin's L on purchase_cost (first L in the row)
    const rows = wrapper.findAll('tr')
    const purchaseCostRow = rows.find(r => r.text().includes('purchase_cost'))
    expect(purchaseCostRow).toBeDefined()
    const lBtns = purchaseCostRow!.findAll('button').filter(b => b.text().includes('L'))
    expect(lBtns.length).toBeGreaterThanOrEqual(2)
    await lBtns[0]!.trigger('click')
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Only r-super PUT
    expect(putPaths).toHaveLength(1)
    expect(putPaths[0]).toBe('/authz/roles/r-super/fields')
  })
})

// ---------------------------------------------------------------------------
// Load-error state
// ---------------------------------------------------------------------------

describe('Field Permission page — load error', () => {
  it('shows error block and retry button when GET /authz/roles returns 500', async () => {
    setHandler((path) => {
      if (path === '/authz/roles') throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      return defaultHandler(path)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // i18n: settings.fieldPermission.loadError
    expect(text).toContain('Gagal memuat field permission.')
    // i18n: settings.fieldPermission.retry
    expect(text).toContain('Coba lagi')
    // Grid should NOT be visible
    expect(text).not.toContain('Superadmin')
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
    expect(wrapper.text()).toContain('Gagal memuat field permission.')

    // Click retry
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    // Should now show the grid
    expect(wrapper.text()).toContain('Superadmin')
    expect(wrapper.text()).toContain('Manager')
    expect(wrapper.text()).not.toContain('Gagal memuat field permission.')
  })
})
