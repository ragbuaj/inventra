<script setup lang="ts">
import type { RowAction } from '~/types'

// Shared kebab (⋮) row-actions trigger — a `UDropdownMenu` grouped via the
// same `buildActionGroups` helper a hand-rolled table's right-click
// `UContextMenu` can reuse directly. Renders nothing when there are no items
// (e.g. the caller has no permission for any action on this row).
const props = defineProps<{
  items: RowAction[]
}>()

const { t } = useI18n()

const groups = computed(() => buildActionGroups(props.items))
</script>

<template>
  <UDropdownMenu
    v-if="groups.length"
    :items="groups"
    :content="{ align: 'end' }"
  >
    <UButton
      icon="i-lucide-ellipsis-vertical"
      color="neutral"
      variant="ghost"
      size="xs"
      :aria-label="t('common.actions')"
    />
  </UDropdownMenu>
</template>
