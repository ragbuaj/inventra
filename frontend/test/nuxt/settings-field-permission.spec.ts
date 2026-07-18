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
  { id: 'r-super', code: 'superadmin', name: 'Superadmin' },
  { id: 'r-manager', code: 'manager', name: 'Manager' }
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
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
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

// Master-detail UI: the first role (Superadmin per fixture order) is
// auto-selected on load; other roles are selected via their list item.
async function selectRole(wrapper: Awaited<ReturnType<typeof mountAndWait>>, code: string) {
  const item = wrapper.find(`[data-testid="fieldperm-role-item-${code}"]`)
  expect(item.exists()).toBe(true)
  await item.trigger('click')
  await new Promise(r => setTimeout(r, 50))
  await wrapper.vm.$nextTick()
}

function fieldRow(wrapper: Awaited<ReturnType<typeof mountAndWait>>, field: string) {
  return wrapper.find(`[data-testid="fieldperm-row-${field}"]`)
}

// ---------------------------------------------------------------------------
// Loaded editor — assets entity (default), Superadmin auto-selected
// ---------------------------------------------------------------------------

describe('Field Permission page — loaded editor (assets)', () => {
  it('renders page title', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Field-Permission')
  })

  it('renders both seeded roles in the role list', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Superadmin')
    expect(text).toContain('Manager')
    expect(wrapper.find('[data-testid="fieldperm-role-item-superadmin"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="fieldperm-role-item-manager"]').exists()).toBe(true)
  })

  it('lazy-loads rules: only the auto-selected role is fetched on mount', async () => {
    const fieldGets: string[] = []
    setHandler((path, opts = {}) => {
      if (/\/fields$/.test(path) && opts?.method !== 'PUT') fieldGets.push(path)
      return defaultHandler(path, opts)
    })
    await mountAndWait()
    expect(fieldGets).toEqual(['/authz/roles/r-super/fields'])
  })

  it('shows purchase_cost field row with i18n label "Harga beli"', async () => {
    const wrapper = await mountAndWait()
    const row = fieldRow(wrapper, 'purchase_cost')
    expect(row.exists()).toBe(true)
    expect(row.text()).toContain('purchase_cost')
    expect(row.text()).toContain('Harga beli')
  })

  it('shows Default badge for fields with no explicit restriction', async () => {
    const wrapper = await mountAndWait()
    // Superadmin has zero restrictions — every row shows the Default badge
    expect(fieldRow(wrapper, 'purchase_cost').text()).toContain('Default')
  })

  it('Save is disabled when first loaded (no dirty changes)', async () => {
    const wrapper = await mountAndWait()
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save).toBeDefined()
    expect(save!.attributes('disabled')).toBeDefined()
  })

  it('purchase_cost for Manager shows view+edit OFF (explicit restriction)', async () => {
    const wrapper = await mountAndWait()
    await selectRole(wrapper, 'manager')
    const row = fieldRow(wrapper, 'purchase_cost')
    expect(row.exists()).toBe(true)
    // purchase_cost has explicit rules → no "Default" badge on that row
    expect(row.text()).not.toContain('Default')
    // The single L (view) toggle must be in the OFF/restricted visual state:
    // offPill class includes 'border-dashed'; view-ON class includes 'text-info'
    const lBtn = row.findAll('button').find(b => b.text().includes('L'))
    expect(lBtn).toBeDefined()
    expect(lBtn!.classes().join(' ')).toContain('border-dashed')
    expect(lBtn!.classes().join(' ')).not.toContain('text-info')
  })
})

// ---------------------------------------------------------------------------
// Switch entity to users
// ---------------------------------------------------------------------------

describe('Field Permission page — entity switch', () => {
  it('switching entity to users shows email row', async () => {
    const wrapper = await mountAndWait()
    // Default is assets — email should not be visible
    expect(fieldRow(wrapper, 'email').exists()).toBe(false)
    // Switch to users via USelect
    const select = wrapper.find('select')
    if (select.exists()) {
      await select.setValue('users')
      await wrapper.vm.$nextTick()
      await new Promise(r => setTimeout(r, 50))
    } else {
      // USelect may not render as a native <select>; set the value on the component
      const selects = wrapper.findAllComponents({ name: 'USelect' })
      expect(selects.length).toBeGreaterThan(0)
      await selects[0]!.setValue('users')
      await wrapper.vm.$nextTick()
      await new Promise(r => setTimeout(r, 50))
    }
    const row = fieldRow(wrapper, 'email')
    expect(row.exists()).toBe(true)
    expect(row.text()).toContain('Email')
  })
})

// ---------------------------------------------------------------------------
// Toggle a cell → dirty + Save enabled; Save issues PUT with correct body
// ---------------------------------------------------------------------------

