<script setup lang="ts">
import { parseDate } from '@internationalized/date'
import type { DateValue } from '@internationalized/date'
import type { PeriodPreset, PeriodValue } from '~/constants/reportMeta'

const props = defineProps<{
  modelValue: PeriodValue
  /** i18n base for preset labels; both `dashboard.period.*` and `reports.period.*` exist. */
  labelBase?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [PeriodValue] }>()

const { t } = useI18n()

/** Preset backend name → existing camelCase i18n label suffix. */
const PRESET_LABEL_SUFFIX: Record<PeriodPreset, string> = {
  last30: 'last30',
  this_month: 'thisMonth',
  this_quarter: 'thisQuarter',
  ytd: 'ytd'
}

const base = computed(() => props.labelBase ?? 'reports.period')

const items = computed(() => [
  ...(Object.keys(PRESET_LABEL_SUFFIX) as PeriodPreset[]).map(preset => ({
    value: preset,
    label: t(`${base.value}.${PRESET_LABEL_SUFFIX[preset]}`)
  })),
  { value: 'custom' as const, label: t('common.periodCustom') }
])

/** Local mirror of the active option so `custom` can be active before a range is picked. */
const localPreset = ref<PeriodPreset | 'custom'>(props.modelValue.preset)
watch(() => props.modelValue.preset, (v) => {
  localPreset.value = v
})

const isCustom = computed(() => localPreset.value === 'custom')

/** Matches Nuxt UI's UCalendar range model (`DateRange`): keys present, values nullable. */
type Range = { start: DateValue | undefined, end: DateValue | undefined }

function toCal(s?: string): DateValue | undefined {
  return s ? parseDate(s) : undefined
}

const range = shallowRef<Range>({
  start: toCal(props.modelValue.from),
  end: toCal(props.modelValue.to)
})

watch(() => [props.modelValue.from, props.modelValue.to], ([from, to]) => {
  range.value = { start: toCal(from), end: toCal(to) }
})

const rangeOpen = ref(false)

const rangeLabel = computed(() => {
  if (props.modelValue.preset === 'custom' && props.modelValue.from && props.modelValue.to) {
    return `${props.modelValue.from} – ${props.modelValue.to}`
  }
  return t('common.periodCustom')
})

function onSelect(value: PeriodPreset | 'custom') {
  localPreset.value = value
  if (value === 'custom') {
    rangeOpen.value = true
    return
  }
  emit('update:modelValue', { preset: value })
}

/** Handle a UCalendar range update; emit only once both ends are set. */
function onCalendarUpdate(r: Range | null) {
  range.value = r ?? { start: undefined, end: undefined }
  const { start, end } = range.value
  if (start && end) {
    emit('update:modelValue', { preset: 'custom', from: start.toString(), to: end.toString() })
    rangeOpen.value = false
  }
}

// Exposed for deterministic testing (driving the teleported calendar via DOM is brittle).
defineExpose({ onCalendarUpdate })
</script>

<template>
  <div class="flex items-center gap-2">
    <USelect
      :model-value="localPreset"
      :items="items"
      value-key="value"
      data-testid="period-filter-select"
      class="w-44"
      @update:model-value="onSelect"
    />

    <UPopover
      v-if="isCustom"
      v-model:open="rangeOpen"
    >
      <UButton
        color="neutral"
        variant="subtle"
        icon="i-lucide-calendar"
        data-testid="period-filter-range"
      >
        {{ rangeLabel }}
      </UButton>

      <template #content>
        <UCalendar
          :model-value="range"
          range
          class="p-2"
          data-testid="period-filter-calendar"
          @update:model-value="onCalendarUpdate"
        />
      </template>
    </UPopover>
  </div>
</template>
