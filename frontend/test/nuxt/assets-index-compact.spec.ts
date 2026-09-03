// @vitest-environment nuxt
// Page-level contract for the compact (mobile) catalog: how many fetches a
// single user action costs, and when a cached list may be restored.
//
// Every defect the code review found lived in this wiring layer rather than in
// the composables, precisely because the wiring had no tests. These pin it.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { ref } from 'vue'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { clearListStateCache, useListStateCache, currentDataEpoch, bumpDataEpoch } from '~/composables/useListStateCache'

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

let _handler: RequestHandler = () => {
  throw new Error('No handler set')
}

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => Promise.resolve(_handler(path, opts))
  })
}))

const compact = ref(true)
mockNuxtImport('useIsCompact', () => () => compact)

// eslint-disable-next-line import/first
import CatalogPage from '~/pages/assets/index.vue'

const TOTAL = 45

function makeAssetsPage(offset: number, limit: number) {
  const count = Math.max(0, Math.min(limit, TOTAL - offset))
  const data = Array.from({ length: count }, (_, i) => ({
    id: `a${offset + i}`,
    asset_tag: `TAG-${offset + i}`,
    name: `Aset ${offset + i}`,
    category_id: 'c1',
    office_id: 'o1',
    brand_id: null,
    model_id: null,
    status: 'available',
    asset_class: 'tangible',
    purchase_date: '2026-01-01'
  }))
  return { data, total: TOTAL, limit, offset }
}

const assetCalls: string[] = []

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path.startsWith('/assets')) {
    assetCalls.push(path)
    const q = new URLSearchParams(path.split('?')[1] ?? '')
    return makeAssetsPage(Number(q.get('offset') ?? '0'), Number(q.get('limit') ?? '10'))
  }
  if (path.startsWith('/categories/tree')) return { data: [{ id: 'c1', name: 'Elektronik' }] }
  if (path.startsWith('/brands') || path.startsWith('/models')) return { data: [], total: 0, limit: 20, offset: 0 }
  if (path.startsWith('/offices')) return { data: [{ id: 'o1', name: 'Kantor Pusat' }], total: 1, limit: 100, offset: 0 }
  throw new Error(`Unhandled request: ${path} ${JSON.stringify(opts)}`)
}

enableAutoUnmount(afterEach)

beforeEach(() => {
  assetCalls.length = 0
  _handler = defaultHandler
  compact.value = true
  clearListStateCache()
  history.replaceState({}, '')
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
})

async function mountCatalog() {
  const wrapper = await mountSuspended(CatalogPage)
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  return wrapper
}

type Vm = { fStatus: string, page: number, search: string }

describe('Catalog (compact) — fetch accounting', () => {
  it('loads exactly once on mount', async () => {
    await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(assetCalls[0]).toContain('offset=0')
  })

  it('issues exactly ONE fetch when a filter changes', async () => {
    const w = await mountCatalog()
    assetCalls.length = 0
    ;(w.vm as unknown as Vm).fStatus = 'available'
    await flushPromises()
    await w.vm.$nextTick()
    await flushPromises()
    expect(assetCalls).toHaveLength(1)
  })

  // Regression: the breakpoint watcher used to set `page = 1` AND call load(),
  // so crossing the breakpoint from page >= 2 fired two identical requests.
  it('issues exactly ONE fetch when the breakpoint is crossed from a later page', async () => {
    compact.value = false
    const w = await mountCatalog()
    ;(w.vm as unknown as Vm).page = 3
    await flushPromises()
    await w.vm.$nextTick()
    await flushPromises()

    assetCalls.length = 0
    compact.value = true
    await flushPromises()
    await w.vm.$nextTick()
    await flushPromises()

    expect(assetCalls).toHaveLength(1)
    expect(assetCalls[0]).toContain('offset=0')
  })

  it('issues exactly ONE fetch when the breakpoint is crossed from page 1', async () => {
    const w = await mountCatalog()
    assetCalls.length = 0
    compact.value = false
    await flushPromises()
    await w.vm.$nextTick()
    await flushPromises()
    expect(assetCalls).toHaveLength(1)
  })
})

