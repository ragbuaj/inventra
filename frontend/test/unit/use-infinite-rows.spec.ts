import { describe, it, expect, vi } from 'vitest'
import { useInfiniteRows } from '~/composables/useInfiniteRows'
import type { PageArg, PageResult } from '~/composables/useInfiniteRows'

interface Row { id: number }

/** Rows [offset+1 .. offset+n], so a test can tell pages apart by id. */
function page(offset: number, n: number, total: number): PageResult<Row> {
  return { data: Array.from({ length: n }, (_, i) => ({ id: offset + i + 1 })), total }
}

/** A fetcher backed by a fixed dataset of `total` rows. */
function dataset(total: number) {
  const calls: PageArg[] = []
  const fetchPage = vi.fn(async (arg: PageArg) => {
    calls.push(arg)
    const n = Math.max(0, Math.min(arg.limit, total - arg.offset))
    return page(arg.offset, n, total)
  })
  return { fetchPage, calls }
}

/** A fetcher whose promises are resolved by the test, one at a time. */
function deferredSource() {
  const pending: { resolve: (r: PageResult<Row>) => void, reject: (e: unknown) => void, arg: PageArg }[] = []
  const fetchPage = (arg: PageArg) => new Promise<PageResult<Row>>((resolve, reject) => {
    pending.push({ resolve, reject, arg })
  })
  return { fetchPage, pending }
}

describe('useInfiniteRows — first page', () => {
  it('fills rows and total', async () => {
    const { fetchPage } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    expect(l.rows.value.map(r => r.id)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
    expect(l.total.value).toBe(25)
  })

  it('requests offset 0 with the configured limit', async () => {
    const { fetchPage, calls } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 5 })
    await l.loadFirst()
    expect(calls[0]).toEqual({ limit: 5, offset: 0 })
  })

  it('defaults the limit to 10', async () => {
    const { fetchPage, calls } = dataset(25)
    const l = useInfiniteRows(fetchPage)
    await l.loadFirst()
    expect(calls[0]!.limit).toBe(10)
  })

  it('raises loading, not loadingMore, while in flight', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const p = l.loadFirst()
    expect(l.loading.value).toBe(true)
    expect(l.loadingMore.value).toBe(false)
    pending[0]!.resolve(page(0, 10, 25))
    await p
    expect(l.loading.value).toBe(false)
  })

  it('replaces rows rather than appending when called again', async () => {
    const { fetchPage } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    expect(l.rows.value).toHaveLength(20)
    await l.loadFirst()
    expect(l.rows.value).toHaveLength(10)
  })
})

describe('useInfiniteRows — appending', () => {
  it('appends the next page behind the existing rows', async () => {
    const { fetchPage } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    expect(l.rows.value).toHaveLength(20)
    expect(l.rows.value[0]!.id).toBe(1)
    expect(l.rows.value[19]!.id).toBe(20)
  })

  it('requests the offset that follows the rows already held', async () => {
    const { fetchPage, calls } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    expect(calls[1]).toEqual({ limit: 10, offset: 10 })
  })

  it('raises loadingMore, not loading, while appending', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const first = l.loadFirst()
    pending[0]!.resolve(page(0, 10, 25))
    await first

    const more = l.loadMore()
    expect(l.loadingMore.value).toBe(true)
    expect(l.loading.value).toBe(false)
    pending[1]!.resolve(page(10, 10, 25))
    await more
    expect(l.loadingMore.value).toBe(false)
  })

  it('handles a final partial page', async () => {
    const { fetchPage } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    await l.loadMore()
    expect(l.rows.value).toHaveLength(25)
    expect(l.done.value).toBe(true)
  })
})

describe('useInfiniteRows — paged mode (loadPage)', () => {
  it('replaces the rows with the requested page', async () => {
    const { fetchPage } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadPage(30)
    expect(l.rows.value.map(r => r.id)).toEqual([31, 32, 33, 34, 35, 36, 37, 38, 39, 40])
  })

  it('requests exactly the offset it was given', async () => {
    const { fetchPage, calls } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadPage(50)
    expect(calls[0]).toEqual({ limit: 10, offset: 50 })
  })

  it('does not accumulate across page changes', async () => {
    const { fetchPage } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadPage(0)
    await l.loadPage(10)
    await l.loadPage(20)
    expect(l.rows.value).toHaveLength(10)
    expect(l.rows.value[0]!.id).toBe(21)
  })

  it('raises loading, not loadingMore', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const p = l.loadPage(30)
    expect(l.loading.value).toBe(true)
    expect(l.loadingMore.value).toBe(false)
    pending[0]!.resolve(page(30, 10, 100))
    await p
  })

  it('retries a failed middle page as a replace, never as an append', async () => {
    const { fetchPage, pending } = deferredSource()
    const spy = vi.fn(fetchPage)
    const l = useInfiniteRows(spy, { limit: 10 })

    const first = l.loadPage(0)
    pending[0]!.resolve(page(0, 10, 100))
    await first

    const bad = l.loadPage(30)
    pending[1]!.reject(new Error('offline'))
    await bad
    expect(l.error.value).toBe(true)

    const again = l.retry()
    expect(spy.mock.calls[2]![0]).toEqual({ limit: 10, offset: 30 })
    pending[2]!.resolve(page(30, 10, 100))
    await again

    // Replaced, not spliced onto the end.
    expect(l.rows.value).toHaveLength(10)
    expect(l.rows.value[0]!.id).toBe(31)
  })
})

