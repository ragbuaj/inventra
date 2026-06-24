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

export function generateId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `id-${Math.random().toString(36).slice(2)}-${performance.now()}`
}

export interface MockStore<T extends { id: string }> {
  all(): T[]
  find(id: string): T | undefined
  insert(row: T): T
  patch(id: string, changes: Partial<T>): T | undefined
  remove(id: string): boolean
}

export function createStore<T extends { id: string }>(seed: T[]): MockStore<T> {
  const rows: T[] = [...seed]
  return {
    all: () => rows,
    find: id => rows.find(r => r.id === id),
    insert(row) {
      rows.unshift(row)
      return row
    },
    patch(id, changes) {
      const row = rows.find(r => r.id === id)
      if (!row) return undefined
      Object.assign(row, changes)
      return row
    },
    remove(id) {
      const i = rows.findIndex(r => r.id === id)
      if (i === -1) return false
      rows.splice(i, 1)
      return true
    }
  }
}
