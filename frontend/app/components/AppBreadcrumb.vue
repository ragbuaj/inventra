<script setup lang="ts">
defineProps<{ items: { label: string, to?: string }[] }>()
</script>

<template>
  <UBreadcrumb :items="items" />
</template>
