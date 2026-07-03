// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { defineComponent } from 'vue'
import { useApiClient } from '~/composables/useApiClient'
import { useAuthStore } from '~/stores/auth'

const fetchMock = vi.fn((_path?: string, _opts?: Record<string, unknown>) => Promise.resolve({} as unknown))
vi.stubGlobal('$fetch', fetchMock)

const Harness = defineComponent({
  setup() {
    return { api: useApiClient() }
  },
  template: '<div />'
})

function lastHeaders(): Record<string, string> {
  const calls = fetchMock.mock.calls as unknown as Array<[string, { headers: Record<string, string> }]>
  return calls.at(-1)![1].headers
}

describe('useApiClient X-Request-ID propagation', () => {
  beforeEach(() => fetchMock.mockClear())

  it('adds a UUID X-Request-ID header when the caller provides none', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api.request('/x')
    expect(lastHeaders()['X-Request-ID']).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i)
  })

  it('preserves a caller-provided X-Request-ID', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api.request('/x', { headers: { 'X-Request-ID': 'fixed-id' } })
    expect(lastHeaders()['X-Request-ID']).toBe('fixed-id')
  })
})

describe('useApiClient requestBlob', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    useAuthStore().setSession('tok', { id: '1', name: 'A', email: 'a@b.com', role_id: 'r', role_name: 'Superadmin' }, ['*'])
  })

  afterEach(() => {
    useAuthStore().clear()
  })

  it('passes responseType blob, carries Authorization, retries once after 401 refresh, and resolves the Blob', async () => {
    const blob = new Blob(['barcode-bytes'])
    // Snapshot each call's options at invocation time — doFetch mutates the
    // shared `headers` object in place before the retry, so reading
    // fetchMock.mock.calls after the fact would only ever see the final value.
    const snapshots: Array<Record<string, unknown>> = []
    function snapshot(opts?: Record<string, unknown>) {
      snapshots.push({ ...opts, headers: { ...(opts?.headers as Record<string, string>) } })
    }
    fetchMock
      .mockImplementationOnce((_path?: string, opts?: Record<string, unknown>) => {
        snapshot(opts)
        return Promise.reject(Object.assign(new Error('unauthorized'), { statusCode: 401 }))
      })
      .mockImplementationOnce((_path?: string, opts?: Record<string, unknown>) => {
        snapshot(opts)
        return Promise.resolve({ access_token: 'tok2' })
      })
      .mockImplementationOnce((_path?: string, opts?: Record<string, unknown>) => {
        snapshot(opts)
        return Promise.resolve(blob)
      })

    const w = await mountSuspended(Harness)
    const result = await (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api
      .requestBlob('/assets/x/barcode?type=qr')

    expect(result).toBe(blob)
    expect(fetchMock).toHaveBeenCalledTimes(3)

    expect(snapshots[0]!.responseType).toBe('blob')
    expect((snapshots[0]!.headers as Record<string, string>).Authorization).toBe('Bearer tok')

    expect(snapshots[2]!.responseType).toBe('blob')
    expect((snapshots[2]!.headers as Record<string, string>).Authorization).toBe('Bearer tok2')
  })
})

describe('useApiClient refresh single-flight', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    useAuthStore().setSession('tok', { id: '1', name: 'A', email: 'a@b.com', role_id: 'r', role_name: 'Superadmin' }, ['*'])
  })

  afterEach(() => {
    useAuthStore().clear()
  })

  it('concurrent refreshToken calls share ONE /auth/refresh request', async () => {
    // The backend refresh token is single-use (rotated on each refresh):
    // two concurrent POST /auth/refresh with the same cookie means the second
    // 401s and the session is nuked. Concurrent callers MUST share one call.
    let resolveRefresh!: (v: { access_token: string }) => void
    fetchMock.mockImplementation((path?: string) => {
      if (String(path).includes('/auth/refresh')) {
        return new Promise((resolve) => {
          resolveRefresh = resolve
        })
      }
      return Promise.resolve({})
    })

    const w = await mountSuspended(Harness)
    const api = (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api

    const p1 = api.refreshToken()
    const p2 = api.refreshToken()
    resolveRefresh({ access_token: 'tok-new' })

    const [r1, r2] = await Promise.all([p1, p2])
    expect(r1).toBe(true)
    expect(r2).toBe(true)
    const refreshCalls = fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))
    expect(refreshCalls).toHaveLength(1)
    expect(useAuthStore().accessToken).toBe('tok-new')
  })

  it('concurrent 401-triggered requests share one refresh and both retry successfully', async () => {
    let resolveRefresh!: (v: { access_token: string }) => void
    let firstAttempts = 0
    fetchMock.mockImplementation((path?: string, opts?: Record<string, unknown>) => {
      const p = String(path)
      if (p.includes('/auth/refresh')) {
        return new Promise((resolve) => {
          resolveRefresh = resolve
        })
      }
      const authz = (opts?.headers as Record<string, string>)?.Authorization
      if (authz === 'Bearer tok') {
        firstAttempts += 1
        return Promise.reject(Object.assign(new Error('unauthorized'), { statusCode: 401 }))
      }
      return Promise.resolve({ ok: p })
    })

    const w = await mountSuspended(Harness)
    const api = (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api

    const q1 = api.request<{ ok: string }>('/a')
    const q2 = api.request<{ ok: string }>('/b')
    // Let both first attempts 401 and enter the refresh path before resolving it.
    await new Promise(r => setTimeout(r, 0))
    resolveRefresh({ access_token: 'tok-new' })

    const [a, b] = await Promise.all([q1, q2])
    expect(firstAttempts).toBe(2)
    expect(a.ok).toContain('/a')
    expect(b.ok).toContain('/b')
    const refreshCalls = fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))
    expect(refreshCalls).toHaveLength(1)
  })

  it('a refresh after a completed one issues a new request (no stale sharing)', async () => {
    fetchMock.mockImplementation((path?: string) => {
      if (String(path).includes('/auth/refresh')) {
        return Promise.resolve({ access_token: 'tok-n' })
      }
      return Promise.resolve({})
    })

    const w = await mountSuspended(Harness)
    const api = (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api

    expect(await api.refreshToken()).toBe(true)
    expect(await api.refreshToken()).toBe(true)
    const refreshCalls = fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))
    expect(refreshCalls).toHaveLength(2)
  })

  it('shared refresh failure propagates false to all concurrent callers', async () => {
    let rejectRefresh!: (e: unknown) => void
    fetchMock.mockImplementation((path?: string) => {
      if (String(path).includes('/auth/refresh')) {
        return new Promise((_r, reject) => {
          rejectRefresh = reject
        })
      }
      return Promise.resolve({})
    })

    const w = await mountSuspended(Harness)
    const api = (w.vm as unknown as { api: ReturnType<typeof useApiClient> }).api

    const p1 = api.refreshToken()
    const p2 = api.refreshToken()
    rejectRefresh(Object.assign(new Error('unauthorized'), { statusCode: 401 }))

    expect(await p1).toBe(false)
    expect(await p2).toBe(false)
    const refreshCalls = fetchMock.mock.calls.filter(c => String(c[0]).includes('/auth/refresh'))
    expect(refreshCalls).toHaveLength(1)
  })
})
