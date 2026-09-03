import type { Ref, ComputedRef } from 'vue'

export interface PageArg {
  limit: number
  offset: number
}

export interface PageResult<T> {
  data: T[]
  total: number
}

export interface InfiniteRows<T> {
  rows: Ref<T[]>
  total: Ref<number>
  /** True only while the *first* page is in flight — drives the skeleton. */
  loading: Ref<boolean>
  /** True while an append is in flight — drives the inline footer spinner. */
  loadingMore: Ref<boolean>
  error: Ref<boolean>
  /** True once every row the server reports has been accumulated. */
  done: ComputedRef<boolean>
  /** Replaces the rows with one page starting at `offset` — the paged mode. */
  loadPage: (offset: number) => Promise<void>
  loadFirst: () => Promise<void>
  loadMore: () => Promise<void>
  retry: () => Promise<void>
  reset: () => void
  /** Seeds rows from a cached snapshot, skipping the network entirely. */
  hydrate: (rows: T[], total: number) => void
}

/**
 * Accumulates server-paginated rows for infinite scrolling.
 *
 * Knows nothing about rendering: hand it a `fetchPage` over the usual
 * `{ limit, offset }` contract and it manages accumulation, the two distinct
 * loading flags, end-of-list detection, and error recovery.
 *
 * Stale responses are dropped via a sequence counter, the same guard the
 * page-based list screens already use. Without it, a slow first request can
 * land after a filter change and repopulate the list with rows nobody asked
 * for any more.
 */
export function useInfiniteRows<T>(
  fetchPage: (arg: PageArg) => Promise<PageResult<T>>,
  opts: { limit?: number } = {}
): InfiniteRows<T> {
  const limit = opts.limit ?? 10

  const rows = ref([]) as Ref<T[]>
  const total = ref(0)
  const loading = ref(false)
  const loadingMore = ref(false)
  const error = ref(false)

  let seq = 0
  // The request that failed, so `retry` repeats exactly that page — in the
  // same mode. Replaying a failed *paged* load as an append would splice a
  // middle page onto the end of the list.
  let failed: { offset: number, append: boolean } | null = null

  const done = computed(() => rows.value.length >= total.value)

  async function fetchAt(offset: number, append: boolean): Promise<void> {
    const mine = ++seq
    if (append) loadingMore.value = true
    else loading.value = true
    error.value = false

    try {
      const res = await fetchPage({ limit, offset })
      if (mine !== seq) return
      rows.value = append ? [...rows.value, ...res.data] : res.data
      total.value = res.total
      failed = null
    } catch {
      if (mine !== seq) return
      error.value = true
      failed = { offset, append }
      // A failed first page has nothing worth keeping; a failed append keeps
      // the rows already on screen so the user doesn't lose their place.
      if (!append) {
        rows.value = []
        total.value = 0
      }
    } finally {
      if (mine === seq) {
        loading.value = false
        loadingMore.value = false
      }
    }
  }

  // Replace-mode load. The regular (paged) layout drives this with the offset
  // of whichever page the user clicked, so both layouts share one engine
  // instead of the page juggling two parallel data sources.
  function loadPage(offset: number): Promise<void> {
    return fetchAt(offset, false)
  }

  function loadFirst(): Promise<void> {
    return fetchAt(0, false)
  }

  function loadMore(): Promise<void> {
    if (loading.value || loadingMore.value || error.value || done.value) return Promise.resolve()
    return fetchAt(rows.value.length, true)
  }

  function retry(): Promise<void> {
    if (!error.value || !failed) return Promise.resolve()
    return fetchAt(failed.offset, failed.append)
  }

  function reset(): void {
    // Bump the sequence so anything already in flight is discarded on arrival.
    seq++
    rows.value = []
    total.value = 0
    loading.value = false
    loadingMore.value = false
    error.value = false
    failed = null
  }

  // Restoring a cached list must also invalidate anything still in flight,
  // otherwise a request started before the user navigated away can land on top
  // of the snapshot we just put back.
  function hydrate(seed: T[], seedTotal: number): void {
    seq++
    rows.value = [...seed]
    total.value = seedTotal
    loading.value = false
    loadingMore.value = false
    error.value = false
    failed = null
  }

  return { rows, total, loading, loadingMore, error, done, loadPage, loadFirst, loadMore, retry, reset, hydrate }
}
