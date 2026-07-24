<script setup lang="ts">
import type { ReferenceRow, RowAction, TableSorting } from '~/types'
import type { ReferenceKey, ReferenceDescriptor, ReferenceField } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'

definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const localePath = useLocalePath()
const { open: confirm } = useConfirm()
const api = useReference()
const officePicker = useOfficePicker()
const floorsApi = useFloors()
const officesApi = useOffices()

const resourceKey = ref<ReferenceKey>(referenceResources[0]!.key)
const descriptor = computed<ReferenceDescriptor>(() =>
  referenceResources.find(r => r.key === resourceKey.value) ?? referenceResources[0]!
)

// Only these reference sub-resources have a registered bulk-import target on
// the backend (see backend/internal/masterdata/reference/importer.go) — the
// Import button only appears for them.
const IMPORTABLE_RESOURCES: ReferenceKey[] = ['provinces', 'cities', 'brands', 'models', 'units']
const importTarget = computed(() => (IMPORTABLE_RESOURCES.includes(resourceKey.value) ? `reference:${resourceKey.value}` : null))

const entityCounts = ref<Partial<Record<ReferenceKey, number>>>({})

const rows = ref<ReferenceRow[]>([])
const total = ref(0)
const limit = ref(10)
const offset = ref(0)
const search = ref('')
const sorting = ref<TableSorting>([])
const loading = ref(true)

// FK option data, keyed by the FK field key (e.g. 'province_id' → province rows).
// Used for BOTH the form picker and the table name resolution.
const fkData = ref<Record<string, { id: string, name: string }[]>>({})

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<Record<string, unknown>>({ is_active: true })

const columns = computed(() => {
  const cols = descriptor.value.fields.map(f => ({ accessorKey: f.key, header: t(f.labelKey), sortable: true }))
  if (descriptor.value.hasActive) {
    cols.push({ accessorKey: 'is_active', header: t('masterdata.reference.statusColumn'), sortable: true })
  }
  return cols
})

// Items for a select field's USelect ({ label, value }).
function fieldSelectItems(field: ReferenceField): { label: string, value: string }[] {
  if (field.type === 'select') return (field.options ?? []).map(o => ({ label: t(o.labelKey), value: o.value }))
  return []
}

// FK form fields use an AsyncSearchPicker (see usePickerSource.ts) — one
// adapter per distinct fkResource across all descriptors, built once so the
// v-for template just looks it up by resource. `fkData`/loadFkOptions()
// above stays as the eager `{limit:100}` id→name map — it's still needed for
// the table's FK name-resolution cells (fkName()), unchanged here.
const fkPickers: Partial<Record<ReferenceKey, ReturnType<typeof useReferencePicker>>> = {}
for (const resource of referenceResources) {
  for (const field of resource.fields) {
    if (field.type === 'fk' && field.fkResource && !fkPickers[field.fkResource]) {
      fkPickers[field.fkResource] = useReferencePicker(field.fkResource)
    }
  }
}

const FK_SEARCH_PLACEHOLDER_KEY: Partial<Record<ReferenceKey, string>> = {
  provinces: 'common.searchProvince',
  brands: 'common.searchBrand'
}
function fkPlaceholder(field: ReferenceField): string {
  const key = field.fkResource ? FK_SEARCH_PLACEHOLDER_KEY[field.fkResource] : undefined
  return t(key ?? 'common.search')
}

// Resolve a FK id to its display name for the table cell.
function fkName(fieldKey: string, id: unknown): string {
  const found = (fkData.value[fieldKey] ?? []).find(o => o.id === id)
  return found?.name ?? '—'
}

// Resolve the tier enum value to its i18n label for the table cell.
function tierLabel(value: unknown): string {
  const field = descriptor.value.fields.find(f => f.key === 'tier')
  const opt = field?.options?.find(o => o.value === value)
  return opt ? t(opt.labelKey) : '—'
}

