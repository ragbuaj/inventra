<script setup lang="ts">
import type { Asset, AssetCreateInput, AssetUpdateInput, Category, Floor, Room } from '~/types'
import { classMeta } from '~/constants/assetMeta'

const props = defineProps<{
  mode: 'new' | 'edit'
  initial?: Asset | null
}>()

const { t } = useI18n()
const toast = useToast()
const localePath = useLocalePath()
const { open: confirm } = useConfirm()

const categoriesApi = useCategories()
const office = useOfficePicker()
const floorsApi = useFloors()
const referenceApi = useReference()
const assetsApi = useAssets()
const assetRequestsApi = useAssetRequests()
const attachmentsApi = useAssetAttachments()

// ---------------------------------------------------------------------------
// Lookup option lists (categories/offices/brands/models/units/vendors) — all
// server-backed, no more hardcoded arrays. `ready` gates the cascade watchers
// below so populating fields from `initial` (edit mode) doesn't itself trigger
// a reset of the very values it just set.
// ---------------------------------------------------------------------------

const categories = ref<Category[]>([])
const brands = ref<{ id: string, name: string }[]>([])
const models = ref<{ id: string, name: string, brand_id?: string }[]>([])
const units = ref<{ id: string, name: string }[]>([])
const vendors = ref<{ id: string, name: string }[]>([])
const floors = ref<Floor[]>([])
const rooms = ref<Room[]>([])
const ready = ref(false)

const form = reactive({
  nama: '', categoryId: '', brandId: '', modelId: '', serialNumber: '', unitId: '',
  officeId: '', floorId: '', roomId: '',
  tglBeli: '', harga: '', vendorId: '', poNumber: '', fundingSource: '', warrantyExpiry: '',
  notes: ''
})
const errors = ref<Record<string, string>>({})
const submitError = ref(false)

if (props.mode === 'edit' && props.initial) {
  const a = props.initial
  Object.assign(form, {
    nama: a.name, categoryId: a.category_id, brandId: a.brand_id ?? '', modelId: a.model_id ?? '',
    serialNumber: a.serial_number ?? '', unitId: a.unit_id ?? '', officeId: a.office_id, roomId: a.room_id ?? '',
    tglBeli: a.purchase_date ?? '', vendorId: a.vendor_id ?? '', poNumber: a.po_number ?? '',
    fundingSource: a.funding_source ?? '', warrantyExpiry: a.warranty_expiry ?? '', notes: a.notes ?? ''
  })
}

const categoryOptions = computed(() => categories.value.map(c => ({ value: c.id, label: c.name })))
const brandOptions = computed(() => brands.value.map(b => ({ value: b.id, label: b.name })))
const modelOptions = computed(() => models.value.filter(m => m.brand_id === form.brandId).map(m => ({ value: m.id, label: m.name })))
const unitOptions = computed(() => units.value.map(u => ({ value: u.id, label: u.name })))
const vendorOptions = computed(() => vendors.value.map(v => ({ value: v.id, label: v.name })))
const floorOptions = computed(() => floors.value.map(f => ({ value: f.id, label: f.name })))
const roomOptions = computed(() => rooms.value.map(r => ({ value: r.id, label: r.name })))

const selectedCategory = computed(() => categories.value.find(c => c.id === form.categoryId))

// Edit mode shows kantor as read-only text — resolved on demand via the
// office picker adapter's resolveFn (no more eager `{ limit: 100 }` list).
const officeName = ref('—')
watch(() => props.initial?.office_id, async (id) => {
  officeName.value = '—'
  if (!id) return
  const item = await office.resolveFn(id)
  if (props.initial?.office_id === id) officeName.value = item?.label ?? id
}, { immediate: true })

// purchase_cost may be absent (field-permission masked) or explicitly null —
// both render as "—" here (no lock affordance in the read-only form field).
const hargaReadOnly = computed(() => {
  const v = props.initial?.purchase_cost
  if (v === undefined || v === null) return '—'
  const n = Number(v)
  return Number.isFinite(n) ? `Rp ${n.toLocaleString('id-ID')}` : '—'
})

function setField(name: keyof typeof form, val: string) {
  form[name] = val
  if (errors.value[name]) {
    const { [name]: _omit, ...rest } = errors.value
    errors.value = rest
  }
}

// ---------------------------------------------------------------------------
// Kantor → Lantai → Ruangan cascade. Ruangan stays disabled until both an
// office and a floor are chosen; picking a new office/floor invalidates the
// finer-grained selections below it.
// ---------------------------------------------------------------------------

