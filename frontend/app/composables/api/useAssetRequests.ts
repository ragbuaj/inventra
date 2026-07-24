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

/**
 * Multiplies a non-negative plain-decimal string by a positive integer without
 * floating-point loss, so the submit `amount` exactly matches the backend's
 * big.Rat cross-check (amount == purchase_cost * quantity). Uses BigInt on the
 * scaled integer so "3000000.25" * 3 stays "9000000.75", not 9000000.749999.
 * Trailing zeros are stripped, so "1500000.50" * 3 returns "4500001.5".
 */
export function multiplyDecimalByInt(decimal: string, factor: number): string {
  const [intPart = '0', fracPart = ''] = decimal.trim().split('.')
  const scaled = BigInt(intPart + fracPart) * BigInt(factor)
  const scale = fracPart.length
  if (scale === 0) return scaled.toString()
  const digits = scaled.toString().padStart(scale + 1, '0')
  const whole = digits.slice(0, -scale)
  const frac = digits.slice(-scale).replace(/0+$/, '')
  return frac ? `${whole}.${frac}` : whole
}

/** Maker-checker asset creation requests, wired to /api/v1/requests. */
export function useAssetRequests() {
  const { request } = useApiClient()

  async function submitCreate(input: AssetCreateInput): Promise<SubmittedRequest> {
    const quantity = input.quantity && input.quantity > 0 ? input.quantity : 1
    // Empty/null purchase_cost normalizes to "0" so the amount math never sees a
    // blank string (BigInt("") throws).
    const cost = input.purchase_cost?.trim() || '0'
    // A single unit needs no arithmetic — pass the cost through verbatim so the
    // decimal string the user typed ("18500000.00") reaches the server unchanged
    // (the multiply helper would normalize away its trailing zeros).
    const amount = quantity === 1 ? cost : multiplyDecimalByInt(cost, quantity)
    return request<SubmittedRequest>('/requests', {
      method: 'POST',
      body: {
        type: 'asset_create',
        amount,
        office_id: input.office_id,
        payload: { ...input, quantity }
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
