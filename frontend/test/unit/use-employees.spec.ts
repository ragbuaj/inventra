import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useEmployees } from '~/composables/api/useEmployees'

beforeEach(() => request.mockReset())

describe('useEmployees', () => {
  it('list builds the query (omits empty search) and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'e1' }], total: 1, limit: 20, offset: 0 })
    const res = await useEmployees().list({ limit: 20, offset: 0 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/employees?')
    expect(path).toContain('limit=20')
    expect(path).not.toContain('search=')
    expect(res.total).toBe(1)
  })

  it('create POSTs /employees with UUID FKs + phone, omitting empty optionals', async () => {
    request.mockResolvedValueOnce({ id: 'e1' })
    await useEmployees().create({ code: '199001', name: 'Andi', office_id: 'o1', status: 'active', department_id: 'd1', position_id: 'p1', email: 'a@x.id', phone: '0812' })
    expect(request).toHaveBeenCalledWith('/employees', { method: 'POST', body: { code: '199001', name: 'Andi', office_id: 'o1', status: 'active', email: 'a@x.id', phone: '0812', department_id: 'd1', position_id: 'p1' } })
  })

  it('create omits empty email/phone/department_id/position_id', async () => {
    request.mockResolvedValueOnce({ id: 'e2' })
    await useEmployees().create({ code: 'X', name: 'B', office_id: 'o1', status: 'active' })
    expect(request).toHaveBeenCalledWith('/employees', { method: 'POST', body: { code: 'X', name: 'B', office_id: 'o1', status: 'active' } })
  })

  it('update PUTs /employees/:id', async () => {
    request.mockResolvedValueOnce({ id: 'e1' })
    await useEmployees().update('e1', { code: 'X', name: 'B', office_id: 'o1', status: 'inactive' })
    expect(request).toHaveBeenCalledWith('/employees/e1', { method: 'PUT', body: { code: 'X', name: 'B', office_id: 'o1', status: 'inactive' } })
  })

  it('get GETs /employees/:id; remove DELETEs', async () => {
    request.mockResolvedValueOnce({ id: 'e1' })
    await useEmployees().get('e1')
    expect(request).toHaveBeenCalledWith('/employees/e1')
    request.mockResolvedValueOnce(undefined)
    await useEmployees().remove('e1')
    expect(request).toHaveBeenCalledWith('/employees/e1', { method: 'DELETE' })
  })
})
