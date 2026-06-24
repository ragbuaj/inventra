import type { Paginated, ListQuery } from '~/types'

export function fakeLatency(ms = 300): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

export function filterBy<T>(rows: T[], query: ListQuery, fields: (keyof T)[]): T[] {
  const term = (query.search ?? '').toString().trim().toLowerCase()
  if (!term) return rows
  return rows.filter(row =>
    fields.some(f => String(row[f] ?? '').toLowerCase().includes(term))
  )
}

export function paginate<T>(rows: T[], query: ListQuery): Paginated<T> {
  const limit = Math.min(Math.max(Number(query.limit) || 20, 1), 100)
  const offset = Math.max(Number(query.offset) || 0, 0)
  return {
    data: rows.slice(offset, offset + limit),
    total: rows.length,
    limit,
    offset
  }
}
