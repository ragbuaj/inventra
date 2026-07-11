import type { MaintenanceStatus, MaintenanceType } from '~/constants/maintenanceMeta'

export interface MaintenanceSchedule {
  id: string
  asset_id: string
  maintenance_category_id: string | null
  interval_months: number
  last_done_date: string | null
  next_due_date: string | null
  is_active: boolean
  asset_name: string | null
  asset_tag: string | null
  office_name: string | null
  category_name: string | null
  created_at: string | null
  updated_at: string | null
}

export interface SchedulePage {
  data: MaintenanceSchedule[]
  total: number
  limit: number
  offset: number
}

export interface MaintenanceRecord {
  id: string
  asset_id: string
  schedule_id: string | null
  maintenance_category_id: string | null
  problem_category_id: string | null
  type: MaintenanceType
  status: MaintenanceStatus
  scheduled_date: string | null
  completed_date: string | null
  cost: string | null
  vendor_id: string | null
  performed_by: string | null
  description: string
  reported_by_id: string | null
  asset_name: string | null
  asset_tag: string | null
  office_name: string | null
  category_name: string | null
  problem_name: string | null
  vendor_name: string | null
  reported_by_name: string | null
  created_at: string | null
  updated_at: string | null
}

export interface RecordPage {
  data: MaintenanceRecord[]
  total: number
  limit: number
  offset: number
}

export interface AttentionItem {
  id: string
  asset_tag: string
  name: string
  office_id: string
  office_name: string | null
}

export interface CreateScheduleInput {
  asset_id: string
  maintenance_category_id?: string | null
  interval_months: number
  start_date: string
}

export interface UpdateScheduleInput {
  maintenance_category_id?: string | null
  interval_months?: number
  is_active?: boolean
}

export interface CreateRecordInput {
  asset_id: string
  schedule_id?: string | null
  maintenance_category_id?: string | null
  problem_category_id?: string | null
  type: MaintenanceType
  status?: MaintenanceStatus
  scheduled_date?: string | null
  completed_date?: string | null
  cost?: string | null
  vendor_id?: string | null
  description: string
}

export interface UpdateRecordInput {
  status?: MaintenanceStatus
  maintenance_category_id?: string | null
  scheduled_date?: string | null
  completed_date?: string | null
  cost?: string | null
  vendor_id?: string | null
  description?: string
}

export interface SubmitReportInput {
  asset_id: string
  problem_category_id: string
  description?: string | null
  photo?: File | null
}

export interface SubmitReportResponse {
  request_id: string
  status: string
}

/** Maintenance (jadwal preventif, catatan, laporan kerusakan), wired to /api/v1/maintenance. */
export function useMaintenance() {
  const { request } = useApiClient()

  async function schedules(q?: { is_active?: boolean, limit?: number, offset?: number }): Promise<SchedulePage> {
    const query: Record<string, string | number> = {}
    if (q?.is_active !== undefined) query.is_active = q.is_active ? 'true' : 'false'
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<SchedulePage>('/maintenance/schedules', { query })
  }

  async function createSchedule(input: CreateScheduleInput): Promise<MaintenanceSchedule> {
    return request<MaintenanceSchedule>('/maintenance/schedules', { method: 'POST', body: input })
  }

  async function updateSchedule(id: string, input: UpdateScheduleInput): Promise<MaintenanceSchedule> {
    return request<MaintenanceSchedule>(`/maintenance/schedules/${id}`, { method: 'PATCH', body: input })
  }

  async function deleteSchedule(id: string): Promise<void> {
    await request(`/maintenance/schedules/${id}`, { method: 'DELETE' })
  }

  async function records(q?: { status?: string, type?: string, q?: string, limit?: number, offset?: number }): Promise<RecordPage> {
    const query: Record<string, string | number> = {}
    if (q?.status) query.status = q.status
    if (q?.type) query.type = q.type
    if (q?.q) query.q = q.q
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<RecordPage>('/maintenance/records', { query })
  }

  async function record(id: string): Promise<MaintenanceRecord> {
    return request<MaintenanceRecord>(`/maintenance/records/${id}`)
  }

  async function createRecord(input: CreateRecordInput): Promise<MaintenanceRecord> {
    return request<MaintenanceRecord>('/maintenance/records', { method: 'POST', body: input })
  }

  async function updateRecord(id: string, input: UpdateRecordInput): Promise<MaintenanceRecord> {
    return request<MaintenanceRecord>(`/maintenance/records/${id}`, { method: 'PATCH', body: input })
  }

  async function attention(): Promise<{ data: AttentionItem[] }> {
    return request<{ data: AttentionItem[] }>('/maintenance/attention')
  }

  async function listByAsset(assetId: string): Promise<{ data: MaintenanceRecord[] }> {
    return request<{ data: MaintenanceRecord[] }>(`/assets/${assetId}/maintenance`)
  }

  async function submitReport(input: SubmitReportInput): Promise<SubmitReportResponse> {
    const formData = new FormData()
    formData.append('asset_id', input.asset_id)
    formData.append('problem_category_id', input.problem_category_id)
    if (input.description) formData.append('description', input.description)
    if (input.photo) formData.append('photo', input.photo)
    return request<SubmitReportResponse>('/maintenance/reports', { method: 'POST', body: formData })
  }

  // My submitted damage reports (maintenance type), for the "Riwayat Laporan" list.
  async function myReports(q?: { status?: string, limit?: number, offset?: number }): Promise<{ data: Record<string, unknown>[], total: number }> {
    const query: Record<string, string | number> = { mine: 'true', type: 'maintenance' }
    if (q?.status) query.status = q.status
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<{ data: Record<string, unknown>[], total: number }>('/requests', { query })
  }

  return { schedules, createSchedule, updateSchedule, deleteSchedule, records, record, createRecord, updateRecord, attention, listByAsset, submitReport, myReports }
}
