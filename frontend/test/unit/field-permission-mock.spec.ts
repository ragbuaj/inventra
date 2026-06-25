import { describe, it, expect, beforeEach } from 'vitest'
import { useFieldPermission } from '~/composables/api/useFieldPermission'
import { FIELD_ENTITIES, FIELD_ROLE_KEYS, fieldPermStore } from '~/mock/fieldPermission'

const { getEntities, getRoleColumns, getRules, saveRules } = useFieldPermission()

beforeEach(() => fieldPermStore.reset())

describe('mock/fieldPermission', () => {
  it('defines 4 entities and 5 role columns', () => {
    expect(FIELD_ENTITIES).toHaveLength(4)
    expect(FIELD_ROLE_KEYS).toHaveLength(5)
    expect(FIELD_ENTITIES.find(e => e.key === 'aset')!.fields).toHaveLength(11)
  })

  it('seeds only the restricted fields with explicit rules', () => {
    expect(Object.keys(fieldPermStore.get('aset')).sort()).toEqual(['harga_beli', 'nilai_buku'])
    expect(Object.keys(fieldPermStore.get('user'))).toHaveLength(0)
    expect(fieldPermStore.get('aset').harga_beli.manager).toEqual({ view: true, edit: false })
    expect(fieldPermStore.get('aset').harga_beli.staf).toEqual({ view: false, edit: false })
  })
})

describe('useFieldPermission', () => {
  it('resolves entity + role labels for the locale', () => {
    expect(getEntities('en')[0].label).toBe('Assets')
    expect(getEntities('id')[1].label).toBe('Pegawai')
    expect(getRoleColumns('en').map(c => c.label)).toContain('Reg. Head')
  })

  it('getRules returns a detached clone', async () => {
    const a = await getRules('aset')
    a.harga_beli.super.view = false
    expect(fieldPermStore.get('aset').harga_beli.super.view).toBe(true)
  })

  it('saveRules persists for the entity', async () => {
    await saveRules('user', { status: { super: { view: true, edit: false }, kakanwil: { view: true, edit: false }, kaunit: { view: true, edit: false }, manager: { view: true, edit: false }, staf: { view: false, edit: false } } })
    expect(Object.keys(fieldPermStore.get('user'))).toEqual(['status'])
    expect(fieldPermStore.get('user').status.staf).toEqual({ view: false, edit: false })
  })
})
