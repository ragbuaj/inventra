<script setup lang="ts">
import type { BadgeColor, RowAction } from '~/types'
import type { UserView, UserStatus, Lookups, EmployeeOption } from '~/composables/api/useUsers'
import { useUsers } from '~/composables/api/useUsers'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const { open: confirm } = useConfirm()
const api = useUsers()

const PAGE_SIZE = 10

const rows = ref<UserView[]>([])
const total = ref(0)
const lookups = ref<Lookups>({ roles: [], offices: [], employees: [] })
const limit = ref(PAGE_SIZE)
const offset = ref(0)
const search = ref('')
const loading = ref(true)
const loadFailed = ref(false)

// id → name maps for table resolution.
const roleMap = computed(() => new Map(lookups.value.roles.map(r => [r.id, r.name])))
const officeMap = computed(() => new Map(lookups.value.offices.map(o => [o.id, o.name])))
const employeeMap = computed(() => new Map(lookups.value.employees.map(e => [e.id, e.name])))
function roleName(id: string): string {
  return roleMap.value.get(id) ?? id
}
function officeName(id: string | null): string {
  return id ? (officeMap.value.get(id) ?? id) : ''
}
function employeeName(id: string | null): string {
  return id ? (employeeMap.value.get(id) ?? id) : ''
}

const columns = [
  { accessorKey: 'name', header: t('settings.users.columns.nama') },
  { accessorKey: 'role', header: t('settings.users.columns.peran') },
  { accessorKey: 'office', header: t('settings.users.columns.kantor') },
  { accessorKey: 'employee', header: t('settings.users.columns.pegawai') },
  { accessorKey: 'login', header: t('settings.users.columns.login') },
  { accessorKey: 'status', header: t('settings.users.columns.status') }
]

const roleFormOptions = computed(() => lookups.value.roles.map(r => ({ value: r.id, label: r.name })))
const officeFormOptions = computed(() => lookups.value.offices.map(o => ({ value: o.id, label: o.name })))
const statusFormOptions = [
  { value: 'active', label: t('settings.users.status.active') },
  { value: 'inactive', label: t('settings.users.status.inactive') },
  { value: 'suspended', label: t('settings.users.status.suspended') }
]

const statusMeta: Record<UserStatus, { color: BadgeColor, dot: string }> = {
  active: { color: 'success', dot: 'bg-success' },
  inactive: { color: 'neutral', dot: 'bg-[var(--ui-text-dimmed)]' },
  suspended: { color: 'warning', dot: 'bg-warning' }
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
}

// ── Form state ────────────────────────────────────────────────────────────────
const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive({
  name: '', email: '', password: '', role_id: '', office_id: '', employee_id: '', status: 'active' as UserStatus
})
const errors = reactive<{ name?: string, email?: string, role_id?: string }>({})
const EMAIL_RE = /^.+@.+\..+$/

