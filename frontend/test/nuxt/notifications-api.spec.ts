// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Replaces the old test/unit/notifications-mock.spec.ts, which asserted the
// mock-backed synchronous composable. useNotifications now talks HTTP, so the
// contract worth pinning is the one against useApiClient: paths, query building
// and envelope unwrapping.
const requestMock = vi.fn()

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: vi.fn(), refreshToken: vi.fn() })
}))

// eslint-disable-next-line import/first
import { useNotifications } from '~/composables/api/useNotifications'

const emptyPage = { data: [], total: 0, limit: 20, offset: 0 }

describe('useNotifications — list', () => {
  beforeEach(() => vi.clearAllMocks())

  it('GETs /notifications and returns the whole envelope', async () => {
    const page = {
      data: [{ id: 'n-1', type: 'approval_pending', params: { request_type: 'assignment', step: '1' }, entity_type: 'requests', entity_id: 'r-1', read_at: null, created_at: '2026-07-17T09:00:00Z' }],
      total: 42,
      limit: 20,
      offset: 0
    }
    requestMock.mockResolvedValue(page)

    const res = await useNotifications().list()

    expect(requestMock).toHaveBeenCalledWith('/notifications', { query: {} })
    // Unlike unreadCount(), the page envelope is the caller's payload -- total
    // drives server-side pagination -- so it is returned whole.
    expect(res).toEqual(page)
    expect(res.total).toBe(42)
  })

  it('omits undefined query params rather than sending them', async () => {
    requestMock.mockResolvedValue(emptyPage)

    await useNotifications().list({ limit: 10 })

    expect(requestMock).toHaveBeenCalledWith('/notifications', { query: { limit: 10 } })
  })

  it('passes limit, offset and the read filter through', async () => {
    requestMock.mockResolvedValue(emptyPage)

    await useNotifications().list({ read: false, limit: 50, offset: 100 })

    expect(requestMock).toHaveBeenCalledWith('/notifications', { query: { read: false, limit: 50, offset: 100 } })
  })

  it('sends read=false (unread only) rather than dropping the falsy value', async () => {
    requestMock.mockResolvedValue(emptyPage)

    await useNotifications().list({ read: false })

    // A truthiness check here would silently turn "unread only" into "everything".
    expect(requestMock).toHaveBeenCalledWith('/notifications', { query: { read: false } })
  })

  it('sends read=true (read only)', async () => {
    requestMock.mockResolvedValue(emptyPage)

    await useNotifications().list({ read: true })

    expect(requestMock).toHaveBeenCalledWith('/notifications', { query: { read: true } })
  })

  it('sends offset=0 rather than dropping the falsy value', async () => {
    requestMock.mockResolvedValue(emptyPage)

    await useNotifications().list({ offset: 0 })

    expect(requestMock).toHaveBeenCalledWith('/notifications', { query: { offset: 0 } })
  })

  it('rethrows without catching -- useApiClient owns the error toast', async () => {
    requestMock.mockRejectedValue(new Error('boom'))

    await expect(useNotifications().list()).rejects.toThrow('boom')
  })
})

describe('useNotifications — unreadCount', () => {
  beforeEach(() => vi.clearAllMocks())

  it('unwraps the {count} envelope at the composable boundary', async () => {
    requestMock.mockResolvedValue({ count: 7 })

    await expect(useNotifications().unreadCount()).resolves.toBe(7)
    expect(requestMock).toHaveBeenCalledWith('/notifications/unread-count')
  })

  it('returns 0 for an empty feed', async () => {
    requestMock.mockResolvedValue({ count: 0 })

    await expect(useNotifications().unreadCount()).resolves.toBe(0)
  })

  it('rethrows on failure', async () => {
    requestMock.mockRejectedValue(new Error('boom'))

    await expect(useNotifications().unreadCount()).rejects.toThrow('boom')
  })
})

describe('useNotifications — markRead', () => {
  beforeEach(() => vi.clearAllMocks())

  it('POSTs to /notifications/:id/read and returns the updated row', async () => {
    const updated = { id: 'n-1', type: 'asset_returned', params: {}, entity_type: 'assets', entity_id: 'a-1', read_at: '2026-07-17T10:00:00Z', created_at: '2026-07-17T09:00:00Z' }
    requestMock.mockResolvedValue(updated)

    const res = await useNotifications().markRead('n-1')

    expect(requestMock).toHaveBeenCalledWith('/notifications/n-1/read', { method: 'POST' })
    expect(res.read_at).toBe('2026-07-17T10:00:00Z')
  })

  it('rethrows a 404 (an id owned by another user) rather than swallowing it', async () => {
    requestMock.mockRejectedValue(Object.assign(new Error('Not Found'), { statusCode: 404 }))

    await expect(useNotifications().markRead('someone-elses-id')).rejects.toThrow('Not Found')
  })
})

describe('useNotifications — markAllRead', () => {
  beforeEach(() => vi.clearAllMocks())

  it('POSTs to /notifications/read-all and resolves with nothing (204)', async () => {
    requestMock.mockResolvedValue(undefined)

    await expect(useNotifications().markAllRead()).resolves.toBeUndefined()
    expect(requestMock).toHaveBeenCalledWith('/notifications/read-all', { method: 'POST' })
  })

  it('rethrows on failure', async () => {
    requestMock.mockRejectedValue(new Error('boom'))

    await expect(useNotifications().markAllRead()).rejects.toThrow('boom')
  })
})
