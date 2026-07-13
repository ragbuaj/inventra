// @vitest-environment nuxt
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'

// useAccount's getProfile now hits the real backend via useApiClient — stub
// the HTTP client so account.vue's mount doesn't try to reach :8080 (per the
// wiring-composable-breaks-consumer-tests memory).
const requestMock = vi.fn(() => Promise.resolve({
  id: 'u1',
  name: 'Andi Saputra',
  email: 'andi@inventra.local',
  phone: '0812-3456-7890',
  role_id: 'r1',
  office_id: null,
  employee_id: null,
  status: 'active',
  avatar_url: null,
  google_linked: false,
  joined_at: '2024-03-12T00:00:00Z'
}))
vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: vi.fn(), refreshToken: vi.fn() })
}))

// eslint-disable-next-line import/first
import Akun from '~/pages/account.vue'
// eslint-disable-next-line import/first
import { useAuthStore } from '~/stores/auth'

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager', office_id: null }, ['*'])
}
async function mountLoaded() {
  const w = await mountSuspended(Akun)
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Preferensi tab', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
    localStorage.clear()
  })

  it('shows appearance + notification sections', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Preferensi')!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tampilan')
    expect(w.text()).toContain('Notifikasi')
    expect(w.text()).toContain('Keputusan Approval')
  })

  it('persists a notification toggle', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Preferensi')!.trigger('click')
    await flushPromises()
    const before = localStorage.getItem('inventra.account.notif')
    // toggle the first notification switch via data-testid
    const toggle = w.find('[data-testid="notif-approval"]')
    await toggle.trigger('click')
    await flushPromises()
    expect(localStorage.getItem('inventra.account.notif')).not.toBe(before)
  })
})
