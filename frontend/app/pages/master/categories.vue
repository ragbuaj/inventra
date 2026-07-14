<script setup lang="ts">
import type { Category, RowAction } from '~/types'
import type { CategoryInput } from '~/composables/api/useCategories'

definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const { open: confirm } = useConfirm()
const api = useCategories()

const PERM = 'masterdata.global.manage'
const ALL = '__all__'
const PAGE_SIZE = 10

const allRows = ref<Category[]>([])
const search = ref('')
const filterClass = ref<string>(ALL)
const filterGroup = ref<string>(ALL)
const activeOnly = ref(false)
const offset = ref(0)
const loading = ref(true)
const loadFailed = ref(false)

const formOpen = ref(false)
const saving = ref(false)
const editing = ref<Category | null>(null)

const columns = [
  { accessorKey: 'name', header: t('masterdata.categories.columns.name'), sortable: true },
  { accessorKey: 'code', header: t('masterdata.categories.columns.code'), sortable: true },
  { accessorKey: 'class', header: t('masterdata.categories.columns.class') },
  { accessorKey: 'method', header: t('masterdata.categories.columns.method') },
  { accessorKey: 'life', header: t('masterdata.categories.columns.life') },
  { accessorKey: 'fiscalGroup', header: t('masterdata.categories.columns.fiscalGroup') },
  { accessorKey: 'gl', header: t('masterdata.categories.columns.gl') },
  { accessorKey: 'status', header: t('masterdata.categories.columns.status') }
]

const groupOptions = computed((): { value: string, label: string }[] =>
  (['kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen'] as const)
    .map(g => ({ value: g as string, label: t(`masterdata.categories.fiscalGroup.${g}`) }))
)

const anyFilterActive = computed(() =>
  !!(search.value.trim() || filterClass.value !== ALL || filterGroup.value !== ALL || activeOnly.value)
)

const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  return allRows.value.filter((r) => {
    if (q && !r.name.toLowerCase().includes(q) && !(r.code ?? '').toLowerCase().includes(q)) return false
    if (filterClass.value !== ALL && r.asset_class !== filterClass.value) return false
    if (filterGroup.value !== ALL && r.default_fiscal_group !== filterGroup.value) return false
    if (activeOnly.value && !r.is_active) return false
    return true
  })
})

// Keep children directly after their parent for indented display.
const orderedRows = computed(() => {
  const rows = filteredRows.value
  const byParent = new Map<string | null, Category[]>()
  for (const r of rows) {
    const key = r.parent_id
    if (!byParent.has(key)) byParent.set(key, [])
    byParent.get(key)!.push(r)
  }
  const present = new Set(rows.map(r => r.id))
  const pushed = new Set<string>()
  const out: Category[] = []
  const push = (r: Category) => {
    if (pushed.has(r.id)) return
    pushed.add(r.id)
    out.push(r)
    for (const child of byParent.get(r.id) ?? []) push(child)
  }
  // Roots = rows whose parent isn't in the current filtered set.
  for (const r of rows) {
    if (!r.parent_id || !present.has(r.parent_id)) push(r)
  }
  return out
})

const pagedRows = computed(() =>
  orderedRows.value.slice(offset.value, offset.value + PAGE_SIZE)
    .map(r => ({ ...r })) as unknown as Record<string, unknown>[]
)

// Parent options exclude self and the editing row's descendants.
function descendantIds(id: string): Set<string> {
  const ids = new Set<string>()
  const walk = (pid: string) => {
    for (const c of allRows.value) {
      if (c.parent_id === pid && !ids.has(c.id)) {
        ids.add(c.id)
        walk(c.id)
      }
    }
  }
  walk(id)
  return ids
}

const parentOptions = computed(() => {
  const exclude = new Set<string>()
  if (editing.value) {
    exclude.add(editing.value.id)
    for (const d of descendantIds(editing.value.id)) exclude.add(d)
  }
  return allRows.value
    .filter(c => !exclude.has(c.id))
    .map(c => ({ value: c.id, label: c.name }))
})

