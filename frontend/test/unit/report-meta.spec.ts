import { describe, it, expect } from 'vitest'
import {
  REPORT_KEYS,
  REPORT_ICON,
  periodToQuery,
  formatMoneyShort,
  formatTrendPct
} from '~/constants/reportMeta'
import type { ReportKey, PeriodValue } from '~/constants/reportMeta'

describe('constants/reportMeta — REPORT_KEYS + REPORT_ICON', () => {
  it('lists the 4 mockup keys first then the 3 new ones, in order', () => {
    expect(REPORT_KEYS).toEqual([
      'assets',
      'depreciation',
      'utilization',
      'maintenance',
      'transfers',
      'disposals',
      'opname'
    ])
  })

  it('maps every report key to its lucide icon', () => {
    expect(REPORT_ICON).toEqual({
      assets: 'i-lucide-package',
      depreciation: 'i-lucide-trending-down',
      utilization: 'i-lucide-gauge',
      maintenance: 'i-lucide-receipt',
      transfers: 'i-lucide-arrow-left-right',
      disposals: 'i-lucide-trash-2',
      opname: 'i-lucide-clipboard-check'
    })
  })

  it('has an icon for every key with no stray keys', () => {
    expect(Object.keys(REPORT_ICON).sort()).toEqual([...REPORT_KEYS].sort())
    for (const k of REPORT_KEYS) {
      const key: ReportKey = k
      expect(REPORT_ICON[key]).toMatch(/^i-lucide-/)
    }
  })
})

describe('periodToQuery', () => {
  const presets = [
    ['last30', { period: 'last30' }],
    ['this_month', { period: 'this_month' }],
    ['this_quarter', { period: 'this_quarter' }],
    ['ytd', { period: 'ytd' }]
  ] as const

  it.each(presets)('preset %s → { period }', (preset, expected) => {
    const q = periodToQuery({ preset })
    expect(q).toEqual(expected)
  })

  it.each(presets)('preset %s produces only the period key', (preset) => {
    expect(Object.keys(periodToQuery({ preset }))).toEqual(['period'])
  })

  it('custom → { date_from, date_to } with no period key', () => {
    const q = periodToQuery({ preset: 'custom', from: '2026-01-01', to: '2026-03-31' })
    expect(q).toEqual({ date_from: '2026-01-01', date_to: '2026-03-31' })
    expect(Object.keys(q).sort()).toEqual(['date_from', 'date_to'])
    expect(q).not.toHaveProperty('period')
  })
})

describe('formatMoneyShort', () => {
  const cases: Array<[string, string]> = [
    ['0', 'Rp 0'],
    ['950000', 'Rp 950.000'],
    ['999999', 'Rp 999.999'],
    ['1000000', 'Rp 1 Jt'],
    ['42500000', 'Rp 42,5 Jt'],
    ['1000000000', 'Rp 1 M'],
    ['3820000000', 'Rp 3,82 M'],
    ['abc', 'abc']
  ]

  it.each(cases)('formatMoneyShort(%s) === %s', (input, expected) => {
    expect(formatMoneyShort(input)).toBe(expected)
  })

  it('999999 stays full rupiah (just below the million threshold)', () => {
    expect(formatMoneyShort('999999')).toBe('Rp 999.999')
  })

  it('1e6 crosses into the Jt (juta) bucket', () => {
    expect(formatMoneyShort('1000000')).toBe('Rp 1 Jt')
  })

  it('1e9 crosses into the M (miliar) bucket', () => {
    expect(formatMoneyShort('1000000000')).toBe('Rp 1 M')
  })

  it('unparseable input is returned verbatim', () => {
    expect(formatMoneyShort('abc')).toBe('abc')
  })
})

describe('formatTrendPct', () => {
  it('formats a positive number with a leading plus and comma decimal', () => {
    expect(formatTrendPct(8.3)).toBe('+8,3%')
  })

  it('formats a negative number with a real minus sign (U+2212)', () => {
    const out = formatTrendPct(-6.4)
    expect(out).toBe('−6,4%')
    expect(out).toBe('−6,4%')
    expect(out?.includes('-')).toBe(false) // not the ASCII hyphen-minus
  })

  it('returns null for null', () => {
    expect(formatTrendPct(null)).toBeNull()
  })

  it('returns null for undefined', () => {
    expect(formatTrendPct(undefined)).toBeNull()
  })

  it('formats an integer percentage without a decimal part', () => {
    expect(formatTrendPct(12)).toBe('+12%')
  })
})

describe('type surface', () => {
  it('PeriodValue custom carries from/to', () => {
    const v: PeriodValue = { preset: 'custom', from: '2026-01-01', to: '2026-01-31' }
    expect(v.from).toBe('2026-01-01')
  })
})
