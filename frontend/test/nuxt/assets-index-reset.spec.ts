// @vitest-environment nuxt
// Task 12 (Tech-Debt Sweep #2, D4): resetFilters() on the asset catalog page
// used to mutate several filter refs AND `page` in the same tick, which fired
// both the filter-watcher and the `page`-watcher — two GET /assets calls for
// one click of "Reset filter" when starting from page >= 2. This spec pins
// down the single-fetch contract.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Stub API client — same shape as test/nuxt/assets-catalog.spec.ts.
// ---------------------------------------------------------------------------

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

let _handler: RequestHandler = () => {
  throw new Error('No handler set')
}

function setHandler(fn: RequestHandler) {
  _handler = fn
}

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => Promise.resolve(_handler(path, opts))
  })
}))

// eslint-disable-next-line import/first
import CatalogPage from '~/pages/assets/index.vue'

// ---------------------------------------------------------------------------
// Fixtures — 45 rows so page 2 exists (PAGE_SIZE = 20 → 3 pages).
// ---------------------------------------------------------------------------

const CATEGORIES = [{ id: 'c1', name: 'Elektronik' }]
const OFFICES = [{ id: 'o1', name: 'Kantor Pusat' }]

function makeAssetsPage(offset: number, total: number) {
  const count = Math.max(0, Math.min(20, total - offset))
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
  return { data, total, limit: 20, offset }
}

interface Call { path: string, opts?: Record<string, unknown> }

const assetCalls: Call[] = []

function officesHandler(path: string): unknown {
  const m = /^\/offices\/([^/?]+)$/.exec(path)
  if (m) return OFFICES.find(o => o.id === m[1]) ?? null
  return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
}

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path.startsWith('/assets')) {
    assetCalls.push({ path, opts })
    const q = new URLSearchParams(path.split('?')[1] ?? '')
    const offset = Number(q.get('offset') ?? '0')
    return makeAssetsPage(offset, 45)
  }
  if (path.startsWith('/categories/tree')) return { data: CATEGORIES }
  if (path.startsWith('/brands')) return { data: [], total: 0, limit: 20, offset: 0 }
  if (path.startsWith('/models')) return { data: [], total: 0, limit: 20, offset: 0 }
  if (path.startsWith('/offices')) return officesHandler(path)
  throw new Error(`Unhandled request: ${path} ${JSON.stringify(opts)}`)
}

// ---------------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------------

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

beforeEach(() => {
  assetCalls.length = 0
  setHandler(defaultHandler)
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(CatalogPage)
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

type Vm = { fStatus: string, page: number }

describe('Asset Catalog page — resetFilters single fetch', () => {
  it('clicking "Reset filter" from page 2 with a filter set issues exactly ONE GET /assets call and returns to page 1', async () => {
    const wrapper = await mountAndWait()

    // Set a filter (this itself legitimately triggers one refetch + resets
    // to page 1 — not what we're measuring).
    ;(wrapper.vm as unknown as Vm).fStatus = 'available'
    await wrapper.vm.$nextTick()
    await flushPromises()
    await wrapper.vm.$nextTick()

    // Navigate to page 2 within the filtered results — another legitimate
    // refetch, still not what we're measuring.
    const page2 = wrapper.findAll('button').find(b => b.text().trim() === '2')
    expect(page2).toBeDefined()
    await page2!.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect((wrapper.vm as unknown as Vm).page).toBe(2)
    expect((wrapper.vm as unknown as Vm).fStatus).toBe('available')

    // Now the actual measurement: click "Reset filter" from page 2 with a
    // filter active — the reported double-fetch scenario.
    const callsBeforeReset = assetCalls.length

    const resetBtn = wrapper.findAll('button').find(b => b.text().trim() === 'Reset filter')
    expect(resetBtn).toBeDefined()
    await resetBtn!.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()
    await flushPromises()
    await wrapper.vm.$nextTick()

    const callsAfterReset = assetCalls.length
    expect(callsAfterReset - callsBeforeReset).toBe(1)

    // Behavior preserved: filters cleared and back to page 1.
    expect((wrapper.vm as unknown as Vm).fStatus).toBe('__all__')
    expect((wrapper.vm as unknown as Vm).page).toBe(1)
    const lastCall = assetCalls[assetCalls.length - 1]!
    const q = new URLSearchParams(lastCall.path.split('?')[1] ?? '')
    expect(q.get('status')).toBeNull()
    expect(q.get('offset')).toBe('0')
  }, 15000)

  it('calling resetFilters from page 1 (nothing to page-reset) still issues exactly ONE call', async () => {
    const wrapper = await mountAndWait()

    ;(wrapper.vm as unknown as Vm).fStatus = 'available'
    await wrapper.vm.$nextTick()
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect((wrapper.vm as unknown as Vm).page).toBe(1)

    const callsBeforeReset = assetCalls.length

    const resetBtn = wrapper.findAll('button').find(b => b.text().trim() === 'Reset filter')
    expect(resetBtn).toBeDefined()
    await resetBtn!.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(assetCalls.length - callsBeforeReset).toBe(1)
    expect((wrapper.vm as unknown as Vm).fStatus).toBe('__all__')
    expect((wrapper.vm as unknown as Vm).page).toBe(1)
  }, 15000)
})
