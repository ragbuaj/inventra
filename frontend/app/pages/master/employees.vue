<script setup lang="ts">
import type { Employee, ReferenceRow, RowAction, TableSorting } from '~/types'
import type { EmployeeInput } from '~/composables/api/useEmployees'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const localePath = useLocalePath()
const { open: confirm } = useConfirm()
const api = useEmployees()
const refApi = useReference()

const ALL = '__all__'

// `allRows` holds one of two shapes depending on whether an "extra" filter
// (office/dept/position/jabatan/status — none of which the backend list
// endpoint accepts as query params, unlike `search`) is active:
//  - no extra filter: the current *server* page (real pagination — limit 10 +
//    offset, `serverTotal` from the response). No more eager `{ limit: 100 }`
//    load, so browsing beyond 100 employees works.
//  - an extra filter active: an up-to-100-row search-scoped batch, filtered
//    and paginated client-side (same 100-row ceiling as before Task 6 — not a
//    regression, just preserved until the backend grows office/dept/position/
//    status query params).
const allRows = ref<Employee[]>([])
const serverTotal = ref(0)
const limit = ref(10)
const offset = ref(0)
const search = ref('')
const debouncedSearch = ref('')
const filterOffice = ref<string | null>(null)
const filterDept = ref(ALL)
const filterPosition = ref(ALL)
const filterStatus = ref(ALL)
const sorting = ref<TableSorting>([])
const loading = ref(true)
const loadFailed = ref(false)

let searchTimer: ReturnType<typeof setTimeout> | undefined

// Office: async search picker (no more eager `{ limit: 100 }` list) — the
// table's office_id→name cell resolves lazily via the same adapter's
// resolveFn, memoized per id (useResolveCache).
const office = useOfficePicker()
const officeCache = useResolveCache(office.resolveFn)

// Department/position: the CREATE/EDIT FORM fields are async search pickers
// (see usePickerSource.ts). deptOptions/positionOptions stay an eager
// `{limit:100}` id→name list — the filter dropdowns and the table's
// departemen/jabatan cells (out of scope here, Task 6) still read from them.
const position = useReferencePicker('positions')
const deptOptions = ref<{ value: string, label: string }[]>([])
const positionOptions = ref<{ value: string, label: string }[]>([])
const deptMap = computed(() => new Map(deptOptions.value.map(o => [o.value, o.label])))
const positionMap = computed(() => new Map(positionOptions.value.map(o => [o.value, o.label])))

// Legacy-parity Fase 6: departments are per-office (filter by the chosen office;
// legacy departments with a null office stay selectable), plus company + executor
// division masters.
const deptRows = ref<ReferenceRow[]>([])
const companyRows = ref<ReferenceRow[]>([])
const execDivRows = ref<ReferenceRow[]>([])
// USelect forbids an item whose value is the empty string (Reka UI throws
// "A <SelectItem /> must have a value prop that is not an empty string"), so the
// "no selection" entry uses a sentinel that the writable models below translate
// back to '' — the same pattern as the office form.
const NONE = '__none__'
const deptItemsForOffice = computed(() => [
  { value: NONE, label: t('masterdata.employees.selectPlaceholder') },
  ...deptRows.value.filter(d => !d.office_id || d.office_id === form.office_id).map(d => ({ value: d.id, label: d.name }))
])
const companyItems = computed(() => [{ value: NONE, label: t('masterdata.employees.selectPlaceholder') }, ...companyRows.value.map(c => ({ value: c.id, label: c.name }))])
const execDivItems = computed(() => [{ value: NONE, label: t('masterdata.employees.selectPlaceholder') }, ...execDivRows.value.map(e => ({ value: e.id, label: e.name }))])
function officeName(id: string | null): string {
  return officeCache.get(id)
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
  code: '', name: '', email: '', phone: '', department_id: '', position_id: '', office_id: '', status: 'active',
  company_id: '', executor_division_id: ''
})

// Bridge '' (unset) to the NONE sentinel the USelect items use.
function noneModel(key: 'department_id' | 'company_id' | 'executor_division_id') {
  return computed({
    get: () => form[key] || NONE,
    set: (val: string) => { form[key] = val === NONE ? '' : val }
  })
}
const departmentModel = noneModel('department_id')
const companyModel = noneModel('company_id')
const execDivModel = noneModel('executor_division_id')

