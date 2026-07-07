import { describe, it, expect } from 'vitest'
import { ASSIGNMENT_STATUS_TONE, REQUEST_STATUS_TONE, CONDITION_TONE, formatDateID } from '~/constants/assignmentMeta'

describe('assignmentMeta', () => {
  it('maps assignment status tones', () => {
    expect(ASSIGNMENT_STATUS_TONE.active).toBe('info')
    expect(ASSIGNMENT_STATUS_TONE.returned).toBe('neutral')
  })

  it('maps request status tones', () => {
    expect(REQUEST_STATUS_TONE.pending).toBe('warning')
    expect(REQUEST_STATUS_TONE.approved).toBe('success')
    expect(REQUEST_STATUS_TONE.rejected).toBe('error')
    expect(REQUEST_STATUS_TONE.cancelled).toBe('neutral')
  })

  it('maps condition tones', () => {
    expect(CONDITION_TONE.baik).toBe('success')
    expect(CONDITION_TONE.berat).toBe('error')
  })

  it('formats ISO dates in Indonesian short form', () => {
    expect(formatDateID('2026-07-06T00:00:00Z')).toMatch(/^6 Jul 2026$/)
    expect(formatDateID('')).toBe('')
    expect(formatDateID(null)).toBe('')
    expect(formatDateID('not-a-date')).toBe('')
  })
})
