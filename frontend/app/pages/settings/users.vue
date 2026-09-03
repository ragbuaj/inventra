<script setup lang="ts">
import type { BadgeColor, RowAction } from '~/types'
import type { UserView, UserStatus, Lookups } from '~/composables/api/useUsers'
import { useUsers } from '~/composables/api/useUsers'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

const ALL = '__all__'

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const { open: confirm } = useConfirm()
const api = useUsers()
const office = useOfficePicker()
const employee = useEmployeePicker()

const PAGE_SIZE = 10

const isCompact = useIsCompact()
const lookups = ref<Lookups>({ roles: [] })
const limit = ref(PAGE_SIZE)
const offset = ref(0)
const search = ref('')
const fRole = ref<string>(ALL)
const fOffice = ref<string | null>(null)
const fStatus = ref<string>(ALL)

// The scrolling ancestor (see layouts/default.vue) — the sentinel below the
// table observes intersections against it, not the viewport.
const scrollParent = ref<HTMLElement | null>(null)

// One data engine for both layouts: page buttons drive `loadPage`, the compact
// layout accumulates with `loadFirst`/`loadMore`. See useInfiniteRows.
const list = useInfiniteRows<UserView>(
  async ({ limit: l, offset: o }) => {
    const res = await api.list({
      search: search.value.trim() || undefined,
      roleId: fRole.value !== ALL ? fRole.value : undefined,
      officeId: fOffice.value ?? undefined,
      status: fStatus.value !== ALL ? (fStatus.value as UserStatus) : undefined,
      limit: l,
      offset: o
    })
    return { data: res.rows, total: res.total }
  },
  { limit: PAGE_SIZE }
)
// Pulled out as top-level bindings so the template auto-unwraps them.
const rows = list.rows
const total = list.total
const loading = list.loading
const loadingMore = list.loadingMore
const listDone = list.done
const listError = list.error
// The full-page error screen is only for "nothing to show"; a failed append
// keeps the rows on screen and offers its own inline retry.
const loadFailed = computed(() => list.error.value && rows.value.length === 0)