// ---------------------------------------------------------------------------
// Department-only wiring (resources with an office/floor field): a floor picker
// filtered to the selected office, plus office/floor id->name table resolution.
// ---------------------------------------------------------------------------
const hasOfficeField = computed(() => descriptor.value.fields.some(f => f.type === 'office'))
const hasFloorField = computed(() => descriptor.value.fields.some(f => f.type === 'floor'))

// Floor options for the form, filtered to the currently selected office's floors.
const NO_FLOOR = '__nofloor__'
const floorFormOptions = ref<{ label: string, value: string }[]>([])
async function loadFloorFormOptions(officeId: string) {
  if (!officeId) {
    floorFormOptions.value = []
    return
  }
  try {
    const fs = await floorsApi.listByOffice(officeId)
    floorFormOptions.value = fs.map(f => ({ label: f.name, value: f.id }))
  } catch {
    floorFormOptions.value = []
  }
}
// USelect forbids an empty-string item value; a disabled "no floor" sentinel keeps
// the dropdown from being a silent empty popover when the office has no floors yet.
const floorFormItems = computed(() => {
  if (!form.office_id) return []
  if (floorFormOptions.value.length === 0) {
    return [{ label: t('masterdata.reference.noFloor'), value: NO_FLOOR, disabled: true }]
  }
  return floorFormOptions.value
})

// Office field change resets the dependent floor and reloads its options.
async function onOfficeFieldChange(fieldKey: string, val: string | null) {
  form[fieldKey] = val ?? ''
  if (hasFloorField.value) {
    form.floor_id = ''
    await loadFloorFormOptions(String(val ?? ''))
  }
}

// id -> name maps for the office/floor table columns. Loaded non-fatally: a
// failure just leaves the cell showing a dash.
const officeNames = ref<Record<string, string>>({})
const floorNames = ref<Record<string, string>>({})
function currentRowOfficeIds(): string[] {
  return [...new Set(rows.value.map(r => (r as Record<string, unknown>).office_id).filter(Boolean) as string[])]
}
async function loadDeptNameMaps() {
  if (!hasOfficeField.value) return
  try {
    const offices = await officesApi.tree()
    officeNames.value = Object.fromEntries(offices.map(o => [o.id, o.name]))
  } catch { /* leave office names unresolved */ }
  if (!hasFloorField.value) return
  try {
    const entries = await Promise.all(
      currentRowOfficeIds().map(async id => (await floorsApi.listByOffice(id)).map(f => [f.id, f.name] as const))
    )
    floorNames.value = Object.fromEntries(entries.flat())
  } catch { /* leave floor names unresolved */ }
}

async function refresh() {
  loading.value = true
  try {
    const res = await api.list(resourceKey.value, { search: search.value, limit: limit.value, offset: offset.value })
    rows.value = res.data
    total.value = res.total
    await loadDeptNameMaps()
  } finally {
    loading.value = false
  }
}

async function loadFkOptions() {
  const next: Record<string, { id: string, name: string }[]> = {}
  for (const f of descriptor.value.fields) {
    if (f.type === 'fk' && f.fkResource) {
      const res = await api.list(f.fkResource, { limit: 100 })
      next[f.key] = res.data.map(r => ({ id: r.id, name: r.name }))
    }
  }
  fkData.value = next
}

async function fetchAllCounts() {
  const entries = await Promise.all(
    referenceResources.map(async (r) => {
      const res = await api.list(r.key, { limit: 1 })
      return [r.key, res.total] as [ReferenceKey, number]
    })
  )
  entityCounts.value = Object.fromEntries(entries)
}

function resetForm() {
  // eslint-disable-next-line @typescript-eslint/no-dynamic-delete
  for (const k of Object.keys(form)) delete form[k]
  for (const f of descriptor.value.fields) form[f.key] = ''
  form.is_active = true
}

