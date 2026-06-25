import type { Asset, AssetStatus, BadgeColor } from '~/types'

/** Status → semantic badge tone + dot class (ported from the mockup's STATUS map). */
export const ASSET_STATUS_META: Record<AssetStatus, { tone: BadgeColor, dot: string }> = {
  tersedia: { tone: 'success', dot: 'bg-success' },
  dipinjam: { tone: 'info', dot: 'bg-info' },
  maintenance: { tone: 'warning', dot: 'bg-warning' },
  dilepas: { tone: 'neutral', dot: 'bg-[var(--ui-text-dimmed)]' },
  hilang: { tone: 'error', dot: 'bg-error' }
}

export const ASSET_STATUS_KEYS: AssetStatus[] = ['tersedia', 'dipinjam', 'maintenance', 'dilepas', 'hilang']
export const ASSET_CATEGORIES = ['Elektronik', 'Furnitur', 'Kendaraan', 'Perangkat IT']
export const ASSET_OFFICES = ['Cabang Jakarta Selatan', 'Outlet Blok M', 'Outlet Kemang']
export const ASSET_LOCATIONS = ['Lantai 3 — IT', 'Lantai 2 — Operasional', 'Ruang Server', 'Ruang Rapat A', 'Gudang Aset', 'Parkir Basement', 'Lobi']

const a = (
  tag: string, nama: string, kategori: string, brand: string, status: AssetStatus,
  kantor: string, lokasi: string, holder: string, tgl: string, harga: number, buku: number
): Asset => ({ tag, nama, kategori, brand, status, kantor, lokasi, holder, tgl, harga, buku })

