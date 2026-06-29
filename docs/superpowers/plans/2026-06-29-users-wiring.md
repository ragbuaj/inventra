# Wire User Management screen to `/api/v1/users` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mock-backed `useUsers` composable + User Management screen with the real `/api/v1/users` CRUD backend (English DTO + UUID identity), resolving role/office/employee NAMES via real lookups and filtering the employee picker by the selected office.

**Architecture:** `useUsers` calls `useApiClient` against `/api/v1/users` (list/create/update/delete) plus a `lookups()` that fetches roles (`/authz/roles`), offices (`/offices`), employees (`/employees`). The page does server-side search + pagination, resolves FK ids → names for the table via the lookups, and the create/edit form uses real role/office/employee pickers (employee options filtered to the selected office).

**Tech Stack:** Nuxt 4 (SPA), Nuxt UI (`U*`), `@nuxtjs/i18n` (id default + en), Vitest + `@nuxt/test-utils`, Playwright e2e.

## Global Constraints

- Wire ONLY the User Management screen: `pages/settings/users.vue`, `composables/api/useUsers.ts`. Do NOT touch `useOffices`/`useEmployees` (inline-fetch lookups instead) or the shared `~/types` `AuthUser`/auth code.
- English DTO + UUID identity. `UserView{id,name,email,role_id,office_id,employee_id,status,avatar_url,google_linked,created_at,updated_at}` defined in `useUsers.ts` (do NOT reuse the Indonesian `~/types` `User`).
- Backend: list `GET /users` supports ONLY `search/limit/offset` (envelope `{data,total,limit,offset}`, field-permission filtered). Create `POST /users` `{name,email,password?,role_id,office_id?,employee_id?}`. Update `PUT /users/:id` `{name,role_id,status,office_id?,employee_id?}` (no email/password). Delete `DELETE /users/:id` → 204. No `setStatus`/`resetPassword` endpoints.
- DROP the role/office/status filter dropdowns (not server-supported) — keep server-side search + pagination. DROP the reset-password row action (no backend) and the login form field (backend derives `google_linked`).
- Status change = `update()` with the row's current fields + new status. Email `409` → inline form error.
- Lookups: `/authz/roles` (roles), `/offices?limit=100` (offices), `/employees?limit=100` (employees, carry `office_id`). Used for table name resolution AND form pickers. Employee picker options = employees whose `office_id` === the form's selected `office_id`; changing the office clears a now-mismatched `employee_id`.
- All API calls via `useApiClient().request<T>`; never hardcode the backend URL.
- i18n mandatory; gate stays `user.manage`.
- Match `docs/design/Manajemen User.dc.html`; the dropped filters/reset-password/login are an approved deviation.
- Gates: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green. Run from `frontend/`.

---

