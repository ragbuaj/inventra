import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useCategories } from '~/composables/api/useCategories'

beforeEach(() => request.mockReset())

const sample = { id: 'c1', name: 'IT', code: 'ITX', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3', capitalization_threshold: '1000000.00', is_active: true, created_at: '2026-01-01', updated_at: '2026-01-02' }

describe('useCategories', () => {
  it('tree GETs /categories/tree and returns the data array', async () => {
    request.mockResolvedValueOnce({ data: [sample] })
    const rows = await useCategories().tree()
    expect(request).toHaveBeenCalledWith('/categories/tree')
    expect(rows).toHaveLength(1)
    expect(rows[0].id).toBe('c1')
  })

  it('list builds the query (omits empty search) and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [sample], total: 1, limit: 20, offset: 0 })
    const res = await useCategories().list({ limit: 20, offset: 0 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/categories?')
    expect(path).toContain('limit=20')
    expect(path).not.toContain('search=')
    expect(res.total).toBe(1)
  })

  it('get GETs /categories/:id', async () => {
    request.mockResolvedValueOnce(sample)
    await useCategories().get('c1')
    expect(request).toHaveBeenCalledWith('/categories/c1')
  })

  it('create POSTs /categories with the body verbatim', async () => {
    request.mockResolvedValueOnce(sample)
    const input = { name: 'IT', code: 'ITX', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3', capitalization_threshold: '1000000', is_active: true } as const
    await useCategories().create(input)
    expect(request).toHaveBeenCalledWith('/categories', { method: 'POST', body: input })
  })

  it('update PUTs /categories/:id', async () => {
    request.mockResolvedValueOnce(sample)
    await useCategories().update('c1', { name: 'IT2' } as never)
    expect(request).toHaveBeenCalledWith('/categories/c1', { method: 'PUT', body: { name: 'IT2' } })
  })

  it('remove DELETEs /categories/:id', async () => {
    request.mockResolvedValueOnce(undefined)
    await useCategories().remove('c1')
    expect(request).toHaveBeenCalledWith('/categories/c1', { method: 'DELETE' })
  })
})
