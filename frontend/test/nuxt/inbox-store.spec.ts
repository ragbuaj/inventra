// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAuthStore } from '~/stores/auth'
import { useInboxStore } from '~/stores/inbox'

const inboxCountMock = vi.fn()

vi.mock('~/composables/api/useApproval', () => ({
  useApproval: () => ({ inboxCount: inboxCountMock })
}))

function login(permissions: string[]) {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Test', email: 'test@e.com', role_id: 'r1', role_name: 'Role', office_id: null },
    permissions
  )
}

describe('useInboxStore', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore().clear()
    useInboxStore().pendingCount = 0
  })

  it('starts at 0', () => {
    expect(useInboxStore().pendingCount).toBe(0)
  })

  it('refresh() calls inboxCount() and sets pendingCount when the caller has request.decide', async () => {
    login(['request.decide'])
    inboxCountMock.mockResolvedValue(5)

    await useInboxStore().refresh()

    expect(inboxCountMock).toHaveBeenCalledTimes(1)
    expect(useInboxStore().pendingCount).toBe(5)
  })

  it('refresh() sets pendingCount to 0 and does NOT call the API when the caller lacks request.decide', async () => {
    login(['asset.read'])
    inboxCountMock.mockResolvedValue(5)

    await useInboxStore().refresh()

    expect(inboxCountMock).not.toHaveBeenCalled()
    expect(useInboxStore().pendingCount).toBe(0)
  })

  it('refresh() with wildcard "*" permission calls the API', async () => {
    login(['*'])
    inboxCountMock.mockResolvedValue(3)

    await useInboxStore().refresh()

    expect(inboxCountMock).toHaveBeenCalledTimes(1)
    expect(useInboxStore().pendingCount).toBe(3)
  })

  it('refresh() keeps the last known count when the API call fails', async () => {
    login(['request.decide'])
    inboxCountMock.mockResolvedValue(7)
    await useInboxStore().refresh()
    expect(useInboxStore().pendingCount).toBe(7)

    inboxCountMock.mockRejectedValue(new Error('boom'))
    await useInboxStore().refresh()

    expect(useInboxStore().pendingCount).toBe(7)
  })
})
