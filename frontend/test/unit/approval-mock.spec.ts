import { describe, it, expect, beforeEach } from 'vitest'
import { useApproval } from '~/composables/api/useApproval'
import { approvalStore, approvalSeed, loc, REQ_TYPE_KEYS } from '~/mock/approval'

const { list, decide } = useApproval()

beforeEach(() => approvalStore.reset())

describe('mock/approval — seeds', () => {
  it('seeds 7 requests (5 pending, 1 approved, 1 rejected) across 5 types', () => {
    expect(approvalSeed).toHaveLength(7)
    expect(approvalSeed.filter(r => r.status === 'pending')).toHaveLength(5)
    expect(approvalSeed.filter(r => r.status === 'approved')).toHaveLength(1)
    expect(approvalSeed.filter(r => r.status === 'rejected')).toHaveLength(1)
    expect(REQ_TYPE_KEYS).toHaveLength(5)
    expect(approvalStore.pendingCount()).toBe(5)
  })

  it('exposes both summary and diff request shapes', () => {
    expect(approvalSeed.find(r => r.id === 'r1')!.summary).toBeDefined()
    expect(approvalSeed.find(r => r.id === 'r2')!.diff).toBeDefined()
  })
})

describe('loc()', () => {
  it('resolves localized fields and passes plain strings through', () => {
    const r1 = approvalSeed.find(r => r.id === 'r1')!
    expect(loc(r1.role, 'en')).toBe('Asset Manager')
    expect(loc(r1.judul, 'id')).toBe('Registrasi 12 Laptop Asus ExpertBook B1')
  })
})

describe('useApproval', () => {
  it('lists all requests', async () => {
    expect(await list()).toHaveLength(7)
  })

  it('approve flips status, appends a timeline entry with the note, and drops the pending count', async () => {
    const before = approvalStore.find('r1')!.timeline.length
    const updated = await decide('r1', 'approved', 'Sesuai anggaran')
    expect(updated.status).toBe('approved')
    expect(updated.timeline).toHaveLength(before + 1)
    const last = updated.timeline[updated.timeline.length - 1]!
    expect(last.action).toBe('approved')
    expect(last.note).toBe('Sesuai anggaran')
    expect(approvalStore.pendingCount()).toBe(4)
  })

  it('reject flips status to rejected', async () => {
    const updated = await decide('r3', 'rejected', '')
    expect(updated.status).toBe('rejected')
    expect(approvalStore.pendingCount()).toBe(4)
  })

  it('throws the sentinel error deciding a missing request', async () => {
    await expect(decide('nope', 'approved', '')).rejects.toThrow('approval.errNotFound')
  })

  it('reset restores the seed statuses', async () => {
    await decide('r1', 'approved', '')
    approvalStore.reset()
    expect(approvalStore.pendingCount()).toBe(5)
  })
})
