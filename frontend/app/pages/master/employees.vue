<script setup lang="ts">
import type { Employee } from '~/types'
import type { EmployeeInput } from '~/composables/api/useEmployees'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useEmployees()
const officesApi = useOffices()
const refApi = useReference()

const ALL = '__all__'

const allRows = ref<Employee[]>([])
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const filterKantor = ref(ALL)
const filterDept = ref(ALL)
const filterJabatan = ref(ALL)
const filterStatus = ref(ALL)
const loading = ref(true)

const officeMap = ref<Record<string, string>>({})
const officeOptions = ref<{ value: string, label: string }[]>([])
const deptOptions = ref<{ value: string, label: string }[]>([])
const jabatanOptions = ref<{ value: string, label: string }[]>([])

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<EmployeeInput>({
  nip: '', nama: '', email: '', telepon: '', jabatan: '', departemen: '', office_id: '', status: 'active'
})

const columns = [
  { accessorKey: 'nip', header: t('masterdata.employees.columns.nip') },
  { accessorKey: 'nama', header: t('masterdata.employees.columns.nama') },
  { accessorKey: 'departemen', header: t('masterdata.employees.columns.departemen') },
  { accessorKey: 'jabatan', header: t('masterdata.employees.columns.jabatan') },
  { accessorKey: 'kantor', header: t('masterdata.employees.columns.kantor') },
  { accessorKey: 'kontak', header: t('masterdata.employees.columns.kontak') },
  { accessorKey: 'status', header: t('masterdata.employees.columns.status') }
]

function initials(nama: string): string {
  const parts = nama.trim().split(' ')
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
}

const anyFilterActive = computed(() =>
  !!(search.value.trim() || filterKantor.value !== ALL || filterDept.value !== ALL || filterJabatan.value !== ALL || filterStatus.value !== ALL)
)

const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  return allRows.value.filter((r) => {
    if (q && !r.nama.toLowerCase().includes(q) && !r.nip.includes(q) && !r.email.toLowerCase().includes(q)) return false
    if (filterKantor.value !== ALL && r.office_id !== filterKantor.value) return false
    if (filterDept.value !== ALL && r.departemen !== filterDept.value) return false
    if (filterJabatan.value !== ALL && r.jabatan !== filterJabatan.value) return false
    if (filterStatus.value !== ALL && r.status !== filterStatus.value) return false
    return true
  })
})

const paginatedRows = computed(() =>
  filteredRows.value.slice(offset.value, offset.value + limit.value)
)

const tableRows = computed(() =>
  paginatedRows.value.map(r => ({ ...r }))
)

async function refresh() {
  loading.value = true
  const res = await api.list({ limit: 100 })
  allRows.value = res.data
  loading.value = false
}

async function loadOffices() {
  const res = await officesApi.list({ limit: 100 })
  const map: Record<string, string> = {}
  for (const o of res.data) {
    map[o.id] = o.nama
  }
  officeMap.value = map
  officeOptions.value = res.data.map(o => ({ value: o.id, label: o.nama }))
}

async function loadReferenceOptions() {
  const [depts, jabatan] = await Promise.all([
    refApi.list('departments', { limit: 100 }),
    refApi.list('positions', { limit: 100 })
  ])
  deptOptions.value = depts.data.map(d => ({ value: d.name, label: d.name }))
  jabatanOptions.value = jabatan.data.map(j => ({ value: j.name, label: j.name }))
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { nip: '', nama: '', email: '', telepon: '', jabatan: '', departemen: '', office_id: officeOptions.value[0]?.value ?? '', status: 'active' })
  formOpen.value = true
}

