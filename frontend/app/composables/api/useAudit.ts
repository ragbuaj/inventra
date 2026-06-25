import type { AuditAction, AuditLog } from '~/mock/audit'
import { auditSeed, AUDIT_MONTHS } from '~/mock/audit'
import { fakeLatency } from '~/mock/helpers'

type Locale = 'id' | 'en'

export interface AuditDiffView {
  field: string
  before: string
  after: string
  hasBefore: boolean
  hasAfter: boolean
  hasArrow: boolean
}

export interface AuditRow {
  id: number
  date: string
  time: string
  /** 'YYYY-MM-DD' for date-range filtering */
  dateKey: string
  actor: string
  initials: string
  role: string
  action: AuditAction
  entity: string
  summary: string
  office: string
  ip: string
  ref: string
  diff: AuditDiffView[]
}

function initials(name: string): string {
  const parts = name.trim().split(' ')
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
}

function formatDate(dt: string, locale: Locale): string {
  const datePart = dt.slice(0, 10)
  const [y, m, day] = datePart.split('-')
  const month = AUDIT_MONTHS[locale][Number(m) - 1] ?? m
  return `${Number(day)} ${month} ${y}`
}

function resolve(log: AuditLog, locale: Locale): AuditRow {
  return {
    id: log.id,
    date: formatDate(log.dt, locale),
    time: log.dt.slice(11),
    dateKey: log.dt.slice(0, 10),
    actor: log.actor,
    initials: initials(log.actor),
    role: log.role[locale] ?? log.role.id,
    action: log.action,
    entity: log.entity,
    summary: log.summary[locale] ?? log.summary.id,
    office: log.office,
    ip: log.ip,
    ref: log.ref,
    diff: log.diff.map(x => ({
      field: x.field,
      before: x.before ?? '',
      after: x.after ?? '',
      hasBefore: x.before != null,
      hasAfter: x.after != null,
      hasArrow: x.before != null && x.after != null
    }))
  }
}

/**
 * Audit log reader (read-only). Mock-first; the seam a real implementation swaps behind
 * (`/audit/logs`). Returns all resolved rows; the page filters/paginates over them.
 */
export function useAudit() {
  async function list(locale: Locale = 'id'): Promise<AuditRow[]> {
    await fakeLatency()
    return auditSeed.map(l => resolve(l, locale))
  }

  /** Distinct actor names (for the actor filter), in first-seen order. */
  function actors(): string[] {
    return Array.from(new Set(auditSeed.map(l => l.actor)))
  }

  return { list, actors }
}
