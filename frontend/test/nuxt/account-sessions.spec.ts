// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'

enableAutoUnmount(afterEach)

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

const profileResponse = {
  id: 'u1', name: 'Andi', email: 'andi@inventra.local', phone: null,
  role_id: 'r1', office_id: null, employee_id: null, status: 'active',
  avatar_url: null, google_linked: false, joined_at: '2024-03-12T00:00:00Z'
}

const currentSession: SessionApiResponse = {
  id: 's1', browser: 'Chrome', os: 'macOS', device_type: 'desktop',
  ip_address: '1.1.1.1', location: 'Jakarta, Indonesia',
  created_at: '2026-07-15T10:00:00Z', last_seen_at: '2026-07-15T12:00:00Z', current: true
}
const otherSession: SessionApiResponse = {
  id: 's2', browser: 'Safari', os: 'iOS', device_type: 'mobile',
  ip_address: '2.2.2.2', location: '',
  created_at: '2026-07-14T10:00:00Z', last_seen_at: '2026-07-15T10:00:00Z', current: false
}

// The list the /auth/sessions GET returns; mutated by revoke/revoke-others so a
// refetch reflects the new state.
let sessionList: SessionApiResponse[] = []

const requestMock = vi.fn((path: string, opts?: Record<string, unknown>) => {
  if (path === '/auth/sessions') return Promise.resolve({ data: sessionList })
  if (path === '/auth/sessions/revoke-others') {
    sessionList = sessionList.filter(s => s.current)
    return Promise.resolve({ revoked: 1 })
  }
  if (path.startsWith('/auth/sessions/') && opts?.method === 'DELETE') {
    const id = decodeURIComponent(path.split('/').pop() as string)
    sessionList = sessionList.filter(s => s.id !== id)
    return Promise.resolve({ status: 'revoked' })
  }
  return Promise.resolve(profileResponse)
})
vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: vi.fn(), refreshToken: vi.fn() })
}))

// eslint-disable-next-line import/first
import Akun from '~/pages/account.vue'
// eslint-disable-next-line import/first
import { useAuthStore } from '~/stores/auth'

function login() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager', office_id: null }, ['*'])
}

async function mountLoaded() {
  const w = await mountSuspended(Akun, { props: {} })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

async function openSecurityTab(w: Awaited<ReturnType<typeof mountLoaded>>) {
  const tabBtn = w.findAll('button').find(b => b.text().trim() === 'Keamanan')!
  await tabBtn.trigger('click')
  await flushPromises()
}

function revokeButtons(w: Awaited<ReturnType<typeof mountLoaded>>) {
  return w.findAll('button').filter(b => b.text().trim() === 'Keluar')
}

describe('Account page — Sesi & Perangkat', () => {
  beforeEach(() => {
    useAuthStore().clear()
    login()
    sessionList = [structuredClone(currentSession), structuredClone(otherSession)]
    requestMock.mockClear()
  })

  it('renders a row per session with the device label and location/IP meta', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)
    expect(requestMock).toHaveBeenCalledWith('/auth/sessions')
    expect(w.text()).toContain('Chrome · macOS')
    expect(w.text()).toContain('Safari · iOS')
    // The current session shows its resolved location; the other (no GeoIP) shows its IP.
    expect(w.text()).toContain('Jakarta, Indonesia')
    expect(w.text()).toContain('2.2.2.2')
  })

  it('flags only the current session with the "Sesi ini" badge', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)
    const badges = w.findAll('span').filter(s => s.text().trim() === 'Sesi ini')
    expect(badges).toHaveLength(1)
    // The current row has no revoke button; only the other session is revocable.
    expect(revokeButtons(w)).toHaveLength(1)
  })

  it('revoking a session calls DELETE and removes its row', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)
    expect(revokeButtons(w)).toHaveLength(1)

    await revokeButtons(w)[0]!.trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/auth/sessions/s2', expect.objectContaining({ method: 'DELETE' }))
    expect(w.text()).not.toContain('Safari · iOS')
    expect(revokeButtons(w)).toHaveLength(0)
  })

  it('"logout all others" calls revoke-others then re-fetches, collapsing to the current device', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)

    const logoutAllBtn = w.findAll('button').find(b => b.text().trim() === 'Keluar dari semua perangkat')!
    await logoutAllBtn.trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/auth/sessions/revoke-others', expect.objectContaining({ method: 'POST' }))
    // Re-fetched after revoke-others → only the current session remains.
    expect(w.text()).toContain('Chrome · macOS')
    expect(w.text()).not.toContain('Safari · iOS')
    expect(revokeButtons(w)).toHaveLength(0)
  })
})
