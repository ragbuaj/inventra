<script setup lang="ts">
import { parseDate } from '@internationalized/date'
import type { DateValue } from '@internationalized/date'

/**
 * Date field backed by the Nuxt UI calendar (`UCalendar`) picker. The canonical
 * value is an ISO `YYYY-MM-DD` string (empty when unset), so it is a drop-in for
 * the old native `<UInput type="date">`. Users can pick from the calendar popover
 * or type the ISO date directly; both keep the same v-model contract.
 */
const props = withDefaults(defineProps<{
  modelValue?: string | null
  disabled?: boolean
  placeholder?: string
  testid?: string
  ariaLabel?: string
}>(), {
  modelValue: '',
  disabled: false,
  placeholder: 'YYYY-MM-DD',
  testid: undefined,
  ariaLabel: undefined
})

const emit = defineEmits<{ 'update:modelValue': [string] }>()

const { t } = useI18n()
const open = ref(false)

function toCal(s?: string | null): DateValue | undefined {
  if (!s) return undefined
  try {
    return parseDate(s)
  } catch {
    return undefined
  }
}

const calValue = computed(() => toCal(props.modelValue))

function onText(v: string | number) {
  emit('update:modelValue', String(v))
}

// UCalendar's update handler is typed for single/range/multiple modes; this
// picker only uses single mode, so narrow to the lone DateValue.
type CalUpdate = DateValue | DateValue[] | { start?: DateValue, end?: DateValue } | null | undefined

function onPick(v: CalUpdate) {
  const d = Array.isArray(v) ? v[0] : (v && typeof v === 'object' && 'start' in v ? v.start : v)
  if (d) {
    emit('update:modelValue', d.toString())
    open.value = false
  }
}
</script>

<template>
  <UInput
    :model-value="modelValue ?? ''"
    :disabled="disabled"
    :placeholder="placeholder"
    :aria-label="ariaLabel"
    :data-testid="testid"
    class="w-full"
    @update:model-value="onText"
  >
    <template #trailing>
      <UPopover v-model:open="open">
        <UButton
          color="neutral"
          variant="link"
          size="xs"
          icon="i-lucide-calendar"
          :disabled="disabled"
          :aria-label="t('common.pickDate')"
          tabindex="-1"
        />
        <template #content>
          <UCalendar
            :model-value="calValue"
            class="p-2"
            @update:model-value="onPick"
          />
        </template>
      </UPopover>
    </template>
  </UInput>
</template>