### Task 1: Rewrite `useUsers` to the real API

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useUsers.ts`
- Delete: `frontend/test/unit/users-mock.spec.ts`
- Test: `frontend/test/unit/use-users.spec.ts`

**Interfaces:**
- Consumes: `useApiClient().request`.
- Produces: types `UserStatus`, `UserView`, `CreateUserInput`, `UpdateUserInput`, `Option`, `EmployeeOption`, `Lookups`; `useUsers()` → `{ list, create, update, remove, lookups }`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/test/unit/use-users.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

import { useUsers } from '~/composables/api/useUsers'

beforeEach(() => request.mockReset())

describe('useUsers', () => {
  it('list builds the query (omits empty search) and returns {rows,total}', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'u1', name: 'A', email: 'a@x.id', role_id: 'r1', office_id: null, employee_id: null, status: 'active', avatar_url: null, google_linked: false, created_at: null, updated_at: null }], total: 1 })
    const res = await useUsers().list({ search: '', limit: 20, offset: 40 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/users?')
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=40')
    expect(path).not.toContain('search=')
    expect(res).toEqual({ rows: expect.any(Array), total: 1 })
    expect(res.rows[0].id).toBe('u1')
  })

  it('create sends only non-empty optional fields', async () => {
    request.mockResolvedValueOnce({ id: 'n1' })
    await useUsers().create({ name: 'New', email: 'n@x.id', role_id: 'r1' })
    expect(request).toHaveBeenCalledWith('/users', { method: 'POST', body: { name: 'New', email: 'n@x.id', role_id: 'r1' } })
  })

  it('create includes password/office_id/employee_id when present', async () => {
    request.mockResolvedValueOnce({ id: 'n2' })
    await useUsers().create({ name: 'New', email: 'n@x.id', role_id: 'r1', password: 'pw', office_id: 'o1', employee_id: 'e1' })
    expect(request).toHaveBeenCalledWith('/users', { method: 'POST', body: { name: 'New', email: 'n@x.id', role_id: 'r1', password: 'pw', office_id: 'o1', employee_id: 'e1' } })
  })

  it('update PUTs name/role_id/status (+ optional office/employee)', async () => {
    request.mockResolvedValueOnce({ id: 'u1' })
    await useUsers().update('u1', { name: 'A', role_id: 'r1', status: 'inactive', office_id: 'o1' })
    expect(request).toHaveBeenCalledWith('/users/u1', { method: 'PUT', body: { name: 'A', role_id: 'r1', status: 'inactive', office_id: 'o1' } })
  })

  it('remove DELETEs', async () => {
    request.mockResolvedValueOnce(undefined)
    await useUsers().remove('u1')
    expect(request).toHaveBeenCalledWith('/users/u1', { method: 'DELETE' })
  })

  it('lookups maps roles/offices/employees (employees carry office_id)', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }] })       // /authz/roles
      .mockResolvedValueOnce({ data: [{ id: 'o1', name: 'Pusat' }] })          // /offices
      .mockResolvedValueOnce({ data: [{ id: 'e1', name: 'Budi', office_id: 'o1' }] }) // /employees
    const lk = await useUsers().lookups()
    expect(request.mock.calls.map(c => (c[0] as string).split('?')[0])).toEqual(['/authz/roles', '/offices', '/employees'])
    expect(lk.roles).toEqual([{ id: 'r1', name: 'Manager' }])
    expect(lk.offices).toEqual([{ id: 'o1', name: 'Pusat' }])
    expect(lk.employees).toEqual([{ id: 'e1', name: 'Budi', office_id: 'o1' }])
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-users`
Expected: FAIL — new `useUsers` shape undefined.

- [ ] **Step 3: Rewrite `useUsers.ts`**

Replace `frontend/app/composables/api/useUsers.ts` entirely with:

```ts
export type UserStatus = 'active' | 'inactive' | 'suspended'

export interface UserView {
  id: string
  name: string
  email: string
  role_id: string
  office_id: string | null
  employee_id: string | null
  status: UserStatus
  avatar_url: string | null
  google_linked: boolean
  created_at: string | null
  updated_at: string | null
}

export interface CreateUserInput {
  name: string
  email: string
  password?: string
  role_id: string
  office_id?: string
  employee_id?: string
}

export interface UpdateUserInput {
  name: string
  role_id: string
  status: UserStatus
  office_id?: string
  employee_id?: string
}

export interface Option { id: string; name: string }
export interface EmployeeOption extends Option { office_id: string }
export interface Lookups { roles: Option[]; offices: Option[]; employees: EmployeeOption[] }

interface RoleDTO { id: string; name: string }
interface OfficeDTO { id: string; name: string }
interface EmployeeDTO { id: string; name: string; office_id: string }

/**
 * User management, wired to /api/v1/users. List is server-side search+pagination
 * (the backend supports only search/limit/offset). Role/office/employee NAMES are
 * resolved client-side from lookups() (the list returns FK UUIDs only).
 */
export function useUsers() {
  const { request } = useApiClient()

  async function list(params: { search?: string; limit: number; offset: number }): Promise<{ rows: UserView[]; total: number }> {
    const q = new URLSearchParams()
    q.set('limit', String(params.limit))
    q.set('offset', String(params.offset))
    if (params.search) q.set('search', params.search)
    const res = await request<{ data: UserView[]; total: number }>(`/users?${q.toString()}`)
    return { rows: res.data, total: res.total }
  }

  async function create(input: CreateUserInput): Promise<UserView> {
    const body: Record<string, unknown> = { name: input.name, email: input.email, role_id: input.role_id }
    if (input.password) body.password = input.password
    if (input.office_id) body.office_id = input.office_id
    if (input.employee_id) body.employee_id = input.employee_id
    return request<UserView>('/users', { method: 'POST', body })
  }

  async function update(id: string, input: UpdateUserInput): Promise<UserView> {
    const body: Record<string, unknown> = { name: input.name, role_id: input.role_id, status: input.status }
    if (input.office_id) body.office_id = input.office_id
    if (input.employee_id) body.employee_id = input.employee_id
    return request<UserView>(`/users/${id}`, { method: 'PUT', body })
  }

  async function remove(id: string): Promise<void> {
    await request(`/users/${id}`, { method: 'DELETE' })
  }

  async function lookups(): Promise<Lookups> {
    const [roles, offices, employees] = await Promise.all([
      request<{ data: RoleDTO[] }>('/authz/roles'),
      request<{ data: OfficeDTO[] }>('/offices?limit=100'),
      request<{ data: EmployeeDTO[] }>('/employees?limit=100')
    ])
    return {
      roles: roles.data.map(r => ({ id: r.id, name: r.name })),
      offices: offices.data.map(o => ({ id: o.id, name: o.name })),
      employees: employees.data.map(e => ({ id: e.id, name: e.name, office_id: e.office_id }))
    }
  }

  return { list, create, update, remove, lookups }
}
```

