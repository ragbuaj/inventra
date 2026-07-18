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

  return { list, get, getByTag, update }
}
