<script setup lang="ts">
import type { LockedAsset } from '~/components/assignment/AjukanPeminjamanModal.vue'

const props = defineProps<{
  open: boolean
  asset?: LockedAsset | null
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'submitted': []
}>()

const { t } = useI18n()
const toast = useToast()
const maintenanceApi = useMaintenance()
const problemCategoryPicker = useReferencePicker('problem-categories')

const problemId = ref('')
const description = ref('')
const photo = ref<File | null>(null)
const problemError = ref(false)
const submitting = ref(false)

function reset() {
  problemId.value = ''
  description.value = ''
  photo.value = null
  problemError.value = false
  submitting.value = false
}

watch(() => props.open, (isOpen) => {
  if (isOpen) reset()
}, { immediate: true })

function close() {
  emit('update:open', false)
}

function onPhotoChange(e: Event) {
  const input = e.target as HTMLInputElement
  photo.value = input.files?.[0] ?? null
}

async function submit() {
  problemError.value = !problemId.value
  if (!props.asset || problemError.value || submitting.value) return

  submitting.value = true
  try {
    await maintenanceApi.submitReport({
      asset_id: props.asset.id,
      problem_category_id: problemId.value,
      description: description.value.trim() || null,
      photo: photo.value
    })
    toast.add({ title: t('maintenance.report.submitted'), color: 'success' })
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
    :title="t('assets.detail.maintModal.title')"
    :description="t('assets.detail.maintModal.subtitle')"
    @update:open="(v) => emit('update:open', v)"
  >
    <template #body>
      <div
        class="flex flex-col gap-4"
        data-testid="reqmaint-modal"
      >
        <div
          v-if="asset"
          data-testid="reqmaint-modal-locked-asset"
          class="border border-default rounded-xl bg-muted p-3.5"
        >
          <div class="flex items-center gap-2.5">
            <span class="size-[38px] rounded-[9px] bg-default border border-default flex items-center justify-center flex-none text-primary">
              <UIcon
                name="i-lucide-wrench"
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

        <UFormField
          :label="t('maintenance.report.problem')"
          :error="problemError ? t('assets.detail.maintModal.problemErr') : undefined"
          required
        >
          <AsyncSearchPicker
            :model-value="problemId || null"
            :search-fn="problemCategoryPicker.searchFn"
            :resolve-fn="problemCategoryPicker.resolveFn"
            :placeholder="t('common.searchProblemCategory')"
            testid="reqmaint-category"
            clearable
            @update:model-value="(v) => { problemId = v ?? ''; problemError = false }"
          />
        </UFormField>

        <UFormField :label="t('maintenance.report.description')">
          <UTextarea
            v-model="description"
            data-testid="reqmaint-description"
            :rows="3"
            :placeholder="t('maintenance.report.descPlaceholder')"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('maintenance.report.photo')">
          <label
            for="reqmaint-photo"
            class="block border-[1.5px] border-dashed border-default rounded-[11px] p-[18px] text-center cursor-pointer hover:border-primary transition-colors"
          >
            <div class="size-9 mx-auto mb-2 rounded-[9px] bg-muted text-muted flex items-center justify-center">
              <UIcon
                name="i-lucide-camera"
                class="size-[18px]"
              />
            </div>
            <div class="text-[12.5px] font-medium text-muted">
              {{ photo ? photo.name : t('maintenance.report.photoDrop') }}
            </div>
          </label>
          <input
            id="reqmaint-photo"
            type="file"
            accept="image/*"
            class="hidden"
            data-testid="reqmaint-photo-input"
            @change="onPhotoChange"
          >
        </UFormField>

        <div class="text-xs leading-relaxed text-dimmed flex gap-2 items-start">
          <UIcon
            name="i-lucide-info"
            class="size-3.5 flex-none mt-0.5"
          />
          {{ t('maintenance.report.queueNote') }}
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="outline"
          data-testid="reqmaint-cancel"
          @click="close"
        >
          {{ t('common.cancel') }}
        </UButton>
        <UButton
          icon="i-lucide-send"
          :loading="submitting"
          data-testid="reqmaint-submit"
          @click="submit"
        >
          {{ t('maintenance.report.submit') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