- [ ] **Step 4: Run tests + lint**

Run (from `frontend/`): `pnpm test -- use-users && pnpm lint`
Expected: PASS, lint clean. NOTE: `pnpm typecheck` will fail ONLY in `pages/settings/users.vue` (old shape / `~/mock/users` / `~/types` `User`) — EXPECTED, fixed in Task 2. Do NOT edit the page here.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useUsers.ts frontend/test/unit/use-users.spec.ts
git rm frontend/test/unit/users-mock.spec.ts
git commit -m "feat(users): wire useUsers to /api/v1/users (English DTO + lookups)"
```

---

### Task 2: Rewrite the page (server-side, real pickers, employee-by-office) + i18n

**Files:**
- Modify (script + template): `frontend/app/pages/settings/users.vue`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json` (add `settings.users.loadError`/`retry` + `conflict` if missing)

**Interfaces:**
- Consumes: `useUsers()` (Task 1) + `UserView`/`UserStatus`/`CreateUserInput`/`UpdateUserInput`/`Lookups`/`Option`/`EmployeeOption`.

- [ ] **Step 1: Add i18n keys**

In `frontend/i18n/locales/id.json` and `en.json`, under `settings.users`, add `loadError`, `retry`, and `conflict` if not present. Read the section first; insert valid JSON.
- id: `"loadError": "Gagal memuat data user."`, `"retry": "Coba lagi"`, `"conflict": "Email sudah dipakai."`
- en: `"loadError": "Failed to load users."`, `"retry": "Retry"`, `"conflict": "Email already in use."`

- [ ] **Step 2: Rewrite the page `<script setup>`**

Replace the `<script setup>` block of `frontend/app/pages/settings/users.vue` with:

```ts
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
function roleName(id: string): string { return roleMap.value.get(id) ?? id }
function officeName(id: string | null): string { return id ? (officeMap.value.get(id) ?? id) : '' }
function employeeName(id: string | null): string { return id ? (employeeMap.value.get(id) ?? id) : '' }

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

const statusMeta: Record<UserStatus, { color: BadgeColor; dot: string }> = {
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
const errors = reactive<{ name?: string; email?: string; role_id?: string }>({})
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

watch(search, () => { offset.value = 0; loadList() })
watch(offset, () => loadList())
onMounted(() => load())
```

Notes vs the old script: server-side `loadList()` (search/offset refetch); `load()` bootstraps lookups + first page; FK→name resolution via maps; create-vs-edit field rules (email/password only on create; status only on edit); `role_id` required; employee options filtered by `form.office_id` with auto-clear; status toggle via `update()`; reset-password + the 3 filter dropdowns removed; columns are accessor keys `role`/`office`/`employee`/`login`/`status` resolved in cell slots (below).