// id → name maps for table resolution. Role stays an eager map (small,
// bounded reference list); office/employee resolve lazily on demand via the
// picker adapters' resolveFn (no more eager `{ limit: 100 }` lists).
const roleMap = computed(() => new Map(lookups.value.roles.map(r => [r.id, r.name])))
const officeCache = useResolveCache(office.resolveFn)
const employeeCache = useResolveCache(employee.resolveFn)
function roleName(id: string): string {
  return roleMap.value.get(id) ?? id
}
function officeName(id: string | null): string {
  return id ? officeCache.get(id) : ''
}
function employeeName(id: string | null): string {
  return id ? employeeCache.get(id) : ''
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
const statusFormOptions = [
  { value: 'active', label: t('settings.users.status.active') },
  { value: 'inactive', label: t('settings.users.status.inactive') },
  { value: 'suspended', label: t('settings.users.status.suspended') }
]

// Filter-bar options — same source lists as the form, prefixed with an
// "all" clear option so the USelect can represent "no filter".
const roleFilterOptions = computed(() => [
  { value: ALL, label: t('settings.users.filter.allRoles') },
  ...roleFormOptions.value
])
const statusFilterOptions = computed(() => [
  { value: ALL, label: t('settings.users.filter.allStatus') },
  ...statusFormOptions
])

const anyFilter = computed(() =>
  !!(search.value.trim() || fRole.value !== ALL || fOffice.value || fStatus.value !== ALL)
)

// Advanced filters only — the search box stands on its own in the filter bar,
// so it must not inflate the count badge next to the filter button.
const advancedFilterCount = computed(() =>
  (fRole.value !== ALL ? 1 : 0) + (fOffice.value ? 1 : 0) + (fStatus.value !== ALL ? 1 : 0)
)

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

// NOTE (Task 4 deviation): the employee field used to filter its options
// client-side to the selected office (from an eagerly-fetched `{ limit: 100
// }` employees array). The async picker searches all employees server-side —
// there's no office-scoped employee search endpoint, so that narrowing is no
// longer possible without a backend change. The field disables until an
// office is chosen (see below), and any office change still drops a
// previously-picked employee as a safety net (we can no longer verify
// membership client-side). `flush: 'sync'` matters here: openCreate/openEdit
// Object.assign office_id *then* employee_id in the same call — a sync watch
// fires between those two writes (clearing whatever stale employee_id was
// still there from before), then the assign's own employee_id write lands
// right after and is never touched. A user picking a *new* office via the
// AsyncSearchPicker only ever changes office_id, so the clear sticks.
watch(() => form.office_id, () => {
  if (form.employee_id) form.employee_id = ''
}, { flush: 'sync' })

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can('user.manage')) return []
  const r = row as unknown as UserView
  return [
    { label: t('settings.users.actions.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    { label: t('settings.users.actions.resetPassword'), icon: 'i-lucide-key-round', onSelect: () => onResetPassword(r) },
    r.status === 'active'
      ? { label: t('settings.users.actions.deactivate'), icon: 'i-lucide-ban', onSelect: () => onToggleStatus(r) }
      : { label: t('settings.users.actions.activate'), icon: 'i-lucide-circle-check', onSelect: () => onToggleStatus(r) },
    { label: t('settings.users.actions.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

function loadList() {
  return isCompact.value ? list.loadFirst() : list.loadPage(offset.value)
}

async function load() {
  // Lookups are supplementary: a failure there must not take the list down
  // with it, so it is awaited alongside but swallows its own error.
  const lookupsLoad = api.lookups()
    .then((lk) => { lookups.value = lk })
    .catch(() => {})
  await Promise.all([lookupsLoad, loadList()])
}

function scrollToTop() {
  scrollParent.value?.scrollTo({ top: 0 })
}

function reloadFromStart() {
  list.reset()
  scrollToTop()
  // Writing offset only reloads when it actually changes; when it was already
  // 0 the watcher never fires, so this function must load itself. Loading in
  // both paths would fire two requests for one filter change.
  const alreadyFirst = offset.value === 0
  offset.value = 0
  if (alreadyFirst) loadList()
}

function resetFilters() {
  search.value = ''
  fRole.value = ALL
  fOffice.value = null
  fStatus.value = ALL
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

async function onResetPassword(row: UserView) {
  const ok = await confirm({
    title: t('settings.users.resetTitle'),
    description: t('settings.users.resetConfirm', { nama: row.name, email: row.email }),
    confirmLabel: t('settings.users.resetConfirmLabel'),
    color: 'primary'
  })
  if (!ok) return
  try {
    const res = await api.resetPassword(row.id)
    toast.add({
      title: t('settings.users.toast.passwordReset', { email: res.email }),
      color: 'success',
      icon: 'i-lucide-mail-check'
    })
  } catch (err: unknown) {
    if ((err as { statusCode?: number }).statusCode === 422) {
      toast.add({ title: t('settings.users.toast.resetGoogleOnly'), color: 'warning', icon: 'i-lucide-triangle-alert' })
    } else {
      toast.add({ title: t('settings.users.toast.resetError'), color: 'error' })
    }
  }
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

watch([search, fRole, fOffice, fStatus], () => reloadFromStart())
watch(offset, () => loadList())

// Crossing the breakpoint swaps accumulate and replace semantics, so the rows
// held under the old mode no longer describe what the new one shows.
watch(isCompact, () => reloadFromStart())

onMounted(() => {
  scrollParent.value = document.querySelector('main')
  load()
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
    <FilterBar
      v-model:search="search"
      :search-placeholder="t('settings.users.searchPlaceholder')"
      :active-count="advancedFilterCount"
      :show-reset="anyFilter"
      :total="total"
      testid="users-filter"
      @reset="resetFilters"
    >
      <template #filters>
        <USelect
          v-model="fRole"
          data-testid="users-role-filter"
          :items="roleFilterOptions"
          class="min-w-[150px]"
        />
        <AsyncSearchPicker
          :model-value="fOffice"
          :search-fn="office.searchFn"
          :resolve-fn="office.resolveFn"
          :placeholder="t('common.searchOffice')"
          testid="users-filter-office"
          clearable
          class="min-w-[190px]"
          @update:model-value="fOffice = $event"
        />
        <USelect
          v-model="fStatus"
          data-testid="users-status-filter"
          :items="statusFilterOptions"
          class="min-w-[140px]"
        />
      </template>
    </FilterBar>

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
        :empty-title="anyFilter ? t('settings.users.emptyFilter') : t('settings.users.empty')"
        :actions="rowActions"
        infinite
        :loading-more="loadingMore"
        :done="listDone"
        :error="listError"
        :scroll-parent="scrollParent"
        @update:offset="offset = $event"
        @load-more="list.loadMore"
        @retry="list.retry"
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

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-[14px]">
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
          <AsyncSearchPicker
            :model-value="form.office_id || null"
            :search-fn="office.searchFn"
            :resolve-fn="office.resolveFn"
            :placeholder="t('common.searchOffice')"
            testid="office"
            @update:model-value="form.office_id = $event ?? ''"
          />
        </UFormField>

        <UFormField :label="t('settings.users.fields.pegawai')">
          <AsyncSearchPicker
            :model-value="form.employee_id || null"
            :search-fn="employee.searchFn"
            :resolve-fn="employee.resolveFn"
            :disabled="!form.office_id"
            :placeholder="t('common.searchEmployee')"
            testid="employee"
            @update:model-value="form.employee_id = $event ?? ''"
          />
          <template #hint>
            <span class="text-xs text-dimmed mt-1">{{ t('settings.users.pegawaiNote') }}</span>
          </template>
        </UFormField>
      </div>
    </FormSlideover>
  </div>
</template>
