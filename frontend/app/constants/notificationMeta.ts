/**
 * Type-to-presentation catalog for the notification feed.
 *
 * The API returns a `type` plus free-form `params` and never a rendered
 * sentence (backend/internal/notification/dto.go). Everything visual — icon,
 * tint, message key, deep link — is therefore a frontend concern and lives
 * here, in one place, instead of being baked into the data as the old
 * `app/mock/notifications.ts` fixture did. Same idiom as `approvalMeta.ts`.
 */

/** Backend `shared.notification_type` values. */
export type NotificationType = 'approval_pending' | 'approval_decided' | 'maintenance_due' | 'asset_returned'

export const NOTIFICATION_TYPE_KEYS: NotificationType[] = ['approval_pending', 'approval_decided', 'maintenance_due', 'asset_returned']

export interface NotificationMeta {
  /** Lucide icon name for the leading badge. */
  icon: string
  /** Tailwind class for the badge background (semantic tokens, never literal colors). */
  iconBg: string
  /** Tailwind class for the icon color. */
  iconColor: string
  /** i18n message key; interpolate with `notificationI18nParams()`. */
  i18nKey: string
}

/**
 * Icons and tints follow the App Shell mockup (docs/design/App Shell.dc.html
 * lines 276-287): approval -> check icon on a primary tint, wrench -> warning
 * tint, box -> muted. The mockup shows three rows; `approval_decided` joins the
 * approval family it belongs to.
 */
export const NOTIFICATION_META: Record<NotificationType, NotificationMeta> = {
  approval_pending: { icon: 'i-lucide-check-square', iconBg: 'bg-primary/10', iconColor: 'text-primary', i18nKey: 'notifications.item.approval_pending' },
  approval_decided: { icon: 'i-lucide-clipboard-check', iconBg: 'bg-primary/10', iconColor: 'text-primary', i18nKey: 'notifications.item.approval_decided' },
  maintenance_due: { icon: 'i-lucide-wrench', iconBg: 'bg-warning/15', iconColor: 'text-warning', i18nKey: 'notifications.item.maintenance_due' },
  asset_returned: { icon: 'i-lucide-package', iconBg: 'bg-muted', iconColor: 'text-muted', i18nKey: 'notifications.item.asset_returned' }
}

/**
 * Fallback for a type this build does not know. The backend enum can gain a
 * value before the frontend ships support for it, and a bell that throws on an
 * unknown row would take the whole feed down with it — so an unknown type
 * degrades to a neutral bell with a generic message.
 */
export const UNKNOWN_NOTIFICATION_META: NotificationMeta = {
  icon: 'i-lucide-bell',
  iconBg: 'bg-muted',
  iconColor: 'text-muted',
  i18nKey: 'notifications.item.unknown'
}

/** Presentation for `type`, degrading to a neutral bell for unknown values. */
export function notificationMeta(type: string): NotificationMeta {
  return NOTIFICATION_META[type as NotificationType] ?? UNKNOWN_NOTIFICATION_META
}

/**
 * Params whose value is a backend enum key, not display text, mapped to the
 * i18n prefix that translates it. The consumer handlers store raw enum values
 * (`request_type: "asset_create"`, `status: "approved"`) precisely so the
 * message is not frozen into one locale — interpolating them verbatim would
 * render "Pengajuan asset_create Anda telah approved". Every other param
 * (asset_tag, asset_name, due_date, step) is already display-ready.
 */
export const NOTIFICATION_PARAM_LOOKUPS: Partial<Record<NotificationType, Record<string, string>>> = {
  approval_pending: { request_type: 'approval.type' },
  approval_decided: { request_type: 'approval.type', status: 'approval.status' }
}

/** The subset of a notification row this catalog reads. */
export interface NotificationLinkSource {
  type: string
  params?: Record<string, string> | null
  entity_type?: string | null
  entity_id?: string | null
}

/**
 * Interpolation values for `notificationMeta(n.type).i18nKey`, with enum-valued
 * params resolved through `translate`. `translate` is injected rather than
 * pulled from `useI18n()` so this stays a pure function testable outside a Nuxt
 * runtime.
 */
export function notificationI18nParams(n: NotificationLinkSource, translate: (key: string) => string): Record<string, string> {
  const lookups = NOTIFICATION_PARAM_LOOKUPS[n.type as NotificationType] ?? {}
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(n.params ?? {})) {
    const prefix = lookups[key]
    out[key] = prefix ? translate(`${prefix}.${value}`) : String(value)
  }
  return out
}

/**
 * Deep-link target for a notification, or null when none can be derived.
 *
 * Two caveats worth knowing, both verified against the routes that actually
 * exist in `app/pages/`:
 *
 *  - `assets` links go through `params.asset_tag`, NOT `entity_id`. The
 *    consumer sets `entity_id` to the asset UUID, but the detail route is
 *    `/assets/[tag]` and is keyed by asset_tag (useAssets.getByTag), so a UUID
 *    would 404. A row without an asset_tag param therefore has no link.
 *  - `requests` links resolve to `/approval`, the only requests-facing route.
 *    That page is gated on `request.decide` (definePageMeta in approval.vue),
 *    which every `approval_pending` recipient holds by construction but an
 *    `approval_decided` recipient (the maker) may not. Callers that navigate
 *    should gate on `useCan('request.decide')`; there is no maker-facing
 *    request-detail route to link to today.
 */
export function notificationLink(n: NotificationLinkSource): string | null {
  switch (n.entity_type) {
    case 'requests':
      return '/approval'
    case 'assets': {
      const tag = n.params?.asset_tag
      return tag ? `/assets/${encodeURIComponent(tag)}` : null
    }
    default:
      return null
  }
}
