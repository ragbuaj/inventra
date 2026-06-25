import type { BadgeColor } from '~/types'

/** Fixed "today" so relative due-date math is deterministic (mockup uses 2026-06-24). */
export const MAINT_TODAY = '2026-06-24'

export type MaintType = 'preventive' | 'corrective'
export type MaintStatus = 'scheduled' | 'in_progress' | 'completed' | 'cancelled'

/** A data value that carries its own id/en translation (ported from the mockup). */
export type Localized = string | { id: string, en: string }
export function loc(v: Localized, lang: string): string {
  return typeof v === 'string' ? v : (lang === 'en' ? v.en : v.id)
}

export const TYPE_TONE: Record<MaintType, BadgeColor> = { preventive: 'info', corrective: 'warning' }
export const STATUS_TONE: Record<MaintStatus, BadgeColor> = {
  scheduled: 'neutral',
  in_progress: 'info',
  completed: 'success',
  cancelled: 'error'
}
export const MAINT_STATUS_KEYS: MaintStatus[] = ['scheduled', 'in_progress', 'completed', 'cancelled']

export interface ScheduleItem {
  tag: string
  asset: string
  task: Localized
  tipe: MaintType
  /** due date YYYY-MM-DD */
  due: string
  vendor: Localized
}

export interface MaintRecord {
  tag: string
  nama: string
  tipe: MaintType
  kategori: Localized
  /** action date YYYY-MM-DD */
  tanggal: string
  status: MaintStatus
  biaya: number
  vendor: Localized
}

export interface DamageReport {
  tag: string
  nama: string
  /** i18n key suffix under maintenance.problems.* */
  problemKey: string
  desc: string
  /** submitted date YYYY-MM-DD */
  date: string
}

export const scheduleSeed: ScheduleItem[] = [
  { tag: 'JKT01-KEN-2025-00007', asset: 'Toyota Avanza 1.5 G', task: { id: 'Servis berkala 40.000 km', en: 'Scheduled 40,000 km service' }, tipe: 'preventive', due: '2026-06-20', vendor: 'Auto2000' },
  { tag: 'JKT01-ELK-2023-00009', asset: 'AC Daikin FTKC50 · R.301', task: { id: 'Pembersihan filter', en: 'Filter cleaning' }, tipe: 'preventive', due: '2026-06-27', vendor: 'PT Sinar Komputindo' },
  { tag: 'JKT01-ELK-2025-00028', asset: 'Genset Cummins C22 D5', task: { id: 'Inspeksi rutin', en: 'Routine inspection' }, tipe: 'preventive', due: '2026-07-05', vendor: { id: 'Teknisi Internal', en: 'Internal Tech' } },
  { tag: 'JKT01-FUR-2023-00007', asset: 'Meja Rapat Kayu 10-Seat', task: { id: 'Reparasi kaki meja', en: 'Table leg repair' }, tipe: 'corrective', due: '2026-07-12', vendor: 'CV Karya Kayu' },
  { tag: 'JKT01-ITX-2025-00022', asset: 'Switch Cisco Catalyst 1000', task: { id: 'Update firmware', en: 'Firmware update' }, tipe: 'preventive', due: '2026-07-18', vendor: { id: 'Teknisi Internal', en: 'Internal Tech' } }
]

export const recordSeed: MaintRecord[] = [
  { tag: 'JKT01-KEN-2025-00007', nama: 'Toyota Avanza 1.5 G', tipe: 'corrective', kategori: { id: 'Servis Mesin', en: 'Engine Service' }, tanggal: '2026-05-12', status: 'completed', biaya: 2350000, vendor: 'Auto2000' },
  { tag: 'JKT01-ELK-2023-00009', nama: 'AC Daikin FTKC50', tipe: 'preventive', kategori: { id: 'Pembersihan', en: 'Cleaning' }, tanggal: '2026-06-18', status: 'in_progress', biaya: 350000, vendor: 'PT Sinar Komputindo' },
  { tag: 'JKT01-ELK-2026-00001', nama: 'Laptop Dell Latitude 5440', tipe: 'corrective', kategori: { id: 'Penggantian Sparepart', en: 'Part Replacement' }, tanggal: '2026-04-03', status: 'completed', biaya: 1200000, vendor: { id: 'Teknisi Internal — Eko', en: 'Internal — Eko' } },
  { tag: 'JKT01-ELK-2025-00028', nama: 'Genset Cummins C22 D5', tipe: 'preventive', kategori: { id: 'Inspeksi', en: 'Inspection' }, tanggal: '2026-07-05', status: 'scheduled', biaya: 0, vendor: { id: 'Teknisi Internal', en: 'Internal Tech' } },
  { tag: 'JKT01-ELK-2024-00021', nama: 'Printer HP LaserJet Pro', tipe: 'corrective', kategori: { id: 'Penggantian Sparepart', en: 'Part Replacement' }, tanggal: '2026-06-02', status: 'completed', biaya: 680000, vendor: 'PT Sinar Komputindo' },
  { tag: 'JKT01-KEN-2024-00004', nama: 'Honda Vario 160', tipe: 'preventive', kategori: { id: 'Servis Berkala', en: 'Scheduled Service' }, tanggal: '2026-05-28', status: 'cancelled', biaya: 0, vendor: 'Auto2000' }
]

