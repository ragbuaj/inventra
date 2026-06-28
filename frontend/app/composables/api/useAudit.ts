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
  action: AuditAction
  entity_type: string
  entity_id: string
  ip: string
  diff: AuditDiffView[]
}

export interface AuditListParams {
  search?: string
  entity_type?: string
  action?: AuditAction
  from?: string
  to?: string
  limit: number
  offset: number
}

interface AuditChange { before?: unknown, after?: unknown }
interface AuditDTO {
  id: string
  entity_type: string
  entity_id: string
  action: AuditAction
  ip: string
  changes: Record<string, AuditChange> | null
  actor: { id: string, name: string, email: string } | null
  office_id: string | null
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

function toRow(d: AuditDTO): AuditRow {
  const name = d.actor?.name ?? ''
  return {
    id: d.id,
    created_at: d.created_at,
    date: (d.created_at ?? '').slice(0, 10),
    time: (d.created_at ?? '').slice(11, 16),
    actor: name,
    actor_email: d.actor?.email ?? '',
    initials: initials(name),
    action: d.action,
    entity_type: d.entity_type,
    entity_id: d.entity_id,
    ip: d.ip,
    diff: toDiff(d.changes)
  }
}

/**
 * Audit log reader (read-only), wired to GET /api/v1/audit. Filtering and
 * pagination are server-side; the actor name comes from the response (no lookup).
 */
export function useAudit() {
  const { request } = useApiClient()

  async function list(params: AuditListParams): Promise<{ rows: AuditRow[], total: number }> {
    const q = new URLSearchParams()
    q.set('limit', String(params.limit))
    q.set('offset', String(params.offset))
    if (params.search) q.set('search', params.search)
    if (params.entity_type) q.set('entity_type', params.entity_type)
    if (params.action) q.set('action', params.action)
    if (params.from) q.set('from', params.from)
    if (params.to) q.set('to', params.to)
    const res = await request<{ data: AuditDTO[], total: number, limit: number, offset: number }>(`/audit?${q.toString()}`)
    return { rows: res.data.map(toRow), total: res.total }
  }

  return { list }
}
