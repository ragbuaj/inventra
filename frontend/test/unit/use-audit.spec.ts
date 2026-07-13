import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAudit, toRow, entityLabel } from '~/composables/api/useAudit'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

beforeEach(() => request.mockReset())

const apiRow = {
  id: 'a1', entity_type: 'assets', entity_id: 'e9', action: 'update', ip: '10.0.0.1',
  changes: { name: { before: 'Old', after: 'New' }, status: { after: 'available' } },
  actor: { id: 'u1', name: 'Bambang Sukasno', email: 'b@x.id', role: 'admin' },
  office_id: 'o1', office_name: 'KP Test', created_at: '2026-06-24T08:30:00Z'
}

// Fake settings.audit.entity.<key> catalog, mirroring i18n/locales/id.json.
const ENTITY_LABELS: Record<string, string> = { assets: 'Aset', users: 'User', roles: 'Peran' }

// Fake te() reporting which settings.audit.entity.<key> entries exist.
const teMock = (key: string) => {
  const m = key.match(/^settings\.audit\.entity\.(.+)$/)
  return !!m && m[1] in ENTITY_LABELS
}

// Fake t() implementing the summary key contract used by toSummary(), plus
// the settings.audit.entity.<key> lookups used by entityLabel().
const tMock = (key: string, params?: Record<string, unknown>) => {
  const p = params ?? {}
  if (key === 'audit.summary.create') return `Membuat ${p.entity} ${p.id}`
  if (key === 'audit.summary.update') return `Mengubah ${p.count} field pada ${p.entity} ${p.id}`
  if (key === 'audit.summary.delete') return `Menghapus ${p.entity} ${p.id}`
  const m = key.match(/^settings\.audit\.entity\.(.+)$/)
  if (m && m[1] in ENTITY_LABELS) return ENTITY_LABELS[m[1]]
  return key
}

describe('useAudit', () => {
  it('list builds the query from non-empty params and returns {rows,total}', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const res = await useAudit().list({ entity_type: 'assets', action: 'update', search: '', limit: 20, offset: 40 }, tMock, teMock)
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
    await useAudit().list({ actor_id: 'u1', limit: 20, offset: 0 }, tMock, teMock)
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('actor_id=u1')
  })

  it('omits actor_id from the query when absent', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    await useAudit().list({ limit: 20, offset: 0 }, tMock, teMock)
    const path = request.mock.calls[0][0] as string
    expect(path).not.toContain('actor_id=')
  })

  it('maps the API row to a display AuditRow', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const { rows } = await useAudit().list({ limit: 20, offset: 0 }, tMock, teMock)
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

  it('maps role, office_name, and a derived localized summary using the entity label, not the raw key', () => {
    const row = toRow({
      id: '1', entity_type: 'assets', entity_id: 'AST-001', action: 'update', ip: '', created_at: '2026-07-12T03:04:05Z',
      changes: { name: { before: 'A', after: 'B' }, status: { before: 'x', after: 'y' } },
      actor: { id: 'u1', name: 'Budi', email: 'b@x.id', role: 'admin' }, office_id: null, office_name: 'KP Test'
    }, tMock, teMock)
    expect(row.role).toBe('admin')
    expect(row.office_name).toBe('KP Test')
    // Localized: 'Aset' (settings.audit.entity.assets), not the raw 'assets' key.
    expect(row.summary).toBe('Mengubah 2 field pada Aset AST-001')
    expect(row.summary).not.toContain('assets')
  })

  it('derives a create summary with no field count, using the localized entity label', () => {
    const row = toRow({
      id: '2', entity_type: 'users', entity_id: 'u5', action: 'create', ip: '', created_at: '2026-07-12T03:04:05Z',
      changes: null, actor: { id: 'u2', name: 'Siti', email: 's@x.id', role: 'staff' }, office_id: null, office_name: null
    }, tMock, teMock)
    expect(row.summary).toBe('Membuat User u5')
    expect(row.office_name).toBe('')
    expect(row.role).toBe('staff')
  })

  it('derives a delete summary using the localized entity label', () => {
    const row = toRow({
      id: '3', entity_type: 'roles', entity_id: 'r3', action: 'delete', ip: '', created_at: '2026-07-12T03:04:05Z',
      changes: null, actor: null, office_id: null, office_name: null
    }, tMock, teMock)
    expect(row.summary).toBe('Menghapus Peran r3')
    expect(row.role).toBe('')
  })

  it('falls back to the raw entity_type key in the summary when no i18n label exists', () => {
    const row = toRow({
      id: '4', entity_type: 'some_unmapped_entity', entity_id: 'x1', action: 'delete', ip: '', created_at: '2026-07-12T03:04:05Z',
      changes: null, actor: null, office_id: null, office_name: null
    }, tMock, teMock)
    expect(row.summary).toBe('Menghapus some_unmapped_entity x1')
  })

  describe('entityLabel', () => {
    it('returns the localized label when an i18n entry exists', () => {
      expect(entityLabel('assets', tMock, teMock)).toBe('Aset')
    })

    it('falls back to the raw key when no i18n entry exists (never blank)', () => {
      expect(entityLabel('some_unmapped_entity', tMock, teMock)).toBe('some_unmapped_entity')
    })
  })
})
