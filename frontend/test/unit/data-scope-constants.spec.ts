import { describe, it, expect } from 'vitest'
import { SCOPE_LEVEL_KEYS, SCOPE_LEVEL_TONE } from '~/constants/dataScope'

describe('data-scope constants', () => {
  it('has the 4 scope levels in order', () => {
    expect(SCOPE_LEVEL_KEYS).toEqual(['global', 'office_subtree', 'office', 'own'])
  })
  it('maps every level to a tone (mockup-faithful)', () => {
    expect(SCOPE_LEVEL_TONE).toEqual({
      global: 'info', office_subtree: 'primary', office: 'warning', own: 'neutral'
    })
  })
})
