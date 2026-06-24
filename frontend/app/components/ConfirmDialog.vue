<script setup lang="ts">
const { state, resolve } = useConfirm()
const isOpen = computed({
  get: () => state.value.open,
  set: (v) => { if (!v) resolve(false) }
})
</script>

<template>
  <UModal
    v-model:open="isOpen"
    :title="state.title"
    :description="state.description"
  >
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          @click="resolve(false)"
        >
          {{ $t('common.cancel') }}
        </UButton>
        <UButton
          :color="state.color ?? 'error'"
          @click="resolve(true)"
        >
          {{ state.confirmLabel ?? $t('common.delete') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
