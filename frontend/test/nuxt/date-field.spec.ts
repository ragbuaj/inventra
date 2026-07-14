// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import DateField from '~/components/DateField.vue'

function field(props: Record<string, unknown> = {}) {
  return mountSuspended(DateField, { props: { testid: 'when', ...props } })
}

describe('DateField', () => {
  it('forwards the testid to a fillable input and shows the ISO value', async () => {
    const w = await field({ modelValue: '2026-07-04' })
    const input = w.find('[data-testid="when"]')
    expect(input.exists()).toBe(true)
    expect((input.element as HTMLInputElement).value).toBe('2026-07-04')
  })

  it('emits an ISO string when the user types a date', async () => {
    const w = await field({ modelValue: '' })
    await w.find('[data-testid="when"]').setValue('2026-07-15')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['2026-07-15'])
  })

  it('renders the calendar picker trigger', async () => {
    const w = await field()
    // The trailing calendar button carries the pickDate aria-label (resolved i18n).
    const trigger = w.find('button[aria-label="Pilih tanggal"]')
    expect(trigger.exists()).toBe(true)
    expect(w.html()).toContain('i-lucide:calendar')
  })

  it('disables the input when disabled', async () => {
    const w = await field({ disabled: true })
    expect(w.find('[data-testid="when"]').attributes('disabled')).toBeDefined()
  })

  it('renders an empty input for a null model value', async () => {
    const w = await field({ modelValue: null })
    expect((w.find('[data-testid="when"]').element as HTMLInputElement).value).toBe('')
  })

  it('does not throw on a malformed stored value', async () => {
    const w = await field({ modelValue: 'not-a-date' })
    expect((w.find('[data-testid="when"]').element as HTMLInputElement).value).toBe('not-a-date')
  })
})
