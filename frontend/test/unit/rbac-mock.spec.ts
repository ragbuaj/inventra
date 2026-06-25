import { describe, it, expect } from 'vitest'
import { useRbac } from '~/composables/api/useRbac'
import { RBAC_MODULES, ALL_PERMISSION_CODES, roleSeed, roleStore } from '~/mock/rbac'

const { getModules, listRoles, createRole, updateRolePermissions } = useRbac()

describe('mock/rbac catalog', () => {
  it('defines 8 modules', () => {
    expect(RBAC_MODULES).toHaveLength(8)
  })

  it('ALL_PERMISSION_CODES equals the sum of module permissions and is unique', () => {
    const sum = RBAC_MODULES.reduce((n, m) => n + m.perms.length, 0)
    expect(ALL_PERMISSION_CODES).toHaveLength(sum)
    expect(new Set(ALL_PERMISSION_CODES).size).toBe(sum)
  })

  it('seeds 7 roles: 5 system + 2 custom', () => {
    expect(roleSeed).toHaveLength(7)
    expect(roleSeed.filter(r => r.system)).toHaveLength(5)
    expect(roleSeed.filter(r => !r.system)).toHaveLength(2)
  })

  it('grants every permission to the Superadmin role', () => {
    const superadmin = roleSeed.find(r => r.key === 'superadmin')!
    expect([...superadmin.perms].sort()).toEqual([...ALL_PERMISSION_CODES].sort())
  })
})

describe('useRbac.getModules', () => {
  it('resolves module + permission labels for the locale', () => {
    const id = getModules('id')
    const en = getModules('en')
    expect(id[0].label).toBe('Aset')
    expect(en[0].label).toBe('Assets')
    expect(id[0].perms[0]).toMatchObject({ code: 'aset.view', label: 'Lihat aset' })
    expect(en[0].perms[0].label).toBe('View assets')
  })
})

describe('useRbac.listRoles', () => {
  it('resolves role names for the locale', async () => {
    const roles = await listRoles('en')
    const kakanwil = roles.find(r => r.key === 'kakanwil')!
    expect(kakanwil.nama).toBe('Regional Head')
    expect(kakanwil.system).toBe(true)
  })
})

describe('useRbac.createRole', () => {
  it('creates an empty custom role when no source is given', async () => {
    const role = await createRole({ nama: 'Field Operator' }, 'id')
    expect(role.system).toBe(false)
    expect(role.nama).toBe('Field Operator')
    expect(role.perms).toEqual([])
    expect(roleStore.find(role.key)).toBeDefined()
  })

  it('copies permissions from the source role', async () => {
    const role = await createRole({ nama: 'Copy of Staf', copyFromKey: 'staf' }, 'id')
    const staf = roleStore.find('staf')!
    expect([...role.perms].sort()).toEqual([...staf.perms].sort())
  })

  it('uses a localized default description when none is provided', async () => {
    const idRole = await createRole({ nama: 'Tanpa Deskripsi' }, 'id')
    const enRole = await createRole({ nama: 'No Description' }, 'en')
    expect(idRole.desc).toBe('Peran kustom.')
    expect(enRole.desc).toBe('Custom role.')
  })
})

describe('useRbac.updateRolePermissions', () => {
  it('persists the new permission set to the store', async () => {
    await updateRolePermissions('auditor', ['aset.view'])
    expect(roleStore.find('auditor')!.perms).toEqual(['aset.view'])
  })
})