describe('Field Permission page — toggle and save', () => {
  it('toggling a cell marks dirty and enables Save', async () => {
    const wrapper = await mountAndWait()
    // Toggle any L (view) button — Superadmin default-allow
    const lBtn = wrapper.findAll('button').find(b => b.text().trim() === 'L')
    expect(lBtn).toBeDefined()
    await lBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save!.attributes('disabled')).toBeUndefined()
  })

  it('Save PUT body preserves other-entity rows and contains only restriction cells', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await selectRole(wrapper, 'manager')

    // Manager's purchase_cost starts as {can_view:false, can_edit:false}; clicking L
    // sets view=true (edit stays false since toggleView only flips view).
    const row = fieldRow(wrapper, 'purchase_cost')
    const lBtn = row.findAll('button').find(b => b.text().includes('L'))
    expect(lBtn).toBeDefined()
    await lBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    // Click Save
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Dirty clears
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')

    // r-manager MUST have been PUT
    const managerPut = capturedRequests.find(r =>
      r.path === '/authz/roles/r-manager/fields' && r.opts.method === 'PUT'
    )
    expect(managerPut).toBeTruthy()

    const body = managerPut!.opts.body as { fields: FieldRow[] }
    // Manager's users/email restriction must be PRESERVED (cross-entity row)
    const emailRow = body.fields.find(f => f.entity === 'users' && f.field === 'email')
    expect(emailRow).toBeDefined()
    expect(emailRow!.can_view).toBe(true)
    expect(emailRow!.can_edit).toBe(false)
    // purchase_cost became view-only — still a restriction, still present
    const pcRow = body.fields.find(f => f.entity === 'assets' && f.field === 'purchase_cost')
    expect(pcRow).toBeDefined()
    expect(pcRow!.can_view).toBe(true)
    expect(pcRow!.can_edit).toBe(false)
    // Body.fields must contain only restriction cells (never full-allow rows)
    for (const r of body.fields) {
      expect(r.can_view && r.can_edit).toBe(false)
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
  it('toggling one role cell fires exactly one PUT on save', async () => {
    const putPaths: string[] = []
    setHandler((path, opts = {}) => {
      if ((opts as { method?: string }).method === 'PUT') putPaths.push(path)
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // Superadmin is auto-selected — toggle purchase_cost's L (default-allow → explicit)
    const row = fieldRow(wrapper, 'purchase_cost')
    const lBtn = row.findAll('button').find(b => b.text().includes('L'))
    expect(lBtn).toBeDefined()
    await lBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Only one PUT should be issued (only Superadmin's rules changed)
    expect(putPaths).toHaveLength(1)
    expect(putPaths[0]).toBe('/authz/roles/r-super/fields')
  })

  it('edits on two roles survive switching and both PUT on save', async () => {
    const putPaths: string[] = []
    setHandler((path, opts = {}) => {
      if ((opts as { method?: string }).method === 'PUT') putPaths.push(path)
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    // Dirty Superadmin (auto-selected)
    let lBtn = fieldRow(wrapper, 'purchase_cost').findAll('button').find(b => b.text().includes('L'))
    await lBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    // Switch to Manager and dirty it too
    await selectRole(wrapper, 'manager')
    lBtn = fieldRow(wrapper, 'purchase_cost').findAll('button').find(b => b.text().includes('L'))
    await lBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    expect(putPaths).toHaveLength(2)
    expect(putPaths).toContain('/authz/roles/r-super/fields')
    expect(putPaths).toContain('/authz/roles/r-manager/fields')
  })
})

// ---------------------------------------------------------------------------
// Default-allow toggle baseline
// ---------------------------------------------------------------------------

describe('Field Permission page — default-allow toggle baseline', () => {
  it('toggling view OFF on Superadmin (no restriction) creates a restriction PUT', async () => {
    // Superadmin has ZERO stored restrictions (FIELDS_SUPERADMIN = []).
    // purchase_cost appears as default-allow (view+edit ON, dimmed).
    // Clicking its L button turns view OFF, which forces edit OFF too.
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()

    const row = fieldRow(wrapper, 'purchase_cost')
    const lBtn = row.findAll('button').find(b => b.text().includes('L'))
    expect(lBtn).toBeDefined()
    await lBtn!.trigger('click')
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
    // Toggling view OFF also forces edit OFF (toggleView: if !cur.view => cur.edit = false)
    expect(restriction!.can_view).toBe(false)
    expect(restriction!.can_edit).toBe(false)
  })

  it('reset returns an explicit field to default and drops it from the PUT', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await selectRole(wrapper, 'manager')

    // purchase_cost is explicit for Manager — its row shows the reset button
    const row = fieldRow(wrapper, 'purchase_cost')
    const resetBtn = row.findAll('button').find(b => b.attributes('title') === 'Kembalikan ke default')
    expect(resetBtn).toBeDefined()
    await resetBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    // Row shows the Default badge again
    expect(fieldRow(wrapper, 'purchase_cost').text()).toContain('Default')

    // Save
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    const managerPut = capturedRequests.find(r =>
      r.path === '/authz/roles/r-manager/fields' && (r.opts as { method?: string }).method === 'PUT'
    )
    expect(managerPut).toBeDefined()
    const body = managerPut!.opts.body as { fields: FieldRow[] }
    // purchase_cost restriction removed; users/email still preserved
    expect(body.fields.find(f => f.field === 'purchase_cost')).toBeUndefined()
    expect(body.fields.find(f => f.entity === 'users' && f.field === 'email')).toBeDefined()
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
    // Editor should NOT be visible
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

    // Should now show the role list + editor
    expect(wrapper.text()).toContain('Superadmin')
    expect(wrapper.text()).toContain('Manager')
    expect(wrapper.text()).not.toContain('Gagal memuat field permission.')
  })
})
