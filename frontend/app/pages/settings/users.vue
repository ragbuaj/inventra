<script setup lang="ts">
import type { User, BadgeColor, RowAction, TableSorting } from '~/types'
import type { UserInput } from '~/composables/api/useUsers'
import { useUsers } from '~/composables/api/useUsers'
import { ROLES, KANTOR_OPTIONS, PEGAWAI_OPTIONS, userRoleColor } from '~/mock/users'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const { open: confirm } = useConfirm()
const api = useUsers()

const ALL = '__all__'

const allRows = ref<User[]>([])
const limit = ref(10)
const offset = ref(0)
const search = ref('')
const filterRole = ref(ALL)
const filterKantor = ref(ALL)
const filterStatus = ref(ALL)
const sorting = ref<TableSorting>([])
const loading = ref(true)

// Brief loading pulse on filter/search so the table shows its inline loading
// bar (matches the prepared UX; the data itself filters client-side).
const filtering = ref(false)
let filterTimer: ReturnType<typeof setTimeout> | undefined
function pulseFilterLoading() {
  filtering.value = true
  if (filterTimer) clearTimeout(filterTimer)
  filterTimer = setTimeout(() => {
    filtering.value = false
  }, 300)
}

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<Omit<UserInput, 'login'>>({
  nama: '', email: '', password: '', peran: '', kantor: '', pegawai: '', status: 'active'
})
const errors = reactive<{ nama?: string, email?: string }>({})

const EMAIL_RE = /^.+@.+\..+$/

const columns = [
  { accessorKey: 'nama', header: t('settings.users.columns.nama'), sortable: true },
  { accessorKey: 'peran', header: t('settings.users.columns.peran'), sortable: true },
  { accessorKey: 'kantor', header: t('settings.users.columns.kantor'), sortable: true },
  { accessorKey: 'pegawai', header: t('settings.users.columns.pegawai'), sortable: true },
  { accessorKey: 'login', header: t('settings.users.columns.login'), sortable: true },
  { accessorKey: 'status', header: t('settings.users.columns.status'), sortable: true }
]

const roleOptions = [{ value: ALL, label: t('settings.users.filter.allRoles') }, ...ROLES.map(r => ({ value: r, label: r }))]
const kantorFilterOptions = [{ value: ALL, label: t('settings.users.filter.allKantor') }, ...KANTOR_OPTIONS.map(k => ({ value: k, label: k }))]
const statusFilterOptions = [
  { value: ALL, label: t('settings.users.filter.allStatus') },
  { value: 'active', label: t('settings.users.status.active') },
  { value: 'inactive', label: t('settings.users.status.inactive') },
  { value: 'suspended', label: t('settings.users.status.suspended') }
]

const roleFormOptions: { value: string, label: string }[] = ROLES.map(r => ({ value: r, label: r }))
const kantorFormOptions: { value: string, label: string }[] = KANTOR_OPTIONS.map(k => ({ value: k, label: k }))
const pegawaiFormOptions: { value: string, label: string }[] = PEGAWAI_OPTIONS.map(p => ({ value: p, label: p }))
const statusFormOptions = [
  { value: 'active', label: t('settings.users.status.active') },
  { value: 'inactive', label: t('settings.users.status.inactive') },
  { value: 'suspended', label: t('settings.users.status.suspended') }
]

const statusMeta: Record<User['status'], { color: BadgeColor, dot: string }> = {
  active: { color: 'success', dot: 'bg-success' },
  inactive: { color: 'neutral', dot: 'bg-[var(--ui-text-dimmed)]' },
  suspended: { color: 'warning', dot: 'bg-warning' }
}

function initials(nama: string): string {
  const parts = nama.trim().split(' ')
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
}

const anyFilterActive = computed(() =>
  !!(search.value.trim() || filterRole.value !== ALL || filterKantor.value !== ALL || filterStatus.value !== ALL)
)

const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  return allRows.value.filter((r) => {
    if (q && !r.nama.toLowerCase().includes(q) && !r.email.toLowerCase().includes(q)) return false
    if (filterRole.value !== ALL && r.peran !== filterRole.value) return false
    if (filterKantor.value !== ALL && r.kantor !== filterKantor.value) return false
    if (filterStatus.value !== ALL && r.status !== filterStatus.value) return false
    return true
  })
})

const sortedRows = computed(() => sortRows(filteredRows.value, sorting.value))

