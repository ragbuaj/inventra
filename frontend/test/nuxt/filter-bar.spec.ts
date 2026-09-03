// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { ref, h, nextTick } from 'vue'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import FilterBar from '~/components/FilterBar.vue'

// `useIsCompact` decides which branch renders, so every test drives it.
// The ref is shared with the component, which lets a test flip the breakpoint
// mid-mount the same way a real viewport resize would.
const compact = ref(false)
mockNuxtImport('useIsCompact', () => () => compact)

// A stand-in for the advanced controls a page passes through the slot.
function filtersSlot() {
  return h('select', { 'data-testid': 'stub-status-filter' }, [h('option', 'Tersedia')])
}
function trailingSlot() {
  return h('button', { 'data-testid': 'stub-view-toggle' }, 'grid')
}

type Props = InstanceType<typeof FilterBar>['$props']

function mountBar(props: Partial<Props> = {}, slots: Record<string, unknown> = {}) {
  return mountSuspended(FilterBar, {
    props: props as Props,
    slots: { filters: filtersSlot, trailing: trailingSlot, ...slots } as never
  })
}

beforeEach(() => {
  compact.value = false
})

describe('FilterBar — regular width', () => {
  it('renders the advanced controls inline', async () => {
    const w = await mountBar()
    expect(w.find('[data-testid="stub-status-filter"]').exists()).toBe(true)
  })

  it('does not render the compact filter toggle', async () => {
    const w = await mountBar()
    expect(w.find('[data-testid="filter-bar-toggle"]').exists()).toBe(false)
  })

  it('renders the trailing slot', async () => {
    const w = await mountBar()
    expect(w.find('[data-testid="stub-view-toggle"]').exists()).toBe(true)
  })

  it('shows the reset button only once an advanced filter is active', async () => {
    const none = await mountBar({ activeCount: 0 })
    expect(none.find('[data-testid="filter-bar-reset"]').exists()).toBe(false)

    const some = await mountBar({ activeCount: 2 })
    expect(some.find('[data-testid="filter-bar-reset"]').exists()).toBe(true)
  })

  it('honours showReset=false even with active filters', async () => {
    const w = await mountBar({ activeCount: 2, showReset: false })
    expect(w.find('[data-testid="filter-bar-reset"]').exists()).toBe(false)
  })

  it('emits reset when the reset button is pressed', async () => {
    const w = await mountBar({ activeCount: 1 })
    await w.find('[data-testid="filter-bar-reset"]').trigger('click')
    expect(w.emitted('reset')).toHaveLength(1)
  })
})

describe('FilterBar — search box', () => {
  it('renders the given search value', async () => {
    const w = await mountBar({ search: 'laptop' })
    const input = w.find('[data-testid="filter-bar-search"] input').exists()
      ? w.find('[data-testid="filter-bar-search"] input')
      : w.find('input')
    expect((input.element as HTMLInputElement).value).toBe('laptop')
  })

  it('emits update:search as the user types', async () => {
    const w = await mountBar()
    const input = w.find('input')
    await input.setValue('meja')
    expect(w.emitted('update:search')?.at(-1)).toEqual(['meja'])
  })

  it('falls back to the generic search placeholder when none is given', async () => {
    const w = await mountBar()
    const ph = w.find('input').attributes('placeholder')
    expect(ph).toBeTruthy()
    expect(ph).not.toContain('common.')
  })

  it('uses the supplied placeholder when given', async () => {
    const w = await mountBar({ searchPlaceholder: 'Cari aset…' })
    expect(w.find('input').attributes('placeholder')).toBe('Cari aset…')
  })
})

