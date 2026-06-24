<script setup lang="ts">
defineProps<{ name: string, sub?: string, src?: string }>()
</script>

<template>
  <div class="flex items-center gap-2">
    <UAvatar
      :src="src"
      :alt="name"
      size="sm"
    />
    <div class="min-w-0">
      <p class="text-sm font-medium truncate">
        {{ name }}
      </p>
      <p
        v-if="sub"
        class="text-xs text-muted truncate"
      >
        {{ sub }}
      </p>
    </div>
  </div>
</template>
