<script setup lang="ts">
import type { NotificationRow } from '~/composables/api/useNotifications'

// No definePageMeta permission gate, unlike every other list screen: the feed
// is per-user and every authenticated user has one (the endpoints carry
// RequireAuth but no RequirePermission). auth.global.ts still guards the route.

/** The server clamps `limit` to 1-100; 20 matches the bell's page size. */
const PAGE_SIZE = 20

type FeedFilter = 'all' | 'unread' | 'read'
const FEED_FILTERS: FeedFilter[] = ['all', 'unread', 'read']

const { t } = useI18n()
const localePath = useLocalePath()
const api = useNotifications()
const store = useNotificationsStore()
// Shared with NotificationBell.vue — see useNotificationLink for the gate.
const { resolveLink } = useNotificationLink()

const rows = ref<NotificationRow[]>([])
const total = ref(0)
const loading = ref(true)
const loadError = ref(false)
const filter = ref<FeedFilter>('all')
const offset = ref(0)
const markingAll = ref(false)

const filterTabs = computed(() => FEED_FILTERS.map(k => ({ key: k, label: t(`notifications.filter.${k}`) })))
// The badge count is the store's — the same number the bell shows, so the two
// can never disagree on this page.
const unread = computed(() => store.unreadCount)

/** `undefined` omits `read` entirely, which the API reads as "the whole feed". */
const readParam = computed<boolean | undefined>(() =>
  filter.value === 'all' ? undefined : filter.value === 'read'
)

async function load() {
  loading.value = true
  loadError.value = false
  try {
    const page = await api.list({ read: readParam.value, limit: PAGE_SIZE, offset: offset.value })
    rows.value = page.data
    total.value = page.total
  } catch {
    // useApiClient already raised the error toast; the flag drives the retry block.
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function markReadOnce(n: NotificationRow) {
  if (n.read_at) return
  try {
    const updated = await api.markRead(n.id)
    // Patch in place rather than reload: a full refetch would flash a skeleton
    // under the click. `n` is the reactive row, so the tint drops immediately.
    n.read_at = updated?.read_at ?? new Date().toISOString()
  } catch {
    // Toasted centrally. A failed mark must not swallow the navigation the
    // click asked for (same contract as the bell).
    return
  }
  await store.refresh()
  // On a filtered tab the row no longer belongs where it sits, and `total`
  // moved with it — only then is a reload worth its cost.
  if (filter.value !== 'all') await load()
}

async function handleRowClick(n: NotificationRow) {
  const target = resolveLink(n)
  await markReadOnce(n)
  if (target) await navigateTo(localePath(target))
}

async function handleMarkAllRead() {
  markingAll.value = true
  try {
    await api.markAllRead()
  } catch {
    // Toasted centrally; still resync below so the UI reflects the real state.
  }
  await Promise.all([store.refresh(), load()])
  markingAll.value = false
}

watch(filter, () => {
  offset.value = 0
  load()
})
watch(offset, () => load())
onMounted(() => load())
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-4">
      <div>
        <div class="flex items-center gap-2.5 mb-[5px]">
          <h1 class="text-[23px] font-bold tracking-tight">
            {{ t('notifications.title') }}
          </h1>
          <UBadge
            v-if="unread > 0"
            color="primary"
            variant="subtle"
            class="rounded-full font-bold"
            data-testid="notifications-unread-badge"
          >
            {{ t('notifications.unreadCount', { n: unread }) }}
          </UBadge>
        </div>
        <p class="text-sm text-muted">
          {{ t('notifications.subtitle') }}
        </p>
      </div>
      <UButton
        icon="i-lucide-check-check"
        color="neutral"
        variant="outline"
        :disabled="unread === 0"
        :loading="markingAll"
        :label="t('notifications.markAllRead')"
        data-testid="notifications-mark-all"
        @click="handleMarkAllRead"
      />
    </div>

    <!-- Filter bar (segmented control — same idiom as pages/approval.vue) -->
    <div class="bg-default border border-default rounded-[13px] shadow-sm p-[14px] mb-4">
      <div class="flex gap-0.5 p-0.5 bg-muted rounded-lg w-full max-w-[360px]">
        <button
          v-for="f in filterTabs"
          :key="f.key"
          class="flex-1 py-1.5 text-xs font-semibold rounded-md transition-colors"
          :class="filter === f.key ? 'bg-default text-default shadow-sm' : 'text-muted hover:text-default'"
          :data-testid="`notifications-tab-${f.key}`"
          @click="filter = f.key"
        >
          {{ f.label }}
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div
      v-if="loading"
      class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      data-testid="notifications-loading"
    >
      <div
        v-for="n in 6"
        :key="n"
        class="flex gap-3 px-4 py-3.5 border-b border-default last:border-b-0"
      >
        <USkeleton class="size-9 rounded-[8px] flex-none" />
        <div class="flex-1 flex flex-col gap-2 py-1">
          <USkeleton class="h-3.5 w-2/3" />
          <USkeleton class="h-3 w-24" />
        </div>
      </div>
    </div>

    <!-- Error -->
    <div
      v-else-if="loadError"
      class="bg-default border border-default rounded-[13px] shadow-sm py-14 px-6 text-center"
      data-testid="notifications-load-error"
    >
      <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-error/10 text-error flex items-center justify-center">
        <UIcon
          name="i-lucide-circle-alert"
          class="size-6"
        />
      </div>
      <div class="text-base font-semibold mb-[18px]">
        {{ t('notifications.loadError') }}
      </div>
      <UButton
        color="neutral"
        variant="outline"
        :label="t('notifications.retry')"
        data-testid="notifications-retry"
        @click="load"
      />
    </div>

    <!-- Populated -->
    <div
      v-else-if="rows.length > 0"
      class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
    >
      <NotificationItem
        v-for="n in rows"
        :key="n.id"
        :notification="n"
        testid="notifications-row"
        @select="handleRowClick"
      />
      <TablePagination
        v-if="total > 0"
        :total="total"
        :limit="PAGE_SIZE"
        :offset="offset"
        @update:offset="offset = $event"
      />
    </div>

    <!-- Empty -->
    <div
      v-else
      class="bg-default border border-default rounded-[14px] shadow-sm py-14 px-6 text-center"
      data-testid="notifications-empty"
    >
      <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
        <UIcon
          name="i-lucide-bell-off"
          class="size-6"
        />
      </div>
      <div class="text-base font-semibold mb-1.5">
        {{ t('notifications.emptyTitle') }}
      </div>
      <div class="text-sm text-muted max-w-[340px] mx-auto">
        {{ t(`notifications.emptySub.${filter}`) }}
      </div>
    </div>
  </div>
</template>
