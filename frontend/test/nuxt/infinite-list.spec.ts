// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { h } from 'vue'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import InfiniteList from '~/components/InfiniteList.vue'

// ---------------------------------------------------------------------------
// IntersectionObserver stub
//
// happy-dom ships no IntersectionObserver, and even where one exists it never
// fires without layout. The stub records every observer instance so a test can
// drive an intersection by hand.
// ---------------------------------------------------------------------------
interface FakeObserver {
  callback: IntersectionObserverCallback
  targets: Element[]
  disconnected: boolean
  fire: (isIntersecting: boolean) => void
}

let observers: FakeObserver[] = []

class FakeIntersectionObserver {
  callback: IntersectionObserverCallback
  targets: Element[] = []
  disconnected = false

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    observers.push(this as unknown as FakeObserver)
  }

  observe(el: Element) {
    this.targets.push(el)
  }

  unobserve() {}

  disconnect() {
    this.disconnected = true
  }

  takeRecords() {
    return []
  }

  fire(isIntersecting: boolean) {
    this.callback(
      this.targets.map(target => ({ target, isIntersecting }) as IntersectionObserverEntry),
      this as unknown as IntersectionObserver
    )
  }
}

/** The observer the component is currently using (the most recent live one). */
function liveObserver(): FakeObserver {
  const live = observers.filter(o => !o.disconnected)
  return live[live.length - 1]!
}

beforeEach(() => {
  observers = []
  vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

// ---------------------------------------------------------------------------

function items(n: number) {
  return Array.from({ length: n }, (_, i) => ({ id: i + 1 }))
}

type Props = InstanceType<typeof InfiniteList>['$props']

function mountList(props: Partial<Props>) {
  return mountSuspended(InfiniteList, {
    props: props as Props,
    slots: {
      item: ({ item }: { item: { id: number } }) =>
        h('div', { 'class': 'stub-item', 'data-id': item?.id }, `row ${item?.id}`)
    } as never
  })
}

describe('InfiniteList — rendering branch', () => {
  it('renders every item in the DOM below the threshold', async () => {
    const w = await mountList({ items: items(50), threshold: 200 })
    expect(w.find('[data-testid="infinite-list-plain"]').exists()).toBe(true)
    expect(w.find('[data-testid="infinite-list-virtual"]').exists()).toBe(false)
    expect(w.findAll('.stub-item')).toHaveLength(50)
  })

  it('renders an empty list without crashing', async () => {
    const w = await mountList({ items: [] })
    expect(w.findAll('.stub-item')).toHaveLength(0)
  })

  it('switches to the windowed branch at the threshold', async () => {
    const w = await mountList({ items: items(200), threshold: 200 })
    expect(w.find('[data-testid="infinite-list-virtual"]').exists()).toBe(true)
    expect(w.find('[data-testid="infinite-list-plain"]').exists()).toBe(false)
  })

  // NOTE: the test runtime has no layout, so the virtualizer resolves a very
  // small window. This asserts the branch keeps the DOM far below the item
  // count; that the *right* items land in the *right* place is verified in a
  // real browser (see the plan's checkpoint B).
  it('keeps the DOM far smaller than the item count once windowed', async () => {
    const w = await mountList({ items: items(500), threshold: 200 })
    expect(w.findAll('.stub-item').length).toBeLessThan(500)
  })

  it('stays on the plain branch one item short of the threshold', async () => {
    const w = await mountList({ items: items(199), threshold: 200 })
    expect(w.find('[data-testid="infinite-list-plain"]').exists()).toBe(true)
  })

  it('hands the slot both the item and its index', async () => {
    const w = await mountList({ items: items(3) })
    expect(w.text()).toContain('row 1')
    expect(w.text()).toContain('row 3')
  })
})

describe('InfiniteList — sentinel', () => {
  it('renders a sentinel and observes it', async () => {
    const w = await mountList({ items: items(10) })
    expect(w.find('[data-testid="infinite-list-sentinel"]').exists()).toBe(true)
    expect(liveObserver().targets).toHaveLength(1)
  })

  it('emits load-more when the sentinel comes into view', async () => {
    const w = await mountList({ items: items(10) })
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('does not emit when the sentinel leaves the viewport', async () => {
    const w = await mountList({ items: items(10) })
    liveObserver().fire(false)
    expect(w.emitted('load-more')).toBeUndefined()
  })

  it('does not emit while an append is already running', async () => {
    const w = await mountList({ items: items(10), loadingMore: true })
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toBeUndefined()
  })

  it('does not emit once the list is done', async () => {
    const w = await mountList({ items: items(10), done: true })
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toBeUndefined()
  })

  it('does not emit while an error is unresolved', async () => {
    const w = await mountList({ items: items(10), error: true })
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toBeUndefined()
  })

  it('re-arms the observer after an append finishes, so a short page cannot stall', async () => {
    const w = await mountList({ items: items(10), loadingMore: true })
    const before = observers.filter(o => !o.disconnected).length
    await w.setProps({ loadingMore: false })
    await flushPromises()
    // The old observer was disconnected and a fresh one now watches the sentinel.
    expect(observers.length).toBeGreaterThan(before)
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('disconnects its observer on unmount', async () => {
    const w = await mountList({ items: items(10) })
    const o = liveObserver()
    w.unmount()
    expect(o.disconnected).toBe(true)
  })
})

describe('InfiniteList — status region', () => {
  it('is a polite live region', async () => {
    const w = await mountList({ items: items(10) })
    const status = w.find('[data-testid="infinite-list-status"]')
    expect(status.attributes('role')).toBe('status')
    expect(status.attributes('aria-live')).toBe('polite')
  })

  it('shows the loading indicator while appending', async () => {
    const w = await mountList({ items: items(10), loadingMore: true })
    expect(w.find('[data-testid="infinite-list-loading"]').exists()).toBe(true)
    expect(w.find('[data-testid="infinite-list-end"]').exists()).toBe(false)
  })

  it('shows the end-of-list marker when everything is loaded', async () => {
    const w = await mountList({ items: items(10), done: true })
    const end = w.find('[data-testid="infinite-list-end"]')
    expect(end.exists()).toBe(true)
    expect(end.text()).not.toContain('common.')
  })

  it('hides the end-of-list marker for an empty list', async () => {
    const w = await mountList({ items: [], done: true })
    expect(w.find('[data-testid="infinite-list-end"]').exists()).toBe(false)
  })

  it('shows the error message with a retry control', async () => {
    const w = await mountList({ items: items(10), error: true })
    expect(w.find('[data-testid="infinite-list-error"]').exists()).toBe(true)
    expect(w.find('[data-testid="infinite-list-retry"]').exists()).toBe(true)
  })

  it('emits retry when the retry control is pressed', async () => {
    const w = await mountList({ items: items(10), error: true })
    await w.find('[data-testid="infinite-list-retry"]').trigger('click')
    expect(w.emitted('retry')).toHaveLength(1)
  })

  it('prefers the loading indicator over the error message', async () => {
    const w = await mountList({ items: items(10), loadingMore: true, error: true })
    expect(w.find('[data-testid="infinite-list-loading"]').exists()).toBe(true)
    expect(w.find('[data-testid="infinite-list-error"]').exists()).toBe(false)
  })

  it('shows nothing in the status region during a plain idle state', async () => {
    const w = await mountList({ items: items(10) })
    expect(w.find('[data-testid="infinite-list-status"]').text()).toBe('')
  })
})
