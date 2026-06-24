<script setup lang="ts">
defineProps<{ label: string, value: string | number, icon?: string, trend?: string }>()
</script>

<template>
  <UCard>
    <div class="flex items-start justify-between">
      <div>
        <p class="text-sm text-muted">
          {{ label }}
        </p>
        <p class="text-2xl font-semibold mt-1">
          {{ value }}
        </p>
        <p
          v-if="trend"
          class="text-xs text-muted mt-1"
        >
          {{ trend }}
        </p>
      </div>
      <UIcon
        v-if="icon"
        :name="icon"
        class="size-6 text-primary"
      />
    </div>
  </UCard>
</template>
