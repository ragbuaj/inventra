<script setup lang="ts">
export interface ValuationExceptionAsset {
  id: string
  name: string
  asset_tag: string
  office_id: string
  excluded_from_valuation?: boolean
}

const props = defineProps<{
  open: boolean
  asset?: ValuationExceptionAsset | null
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'submitted': []
}>()

const { t } = useI18n()
const toast = useToast()
const requestsApi = useAssetRequests()

const reason = ref('')
const reasonError = ref(false)
const submitting = ref(false)

const alreadyExcluded = computed(() => props.asset?.excluded_from_valuation === true)

function reset() {
  reason.value = ''
  reasonError.value = false
  submitting.value = false
}

watch(() => props.open, (isOpen) => {
  if (isOpen) reset()
}, { immediate: true })

function close() {
  emit('update:open', false)
}

async function submit() {
  if (alreadyExcluded.value) return
  const trimmed = reason.value.trim()
  reasonError.value = !trimmed
  if (!props.asset || reasonError.value || submitting.value) return

  submitting.value = true
  try {
    await requestsApi.submitValuationExclusion({
      asset_id: props.asset.id,
      office_id: props.asset.office_id,
      reason: trimmed
    })
    toast.add({ title: t('assets.detail.valexModal.success'), color: 'success' })
    emit('submitted')
    close()
  } catch {
    // useApiClient already raised an error toast
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <UModal
    :open="open"
    :title="t('assets.detail.valexModal.title')"
    :description="t('assets.detail.valexModal.subtitle')"
    @update:open="(v) => emit('update:open', v)"
  >
    <template #body>
      <div
        class="flex flex-col gap-4"
        data-testid="valex-modal"
      >
        <div
          v-if="asset"
          data-testid="valex-modal-locked-asset"
          class="border border-default rounded-xl bg-muted p-3.5"
        >
          <div class="flex items-center gap-2.5">
            <span class="size-[38px] rounded-[9px] bg-default border border-default flex items-center justify-center flex-none text-primary">
              <UIcon
                name="i-lucide-badge-dollar-sign"
                class="size-[19px]"
              />
            </span>
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-sm truncate">
                {{ asset.name }}
              </div>
              <div class="font-mono text-[11.5px] text-dimmed">
                {{ asset.asset_tag }}
              </div>
            </div>
            <UIcon
              name="i-lucide-lock"
              class="size-[15px] text-dimmed flex-none"
              :title="t('peminjaman.modal.lockedTip')"
            />
          </div>
        </div>

        <div
          v-if="alreadyExcluded"
          data-testid="valex-already-excluded"
          class="flex gap-2.5 items-start px-3.5 py-3 rounded-[11px] bg-warning/10 border border-warning/25"
        >
          <UIcon
            name="i-lucide-triangle-alert"
            class="size-4 flex-none mt-px text-warning"
          />
          <span class="text-[12.5px] leading-relaxed font-medium text-warning">{{ t('assets.detail.valexModal.alreadyExcluded') }}</span>
        </div>

        <template v-else>
          <UFormField
            :label="t('assets.detail.valexModal.reason')"
            :error="reasonError ? t('assets.detail.valexModal.reasonErr') : undefined"
            required
          >
            <UTextarea
              v-model="reason"
              data-testid="valex-reason"
              :rows="3"
              :placeholder="t('assets.detail.valexModal.reasonPlaceholder')"
              class="w-full"
              @update:model-value="reasonError = false"
            />
          </UFormField>

          <div class="flex gap-2.5 items-start px-3.5 py-3 rounded-[11px] bg-primary/10 border border-primary/25">
            <UIcon
              name="i-lucide-info"
              class="size-4 flex-none mt-px text-primary"
            />
            <span class="text-[12.5px] leading-relaxed font-medium text-primary">{{ t('assets.detail.valexModal.info') }}</span>
          </div>
        </template>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="outline"
          data-testid="valex-cancel"
          @click="close"
        >
          {{ t('common.cancel') }}
        </UButton>
        <UButton
          icon="i-lucide-send"
          :loading="submitting"
          :disabled="alreadyExcluded"
          data-testid="valex-submit"
          @click="submit"
        >
          {{ t('peminjaman.modal.submit') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
