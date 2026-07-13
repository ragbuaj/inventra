// @vitest-environment nuxt
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'

// useAccount's getProfile/updateProfile now hit the real backend via
// useApiClient — stub the HTTP client so account.vue's mount doesn't try to
// reach :8080 (per the wiring-composable-breaks-consumer-tests memory).
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
  const w = await mountSuspended(Akun, { route: '/account' })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Profil tab', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
  })

  it('renders the profile header and personal data', async () => {
    const w = await mountLoaded()
    expect(w.text()).toContain('Andi Saputra')
    expect(w.text()).toContain('Asset Manager')
    expect(w.text()).toContain('Data Diri')
  })

  it('shows the required error when saving with an empty name', async () => {
    const w = await mountLoaded()
    const nameInput = w.findAll('input')[0]!
    await nameInput.setValue('')
    const saveBtn = w.findAll('button').find(b => b.text().includes('Simpan Perubahan'))!
    await saveBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Wajib diisi')
  })
})
