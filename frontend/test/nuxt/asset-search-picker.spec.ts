// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import type { Asset } from '~/types'

// ---------------------------------------------------------------------------
// Stub useAssets() directly (not via useApiClient) — the picker calls
// useAssets().list({ search, status, limit: 20 }) once per status in its
// `statuses` prop, in parallel.
// ---------------------------------------------------------------------------

const listMock = vi.fn()

vi.mock('~/composables/api/useAssets', () => ({
  useAssets: () => ({ list: listMock })
}))

// eslint-disable-next-line import/first
import AssetSearchPicker from '~/components/AssetSearchPicker.vue'

const ASSET_A: Asset = {
  id: 'a1', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440',
  category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible'
}
const ASSET_B: Asset = {
  id: 'a2', asset_tag: 'JKT01-ELK-2026-00002', name: 'Proyektor Epson EB-X51',
  category_id: 'c1', office_id: 'o2', status: 'available', asset_class: 'tangible'
}

function pageOf(assets: Asset[]) {
  return { data: assets, total: assets.length, limit: 20, offset: 0 }
}

enableAutoUnmount(afterEach)

beforeEach(() => {
  listMock.mockReset()
  listMock.mockResolvedValue(pageOf([]))
})

async function mountPicker(props: Partial<InstanceType<typeof AssetSearchPicker>['$props']> = {}) {
  const wrapper = await mountSuspended(AssetSearchPicker, {
    props: {
      statuses: ['available'],
      placeholder: 'Cari nama / kode aset…',
      ...props
    }
  })
  await flushPromises()
  return wrapper
}

function input(wrapper: Awaited<ReturnType<typeof mountPicker>>) {
  return wrapper.find('[data-testid="asset-picker-input"]')
}

function items(wrapper: Awaited<ReturnType<typeof mountPicker>>) {
  return wrapper.findAll('[data-testid="asset-picker-item"]')
}

// Selection tests need to click a rendered result row *after* the debounced
// search settles. They deliberately wait out the real 300ms debounce instead
// of using fake timers: @vue/test-utils' `trigger()` stamps a synthetic
// `_vts` from the real `Date.now()` to work around Vue's own-event guard
// (https://github.com/vuejs/test-utils/issues/1854) — but a listener that was
// attached to the DOM while fake timers had already fast-forwarded time
// ends up with an `attached` timestamp *ahead* of the real clock once fake
// timers are torn down, so the very next `trigger('click')` gets silently
// swallowed by that same guard. Real timers (a real wait) sidestep the bug.
async function mountWithResults(assets: Asset[], props: Partial<InstanceType<typeof AssetSearchPicker>['$props']> = {}) {
  listMock.mockResolvedValue(pageOf(assets))
  const wrapper = await mountPicker(props)
  await input(wrapper).setValue('Laptop')
  await new Promise(resolve => setTimeout(resolve, 350))
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('AssetSearchPicker — base rendering', () => {
  it('renders the input with the given placeholder', async () => {
    const wrapper = await mountPicker({ placeholder: 'Cari aset…' })
    expect(input(wrapper).attributes('placeholder')).toBe('Cari aset…')
  })

  it('renders the hint when provided', async () => {
    const wrapper = await mountPicker({ hint: 'Aset berstatus "Dipinjam" harus di-check-in dulu.' })
    expect(wrapper.text()).toContain('Aset berstatus "Dipinjam" harus di-check-in dulu.')
  })

  it('renders no hint text when omitted', async () => {
    const wrapper = await mountPicker()
    expect(wrapper.find('[data-testid="asset-picker-hint"]').exists()).toBe(false)
  })

  it('disables the input when the disabled prop is set', async () => {
    const wrapper = await mountPicker({ disabled: true })
    expect(input(wrapper).attributes('disabled')).toBeDefined()
  })

  it('does not show a dropdown before the user types anything', async () => {
    const wrapper = await mountPicker()
    expect(items(wrapper)).toHaveLength(0)
  })
})

describe('AssetSearchPicker — debounced search', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('does not call useAssets().list immediately on keystroke', async () => {
    const wrapper = await mountPicker()
    vi.useFakeTimers()

    await input(wrapper).setValue('Laptop')
    await wrapper.vm.$nextTick()

    expect(listMock).not.toHaveBeenCalled()
  })

  it('calls list() ~300ms after the last keystroke, once per status', async () => {
    listMock.mockResolvedValue(pageOf([ASSET_A]))
    const wrapper = await mountPicker({ statuses: ['available', 'assigned'] })
    vi.useFakeTimers()

    await input(wrapper).setValue('Laptop')
    await wrapper.vm.$nextTick()
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(listMock).toHaveBeenCalledTimes(2)
    expect(listMock).toHaveBeenCalledWith({ search: 'Laptop', status: 'available', limit: 20 })
    expect(listMock).toHaveBeenCalledWith({ search: 'Laptop', status: 'assigned', limit: 20 })
  })

  it('resets the debounce timer on rapid keystrokes — only the final value is searched', async () => {
    listMock.mockResolvedValue(pageOf([ASSET_A]))
    const wrapper = await mountPicker()
    vi.useFakeTimers()

    await input(wrapper).setValue('Lap')
    await vi.advanceTimersByTimeAsync(150)
    await input(wrapper).setValue('Laptop')
    await vi.advanceTimersByTimeAsync(150)
    expect(listMock).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(150)
    await flushPromises()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(listMock).toHaveBeenCalledWith({ search: 'Laptop', status: 'available', limit: 20 })
  })
})

