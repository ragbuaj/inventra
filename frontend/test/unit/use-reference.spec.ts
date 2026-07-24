import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useReference } from '~/composables/api/useReference'
// eslint-disable-next-line import/first
import { referenceResources } from '~/composables/api/referenceResources'

beforeEach(() => request.mockReset())

describe('useReference', () => {
  // Asserting the exact key set (not just a count) makes this fail loudly with a
  // readable diff when a resource is added or removed — a bare length check only
  // says "expected 11, got 15". The last four landed with legacy-parity Fase 4.
  it('declares every reference resource (descriptor sanity)', () => {
    expect(referenceResources.map(r => r.key)).toEqual([
      'office-types', 'departments', 'positions', 'units',
      'maintenance-categories', 'problem-categories', 'brands', 'vendors',
      'provinces', 'cities', 'models',
      'office-classes', 'executor-divisions', 'companies', 'building-classifications'
    ])
  })

  it('list builds /key query (omits empty search) and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'a', name: 'A' }], total: 1, limit: 20, offset: 0 })
    const res = await useReference().list('office-types', { limit: 20, offset: 0 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/office-types?')
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=0')
    expect(path).not.toContain('search=')
    expect(res.total).toBe(1)
  })

  it('list includes search when present', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 20, offset: 0 })
    await useReference().list('cities', { search: 'jak', limit: 20, offset: 0 })
    expect(request.mock.calls[0][0]).toContain('search=jak')
  })

  it('create POSTs to /key with the body verbatim (is_active + FK keys)', async () => {
    request.mockResolvedValueOnce({ id: 'c1', name: 'Jakarta' })
    await useReference().create('cities', { province_id: 'p1', name: 'Jakarta', code: '31', is_active: true })
    expect(request).toHaveBeenCalledWith('/cities', { method: 'POST', body: { province_id: 'p1', name: 'Jakarta', code: '31', is_active: true } })
  })

  it('update PUTs to /key/:id', async () => {
    request.mockResolvedValueOnce({ id: 'o1', name: 'KP', tier: 'pusat' })
    await useReference().update('office-types', 'o1', { name: 'KP', tier: 'pusat', is_active: true })
    expect(request).toHaveBeenCalledWith('/office-types/o1', { method: 'PUT', body: { name: 'KP', tier: 'pusat', is_active: true } })
  })

  it('remove DELETEs /key/:id', async () => {
    request.mockResolvedValueOnce(undefined)
    await useReference().remove('brands', 'b1')
    expect(request).toHaveBeenCalledWith('/brands/b1', { method: 'DELETE' })
  })
})