export const assetSeed: Asset[] = [
  a('JKT01-ELK-2026-00001', 'Laptop Dell Latitude 5440', 'Elektronik', 'Dell Latitude 5440', 'tersedia', 'Cabang Jakarta Selatan', 'Lantai 3 — IT', '—', '2026-01-12', 18500000, 16200000),
  a('JKT01-ELK-2026-00002', 'Proyektor Epson EB-X51', 'Elektronik', 'Epson EB-X51', 'dipinjam', 'Cabang Jakarta Selatan', 'Ruang Rapat A', 'Andi Saputra', '2026-01-20', 7200000, 6500000),
  a('JKT01-KEN-2025-00007', 'Toyota Avanza 1.5 G', 'Kendaraan', 'Toyota Avanza', 'maintenance', 'Cabang Jakarta Selatan', 'Parkir Basement', '—', '2025-03-04', 235000000, 198000000),
  a('JKT01-FUR-2025-00011', 'Meja Kerja Ergonomis', 'Furnitur', 'IKEA BEKANT', 'tersedia', 'Cabang Jakarta Selatan', 'Lantai 2 — Operasional', '—', '2025-06-18', 2400000, 1900000),
  a('JKT01-ELK-2024-00021', 'Printer HP LaserJet Pro M404', 'Elektronik', 'HP M404dn', 'hilang', 'Cabang Jakarta Selatan', 'Lantai 2 — Operasional', '—', '2024-02-09', 4100000, 0),
  a('JKT01-ELK-2026-00005', 'Monitor LG 27UL550', 'Elektronik', 'LG 27UL550', 'dipinjam', 'Cabang Jakarta Selatan', 'Lantai 3 — IT', 'Rina Putri', '2026-02-02', 3800000, 3500000),
  a('JKT01-ITX-2025-00014', 'Router MikroTik RB4011', 'Perangkat IT', 'MikroTik RB4011', 'tersedia', 'Cabang Jakarta Selatan', 'Lantai 3 — IT', '—', '2025-09-15', 2700000, 2300000),
  a('JKT01-ELK-2023-00009', 'AC Daikin FTKC50', 'Elektronik', 'Daikin FTKC50', 'maintenance', 'Cabang Jakarta Selatan', 'Ruang Server', '—', '2023-11-22', 8900000, 5400000),
  a('JKT01-FUR-2026-00003', 'Kursi Ergonomis Ergotec', 'Furnitur', 'Ergotec GL-905', 'tersedia', 'Outlet Kemang', 'Lantai 2 — Operasional', '—', '2026-01-08', 1850000, 1700000),
  a('JKT01-ITX-2026-00006', 'CCTV Hikvision DS-2CD', 'Perangkat IT', 'Hikvision DS-2CD2143', 'dipinjam', 'Cabang Jakarta Selatan', 'Lobi', 'Rina Putri', '2026-02-14', 1600000, 1500000),
  a('JKT01-ELK-2025-00018', 'UPS APC Smart-UPS 1500', 'Elektronik', 'APC SMT1500', 'tersedia', 'Cabang Jakarta Selatan', 'Ruang Server', '—', '2025-07-30', 6700000, 5900000),
  a('JKT01-KEN-2024-00004', 'Honda Vario 160', 'Kendaraan', 'Honda Vario 160', 'dipinjam', 'Cabang Jakarta Selatan', 'Parkir Basement', 'Budi Hartono', '2024-08-12', 28500000, 22000000),
  a('JKT01-FUR-2024-00016', 'Lemari Arsip Besi', 'Furnitur', 'Brother B-204', 'dilepas', 'Cabang Jakarta Selatan', 'Gudang Aset', '—', '2024-04-19', 3200000, 1200000),
  a('JKT01-ELK-2026-00008', 'Laptop MacBook Air M3', 'Elektronik', 'Apple MacBook Air M3', 'dipinjam', 'Cabang Jakarta Selatan', 'Lantai 3 — IT', 'Rina Putri', '2026-03-01', 21500000, 20800000),
  a('JKT01-ITX-2025-00022', 'Switch Cisco Catalyst 1000', 'Perangkat IT', 'Cisco C1000-24T', 'tersedia', 'Cabang Jakarta Selatan', 'Ruang Server', '—', '2025-10-05', 9800000, 8600000),
  a('JKT01-ELK-2025-00025', 'Scanner Fujitsu fi-7160', 'Elektronik', 'Fujitsu fi-7160', 'tersedia', 'Cabang Jakarta Selatan', 'Lantai 2 — Operasional', '—', '2025-05-11', 12500000, 10200000),
  a('JKT01-FUR-2026-00010', 'Sofa Ruang Tamu 3-Seat', 'Furnitur', 'Informa Vesta', 'tersedia', 'Outlet Blok M', 'Lobi', '—', '2026-01-25', 5600000, 5300000),
  a('JKT01-KEN-2026-00002', 'Toyota Hiace Commuter', 'Kendaraan', 'Toyota Hiace', 'tersedia', 'Cabang Jakarta Selatan', 'Parkir Basement', '—', '2026-02-28', 588000000, 575000000),
  a('JKT01-ELK-2024-00030', 'Televisi Samsung 55" Crystal', 'Elektronik', 'Samsung UA55CU8000', 'dipinjam', 'Cabang Jakarta Selatan', 'Ruang Rapat A', 'Andi Saputra', '2024-12-03', 9300000, 7100000),
  a('JKT01-ITX-2026-00012', 'Access Point Ubiquiti U6-Pro', 'Perangkat IT', 'Ubiquiti U6-Pro', 'tersedia', 'Outlet Blok M', 'Lantai 3 — IT', '—', '2026-03-08', 2300000, 2200000),
  a('JKT01-FUR-2023-00007', 'Meja Rapat Kayu 10-Seat', 'Furnitur', 'Custom Jati', 'maintenance', 'Cabang Jakarta Selatan', 'Ruang Rapat A', '—', '2023-09-17', 14500000, 9800000),
  a('JKT01-ELK-2025-00028', 'Genset Cummins C22 D5', 'Elektronik', 'Cummins C22D5', 'tersedia', 'Outlet Kemang', 'Gudang Aset', '—', '2025-08-21', 78000000, 69000000),
  a('JKT01-ELK-2026-00015', 'Laptop Lenovo ThinkPad E14', 'Elektronik', 'Lenovo ThinkPad E14', 'dipinjam', 'Cabang Jakarta Selatan', 'Lantai 3 — IT', 'Rina Putri', '2026-03-12', 13200000, 12900000),
  a('JKT01-KEN-2022-00001', 'Mitsubishi L300 Box', 'Kendaraan', 'Mitsubishi L300', 'dilepas', 'Cabang Jakarta Selatan', 'Parkir Basement', '—', '2022-05-30', 165000000, 60000000),
  a('JKT01-FUR-2026-00018', 'Filing Cabinet 4-Laci', 'Furnitur', 'Datascrip FC-4', 'tersedia', 'Outlet Blok M', 'Gudang Aset', '—', '2026-02-19', 2100000, 2000000),
  a('JKT01-ELK-2024-00033', 'Printer Epson L3210', 'Elektronik', 'Epson L3210', 'tersedia', 'Cabang Jakarta Selatan', 'Lantai 2 — Operasional', 'Rina Putri', '2024-10-14', 2750000, 1900000)
]

