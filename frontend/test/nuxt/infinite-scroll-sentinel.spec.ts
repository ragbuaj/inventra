// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import InfiniteScrollSentinel from '~/components/InfiniteScrollSentinel.vue'

// happy-dom ships no IntersectionObserver, and a real one never fires without
// layout — so the component gets a stub the test drives by hand.
interface FakeObserver { targets: Element[], disconnected: boolean, fire: (v: boolean) => void }
let observers: FakeObserver[] = []

class FakeIntersectionObserver {
  callback: IntersectionObserverCallback
  targets: Element[] = []
  disconnected = false

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    observers.push(this as unknown as FakeObserver)
  }

  observe(el: Element) { this.targets.push(el) }
  unobserve() {}
  disconnect() { this.disconnected = true }
  takeRecords() { return [] }
  fire(isIntersecting: boolean) {
    this.callback(
      this.targets.map(target => ({ target, isIntersecting }) as IntersectionObserverEntry),
      this as unknown as IntersectionObserver
    )
  }
}

function liveObserver(): FakeObserver {
  const live = observers.filter(o => !o.disconnected)
  return live[live.length - 1]!
}

type Props = InstanceType<typeof InfiniteScrollSentinel>['$props']

function mountSentinel(props: Partial<Props> = {}) {
  return mountSuspended(InfiniteScrollSentinel, { props: props as Props })
}

beforeEach(() => {
  observers = []
  vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('InfiniteScrollSentinel — asking for more', () => {
  it('emits load-more when the sentinel enters view', async () => {
    const w = await mountSentinel()
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('stays quiet when the sentinel leaves view', async () => {
    const w = await mountSentinel()
    liveObserver().fire(false)
    expect(w.emitted('load-more')).toBeUndefined()
  })

  it.each([
    ['an append is running', { loadingMore: true }],
    ['the list is done', { done: true }],
    ['an error is unresolved', { error: true }],
    ['loading is manual', { manual: true }]
  ])('refuses to ask while %s', async (_label, props) => {
    const w = await mountSentinel(props)
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toBeUndefined()
  })

  it('disconnects its observer on unmount', async () => {
    const w = await mountSentinel()
    const o = liveObserver()
    w.unmount()
    expect(o.disconnected).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Regression guard.
//
// IntersectionObserver only calls back when the intersection *changes*. On a
// first render `rows` and `total` are both 0, so `done` is true and the very
// first callback is refused. If nothing re-observes once `done` clears, the
// sentinel — already on screen — never fires again and the list stalls
// forever. This is the bug that shipped in the first draft and only showed up
// in a real browser (the table path stalled at exactly one page).
// ---------------------------------------------------------------------------
describe('InfiniteScrollSentinel — re-arming after a block clears', () => {
  it('asks again once done flips false, without the sentinel having to move', async () => {
    const w = await mountSentinel({ done: true })

    // First callback arrives while blocked and is refused.
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toBeUndefined()

    // The first page lands: total is now known, so the list is not done.
    await w.setProps({ done: false })
    await flushPromises()

    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('re-observes rather than reusing the observer that already reported', async () => {
    const w = await mountSentinel({ done: true })
    const first = liveObserver()
    await w.setProps({ done: false })
    await flushPromises()
    expect(first.disconnected).toBe(true)
    expect(liveObserver()).not.toBe(first)
  })

  it('re-arms after an append finishes, so a short page cannot stall', async () => {
    const w = await mountSentinel({ loadingMore: true })
    await w.setProps({ loadingMore: false })
    await flushPromises()
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('re-arms after an error is cleared', async () => {
    const w = await mountSentinel({ error: true })
    await w.setProps({ error: false })
    await flushPromises()
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('re-arms when the row cap is lifted', async () => {
    const w = await mountSentinel({ manual: true })
    await w.setProps({ manual: false })
    await flushPromises()
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('does not re-arm when a block is newly applied', async () => {
    const w = await mountSentinel()
    const first = liveObserver()
    await w.setProps({ done: true })
    await flushPromises()
    expect(first.disconnected).toBe(false)
  })
})

describe('InfiniteScrollSentinel — status region', () => {
  it('is a polite live region', async () => {
    const w = await mountSentinel()
    const status = w.find('[data-testid="infinite-status"]')
    expect(status.attributes('role')).toBe('status')
    expect(status.attributes('aria-live')).toBe('polite')
  })

  it('shows the loading indicator while appending', async () => {
    const w = await mountSentinel({ loadingMore: true })
    expect(w.find('[data-testid="infinite-loading"]').exists()).toBe(true)
  })

  it('shows the end marker only when there are items', async () => {
    const withItems = await mountSentinel({ done: true, hasItems: true })
    expect(withItems.find('[data-testid="infinite-end"]').exists()).toBe(true)

    const empty = await mountSentinel({ done: true, hasItems: false })
    expect(empty.find('[data-testid="infinite-end"]').exists()).toBe(false)
  })

  it('offers a retry on error and emits it', async () => {
    const w = await mountSentinel({ error: true })
    expect(w.find('[data-testid="infinite-error"]').exists()).toBe(true)
    await w.find('[data-testid="infinite-retry"]').trigger('click')
    expect(w.emitted('retry')).toHaveLength(1)
  })

  it('offers an explicit load-more control in manual mode', async () => {
    const w = await mountSentinel({ manual: true, hasItems: true })
    const button = w.find('[data-testid="infinite-load-more"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('prefers the end marker over the manual control once done', async () => {
    const w = await mountSentinel({ manual: true, done: true, hasItems: true })
    expect(w.find('[data-testid="infinite-end"]').exists()).toBe(true)
    expect(w.find('[data-testid="infinite-load-more"]').exists()).toBe(false)
  })

  it('namespaces its testids with the supplied prefix', async () => {
    const w = await mountSentinel({ testid: 'users-table', loadingMore: true })
    expect(w.find('[data-testid="users-table-sentinel"]').exists()).toBe(true)
    expect(w.find('[data-testid="users-table-loading"]').exists()).toBe(true)
  })
})
