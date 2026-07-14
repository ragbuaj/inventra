import { describe, it, expect } from 'vitest'
import { formatRupiahCompact } from '~/utils/format'

describe('formatRupiahCompact', () => {
  it('returns em dash for absent/invalid', () => {
    expect(formatRupiahCompact(null)).toBe('—')
    expect(formatRupiahCompact('')).toBe('—')
    expect(formatRupiahCompact('abc')).toBe('—')
  })
  it('keeps small values ungrouped-compact', () => {
    expect(formatRupiahCompact(500)).toBe('Rp 500')
    expect(formatRupiahCompact('999')).toBe('Rp 999')
  })
  it('scales thousands/millions/billions/trillions', () => {
    expect(formatRupiahCompact(1500)).toBe('Rp 1,5 rb')
    expect(formatRupiahCompact(2_300_000)).toBe('Rp 2,3 jt')
    expect(formatRupiahCompact(1_234_567_890)).toBe('Rp 1,23 M')
    expect(formatRupiahCompact('1234567890000')).toBe('Rp 1,23 T')
  })
  it('handles negatives', () => {
    expect(formatRupiahCompact(-2_300_000)).toBe('-Rp 2,3 jt')
  })
  it('handles zero', () => {
    expect(formatRupiahCompact(0)).toBe('Rp 0')
    expect(formatRupiahCompact('0')).toBe('Rp 0')
  })
  it('handles exact scale boundaries', () => {
    expect(formatRupiahCompact(1_000)).toBe('Rp 1 rb')
    expect(formatRupiahCompact(1_000_000)).toBe('Rp 1 jt')
    expect(formatRupiahCompact(1_000_000_000)).toBe('Rp 1 M')
    expect(formatRupiahCompact(1_000_000_000_000)).toBe('Rp 1 T')
  })
  it('rounds at the top of a digits bracket, staying in the lower scale bucket', () => {
    // 9.999 rb rounds to 10.00 at 2 decimal places -> displays as '10' (still 'rb', not bumped to 'jt')
    expect(formatRupiahCompact(9_999)).toBe('Rp 10 rb')
    // 99.999 rb rounds to 100.0 at 1 decimal place -> displays as '100' (still 'rb')
    expect(formatRupiahCompact(99_999)).toBe('Rp 100 rb')
    // 999.999 rb rounds to 1000 at 0 decimals -> displays as '1.000' (still labeled 'rb', not 'jt',
    // because the scale bucket is chosen from the raw value before rounding: 999999 < 1e6)
    expect(formatRupiahCompact(999_999)).toBe('Rp 1.000 rb')
    expect(formatRupiahCompact(999_999_999)).toBe('Rp 1.000 jt')
  })
})
