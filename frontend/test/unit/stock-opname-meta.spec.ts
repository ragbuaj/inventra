import { describe, it, expect } from 'vitest'
import { SESSION_STATUS_TONE, ITEM_RESULT_TONE, ITEM_RESULT_KEYS, RESULT_ACTION } from '~/constants/stockOpnameMeta'

describe('stockOpnameMeta', () => {
  it('has a tone for every session status incl. reconciling', () => {
    expect(SESSION_STATUS_TONE.open).toBe('neutral')
    expect(SESSION_STATUS_TONE.counting).toBe('info')
    expect(SESSION_STATUS_TONE.reconciling).toBe('warning')
    expect(SESSION_STATUS_TONE.closed).toBe('success')
  })
  it('maps each variance result to a follow-up action', () => {
    expect(RESULT_ACTION.not_found).toBe('disposal')
    expect(RESULT_ACTION.misplaced).toBe('transfer')
    expect(RESULT_ACTION.damaged).toBe('maintenance') // disabled in UI
  })
  it('lists all five item results', () => {
    expect(ITEM_RESULT_KEYS).toEqual(['pending', 'found', 'not_found', 'damaged', 'misplaced'])
    expect(ITEM_RESULT_TONE.found).toBe('success')
  })
})
