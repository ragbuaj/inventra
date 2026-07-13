// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'

// ---------------------------------------------------------------------------
// Mock the underlying HTTP client (same idiom as use-dashboard.spec.ts) so
// getProfile/updateProfile/requestEmailChange/requestPasswordChange never hit
// the real backend at :8080. The public confirm-email endpoint goes through
// raw `$fetch` (same pattern as requestPasswordReset/resetPassword), so that
// global is stubbed separately below.
// ---------------------------------------------------------------------------
const requestMock = vi.fn()
vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: vi.fn(), refreshToken: vi.fn() })
}))

const fetchMock = vi.fn((_url?: string, _opts?: Record<string, unknown>) => Promise.resolve({} as unknown))
vi.stubGlobal('$fetch', fetchMock)

// eslint-disable-next-line import/first
import { useAccount } from '~/composables/api/useAccount'
// eslint-disable-next-line import/first
import { useAuthStore } from '~/stores/auth'

const PROFILE_RESPONSE = {
  id: 'u1',
  name: 'Andi Saputra',
  email: 'andi@inventra.local',
  phone: '0812-3456-7890',
  role_id: 'r1',
  office_id: 'o1',
  employee_id: 'e1',
  status: 'active',
  avatar_url: null,
  google_linked: false,
  joined_at: '2024-03-12T00:00:00Z'
}

