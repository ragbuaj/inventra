import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useListStateCache, clearListStateCache } from '~/composables/useListStateCache'

interface Row { id: number }

function snapshot(over: Partial<{ rows: Row[], total: number, scrollTop: number, signature: string }> = {}) {
  return {
    rows: [{ id: 1 }, { id: 2 }],
    total: 40,
    scrollTop: 820,
    signature: 'q=|status=all',
    ...over
  }
}

beforeEach(() => {
  clearListStateCache()
})

describe('useListStateCache', () => {
  it('returns null when nothing was ever saved', () => {
    const c = useListStateCache<Row>('/assets')
    expect(c.restore('q=|status=all')).toBeNull()
  })

  it('restores a snapshot saved under the same signature', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    const got = c.restore('q=|status=all')
    expect(got?.rows).toHaveLength(2)
    expect(got?.total).toBe(40)
    expect(got?.scrollTop).toBe(820)
  })

  it('refuses to restore when the signature changed', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore('q=laptop|status=all')).toBeNull()
  })

  it('drops the entry after a signature mismatch, so a retry stays empty', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    c.restore('q=laptop|status=all')
    expect(c.restore('q=|status=all')).toBeNull()
  })

  it('keeps snapshots for different routes apart', () => {
    const assets = useListStateCache<Row>('/assets')
    const users = useListStateCache<Row>('/settings/users')
    assets.save(snapshot({ scrollTop: 100 }))
    users.save(snapshot({ scrollTop: 900 }))
    expect(assets.restore('q=|status=all')?.scrollTop).toBe(100)
    expect(users.restore('q=|status=all')?.scrollTop).toBe(900)
  })

  it('overwrites the previous snapshot for the same route', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot({ scrollTop: 100 }))
    c.save(snapshot({ scrollTop: 250 }))
    expect(c.restore('q=|status=all')?.scrollTop).toBe(250)
  })

  it('drops an entry on demand', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    c.drop()
    expect(c.restore('q=|status=all')).toBeNull()
  })

  it('survives repeated restores while the signature holds', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore('q=|status=all')).not.toBeNull()
    expect(c.restore('q=|status=all')).not.toBeNull()
  })

  it('clears every route at once', () => {
    const assets = useListStateCache<Row>('/assets')
    const users = useListStateCache<Row>('/settings/users')
    assets.save(snapshot())
    users.save(snapshot())
    clearListStateCache()
    expect(assets.restore('q=|status=all')).toBeNull()
    expect(users.restore('q=|status=all')).toBeNull()
  })
})

describe('useListStateCache — never touches persistent storage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('writes nothing to sessionStorage, localStorage or indexedDB', () => {
    const sessionSet = vi.fn()
    const localSet = vi.fn()
    const idbOpen = vi.fn()

    vi.stubGlobal('sessionStorage', { setItem: sessionSet, getItem: vi.fn(), removeItem: vi.fn() })
    vi.stubGlobal('localStorage', { setItem: localSet, getItem: vi.fn(), removeItem: vi.fn() })
    vi.stubGlobal('indexedDB', { open: idbOpen })

    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    c.restore('q=|status=all')
    c.drop()
    clearListStateCache()

    expect(sessionSet).not.toHaveBeenCalled()
    expect(localSet).not.toHaveBeenCalled()
    expect(idbOpen).not.toHaveBeenCalled()
  })
})
