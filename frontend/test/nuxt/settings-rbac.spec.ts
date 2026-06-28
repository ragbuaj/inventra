// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import RbacPage from '~/pages/settings/rbac.vue'

// ---------------------------------------------------------------------------
// Stub API client — all calls to useApiClient().request are intercepted here.
// Individual tests call setHandlers() to configure per-request behaviour.
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
// Shared fixtures — matches the catalog+roles shape the real backend returns.
// ---------------------------------------------------------------------------

const CATALOG_RESPONSE = {
  permissions: [
    {
      group: 'Aset',
      items: [
        { key: 'asset.view', label: 'Lihat aset' },
        { key: 'asset.manage', label: 'Kelola aset' }
      ]
    },
    {
      group: 'Sistem',
      items: [
        { key: 'user.manage', label: 'Kelola user' },
        { key: 'role.manage', label: 'Kelola peran & RBAC' }
      ]
    }
  ],
  scope_levels: [],
  scope_modules: []
}

const ROLES = [
  { id: 'r-superadmin', code: 'superadmin', name: 'Superadmin', is_system: true, description: 'Akses penuh' },
  { id: 'r-manager', code: 'manager', name: 'Manager', is_system: true, description: 'Manajer aset' },
  { id: 'r-auditor', code: 'auditor', name: 'Auditor', is_system: false, description: 'Akses baca-saja' }
]

const PERMS: Record<string, string[]> = {
  'r-superadmin': ['asset.view', 'asset.manage', 'user.manage', 'role.manage'],
  'r-manager': ['asset.view', 'asset.manage'],
  'r-auditor': ['asset.view']
}

/** Default handler: serves catalog, roles list, and per-role permissions. */
function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path === '/authz/catalog') return CATALOG_RESPONSE

  if (path === '/authz/roles') {
    if (opts?.method === 'POST') {
      const body = opts.body as { code: string, name: string }
      return { id: 'r-new', code: body.code, name: body.name, is_system: false }
    }
    return { data: ROLES, total: ROLES.length }
  }

  const permsMatch = path.match(/^\/authz\/roles\/([\w-]+)\/permissions$/)
  if (permsMatch) {
    const id = permsMatch[1]!
    if (opts?.method === 'PUT') return { permissions: (opts.body as { permissions: string[] }).permissions }
    return { permissions: PERMS[id] ?? [] }
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
  const wrapper = await mountSuspended(RbacPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

// ---------------------------------------------------------------------------
// Loaded state
// ---------------------------------------------------------------------------

describe('RBAC page — loaded state', () => {
  it('renders module cards from catalog with resolved group labels and permission codes', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // Group labels (resolved via i18n, group "Aset" → "Aset" from catalog.group key)
    expect(text).toContain('Aset')
    // Per-permission i18n label for asset.view
    expect(text).toContain('Lihat aset')
    // Permission code always rendered in the card
    expect(text).toContain('asset.view')
  })

  it('renders role list with all seeded role names', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Superadmin')
    expect(text).toContain('Manager')
    expect(text).toContain('Auditor')
  })

  it('shows per-role permission counts in the role list', async () => {
    const wrapper = await mountAndWait()
    // Superadmin has 4 perms — "4 izin" label
    expect(wrapper.text()).toContain('4 izin')
    // Auditor has 1 perm
    expect(wrapper.text()).toContain('1 izin')
  })

  it('shows the Add Role button', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Tambah Peran')
  })
})

// ---------------------------------------------------------------------------
// Default selection: role with code === 'manager'
// ---------------------------------------------------------------------------

