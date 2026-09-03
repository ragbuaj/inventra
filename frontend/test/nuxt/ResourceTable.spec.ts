// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { h, ref } from 'vue'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import ResourceTable from '~/components/ResourceTable.vue'

// Infinite mode is compact-width only, so the breakpoint has to be drivable.
// It stays `false` for every pre-existing test, which is what those assert.
const compact = ref(false)
mockNuxtImport('useIsCompact', () => () => compact)

beforeEach(() => {
  compact.value = false
})

const columns = [
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'status', header: 'Status' }
]

const rows = [
  { name: 'Laptop A', status: 'available' },
  { name: 'Monitor B', status: 'assigned' }
]

describe('ResourceTable', () => {
  it('renders row data when rows are provided', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows, columns }
    })
    const html = wrapper.html()
    // Both row names should appear in the rendered output
    expect(html).toContain('Laptop A')
    expect(html).toContain('Monitor B')
  })

  it('renders custom slot content for a column cell', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows, columns },
      slots: {
        'status-cell': ({ row }: { row: Record<string, unknown> }) =>
          h('span', { class: 'custom-status' }, `STATUS:${row?.status ?? ''}`)
      }
    })
    const html = wrapper.html()
    expect(html).toContain('STATUS:')
  })

  it('renders EmptyState when rows is empty and not loading', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows: [], columns, loading: false }
    })
    // No row data from our fixture appears
    expect(wrapper.html()).not.toContain('Laptop A')
    // EmptyState renders the default common.noData i18n text (id locale: "Belum ada data")
    expect(wrapper.html()).toContain('Belum ada data')
  })

  it('renders a custom empty title when emptyTitle is provided', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows: [], columns, loading: false, emptyTitle: 'Tidak ada aset' }
    })
    expect(wrapper.html()).toContain('Tidak ada aset')
    // The default fallback text should NOT appear when a custom title is given
    expect(wrapper.html()).not.toContain('Belum ada data')
  })

  it('renders TableSkeleton when loading is true', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows: [], columns, loading: true }
    })
    const html = wrapper.html()
    // Row data should NOT appear while loading
    expect(html).not.toContain('Laptop A')
    // The EmptyState text must NOT appear — we are loading, not empty
    expect(html).not.toContain('Belum ada data')
    // TableSkeleton renders USkeleton elements with a stable animate-pulse class
    expect(wrapper.findAll('.animate-pulse').length).toBeGreaterThan(0)
  })

  it('does not render EmptyState when loading is true', async () => {
    const wrapperLoading = await mountSuspended(ResourceTable, {
      props: { rows: [], columns, loading: true }
    })
    const wrapperEmpty = await mountSuspended(ResourceTable, {
      props: { rows: [], columns, loading: false }
    })
    // HTML should differ — loading shows skeleton, empty shows EmptyState
    expect(wrapperLoading.html()).not.toBe(wrapperEmpty.html())
  })

  it('renders one icon-only (ellipsis) dropdown trigger per row when actions are provided', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: {
        rows,
        columns,
        actions: () => [
          { label: 'Ubah', icon: 'i-lucide-pencil', onSelect: () => {} },
          { label: 'Hapus', icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => {} }
        ]
      }
    })
    const triggers = wrapper.findAll('button[aria-haspopup="menu"]')
    // One dropdown trigger per data row…
    expect(triggers.length).toBe(rows.length)
    // …and the trigger is icon-only (no visible "Aksi" text label on the button)
    expect(triggers[0]!.text().trim()).toBe('')
    expect(triggers[0]!.attributes('aria-label')).toBe('Aksi')
  })

  it('opens the actions menu and invokes onSelect when an item is clicked', async () => {
    let edited = 0
    const wrapper = await mountSuspended(ResourceTable, {
      props: {
        rows: [rows[0]!],
        columns,
        actions: (row: Record<string, unknown>) => [
          { label: 'Ubah', icon: 'i-lucide-pencil', onSelect: () => { if (row.name === 'Laptop A') edited++ } }
        ]
      }
    })
    await wrapper.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(r => setTimeout(r, 0))
    // The menu item resolves the i18n-free label and fires the callback
    const item = document.querySelector('[role="menuitem"]') as HTMLElement | null
    expect(item?.textContent).toContain('Ubah')
    item?.click()
    expect(edited).toBe(1)
  })

  it('renders no dropdown trigger when actions returns an empty list (e.g. no permission)', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows, columns, actions: () => [] }
    })
    expect(wrapper.findAll('button[aria-haspopup="menu"]').length).toBe(0)
  })

  it('still supports a custom row-actions slot for bespoke cells', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows, columns },
      slots: {
        'row-actions': () => h('button', { class: 'legacy-action' }, 'Legacy')
      }
    })
    expect(wrapper.findAll('.legacy-action').length).toBe(rows.length)
  })

  const sortableColumns = [
    { accessorKey: 'name', header: 'Name', sortable: true },
    { accessorKey: 'status', header: 'Status' }
  ]

  it('renders a clickable sort control only for sortable columns', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows, columns: sortableColumns }
    })
    const headerButtons = wrapper.findAll('thead button')
    // Only the "name" column is sortable → exactly one header sort button
    expect(headerButtons.length).toBe(1)
    expect(headerButtons[0]!.text()).toContain('Name')
  })

  it('reorders rows and emits sorting state when a sort header is toggled', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows, columns: sortableColumns }
    })
    const sortBtn = wrapper.findAll('thead button')[0]!
    // First click → ascending (Laptop A before Monitor B)
    await sortBtn.trigger('click')
    let html = wrapper.html()
    expect(html.indexOf('Laptop A')).toBeLessThan(html.indexOf('Monitor B'))
    // Second click → descending (Monitor B before Laptop A)
    await sortBtn.trigger('click')
    html = wrapper.html()
    expect(html.indexOf('Monitor B')).toBeLessThan(html.indexOf('Laptop A'))

    const emitted = wrapper.emitted('update:sorting')
    expect(emitted).toBeTruthy()
    expect(emitted!.at(-1)).toEqual([[{ id: 'name', desc: true }]])
  })

  it('does not flash the skeleton on subsequent loads once data has arrived (filter/refetch)', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows: [], columns, loading: true }
    })
    // Initial load → skeleton
    expect(wrapper.findAll('.animate-pulse').length).toBeGreaterThan(0)
    // Data arrives
    await wrapper.setProps({ rows, loading: false })
    expect(wrapper.html()).toContain('Laptop A')
    // A subsequent fetch (e.g. applying a filter) keeps the table visible —
    // the rendered rows are NOT replaced by a skeleton.
    await wrapper.setProps({ loading: true })
    expect(wrapper.html()).toContain('Laptop A')
    expect(wrapper.html()).toContain('Monitor B')
  })

  it('opens a right-click context menu exposing the row actions', async () => {
    let deleted = 0
    const wrapper = await mountSuspended(ResourceTable, {
      props: {
        rows: [rows[0]!],
        columns,
        actions: () => [
          { label: 'Ubah', icon: 'i-lucide-pencil', onSelect: () => {} },
          { label: 'Hapus', icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => { deleted++ } }
        ]
      }
    })
    const tr = wrapper.find('tbody tr').element
    tr.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(r => setTimeout(r, 0))
    const items = Array.from(document.querySelectorAll('[role="menuitem"]')) as HTMLElement[]
    const labels = items.map(i => i.textContent?.trim())
    expect(labels).toContain('Ubah')
    expect(labels).toContain('Hapus')
    items.find(i => i.textContent?.includes('Hapus'))?.click()
    expect(deleted).toBe(1)
  })
})