- [ ] **Step 3: Update the page template**

Apply these template edits to `users.vue`:
- **Filter bar**: DELETE the three `<USelect v-model="filterRole|filterKantor|filterStatus">` blocks and the reset `UButton` (search-only now). Keep the search `<UInput v-model="search">`.
- **Loading/error states**: after the filter bar and before `<ResourceTable>`, add a load-error block; render the table only when not failed:
```vue
    <div
      v-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-20 text-muted"
    >
      <UIcon name="i-lucide-circle-alert" class="size-6" />
      <span class="text-sm">{{ t('settings.users.loadError') }}</span>
      <UButton color="neutral" variant="subtle" @click="load">
        {{ t('settings.users.retry') }}
      </UButton>
    </div>
```
  and wrap the `<ResourceTable>` in `<template v-else>` (or add `v-else` on its wrapper).
- **ResourceTable props**: `:rows="(rows as unknown as Record<string, unknown>[])"`, `:total="total"`, `:loading="loading"`, `:limit="limit"`, `:offset="offset"`, `:empty-title="search ? t('settings.users.emptyFilter') : t('settings.users.empty')"`, `:actions="rowActions"`, `@update:offset="offset = $event"`. Remove `v-model:sorting` and the `sorting`-based props (server-side; no sort).
- **Cell slots**: update the slot names + bindings to the new accessor keys and resolved names:
  - `#name-cell` (was `#nama-cell`): `initials((row as unknown as UserView).name)`, `{{ (row as unknown as UserView).name }}`, `{{ (row as unknown as UserView).email }}`.
  - `#role-cell` (was `#peran-cell`): `<UBadge color="primary" variant="subtle" class="rounded-full">{{ roleName((row as unknown as UserView).role_id) }}</UBadge>`.
  - `#office-cell` (was `#kantor-cell`): `{{ officeName((row as unknown as UserView).office_id) || '—' }}`.
  - `#employee-cell` (was `#pegawai-cell`): `{{ employeeName((row as unknown as UserView).employee_id) || '—' }}`.
  - `#login-cell`: drive off `google_linked`: icon `(row as unknown as UserView).google_linked ? 'i-simple-icons-google' : 'i-lucide-mail'`; label `t((row as unknown as UserView).google_linked ? 'settings.users.login.google' : 'settings.users.login.email')`.
  - `#status-cell`: `statusMeta[(row as unknown as UserView).status]` + `t('settings.users.status.' + (row as unknown as UserView).status)`.
- **Form (FormSlideover)**: rebind to the new `form` keys and add create/edit conditionals:
  - Name: `v-model="form.name"`, `:error="errors.name"`.
  - Email: `v-model="form.email"`; wrap in `<template v-if="!editingId">` (email only on create) — on edit, show it read-only/disabled OR omit. Use `:disabled="!!editingId"` if you keep it visible; simplest: only render the email + password fields when `!editingId`.
  - Password: only render when `!editingId` (`<template v-if="!editingId">`).
  - Role: `<USelect v-model="form.role_id" :items="roleFormOptions" ... />` with `:error`-style required (bind `errors.role_id` via a `UFormField :error`).
  - Status: only render when `editingId` (`<template v-if="editingId">`).
  - Office: `<USelect v-model="form.office_id" :items="officeFormOptions" .../>`.
  - Employee: `<USelect v-model="form.employee_id" :items="employeeFormOptions" :disabled="!form.office_id" .../>` (disabled until an office is chosen; options already filtered to that office). Keep the `pegawaiNote` hint.
  - DELETE the `login` form field (none exists in the new form).

