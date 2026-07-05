import type { DepreciationBasis, PeriodStatus } from '~/constants/depreciationMeta'

export interface DepreciationPeriod {
  period: string
  status: PeriodStatus
  asset_count: number
  total_amount: string
  skipped_count: number
}

export interface ScheduleRow {
  asset_id: string
  asset_name: string
  asset_tag: string
  category_name: string | null
  office_name: string | null
  method: string
  life_months: number
  opening: string
  amount: string
  accumulated: string
  closing: string
  impaired: boolean
  fully_depreciated: boolean
}

export interface ScheduleResponse {
  kpi: {
    total_cost: string
    total_accumulated: string
    total_book_value: string
    period_expense: string
  }
  rows: ScheduleRow[]
  totals: {
    opening: string
    amount: string
    accumulated: string
    closing: string
  }
}

export interface JournalRow {
  account_code: string
  account_name: string
  debit: string
  credit: string
}

export interface JournalResponse {
  rows: JournalRow[]
  total_debit: string
  total_credit: string
  balanced: boolean
}

export interface AssetDepreciationEntry {
  basis: DepreciationBasis
  period: string
  opening: string
  amount: string
  closing: string
  method: string
}

export interface AssetDepreciationResponse {
  masked: boolean
  computed_book_value: string | null
  entries: AssetDepreciationEntry[]
}

export interface ScheduleQuery {
  period: string
  basis: DepreciationBasis
  search?: string
  category_id?: string
  office_id?: string
}

export interface ImpairmentResult {
  book_value: string
  impairment_loss: string
}

/** Lean response of POST /depreciation/periods/:period/close ({period, status} only). */
export interface ClosedPeriod {
  period: string
  status: PeriodStatus
}

/** Depreciation (penyusutan): periods, per-asset schedule, journal recap, impairment. */
export function useDepreciation() {
  const { request, requestBlob } = useApiClient()

  async function periods(): Promise<DepreciationPeriod[]> {
    // Backend wraps the list in a {data: [...]} envelope.
    return (await request<{ data: DepreciationPeriod[] }>('/depreciation/periods')).data
  }

  async function compute(period: string): Promise<DepreciationPeriod> {
    // `period` is a PATH param; the endpoint takes no body.
    return request<DepreciationPeriod>(`/depreciation/periods/${period}/compute`, {
      method: 'POST'
    })
  }

  async function close(period: string): Promise<ClosedPeriod> {
    // `period` is a PATH param; response is the lean {period, status} shape.
    return request<ClosedPeriod>(`/depreciation/periods/${period}/close`, {
      method: 'POST'
    })
  }

  async function schedule(q: ScheduleQuery): Promise<ScheduleResponse> {
    const query: Record<string, string> = { period: q.period, basis: q.basis }
    if (q.search !== undefined) query.search = q.search
    if (q.category_id !== undefined) query.category_id = q.category_id
    if (q.office_id !== undefined) query.office_id = q.office_id
    return request<ScheduleResponse>('/depreciation/schedule', { query })
  }

  async function journal(period: string, basis: DepreciationBasis): Promise<JournalResponse> {
    return request<JournalResponse>('/depreciation/journal', {
      query: { period, basis }
    })
  }

  async function exportJournal(period: string, basis: DepreciationBasis, format: 'xlsx' | 'pdf'): Promise<Blob> {
    return requestBlob('/depreciation/journal/export', {
      query: { period, basis, format }
    })
  }

  async function assetSchedule(assetId: string): Promise<AssetDepreciationResponse> {
    // Read is mounted under the /assets prefix, suffix `/depreciation`.
    return request<AssetDepreciationResponse>(`/assets/${assetId}/depreciation`)
  }

  async function recordImpairment(assetId: string, recoverable: string, reason: string): Promise<ImpairmentResult> {
    return request<ImpairmentResult>(`/assets/${assetId}/impairment`, {
      method: 'POST',
      body: { recoverable_amount: recoverable, reason }
    })
  }

  return { periods, compute, close, schedule, journal, exportJournal, assetSchedule, recordImpairment }
}
