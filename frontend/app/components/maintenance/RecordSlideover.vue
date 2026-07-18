<script setup lang="ts">
import type { Asset, AssetStatus } from '~/types'
import type { MaintenanceRecord } from '~/composables/api/useMaintenance'
import type { MaintenanceStatus, MaintenanceType } from '~/constants/maintenanceMeta'

export interface RecordPrefillAsset {
  id: string
  name: string
  asset_tag: string
}

export interface RecordPrefill {
  asset?: RecordPrefillAsset
  scheduleId?: string
  maintenanceCategoryId?: string
  type?: MaintenanceType
}

const props = defineProps<{
  record: MaintenanceRecord | null
  prefill?: RecordPrefill | null
}>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ saved: [] }>()

const ASSET_STATUSES: AssetStatus[] = ['available', 'assigned', 'under_maintenance']
const TYPE_OPTIONS: MaintenanceType[] = ['preventive', 'corrective']

const { t } = useI18n()
const toast = useToast()
const maintenanceApi = useMaintenance()
const category = useReferencePicker('maintenance-categories')
const vendor = useReferencePicker('vendors')

const isEdit = computed(() => props.record !== null)
const isLocked = computed(() => isEdit.value || !!props.prefill?.asset)
const isTerminal = computed(() => isEdit.value && (props.record!.status === 'completed' || props.record!.status === 'cancelled'))

interface FormState {
  assetId: string
  assetName: string
  assetTag: string
  type: MaintenanceType
  categoryId: string
  scheduledDate: string
  status: MaintenanceStatus
  cost: string
  vendorId: string
  description: string
  completedDate: string
}

function todayStr(): string {
  return new Date().toISOString().slice(0, 10)
}

function emptyForm(): FormState {
  return {
    assetId: '', assetName: '', assetTag: '', type: 'preventive', categoryId: '',
    scheduledDate: '', status: 'scheduled', cost: '', vendorId: '', description: '',
    completedDate: todayStr()
  }
}

const form = reactive<FormState>(emptyForm())

const submitting = ref(false)
const errorMsg = ref('')

const typeItems = computed(() => TYPE_OPTIONS.map(v => ({ value: v, label: t(`maintenance.type.${v}`) })))

/** Mirrors backend validTransition: scheduled -> all four, in_progress -> in_progress/completed/cancelled, terminal -> none. */
function nextStatuses(current: MaintenanceStatus): MaintenanceStatus[] {
  if (current === 'scheduled') return ['scheduled', 'in_progress', 'completed', 'cancelled']
  if (current === 'in_progress') return ['in_progress', 'completed', 'cancelled']
  return []
}

const statusItems = computed(() => {
  const allowed = isEdit.value
    ? nextStatuses(props.record!.status)
    : (['scheduled', 'in_progress', 'completed', 'cancelled'] as MaintenanceStatus[])
  return allowed.map(v => ({ value: v, label: t(`maintenance.status.${v}`) }))
})

const showCompletedDate = computed(() => form.status === 'completed')

function hydrate() {
  errorMsg.value = ''
  const r = props.record
  const p = props.prefill
  if (r) {
    Object.assign(form, {
      assetId: r.asset_id,
      assetName: r.asset_name ?? '',
      assetTag: r.asset_tag ?? '',
      type: r.type,
      categoryId: r.maintenance_category_id ?? '',
      scheduledDate: r.scheduled_date ?? '',
      status: r.status,
      cost: r.cost ?? '',
      vendorId: r.vendor_id ?? '',
      description: r.description ?? '',
      completedDate: r.completed_date ?? todayStr()
    })
    return
  }
  Object.assign(form, emptyForm(), {
    assetId: p?.asset?.id ?? '',
    assetName: p?.asset?.name ?? '',
    assetTag: p?.asset?.asset_tag ?? '',
    type: p?.type ?? 'preventive',
    categoryId: p?.maintenanceCategoryId ?? ''
  })
}

watch(open, (isOpen) => {
  if (isOpen) hydrate()
}, { immediate: true })

// Default "Tanggal Selesai" to today the moment the user switches into
// completed, without clobbering an already-set value (e.g. hydrated from a
// completed record being re-opened).
watch(() => form.status, (v) => {
  if (v === 'completed' && !form.completedDate) form.completedDate = todayStr()
})

function onSelectAsset(asset: Asset) {
  form.assetId = asset.id
  form.assetName = asset.name
  form.assetTag = asset.asset_tag
}

const canSave = computed(() => {
  if (isTerminal.value) return false
  const baseOk = !!form.scheduledDate && form.description.trim() !== ''
  if (!isEdit.value) return !!form.assetId && baseOk
  return baseOk
})

function costPayload(): string | null {
  return form.cost === '' ? null : form.cost
}

async function onSubmit() {
  if (!canSave.value || submitting.value) return
  submitting.value = true
  errorMsg.value = ''
  try {
    if (isEdit.value) {
      await maintenanceApi.updateRecord(props.record!.id, {
        status: form.status,
        maintenance_category_id: form.categoryId || null,
        scheduled_date: form.scheduledDate,
        completed_date: form.status === 'completed' ? form.completedDate : null,
        cost: costPayload(),
        vendor_id: form.vendorId || null,
        description: form.description.trim()
      })
      toast.add({ title: t('maintenance.note.toastUpdated'), color: 'success' })
    } else {
      await maintenanceApi.createRecord({
        asset_id: form.assetId,
        schedule_id: props.prefill?.scheduleId ?? null,
        maintenance_category_id: form.categoryId || null,
        type: form.type,
        status: form.status,
        scheduled_date: form.scheduledDate,
        completed_date: form.status === 'completed' ? form.completedDate : null,
        cost: costPayload(),
        vendor_id: form.vendorId || null,
        description: form.description.trim()
      })
      toast.add({ title: t('maintenance.note.toastCreated'), color: 'success' })
    }
    emit('saved')
    open.value = false
  } catch {
    errorMsg.value = t('common.error')
  } finally {
    submitting.value = false
  }
}

