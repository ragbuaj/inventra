import type { NavGroup } from '~/types'

/**
 * Full Superadmin nav model — reproduces the App Shell mockup exactly.
 * Built routes: /, /master/offices, /master/employees, /master/reference, /settings/users.
 * Every other item is disabled: true with no `to`.
 */
export const superadminNav: NavGroup[] = [
  {
    labelKey: 'nav.group.operasional',
    items: [
      {
        labelKey: 'nav.dashboard',
        icon: 'i-lucide-layout-dashboard',
        to: '/'
      },
      {
        labelKey: 'nav.assets',
        icon: 'i-lucide-package',
        children: [
          {
            labelKey: 'nav.assetCatalog',
            to: '/assets'
          },
          {
            labelKey: 'nav.assetImport',
            to: '/assets/import'
          },
          {
            labelKey: 'nav.assetLabel',
            to: '/assets/label'
          }
        ]
      },
      {
        labelKey: 'nav.assignment',
        icon: 'i-lucide-clipboard-check',
        to: '/assignment'
      },
      {
        labelKey: 'nav.maintenance',
        icon: 'i-lucide-wrench',
        to: '/maintenance'
      },
      {
        labelKey: 'nav.approval',
        icon: 'i-lucide-check-square',
        disabled: true,
        badgeCount: 8
      },
      {
        labelKey: 'nav.reports',
        icon: 'i-lucide-bar-chart-2',
        disabled: true
      }
    ]
  },
  {
    labelKey: 'nav.group.administrasi',
    items: [
      {
        labelKey: 'nav.masterData',
        icon: 'i-lucide-database',
        children: [
          {
            labelKey: 'nav.offices',
            to: '/master/offices'
          },
          {
            labelKey: 'nav.employees',
            to: '/master/employees'
          },
          {
            labelKey: 'nav.geography',
            disabled: true
          },
          {
            labelKey: 'nav.reference',
            to: '/master/reference'
          }
        ]
      },
      {
        labelKey: 'nav.settings',
        icon: 'i-lucide-settings',
        children: [
          {
            labelKey: 'nav.users',
            to: '/settings/users'
          },
          {
            labelKey: 'nav.rbac',
            to: '/settings/rbac'
          },
          {
            labelKey: 'nav.dataScope',
            to: '/settings/data-scope'
          },
          {
            labelKey: 'nav.fieldPermission',
            to: '/settings/field-permission'
          },
          {
            labelKey: 'nav.auditTrail',
            to: '/settings/audit'
          }
        ]
      }
    ]
  }
]

/** Staff nav — minimal menu for non-admin users */
export const staffNav: NavGroup[] = [
  {
    labelKey: 'nav.group.menu',
    items: [
      {
        labelKey: 'nav.dashboard',
        icon: 'i-lucide-layout-dashboard',
        to: '/'
      },
      {
        labelKey: 'nav.myAssets',
        icon: 'i-lucide-package',
        disabled: true
      },
      {
        labelKey: 'nav.assignment',
        icon: 'i-lucide-clipboard-check',
        disabled: true
      },
      {
        labelKey: 'nav.approvalStaff',
        icon: 'i-lucide-check-square',
        disabled: true,
        badgeCount: 2
      }
    ]
  }
]
