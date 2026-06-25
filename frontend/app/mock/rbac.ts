/**
 * RBAC mock fixtures — ported 1:1 from `docs/design/Peran RBAC.dc.html`.
 *
 * The module/permission catalog and role names are bilingual `{id,en}` because a real backend returns
 * this metadata already-localized (from the permission catalog + `identity.roles`); `useRbac` resolves
 * it by locale. Page chrome (titles, badges, buttons) lives in i18n, not here.
 */

export interface Localized {
  id: string
  en: string
}

export interface PermissionDef {
  code: string
  label: Localized
}

export interface ModuleDef {
  key: string
  label: Localized
  icon: string
  perms: PermissionDef[]
}

export interface Role {
  key: string
  nama: Localized
  /** system roles are built-in and read-only */
  system: boolean
  desc: Localized
  /** granted permission codes */
  perms: string[]
}

const L = (id: string, en: string): Localized => ({ id, en })

export const RBAC_MODULES: ModuleDef[] = [
  {
    key: 'aset', label: L('Aset', 'Assets'), icon: 'i-lucide-package', perms: [
      { code: 'aset.view', label: L('Lihat aset', 'View assets') },
      { code: 'aset.create', label: L('Tambah aset', 'Create asset') },
      { code: 'aset.update', label: L('Ubah aset', 'Update asset') },
      { code: 'aset.delete', label: L('Hapus aset', 'Delete asset') },
      { code: 'aset.import', label: L('Import massal', 'Bulk import') },
      { code: 'aset.label', label: L('Cetak label', 'Print label') },
      { code: 'aset.export', label: L('Ekspor data', 'Export data') }
    ]
  },
  {
    key: 'penugasan', label: L('Penugasan', 'Assignments'), icon: 'i-lucide-clipboard-check', perms: [
      { code: 'penugasan.view', label: L('Lihat penugasan', 'View assignments') },
      { code: 'penugasan.checkout', label: L('Check-out aset', 'Check out') },
      { code: 'penugasan.checkin', label: L('Check-in aset', 'Check in') }
    ]
  },
  {
    key: 'maintenance', label: L('Maintenance', 'Maintenance'), icon: 'i-lucide-wrench', perms: [
      { code: 'maintenance.view', label: L('Lihat maintenance', 'View maintenance') },
      { code: 'maintenance.create', label: L('Tambah catatan', 'Create record') },
      { code: 'maintenance.report', label: L('Laporkan kerusakan', 'Report damage') }
    ]
  },
  {
    key: 'pengajuan', label: L('Pengajuan', 'Requests'), icon: 'i-lucide-check-square', perms: [
      { code: 'pengajuan.view', label: L('Lihat pengajuan', 'View requests') },
      { code: 'pengajuan.create', label: L('Buat pengajuan', 'Create request') },
      { code: 'pengajuan.approve', label: L('Setujui', 'Approve') },
      { code: 'pengajuan.reject', label: L('Tolak', 'Reject') }
    ]
  },
  {
    key: 'master', label: L('Master Data', 'Master Data'), icon: 'i-lucide-database', perms: [
      { code: 'master.view', label: L('Lihat master data', 'View master data') },
      { code: 'master.manage', label: L('Kelola master data', 'Manage master data') }
    ]
  },
  {
    key: 'user', label: L('User', 'Users'), icon: 'i-lucide-users', perms: [
      { code: 'user.view', label: L('Lihat user', 'View users') },
      { code: 'user.manage', label: L('Kelola user', 'Manage users') },
      { code: 'user.rbac', label: L('Atur peran & izin', 'Manage roles & RBAC') }
    ]
  },
  {
    key: 'laporan', label: L('Laporan', 'Reports'), icon: 'i-lucide-bar-chart-2', perms: [
      { code: 'laporan.view', label: L('Lihat laporan', 'View reports') },
      { code: 'laporan.export', label: L('Ekspor laporan', 'Export reports') }
    ]
  },
  {
    key: 'audit', label: L('Audit Trail', 'Audit Trail'), icon: 'i-lucide-history', perms: [
      { code: 'audit.view', label: L('Lihat audit trail', 'View audit trail') }
    ]
  }
]