function openEdit(row: Employee) {
  editingId.value = row.id
  Object.assign(form, {
    nip: row.nip, nama: row.nama, email: row.email, telepon: row.telepon,
    jabatan: row.jabatan, departemen: row.departemen, office_id: row.office_id, status: row.status
  })
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    if (editingId.value) await api.update(editingId.value, { ...form })
    else await api.create({ ...form })
    formOpen.value = false
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete(row: Employee) {
  const ok = await confirm({ title: t('common.delete'), description: t('masterdata.employees.deleteConfirm') })
  if (!ok) return
  await api.remove(row.id)
  await refresh()
}

function resetFilters() {
  search.value = ''
  filterKantor.value = ALL
  filterDept.value = ALL
  filterJabatan.value = ALL
  filterStatus.value = ALL
  offset.value = 0
}

watch([search, filterKantor, filterDept, filterJabatan, filterStatus], () => {
  offset.value = 0
})

onMounted(() => {
  refresh()
  loadOffices()
  loadReferenceOptions()
})
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.employees.title')"
      :subtitle="t('masterdata.employees.subtitle')"
    >
      <template #actions>
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
        v-model="filterKantor"
        :items="[{ value: ALL, label: t('masterdata.employees.filter.allKantor') }, ...officeOptions]"
        class="min-w-[150px]"
      />

      <USelect
        v-model="filterDept"
        :items="[{ value: ALL, label: t('masterdata.employees.filter.allDept') }, ...deptOptions]"
        class="min-w-[150px]"
      />

      <USelect
        v-model="filterJabatan"
        :items="[{ value: ALL, label: t('masterdata.employees.filter.allJabatan') }, ...jabatanOptions]"
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

    <ResourceTable
      :rows="(tableRows as unknown as Record<string, unknown>[])"
      :columns="columns"
      :loading="loading"
      :total="filteredRows.length"
      :limit="limit"
      :offset="offset"
      :empty-title="anyFilterActive ? t('masterdata.employees.emptyFilter') : t('masterdata.employees.empty')"
      @update:offset="offset = $event"
    >
      <template #nip-cell="{ row }">
        <span class="font-mono text-sm text-muted">
          {{ (row as unknown as Employee).nip }}
        </span>
      </template>

      <template #nama-cell="{ row }">
        <div class="flex items-center gap-[10px]">
          <span class="w-[30px] h-[30px] rounded-full bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px] flex-none">
            {{ initials((row as unknown as Employee).nama) }}
          </span>
          <span class="font-medium">{{ (row as unknown as Employee).nama }}</span>
        </div>
      </template>

      <template #departemen-cell="{ row }">
        <span class="text-muted">{{ (row as unknown as Employee).departemen }}</span>
      </template>

      <template #jabatan-cell="{ row }">
        <UBadge
          color="neutral"
          variant="subtle"
        >
          {{ (row as unknown as Employee).jabatan }}
        </UBadge>
      </template>

      <template #kantor-cell="{ row }">
        <span class="text-muted">{{ officeMap[(row as unknown as Employee).office_id] ?? (row as unknown as Employee).office_id }}</span>
      </template>

      <template #kontak-cell="{ row }">
        <div>
          <div class="text-sm">
            {{ (row as unknown as Employee).email }}
          </div>
          <div class="text-xs text-dimmed">
            {{ (row as unknown as Employee).telepon }}
          </div>
        </div>
      </template>

      <template #status-cell="{ row }">
        <UBadge
          :color="(row as unknown as Employee).status === 'active' ? 'success' : 'neutral'"
          variant="subtle"
        >
          {{ t(`masterdata.employees.status.${(row as unknown as Employee).status}`) }}
        </UBadge>
      </template>

      <template #row-actions="{ row }">
        <Can permission="masterdata.office.manage">
          <div class="flex gap-1">
            <UButton
              color="neutral"
              variant="ghost"
              icon="i-lucide-pencil"
              size="xs"
              @click="openEdit(row as unknown as Employee)"
            />
            <UButton
              color="error"
              variant="ghost"
              icon="i-lucide-trash-2"
              size="xs"
              @click="onDelete(row as unknown as Employee)"
            />
          </div>
        </Can>
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
              v-model="form.nip"
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
            v-model="form.nama"
            :placeholder="t('masterdata.employees.placeholders.nama')"
            class="w-full"
          />
        </UFormField>

        <!-- Row 3: Departemen + Jabatan -->
        <div class="grid grid-cols-2 gap-[14px]">
          <UFormField :label="t('masterdata.employees.fields.departemen')">
            <USelect
              v-model="form.departemen"
              :items="deptOptions"
              :placeholder="t('masterdata.employees.placeholders.pilih')"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('masterdata.employees.fields.jabatan')">
            <USelect
              v-model="form.jabatan"
              :items="jabatanOptions"
              :placeholder="t('masterdata.employees.placeholders.pilih')"
              class="w-full"
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
              v-model="form.telepon"
              placeholder="08xx-xxxx-xxxx"
              class="w-full"
            />
          </UFormField>
        </div>
      </div>
    </FormSlideover>
  </div>
</template>
