// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import AsyncSearchPicker from '~/components/AsyncSearchPicker.vue'
import type { PickerItem } from '~/types'

const items: PickerItem[] = [
  { id: 'o1', label: 'Kantor Pusat', sublabel: 'KP-001' },
  { id: 'o2', label: 'Kanwil Jakarta', sublabel: 'KW-002' }
]

function picker(overrides: Record<string, unknown> = {}, mountOptions: Record<string, unknown> = {}) {
  return mountSuspended(AsyncSearchPicker, {
    props: {
      modelValue: null,
      searchFn: vi.fn(async (term: string) => items.filter(i => i.label.toLowerCase().includes(term.toLowerCase()))),
      placeholder: 'Cari kantor',
      testid: 'office',
      ...overrides
    },
    ...mountOptions
  })
}

// Vue attaches native `addEventListener` handlers and, under `vi.useFakeTimers()`, tracks a
// `_vts` guard timestamp to decide whether to invoke a handler for a given event (see the
// fake-timer workaround VTU's own `trigger()` applies, referenced in the click-test comment
// below). `wrapper.trigger()` doesn't hand back the dispatched Event, so tests that need to
// assert `defaultPrevented` dispatch a real KeyboardEvent directly and apply the same `_vts`
// workaround, keeping the handler-invocation behavior identical to `trigger()`.
function keydown(target: Element, key: string) {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
  ;(event as unknown as { _vts: number })._vts = Date.now() + 1
  target.dispatchEvent(event)
  return event
}

