<script setup lang="ts">
import type { Employee } from '~/types'
import type { EmployeeInput } from '~/composables/api/useEmployees'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useEmployees()
const officesApi = useOffices()

const rows = ref<Employee[]>([])
const total = ref(0)
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const loading = ref(true)

const officeMap = ref<Record<string, string>>({})

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<EmployeeInput>({
  nip: '', nama: '', email: '', telepon: '', jabatan: '', departemen: '', office_id: 'o-jkt', status: 'active'
})

const columns = [
  { accessorKey: 'nip', header: t('masterdata.employees.columns.nip') },
  { accessorKey: 'nama', header: t('masterdata.employees.columns.nama') },
  { accessorKey: 'jabatan', header: t('masterdata.employees.columns.jabatan') },
  { accessorKey: 'kantor', header: t('masterdata.employees.columns.kantor') },
  { accessorKey: 'status', header: t('masterdata.employees.columns.status') }
]

const statusOptions = (['active', 'inactive'] as const).map(v => ({
  value: v, label: t(`masterdata.employees.status.${v}`)
}))

async function refresh() {
  loading.value = true
  const res = await api.list({ search: search.value, limit: limit.value, offset: offset.value })
  rows.value = res.data
  total.value = res.total
  loading.value = false
}

async function loadOffices() {
  const res = await officesApi.list({ limit: 100 })
  const map: Record<string, string> = {}
  for (const o of res.data) {
    map[o.id] = o.nama
  }
  officeMap.value = map
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { nip: '', nama: '', email: '', telepon: '', jabatan: '', departemen: '', office_id: 'o-jkt', status: 'active' })
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

watch([search, offset], refresh)
onMounted(() => {
  refresh()
  loadOffices()
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

    <DataToolbar
      v-model:search="search"
      @reset="search = ''"
    />

    <ResourceTable
      :rows="rows"
      :columns="columns"
      :loading="loading"
      :total="total"
      :limit="limit"
      :offset="offset"
      :empty-title="t('masterdata.employees.empty')"
      @update:offset="offset = $event"
    >
      <template #kantor-cell="{ row }">
        {{ officeMap[(row as unknown as Employee).office_id] ?? (row as unknown as Employee).office_id }}
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

    <FormModal
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.employees.editTitle') : t('masterdata.employees.createTitle')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField :label="t('masterdata.employees.fields.nip')">
          <UInput
            v-model="form.nip"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.nama')">
          <UInput
            v-model="form.nama"
            class="w-full"
          />
        </UFormField>
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
        <UFormField :label="t('masterdata.employees.fields.jabatan')">
          <UInput
            v-model="form.jabatan"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.departemen')">
          <UInput
            v-model="form.departemen"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.employees.fields.status')">
          <USelect
            v-model="form.status"
            :items="statusOptions"
            class="w-full"
          />
        </UFormField>
      </div>
    </FormModal>
  </div>
</template>
