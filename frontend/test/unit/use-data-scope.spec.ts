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

  it('listRoles is a single GET (no per-role fan-out)', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'r1', code: 'manager', name: 'Manager', description: 'Ops' }], total: 1 })
    const roles = await useDataScope().listRoles()
    expect(request).toHaveBeenCalledTimes(1)
    expect(request).toHaveBeenCalledWith('/authz/roles')
    expect(roles).toEqual([{ id: 'r1', code: 'manager', name: 'Manager', sub: 'Ops' }])
  })

  it('getRoleScope maps policies to def + ov', async () => {
    request.mockResolvedValueOnce({ policies: [{ module: '*', scope_level: 'office' }, { module: 'assets', scope_level: 'office_subtree' }] })
    const scope = await useDataScope().getRoleScope('r1')
    expect(request).toHaveBeenCalledWith('/authz/roles/r1/scope')
    expect(scope).toEqual({ def: 'office', ov: { assets: 'office_subtree' } })
  })

  it('getRoleScope falls back to own when no "*" policy', async () => {
    request.mockResolvedValueOnce({ policies: [] })
    const scope = await useDataScope().getRoleScope('r2')
    expect(scope).toEqual({ def: 'own', ov: {} })
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