// Clear a chosen department when it no longer belongs to the selected office.
// MUST stay below `form`: watch() invokes its getter immediately, so declaring
// it earlier reads `form` inside its temporal dead zone and throws
// "Cannot access 'form' before initialization", which blanks the whole page.
watch(() => form.office_id, () => {
  if (form.department_id && !deptRows.value.some(d => d.id === form.department_id && (!d.office_id || d.office_id === form.office_id))) {
    form.department_id = ''
  }
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

const anyExtraFilter = computed(() =>
  !!(filterOffice.value || filterDept.value !== ALL || filterPosition.value !== ALL || filterStatus.value !== ALL)
)
const anyFilterActive = computed(() =>
  !!(search.value.trim() || anyExtraFilter.value)
)

// Client-side narrowing — only applied in "extra filter" mode (see the
// `allRows` comment above). In server-paginated mode `allRows` is already
// exactly the rows to display.
const filteredRows = computed(() => {
  if (!anyExtraFilter.value) return allRows.value
  return allRows.value.filter((r) => {
    if (filterOffice.value && r.office_id !== filterOffice.value) return false
    if (filterDept.value !== ALL && r.department_id !== filterDept.value) return false
    if (filterPosition.value !== ALL && r.position_id !== filterPosition.value) return false
    if (filterStatus.value !== ALL && r.status !== filterStatus.value) return false
    return true
  })
})

const sortedRows = computed(() => sortRows(filteredRows.value, sorting.value))
// Server-paginated mode already sliced to the current page server-side.
const paginatedRows = computed(() => anyExtraFilter.value ? sortedRows.value.slice(offset.value, offset.value + limit.value) : sortedRows.value)
const tableRows = computed(() => paginatedRows.value.map(r => ({ ...r })))
const displayTotal = computed(() => anyExtraFilter.value ? filteredRows.value.length : serverTotal.value)

let seq = 0
async function refresh() {
  const mine = ++seq
  loading.value = true
  loadFailed.value = false
  try {
    const searchParam = debouncedSearch.value.trim() || undefined
    if (anyExtraFilter.value) {
      const res = await api.list({ search: searchParam, limit: 100 })
      if (mine !== seq) return
      allRows.value = res.data
    } else {
      const res = await api.list({ search: searchParam, limit: limit.value, offset: offset.value })
      if (mine !== seq) return
      allRows.value = res.data
      serverTotal.value = res.total
    }
  } catch {
    if (mine !== seq) return
    loadFailed.value = true
  } finally {
    if (mine === seq) loading.value = false
  }
}

async function loadFkData() {
  const [depts, positions, companies, execDivs] = await Promise.all([
    refApi.list('departments', { limit: 100 }),
    refApi.list('positions', { limit: 100 }),
    refApi.list('companies', { limit: 100 }),
    refApi.list('executor-divisions', { limit: 100 })
  ])
  deptRows.value = depts.data
  deptOptions.value = depts.data.map(d => ({ value: d.id, label: d.name }))
  positionOptions.value = positions.data.map(p => ({ value: p.id, label: p.name }))
  companyRows.value = companies.data
  execDivRows.value = execDivs.data
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { code: '', name: '', email: '', phone: '', department_id: '', position_id: '', office_id: '', status: 'active', company_id: '', executor_division_id: '' })
  formOpen.value = true
}

function openEdit(row: Employee) {
  editingId.value = row.id
  Object.assign(form, {
    code: row.code, name: row.name, email: row.email ?? '', phone: row.phone ?? '',
    department_id: row.department_id ?? '', position_id: row.position_id ?? '', office_id: row.office_id,
    status: row.status === 'suspended' ? 'suspended' : row.status,
    company_id: row.company_id ?? '', executor_division_id: row.executor_division_id ?? ''
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
      department_id: form.department_id || undefined, position_id: form.position_id || undefined,
      company_id: form.company_id || null, executor_division_id: form.executor_division_id || null
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
  debouncedSearch.value = ''
  filterOffice.value = null
  filterDept.value = ALL
  filterPosition.value = ALL
  filterStatus.value = ALL
  // Don't reset `offset` here — the multi-ref filter watcher below reads it
  // to decide whether it (vs. the separate offset watcher) should refresh(),
  // and needs to see the real pre-reset value to avoid a double-fetch.
}

watch(search, (v) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    debouncedSearch.value = v
  }, 300)
})

