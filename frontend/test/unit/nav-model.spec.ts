import { describe, it, expect } from 'vitest'
import { superadminNav, staffNav } from '~/utils/nav'
import type { NavItem } from '~/types'

const BUILT_ROUTES = ['/', '/master/offices', '/master/employees', '/master/categories', '/master/map', '/master/reference', '/settings/users', '/settings/rbac', '/settings/data-scope', '/settings/field-permission', '/settings/audit', '/assets', '/assets/import', '/assets/label', '/assignment', '/stock-opname', '/transfers', '/disposals', '/depreciation', '/maintenance', '/approval', '/reports']

function collectItems(items: NavItem[]): NavItem[] {
  return items.flatMap(item => [item, ...(item.children ? collectItems(item.children) : [])])
}

function allItems(nav: typeof superadminNav): NavItem[] {
  return nav.flatMap(g => collectItems(g.items))
}

describe('superadminNav — structure', () => {
  it('has exactly 2 groups', () => {
    expect(superadminNav).toHaveLength(2)
  })

  it('first group labelKey is nav.group.operasional', () => {
    expect(superadminNav[0].labelKey).toBe('nav.group.operasional')
  })

  it('second group labelKey is nav.group.administrasi', () => {
    expect(superadminNav[1].labelKey).toBe('nav.group.administrasi')
  })

  it('Operasional has 10 top-level items', () => {
    expect(superadminNav[0].items).toHaveLength(10)
  })

  it('Administrasi has 2 top-level items (Master Data, Pengaturan)', () => {
    expect(superadminNav[1].items).toHaveLength(2)
  })
})

describe('superadminNav — built items have `to`, unbuilt are disabled', () => {
  const items = allItems(superadminNav)

  it('every item with `to` is one of the known built routes', () => {
    const withTo = items.filter(i => i.to !== undefined)
    for (const item of withTo) {
      expect(BUILT_ROUTES).toContain(item.to)
    }
  })

  it('every built route appears exactly once', () => {
    const tos = items.map(i => i.to).filter(Boolean)
    for (const route of BUILT_ROUTES) {
      expect(tos.filter(t => t === route)).toHaveLength(1)
    }
  })

  it('items without `to` have disabled=true', () => {
    const withoutTo = items.filter(i => i.to === undefined && !i.children)
    for (const item of withoutTo) {
      expect(item.disabled).toBe(true)
    }
  })

  it('no item has both `to` and disabled=true', () => {
    for (const item of items) {
      if (item.to) {
        expect(item.disabled).toBeFalsy()
      }
    }
  })
})

describe('superadminNav — assignment', () => {
  it('assignment item links to /assignment and is gated by assignment.manage', () => {
    const assignment = superadminNav[0].items.find(i => i.labelKey === 'nav.assignment')
    expect(assignment?.to).toBe('/assignment')
    expect(assignment?.permission).toBe('assignment.manage')
  })
})

describe('superadminNav — approval', () => {
  it('approval item is gated by the request.decide permission and has no hardcoded badge', () => {
    const approval = superadminNav[0].items.find(i => i.labelKey === 'nav.approval')
    expect(approval?.permission).toBe('request.decide')
    expect(approval?.badgeCount).toBeUndefined()
  })
})

describe('superadminNav — stock opname', () => {
  it('stockOpname item links to /stock-opname and is gated by stockopname.view', () => {
    const stockOpname = superadminNav[0].items.find(i => i.labelKey === 'nav.stockOpname')
    expect(stockOpname?.to).toBe('/stock-opname')
    expect(stockOpname?.permission).toBe('stockopname.view')
    expect(stockOpname?.icon).toBe('i-lucide-clipboard-list')
  })

  it('stockOpname appears after assignment and before transfers', () => {
    const keys = superadminNav[0].items.map(i => i.labelKey)
    const assignmentIdx = keys.indexOf('nav.assignment')
    const stockOpnameIdx = keys.indexOf('nav.stockOpname')
    const transfersIdx = keys.indexOf('nav.transfers')
    expect(assignmentIdx).toBeLessThan(stockOpnameIdx)
    expect(stockOpnameIdx).toBeLessThan(transfersIdx)
  })
})

