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
  role_name: 'Asset Manager',
  office_id: 'o1',
  office_name: 'Cabang Jakarta Selatan',
  employee_id: 'e1',
  employee_name: 'Andi Saputra',
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
      expect(p.peran).toBe('Asset Manager')
      expect(p.kantor).toBe('Cabang Jakarta Selatan')
      expect(p.pegawai).toBe('Andi Saputra')
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

    it('prefers the API role_name over the auth store', async () => {
      useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Stale Store Role', office_id: null }, ['*'])
      requestMock.mockResolvedValueOnce({ ...PROFILE_RESPONSE, role_name: 'Asset Manager' })
      const p = await useAccount().getProfile()
      expect(p.peran).toBe('Asset Manager')
    })

    it('falls back to the auth store role_name when the API role_name is empty', async () => {
      requestMock.mockResolvedValueOnce({ ...PROFILE_RESPONSE, role_name: null })
      const p = await useAccount().getProfile()
      expect(p.peran).toBe('Asset Manager') // from the seeded auth store
    })

    it('maps null office_name / employee_name to empty strings', async () => {
      requestMock.mockResolvedValueOnce({ ...PROFILE_RESPONSE, office_name: null, employee_name: null, employee_id: null })
      const p = await useAccount().getProfile()
      expect(p.kantor).toBe('')
      expect(p.pegawai).toBe('')
      expect(p.hasEmployee).toBe(false)
    })

    it('uses employee_name (not the user name) for pegawai', async () => {
      requestMock.mockResolvedValueOnce({ ...PROFILE_RESPONSE, name: 'Login User', employee_name: 'Andi Saputra' })
      const p = await useAccount().getProfile()
      expect(p.nama).toBe('Login User')
      expect(p.pegawai).toBe('Andi Saputra')
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
        body: { new_email: 'new@inventra.local', current_password: 'secret123' },
        suppressErrorToast: true
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
        body: { current_password: 'secret123' },
        suppressErrorToast: true
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

  it('GETs /auth/sessions and maps SessionView → AccountSession (icon, device, one current)', async () => {
    requestMock.mockResolvedValueOnce({
      data: [
        { id: 's1', browser: 'Chrome', os: 'macOS', device_type: 'desktop', ip_address: '1.1.1.1', location: 'Jakarta, Indonesia', created_at: '2026-07-15T10:00:00Z', last_seen_at: '2026-07-15T12:00:00Z', current: true },
        { id: 's2', browser: 'Safari', os: 'iOS', device_type: 'mobile', ip_address: '2.2.2.2', location: '', created_at: '2026-07-14T10:00:00Z', last_seen_at: '2026-07-15T10:00:00Z', current: false }
      ]
    })
    const s = await useAccount().listSessions()
    expect(requestMock).toHaveBeenCalledWith('/auth/sessions')
    expect(s).toHaveLength(2)
    expect(s.filter(x => x.current)).toHaveLength(1)
    expect(s[0]).toMatchObject({ id: 's1', device: 'Chrome · macOS', icon: 'i-lucide-monitor', current: true })
    expect(s[1]).toMatchObject({ id: 's2', device: 'Safari · iOS', icon: 'i-lucide-smartphone', current: false })
    // Current session's meta uses the resolved location + the "now" label.
    expect(s[0]!.meta).toContain('Jakarta, Indonesia')
    // The other session (no GeoIP) falls back to its IP.
    expect(s[1]!.meta).toContain('2.2.2.2')
  })

  it('maps an unknown user-agent to a generic device label + globe icon', async () => {
    requestMock.mockResolvedValueOnce({
      data: [{ id: 's3', browser: '', os: '', device_type: 'unknown', ip_address: '3.3.3.3', location: '', created_at: '2026-07-15T10:00:00Z', last_seen_at: '2026-07-15T11:00:00Z', current: false }]
    })
    const s = await useAccount().listSessions()
    expect(s[0]).toMatchObject({ icon: 'i-lucide-globe' })
    expect(s[0]!.device).toBe('Perangkat tidak dikenal')
  })

  it('returns an empty list when the API sends no data', async () => {
    requestMock.mockResolvedValueOnce({})
    expect(await useAccount().listSessions()).toEqual([])
  })

  it('persists notification preferences', () => {
    const a = useAccount()
    a.setNotifPrefs({ approval: false, maint: true, assign: true })
    expect(a.getNotifPrefs()).toEqual({ approval: false, maint: true, assign: true })
  })

  it('revokeSession DELETEs /auth/sessions/:id (id url-encoded)', async () => {
    requestMock.mockResolvedValueOnce(undefined)
    await useAccount().revokeSession('s2')
    expect(requestMock).toHaveBeenCalledWith('/auth/sessions/s2', { method: 'DELETE' })
  })

  it('logoutAllOthers POSTs /auth/sessions/revoke-others', async () => {
    requestMock.mockResolvedValueOnce(undefined)
    await useAccount().logoutAllOthers()
    expect(requestMock).toHaveBeenCalledWith('/auth/sessions/revoke-others', { method: 'POST' })
  })
})