describe('AsyncSearchPicker', () => {
  it('renders the input with placeholder and testid', async () => {
    const w = await picker()
    const input = w.find('[data-testid="office-picker-input"]')
    expect(input.exists()).toBe(true)
    expect(input.attributes('placeholder')).toBe('Cari kantor')
  })

  it('searches (debounced) and lists results, then emits the id on select', async () => {
    vi.useFakeTimers()
    const w = await picker()
    await w.find('[data-testid="office-picker-input"]').setValue('kanwil')
    vi.advanceTimersByTime(300)
    await flushPromises()
    const rows = w.findAll('[data-testid="office-picker-item"]')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('Kanwil Jakarta')
    // vi.useRealTimers() runs *after* the click, not before: switching back to
    // real time before trigger('click') leaves the freshly-rendered <li>'s
    // listener "attached" timestamp (stamped under fake-advanced time) ahead
    // of the click's real timeStamp, so Vue's own-event guard silently
    // swallows the click — see the equivalent workaround/comment in
    // asset-search-picker.spec.ts (mountWithResults).
    await rows[0]!.trigger('click')
    vi.useRealTimers()
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['o2'])
  })

  it('shows a No Data empty state when search yields nothing', async () => {
    vi.useFakeTimers()
    const w = await picker({ searchFn: vi.fn(async () => []) })
    await w.find('[data-testid="office-picker-input"]').setValue('zzz')
    vi.advanceTimersByTime(300)
    await flushPromises()
    vi.useRealTimers()
    expect(w.find('[data-testid="office-picker-empty"]').exists()).toBe(true)
  })

  it('resolves and displays a preselected value via resolveFn', async () => {
    const resolveFn = vi.fn(async (id: string) => items.find(i => i.id === id) ?? null)
    const w = await picker({ modelValue: 'o1', resolveFn })
    await flushPromises()
    expect(resolveFn).toHaveBeenCalledWith('o1')
    expect((w.find('[data-testid="office-picker-input"]').element as HTMLInputElement).value).toBe('Kantor Pusat')
  })

  it('does not search or open when disabled', async () => {
    vi.useFakeTimers()
    const searchFn = vi.fn(async () => items)
    const w = await picker({ disabled: true, searchFn })
    await w.find('[data-testid="office-picker-input"]').setValue('kan')
    vi.advanceTimersByTime(300)
    await flushPromises()
    vi.useRealTimers()
    expect(searchFn).not.toHaveBeenCalled()
  })

  it('does not render a clear button when clearable is false (default) even with a value selected', async () => {
    const resolveFn = vi.fn(async (id: string) => items.find(i => i.id === id) ?? null)
    const w = await picker({ modelValue: 'o1', resolveFn })
    await flushPromises()
    expect(w.find('[data-testid="office-picker-clear"]').exists()).toBe(false)
  })

  it('renders a clear button when clearable is true and a value is selected, and clicking it clears the input and emits null', async () => {
    const resolveFn = vi.fn(async (id: string) => items.find(i => i.id === id) ?? null)
    const w = await picker({ modelValue: 'o1', resolveFn, clearable: true })
    await flushPromises()
    const clearBtn = w.find('[data-testid="office-picker-clear"]')
    expect(clearBtn.exists()).toBe(true)

    await clearBtn.trigger('click')
    await flushPromises()

    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([null])
    expect((w.find('[data-testid="office-picker-input"]').element as HTMLInputElement).value).toBe('')
  })

  it('does not render a clear button when clearable is true but no value is selected', async () => {
    const w = await picker({ clearable: true })
    await flushPromises()
    expect(w.find('[data-testid="office-picker-clear"]').exists()).toBe(false)
  })

  describe('a11y: combobox/listbox roles and keyboard navigation', () => {
    async function openWithResults(w: Awaited<ReturnType<typeof picker>>, term = 'ka') {
      await w.find('[data-testid="office-picker-input"]').setValue(term)
      vi.advanceTimersByTime(300)
      await flushPromises()
      await w.vm.$nextTick()
    }

    it('renders the input as a combobox with aria-expanded reflecting open state', async () => {
      const w = await picker()
      const input = w.find('[data-testid="office-picker-input"]')
      expect(input.attributes('role')).toBe('combobox')
      expect(input.attributes('aria-haspopup')).toBe('listbox')
      expect(input.attributes('aria-expanded')).toBe('false')

      vi.useFakeTimers()
      await openWithResults(w)
      expect(w.find('[data-testid="office-picker-input"]').attributes('aria-expanded')).toBe('true')
      vi.useRealTimers()
    })

    it('renders the listbox and options with proper roles and aria-selected', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)
      vi.useRealTimers()

      const list = w.find('ul')
      expect(list.attributes('role')).toBe('listbox')
      const options = w.findAll('[data-testid="office-picker-item"]')
      expect(options).toHaveLength(2)
      for (const opt of options) {
        expect(opt.attributes('role')).toBe('option')
        expect(opt.attributes('aria-selected')).toBe('false')
      }
    })

    it('ArrowDown moves activeIndex and sets aria-activedescendant on the input', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)

      await w.find('[data-testid="office-picker-input"]').trigger('keydown', { key: 'ArrowDown' })
      await w.vm.$nextTick()
      vi.useRealTimers()

      const input = w.find('[data-testid="office-picker-input"]')
      const options = w.findAll('[data-testid="office-picker-item"]')
      const activeId = options[0]!.attributes('id')
      expect(activeId).toBeTruthy()
      expect(input.attributes('aria-activedescendant')).toBe(activeId)
      expect(options[0]!.attributes('aria-selected')).toBe('true')
    })

    it('Enter selects the active option and emits update:modelValue', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)

      const input = w.find('[data-testid="office-picker-input"]')
      await input.trigger('keydown', { key: 'ArrowDown' })
      await input.trigger('keydown', { key: 'ArrowDown' })
      await input.trigger('keydown', { key: 'Enter' })
      await w.vm.$nextTick()
      vi.useRealTimers()

      expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['o2'])
    })

    it('Escape closes the popover', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)

      const input = w.find('[data-testid="office-picker-input"]')
      expect(input.attributes('aria-expanded')).toBe('true')
      await input.trigger('keydown', { key: 'Escape' })
      await w.vm.$nextTick()
      vi.useRealTimers()

      expect(w.find('[data-testid="office-picker-input"]').attributes('aria-expanded')).toBe('false')
      expect(w.find('ul').exists()).toBe(false)
    })

    it('does nothing destructive on ArrowDown/Enter when results are empty', async () => {
      vi.useFakeTimers()
      const w = await picker({ searchFn: vi.fn(async () => []) })
      await openWithResults(w, 'zzz')

      const input = w.find('[data-testid="office-picker-input"]')
      expect(input.attributes('aria-expanded')).toBe('true')
      await input.trigger('keydown', { key: 'ArrowDown' })
      await input.trigger('keydown', { key: 'Enter' })
      await w.vm.$nextTick()
      vi.useRealTimers()

      expect(w.emitted('update:modelValue')).toBeUndefined()
      expect(w.find('[data-testid="office-picker-input"]').attributes('aria-activedescendant')).toBeUndefined()
    })

    it('shows role=status on the loading skeleton', async () => {
      vi.useFakeTimers()
      const w = await picker({ searchFn: vi.fn(() => new Promise(() => {})) })
      await w.find('[data-testid="office-picker-input"]').setValue('ka')
      vi.advanceTimersByTime(300)
      await flushPromises()
      await w.vm.$nextTick()
      vi.useRealTimers()

      const loadingEls = w.findAll('[role="status"]')
      expect(loadingEls.length).toBeGreaterThan(0)
    })

    it('shows role=status on the empty state', async () => {
      vi.useFakeTimers()
      const w = await picker({ searchFn: vi.fn(async () => []) })
      await openWithResults(w, 'zzz')
      vi.useRealTimers()

      const empty = w.find('[data-testid="office-picker-empty"]')
      expect(empty.attributes('role')).toBe('status')
      expect(empty.attributes('aria-live')).toBe('polite')
    })

    it('Escape closes the popover and refocuses the input', async () => {
      vi.useFakeTimers()
      const w = await picker({}, { attachTo: document.body })
      await openWithResults(w)

      const inputEl = w.find('[data-testid="office-picker-input"]').element as HTMLInputElement
      inputEl.focus()
      expect(document.activeElement).toBe(inputEl)
      // move focus elsewhere first, so refocus-after-Escape is a real assertion
      const decoy = document.createElement('button')
      document.body.appendChild(decoy)
      decoy.focus()
      expect(document.activeElement).toBe(decoy)

      await w.find('[data-testid="office-picker-input"]').trigger('keydown', { key: 'Escape' })
      await w.vm.$nextTick()
      vi.useRealTimers()

      expect(document.activeElement).toBe(inputEl)
      w.unmount()
      decoy.remove()
    })

    it('ArrowUp moves activeIndex up, wrapping to the last option from the top', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)

      const input = w.find('[data-testid="office-picker-input"]')
      // activeIndex starts at -1 (open resets it); ArrowUp wraps to the last option
      await input.trigger('keydown', { key: 'ArrowUp' })
      await w.vm.$nextTick()

      let options = w.findAll('[data-testid="office-picker-item"]')
      expect(options[1]!.attributes('aria-selected')).toBe('true')
      expect(input.attributes('aria-activedescendant')).toBe(options[1]!.attributes('id'))

      // a second ArrowUp moves it up to the first option
      await input.trigger('keydown', { key: 'ArrowUp' })
      await w.vm.$nextTick()
      vi.useRealTimers()

      options = w.findAll('[data-testid="office-picker-item"]')
      expect(options[0]!.attributes('aria-selected')).toBe('true')
    })

    it('Home selects the first option and End selects the last, once navigation has started', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)

      const input = w.find('[data-testid="office-picker-input"]')
      // ArrowDown starts navigation (activeIndex = 0), enabling Home/End
      await input.trigger('keydown', { key: 'ArrowDown' })
      await input.trigger('keydown', { key: 'ArrowDown' })
      await w.vm.$nextTick()
      let options = w.findAll('[data-testid="office-picker-item"]')
      expect(options[1]!.attributes('aria-selected')).toBe('true')

      await input.trigger('keydown', { key: 'Home' })
      await w.vm.$nextTick()
      options = w.findAll('[data-testid="office-picker-item"]')
      expect(options[0]!.attributes('aria-selected')).toBe('true')

      await input.trigger('keydown', { key: 'End' })
      await w.vm.$nextTick()
      vi.useRealTimers()

      options = w.findAll('[data-testid="office-picker-item"]')
      expect(options[1]!.attributes('aria-selected')).toBe('true')
    })

    it('Home/End do not hijack the text cursor before list navigation has started', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)

      const input = w.find('[data-testid="office-picker-input"]')
      // activeIndex is still -1 (no ArrowDown/Up yet) — Home/End must be a no-op on the list
      const homeEvent = keydown(input.element, 'Home')
      await w.vm.$nextTick()
      const endEvent = keydown(input.element, 'End')
      await w.vm.$nextTick()
      vi.useRealTimers()

      expect(homeEvent.defaultPrevented).toBe(false)
      expect(endEvent.defaultPrevented).toBe(false)
      expect(input.attributes('aria-activedescendant')).toBeUndefined()
    })

    it('prevents default on Enter while the popover is open with results, even before any arrow navigation, and does not select', async () => {
      vi.useFakeTimers()
      const w = await picker()
      await openWithResults(w)

      const input = w.find('[data-testid="office-picker-input"]')
      expect(input.attributes('aria-activedescendant')).toBeUndefined()

      const enterEvent = keydown(input.element, 'Enter')
      await w.vm.$nextTick()
      vi.useRealTimers()

      expect(enterEvent.defaultPrevented).toBe(true)
      expect(w.emitted('update:modelValue')).toBeUndefined()
      // popover stays open — Enter was swallowed, not treated as a close/select action
      expect(w.find('[data-testid="office-picker-input"]').attributes('aria-expanded')).toBe('true')
    })

    it('lets Enter pass through to a wrapping form when the popover is closed', async () => {
      const w = await picker()
      const input = w.find('[data-testid="office-picker-input"]')
      expect(input.attributes('aria-expanded')).toBe('false')

      const enterEvent = keydown(input.element, 'Enter')
      await w.vm.$nextTick()

      expect(enterEvent.defaultPrevented).toBe(false)
    })

    it('resets activeIndex to -1 when a new search starts loading', async () => {
      vi.useFakeTimers()
      const searchFn = vi.fn(async (term: string) => items.filter(i => i.label.toLowerCase().includes(term.toLowerCase())))
      const w = await picker({ searchFn })
      await openWithResults(w)

      const input = w.find('[data-testid="office-picker-input"]')
      await input.trigger('keydown', { key: 'ArrowDown' })
      await w.vm.$nextTick()
      expect(input.attributes('aria-activedescendant')).toBeTruthy()

      // the next search hangs (never resolves), so `results` never changes reference — isolates
      // the reset to the `loading` watcher rather than the pre-existing `results` watcher
      searchFn.mockImplementationOnce(() => new Promise(() => {}))
      await input.setValue('kan')
      vi.advanceTimersByTime(300)
      await w.vm.$nextTick()
      vi.useRealTimers()

      expect(w.find('[data-testid="office-picker-input"]').attributes('aria-activedescendant')).toBeUndefined()
    })
  })
})
