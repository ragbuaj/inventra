import type { BadgeColor } from '~/types'

export type ReqType = 'registrasi' | 'penghapusan' | 'peminjaman' | 'maintenance' | 'valuasi'
export type ReqStatus = 'pending' | 'approved' | 'rejected'
export type TimelineAction = 'submitted' | 'approved' | 'rejected'

/** A data value carrying its own id/en translation (ported from the mockup). */
export type Localized = string | { id: string, en: string }
export function loc(v: Localized, lang: string): string {
  return typeof v === 'string' ? v : (lang === 'en' ? v.en : v.id)
}

export interface SummaryRow { label: Localized, value: Localized }
export interface DiffRow { label: Localized, before: Localized, after: Localized }
export interface TimelineEntry { action: TimelineAction, actor: Localized, role: Localized, date: string, note: Localized }

export interface ApprovalRequest {
  id: string
  tipe: ReqType
  judul: string
  pengaju: string
  role: Localized
  kantor: string
  ini: string
  tgl: string
  status: ReqStatus
  summary?: SummaryRow[]
  diff?: DiffRow[]
  alasan: Localized
  files: string[]
  timeline: TimelineEntry[]
}

export const TYPE_META: Record<ReqType, { icon: string, tone: BadgeColor, sensitive: boolean }> = {
  registrasi: { icon: 'i-lucide-package', tone: 'info', sensitive: false },
  penghapusan: { icon: 'i-lucide-trash-2', tone: 'error', sensitive: true },
  peminjaman: { icon: 'i-lucide-clipboard-list', tone: 'primary', sensitive: false },
  maintenance: { icon: 'i-lucide-wrench', tone: 'warning', sensitive: false },
  valuasi: { icon: 'i-lucide-coins', tone: 'warning', sensitive: true }
}
export const REQ_TYPE_KEYS: ReqType[] = ['registrasi', 'penghapusan', 'peminjaman', 'maintenance', 'valuasi']

export const STATUS_TONE: Record<ReqStatus, BadgeColor> = { pending: 'warning', approved: 'success', rejected: 'error' }
export const STATUS_FILTERS: (ReqStatus | 'all')[] = ['pending', 'approved', 'rejected', 'all']

/** The reviewer acting on these requests (a Unit Head, per the mockup). */
export const YOU_ACTOR = { id: 'Anda (Kepala Unit)', en: 'You (Unit Head)' }
export const YOU_ROLE = { id: 'Kepala Unit', en: 'Unit Head' }
/** Fixed "now" stamp for decisions so output is deterministic. */
export const DECIDE_STAMP = '24 Jun 2026 · 09:00'

const MGR = { id: 'Asset Manager', en: 'Asset Manager' }