async function loadFloorsForOffice(officeId: string) {
  floors.value = officeId ? await floorsApi.listByOffice(officeId).catch(() => []) : []
}
async function loadRoomsForFloor(floorId: string) {
  rooms.value = floorId ? await floorsApi.roomsByFloor(floorId).catch(() => []) : []
}

async function resolveInitialRoom(officeId: string, roomId: string) {
  const perFloor = await Promise.all(floors.value.map(async floor => ({
    floor, rooms: await floorsApi.roomsByFloor(floor.id).catch(() => [])
  })))
  for (const { floor, rooms: floorRooms } of perFloor) {
    if (floorRooms.some(r => r.id === roomId)) {
      form.floorId = floor.id
      rooms.value = floorRooms
      form.roomId = roomId
      return
    }
  }
}

watch(() => form.officeId, async (v, old) => {
  if (!ready.value || v === old) return
  form.floorId = ''
  form.roomId = ''
  rooms.value = []
  await loadFloorsForOffice(v)
})
watch(() => form.floorId, async (v, old) => {
  if (!ready.value || v === old) return
  form.roomId = ''
  await loadRoomsForFloor(v)
})
watch(() => form.brandId, (v, old) => {
  if (!ready.value || v === old) return
  form.modelId = ''
})

// ---------------------------------------------------------------------------
// Depreciation — read-only, informational: derived from the chosen category's
// default depreciation config (no editable inputs; the category owns this).
// ---------------------------------------------------------------------------

const DEPR_METHOD_KEY: Record<string, string> = {
  straight_line: 'assets.detail.deprMethodValue.straight_line',
  declining_balance: 'assets.detail.deprMethodValue.declining_balance'
}
function deprMethodLabel(v: string | null | undefined): string {
  if (!v) return '—'
  return t(DEPR_METHOD_KEY[v] ?? v)
}
const deprInfo = computed(() => {
  const c = selectedCategory.value
  if (!c) return null
  return {
    method: deprMethodLabel(c.default_depreciation_method),
    life: c.default_useful_life_months != null ? t('assets.detail.months', { n: c.default_useful_life_months }) : '—',
    salvage: c.default_salvage_rate != null ? `${(Number(c.default_salvage_rate) * 100).toFixed(0)}%` : '—'
  }
})

// ---------------------------------------------------------------------------
// Attachments — live (list/upload/remove) in edit mode only; disabled in new
// mode until the asset request is approved and an asset id exists.
// ---------------------------------------------------------------------------

interface AttachmentRow { id: string, name: string, sizeLabel: string }
const attachments = ref<AttachmentRow[]>([])
const uploading = ref(false)
const fileInput = ref<HTMLInputElement>()

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

async function loadAttachments() {
  if (props.mode !== 'edit' || !props.initial) return
  try {
    const res = await attachmentsApi.list(props.initial.id)
    attachments.value = res.data.map(a => ({ id: a.id, name: a.original_filename, sizeLabel: formatBytes(a.size_bytes) }))
  } catch {
    attachments.value = []
  }
}

function openFilePicker() {
  if (props.mode !== 'edit') return
  fileInput.value?.click()
}

async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || !props.initial) return
  uploading.value = true
  try {
    await attachmentsApi.upload(props.initial.id, file)
    await loadAttachments()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    uploading.value = false
  }
}

async function removeAttachment(att: AttachmentRow) {
  if (!props.initial) return
  const ok = await confirm({
    title: t('common.delete'),
    description: t('assets.form.attachmentRemoveConfirm', { name: att.name })
  })
  if (!ok) return
  try {
    await attachmentsApi.remove(props.initial.id, att.id)
    await loadAttachments()
  } catch {
    // useApiClient surfaces the error toast
  }
}

// ---------------------------------------------------------------------------
// Validation + submit
// ---------------------------------------------------------------------------

function validate(): boolean {
  const next: Record<string, string> = {}
  if (!form.nama.trim()) next.nama = t('assets.form.errors.nama')
  if (!form.categoryId) next.kategori = t('assets.form.errors.kategori')
  if (props.mode === 'new') {
    if (!form.officeId) next.kantor = t('assets.form.errors.kantor')
    if (!form.tglBeli) next.tglBeli = t('assets.form.errors.tglBeli')
    const hargaNum = Number(form.harga)
    if (!form.harga.trim() || Number.isNaN(hargaNum) || hargaNum < 0) next.harga = t('assets.form.errors.harga')
  }
  errors.value = next
  return Object.keys(next).length === 0
}

