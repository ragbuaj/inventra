// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { defineComponent } from 'vue'
import { useApiClient } from '~/composables/useApiClient'

const fetchMock = vi.fn(() => Promise.resolve({}))
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
