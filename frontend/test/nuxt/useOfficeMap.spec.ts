// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { useOfficeMap } from '~/composables/api/useOfficeMap'

describe('useOfficeMap', () => {
  it('lists the 9 mockup offices with coordinates', async () => {
    const rows = await useOfficeMap().list()
    expect(rows).toHaveLength(9)
    expect(rows.every(r => typeof r.lat === 'number' && typeof r.lng === 'number')).toBe(true)
    expect(rows.map(r => r.jenis)).toContain('Pusat')
  })
})
