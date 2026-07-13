import type { AccountProfile, AccountSession, NotifPrefs } from '~/types'
import { fakeLatency } from '~/mock/helpers'

const NOTIF_KEY = 'inventra.account.notif'
const DEFAULT_NOTIF: NotifPrefs = { approval: true, maint: true, assign: false }

export interface ProfileInput { nama: string, telepon: string }
export interface PasswordInput { oldPass: string, newPass: string, confirmPass: string }

export function useAccount() {
  const auth = useAuthStore()
  const client = useApiClient()
  const config = useRuntimeConfig()
  const base = config.public.apiBase as string

  async function getProfile(): Promise<AccountProfile> {
    await fakeLatency(400)
    return {
      nama: auth.user?.name ?? '',
      email: auth.user?.email ?? '',
      telepon: '0812-3456-7890',
      peran: auth.user?.role_name ?? '',
      kantor: 'Cabang Jakarta Selatan',
      pegawai: auth.user?.name ?? '',
      loginMethod: 'email',
      joinDate: '2024-03-12'
    }
  }

  async function updateProfile(input: ProfileInput): Promise<void> {
    if (!input.nama.trim()) throw new Error('account.errRequired')
    await fakeLatency()
  }

  async function changePassword(input: PasswordInput): Promise<void> {
    if (!input.oldPass || !input.newPass || !input.confirmPass) throw new Error('account.errRequired')
    if (input.newPass !== input.confirmPass) throw new Error('account.errConfirmMismatch')
    if (input.newPass.length < 8) throw new Error('account.errWeak')
    await client.request('/auth/password', {
      method: 'PUT',
      body: { old_password: input.oldPass, new_password: input.newPass }
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

  return { getProfile, updateProfile, changePassword, requestPasswordReset, resetPassword, listSessions, revokeSession, logoutAllOthers, getNotifPrefs, setNotifPrefs }
}
