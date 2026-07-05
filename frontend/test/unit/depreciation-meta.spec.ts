import { describe, it, expect } from 'vitest'
import { PERIOD_STATUS_TONE, BASIS_META } from '~/constants/depreciationMeta'
import type { PeriodStatus, DepreciationBasis } from '~/constants/depreciationMeta'

describe('constants/depreciationMeta', () => {
  it('has a tone for every period status incl. closed:neutral, open:warning, computed:info', () => {
    expect(PERIOD_STATUS_TONE).toEqual({
      open: 'warning',
      computed: 'info',
      closed: 'neutral'
    })
  })

  it('every period status key has a tone', () => {
    const keys: PeriodStatus[] = ['open', 'computed', 'closed']
    for (const k of keys) {
      expect(PERIOD_STATUS_TONE[k]).toBeTruthy()
    }
  })

  it('has BASIS_META entries for commercial and fiscal with labelKey/refKey', () => {
    expect(BASIS_META).toEqual({
      commercial: { labelKey: 'depreciation.basis.commercial', refKey: 'depreciation.basis.refCommercial' },
      fiscal: { labelKey: 'depreciation.basis.fiscal', refKey: 'depreciation.basis.refFiscal' }
    })
  })

  it('every basis key has both labelKey and refKey', () => {
    const keys: DepreciationBasis[] = ['commercial', 'fiscal']
    for (const k of keys) {
      expect(BASIS_META[k].labelKey).toBeTruthy()
      expect(BASIS_META[k].refKey).toBeTruthy()
    }
  })
})
