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
