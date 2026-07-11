import type { BadgeColor } from '~/types'

export type MaintenanceStatus = 'scheduled' | 'in_progress' | 'completed' | 'cancelled'
export type MaintenanceType = 'preventive' | 'corrective'

export const MAINT_STATUS_TONE: Record<MaintenanceStatus, BadgeColor> = {
  scheduled: 'neutral',
  in_progress: 'info',
  completed: 'success',
  cancelled: 'error'
}

export const MAINT_TYPE_TONE: Record<MaintenanceType, BadgeColor> = {
  preventive: 'info',
  corrective: 'warning'
}

export type DueKind = 'overdue' | 'today' | 'soon' | 'normal'

/** Whole-day difference next_due - today (negative = overdue). */
export function dueDiffDays(nextDue: string | null | undefined, today: Date = new Date()): number | null {
  if (!nextDue) return null
  const d = new Date(nextDue)
  if (Number.isNaN(d.getTime())) return null
  const t0 = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate())
  const t1 = Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())
  return Math.round((t1 - t0) / 86400000)
}

/** Mockup badge semantics: overdue/today = red, <=7 days = yellow, else neutral. */
export function dueKind(diff: number | null): DueKind {
  if (diff === null) return 'normal'
  if (diff < 0) return 'overdue'
  if (diff === 0) return 'today'
  if (diff <= 7) return 'soon'
  return 'normal'
}

/** "2350000" → "Rp 2.350.000"; empty/zero-ish → "—". */
export function formatRupiah(v: string | number | null | undefined): string {
  const n = typeof v === 'string' ? Number(v) : v
  if (!n || Number.isNaN(n)) return '—'
  return `Rp ${n.toLocaleString('id-ID')}`
}
