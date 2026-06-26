import { describe, it, expect } from 'vitest'
import { computeReport, reportHasData, REPORT_KEYS, ASET_ROWS, DEPR_ROWS, rpJt } from '~/mock/reports'

describe('mock/reports — seeds', () => {
  it('exposes 4 report kinds and seed rows', () => {
    expect(REPORT_KEYS).toEqual(['aset', 'depr', 'util', 'biaya'])
    expect(ASET_ROWS).toHaveLength(8)
    expect(DEPR_ROWS).toHaveLength(6)
  })
})

describe('computeReport — aset', () => {
  it('totals the full asset list and groups book value by category', () => {
    const r = computeReport('aset', {})
    expect(r.kind).toBe('aset')
    expect(r.rows).toHaveLength(8)
    expect(r.totalBuku).toBe(321800000)
    expect(r.byCategory.Elektronik).toBe(97600000) // 16.2 + 6.5 + 69 + 5.9 jt
    expect(reportHasData(r)).toBe(true)
  })

  it('filters by category and status', () => {
    const r = computeReport('aset', { kat: 'Elektronik', status: 'tersedia' })
    expect(r.rows.every(a => a.kat === 'Elektronik' && a.status === 'tersedia')).toBe(true)
    expect(r.rows).toHaveLength(3) // Genset, UPS, Laptop Dell
  })

  it('returns no data for an impossible filter combo', () => {
    const r = computeReport('aset', { kat: 'Furnitur', status: 'dipinjam' })
    expect(reportHasData(r)).toBe(false)
  })
})

describe('computeReport — depr / util / biaya', () => {
  it('depreciation totals the yearly depreciation column', () => {
    const r = computeReport('depr', {})
    expect(r.kind).toBe('depr')
    if (r.kind === 'depr') expect(r.totalDeprec).toBe(408000000)
  })

  it('utilization averages correctly overall and per filtered category', () => {
    const all = computeReport('util', {})
    if (all.kind === 'util') {
      expect(all.avg).toBe(57) // round(340/6)
      expect(all.totalHari).toBe(620)
      expect(all.loaned).toBe(6)
    }
    const elk = computeReport('util', { kat: 'Elektronik' })
    if (elk.kind === 'util') {
      expect(elk.rows).toHaveLength(3)
      expect(elk.avg).toBe(72) // round((78+73+66)/3)
    }
  })

  it('cost splits preventive vs corrective and totals', () => {
    const r = computeReport('biaya', {})
    if (r.kind === 'biaya') {
      expect(r.total).toBe(8650000)
      expect(r.preventive).toBe(3250000)
      expect(r.corrective).toBe(5400000)
      expect(r.totalN).toBe(13)
    }
  })
})

describe('rpJt formatter', () => {
  it('formats to millions with the Jt suffix', () => {
    expect(rpJt(174000000)).toBe('Rp 174 Jt')
    expect(rpJt(1500000)).toBe('Rp 1,5 Jt')
  })
})