// ── Detail-screen supplementary sample data (mock; same for any asset) ──────────
export type AssetCondition = 'Baik' | 'Perlu Servis'
export const ASSET_CONDITION_TONE: Record<AssetCondition, BadgeColor> = { 'Baik': 'success', 'Perlu Servis': 'warning' }

export interface AssignmentRecord {
  initials: string
  holder: string
  from: string
  to: string | null
  cond: AssetCondition
  note: string
}
export interface MaintenanceRecord {
  date: string
  type: 'preventive' | 'corrective'
  status: 'selesai' | 'berjalan' | 'dijadwalkan'
  cost: number
  vendor: string
}
export interface DepreciationRow {
  period: string
  open: number
  deprec: number
  close: number
  current: boolean
}

export const MAINTENANCE_TYPE_TONE: Record<MaintenanceRecord['type'], BadgeColor> = { preventive: 'info', corrective: 'warning' }
export const MAINTENANCE_STATUS_TONE: Record<MaintenanceRecord['status'], BadgeColor> = { selesai: 'success', berjalan: 'info', dijadwalkan: 'neutral' }

export const sampleAssignments: AssignmentRecord[] = [
  { initials: 'RP', holder: 'Rina Putri', from: '01 Feb 2026', to: '18 Feb 2026', cond: 'Baik', note: 'Dukungan presentasi cabang' },
  { initials: 'AS', holder: 'Andi Saputra', from: '20 Jan 2026', to: '31 Jan 2026', cond: 'Baik', note: 'Kerja lapangan' },
  { initials: '—', holder: 'Tersedia di gudang', from: '18 Feb 2026', to: null, cond: 'Baik', note: 'Belum ditugaskan' }
]

export const sampleMaintenance: MaintenanceRecord[] = [
  { date: '15 Feb 2026', type: 'preventive', status: 'selesai', cost: 350000, vendor: 'PT Sinar Komputindo' },
  { date: '03 Jan 2026', type: 'corrective', status: 'selesai', cost: 1200000, vendor: 'Teknisi Internal — Eko' },
  { date: '28 Mar 2026', type: 'preventive', status: 'dijadwalkan', cost: 0, vendor: 'PT Sinar Komputindo' }
]

/** Straight-line depreciation schedule derived from an asset's buy price over a useful life. */
export function depreciationSchedule(asset: Asset, life = 4): DepreciationRow[] {
  const startYear = Number(asset.tgl.slice(0, 4)) || 2026
  const currentYear = 2026
  const annual = Math.round(asset.harga / life)
  const rows: DepreciationRow[] = []
  let open = asset.harga
  for (let i = 0; i < life; i++) {
    const period = String(startYear + i)
    const deprec = i === life - 1 ? open : annual
    const close = Math.max(0, open - deprec)
    rows.push({ period, open, deprec, close, current: Number(period) === currentYear })
    open = close
  }
  return rows
}