describe('useInfiniteRows — refreshAll', () => {
  it('refetches every accumulated row in ONE request, keeping the length', async () => {
    const { fetchPage, calls } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    await l.loadMore()
    expect(l.rows.value).toHaveLength(30)

    await l.refreshAll()
    expect(calls.at(-1)).toEqual({ limit: 30, offset: 0 })
    expect(l.rows.value).toHaveLength(30)
  })

  it('never asks for fewer rows than one page', async () => {
    const { fetchPage, calls } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.refreshAll()
    expect(calls.at(-1)).toEqual({ limit: 10, offset: 0 })
  })

  it('replaces rather than appends', async () => {
    const { fetchPage } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    await l.refreshAll()
    expect(l.rows.value).toHaveLength(20)
    expect(l.rows.value[0]!.id).toBe(1)
  })

  it('shrinks the list when rows disappeared server-side', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const first = l.loadFirst()
    pending[0]!.resolve(page(0, 10, 25))
    await first

    const refresh = l.refreshAll()
    // One row was deleted elsewhere.
    pending[1]!.resolve(page(0, 9, 24))
    await refresh
    expect(l.rows.value).toHaveLength(9)
    expect(l.total.value).toBe(24)
  })
})

describe('useInfiniteRows — done', () => {
  it('is false while rows remain', async () => {
    const { fetchPage } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    expect(l.done.value).toBe(false)
  })

  it('is true once every row is accumulated', async () => {
    const { fetchPage } = dataset(20)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    expect(l.done.value).toBe(true)
  })

  it('is true for an empty result set', async () => {
    const { fetchPage } = dataset(0)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    expect(l.rows.value).toHaveLength(0)
    expect(l.done.value).toBe(true)
  })

  it('is true when a single page covers the whole set exactly', async () => {
    const { fetchPage } = dataset(10)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    expect(l.done.value).toBe(true)
  })
})

describe('useInfiniteRows — loadMore guards', () => {
  it('does nothing once done', async () => {
    const { fetchPage } = dataset(10)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    await l.loadMore()
    expect(fetchPage).toHaveBeenCalledTimes(1)
  })

  it('does nothing while the first page is still in flight', async () => {
    const { fetchPage, pending } = deferredSource()
    const spy = vi.fn(fetchPage)
    const l = useInfiniteRows(spy, { limit: 10 })
    const first = l.loadFirst()
    await l.loadMore()
    expect(spy).toHaveBeenCalledTimes(1)
    pending[0]!.resolve(page(0, 10, 25))
    await first
  })

  it('does nothing while an append is still in flight', async () => {
    const { fetchPage, pending } = deferredSource()
    const spy = vi.fn(fetchPage)
    const l = useInfiniteRows(spy, { limit: 10 })
    const first = l.loadFirst()
    pending[0]!.resolve(page(0, 10, 25))
    await first

    const more = l.loadMore()
    await l.loadMore()
    expect(spy).toHaveBeenCalledTimes(2)
    pending[1]!.resolve(page(10, 10, 25))
    await more
  })

  it('does nothing while an error is unresolved', async () => {
    const { fetchPage, pending } = deferredSource()
    const spy = vi.fn(fetchPage)
    const l = useInfiniteRows(spy, { limit: 10 })
    const first = l.loadFirst()
    pending[0]!.resolve(page(0, 10, 25))
    await first

    const more = l.loadMore()
    pending[1]!.reject(new Error('offline'))
    await more
    expect(l.error.value).toBe(true)

    await l.loadMore()
    expect(spy).toHaveBeenCalledTimes(2)
  })
})

