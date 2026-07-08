import type { AssignmentStatus, AssetCondition } from '~/constants/assignmentMeta'

export interface Assignment {
  id: string
  asset_id: string
  employee_id: string
  assigned_by_id: string
  checkout_date: string | null
  due_date: string | null
  checkin_date: string | null
  condition_out: string | null
  condition_in: string | null
  status: AssignmentStatus
  notes: string | null
  asset_name: string | null
  asset_tag: string | null
  employee_name: string | null
  assigned_by_name: string | null
  office_name: string | null
  created_at: string | null
  updated_at: string | null
}

export interface AvailableAsset {
  id: string
  asset_tag: string
  name: string
}

export interface AssignmentListPage {
  data: Assignment[]
  total: number
  limit: number
  offset: number
}

export interface CheckoutInput {
  asset_id: string
  employee_id: string
  checkout_date: string
  due_date?: string | null
  condition_out?: string | null
  notes?: string | null
}

export interface CheckinInput {
  checkin_date?: string | null
  condition_in?: AssetCondition | null
  needs_maintenance?: boolean
}

export interface BorrowInput {
  asset_id: string
  due_date?: string | null
  condition_out?: AssetCondition | null
  notes?: string | null
}

export interface SubmitResponse {
  request_id: string
  status: string
}

/** Asset assignment (penugasan) + Staf borrow (peminjaman), wired to /api/v1. */
export function useAssignment() {
  const { request } = useApiClient()

  async function list(q?: { status?: string, employee_id?: string, search?: string, limit?: number, offset?: number }): Promise<AssignmentListPage> {
    const query: Record<string, string | number> = {}
    if (q?.status) query.status = q.status
    if (q?.employee_id) query.employee_id = q.employee_id
    if (q?.search) query.search = q.search
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<AssignmentListPage>('/assignments', { query })
  }

  async function available(): Promise<{ data: AvailableAsset[] }> {
    return request<{ data: AvailableAsset[] }>('/assignments/available')
  }

  async function checkout(input: CheckoutInput): Promise<Assignment> {
    return request<Assignment>('/assignments', { method: 'POST', body: input })
  }

  async function checkin(id: string, input: CheckinInput): Promise<Assignment> {
    return request<Assignment>(`/assignments/${id}/checkin`, { method: 'POST', body: input })
  }

  async function borrow(input: BorrowInput): Promise<SubmitResponse> {
    return request<SubmitResponse>('/assignments/borrow', { method: 'POST', body: input })
  }

  // My submitted borrow requests (assignment type), for the "Pengajuan Saya" list.
  async function myRequests(q?: { status?: string, limit?: number, offset?: number }): Promise<{ data: Record<string, unknown>[], total: number }> {
    const query: Record<string, string | number> = { mine: 'true', type: 'assignment' }
    if (q?.status) query.status = q.status
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<{ data: Record<string, unknown>[], total: number }>('/requests', { query })
  }

  async function cancel(id: string): Promise<Record<string, unknown>> {
    return request<Record<string, unknown>>(`/requests/${id}/cancel`, { method: 'POST' })
  }

  return { list, available, checkout, checkin, borrow, myRequests, cancel }
}