defineExpose({ form, canSave, onSubmit, statusItems })
</script>

<template>
  <FormSlideover
    v-model:open="open"
    :title="isEdit ? t('maintenance.note.editTitle') : t('maintenance.note.title')"
    :subtitle="isEdit ? t('maintenance.note.editSubtitle') : t('maintenance.note.subtitle')"
    :loading="submitting"
    :disabled="!canSave"
    :hide-save="isTerminal"
    :save-label="t('maintenance.note.saveLabel')"
    @submit="onSubmit"
  >
    <div class="flex flex-col gap-4">
      <div
        v-if="errorMsg"
        data-testid="record-slideover-error"
        class="px-3.5 py-2.5 rounded-[10px] bg-error/10 border border-error/25 text-[12.5px] font-medium text-error"
      >
        {{ errorMsg }}
      </div>

      <div
        v-if="isTerminal"
        data-testid="record-slideover-readonly-hint"
        class="flex gap-2.5 items-start px-3.5 py-3 rounded-[11px] bg-muted border border-default"
      >
        <UIcon
          name="i-lucide-lock"
          class="size-4 flex-none mt-px text-dimmed"
        />
        <span class="text-[12.5px] leading-relaxed font-medium text-muted">{{ t('maintenance.note.readOnlyHint') }}</span>
      </div>

      <div
        v-if="isLocked"
        data-testid="record-slideover-locked-asset"
        class="border border-default rounded-xl bg-muted p-3.5 flex items-center gap-2.5"
      >
        <span class="size-[38px] rounded-[9px] bg-default border border-default flex items-center justify-center flex-none text-primary">
          <UIcon
            name="i-lucide-box"
            class="size-[19px]"
          />
        </span>
        <div class="flex-1 min-w-0">
          <div class="font-semibold text-sm truncate">
            {{ form.assetName }}
          </div>
          <div class="font-mono text-[11.5px] text-dimmed">
            {{ form.assetTag }}
          </div>
        </div>
        <UIcon
          name="i-lucide-lock"
          class="size-[15px] text-dimmed flex-none"
          :title="t('maintenance.note.lockedTip')"
        />
      </div>

      <UFormField
        v-else
        :label="t('maintenance.note.asset')"
        required
      >
        <AssetSearchPicker
          data-testid="record-slideover-asset-picker"
          :statuses="ASSET_STATUSES"
          :placeholder="t('maintenance.note.assetPlaceholder')"
          @select="onSelectAsset"
        />
      </UFormField>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <UFormField :label="t('maintenance.note.type')">
          <USelect
            v-model="form.type"
            data-testid="record-slideover-type"
            value-key="value"
            :items="typeItems"
            :disabled="isEdit || isTerminal"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('maintenance.note.category')">
          <AsyncSearchPicker
            :model-value="form.categoryId || null"
            :search-fn="category.searchFn"
            :resolve-fn="category.resolveFn"
            :disabled="isTerminal"
            :placeholder="t('common.searchMaintenanceCategory')"
            testid="record-slideover-category"
            clearable
            @update:model-value="form.categoryId = $event ?? ''"
          />
        </UFormField>
        <UFormField
          :label="t('maintenance.note.date')"
          required
        >
          <DateField
            v-model="form.scheduledDate"
            testid="record-slideover-date"
            :disabled="isTerminal"
          />
        </UFormField>
        <UFormField :label="t('maintenance.note.status')">
          <USelect
            v-model="form.status"
            data-testid="record-slideover-status"
            value-key="value"
            :items="statusItems"
            :disabled="isTerminal"
            class="w-full"
          />
        </UFormField>
      </div>

      <UFormField
        v-if="showCompletedDate"
        :label="t('maintenance.note.completedDate')"
      >
        <DateField
          v-model="form.completedDate"
          testid="record-slideover-completed-date"
          :disabled="isTerminal"
        />
      </UFormField>

      <UFormField :label="t('maintenance.note.cost')">
        <NumberInput
          v-model="form.cost"
          money
          data-testid="record-slideover-cost"
          placeholder="0"
          :disabled="isTerminal"
          class="w-full"
        />
      </UFormField>

      <UFormField :label="t('maintenance.note.vendor')">
        <AsyncSearchPicker
          :model-value="form.vendorId || null"
          :search-fn="vendor.searchFn"
          :resolve-fn="vendor.resolveFn"
          :disabled="isTerminal"
          :placeholder="t('common.searchVendor')"
          testid="record-slideover-vendor"
          clearable
          @update:model-value="form.vendorId = $event ?? ''"
        />
      </UFormField>

      <UFormField
        :label="t('maintenance.note.description')"
        required
      >
        <UTextarea
          v-model="form.description"
          data-testid="record-slideover-description"
          :rows="3"
          :placeholder="t('maintenance.note.descPlaceholder')"
          :disabled="isTerminal"
          class="w-full"
        />
      </UFormField>
    </div>
  </FormSlideover>
</template>
