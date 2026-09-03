import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useListStateCache, clearListStateCache } from '~/composables/useListStateCache'

interface Row { id: number }

const SIG = 'q=|status=all'
const LEFT_TO = '/assets/YOG02KOM201900144'

function snapshot(over: Partial<{ rows: Row[], total: number, scrollTop: number, signature: string, leftTo: string }> = {}) {
  return {
    rows: [{ id: 1 }, { id: 2 }],
    total: 40,
    scrollTop: 820,
    signature: SIG,
    leftTo: LEFT_TO,
    ...over
  }
}

beforeEach(() => {
  clearListStateCache()
})

describe('useListStateCache — restoring a return trip', () => {
  it('returns null when nothing was ever saved', () => {
    const c = useListStateCache<Row>('/assets')
    expect(c.restore(SIG, LEFT_TO)).toBeNull()
  })

  it('restores a snapshot for a return from the route it was saved against', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    const got = c.restore(SIG, LEFT_TO)
    expect(got?.rows).toHaveLength(2)
    expect(got?.total).toBe(40)
    expect(got?.scrollTop).toBe(820)
  })

  it('keeps snapshots for different routes apart', () => {
    const assets = useListStateCache<Row>('/assets')
    const users = useListStateCache<Row>('/settings/users')
    assets.save(snapshot({ scrollTop: 100 }))
    users.save(snapshot({ scrollTop: 900 }))
    expect(assets.restore(SIG, LEFT_TO)?.scrollTop).toBe(100)
    expect(users.restore(SIG, LEFT_TO)?.scrollTop).toBe(900)
  })

  it('overwrites the previous snapshot for the same route', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot({ scrollTop: 100 }))
    c.save(snapshot({ scrollTop: 250 }))
    expect(c.restore(SIG, LEFT_TO)?.scrollTop).toBe(250)
  })

  it('drops an entry on demand', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    c.drop()
    expect(c.restore(SIG, LEFT_TO)).toBeNull()
  })
})

describe('useListStateCache — filter guard', () => {
  it('refuses to restore when the signature changed', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore('q=laptop|status=all', LEFT_TO)).toBeNull()
  })

  it('drops the entry after a signature mismatch, so a retry stays empty', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    c.restore('q=laptop|status=all', LEFT_TO)
    expect(c.restore(SIG, LEFT_TO)).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Regression guard.
//
// Without a direction guard, ANY mount of the list route consumed the
// snapshot. Saving an edit redirects to the list with a plain push, so the
// user's own change came back invisible: stale rows, no request. Only a
// genuine return from the route we left to may restore.
// ---------------------------------------------------------------------------
describe('useListStateCache — navigation-direction guard', () => {
  it('refuses to restore on a fresh push (no forward entry)', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore(SIG, null)).toBeNull()
  })

  it('refuses to restore on an undefined forward entry', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore(SIG, undefined)).toBeNull()
  })

  it('refuses to restore when returning from some other route', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore(SIG, '/dashboard')).toBeNull()
  })

  it('refuses to restore when returning from the edit screen of the same asset', () => {
    // Left to the detail screen, came back from the edit screen: the row very
    // likely changed, so a reload is the only correct answer.
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot({ leftTo: '/assets/TAG' }))
    expect(c.restore(SIG, '/assets/TAG/edit')).toBeNull()
  })

  it('consumes the snapshot even when it is refused', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore(SIG, null)).toBeNull()
    // A later, genuine return must not resurrect the discarded snapshot.
    expect(c.restore(SIG, LEFT_TO)).toBeNull()
  })

  it('consumes the snapshot after a successful restore', () => {
    const c = useListStateCache<Row>('/assets')
    c.save(snapshot())
    expect(c.restore(SIG, LEFT_TO)).not.toBeNull()
    expect(c.restore(SIG, LEFT_TO)).toBeNull()
  })
})

describe('useListStateCache — clearing', () => {
  it('clears every route at once', () => {
    const assets = useListStateCache<Row>('/assets')
    const users = useListStateCache<Row>('/settings/users')
    assets.save(snapshot())
    users.save(snapshot())
    clearListStateCache()
    expect(assets.restore(SIG, LEFT_TO)).toBeNull()
    expect(users.restore(SIG, LEFT_TO)).toBeNull()
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
    c.restore(SIG, LEFT_TO)
    c.drop()
    clearListStateCache()

    expect(sessionSet).not.toHaveBeenCalled()
    expect(localSet).not.toHaveBeenCalled()
    expect(idbOpen).not.toHaveBeenCalled()
  })
})
