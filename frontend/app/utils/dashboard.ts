/**
 * Pure derivation helpers for the Dashboard screen.
 *
 * Kept framework-free so the math is unit-testable without mounting. Consumed by
 * `DashboardDonut` (Unovis + legend) and `DashboardBarList`. Colors are emitted as
 * CSS-var strings so light/dark theming follows the design tokens automatically.
 */

/**
 * i18n suffix keys under `dashboard.status.*`, in the backend `by_status` order
 * (report/service.go): available, assigned, under_maintenance, in_transfer,
 * retired, disposed, lost.
 */
export const STATUS_KEYS = ['available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost'] as const

/** Segment colors — semantic token CSS vars (one per status, same order as `STATUS_KEYS`). */
export const STATUS_COLORS = [
  'var(--ui-success)',
  'var(--ui-info)',
  'var(--ui-warning)',
  'var(--ui-secondary)',
  'var(--ui-text-muted)',
  'var(--ui-text-dimmed)',
  'var(--ui-error)'
] as const

export interface StatusSegment {
  /** i18n suffix, e.g. 'available' → `dashboard.status.available` */
  key: string
  /** CSS color string for the donut arc + legend dot */
  color: string
  count: number
  /** integer percent of the total */
  pct: number
}

export interface DonutModel {
  total: number
  segments: StatusSegment[]
}

export interface BarItem {
  label: string
  count: number
  /** bar width as an integer percent (0–100) of the largest item */
  w: number
}

/** Build the status donut model (total + per-segment count/pct) from raw counts. */
export function buildDonut(status: number[]): DonutModel {
  const total = status.reduce((sum, c) => sum + c, 0)
  const segments: StatusSegment[] = status.map((count, i) => ({
    key: STATUS_KEYS[i] ?? `seg${i}`,
    color: STATUS_COLORS[i] ?? 'var(--ui-text-dimmed)',
    count,
    pct: total === 0 ? 0 : Math.round((count / total) * 100)
  }))
  return { total, segments }
}

/** Convert `[label, count]` rows into bar items with widths relative to the max. */
export function barWidths(items: [string, number][]): BarItem[] {
  const max = items.reduce((m, [, c]) => Math.max(m, c), 0)
  return items.map(([label, count]) => ({
    label,
    count,
    w: max === 0 ? 0 : Math.round((count / max) * 100)
  }))
}

/** Group a number with Indonesian thousands separators (matches the mockup's `fmt`). */
export function formatCount(n: number): string {
  return n.toLocaleString('id-ID')
}

/**
 * A translatable due-date descriptor: `key` is an i18n key under
 * `dashboard.panel.due.*`, `n` (when present) is the day count interpolated as
 * `{ n }`. Kept framework-free so the day math is unit-testable without Vue.
 */
export interface DueLabel {
  key: string
  n?: number
}

/** Whole-day difference between a `YYYY-MM-DD` date and `today`, using local midnights. */
export function dueDiffDays(nextDueDate: string, today: Date = new Date()): number {
  const [y, m, d] = nextDueDate.split('-').map(Number)
  const due = new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1).getTime()
  const now = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime()
  return Math.round((due - now) / 86_400_000)
}

/**
 * Map a maintenance due date to a localized descriptor:
 * overdue → `due.overdue` (n = days late), 0 → `due.today`, 1 → `due.tomorrow`,
 * >1 → `due.inDays` (n = days left).
 */
export function dueLabel(nextDueDate: string, today: Date = new Date()): DueLabel {
  const diff = dueDiffDays(nextDueDate, today)
  if (diff < 0) return { key: 'dashboard.panel.due.overdue', n: -diff }
  if (diff === 0) return { key: 'dashboard.panel.due.today' }
  if (diff === 1) return { key: 'dashboard.panel.due.tomorrow' }
  return { key: 'dashboard.panel.due.inDays', n: diff }
}