describe('useInfiniteRows — errors', () => {
  it('empties the list when the first page fails', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const p = l.loadFirst()
    pending[0]!.reject(new Error('boom'))
    await p
    expect(l.error.value).toBe(true)
    expect(l.rows.value).toHaveLength(0)
    expect(l.loading.value).toBe(false)
  })

  it('keeps the visible rows when an append fails', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const first = l.loadFirst()
    pending[0]!.resolve(page(0, 10, 25))
    await first

    const more = l.loadMore()
    pending[1]!.reject(new Error('offline'))
    await more
    expect(l.error.value).toBe(true)
    expect(l.rows.value).toHaveLength(10)
    expect(l.loadingMore.value).toBe(false)
  })

  it('retries exactly the page that failed, not the whole list', async () => {
    const { fetchPage, pending } = deferredSource()
    const spy = vi.fn(fetchPage)
    const l = useInfiniteRows(spy, { limit: 10 })
    const first = l.loadFirst()
    pending[0]!.resolve(page(0, 10, 25))
    await first

    const more = l.loadMore()
    pending[1]!.reject(new Error('offline'))
    await more

    const again = l.retry()
    expect(spy.mock.calls[2]![0]).toEqual({ limit: 10, offset: 10 })
    pending[2]!.resolve(page(10, 10, 25))
    await again
    expect(l.error.value).toBe(false)
    expect(l.rows.value).toHaveLength(20)
  })

  it('retries the first page when that is what failed', async () => {
    const { fetchPage, pending } = deferredSource()
    const spy = vi.fn(fetchPage)
    const l = useInfiniteRows(spy, { limit: 10 })
    const p = l.loadFirst()
    pending[0]!.reject(new Error('boom'))
    await p

    const again = l.retry()
    expect(spy.mock.calls[1]![0]).toEqual({ limit: 10, offset: 0 })
    pending[1]!.resolve(page(0, 10, 25))
    await again
    expect(l.rows.value).toHaveLength(10)
  })

  it('does nothing when retry is called with no error pending', async () => {
    const { fetchPage } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.retry()
    expect(fetchPage).toHaveBeenCalledTimes(1)
  })

  it('clears the error flag on the next successful first page', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const p = l.loadFirst()
    pending[0]!.reject(new Error('boom'))
    await p
    expect(l.error.value).toBe(true)

    const p2 = l.loadFirst()
    pending[1]!.resolve(page(0, 10, 25))
    await p2
    expect(l.error.value).toBe(false)
  })
})

describe('useInfiniteRows — stale responses', () => {
  it('drops a first-page response superseded by a newer first page', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })

    const stale = l.loadFirst()
    const fresh = l.loadFirst()

    // The slow first request lands last, carrying rows nobody asked for.
    pending[1]!.resolve(page(0, 10, 25))
    await fresh
    pending[0]!.resolve({ data: [{ id: 999 }], total: 1 })
    await stale

    expect(l.rows.value.map(r => r.id)).not.toContain(999)
    expect(l.total.value).toBe(25)
  })

  it('drops a stale response that fails, leaving no error behind', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })

    const stale = l.loadFirst()
    const fresh = l.loadFirst()
    pending[1]!.resolve(page(0, 10, 25))
    await fresh
    pending[0]!.reject(new Error('too late'))
    await stale

    expect(l.error.value).toBe(false)
    expect(l.rows.value).toHaveLength(10)
  })

  it('drops an in-flight response after reset', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const p = l.loadFirst()
    l.reset()
    pending[0]!.resolve(page(0, 10, 25))
    await p
    expect(l.rows.value).toHaveLength(0)
    expect(l.total.value).toBe(0)
  })
})

describe('useInfiniteRows — hydrate', () => {
  it('seeds rows and total without touching the network', () => {
    const { fetchPage } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    l.hydrate([{ id: 7 }, { id: 8 }], 40)
    expect(l.rows.value.map(r => r.id)).toEqual([7, 8])
    expect(l.total.value).toBe(40)
    expect(fetchPage).not.toHaveBeenCalled()
  })

  it('leaves the list ready to append from where the snapshot ended', async () => {
    const { fetchPage, calls } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    l.hydrate(Array.from({ length: 20 }, (_, i) => ({ id: i + 1 })), 100)
    await l.loadMore()
    expect(calls[0]).toEqual({ limit: 10, offset: 20 })
    expect(l.rows.value).toHaveLength(30)
  })

  it('copies the seed so later mutations cannot reach back into the cache', () => {
    const { fetchPage } = dataset(100)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const seed = [{ id: 1 }]
    l.hydrate(seed, 40)
    seed.push({ id: 2 })
    expect(l.rows.value).toHaveLength(1)
  })

  it('clears a pending error', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const p = l.loadFirst()
    pending[0]!.reject(new Error('boom'))
    await p
    l.hydrate([{ id: 1 }], 10)
    expect(l.error.value).toBe(false)
  })

  it('discards a response that was already in flight', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const p = l.loadFirst()
    l.hydrate([{ id: 99 }], 40)
    pending[0]!.resolve(page(0, 10, 25))
    await p
    expect(l.rows.value.map(r => r.id)).toEqual([99])
    expect(l.total.value).toBe(40)
  })
})

describe('useInfiniteRows — reset', () => {
  it('clears every piece of state', async () => {
    const { fetchPage, pending } = deferredSource()
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    const first = l.loadFirst()
    pending[0]!.resolve(page(0, 10, 25))
    await first

    const more = l.loadMore()
    pending[1]!.reject(new Error('offline'))
    await more

    l.reset()
    expect(l.rows.value).toEqual([])
    expect(l.total.value).toBe(0)
    expect(l.loading.value).toBe(false)
    expect(l.loadingMore.value).toBe(false)
    expect(l.error.value).toBe(false)
  })

  it('lets a fresh load start from offset 0 after reset', async () => {
    const { fetchPage, calls } = dataset(25)
    const l = useInfiniteRows(fetchPage, { limit: 10 })
    await l.loadFirst()
    await l.loadMore()
    l.reset()
    await l.loadFirst()
    expect(calls.at(-1)).toEqual({ limit: 10, offset: 0 })
    expect(l.rows.value).toHaveLength(10)
  })
})
