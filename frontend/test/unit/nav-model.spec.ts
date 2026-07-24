import { describe, it, expect } from 'vitest'
import { appNav } from '~/utils/nav'
import type { NavItem } from '~/types'

// ---------------------------------------------------------------------------
// Visibility logic — mirrors AppSidebar.hasAny/isVisible + useCan (which treats
// the wildcard '*' as "all permissions"). Kept in lockstep with the component
// so this unit test is an honest oracle for what the sidebar renders.
// ---------------------------------------------------------------------------

function can(perms: Set<string>, key: string): boolean {
  return perms.has('*') || perms.has(key)
}

function hasAny(perms: Set<string>, permission?: string | string[]): boolean {
  if (!permission) return true
  return Array.isArray(permission)
    ? permission.some(p => can(perms, p))
    : can(perms, permission)
}

function isVisible(perms: Set<string>, item: NavItem): boolean {
  if (item.children) return item.children.some(c => isVisible(perms, c))
  return hasAny(perms, item.permission)
}

/** Flattened set of leaf `to` routes visible to a caller holding `perms`. */
function visibleLeafRoutes(perms: Set<string>): string[] {
  const out: string[] = []
  function walk(items: NavItem[]) {
    for (const item of items) {
      if (item.children) {
        walk(item.children)
      } else if (item.to && isVisible(perms, item)) {
        out.push(item.to)
      }
    }
  }
  for (const g of appNav) walk(g.items)
  return out.sort()
}

function collectItems(items: NavItem[]): NavItem[] {
  return items.flatMap(item => [item, ...(item.children ? collectItems(item.children) : [])])
}

const ALL_ITEMS = appNav.flatMap(g => collectItems(g.items))

// ---------------------------------------------------------------------------
// Seed role -> permission matrix (from the design spec's authoritative table).
// superadmin holds the wildcard; the rest are enumerated explicitly.
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

