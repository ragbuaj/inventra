export type ReportKey = 'aset' | 'depr' | 'util' | 'biaya'
export const REPORT_KEYS: ReportKey[] = ['aset', 'depr', 'util', 'biaya']

export const REPORT_ICON: Record<ReportKey, string> = {
  aset: 'i-lucide-package',
  depr: 'i-lucide-trending-down',
  util: 'i-lucide-gauge',
  biaya: 'i-lucide-receipt'
}

export const REPORT_CATEGORIES = ['Elektronik', 'Furnitur', 'Kendaraan', 'Perangkat IT']
export const REPORT_OFFICES = ['Cabang Jakarta Selatan', 'Outlet Blok M', 'Outlet Kemang']
/** Status keys shared with the assets catalog (labels via assets.status.*). */
export const REPORT_STATUS_KEYS = ['tersedia', 'dipinjam', 'maintenance', 'dilepas', 'hilang'] as const
export type ReportStatusKey = typeof REPORT_STATUS_KEYS[number]

export function rp(v: number): string {
  return `Rp ${v.toLocaleString('id-ID')}`
}
export function rpJt(v: number): string {
  return `Rp ${(v / 1_000_000).toLocaleString('id-ID', { maximumFractionDigits: 1 })} Jt`
}

export interface AsetRow { kode: string, nama: string, kat: string, status: ReportStatusKey, harga: number, akum: number, buku: number }
export const ASET_ROWS: AsetRow[] = [
  { kode: 'JKT01-ELK-2026-00001', nama: 'Laptop Dell Latitude 5440', kat: 'Elektronik', status: 'tersedia', harga: 18500000, akum: 2300000, buku: 16200000 },
  { kode: 'JKT01-ELK-2026-00002', nama: 'Proyektor Epson EB-X51', kat: 'Elektronik', status: 'dipinjam', harga: 7200000, akum: 700000, buku: 6500000 },
  { kode: 'JKT01-KEN-2025-00007', nama: 'Toyota Avanza 1.5 G', kat: 'Kendaraan', status: 'maintenance', harga: 235000000, akum: 37000000, buku: 198000000 },
  { kode: 'JKT01-FUR-2025-00011', nama: 'Meja Kerja Ergonomis', kat: 'Furnitur', status: 'tersedia', harga: 2400000, akum: 500000, buku: 1900000 },
  { kode: 'JKT01-ITX-2025-00014', nama: 'Router MikroTik RB4011', kat: 'Perangkat IT', status: 'tersedia', harga: 2700000, akum: 400000, buku: 2300000 },
  { kode: 'JKT01-ELK-2025-00028', nama: 'Genset Cummins C22 D5', kat: 'Elektronik', status: 'tersedia', harga: 78000000, akum: 9000000, buku: 69000000 },
  { kode: 'JKT01-ELK-2025-00018', nama: 'UPS APC Smart-UPS 1500', kat: 'Elektronik', status: 'tersedia', harga: 6700000, akum: 800000, buku: 5900000 },
  { kode: 'JKT01-KEN-2024-00004', nama: 'Honda Vario 160', kat: 'Kendaraan', status: 'dipinjam', harga: 28500000, akum: 6500000, buku: 22000000 }
]

export interface DeprRow { period: string, opening: number, deprec: number, closing: number }
export const DEPR_ROWS: DeprRow[] = [
  { period: '2024', opening: 420000000, deprec: 84000000, closing: 336000000 },
  { period: '2025', opening: 336000000, deprec: 84000000, closing: 252000000 },
  { period: '2026', opening: 252000000, deprec: 84000000, closing: 168000000 },
  { period: '2027', opening: 168000000, deprec: 76000000, closing: 92000000 },
  { period: '2028', opening: 92000000, deprec: 56000000, closing: 36000000 },
  { period: '2029', opening: 36000000, deprec: 24000000, closing: 12000000 }
]

export interface UtilRow { nama: string, kat: string, hari: number, pinjam: number, util: number }
export const UTIL_ROWS: UtilRow[] = [
  { nama: 'Proyektor Epson EB-X51', kat: 'Elektronik', hari: 142, pinjam: 18, util: 78 },
  { nama: 'MacBook Air M3', kat: 'Elektronik', hari: 134, pinjam: 14, util: 73 },
  { nama: 'Monitor LG 27UL550', kat: 'Elektronik', hari: 120, pinjam: 12, util: 66 },
  { nama: 'Toyota Avanza 1.5 G', kat: 'Kendaraan', hari: 96, pinjam: 9, util: 53 },
  { nama: 'Honda Vario 160', kat: 'Kendaraan', hari: 88, pinjam: 7, util: 48 },
  { nama: 'Kursi Ergonomis Ergotec', kat: 'Furnitur', hari: 40, pinjam: 3, util: 22 }
]

