import type { RequestType, RequestStatus } from '~/constants/approvalMeta'

export interface ApprovalRequestRow {
  id: string
  type: RequestType
  status: RequestStatus
  amount?: string | null
  current_step: number
  office_id: string | null
  office_name: string | null
  target_id: string | null
  target_entity: string | null
  reason?: string | null
  requested_by_id: string
  requested_by_name: string | null
  requested_by_role: string | null
  decided_by_id: string | null
  decision_note: string | null
  created_at: string | null
}

export interface ApprovalStep {
  step_order: number
  required_level: string
  approver_id: string | null
  approver_name: string | null
  decision: RequestStatus
  note: string | null
  decided_at: string | null
}

export interface ApprovalRequestDetail extends ApprovalRequestRow {
  /** Raw submitted payload; absent/undefined when masked by field permissions. */
  payload?: Record<string, unknown> | null
  steps: ApprovalStep[]
}

export interface ApprovalListQuery {
  status?: RequestStatus
  type?: RequestType
  limit?: number
  offset?: number
}

export interface ApprovalListPage {
  data: ApprovalRequestRow[]
  total: number
  limit: number
  offset: number
}

/** Approval inbox + decisions, wired to /api/v1/requests. */
export function useApproval() {
  const { request } = useApiClient()

  async function inbox(): Promise<ApprovalRequestRow[]> {
    const res = await request<{ data: ApprovalRequestRow[], total: number }>('/requests/inbox')
    return res.data
  }

  async function list(q: ApprovalListQuery = {}): Promise<ApprovalListPage> {
    const query: Record<string, string | number> = {}
    if (q.status) query.status = q.status
    if (q.type) query.type = q.type
    if (q.limit !== undefined) query.limit = q.limit
    if (q.offset !== undefined) query.offset = q.offset
    return request<ApprovalListPage>('/requests', { query })
  }

  async function get(id: string): Promise<ApprovalRequestDetail> {
    return request<ApprovalRequestDetail>(`/requests/${id}`)
  }

  // The backend DecideRequest binding requires `decision` whenever a body is
  // present (only a fully-empty body is tolerated), so both calls send it
  // explicitly even though the endpoint is already action-specific.
  // NOTE: decide responses are the PLAIN request serialization (no
  // requested_by_name/office_name enrichment) — callers must not rely on the
  // enrichment fields here; the page refreshes via loadTab()+get() instead.
  async function approve(id: string, note?: string): Promise<ApprovalRequestRow> {
    return request<ApprovalRequestRow>(`/requests/${id}/approve`, {
      method: 'POST',
      body: { decision: 'approve', note: note || undefined }
    })
  }

  async function reject(id: string, note?: string): Promise<ApprovalRequestRow> {
    return request<ApprovalRequestRow>(`/requests/${id}/reject`, {
      method: 'POST',
      body: { decision: 'reject', note: note || undefined }
    })
  }

  return { inbox, list, get, approve, reject }
}
