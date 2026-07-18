import type { NavGroup } from '~/types'

/**
 * Single per-permission nav model — the source of truth for the sidebar and
 * the topbar breadcrumb. Every item carries the exact permission(s) that gate
 * its page and endpoints, so menu visibility equals page reachability equals
 * endpoint authorization. An item with no `permission` is visible to every
 * authenticated user (Dashboard). Array permissions use OR semantics (visible
 * when the caller holds any one of them).
 */
export const appNav: NavGroup[] = [
  {
    labelKey: 'nav.group.operasional',
    items: [
      {
        labelKey: 'nav.dashboard',
        icon: 'i-lucide-layout-dashboard',
        to: '/'
      },
      {
        // No `permission`: the notification feed is per-user and every
        // authenticated user has one (the endpoints are RequireAuth-only). The
        // entry also feeds AppTopbar's breadcrumb — without it the page title
        // would fall back to the app name.
        labelKey: 'nav.notifications',
        icon: 'i-lucide-bell',
        to: '/notifications'
      },
      {
        labelKey: 'nav.assets',
        icon: 'i-lucide-package',
        children: [
          {
            labelKey: 'nav.assetCatalog',
            to: '/assets',
            permission: 'asset.view'
          },
          {
            labelKey: 'nav.assetImport',
            to: '/assets/import',
            permission: 'asset.manage'
          },
          {
            labelKey: 'nav.assetLabel',
            to: '/assets/label',
            permission: 'asset.view'
          }
        ]
      },
      {
        labelKey: 'nav.peminjaman',
        icon: 'i-lucide-hand',
        to: '/peminjaman',
        permission: 'request.create'
      },
      {
        labelKey: 'nav.assignment',
        icon: 'i-lucide-clipboard-check',
        to: '/assignment',
        permission: 'assignment.view'
      },
      {
        labelKey: 'nav.stockOpname',
        icon: 'i-lucide-clipboard-list',
        to: '/stock-opname',
        permission: 'stockopname.view'
      },
      {
        labelKey: 'nav.transfers',
        icon: 'i-lucide-arrow-right-left',
        to: '/transfers',
        permission: 'transfer.view'
      },
      {
        labelKey: 'nav.disposals',
        icon: 'i-lucide-trash-2',
        to: '/disposals',
        permission: 'disposal.view'
      },
      {
        labelKey: 'nav.depreciation',
        icon: 'i-lucide-trending-down',
        to: '/depreciation',
        permission: 'depreciation.view'
      },
      {
        labelKey: 'nav.maintenance',
        icon: 'i-lucide-wrench',
        to: '/maintenance',
        permission: ['maintenance.view', 'request.create']
      },
      {
        labelKey: 'nav.approval',
        icon: 'i-lucide-check-square',
        to: '/approval',
        permission: 'request.decide'
      },
      {
        labelKey: 'nav.reports',
        icon: 'i-lucide-bar-chart-2',
        to: '/reports',
        permission: 'report.view'
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
            to: '/master/offices',
            permission: 'masterdata.office.manage'
          },
          {
            labelKey: 'nav.employees',
            to: '/master/employees',
            permission: 'masterdata.office.manage'
          },
          {
            labelKey: 'nav.categories',
            to: '/master/categories',
            permission: 'masterdata.global.manage'
          },
          {
            labelKey: 'nav.officeMap',
            to: '/master/map',
            permission: 'masterdata.office.manage'
          },
          {
            labelKey: 'nav.reference',
            to: '/master/reference',
            permission: 'masterdata.global.manage'
          },
          {
            labelKey: 'nav.masterImport',
            to: '/master/import',
            permission: ['masterdata.employee.manage', 'masterdata.office.manage', 'masterdata.global.manage'],
            // Bulk CSV import is a desktop workflow; hide the menu on mobile.
            desktopOnly: true
          }
        ]
      },
      {
        labelKey: 'nav.settings',
        icon: 'i-lucide-settings',
        children: [
          {
            labelKey: 'nav.users',
            to: '/settings/users',
            permission: 'user.manage'
          },
          {
            labelKey: 'nav.rbac',
            to: '/settings/rbac',
            permission: 'role.manage'
          },
          {
            labelKey: 'nav.dataScope',
            to: '/settings/data-scope',
            permission: 'scope.manage'
          },
          {
            labelKey: 'nav.fieldPermission',
            to: '/settings/field-permission',
            permission: 'fieldperm.manage'
          },
          {
            labelKey: 'nav.auditTrail',
            to: '/settings/audit',
            permission: 'audit.view'
          }
        ]
      }
    ]
  }
]
