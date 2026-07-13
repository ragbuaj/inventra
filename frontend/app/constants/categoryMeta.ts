import type { FiscalGroup } from '~/types'
import { formatThousands, parseThousands } from '~/utils/format'

// Form-select order (mockup); excludes non_susut.
export const FISCAL_GROUPS: FiscalGroup[] = [
  'kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen'
]

export function isBuildingGroup(g: FiscalGroup | null | undefined): boolean {
  return g === 'bangunan_permanen' || g === 'bangunan_non_permanen'
}

export { formatThousands, parseThousands }
