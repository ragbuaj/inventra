import type { PeriodValue } from '~/constants/reportMeta'
import { periodToQuery } from '~/constants/reportMeta'

export interface DashboardTrends {
  acquisition_pct: number | null
  book_value_pct: number | null
  maintenance_cost_pct: number | null
}

export interface DashboardKpi {
  total_assets: number
  acquisition_value: string
  book_value: string
  overdue_assets: number
  maintenance_due: number
  maintenance_cost: string
  trends: DashboardTrends
}

export interface StatusCount {
  status: string
  count: number
}

/** `name: null` is the "no category" / "no room" bucket — callers localize it. */
export interface NamedCount {
  name: string | null
  count: number
}

export interface MaintenanceDueItem {
  id: string
  asset_name: string
  asset_tag: string
  category_name: string | null
  next_due_date: string // YYYY-MM-DD
}

/** The JSON body of GET /dashboard/summary (report/dto.go DashboardSummary). */
export interface DashboardSummary {
  office_name: string | null
  kpi: DashboardKpi
  by_status: StatusCount[]
  by_category: NamedCount[]
  location_kind: 'office' | 'room'
  by_location: NamedCount[]
  maintenance_due_list: MaintenanceDueItem[]
  excluded_count: number
}

export interface DashboardQuery {
  officeId?: string
  period: PeriodValue
}

/**
 * Compat aliases kept ONLY so `DashboardMaintenancePanel`/`DashboardApprovalPanel`
 * (which still render the mockup's localized row shapes) keep compiling. They are
 * NOT part of the backend contract — the backend has no `appr` field at all, and
 * `maintenance_due_list` above is shaped differently. Tasks 12/13 rework the
 * panels/pages onto the real shapes; these aliases are removed then.
 */
export interface MaintenanceItem {
  asset: string
  task: string
  icon: string
  urg: 0 | 1
  due: string
}

export interface ApprovalItem {
  id: string
  title: string
  meta: string
  icon: string
  tone: 'info' | 'primary' | 'neutral'
}

/** Dashboard aggregates: KPIs, status/category/location breakdowns, maintenance-due list. */
export function useDashboard() {
  const { request, requestBlob } = useApiClient()

  function buildQuery(q: DashboardQuery): Record<string, string> {
    const query: Record<string, string> = { ...periodToQuery(q.period) }
    if (q.officeId !== undefined) query.office_id = q.officeId
    return query
  }

  async function summary(q: DashboardQuery): Promise<DashboardSummary> {
    return request<DashboardSummary>('/dashboard/summary', { query: buildQuery(q) })
  }

  async function exportSummary(q: DashboardQuery, format: 'xlsx' | 'pdf'): Promise<Blob> {
    return requestBlob('/dashboard/export', { query: { ...buildQuery(q), format } })
  }

  return { summary, exportSummary }
}
