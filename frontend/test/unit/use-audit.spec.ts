import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAudit, toRow } from '~/composables/api/useAudit'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

beforeEach(() => request.mockReset())

const apiRow = {
  id: 'a1', entity_type: 'assets', entity_id: 'e9', action: 'update', ip: '10.0.0.1',
  changes: { name: { before: 'Old', after: 'New' }, status: { after: 'available' } },
  actor: { id: 'u1', name: 'Bambang Sukasno', email: 'b@x.id', role: 'admin' },
  office_id: 'o1', office_name: 'KP Test', created_at: '2026-06-24T08:30:00Z'
}

// Fake t() implementing the summary key contract used by toSummary().
const tMock = (key: string, params?: Record<string, unknown>) => {
  const p = params ?? {}
  if (key === 'audit.summary.create') return `Membuat ${p.entity} ${p.id}`
  if (key === 'audit.summary.update') return `Mengubah ${p.count} field pada ${p.entity} ${p.id}`
  if (key === 'audit.summary.delete') return `Menghapus ${p.entity} ${p.id}`
  return key
}

describe('useAudit', () => {
  it('list builds the query from non-empty params and returns {rows,total}', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const res = await useAudit().list({ entity_type: 'assets', action: 'update', search: '', limit: 20, offset: 40 }, tMock)
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/audit?')
    expect(path).toContain('entity_type=assets')
    expect(path).toContain('action=update')
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=40')
    expect(path).not.toContain('search=') // empty search omitted
    expect(res.total).toBe(1)
  })

  it('wires actor_id into the query when present', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    await useAudit().list({ actor_id: 'u1', limit: 20, offset: 0 }, tMock)
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('actor_id=u1')
  })

  it('omits actor_id from the query when absent', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    await useAudit().list({ limit: 20, offset: 0 }, tMock)
    const path = request.mock.calls[0][0] as string
    expect(path).not.toContain('actor_id=')
  })

  it('maps the API row to a display AuditRow', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const { rows } = await useAudit().list({ limit: 20, offset: 0 }, tMock)
    const r = rows[0]
    expect(r).toMatchObject({
      id: 'a1', actor: 'Bambang Sukasno', actor_email: 'b@x.id', initials: 'BS',
      action: 'update', entity_type: 'assets', entity_id: 'e9', ip: '10.0.0.1',
      date: '2026-06-24', time: '08:30', role: 'admin', office_name: 'KP Test'
    })
    // changes → diff view
    expect(r.diff).toContainEqual({ field: 'name', before: 'Old', after: 'New', hasBefore: true, hasAfter: true, hasArrow: true })
    expect(r.diff).toContainEqual({ field: 'status', before: '', after: 'available', hasBefore: false, hasAfter: true, hasArrow: false })
  })

  it('maps role, office_name, and a derived localized summary', () => {
    const row = toRow({
      id: '1', entity_type: 'assets', entity_id: 'AST-001', action: 'update', ip: '', created_at: '2026-07-12T03:04:05Z',
      changes: { name: { before: 'A', after: 'B' }, status: { before: 'x', after: 'y' } },
      actor: { id: 'u1', name: 'Budi', email: 'b@x.id', role: 'admin' }, office_id: null, office_name: 'KP Test'
    }, tMock)
    expect(row.role).toBe('admin')
    expect(row.office_name).toBe('KP Test')
    expect(row.summary).toBe('Mengubah 2 field pada assets AST-001')
  })

  it('derives a create summary with no field count', () => {
    const row = toRow({
      id: '2', entity_type: 'users', entity_id: 'u5', action: 'create', ip: '', created_at: '2026-07-12T03:04:05Z',
      changes: null, actor: { id: 'u2', name: 'Siti', email: 's@x.id', role: 'staff' }, office_id: null, office_name: null
    }, tMock)
    expect(row.summary).toBe('Membuat users u5')
    expect(row.office_name).toBe('')
    expect(row.role).toBe('staff')
  })

  it('derives a delete summary', () => {
    const row = toRow({
      id: '3', entity_type: 'roles', entity_id: 'r3', action: 'delete', ip: '', created_at: '2026-07-12T03:04:05Z',
      changes: null, actor: null, office_id: null, office_name: null
    }, tMock)
    expect(row.summary).toBe('Menghapus roles r3')
    expect(row.role).toBe('')
  })
})
