import type { AccountProfile, AccountSession, NotifPrefs } from '~/types'
import { formatRelativeTime } from '~/utils/format'

const NOTIF_KEY = 'inventra.account.notif'
const DEFAULT_NOTIF: NotifPrefs = { approval: true, maint: true, assign: false }

export interface ProfileInput { nama: string, telepon: string }

// Shape returned by GET /auth/sessions (backend `SessionView`, snake_case).
interface SessionApiResponse {
  id: string
  browser: string
  os: string
  device_type: string
  ip_address: string
  location: string
  created_at: string
  last_seen_at: string
  current: boolean
}

// Lucide icon per device-type bucket returned by the backend UA parser.
const DEVICE_ICONS: Record<string, string> = {
  desktop: 'i-lucide-monitor',
  mobile: 'i-lucide-smartphone',
  tablet: 'i-lucide-tablet'
}

// Shape returned by GET/PUT /auth/profile (backend `ProfileView`, snake_case).
interface ProfileApiResponse {
  id: string
  name: string
  email: string
  phone: string | null
  role_id: string
  role_name: string | null
  office_id: string | null
  office_name: string | null
  employee_id: string | null
  employee_name: string | null
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
  // useI18n() requires an active component instance; this composable is also
  // called from plain code (its own spec), so resolve t/locale off the nuxt app
  // instance instead — same pattern as useGlobalSearch.ts.
  const i18n = useNuxtApp().$i18n as { t: (key: string) => string, locale: { value: string } }
  const t = i18n.t

  // Maps a backend SessionView onto the UI-facing AccountSession. device is
  // "Browser · OS" (falling back to a generic label when neither is known);
  // meta is "location-or-IP · relative-last-seen" (the current session reads
  // "Now" rather than a computed delta); icon is chosen from the device type.
  function mapSession(raw: SessionApiResponse): AccountSession {
    const parts = [raw.browser, raw.os].filter(Boolean)
    const device = parts.length ? parts.join(' · ') : t('account.unknownDevice')
    const where = raw.location || raw.ip_address
    const when = raw.current ? t('account.now') : formatRelativeTime(raw.last_seen_at, i18n.locale.value)
    const meta = [where, when].filter(Boolean).join(' · ')
    return {
      id: raw.id,
      device,
      meta,
      icon: DEVICE_ICONS[raw.device_type] ?? 'i-lucide-globe',
      current: raw.current
    }
  }

  // Maps the backend ProfileView (snake_case) onto the UI-facing AccountProfile
  // (Indonesian field names). The API resolves the display names server-side
  // (role_name/office_name/employee_name via masterdata joins); `peran` still
  // falls back to the auth store when the API name is absent (e.g. a removed
  // role), and `kantor`/`pegawai` render '—' in the page when empty.
  function mapProfile(raw: ProfileApiResponse): AccountProfile {
    const hasEmployee = raw.employee_id != null
    return {
      nama: raw.name,
      email: raw.email,
      telepon: raw.phone ?? '',
      peran: raw.role_name || (auth.user?.role_name ?? ''),
      kantor: raw.office_name ?? '',
      pegawai: raw.employee_name ?? '',
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
      body: { new_email: newEmail, current_password: currentPassword },
      suppressErrorToast: true
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
      body: { current_password: currentPassword },
      suppressErrorToast: true
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
    const res = await client.request<{ data: SessionApiResponse[] }>('/auth/sessions')
    return (res.data ?? []).map(mapSession)
  }

  async function revokeSession(id: string): Promise<void> {
    await client.request(`/auth/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' })
  }

  async function logoutAllOthers(): Promise<void> {
    await client.request('/auth/sessions/revoke-others', { method: 'POST' })
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
