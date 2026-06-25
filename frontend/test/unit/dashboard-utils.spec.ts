import { describe, it, expect } from 'vitest'
import { buildDonut, barWidths, formatCount, STATUS_KEYS, STATUS_COLORS } from '~/utils/dashboard'

describe('buildDonut', () => {
  it('sums the total and maps each count to a segment', () => {
    const { total, segments } = buildDonut([58, 22, 9, 4, 3])
    expect(total).toBe(96)
    expect(segments).toHaveLength(5)
    expect(segments.map(s => s.count)).toEqual([58, 22, 9, 4, 3])
  })

  it('assigns the status key and color per segment in order', () => {
    const { segments } = buildDonut([1, 1, 1, 1, 1])
    expect(segments.map(s => s.key)).toEqual([...STATUS_KEYS])
    expect(segments.map(s => s.color)).toEqual([...STATUS_COLORS])
  })

  it('rounds percentages to integers that reflect the share', () => {
    const { segments } = buildDonut([50, 50, 0, 0, 0])
    expect(segments[0].pct).toBe(50)
    expect(segments[1].pct).toBe(50)
    expect(segments[2].pct).toBe(0)
  })

  it('never divides by zero when every count is zero', () => {
    const { total, segments } = buildDonut([0, 0, 0, 0, 0])
    expect(total).toBe(0)
    expect(segments.every(s => s.pct === 0)).toBe(true)
  })

  it('handles fewer than five counts without throwing', () => {
    const { total, segments } = buildDonut([10, 5])
    expect(total).toBe(15)
    expect(segments).toHaveLength(2)
    expect(segments[0].key).toBe('available')
  })
})

describe('barWidths', () => {
  it('sets the largest item to 100% and scales the rest', () => {
    const bars = barWidths([['A', 41], ['B', 28], ['C', 12]])
    expect(bars[0]).toMatchObject({ label: 'A', count: 41, w: 100 })
    expect(bars[1].w).toBe(Math.round(28 / 41 * 100))
    expect(bars[2].w).toBe(Math.round(12 / 41 * 100))
  })

  it('returns width 0 for every item when the max is zero', () => {
    const bars = barWidths([['A', 0], ['B', 0]])
    expect(bars.every(b => b.w === 0)).toBe(true)
  })

  it('returns an empty array for empty input', () => {
    expect(barWidths([])).toEqual([])
  })
})

describe('formatCount', () => {
  it('groups thousands in id-ID style', () => {
    expect(formatCount(1248)).toBe('1.248')
    expect(formatCount(96)).toBe('96')
  })
})
