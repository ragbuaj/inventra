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
})
