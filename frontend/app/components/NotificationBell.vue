<script setup lang="ts">
import { notificationMeta, notificationI18nParams, notificationLink } from '~/constants/notificationMeta'
import type { NotificationRow } from '~/composables/api/useNotifications'
import { formatRelativeTime } from '~/utils/format'

const { t, locale } = useI18n()
const localePath = useLocalePath()
const can = useCan()
const notifs = useNotifications()
const store = useNotificationsStore()

const open = ref(false)

const list = computed(() => store.items)
const unread = computed(() => store.unreadCount)

/**
 * Where a row navigates to, or null when it is not navigable.
 *
 * notificationLink() answers with the route the entity lives on; the extra gate
 * here is authorization. `requests` rows resolve to /approval, which is gated on
 * `request.decide` (definePageMeta in pages/approval.vue). An `approval_pending`
 * recipient always holds that permission, but an `approval_decided` recipient is
 * the MAKER, who often does not — sending them there would land them on a 403.
 * There is no maker-facing request-detail route today, so such a row is
 * click-to-mark-read only.
 */
function resolveLink(n: NotificationRow): string | null {
  const link = notificationLink(n)
  if (link === '/approval' && !can('request.decide')) return null
  return link
}

async function markReadOnce(n: NotificationRow) {
  if (n.read_at) return
  try {
    await notifs.markRead(n.id)
    await store.refresh()
  } catch {
    // useApiClient already raised the error toast; a failed mark must not
    // swallow the navigation the click asked for.
  }
}

async function handleRowClick(n: NotificationRow) {
  const target = resolveLink(n)
  open.value = false
  await markReadOnce(n)
  if (target) await navigateTo(localePath(target))
}

async function handleMarkRead() {
  open.value = false
  try {
    await notifs.markAllRead()
  } catch {
    // toasted centrally
  }
  await store.refresh()
}

async function handleViewAll() {
  open.value = false
  await navigateTo(localePath('/notifications'))
}
</script>

<template>
  <UPopover v-model:open="open">
    <div class="relative">
      <button
        class="flex items-center justify-center w-9 h-9 rounded-[9px] border border-default bg-transparent text-muted cursor-pointer hover:bg-muted hover:text-default transition-colors"
        :title="t('notifications.title')"
        data-testid="notification-bell"
      >
        <UIcon
          name="i-lucide-bell"
          class="size-[17px]"
        />
      </button>
      <!-- Ring is the topbar background, so the badge reads as punched out of it
           (mockup: border 2px solid var(--bg-base); the topbar is bg-default). -->
      <span
        v-if="unread > 0"
        class="absolute -top-[3px] -right-[3px] min-w-[17px] h-[17px] px-1 flex items-center justify-center text-[10px] font-bold text-inverted bg-error border-2 border-[var(--ui-bg)] rounded-full pointer-events-none"
        data-testid="notification-badge"
      >
        {{ unread }}
      </span>
    </div>

    <template #content>
      <div class="w-[330px] overflow-hidden rounded-[13px]">
        <!-- Header -->
        <div class="flex items-center justify-between px-[15px] py-[13px] border-b border-default">
          <span class="text-[14px] font-semibold">{{ t('notifications.title') }}</span>
          <button
            class="text-[12px] font-medium text-primary bg-transparent border-0 cursor-pointer hover:underline"
            data-testid="notification-mark-all"
            @click="handleMarkRead"
          >
            {{ t('notifications.markRead') }}
          </button>
        </div>

        <!-- List -->
        <div class="max-h-[300px] overflow-y-auto">
          <div
            v-if="store.loading"
            class="flex flex-col gap-2 p-[15px]"
            data-testid="notification-loading"
          >
            <USkeleton
              v-for="n in 3"
              :key="n"
              class="h-[46px] w-full rounded-[8px]"
            />
          </div>

          <template v-else-if="list.length > 0">
            <button
              v-for="n in list"
              :key="n.id"
              :class="[
                'flex gap-[11px] w-full px-[15px] py-3 border-b border-default text-left cursor-pointer hover:bg-muted transition-colors',
                !n.read_at ? 'bg-primary/5' : ''
              ]"
              data-testid="notification-row"
              @click="handleRowClick(n)"
            >
              <span
                :class="['w-8 h-8 rounded-[8px] flex items-center justify-center flex-none', notificationMeta(n.type).iconBg]"
              >
                <UIcon
                  :name="notificationMeta(n.type).icon"
                  :class="['size-[15px]', notificationMeta(n.type).iconColor]"
                />
              </span>
              <div class="min-w-0">
                <div class="text-[13px] font-medium leading-snug text-default">
                  {{ t(notificationMeta(n.type).i18nKey, notificationI18nParams(n, t)) }}
                </div>
                <div class="text-[12px] text-dimmed mt-0.5">
                  {{ formatRelativeTime(n.created_at, locale) }}
                </div>
              </div>
            </button>
          </template>

          <div
            v-else-if="store.error"
            class="py-[34px] px-5 text-center"
            data-testid="notification-load-error"
          >
            <div class="text-sm font-semibold mb-2">
              {{ t('notifications.loadError') }}
            </div>
            <UButton
              size="sm"
              variant="soft"
              :label="t('notifications.retry')"
              data-testid="notification-retry"
              @click="store.refresh()"
            />
          </div>

          <div
            v-else
            class="py-[34px] px-5 text-center"
            data-testid="notification-empty"
          >
            <div class="size-12 mx-auto mb-3 rounded-[13px] bg-muted text-dimmed flex items-center justify-center">
              <UIcon
                name="i-lucide-bell-off"
                class="size-6"
              />
            </div>
            <div class="text-[12.5px] text-muted">
              {{ t('notifications.empty') }}
            </div>
          </div>
        </div>

        <!-- Footer -->
        <button
          class="w-full py-[11px] text-[13px] font-medium text-primary bg-transparent border-0 cursor-pointer hover:bg-muted transition-colors"
          data-testid="notification-view-all"
          @click="handleViewAll"
        >
          {{ t('notifications.viewAll') }}
        </button>
      </div>
    </template>
  </UPopover>
</template>
