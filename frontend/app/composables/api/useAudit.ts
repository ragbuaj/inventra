export type AuditAction = 'create' | 'update' | 'delete'

export interface AuditDiffView {
  field: string
  before: string
  after: string
  hasBefore: boolean
  hasAfter: boolean
  hasArrow: boolean
}

export interface AuditRow {
  id: string
  created_at: string
  date: string
  time: string
  actor: string
  actor_email: string
  initials: string
  role: string
  action: AuditAction
  entity_type: string
  entity_id: string
  ip: string
  office_name: string
  summary: string
  diff: AuditDiffView[]
}

export interface AuditListParams {
  search?: string
  entity_type?: string
  action?: AuditAction
  actor_id?: string
  from?: string
  to?: string
  limit: number
  offset: number
}

export type Translate = (key: string, params?: Record<string, unknown>) => string

interface AuditChange { before?: unknown, after?: unknown }
export interface AuditDTO {
  id: string
  entity_type: string
  entity_id: string
  action: AuditAction
  ip: string
  changes: Record<string, AuditChange> | null
  actor: { id: string, name: string, email: string, role: string | null } | null
  office_id: string | null
  office_name: string | null
  created_at: string
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
}

function toDiff(changes: Record<string, AuditChange> | null): AuditDiffView[] {
  if (!changes) return []
  return Object.entries(changes).map(([field, c]) => {
    const hasBefore = c.before != null
    const hasAfter = c.after != null
    return {
      field,
      before: hasBefore ? String(c.before) : '',
      after: hasAfter ? String(c.after) : '',
      hasBefore,
      hasAfter,
      hasArrow: hasBefore && hasAfter
    }
  })
}

/** Derives a localized one-line summary from the action + entity + changed-field count. */
function toSummary(d: AuditDTO, t: Translate): string {
  const count = d.changes ? Object.keys(d.changes).length : 0
  const params = { entity: d.entity_type, id: d.entity_id, count }
  if (d.action === 'create') return t('audit.summary.create', params)
  if (d.action === 'delete') return t('audit.summary.delete', params)
  return t('audit.summary.update', params)
}

export function toRow(d: AuditDTO, t: Translate): AuditRow {
  const name = d.actor?.name ?? ''
  return {
    id: d.id,
    created_at: d.created_at,
    date: (d.created_at ?? '').slice(0, 10),
    time: (d.created_at ?? '').slice(11, 16),
    actor: name,
    actor_email: d.actor?.email ?? '',
    initials: initials(name),
    role: d.actor?.role ?? '',
    action: d.action,
    entity_type: d.entity_type,
    entity_id: d.entity_id,
    ip: d.ip,
    office_name: d.office_name ?? '',
    summary: toSummary(d, t),
    diff: toDiff(d.changes)
  }
}

/**
 * Audit log reader (read-only), wired to GET /api/v1/audit. Filtering and
 * pagination are server-side; the actor name comes from the response (no lookup).
 */
export function useAudit() {
  const { request } = useApiClient()

  async function list(params: AuditListParams, t: Translate): Promise<{ rows: AuditRow[], total: number }> {
    const q = new URLSearchParams()
    q.set('limit', String(params.limit))
    q.set('offset', String(params.offset))
    if (params.search) q.set('search', params.search)
    if (params.entity_type) q.set('entity_type', params.entity_type)
    if (params.action) q.set('action', params.action)
    if (params.actor_id) q.set('actor_id', params.actor_id)
    if (params.from) q.set('from', params.from)
    if (params.to) q.set('to', params.to)
    const res = await request<{ data: AuditDTO[], total: number, limit: number, offset: number }>(`/audit?${q.toString()}`)
    return { rows: res.data.map(d => toRow(d, t)), total: res.total }
  }

  return { list }
}
