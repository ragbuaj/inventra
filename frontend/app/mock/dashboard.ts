/**
 * Dashboard mock fixtures — ported 1:1 from `docs/design/Dashboard.dc.html` (the `DATA` object).
 *
 * Three data scopes (`jaksel` / `kanwil` / `pusat`) stand in for the caller's resolved data scope.
 * Static UI chrome (KPI/status/chart/panel labels) lives in i18n, NOT here. Dynamic record text the
 * mockup localizes (maintenance task/due, approval title/meta, scope name) is stored as `{id,en}` and
 * resolved by the current locale in `useDashboard`; a real API would return it already localized.
 */

export type Scope = 'jaksel' | 'kanwil' | 'pusat'

/** A bilingual string for mock record text the mockup localizes. */
export interface Localized {
  id: string
  en: string
}

export interface MaintenanceSeed {
  /** asset display string (data, not localized) */
  asset: string
  task: Localized
  icon: string
  /** 1 = urgent (warning pill), 0 = normal (neutral pill) */
  urg: 0 | 1
  due: Localized
}

export interface ApprovalSeed {
  id: string
  title: Localized
  meta: Localized
  icon: string
  /** maps to the request icon's token tone */
  tone: 'info' | 'primary' | 'neutral'
}

export interface DashboardData {
  scope: Scope
  name: Localized
  total: number
  /** pre-formatted money strings (as in the mockup) */
  perolehan: string
  buku: string
  overdue: number
  due: number
  biaya: string
  /** five status counts: available, inUse, maintenance, disposed, lost */
  status: number[]
  /** [label, count] — labels are data, not localized */
  kategori: [string, number][]
  lokasi: [string, number][]
  maint: MaintenanceSeed[]
  appr: ApprovalSeed[]
}

