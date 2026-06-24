import { describe, it, expect } from 'vitest'
import { formatRupiah, formatDate } from '~/utils/format'

describe('formatRupiah', () => {
  it('formats a number with Rp prefix and no decimals', () => {
    const result = formatRupiah(1500000)
    // Intl.NumberFormat id-ID uses 'Rp' and period as thousands separator
    expect(result).toMatch(/^Rp/)
    expect(result).toContain('1')
    expect(result).not.toMatch(/[,.]00$/)
  })

  it('formats a numeric string', () => {
    const result = formatRupiah('2000000')
    expect(result).toMatch(/^Rp/)
    expect(result).toContain('2')
  })

  it('returns em dash for null', () => {
    expect(formatRupiah(null)).toBe('—')
  })

  it('returns em dash for empty string', () => {
    expect(formatRupiah('')).toBe('—')
  })

  it('returns em dash for NaN input', () => {
    expect(formatRupiah('not-a-number')).toBe('—')
  })

  it('returns em dash for NaN number', () => {
    expect(formatRupiah(NaN)).toBe('—')
  })

  it('formats zero', () => {
    const result = formatRupiah(0)
    expect(result).toMatch(/^Rp/)
    expect(result).toContain('0')
  })
})

describe('formatDate', () => {
  it('formats a valid ISO date in id-ID medium style', () => {
    // 2024-01-15 should produce something like "15 Jan 2024" in id-ID
    const result = formatDate('2024-01-15')
    expect(result).toContain('2024')
    expect(result).not.toBe('—')
  })

  it('returns em dash for null', () => {
    expect(formatDate(null)).toBe('—')
  })

  it('returns em dash for an invalid date string', () => {
    expect(formatDate('not-a-date')).toBe('—')
  })

  it('includes time when withTime is true', () => {
    const withTime = formatDate('2024-01-15T10:30:00', { withTime: true })
    const withoutTime = formatDate('2024-01-15T10:30:00')
    // withTime result should be longer (has time component)
    expect(withTime.length).toBeGreaterThan(withoutTime.length)
    expect(withTime).not.toBe('—')
  })

  it('does not include time when withTime is false (default)', () => {
    const result = formatDate('2024-06-20T14:45:00')
    expect(result).toContain('2024')
    // In id-ID without time, no colon expected (HH:MM pattern)
    expect(result).not.toMatch(/\d{2}:\d{2}/)
  })
})
