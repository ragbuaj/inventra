<script setup lang="ts">
defineProps<{ title: string, subtitle?: string }>()
</script>

<template>
  <div class="flex items-center justify-between gap-4 mb-4">
    <div>
      <h1 class="text-xl font-semibold">
        {{ title }}
      </h1>
      <p
        v-if="subtitle"
        class="text-sm text-muted"
      >
        {{ subtitle }}
      </p>
    </div>
    <div class="flex items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