describe('superadminNav — transfers and disposals', () => {
  it('transfers item links to /transfers and is gated by transfer.view', () => {
    const transfers = superadminNav[0].items.find(i => i.labelKey === 'nav.transfers')
    expect(transfers?.to).toBe('/transfers')
    expect(transfers?.permission).toBe('transfer.view')
  })

  it('disposals item links to /disposals and is gated by disposal.view', () => {
    const disposals = superadminNav[0].items.find(i => i.labelKey === 'nav.disposals')
    expect(disposals?.to).toBe('/disposals')
    expect(disposals?.permission).toBe('disposal.view')
  })

  it('depreciation item links to /depreciation and is gated by depreciation.view', () => {
    const depreciation = superadminNav[0].items.find(i => i.labelKey === 'nav.depreciation')
    expect(depreciation?.to).toBe('/depreciation')
    expect(depreciation?.permission).toBe('depreciation.view')
    expect(depreciation?.icon).toBe('i-lucide-trending-down')
  })

  it('transfers appears after assignment and before maintenance', () => {
    const keys = superadminNav[0].items.map(i => i.labelKey)
    const assignmentIdx = keys.indexOf('nav.assignment')
    const transfersIdx = keys.indexOf('nav.transfers')
    const disposalsIdx = keys.indexOf('nav.disposals')
    const depreciationIdx = keys.indexOf('nav.depreciation')
    const maintenanceIdx = keys.indexOf('nav.maintenance')
    expect(assignmentIdx).toBeLessThan(transfersIdx)
    expect(transfersIdx).toBeLessThan(disposalsIdx)
    expect(disposalsIdx).toBeLessThan(depreciationIdx)
    expect(depreciationIdx).toBeLessThan(maintenanceIdx)
  })
})

describe('superadminNav — maintenance', () => {
  it('maintenance item links to /maintenance and is gated by maintenance.view', () => {
    const maintenance = superadminNav[0].items.find(i => i.labelKey === 'nav.maintenance')
    expect(maintenance?.to).toBe('/maintenance')
    expect(maintenance?.permission).toBe('maintenance.view')
    expect(maintenance?.icon).toBe('i-lucide-wrench')
  })

  it('maintenance appears after depreciation and before approval', () => {
    const keys = superadminNav[0].items.map(i => i.labelKey)
    const depreciationIdx = keys.indexOf('nav.depreciation')
    const maintenanceIdx = keys.indexOf('nav.maintenance')
    const approvalIdx = keys.indexOf('nav.approval')
    expect(depreciationIdx).toBeLessThan(maintenanceIdx)
    expect(maintenanceIdx).toBeLessThan(approvalIdx)
  })
})

describe('superadminNav — reports', () => {
  it('reports item links to /reports and is gated by report.view', () => {
    const reports = superadminNav[0].items.find(i => i.labelKey === 'nav.reports')
    expect(reports?.to).toBe('/reports')
    expect(reports?.permission).toBe('report.view')
    expect(reports?.icon).toBe('i-lucide-bar-chart-2')
  })
})

describe('superadminNav — children groups', () => {
  it('Aset parent has 3 children (Katalog/Import/Label)', () => {
    const aset = superadminNav[0].items.find(i => i.labelKey === 'nav.assets')
    expect(aset?.children).toHaveLength(3)
  })

  it('Master Data parent has 5 children', () => {
    const master = superadminNav[1].items.find(i => i.labelKey === 'nav.masterData')
    expect(master?.children).toHaveLength(5)
  })

  it('includes a Kategori entry under Master Data linking to /master/categories', () => {
    const master = superadminNav
      .flatMap(g => g.items)
      .find(i => i.labelKey === 'nav.masterData')
    expect(master?.children?.some(c => c.to === '/master/categories' && c.labelKey === 'nav.categories')).toBe(true)
  })

  it('Pengaturan parent has 5 children', () => {
    const settings = superadminNav[1].items.find(i => i.labelKey === 'nav.settings')
    expect(settings?.children).toHaveLength(5)
  })
})

describe('staffNav', () => {
  it('has 1 group with 5 items', () => {
    expect(staffNav).toHaveLength(1)
    expect(staffNav[0].items).toHaveLength(5)
  })

  it('Dashboard has route /', () => {
    const dash = staffNav[0].items.find(i => i.labelKey === 'nav.dashboard')
    expect(dash?.to).toBe('/')
  })

  it('has an enabled nav.peminjaman item linking to /peminjaman, gated by request.create', () => {
    const peminjaman = staffNav[0].items.find(i => i.labelKey === 'nav.peminjaman')
    expect(peminjaman?.to).toBe('/peminjaman')
    expect(peminjaman?.permission).toBe('request.create')
    expect(peminjaman?.icon).toBe('i-lucide-hand')
    expect(peminjaman?.disabled).toBeFalsy()
  })

  it('has an enabled nav.maintenance item linking to /maintenance, gated by request.create', () => {
    const maintenance = staffNav[0].items.find(i => i.labelKey === 'nav.maintenance')
    expect(maintenance?.to).toBe('/maintenance')
    expect(maintenance?.permission).toBe('request.create')
    expect(maintenance?.icon).toBe('i-lucide-wrench')
    expect(maintenance?.disabled).toBeFalsy()
  })

  it('no longer has a disabled nav.assignment item', () => {
    const assignment = staffNav[0].items.find(i => i.labelKey === 'nav.assignment')
    expect(assignment).toBeUndefined()
  })
})
