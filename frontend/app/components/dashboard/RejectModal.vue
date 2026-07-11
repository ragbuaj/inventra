<script setup lang="ts">
/**
 * Reject-confirmation modal for the dashboard approval panel. The note is
 * REQUIRED — the confirm button stays disabled until it is non-empty. Emits
 * `confirm(note)` on submit and `cancel` on dismissal.
 */
const open = defineModel<boolean>('open', { required: true })

const emit = defineEmits<{ confirm: [note: string], cancel: [] }>()

const { t } = useI18n()

const note = ref('')

// Reset the field each time the modal opens so a stale note never carries over.
watch(open, (isOpen) => {
  if (isOpen) note.value = ''
})

const canConfirm = computed(() => note.value.trim().length > 0)

function onConfirm() {
  if (!canConfirm.value) return
  emit('confirm', note.value.trim())
  open.value = false
}

function onCancel() {
  emit('cancel')
  open.value = false
}
</script>

<template>
  <UModal
    v-model:open="open"
    :title="t('dashboard.panel.rejectTitle')"
  >
    <template #body>
      <UFormField
        :label="t('dashboard.panel.rejectNoteLabel')"
        required
      >
        <UTextarea
          v-model="note"
          data-testid="dashboard-reject-note"
          :rows="3"
          :placeholder="t('dashboard.panel.rejectNotePlaceholder')"
          class="w-full"
        />
      </UFormField>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          data-testid="dashboard-reject-cancel"
          @click="onCancel"
        >
          {{ t('dashboard.panel.rejectCancel') }}
        </UButton>
        <UButton
          color="error"
          data-testid="dashboard-reject-confirm"
          :disabled="!canConfirm"
          @click="onConfirm"
        >
          {{ t('dashboard.panel.rejectConfirm') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