/** Every permission code, in module order (used by the Superadmin seed). */
export const ALL_PERMISSION_CODES: string[] = RBAC_MODULES.flatMap(m => m.perms.map(p => p.code))

export const roleSeed: Role[] = [
  { key: 'superadmin', nama: L('Superadmin', 'Superadmin'), system: true, desc: L('Akses penuh ke seluruh modul & konfigurasi.', 'Full access to all modules & configuration.'), perms: [...ALL_PERMISSION_CODES] },
  { key: 'kakanwil', nama: L('Kepala Kanwil', 'Regional Head'), system: true, desc: L('Approval & laporan dalam lingkup wilayah.', 'Approval & reports within the region.'), perms: ['aset.view', 'aset.export', 'penugasan.view', 'maintenance.view', 'pengajuan.view', 'pengajuan.approve', 'pengajuan.reject', 'master.view', 'user.view', 'laporan.view', 'laporan.export', 'audit.view'] },
  { key: 'kaunit', nama: L('Kepala Unit', 'Unit Head'), system: true, desc: L('Approval & laporan dalam lingkup kantornya.', 'Approval & reports within the office.'), perms: ['aset.view', 'penugasan.view', 'maintenance.view', 'pengajuan.view', 'pengajuan.approve', 'pengajuan.reject', 'laporan.view', 'audit.view'] },
  { key: 'manager', nama: L('Manager (Asset Manager)', 'Manager (Asset Manager)'), system: true, desc: L('Operasional aset penuh dalam lingkup kantor.', 'Full asset operations within the office.'), perms: ['aset.view', 'aset.create', 'aset.update', 'aset.delete', 'aset.import', 'aset.label', 'aset.export', 'penugasan.view', 'penugasan.checkout', 'penugasan.checkin', 'maintenance.view', 'maintenance.create', 'pengajuan.view', 'pengajuan.create', 'laporan.view', 'laporan.export'] },
  { key: 'staf', nama: L('Staf', 'Staff'), system: true, desc: L('Aset yang dipegang; pengajuan & laporan kerusakan.', 'Held assets; requests & damage reports.'), perms: ['aset.view', 'penugasan.view', 'maintenance.report', 'pengajuan.view', 'pengajuan.create'] },
  { key: 'auditor', nama: L('Auditor Internal', 'Internal Auditor'), system: false, desc: L('Akses baca-saja untuk audit & laporan.', 'Read-only access for audit & reports.'), perms: ['aset.view', 'maintenance.view', 'pengajuan.view', 'laporan.view', 'laporan.export', 'audit.view'] },
  { key: 'gudang', nama: L('Operator Gudang', 'Warehouse Operator'), system: false, desc: L('Check-out/in & pelabelan aset gudang.', 'Check-out/in & labeling of warehouse assets.'), perms: ['aset.view', 'aset.label', 'penugasan.view', 'penugasan.checkout', 'penugasan.checkin', 'maintenance.view'] }
]

/** Mutable in-memory role store keyed by `key` (roles use `key`, not the generic `id`). */
const roles: Role[] = roleSeed.map(r => ({ ...r, perms: [...r.perms] }))

export const roleStore = {
  all(): Role[] {
    return roles
  },
  find(key: string): Role | undefined {
    return roles.find(r => r.key === key)
  },
  insert(role: Role): Role {
    roles.push(role)
    return role
  },
  setPerms(key: string, perms: string[]): Role | undefined {
    const r = roles.find(x => x.key === key)
    if (r) r.perms = [...perms]
    return r
  },
  /** Restore the store to its seed (used by tests). */
  reset(): void {
    roles.length = 0
    for (const r of roleSeed) roles.push({ ...r, perms: [...r.perms] })
  }
}
