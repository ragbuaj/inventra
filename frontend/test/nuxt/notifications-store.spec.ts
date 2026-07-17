// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useAuthStore } from '~/stores/auth'
import { useNotificationsStore } from '~/stores/notifications'
import type { NotificationRow } from '~/composables/api/useNotifications'

const listMock = vi.fn()
const unreadCountMock = vi.fn()
const markAllReadMock = vi.fn()
const markReadMock = vi.fn()

vi.mock('~/composables/api/useNotifications', () => ({
  useNotifications: () => ({
    list: listMock,
    unreadCount: unreadCountMock,
    markAllRead: markAllReadMock,
    markRead: markReadMock
  })
}))

function row(id: string, overrides: Partial<NotificationRow> = {}): NotificationRow {
  return {
    id,
    type: 'approval_pending',
    params: { request_type: 'assignment', step: '1' },
    entity_type: 'requests',
    entity_id: 'req-1',
    read_at: null,
    created_at: '2026-07-17T09:00:00Z',
    ...overrides
  }
}

function page(data: NotificationRow[], total = data.length) {
  return { data, total, limit: 20, offset: 0 }
}

function login() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Test', email: 'test@e.com', role_id: 'r1', role_name: 'Role', office_id: null },
    []
  )
}

describe('useNotificationsStore', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore().clear()
    const store = useNotificationsStore()
    store.items = []
    store.unreadCount = 0
  })

  it('starts empty', () => {
    const store = useNotificationsStore()
    expect(store.items).toEqual([])
    expect(store.unreadCount).toBe(0)
  })

  it('refresh() populates items and unreadCount from the API', async () => {
    login()
    listMock.mockResolvedValue(page([row('n-1'), row('n-2', { read_at: '2026-07-17T10:00:00Z' })]))
    unreadCountMock.mockResolvedValue(1)

    await useNotificationsStore().refresh()

    const store = useNotificationsStore()
    expect(store.items).toHaveLength(2)
    expect(store.items.map(n => n.id)).toEqual(['n-1', 'n-2'])
    expect(store.items[0]!.read_at).toBeNull()
    expect(store.unreadCount).toBe(1)
  })

  it('refresh() requests only the bell page size, unfiltered by read state', async () => {
    login()
    listMock.mockResolvedValue(page([]))
    unreadCountMock.mockResolvedValue(0)

    await useNotificationsStore().refresh()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(listMock).toHaveBeenCalledWith({ limit: 20 })
    // The bell shows read and unread alike, so no `read` filter is sent.
    expect(listMock.mock.calls[0]![0]).not.toHaveProperty('read')
  })

  it('refresh() is NOT permission-gated -- a user with no permissions still gets a feed', async () => {
    login() // deliberately no permissions at all
    listMock.mockResolvedValue(page([row('n-1')]))
    unreadCountMock.mockResolvedValue(1)

    await useNotificationsStore().refresh()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(useNotificationsStore().unreadCount).toBe(1)
  })

  it('refresh() without a session clears state and does NOT call the API', async () => {
    // No login(): auth.isAuthenticated is false.
    await useNotificationsStore().refresh()

    expect(listMock).not.toHaveBeenCalled()
    expect(unreadCountMock).not.toHaveBeenCalled()
    expect(useNotificationsStore().items).toEqual([])
    expect(useNotificationsStore().unreadCount).toBe(0)
  })

  it('refresh() after logout clears a previously populated feed', async () => {
    login()
    listMock.mockResolvedValue(page([row('n-1')]))
    unreadCountMock.mockResolvedValue(1)
    await useNotificationsStore().refresh()
    expect(useNotificationsStore().unreadCount).toBe(1)

    useAuthStore().clear()
    await useNotificationsStore().refresh()

    // A signed-out session must not keep the previous user's rows on screen.
    expect(useNotificationsStore().items).toEqual([])
    expect(useNotificationsStore().unreadCount).toBe(0)
  })

  it('refresh() keeps the last known items and count when the list call fails', async () => {
    login()
    listMock.mockResolvedValue(page([row('n-1')]))
    unreadCountMock.mockResolvedValue(4)
    await useNotificationsStore().refresh()
    expect(useNotificationsStore().unreadCount).toBe(4)

    listMock.mockRejectedValue(new Error('boom'))
    unreadCountMock.mockResolvedValue(9)
    await useNotificationsStore().refresh()

    expect(useNotificationsStore().items.map(n => n.id)).toEqual(['n-1'])
    expect(useNotificationsStore().unreadCount).toBe(4)
  })

  it('refresh() keeps the last known items and count when the count call fails', async () => {
    login()
    listMock.mockResolvedValue(page([row('n-1')]))
    unreadCountMock.mockResolvedValue(4)
    await useNotificationsStore().refresh()

    unreadCountMock.mockRejectedValue(new Error('boom'))
    listMock.mockResolvedValue(page([row('n-2'), row('n-3')]))
    await useNotificationsStore().refresh()

    // Neither half is applied: a list without its matching count would render a
    // badge that disagrees with the rows below it.
    expect(useNotificationsStore().items.map(n => n.id)).toEqual(['n-1'])
    expect(useNotificationsStore().unreadCount).toBe(4)
  })

  it('refresh() does not rethrow -- useApiClient already toasted the error', async () => {
    login()
    listMock.mockRejectedValue(new Error('boom'))
    unreadCountMock.mockRejectedValue(new Error('boom'))

    await expect(useNotificationsStore().refresh()).resolves.toBeUndefined()
  })

  it('refresh() recovers on the next successful call after a failure', async () => {
    login()
    listMock.mockRejectedValue(new Error('boom'))
    unreadCountMock.mockRejectedValue(new Error('boom'))
    await useNotificationsStore().refresh()
    expect(useNotificationsStore().unreadCount).toBe(0)

    listMock.mockResolvedValue(page([row('n-9')]))
    unreadCountMock.mockResolvedValue(1)
    await useNotificationsStore().refresh()

    expect(useNotificationsStore().items.map(n => n.id)).toEqual(['n-9'])
    expect(useNotificationsStore().unreadCount).toBe(1)
  })

  it('refresh() replaces items rather than appending them', async () => {
    login()
    listMock.mockResolvedValue(page([row('n-1'), row('n-2')]))
    unreadCountMock.mockResolvedValue(2)
    await useNotificationsStore().refresh()

    listMock.mockResolvedValue(page([row('n-3')]))
    unreadCountMock.mockResolvedValue(1)
    await useNotificationsStore().refresh()

    expect(useNotificationsStore().items.map(n => n.id)).toEqual(['n-3'])
    expect(useNotificationsStore().unreadCount).toBe(1)
  })

  it('does not poll: one refresh() is exactly one call to each endpoint', async () => {
    vi.useFakeTimers()
    try {
      login()
      listMock.mockResolvedValue(page([row('n-1')]))
      unreadCountMock.mockResolvedValue(1)

      await useNotificationsStore().refresh()
      expect(listMock).toHaveBeenCalledTimes(1)
      expect(unreadCountMock).toHaveBeenCalledTimes(1)

      // Advancing far past any plausible poll interval must not trigger a refetch:
      // the store is event-driven (fetchMe choke point), never timer-driven.
      await vi.advanceTimersByTimeAsync(10 * 60 * 1000)

      expect(listMock).toHaveBeenCalledTimes(1)
      expect(unreadCountMock).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })
})
