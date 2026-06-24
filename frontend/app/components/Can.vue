<script setup lang="ts">
const props = defineProps<{ permission: string }>()
const can = useCan()
const allowed = computed(() => can(props.permission))
</script>

<template>
  <slot v-if="allowed" />
</template>
