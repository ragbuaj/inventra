export type UserStatus = 'active' | 'inactive' | 'suspended'

export interface UserView {
  id: string
  name: string
  email: string
  role_id: string
  office_id: string | null
  employee_id: string | null
  status: UserStatus
  avatar_url: string | null
  google_linked: boolean
  created_at: string | null
  updated_at: string | null
}

export interface CreateUserInput {
  name: string
  email: string
  password?: string
  role_id: string
  office_id?: string
  employee_id?: string
}

export interface UpdateUserInput {
  name: string
  role_id: string
  status: UserStatus
  office_id?: string
  employee_id?: string
}

export interface Option { id: string, name: string }
export interface EmployeeOption extends Option { office_id: string }
export interface Lookups { roles: Option[], offices: Option[], employees: EmployeeOption[] }

interface RoleDTO { id: string, name: string }
interface OfficeDTO { id: string, name: string }
interface EmployeeDTO { id: string, name: string, office_id: string }

/**
 * User management, wired to /api/v1/users. List is server-side search+pagination
 * (the backend supports only search/limit/offset). Role/office/employee NAMES are
 * resolved client-side from lookups() (the list returns FK UUIDs only).
 */
export function useUsers() {
  const { request } = useApiClient()

  async function list(params: { search?: string, limit: number, offset: number }): Promise<{ rows: UserView[], total: number }> {
    const q = new URLSearchParams()
    q.set('limit', String(params.limit))
    q.set('offset', String(params.offset))
    if (params.search) q.set('search', params.search)
    const res = await request<{ data: UserView[], total: number }>(`/users?${q.toString()}`)
    return { rows: res.data, total: res.total }
  }

  async function create(input: CreateUserInput): Promise<UserView> {
    const body: Record<string, unknown> = { name: input.name, email: input.email, role_id: input.role_id }
    if (input.password) body.password = input.password
    if (input.office_id) body.office_id = input.office_id
    if (input.employee_id) body.employee_id = input.employee_id
    return request<UserView>('/users', { method: 'POST', body })
  }

  async function update(id: string, input: UpdateUserInput): Promise<UserView> {
    const body: Record<string, unknown> = { name: input.name, role_id: input.role_id, status: input.status }
    if (input.office_id) body.office_id = input.office_id
    if (input.employee_id) body.employee_id = input.employee_id
    return request<UserView>(`/users/${id}`, { method: 'PUT', body })
  }

  async function remove(id: string): Promise<void> {
    await request(`/users/${id}`, { method: 'DELETE' })
  }

  async function lookups(): Promise<Lookups> {
    const [roles, offices, employees] = await Promise.all([
      request<{ data: RoleDTO[] }>('/authz/roles'),
      request<{ data: OfficeDTO[] }>('/offices?limit=100'),
      request<{ data: EmployeeDTO[] }>('/employees?limit=100')
    ])
    return {
      roles: roles.data.map(r => ({ id: r.id, name: r.name })),
      offices: offices.data.map(o => ({ id: o.id, name: o.name })),
      employees: employees.data.map(e => ({ id: e.id, name: e.name, office_id: e.office_id }))
    }
  }

  return { list, create, update, remove, lookups }
}
