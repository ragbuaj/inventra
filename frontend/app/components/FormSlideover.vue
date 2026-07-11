<script setup lang="ts">
const props = defineProps<{
  title: string
  subtitle?: string
  loading?: boolean
  disabled?: boolean
  hideSave?: boolean
  saveLabel?: string
}>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ submit: [] }>()
</script>

<template>
  <USlideover
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
          v-if="!props.hideSave"
          :loading="props.loading"
          :disabled="props.disabled"
          @click="emit('submit')"
        >
          {{ props.saveLabel ?? $t('common.save') }}
        </UButton>
      </div>
    </template>
  </USlideover>
</template>
