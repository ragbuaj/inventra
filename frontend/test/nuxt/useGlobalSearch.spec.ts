// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import type { SearchEntityType } from '~/types'
import { useGlobalSearch } from '~/composables/api/useGlobalSearch'

describe('useGlobalSearch', () => {
  const { search } = useGlobalSearch()

  it('returns no groups for an empty or whitespace query', async () => {
    expect(await search('')).toEqual([])
    expect(await search('   ')).toEqual([])
  })

  it('matches assets by name and tag, case-insensitively', async () => {
    const groups = await search('latitude')
    const aset = groups.find(g => g.type === 'aset')
    expect(aset).toBeTruthy()
    expect(aset!.items.some(i => i.title.includes('Latitude'))).toBe(true)
    expect(aset!.items[0]!.to).toMatch(/^\/assets\//)
    expect(aset!.items[0]!.icon).toBe('i-lucide-package')
    expect(aset!.items[0]!.type).toBe('aset')
  })

  it('groups results in fixed order and reports a total per group', async () => {
    const groups = await search('a')
    const order = groups.map(g => g.type)
    const expected = (['aset', 'pegawai', 'kantor', 'user', 'pengajuan'] as SearchEntityType[]).filter(t => order.includes(t))
    expect(order).toEqual(expected)
    for (const g of groups) {
      expect(g.total).toBeGreaterThanOrEqual(g.items.length)
      expect(g.items.length).toBeLessThanOrEqual(5)
    }
    // total must reflect the FULL match count, not the capped 5-item slice:
    // a high-cardinality query yields at least one group where total exceeds the slice.
    expect(groups.some(g => g.total > g.items.length)).toBe(true)
  })

  it('returns an empty array when nothing matches', async () => {
    expect(await search('zzzzzzz-no-match')).toEqual([])
  })
})
