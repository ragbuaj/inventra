import { describe, it, expect } from 'vitest'
import { paginate, filterBy } from '~/mock/helpers'
import type { ListQuery } from '~/types'

// 25 items for pagination tests
const rows = Array.from({ length: 25 }, (_, i) => ({ id: i + 1, name: `Item ${i + 1}` }))

describe('paginate', () => {
  it('returns default limit of 20 when not specified', () => {
    const result = paginate(rows, {})
    expect(result.limit).toBe(20)
    expect(result.data).toHaveLength(20)
  })

  it('applies offset correctly', () => {
    const result = paginate(rows, { limit: 10, offset: 10 })
    expect(result.data[0].id).toBe(11)
    expect(result.data).toHaveLength(10)
  })

  it('returns correct total regardless of pagination', () => {
    const result = paginate(rows, { limit: 5, offset: 0 })
    expect(result.total).toBe(25)
  })

  it('returns offset in the result', () => {
    const result = paginate(rows, { limit: 5, offset: 7 })
    expect(result.offset).toBe(7)
  })

  it('falls back to default 20 when limit is 0 (falsy coercion)', () => {
    // limit=0 hits the `|| 20` fallback before Math.max(_, 1), so result is 20
    const result = paginate(rows, { limit: 0 })
    expect(result.limit).toBe(20)
    expect(result.data).toHaveLength(20)
  })

  it('clamps limit to minimum of 1 for positive sub-1 values', () => {
    // limit=0.5 → Number(0.5) || 20 = 0.5, then Math.max(0.5, 1) = 1
    const result = paginate(rows, { limit: 0.5 })
    expect(result.limit).toBe(1)
    expect(result.data).toHaveLength(1)
  })

  it('clamps limit to maximum of 100', () => {
    const big = Array.from({ length: 200 }, (_, i) => ({ id: i }))
    const result = paginate(big, { limit: 500 })
    expect(result.limit).toBe(100)
    expect(result.data).toHaveLength(100)
  })

  it('clamps negative limit to 1', () => {
    const result = paginate(rows, { limit: -5 })
    expect(result.limit).toBe(1)
  })

  it('handles offset beyond rows length — returns empty data', () => {
    const result = paginate(rows, { limit: 10, offset: 100 })
    expect(result.data).toHaveLength(0)
    expect(result.total).toBe(25)
  })

  it('returns empty data for empty rows', () => {
    const result = paginate([], { limit: 20, offset: 0 })
    expect(result.data).toHaveLength(0)
    expect(result.total).toBe(0)
  })
})

describe('filterBy', () => {
  const items = [
    { id: 1, name: 'Alice Smith', role: 'admin' },
    { id: 2, name: 'Bob Jones', role: 'user' },
    { id: 3, name: 'Charlie Brown', role: 'admin' }
  ]

  it('returns all rows when search is empty', () => {
    const query: ListQuery = { search: '' }
    expect(filterBy(items, query, ['name'])).toHaveLength(3)
  })

  it('returns all rows when search is whitespace only', () => {
    const query: ListQuery = { search: '   ' }
    expect(filterBy(items, query, ['name'])).toHaveLength(3)
  })

  it('returns all rows when search is undefined', () => {
    const query: ListQuery = {}
    expect(filterBy(items, query, ['name'])).toHaveLength(3)
  })

  it('matches case-insensitively', () => {
    const query: ListQuery = { search: 'ALICE' }
    const result = filterBy(items, query, ['name'])
    expect(result).toHaveLength(1)
    expect(result[0].name).toBe('Alice Smith')
  })

  it('matches partial strings', () => {
    const query: ListQuery = { search: 'jones' }
    const result = filterBy(items, query, ['name'])
    expect(result).toHaveLength(1)
    expect(result[0].id).toBe(2)
  })

  it('searches across multiple fields', () => {
    // "admin" appears in role for Alice and Charlie
    const query: ListQuery = { search: 'admin' }
    const result = filterBy(items, query, ['name', 'role'])
    expect(result).toHaveLength(2)
  })

  it('returns empty array when no match', () => {
    const query: ListQuery = { search: 'xyz-no-match' }
    expect(filterBy(items, query, ['name'])).toHaveLength(0)
  })

  it('returns empty array for empty input rows', () => {
    const query: ListQuery = { search: 'alice' }
    expect(filterBy([], query, ['name'])).toHaveLength(0)
  })
})
