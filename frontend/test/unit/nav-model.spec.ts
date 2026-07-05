import { describe, it, expect } from 'vitest'
import { superadminNav, staffNav } from '~/utils/nav'
import type { NavItem } from '~/types'

const BUILT_ROUTES = ['/', '/master/offices', '/master/employees', '/master/categories', '/master/map', '/master/reference', '/settings/users', '/settings/rbac', '/settings/data-scope', '/settings/field-permission', '/settings/audit', '/assets', '/assets/import', '/assets/label', '/assignment', '/transfers', '/disposals', '/maintenance', '/approval', '/reports']

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

  it('Operasional has 8 top-level items', () => {
    expect(superadminNav[0].items).toHaveLength(8)
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

describe('superadminNav — approval', () => {
  it('approval item is gated by the request.decide permission and has no hardcoded badge', () => {
    const approval = superadminNav[0].items.find(i => i.labelKey === 'nav.approval')
    expect(approval?.permission).toBe('request.decide')
    expect(approval?.badgeCount).toBeUndefined()
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

  it('transfers appears after assignment and before maintenance', () => {
    const keys = superadminNav[0].items.map(i => i.labelKey)
    const assignmentIdx = keys.indexOf('nav.assignment')
    const transfersIdx = keys.indexOf('nav.transfers')
    const disposalsIdx = keys.indexOf('nav.disposals')
    const maintenanceIdx = keys.indexOf('nav.maintenance')
    expect(assignmentIdx).toBeLessThan(transfersIdx)
    expect(transfersIdx).toBeLessThan(disposalsIdx)
    expect(disposalsIdx).toBeLessThan(maintenanceIdx)
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
  it('has 1 group with 4 items', () => {
    expect(staffNav).toHaveLength(1)
    expect(staffNav[0].items).toHaveLength(4)
  })

  it('Dashboard has route /', () => {
    const dash = staffNav[0].items.find(i => i.labelKey === 'nav.dashboard')
    expect(dash?.to).toBe('/')
  })
})
