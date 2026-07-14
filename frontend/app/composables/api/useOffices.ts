import type { ListQuery, Office, Paginated } from '~/types'

export interface OfficeInput {
  parent_id: string | null
  office_type_id: string
  province_id: string | null
  city_id: string | null
  name: string
  code: string
  address?: string | null
  is_active: boolean
  latitude?: number | null
  longitude?: number | null
}

/** Offices, wired to /api/v1/offices (server-enforced `offices` data-scope). */
export function useOffices() {
  const { request } = useApiClient()

  async function list(query: ListQuery = {}): Promise<Paginated<Office>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 10))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    return request<Paginated<Office>>(`/offices?${q.toString()}`)
  }

  async function get(id: string): Promise<Office> {
    return request<Office>(`/offices/${id}`)
  }

  async function tree(): Promise<Office[]> {
    const res = await request<{ data: Office[], total: number }>('/offices/tree')
    return res.data
  }

  // parent_id is sent as-is (null → head office); the backend validates it as an
  // optional UUID. Empty optional FKs / coordinates are omitted so they stay null.
  function toBody(input: OfficeInput): Record<string, unknown> {
    const body: Record<string, unknown> = {
      parent_id: input.parent_id,
      office_type_id: input.office_type_id,
      name: input.name,
      code: input.code,
      is_active: input.is_active
    }
    if (input.province_id) body.province_id = input.province_id
    if (input.city_id) body.city_id = input.city_id
    if (input.address) body.address = input.address
    if (input.latitude != null) body.latitude = input.latitude
    if (input.longitude != null) body.longitude = input.longitude
    return body
  }

  async function create(input: OfficeInput): Promise<Office> {
    return request<Office>('/offices', { method: 'POST', body: toBody(input) })
  }

  async function update(id: string, input: OfficeInput): Promise<Office> {
    return request<Office>(`/offices/${id}`, { method: 'PUT', body: toBody(input) })
  }

  async function remove(id: string): Promise<void> {
    await request(`/offices/${id}`, { method: 'DELETE' })
  }

  return { list, get, tree, create, update, remove }
}
