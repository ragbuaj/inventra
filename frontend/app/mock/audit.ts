/**
 * Audit Trail mock fixtures — ported 1:1 from `docs/design/Audit Trail.dc.html`.
 * Imported directly (not via the mock barrel) to avoid clashing `Localized` re-exports.
 */
import type { BadgeColor } from '~/types'

export interface Localized {
  id: string
  en: string
}

export type AuditAction = 'create' | 'update' | 'delete'

export interface AuditDiff {
  field: string
  before: string | null
  after: string | null
}

export interface AuditLog {
  id: number
  /** 'YYYY-MM-DD HH:mm' */
  dt: string
  actor: string
  role: Localized
  action: AuditAction
  entity: string
  ref: string
  summary: Localized
  office: string
  ip: string
  diff: AuditDiff[]
}

export const AUDIT_ACTION_META: Record<AuditAction, { tone: BadgeColor, icon: string }> = {
  create: { tone: 'success', icon: 'i-lucide-plus' },
  update: { tone: 'info', icon: 'i-lucide-pencil' },
  delete: { tone: 'error', icon: 'i-lucide-trash-2' }
}

export const AUDIT_ENTITIES = ['Aset', 'Pengajuan', 'User', 'Peran', 'Field-Permission', 'Master Data', 'Kantor', 'Maintenance', 'Pegawai']

/** Month abbreviations for date formatting, per locale. */
export const AUDIT_MONTHS: Record<'id' | 'en', string[]> = {
  id: ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des'],
  en: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
}

const L = (id: string, en: string): Localized => ({ id, en })
const d = (field: string, before: string | null, after: string | null): AuditDiff => ({ field, before, after })

