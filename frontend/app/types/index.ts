export interface Paginated<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

export interface ListQuery {
  search?: string
  limit?: number
  offset?: number
  [key: string]: unknown
}

export interface AuthUser {
  id: string
  name: string
  email: string
  role_id: string
  role_name: string
}

export type BadgeColor = 'primary' | 'success' | 'warning' | 'error' | 'neutral' | 'info'

export interface RowAction {
  label: string
  icon?: string
  color?: BadgeColor | 'secondary'
  separator?: boolean
  disabled?: boolean
  onSelect?: () => void
}

export type RowActions = (row: Record<string, unknown>) => RowAction[]

export interface SortState {
  id: string
  desc: boolean
}

export type TableSorting = SortState[]

export interface Office {
  id: string
  nama: string
  kode: string
  tipe: 'pusat' | 'kanwil' | 'cabang' | 'unit'
  parent_id: string | null
  provinsi: string
  kota: string
  alamat: string
  active: boolean
  created_at: string
}

export interface Floor {
  id: string
  office_id: string
  nama: string
  lantai: number
  created_at: string
}

export interface Room {
  id: string
  floor_id: string
  office_id: string
  nama: string
  kode: string
  created_at: string
}

export interface Employee {
  id: string
  nip: string
  nama: string
  email: string
  telepon: string
  jabatan: string
  departemen: string
  office_id: string
  status: 'active' | 'inactive'
  created_at: string
}

export type AssetClass = 'tangible' | 'intangible'
export type DepreciationMethod = 'straight_line' | 'declining_balance'
export type FiscalGroup
  = | 'kelompok_1' | 'kelompok_2' | 'kelompok_3' | 'kelompok_4'
    | 'bangunan_permanen' | 'bangunan_non_permanen' | 'non_susut'

export interface Category {
  id: string
  name: string
  code: string | null
  parent_id: string | null
  default_depreciation_method: DepreciationMethod | null
  default_useful_life_months: number | null
  default_salvage_rate: string | null
  asset_class: AssetClass
  default_fiscal_group: FiscalGroup | null
  default_fiscal_life_months: number | null
  gl_account_code: string | null
  capitalization_threshold: string | null
  is_active: boolean
  created_at: string
}

export interface User {
  id: string
  nama: string
  email: string
  peran: string
  kantor: string
  pegawai: string
  login: 'email' | 'google'
  status: 'active' | 'inactive' | 'suspended'
  created_at: string
}

export type AssetStatus = 'tersedia' | 'dipinjam' | 'maintenance' | 'dilepas' | 'hilang'

export interface Asset {
  tag: string
  nama: string
  kategori: string
  brand: string
  status: AssetStatus
  kantor: string
  lokasi: string
  /** holder name, or '—' when unassigned */
  holder: string
  /** buy date YYYY-MM-DD */
  tgl: string
  harga: number
  buku: number
}

export interface ReferenceRow {
  id: string
  name: string
  code?: string
  active?: boolean
  [key: string]: unknown
}

export interface TreeNode {
  id: string
  label: string
  icon?: string
  iconBg?: string
  iconColor?: string
  inactive?: boolean
  childCount?: number
  children?: TreeNode[]
}

export interface NavItem {
  labelKey: string
  icon?: string
  to?: string
  permission?: string
  badgeCount?: number
  disabled?: boolean
  children?: NavItem[]
}

export interface NavGroup {
  labelKey: string
  items: NavItem[]
}

export type SearchEntityType = 'aset' | 'pegawai' | 'kantor' | 'user' | 'pengajuan'

export interface SearchItem {
  type: SearchEntityType
  title: string
  sub: string
  status: string | null
  icon: string
  to: string
}

export interface SearchGroup {
  type: SearchEntityType
  labelKey: string
  total: number
  items: SearchItem[]
}

export type OfficeTier = 'pusat' | 'wilayah' | 'office'

export interface MapOffice {
  id: string
  name: string
  code: string
  office_type_name: string | null
  tier: OfficeTier
  province_name: string | null
  city_name: string | null
  address: string | null
  asset_count: number
  latitude: number | null
  longitude: number | null
}

export interface AccountProfile {
  nama: string
  email: string
  telepon: string
  peran: string
  kantor: string
  pegawai: string
  loginMethod: 'email' | 'google'
  joinDate: string
}

export interface AccountSession {
  id: string
  device: string
  meta: string
  icon: string
  current: boolean
}

export interface NotifPrefs {
  approval: boolean
  maint: boolean
  assign: boolean
}
