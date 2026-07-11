import { describe, it, expect } from 'vitest'
import { MAINT_STATUS_TONE, MAINT_TYPE_TONE, dueDiffDays, dueKind, formatRupiah } from '~/constants/maintenanceMeta'

describe('constants/maintenanceMeta', () => {
  it('maps every maintenance status to a badge tone', () => {
    expect(MAINT_STATUS_TONE.scheduled).toBe('neutral')
    expect(MAINT_STATUS_TONE.in_progress).toBe('info')
    expect(MAINT_STATUS_TONE.completed).toBe('success')
    expect(MAINT_STATUS_TONE.cancelled).toBe('error')
  })

  it('maps every maintenance type to a badge tone', () => {
    expect(MAINT_TYPE_TONE.preventive).toBe('info')
    expect(MAINT_TYPE_TONE.corrective).toBe('warning')
  })

  describe('dueDiffDays', () => {
    const today = new Date('2026-07-11T15:30:00Z')

    it('is negative for an overdue date', () => {
      expect(dueDiffDays('2026-07-08', today)).toBe(-3)
    })

    it('is 0 for today', () => {
      expect(dueDiffDays('2026-07-11', today)).toBe(0)
    })

    it('is positive for a future date', () => {
      expect(dueDiffDays('2026-07-18', today)).toBe(7)
    })

    it('is null for null/undefined/empty input', () => {
      expect(dueDiffDays(null, today)).toBeNull()
      expect(dueDiffDays(undefined, today)).toBeNull()
      expect(dueDiffDays('', today)).toBeNull()
    })

    it('is null for garbage input', () => {
      expect(dueDiffDays('not-a-date', today)).toBeNull()
    })
  })

  describe('dueKind', () => {
    it('is overdue for negative diffs', () => {
      expect(dueKind(-1)).toBe('overdue')
    })

    it('is today for a diff of 0', () => {
      expect(dueKind(0)).toBe('today')
    })

    it('is soon at the 1-day and 7-day boundaries', () => {
      expect(dueKind(1)).toBe('soon')
      expect(dueKind(7)).toBe('soon')
    })

    it('is normal past the 7-day boundary', () => {
      expect(dueKind(8)).toBe('normal')
    })

    it('is normal for null (no due date)', () => {
      expect(dueKind(null)).toBe('normal')
    })
  })

  describe('formatRupiah', () => {
    it('formats a numeric string with thousands separators', () => {
      expect(formatRupiah('2350000')).toBe('Rp 2.350.000')
    })

    it('formats a plain number', () => {
      expect(formatRupiah(2350000)).toBe('Rp 2.350.000')
    })

    it('renders an em-dash for null/undefined', () => {
      expect(formatRupiah(null)).toBe('—')
      expect(formatRupiah(undefined)).toBe('—')
    })

    it('renders an em-dash for a zero-ish value', () => {
      expect(formatRupiah('0')).toBe('—')
      expect(formatRupiah(0)).toBe('—')
    })

    it('renders an em-dash for garbage input', () => {
      expect(formatRupiah('not-a-number')).toBe('—')
    })
  })
})
