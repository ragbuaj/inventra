import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useOfficeMap } from '~/composables/api/useOfficeMap'

beforeEach(() => request.mockReset())

describe('useOfficeMap', () => {
  it('GETs /offices/map and passes through resolved fields', async () => {
    request.mockResolvedValueOnce({ data: [{
      id: 'o1', name: 'Kantor Pusat', code: 'PST', office_type_name: 'Kantor Pusat',
      tier: 'pusat', province_name: 'DKI Jakarta', city_name: 'Jakarta Pusat',
      address: 'Jl. Merdeka 1', asset_count: 12, latitude: -6.1754, longitude: 106.8272
    }] })
    const rows = await useOfficeMap().list()
    expect(request).toHaveBeenCalledWith('/offices/map')
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({ id: 'o1', name: 'Kantor Pusat', tier: 'pusat', province_name: 'DKI Jakarta', asset_count: 12, latitude: -6.1754 })
  })

  it('maps null / office_subtree / office tier to the office bucket', async () => {
    request.mockResolvedValueOnce({ data: [
      { id: 'a', name: 'A', code: 'A', office_type_name: null, tier: null, province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null },
      { id: 'b', name: 'B', code: 'B', office_type_name: 'X', tier: 'office_subtree', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null },
      { id: 'c', name: 'C', code: 'C', office_type_name: 'Y', tier: 'office', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null }
    ] })
    const rows = await useOfficeMap().list()
    expect(rows.map(r => r.tier)).toEqual(['office', 'office', 'office'])
  })

  it('keeps pusat and wilayah tiers', async () => {
    request.mockResolvedValueOnce({ data: [
      { id: 'a', name: 'A', code: 'A', office_type_name: 'X', tier: 'pusat', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null },
      { id: 'b', name: 'B', code: 'B', office_type_name: 'Y', tier: 'wilayah', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null }
    ] })
    const rows = await useOfficeMap().list()
    expect(rows.map(r => r.tier)).toEqual(['pusat', 'wilayah'])
  })
})
