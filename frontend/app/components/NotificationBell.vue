<script setup lang="ts">
import { notificationMeta, notificationI18nParams } from '~/constants/notificationMeta'
import { formatRelativeTime } from '~/utils/format'

// Minimal wiring only: the composable went sync -> async, so the old
// `computed(() => notifs.list())` no longer type-checks or reacts. State now
// comes from the store (primed at the fetchMe choke point). The full rewrite --
// click to mark-read + navigate, "view all" -> /notifications, loading/error
// states, mockup comparison -- is Task 15.
const { t, locale } = useI18n()
const notifs = useNotifications()
const store = useNotificationsStore()

const open = ref(false)

const list = computed(() => store.items)
const unread = computed(() => store.unreadCount)

async function handleMarkRead() {
  await notifs.markAllRead()
  await store.refresh()
}
</script>

<template>
  <UPopover v-model:open="open">
    <div class="relative">
      <button
        class="flex items-center justify-center w-9 h-9 rounded-[9px] border border-default bg-transparent text-muted cursor-pointer hover:bg-muted hover:text-default transition-colors"
        :title="t('notifications.title')"
      >
        <UIcon
          name="i-lucide-bell"
          class="size-[17px]"
        />
      </button>
      <span
        v-if="unread > 0"
        class="absolute -top-[3px] -right-[3px] min-w-[17px] h-[17px] px-1 flex items-center justify-center text-[10px] font-bold text-inverted bg-error border-2 border-default rounded-full pointer-events-none"
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
            @click="handleMarkRead"
          >
            {{ t('notifications.markRead') }}
          </button>
        </div>

        <!-- List -->
        <div class="max-h-[300px] overflow-y-auto">
          <div
            v-if="list.length === 0"
            class="px-4 py-6 text-center text-[13px] text-muted"
          >
            {{ t('notifications.empty') }}
          </div>
          <div
            v-for="n in list"
            :key="n.id"
            :class="[
              'flex gap-[11px] px-[15px] py-3 border-b border-default cursor-pointer hover:bg-muted transition-colors',
              !n.read_at ? 'bg-primary/5' : ''
            ]"
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
          </div>
        </div>

        <!-- Footer -->
        <button
          class="w-full py-[11px] text-[13px] font-medium text-primary bg-transparent border-0 cursor-pointer hover:bg-muted transition-colors"
          @click="open = false"
        >
          {{ t('notifications.viewAll') }}
        </button>
      </div>
    </template>
  </UPopover>
</template>
