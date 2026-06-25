import type { BadgeColor } from '~/types'

export type AssignmentStatus = 'active' | 'returned'
export type AssetCondition = 'baik' | 'ringan' | 'berat'

export interface Assignment {
  id: string
  tag: string
  nama: string
  pemegang: string
  /** recipient initials */
  ini: string
  /** borrow date YYYY-MM-DD */
  pinjam: string
  /** return date YYYY-MM-DD, '' while active */
  kembali: string
  status: AssignmentStatus
  kondisi: AssetCondition
}

export interface AvailableAsset {
  tag: string
  nama: string
}

export interface Recipient {
  name: string
  /** i18n key suffix under assignment.roles.* */
  roleKey: string
  ini: string
}

/** Condition → semantic badge tone (ported from the mockup's KOND map). */
export const CONDITION_TONE: Record<AssetCondition, BadgeColor> = {
  baik: 'success',
  ringan: 'warning',
  berat: 'error'
}
export const CONDITION_KEYS: AssetCondition[] = ['baik', 'ringan', 'berat']

/** Assets eligible for check-out (status "tersedia" in the catalog). */
export const availableSeed: AvailableAsset[] = [
  { tag: 'JKT01-ELK-2026-00001', nama: 'Laptop Dell Latitude 5440' },
  { tag: 'JKT01-ITX-2025-00014', nama: 'Router MikroTik RB4011' },
  { tag: 'JKT01-ELK-2025-00018', nama: 'UPS APC Smart-UPS 1500' },
  { tag: 'JKT01-FUR-2026-00003', nama: 'Kursi Ergonomis Ergotec' },
  { tag: 'JKT01-ITX-2025-00022', nama: 'Switch Cisco Catalyst 1000' },
  { tag: 'JKT01-ELK-2025-00025', nama: 'Scanner Fujitsu fi-7160' }
]

export const recipientSeed: Recipient[] = [
  { name: 'Andi Saputra', roleKey: 'operations', ini: 'AS' },
  { name: 'Rina Putri', roleKey: 'it', ini: 'RP' },
  { name: 'Budi Hartono', roleKey: 'general', ini: 'BH' },
  { name: 'Dewi Lestari', roleKey: 'manager', ini: 'DL' },
  { name: 'Eko Prasetyo', roleKey: 'field', ini: 'EP' }
]

export const assignmentSeed: Assignment[] = [
  { id: 'a1', tag: 'JKT01-ELK-2026-00002', nama: 'Proyektor Epson EB-X51', pemegang: 'Andi Saputra', ini: 'AS', pinjam: '2026-01-20', kembali: '', status: 'active', kondisi: 'baik' },
  { id: 'a2', tag: 'JKT01-ELK-2026-00005', nama: 'Monitor LG 27UL550', pemegang: 'Rina Putri', ini: 'RP', pinjam: '2026-02-02', kembali: '', status: 'active', kondisi: 'baik' },
  { id: 'a3', tag: 'JKT01-KEN-2024-00004', nama: 'Honda Vario 160', pemegang: 'Budi Hartono', ini: 'BH', pinjam: '2026-01-12', kembali: '', status: 'active', kondisi: 'baik' },
  { id: 'r1', tag: 'JKT01-ELK-2024-00030', nama: 'Televisi Samsung 55" Crystal', pemegang: 'Andi Saputra', ini: 'AS', pinjam: '2025-12-03', kembali: '2025-12-18', status: 'returned', kondisi: 'baik' },
  { id: 'r2', tag: 'JKT01-FUR-2025-00011', nama: 'Meja Kerja Ergonomis', pemegang: 'Dewi Lestari', ini: 'DL', pinjam: '2025-11-10', kembali: '2025-12-02', status: 'returned', kondisi: 'baik' },
  { id: 'r3', tag: 'JKT01-ELK-2026-00008', nama: 'Laptop MacBook Air M3', pemegang: 'Rina Putri', ini: 'RP', pinjam: '2026-02-01', kembali: '2026-02-18', status: 'returned', kondisi: 'ringan' }
]

function clone(list: Assignment[]): Assignment[] {
  return list.map(x => ({ ...x }))
}

let rows: Assignment[] = clone(assignmentSeed)
let lent = new Set<string>()
let seq = 1

export const assignmentStore = {
  all(): Assignment[] {
    return rows
  },
  find(id: string): Assignment | undefined {
    return rows.find(r => r.id === id)
  },
  /** Assets still available = seed pool minus those currently lent out via check-out. */
  available(): AvailableAsset[] {
    return availableSeed.filter(a => !lent.has(a.tag))
  },
  checkout(input: { tag: string, nama: string, pemegang: string, ini: string, pinjam: string, kondisi: AssetCondition }): Assignment {
    const row: Assignment = { id: `n${seq++}`, ...input, kembali: '', status: 'active' }
    rows.unshift(row)
    lent.add(input.tag)
    return row
  },
  checkin(id: string, input: { kembali: string, kondisi: AssetCondition }): Assignment | undefined {
    const row = rows.find(r => r.id === id)
    if (!row) return undefined
    row.status = 'returned'
    row.kembali = input.kembali
    row.kondisi = input.kondisi
    lent.delete(row.tag)
    return row
  },
  reset(): void {
    rows = clone(assignmentSeed)
    lent = new Set<string>()
    seq = 1
  }
}
