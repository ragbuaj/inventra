// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import LoginPage from '~/pages/login.vue'

// Hoisted mocks — must be created before any mockNuxtImport calls.
const { refreshMock, fetchMeMock, navigateToMock, toastAddMock, routeQuery } = vi.hoisted(() => {
  const routeQuery: Record<string, string | undefined> = {}
  return {
    refreshMock: vi.fn(() => Promise.resolve(true)),
    fetchMeMock: vi.fn(() => Promise.resolve()),
    navigateToMock: vi.fn(),
    toastAddMock: vi.fn(),
    routeQuery
  }
})

mockNuxtImport('useAuthApi', () => () => ({
  login: vi.fn(),
  logout: vi.fn(),
  refresh: refreshMock,
  fetchMe: fetchMeMock
}))
mockNuxtImport('navigateTo', () => navigateToMock)
mockNuxtImport('useRoute', () => () => ({ query: routeQuery }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

beforeEach(() => {
  refreshMock.mockClear()
  fetchMeMock.mockClear()
  navigateToMock.mockClear()
  toastAddMock.mockClear()
  // Reset query on each test by clearing known keys
  routeQuery.oauth = undefined
  routeQuery.reason = undefined
  routeQuery.reset = undefined
})

describe('login.vue Google landing', () => {
  it('on ?oauth=success it refreshes, fetches me, and navigates home', async () => {
    routeQuery.oauth = 'success'
    await mountSuspended(LoginPage)
    await new Promise(r => setTimeout(r, 10))
    expect(refreshMock).toHaveBeenCalled()
    expect(fetchMeMock).toHaveBeenCalled()
    expect(navigateToMock).toHaveBeenCalledWith('/')
  })

  it('on ?oauth=error it shows the reason message', async () => {
    routeQuery.oauth = 'error'
    routeQuery.reason = 'not_registered'
    const w = await mountSuspended(LoginPage)
    await new Promise(r => setTimeout(r, 10))
    // The not_registered message from i18n appears on the page (text differs by locale;
    // the test environment resolves to 'en', so check the English key text).
    const html = w.html()
    const hasId = html.includes('belum terdaftar')
    const hasEn = html.includes('No account exists for this Google email')
    expect(hasId || hasEn).toBe(true)
  })

  it('on ?reset=success it shows a success toast', async () => {
    routeQuery.reset = 'success'
    await mountSuspended(LoginPage)
    await new Promise(r => setTimeout(r, 10))
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ color: 'success' }))
  })
})
