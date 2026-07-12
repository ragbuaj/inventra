import { describe, it, expect } from 'vitest'
import { REQUEST_TYPE_KEYS, TYPE_META, STATUS_TONE, STATUS_FILTERS } from '~/constants/approvalMeta'

describe('constants/approvalMeta', () => {
  it('covers exactly the 7 submittable backend request types', () => {
    expect(REQUEST_TYPE_KEYS).toEqual(['asset_create', 'asset_disposal', 'asset_transfer', 'assignment', 'maintenance', 'valuation_exclusion', 'asset_import'])
  })

  it('marks disposal and valuation exclusion as sensitive', () => {
    expect(TYPE_META.asset_disposal.sensitive).toBe(true)
    expect(TYPE_META.valuation_exclusion.sensitive).toBe(true)
    expect(TYPE_META.asset_create.sensitive).toBe(false)
    expect(TYPE_META.asset_transfer.sensitive).toBe(false)
    expect(TYPE_META.assignment.sensitive).toBe(false)
    expect(TYPE_META.maintenance.sensitive).toBe(false)
    expect(TYPE_META.asset_import.sensitive).toBe(false)
  })

  it('has a tone for every status incl. cancelled and a cancelled filter tab', () => {
    expect(STATUS_TONE.cancelled).toBe('neutral')
    expect(STATUS_FILTERS).toEqual(['pending', 'approved', 'rejected', 'cancelled', 'all'])
  })

  it('every type has an icon and tone', () => {
    for (const k of REQUEST_TYPE_KEYS) {
      expect(TYPE_META[k].icon).toMatch(/^i-lucide-/)
      expect(TYPE_META[k].tone).toBeTruthy()
    }
  })
})
