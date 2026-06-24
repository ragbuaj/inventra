<script setup lang="ts">
import type { ReferenceRow } from '~/types'
import type { ReferenceKey, ReferenceDescriptor } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'

definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useReference()

// Active resource
const resourceKey = ref<ReferenceKey>(referenceResources[0]!.key)
const descriptor = computed<ReferenceDescriptor>(() =>
  referenceResources.find(r => r.key === resourceKey.value) ?? referenceResources[0]!
)

// Per-entity counts (fetched on mount)
const entityCounts = ref<Partial<Record<ReferenceKey, number>>>({})

// Main table state
const rows = ref<ReferenceRow[]>([])
const total = ref(0)
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const loading = ref(true)

// Form state
const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<Record<string, unknown>>({ active: true })

// Table columns (descriptor fields + Status column)
const columns = computed(() => [
  ...descriptor.value.fields.map(f => ({
    accessorKey: f.key,
    header: t(f.labelKey)
  })),
  {
    accessorKey: 'active',
    header: t('masterdata.reference.statusColumn')
  }
])

async function refresh() {
  loading.value = true
  const res = await api.list(resourceKey.value, {
    search: search.value,
    limit: limit.value,
    offset: offset.value
  })
  rows.value = res.data
  total.value = res.total
  loading.value = false
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
  form.active = true
}

function openCreate() {
  editingId.value = undefined
  resetForm()
  formOpen.value = true
}

function openEdit(row: ReferenceRow) {
  editingId.value = row.id
  resetForm()
  for (const f of descriptor.value.fields) form[f.key] = row[f.key] ?? ''
  form.active = row.active !== false
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    if (editingId.value) {
      await api.update(resourceKey.value, editingId.value, { ...form })
    } else {
      await api.create(resourceKey.value, { ...form })
    }
    formOpen.value = false
    await refresh()
    await fetchAllCounts()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete(row: ReferenceRow) {
  const ok = await confirm({
    title: t('common.delete'),
    description: t('masterdata.reference.deleteConfirm')
  })
  if (!ok) return
  await api.remove(resourceKey.value, row.id)
  await refresh()
  await fetchAllCounts()
}

async function toggleActive(row: ReferenceRow) {
  try {
    await api.update(resourceKey.value, row.id, { active: !row.active })
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  }
}

function selectResource(key: ReferenceKey) {
  resourceKey.value = key
}

watch(resourceKey, () => {
  offset.value = 0
  search.value = ''
  refresh()
})
watch([search, offset], refresh)
onMounted(async () => {
  await Promise.all([refresh(), fetchAllCounts()])
})
</script>

<template>
  <div class="flex h-full min-h-0">
    <!-- Secondary entity-nav panel (218px) -->
    <aside class="w-[218px] flex-none border-e border-default bg-default flex flex-col overflow-hidden">
      <!-- Panel header -->
      <div class="flex-none px-4 pt-4 pb-2">
        <div class="font-bold text-sm">
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
      <div class="px-8 py-7">
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
          <Can permission="masterdata.global.manage">
            <UButton
              icon="i-lucide-plus"
              @click="openCreate"
            >
              {{ t('masterdata.reference.add') }}
            </UButton>
          </Can>
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
          :rows="rows"
          :columns="columns"
          :loading="loading"
          :total="total"
          :limit="limit"
          :offset="offset"
          :empty-title="t('masterdata.reference.empty')"
          @update:offset="offset = $event"
        >
          <!-- Status toggle cell -->
          <template #active-cell="{ row }">
            <button
              class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border cursor-pointer transition-colors text-[11.5px] font-semibold"
              :class="(row as unknown as ReferenceRow).active !== false
                ? 'border-success/30 bg-success/10 text-success'
                : 'border-muted text-muted bg-transparent'"
              @click="toggleActive(row as unknown as ReferenceRow)"
            >
              <span>
                {{ (row as unknown as ReferenceRow).active !== false
                  ? t('masterdata.reference.aktif')
                  : t('masterdata.reference.nonaktif') }}
              </span>
              <span
                class="relative flex-none w-7 h-[17px] rounded-full transition-colors"
                :class="(row as unknown as ReferenceRow).active !== false ? 'bg-success' : 'bg-muted'"
              >
                <span
                  class="absolute top-0.5 w-[13px] h-[13px] rounded-full bg-white shadow-sm transition-all"
                  :class="(row as unknown as ReferenceRow).active !== false ? 'left-[13px]' : 'left-0.5'"
                />
              </span>
            </button>
          </template>

          <!-- Row actions -->
          <template #row-actions="{ row }">
            <Can permission="masterdata.global.manage">
              <div class="flex gap-1">
                <UButton
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-pencil"
                  size="xs"
                  @click="openEdit(row as unknown as ReferenceRow)"
                />
                <UButton
                  color="error"
                  variant="ghost"
                  icon="i-lucide-trash-2"
                  size="xs"
                  @click="onDelete(row as unknown as ReferenceRow)"
                />
              </div>
            </Can>
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
          <UInput
            :model-value="form[field.key] as string"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
        </UFormField>

        <!-- Aktif toggle row -->
        <label class="flex items-center justify-between gap-2.5 px-[13px] py-[11px] rounded-[11px] bg-muted cursor-pointer">
          <span>
            <span class="block text-[13.5px] font-semibold text-default">
              {{ t('masterdata.reference.aktif') }}
            </span>
            <span class="block text-xs text-muted">
              {{ t('masterdata.reference.aktifHint') }}
            </span>
          </span>
          <USwitch
            :model-value="form.active as boolean"
            @update:model-value="form.active = $event"
          />
        </label>
      </div>
    </FormModal>
  </div>
</template>