describe('RBAC page — default selection', () => {
  it('auto-selects the manager role on load', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // Right pane header shows the selected role name
    expect(text).toContain('Manager')
    // System badge appears (manager is_system = true)
    expect(text).toContain('Sistem')
    // Lock note for system role (updated text: name/code locked, perms configurable)
    expect(text).toContain('tetap dapat dikonfigurasi')
  })

  it('Save is disabled when first loaded (no dirty changes)', async () => {
    const wrapper = await mountAndWait()
    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    expect(save).toBeDefined()
    expect(save!.attributes('disabled')).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// Toggling a permission
// ---------------------------------------------------------------------------

describe('RBAC page — toggling permissions', () => {
  async function selectAuditor(wrapper: Awaited<ReturnType<typeof mountAndWait>>) {
    const roleBtn = wrapper.findAll('button').find(b => b.text().includes('Auditor'))
    expect(roleBtn).toBeDefined()
    await roleBtn!.trigger('click')
    await wrapper.vm.$nextTick()
  }

  it('toggling a permission marks dirty and enables Save', async () => {
    const wrapper = await mountAndWait()
    await selectAuditor(wrapper)
    // Auditor has asset.view but not asset.manage — toggle asset.manage on
    const permBtn = wrapper.findAll('button').find(b => b.text().includes('asset.manage'))
    expect(permBtn).toBeDefined()
    await permBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    expect(save!.attributes('disabled')).toBeUndefined()
  })

  it('Save calls PUT /authz/roles/:id/permissions with the updated set and clears dirty', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    await selectAuditor(wrapper)

    // Toggle asset.manage on (auditor only had asset.view)
    const permBtn = wrapper.findAll('button').find(b => b.text().includes('asset.manage'))
    await permBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    // Dirty state should be cleared after save
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')

    // Assert the PUT request was issued with the updated permission set
    const putReq = capturedRequests.find(r => r.path === `/authz/roles/r-auditor/permissions` && r.opts.method === 'PUT')
    expect(putReq).toBeDefined()
    const body = putReq!.opts.body as { permissions: string[] }
    expect(body.permissions).toContain('asset.view')
    expect(body.permissions).toContain('asset.manage')
  })
})

// ---------------------------------------------------------------------------
// System role — lock badge and note
// ---------------------------------------------------------------------------

describe('RBAC page — system role', () => {
  it('shows lock badge and updated lock note for a system role', async () => {
    const wrapper = await mountAndWait()
    // Manager is selected by default (is_system = true)
    expect(wrapper.text()).toContain('Sistem')
    // New note: name/code locked, but permissions configurable
    expect(wrapper.text()).toContain('tetap dapat dikonfigurasi')
  })

  it('Save is disabled initially for a system role (no changes yet)', async () => {
    const wrapper = await mountAndWait()
    // No dirty state on load — clean state, save should be disabled
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    expect(save!.attributes('disabled')).toBeDefined()
  })

  it('toggling a permission on a system role enables Save and issues PUT', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    // Manager is auto-selected (is_system = true); it has asset.view + asset.manage
    // Toggle user.manage ON (manager does not have it initially)
    const permBtn = wrapper.findAll('button').find(b => b.text().includes('user.manage'))
    expect(permBtn).toBeDefined()
    await permBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    // (a) Dirty indicator appears and Save becomes enabled
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    expect(save!.attributes('disabled')).toBeUndefined()

    // (b) Click Save → PUT /authz/roles/r-manager/permissions is issued
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()

    const putReq = capturedRequests.find(
      r => r.path === '/authz/roles/r-manager/permissions' && r.opts.method === 'PUT'
    )
    expect(putReq).toBeDefined()
    const body = putReq!.opts.body as { permissions: string[] }
    expect(body.permissions).toContain('asset.view')
    expect(body.permissions).toContain('asset.manage')
    expect(body.permissions).toContain('user.manage')

    // Dirty state clears after successful save
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')
  })
})

// ---------------------------------------------------------------------------
// Add Role modal
// ---------------------------------------------------------------------------

describe('RBAC page — add role', () => {
  it('opens the modal when Add Role is clicked', async () => {
    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah Peran'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(document.body.textContent).toContain('Buat Peran')
  })

  it('shows required-field error when submitting without a name', async () => {
    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah Peran'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const create = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.trim() === 'Buat Peran')
    expect(create).toBeDefined()
    create!.click()
    await wrapper.vm.$nextTick()
    expect(document.body.textContent).toContain('Wajib diisi')
  })

  it('creates a role with slugified code from the name and closes the modal', async () => {
    const capturedRequests: Array<{ path: string, opts: Record<string, unknown> }> = []
    setHandler((path, opts = {}) => {
      capturedRequests.push({ path, opts })
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah Peran'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    const nameInput = document.body.querySelector('input[placeholder="mis. Operator Lapangan"]') as HTMLInputElement
    expect(nameInput).toBeTruthy()
    nameInput.value = 'Auditor Baru'
    nameInput.dispatchEvent(new Event('input', { bubbles: true }))
    await wrapper.vm.$nextTick()

    const create = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.trim() === 'Buat Peran')
    create!.click()
    await new Promise(r => setTimeout(r, 450))
    await wrapper.vm.$nextTick()

    // Role should now appear in the list
    expect(wrapper.text()).toContain('Auditor Baru')
    // Custom badge should be shown (new role is not a system role)
    expect(wrapper.text()).toContain('Kustom')

    // Assert POST body has the slugified code
    const postReq = capturedRequests.find(r => r.path === '/authz/roles' && r.opts.method === 'POST')
    expect(postReq).toBeDefined()
    const body = postReq!.opts.body as { code: string, name: string }
    expect(body.code).toBe('auditor_baru')
    expect(body.name).toBe('Auditor Baru')
  })

  it('shows inline conflict error on 409 response', async () => {
    setHandler((path, opts = {}) => {
      if (path === '/authz/roles' && opts.method === 'POST') {
        const err = Object.assign(new Error('Conflict'), { statusCode: 409 })
        throw err
      }
      return defaultHandler(path, opts)
    })

    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah Peran'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    const nameInput = document.body.querySelector('input[placeholder="mis. Operator Lapangan"]') as HTMLInputElement
    nameInput.value = 'Superadmin'
    nameInput.dispatchEvent(new Event('input', { bubbles: true }))
    await wrapper.vm.$nextTick()

    const create = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.trim() === 'Buat Peran')
    create!.click()
    await new Promise(r => setTimeout(r, 300))
    await wrapper.vm.$nextTick()

    // The inline conflict message from i18n settings.rbac.add.conflict
    expect(document.body.textContent).toContain('Nama peran sudah dipakai')
  })
})

// ---------------------------------------------------------------------------
// Load-error state
// ---------------------------------------------------------------------------

describe('RBAC page — load error', () => {
  it('shows the error block and retry button when roles fetch fails', async () => {
    setHandler((path) => {
      if (path === '/authz/roles') throw Object.assign(new Error('Server Error'), { statusCode: 500 })
      if (path === '/authz/catalog') return CATALOG_RESPONSE
      return defaultHandler(path)
    })

    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Gagal memuat data peran')
    expect(text).toContain('Coba lagi')
  })

  it('recovers when retry succeeds after initial failure', async () => {
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
    expect(wrapper.text()).toContain('Gagal memuat data peran')

    // Click retry
    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retryBtn).toBeDefined()
    await retryBtn!.trigger('click')
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    // Should now show role list
    expect(wrapper.text()).toContain('Manager')
    expect(wrapper.text()).not.toContain('Gagal memuat data peran')
  })
})