// Employee options for the form: only employees of the selected office.
const employeeFormOptions = computed(() =>
  lookups.value.employees
    .filter((e: EmployeeOption) => e.office_id === form.office_id)
    .map(e => ({ value: e.id, label: e.name }))
)
// When the office changes, drop a now-mismatched employee selection.
watch(() => form.office_id, () => {
  if (form.employee_id && !employeeFormOptions.value.some(o => o.value === form.employee_id)) {
    form.employee_id = ''
  }
})

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can('user.manage')) return []
  const r = row as unknown as UserView
  return [
    { label: t('settings.users.actions.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    r.status === 'active'
      ? { label: t('settings.users.actions.deactivate'), icon: 'i-lucide-ban', onSelect: () => onToggleStatus(r) }
      : { label: t('settings.users.actions.activate'), icon: 'i-lucide-circle-check', onSelect: () => onToggleStatus(r) },
    { label: t('settings.users.actions.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

async function loadList() {
  loading.value = true
  loadFailed.value = false
  try {
    const res = await api.list({ search: search.value.trim() || undefined, limit: limit.value, offset: offset.value })
    rows.value = res.rows
    total.value = res.total
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const [lk, res] = await Promise.all([
      api.lookups(),
      api.list({ search: search.value.trim() || undefined, limit: limit.value, offset: offset.value })
    ])
    lookups.value = lk
    rows.value = res.rows
    total.value = res.total
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function clearErrors() {
  delete errors.name
  delete errors.email
  delete errors.role_id
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { name: '', email: '', password: '', role_id: '', office_id: '', employee_id: '', status: 'active' })
  clearErrors()
  formOpen.value = true
}

function openEdit(row: UserView) {
  editingId.value = row.id
  Object.assign(form, {
    name: row.name, email: row.email, password: '', role_id: row.role_id,
    office_id: row.office_id ?? '', employee_id: row.employee_id ?? '', status: row.status
  })
  clearErrors()
  formOpen.value = true
}

function validate(): boolean {
  clearErrors()
  if (!form.name.trim()) errors.name = t('settings.users.required')
  if (!editingId.value) {
    if (!form.email.trim()) errors.email = t('settings.users.required')
    else if (!EMAIL_RE.test(form.email)) errors.email = t('settings.users.invalidEmail')
  }
  if (!form.role_id) errors.role_id = t('settings.users.required')
  return !errors.name && !errors.email && !errors.role_id
}

async function onSubmit() {
  if (!validate()) return
  saving.value = true
  try {
    if (editingId.value) {
      await api.update(editingId.value, {
        name: form.name, role_id: form.role_id, status: form.status,
        office_id: form.office_id || undefined, employee_id: form.employee_id || undefined
      })
    } else {
      await api.create({
        name: form.name, email: form.email, password: form.password || undefined,
        role_id: form.role_id, office_id: form.office_id || undefined, employee_id: form.employee_id || undefined
      })
    }
    formOpen.value = false
    await loadList()
  } catch (err: unknown) {
    if ((err as { statusCode?: number }).statusCode === 409) errors.email = t('settings.users.conflict')
    else toast.add({ title: t('settings.users.loadError'), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onToggleStatus(row: UserView) {
  const next: UserStatus = row.status === 'active' ? 'inactive' : 'active'
  try {
    await api.update(row.id, {
      name: row.name, role_id: row.role_id, status: next,
      office_id: row.office_id ?? undefined, employee_id: row.employee_id ?? undefined
    })
    toast.add({ title: t('settings.users.toast.statusChanged'), color: 'success', icon: 'i-lucide-check' })
    await loadList()
  } catch { /* useApiClient toasts */ }
}

async function onDelete(row: UserView) {
  const ok = await confirm({
    title: t('settings.users.deleteTitle'),
    description: t('settings.users.deleteConfirm', { nama: row.name, email: row.email })
  })
  if (!ok) return
  try {
    await api.remove(row.id)
    await loadList()
  } catch { /* useApiClient toasts */ }
}

watch(search, () => {
  offset.value = 0
  loadList()
})
watch(offset, () => loadList())
onMounted(() => load())
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
    </div>

    <div
      v-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-20 text-muted"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-6"
      />
      <span class="text-sm">{{ t('settings.users.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="load"
      >
        {{ t('settings.users.retry') }}
      </UButton>
    </div>

    <template v-else>
      <ResourceTable
        :rows="(rows as unknown as Record<string, unknown>[])"
        :columns="columns"
        :loading="loading"
        :total="total"
        :limit="limit"
        :offset="offset"
        :empty-title="search ? t('settings.users.emptyFilter') : t('settings.users.empty')"
        :actions="rowActions"
        @update:offset="offset = $event"
      >
        <template #name-cell="{ row }">
          <div class="flex items-center gap-[11px]">
            <span class="w-[34px] h-[34px] rounded-full bg-primary/10 text-primary flex items-center justify-center font-bold text-[12px] flex-none">
              {{ initials((row as unknown as UserView).name) }}
            </span>
            <div class="min-w-0">
              <div class="font-semibold text-[13.5px]">
                {{ (row as unknown as UserView).name }}
              </div>
              <div class="text-xs text-muted">
                {{ (row as unknown as UserView).email }}
              </div>
            </div>
          </div>
        </template>

        <template #role-cell="{ row }">
          <UBadge
            color="primary"
            variant="subtle"
            class="rounded-full"
          >
            {{ roleName((row as unknown as UserView).role_id) }}
          </UBadge>
        </template>

        <template #office-cell="{ row }">
          <span class="text-muted">{{ officeName((row as unknown as UserView).office_id) || '—' }}</span>
        </template>

        <template #employee-cell="{ row }">
          <span :class="(row as unknown as UserView).employee_id ? 'text-default' : 'text-dimmed'">
            {{ employeeName((row as unknown as UserView).employee_id) || '—' }}
          </span>
        </template>

        <template #login-cell="{ row }">
          <span class="inline-flex items-center gap-[7px] text-[13px] text-muted">
            <UIcon
              :name="(row as unknown as UserView).google_linked ? 'i-simple-icons-google' : 'i-lucide-mail'"
              class="size-[15px]"
            />
            {{ t((row as unknown as UserView).google_linked ? 'settings.users.login.google' : 'settings.users.login.email') }}
          </span>
        </template>

        <template #status-cell="{ row }">
          <UBadge
            :color="statusMeta[(row as unknown as UserView).status].color"
            variant="subtle"
            class="rounded-full gap-1.5"
          >
            <span
              class="size-1.5 rounded-full"
              :class="statusMeta[(row as unknown as UserView).status].dot"
            />
            {{ t('settings.users.status.' + (row as unknown as UserView).status) }}
          </UBadge>
        </template>
      </ResourceTable>
    </template>

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
          :error="errors.name"
        >
          <UInput
            v-model="form.name"
            :placeholder="t('settings.users.placeholders.nama')"
            class="w-full"
          />
        </UFormField>

        <template v-if="!editingId">
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
        </template>

        <div class="grid grid-cols-2 gap-[14px]">
          <UFormField
            :label="t('settings.users.fields.peran')"
            required
            :error="errors.role_id"
          >
            <USelect
              v-model="form.role_id"
              :items="roleFormOptions"
              :placeholder="t('settings.users.placeholders.pilih')"
              class="w-full"
            />
          </UFormField>

          <template v-if="editingId">
            <UFormField :label="t('settings.users.fields.status')">
              <USelect
                v-model="form.status"
                :items="statusFormOptions"
                class="w-full"
              />
            </UFormField>
          </template>
        </div>

        <UFormField :label="t('settings.users.fields.kantor')">
          <USelect
            v-model="form.office_id"
            :items="officeFormOptions"
            :placeholder="t('settings.users.placeholders.pilih')"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('settings.users.fields.pegawai')">
          <USelect
            v-model="form.employee_id"
            :items="employeeFormOptions"
            :disabled="!form.office_id"
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
