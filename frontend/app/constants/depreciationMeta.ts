import type { BadgeColor } from '~/types'

/** Depreciation basis: commercial (PSAK 16 / IAS 16) vs fiscal (PMK 72/2023). */
export type DepreciationBasis = 'commercial' | 'fiscal'

/** Lifecycle status of a depreciation period. */
export type PeriodStatus = 'open' | 'computed' | 'closed'

export const PERIOD_STATUS_TONE: Record<PeriodStatus, BadgeColor> = {
  open: 'warning',
  computed: 'info',
  closed: 'neutral'
}

export const BASIS_META: Record<DepreciationBasis, { labelKey: string, refKey: string }> = {
  commercial: { labelKey: 'depreciation.basis.commercial', refKey: 'depreciation.basis.refCommercial' },
  fiscal: { labelKey: 'depreciation.basis.fiscal', refKey: 'depreciation.basis.refFiscal' }
}
