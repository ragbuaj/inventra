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
