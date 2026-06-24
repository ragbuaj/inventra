<script setup lang="ts">
const props = withDefaults(defineProps<{ count?: number }>(), { count: 0 })
</script>

<template>
  <UChip
    :show="props.count > 0"
    :text="String(props.count)"
    size="2xl"
    color="error"
  >
    <UButton
      color="neutral"
      variant="ghost"
      icon="i-lucide-bell"
    />
  </UChip>
</template>