function openCreate() {
  editingId.value = undefined
  resetForm()
  floorFormOptions.value = []
  formOpen.value = true
}

function openEdit(row: ReferenceRow) {
  editingId.value = row.id
  resetForm()
  for (const f of descriptor.value.fields) form[f.key] = row[f.key] ?? ''
  form.is_active = row.is_active !== false
  formOpen.value = true
  // Preload the floor options for the row's office so the picker shows its label.
  if (hasFloorField.value) void loadFloorFormOptions(String(form.office_id ?? ''))
}

function validate(): boolean {
  for (const f of descriptor.value.fields) {
    if (f.required && !String(form[f.key] ?? '').trim()) {
      toast.add({ title: t('masterdata.reference.requiredField', { field: t(f.labelKey) }), color: 'error' })
      return false
    }
  }
  return true
}

async function onSubmit() {
  if (!validate()) return
  saving.value = true
  try {
    if (editingId.value) {
      await api.update(resourceKey.value, editingId.value, { ...form })
    } else {
      await api.create(resourceKey.value, { ...form })
    }
    formOpen.value = false
    await Promise.all([refresh(), fetchAllCounts()])
  } catch { /* useApiClient surfaces the error toast */ } finally {
    saving.value = false
  }
}

async function onDelete(row: ReferenceRow) {
  const ok = await confirm({
    title: t('common.delete'),
    description: t('masterdata.reference.deleteConfirm', { name: row.name })
  })
  if (!ok) return
  try {
    await api.remove(resourceKey.value, row.id)
    await Promise.all([refresh(), fetchAllCounts()])
  } catch { /* useApiClient surfaces the error toast */ }
}

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can('masterdata.global.manage')) return []
  const r = row as unknown as ReferenceRow
  return [
    { label: t('common.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    { label: t('common.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

async function toggleActive(row: ReferenceRow) {
  const prev = row.is_active !== false
  row.is_active = !prev
  try {
    await api.update(resourceKey.value, row.id, { is_active: !prev })
  } catch {
    row.is_active = prev
  }
}

function selectResource(key: ReferenceKey) {
  resourceKey.value = key
}

watch(resourceKey, async () => {
  offset.value = 0
  search.value = ''
  await Promise.all([refresh(), loadFkOptions()])
})
watch([search, offset], refresh)
onMounted(async () => {
  await Promise.all([refresh(), loadFkOptions(), fetchAllCounts()])
})
</script>

<template>
  <div class="flex h-full min-h-0">
    <!-- Secondary entity-nav panel (218px, lg+ only — mobile uses the chip bar below) -->
    <aside class="hidden lg:flex w-[218px] flex-none border-e border-default bg-default flex-col overflow-hidden">
      <!-- Panel header -->
      <div class="flex-none px-4 pt-4 pb-2">
        <div
          class="font-bold text-sm"
          data-testid="reference-panel-title"
        >
          {{ t('masterdata.reference.panelTitle') }}
        </div>
        <div class="text-xs text-muted mt-0.5">
          {{ t('masterdata.reference.panelSubtitle') }}
        </div>
      </div>
      <!-- Entity list -->
      <div class="flex-1 overflow-y-auto px-2.5 pb-3.5 pt-1">
        <button
          v-for="res in referenceResources"
          :key="res.key"
          :data-testid="`ref-nav-${res.key}`"
          class="flex items-center justify-between gap-2 w-full px-[11px] py-2 mb-0.5 text-[13px] rounded-lg border-none cursor-pointer text-left transition-colors"
          :class="resourceKey === res.key
            ? 'bg-primary/10 text-primary font-semibold'
            : 'bg-transparent text-default hover:bg-muted font-normal'"
          :style="resourceKey === res.key ? 'box-shadow: inset 3px 0 0 var(--ui-primary)' : ''"
          @click="selectResource(res.key)"
        >
          <span class="truncate whitespace-nowrap">{{ t(res.labelKey) }}</span>
          <span
            class="flex-none font-mono text-[11px] font-semibold"
            :class="resourceKey === res.key ? 'text-primary' : 'text-muted'"
          >
            {{ entityCounts[res.key] ?? '…' }}
          </span>
        </button>
      </div>
    </aside>

    <!-- Main content column -->
    <div class="flex-1 flex flex-col min-w-0 overflow-y-auto">
      <!-- Mobile entity chip bar (horizontal scroll, mirrors the aside nav) -->
      <div class="lg:hidden flex-none px-4 pt-4 sm:px-6">
        <div class="flex gap-2 overflow-x-auto pb-1">
          <button
            v-for="res in referenceResources"
            :key="res.key"
            :data-testid="`ref-nav-chip-${res.key}`"
            class="flex-none inline-flex items-center gap-1.5 px-3 py-1.5 text-[12.5px] rounded-full border cursor-pointer whitespace-nowrap transition-colors"
            :class="resourceKey === res.key
              ? 'bg-primary/10 border-primary text-primary font-semibold'
              : 'bg-default border-default text-default hover:bg-muted font-normal'"
            @click="selectResource(res.key)"
          >
            <span>{{ t(res.labelKey) }}</span>
            <span
              class="font-mono text-[11px] font-semibold"
              :class="resourceKey === res.key ? 'text-primary' : 'text-muted'"
            >
              {{ entityCounts[res.key] ?? '…' }}
            </span>
          </button>
        </div>
      </div>
      <div class="px-4 py-5 sm:px-6 lg:px-8 lg:py-7">
        <!-- Page header -->
        <div class="flex items-start justify-between gap-4 flex-wrap mb-[22px]">
          <div>
            <h1 class="text-2xl font-bold tracking-tight m-0 mb-1">
              {{ t(descriptor.labelKey) }}
            </h1>
            <p class="text-sm text-muted m-0">
              {{ t('masterdata.reference.entitySubtitle') }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <Can
              v-if="importTarget"
              permission="masterdata.global.manage"
            >
              <UButton
                icon="i-lucide-upload"
                color="neutral"
                variant="outline"
                :to="localePath(`/master/import?target=${importTarget}`)"
              >
                {{ t('common.import') }}
              </UButton>
            </Can>
            <Can permission="masterdata.global.manage">
              <UButton
                icon="i-lucide-plus"
                @click="openCreate"
              >
                {{ t('masterdata.reference.add') }}
              </UButton>
            </Can>
          </div>
        </div>

        <!-- Search bar only (no Reset, no entity dropdown) -->
        <div class="mb-3.5 max-w-[340px]">
          <UInput
            v-model="search"
            icon="i-lucide-search"
            :placeholder="t('common.search')"
            class="w-full"
          />
        </div>

        <!-- Table -->
        <ResourceTable
          v-model:sorting="sorting"
          :rows="rows"
          :columns="columns"
          :loading="loading"
          :total="total"
          :limit="limit"
          :offset="offset"
          :empty-title="t('masterdata.reference.empty')"
          :actions="rowActions"
          @update:offset="offset = $event"
        >
          <!-- FK name + tier label + status cells -->
          <template #province_id-cell="{ row }">
            {{ fkName('province_id', (row as Record<string, unknown>).province_id) }}
          </template>
          <template #brand_id-cell="{ row }">
            {{ fkName('brand_id', (row as Record<string, unknown>).brand_id) }}
          </template>
          <template #office_id-cell="{ row }">
            {{ officeNames[(row as Record<string, unknown>).office_id as string] ?? '—' }}
          </template>
          <template #floor_id-cell="{ row }">
            {{ floorNames[(row as Record<string, unknown>).floor_id as string] ?? '—' }}
          </template>
          <template #tier-cell="{ row }">
            {{ tierLabel((row as Record<string, unknown>).tier) }}
          </template>
          <template #is_active-cell="{ row }">
            <button
              class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border cursor-pointer transition-colors text-[11.5px] font-semibold"
              :class="(row as unknown as ReferenceRow).is_active !== false
                ? 'border-success/30 bg-success/10 text-success'
                : 'border-muted text-muted bg-muted'"
              @click="toggleActive(row as unknown as ReferenceRow)"
            >
              <span>
                {{ (row as unknown as ReferenceRow).is_active !== false
                  ? t('masterdata.reference.aktif')
                  : t('masterdata.reference.nonaktif') }}
              </span>
              <span
                class="relative flex-none w-7 h-[17px] rounded-full transition-colors"
                :class="(row as unknown as ReferenceRow).is_active !== false ? 'bg-success' : 'bg-muted'"
              >
                <span
                  class="absolute top-0.5 w-[13px] h-[13px] rounded-full bg-default shadow-sm transition-all"
                  :class="(row as unknown as ReferenceRow).is_active !== false ? 'left-[13px]' : 'left-0.5'"
                />
              </span>
            </button>
          </template>
        </ResourceTable>
      </div>
    </div>

    <!-- Form modal -->
    <FormModal
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.reference.editTitle') : t('masterdata.reference.createTitle')"
      :subtitle="t(descriptor.labelKey)"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField
          v-for="field in descriptor.fields"
          :key="field.key"
          :label="t(field.labelKey)"
        >
          <AsyncSearchPicker
            v-if="field.type === 'fk' && field.fkResource"
            :model-value="(form[field.key] as string) || null"
            :search-fn="fkPickers[field.fkResource]!.searchFn"
            :resolve-fn="fkPickers[field.fkResource]!.resolveFn"
            :placeholder="fkPlaceholder(field)"
            :testid="`ref-field-${field.key}`"
            @update:model-value="form[field.key] = $event ?? ''"
          />
          <AsyncSearchPicker
            v-else-if="field.type === 'office'"
            :model-value="(form[field.key] as string) || null"
            :search-fn="officePicker.searchFn"
            :resolve-fn="officePicker.resolveFn"
            :placeholder="t('masterdata.reference.searchOffice')"
            :testid="`ref-field-${field.key}`"
            clearable
            @update:model-value="onOfficeFieldChange(field.key, $event)"
          />
          <USelect
            v-else-if="field.type === 'floor'"
            :model-value="form[field.key] as string"
            :items="floorFormItems"
            :disabled="!form.office_id"
            :placeholder="form.office_id ? t('masterdata.reference.selectFloor') : t('masterdata.reference.selectOfficeFirst')"
            :data-testid="`ref-field-${field.key}`"
            class="w-full"
            @update:model-value="form[field.key] = ($event === NO_FLOOR ? '' : $event)"
          />
          <USelect
            v-else-if="field.type === 'select'"
            :model-value="form[field.key] as string"
            :items="fieldSelectItems(field)"
            :data-testid="`ref-field-${field.key}`"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
          <NumberInput
            v-else-if="field.type === 'number'"
            :model-value="form[field.key] as string"
            :data-testid="`ref-field-${field.key}`"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
          <UInput
            v-else
            :model-value="form[field.key] as string"
            :data-testid="`ref-field-${field.key}`"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
        </UFormField>

        <!-- Aktif toggle row (only for resources that have is_active) -->
        <label
          v-if="descriptor.hasActive"
          class="flex items-center justify-between gap-2.5 px-[13px] py-[11px] rounded-[11px] bg-muted cursor-pointer"
        >
          <span>
            <span class="block text-[13.5px] font-semibold text-default">
              {{ t('masterdata.reference.aktif') }}
            </span>
            <span class="block text-xs text-muted">
              {{ t('masterdata.reference.aktifHint') }}
            </span>
          </span>
          <USwitch
            :model-value="form.is_active as boolean"
            @update:model-value="form.is_active = $event"
          />
        </label>
      </div>
    </FormModal>
  </div>
</template>