watch([debouncedSearch, filterOffice, filterDept, filterPosition, filterStatus], () => {
  // In server-paginated mode, resetting a non-zero offset to 0 below already
  // triggers the offset watcher's own refresh() — calling refresh() again
  // here would double-fire. In client-filtered mode, offset resets don't
  // refetch (see the offset watcher), so this must always explicitly
  // refresh. `anyExtraFilter` reflects the *new* filter state already (this
  // watcher fires reacting to it), so it also covers mode switches.
  const wasFirstPage = offset.value === 0
  offset.value = 0
  if (anyExtraFilter.value || wasFirstPage) refresh()
})
watch(sorting, () => {
  offset.value = 0
})
watch(offset, () => {
  if (!anyExtraFilter.value) refresh()
})

onMounted(() => {
  refresh()
  // Non-fatal: the FK lists only populate filter/form dropdowns. An unhandled
  // rejection here would surface as a page-level error and block the whole
  // screen, so degrade to empty dropdowns instead.
  loadFkData().catch(() => {})
})

onUnmounted(() => {
  if (searchTimer) clearTimeout(searchTimer)
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

      <AsyncSearchPicker
        :model-value="filterOffice"
        :search-fn="office.searchFn"
        :resolve-fn="office.resolveFn"
        :placeholder="t('common.searchOffice')"
        testid="office-filter"
        class="min-w-[200px]"
        clearable
        @update:model-value="filterOffice = $event"
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
      :loading="loading"
      :total="displayTotal"
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
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-[14px]">
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

        <!-- Row 3: Kantor + scope note (comes first — department options are
             scoped to the chosen office, so office must be picked before the
             department/position row below). -->
        <UFormField :label="t('masterdata.employees.fields.office')">
          <AsyncSearchPicker
            :model-value="form.office_id || null"
            :search-fn="office.searchFn"
            :resolve-fn="office.resolveFn"
            :placeholder="t('common.searchOffice')"
            testid="office"
            @update:model-value="form.office_id = $event ?? ''"
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

        <!-- Row 4: Departemen + Jabatan -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-[14px]">
          <UFormField
            :label="t('masterdata.employees.fields.departemen')"
            :hint="t('masterdata.employees.deptOfficeHint')"
          >
            <USelect
              v-model="departmentModel"
              :items="deptItemsForOffice"
              :disabled="!form.office_id"
              :placeholder="t('masterdata.employees.selectPlaceholder')"
              class="w-full"
              data-testid="employee-department"
            />
          </UFormField>
          <UFormField :label="t('masterdata.employees.fields.jabatan')">
            <AsyncSearchPicker
              :model-value="form.position_id || null"
              :search-fn="position.searchFn"
              :resolve-fn="position.resolveFn"
              :placeholder="t('common.searchPosition')"
              testid="employee-position"
              clearable
              @update:model-value="form.position_id = $event ?? ''"
            />
          </UFormField>
        </div>

        <!-- Legacy-parity Fase 6: perusahaan + divisi pelaksana -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-[14px]">
          <UFormField :label="t('masterdata.employees.fields.company')">
            <USelect
              v-model="companyModel"
              :items="companyItems"
              :placeholder="t('masterdata.employees.selectPlaceholder')"
              class="w-full"
              data-testid="employee-company"
            />
          </UFormField>
          <UFormField :label="t('masterdata.employees.fields.executorDivision')">
            <USelect
              v-model="execDivModel"
              :items="execDivItems"
              :placeholder="t('masterdata.employees.selectPlaceholder')"
              class="w-full"
              data-testid="employee-executor-division"
            />
          </UFormField>
        </div>

        <!-- Row 5: Email + Telepon -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-[14px]">
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
