import type { NotificationType } from '~/constants/notificationMeta'

/**
 * One row of the caller's feed (openapi.yaml `Notification`). `params` carries
 * only interpolation values — the backend never sends a rendered sentence, so
 * the message is built client-side from `type` + `params`.
 */
export interface NotificationRow {
  id: string
  type: NotificationType
  params: Record<string, string>
  entity_type: string | null
  entity_id: string | null
  /** Null while unread. */
  read_at: string | null
  created_at: string
}

export interface NotificationListQuery {
  /** `false` returns unread only, `true` read only; omit for the whole feed. */
  read?: boolean
  limit?: number
  offset?: number
}

export interface NotificationListPage {
  data: NotificationRow[]
  total: number
  limit: number
  offset: number
}

/**
 * Per-user notification feed, wired to /api/v1/notifications.
 *
 * No permission key gates these endpoints — ownership is enforced server-side
 * by `WHERE user_id = caller`, so another user's rows are never reachable and a
 * foreign id returns 404, not 403.
 *
 * Errors are deliberately not caught here: useApiClient already raises the
 * generic error toast centrally and rethrows, so a second toast would double up.
 */
export function useNotifications() {
  const { request } = useApiClient()

  async function list(q: NotificationListQuery = {}): Promise<NotificationListPage> {
    const query: Record<string, string | number | boolean> = {}
    if (q.read !== undefined) query.read = q.read
    if (q.limit !== undefined) query.limit = q.limit
    if (q.offset !== undefined) query.offset = q.offset
    return request<NotificationListPage>('/notifications', { query })
  }

  async function unreadCount(): Promise<number> {
    const res = await request<{ count: number }>('/notifications/unread-count')
    return res.count
  }

  async function markRead(id: string): Promise<NotificationRow> {
    return request<NotificationRow>(`/notifications/${id}/read`, { method: 'POST' })
  }

  /** Marks every unread row read. The endpoint answers 204 — there is no body. */
  async function markAllRead(): Promise<void> {
    await request('/notifications/read-all', { method: 'POST' })
  }

  return { list, unreadCount, markRead, markAllRead }
}
