// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { defineComponent } from 'vue'
import { useApiClient } from '~/composables/useApiClient'
import { useAuthApi } from '~/composables/useAuthApi'

const fetchMock = vi.fn((url: string) => {
  const u = String(url)
  if (u.includes('/auth/login') || u.includes('/auth/refresh')) return Promise.resolve({ access_token: 'acc' })
  if (u.includes('/auth/permissions')) return Promise.resolve({ permissions: [] })
  if (u.includes('/auth/logout')) return Promise.resolve({ status: 'logged_out' })
  return Promise.resolve({ id: '1', name: 'A', email: 'a@b.com', role_id: 'r' }) // /auth/me
})
vi.stubGlobal('$fetch', fetchMock)
mockNuxtImport('navigateTo', () => vi.fn())

const Harness = defineComponent({
  setup() {
    return { client: useApiClient(), authApi: useAuthApi() }
  },
  template: '<div />'
})

function callFor(part: string): [string, Record<string, unknown>] | undefined {
  const c = fetchMock.mock.calls.find(([u]) => String(u).includes(part))
  return c as [string, Record<string, unknown>] | undefined
}

describe('auth httpOnly cookie flow', () => {
  beforeEach(() => fetchMock.mockClear())

  it('refreshToken posts /auth/refresh with credentials include and no body', async () => {
    const w = await mountSuspended(Harness)
    const ok = await (w.vm as unknown as { client: ReturnType<typeof useApiClient> }).client.refreshToken()
    expect(ok).toBe(true)
    const call = callFor('/auth/refresh')
    expect(call).toBeTruthy()
    expect(call![1].credentials).toBe('include')
    expect(call![1].body).toBeUndefined()
  })

  it('login posts /auth/login with credentials include', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { authApi: ReturnType<typeof useAuthApi> }).authApi.login('a@b.com', 'pw')
    const call = callFor('/auth/login')
    expect(call).toBeTruthy()
    expect(call![1].credentials).toBe('include')
  })

  it('logout posts /auth/logout with credentials include', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { authApi: ReturnType<typeof useAuthApi> }).authApi.logout()
    const call = callFor('/auth/logout')
    expect(call).toBeTruthy()
    expect(call![1].credentials).toBe('include')
  })
})
