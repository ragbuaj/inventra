// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import type { SearchGroup } from '~/types'

// ---------------------------------------------------------------------------
// Stub useGlobalSearch() directly — the palette's query watcher now debounces
// then calls search(q) against the real API (Task 6). Tests control what
// search() resolves to and assert the debounce/call behavior.
// ---------------------------------------------------------------------------

const searchMock = vi.fn<(q: string) => Promise<SearchGroup[]>>()

vi.mock('~/composables/api/useGlobalSearch', () => ({
  useGlobalSearch: () => ({ search: searchMock })
}))

// eslint-disable-next-line import/first
import CommandPalette from '~/components/CommandPalette.vue'
// eslint-disable-next-line import/first
import { useCommandPalette } from '~/composables/useCommandPalette'
// eslint-disable-next-line import/first
import { useAuthStore } from '~/stores/auth'

function admin() {
  useAuthStore().setSession('t', { id: '1', name: 'A', email: 'a@e.com', role_id: 'r', role_name: 'Superadmin', office_id: null }, ['*'])
}

function nonAdmin() {
  useAuthStore().setSession('t', { id: '2', name: 'B', email: 'b@e.com', role_id: 'r2', role_name: 'Viewer', office_id: null }, ['reports.read'])
}

function assetGroup(): SearchGroup {
  return {
    type: 'aset',
    labelKey: 'search.group.aset',
    total: 1,
    items: [{
      type: 'aset',
      title: 'Laptop Dell Latitude 5440',
      sub: 'JKT01-ELK-2026-00001',
      status: null,
      icon: 'i-lucide-package',
      to: '/assets/JKT01-ELK-2026-00001'
    }]
  }
}

function pengajuanGroup(): SearchGroup {
  return {
    type: 'pengajuan',
    labelKey: 'search.group.pengajuan',
    total: 1,
    items: [{
      type: 'pengajuan',
      title: 'Peminjaman · Kantor Pusat',
      sub: 'PJM-0001',
      status: 'pending',
      icon: 'i-lucide-check-square',
      to: '/approval'
    }]
  }
}

// Teleport sends the overlay to document.body, so query there rather than the wrapper.
function bodyText() {
  return document.body.textContent ?? ''
}

function bodyInput() {
  return document.body.querySelector('input')
}

let wrapper: VueWrapper | null = null

async function mount() {
  wrapper = await mountSuspended(CommandPalette)
  return wrapper
}

describe('CommandPalette', () => {
  beforeEach(() => {
    useAuthStore().clear()
    useCommandPalette().close()
    searchMock.mockReset()
    searchMock.mockImplementation(async (q: string) => q.includes('latitude') ? [assetGroup()] : [])
  })

  afterEach(() => {
    // Close + unmount so the teleported overlay is removed before the next test.
    useCommandPalette().close()
    wrapper?.unmount()
    wrapper = null
    // Safety net: a test that forgets to restore real timers (e.g. an
    // assertion throws before it gets there) must not bleed into the next.
    vi.useRealTimers()
  })

  it('renders nothing when closed', async () => {
    await mount()
    expect(bodyInput()).toBeNull()
  })

  it('shows the initial state with quick actions when opened', async () => {
    admin()
    await mount()
    useCommandPalette().open()
    await flushPromises()
    expect(bodyText()).toContain('Aksi Cepat')
    expect(bodyText()).toContain('Tambah Aset')
  })

  it('searches and shows grouped results', async () => {
    admin()
    await mount()
    useCommandPalette().open()
    await flushPromises()
    vi.useFakeTimers()
    const input = bodyInput()!
    input.value = 'latitude'
    input.dispatchEvent(new Event('input'))
    await vi.advanceTimersByTimeAsync(250)
    await flushPromises()
    vi.useRealTimers()
    expect(bodyText()).toContain('Aset')
    expect(bodyText()).toContain('Latitude')
  })

  it('shows the empty state when nothing matches', async () => {
    admin()
    await mount()
    useCommandPalette().open()
    await flushPromises()
    vi.useFakeTimers()
    const input = bodyInput()!
    input.value = 'zzzzz-nomatch'
    input.dispatchEvent(new Event('input'))
    await vi.advanceTimersByTimeAsync(250)
    await flushPromises()
    vi.useRealTimers()
    expect(bodyText()).toContain('Tidak ada hasil')
  })

  it('Esc keydown on the input closes the palette', async () => {
    admin()
    await mount()
    const cp = useCommandPalette()
    cp.open()
    await flushPromises()
    expect(bodyInput()).not.toBeNull()
    const input = bodyInput()!
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(cp.isOpen.value).toBe(false)
    expect(bodyInput()).toBeNull()
  })

  it('hides the Tambah Aset quick action for a user without masterdata.office.manage', async () => {
    nonAdmin()
    await mount()
    useCommandPalette().open()
    await flushPromises()
    // Quick Actions heading still shows, but the gated action does not
    expect(bodyText()).toContain('Aksi Cepat')
    expect(bodyText()).not.toContain('Tambah Aset')
    // Ungated actions remain visible
    expect(bodyText()).toContain('Buka Laporan')
  })

  it('shows the Tambah Aset quick action for an admin', async () => {
    admin()
    await mount()
    useCommandPalette().open()
    await flushPromises()
    expect(bodyText()).toContain('Tambah Aset')
  })

  it('clicking a recent-search entry fills the query input', async () => {
    admin()
    const cp = useCommandPalette()
    cp.pushRecent('Laptop Dell Latitude')
    await mount()
    cp.open()
    await flushPromises()
    // Find the recent row button by its text and click it
    const buttons = Array.from(document.body.querySelectorAll('button'))
    const recentBtn = buttons.find(b => (b.textContent ?? '').includes('Laptop Dell Latitude'))
    expect(recentBtn).toBeTruthy()
    recentBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    const input = bodyInput() as HTMLInputElement
    expect(input.value).toBe('Laptop Dell Latitude')
  })

  it('debounces: rapid typing triggers one search call', async () => {
    admin()
    await mount()
    useCommandPalette().open()
    await flushPromises()
    vi.useFakeTimers()
    const input = bodyInput()!

    input.value = 'lap'
    input.dispatchEvent(new Event('input'))
    await vi.advanceTimersByTimeAsync(100)
    input.value = 'lapt'
    input.dispatchEvent(new Event('input'))
    await vi.advanceTimersByTimeAsync(250)
    await flushPromises()
    vi.useRealTimers()

    expect(searchMock).toHaveBeenCalledTimes(1)
    expect(searchMock).toHaveBeenCalledWith('lapt')
  })

  it('renders request status badges with kind=approval', async () => {
    searchMock.mockImplementation(async () => [pengajuanGroup()])
    admin()
    await mount()
    useCommandPalette().open()
    await flushPromises()
    vi.useFakeTimers()
    const input = bodyInput()!
    input.value = 'pjm'
    input.dispatchEvent(new Event('input'))
    await vi.advanceTimersByTimeAsync(250)
    await flushPromises()
    vi.useRealTimers()

    expect(bodyText()).toContain('Pengajuan')
    expect(bodyText()).toContain('Menunggu')
    expect(bodyText()).not.toContain('pending')
  })
})
