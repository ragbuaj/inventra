import { describe, it, expect } from 'vitest'
import { TRANSFER_STATUS_TONE, CONDITION_TONE, CONDITION_KEYS } from '~/constants/transferMeta'
import { METHOD_KEYS, METHOD_TONE } from '~/constants/disposalMeta'

describe('constants/transferMeta', () => {
  it('has a tone for every transfer row status incl. returned as error', () => {
    expect(TRANSFER_STATUS_TONE).toEqual({
      approved: 'info',
      in_transit: 'info',
      received: 'success',
      returned: 'error'
    })
  })

  it('has a tone for every condition', () => {
    expect(CONDITION_TONE).toEqual({
      baik: 'success',
      rusak_ringan: 'warning',
      rusak_berat: 'error'
    })
  })

  it('lists condition keys in order baik/rusak_ringan/rusak_berat', () => {
    expect(CONDITION_KEYS).toEqual(['baik', 'rusak_ringan', 'rusak_berat'])
  })
})

describe('constants/disposalMeta', () => {
  it('lists the 4 backend disposal methods', () => {
    expect(METHOD_KEYS).toEqual(['sale', 'auction', 'donation', 'write_off'])
  })

  it('has a tone for every method', () => {
    expect(METHOD_TONE).toEqual({
      sale: 'info',
      auction: 'primary',
      donation: 'success',
      write_off: 'neutral'
    })
  })

  it('every method key has a tone', () => {
    for (const k of METHOD_KEYS) {
      expect(METHOD_TONE[k]).toBeTruthy()
    }
  })
})
