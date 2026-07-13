import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useOffices } from '~/composables/api/useOffices'

beforeEach(() => request.mockReset())

describe('useOffices', () => {
  it('list builds the query (omits empty search) and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'o1' }], total: 1, limit: 20, offset: 0 })
    const res = await useOffices().list({ limit: 20, offset: 0 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/offices?')
    expect(path).toContain('limit=20')
    expect(path).not.toContain('search=')
    expect(res.total).toBe(1)
  })

  it('list forwards the search term when present', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 20, offset: 0 })
    await useOffices().list({ search: 'pusat' })
    expect(request.mock.calls[0][0]).toContain('search=pusat')
  })

  it('create POSTs /offices with parent_id + required fields, omitting empty optionals', async () => {
    request.mockResolvedValueOnce({ id: 'o1' })
    await useOffices().create({ parent_id: null, office_type_id: 'ot1', province_id: null, city_id: null, name: 'Pusat', code: 'PST', is_active: true })
    expect(request).toHaveBeenCalledWith('/offices', { method: 'POST', body: { parent_id: null, office_type_id: 'ot1', name: 'Pusat', code: 'PST', is_active: true } })
  })

  it('create includes province/city/address/coordinates when set', async () => {
    request.mockResolvedValueOnce({ id: 'o2' })
    await useOffices().create({ parent_id: 'p1', office_type_id: 'ot1', province_id: 'pr1', city_id: 'c1', name: 'Cabang', code: 'CB1', address: 'Jl. X', is_active: false, latitude: -6.2, longitude: 106.8 })
    expect(request).toHaveBeenCalledWith('/offices', { method: 'POST', body: { parent_id: 'p1', office_type_id: 'ot1', name: 'Cabang', code: 'CB1', is_active: false, province_id: 'pr1', city_id: 'c1', address: 'Jl. X', latitude: -6.2, longitude: 106.8 } })
  })

  it('create keeps latitude 0 (only null/undefined are omitted)', async () => {
    request.mockResolvedValueOnce({ id: 'o3' })
    await useOffices().create({ parent_id: null, office_type_id: 'ot1', province_id: null, city_id: null, name: 'Nol', code: 'N0', is_active: true, latitude: 0, longitude: 0 })
    const body = request.mock.calls[0][1] as { body: Record<string, unknown> }
    expect(body.body.latitude).toBe(0)
    expect(body.body.longitude).toBe(0)
  })

  it('update PUTs /offices/:id', async () => {
    request.mockResolvedValueOnce({ id: 'o1' })
    await useOffices().update('o1', { parent_id: null, office_type_id: 'ot1', province_id: null, city_id: null, name: 'X', code: 'X1', is_active: true })
    expect(request).toHaveBeenCalledWith('/offices/o1', { method: 'PUT', body: { parent_id: null, office_type_id: 'ot1', name: 'X', code: 'X1', is_active: true } })
  })

  it('get GETs /offices/:id; remove DELETEs', async () => {
    request.mockResolvedValueOnce({ id: 'o1' })
    await useOffices().get('o1')
    expect(request).toHaveBeenCalledWith('/offices/o1')
    request.mockResolvedValueOnce(undefined)
    await useOffices().remove('o1')
    expect(request).toHaveBeenCalledWith('/offices/o1', { method: 'DELETE' })
  })

  it('tree GETs /offices/tree and returns the flat data array', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'o1' }, { id: 'o2' }], total: 2 })
    const res = await useOffices().tree()
    expect(request).toHaveBeenCalledWith('/offices/tree')
    expect(res).toEqual([{ id: 'o1' }, { id: 'o2' }])
  })
})
