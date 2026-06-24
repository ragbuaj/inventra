<script setup lang="ts">
import { assetStatusMeta, approvalStatusMeta } from '~/utils/statusMeta'

const props = withDefaults(defineProps<{ status: string, kind?: 'asset' | 'approval' }>(), {
  kind: 'asset'
})
const { t } = useI18n()
const meta = computed(() => {
  const map = props.kind === 'approval' ? approvalStatusMeta : assetStatusMeta
  return map[props.status] ?? { color: 'neutral' as const, labelKey: '' }
})
const label = computed(() => meta.value.labelKey ? t(meta.value.labelKey) : props.status)
</script>

<template>
  <UBadge
    :color="meta.color"
    variant="subtle"
  >
    {{ label }}
  </UBadge>
</template>
