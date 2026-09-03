<script setup lang="ts">
import type { ComponentPublicInstance } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'

/**
 * Renders an accumulating card list for the compact (mobile) layout. The tail
 * — sentinel plus status region — is delegated to InfiniteScrollSentinel.
 *
 * DOM windowing is *conditional*. Below `threshold` every item stays in the
 * DOM, which keeps browser find-in-page, scroll anchoring, and screen-reader
 * traversal intact. Windowing only switches on once the list is large enough
 * for those costs to be worth paying.
 */
const props = withDefaults(defineProps<{
  items: unknown[]
  loadingMore?: boolean
  done?: boolean
  error?: boolean
  /** Item count from which DOM windowing switches on. */
  threshold?: number
  /** Rough height of one item, in px — the virtualizer's starting guess. */
  estimateSize?: number
  /** The scrolling ancestor. Null falls back to the viewport. */
  scrollParent?: HTMLElement | null
  testid?: string
}>(), {
  loadingMore: false,
  done: false,
  error: false,
  threshold: 200,
  estimateSize: 96,
  scrollParent: null,
  testid: 'infinite-list'
})

const emit = defineEmits<{ 'load-more': [], 'retry': [] }>()

const virtualized = computed(() => props.items.length >= props.threshold)

const listEl = ref<HTMLElement | null>(null)
// Distance from the top of the scroll container's content to the top of this
// list. Without it the virtualizer places rows as if the list started at the
// very top of the scroller, and everything above it (the filter bar, the page
// header) shifts the window by exactly that much.
const listOffset = ref(0)

function measureListOffset() {
  const el = listEl.value
  const parent = props.scrollParent
  if (!el || !parent) {
    listOffset.value = 0
    return
  }
  listOffset.value = el.getBoundingClientRect().top - parent.getBoundingClientRect().top + parent.scrollTop
}

const virtualizer = useVirtualizer(computed(() => ({
  count: virtualized.value ? props.items.length : 0,
  getScrollElement: () => props.scrollParent,
  estimateSize: () => props.estimateSize,
  overscan: 6,
  scrollMargin: listOffset.value
})))

const virtualItems = computed(() => virtualizer.value.getVirtualItems())
const totalSize = computed(() => virtualizer.value.getTotalSize())

function measureItem(el: Element | ComponentPublicInstance | null) {
  if (el instanceof HTMLElement) virtualizer.value.measureElement(el)
}

// Anything above the list changing height moves its start point: the bulk
// action bar appearing on selection, the filter bar wrapping to a second line,
// an orientation change. A stale `scrollMargin` makes the virtualizer compute
// its window from the wrong origin and render the wrong slice, so observe the
// scroll container and re-measure rather than only measuring at mount.
let resizeObserver: ResizeObserver | null = null

function observeResize() {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (!import.meta.client || typeof ResizeObserver === 'undefined') return
  const parent = props.scrollParent
  if (!parent) return
  resizeObserver = new ResizeObserver(() => measureListOffset())
  resizeObserver.observe(parent)
  if (listEl.value) resizeObserver.observe(listEl.value)
}

onMounted(() => {
  measureListOffset()
  observeResize()
})
onUnmounted(() => resizeObserver?.disconnect())

watch(() => props.scrollParent, () => {
  measureListOffset()
  observeResize()
})
watch(virtualized, () => nextTick(() => {
  measureListOffset()
  observeResize()
}))
</script>

<template>
  <div :data-testid="props.testid">
    <!-- Plain: every item in the DOM. -->
    <div
      v-if="!virtualized"
      data-testid="infinite-list-plain"
    >
      <slot
        v-for="(item, index) in props.items"
        name="item"
        :item="item"
        :index="index"
      />
    </div>

    <!-- Windowed: only the visible slice plus overscan. -->
    <div
      v-else
      ref="listEl"
      data-testid="infinite-list-virtual"
      class="relative w-full"
      :style="{ height: `${totalSize}px` }"
    >
      <div
        v-for="v in virtualItems"
        :key="v.key"
        :ref="measureItem"
        :data-index="v.index"
        class="absolute top-0 left-0 w-full"
        :style="{ transform: `translateY(${v.start - listOffset}px)` }"
      >
        <slot
          name="item"
          :item="props.items[v.index]"
          :index="v.index"
        />
      </div>
    </div>

    <InfiniteScrollSentinel
      :loading-more="props.loadingMore"
      :done="props.done"
      :error="props.error"
      :has-items="props.items.length > 0"
      :scroll-parent="props.scrollParent"
      testid="infinite-list"
      @load-more="emit('load-more')"
      @retry="emit('retry')"
    />
  </div>
</template>
