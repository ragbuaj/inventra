<script setup lang="ts">
import type { AssetStatus } from '~/types'
import { ASSET_STATUS_META } from '~/mock/assets'

const props = defineProps<{ status: AssetStatus }>()
const { t } = useI18n()

const meta = computed(() => ASSET_STATUS_META[props.status])
</script>

<template>
  <UBadge
    :color="meta.tone"
    variant="subtle"
    class="rounded-full gap-1.5"
  >
    <span
      class="size-1.5 rounded-full"
      :class="meta.dot"
    />
    {{ t(`assets.status.${status}`) }}
  </UBadge>
</template>