describe('AssetSearchPicker — results', () => {
  it('renders a green-dot row per result with name + tag · office', async () => {
    listMock.mockResolvedValue(pageOf([ASSET_A]))
    const wrapper = await mountPicker({ officeNames: new Map([['o1', 'Kantor Pusat']]) })
    vi.useFakeTimers()
    await input(wrapper).setValue('Laptop')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await wrapper.vm.$nextTick()
    vi.useRealTimers()

    const rows = items(wrapper)
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('Laptop Dell Latitude 5440')
    expect(rows[0]!.text()).toContain('JKT01-ELK-2026-00001')
    expect(rows[0]!.text()).toContain('Kantor Pusat')
  })

  it('falls back to an em dash when the office id has no entry in officeNames', async () => {
    listMock.mockResolvedValue(pageOf([ASSET_A]))
    const wrapper = await mountPicker()
    vi.useFakeTimers()
    await input(wrapper).setValue('Laptop')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()

    expect(items(wrapper)[0]!.text()).toContain('—')
  })

  it('merges and de-dupes results from multiple statuses by asset id', async () => {
    listMock.mockImplementation(({ status }: { status: string }) =>
      Promise.resolve(pageOf(status === 'available' ? [ASSET_A, ASSET_B] : [ASSET_A])))
    const wrapper = await mountPicker({ statuses: ['available', 'assigned'] })
    vi.useFakeTimers()
    await input(wrapper).setValue('a')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await wrapper.vm.$nextTick()
    vi.useRealTimers()

    expect(items(wrapper)).toHaveLength(2)
  })

  it('shows the empty state when the search resolves with no results', async () => {
    listMock.mockResolvedValue(pageOf([]))
    const wrapper = await mountPicker()
    vi.useFakeTimers()
    await input(wrapper).setValue('Nonexistent')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await wrapper.vm.$nextTick()
    vi.useRealTimers()

    expect(wrapper.text()).toContain('Tidak ada data')
    expect(items(wrapper)).toHaveLength(0)
  })

  it('discards a late-resolving stale response once a newer search has already resolved', async () => {
    let resolveFirst!: (v: unknown) => void
    let callCount = 0
    listMock.mockImplementation(() => {
      callCount++
      if (callCount === 1) {
        return new Promise((resolve) => {
          resolveFirst = resolve
        })
      }
      return Promise.resolve(pageOf([ASSET_B]))
    })
    const wrapper = await mountPicker()
    vi.useFakeTimers()

    await input(wrapper).setValue('Laptop')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.vm.$nextTick()

    await input(wrapper).setValue('Proyektor')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(items(wrapper)).toHaveLength(1)
    expect(items(wrapper)[0]!.text()).toContain('Proyektor Epson EB-X51')

    resolveFirst(pageOf([ASSET_A]))
    await flushPromises()
    await wrapper.vm.$nextTick()
    vi.useRealTimers()

    expect(items(wrapper)).toHaveLength(1)
    expect(items(wrapper)[0]!.text()).toContain('Proyektor Epson EB-X51')
  })
})

