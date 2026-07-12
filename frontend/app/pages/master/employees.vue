<script setup lang="ts">
import type { Employee, RowAction, TableSorting } from '~/types'
import type { EmployeeInput } from '~/composables/api/useEmployees'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const localePath = useLocalePath()
const { open: confirm } = useConfirm()
const api = useEmployees()
const refApi = useReference()
const { request } = useApiClient()

const ALL = '__all__'

const allRows = ref<Employee[]>([])
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const filterOffice = ref(ALL)
const filterDept = ref(ALL)
const filterPosition = ref(ALL)
const filterStatus = ref(ALL)
const sorting = ref<TableSorting>([])
const loading = ref(true)
const loadFailed = ref(false)

const filtering = ref(false)
let filterTimer: ReturnType<typeof setTimeout> | undefined
function pulseFilterLoading() {
  filtering.value = true
  if (filterTimer) clearTimeout(filterTimer)
  filterTimer = setTimeout(() => {
    filtering.value = false
  }, 300)
}

// FK option lists + id→name maps (offices via inline scoped /offices; dept/position via wired useReference).
const officeOptions = ref<{ value: string, label: string }[]>([])
const deptOptions = ref<{ value: string, label: string }[]>([])
const positionOptions = ref<{ value: string, label: string }[]>([])
const officeMap = computed(() => new Map(officeOptions.value.map(o => [o.value, o.label])))
const deptMap = computed(() => new Map(deptOptions.value.map(o => [o.value, o.label])))
const positionMap = computed(() => new Map(positionOptions.value.map(o => [o.value, o.label])))
function officeName(id: string | null): string {
  return id ? (officeMap.value.get(id) ?? id) : '—'
}
function deptName(id: string | null): string {
  return id ? (deptMap.value.get(id) ?? id) : '—'
}
function positionName(id: string | null): string {
  return id ? (positionMap.value.get(id) ?? id) : '—'
}

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<EmployeeInput>({
  code: '', name: '', email: '', phone: '', department_id: '', position_id: '', office_id: '', status: 'active'
})

const columns = [
  { accessorKey: 'code', header: t('masterdata.employees.columns.nip'), sortable: true },
  { accessorKey: 'name', header: t('masterdata.employees.columns.nama'), sortable: true },
  { accessorKey: 'departemen', header: t('masterdata.employees.columns.departemen') },
  { accessorKey: 'jabatan', header: t('masterdata.employees.columns.jabatan') },
  { accessorKey: 'kantor', header: t('masterdata.employees.columns.kantor') },
  { accessorKey: 'kontak', header: t('masterdata.employees.columns.kontak') },
  { accessorKey: 'status', header: t('masterdata.employees.columns.status'), sortable: true }
]

function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
}

const anyFilterActive = computed(() =>
  !!(search.value.trim() || filterOffice.value !== ALL || filterDept.value !== ALL || filterPosition.value !== ALL || filterStatus.value !== ALL)
)

const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  return allRows.value.filter((r) => {
    if (q && !r.name.toLowerCase().includes(q) && !r.code.toLowerCase().includes(q) && !(r.email ?? '').toLowerCase().includes(q)) return false
    if (filterOffice.value !== ALL && r.office_id !== filterOffice.value) return false
    if (filterDept.value !== ALL && r.department_id !== filterDept.value) return false
    if (filterPosition.value !== ALL && r.position_id !== filterPosition.value) return false
    if (filterStatus.value !== ALL && r.status !== filterStatus.value) return false
    return true
  })
})

const sortedRows = computed(() => sortRows(filteredRows.value, sorting.value))
const paginatedRows = computed(() => sortedRows.value.slice(offset.value, offset.value + limit.value))
const tableRows = computed(() => paginatedRows.value.map(r => ({ ...r })))

