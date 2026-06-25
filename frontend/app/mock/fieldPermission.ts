/**
 * Field-Permission mock fixtures — ported 1:1 from `docs/design/Field Permission.dc.html`.
 * Imported directly (not via the mock barrel) to avoid clashing `Localized` re-exports.
 */

export interface Localized {
  id: string
  en: string
}

export interface FieldDef {
  code: string
  label: Localized
}

export interface EntityDef {
  key: string
  label: Localized
  fields: FieldDef[]
}

export interface CellRule {
  view: boolean
  edit: boolean
}

/** roleKey → rule. A field present in EntityRules has an explicit rule; absent = follows default. */
export type FieldRule = Record<string, CellRule>
export type EntityRules = Record<string, FieldRule>

export const FIELD_ROLE_KEYS = ['super', 'kakanwil', 'kaunit', 'manager', 'staf'] as const

export const FIELD_ROLE_LABELS: Record<string, Localized> = {
  super: { id: 'Superadmin', en: 'Superadmin' },
  kakanwil: { id: 'Ka. Kanwil', en: 'Reg. Head' },
  kaunit: { id: 'Ka. Unit', en: 'Unit Head' },
  manager: { id: 'Manager', en: 'Manager' },
  staf: { id: 'Staf', en: 'Staff' }
}

const f = (code: string, id: string, en: string): FieldDef => ({ code, label: { id, en } })

export const FIELD_ENTITIES: EntityDef[] = [
  {
    key: 'aset', label: { id: 'Aset', en: 'Assets' }, fields: [
      f('nama', 'Nama aset', 'Asset name'), f('kategori', 'Kategori', 'Category'), f('brand_model', 'Brand / Model', 'Brand / Model'),
      f('kantor', 'Kantor', 'Office'), f('lokasi', 'Lokasi', 'Location'), f('vendor', 'Vendor', 'Vendor'),
      f('tanggal_beli', 'Tanggal beli', 'Buy date'), f('harga_beli', 'Harga beli', 'Buy price'), f('nilai_buku', 'Nilai buku', 'Book value'),
      f('metode_depresiasi', 'Metode depresiasi', 'Depreciation method'), f('kondisi', 'Kondisi', 'Condition')
    ]
  },
  {
    key: 'pegawai', label: { id: 'Pegawai', en: 'Employee' }, fields: [
      f('nip', 'NIP', 'NIP'), f('nama', 'Nama', 'Name'), f('departemen', 'Departemen', 'Department'), f('jabatan', 'Jabatan', 'Title'),
      f('kantor', 'Kantor', 'Office'), f('email', 'Email', 'Email'), f('telepon', 'Telepon', 'Phone'), f('gaji_pokok', 'Gaji pokok', 'Base salary')
    ]
  },
  {
    key: 'user', label: { id: 'User', en: 'User' }, fields: [
      f('nama', 'Nama', 'Name'), f('email', 'Email', 'Email'), f('peran', 'Peran', 'Role'), f('kantor', 'Kantor penempatan', 'Assigned office'),
      f('pegawai_tertaut', 'Pegawai tertaut', 'Linked employee'), f('status', 'Status', 'Status'), f('metode_login', 'Metode login', 'Login method')
    ]
  },
  {
    key: 'pengajuan', label: { id: 'Pengajuan', en: 'Request' }, fields: [
      f('tipe', 'Tipe', 'Type'), f('pengaju', 'Pengaju', 'Requester'), f('kantor', 'Kantor', 'Office'),
      f('nilai_sebelum', 'Nilai sebelum', 'Value before'), f('nilai_sesudah', 'Nilai sesudah', 'Value after'),
      f('alasan', 'Alasan', 'Reason'), f('lampiran', 'Lampiran', 'Attachment'), f('status', 'Status', 'Status')
    ]
  }
]

const ve = (view: boolean, edit: boolean): CellRule => ({ view, edit })

/** Build a field rule from per-role [view, edit] in FIELD_ROLE_KEYS order. */
function rule(s: CellRule, ka: CellRule, ku: CellRule, m: CellRule, st: CellRule): FieldRule {
  return { super: s, kakanwil: ka, kaunit: ku, manager: m, staf: st }
}

const seed: Record<string, EntityRules> = {
  aset: {
    harga_beli: rule(ve(true, true), ve(false, false), ve(false, false), ve(true, false), ve(false, false)),
    nilai_buku: rule(ve(true, true), ve(false, false), ve(false, false), ve(false, false), ve(false, false))
  },
  pegawai: {
    gaji_pokok: rule(ve(true, true), ve(false, false), ve(false, false), ve(false, false), ve(false, false))
  },
  user: {},
  pengajuan: {
    nilai_sebelum: rule(ve(true, true), ve(true, false), ve(true, false), ve(false, false), ve(false, false)),
    nilai_sesudah: rule(ve(true, true), ve(true, false), ve(true, false), ve(false, false), ve(false, false))
  }
}

function cloneRules(rules: EntityRules): EntityRules {
  const out: EntityRules = {}
  for (const [fc, fr] of Object.entries(rules)) {
    out[fc] = {}
    for (const [rk, cr] of Object.entries(fr)) out[fc][rk] = { ...cr }
  }
  return out
}

let store: Record<string, EntityRules> = {}
function reseed(): void {
  const next: Record<string, EntityRules> = {}
  for (const [ek, rules] of Object.entries(seed)) next[ek] = cloneRules(rules)
  store = next
}
reseed()

export const fieldPermStore = {
  get(entityKey: string): EntityRules {
    return cloneRules(store[entityKey] ?? {})
  },
  set(entityKey: string, rules: EntityRules): void {
    store[entityKey] = cloneRules(rules)
  },
  reset(): void {
    reseed()
  }
}
