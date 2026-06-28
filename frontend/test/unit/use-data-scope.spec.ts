import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useDataScope } from '~/composables/api/useDataScope'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

beforeEach(() => request.mockReset())

describe('useDataScope', () => {
  it('getModules drops the "*" sentinel', async () => {
    request.mockResolvedValueOnce({ scope_modules: ['*', 'offices', 'assets'] })
    const mods = await useDataScope().getModules()
    expect(request).toHaveBeenCalledWith('/authz/catalog')
    expect(mods).toEqual([{ key: 'offices' }, { key: 'assets' }])
  })

  it('listRoles maps policies to def + ov', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', code: 'manager', name: 'Manager', description: 'Ops' }], total: 1 })
      .mockResolvedValueOnce({ policies: [{ module: '*', scope_level: 'office' }, { module: 'assets', scope_level: 'office_subtree' }] })
    const roles = await useDataScope().listRoles()
    expect(request).toHaveBeenNthCalledWith(1, '/authz/roles')
    expect(request).toHaveBeenNthCalledWith(2, '/authz/roles/r1/scope')
    expect(roles[0]).toEqual({ id: 'r1', code: 'manager', name: 'Manager', sub: 'Ops', def: 'office', ov: { assets: 'office_subtree' } })
  })

  it('listRoles falls back to own when no "*" policy', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r2', code: 'staf', name: 'Staf' }], total: 1 })
      .mockResolvedValueOnce({ policies: [] })
    const roles = await useDataScope().listRoles()
    expect(roles[0].def).toBe('own')
    expect(roles[0].ov).toEqual({})
    expect(roles[0].sub).toBe('')
  })

  it('saveRoleScope always includes the "*" default plus overrides', async () => {
    request.mockResolvedValueOnce({ policies: [] })
    await useDataScope().saveRoleScope('r1', 'office', { assets: 'office_subtree' })
    expect(request).toHaveBeenCalledWith('/authz/roles/r1/scope', {
      method: 'PUT',
      body: { policies: [{ module: '*', scope_level: 'office' }, { module: 'assets', scope_level: 'office_subtree' }] }
    })
  })
})
