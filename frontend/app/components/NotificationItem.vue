<script setup lang="ts">
import { notificationMeta, notificationI18nParams } from '~/constants/notificationMeta'
import type { NotificationRow } from '~/composables/api/useNotifications'
import { formatRelativeTime } from '~/utils/format'

/**
 * One row of the notification feed: type icon chip + resolved sentence +
 * relative time, tinted while unread.
 *
 * Shared by the bell dropdown (`dense`) and the full feed page, so the two can
 * never drift apart visually. The markup is the App Shell mockup's bell row
 * (docs/design/App Shell.dc.html:129) — `dense` is that row verbatim; the
 * default is the same row at page scale (the mockup has no full-page feed).
 */
const props = withDefaults(defineProps<{
  notification: NotificationRow
  /** Bell-dropdown scale. Default is the roomier page scale. */
  dense?: boolean
  testid?: string
}>(), {
  dense: false,
  testid: 'notification-row'
})

defineEmits<{ select: [NotificationRow] }>()

const { t, locale } = useI18n()

const meta = computed(() => notificationMeta(props.notification.type))
const message = computed(() => t(meta.value.i18nKey, notificationI18nParams(props.notification, t)))
const relative = computed(() => formatRelativeTime(props.notification.created_at, locale.value))
const unread = computed(() => !props.notification.read_at)
</script>

<template>
  <button
    :class="[
      'flex w-full border-b border-default text-left cursor-pointer hover:bg-muted transition-colors',
      dense ? 'gap-[11px] px-[15px] py-3' : 'gap-3 px-4 py-3.5',
      unread ? 'bg-primary/5' : ''
    ]"
    :data-testid="testid"
    @click="$emit('select', notification)"
  >
    <span
      :class="[
        'rounded-[8px] flex items-center justify-center flex-none',
        dense ? 'w-8 h-8' : 'size-9',
        meta.iconBg
      ]"
    >
      <UIcon
        :name="meta.icon"
        :class="[dense ? 'size-[15px]' : 'size-[17px]', meta.iconColor]"
      />
    </span>
    <div class="min-w-0 flex-1">
      <div
        :class="[
          'font-medium leading-snug text-default',
          dense ? 'text-[13px]' : 'text-[13.5px]'
        ]"
      >
        {{ message }}
      </div>
      <div class="text-[12px] text-dimmed mt-0.5">
        {{ relative }}
      </div>
    </div>
    <!-- Unread marker: the tint alone is easy to miss on a full-width page row.
         Suppressed in the bell, whose narrow rows read the tint fine. -->
    <span
      v-if="!dense && unread"
      class="size-2 rounded-full bg-primary flex-none mt-1.5"
      data-testid="notification-unread-dot"
    />
  </button>
</template>