export const approvalSeed: ApprovalRequest[] = [
  {
    id: 'r1', tipe: 'registrasi', judul: 'Registrasi 12 Laptop Asus ExpertBook B1', pengaju: 'Andi Saputra', role: MGR, kantor: 'Cabang Jakarta Selatan', ini: 'AS', tgl: '22 Jun 2026', status: 'pending',
    summary: [
      { label: { id: 'Jumlah Unit', en: 'Units' }, value: '12 unit' },
      { label: { id: 'Kategori', en: 'Category' }, value: 'Elektronik' },
      { label: { id: 'Brand / Model', en: 'Brand / Model' }, value: 'Asus ExpertBook B1' },
      { label: { id: 'Estimasi Nilai', en: 'Estimated Value' }, value: 'Rp 174.000.000' },
      { label: { id: 'Vendor', en: 'Vendor' }, value: 'CV Teknologi Nusantara' }
    ],
    alasan: { id: 'Pengadaan kuartal II untuk penguatan tim operasional cabang.', en: 'Q2 procurement to strengthen the branch operations team.' },
    files: ['faktur-pengadaan.pdf', 'spesifikasi-teknis.xlsx'],
    timeline: [{ action: 'submitted', actor: 'Andi Saputra', role: MGR, date: '22 Jun 2026 · 09:14', note: '' }]
  },
  {
    id: 'r2', tipe: 'valuasi', judul: 'Pengecualian Valuasi — Genset Cummins C22 D5', pengaju: 'Dewi Lestari', role: MGR, kantor: 'Cabang Jakarta Selatan', ini: 'DL', tgl: '23 Jun 2026', status: 'pending',
    diff: [
      { label: { id: 'Nilai Buku', en: 'Book Value' }, before: 'Rp 69.000.000', after: 'Rp 45.000.000' },
      { label: { id: 'Metode', en: 'Method' }, before: { id: 'Garis Lurus', en: 'Straight Line' }, after: { id: 'Penurunan Nilai', en: 'Impairment' } },
      { label: { id: 'Kondisi', en: 'Condition' }, before: { id: 'Baik', en: 'Good' }, after: { id: 'Rusak Berat', en: 'Major Damage' } }
    ],
    alasan: { id: 'Penurunan kondisi signifikan akibat banjir gudang pada Mei 2026; perlu penyesuaian nilai buku.', en: 'Significant condition decline due to the May 2026 storage flood; book value adjustment needed.' },
    files: ['berita-acara-banjir.pdf', 'foto-kondisi.jpg'],
    timeline: [{ action: 'submitted', actor: 'Dewi Lestari', role: MGR, date: '23 Jun 2026 · 11:02', note: '' }]
  },
  {
    id: 'r3', tipe: 'peminjaman', judul: 'Peminjaman Proyektor Epson EB-X51', pengaju: 'Rina Putri', role: { id: 'Staf IT', en: 'IT Staff' }, kantor: 'Cabang Jakarta Selatan', ini: 'RP', tgl: '23 Jun 2026', status: 'pending',
    summary: [
      { label: { id: 'Aset', en: 'Asset' }, value: 'JKT01-ELK-2026-00002' },
      { label: { id: 'Penerima', en: 'Recipient' }, value: 'Rina Putri' },
      { label: { id: 'Durasi', en: 'Duration' }, value: '24–28 Jun 2026 (5 hari)' },
      { label: { id: 'Keperluan', en: 'Purpose' }, value: { id: 'Presentasi audit internal', en: 'Internal audit presentation' } }
    ],
    alasan: { id: 'Mendukung rangkaian presentasi audit internal pekan ini.', en: 'Supporting this week’s internal audit presentations.' },
    files: [],
    timeline: [{ action: 'submitted', actor: 'Rina Putri', role: { id: 'Staf IT', en: 'IT Staff' }, date: '23 Jun 2026 · 08:40', note: '' }]
  },
  {
    id: 'r4', tipe: 'penghapusan', judul: 'Penghapusan Printer HP LaserJet (Hilang)', pengaju: 'Budi Hartono', role: { id: 'Staf Umum', en: 'General Staff' }, kantor: 'Cabang Jakarta Selatan', ini: 'BH', tgl: '21 Jun 2026', status: 'pending',
    diff: [
      { label: { id: 'Status', en: 'Status' }, before: { id: 'Hilang', en: 'Lost' }, after: { id: 'Dihapus dari inventaris', en: 'Removed from inventory' } },
      { label: { id: 'Nilai Buku', en: 'Book Value' }, before: 'Rp 0', after: '—' }
    ],
    alasan: { id: 'Aset dinyatakan hilang sejak Feb 2026; sudah dilaporkan dan tidak ditemukan.', en: 'Asset declared lost since Feb 2026; already reported and not found.' },
    files: ['berita-acara-kehilangan.pdf'],
    timeline: [{ action: 'submitted', actor: 'Budi Hartono', role: { id: 'Staf Umum', en: 'General Staff' }, date: '21 Jun 2026 · 14:25', note: '' }]
  },
  {
    id: 'r5', tipe: 'maintenance', judul: 'Permintaan Maintenance — AC Daikin R.301', pengaju: 'Eko Prasetyo', role: { id: 'Staf Lapangan', en: 'Field Staff' }, kantor: 'Cabang Jakarta Selatan', ini: 'EP', tgl: '20 Jun 2026', status: 'pending',
    summary: [
      { label: { id: 'Aset', en: 'Asset' }, value: 'JKT01-ELK-2023-00009' },
      { label: { id: 'Masalah', en: 'Issue' }, value: { id: 'Tidak dingin / bunyi kompresor', en: 'Not cooling / compressor noise' } },
      { label: { id: 'Prioritas', en: 'Priority' }, value: { id: 'Tinggi', en: 'High' } },
      { label: { id: 'Estimasi Biaya', en: 'Estimated Cost' }, value: 'Rp 1.500.000' }
    ],
    alasan: { id: 'AC ruang server tidak dingin, berisiko terhadap perangkat.', en: 'Server room AC not cooling, risking the equipment.' },
    files: ['foto-unit.jpg'],
    timeline: [{ action: 'submitted', actor: 'Eko Prasetyo', role: { id: 'Staf Lapangan', en: 'Field Staff' }, date: '20 Jun 2026 · 16:08', note: '' }]
  },
  {
    id: 'r6', tipe: 'registrasi', judul: 'Registrasi Meja Rapat Kayu 10-Seat', pengaju: 'Andi Saputra', role: MGR, kantor: 'Cabang Jakarta Selatan', ini: 'AS', tgl: '15 Jun 2026', status: 'approved',
    summary: [
      { label: { id: 'Jumlah Unit', en: 'Units' }, value: '1 unit' },
      { label: { id: 'Kategori', en: 'Category' }, value: 'Furnitur' },
      { label: { id: 'Nilai', en: 'Value' }, value: 'Rp 14.500.000' },
      { label: { id: 'Vendor', en: 'Vendor' }, value: 'CV Karya Kayu' }
    ],
    alasan: { id: 'Penggantian meja rapat lama yang sudah rusak.', en: 'Replacement for the old, damaged meeting table.' },
    files: ['faktur.pdf'],
    timeline: [
      { action: 'submitted', actor: 'Andi Saputra', role: MGR, date: '15 Jun 2026 · 10:11', note: '' },
      { action: 'approved', actor: YOU_ACTOR, role: YOU_ROLE, date: '16 Jun 2026 · 09:30', note: { id: 'Sesuai anggaran tahunan.', en: 'Within annual budget.' } }
    ]
  },
  {
    id: 'r7', tipe: 'peminjaman', judul: 'Peminjaman Kendaraan Dinas Toyota Hiace', pengaju: 'Budi Hartono', role: { id: 'Staf Umum', en: 'General Staff' }, kantor: 'Cabang Jakarta Selatan', ini: 'BH', tgl: '12 Jun 2026', status: 'rejected',
    summary: [
      { label: { id: 'Aset', en: 'Asset' }, value: 'JKT01-KEN-2026-00002' },
      { label: { id: 'Durasi', en: 'Duration' }, value: '12–20 Jun 2026' },
      { label: { id: 'Keperluan', en: 'Purpose' }, value: { id: 'Perjalanan dinas luar kota', en: 'Out-of-town official trip' } }
    ],
    alasan: { id: 'Perjalanan dinas ke kantor wilayah.', en: 'Official trip to the regional office.' },
    files: [],
    timeline: [
      { action: 'submitted', actor: 'Budi Hartono', role: { id: 'Staf Umum', en: 'General Staff' }, date: '12 Jun 2026 · 13:50', note: '' },
      { action: 'rejected', actor: YOU_ACTOR, role: YOU_ROLE, date: '13 Jun 2026 · 08:15', note: { id: 'Kendaraan sudah dijadwalkan untuk unit lain.', en: 'Vehicle already scheduled for another unit.' } }
    ]
  }
]

function clone(list: ApprovalRequest[]): ApprovalRequest[] {
  return list.map(r => ({ ...r, timeline: r.timeline.map(e => ({ ...e })) }))
}

let rows: ApprovalRequest[] = clone(approvalSeed)

export const approvalStore = {
  all(): ApprovalRequest[] {
    return rows
  },
  find(id: string): ApprovalRequest | undefined {
    return rows.find(r => r.id === id)
  },
  pendingCount(): number {
    return rows.filter(r => r.status === 'pending').length
  },
  decide(id: string, action: 'approved' | 'rejected', note: string): ApprovalRequest | undefined {
    const row = rows.find(r => r.id === id)
    if (!row) return undefined
    row.status = action
    row.timeline = [...row.timeline, { action, actor: YOU_ACTOR, role: YOU_ROLE, date: DECIDE_STAMP, note }]
    return row
  },
  reset(): void {
    rows = clone(approvalSeed)
  }
}
