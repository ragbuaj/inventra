<script setup lang="ts">
defineProps<{ title: string, subtitle?: string }>()
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-3 mb-[22px]">
    <div class="min-w-0">
      <h1 class="text-2xl font-bold tracking-tight">
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
