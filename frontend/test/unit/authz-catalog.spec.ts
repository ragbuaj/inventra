import { describe, it, expect } from 'vitest'
import { iconForGroup } from '~/constants/authzCatalog'

describe('iconForGroup', () => {
  it('maps known groups to icons', () => {
    expect(iconForGroup('Sistem')).toBe('i-lucide-shield')
    expect(iconForGroup('Aset')).toBe('i-lucide-box')
  })
  it('falls back for unknown groups', () => {
    expect(iconForGroup('Tak Dikenal')).toBe('i-lucide-key')
  })
})
