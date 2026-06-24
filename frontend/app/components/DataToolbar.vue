<script setup lang="ts">
const props = withDefaults(defineProps<{ search?: string, showReset?: boolean }>(), {
  showReset: true
})
const emit = defineEmits<{ 'update:search': [string], 'reset': [] }>()
</script>

<template>
  <div class="flex flex-wrap items-center gap-2 mb-4">
    <UInput
      :model-value="props.search"
      icon="i-lucide-search"
      :placeholder="$t('common.search')"
      class="w-64"
      @update:model-value="emit('update:search', String($event))"
    />
    <slot name="filters" />
    <UButton
      v-if="props.showReset"
      color="neutral"
      variant="ghost"
      icon="i-lucide-rotate-ccw"
      @click="emit('reset')"
    >
      {{ $t('common.reset') }}
    </UButton>
    <div class="ms-auto">
      <slot name="view" />
    </div>
  </div>
</template>
