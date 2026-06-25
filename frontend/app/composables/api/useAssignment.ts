import type { Assignment, AssetCondition, AvailableAsset } from '~/mock/assignment'
import { fakeLatency } from '~/mock/helpers'
import { assignmentStore } from '~/mock/assignment'

export interface CheckoutInput {
  tag: string
  nama: string
  pemegang: string
  ini: string
  pinjam: string
  kondisi: AssetCondition
}

export function useAssignment() {
  async function list(): Promise<Assignment[]> {
    await fakeLatency(700)
    return assignmentStore.all().map(r => ({ ...r }))
  }

  async function available(): Promise<AvailableAsset[]> {
    await fakeLatency(300)
    return assignmentStore.available()
  }

  async function checkout(input: CheckoutInput): Promise<Assignment> {
    await fakeLatency()
    return assignmentStore.checkout(input)
  }

  async function checkin(id: string, input: { kembali: string, kondisi: AssetCondition }): Promise<Assignment> {
    await fakeLatency()
    const row = assignmentStore.checkin(id, input)
    if (!row) throw new Error('assignment.errNotFound')
    return row
  }

  return { list, available, checkout, checkin }
}
