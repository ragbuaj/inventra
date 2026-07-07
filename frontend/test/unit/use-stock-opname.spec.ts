import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
const requestBlob = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request, requestBlob }) }))

// eslint-disable-next-line import/first
import { useStockOpname } from '~/composables/api/useStockOpname'

beforeEach(() => {
  request.mockReset()
  requestBlob.mockReset()
})

describe('useStockOpname', () => {
  it('list builds the query with status filter and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 's1' }], total: 1, limit: 20, offset: 0 })
    const res = await useStockOpname().list({ status: 'counting', limit: 20, offset: 0 })
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions', { query: { status: 'counting', limit: 20, offset: 0 } })
    expect(res.total).toBe(1)
  })

  it('list omits undefined filters', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 20, offset: 0 })
    await useStockOpname().list()
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions', { query: {} })
  })

  it('get GETs the session detail (session + kpi)', async () => {
    request.mockResolvedValueOnce({ id: 's1', status: 'counting', total: 10, found: 4, pending: 6, variance: 0 })
    const res = await useStockOpname().get('s1')
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1')
    expect(res.total).toBe(10)
  })

  it('items GETs the session items with only the result filter', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 0, offset: 0 })
    await useStockOpname().items('s1', { result: 'found' })
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/items', { query: { result: 'found' } })
  })

  it('items omits the filter entirely when not provided', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 0, offset: 0 })
    await useStockOpname().items('s1')
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/items', { query: {} })
  })

  it('creates a session with the exact body', async () => {
    request.mockResolvedValueOnce({ id: 's1' })
    await useStockOpname().create({ office_id: 'o1', name: 'Opname', period: '2026-07' })
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions', { method: 'POST', body: { office_id: 'o1', name: 'Opname', period: '2026-07' } })
  })

  it('create omits an absent optional name', async () => {
    request.mockResolvedValueOnce({ id: 's1' })
    await useStockOpname().create({ office_id: 'o1', period: '2026-07' })
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions', { method: 'POST', body: { office_id: 'o1', period: '2026-07' } })
  })

  it('starts a session', async () => {
    request.mockResolvedValueOnce({ id: 's1', status: 'counting' })
    await useStockOpname().start('s1')
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/start', { method: 'POST' })
  })

  it('scans an asset tag', async () => {
    request.mockResolvedValueOnce({ id: 'i1', session_id: 's1', asset_id: 'a1', expected: true, result: 'found' })
    const res = await useStockOpname().scan('s1', 'AST-001')
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/scan', { method: 'POST', body: { asset_tag: 'AST-001' } })
    expect(res.result).toBe('found')
  })

  it('sets an item result', async () => {
    request.mockResolvedValueOnce({ id: 'i1', session_id: 's1', asset_id: 'a1', expected: true, result: 'found', note: null, counted_at: '2026-07-07T00:00:00Z' })
    await useStockOpname().setResult('s1', 'i1', { result: 'found', note: null })
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/items/i1', { method: 'PATCH', body: { result: 'found', note: null } })
  })

  it('reconciles a session', async () => {
    request.mockResolvedValueOnce({ id: 's1', status: 'reconciling' })
    await useStockOpname().reconcile('s1')
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/reconcile', { method: 'POST' })
  })

  it('posts a follow-up and returns {request_id, type} (not request_type)', async () => {
    request.mockResolvedValueOnce({ request_id: 'r1', type: 'asset_disposal' })
    const res = await useStockOpname().followup('s1', 'i1', { to_office_id: null, to_room_id: null, reason: null })
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/items/i1/follow-up', { method: 'POST', body: { to_office_id: null, to_room_id: null, reason: null } })
    expect(res.type).toBe('asset_disposal')
    expect((res as unknown as { request_type?: string }).request_type).toBeUndefined()
  })

  it('closes a session', async () => {
    request.mockResolvedValueOnce({ id: 's1', status: 'closed' })
    await useStockOpname().close('s1')
    expect(request).toHaveBeenCalledWith('/stock-opname/sessions/s1/close', { method: 'POST' })
  })

  it('reportUrl fetches the report blob for the given format', async () => {
    const blob = new Blob(['pdf-bytes'])
    requestBlob.mockResolvedValueOnce(blob)
    const res = await useStockOpname().reportUrl('s1', 'pdf')
    expect(requestBlob).toHaveBeenCalledWith('/stock-opname/sessions/s1/report', { query: { format: 'pdf' } })
    expect(res).toBe(blob)
  })
})
