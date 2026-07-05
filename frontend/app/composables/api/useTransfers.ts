import type { TransferCondition } from '~/constants/transferMeta'

export interface Transfer {
  id: string
  asset_id: string
  from_office_id: string
  to_office_id: string
  to_room_id: string | null
  status: 'approved' | 'in_transit' | 'received' | 'returned'
  reason: string | null
  requested_by_id: string
  approved_by_id: string | null
  shipped_date: string | null
  received_date: string | null
  received_by_id: string | null
  bast_no: string | null
  request_id: string | null
  condition_sent: TransferCondition | null
  transfer_date: string | null
  return_note: string | null
  asset_name: string | null
  asset_tag: string | null
  from_office_name: string | null
  to_office_name: string | null
  to_room_name: string | null
  requested_by_name: string | null
  received_by_name: string | null
  created_at: string | null
  updated_at: string | null
}

export interface TransferSubmitInput {
  asset_id: string
  to_office_id: string
  to_room_id?: string | null
  reason?: string | null
  condition_sent: TransferCondition
  transfer_date: string
}

export interface ReceiveInput {
  bast_no?: string
  received_date?: string
  to_room_id?: string
  file?: File | null
}

export interface TransferListPage {
  data: Transfer[]
  total: number
  limit: number
  offset: number
}

export interface TransferResponse {
  request_id: string
  status: string
}

/** Asset transfers (mutasi), wired to /api/v1/transfers. */
export function useTransfers() {
  const { request } = useApiClient()

  async function list(q?: { status?: string, limit?: number, offset?: number }): Promise<TransferListPage> {
    const query: Record<string, string | number> = {}
    if (q?.status) query.status = q.status
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<TransferListPage>('/transfers', { query })
  }

  async function get(id: string): Promise<Transfer> {
    return request<Transfer>(`/transfers/${id}`)
  }

  async function submit(input: TransferSubmitInput): Promise<TransferResponse> {
    return request<TransferResponse>('/transfers', {
      method: 'POST',
      body: input
    })
  }

  async function ship(id: string, shippedDate?: string): Promise<Transfer> {
    return request<Transfer>(`/transfers/${id}/ship`, {
      method: 'POST',
      body: { shipped_date: shippedDate || undefined }
    })
  }

  async function receive(id: string, input: ReceiveInput): Promise<Transfer> {
    // If file is present, use FormData (multipart); otherwise use JSON body
    if (input.file) {
      const formData = new FormData()
      if (input.bast_no) formData.append('bast_no', input.bast_no)
      if (input.received_date) formData.append('received_date', input.received_date)
      if (input.to_room_id) formData.append('to_room_id', input.to_room_id)
      formData.append('file', input.file)
      return request<Transfer>(`/transfers/${id}/receive`, {
        method: 'POST',
        body: formData
      })
    } else {
      // No file, send plain JSON
      const body: Record<string, unknown> = {}
      if (input.bast_no) body.bast_no = input.bast_no
      if (input.received_date) body.received_date = input.received_date
      if (input.to_room_id) body.to_room_id = input.to_room_id
      return request<Transfer>(`/transfers/${id}/receive`, {
        method: 'POST',
        body
      })
    }
  }

  async function rejectReceive(id: string, note?: string): Promise<Transfer> {
    return request<Transfer>(`/transfers/${id}/reject-receive`, {
      method: 'POST',
      body: { note: note || undefined }
    })
  }

  return { list, get, submit, ship, receive, rejectReceive }
}
