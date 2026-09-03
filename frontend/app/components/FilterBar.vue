<script setup lang="ts">
/**
 * Shared filter bar for every data list screen.
 *
 * Regular width (>= md) renders exactly what each page used to hand-roll: a
 * card holding the search box, the page's own advanced controls inline, a
 * reset button, and a trailing area for things like a table/grid switch.
 *
 * Compact width (< md) shows only the search box, a filter button carrying a
 * count badge, and the trailing area. The advanced controls move into a
 * bottom slideover.
 *
 * Filters apply live — the controls stay bound to the page's own refs through
 * the `filters` slot, so changing one updates the list behind the panel
 * immediately. The footer button only closes the panel. A draft/apply model
 * would require every page to duplicate its filter state so this component
 * could buffer it, and it introduces the "changed it but forgot to press
 * Apply" class of bug for no gain.
 */
const props = withDefaults(defineProps<{
  search?: string
  searchPlaceholder?: string
  /** Active *advanced* filters only — the page computes it; search is excluded. */
  activeCount?: number
  /**
   * Whether the reset control is offered. Pages bind their own "anything is
   * filtered" condition, which usually includes the search term — that's
   * deliberately *not* `activeCount`, since the badge counts only the
   * advanced filters hidden behind the button.
   */
  showReset?: boolean
  /** Reset label. Screens differ (see each screen's mockup). */
  resetLabel?: string
  /** Optional row count, shown on the compact panel's close button. */
  total?: number
  testid?: string
}>(), {
  search: '',
  searchPlaceholder: '',
  activeCount: 0,
  showReset: false,
  resetLabel: '',
  testid: 'filter-bar'
})

const emit = defineEmits<{ 'update:search': [string], 'reset': [] }>()

const { t } = useI18n()
const isCompact = useIsCompact()
const open = ref(false)

// Leaving the compact breakpoint while the panel is open would strand an
// overlay on top of a layout that already shows every control inline.
watch(isCompact, (compact) => {
  if (!compact) open.value = false
})

const searchPlaceholderText = computed(() => props.searchPlaceholder || t('common.search'))
const resetLabelText = computed(() => props.resetLabel || t('common.reset'))
const hasActive = computed(() => props.activeCount > 0)
const toggleLabel = computed(() => hasActive.value
  ? t('common.filterBar.ariaActive', { n: props.activeCount })
  : t('common.filterBar.aria'))
const applyLabel = computed(() => props.total === undefined
  ? t('common.filterBar.apply')
  : t('common.filterBar.applyCount', { n: props.total }))
</script>

<template>
  <div class="bg-default border border-default rounded-[13px] p-[14px] shadow-sm mb-4 flex items-center gap-2.5 flex-wrap">
    <UInput
      :model-value="props.search"
      icon="i-lucide-search"
      :placeholder="searchPlaceholderText"
      class="flex-1 min-w-[180px]"
      :data-testid="`${props.testid}-search`"
      @update:model-value="emit('update:search', String($event))"
    />

    <!-- Regular: every advanced control inline, as before. -->
    <template v-if="!isCompact">
      <slot name="filters" />
      <UButton
        v-if="props.showReset"
        color="error"
        variant="ghost"
        icon="i-lucide-x"
        :label="resetLabelText"
        :data-testid="`${props.testid}-reset`"
        @click="emit('reset')"
      />
      <div class="flex-1" />
    </template>

    <!-- Compact: one button standing in for the whole advanced set. -->
    <UChip
      v-else
      :text="props.activeCount"
      :show="hasActive"
      size="2xl"
      class="flex-none"
    >
      <UButton
        icon="i-lucide-sliders-horizontal"
        :color="hasActive ? 'primary' : 'neutral'"
        :variant="hasActive ? 'soft' : 'outline'"
        square
        :aria-label="toggleLabel"
        :aria-expanded="open"
        :data-testid="`${props.testid}-toggle`"
        @click="() => { open = true }"
      />
    </UChip>

    <slot name="trailing" />

    <USlideover
      v-if="isCompact"
      v-model:open="open"
      side="bottom"
      :title="$t('common.filterBar.title')"
      :ui="{ content: 'max-h-[85vh]' }"
    >
      <template #body>
        <!-- Pages size their inline controls with `min-w-[…]`; inside the panel
             each one should simply fill the sheet. -->
        <div
          class="flex flex-col gap-3 [&>*]:w-full [&>*]:min-w-0"
          :data-testid="`${props.testid}-panel`"
        >
          <slot name="filters" />
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton
            v-if="props.showReset"
            color="error"
            variant="ghost"
            icon="i-lucide-x"
            :label="resetLabelText"
            :data-testid="`${props.testid}-reset`"
            @click="emit('reset')"
          />
          <UButton
            :label="applyLabel"
            :data-testid="`${props.testid}-apply`"
            @click="() => { open = false }"
          />
        </div>
      </template>
    </USlideover>
  </div>
</template>
