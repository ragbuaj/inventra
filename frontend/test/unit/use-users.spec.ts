import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useUsers } from '~/composables/api/useUsers'

beforeEach(() => request.mockReset())

describe('useUsers', () => {
  it('list builds the query (omits empty search) and returns {rows,total}', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'u1', name: 'A', email: 'a@x.id', role_id: 'r1', office_id: null, employee_id: null, status: 'active', has_avatar: false, google_linked: false, created_at: null, updated_at: null }], total: 1 })
    const res = await useUsers().list({ search: '', limit: 20, offset: 40 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/users?')
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=40')
    expect(path).not.toContain('search=')
    expect(res).toEqual({ rows: expect.any(Array), total: 1 })
    expect(res.rows[0].id).toBe('u1')
  })

  it('create sends only non-empty optional fields', async () => {
    request.mockResolvedValueOnce({ id: 'n1' })
    await useUsers().create({ name: 'New', email: 'n@x.id', role_id: 'r1' })
    expect(request).toHaveBeenCalledWith('/users', { method: 'POST', body: { name: 'New', email: 'n@x.id', role_id: 'r1' } })
  })

  it('create includes password/office_id/employee_id when present', async () => {
    request.mockResolvedValueOnce({ id: 'n2' })
    await useUsers().create({ name: 'New', email: 'n@x.id', role_id: 'r1', password: 'pw', office_id: 'o1', employee_id: 'e1' })
    expect(request).toHaveBeenCalledWith('/users', { method: 'POST', body: { name: 'New', email: 'n@x.id', role_id: 'r1', password: 'pw', office_id: 'o1', employee_id: 'e1' } })
  })

  it('update PUTs name/role_id/status (+ optional office/employee)', async () => {
    request.mockResolvedValueOnce({ id: 'u1' })
    await useUsers().update('u1', { name: 'A', role_id: 'r1', status: 'inactive', office_id: 'o1' })
    expect(request).toHaveBeenCalledWith('/users/u1', { method: 'PUT', body: { name: 'A', role_id: 'r1', status: 'inactive', office_id: 'o1' } })
  })

  it('remove DELETEs', async () => {
    request.mockResolvedValueOnce(undefined)
    await useUsers().remove('u1')
    expect(request).toHaveBeenCalledWith('/users/u1', { method: 'DELETE' })
  })

  it('lookups maps roles only — office/employee names now resolve via the async picker adapters, not an eager list', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }] }) // /authz/roles
    const lk = await useUsers().lookups()
    expect(request.mock.calls.map(c => (c[0] as string).split('?')[0])).toEqual(['/authz/roles'])
    expect(lk.roles).toEqual([{ id: 'r1', name: 'Manager' }])
  })
})