// ---------------------------------------------------------------------------
// Regression guards for the two critical review findings.
// ---------------------------------------------------------------------------
describe('Catalog (compact) — cached list is only restored on a real return trip', () => {
  // epoch|id|role|search|status|category|office|class — see filterSignature.
  const sig = () => `${currentDataEpoch()}|1|r1||__all__|__all__||__all__`

  function seedSnapshot(leftTo: string) {
    useListStateCache('/assets').save({
      rows: [{ id: 'cached', asset_tag: 'CACHED', name: 'Dari cache', category_id: 'c1', office_id: 'o1', status: 'available' }],
      total: 99,
      scrollTop: 500,
      signature: sig(),
      leftTo
    })
  }

  it('restores without a fetch when returning from the route it was saved against', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({ forward: '/assets/TAG-0' }, '')

    const w = await mountCatalog()
    expect(assetCalls).toHaveLength(0)
    expect(w.html()).toContain('Dari cache')
  })

  // Saving an edit redirects to the list with a plain push. Restoring there
  // would show the user their own change had not happened.
  it('reloads instead of restoring on a fresh push (no forward entry)', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({}, '')

    const w = await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(w.html()).not.toContain('Dari cache')
  })

  it('reloads when returning from an unrelated route', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({ forward: '/dashboard' }, '')

    await mountCatalog()
    expect(assetCalls).toHaveLength(1)
  })

  // Snapshots are only ever written by the compact layout; replaying one into
  // the paged layout shows accumulated rows under a "1-10 of N" paginator.
  it('never restores into the regular layout', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({ forward: '/assets/TAG-0' }, '')
    compact.value = false

    const w = await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(w.html()).not.toContain('Dari cache')
  })

  it('reloads when the filters changed while away', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({ forward: '/assets/TAG-0' }, '')
    // A different signature than the page will compute on mount.
    useListStateCache('/assets').save({
      rows: [{ id: 'cached', asset_tag: 'CACHED', name: 'Dari cache', category_id: 'c1', office_id: 'o1', status: 'available' }],
      total: 99,
      scrollTop: 500,
      signature: `${currentDataEpoch()}|1|r1|kursi|available|__all__||__all__`,
      leftTo: '/assets/TAG-0'
    })

    const w = await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(w.html()).not.toContain('Dari cache')
  })

  // The rows in a snapshot were already filtered by the backend for the
  // caller's role and data scope. Binding identity into the signature makes the
  // cache safe by construction, not merely because every session path happens
  // to clear it.
  it('reloads when a different user is signed in', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({ forward: '/assets/TAG-0' }, '')
    useAuthStore().setSession(
      'tok',
      { id: '2', name: 'Staf', email: 'staf@test.com', role_id: 'r1', role_name: 'Staf', office_id: null },
      ['*']
    )

    const w = await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(w.html()).not.toContain('Dari cache')
  })

  it('reloads when the same user comes back under a different role', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({ forward: '/assets/TAG-0' }, '')
    useAuthStore().setSession(
      'tok',
      { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r9', role_name: 'Auditor', office_id: null },
      ['*']
    )

    const w = await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(w.html()).not.toContain('Dari cache')
  })

  // Going Back from a detail screen where the user just changed something —
  // a check-out, a maintenance request, an edit — is a legitimate return trip,
  // so the direction guard alone would happily restore rows that no longer
  // describe the data. Any successful write bumps the epoch and invalidates it.
  it('reloads when anything was mutated while away', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({ forward: '/assets/TAG-0' }, '')
    bumpDataEpoch()

    const w = await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(w.html()).not.toContain('Dari cache')
  })

  it('does not replay a snapshot onto a later visit', async () => {
    seedSnapshot('/assets/TAG-0')
    history.replaceState({}, '')
    await mountCatalog()
    expect(assetCalls).toHaveLength(1)

    // Second visit, this time a genuine return — the snapshot is long gone.
    assetCalls.length = 0
    history.replaceState({ forward: '/assets/TAG-0' }, '')
    const again = await mountCatalog()
    expect(assetCalls).toHaveLength(1)
    expect(again.html()).not.toContain('Dari cache')
  })
})

describe('Catalog (compact) — session end clears the cache', () => {
  it('drops every snapshot when the auth store is cleared', async () => {
    useListStateCache('/assets').save({
      rows: [{ id: 'x' }],
      total: 1,
      scrollTop: 10,
      signature: 'sig',
      leftTo: '/assets/TAG-0'
    })
    useAuthStore().clear()
    expect(useListStateCache('/assets').restore('sig', '/assets/TAG-0')).toBeNull()
  })
})