describe('AssetSearchPicker — selection', () => {
  it('emits select with the full Asset object and fills the input on click', async () => {
    const wrapper = await mountWithResults([ASSET_A])

    await items(wrapper)[0]!.trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('select')).toBeTruthy()
    expect(wrapper.emitted('select')![0]).toEqual([ASSET_A])
    expect((input(wrapper).element as HTMLInputElement).value).toBe('Laptop Dell Latitude 5440')
  })

  it('closes the dropdown after a selection', async () => {
    const wrapper = await mountWithResults([ASSET_A])

    await items(wrapper)[0]!.trigger('click')
    await wrapper.vm.$nextTick()

    expect(items(wrapper)).toHaveLength(0)
  })

  it('does not re-search or reopen the dropdown after a selection (regression)', async () => {
    // Filling the input with the chosen name mutates `query`; the watcher
    // must NOT treat that programmatic write as a new user search — the bug
    // was a stray list() call ~300ms later that reopened the dropdown.
    const wrapper = await mountWithResults([ASSET_A])
    await items(wrapper)[0]!.trigger('click')
    await wrapper.vm.$nextTick()

    const callsAfterSelect = listMock.mock.calls.length
    await new Promise(resolve => setTimeout(resolve, 350))
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(listMock.mock.calls.length).toBe(callsAfterSelect)
    expect(listMock).not.toHaveBeenCalledWith(
      expect.objectContaining({ search: ASSET_A.name })
    )
    expect(items(wrapper)).toHaveLength(0)
  })

  it('typing again after a selection still searches normally (suppression is one-shot)', async () => {
    const wrapper = await mountWithResults([ASSET_A])
    await items(wrapper)[0]!.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(resolve => setTimeout(resolve, 350))
    await flushPromises()
    const callsAfterSelect = listMock.mock.calls.length

    listMock.mockResolvedValue(pageOf([ASSET_B]))
    await input(wrapper).setValue('Proyektor')
    await new Promise(resolve => setTimeout(resolve, 350))
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(listMock.mock.calls.length).toBeGreaterThan(callsAfterSelect)
    expect(listMock).toHaveBeenCalledWith({ search: 'Proyektor', status: 'available', limit: 20 })
    expect(items(wrapper)).toHaveLength(1)
    expect(items(wrapper)[0]!.text()).toContain('Proyektor Epson EB-X51')
  })
})

describe('AssetSearchPicker — disabled', () => {
  it('never calls list() while disabled, even after typing + debounce', async () => {
    const wrapper = await mountPicker({ disabled: true })
    vi.useFakeTimers()
    await input(wrapper).setValue('Laptop')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()

    expect(listMock).not.toHaveBeenCalled()
  })
})

describe('AssetSearchPicker — outside click', () => {
  it('closes the dropdown on an outside click', async () => {
    listMock.mockResolvedValue(pageOf([ASSET_A]))
    const wrapper = await mountPicker()
    vi.useFakeTimers()
    await input(wrapper).setValue('Laptop')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await wrapper.vm.$nextTick()
    vi.useRealTimers()

    expect(items(wrapper)).toHaveLength(1)

    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    await wrapper.vm.$nextTick()

    expect(items(wrapper)).toHaveLength(0)
  })
})
