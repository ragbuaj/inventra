import { describe, it, expect } from 'vitest'
import { referenceResources } from '~/composables/api/referenceResources'
import { referenceStores } from '~/mock/reference'

describe('reference resources', () => {
  it('declares all 11 reference resources', () => {
    expect(referenceResources).toHaveLength(11)
  })

  it('every descriptor has at least one field', () => {
    expect(referenceResources.every(r => r.fields.length >= 1)).toBe(true)
  })

  it('has a backing store for every declared resource', () => {
    for (const r of referenceResources) {
      expect(referenceStores[r.key]).toBeDefined()
    }
  })

  it('provinces store is seeded', () => {
    expect(referenceStores.provinces.all().length).toBeGreaterThan(0)
  })
})
