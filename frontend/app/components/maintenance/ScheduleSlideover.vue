<script setup lang="ts">
import type { Asset, AssetStatus } from '~/types'
import type { MaintenanceSchedule } from '~/composables/api/useMaintenance'

const props = defineProps<{
  schedule: MaintenanceSchedule | null
}>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ saved: [] }>()

const ASSET_STATUSES: AssetStatus[] = ['available', 'assigned', 'under_maintenance']

const { t } = useI18n()
const toast = useToast()
const maintenanceApi = useMaintenance()
const referenceApi = useReference()

const isEdit = computed(() => props.schedule !== null)

interface FormState {
  assetId: string
  assetName: string
  assetTag: string
  categoryId: string
  intervalMonths: string
  // "Tanggal Mulai" on create (editable, required); read-only "Jatuh Tempo
  // Berikut" display on edit (not part of the update payload).
  dateValue: string
  isActive: boolean
}

function emptyForm(): FormState {
  return { assetId: '', assetName: '', assetTag: '', categoryId: '', intervalMonths: '', dateValue: '', isActive: true }
}

const form = reactive<FormState>(emptyForm())

const submitting = ref(false)
const errorMsg = ref('')

const categories = ref<{ id: string, name: string }[]>([])
const categoryItems = computed(() => categories.value.map(c => ({ value: c.id, label: c.name })))

function hydrate() {
  errorMsg.value = ''
  const s = props.schedule
  if (!s) {
    Object.assign(form, emptyForm())
    return
  }
  Object.assign(form, {
    assetId: s.asset_id,
    assetName: s.asset_name ?? '',
    assetTag: s.asset_tag ?? '',
    categoryId: s.maintenance_category_id ?? '',
    intervalMonths: String(s.interval_months),
    dateValue: s.next_due_date ?? '',
    isActive: s.is_active
  })
}

async function loadCategories() {
  try {
    const res = await referenceApi.list('maintenance-categories', { limit: 100 })
    categories.value = res.data
  } catch {
    categories.value = []
  }
}

watch(open, (isOpen) => {
  if (isOpen) {
    hydrate()
    loadCategories()
  }
}, { immediate: true })

function onSelectAsset(asset: Asset) {
  form.assetId = asset.id
  form.assetName = asset.name
  form.assetTag = asset.asset_tag
}

const intervalValue = computed(() => Number(form.intervalMonths))
const intervalOk = computed(() => Number.isFinite(intervalValue.value) && intervalValue.value >= 1)

const canSave = computed(() => {
  if (isEdit.value) return intervalOk.value
  return !!form.assetId && intervalOk.value && !!form.dateValue
})

async function onSubmit() {
  if (!canSave.value || submitting.value) return
  submitting.value = true
  errorMsg.value = ''
  try {
    if (isEdit.value) {
      await maintenanceApi.updateSchedule(props.schedule!.id, {
        maintenance_category_id: form.categoryId || null,
        interval_months: intervalValue.value,
        is_active: form.isActive
      })
      toast.add({ title: t('maintenance.schedule.toastUpdated'), color: 'success' })
    } else {
      await maintenanceApi.createSchedule({
        asset_id: form.assetId,
        maintenance_category_id: form.categoryId || null,
        interval_months: intervalValue.value,
        start_date: form.dateValue
      })
      toast.add({ title: t('maintenance.schedule.toastCreated'), color: 'success' })
    }
    emit('saved')
    open.value = false
  } catch {
    errorMsg.value = t('common.error')
  } finally {
    submitting.value = false
  }
}

defineExpose({ form, canSave, onSubmit })
</script>

<template>
  <FormSlideover
    v-model:open="open"
    :title="isEdit ? t('maintenance.schedule.editTitle') : t('maintenance.schedule.createTitle')"
    :subtitle="isEdit ? t('maintenance.schedule.editSubtitle') : t('maintenance.schedule.createSubtitle')"
    :loading="submitting"
    :disabled="!canSave"
    @submit="onSubmit"
  >
    <div class="flex flex-col gap-4">
      <div
        v-if="errorMsg"
        data-testid="schedule-slideover-error"
        class="px-3.5 py-2.5 rounded-[10px] bg-error/10 border border-error/25 text-[12.5px] font-medium text-error"
      >
        {{ errorMsg }}
      </div>

      <div
        v-if="isEdit"
        data-testid="schedule-slideover-locked-asset"
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
          :title="t('maintenance.schedule.lockedTip')"
        />
      </div>

      <UFormField
        v-else
        :label="t('maintenance.schedule.asset')"
        required
      >
        <AssetSearchPicker
          data-testid="schedule-slideover-asset-picker"
          :statuses="ASSET_STATUSES"
          :placeholder="t('maintenance.schedule.assetPlaceholder')"
          @select="onSelectAsset"
        />
      </UFormField>

      <UFormField :label="t('maintenance.schedule.category')">
        <USelectMenu
          v-model="form.categoryId"
          data-testid="schedule-slideover-category"
          value-key="value"
          :items="categoryItems"
          :placeholder="t('maintenance.schedule.selectPlaceholder')"
          class="w-full"
        />
      </UFormField>

      <UFormField
        :label="t('maintenance.schedule.interval')"
        required
      >
        <UInput
          v-model="form.intervalMonths"
          data-testid="schedule-slideover-interval"
          type="number"
          min="1"
          class="w-full"
        />
      </UFormField>

      <UFormField
        v-if="!isEdit"
        :label="t('maintenance.schedule.startDate')"
        required
      >
        <UInput
          v-model="form.dateValue"
          data-testid="schedule-slideover-date"
          type="date"
          class="w-full"
        />
      </UFormField>
      <UFormField
        v-else
        :label="t('maintenance.schedule.nextDue')"
      >
        <UInput
          :model-value="form.dateValue"
          data-testid="schedule-slideover-date"
          type="date"
          disabled
          class="w-full"
        />
      </UFormField>

      <label
        v-if="isEdit"
        class="flex items-center justify-between gap-2 rounded-[10px] bg-muted px-3 h-11 cursor-pointer"
      >
        <span class="text-sm font-semibold">{{ t('maintenance.schedule.active') }}</span>
        <USwitch
          v-model="form.isActive"
          data-testid="schedule-slideover-active"
        />
      </label>
    </div>
  </FormSlideover>
</template>
