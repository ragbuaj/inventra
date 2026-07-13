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

function picker(overrides: Record<string, unknown> = {}) {
  return mountSuspended(AsyncSearchPicker, {
    props: {
      modelValue: null,
      searchFn: vi.fn(async (term: string) => items.filter(i => i.label.toLowerCase().includes(term.toLowerCase()))),
      placeholder: 'Cari kantor',
      testid: 'office',
      ...overrides
    }
  })
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
})
