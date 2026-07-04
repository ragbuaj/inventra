export interface PreviewStep {
  step_order: number
  required_level: string
}

export interface PreviewStepsResponse {
  steps: PreviewStep[]
}

/** Approval threshold preview for asset_transfer and asset_disposal requests. */
export function useApprovalPreview() {
  const { request } = useApiClient()

  async function preview(requestType: 'asset_disposal' | 'asset_transfer', amount: string): Promise<PreviewStep[]> {
    const query: Record<string, string> = {
      request_type: requestType,
      amount
    }
    const res = await request<PreviewStepsResponse>('/approval-thresholds/preview', { query })
    return res.steps
  }

  return { preview }
}
