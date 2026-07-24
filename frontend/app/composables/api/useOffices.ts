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
  // Legacy-parity Fase 5 fields.
  ownership_status?: string | null
  office_class_id?: string | null
  building_classification_id?: string | null
  floor_count?: number | null
  building_area?: string | null
  office_kind?: string | null
  description?: string | null
  head_employee_id?: string | null
  contact?: string | null
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
    // Legacy-parity Fase 5 fields. Empty optionals are omitted so they stay null;
    // floor_count uses a null check because 0 is a valid value.
    if (input.ownership_status) body.ownership_status = input.ownership_status
    if (input.office_class_id) body.office_class_id = input.office_class_id
    if (input.building_classification_id) body.building_classification_id = input.building_classification_id
    if (input.floor_count != null) body.floor_count = input.floor_count
    if (input.building_area) body.building_area = input.building_area
    if (input.office_kind) body.office_kind = input.office_kind
    if (input.description) body.description = input.description
    if (input.head_employee_id) body.head_employee_id = input.head_employee_id
    if (input.contact) body.contact = input.contact
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
