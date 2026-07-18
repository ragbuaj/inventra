<script setup lang="ts">
const props = defineProps<{ total: number, limit: number, offset: number }>()
const emit = defineEmits<{ 'update:offset': [number] }>()

// Cap the visible page-number buttons; the window slides to keep the current
// page centred where possible.
const MAX_PAGE_BUTTONS = 3

const page = computed({
  get: () => Math.floor(props.offset / props.limit) + 1,
  set: (p: number) => emit('update:offset', (p - 1) * props.limit)
})
const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.limit)))
const from = computed(() => props.total === 0 ? 0 : props.offset + 1)
const to = computed(() => Math.min(props.offset + props.limit, props.total))

// The sliding window of at most MAX_PAGE_BUTTONS consecutive page numbers.
const pages = computed<number[]>(() => {
  const tp = totalPages.value
  if (tp <= MAX_PAGE_BUTTONS) {
    return Array.from({ length: tp }, (_, i) => i + 1)
  }
  let start = page.value - Math.floor(MAX_PAGE_BUTTONS / 2)
  if (start < 1) start = 1
  if (start > tp - MAX_PAGE_BUTTONS + 1) start = tp - MAX_PAGE_BUTTONS + 1
  return Array.from({ length: MAX_PAGE_BUTTONS }, (_, i) => start + i)
})

function goTo(p: number) {
  if (p < 1 || p > totalPages.value || p === page.value) return
  page.value = p
}
</script>

<template>
  <div class="flex flex-col gap-3 px-4 py-3 border-t border-default sm:flex-row sm:items-center sm:justify-between sm:gap-4">
    <p class="text-sm text-muted">
      {{ $t('common.showingRange', { from, to, total }) }}
    </p>
    <div class="flex items-center gap-1 self-end sm:self-auto">
      <UButton
        icon="i-lucide-chevron-left"
        color="neutral"
        variant="ghost"
        size="sm"
        square
        :disabled="page <= 1"
        :aria-label="$t('common.prevPage')"
        data-testid="pagination-prev"
        @click="goTo(page - 1)"
      />
      <UButton
        v-for="p in pages"
        :key="p"
        :color="p === page ? 'primary' : 'neutral'"
        :variant="p === page ? 'solid' : 'ghost'"
        size="sm"
        square
        :aria-current="p === page ? 'page' : undefined"
        :aria-label="$t('common.goToPage', { n: p })"
        data-testid="pagination-page"
        @click="goTo(p)"
      >
        {{ p }}
      </UButton>
      <UButton
        icon="i-lucide-chevron-right"
        color="neutral"
        variant="ghost"
        size="sm"
        square
        :disabled="page >= totalPages"
        :aria-label="$t('common.nextPage')"
        data-testid="pagination-next"
        @click="goTo(page + 1)"
      />
    </div>
  </div>
</template>
