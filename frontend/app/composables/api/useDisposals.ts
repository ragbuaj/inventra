import type { DisposalMethod } from '~/constants/disposalMeta'

export interface Disposal {
  id: string
  asset_id: string
  method: DisposalMethod
  disposal_date: string | null
  proceeds: string | null
  book_value_at_disposal: string | null
  gain_loss: string | null
  bast_no: string | null
  approved_by_id: string | null
  request_id: string | null
  created_by_id: string | null
  asset_name: string
  asset_tag: string
  office_name: string | null
  created_by_name: string | null
  created_at: string | null
  updated_at: string | null
}

export interface DisposalSubmitInput {
  asset_id: string
  method: DisposalMethod
  disposal_date: string
  proceeds?: string | null
  book_value_at_disposal?: string | null
  bast_no?: string | null
  reason?: string | null
}

export interface AttachDocumentInput {
  bast_no?: string
  doc_no?: string
  doc_date?: string
  counterparty?: string
  file?: File | null
}

export interface DisposalListPage {
  data: Disposal[]
  total: number
  limit: number
  offset: number
}

export interface DisposalResponse {
  request_id: string
  status: string
}

export interface AttachDocumentResponse {
  document_id: string
  disposal_id: string
}

/** Asset disposals (penghapusan), wired to /api/v1/disposals. */
export function useDisposals() {
  const { request } = useApiClient()

  async function list(q?: { limit?: number, offset?: number }): Promise<DisposalListPage> {
    const query: Record<string, string | number> = {}
    if (q?.limit !== undefined) query.limit = q.limit
    if (q?.offset !== undefined) query.offset = q.offset
    return request<DisposalListPage>('/disposals', { query })
  }

  async function get(id: string): Promise<Disposal> {
    return request<Disposal>(`/disposals/${id}`)
  }

  async function submit(input: DisposalSubmitInput): Promise<DisposalResponse> {
    return request<DisposalResponse>('/disposals', {
      method: 'POST',
      body: input
    })
  }

  async function attachDocument(id: string, input: AttachDocumentInput): Promise<AttachDocumentResponse> {
    // If file is present, use FormData (multipart); otherwise use JSON body
    if (input.file) {
      const formData = new FormData()
      if (input.bast_no) formData.append('bast_no', input.bast_no)
      if (input.doc_no) formData.append('doc_no', input.doc_no)
      if (input.doc_date) formData.append('doc_date', input.doc_date)
      if (input.counterparty) formData.append('counterparty', input.counterparty)
      formData.append('file', input.file)
      return request<AttachDocumentResponse>(`/disposals/${id}/document`, {
        method: 'POST',
        body: formData
      })
    } else {
      // No file, send plain JSON
      const body: Record<string, unknown> = {}
      if (input.bast_no) body.bast_no = input.bast_no
      if (input.doc_no) body.doc_no = input.doc_no
      if (input.doc_date) body.doc_date = input.doc_date
      if (input.counterparty) body.counterparty = input.counterparty
      return request<AttachDocumentResponse>(`/disposals/${id}/document`, {
        method: 'POST',
        body
      })
    }
  }

  return { list, get, submit, attachDocument }
}
