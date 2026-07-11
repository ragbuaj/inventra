// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { CalendarDate } from '@internationalized/date'
import PeriodFilter from '~/components/PeriodFilter.vue'
import type { PeriodValue } from '~/constants/reportMeta'

function lastEmitted(wrapper: Awaited<ReturnType<typeof mountSuspended>>): PeriodValue {
  const events = wrapper.emitted('update:modelValue')
  expect(events).toBeTruthy()
  return events![events!.length - 1]![0] as PeriodValue
}

describe('PeriodFilter', () => {
  it('renders the select with the 4 presets + custom (5 options)', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'last30' } as PeriodValue }
    })
    const select = wrapper.find('[data-testid="period-filter-select"]')
    expect(select.exists()).toBe(true)

    const cmp = wrapper.findComponent({ name: 'USelect' })
    const items = cmp.props('items') as Array<{ value: string }>
    expect(items).toHaveLength(5)
    expect(items.map(i => i.value)).toEqual([
      'last30',
      'this_month',
      'this_quarter',
      'ytd',
      'custom'
    ])
  })

  it('resolves the custom option label from the common.periodCustom i18n key', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'last30' } as PeriodValue }
    })
    const items = wrapper.findComponent({ name: 'USelect' }).props('items') as Array<{ value: string, label: string }>
    const custom = items.find(i => i.value === 'custom')
    // default locale is `id`
    expect(custom?.label).toBe('Rentang kustom…')
  })

  it('emits { preset } when a preset is selected', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'last30' } as PeriodValue }
    })
    const select = wrapper.findComponent({ name: 'USelect' })
    select.vm.$emit('update:modelValue', 'this_quarter')
    await wrapper.vm.$nextTick()
    expect(lastEmitted(wrapper)).toEqual({ preset: 'this_quarter' })
  })

  it('does not show the range button while a preset is active', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'this_month' } as PeriodValue }
    })
    expect(wrapper.find('[data-testid="period-filter-range"]').exists()).toBe(false)
  })

  it('shows the range button once custom is selected', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'last30' } as PeriodValue }
    })
    wrapper.findComponent({ name: 'USelect' }).vm.$emit('update:modelValue', 'custom')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="period-filter-range"]').exists()).toBe(true)
  })

  it('renders the range button immediately when the model is already custom', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'custom', from: '2026-01-01', to: '2026-01-31' } as PeriodValue }
    })
    const btn = wrapper.find('[data-testid="period-filter-range"]')
    expect(btn.exists()).toBe(true)
    expect(btn.text()).toContain('2026-01-01')
    expect(btn.text()).toContain('2026-01-31')
  })

  it('emits { preset:"custom", from, to } when a complete range is chosen', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'last30' } as PeriodValue }
    })
    wrapper.findComponent({ name: 'USelect' }).vm.$emit('update:modelValue', 'custom')
    await wrapper.vm.$nextTick()

    // drive the calendar via the exposed handler (DOM-clicking the teleported
    // popover calendar is brittle) — assert the emitted payload
    ;(wrapper.vm as unknown as { onCalendarUpdate: (r: unknown) => void }).onCalendarUpdate({
      start: new CalendarDate(2026, 2, 3),
      end: new CalendarDate(2026, 2, 20)
    })
    await wrapper.vm.$nextTick()

    expect(lastEmitted(wrapper)).toEqual({
      preset: 'custom',
      from: '2026-02-03',
      to: '2026-02-20'
    })
  })

  it('does not emit until BOTH ends of the range are chosen', async () => {
    const wrapper = await mountSuspended(PeriodFilter, {
      props: { modelValue: { preset: 'custom' } as PeriodValue }
    })
    ;(wrapper.vm as unknown as { onCalendarUpdate: (r: unknown) => void }).onCalendarUpdate({
      start: new CalendarDate(2026, 2, 3),
      end: undefined
    })
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('update:modelValue')).toBeFalsy()
  })
})
