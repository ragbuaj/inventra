import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useRbac, slugifyRoleCode } from '~/composables/api/useRbac'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

beforeEach(() => request.mockReset())

describe('slugifyRoleCode', () => {
  it('lowercases and underscores non-alphanumerics', () => {
    expect(slugifyRoleCode('Auditor Cabang')).toBe('auditor_cabang')
    expect(slugifyRoleCode('  Kepala  Unit!! ')).toBe('kepala_unit')
    expect(slugifyRoleCode('Tim A/B')).toBe('tim_a_b')
  })
})

describe('useRbac', () => {
  it('getCatalog maps groups to modules with icon + perms', async () => {
    request.mockResolvedValueOnce({
      permissions: [{ group: 'Aset', items: [{ key: 'asset.view', label: 'Lihat aset' }] }],
      scope_levels: [], scope_modules: []
    })
    const mods = await useRbac().getCatalog()
    expect(request).toHaveBeenCalledWith('/authz/catalog')
    expect(mods[0]).toMatchObject({ key: 'Aset', icon: 'i-lucide-box' })
    expect(mods[0].perms[0]).toEqual({ code: 'asset.view', label: 'Lihat aset' })
  })

  it('listRoles returns data array', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'u1', code: 'manager', name: 'Manager', is_system: true }], total: 1 })
    const roles = await useRbac().listRoles()
    expect(request).toHaveBeenCalledWith('/authz/roles')
    expect(roles).toHaveLength(1)
    expect(roles[0]).toMatchObject({ id: 'u1', code: 'manager', is_system: true })
  })

  it('getRolePermissions unwraps permissions', async () => {
    request.mockResolvedValueOnce({ permissions: ['asset.view', 'asset.manage'] })
    expect(await useRbac().getRolePermissions('u1')).toEqual(['asset.view', 'asset.manage'])
    expect(request).toHaveBeenCalledWith('/authz/roles/u1/permissions')
  })

  it('updateRolePermissions PUTs the permission set', async () => {
    request.mockResolvedValueOnce({ permissions: ['asset.view'] })
    await useRbac().updateRolePermissions('u1', ['asset.view'])
    expect(request).toHaveBeenCalledWith('/authz/roles/u1/permissions', {
      method: 'PUT', body: { permissions: ['asset.view'] }
    })
  })

  it('createRole derives code, posts, and copies perms when copyFromId set', async () => {
    request
      .mockResolvedValueOnce({ permissions: ['asset.view', 'asset.manage'] }) // get source perms
      .mockResolvedValueOnce({ id: 'new1', code: 'auditor', name: 'Auditor', is_system: false }) // post
      .mockResolvedValueOnce({ permissions: ['asset.view', 'asset.manage'] }) // put new perms
    const role = await useRbac().createRole({ name: 'Auditor', copyFromId: 'src1' })
    expect(request).toHaveBeenNthCalledWith(1, '/authz/roles/src1/permissions')
    expect(request).toHaveBeenNthCalledWith(2, '/authz/roles', { method: 'POST', body: { code: 'auditor', name: 'Auditor', description: undefined } })
    expect(request).toHaveBeenNthCalledWith(3, '/authz/roles/new1/permissions', { method: 'PUT', body: { permissions: ['asset.view', 'asset.manage'] } })
    expect(role.id).toBe('new1')
  })

  it('createRole without copyFromId only posts', async () => {
    request.mockResolvedValueOnce({ id: 'new2', code: 'gudang', name: 'Gudang', is_system: false })
    const role = await useRbac().createRole({ name: 'Gudang' })
    expect(request).toHaveBeenCalledTimes(1)
    expect(role.code).toBe('gudang')
  })
})
