import type { Employee, EmployeeStatus, ListQuery, Paginated } from '~/types'

export interface EmployeeInput {
  code: string
  name: string
  email?: string
  phone?: string
  department_id?: string
  position_id?: string
  office_id: string
  status: EmployeeStatus
}

/** Employees, wired to /api/v1/employees (server-enforced `employees` data-scope). */
export function useEmployees() {
  const { request } = useApiClient()

  async function list(query: ListQuery = {}): Promise<Paginated<Employee>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 10))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    return request<Paginated<Employee>>(`/employees?${q.toString()}`)
  }

  async function get(id: string): Promise<Employee> {
    return request<Employee>(`/employees/${id}`)
  }

  function toBody(input: EmployeeInput): Record<string, unknown> {
    const body: Record<string, unknown> = { code: input.code, name: input.name, office_id: input.office_id, status: input.status }
    if (input.email) body.email = input.email
    if (input.phone) body.phone = input.phone
    if (input.department_id) body.department_id = input.department_id
    if (input.position_id) body.position_id = input.position_id
    return body
  }

  async function create(input: EmployeeInput): Promise<Employee> {
    return request<Employee>('/employees', { method: 'POST', body: toBody(input) })
  }

  async function update(id: string, input: EmployeeInput): Promise<Employee> {
    return request<Employee>(`/employees/${id}`, { method: 'PUT', body: toBody(input) })
  }

  async function remove(id: string): Promise<void> {
    await request(`/employees/${id}`, { method: 'DELETE' })
  }

  return { list, get, create, update, remove }
}
