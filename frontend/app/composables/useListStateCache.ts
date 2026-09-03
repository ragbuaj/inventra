export interface ListSnapshot<T = unknown> {
  rows: T[]
  total: number
  scrollTop: number
  /** Serialized filter state the rows were fetched under. */
  signature: string
  /** Route the user navigated to when this snapshot was taken. */
  leftTo: string
}

/**
 * Per-route snapshot of an infinite list, so returning from a detail screen
 * lands the user back where they were instead of at row one.
 *
 * Deliberately a module-scoped Map, not `sessionStorage`: the snapshots hold
 * operational bank data (asset rows, book values), which must not outlive the
 * tab or be readable from disk. The trade-off is that a full page reload
 * starts fresh, which is the behaviour we want anyway.
 *
 * A snapshot is only handed back when its `signature` still matches, so a
 * filter change between leaving and returning forces a clean reload.
 */
const cache = new Map<string, ListSnapshot>()

export function useListStateCache<T>(key: string) {
  function save(snapshot: ListSnapshot<T>): void {
    cache.set(key, snapshot as ListSnapshot)
  }

  /**
   * Hands back the snapshot only for a genuine return trip, and consumes it
   * either way. Two guards, both load-bearing:
   *
   * - `signature` must still match, otherwise the rows describe other filters.
   * - `cameBackFrom` must be the very route the user left to. Vue Router only
   *   populates `history.state.forward` when the current entry was reached by
   *   going *back*, so a fresh push to this list — the redirect after saving an
   *   edit, for instance — has no forward entry and correctly gets a reload
   *   instead of stale rows.
   *
   * Reading is one-shot: the entry is dropped whether or not it was returned,
   * so a snapshot can never be replayed onto a later, unrelated visit.
   */
  function restore(signature: string, cameBackFrom: string | null | undefined): ListSnapshot<T> | null {
    const snap = cache.get(key) as ListSnapshot<T> | undefined
    cache.delete(key)
    if (!snap) return null
    if (snap.signature !== signature) return null
    if (!cameBackFrom || cameBackFrom !== snap.leftTo) return null
    return snap
  }

  function drop(): void {
    cache.delete(key)
  }

  return { save, restore, drop }
}

/** Wipes every cached list. Called on logout so no rows survive the session. */
export function clearListStateCache(): void {
  cache.clear()
}
