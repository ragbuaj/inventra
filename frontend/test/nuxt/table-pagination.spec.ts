// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import TablePagination from '~/components/TablePagination.vue'

function pager(props: { total: number, limit: number, offset: number }) {
  return mountSuspended(TablePagination, { props })
}

// Numbered page buttons (excludes the prev/next chevron controls).
function pageButtons(wrapper: Awaited<ReturnType<typeof pager>>) {
  return wrapper.findAll('[data-testid="pagination-page"]')
}

describe('TablePagination', () => {
  it('shows every page when there are 3 or fewer', async () => {
    const w = await pager({ total: 25, limit: 10, offset: 0 }) // 3 pages
    expect(pageButtons(w).map(b => b.text())).toEqual(['1', '2', '3'])
  })

  it('caps the visible page buttons at 3 for many pages', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 0 }) // 20 pages
    expect(pageButtons(w)).toHaveLength(3)
  })

  it('keeps the window at the start on page 1', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 0 })
    expect(pageButtons(w).map(b => b.text())).toEqual(['1', '2', '3'])
  })

  it('centres the window on the current page', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 90 }) // page 10
    expect(pageButtons(w).map(b => b.text())).toEqual(['9', '10', '11'])
  })

  it('clamps the window at the last page', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 190 }) // page 20 (last)
    expect(pageButtons(w).map(b => b.text())).toEqual(['18', '19', '20'])
  })

  it('marks the current page with aria-current', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 90 })
    const current = pageButtons(w).find(b => b.text() === '10')
    expect(current?.attributes('aria-current')).toBe('page')
  })

  it('emits the new offset when a page button is clicked', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 0 })
    await pageButtons(w)[2]!.trigger('click') // page 3 → offset 20
    expect(w.emitted('update:offset')?.[0]).toEqual([20])
  })

  it('disables prev on the first page and next on the last', async () => {
    const first = await pager({ total: 200, limit: 10, offset: 0 })
    expect(first.find('[data-testid="pagination-prev"]').attributes('disabled')).toBeDefined()
    const last = await pager({ total: 200, limit: 10, offset: 190 })
    expect(last.find('[data-testid="pagination-next"]').attributes('disabled')).toBeDefined()
  })

  it('steps back one page via the prev control', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 90 }) // page 10
    await w.find('[data-testid="pagination-prev"]').trigger('click')
    expect(w.emitted('update:offset')?.[0]).toEqual([80]) // page 9
  })

  it('renders the showing-range summary', async () => {
    const w = await pager({ total: 200, limit: 10, offset: 0 })
    expect(w.text()).toContain('1')
    expect(w.text()).toContain('200')
  })

  it('shows a single page button when there is no data', async () => {
    const w = await pager({ total: 0, limit: 10, offset: 0 })
    expect(pageButtons(w).map(b => b.text())).toEqual(['1'])
  })
})
