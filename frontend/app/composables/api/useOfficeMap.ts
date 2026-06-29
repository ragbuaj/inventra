import type { MapOffice, OfficeTier } from '~/types'

interface MapOfficeDTO {
  id: string
  name: string
  code: string
  office_type_name: string | null
  tier: string | null
  province_name: string | null
  city_name: string | null
  address: string | null
  asset_count: number
  latitude: number | null
  longitude: number | null
}

function toTier(raw: string | null): OfficeTier {
  return raw === 'pusat' || raw === 'wilayah' ? raw : 'office'
}

export function useOfficeMap() {
  const { request } = useApiClient()

  async function list(): Promise<MapOffice[]> {
    const res = await request<{ data: MapOfficeDTO[] }>('/offices/map')
    return res.data.map(o => ({
      id: o.id,
      name: o.name,
      code: o.code,
      office_type_name: o.office_type_name,
      tier: toTier(o.tier),
      province_name: o.province_name,
      city_name: o.city_name,
      address: o.address,
      asset_count: o.asset_count,
      latitude: o.latitude,
      longitude: o.longitude
    }))
  }

  return { list }
}