// ---------------------------------------------------------------------------
// Infinite mode (compact width). happy-dom ships no IntersectionObserver, so
// the sentinel gets a stub the test can fire by hand.
// ---------------------------------------------------------------------------
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

function manyRows(n: number) {
  return Array.from({ length: n }, (_, i) => ({ name: `Aset ${i + 1}`, status: 'available' }))
}

describe('ResourceTable — infinite mode', () => {
  beforeEach(() => {
    observers = []
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('keeps page buttons at regular width even when infinite is set', async () => {
    compact.value = false
    const w = await mountSuspended(ResourceTable, {
      props: { rows, columns, total: 200, limit: 10, offset: 0, infinite: true }
    })
    expect(w.find('[data-testid="pagination-next"]').exists()).toBe(true)
    expect(w.find('[data-testid="resource-table-infinite-sentinel"]').exists()).toBe(false)
  })

  it('replaces page buttons with a sentinel at compact width', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows, columns, total: 200, limit: 10, offset: 0, infinite: true }
    })
    expect(w.find('[data-testid="pagination-next"]').exists()).toBe(false)
    expect(w.find('[data-testid="resource-table-infinite-sentinel"]').exists()).toBe(true)
  })

  it('keeps page buttons at compact width when infinite is not requested', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows, columns, total: 200, limit: 10, offset: 0 }
    })
    expect(w.find('[data-testid="pagination-next"]').exists()).toBe(true)
  })

  it('emits load-more when the sentinel comes into view', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows, columns, total: 200, limit: 10, offset: 0, infinite: true }
    })
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('does not auto-load while an append is running, is done, or has errored', async () => {
    compact.value = true
    for (const state of [{ loadingMore: true }, { done: true }, { error: true }]) {
      observers = []
      const w = await mountSuspended(ResourceTable, {
        props: { rows, columns, total: 200, limit: 10, offset: 0, infinite: true, ...state }
      })
      liveObserver().fire(true)
      expect(w.emitted('load-more')).toBeUndefined()
    }
  })

  it('stops auto-loading at the 300-row cap and offers an explicit control', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows: manyRows(300), columns, total: 900, limit: 10, offset: 0, infinite: true }
    })
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toBeUndefined()

    const button = w.find('[data-testid="resource-table-infinite-load-more"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('still auto-loads one row short of the cap', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows: manyRows(299), columns, total: 900, limit: 10, offset: 0, infinite: true }
    })
    expect(w.find('[data-testid="resource-table-infinite-load-more"]').exists()).toBe(false)
    liveObserver().fire(true)
    expect(w.emitted('load-more')).toHaveLength(1)
  })

  it('shows the end marker rather than the load-more button once done at the cap', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows: manyRows(300), columns, total: 300, limit: 10, offset: 0, infinite: true, done: true }
    })
    expect(w.find('[data-testid="resource-table-infinite-end"]').exists()).toBe(true)
    expect(w.find('[data-testid="resource-table-infinite-load-more"]').exists()).toBe(false)
  })

  it('surfaces an inline retry after a failed append', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows, columns, total: 200, limit: 10, offset: 0, infinite: true, error: true }
    })
    await w.find('[data-testid="resource-table-infinite-retry"]').trigger('click')
    expect(w.emitted('retry')).toHaveLength(1)
  })

  it('exposes the status region as a polite live region', async () => {
    compact.value = true
    const w = await mountSuspended(ResourceTable, {
      props: { rows, columns, total: 200, limit: 10, offset: 0, infinite: true }
    })
    const status = w.find('[data-testid="resource-table-infinite-status"]')
    expect(status.attributes('role')).toBe('status')
    expect(status.attributes('aria-live')).toBe('polite')
  })
})
