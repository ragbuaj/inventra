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
