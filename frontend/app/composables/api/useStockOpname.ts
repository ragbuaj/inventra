export interface OpnameSession {
  id: string
  office_id: string
  name: string | null
  period: string
  status: string
  started_by_id: string | null
  started_at: string | null
  closed_by_id: string | null
  closed_at: string | null
  created_at: string | null
  updated_at: string | null
  office_name: string | null
  started_by_name: string | null
  closed_by_name: string | null
}

export interface OpnameSessionKpi {
  total: number
  found: number
  pending: number
  variance: number
}

export type OpnameSessionDetail = OpnameSession & OpnameSessionKpi

export interface OpnameItem {
  id: string
  session_id: string
  asset_id: string
  asset_name: string | null
  asset_tag: string | null
  office_name: string | null
  room_name: string | null
  floor_name: string | null
  expected: boolean
  result: string
  note: string | null
  counted_by_name: string | null
  counted_at: string | null
  followup_request_id: string | null
}

export interface OpnameSessionListPage {
  data: OpnameSession[]
  total: number
  limit: number
  offset: number
}

export interface OpnameItemListPage {
  data: OpnameItem[]
  total: number
  limit: number
  offset: number
}

export interface CreateSessionInput {
  office_id: string
  name?: string
  period: string
}

export interface ScanResponse {
  id: string
  session_id: string
  asset_id: string
  expected: boolean
  result: string
}

export interface SetResultInput {
  result: string
  note?: string | null
}

export interface SetResultResponse {
  id: string
  session_id: string
  asset_id: string
  expected: boolean
  result: string
  note: string | null
  counted_at: string | null
}

export interface FollowupInput {
  to_office_id?: string | null
  to_room_id?: string | null
  reason?: string | null
}

export interface FollowupResponse {
  request_id: string
  type: string
}

/** Physical stock-take (stock opname), wired to /api/v1/stock-opname/sessions. */
export function useStockOpname() {
  const { request, requestBlob } = useApiClient()

  async function list(q?: { status?: string, limit?: number, offset?: number }): Promise<OpnameSessionListPage> {
    const query: Record<string, string | number> = {}
    if (q?.status !== undefined) query.status = q.status
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<OpnameSessionListPage>('/stock-opname/sessions', { query })
  }

  async function get(id: string): Promise<OpnameSessionDetail> {
    return request<OpnameSessionDetail>(`/stock-opname/sessions/${id}`)
  }

  async function items(id: string, q?: { result?: string }): Promise<OpnameItemListPage> {
    const query: Record<string, string> = {}
    if (q?.result !== undefined) query.result = q.result
    return request<OpnameItemListPage>(`/stock-opname/sessions/${id}/items`, { query })
  }

  async function create(input: CreateSessionInput): Promise<OpnameSessionDetail> {
    return request<OpnameSessionDetail>('/stock-opname/sessions', {
      method: 'POST',
      body: input
    })
  }

  async function start(id: string): Promise<OpnameSessionDetail> {
    return request<OpnameSessionDetail>(`/stock-opname/sessions/${id}/start`, {
      method: 'POST'
    })
  }

  async function scan(id: string, assetTag: string): Promise<ScanResponse> {
    return request<ScanResponse>(`/stock-opname/sessions/${id}/scan`, {
      method: 'POST',
      body: { asset_tag: assetTag }
    })
  }

  async function setResult(id: string, itemId: string, input: SetResultInput): Promise<SetResultResponse> {
    return request<SetResultResponse>(`/stock-opname/sessions/${id}/items/${itemId}`, {
      method: 'PATCH',
      body: input
    })
  }

  async function reconcile(id: string): Promise<OpnameSessionDetail> {
    return request<OpnameSessionDetail>(`/stock-opname/sessions/${id}/reconcile`, {
      method: 'POST'
    })
  }

  async function followup(id: string, itemId: string, input: FollowupInput): Promise<FollowupResponse> {
    return request<FollowupResponse>(`/stock-opname/sessions/${id}/items/${itemId}/follow-up`, {
      method: 'POST',
      body: input
    })
  }

  async function close(id: string): Promise<OpnameSessionDetail> {
    return request<OpnameSessionDetail>(`/stock-opname/sessions/${id}/close`, {
      method: 'POST'
    })
  }

  async function reportUrl(id: string, format: 'pdf' | 'xlsx'): Promise<Blob> {
    return requestBlob(`/stock-opname/sessions/${id}/report`, {
      query: { format }
    })
  }

  return { list, get, items, create, start, scan, setResult, reconcile, followup, close, reportUrl }
}
