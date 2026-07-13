<script setup lang="ts">
import { formatThousands } from '~/utils/format'

const model = defineModel<string>({ default: '' })
const props = withDefaults(defineProps<{
  allowNegative?: boolean
  thousandSeparator?: boolean
  decimals?: number
  money?: boolean
  min?: number | string
  max?: number | string
  placeholder?: string
  disabled?: boolean
  id?: string
  dataTestid?: string
}>(), {
  allowNegative: false,
  thousandSeparator: false,
  decimals: 0,
  money: false
})

const useThousands = computed(() => props.money || props.thousandSeparator)

// Keep only allowed characters in a raw string: digits, optional leading '-', optional '.'
function sanitizeRaw(input: string): string {
  let s = input.replace(/[^\d.-]/g, '')
  // minus: only when allowed and only at position 0
  const neg = props.allowNegative && s.startsWith('-')
  s = s.replace(/-/g, '')
  if (props.decimals > 0) {
    const parts = s.split('.')
    const dec = parts.slice(1).join('').slice(0, props.decimals)
    s = parts[0] + (parts.length > 1 ? '.' + dec : '')
  } else {
    s = s.replace(/\./g, '')
  }
  return (neg ? '-' : '') + s
}

function toDisplay(raw: string): string {
  if (!raw) return ''
  if (!useThousands.value) return raw
  const neg = raw.startsWith('-')
  const body = neg ? raw.slice(1) : raw
  const [int, dec] = body.split('.')
  const grouped = formatThousands(int || '0')
  return (neg ? '-' : '') + grouped + (dec !== undefined ? '.' + dec : '')
}

const display = ref(toDisplay(model.value))
watch(model, (v) => {
  display.value = toDisplay(v)
})

function onInput(val: string) {
  // when grouping, strip separators first, then sanitize
  const rawInput = useThousands.value ? parseThousandsKeepDecimal(val) : val
  const raw = sanitizeRaw(rawInput)
  model.value = raw
  display.value = toDisplay(raw)
}

// Strip thousand-group separators ('.') while preserving a leading '-' and a decimal point.
// Grouped display uses '.' both as the group separator and (when decimals are enabled) as the
// decimal point, so only the LAST '.' is treated as a decimal point; all earlier ones are grouping.
function parseThousandsKeepDecimal(v: string): string {
  const neg = v.trim().startsWith('-')
  const cleaned = v.replace(/[^\d.]/g, '')
  const lastDot = cleaned.lastIndexOf('.')
  const hasDecimals = props.decimals > 0
  let result: string
  if (hasDecimals && lastDot !== -1) {
    const intPart = cleaned.slice(0, lastDot).replace(/\./g, '')
    const decPart = cleaned.slice(lastDot + 1)
    result = intPart + '.' + decPart
  } else {
    result = cleaned.replace(/\./g, '')
  }
  return (neg ? '-' : '') + result
}
</script>

<template>
  <UInput
    :id="id"
    :model-value="display"
    inputmode="decimal"
    :placeholder="placeholder"
    :disabled="disabled"
    :min="min"
    :max="max"
    :data-testid="dataTestid"
    class="w-full"
    @update:model-value="onInput(String($event))"
  >
    <template
      v-if="money"
      #leading
    >
      <span class="text-muted text-sm">Rp</span>
    </template>
    <template
      v-if="$slots.trailing"
      #trailing
    >
      <slot name="trailing" />
    </template>
  </UInput>
</template>
