<script setup lang="ts">
import type { AssetStatus } from '~/types'
import { statusMeta } from '~/constants/assetMeta'

// `status` is typed AssetStatus but stays tolerant of the old Indonesian
// mock-status strings (tersedia/dipinjam/...) still passed by not-yet-rewired
// pages (Tasks 6–9 rewire those pages to the real AssetStatus values).
const props = defineProps<{ status: AssetStatus | string }>()
const { t } = useI18n()

const DOT_CLASS: Record<string, string> = {
  success: 'bg-success',
  info: 'bg-info',
  warning: 'bg-warning',
  error: 'bg-error',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}

const meta = computed(() => statusMeta[props.status as AssetStatus])
const color = computed(() => meta.value?.color ?? 'neutral')
const dotClass = computed(() => DOT_CLASS[color.value] ?? DOT_CLASS.neutral)
</script>

<template>
  <UBadge
    :color="color"
    variant="subtle"
    class="rounded-full gap-1.5"
  >
    <span
      class="size-1.5 rounded-full"
      :class="dotClass"
    />
    {{ t(meta?.labelKey ?? `assets.status.${status}`) }}
  </UBadge>
</template>