// ── Bulk-import sample rows (mock validation preview) ───────────────────────────
export interface ImportRow {
  tag: string
  nama: string
  kategori: string
  kantor: string
  tgl: string
  harga: string
  /** field codes with a validation error */
  errFields: string[]
  /** i18n suffix under assets.import.errors.* (null = valid) */
  errKey: string | null
}

const ir = (tag: string, nama: string, kategori: string, kantor: string, tgl: string, harga: string, errFields: string[] = [], errKey: string | null = null): ImportRow =>
  ({ tag, nama, kategori, kantor, tgl, harga, errFields, errKey })

export const IMPORT_SAMPLE_ROWS: ImportRow[] = [
  ir('JKT01-ELK-2026-00040', 'Laptop Asus ExpertBook', 'Elektronik', 'Cabang Jakarta Selatan', '2026-02-10', '14.500.000'),
  ir('JKT01-ELK-2026-00041', 'Monitor Dell P2422H', 'Elektronik', 'Cabang Jakarta Selatan', '2026-02-10', '3.200.000'),
  ir('JKT01-FUR-2026-00012', 'Kursi Kantor Donati', 'Furnitur', 'Outlet Blok M', '2026-01-28', '1.450.000'),
  ir('JKT01-ELK-2026-00041', 'Keyboard Logitech MX', 'Elektronik', 'Cabang Jakarta Selatan', '2026-02-11', '850.000', ['tag'], 'dupTag'),
  ir('JKT01-XYZ-2026-00001', 'Meja Lipat Portable', 'Elektronikk', 'Cabang Jakarta Selatan', '2026-02-12', '600.000', ['kategori'], 'kat'),
  ir('JKT01-KEN-2026-00003', 'Motor Listrik Gesits', 'Kendaraan', 'Cabang Jakarta Selatan', '2026-13-40', '28.000.000', ['tgl'], 'tgl'),
  ir('JKT01-ELK-2026-00042', '', 'Elektronik', 'Cabang Jakarta Selatan', '2026-02-09', '4.200.000', ['nama'], 'nama'),
  ir('JKT01-ITX-2026-00020', 'Switch TP-Link 8 Port', 'Perangkat IT', 'Outlet Kemang', '2026-02-08', '1.900.000'),
  ir('JKT01-ELK-2026-00043', 'Proyektor BenQ MW560', 'Elektronik', 'Cabang Jakarta Selatan', '2026-02-07', 'dua juta', ['harga'], 'harga'),
  ir('JKT01-FUR-2026-00013', 'Lemari Arsip 4 Pintu', 'Furnitur', 'Cabang Jakarta Selatan', '2026-02-06', '2.750.000'),
  ir('JKT01-ELK-2026-00044', 'Printer Canon G3060', 'Elektronik', 'Cabang Jakarta Selatan', '2026-02-05', '3.100.000'),
  ir('JKT01-ITX-2026-00021', 'Access Point Aruba', 'Perangkat IT', 'Outlet Blok M', '2026-02-04', '5.400.000')
]

/** Expected import columns: [name, required]. */
export const IMPORT_COLUMNS: [string, boolean][] = [
  ['asset_tag', false], ['nama', true], ['kategori', true], ['kantor', true],
  ['tgl_beli', true], ['harga', true], ['vendor', false], ['lokasi', false]
]

function clone(list: Asset[]): Asset[] {
  return list.map(x => ({ ...x }))
}

let rows: Asset[] = clone(assetSeed)

export const assetStore = {
  all(): Asset[] {
    return rows
  },
  find(tag: string): Asset | undefined {
    return rows.find(r => r.tag === tag)
  },
  insert(asset: Asset): Asset {
    rows.unshift(asset)
    return asset
  },
  update(tag: string, changes: Partial<Asset>): Asset | undefined {
    const r = rows.find(x => x.tag === tag)
    if (r) Object.assign(r, changes)
    return r
  },
  remove(tag: string): boolean {
    const i = rows.findIndex(r => r.tag === tag)
    if (i === -1) return false
    rows.splice(i, 1)
    return true
  },
  reset(): void {
    rows = clone(assetSeed)
  }
}