export interface BiayaRow { nama: string, kat: string, tipe: 'Preventive' | 'Corrective', n: number, biaya: number }
export const BIAYA_ROWS: BiayaRow[] = [
  { nama: 'Toyota Avanza 1.5 G', kat: 'Kendaraan', tipe: 'Corrective', n: 3, biaya: 4200000 },
  { nama: 'AC Daikin FTKC50', kat: 'Elektronik', tipe: 'Preventive', n: 5, biaya: 1750000 },
  { nama: 'Genset Cummins C22', kat: 'Elektronik', tipe: 'Preventive', n: 2, biaya: 900000 },
  { nama: 'Laptop Dell Latitude', kat: 'Elektronik', tipe: 'Corrective', n: 1, biaya: 1200000 },
  { nama: 'Honda Vario 160', kat: 'Kendaraan', tipe: 'Preventive', n: 2, biaya: 600000 }
]

export const ALL = '__all__'

function sum<T>(rows: T[], pick: (r: T) => number): number {
  return rows.reduce((a, r) => a + pick(r), 0)
}
function groupSum<T>(rows: T[], key: (r: T) => string, pick: (r: T) => number): Record<string, number> {
  const out: Record<string, number> = {}
  for (const r of rows) out[key(r)] = (out[key(r)] ?? 0) + pick(r)
  return out
}

export interface AsetReport { kind: 'aset', rows: AsetRow[], totalHarga: number, totalAkum: number, totalBuku: number, byCategory: Record<string, number> }
export interface DeprReport { kind: 'depr', rows: DeprRow[], totalDeprec: number }
export interface UtilReport { kind: 'util', rows: UtilRow[], avg: number, totalHari: number, totalPinjam: number, loaned: number, avgByCategory: Record<string, number> }
export interface BiayaReport { kind: 'biaya', rows: BiayaRow[], total: number, preventive: number, corrective: number, totalN: number, byCategory: Record<string, number> }
export type ReportResult = AsetReport | DeprReport | UtilReport | BiayaReport

/** Pure report computation (ported from the mockup's build()). */
export function computeReport(report: ReportKey, filters: { kat?: string, status?: string }): ReportResult {
  const kat = filters.kat ?? ALL
  const status = filters.status ?? ALL
  const passKat = (k: string) => kat === ALL || k === kat
  const passStatus = (s: string) => status === ALL || s === status

  if (report === 'aset') {
    const rows = ASET_ROWS.filter(a => passKat(a.kat) && passStatus(a.status))
    return {
      kind: 'aset',
      rows,
      totalHarga: sum(rows, r => r.harga),
      totalAkum: sum(rows, r => r.akum),
      totalBuku: sum(rows, r => r.buku),
      byCategory: groupSum(rows, r => r.kat, r => r.buku)
    }
  }
  if (report === 'depr') {
    return { kind: 'depr', rows: DEPR_ROWS, totalDeprec: sum(DEPR_ROWS, r => r.deprec) }
  }
  if (report === 'util') {
    const rows = UTIL_ROWS.filter(a => passKat(a.kat))
    const avg = rows.length ? Math.round(sum(rows, r => r.util) / rows.length) : 0
    const grp = groupSum(rows, r => r.kat, r => r.util)
    const cnt = groupSum(rows, r => r.kat, () => 1)
    const avgByCategory: Record<string, number> = {}
    for (const k of Object.keys(grp)) avgByCategory[k] = Math.round(grp[k]! / cnt[k]!)
    return {
      kind: 'util',
      rows,
      avg,
      totalHari: sum(rows, r => r.hari),
      totalPinjam: sum(rows, r => r.pinjam),
      loaned: rows.filter(r => r.util > 0).length,
      avgByCategory
    }
  }
  const rows = BIAYA_ROWS.filter(a => passKat(a.kat))
  return {
    kind: 'biaya',
    rows,
    total: sum(rows, r => r.biaya),
    preventive: sum(rows.filter(r => r.tipe === 'Preventive'), r => r.biaya),
    corrective: sum(rows.filter(r => r.tipe === 'Corrective'), r => r.biaya),
    totalN: sum(rows, r => r.n),
    byCategory: groupSum(rows, r => r.kat, r => r.biaya)
  }
}

export function reportHasData(r: ReportResult): boolean {
  return r.rows.length > 0
}
