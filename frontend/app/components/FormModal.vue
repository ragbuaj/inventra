<script setup lang="ts">
// hideFooter lets a caller replace the default Cancel/Save footer with its
// own controls in the body slot (e.g. the account email-change modal swaps
// to a "sent" state with a resend button instead of a submit action).
const props = defineProps<{ title: string, subtitle?: string, loading?: boolean, hideFooter?: boolean }>()
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
    <template
      v-if="!props.hideFooter"
      #footer
    >
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          @click="() => { open = false }"
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
