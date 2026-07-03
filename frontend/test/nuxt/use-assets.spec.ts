import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useAssets } from '~/composables/api/useAssets'

const sampleAsset = {
  id: 'a1',
  asset_tag: 'JKT01-ELK-2026-00001',
  name: 'Laptop Dell Latitude 5440',
  category_id: 'c1',
  office_id: 'o1',
  status: 'available',
  asset_class: 'tangible',
  purchase_cost: '18500000.00'
}

beforeEach(() => request.mockReset())

describe('useAssets.list', () => {
  it('builds the default query (limit=20, offset=0) with no other params', async () => {
    request.mockResolvedValueOnce({ data: [sampleAsset], total: 1, limit: 20, offset: 0 })
    const res = await useAssets().list()
    expect(request).toHaveBeenCalledWith('/assets?limit=20&offset=0')
    expect(res.total).toBe(1)
  })

  it('appends only the provided filters, omitting the rest', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 20, offset: 0 })
    await useAssets().list({ status: 'available', category_id: 'c1' })
    const path = request.mock.calls[0]![0] as string
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=0')
    expect(path).toContain('status=available')
    expect(path).toContain('category_id=c1')
    expect(path).not.toContain('search=')
    expect(path).not.toContain('office_id=')
    expect(path).not.toContain('asset_class=')
  })

  it('honors a custom limit/offset and URL-encodes the search term', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 5, offset: 10 })
    await useAssets().list({ limit: 5, offset: 10, search: 'laptop dell' })
    const path = request.mock.calls[0]![0] as string
    expect(path).toContain('limit=5')
    expect(path).toContain('offset=10')
    expect(path).toContain('search=laptop+dell')
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('network down'))
    await expect(useAssets().list()).rejects.toThrow('network down')
  })
})

describe('useAssets.get', () => {
  it('GETs /assets/:id', async () => {
    request.mockResolvedValueOnce(sampleAsset)
    const res = await useAssets().get('a1')
    expect(request).toHaveBeenCalledWith('/assets/a1')
    expect(res.id).toBe('a1')
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('not found'))
    await expect(useAssets().get('nope')).rejects.toThrow('not found')
  })
})

describe('useAssets.getByTag', () => {
  it('GETs /assets/by-tag/:tag', async () => {
    request.mockResolvedValueOnce(sampleAsset)
    const res = await useAssets().getByTag('JKT01-ELK-2026-00001')
    expect(request).toHaveBeenCalledWith('/assets/by-tag/JKT01-ELK-2026-00001')
    expect(res.asset_tag).toBe('JKT01-ELK-2026-00001')
  })

  it('URL-encodes a tag containing special characters', async () => {
    request.mockResolvedValueOnce(sampleAsset)
    await useAssets().getByTag('JKT 01/ELK#1')
    expect(request).toHaveBeenCalledWith(`/assets/by-tag/${encodeURIComponent('JKT 01/ELK#1')}`)
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('not found'))
    await expect(useAssets().getByTag('nope')).rejects.toThrow('not found')
  })
})

describe('useAssets.update', () => {
  it('PUTs /assets/:id with exactly the AssetUpdateInput keys — no purchase_cost/status/asset_class', async () => {
    request.mockResolvedValueOnce(sampleAsset)
    const input = { name: 'New name', category_id: 'c2', serial_number: 'SN-1' }
    await useAssets().update('a1', input)
    expect(request).toHaveBeenCalledWith('/assets/a1', { method: 'PUT', body: input })
    const [, opts] = request.mock.calls[0] as [string, { body: Record<string, unknown> }]
    expect(opts.body).not.toHaveProperty('purchase_cost')
    expect(opts.body).not.toHaveProperty('status')
    expect(opts.body).not.toHaveProperty('asset_class')
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('validation failed'))
    await expect(useAssets().update('a1', { name: 'X', category_id: 'c1' })).rejects.toThrow('validation failed')
  })
})