const tableRows = computed(() =>
  sortedRows.value.slice(offset.value, offset.value + limit.value).map(r => ({ ...r }))
)

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can('user.manage')) return []
  const r = row as unknown as User
  return [
    { label: t('settings.users.actions.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    { label: t('settings.users.actions.resetPassword'), icon: 'i-lucide-key-round', onSelect: () => onResetPassword(r) },
    r.status === 'active'
      ? { label: t('settings.users.actions.deactivate'), icon: 'i-lucide-ban', onSelect: () => onToggleStatus(r) }
      : { label: t('settings.users.actions.activate'), icon: 'i-lucide-circle-check', onSelect: () => onToggleStatus(r) },
    { label: t('settings.users.actions.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

async function refresh() {
  loading.value = true
  const res = await api.list({ limit: 100 })
  allRows.value = res.data
  loading.value = false
}

function clearErrors() {
  delete errors.nama
  delete errors.email
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { nama: '', email: '', password: '', peran: '', kantor: '', pegawai: '', status: 'active' })
  clearErrors()
  formOpen.value = true
}

function openEdit(row: User) {
  editingId.value = row.id
  Object.assign(form, {
    nama: row.nama, email: row.email, password: '', peran: row.peran,
    kantor: row.kantor, pegawai: row.pegawai, status: row.status
  })
  clearErrors()
  formOpen.value = true
}

function validate(): boolean {
  clearErrors()
  if (!form.nama.trim()) errors.nama = t('settings.users.required')
  if (!form.email.trim()) errors.email = t('settings.users.required')
  else if (!EMAIL_RE.test(form.email)) errors.email = t('settings.users.invalidEmail')
  return !errors.nama && !errors.email
}

async function onSubmit() {
  if (!validate()) return
  saving.value = true
  try {
    const existing = editingId.value ? allRows.value.find(r => r.id === editingId.value) : undefined
    const input: UserInput = { ...form, login: existing?.login ?? 'email' }
    if (editingId.value) await api.update(editingId.value, input)
    else await api.create(input)
    formOpen.value = false
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onResetPassword(row: User) {
  await api.resetPassword(row.id)
  toast.add({ title: t('settings.users.toast.passwordReset'), color: 'success', icon: 'i-lucide-key-round' })
}

async function onToggleStatus(row: User) {
  const next = row.status === 'active' ? 'inactive' : 'active'
  await api.setStatus(row.id, next)
  toast.add({ title: t('settings.users.toast.statusChanged'), color: 'success', icon: 'i-lucide-check' })
  await refresh()
}

async function onDelete(row: User) {
  const ok = await confirm({
    title: t('settings.users.deleteTitle'),
    description: t('settings.users.deleteConfirm', { nama: row.nama, email: row.email })
  })
  if (!ok) return
  await api.remove(row.id)
  await refresh()
}

function resetFilters() {
  search.value = ''
  filterRole.value = ALL
  filterKantor.value = ALL
  filterStatus.value = ALL
  offset.value = 0
}

watch([search, filterRole, filterKantor, filterStatus], () => {
  offset.value = 0
  pulseFilterLoading()
})

watch(sorting, () => {
  offset.value = 0
})

onMounted(() => {
  refresh()
})
</script>

<template>
  <div>
    <PageHeader
      :title="t('settings.users.title')"
      :subtitle="t('settings.users.subtitle')"
    >
      <template #actions>
        <Can permission="user.manage">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('settings.users.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <!-- Filter bar -->
    <div class="bg-default border border-default rounded-[13px] shadow p-[14px] mb-4 flex flex-wrap items-center gap-[10px]">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('settings.users.searchPlaceholder')"
        class="flex-1 min-w-[200px]"
      />

      <USelect
        v-model="filterRole"
        :items="roleOptions"
        class="min-w-[150px]"
      />
      <USelect
        v-model="filterKantor"
        :items="kantorFilterOptions"
        class="min-w-[170px]"
      />
      <USelect
        v-model="filterStatus"
        :items="statusFilterOptions"
        class="min-w-[140px]"
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
      v-model:sorting="sorting"
      :rows="(tableRows as unknown as Record<string, unknown>[])"
      :columns="columns"
      :loading="loading || filtering"
      :total="filteredRows.length"
      :limit="limit"
      :offset="offset"
      :empty-title="anyFilterActive ? t('settings.users.emptyFilter') : t('settings.users.empty')"
      :actions="rowActions"
      @update:offset="offset = $event"
    >
      <template #nama-cell="{ row }">
        <div class="flex items-center gap-[11px]">
          <span class="w-[34px] h-[34px] rounded-full bg-primary/10 text-primary flex items-center justify-center font-bold text-[12px] flex-none">
            {{ initials((row as unknown as User).nama) }}
          </span>
          <div class="min-w-0">
            <div class="font-semibold text-[13.5px]">
              {{ (row as unknown as User).nama }}
            </div>
            <div class="text-xs text-muted">
              {{ (row as unknown as User).email }}
            </div>
          </div>
        </div>
      </template>

      <template #peran-cell="{ row }">
        <UBadge
          :color="userRoleColor((row as unknown as User).peran)"
          variant="subtle"
          class="rounded-full"
        >
          {{ (row as unknown as User).peran }}
        </UBadge>
      </template>

      <template #kantor-cell="{ row }">
        <span class="text-muted">{{ (row as unknown as User).kantor }}</span>
      </template>

      <template #pegawai-cell="{ row }">
        <span :class="(row as unknown as User).pegawai ? 'text-default' : 'text-dimmed'">
          {{ (row as unknown as User).pegawai || '—' }}
        </span>
      </template>

      <template #login-cell="{ row }">
        <span class="inline-flex items-center gap-[7px] text-[13px] text-muted">
          <UIcon
            :name="(row as unknown as User).login === 'google' ? 'i-simple-icons-google' : 'i-lucide-mail'"
            class="size-[15px]"
          />
          {{ t(`settings.users.login.${(row as unknown as User).login}`) }}
        </span>
      </template>

      <template #status-cell="{ row }">
        <UBadge
          :color="statusMeta[(row as unknown as User).status].color"
          variant="subtle"
          class="rounded-full gap-1.5"
        >
          <span
            class="size-1.5 rounded-full"
            :class="statusMeta[(row as unknown as User).status].dot"
          />
          {{ t(`settings.users.status.${(row as unknown as User).status}`) }}
        </UBadge>
      </template>
    </ResourceTable>

    <FormSlideover
      v-model:open="formOpen"
      :title="editingId ? t('settings.users.editTitle') : t('settings.users.createTitle')"
      :subtitle="editingId ? t('settings.users.editSub') : t('settings.users.createSub')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField
          :label="t('settings.users.fields.nama')"
          required
          :error="errors.nama"
        >
          <UInput
            v-model="form.nama"
            :placeholder="t('settings.users.placeholders.nama')"
            class="w-full"
          />
        </UFormField>

        <UFormField
          :label="t('settings.users.fields.email')"
          required
          :error="errors.email"
        >
          <UInput
            v-model="form.email"
            type="email"
            placeholder="nama@inventra.go.id"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('settings.users.fields.password')">
          <UInput
            v-model="form.password"
            type="password"
            placeholder="••••••••"
            class="w-full"
          />
          <template #hint>
            <span class="flex items-center gap-1 text-xs text-dimmed mt-1">
              <UIcon
                name="i-lucide-info"
                class="size-3"
              />
              {{ t('settings.users.passwordNote') }}
            </span>
          </template>
        </UFormField>

        <div class="grid grid-cols-2 gap-[14px]">
          <UFormField
            :label="t('settings.users.fields.peran')"
            required
          >
            <USelect
              v-model="form.peran"
              :items="roleFormOptions"
              :placeholder="t('settings.users.placeholders.pilih')"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('settings.users.fields.status')">
            <USelect
              v-model="form.status"
              :items="statusFormOptions"
              class="w-full"
            />
          </UFormField>
        </div>

        <UFormField :label="t('settings.users.fields.kantor')">
          <USelect
            v-model="form.kantor"
            :items="kantorFormOptions"
            :placeholder="t('settings.users.placeholders.pilih')"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('settings.users.fields.pegawai')">
          <USelect
            v-model="form.pegawai"
            :items="pegawaiFormOptions"
            :placeholder="t('settings.users.placeholders.pilih')"
            class="w-full"
          />
          <template #hint>
            <span class="text-xs text-dimmed mt-1">{{ t('settings.users.pegawaiNote') }}</span>
          </template>
        </UFormField>
      </div>
    </FormSlideover>
  </div>
</template>
