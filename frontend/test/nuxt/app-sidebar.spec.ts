// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import AppSidebar from '~/components/AppSidebar.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { useInboxStore } from '~/stores/inbox'

// ---------------------------------------------------------------------------
// Per-role render: mount AppSidebar with each seed role's permission set and
// assert the rendered menu (anchor hrefs + labels) equals what that role may
// reach. Also proves a section whose items are all hidden is not rendered.
// ---------------------------------------------------------------------------

const ROLE_PERMS: Record<string, string[]> = {
  superadmin: ['*'],
  kepala_kanwil: [
    'audit.view',
    'masterdata.office.manage', 'masterdata.employee.manage',
    'asset.view',
    'request.create', 'request.decide',
    'valuation.exclude.approve',
    'report.view', 'report.export',
    'transfer.view', 'transfer.manage',
    'disposal.view', 'disposal.manage',
    'stockopname.view', 'stockopname.manage',
    'assignment.view',
    'maintenance.view'
  ],
  kepala_unit: [
    'audit.view',
    'asset.view',
    'request.create', 'request.decide',
    'report.view', 'report.export',
    'transfer.view', 'transfer.manage',
    'disposal.view', 'disposal.manage',
    'stockopname.view', 'stockopname.manage',
    'assignment.view',
    'maintenance.view'
  ],
  manager: [
    'asset.view', 'asset.manage',
    'request.create', 'request.decide',
    'report.view', 'report.export',
    'transfer.view', 'transfer.manage',
    'disposal.view', 'disposal.manage',
    'stockopname.view', 'stockopname.manage',
    'assignment.view', 'assignment.manage',
    'maintenance.view', 'maintenance.manage'
  ],
  staf: [
    'asset.view',
    'request.create',
    'report.view'
  ]
}

const EXPECTED_ROUTES: Record<string, string[]> = {
  superadmin: [
    '/', '/notifications', '/assets', '/assets/import', '/assets/label', '/peminjaman', '/assignment',
    '/stock-opname', '/transfers', '/disposals', '/depreciation', '/maintenance',
    '/approval', '/reports',
    '/master/offices', '/master/employees', '/master/categories', '/master/map',
    '/master/reference', '/master/import',
    '/settings/users', '/settings/rbac', '/settings/data-scope',
    '/settings/field-permission', '/settings/audit'
  ],
  kepala_kanwil: [
    '/', '/notifications', '/assets', '/assets/label', '/peminjaman', '/assignment', '/stock-opname',
    '/transfers', '/disposals', '/maintenance', '/approval', '/reports',
    '/master/offices', '/master/employees', '/master/map', '/master/import',
    '/settings/audit'
  ],
  kepala_unit: [
    '/', '/notifications', '/assets', '/assets/label', '/peminjaman', '/assignment', '/stock-opname',
    '/transfers', '/disposals', '/maintenance', '/approval', '/reports',
    '/settings/audit'
  ],
  manager: [
    '/', '/notifications', '/assets', '/assets/import', '/assets/label', '/peminjaman', '/assignment',
    '/stock-opname', '/transfers', '/disposals', '/maintenance', '/approval', '/reports'
  ],
  staf: [
    '/', '/notifications', '/assets', '/assets/label', '/peminjaman', '/maintenance', '/reports'
  ]
}

function login(role: string) {
  useAuthStore().setSession(
    'tok',
    { id: 'u1', name: 'Uji Coba', email: 'uji@inventra.local', role_id: 'r1', role_name: role, office_id: null },
    ROLE_PERMS[role]!
  )
}

function hrefs(wrapper: { findAll: (s: string) => Array<{ attributes: (k: string) => string | undefined }> }): string[] {
  return wrapper
    .findAll('a')
    .map(a => a.attributes('href'))
    .filter((h): h is string => typeof h === 'string')
    .sort()
}

enableAutoUnmount(afterEach)

beforeEach(() => {
  useAuthStore().clear()
  useUiStore().sidebarCollapsed = false
  useInboxStore().pendingCount = 0
})

describe('AppSidebar — per-role visible routes equal the reachable set', () => {
  for (const role of Object.keys(ROLE_PERMS)) {
    it(`${role} renders exactly its reachable menu links`, async () => {
      login(role)
      const wrapper = await mountSuspended(AppSidebar)
      expect(hrefs(wrapper)).toEqual([...EXPECTED_ROUTES[role]!].sort())
    })
  }
})

describe('AppSidebar — group + label rendering', () => {
  it('superadmin renders both section labels and every group', async () => {
    login('superadmin')
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    expect(html).toContain('Operasional')
    expect(html).toContain('Administrasi')
    expect(html).toContain('Master Data')
    expect(html).toContain('Pengaturan')
    // The newly surfaced Master > Impor label resolves (no raw key fallback)
    expect(html).toContain('Impor Data')
    expect(html).not.toContain('nav.masterImport')
  })

  it('kepala_kanwil sees ops items it owns but not the admin-only settings', async () => {
    login('kepala_kanwil')
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    // Operational items the role owns ('Pengajuan' = the Approval leaf; its full
    // label 'Pengajuan & Approval' renders with an HTML-escaped ampersand).
    for (const label of ['Mutasi Aset', 'Penghapusan', 'Stock Opname', 'Pengajuan', 'Laporan', 'Audit Trail']) {
      expect(html).toContain(label)
    }
    // Administrasi is present (Audit Trail lives there) but the SoD-restricted
    // settings and out-of-scope master children are absent.
    expect(html).toContain('Administrasi')
    for (const label of ['Peran & RBAC', 'Data Scope', 'Field-Permission', 'Depresiasi']) {
      expect(html).not.toContain(label)
    }
    // In-scope master children show; global-only ones do not.
    expect(html).toContain('Kantor')
    expect(html).not.toContain('Kategori Aset')
    expect(html).not.toContain('Referensi')
  })

  it('kepala_unit renders the Settings group with only Audit Trail and no Master Data group', async () => {
    login('kepala_unit')
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    expect(html).toContain('Administrasi')
    expect(html).toContain('Pengaturan')
    expect(html).toContain('Audit Trail')
    // No masterdata permissions -> the whole Master Data parent auto-hides
    expect(html).not.toContain('Master Data')
    // No user/role/scope/field settings children
    for (const href of ['/settings/users', '/settings/rbac', '/settings/data-scope', '/settings/field-permission']) {
      expect(hrefs(wrapper)).not.toContain(href)
    }
  })
})

describe('AppSidebar — empty group auto-hide', () => {
  it('staf does not render the Administrasi section at all', async () => {
    login('staf')
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    // The whole Administrasi group (label + both parents) is gone
    expect(html).not.toContain('Administrasi')
    expect(html).not.toContain('Master Data')
    expect(html).not.toContain('Pengaturan')
    // ... and no admin route is reachable
    expect(hrefs(wrapper).some(h => h.startsWith('/settings') || h.startsWith('/master'))).toBe(false)
    // The Operasional section it does have is still rendered
    expect(html).toContain('Operasional')
  })

  it('manager renders no Administrasi group (no admin/master/audit permissions)', async () => {
    login('manager')
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    expect(html).not.toContain('Administrasi')
    expect(hrefs(wrapper).some(h => h.startsWith('/settings') || h.startsWith('/master'))).toBe(false)
  })
})
