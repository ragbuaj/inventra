<script setup lang="ts">
/**
 * The tail of an infinitely scrolling list: an IntersectionObserver sentinel
 * that asks for the next page, plus a live status region reporting loading /
 * end-of-list / error.
 *
 * Shared by the card path (InfiniteList) and the table path (ResourceTable),
 * so the observer's lifecycle rules live in exactly one place.
 */
const props = withDefaults(defineProps<{
  loadingMore?: boolean
  done?: boolean
  error?: boolean
  /** True once the list holds items — gates the end-of-list marker. */
  hasItems?: boolean
  /**
   * Stops automatic loading and offers an explicit button instead. The table
   * path uses this at its row cap so the DOM can't grow without the user
   * asking for it.
   */
  manual?: boolean
  /** The scrolling ancestor. Null falls back to the viewport. */
  scrollParent?: HTMLElement | null
  testid?: string
}>(), {
  loadingMore: false,
  done: false,
  error: false,
  hasItems: false,
  manual: false,
  scrollParent: null,
  testid: 'infinite'
})

const emit = defineEmits<{ 'load-more': [], 'retry': [] }>()

const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

const blocked = computed(() => props.loadingMore || props.done || props.error || props.manual)

function requestMore() {
  if (blocked.value) return
  emit('load-more')
}

function teardownObserver() {
  observer?.disconnect()
  observer = null
}

function setupObserver() {
  teardownObserver()
  if (!import.meta.client || typeof IntersectionObserver === 'undefined') return
  if (!sentinel.value) return
  observer = new IntersectionObserver((entries) => {
    if (entries.some(e => e.isIntersecting)) requestMore()
  }, {
    root: props.scrollParent ?? null,
    // Start fetching before the user actually reaches the bottom.
    rootMargin: '400px 0px'
  })
  observer.observe(sentinel.value)
}

onMounted(setupObserver)
onUnmounted(teardownObserver)

// The scroll parent is resolved by the page after mount, so rebuild on it.
watch(() => props.scrollParent, setupObserver)

// IntersectionObserver only calls back when the intersection *changes*. A
// sentinel that was already on screen while loading was blocked therefore
// never fires again once the block clears, and the list stalls forever. This
// is not hypothetical: at mount `rows` and `total` are both 0, so `done` is
// true, the first callback is refused, and nothing re-triggers it after the
// first page lands. Re-observing re-evaluates the current intersection, so
// watch every blocking input, not just `loadingMore`.
watch(blocked, (isBlocked) => {
  if (!isBlocked) nextTick(setupObserver)
})
</script>

<template>
  <div>
    <div
      ref="sentinel"
      aria-hidden="true"
      class="h-px w-full"
      :data-testid="`${props.testid}-sentinel`"
    />

    <div
      role="status"
      aria-live="polite"
      class="py-4 text-center text-sm text-muted"
      :data-testid="`${props.testid}-status`"
    >
      <span
        v-if="props.loadingMore"
        class="inline-flex items-center gap-2"
        :data-testid="`${props.testid}-loading`"
      >
        <UIcon
          name="i-lucide-loader-circle"
          class="size-4 animate-spin"
        />
        {{ $t('common.loading') }}
      </span>

      <span
        v-else-if="props.error"
        class="inline-flex items-center gap-2"
        :data-testid="`${props.testid}-error`"
      >
        {{ $t('common.loadError') }}
        <UButton
          size="xs"
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-cw"
          :label="$t('common.retry')"
          :data-testid="`${props.testid}-retry`"
          @click="emit('retry')"
        />
      </span>

      <span
        v-else-if="props.done && props.hasItems"
        :data-testid="`${props.testid}-end`"
      >
        {{ $t('common.endOfList') }}
      </span>

      <UButton
        v-else-if="props.manual"
        size="sm"
        color="neutral"
        variant="outline"
        icon="i-lucide-chevron-down"
        :label="$t('common.loadMore')"
        :data-testid="`${props.testid}-load-more`"
        @click="emit('load-more')"
      />
    </div>
  </div>
</template>
