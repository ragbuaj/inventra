import type { TableSorting } from '~/types'

// Locale-aware comparison that tolerates mixed value types (string / number /
// boolean / null) so a single comparator works across every table column.
function compareValues(a: unknown, b: unknown): number {
  if (a == null && b == null) return 0
  if (a == null) return -1
  if (b == null) return 1
  if (typeof a === 'number' && typeof b === 'number') return a - b
  if (typeof a === 'boolean' && typeof b === 'boolean') return a === b ? 0 : a ? 1 : -1
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
}

// Returns a new, stably-sorted copy of `rows` ordered by the sort state. The
// original array is never mutated. Supports multi-column sort (primary first).
export function sortRows<T extends Record<string, unknown>>(rows: T[], sorting: TableSorting): T[] {
  if (!sorting?.length) return rows
  return [...rows].sort((ra, rb) => {
    for (const s of sorting) {
      const result = compareValues(ra[s.id], rb[s.id])
      if (result !== 0) return s.desc ? -result : result
    }
    return 0
  })
}
