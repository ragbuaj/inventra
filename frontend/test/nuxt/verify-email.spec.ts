// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import VerifyEmail from '~/pages/verify-email.vue'
import { useAuthStore } from '~/stores/auth'

// Hoisted mocks — must be created before any mockNuxtImport calls.
const { confirmEmailChange, getProfile, routeQuery } = vi.hoisted(() => {
  const routeQuery: Record<string, string | undefined> = { token: 'abc' }
  return {
    confirmEmailChange: vi.fn(),
    getProfile: vi.fn(),
    routeQuery
  }
})

mockNuxtImport('useAccount', () => () => ({ confirmEmailChange, getProfile }))
mockNuxtImport('useRoute', () => () => ({ query: routeQuery }))

beforeEach(() => {
  confirmEmailChange.mockReset()
  getProfile.mockReset()
  routeQuery.token = 'abc'
  useAuthStore().clear()
})

describe('verify-email page', () => {
  it('with ?token=abc, calls confirmEmailChange on mount and renders success', async () => {
    confirmEmailChange.mockResolvedValueOnce(undefined)
    const wrapper = await mountSuspended(VerifyEmail)
    await flushPromises()

    expect(confirmEmailChange).toHaveBeenCalledWith('abc')
    expect(wrapper.find('[data-testid="verify-email-success"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="verify-email-loading"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="verify-email-error"]').exists()).toBe(false)
  })

  it('shows the loading state before confirmEmailChange resolves', async () => {
    let resolveConfirm!: () => void
    confirmEmailChange.mockReturnValueOnce(new Promise<void>((resolve) => {
      resolveConfirm = resolve
    }))
    const wrapper = await mountSuspended(VerifyEmail)
    expect(wrapper.find('[data-testid="verify-email-loading"]').exists()).toBe(true)
    resolveConfirm()
    await flushPromises()
    expect(wrapper.find('[data-testid="verify-email-success"]').exists()).toBe(true)
  })

  it('a rejected confirm renders the error state', async () => {
    confirmEmailChange.mockRejectedValueOnce(Object.assign(new Error('tautan tidak valid'), { statusCode: 400 }))
    const wrapper = await mountSuspended(VerifyEmail)
    await flushPromises()

    expect(confirmEmailChange).toHaveBeenCalledWith('abc')
    expect(wrapper.find('[data-testid="verify-email-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="verify-email-success"]').exists()).toBe(false)
  })

  it('renders the error state without calling the API when ?token is absent', async () => {
    routeQuery.token = undefined
    const wrapper = await mountSuspended(VerifyEmail)
    await flushPromises()

    expect(confirmEmailChange).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="verify-email-error"]').exists()).toBe(true)
  })

  it('best-effort refreshes the cached profile when already logged in on success', async () => {
    useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'old@inventra.local', role_id: 'r', role_name: 'Asset Manager', office_id: null }, ['*'])
    confirmEmailChange.mockResolvedValueOnce(undefined)
    getProfile.mockResolvedValueOnce({
      nama: 'Andi Saputra',
      email: 'new@inventra.local',
      telepon: '',
      peran: 'Asset Manager',
      kantor: '',
      pegawai: '',
      loginMethod: 'email',
      joinDate: '2024-01-01',
      hasEmployee: false
    })
    const wrapper = await mountSuspended(VerifyEmail)
    await flushPromises()

    expect(wrapper.find('[data-testid="verify-email-success"]').exists()).toBe(true)
    expect(getProfile).toHaveBeenCalled()
    expect(useAuthStore().user?.email).toBe('new@inventra.local')
  })

  it('does not fail the success state when the best-effort profile refresh rejects', async () => {
    useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'old@inventra.local', role_id: 'r', role_name: 'Asset Manager', office_id: null }, ['*'])
    confirmEmailChange.mockResolvedValueOnce(undefined)
    getProfile.mockRejectedValueOnce(new Error('network error'))
    const wrapper = await mountSuspended(VerifyEmail)
    await flushPromises()

    expect(wrapper.find('[data-testid="verify-email-success"]').exists()).toBe(true)
  })
})
