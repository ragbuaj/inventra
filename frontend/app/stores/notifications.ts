import { defineStore } from 'pinia'
import type { NotificationRow } from '~/composables/api/useNotifications'

/**
 * How many rows the bell dropdown holds. The full feed lives on /notifications,
 * which paginates server-side on its own rather than reading this store.
 */
export const BELL_PAGE_SIZE = 20

export const useNotificationsStore = defineStore('notifications', {
  state: () => ({
    items: [] as NotificationRow[],
    unreadCount: 0,
    /** True while a refresh is in flight — drives the bell's skeleton. */
    loading: false,
    /** True when the last refresh failed — drives the bell's retry block. */
    error: false
  }),
  actions: {
    async refresh() {
      // No can() guard here, unlike stores/inbox.ts: the inbox is gated on
      // request.decide, but notifications are NOT permission-gated — the feed is
      // per-user and every authenticated user has one. A session is the only
      // precondition, and without one there is nothing to fetch (the request
      // would 401 and tear the session down).
      const auth = useAuthStore()
      if (!auth.isAuthenticated) {
        this.items = []
        this.unreadCount = 0
        this.loading = false
        this.error = false
        return
      }
      const api = useNotifications()
      this.loading = true
      try {
        const [page, count] = await Promise.all([
          api.list({ limit: BELL_PAGE_SIZE }),
          api.unreadCount()
        ])
        // Committed only once both calls resolved: a half-applied refresh would
        // leave a list and a badge that disagree with each other.
        this.items = page.data
        this.unreadCount = count
        this.error = false
      } catch {
        // Keep the last known items/count rather than zeroing the badge on a
        // transient failure (precedent: stores/inbox.ts). useApiClient has
        // already toasted the error. The flag only lets the bell offer a retry
        // when it has nothing else to show.
        this.error = true
      } finally {
        this.loading = false
      }
    }
  }
})
