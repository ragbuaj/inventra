<script setup lang="ts">
const props = defineProps<{ title: string, subtitle?: string, loading?: boolean }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ submit: [] }>()
</script>

<template>
  <UModal
    v-model:open="open"
    :title="props.title"
    :description="props.subtitle"
  >
    <template #body>
      <slot />
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          @click="open = false"
        >
          {{ $t('common.cancel') }}
        </UButton>
        <UButton
          :loading="props.loading"
          @click="emit('submit')"
        >
          {{ $t('common.save') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
