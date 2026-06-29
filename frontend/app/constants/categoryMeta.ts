import type { FiscalGroup } from '~/types'

// Form-select order (mockup); excludes non_susut.
export const FISCAL_GROUPS: FiscalGroup[] = [
  'kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen'
]

export function isBuildingGroup(g: FiscalGroup | null | undefined): boolean {
  return g === 'bangunan_permanen' || g === 'bangunan_non_permanen'
}

// Display a numeric string with id-ID thousands grouping ('1000000' → '1.000.000').
export function formatThousands(v: string | number | null | undefined): string {
  const s = String(v ?? '').replace(/\D/g, '')
  if (!s) return ''
  return Number(s).toLocaleString('id-ID')
}

// Strip grouping back to a bare digit string ('1.000.000' → '1000000').
export function parseThousands(v: string | null | undefined): string {
  return String(v ?? '').replace(/\D/g, '')
}