/** Assets the current (staff) user holds — for the damage-report form. */
export const myAssets: { tag: string, nama: string }[] = [
  { tag: 'JKT01-ELK-2026-00005', nama: 'Monitor LG 27UL550' },
  { tag: 'JKT01-ELK-2026-00008', nama: 'Laptop MacBook Air M3' },
  { tag: 'JKT01-ELK-2026-00015', nama: 'Laptop Lenovo ThinkPad E14' },
  { tag: 'JKT01-ITX-2026-00006', nama: 'CCTV Hikvision DS-2CD' },
  { tag: 'JKT01-ELK-2024-00033', nama: 'Printer Epson L3210' }
]

/** Assets available to attach a maintenance note to. */
export const allAssets: { tag: string, nama: string }[] = [
  { tag: 'JKT01-ELK-2026-00001', nama: 'Laptop Dell Latitude 5440' },
  { tag: 'JKT01-ELK-2026-00002', nama: 'Proyektor Epson EB-X51' },
  { tag: 'JKT01-KEN-2025-00007', nama: 'Toyota Avanza 1.5 G' },
  { tag: 'JKT01-ELK-2023-00009', nama: 'AC Daikin FTKC50' },
  { tag: 'JKT01-ELK-2025-00028', nama: 'Genset Cummins C22 D5' },
  { tag: 'JKT01-FUR-2023-00007', nama: 'Meja Rapat Kayu 10-Seat' },
  { tag: 'JKT01-ITX-2025-00022', nama: 'Switch Cisco Catalyst 1000' }
]

export const careCategories = ['Servis Berkala', 'Pembersihan', 'Reparasi', 'Penggantian Sparepart', 'Inspeksi', 'Kalibrasi']
export const vendors = ['Auto2000', 'PT Sinar Komputindo', 'CV Karya Kayu', 'Teknisi Internal — Eko', 'CV Teknologi Nusantara']
/** i18n key suffixes under maintenance.problems.* */
export const problemKeys = ['display', 'dead', 'noise', 'battery', 'connectivity', 'other']

export type DueLevel = 'overdue' | 'today' | 'soon' | 'later'

/** Whole-day difference between a YYYY-MM-DD due date and a base date. */
export function dayDiff(due: string, base: string = MAINT_TODAY): number {
  const d = Date.parse(`${due}T00:00:00Z`)
  const b = Date.parse(`${base}T00:00:00Z`)
  return Math.round((d - b) / 86400000)
}

export function dueLevel(diff: number): DueLevel {
  if (diff < 0) return 'overdue'
  if (diff === 0) return 'today'
  if (diff <= 7) return 'soon'
  return 'later'
}

export const DUE_TONE: Record<DueLevel, BadgeColor> = {
  overdue: 'error',
  today: 'error',
  soon: 'warning',
  later: 'neutral'
}

function cloneRecords(list: MaintRecord[]): MaintRecord[] {
  return list.map(x => ({ ...x }))
}

let records: MaintRecord[] = cloneRecords(recordSeed)
let reports: DamageReport[] = []

export const maintenanceStore = {
  schedule(): ScheduleItem[] {
    return scheduleSeed
  },
  records(): MaintRecord[] {
    return records
  },
  reports(): DamageReport[] {
    return reports
  },
  addRecord(rec: MaintRecord): MaintRecord {
    records = [rec, ...records]
    return rec
  },
  addReport(rep: DamageReport): DamageReport {
    reports = [rep, ...reports]
    return rep
  },
  reset(): void {
    records = cloneRecords(recordSeed)
    reports = []
  }
}
