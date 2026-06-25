import { describe, it, expect, beforeEach } from 'vitest'
import { useDataScope } from '~/composables/api/useDataScope'
import { DATA_SCOPE_MODULES, SCOPE_LEVEL_KEYS, dataScopeStore } from '~/mock/dataScope'

const { getModules, listRoles, saveScopes } = useDataScope()

beforeEach(() => dataScopeStore.reset())

describe('mock/dataScope', () => {
  it('defines 4 scope levels and 5 modules', () => {
    expect(SCOPE_LEVEL_KEYS).toEqual(['global', 'office_subtree', 'office', 'own'])
    expect(DATA_SCOPE_MODULES).toHaveLength(5)
  })

  it('seeds 6 roles; Manager carries per-module overrides', () => {
    const roles = dataScopeStore.all()
    expect(roles).toHaveLength(6)
    const manager = roles.find(r => r.key === 'manager')!
    expect(manager.def).toBe('office')
    expect(manager.ov).toEqual({ aset: 'office_subtree', pengajuan: 'own' })
  })
})

describe('useDataScope', () => {
  it('resolves module + role labels for the locale', async () => {
    expect(getModules('en')[0].label).toBe('Assets')
    expect(getModules('id')[0].label).toBe('Aset')
    const roles = await listRoles('en')
    expect(roles.find(r => r.key === 'kakanwil')!.nama).toBe('Regional Head')
  })

  it('persists a saved matrix back to the store', async () => {
    const roles = await listRoles('id')
    const superadmin = roles.find(r => r.key === 'superadmin')!
    superadmin.def = 'own'
    superadmin.ov = { aset: 'office' }
    await saveScopes(roles)
    const stored = dataScopeStore.all().find(r => r.key === 'superadmin')!
    expect(stored.def).toBe('own')
    expect(stored.ov).toEqual({ aset: 'office' })
  })
})
