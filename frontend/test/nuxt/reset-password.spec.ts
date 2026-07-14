// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import ResetPassword from '~/pages/reset-password.vue'
import { useAuthStore } from '~/stores/auth'

// Hoisted mocks — must be created before any mockNuxtImport calls.
const { resetPassword, routeQuery, navigateToMock } = vi.hoisted(() => {
  const routeQuery: Record<string, string | undefined> = { token: 'tok123' }
  return {
    resetPassword: vi.fn(),
    routeQuery,
    navigateToMock: vi.fn()
  }
})

mockNuxtImport('useAccount', () => () => ({ resetPassword }))
mockNuxtImport('useRoute', () => () => ({ query: routeQuery }))
mockNuxtImport('navigateTo', () => navigateToMock)

beforeEach(() => {
  resetPassword.mockClear()
  navigateToMock.mockClear()
  routeQuery.token = 'tok123'
})

describe('reset-password page', () => {
  it('rejects mismatched confirmation without calling the API', async () => {
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('brandnewpass')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('different1')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(resetPassword).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="reset-error"]').exists()).toBe(true)
  })

  it('rejects a weak (too short) new password without calling the API', async () => {
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('short')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('short')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(resetPassword).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="reset-error"]').exists()).toBe(true)
  })

  it('calls resetPassword with the token and new password', async () => {
    resetPassword.mockResolvedValueOnce(undefined)
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('brandnewpass')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('brandnewpass')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(resetPassword).toHaveBeenCalledWith('tok123', 'brandnewpass')
  })

  it('navigates to /login with ?reset=success on a successful reset', async () => {
    resetPassword.mockResolvedValueOnce(undefined)
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('brandnewpass')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('brandnewpass')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    // localePath resolves against the active test-env locale (prefix_except_default,
    // default 'id') — the vitest/nuxt environment resolves to 'en', so the path is
    // prefixed. What matters here is the reset=success query survives the navigation.
    expect(navigateToMock).toHaveBeenCalledWith({ path: expect.stringMatching(/\/login$/), query: { reset: 'success' } })
  })

  it('clears the local auth session on a successful reset (logged-in user is logged out)', async () => {
    resetPassword.mockResolvedValueOnce(undefined)
    const auth = useAuthStore()
    auth.setSession('stale-token', { id: 'u1', name: 'Admin', email: 'admin@inventra.local', role_id: 'r1' } as never, [])
    expect(auth.isAuthenticated).toBe(true)
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('brandnewpass')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('brandnewpass')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    // Password reset invalidates all sessions server-side (epoch); the frontend must
    // drop its now-stale token so auth.global.ts doesn't bounce /login -> / for a
    // still-"authenticated" client.
    expect(auth.isAuthenticated).toBe(false)
  })

  it('shows the invalid-token error inline when the API rejects with 400', async () => {
    resetPassword.mockRejectedValueOnce({ statusCode: 400 })
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('brandnewpass')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('brandnewpass')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(wrapper.find('[data-testid="reset-error"]').exists()).toBe(true)
  })

  it('renders the no-token error panel and no form when ?token is absent', async () => {
    routeQuery.token = undefined
    const wrapper = await mountSuspended(ResetPassword)
    expect(wrapper.find('[data-testid="reset-notoken"]').exists()).toBe(true)
    expect(wrapper.find('form').exists()).toBe(false)
  })
})