function buildUpdateBody(): AssetUpdateInput {
  return {
    name: form.nama.trim(),
    category_id: form.categoryId,
    brand_id: form.brandId || null,
    model_id: form.modelId || null,
    room_id: form.roomId || null,
    unit_id: form.unitId || null,
    vendor_id: form.vendorId || null,
    serial_number: form.serialNumber.trim() || null,
    po_number: form.poNumber.trim() || null,
    funding_source: form.fundingSource.trim() || null,
    purchase_date: form.tglBeli || null,
    warranty_expiry: form.warrantyExpiry || null,
    notes: form.notes.trim() || null
  }
}
function buildCreateInput(): AssetCreateInput {
  return {
    ...buildUpdateBody(),
    office_id: form.officeId,
    asset_class: selectedCategory.value?.asset_class ?? 'tangible',
    purchase_cost: form.harga.trim()
  }
}

const saving = ref(false)
async function save() {
  submitError.value = false
  if (!validate()) {
    toast.add({ title: t('assets.form.fixErrors'), color: 'error', icon: 'i-lucide-circle-alert' })
    return
  }
  saving.value = true
  try {
    if (props.mode === 'edit' && props.initial) {
      await assetsApi.update(props.initial.id, buildUpdateBody())
      toast.add({ title: t('assets.form.savedToast'), color: 'success', icon: 'i-lucide-save' })
      navigateTo(localePath(`/assets/${props.initial.asset_tag}`))
    } else {
      await assetRequestsApi.submitCreate(buildCreateInput())
      toast.add({ title: t('assets.form.requestSubmitted'), color: 'success', icon: 'i-lucide-send' })
      navigateTo(localePath('/assets'))
    }
  } catch {
    submitError.value = true
  } finally {
    saving.value = false
  }
}

function cancel() {
  navigateTo(localePath(props.mode === 'edit' && props.initial ? `/assets/${props.initial.asset_tag}` : '/assets'))
}

onMounted(async () => {
  const [cats, br, md, un, vd] = await Promise.all([
    categoriesApi.tree().catch(() => []),
    referenceApi.list('brands', { limit: 100 }).catch(() => ({ data: [] })),
    referenceApi.list('models', { limit: 100 }).catch(() => ({ data: [] })),
    referenceApi.list('units', { limit: 100 }).catch(() => ({ data: [] })),
    referenceApi.list('vendors', { limit: 100 }).catch(() => ({ data: [] }))
  ])
  categories.value = cats
  brands.value = br.data as { id: string, name: string }[]
  models.value = md.data as { id: string, name: string, brand_id?: string }[]
  units.value = un.data as { id: string, name: string }[]
  vendors.value = vd.data as { id: string, name: string }[]

  if (props.mode === 'edit' && props.initial) {
    await loadFloorsForOffice(props.initial.office_id)
    if (props.initial.room_id) await resolveInitialRoom(props.initial.office_id, props.initial.room_id)
    await loadAttachments()
  }
  ready.value = true
})
</script>

