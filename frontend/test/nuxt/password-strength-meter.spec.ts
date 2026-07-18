// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import PasswordStrengthMeter from '~/components/PasswordStrengthMeter.vue'

const SEGMENTS = '[data-testid="password-strength"] span.flex-1'
const LABEL = '[data-testid="password-strength-label"]'

function filledCount(wrapper: { findAll: (s: string) => { classes: () => string[] }[] }) {
  return wrapper.findAll(SEGMENTS).filter(s => !s.classes().includes('bg-elevated')).length
}

describe('PasswordStrengthMeter', () => {
  it('renders nothing for an empty password', async () => {
    const wrapper = await mountSuspended(PasswordStrengthMeter, { props: { password: '' } })
    expect(wrapper.find('[data-testid="password-strength"]').exists()).toBe(false)
  })

  it('renders four segments once a password is typed', async () => {
    const wrapper = await mountSuspended(PasswordStrengthMeter, { props: { password: 'a' } })
    expect(wrapper.find('[data-testid="password-strength"]').exists()).toBe(true)
    expect(wrapper.findAll(SEGMENTS)).toHaveLength(4)
  })

  // Strings are the resolved 'id' locale (the app default) — asserting them
  // verifies the i18n keys actually exist rather than rendering raw key names.
  it.each([
    ['a', 0, '—'],
    ['abcdefgh', 1, 'Lemah'],
    ['abcdefgh1', 2, 'Sedang'],
    ['abcdefG1', 3, 'Kuat'],
    ['abcdefG1!', 4, 'Sangat Kuat']
  ])('scores %s as %i (%s)', async (password, score, label) => {
    const wrapper = await mountSuspended(PasswordStrengthMeter, { props: { password } })
    expect(wrapper.find(LABEL).text()).toBe(label)
    expect(filledCount(wrapper)).toBe(score)
  })

  it('shows a short-but-complex password as weaker than the same password at length', async () => {
    const short = await mountSuspended(PasswordStrengthMeter, { props: { password: 'aB1!' } })
    const long = await mountSuspended(PasswordStrengthMeter, { props: { password: 'aB1!aB1!' } })
    expect(short.find(LABEL).text()).toBe('Kuat')
    expect(long.find(LABEL).text()).toBe('Sangat Kuat')
  })

  it('renders the localized "password strength" caption alongside the score', async () => {
    const wrapper = await mountSuspended(PasswordStrengthMeter, { props: { password: 'abcdefgh' } })
    const text = wrapper.find('[data-testid="password-strength"] p').text()
    expect(text).toContain('Kekuatan password')
    expect(text).toContain('Lemah')
  })

  it('reacts to the password changing', async () => {
    const wrapper = await mountSuspended(PasswordStrengthMeter, { props: { password: 'abcdefgh' } })
    expect(wrapper.find(LABEL).text()).toBe('Lemah')
    await wrapper.setProps({ password: 'abcdefG1!' })
    expect(wrapper.find(LABEL).text()).toBe('Sangat Kuat')
    await wrapper.setProps({ password: '' })
    expect(wrapper.find('[data-testid="password-strength"]').exists()).toBe(false)
  })
})
