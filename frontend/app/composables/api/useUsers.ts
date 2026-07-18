export type UserStatus = 'active' | 'inactive' | 'suspended'

export interface UserView {
  id: string
  name: string
  email: string
  role_id: string
  office_id: string | null
  employee_id: string | null
  status: UserStatus
  has_avatar: boolean
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
export interface Lookups { roles: Option[] }

interface RoleDTO { id: string, name: string }

/**
 * User management, wired to /api/v1/users. List is server-side search+filter+
 * pagination (search, role_id, office_id, status, limit, offset). Role names
 * are resolved client-side from lookups() (the list returns FK UUIDs only) —
 * office/employee names resolve on demand via the office/employee picker
 * adapters (usePickerSource.ts) instead of an eager `{ limit: 100 }` list.
 */
export function useUsers() {
  const { request } = useApiClient()

  async function list(params: { search?: string, roleId?: string, officeId?: string, status?: UserStatus, limit: number, offset: number }): Promise<{ rows: UserView[], total: number }> {
    const q = new URLSearchParams()
    q.set('limit', String(params.limit))
    q.set('offset', String(params.offset))
    if (params.search) q.set('search', params.search)
    if (params.roleId) q.set('role_id', params.roleId)
    if (params.officeId) q.set('office_id', params.officeId)
    if (params.status) q.set('status', params.status)
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

  // Admin-initiated password reset: the backend generates a single-use token
  // and emails the reset link to the user. Resolves with the notified address;
  // a 422 (Google-only account, no password login) propagates to the caller.
  async function resetPassword(id: string): Promise<{ status: string, email: string }> {
    // suppressErrorToast: the page handles its own toasts (a Google-only 422 is
    // a warning, not a generic error), so skip useApiClient's default error toast.
    return request<{ status: string, email: string }>(`/users/${id}/reset-password`, { method: 'POST', suppressErrorToast: true })
  }

  async function lookups(): Promise<Lookups> {
    const roles = await request<{ data: RoleDTO[] }>('/authz/roles')
    return { roles: roles.data.map(r => ({ id: r.id, name: r.name })) }
  }

  return { list, create, update, remove, resetPassword, lookups }
}
