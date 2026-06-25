import type { DashboardData, Localized, Scope } from '~/mock/dashboard'
import { dashboardData } from '~/mock/dashboard'
import { fakeLatency } from '~/mock/helpers'

export type Locale = 'id' | 'en'

/** Maintenance row with localized text resolved (a real API would return it pre-localized). */
export interface MaintenanceItem {
  asset: string
  task: string
  icon: string
  urg: 0 | 1
  due: string
}

/** Approval row with localized text resolved. */
export interface ApprovalItem {
  id: string
  title: string
  meta: string
  icon: string
  tone: 'info' | 'primary' | 'neutral'
}

/** The dashboard payload as consumed by the page — every string already in the active locale. */
export interface DashboardSummary {
  scope: Scope
  name: string
  total: number
  perolehan: string
  buku: string
  overdue: number
  due: number
  biaya: string
  status: number[]
  kategori: [string, number][]
  lokasi: [string, number][]
  maint: MaintenanceItem[]
  appr: ApprovalItem[]
}

/**
 * Dashboard data source. Mock-first today; the single seam a real implementation swaps behind
 * (`$fetch('/dashboard/summary', { query: { scope, period } })`). `period` is accepted but cosmetic —
 * it only triggers a reload, matching the mockup, which shows the same figures for every period.
 */
export function useDashboard() {
  async function summary(scope: Scope, _period: string, locale: Locale = 'id'): Promise<DashboardSummary> {
    await fakeLatency(700)
    const d: DashboardData = dashboardData[scope] ?? dashboardData.jaksel
    const pick = (l: Localized) => l[locale] ?? l.id
    return {
      scope: d.scope,
      name: pick(d.name),
      total: d.total,
      perolehan: d.perolehan,
      buku: d.buku,
      overdue: d.overdue,
      due: d.due,
      biaya: d.biaya,
      status: d.status,
      kategori: d.kategori,
      lokasi: d.lokasi,
      maint: d.maint.map(m => ({ asset: m.asset, task: pick(m.task), icon: m.icon, urg: m.urg, due: pick(m.due) })),
      appr: d.appr.map(a => ({ id: a.id, title: pick(a.title), meta: pick(a.meta), icon: a.icon, tone: a.tone }))
    }
  }

  return { summary }
}
