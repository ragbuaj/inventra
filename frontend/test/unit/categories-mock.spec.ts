import { describe, it, expect } from 'vitest'
import { categorySeed, categoryStore, isBuildingGroup, formatThousands, parseThousands } from '~/mock/categories'
import { filterBy, paginate } from '~/mock/helpers'

describe('categories mock', () => {
  it('seeds more than one category including a parent/child pair', () => {
    expect(categorySeed.length).toBeGreaterThan(1)
    const child = categorySeed.find(c => c.parent_id)
    expect(child).toBeTruthy()
    expect(categorySeed.some(c => c.id === child!.parent_id)).toBe(true)
  })

  it('seeds at least one intangible and one inactive category', () => {
    expect(categorySeed.some(c => c.asset_class === 'intangible')).toBe(true)
    expect(categorySeed.some(c => !c.is_active)).toBe(true)
  })

  it('filterBy matches by name and code', () => {
    const all = categoryStore.all()
    expect(filterBy(all, { search: 'Kendaraan' }, ['name', 'code'])).toHaveLength(1)
    expect(filterBy(all, { search: 'ELK' }, ['name', 'code'])[0].code).toBe('ELK')
  })

  it('paginate slices to page size 7', () => {
    const page = paginate(categoryStore.all(), { limit: 7, offset: 0 })
    expect(page.data.length).toBeLessThanOrEqual(7)
    expect(page.total).toBe(categorySeed.length)
  })

  it('isBuildingGroup is true only for bangunan_* groups', () => {
    expect(isBuildingGroup('bangunan_permanen')).toBe(true)
    expect(isBuildingGroup('bangunan_non_permanen')).toBe(true)
    expect(isBuildingGroup('kelompok_1')).toBe(false)
    expect(isBuildingGroup(null)).toBe(false)
  })

  it('formatThousands / parseThousands round-trip with id-ID grouping', () => {
    expect(formatThousands('1000000')).toBe('1.000.000')
    expect(formatThousands('')).toBe('')
    expect(parseThousands('1.000.000')).toBe('1000000')
  })
})