- [ ] **Step 4: Verify build/lint/typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: exit 0. NOTE: `pnpm test` will still FAIL on `test/nuxt/settings-users.spec.ts` (old mock stub) — fixed in Task 3. Run `pnpm test -- use-users` to confirm Task 1 units pass.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/settings/users.vue frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "feat(users): page on real API (server-side, real pickers, employee-by-office)"
```

---

### Task 3: Nuxt component test for the wired page

**Files:**
- Modify (rewrite): `frontend/test/nuxt/settings-users.spec.ts`

**Interfaces:**
- Consumes: the wired page; mock the HTTP layer the way the Data Scope / Field Permission / Audit component tests do.

- [ ] **Step 1: Study the patterns**

Read the CURRENT `frontend/test/nuxt/settings-users.spec.ts` AND `frontend/test/nuxt/settings-audit.spec.ts` (a wired screen — `vi.mock('~/composables/useApiClient', ...)` + per-test `setHandler` capturing method+path+body, `useAuthStore().setSession(token, user, ['*'])` + `mountSuspended`; and note the technique of setting a page reactive ref directly via `wrapper.vm` when a Nuxt UI `USelect`'s jsdom click won't propagate). The page calls `GET /users`, `GET /authz/roles`, `GET /offices`, `GET /employees`, `POST /users`, `PUT /users/:id`, `DELETE /users/:id`.

- [ ] **Step 2: Write the rewritten test**

Rewrite `frontend/test/nuxt/settings-users.spec.ts` to stub the endpoints and assert real behavior. Fixtures: a users page (e.g. 2 users with role_id/office_id/employee_id), a roles list (`[{id:'r1',name:'Manager'},...]`), offices (`[{id:'o1',name:'Pusat'},{id:'o2',name:'Cabang'}]`), employees (`[{id:'e1',name:'Budi',office_id:'o1'},{id:'e2',name:'Sari',office_id:'o2'}]`). Cover:
- Loaded rows render with RESOLVED names: a user's role shows "Manager" (not the UUID), office shows "Pusat", employee shows the name; login badge from `google_linked`; status badge.
- Open Create → role/office pickers list the lookup names. **Employee picker filtered by office**: set `form.office_id` (via `wrapper.vm` if the USelect click won't propagate) to `o1` → the employee options include only `e1` (Budi), not `e2`; switching to `o2` clears a previously-selected `e1`.
- Submit Create → `POST /users` with body `{name,email,role_id,office_id,employee_id}` (assert the captured body; omitted password); a stubbed `409` → inline email conflict error renders.
- Edit a row → `PUT /users/:id` with `{name,role_id,status,office_id?,employee_id?}` (assert body; no email/password); status change via the row action issues a `PUT` with the new status.
- Delete a row (confirm) → `DELETE /users/:id`.
- Search → `GET /users?...search=...&offset=0`. Load-error: `GET /users` 500 → error block + retry.

Assert real rendered text + captured request bodies — no hollow checks. Use the harness default locale.

- [ ] **Step 3: Run the test + whole suite**

Run (from `frontend/`): `pnpm test -- settings-users` then `pnpm test`
Expected: target PASS; whole suite green.

- [ ] **Step 4: Commit**

```bash
git add frontend/test/nuxt/settings-users.spec.ts
git commit -m "test(users): component test against stubbed /users endpoints"
```

---

### Task 4: E2E + delete mock + mockup + PROGRESS + gate

**Files:**
- Modify: `frontend/e2e/settings.spec.ts`
- Delete (if orphaned): `frontend/app/mock/users.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Update the e2e User Management assertions**

Read `frontend/e2e/settings.spec.ts` + `frontend/e2e/helpers.ts` (`login()`). Replace the existing `'User management lists seeded users'` smoke test (which asserts mock names like "Bambang Sukasno") with a real-backend spec at `/settings/users`: the seeded admin (`admin@inventra.local`) is a real user, so assert the heading + that the table renders the admin row (by its email `admin@inventra.local` or name) OR the empty-state — deterministically via `await expect(page.getByText('admin@inventra.local').or(page.getByText(<empty i18n>))).toBeVisible()`. Also assert the "Add" button (gated by `user.manage`) is visible, and open the create form (click Add → assert the form slideover heading renders). Keep RBAC/Data Scope/Field Permission/audit specs untouched. ROBUST locators only (text/role; for any USelect, trigger+option — NOT `selectOption`; NO Tailwind-class selectors, NO `isVisible()`/`isEnabled()` snapshot booleans driving control flow, NO `.last()`/`.first()` on broad `div` filters). You likely cannot RUN `pnpm test:e2e` here (needs full backend stack); ensure the spec compiles + lints; it runs in CI. State that in the report.