describe('useAccount', () => {
  beforeEach(() => {
    localStorage.clear()
    requestMock.mockReset()
    fetchMock.mockClear()
    useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager', office_id: null }, ['*'])
  })

  describe('getProfile', () => {
    it('GETs /auth/profile and maps snake_case to AccountProfile', async () => {
      requestMock.mockResolvedValueOnce(PROFILE_RESPONSE)
      const p = await useAccount().getProfile()
      expect(requestMock).toHaveBeenCalledWith('/auth/profile')
      expect(p.nama).toBe('Andi Saputra')
      expect(p.email).toBe('andi@inventra.local')
      expect(p.telepon).toBe('0812-3456-7890')
      expect(p.loginMethod).toBe('email')
      expect(p.joinDate).toBe('2024-03-12T00:00:00Z')
      expect(p.hasEmployee).toBe(true)
    })

    it('maps a null phone to an empty string', async () => {
      requestMock.mockResolvedValueOnce({ ...PROFILE_RESPONSE, phone: null })
      const p = await useAccount().getProfile()
      expect(p.telepon).toBe('')
    })

    it('sets hasEmployee false and loginMethod google when appropriate', async () => {
      requestMock.mockResolvedValueOnce({ ...PROFILE_RESPONSE, employee_id: null, google_linked: true })
      const p = await useAccount().getProfile()
      expect(p.hasEmployee).toBe(false)
      expect(p.loginMethod).toBe('google')
    })

    it('falls back to the auth store role_name (API has no role_name field)', async () => {
      requestMock.mockResolvedValueOnce(PROFILE_RESPONSE)
      const p = await useAccount().getProfile()
      expect(p.peran).toBe('Asset Manager')
    })

    it('propagates a backend error', async () => {
      requestMock.mockRejectedValueOnce(Object.assign(new Error('not found'), { statusCode: 404 }))
      await expect(useAccount().getProfile()).rejects.toThrow('not found')
    })
  })

  describe('updateProfile', () => {
    it('rejects an empty name without calling the API', async () => {
      await expect(useAccount().updateProfile({ nama: '', telepon: '0812' })).rejects.toThrow('account.errRequired')
      expect(requestMock).not.toHaveBeenCalled()
    })

    it('PUTs /auth/profile with { name, phone } and maps the response', async () => {
      requestMock.mockResolvedValueOnce({ ...PROFILE_RESPONSE, name: 'Andi Baru', phone: '0899' })
      const p = await useAccount().updateProfile({ nama: 'Andi Baru', telepon: '0899' })
      expect(requestMock).toHaveBeenCalledWith('/auth/profile', { method: 'PUT', body: { name: 'Andi Baru', phone: '0899' } })
      expect(p.nama).toBe('Andi Baru')
      expect(p.telepon).toBe('0899')
    })

    it('propagates a backend validation error', async () => {
      requestMock.mockRejectedValueOnce(Object.assign(new Error('invalid input'), { statusCode: 422 }))
      await expect(useAccount().updateProfile({ nama: 'X', telepon: '' })).rejects.toThrow('invalid input')
    })
  })

  describe('requestEmailChange', () => {
    it('POSTs /auth/email/change-request with new_email + current_password', async () => {
      requestMock.mockResolvedValueOnce({ status: 'ok' })
      await useAccount().requestEmailChange('new@inventra.local', 'secret123')
      expect(requestMock).toHaveBeenCalledWith('/auth/email/change-request', {
        method: 'POST',
        body: { new_email: 'new@inventra.local', current_password: 'secret123' }
      })
    })

    it('propagates a wrong-password (400) error', async () => {
      requestMock.mockRejectedValueOnce(Object.assign(new Error('password salah'), { statusCode: 400 }))
      await expect(useAccount().requestEmailChange('new@inventra.local', 'wrong')).rejects.toThrow('password salah')
    })

    it('propagates an email-in-use (409) error', async () => {
      requestMock.mockRejectedValueOnce(Object.assign(new Error('email in use'), { statusCode: 409 }))
      await expect(useAccount().requestEmailChange('taken@inventra.local', 'secret123')).rejects.toThrow('email in use')
    })
  })

  describe('confirmEmailChange', () => {
    it('POSTs to the public /auth/email/confirm endpoint via raw $fetch with the token', async () => {
      fetchMock.mockResolvedValueOnce({ status: 'email_changed' })
      await useAccount().confirmEmailChange('tok-123')
      expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/api/v1/auth/email/confirm', {
        method: 'POST',
        body: { token: 'tok-123' }
      })
    })

    it('propagates an invalid/expired token error', async () => {
      fetchMock.mockRejectedValueOnce(Object.assign(new Error('tautan tidak valid'), { statusCode: 400 }))
      await expect(useAccount().confirmEmailChange('bad')).rejects.toThrow('tautan tidak valid')
    })
  })

  describe('requestPasswordChange', () => {
    it('POSTs /auth/password/change-request with current_password', async () => {
      requestMock.mockResolvedValueOnce({ status: 'ok' })
      await useAccount().requestPasswordChange('secret123')
      expect(requestMock).toHaveBeenCalledWith('/auth/password/change-request', {
        method: 'POST',
        body: { current_password: 'secret123' }
      })
    })

    it('propagates a wrong current-password (400) error', async () => {
      requestMock.mockRejectedValueOnce(Object.assign(new Error('password lama salah'), { statusCode: 400 }))
      await expect(useAccount().requestPasswordChange('wrong')).rejects.toThrow('password lama salah')
    })
  })

  it('requestPasswordReset POSTs to the public forgot-password endpoint via raw $fetch', async () => {
    fetchMock.mockResolvedValueOnce({ status: 'ok' })
    await useAccount().requestPasswordReset('u@example.com')
    expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/api/v1/auth/password/forgot', {
      method: 'POST',
      body: { email: 'u@example.com' }
    })
  })

  it('resetPassword rejects a short new password without calling $fetch', async () => {
    await expect(useAccount().resetPassword('sometoken', 'short')).rejects.toThrow('account.errWeak')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('resetPassword POSTs to the public reset-password endpoint via raw $fetch', async () => {
    fetchMock.mockResolvedValueOnce({ status: 'ok' })
    await useAccount().resetPassword('sometoken', 'brandnewpass')
    expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/api/v1/auth/password/reset', {
      method: 'POST',
      body: { token: 'sometoken', new_password: 'brandnewpass' }
    })
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

  it('resolves revokeSession and logoutAllOthers without throwing', async () => {
    const a = useAccount()
    await expect(a.revokeSession('s2')).resolves.toBeUndefined()
    await expect(a.logoutAllOthers()).resolves.toBeUndefined()
  })
})