export const auditSeed: AuditLog[] = [
  { id: 1, dt: '2026-06-24 09:42', actor: 'Dewi Lestari', role: L('Asset Manager', 'Asset Manager'), action: 'update', entity: 'Aset', ref: 'JKT01-ELK-2025-00028', summary: L('Ubah valuasi Genset Cummins C22', 'Update valuation of Genset Cummins C22'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.14', diff: [d('nilai_buku', 'Rp 69.000.000', 'Rp 45.000.000'), d('kondisi', 'Baik', 'Rusak Berat')] },
  { id: 2, dt: '2026-06-24 08:15', actor: 'Andi Saputra', role: L('Asset Manager', 'Asset Manager'), action: 'create', entity: 'Aset', ref: 'JKT01-ELK-2026-00040', summary: L('Tambah Laptop Asus ExpertBook B1', 'Add Laptop Asus ExpertBook B1'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.22', diff: [d('nama', null, 'Laptop Asus ExpertBook B1'), d('kategori', null, 'Elektronik'), d('harga_beli', null, 'Rp 14.500.000')] },
  { id: 3, dt: '2026-06-23 16:30', actor: 'Super Admin', role: L('Superadmin', 'Superadmin'), action: 'update', entity: 'Peran', ref: 'role:operator_gudang', summary: L('Ubah izin peran Operator Gudang', 'Update Warehouse Operator role permissions'), office: 'Kantor Pusat', ip: '10.0.0.2', diff: [d('permissions', '—', '+ aset.label'), d('permissions', '—', '+ penugasan.checkin')] },
  { id: 4, dt: '2026-06-23 14:05', actor: 'Siti Aminah', role: L('Kepala Unit', 'Unit Head'), action: 'update', entity: 'Pengajuan', ref: 'REG-2026-0012', summary: L('Setujui registrasi 12 Laptop', 'Approve registration of 12 laptops'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.5', diff: [d('status', 'Menunggu', 'Disetujui'), d('approver', null, 'Siti Aminah')] },
  { id: 5, dt: '2026-06-23 11:20', actor: 'Bambang Sukasno', role: L('Kepala Kanwil', 'Regional Head'), action: 'delete', entity: 'Aset', ref: 'JKT01-ELK-2024-00021', summary: L('Hapus Printer HP LaserJet (hilang)', 'Delete HP LaserJet Printer (lost)'), office: 'Kanwil DKI Jakarta', ip: '10.10.0.8', diff: [d('nama', 'Printer HP LaserJet Pro', null), d('status', 'Hilang', null)] },
  { id: 6, dt: '2026-06-22 17:48', actor: 'Super Admin', role: L('Superadmin', 'Superadmin'), action: 'create', entity: 'User', ref: 'fajar.nugroho@inventra.go.id', summary: L('Buat akun Fajar Nugroho', 'Create account for Fajar Nugroho'), office: 'Kantor Pusat', ip: '10.0.0.2', diff: [d('email', null, 'fajar.nugroho@inventra.go.id'), d('peran', null, 'Staf'), d('kantor', null, 'Cabang Jakarta Pusat')] },
  { id: 7, dt: '2026-06-22 13:10', actor: 'Rina Putri', role: L('Staf', 'Staff'), action: 'create', entity: 'Pengajuan', ref: 'PMJ-2026-0048', summary: L('Ajukan peminjaman Proyektor Epson', 'Request loan of Epson Projector'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.31', diff: [d('tipe', null, 'Peminjaman'), d('aset', null, 'JKT01-ELK-2026-00002')] },
  { id: 8, dt: '2026-06-22 09:05', actor: 'Dewi Lestari', role: L('Asset Manager', 'Asset Manager'), action: 'update', entity: 'Aset', ref: 'JKT01-ELK-2026-00005', summary: L('Check-out Monitor LG ke Rina Putri', 'Check out Monitor LG to Rina Putri'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.14', diff: [d('status', 'Tersedia', 'Dipinjam'), d('pemegang', '—', 'Rina Putri')] },
  { id: 9, dt: '2026-06-21 15:25', actor: 'Super Admin', role: L('Superadmin', 'Superadmin'), action: 'update', entity: 'Field-Permission', ref: 'aset.harga_beli', summary: L('Batasi field harga_beli', 'Restrict harga_beli field'), office: 'Kantor Pusat', ip: '10.0.0.2', diff: [d('staf.view', 'true', 'false'), d('kaunit.view', 'true', 'false')] },
  { id: 10, dt: '2026-06-21 10:40', actor: 'Andi Saputra', role: L('Asset Manager', 'Asset Manager'), action: 'update', entity: 'Master Data', ref: 'vendor:sinarkom', summary: L('Ubah kontak PT Sinar Komputindo', 'Update PT Sinar Komputindo contact'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.22', diff: [d('telepon', '021-5550100', '021-5550123')] },
  { id: 11, dt: '2026-06-20 16:12', actor: 'Siti Aminah', role: L('Kepala Unit', 'Unit Head'), action: 'update', entity: 'Pengajuan', ref: 'PMJ-2026-0041', summary: L('Tolak peminjaman kendaraan dinas', 'Reject official vehicle loan'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.5', diff: [d('status', 'Menunggu', 'Ditolak'), d('catatan', null, 'Kendaraan terjadwal untuk unit lain')] },
  { id: 12, dt: '2026-06-20 09:30', actor: 'Super Admin', role: L('Superadmin', 'Superadmin'), action: 'create', entity: 'Kantor', ref: 'JKT01-KM', summary: L('Tambah Outlet Kemang', 'Add Outlet Kemang'), office: 'Kantor Pusat', ip: '10.0.0.2', diff: [d('nama', null, 'Outlet Kemang'), d('jenis', null, 'Outlet'), d('induk', null, 'Cabang Jakarta Selatan')] },
  { id: 13, dt: '2026-06-19 14:00', actor: 'Dewi Lestari', role: L('Asset Manager', 'Asset Manager'), action: 'create', entity: 'Maintenance', ref: 'MNT-2026-0077', summary: L('Catat servis mesin Toyota Avanza', 'Log engine service for Toyota Avanza'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.14', diff: [d('tipe', null, 'Corrective'), d('biaya', null, 'Rp 2.350.000'), d('vendor', null, 'Auto2000')] },
  { id: 14, dt: '2026-06-19 08:22', actor: 'Rina Putri', role: L('Staf', 'Staff'), action: 'update', entity: 'Pegawai', ref: '199205202012012002', summary: L('Perbarui nomor telepon', 'Update phone number'), office: 'Cabang Jakarta Selatan', ip: '10.20.1.31', diff: [d('telepon', '0813-9087-1100', '0813-9087-1122')] }
]
