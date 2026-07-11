import { describe, it, expect } from 'vitest'
import { buildDonut, barWidths, formatCount, dueDiffDays, dueLabel, STATUS_KEYS, STATUS_COLORS } from '~/utils/dashboard'

describe('buildDonut', () => {
  it('sums the total and maps each count to a segment', () => {
    const { total, segments } = buildDonut([58, 22, 9, 4, 3, 2, 1])
    expect(total).toBe(99)
    expect(segments).toHaveLength(7)
    expect(segments.map(s => s.count)).toEqual([58, 22, 9, 4, 3, 2, 1])
  })

  it('assigns the status key and color per segment in order (7 statuses)', () => {
    const { segments } = buildDonut([1, 1, 1, 1, 1, 1, 1])
    expect(segments.map(s => s.key)).toEqual([...STATUS_KEYS])
    expect(segments.map(s => s.color)).toEqual([...STATUS_COLORS])
    expect(STATUS_KEYS).toHaveLength(7)
    expect(STATUS_COLORS).toHaveLength(7)
    expect(STATUS_KEYS[1]).toBe('assigned')
    expect(STATUS_KEYS[2]).toBe('under_maintenance')
    expect(STATUS_KEYS[3]).toBe('in_transfer')
    expect(STATUS_KEYS[4]).toBe('retired')
  })

  it('rounds percentages to integers that reflect the share', () => {
    const { segments } = buildDonut([50, 50, 0, 0, 0, 0, 0])
    expect(segments[0].pct).toBe(50)
    expect(segments[1].pct).toBe(50)
    expect(segments[2].pct).toBe(0)
  })

  it('never divides by zero when every count is zero', () => {
    const { total, segments } = buildDonut([0, 0, 0, 0, 0, 0, 0])
    expect(total).toBe(0)
    expect(segments.every(s => s.pct === 0)).toBe(true)
    expect(segments.every(s => Number.isFinite(s.pct))).toBe(true)
  })

  it('handles fewer than seven counts without throwing', () => {
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

describe('dueDiffDays', () => {
  const today = new Date(2026, 6, 12) // 2026-07-12 (local)

  it('returns 0 for the same day', () => {
    expect(dueDiffDays('2026-07-12', today)).toBe(0)
  })

  it('returns 1 for tomorrow and 5 for five days out', () => {
    expect(dueDiffDays('2026-07-13', today)).toBe(1)
    expect(dueDiffDays('2026-07-17', today)).toBe(5)
  })

  it('returns a negative diff for a past date', () => {
    expect(dueDiffDays('2026-07-09', today)).toBe(-3)
  })
})

describe('dueLabel', () => {
  const today = new Date(2026, 6, 12)

  it('maps today / tomorrow / n-days / overdue to the right keys', () => {
    expect(dueLabel('2026-07-12', today)).toEqual({ key: 'dashboard.panel.due.today' })
    expect(dueLabel('2026-07-13', today)).toEqual({ key: 'dashboard.panel.due.tomorrow' })
    expect(dueLabel('2026-07-17', today)).toEqual({ key: 'dashboard.panel.due.inDays', n: 5 })
    expect(dueLabel('2026-07-09', today)).toEqual({ key: 'dashboard.panel.due.overdue', n: 3 })
  })
})
