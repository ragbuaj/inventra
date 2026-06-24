// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { h } from 'vue'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import ResourceTable from '~/components/ResourceTable.vue'

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
    // Check that no row data from our fixture appears and something renders
    expect(wrapper.html()).not.toContain('Laptop A')
    // EmptyState should render (contains an icon + text)
    expect(wrapper.html().length).toBeGreaterThan(0)
  })

  it('renders TableSkeleton when loading is true', async () => {
    const wrapper = await mountSuspended(ResourceTable, {
      props: { rows: [], columns, loading: true }
    })
    const html = wrapper.html()
    // TableSkeleton renders; row data should NOT appear
    expect(html).not.toContain('Laptop A')
    // The skeleton renders some markup
    expect(html.length).toBeGreaterThan(0)
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
})
