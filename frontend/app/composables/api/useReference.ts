import type { ListQuery, Paginated, ReferenceRow } from '~/types'
import type { ReferenceKey } from './referenceResources'

/**
 * Reference master data, wired to the generic engine at /api/v1/<key>.
 * The descriptor key is the backend path. List is server-side search+pagination.
 */
export function useReference() {
  const { request } = useApiClient()

  async function list(key: ReferenceKey, query: ListQuery = {}): Promise<Paginated<ReferenceRow>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 20))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    return request<Paginated<ReferenceRow>>(`/${key}?${q.toString()}`)
  }

  async function create(key: ReferenceKey, input: Record<string, unknown>): Promise<ReferenceRow> {
    return request<ReferenceRow>(`/${key}`, { method: 'POST', body: input })
  }

  async function update(key: ReferenceKey, id: string, input: Record<string, unknown>): Promise<ReferenceRow> {
    return request<ReferenceRow>(`/${key}/${id}`, { method: 'PUT', body: input })
  }

  async function remove(key: ReferenceKey, id: string): Promise<void> {
    await request(`/${key}/${id}`, { method: 'DELETE' })
  }

  async function get(key: ReferenceKey, id: string): Promise<ReferenceRow> {
    return request<ReferenceRow>(`/${key}/${id}`)
  }

  return { list, get, create, update, remove }
}
