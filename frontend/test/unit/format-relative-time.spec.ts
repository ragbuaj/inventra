import { describe, it, expect } from 'vitest'
import { formatRelativeTime } from '~/utils/format'

// A fixed "now" so the deltas are deterministic.
const NOW = new Date('2026-07-15T12:00:00Z').getTime()
const ago = (ms: number) => new Date(NOW - ms).toISOString()

const SEC = 1000
const MIN = 60 * SEC
const HOUR = 60 * MIN
const DAY = 24 * HOUR

describe('formatRelativeTime', () => {
  it('renders sub-minute as "now" (id + en)', () => {
    // Intl.RelativeTimeFormat renders 0 seconds (numeric:auto) as "sekarang"/"now".
    expect(formatRelativeTime(ago(5 * SEC), 'id', NOW)).toBe('sekarang')
    expect(formatRelativeTime(ago(5 * SEC), 'en', NOW)).toBe('now')
  })

  it('renders minutes ago', () => {
    expect(formatRelativeTime(ago(5 * MIN), 'en', NOW)).toBe('5 minutes ago')
    expect(formatRelativeTime(ago(5 * MIN), 'id', NOW)).toContain('5 menit')
  })

  it('renders hours ago', () => {
    expect(formatRelativeTime(ago(2 * HOUR), 'en', NOW)).toBe('2 hours ago')
    expect(formatRelativeTime(ago(2 * HOUR), 'id', NOW)).toContain('2 jam')
  })

  it('renders "yesterday" for ~1 day via numeric:auto', () => {
    expect(formatRelativeTime(ago(DAY), 'en', NOW)).toBe('yesterday')
    expect(formatRelativeTime(ago(DAY), 'id', NOW)).toBe('kemarin')
  })

  it('renders multiple days ago', () => {
    expect(formatRelativeTime(ago(3 * DAY), 'en', NOW)).toBe('3 days ago')
  })

  it('returns empty string for missing or invalid input', () => {
    expect(formatRelativeTime('', 'en', NOW)).toBe('')
    expect(formatRelativeTime(null, 'en', NOW)).toBe('')
    expect(formatRelativeTime('not-a-date', 'en', NOW)).toBe('')
  })
})
