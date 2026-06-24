<script setup lang="ts">
withDefaults(defineProps<{ title: string, description?: string, icon?: string }>(), {
  icon: 'i-lucide-inbox'
})
</script>

<template>
  <div class="flex flex-col items-center justify-center text-center py-12 px-4">
    <UIcon
      :name="icon"
      class="size-10 text-dimmed mb-3"
    />
    <p class="font-medium">
      {{ title }}
    </p>
    <p
      v-if="description"
      class="text-sm text-muted mt-1"
    >
      {{ description }}
    </p>
    <div class="mt-4">
      <slot name="action" />
    </div>
  </div>
</template>
