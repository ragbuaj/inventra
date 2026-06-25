import type { ScheduleItem, MaintRecord, DamageReport } from '~/mock/maintenance'
import { fakeLatency } from '~/mock/helpers'
import { maintenanceStore } from '~/mock/maintenance'

export function useMaintenance() {
  async function schedule(): Promise<ScheduleItem[]> {
    await fakeLatency(500)
    return maintenanceStore.schedule().map(s => ({ ...s }))
  }

  async function records(): Promise<MaintRecord[]> {
    await fakeLatency(600)
    return maintenanceStore.records().map(r => ({ ...r }))
  }

  async function reports(): Promise<DamageReport[]> {
    await fakeLatency(300)
    return maintenanceStore.reports().map(r => ({ ...r }))
  }

  async function addRecord(rec: MaintRecord): Promise<MaintRecord> {
    await fakeLatency()
    return maintenanceStore.addRecord(rec)
  }

  async function addReport(rep: DamageReport): Promise<DamageReport> {
    await fakeLatency()
    return maintenanceStore.addReport(rep)
  }

  return { schedule, records, reports, addRecord, addReport }
}