async function refresh() {
  loading.value = true
  loadFailed.value = false
  try {
    allRows.value = await api.tree()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  formOpen.value = true
}

function openEdit(row: Category) {
  editing.value = row
  formOpen.value = true
}

async function onSubmit(input: CategoryInput) {
  saving.value = true
  try {
    if (editing.value) await api.update(editing.value.id, input)
    else await api.create(input)
    formOpen.value = false
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete(row: Category) {
  const ok = await confirm({
    title: t('common.delete'),
    description: t('masterdata.categories.deleteConfirm', { name: row.name })
  })
  if (!ok) return
  await api.remove(row.id)
  await refresh()
}

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can(PERM)) return []
  const r = row as unknown as Category
  return [
    { label: t('common.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    { label: t('common.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

function resetFilters() {
  search.value = ''
  filterClass.value = ALL
  filterGroup.value = ALL
  activeOnly.value = false
  offset.value = 0
}

watch([search, filterClass, filterGroup, activeOnly], () => {
  offset.value = 0
})

onMounted(refresh)

defineExpose({ openCreate, openEdit, filterClass, filterGroup, activeOnly, formOpen, orderedRows, parentOptions, search, rowActions })
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.categories.title')"
      :subtitle="t('masterdata.categories.subtitle')"
    >
      <template #actions>
        <Can :permission="PERM">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.categories.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <!-- Filter bar -->
    <div class="bg-default border border-default rounded-[13px] shadow p-[14px] mb-4 flex flex-wrap items-center gap-[10px]">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('masterdata.categories.searchPlaceholder')"
        class="flex-1 min-w-[200px]"
      />
      <USelect
        v-model="filterClass"
        :items="[
          { value: ALL, label: t('masterdata.categories.filter.allClass') },
          { value: 'tangible', label: t('masterdata.categories.class.tangible') },
          { value: 'intangible', label: t('masterdata.categories.class.intangible') }
        ]"
        class="min-w-[150px]"
      />
      <USelect
        v-model="filterGroup"
        :items="[{ value: ALL, label: t('masterdata.categories.filter.allGroup') }, ...groupOptions]"
        class="min-w-[170px]"
      />
      <label class="flex items-center gap-2 px-3 h-9 rounded-[9px] border border-default cursor-pointer">
        <USwitch v-model="activeOnly" />
        <span class="text-sm text-muted">{{ t('masterdata.categories.filter.activeOnly') }}</span>
      </label>
      <UButton
        v-if="anyFilterActive"
        color="error"
        variant="ghost"
        icon="i-lucide-x"
        @click="resetFilters"
      >
        {{ t('common.reset') }}
      </UButton>
    </div>

    <div
      v-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-16 text-muted"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-6"
      />
      <span class="text-sm">{{ t('masterdata.categories.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="refresh"
      >
        {{ t('common.retry') }}
      </UButton>
    </div>
    <ResourceTable
      v-else
      :rows="pagedRows"
      :columns="columns"
      :loading="loading"
      :total="orderedRows.length"
      :limit="PAGE_SIZE"
      :offset="offset"
      :empty-title="anyFilterActive ? t('masterdata.categories.emptyFilter') : t('masterdata.categories.empty')"
      :actions="rowActions"
      @update:offset="offset = $event"
    >
      <template #name-cell="{ row }">
        <div
          class="flex items-center gap-2"
          :class="(row as unknown as Category).parent_id ? 'ps-6' : ''"
        >
          <UIcon
            v-if="(row as unknown as Category).parent_id"
            name="i-lucide-corner-down-right"
            class="size-3.5 text-dimmed flex-none"
          />
          <span class="font-medium">{{ (row as unknown as Category).name }}</span>
        </div>
      </template>

      <template #code-cell="{ row }">
        <UBadge
          color="neutral"
          variant="subtle"
          class="font-mono"
        >
          {{ (row as unknown as Category).code ?? '—' }}
        </UBadge>
      </template>

      <template #class-cell="{ row }">
        <UBadge
          :color="(row as unknown as Category).asset_class === 'intangible' ? 'info' : 'success'"
          variant="subtle"
        >
          {{ t(`masterdata.categories.class.${(row as unknown as Category).asset_class}`) }}
        </UBadge>
      </template>

      <template #method-cell="{ row }">
        <span class="text-muted">
          {{ (row as unknown as Category).default_depreciation_method
            ? t(`masterdata.categories.method.${(row as unknown as Category).default_depreciation_method}`)
            : '—' }}
        </span>
      </template>

      <template #life-cell="{ row }">
        <span class="tabular-nums">{{ (row as unknown as Category).default_useful_life_months ?? '—' }}</span>
      </template>

      <template #fiscalGroup-cell="{ row }">
        <span class="text-muted">
          {{ (row as unknown as Category).default_fiscal_group
            ? t(`masterdata.categories.fiscalGroup.${(row as unknown as Category).default_fiscal_group}`)
            : '—' }}
        </span>
      </template>

      <template #gl-cell="{ row }">
        <span class="font-mono text-sm text-muted">{{ (row as unknown as Category).gl_account_code ?? '—' }}</span>
      </template>

      <template #status-cell="{ row }">
        <UBadge
          :color="(row as unknown as Category).is_active ? 'success' : 'neutral'"
          variant="subtle"
        >
          {{ (row as unknown as Category).is_active ? t('common.active') : t('common.inactive') }}
        </UBadge>
      </template>
    </ResourceTable>

    <CategoryFormSlideover
      v-model:open="formOpen"
      :category="editing"
      :parent-options="parentOptions"
      :loading="saving"
      @submit="onSubmit"
    />
  </div>
</template>
