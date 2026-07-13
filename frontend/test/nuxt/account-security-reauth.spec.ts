// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import Akun from '~/pages/account.vue'
import { useAuthStore } from '~/stores/auth'

// Hoisted mocks — must be created before any mockNuxtImport calls.
const { changePasswordMock, navigateToMock } = vi.hoisted(() => ({
  changePasswordMock: vi.fn(() => Promise.resolve()),
  navigateToMock: vi.fn()
}))

mockNuxtImport('useAccount', () => () => ({
  getProfile: vi.fn(() => Promise.resolve({
    nama: 'Andi Saputra',
    email: 'andi@inventra.local',
    telepon: '0812-3456-7890',
    peran: 'Asset Manager',
    kantor: 'Cabang Jakarta Selatan',
    pegawai: 'Andi Saputra',
    loginMethod: 'email',
    joinDate: '2024-03-12'
  })),
  updateProfile: vi.fn(() => Promise.resolve()),
  changePassword: changePasswordMock,
  requestPasswordReset: vi.fn(() => Promise.resolve()),
  resetPassword: vi.fn(() => Promise.resolve()),
  listSessions: vi.fn(() => Promise.resolve([])),
  revokeSession: vi.fn(() => Promise.resolve()),
  logoutAllOthers: vi.fn(() => Promise.resolve()),
  getNotifPrefs: vi.fn(() => ({ approval: true, maint: true, assign: false })),
  setNotifPrefs: vi.fn()
}))
mockNuxtImport('navigateTo', () => navigateToMock)

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager', office_id: null }, ['*'])
}

async function mountLoaded() {
  const w = await mountSuspended(Akun, { props: {} })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Keamanan tab — password change re-auth', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
    changePasswordMock.mockClear()
    navigateToMock.mockClear()
  })

  it('on successful password change, clears the session and redirects to login', async () => {
    const w = await mountLoaded()
    // Locale can resolve to either 'id' or 'en' depending on the jsdom-detected
    // browser language (@nuxtjs/i18n's detectBrowserLanguage) — match either.
    const tabBtn = w.findAll('button').find(b => ['Keamanan', 'Security'].includes(b.text().trim()))!
    await tabBtn.trigger('click')
    await flushPromises()
    const pw = w.findAll('input[type="password"]')
    await pw[0]!.setValue('oldpass123')
    await pw[1]!.setValue('Abcdefg1!')
    await pw[2]!.setValue('Abcdefg1!')
    const submitBtn = w.findAll('button').find(b => b.text().includes('Ganti Password') || b.text().includes('Change Password'))!
    await submitBtn.trigger('click')
    await flushPromises()

    expect(changePasswordMock).toHaveBeenCalledWith({ oldPass: 'oldpass123', newPass: 'Abcdefg1!', confirmPass: 'Abcdefg1!' })
    expect(useAuthStore().accessToken).toBeNull()
    expect(useAuthStore().user).toBeNull()
    expect(navigateToMock).toHaveBeenCalledWith('/login')
  })
})
