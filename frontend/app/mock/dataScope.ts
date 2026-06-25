/**
 * Data Scope mock fixtures — ported 1:1 from `docs/design/Data Scope.dc.html`.
 * Imported directly (not via the mock barrel) to avoid clashing `Localized` re-exports.
 */

export interface Localized {
  id: string
  en: string
}

export type ScopeLevel = 'global' | 'office_subtree' | 'office' | 'own'
export type ScopeTone = 'info' | 'primary' | 'warning' | 'neutral'

export interface ScopeLevelDef {
  key: ScopeLevel
  tone: ScopeTone
  desc: Localized
}

export interface ScopeModuleDef {
  key: string
  label: Localized
}

export interface ScopeRole {
  key: string
  nama: Localized
  sub: Localized
  /** role-wide default level */
  def: ScopeLevel
  /** per-module overrides (absent → inherits `def`) */
  ov: Record<string, ScopeLevel>
}

export const SCOPE_LEVELS: Record<ScopeLevel, ScopeLevelDef> = {
  global: { key: 'global', tone: 'info', desc: { id: 'Semua data lintas kantor', en: 'All data across offices' } },
  office_subtree: { key: 'office_subtree', tone: 'primary', desc: { id: 'Kantor sendiri + seluruh turunannya', en: 'Own office + all its descendants' } },
  office: { key: 'office', tone: 'warning', desc: { id: 'Hanya kantor sendiri', en: 'Own office only' } },
  own: { key: 'own', tone: 'neutral', desc: { id: 'Hanya data miliknya', en: 'Only their own data' } }
}

export const SCOPE_LEVEL_KEYS: ScopeLevel[] = ['global', 'office_subtree', 'office', 'own']

export const DATA_SCOPE_MODULES: ScopeModuleDef[] = [
  { key: 'aset', label: { id: 'Aset', en: 'Assets' } },
  { key: 'pengajuan', label: { id: 'Pengajuan', en: 'Requests' } },
  { key: 'maintenance', label: { id: 'Maintenance', en: 'Maintenance' } },
  { key: 'master', label: { id: 'Master Data', en: 'Master Data' } },
  { key: 'laporan', label: { id: 'Laporan', en: 'Reports' } }
]

const seed: ScopeRole[] = [
  { key: 'superadmin', nama: { id: 'Superadmin', en: 'Superadmin' }, sub: { id: 'Akses penuh', en: 'Full access' }, def: 'global', ov: {} },
  { key: 'kakanwil', nama: { id: 'Kepala Kanwil', en: 'Regional Head' }, sub: { id: 'Lingkup wilayah', en: 'Region scope' }, def: 'office_subtree', ov: {} },
  { key: 'kaunit', nama: { id: 'Kepala Unit', en: 'Unit Head' }, sub: { id: 'Lingkup kantor', en: 'Office scope' }, def: 'office', ov: {} },
  { key: 'manager', nama: { id: 'Manager', en: 'Manager' }, sub: { id: 'Operasional aset', en: 'Asset operations' }, def: 'office', ov: { aset: 'office_subtree', pengajuan: 'own' } },
  { key: 'staf', nama: { id: 'Staf', en: 'Staff' }, sub: { id: 'Data miliknya', en: 'Own data' }, def: 'own', ov: {} },
  { key: 'auditor', nama: { id: 'Auditor Internal', en: 'Internal Auditor' }, sub: { id: 'Baca-saja', en: 'Read-only' }, def: 'office', ov: { aset: 'office_subtree', laporan: 'office_subtree' } }
]

function clone(roles: ScopeRole[]): ScopeRole[] {
  return roles.map(r => ({ ...r, ov: { ...r.ov } }))
}

const roles: ScopeRole[] = clone(seed)

export const dataScopeStore = {
  all(): ScopeRole[] {
    return roles
  },
  replace(next: ScopeRole[]): void {
    roles.length = 0
    for (const r of clone(next)) roles.push(r)
  },
  reset(): void {
    roles.length = 0
    for (const r of clone(seed)) roles.push(r)
  }
}
