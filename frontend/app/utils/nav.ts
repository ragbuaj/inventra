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
        disabled: true,
        children: [
          {
            labelKey: 'nav.assetCatalog',
            disabled: true
          },
          {
            labelKey: 'nav.assetImport',
            disabled: true
          },
          {
            labelKey: 'nav.assetLabel',
            disabled: true
          }
        ]
      },
      {
        labelKey: 'nav.assignment',
        icon: 'i-lucide-clipboard-check',
        disabled: true
      },
      {
        labelKey: 'nav.maintenance',
        icon: 'i-lucide-wrench',
        disabled: true
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
            disabled: true
          },
          {
            labelKey: 'nav.dataScope',
            disabled: true
          },
          {
            labelKey: 'nav.fieldPermission',
            disabled: true
          },
          {
            labelKey: 'nav.auditTrail',
            disabled: true
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
