<script setup lang="ts">
const { t } = useI18n()
const notifs = useNotifications()

const open = ref(false)

const list = computed(() => notifs.list())
const unread = computed(() => notifs.unreadCount())

function handleMarkRead() {
  notifs.markAllRead()
}
</script>

<template>
  <UPopover v-model:open="open">
    <div class="relative">
      <button
        class="flex items-center justify-center w-9 h-9 rounded-[9px] border border-default bg-transparent text-muted cursor-pointer hover:bg-muted hover:text-default transition-colors"
        :title="t('notifications.title')"
        @click="open = !open"
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
              !n.read ? 'bg-primary/5' : ''
            ]"
          >
            <span
              :class="['w-8 h-8 rounded-[8px] flex items-center justify-center flex-none', n.iconBg]"
            >
              <UIcon
                :name="n.icon"
                :class="['size-[15px]', n.iconColor]"
              />
            </span>
            <div class="min-w-0">
              <div class="text-[13px] font-medium leading-snug text-default">
                {{ t(n.title) }}
              </div>
              <div class="text-[12px] text-dimmed mt-0.5">
                {{ t(n.time) }}
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
