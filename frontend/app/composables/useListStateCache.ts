export interface ListSnapshot<T = unknown> {
  rows: T[]
  total: number
  scrollTop: number
  /** Serialized filter state the rows were fetched under. */
  signature: string
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
   * Returns the snapshot when it was taken under the same filters, otherwise
   * drops it and returns null. Reading is one-shot in the sense that a
   * mismatch clears the entry.
   */
  function restore(signature: string): ListSnapshot<T> | null {
    const snap = cache.get(key) as ListSnapshot<T> | undefined
    if (!snap) return null
    if (snap.signature !== signature) {
      cache.delete(key)
      return null
    }
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
