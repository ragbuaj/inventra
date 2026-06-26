// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { useCommandPalette } from '~/composables/useCommandPalette'

describe('useCommandPalette', () => {
  beforeEach(() => {
    localStorage.clear()
    const { close, recent } = useCommandPalette()
    close()
    recent.value = []
  })

  it('opens, closes, and toggles', () => {
    const p = useCommandPalette()
    expect(p.isOpen.value).toBe(false)
    p.open()
    expect(p.isOpen.value).toBe(true)
    p.toggle()
    expect(p.isOpen.value).toBe(false)
  })

  it('shares state across calls (singleton)', () => {
    useCommandPalette().open()
    expect(useCommandPalette().isOpen.value).toBe(true)
  })

  it('pushes recent searches, most-recent-first, de-duplicated, capped at 5', () => {
    const p = useCommandPalette()
    for (const q of ['a', 'b', 'c', 'd', 'e', 'f']) p.pushRecent(q)
    expect(p.recent.value).toEqual(['f', 'e', 'd', 'c', 'b'])
    p.pushRecent('e')
    expect(p.recent.value[0]).toBe('e')
    expect(p.recent.value.filter(x => x === 'e')).toHaveLength(1)
  })

  it('ignores blank recent entries', () => {
    const p = useCommandPalette()
    p.pushRecent('   ')
    expect(p.recent.value).toEqual([])
  })

  it('persists recent searches to localStorage', () => {
    const p = useCommandPalette()
    p.pushRecent('hello')
    const stored = JSON.parse(localStorage.getItem('inventra.search.recent') ?? '[]') as string[]
    expect(stored).toEqual(['hello'])
  })
})
