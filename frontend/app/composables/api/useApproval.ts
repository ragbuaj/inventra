import type { ApprovalRequest } from '~/mock/approval'
import { fakeLatency } from '~/mock/helpers'
import { approvalStore } from '~/mock/approval'

export function useApproval() {
  async function list(): Promise<ApprovalRequest[]> {
    await fakeLatency(600)
    return approvalStore.all().map(r => ({ ...r, timeline: r.timeline.map(e => ({ ...e })) }))
  }

  async function decide(id: string, action: 'approved' | 'rejected', note: string): Promise<ApprovalRequest> {
    await fakeLatency()
    const row = approvalStore.decide(id, action, note)
    if (!row) throw new Error('approval.errNotFound')
    return row
  }

  return { list, decide }
}
