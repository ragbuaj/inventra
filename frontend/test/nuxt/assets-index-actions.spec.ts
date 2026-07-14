// @vitest-environment nuxt
// Task 7: verifies the assets/index.vue table row's RowActionsMenu wiring
// actually invokes navigateTo() with the correct target for each action
// (View → detail, Edit → edit form, Print label → label print, scoped to
// just that one row's asset tag).
//
// `navigateTo` is mocked here (unlike assets-index.spec.ts / assets-catalog.
// spec.ts) specifically to observe its call arguments. Mocking it is known to
// short-circuit @nuxtjs/i18n's initial locale-detection redirect (see the
// comment in assets-form.spec.ts), which leaves the page on the English
// fallback catalog — so this file locates menu items by icon class (locale-
// independent) rather than by translated label text; the Indonesian-label
// assertions live in assets-index.spec.ts, which deliberately leaves
// navigateTo unmocked.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

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

const { navigateToMock } = vi.hoisted(() => ({ navigateToMock: vi.fn() }))
mockNuxtImport('navigateTo', () => navigateToMock)

// eslint-disable-next-line import/first
import CatalogPage from '~/pages/assets/index.vue'

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
    purchase_date: '2026-01-12'
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
  navigateToMock.mockClear()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(CatalogPage)
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

// Nuxt UI renders the icon as `<span class="iconify i-lucide:eye ...">`
// (colon, not the dash used in the `icon="i-lucide-eye"` prop) — match on the
// lucide icon name suffix rather than the literal prop string.
function menuItemByIcon(iconProp: string): HTMLElement | undefined {
  const name = iconProp.replace('i-lucide-', '')
  return Array.from(document.querySelectorAll('[role="menuitem"]'))
    .find((el) => {
      const icon = el.querySelector('[data-slot="itemLeadingIcon"]')
      return icon?.className.includes(`i-lucide:${name}`)
    }) as HTMLElement | undefined
}

// Mocking navigateTo (see file header) short-circuits the i18n locale
// redirect, so the page stays on the 'en' fallback locale — localePath()
// therefore prefixes routes with /en (prefix_except_default: the default
// 'id' locale is unprefixed, every other locale is). Assert on the
// locale-independent path suffix rather than a hardcoded prefix, so this
// stays correct regardless of which locale the redirect happens to land on.
function calledWithPathEnding(suffix: string): boolean {
  return navigateToMock.mock.calls.some(([to]) => typeof to === 'string' && to.endsWith(suffix))
}

describe('Asset Catalog page — row action navigation targets', () => {
  it('the "view" kebab item (i-lucide-eye) navigates to the asset detail route', async () => {
    const wrapper = await mountAndWait()
    const row = wrapper.findAll('tbody tr')[0]!
    await row.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(r => setTimeout(r, 0))
    menuItemByIcon('i-lucide-eye')!.click()
    expect(calledWithPathEnding(`/assets/${TAG}`)).toBe(true)
  })

  it('the "edit" kebab item (i-lucide-pencil) navigates to the asset edit route', async () => {
    const wrapper = await mountAndWait()
    const row = wrapper.findAll('tbody tr')[0]!
    await row.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(r => setTimeout(r, 0))
    menuItemByIcon('i-lucide-pencil')!.click()
    expect(calledWithPathEnding(`/assets/${TAG}/edit`)).toBe(true)
  })

  it('the "print label" kebab item (i-lucide-printer) navigates to the label route scoped to just that asset tag', async () => {
    const wrapper = await mountAndWait()
    const row = wrapper.findAll('tbody tr')[0]!
    await row.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(r => setTimeout(r, 0))
    menuItemByIcon('i-lucide-printer')!.click()
    expect(calledWithPathEnding(`/assets/label?tags=${TAG}`)).toBe(true)
  })

  it('right-click "edit" navigates to the same edit route as the kebab menu', async () => {
    const wrapper = await mountAndWait()
    const tr = wrapper.findAll('tbody tr')[0]!.element
    tr.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(r => setTimeout(r, 0))
    menuItemByIcon('i-lucide-pencil')!.click()
    expect(calledWithPathEnding(`/assets/${TAG}/edit`)).toBe(true)
  })
})