describe('FilterBar — compact width', () => {
  beforeEach(() => {
    compact.value = true
  })

  it('hides the advanced controls until the panel is opened', async () => {
    const w = await mountBar()
    expect(w.find('[data-testid="stub-status-filter"]').exists()).toBe(false)
  })

  it('still renders the search box and the trailing slot', async () => {
    const w = await mountBar()
    expect(w.find('input').exists()).toBe(true)
    expect(w.find('[data-testid="stub-view-toggle"]').exists()).toBe(true)
  })

  it('renders the filter toggle', async () => {
    const w = await mountBar()
    expect(w.find('[data-testid="filter-bar-toggle"]').exists()).toBe(true)
  })

  it('opens the panel and reveals the advanced controls', async () => {
    const w = await mountBar({ activeCount: 1 })
    await w.find('[data-testid="filter-bar-toggle"]').trigger('click')
    await flushPromises()
    // The slideover teleports its content, so assert against the document.
    expect(document.querySelector('[data-testid="filter-bar-panel"]')).not.toBeNull()
    expect(document.querySelector('[data-testid="stub-status-filter"]')).not.toBeNull()
  })

  it('reflects the open state on aria-expanded', async () => {
    const w = await mountBar()
    const toggle = w.find('[data-testid="filter-bar-toggle"]')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    await toggle.trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="filter-bar-toggle"]').attributes('aria-expanded')).toBe('true')
  })

  it('labels the toggle with the active count when filters are on', async () => {
    const w = await mountBar({ activeCount: 3 })
    const label = w.find('[data-testid="filter-bar-toggle"]').attributes('aria-label')
    expect(label).toContain('3')
    expect(label).not.toContain('common.')
  })

  it('labels the toggle without a count when no filter is active', async () => {
    const w = await mountBar({ activeCount: 0 })
    const label = w.find('[data-testid="filter-bar-toggle"]').attributes('aria-label')
    expect(label).toBeTruthy()
    expect(label).not.toContain('0')
    expect(label).not.toContain('common.')
  })

  it('shows the count badge only when a filter is active', async () => {
    const off = await mountBar({ activeCount: 0 })
    expect(off.text()).not.toContain('0')

    const on = await mountBar({ activeCount: 4 })
    expect(on.text()).toContain('4')
  })

  it('labels the panel footer button with the result count when total is given', async () => {
    const w = await mountBar({ total: 128 })
    await w.find('[data-testid="filter-bar-toggle"]').trigger('click')
    await flushPromises()
    const apply = document.querySelector('[data-testid="filter-bar-apply"]')
    expect(apply?.textContent).toContain('128')
  })

  it('falls back to a plain apply label when total is absent', async () => {
    const w = await mountBar()
    await w.find('[data-testid="filter-bar-toggle"]').trigger('click')
    await flushPromises()
    const apply = document.querySelector('[data-testid="filter-bar-apply"]')
    expect(apply?.textContent?.trim()).toBeTruthy()
    expect(apply?.textContent).not.toContain('common.')
  })

  it('emits reset from the panel footer', async () => {
    const w = await mountBar({ activeCount: 2 })
    await w.find('[data-testid="filter-bar-toggle"]').trigger('click')
    await flushPromises()
    const reset = document.querySelector('[data-testid="filter-bar-reset"]') as HTMLElement | null
    expect(reset).not.toBeNull()
    reset!.click()
    await flushPromises()
    expect(w.emitted('reset')).toHaveLength(1)
  })

  it('closes the panel when the viewport grows back to regular width', async () => {
    const w = await mountBar()
    await w.find('[data-testid="filter-bar-toggle"]').trigger('click')
    await flushPromises()
    expect(document.querySelector('[data-testid="filter-bar-panel"]')).not.toBeNull()

    compact.value = false
    await nextTick()
    await flushPromises()

    // Back inline, and the overlay is gone.
    expect(w.find('[data-testid="stub-status-filter"]').exists()).toBe(true)
    expect(document.querySelector('[data-testid="filter-bar-panel"]')).toBeNull()
  })

  it('keeps the search value across a breakpoint change', async () => {
    const w = await mountBar({ search: 'kursi' })
    compact.value = false
    await nextTick()
    await flushPromises()
    expect((w.find('input').element as HTMLInputElement).value).toBe('kursi')
  })
})

describe('FilterBar — testid prefix', () => {
  it('namespaces every testid with the supplied prefix', async () => {
    compact.value = true
    const w = await mountBar({ testid: 'users-filters', activeCount: 1 })
    expect(w.find('[data-testid="users-filters-search"]').exists()).toBe(true)
    expect(w.find('[data-testid="users-filters-toggle"]').exists()).toBe(true)
    expect(w.find('[data-testid="filter-bar-toggle"]').exists()).toBe(false)
  })
})
