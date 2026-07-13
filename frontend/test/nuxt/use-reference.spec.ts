// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useReference } from '~/composables/api/useReference'

const sampleRow = {
  id: 'abc',
  name: 'Sample Brand',
  code: 'SMB'
}

beforeEach(() => request.mockReset())

describe('useReference.get', () => {
  it('GETs /{key}/{id}', async () => {
    request.mockResolvedValueOnce(sampleRow)
    const res = await useReference().get('brands', 'abc')
    expect(request).toHaveBeenCalledWith('/brands/abc')
    expect(res.id).toBe('abc')
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('not found'))
    await expect(useReference().get('brands', 'xyz')).rejects.toThrow('not found')
  })
})