<template>
  <div>
    <div class="flex items-start justify-between gap-4 flex-wrap mb-1.5">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ mode === 'edit' ? t('assets.form.titleEdit') : t('assets.form.titleNew') }}
        </h1>
        <p class="text-sm text-muted">
          {{ mode === 'edit' ? t('assets.form.subEdit') : t('assets.form.subNew') }}
        </p>
      </div>
    </div>

    <!-- maker-checker banner (create only) -->
    <div
      v-if="mode === 'new'"
      class="flex items-start gap-2.5 px-3.5 py-3 my-4 rounded-[11px] bg-info/10 border border-info/30"
    >
      <UIcon
        name="i-lucide-info"
        class="size-4 text-info mt-0.5 flex-none"
      />
      <span class="text-[13px] leading-snug text-info">{{ t('assets.form.banner') }}</span>
    </div>

    <!-- submit error banner -->
    <div
      v-if="submitError"
      class="flex items-start gap-2.5 px-3.5 py-3 my-4 rounded-[11px] bg-error/10 border border-error/30"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-4 text-error mt-0.5 flex-none"
      />
      <span class="text-[13px] leading-snug text-error">{{ t('assets.form.submitError') }}</span>
    </div>

    <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden mt-4">
      <div class="p-6 space-y-7">
        <!-- Identity -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.identity') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <UFormField
              :label="t('assets.form.fields.nama')"
              required
              :error="errors.nama"
              class="sm:col-span-2"
            >
              <UInput
                :model-value="form.nama"
                :placeholder="t('assets.form.placeholders.nama')"
                class="w-full"
                @update:model-value="setField('nama', String($event))"
              />
            </UFormField>
            <UFormField
              :label="t('assets.form.fields.kategori')"
              required
              :error="errors.kategori"
            >
              <USelect
                :model-value="form.categoryId"
                :items="categoryOptions"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                data-testid="asset-form-kategori-select"
                @update:model-value="setField('categoryId', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.kode')">
              <UInput
                :model-value="mode === 'edit' ? (initial?.asset_tag ?? '') : ''"
                disabled
                placeholder="—"
                class="w-full font-mono"
              />
              <template #hint>
                <span class="text-xs text-dimmed mt-1">{{ mode === 'edit' ? t('assets.form.kodeNote') : t('assets.form.tagAutoHint') }}</span>
              </template>
            </UFormField>
            <UFormField :label="t('assets.form.fields.brand')">
              <USelect
                :model-value="form.brandId"
                :items="brandOptions"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                data-testid="asset-form-brand-select"
                @update:model-value="setField('brandId', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.model')">
              <USelect
                :model-value="form.modelId"
                :items="modelOptions"
                :disabled="!form.brandId"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                data-testid="asset-form-model-select"
                @update:model-value="setField('modelId', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.serial')">
              <UInput
                :model-value="form.serialNumber"
                class="w-full"
                @update:model-value="setField('serialNumber', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.unit')">
              <USelect
                :model-value="form.unitId"
                :items="unitOptions"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                data-testid="asset-form-unit-select"
                @update:model-value="setField('unitId', String($event))"
              />
            </UFormField>
            <template v-if="mode === 'edit'">
              <UFormField :label="t('assets.form.fields.assetClass')">
                <UInput
                  :model-value="initial ? t(classMeta[initial.asset_class].labelKey) : '—'"
                  disabled
                  class="w-full"
                />
              </UFormField>
              <UFormField :label="t('assets.form.fields.status')">
                <div class="flex items-center h-10">
                  <AssetStatusBadge
                    v-if="initial"
                    :status="initial.status"
                  />
                </div>
              </UFormField>
            </template>
          </div>
        </section>

        <!-- Placement -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.placement') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <UFormField
              :label="t('assets.form.fields.kantor')"
              :required="mode === 'new'"
              :error="errors.kantor"
            >
              <AsyncSearchPicker
                v-if="mode === 'new'"
                :model-value="form.officeId || null"
                :search-fn="office.searchFn"
                :resolve-fn="office.resolveFn"
                :placeholder="t('common.searchOffice')"
                testid="office"
                @update:model-value="setField('officeId', $event ?? '')"
              />
              <UInput
                v-else
                :model-value="officeName"
                disabled
                class="w-full"
              />
              <template
                v-if="mode !== 'new'"
                #hint
              >
                <span class="text-xs text-dimmed mt-1">{{ t('assets.form.readOnlyHint') }}</span>
              </template>
            </UFormField>
            <UFormField :label="t('assets.form.fields.lantai')">
              <USelect
                :model-value="form.floorId"
                :items="floorOptions"
                :disabled="!form.officeId"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                data-testid="asset-form-lantai-select"
                @update:model-value="setField('floorId', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.ruangan')">
              <USelect
                :model-value="form.roomId"
                :items="roomOptions"
                :disabled="!form.officeId || !form.floorId"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                data-testid="asset-form-ruangan-select"
                @update:model-value="setField('roomId', String($event))"
              />
            </UFormField>
          </div>
        </section>

        <!-- Purchase -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.purchase') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <UFormField
              :label="t('assets.form.fields.tglBeli')"
              :required="mode === 'new'"
              :error="errors.tglBeli"
            >
              <UInput
                :model-value="form.tglBeli"
                type="date"
                class="w-full"
                @update:model-value="setField('tglBeli', String($event))"
              />
            </UFormField>
            <UFormField
              :label="t('assets.form.fields.harga')"
              :required="mode === 'new'"
              :error="errors.harga"
            >
              <UInput
                v-if="mode === 'new'"
                :model-value="form.harga"
                type="number"
                placeholder="0"
                class="w-full"
                @update:model-value="setField('harga', String($event))"
              />
              <UInput
                v-else
                :model-value="hargaReadOnly"
                disabled
                class="w-full"
              />
              <template
                v-if="mode !== 'new'"
                #hint
              >
                <span class="text-xs text-dimmed mt-1">{{ t('assets.form.readOnlyHint') }}</span>
              </template>
            </UFormField>
            <UFormField :label="t('assets.form.fields.vendor')">
              <USelect
                :model-value="form.vendorId"
                :items="vendorOptions"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                data-testid="asset-form-vendor-select"
                @update:model-value="setField('vendorId', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.poNumber')">
              <UInput
                :model-value="form.poNumber"
                class="w-full"
                @update:model-value="setField('poNumber', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.fundingSource')">
              <UInput
                :model-value="form.fundingSource"
                class="w-full"
                @update:model-value="setField('fundingSource', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.warranty')">
              <UInput
                :model-value="form.warrantyExpiry"
                type="date"
                class="w-full"
                @update:model-value="setField('warrantyExpiry', String($event))"
              />
            </UFormField>
            <UFormField
              :label="t('assets.form.fields.notes')"
              class="sm:col-span-2"
            >
              <UTextarea
                :model-value="form.notes"
                class="w-full"
                @update:model-value="setField('notes', String($event))"
              />
            </UFormField>
          </div>
        </section>

        <!-- Depreciation (read-only, derived from category) -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.depreciation') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div
            v-if="deprInfo"
            class="grid grid-cols-1 sm:grid-cols-3 gap-4"
          >
            <div class="flex flex-col gap-0.5">
              <span class="text-xs text-muted">{{ t('assets.form.fields.metode') }}</span>
              <span class="text-sm font-medium">{{ deprInfo.method }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-xs text-muted">{{ t('assets.form.fields.masa') }}</span>
              <span class="text-sm font-medium">{{ deprInfo.life }}</span>
            </div>
            <div class="flex flex-col gap-0.5">
              <span class="text-xs text-muted">{{ t('assets.form.fields.residu') }}</span>
              <span class="text-sm font-medium">{{ deprInfo.salvage }}</span>
            </div>
          </div>
          <p
            v-else
            class="text-sm text-muted"
          >
            {{ t('assets.form.deprNoCategory') }}
          </p>
        </section>

        <!-- Attachments -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.attachments') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>

          <input
            ref="fileInput"
            type="file"
            accept=".jpg,.jpeg,.png,.pdf,image/jpeg,image/png,application/pdf"
            class="hidden"
            @change="onFileChange"
          >
          <button
            type="button"
            :disabled="mode === 'new' || uploading"
            class="w-full flex flex-col items-center justify-center gap-2 py-8 px-4 rounded-[12px] border-2 border-dashed border-default text-center transition-colors"
            :class="mode === 'new' ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:border-primary'"
            @click="openFilePicker"
          >
            <UIcon
              name="i-lucide-upload-cloud"
              class="size-7 text-dimmed"
            />
            <span class="text-sm font-medium">{{ t('assets.form.dropTitle') }}</span>
            <span class="text-xs text-dimmed">{{ mode === 'new' ? t('assets.form.attachmentsAfterApproval') : t('assets.form.dropSub') }}</span>
          </button>

          <div
            v-if="mode === 'edit' && attachments.length"
            class="flex flex-col gap-2 mt-3"
          >
            <div
              v-for="att in attachments"
              :key="att.id"
              class="flex items-center gap-2.5 px-3 py-2.5 border border-default rounded-[10px] bg-default"
            >
              <UIcon
                name="i-lucide-file"
                class="size-4 text-muted flex-none"
              />
              <div class="flex-1 min-w-0">
                <div class="text-sm font-medium truncate">
                  {{ att.name }}
                </div>
                <div class="text-xs text-dimmed">
                  {{ att.sizeLabel }}
                </div>
              </div>
              <UButton
                icon="i-lucide-x"
                color="neutral"
                variant="ghost"
                size="xs"
                :aria-label="t('common.delete')"
                @click="removeAttachment(att)"
              />
            </div>
          </div>
          <p
            v-else-if="mode === 'edit'"
            class="text-xs text-dimmed mt-3"
          >
            {{ t('assets.form.noAttachments') }}
          </p>
        </section>
      </div>

      <!-- footer -->
      <div class="flex items-center justify-between gap-3 px-6 py-4 border-t border-default bg-elevated">
        <span class="text-[12.5px] text-dimmed">{{ t('assets.form.requiredNote') }}</span>
        <div class="flex gap-2.5">
          <UButton
            color="neutral"
            variant="outline"
            :label="t('common.cancel')"
            @click="cancel"
          />
          <UButton
            icon="i-lucide-save"
            :loading="saving"
            :label="t('common.save')"
            @click="save"
          />
        </div>
      </div>
    </div>
  </div>
</template>
