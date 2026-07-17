<script setup lang="ts">
import type { NotificationRow } from '~/composables/api/useNotifications'

const { t } = useI18n()
const localePath = useLocalePath()
const notifs = useNotifications()
const store = useNotificationsStore()
// The /approval authorization gate is shared with pages/notifications.vue so
// the two feeds can never disagree about what is navigable.
const { resolveLink } = useNotificationLink()

const open = ref(false)

const list = computed(() => store.items)
const unread = computed(() => store.unreadCount)

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
            <NotificationItem
              v-for="n in list"
              :key="n.id"
              :notification="n"
              dense
              @select="handleRowClick"
            />
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
