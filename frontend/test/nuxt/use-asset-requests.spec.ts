import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useAssetRequests } from '~/composables/api/useAssetRequests'

const sampleRequest = {
  id: 'r1',
  type: 'asset_create',
  status: 'pending',
  amount: '18500000.00',
  office_id: 'o1',
  created_at: '2026-07-03T00:00:00Z'
}

beforeEach(() => request.mockReset())

describe('useAssetRequests.submitCreate', () => {
  it('POSTs /requests with type=asset_create, amount as string from purchase_cost, office_id, and the full payload', async () => {
    request.mockResolvedValueOnce(sampleRequest)
    const input = {
      office_id: 'o1',
      asset_class: 'tangible' as const,
      name: 'Laptop Dell Latitude 5440',
      category_id: 'c1',
      purchase_cost: '18500000.00'
    }
    const res = await useAssetRequests().submitCreate(input)
    expect(request).toHaveBeenCalledWith('/requests', {
      method: 'POST',
      body: {
        type: 'asset_create',
        amount: '18500000.00',
        office_id: 'o1',
        payload: input
      }
    })
    expect(res.id).toBe('r1')
  })

  it('defaults amount to "0" (string) when purchase_cost is absent', async () => {
    request.mockResolvedValueOnce(sampleRequest)
    const input = { office_id: 'o1', asset_class: 'tangible' as const, name: 'Kursi kantor', category_id: 'c2' }
    await useAssetRequests().submitCreate(input)
    const [, opts] = request.mock.calls[0] as [string, { body: { amount: unknown } }]
    expect(opts.body.amount).toBe('0')
    expect(typeof opts.body.amount).toBe('string')
  })

  it('defaults amount to "0" (string) when purchase_cost is null', async () => {
    request.mockResolvedValueOnce(sampleRequest)
    const input = { office_id: 'o1', asset_class: 'tangible' as const, name: 'Kursi kantor', category_id: 'c2', purchase_cost: null }
    await useAssetRequests().submitCreate(input)
    const [, opts] = request.mock.calls[0] as [string, { body: { amount: unknown } }]
    expect(opts.body.amount).toBe('0')
    expect(typeof opts.body.amount).toBe('string')
  })

  it('passes through every AssetCreateInput key unchanged in payload', async () => {
    request.mockResolvedValueOnce(sampleRequest)
    const input = {
      office_id: 'o1',
      asset_class: 'tangible' as const,
      name: 'Laptop Dell Latitude 5440',
      category_id: 'c1',
      brand_id: 'b1',
      model_id: 'm1',
      room_id: 'rm1',
      unit_id: 'u1',
      vendor_id: 'v1',
      serial_number: 'SN-1',
      po_number: 'PO-1',
      funding_source: 'APBN',
      purchase_date: '2026-01-01',
      purchase_cost: '18500000.00',
      warranty_expiry: '2028-01-01',
      notes: 'catatan'
    }
    await useAssetRequests().submitCreate(input)
    const [, opts] = request.mock.calls[0] as [string, { body: { payload: unknown } }]
    expect(opts.body.payload).toEqual(input)
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('validation failed'))
    const input = { office_id: 'o1', asset_class: 'tangible' as const, name: 'X', category_id: 'c1' }
    await expect(useAssetRequests().submitCreate(input)).rejects.toThrow('validation failed')
  })
})
