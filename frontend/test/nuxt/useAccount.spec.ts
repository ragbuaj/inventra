// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { useAccount } from '~/composables/api/useAccount'
import { useAuthStore } from '~/stores/auth'

describe('useAccount', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager' }, ['*'])
  })

  it('builds a profile from the auth user merged with mock fields', async () => {
    const p = await useAccount().getProfile()
    expect(p.nama).toBe('Andi Saputra')
    expect(p.email).toBe('andi@inventra.local')
    expect(p.peran).toBe('Asset Manager')
    expect(p.loginMethod).toBe('email')
  })

  it('rejects a password change with mismatched confirmation', async () => {
    await expect(useAccount().changePassword({ oldPass: 'x', newPass: 'Abcdefg1!', confirmPass: 'nope' }))
      .rejects.toThrow('account.errConfirmMismatch')
  })

  it('rejects a password change with a blank field', async () => {
    await expect(useAccount().changePassword({ oldPass: '', newPass: 'Abcdefg1!', confirmPass: 'Abcdefg1!' }))
      .rejects.toThrow('account.errRequired')
  })

  it('lists sessions with exactly one current session', async () => {
    const s = await useAccount().listSessions()
    expect(s.length).toBeGreaterThanOrEqual(1)
    expect(s.filter(x => x.current)).toHaveLength(1)
  })

  it('persists notification preferences', () => {
    const a = useAccount()
    a.setNotifPrefs({ approval: false, maint: true, assign: true })
    expect(a.getNotifPrefs()).toEqual({ approval: false, maint: true, assign: true })
  })

  it('rejects updateProfile with an empty name', async () => {
    await expect(useAccount().updateProfile({ nama: '', telepon: '0812' })).rejects.toThrow('account.errRequired')
  })

  it('resolves revokeSession and logoutAllOthers without throwing', async () => {
    const a = useAccount()
    await expect(a.revokeSession('s2')).resolves.toBeUndefined()
    await expect(a.logoutAllOthers()).resolves.toBeUndefined()
  })
})
