import type { Notification } from '~/mock/notifications'
import { notificationStore } from '~/mock/notifications'

export function useNotifications() {
  function list(): Notification[] {
    return notificationStore.all()
  }

  function unreadCount(): number {
    return notificationStore.all().filter(n => !n.read).length
  }

  function markAllRead(): void {
    for (const n of notificationStore.all()) {
      notificationStore.patch(n.id, { read: true })
    }
  }

  function markRead(id: string): void {
    notificationStore.patch(id, { read: true })
  }

  return {
    list,
    unreadCount,
    markAllRead,
    markRead
  }
}
