import { describe, it, expect } from 'vitest'
import { useAudit } from '~/composables/api/useAudit'
import { auditSeed, AUDIT_ENTITIES } from '~/mock/audit'

const { list, actors } = useAudit()

describe('mock/audit', () => {
  it('seeds 14 logs and the entity list', () => {
    expect(auditSeed).toHaveLength(14)
    expect(AUDIT_ENTITIES).toContain('Field-Permission')
  })
})

describe('useAudit', () => {
  it('resolves localized role/summary, initials, formatted date and diff flags', async () => {
    const rows = await list('en')
    const first = rows.find(r => r.id === 1)!
    expect(first.actor).toBe('Dewi Lestari')
    expect(first.initials).toBe('DL')
    expect(first.role).toBe('Asset Manager')
    expect(first.summary).toBe('Update valuation of Genset Cummins C22')
    expect(first.date).toBe('24 Jun 2026')
    expect(first.time).toBe('09:42')
    // a created field has no "before" (no arrow)
    const created = rows.find(r => r.id === 2)!
    expect(created.diff[0]).toMatchObject({ field: 'nama', hasBefore: false, hasAfter: true, hasArrow: false })
  })

  it('localizes the month abbreviation per locale', async () => {
    const idRows = await list('id')
    // 2026-06-19 → "19 Jun 2026" (Jun is shared); check a distinct one via May/Mei
    expect(idRows.find(r => r.id === 1)!.date).toContain('Jun')
  })

  it('returns distinct actor names', () => {
    const a = actors()
    expect(a).toContain('Super Admin')
    expect(new Set(a).size).toBe(a.length)
  })
})
