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
  office_id: string | null
  employee_id?: string | null
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
  parent_id: string | null
  office_type_id: string
  province_id: string | null
  city_id: string | null
  name: string
  code: string
  address: string | null
  is_active: boolean
  latitude: number | null
  longitude: number | null
  created_at: string | null
  updated_at: string | null
}

export interface Floor {
  id: string
  office_id: string
  name: string
  level: number | null
  created_at: string | null
  updated_at: string | null
}

export interface Room {
  id: string
  floor_id: string
  name: string
  code: string | null
  created_at: string | null
  updated_at: string | null
}

export type EmployeeStatus = 'active' | 'inactive' | 'suspended'

export interface Employee {
  id: string
  code: string
  name: string
  email: string | null
  phone: string | null
  department_id: string | null
  position_id: string | null
  office_id: string
  status: EmployeeStatus
  avatar_key?: string | null
  created_at: string | null
  updated_at: string | null
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
  updated_at?: string | null
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

export type AssetStatus = 'available' | 'assigned' | 'under_maintenance'
  | 'in_transfer' | 'retired' | 'disposed' | 'lost'

export interface Asset {
  id: string
  asset_tag: string
  name: string
  category_id: string
  office_id: string
  brand_id?: string | null
  model_id?: string | null
  room_id?: string | null
  unit_id?: string | null
  vendor_id?: string | null
  current_holder_employee_id?: string | null
  created_by_id?: string | null
  status: AssetStatus
  asset_class: AssetClass
  serial_number?: string | null
  purchase_date?: string | null
  purchase_cost?: string | null // absent ⇒ masked by field permission
  book_value?: string | null // absent ⇒ masked
  accumulated_depreciation?: string | null // absent ⇒ masked
  salvage_value?: string | null
  po_number?: string | null
  funding_source?: string | null
  warranty_expiry?: string | null
  capitalized?: boolean
  depreciation_method?: string | null
  useful_life_months?: number | null
  fiscal_group?: string | null
  fiscal_life_months?: number | null
  acquisition_bast_no?: string | null
  excluded_from_valuation?: boolean
  valuation_exclusion_reason?: string | null
  notes?: string | null
  created_at?: string
  updated_at?: string
}

export interface AssetUpdateInput {
  name: string
  category_id: string
  brand_id?: string | null
  model_id?: string | null
  room_id?: string | null
  unit_id?: string | null
  vendor_id?: string | null
  serial_number?: string | null
  po_number?: string | null
  funding_source?: string | null
  purchase_date?: string | null
  warranty_expiry?: string | null
  notes?: string | null
}

export interface AssetCreateInput extends AssetUpdateInput {
  office_id: string
  asset_class: AssetClass
  purchase_cost?: string | null
}

export interface AssetAttachment {
  id: string
  asset_id: string
  kind: string
  original_filename: string
  size_bytes: number
  mime_type: string
  has_thumbnail: boolean
  created_at: string
}

export interface ReferenceRow {
  id: string
  name: string
  code?: string
  is_active?: boolean
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
  permission?: string | string[]
  badgeCount?: number
  disabled?: boolean
  /** Hide this item below the lg breakpoint (e.g. CSV import needs a desktop). */
  desktopOnly?: boolean
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
  hasEmployee: boolean
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

export interface PickerItem {
  id: string
  label: string
  sublabel?: string
}
