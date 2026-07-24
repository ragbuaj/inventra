import { describe, it, expect } from 'vitest'
import { referenceResources } from '~/composables/api/referenceResources'

describe('referenceResources — legacy-parity Fase 4 masters', () => {
  it('includes the four new master resources', () => {
    const keys = referenceResources.map(r => r.key)
    expect(keys).toContain('office-classes')
    expect(keys).toContain('executor-divisions')
    expect(keys).toContain('companies')
    expect(keys).toContain('building-classifications')
  })

  it('the three flat masters have a single name field and an active toggle', () => {
    for (const key of ['office-classes', 'executor-divisions', 'companies'] as const) {
      const r = referenceResources.find(x => x.key === key)!
      expect(r, key).toBeTruthy()
      expect(r.hasActive).toBe(true)
      expect(r.fields.map(f => f.key)).toEqual(['name'])
    }
  })

  it('building-classifications has numeric min/max floor fields (min required, max optional)', () => {
    const bc = referenceResources.find(r => r.key === 'building-classifications')!
    expect(bc).toBeTruthy()
    const min = bc.fields.find(f => f.key === 'min_floors')!
    const max = bc.fields.find(f => f.key === 'max_floors')!
    expect(min.type).toBe('number')
    expect(min.required).toBe(true)
    expect(max.type).toBe('number')
    expect(max.required).toBeFalsy()
  })
})
