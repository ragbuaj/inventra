import type { Asset, AssetClass, AssetStatus, AssetUpdateInput, Paginated } from '~/types'

export interface AssetListQuery {
  limit?: number
  offset?: number
  search?: string
  status?: AssetStatus
  category_id?: string
  office_id?: string
  asset_class?: AssetClass
}

/** One asset location-change record (spec legacy-parity Fase 3). */
export interface AssetLocationHistory {
  id: string
  office_id: string
  office_name: string
  floor_id?: string | null
  floor_name?: string | null
  room_id?: string | null
  room_name?: string | null
  source: 'registration' | 'edit' | 'transfer' | 'migration'
  moved_at?: string | null
  moved_by_id?: string | null
  moved_by_name?: string | null
  transfer_id?: string | null
  note?: string | null
}

/** One asset PIC (person-in-charge) record (spec legacy-parity Fase 3). */
export interface AssetPICHistory {
  id: string
  pic_employee_id: string
  pic_name: string
  pic_code: string
  assigned_at?: string | null
  released_at?: string | null
  assigned_by_id?: string | null
  assigned_by_name?: string | null
  note?: string | null
}

/** Assets, wired to /api/v1/assets (server-enforced `assets` data-scope). */
export function useAssets() {
  const { request } = useApiClient()

  async function list(query: AssetListQuery = {}): Promise<Paginated<Asset>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 10))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    if (query.status) q.set('status', query.status)
    if (query.category_id) q.set('category_id', query.category_id)
    if (query.office_id) q.set('office_id', query.office_id)
    if (query.asset_class) q.set('asset_class', query.asset_class)
    return request<Paginated<Asset>>(`/assets?${q.toString()}`)
  }

  async function get(id: string): Promise<Asset> {
    return request<Asset>(`/assets/${id}`)
  }

  async function getByTag(tag: string, opts?: { suppressErrorToast?: boolean }): Promise<Asset> {
    const path = `/assets/by-tag/${encodeURIComponent(tag)}`
    // Forward opts only when given so existing callers keep the exact
    // single-argument request(path) call shape.
    return opts ? request<Asset>(path, opts) : request<Asset>(path)
  }

  async function update(id: string, input: AssetUpdateInput): Promise<Asset> {
    return request<Asset>(`/assets/${id}`, { method: 'PUT', body: input })
  }

  async function locationHistory(id: string): Promise<AssetLocationHistory[]> {
    const res = await request<{ data: AssetLocationHistory[] }>(`/assets/${id}/location-history`)
    return res.data ?? []
  }

  async function picHistory(id: string): Promise<AssetPICHistory[]> {
    const res = await request<{ data: AssetPICHistory[] }>(`/assets/${id}/pic-history`)
    return res.data ?? []
  }

  return { list, get, getByTag, update, locationHistory, picHistory }
}
