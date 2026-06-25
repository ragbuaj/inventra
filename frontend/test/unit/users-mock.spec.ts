import { describe, it, expect, beforeEach } from 'vitest'
import { useUsers } from '~/composables/api/useUsers'
import { userStore, userSeed, ROLES, userRoleColor } from '~/mock/users'

const { list, create, update, remove, setStatus } = useUsers()

// The mock store is a shared singleton; restore the seed before each test.
beforeEach(() => {
  for (const r of userStore.all().slice()) userStore.remove(r.id)
  for (const r of userSeed) userStore.insert({ ...r })
})

describe('mock/users seed', () => {
  it('seeds the 12 mockup users', () => {
    expect(userSeed).toHaveLength(12)
  })

  it('maps each role to a badge color', () => {
    expect(userRoleColor('Superadmin')).toBe('primary')
    expect(userRoleColor('Kepala Kanwil')).toBe('info')
    expect(userRoleColor('Asset Manager')).toBe('warning')
    expect(userRoleColor('Staf')).toBe('neutral')
    expect(userRoleColor('Unknown Role')).toBe('neutral')
    expect(ROLES).toContain('Superadmin')
  })
})

describe('useUsers.list', () => {
  it('returns all users by default', async () => {
    const res = await list({ limit: 100 })
    expect(res.total).toBe(12)
  })

  it('filters by name (case-insensitive)', async () => {
    const res = await list({ search: 'siti', limit: 100 })
    expect(res.data.every(u => /siti/i.test(u.nama) || /siti/i.test(u.email))).toBe(true)
    expect(res.data.length).toBeGreaterThan(0)
  })

  it('filters by email fragment', async () => {
    const res = await list({ search: 'bambang.s@', limit: 100 })
    expect(res.data).toHaveLength(1)
    expect(res.data[0].nama).toBe('Bambang Sukasno')
  })

  it('paginates with limit/offset', async () => {
    const res = await list({ limit: 10, offset: 0 })
    expect(res.data).toHaveLength(10)
    expect(res.total).toBe(12)
  })
})

describe('useUsers.create', () => {
  it('prepends a new user and never persists the password', async () => {
    const created = await create({
      nama: 'Test User', email: 'test.user@inventra.go.id', password: 'secret123',
      peran: 'Staf', kantor: 'Kantor Pusat', pegawai: '', login: 'email', status: 'active'
    })
    expect(created.id).toBeTruthy()
    expect('password' in created).toBe(false)
    expect(userStore.all()[0].email).toBe('test.user@inventra.go.id')
    expect(userStore.all()).toHaveLength(13)
  })
})

describe('useUsers.update', () => {
  it('patches an existing user', async () => {
    const updated = await update('admin@inventra.go.id', {
      nama: 'Super Admin Renamed', email: 'admin@inventra.go.id', peran: 'Superadmin',
      kantor: 'Kantor Pusat', pegawai: '', login: 'email', status: 'active'
    })
    expect(updated.nama).toBe('Super Admin Renamed')
  })

  it('throws the sentinel error for a missing user', async () => {
    await expect(update('nope@x.com', {
      nama: 'x', email: 'nope@x.com', peran: 'Staf', kantor: '', pegawai: '', login: 'email', status: 'active'
    })).rejects.toThrow('settings.users.errNotFound')
  })
})

describe('useUsers.setStatus', () => {
  it('flips a user status', async () => {
    const row = await setStatus('admin@inventra.go.id', 'inactive')
    expect(row.status).toBe('inactive')
  })
})

describe('useUsers.remove', () => {
  it('deletes a user', async () => {
    await remove('admin@inventra.go.id')
    expect(userStore.find('admin@inventra.go.id')).toBeUndefined()
    expect(userStore.all()).toHaveLength(11)
  })
})
