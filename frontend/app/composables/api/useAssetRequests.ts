import type { AssetCreateInput } from '~/types'

export interface SubmittedRequest {
  id: string
  type: string
  status: string
  amount: string
  office_id: string
  created_at?: string
}

/** Maker-checker asset creation requests, wired to /api/v1/requests. */
export function useAssetRequests() {
  const { request } = useApiClient()

  async function submitCreate(input: AssetCreateInput): Promise<SubmittedRequest> {
    return request<SubmittedRequest>('/requests', {
      method: 'POST',
      body: {
        type: 'asset_create',
        amount: input.purchase_cost ?? '0',
        office_id: input.office_id,
        payload: input
      }
    })
  }

  return { submitCreate }
}
