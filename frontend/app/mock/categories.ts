import type { Category, FiscalGroup } from '~/types'
import { createStore } from './helpers'

export const categorySeed: Category[] = [
  { id: 'c-it', name: 'Perangkat IT', code: 'ITX', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3.00', capitalization_threshold: '1000000', is_active: true, created_at: '2026-01-05' },
  { id: 'c-laptop', name: 'Komputer & Laptop', code: 'ELK', parent_id: 'c-it', default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3.01', capitalization_threshold: '1000000', is_active: true, created_at: '2026-01-06' },
  { id: 'c-vehicle', name: 'Kendaraan Bermotor', code: 'KEN', parent_id: null, default_depreciation_method: 'declining_balance', default_useful_life_months: 96, default_salvage_rate: '10', asset_class: 'tangible', default_fiscal_group: 'kelompok_2', default_fiscal_life_months: 96, gl_account_code: '1.2.4.00', capitalization_threshold: '10000000', is_active: true, created_at: '2026-01-07' },
  { id: 'c-building', name: 'Bangunan Kantor', code: 'BGN', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 240, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'bangunan_permanen', default_fiscal_life_months: 240, gl_account_code: '1.2.1.00', capitalization_threshold: '50000000', is_active: true, created_at: '2026-01-08' },
  { id: 'c-atm', name: 'Mesin ATM', code: 'ATM', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 96, default_salvage_rate: '5', asset_class: 'tangible', default_fiscal_group: 'kelompok_2', default_fiscal_life_months: 96, gl_account_code: '1.2.3.05', capitalization_threshold: '25000000', is_active: true, created_at: '2026-01-09' },
  { id: 'c-furniture', name: 'Mebel & Inventaris Kantor', code: 'MBL', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.5.00', capitalization_threshold: '1000000', is_active: true, created_at: '2026-01-10' },
  { id: 'c-software', name: 'Software / Lisensi', code: 'SFT', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'intangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.6.00', capitalization_threshold: '5000000', is_active: true, created_at: '2026-01-11' },
  { id: 'c-network', name: 'Peralatan Jaringan (Legacy)', code: 'NET', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3.09', capitalization_threshold: '1000000', is_active: false, created_at: '2026-01-12' }
]

export const categoryStore = createStore<Category>(categorySeed)

// Form-select order (mockup); excludes non_susut.
export const FISCAL_GROUPS: FiscalGroup[] = [
  'kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen'
]

export function isBuildingGroup(g: FiscalGroup | null | undefined): boolean {
  return g === 'bangunan_permanen' || g === 'bangunan_non_permanen'
}

// Display a numeric string with id-ID thousands grouping ('1000000' → '1.000.000').
export function formatThousands(v: string | number | null | undefined): string {
  const n = Number(String(v ?? '').replace(/\D/g, ''))
  return n ? n.toLocaleString('id-ID') : ''
}

// Strip grouping back to a bare digit string ('1.000.000' → '1000000').
export function parseThousands(v: string | null | undefined): string {
  return String(v ?? '').replace(/\D/g, '')
}
