// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import ForgotPassword from '~/pages/forgot-password.vue'

const requestPasswordReset = vi.fn()
mockNuxtImport('useAccount', () => () => ({ requestPasswordReset }))

describe('forgot-password page', () => {
  it('shows the success panel after submitting', async () => {
    requestPasswordReset.mockResolvedValueOnce(undefined)
    const wrapper = await mountSuspended(ForgotPassword)
    await wrapper.find('[data-testid="forgot-email"]').setValue('u@example.com')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(requestPasswordReset).toHaveBeenCalledWith('u@example.com')
    expect(wrapper.find('[data-testid="forgot-sent"]').exists()).toBe(true)
  })

  it('renders the email field before submission', async () => {
    const wrapper = await mountSuspended(ForgotPassword)
    expect(wrapper.find('[data-testid="forgot-email"]').exists()).toBe(true)
  })

  it('email input is full width', async () => {
    const wrapper = await mountSuspended(ForgotPassword)
    expect(wrapper.find('[data-testid="forgot-email"]').classes()).toContain('w-full')
  })

  it('shows resend with countdown after sending', async () => {
    requestPasswordReset.mockResolvedValueOnce(undefined)
    const wrapper = await mountSuspended(ForgotPassword)
    await wrapper.find('[data-testid="forgot-email"]').setValue('u@example.com')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    const resend = wrapper.find('[data-testid="forgot-resend"]')
    expect(resend.exists()).toBe(true)
    expect(resend.attributes('disabled')).toBeDefined()
    expect(resend.text()).toContain('s')
  })

  it('resend button re-invokes the request and restarts the cooldown', async () => {
    vi.useFakeTimers()
    requestPasswordReset.mockClear()
    requestPasswordReset.mockResolvedValue(undefined)
    const wrapper = await mountSuspended(ForgotPassword)
    await wrapper.find('[data-testid="forgot-email"]').setValue('u@example.com')
    await wrapper.find('form').trigger('submit')
    await vi.advanceTimersByTimeAsync(0)
    expect(requestPasswordReset).toHaveBeenCalledTimes(1)

    // still within the first cooldown window: resend is disabled
    let resend = wrapper.find('[data-testid="forgot-resend"]')
    expect(resend.attributes('disabled')).toBeDefined()

    // advance past the first cooldown (base 30s)
    await vi.advanceTimersByTimeAsync(30_000)
    resend = wrapper.find('[data-testid="forgot-resend"]')
    expect(resend.attributes('disabled')).toBeUndefined()

    await resend.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    expect(requestPasswordReset).toHaveBeenCalledTimes(2)

    // second cooldown is longer (exponential backoff) and should be active again
    resend = wrapper.find('[data-testid="forgot-resend"]')
    expect(resend.attributes('disabled')).toBeDefined()
    vi.useRealTimers()
  })
})
