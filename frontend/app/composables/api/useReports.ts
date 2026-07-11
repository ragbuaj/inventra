import type { PeriodValue, ReportKey } from '~/constants/reportMeta'
import { periodToQuery } from '~/constants/reportMeta'

export interface ReportKpi { key: string, value: string }
export interface ChartBar { label: string, value: string }

export interface AssetReportRow {
  asset_tag: string
  name: string
  category_name: string
  status: string
  purchase_cost: string
  accum_deprec: string
  book_value: string
}

export interface DeprReportRow {
  period: string
  opening: string
  amount: string
  closing: string
}

export interface UtilReportRow {
  name: string
  asset_tag: string
  category_name: string
  days_loaned: number
  loan_count: number
  utilization_pct: number
}

export interface MaintReportRow {
  asset_name: string
  category_name: string
  type: string
  actions: number
  total_cost: string
}

/**
 * Backend note (report/service.go): shipped_date/received_date/bast_no are
 * plain `string`, NOT `string | null` — they arrive as `""` when the transfer
 * hasn't shipped/received or has no BAST number yet (formatDate/strOrEmpty
 * never emit null). This deliberately deviates from the task-11 brief, which
 * listed them as `string | null`.
 */
export interface TransferReportRow {
  asset_name: string
  asset_tag: string
  from_office: string
  to_office: string
  status: string
  shipped_date: string
  received_date: string
  bast_no: string
}

export interface DisposalReportRow {
  asset_name: string
  asset_tag: string
  method: string
  disposal_date: string
  book_value: string
  proceeds: string
  gain_loss: string
}

export interface OpnameReportRow {
  session_id: string
  name: string
  office_name: string
  period: string
  status: string
  total_items: number
  variance: number
}

export type ReportRow
  = | AssetReportRow
    | DeprReportRow
    | UtilReportRow
    | MaintReportRow
    | TransferReportRow
    | DisposalReportRow
    | OpnameReportRow

/** The JSON body of GET /reports/:type (report/dto.go ReportResult). */
export interface ReportResult {
  type: ReportKey
  kpis: ReportKpi[]
  chart: ChartBar[]
  rows: ReportRow[]
  totals: Record<string, string>
  row_count: number
  truncated: boolean
}

export interface ReportFilters {
  period: PeriodValue
  officeId?: string
  categoryId?: string
  status?: string
  basis?: 'commercial' | 'fiscal'
}

/**
 * Report builder (7 types): JSON run + xlsx/pdf export, plus the stock-opname
 * physical-count Berita Acara export (mounted under /stock-opname, not /reports).
 */
export function useReports() {
  const { request, requestBlob } = useApiClient()

  function buildQuery(f: ReportFilters): Record<string, string> {
    const query: Record<string, string> = { ...periodToQuery(f.period) }
    if (f.officeId !== undefined) query.office_id = f.officeId
    if (f.categoryId !== undefined) query.category_id = f.categoryId
    if (f.status !== undefined) query.status = f.status
    if (f.basis !== undefined) query.basis = f.basis
    return query
  }

  async function run(type: ReportKey, f: ReportFilters): Promise<ReportResult> {
    return request<ReportResult>(`/reports/${type}`, { query: buildQuery(f) })
  }

  async function exportReport(
    type: ReportKey,
    f: ReportFilters,
    format: 'xlsx' | 'pdf',
    variant?: 'table' | 'gl_recap'
  ): Promise<Blob> {
    const query: Record<string, string> = { ...buildQuery(f), format }
    if (variant !== undefined) query.variant = variant
    return requestBlob(`/reports/${type}/export`, { query })
  }

  async function opnameBa(sessionId: string, format: 'xlsx' | 'pdf'): Promise<Blob> {
    return requestBlob(`/stock-opname/sessions/${sessionId}/report`, { query: { format } })
  }

  return { run, exportReport, opnameBa }
}
