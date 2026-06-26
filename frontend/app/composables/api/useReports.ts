import type { ReportKey, ReportResult } from '~/mock/reports'
import { fakeLatency } from '~/mock/helpers'
import { computeReport } from '~/mock/reports'

export function useReports() {
  async function run(report: ReportKey, filters: { kat?: string, status?: string }): Promise<ReportResult> {
    await fakeLatency(500)
    return computeReport(report, filters)
  }

  return { run }
}
