import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useAssetRequests, multiplyDecimalByInt } from '~/composables/api/useAssetRequests'

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
        // quantity defaults to 1 and is injected into the payload.
        payload: { ...input, quantity: 1 }
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
    expect(opts.body.payload).toEqual({ ...input, quantity: 1 })
  })

  it('multiplies amount by quantity for a batch and injects quantity into payload', async () => {
    request.mockResolvedValueOnce(sampleRequest)
    const input = {
      office_id: 'o1',
      asset_class: 'tangible' as const,
      name: 'Kursi Kantor',
      category_id: 'c1',
      purchase_cost: '3000000',
      quantity: 10
    }
    await useAssetRequests().submitCreate(input)
    const [, opts] = request.mock.calls[0] as [string, { body: { amount: unknown, payload: { quantity: unknown } } }]
    expect(opts.body.amount).toBe('30000000')
    expect(opts.body.payload.quantity).toBe(10)
  })

  it('clamps a non-positive quantity to 1 for the amount and payload', async () => {
    request.mockResolvedValueOnce(sampleRequest)
    const input = {
      office_id: 'o1',
      asset_class: 'tangible' as const,
      name: 'X',
      category_id: 'c1',
      purchase_cost: '5000000',
      quantity: 0
    }
    await useAssetRequests().submitCreate(input)
    const [, opts] = request.mock.calls[0] as [string, { body: { amount: unknown, payload: { quantity: unknown } } }]
    expect(opts.body.amount).toBe('5000000')
    expect(opts.body.payload.quantity).toBe(1)
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('validation failed'))
    const input = { office_id: 'o1', asset_class: 'tangible' as const, name: 'X', category_id: 'c1' }
    await expect(useAssetRequests().submitCreate(input)).rejects.toThrow('validation failed')
  })
})

describe('multiplyDecimalByInt', () => {
  it('multiplies whole-rupiah costs exactly', () => {
    expect(multiplyDecimalByInt('3000000', 10)).toBe('30000000')
    expect(multiplyDecimalByInt('1500000', 1)).toBe('1500000')
    expect(multiplyDecimalByInt('0', 5)).toBe('0')
  })

  it('preserves decimal fractions without floating-point drift', () => {
    expect(multiplyDecimalByInt('3000000.25', 3)).toBe('9000000.75')
    expect(multiplyDecimalByInt('0.1', 3)).toBe('0.3')
    expect(multiplyDecimalByInt('1500000.50', 2)).toBe('3000001')
  })

  it('trims trailing-zero fractions to a plain integer', () => {
    expect(multiplyDecimalByInt('1000.50', 2)).toBe('2001')
    expect(multiplyDecimalByInt('2500.00', 4)).toBe('10000')
  })

  it('handles large values beyond safe-integer range via BigInt', () => {
    expect(multiplyDecimalByInt('9007199254740993', 2)).toBe('18014398509481986')
  })
})