// Hand-computed expected visible leaf routes per role (the test oracle).
const EXPECTED_ROUTES: Record<string, string[]> = {
  superadmin: [
    '/', '/notifications', '/assets', '/assets/import', '/assets/label', '/peminjaman', '/assignment',
    '/stock-opname', '/transfers', '/disposals', '/depreciation', '/maintenance',
    '/approval', '/reports',
    '/master/offices', '/master/employees', '/master/categories', '/master/map',
    '/master/reference',
    '/settings/users', '/settings/rbac', '/settings/data-scope',
    '/settings/field-permission', '/settings/audit'
  ],
  kepala_kanwil: [
    '/', '/notifications', '/assets', '/assets/label', '/peminjaman', '/assignment', '/stock-opname',
    '/transfers', '/disposals', '/maintenance', '/approval', '/reports',
    '/master/offices', '/master/employees', '/master/map',
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

// ---------------------------------------------------------------------------
// Structure
// ---------------------------------------------------------------------------

describe('appNav — structure', () => {
  it('has exactly 2 groups: Operasional then Administrasi', () => {
    expect(appNav).toHaveLength(2)
    expect(appNav[0]!.labelKey).toBe('nav.group.operasional')
    expect(appNav[1]!.labelKey).toBe('nav.group.administrasi')
  })

  it('Operasional has 12 top-level items (Aset is a parent, the rest leaves)', () => {
    expect(appNav[0]!.items).toHaveLength(12)
  })

  it('Administrasi has 2 parents: Master Data (5 children) and Pengaturan (5 children)', () => {
    const master = appNav[1]!.items.find(i => i.labelKey === 'nav.masterData')
    const settings = appNav[1]!.items.find(i => i.labelKey === 'nav.settings')
    // Bulk "Impor Data" is intentionally not a sidebar entry — the import flow is
    // reached from each master screen's own Import button.
    expect(master?.children).toHaveLength(5)
    expect(settings?.children).toHaveLength(5)
  })

  it('no disabled placeholder items remain (My Assets / staff Approval removed)', () => {
    expect(ALL_ITEMS.some(i => i.disabled)).toBe(false)
    expect(ALL_ITEMS.some(i => i.labelKey === 'nav.myAssets')).toBe(false)
    expect(ALL_ITEMS.some(i => i.labelKey === 'nav.approvalStaff')).toBe(false)
  })

  it('Dashboard has no permission (visible to every authenticated user)', () => {
    const dash = appNav[0]!.items.find(i => i.labelKey === 'nav.dashboard')
    expect(dash?.to).toBe('/')
    expect(dash?.permission).toBeUndefined()
  })

  it('Notifikasi has no permission — the feed is per-user, not permission-gated', () => {
    const notif = appNav[0]!.items.find(i => i.labelKey === 'nav.notifications')
    expect(notif?.to).toBe('/notifications')
    expect(notif?.permission).toBeUndefined()
  })

  it('Dashboard and Notifikasi are the only permission-free leaves', () => {
    const free = ALL_ITEMS.filter(i => i.to && !i.permission).map(i => i.to)
    expect(free.sort()).toEqual(['/', '/notifications'])
  })
})

describe('appNav — key per-item permissions match the spec map', () => {
  const byRoute = new Map(ALL_ITEMS.filter(i => i.to).map(i => [i.to!, i]))

  it('Maintenance is an OR of maintenance.view and request.create', () => {
    expect(byRoute.get('/maintenance')?.permission).toEqual(['maintenance.view', 'request.create'])
  })

  it('Master Data has no bulk-import leaf (import is reached per master screen)', () => {
    expect(byRoute.get('/master/import')).toBeUndefined()
  })

  it('Penugasan (/assignment) is gated by assignment.view, Peminjaman by request.create', () => {
    expect(byRoute.get('/assignment')?.permission).toBe('assignment.view')
    expect(byRoute.get('/peminjaman')?.permission).toBe('request.create')
  })

  it('each settings child carries its dedicated manage/view key', () => {
    expect(byRoute.get('/settings/rbac')?.permission).toBe('role.manage')
    expect(byRoute.get('/settings/data-scope')?.permission).toBe('scope.manage')
    expect(byRoute.get('/settings/field-permission')?.permission).toBe('fieldperm.manage')
    expect(byRoute.get('/settings/users')?.permission).toBe('user.manage')
    expect(byRoute.get('/settings/audit')?.permission).toBe('audit.view')
  })
})

// ---------------------------------------------------------------------------
// Per-role visible set = permission set
// ---------------------------------------------------------------------------

describe('appNav — per-role visible leaf routes equal the permission-derived set', () => {
  for (const role of Object.keys(ROLE_PERMS)) {
    it(`${role} sees exactly the expected routes`, () => {
      const perms = new Set(ROLE_PERMS[role]!)
      expect(visibleLeafRoutes(perms)).toEqual([...EXPECTED_ROUTES[role]!].sort())
    })
  }
})

describe('appNav — kepala_kanwil visibility (explicit)', () => {
  const perms = new Set(ROLE_PERMS.kepala_kanwil)
  const routes = visibleLeafRoutes(perms)

  it('SEES Mutasi/Penghapusan/Stock Opname/Approval/Laporan/Audit', () => {
    for (const r of ['/transfers', '/disposals', '/stock-opname', '/approval', '/reports', '/settings/audit']) {
      expect(routes).toContain(r)
    }
  })

  it('does NOT see RBAC/Data-scope/Field-permission/Depreciation', () => {
    for (const r of ['/settings/rbac', '/settings/data-scope', '/settings/field-permission', '/depreciation']) {
      expect(routes).not.toContain(r)
    }
  })
})

describe('appNav — staf sees no Administrasi group', () => {
  const perms = new Set(ROLE_PERMS.staf)

  it('renders neither the Administrasi section nor any of its routes', () => {
    // No leaf under /master or /settings is visible ...
    const routes = visibleLeafRoutes(perms)
    expect(routes.some(r => r.startsWith('/master') || r.startsWith('/settings'))).toBe(false)
    // ... so both Administrasi parents auto-hide, and the whole group drops out.
    const administrasi = appNav[1]!
    expect(administrasi.items.some(item => isVisible(perms, item))).toBe(false)
  })
})