async function refresh() {
  loading.value = true
  loadFailed.value = false
  try {
    const res = await api.list({ limit: 100 })
    allRows.value = res.data
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function loadFkData() {
  const [offices, depts, positions] = await Promise.all([
    request<{ data: { id: string, name: string }[] }>('/offices?limit=100'),
    refApi.list('departments', { limit: 100 }),
    refApi.list('positions', { limit: 100 })
  ])
  officeOptions.value = offices.data.map(o => ({ value: o.id, label: o.name }))
  deptOptions.value = depts.data.map(d => ({ value: d.id, label: d.name }))
  positionOptions.value = positions.data.map(p => ({ value: p.id, label: p.name }))
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { code: '', name: '', email: '', phone: '', department_id: '', position_id: '', office_id: officeOptions.value[0]?.value ?? '', status: 'active' })
  formOpen.value = true
}

function openEdit(row: Employee) {
  editingId.value = row.id
  Object.assign(form, {
    code: row.code, name: row.name, email: row.email ?? '', phone: row.phone ?? '',
    department_id: row.department_id ?? '', position_id: row.position_id ?? '', office_id: row.office_id,
    status: row.status === 'suspended' ? 'suspended' : row.status
  })
  formOpen.value = true
}

async function onSubmit() {
  if (!form.code.trim() || !form.name.trim() || !form.office_id) {
    toast.add({ title: t('masterdata.employees.required'), color: 'error' })
    return
  }
  saving.value = true
  try {
    const input: EmployeeInput = {
      code: form.code, name: form.name, office_id: form.office_id, status: form.status,
      email: form.email || undefined, phone: form.phone || undefined,
      department_id: form.department_id || undefined, position_id: form.position_id || undefined
    }
    if (editingId.value) await api.update(editingId.value, input)
    else await api.create(input)
    formOpen.value = false
    await refresh()
  } catch { /* useApiClient surfaces the error toast */ } finally {
    saving.value = false
  }
}

async function onDelete(row: Employee) {
  const ok = await confirm({
    title: t('common.delete'),
    description: t('masterdata.employees.deleteConfirm', { nama: row.name, nip: row.code })
  })
  if (!ok) return
  try {
    await api.remove(row.id)
    await refresh()
  } catch { /* useApiClient surfaces the error toast */ }
}

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can('masterdata.office.manage')) return []
  const r = row as unknown as Employee
  return [
    { label: t('common.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    { label: t('common.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

function resetFilters() {
  search.value = ''
  filterOffice.value = ALL
  filterDept.value = ALL
  filterPosition.value = ALL
  filterStatus.value = ALL
  offset.value = 0
}

watch([search, filterOffice, filterDept, filterPosition, filterStatus], () => {
  offset.value = 0
  pulseFilterLoading()
})
watch(sorting, () => {
  offset.value = 0
})

onMounted(() => {
  refresh()
  loadFkData()
})
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.employees.title')"
      :subtitle="t('masterdata.employees.subtitle')"
    >
      <template #actions>
        <Can permission="masterdata.employee.manage">
          <UButton
            icon="i-lucide-upload"
            color="neutral"
            variant="outline"
            :to="localePath('/master/import?target=employee')"
          >
            {{ t('common.import') }}
          </UButton>
        </Can>
        <Can permission="masterdata.office.manage">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.employees.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <!-- Filter bar -->
    <div class="bg-default border border-default rounded-[13px] shadow p-[14px] mb-4 flex flex-wrap items-center gap-[10px]">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('masterdata.employees.searchPlaceholder')"
        class="flex-1 min-w-[200px]"
      />

      <USelect
        v-model="filterOffice"
        :items="[{ value: ALL, label: t('masterdata.employees.filter.allKantor') }, ...officeOptions]"
        class="min-w-[150px]"
      />

      <USelect
        v-model="filterDept"
        :items="[{ value: ALL, label: t('masterdata.employees.filter.allDept') }, ...deptOptions]"
        class="min-w-[150px]"
      />

      <USelect
        v-model="filterPosition"
        :items="[{ value: ALL, label: t('masterdata.employees.filter.allJabatan') }, ...positionOptions]"
        class="min-w-[150px]"
      />

      <USelect
        v-model="filterStatus"
        :items="[
          { value: ALL, label: t('masterdata.employees.filter.allStatus') },
          { value: 'active', label: t('masterdata.employees.status.active') },
          { value: 'inactive', label: t('masterdata.employees.status.inactive') }
        ]"
        class="min-w-[130px]"
      />

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
      <span class="text-sm">{{ t('masterdata.employees.loadError') }}</span>
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
      v-model:sorting="sorting"
      :rows="(tableRows as unknown as Record<string, unknown>[])"
      :columns="columns"
      :loading="loading || filtering"
      :total="filteredRows.length"
      :limit="limit"
      :offset="offset"
      :empty-title="anyFilterActive ? t('masterdata.employees.emptyFilter') : t('masterdata.employees.empty')"
      :actions="rowActions"
      @update:offset="offset = $event"
    >
      <template #code-cell="{ row }">
        <span class="font-mono text-sm text-muted">
          {{ (row as unknown as Employee).code }}
        </span>
      </template>

      <template #name-cell="{ row }">
        <div class="flex items-center gap-[10px]">
          <span class="w-[30px] h-[30px] rounded-full bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px] flex-none">
            {{ initials((row as unknown as Employee).name) }}
          </span>
          <span class="font-medium">{{ (row as unknown as Employee).name }}</span>
        </div>
      </template>

      <template #departemen-cell="{ row }">
        <span class="text-muted">{{ deptName((row as unknown as Employee).department_id) }}</span>
      </template>

      <template #jabatan-cell="{ row }">
        <UBadge
          color="neutral"
          variant="subtle"
        >
          {{ positionName((row as unknown as Employee).position_id) }}
        </UBadge>
      </template>

      <template #kantor-cell="{ row }">
        <span class="text-muted">{{ officeName((row as unknown as Employee).office_id) }}</span>
      </template>

      <template #kontak-cell="{ row }">
        <div>
          <div class="text-sm">
            {{ (row as unknown as Employee).email ?? '—' }}
          </div>
          <div class="text-xs text-dimmed">
            {{ (row as unknown as Employee).phone ?? '—' }}
          </div>
        </div>
      </template>

      <template #status-cell="{ row }">
        <UBadge
          :color="(row as unknown as Employee).status === 'active' ? 'success' : 'neutral'"
          variant="subtle"
        >
          {{ t('masterdata.employees.status.' + (row as unknown as Employee).status) }}
        </UBadge>
      </template>
    </ResourceTable>

    <FormSlideover
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.employees.editTitle') : t('masterdata.employees.createTitle')"
      :subtitle="editingId ? t('masterdata.employees.editSub') : t('masterdata.employees.createSub')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <!-- Row 1: NIP + Status toggle -->
        <div class="grid grid-cols-2 gap-[14px]">
          <UFormField :label="t('masterdata.employees.fields.nip')">
            <UInput
              v-model="form.code"
              placeholder="mis. 1990…"
              class="w-full font-mono"
            />
          </UFormField>
          <UFormField :label="t('masterdata.employees.fields.status')">
            <div class="flex items-center justify-between gap-2 rounded-[10px] bg-muted px-3 h-10">
              <span class="text-sm font-medium">
                {{ form.status === 'active' ? t('masterdata.employees.status.active') : t('masterdata.employees.status.inactive') }}
              </span>
              <USwitch
                :model-value="form.status === 'active'"
                @update:model-value="form.status = $event ? 'active' : 'inactive'"
              />
            </div>
          </UFormField>
        </div>

        <!-- Row 2: Nama full-width -->
        <UFormField :label="t('masterdata.employees.fields.nama')">
          <UInput
            v-model="form.name"
            :placeholder="t('masterdata.employees.placeholders.nama')"
            class="w-full"
          />
        </UFormField>

        <!-- Row 3: Departemen + Jabatan -->
        <div class="grid grid-cols-2 gap-[14px]">
          <UFormField :label="t('masterdata.employees.fields.departemen')">
            <USelect
              v-model="form.department_id"
              :items="deptOptions"
              :placeholder="t('masterdata.employees.placeholders.pilih')"
              class="w-full"
              data-testid="employee-dept-select"
            />
          </UFormField>
          <UFormField :label="t('masterdata.employees.fields.jabatan')">
            <USelect
              v-model="form.position_id"
              :items="positionOptions"
              :placeholder="t('masterdata.employees.placeholders.pilih')"
              class="w-full"
              data-testid="employee-position-select"
            />
          </UFormField>
        </div>

        <!-- Row 4: Kantor + scope note -->
        <UFormField :label="t('masterdata.employees.fields.office')">
          <USelect
            v-model="form.office_id"
            :items="officeOptions"
            :placeholder="t('masterdata.employees.placeholders.pilih')"
            class="w-full"
            data-testid="employee-office-select"
          />
          <template #hint>
            <span class="flex items-center gap-1 text-xs text-dimmed mt-1">
              <UIcon
                name="i-lucide-lock"
                class="size-3"
              />
              {{ t('masterdata.employees.scopeNote') }}
            </span>
          </template>
        </UFormField>

        <!-- Row 5: Email + Telepon -->
        <div class="grid grid-cols-2 gap-[14px]">
          <UFormField :label="t('masterdata.employees.fields.email')">
            <UInput
              v-model="form.email"
              type="email"
              placeholder="nama@inventra.go.id"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('masterdata.employees.fields.telepon')">
            <UInput
              v-model="form.phone"
              placeholder="08xx-xxxx-xxxx"
              class="w-full"
            />
          </UFormField>
        </div>
      </div>
    </FormSlideover>
  </div>
</template>
