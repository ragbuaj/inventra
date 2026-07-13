import type { AccountProfile, AccountSession, NotifPrefs } from '~/types'
import { fakeLatency } from '~/mock/helpers'

const NOTIF_KEY = 'inventra.account.notif'
const DEFAULT_NOTIF: NotifPrefs = { approval: true, maint: true, assign: false }

export interface ProfileInput { nama: string, telepon: string }

// Shape returned by GET/PUT /auth/profile (backend `ProfileView`, snake_case).
interface ProfileApiResponse {
  id: string
  name: string
  email: string
  phone: string | null
  role_id: string
  office_id: string | null
  employee_id: string | null
  status: string
  avatar_url: string | null
  google_linked: boolean
  joined_at: string
}

export function useAccount() {
  const auth = useAuthStore()
  const client = useApiClient()
  const config = useRuntimeConfig()
  const base = config.public.apiBase as string

  // Maps the backend ProfileView (snake_case) onto the UI-facing AccountProfile
  // (Indonesian field names). `peran`/`kantor`/`pegawai` have no display-name
  // equivalent in the profile payload (the API only returns role_id/office_id/
  // employee_id) — fall back to what the auth store already knows rather than
  // hardcoding fake values.
  function mapProfile(raw: ProfileApiResponse): AccountProfile {
    const hasEmployee = raw.employee_id != null
    return {
      nama: raw.name,
      email: raw.email,
      telepon: raw.phone ?? '',
      peran: auth.user?.role_name ?? '',
      kantor: '',
      pegawai: hasEmployee ? raw.name : '',
      loginMethod: raw.google_linked ? 'google' : 'email',
      joinDate: raw.joined_at,
      hasEmployee
    }
  }

  async function getProfile(): Promise<AccountProfile> {
    const raw = await client.request<ProfileApiResponse>('/auth/profile')
    return mapProfile(raw)
  }

  async function updateProfile(input: ProfileInput): Promise<AccountProfile> {
    if (!input.nama.trim()) throw new Error('account.errRequired')
    const raw = await client.request<ProfileApiResponse>('/auth/profile', {
      method: 'PUT',
      body: { name: input.nama, phone: input.telepon }
    })
    return mapProfile(raw)
  }

  // Verifies the current password and emails a confirmation link to the NEW
  // address; the change completes when the user opens the link (confirmEmailChange).
  async function requestEmailChange(newEmail: string, currentPassword: string): Promise<void> {
    await client.request('/auth/email/change-request', {
      method: 'POST',
      body: { new_email: newEmail, current_password: currentPassword }
    })
  }

  // Public route (reached from an emailed link, no session yet) — raw $fetch,
  // same pattern as requestPasswordReset/resetPassword below.
  async function confirmEmailChange(token: string): Promise<void> {
    await $fetch(`${base}/auth/email/confirm`, { method: 'POST', body: { token } })
  }

  // Verifies the current password and emails a reset link to complete the
  // change (reuses the forgot-password flow) — the Keamanan tab's "Ganti
  // Password" modal (account.vue) calls this instead of changing inline.
  async function requestPasswordChange(currentPassword: string): Promise<void> {
    await client.request('/auth/password/change-request', {
      method: 'POST',
      body: { current_password: currentPassword }
    })
  }

  async function requestPasswordReset(email: string): Promise<void> {
    await $fetch(`${base}/auth/password/forgot`, { method: 'POST', body: { email } })
  }

  async function resetPassword(token: string, newPass: string): Promise<void> {
    if (newPass.length < 8) throw new Error('account.errWeak')
    await $fetch(`${base}/auth/password/reset`, { method: 'POST', body: { token, new_password: newPass } })
  }

  async function listSessions(): Promise<AccountSession[]> {
    await fakeLatency(300)
    return [
      { id: 's1', device: 'Chrome · macOS', meta: 'Jakarta, Indonesia · Sekarang', icon: 'i-lucide-laptop', current: true },
      { id: 's2', device: 'Safari · iPhone 15', meta: 'Jakarta, Indonesia · 2 jam lalu', icon: 'i-lucide-smartphone', current: false },
      { id: 's3', device: 'Edge · Windows 11', meta: 'Bandung, Indonesia · kemarin', icon: 'i-lucide-monitor', current: false }
    ]
  }

  async function revokeSession(_id: string): Promise<void> {
    await fakeLatency()
  }

  async function logoutAllOthers(): Promise<void> {
    await fakeLatency()
  }

  function getNotifPrefs(): NotifPrefs {
    if (import.meta.client) {
      try {
        const raw = localStorage.getItem(NOTIF_KEY)
        if (raw) return JSON.parse(raw) as NotifPrefs
      } catch { /* ignore */ }
    }
    return { ...DEFAULT_NOTIF }
  }

  function setNotifPrefs(p: NotifPrefs): void {
    if (import.meta.client) {
      try {
        localStorage.setItem(NOTIF_KEY, JSON.stringify(p))
      } catch { /* ignore */ }
    }
  }

  return { getProfile, updateProfile, requestEmailChange, confirmEmailChange, requestPasswordChange, requestPasswordReset, resetPassword, listSessions, revokeSession, logoutAllOthers, getNotifPrefs, setNotifPrefs }
}
