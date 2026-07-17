import { notificationLink, type NotificationLinkSource } from '~/constants/notificationMeta'

/**
 * Where a notification row navigates to, or null when it is not navigable.
 *
 * `notificationLink()` (constants/notificationMeta.ts) answers with the route
 * the entity lives on; the extra gate here is authorization. `requests` rows
 * resolve to /approval, which is gated on `request.decide` (definePageMeta in
 * pages/approval.vue). An `approval_pending` recipient always holds that
 * permission, but an `approval_decided` recipient is the MAKER, who often does
 * not — sending them there would land them on a 403. There is no maker-facing
 * request-detail route today, so such a row is click-to-mark-read only.
 *
 * Lives in a composable rather than in either consumer because the bell
 * (components/NotificationBell.vue) and the full feed (pages/notifications.vue)
 * must gate identically — a copy in each is a permission bug waiting to drift.
 * It is not in notificationMeta.ts because the gate needs `useCan()`, and that
 * catalog is deliberately a pure, Nuxt-free module.
 */
export function useNotificationLink() {
  const can = useCan()

  function resolveLink(n: NotificationLinkSource): string | null {
    const link = notificationLink(n)
    if (link === '/approval' && !can('request.decide')) return null
    return link
  }

  return { resolveLink }
}
