import { describe, it, expect, beforeEach } from 'vitest'
import { approvalStore, approvalSeed, loc, REQ_TYPE_KEYS } from '~/mock/approval'

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
