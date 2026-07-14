// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import NumberInput from '~/components/NumberInput.vue'

describe('NumberInput', () => {
  it('renders raw value with thousand separator', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '1000000', thousandSeparator: true } })
    expect(c.find('input').element.value).toBe('1.000.000')
  })
  it('shows Rp leading in money mode', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '2500', money: true } })
    expect(c.text()).toContain('Rp')
    expect(c.find('input').element.value).toBe('2.500')
  })
  it('emits raw digits on input', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', thousandSeparator: true } })
    const input = c.find('input')
    await input.setValue('1.234.567')
    const emits = c.emitted('update:modelValue')
    expect(emits?.at(-1)?.[0]).toBe('1234567')
  })
  it('strips a minus when allowNegative is false', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', allowNegative: false } })
    await c.find('input').setValue('-42')
    expect(c.emitted('update:modelValue')?.at(-1)?.[0]).toBe('42')
  })
  it('keeps minus and decimals when configured', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', allowNegative: true, decimals: 7 } })
    await c.find('input').setValue('-6.2000000')
    expect(c.emitted('update:modelValue')?.at(-1)?.[0]).toBe('-6.2000000')
  })
  it('does not corrupt raw value when sequentially typing a decimal money amount', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', money: true, decimals: 2 } })
    const input = c.find('input')
    for (const key of ['1', '2', '3', '4', '5', ',', '6', '7']) {
      const current = input.element.value
      await input.setValue(current + key)
    }
    expect(c.emitted('update:modelValue')?.at(-1)?.[0]).toBe('12345.67')
    expect(input.element.value).toBe('12.345,67')
  })
  it('does not corrupt raw value when sequentially typing with thousandSeparator and decimals', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', thousandSeparator: true, decimals: 2 } })
    const input = c.find('input')
    for (const key of ['1', '2', '3', '4', '5', ',', '6', '7']) {
      const current = input.element.value
      await input.setValue(current + key)
    }
    expect(c.emitted('update:modelValue')?.at(-1)?.[0]).toBe('12345.67')
    expect(input.element.value).toBe('12.345,67')
  })
})