export const dashboardData: Record<Scope, DashboardData> = {
  jaksel: {
    scope: 'jaksel',
    name: { id: 'Kantor Cabang Jakarta Selatan', en: 'Jakarta Selatan Branch' },
    total: 96,
    perolehan: 'Rp 3,82 M',
    buku: 'Rp 2,14 M',
    overdue: 4,
    due: 3,
    biaya: 'Rp 42,5 Jt',
    status: [58, 22, 9, 4, 3],
    kategori: [['Elektronik', 41], ['Furnitur', 28], ['Perangkat IT', 12], ['Kendaraan', 9], ['Lainnya', 6]],
    lokasi: [['Lantai 2 — Operasional', 31], ['Gudang Aset', 24], ['Lantai 3 — IT', 22], ['Outlet Blok M', 19]],
    maint: [
      { asset: 'Toyota Avanza · B 1234 XYZ', task: { id: 'Servis berkala 40.000 km', en: 'Scheduled 40,000 km service' }, icon: 'i-lucide-truck', urg: 1, due: { id: 'Besok', en: 'Tomorrow' } },
      { asset: 'AC Daikin · R.301', task: { id: 'Pembersihan filter', en: 'Filter cleaning' }, icon: 'i-lucide-wrench', urg: 0, due: { id: '3 hari lagi', en: 'In 3 days' } },
      { asset: 'Genset Cummins · Gudang', task: { id: 'Inspeksi rutin', en: 'Routine inspection' }, icon: 'i-lucide-wrench', urg: 0, due: { id: '5 hari lagi', en: 'In 5 days' } }
    ],
    appr: [
      { id: 'a1', title: { id: 'Peminjaman Proyektor Epson EB-X51', en: 'Loan: Epson EB-X51 Projector' }, meta: { id: 'Andi Saputra · Staf Ops · 2 jam lalu', en: 'Andi Saputra · Ops Staff · 2h ago' }, icon: 'i-lucide-projector', tone: 'info' },
      { id: 'a2', title: { id: 'Mutasi 3 Laptop ke Outlet Blok M', en: 'Transfer 3 laptops to Blok M' }, meta: { id: 'Rina Putri · Staf · 5 jam lalu', en: 'Rina Putri · Staff · 5h ago' }, icon: 'i-lucide-package', tone: 'primary' },
      { id: 'a3', title: { id: 'Pelepasan Printer HP rusak', en: 'Disposal: broken HP printer' }, meta: { id: 'Budi Hartono · Staf · kemarin', en: 'Budi Hartono · Staff · yesterday' }, icon: 'i-lucide-printer', tone: 'neutral' }
    ]
  },
  kanwil: {
    scope: 'kanwil',
    name: { id: 'Kanwil DKI Jakarta', en: 'DKI Jakarta Regional' },
    total: 430,
    perolehan: 'Rp 18,6 M',
    buku: 'Rp 11,2 M',
    overdue: 14,
    due: 9,
    biaya: 'Rp 186,2 Jt',
    status: [268, 92, 41, 18, 11],
    kategori: [['Elektronik', 184], ['Furnitur', 96], ['Perangkat IT', 78], ['Kendaraan', 42], ['Lainnya', 30]],
    lokasi: [['Cabang Jakarta Pusat', 112], ['Cabang Jakarta Selatan', 96], ['Cabang Jakarta Timur', 88], ['Cabang Jakarta Barat', 78], ['Kanwil (langsung)', 56]],
    maint: [
      { asset: 'Toyota Hiace · B 7788 KK', task: { id: 'Servis berkala', en: 'Scheduled service' }, icon: 'i-lucide-truck', urg: 1, due: { id: 'Hari ini', en: 'Today' } },
      { asset: 'Lift Barang · Cab. Jakpus', task: { id: 'Sertifikasi tahunan', en: 'Annual certification' }, icon: 'i-lucide-wrench', urg: 1, due: { id: '2 hari lagi', en: 'In 2 days' } },
      { asset: 'Server Rack · Kanwil', task: { id: 'Pembersihan & cek suhu', en: 'Cleaning & temp check' }, icon: 'i-lucide-wrench', urg: 0, due: { id: '6 hari lagi', en: 'In 6 days' } }
    ],
    appr: [
      { id: 'a1', title: { id: 'Pengadaan 12 Monitor LG', en: 'Procure 12 LG monitors' }, meta: { id: 'Cab. Jakarta Timur · 1 jam lalu', en: 'Jaktim Branch · 1h ago' }, icon: 'i-lucide-package', tone: 'primary' },
      { id: 'a2', title: { id: 'Peminjaman Kendaraan Dinas', en: 'Official vehicle loan' }, meta: { id: 'Cab. Jakarta Barat · 3 jam lalu', en: 'Jakbar Branch · 3h ago' }, icon: 'i-lucide-truck', tone: 'info' },
      { id: 'a3', title: { id: 'Pelepasan 5 PC lama', en: 'Dispose 5 old PCs' }, meta: { id: 'Cab. Jakarta Pusat · kemarin', en: 'Jakpus Branch · yesterday' }, icon: 'i-lucide-printer', tone: 'neutral' }
    ]
  },
  pusat: {
    scope: 'pusat',
    name: { id: 'Konsolidasi Nasional', en: 'National Consolidation' },
    total: 1248,
    perolehan: 'Rp 52,4 M',
    buku: 'Rp 31,8 M',
    overdue: 38,
    due: 24,
    biaya: 'Rp 512,9 Jt',
    status: [781, 268, 121, 52, 26],
    kategori: [['Elektronik', 528], ['Furnitur', 286], ['Perangkat IT', 214], ['Kendaraan', 132], ['Lainnya', 88]],
    lokasi: [['Kanwil DKI Jakarta', 430], ['Kanwil Jawa Barat', 318], ['Kanwil Jawa Timur', 264], ['Kanwil Sumut', 142], ['Pusat (langsung)', 94]],
    maint: [
      { asset: 'Armada Dinas · 6 unit', task: { id: 'Servis terjadwal lintas wilayah', en: 'Cross-region scheduled service' }, icon: 'i-lucide-truck', urg: 1, due: { id: 'Pekan ini', en: 'This week' } },
      { asset: 'Data Center · Pusat', task: { id: 'Audit UPS & pendingin', en: 'UPS & cooling audit' }, icon: 'i-lucide-wrench', urg: 1, due: { id: '3 hari lagi', en: 'In 3 days' } },
      { asset: 'Genset 200kVA · Kanwil Jabar', task: { id: 'Penggantian oli', en: 'Oil change' }, icon: 'i-lucide-wrench', urg: 0, due: { id: 'Minggu depan', en: 'Next week' } }
    ],
    appr: [
      { id: 'a1', title: { id: 'Pengadaan Aset Lintas Kanwil', en: 'Cross-region asset procurement' }, meta: { id: 'Kanwil Jawa Barat · 30 menit lalu', en: 'West Java Regional · 30m ago' }, icon: 'i-lucide-package', tone: 'primary' },
      { id: 'a2', title: { id: 'Penghapusan Massal 24 Aset', en: 'Bulk write-off: 24 assets' }, meta: { id: 'Kanwil Jawa Timur · 2 jam lalu', en: 'East Java Regional · 2h ago' }, icon: 'i-lucide-printer', tone: 'neutral' },
      { id: 'a3', title: { id: 'Mutasi Armada Antar Wilayah', en: 'Inter-region fleet transfer' }, meta: { id: 'Kanwil Sumut · 4 jam lalu', en: 'North Sumatra Regional · 4h ago' }, icon: 'i-lucide-truck', tone: 'info' }
    ]
  }
}

/** Selectable scope options for the dashboard scope switcher (order matches the mockup). */
export const scopeOrder: Scope[] = ['jaksel', 'kanwil', 'pusat']
