import type { AssetCreateInput } from '~/types'

export interface SubmittedRequest {
  id: string
  type: string
  status: string
  amount: string
  office_id: string
  created_at?: string
}

export interface ValuationExclusionInput {
  asset_id: string
  office_id: string
  reason: string
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

  // Valuation exclusion is a generic maker-checker request: the backend
  // (approval.SubmitRequest) requires type/amount/office_id and takes the
  // asset as target_id plus a free-text reason. Amount is bound "required"
  // server-side, so a literal "0" is sent — the exclusion has no monetary
  // amount of its own (mirrors the backend integration test's Submit body).
  async function submitValuationExclusion(input: ValuationExclusionInput): Promise<SubmittedRequest> {
    return request<SubmittedRequest>('/requests', {
      method: 'POST',
      body: {
        type: 'valuation_exclusion',
        amount: '0',
        office_id: input.office_id,
        target_id: input.asset_id,
        reason: input.reason
      }
    })
  }

  return { submitCreate, submitValuationExclusion }
}
