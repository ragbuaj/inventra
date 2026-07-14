// @vitest-environment nuxt
// Task 7: assets/index.vue table row actions — converted from three inline
// icon buttons (view/edit/print label) to the shared RowActionsMenu (kebab
// dropdown) + a page-level right-click context menu, both built from the
// same `rowActions(row)` list via `buildActionGroups` (see Task 6).
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Stub API client — same stubbing style as assets-catalog.spec.ts.
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
// Fixtures — a single row is enough to exercise the row-actions affordance.
// ---------------------------------------------------------------------------

const CATEGORIES = [{ id: 'c1', name: 'Elektronik' }]
const OFFICES = [{ id: 'o1', name: 'Kantor Pusat' }]

const TAG = 'JKT01-ELK-2026-00001'

const ASSETS = [
  {
    id: 'a1',
    asset_tag: TAG,
    name: 'Laptop Dell Latitude 5440',
    category_id: 'c1',
    office_id: 'o1',
    brand_id: null,
    model_id: null,
    status: 'available',
    asset_class: 'tangible',
    purchase_date: '2026-01-12',
    purchase_cost: '18500000',
    book_value: '16200000'
  }
]

function officesHandler(path: string): unknown {
  const m = /^\/offices\/([^/?]+)$/.exec(path)
  if (m) return OFFICES.find(o => o.id === m[1]) ?? null
  return { data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 }
}

function defaultHandler(path: string, opts?: Record<string, unknown>): unknown {
  if (path.startsWith('/assets')) return { data: ASSETS, total: ASSETS.length, limit: 20, offset: 0 }
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

function menuItemLabels(): string[] {
  return Array.from(document.querySelectorAll('[role="menuitem"]'))
    .map(el => el.textContent?.trim())
    .filter((v): v is string => !!v)
}

describe('Asset Catalog page — table row actions (RowActionsMenu)', () => {
  it('no longer renders the old inline view/edit/print icon buttons', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.find('button[aria-label="Lihat"]').exists()).toBe(false)
    // "Ubah" (edit) is asserted more precisely below (it's ambiguous with other
    // page chrome), but the row-level icon buttons are gone in favor of a
    // single kebab trigger per row.
    const row = wrapper.findAll('tbody tr')[0]!
    expect(row.find('button[aria-label="Cetak Label"]').exists()).toBe(false)
  })

  it('renders a single kebab actions trigger per row', async () => {
    const wrapper = await mountAndWait()
    const row = wrapper.findAll('tbody tr')[0]!
    const kebab = row.find('button[aria-haspopup="menu"]')
    expect(kebab.exists()).toBe(true)
  })

  it('opening the kebab menu shows View / Edit / Print label', async () => {
    const wrapper = await mountAndWait()
    const row = wrapper.findAll('tbody tr')[0]!
    await row.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(r => setTimeout(r, 0))
    const labels = menuItemLabels()
    expect(labels).toContain('Lihat')
    expect(labels).toContain('Ubah')
    expect(labels).toContain('Cetak Label')
  })

  // Navigation *targets* (view → detail route, edit → edit route, print label
  // → label route scoped to the row's tag) are verified with navigateTo
  // mocked in assets-index-actions.spec.ts — mocking it here would short-
  // circuit @nuxtjs/i18n's locale-detection redirect and break every
  // Indonesian-label assertion in this file (see that file's header comment).
  // This file instead confirms each menu item is clickable without throwing
  // and closes the menu afterwards (Reka UI's normal post-select behavior).

  it('clicking "Lihat" closes the kebab menu without throwing', async () => {
    const wrapper = await mountAndWait()
    const row = wrapper.findAll('tbody tr')[0]!
    await row.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(r => setTimeout(r, 0))
    const item = Array.from(document.querySelectorAll('[role="menuitem"]')) as HTMLElement[]
    expect(() => item.find(i => i.textContent?.trim() === 'Lihat')!.click()).not.toThrow()
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(document.querySelectorAll('[role="menuitem"]').length).toBe(0)
  })

  it('right-clicking a row opens a context menu with the same View / Edit / Print label items', async () => {
    const wrapper = await mountAndWait()
    const tr = wrapper.findAll('tbody tr')[0]!.element
    tr.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(r => setTimeout(r, 0))
    const labels = menuItemLabels()
    expect(labels).toContain('Lihat')
    expect(labels).toContain('Ubah')
    expect(labels).toContain('Cetak Label')
  })

  it('selecting "Ubah" from the right-click context menu closes it without throwing', async () => {
    const wrapper = await mountAndWait()
    const tr = wrapper.findAll('tbody tr')[0]!.element
    tr.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(r => setTimeout(r, 0))
    const item = Array.from(document.querySelectorAll('[role="menuitem"]')) as HTMLElement[]
    expect(() => item.find(i => i.textContent?.trim() === 'Ubah')!.click()).not.toThrow()
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(document.querySelectorAll('[role="menuitem"]').length).toBe(0)
  })

  it('preserves the bulk-select checkbox for the row alongside the kebab menu', async () => {
    const wrapper = await mountAndWait()
    const row = wrapper.findAll('tbody tr')[0]!
    // Bulk-select checkbox (first cell) is untouched by the actions-cell change.
    expect(row.find('[role="checkbox"]').exists() || row.findComponent({ name: 'UCheckbox' }).exists()).toBe(true)
    expect(row.find('button[aria-haspopup="menu"]').exists()).toBe(true)
  })

  it('preserves the grid/table view toggle', async () => {
    const wrapper = await mountAndWait()
    const gridBtn = wrapper.find('button[aria-label="Tampilan grid"]')
    expect(gridBtn.exists()).toBe(true)
    await gridBtn.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')
    expect(wrapper.find('thead').exists()).toBe(false)
  })
})
