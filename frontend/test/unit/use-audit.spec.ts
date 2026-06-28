import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAudit } from '~/composables/api/useAudit'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

beforeEach(() => request.mockReset())

const apiRow = {
  id: 'a1', entity_type: 'assets', entity_id: 'e9', action: 'update', ip: '10.0.0.1',
  changes: { name: { before: 'Old', after: 'New' }, status: { after: 'available' } },
  actor: { id: 'u1', name: 'Bambang Sukasno', email: 'b@x.id' },
  office_id: 'o1', created_at: '2026-06-24T08:30:00Z'
}

describe('useAudit', () => {
  it('list builds the query from non-empty params and returns {rows,total}', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const res = await useAudit().list({ entity_type: 'assets', action: 'update', search: '', limit: 20, offset: 40 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/audit?')
    expect(path).toContain('entity_type=assets')
    expect(path).toContain('action=update')
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=40')
    expect(path).not.toContain('search=') // empty search omitted
    expect(res.total).toBe(1)
  })

  it('maps the API row to a display AuditRow', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const { rows } = await useAudit().list({ limit: 20, offset: 0 })
    const r = rows[0]
    expect(r).toMatchObject({
      id: 'a1', actor: 'Bambang Sukasno', actor_email: 'b@x.id', initials: 'BS',
      action: 'update', entity_type: 'assets', entity_id: 'e9', ip: '10.0.0.1',
      date: '2026-06-24', time: '08:30'
    })
    // changes → diff view
    expect(r.diff).toContainEqual({ field: 'name', before: 'Old', after: 'New', hasBefore: true, hasAfter: true, hasArrow: true })
    expect(r.diff).toContainEqual({ field: 'status', before: '', after: 'available', hasBefore: false, hasAfter: true, hasArrow: false })
  })
})
