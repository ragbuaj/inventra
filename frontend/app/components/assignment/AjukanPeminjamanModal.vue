<script setup lang="ts">
import type { AvailableAsset } from '~/composables/api/useAssignment'

export interface LockedAsset {
  id: string
  name: string
  asset_tag: string
  category?: string | null
  office?: string | null
  location?: string | null
}

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
const assignmentApi = useAssignment()

const pickedAssetId = ref('')
const dueDate = ref('')
const notes = ref('')
const notesError = ref(false)
const submitting = ref(false)

const availableAssets = ref<AvailableAsset[]>([])
const availableLoading = ref(false)

const isLocked = computed(() => !!props.asset)

const assetItems = computed(() => availableAssets.value.map(a => ({ value: a.id, label: `${a.name} · ${a.asset_tag}` })))

function reset() {
  pickedAssetId.value = ''
  dueDate.value = ''
  notes.value = ''
  notesError.value = false
  submitting.value = false
}

async function loadAvailable() {
  if (isLocked.value) return
  availableLoading.value = true
  try {
    const res = await assignmentApi.available()
    availableAssets.value = res.data
  } catch {
    availableAssets.value = []
  } finally {
    availableLoading.value = false
  }
}

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    reset()
    loadAvailable()
  }
}, { immediate: true })

function close() {
  emit('update:open', false)
}

async function submit() {
  const assetId = isLocked.value ? props.asset!.id : pickedAssetId.value
  const reason = notes.value.trim()

  notesError.value = !reason
  if (!assetId || notesError.value || submitting.value) return

  submitting.value = true
  try {
    await assignmentApi.borrow({
      asset_id: assetId,
      due_date: dueDate.value || null,
      notes: reason
    })
    emit('submitted')
    close()
    toast.add({ title: t('peminjaman.toast.sent'), color: 'success' })
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
    :title="t('peminjaman.modal.title')"
    :description="t('peminjaman.modal.subtitle')"
    @update:open="(v) => emit('update:open', v)"
  >
    <template #body>
      <div class="flex flex-col gap-4">
        <div
          v-if="isLocked"
          data-testid="peminjaman-modal-locked-asset"
          class="border border-default rounded-xl bg-muted p-3.5"
        >
          <div class="flex items-center gap-2.5 mb-3">
            <span class="size-[38px] rounded-[9px] bg-default border border-default flex items-center justify-center flex-none text-primary">
              <UIcon
                name="i-lucide-monitor"
                class="size-[19px]"
              />
            </span>
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-sm truncate">
                {{ asset!.name }}
              </div>
              <div class="font-mono text-[11.5px] text-dimmed">
                {{ asset!.asset_tag }}
              </div>
            </div>
            <UIcon
              name="i-lucide-lock"
              class="size-[15px] text-dimmed flex-none"
              :title="t('peminjaman.modal.lockedTip')"
            />
          </div>
          <div class="grid grid-cols-2 gap-x-4 gap-y-2">
            <div>
              <div class="text-[11px] text-muted">
                {{ t('peminjaman.modal.kategori') }}
              </div>
              <div class="text-[12.5px] font-medium mt-0.5">
                {{ asset!.category ?? '—' }}
              </div>
            </div>
            <div>
              <div class="text-[11px] text-muted">
                {{ t('peminjaman.modal.kantor') }}
              </div>
              <div class="text-[12.5px] font-medium mt-0.5">
                {{ asset!.office ?? '—' }}
              </div>
            </div>
            <div class="col-span-2">
              <div class="text-[11px] text-muted">
                {{ t('peminjaman.modal.lokasi') }}
              </div>
              <div class="text-[12.5px] font-medium mt-0.5">
                {{ asset!.location ?? '—' }}
              </div>
            </div>
          </div>
        </div>

        <UFormField
          v-else
          :label="t('peminjaman.form.aset')"
          :hint="t('peminjaman.form.asetHint')"
          required
        >
          <USelectMenu
            v-model="pickedAssetId"
            data-testid="peminjaman-modal-asset-picker"
            value-key="value"
            :items="assetItems"
            :loading="availableLoading"
            icon="i-lucide-search"
            :placeholder="t('peminjaman.form.asetPlaceholder')"
            :search-input="{ placeholder: t('peminjaman.form.asetPlaceholder') }"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('peminjaman.form.tempo')">
          <UInput
            v-model="dueDate"
            data-testid="peminjaman-modal-due-date"
            type="date"
            class="w-full"
          />
          <template #hint>
            <span class="text-xs text-dimmed">{{ t('peminjaman.form.tempoHint') }}</span>
          </template>
        </UFormField>

        <UFormField
          :label="t('peminjaman.form.alasan')"
          :error="notesError ? t('peminjaman.form.alasanErr') : undefined"
          required
        >
          <UTextarea
            v-model="notes"
            data-testid="peminjaman-modal-notes"
            :rows="3"
            :placeholder="t('peminjaman.form.alasanPlaceholder')"
            class="w-full"
            @update:model-value="notesError = false"
          />
        </UFormField>

        <div class="flex gap-2.5 items-start px-3.5 py-3 rounded-[11px] bg-primary/10 border border-primary/25">
          <UIcon
            name="i-lucide-info"
            class="size-4 flex-none mt-px text-primary"
          />
          <span class="text-[12.5px] leading-relaxed font-medium text-primary">{{ t('peminjaman.infoBanner') }}</span>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="outline"
          data-testid="peminjaman-modal-cancel"
          @click="close"
        >
          {{ t('peminjaman.modal.cancel') }}
        </UButton>
        <UButton
          icon="i-lucide-send"
          :loading="submitting"
          data-testid="peminjaman-modal-submit"
          @click="submit"
        >
          {{ t('peminjaman.modal.submit') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