- [ ] **Step 2: Delete the orphaned mock**

Run `grep -rn "mock/users" frontend/app frontend/test` (exclude the file itself). After Tasks 1–3 the composable/page/old-tests no longer reference it. If ZERO importers remain, `git rm frontend/app/mock/users.ts`. If something still imports it (e.g. another screen using `ROLES`/`KANTOR_OPTIONS`), do NOT delete — report what does, and leave it.

- [ ] **Step 3: Mockup fidelity comparison**

Reference `docs/design/Manajemen User.dc.html`. Structural comparison (read the `.dc.html` + the built `pages/settings/users.vue`): verify header/filter-bar/table/row-actions/create-edit-slideover match. The dropped role/office/status filter dropdowns + reset-password action + login form field are an APPROVED deviation (backend has no server filters / no reset endpoint / derives google_linked) — not a regression. Fix any other genuine deviation; report the result.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`:
- Under the frontend "Wire screens to real backend APIs" sub-list, mark **User Management ✅ wired to `/api/v1/users`** (CRUD; server-side search+pagination; role/office/employee pickers from real lookups; employee picker filtered by office). Note the **authz/settings screen wiring batch is now complete** (RBAC, Data Scope, Field Permission, Audit, User Management).
- Add a TODO note: server-side **role/office/status filters** + **reset-password** are dropped pending backend support; the office/employee **lookup is capped at 100** (a searchable async picker is a follow-up if user/employee counts grow).
- Refresh "▶ Next session — start here": settings-screen wiring done → next is backend bank-FAM (e.g. asset transfer/mutasi, stock opname, disposal) or the field-permission enforcement extension. Don't invent status for other screens.

- [ ] **Step 5: Full frontend gate**

Run (from `frontend/`):
```
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```
Expected: all green. (E2E runs in CI's e2e job.)

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/settings.spec.ts docs/PROGRESS.md
git commit -m "test(users): e2e against real backend + progress; drop orphaned mock"
```
(If `mock/users.ts` was deleted, the `git rm` is staged — include it.)

---

## Self-Review

**Spec coverage:**
- §2 composable rewrite (list/create/update/remove/lookups, English+UUID, omit empties) → Task 1. ✓
- §3 page (server-side search+pagination, drop filters, FK→name resolution, create/edit forms, employee-by-office, status via update, drop reset-password/login, load error) → Task 2. ✓
- §4 types/i18n (UserView in composable not `~/types`; loadError/retry/conflict) → Task 1 (types) + Task 2 (i18n). ✓
- §5 tests (unit/component/e2e) → Tasks 1, 3, 4. ✓
- §6 done (delete mock, mockup, PROGRESS + TODO, gate) → Task 4. ✓

**Placeholder scan:** Tasks 3 & 4 give explicit assertion lists / steps (read the established wired-screen test/e2e patterns first; the USelect/`vm` technique + robust-locator rules are spelled out from the prior screens' CI lessons). Concrete checklists, not "TODO"s.

**Type consistency:** `UserView{id,name,email,role_id,office_id,employee_id,status,avatar_url,google_linked,created_at,updated_at}`, `CreateUserInput`/`UpdateUserInput`, `Lookups{roles,offices,employees}`, `EmployeeOption{...,office_id}`, `useUsers()→{list,create,update,remove,lookups}` consistent across Tasks 1/2/3. Page uses `roleName/officeName/employeeName` maps from `lookups`, `employeeFormOptions` filtered by `form.office_id`, `loadList`/`load`, server-side `rows`/`total` — all consistent with the Task-1 composable. Cell slots renamed to `role`/`office`/`employee` accessor keys resolved via the maps. The shared `~/types` `User`/`AuthUser` and `useOffices`/`useEmployees` are deliberately untouched.
