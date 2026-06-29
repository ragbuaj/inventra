# Wire Master Data Pegawai to `/api/v1/employees` (+ employee phone) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the Pegawai screen from mock to the real `/api/v1/employees` backend — Indonesian→English keys, department/position name→UUID FK pickers with table name-resolution, a new backend `phone` column, and the existing server-enforced `employees` data-scope.

**Architecture:** Backend adds a nullable `phone` column to `masterdata.employees` (migration + DTO + query). Frontend rewrites the `Employee` type + `useEmployees` to the English backend contract, and rewrites the page to use UUID FK pickers (departments/positions via the already-wired `useReference`, offices via an inline scoped `GET /offices`) and resolve FK ids → names in the table. Data-scope is entirely server-enforced; the frontend renders what it gets.

**Tech Stack:** Go 1.25 + Gin + sqlc + golang-migrate (backend); Nuxt 4 SPA + Nuxt UI + @nuxtjs/i18n + Vitest + Playwright (frontend).

## Global Constraints

- **English DTO keys + UUID identity.** Backend employee JSON: `code`(req, the NIP), `name`(req), `email?`, `phone?` (NEW), `avatar_key?`, `department_id?`(UUID), `position_id?`(UUID), `office_id`(req,UUID), `status`(`active|inactive|suspended`, default active). Response also `id, created_at, updated_at`.
- **`nip`→`code`, `nama`→`name`.** `department`/`position` are **UUID FKs** (form picker value = id, NOT name). `telepon` is replaced by the new backend `phone`.
- **Data-scope `employees` is server-enforced per verb** (list/get filtered; create/update reject out-of-scope office → 403; get/delete out-of-scope → 404). The frontend does NO scope logic. Do not change the scope predicate.
- List envelope `{data,total,limit,offset}`; single-row flat; delete 204. Reads `authMW`; writes `authMW + masterdata.office.manage`. Page guard stays `definePageMeta({ middleware:'can', permission:'masterdata.office.manage' })`.
- FK name resolution in the table = frontend maps (departments/positions from `useReference`, offices from inline `GET /offices?limit=100`) — like the office-map/users screens. `useOffices` stays mock (the Kantor sub-project wires it).
- Status: the form toggle sets `active`↔`inactive` (per mockup); `suspended` rows from the backend render with a badge/label (add i18n) but the UI doesn't set `suspended`.
- All frontend HTTP via `useApiClient().request`; i18n both locales; no hardcoded strings; ESLint no-trailing-commas + 1tbs.
- **#40 lesson:** wiring `useEmployees` mock→HTTP makes the page's component test hit the real backend unless it stubs the API. The ONLY `useEmployees` consumer is `pages/master/employees.vue`. `useGlobalSearch` imports the mock `employeeStore` directly (not `useEmployees`) → unaffected; **keep `mock/employees.ts`**. Verify the FULL `pnpm test` exit code is 0.
- Backend gates (from `backend/`): `go build ./...`, `go vet ./...`, `go test ./...`, **and `go test -tags=integration ./...`**, plus Spectral on `backend/api/openapi.yaml`.
- Frontend gates (from `frontend/`): `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
- After migrations/queries: `sqlc generate` (from `backend/`). Don't hand-edit `backend/db/sqlc/`.
- Pre-existing files to handle (not create fresh): `frontend/test/nuxt/master-employees.spec.ts` (mock-based → REWRITE), `frontend/test/unit/employees-mock.spec.ts` (→ DELETE). No e2e employees spec exists yet (CREATE).

---

### Task 1: Backend — add `phone` to employees

**Files:**
- Create: `backend/db/migrations/000019_employee_phone.up.sql`, `.down.sql`
- Modify: `backend/db/queries/employees.sql` (CreateEmployee, UpdateEmployee)
- Regenerate: `backend/db/sqlc/`
- Modify: `backend/internal/masterdata/employee/dto.go` (Request, Response, toInput, toResponse)
- Modify: `backend/internal/masterdata/employee/service.go` (CreateInput, Create, Update)
- Modify: `backend/api/openapi.yaml`
- Test: `backend/internal/masterdata/employee/employee_integration_test.go` (add a phone test)

**Interfaces:**
- Produces: `employee.CreateInput` gains `Phone *string`; `employee.Request`/`Response` gain `Phone *string json:"phone"`; sqlc `MasterdataEmployee`/`CreateEmployeeParams`/`UpdateEmployeeParams` gain `Phone *string`. Frontend Task 2/3 consume `phone`.

- [ ] **Step 1: Write the migration**

`backend/db/migrations/000019_employee_phone.up.sql`:
```sql
ALTER TABLE masterdata.employees ADD COLUMN phone text;
```
`backend/db/migrations/000019_employee_phone.down.sql`:
```sql
ALTER TABLE masterdata.employees DROP COLUMN phone;
```

- [ ] **Step 2: Apply the migration**

Run (from `backend/`, dev DB on :5433):
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: migration `000019` applied.

- [ ] **Step 3: Extend the CreateEmployee/UpdateEmployee queries**

In `backend/db/queries/employees.sql`, replace the `CreateEmployee` and `UpdateEmployee` blocks with:
```sql
-- name: CreateEmployee :one
INSERT INTO masterdata.employees (
  code, name, email, phone, avatar_key, department_id, position_id, office_id, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateEmployee :one
UPDATE masterdata.employees
SET code = sqlc.arg(code),
    name = sqlc.arg(name),
    email = sqlc.narg(email),
    phone = sqlc.narg(phone),
    avatar_key = sqlc.narg(avatar_key),
    department_id = sqlc.narg(department_id),
    position_id = sqlc.narg(position_id),
    office_id = sqlc.arg(office_id),
    status = sqlc.arg(status)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
RETURNING *;
```

- [ ] **Step 4: Regenerate sqlc**

Run (from `backend/`): `sqlc generate`
Expected: `MasterdataEmployee`, `CreateEmployeeParams`, `UpdateEmployeeParams` now carry `Phone *string`.

- [ ] **Step 5: Thread phone through DTO + service**

In `backend/internal/masterdata/employee/dto.go`:
- Add to `Request` (after `Email`): `Phone *string \`json:"phone"\``
- Add to `Response` (after `Email`): `Phone *string \`json:"phone"\``
- In `toInput()`, add `Phone: r.Phone,` to the returned `CreateInput{...}`.
- In `toResponse()`, add `Phone: e.Phone,`.

In `backend/internal/masterdata/employee/service.go`:
- Add to `CreateInput` (after `Email *string`): `Phone *string`
- In `Create()`, add `Phone: in.Phone,` to `sqlc.CreateEmployeeParams{...}`.
- In `Update()`, add `Phone: in.Phone,` to `sqlc.UpdateEmployeeParams{...}`.

- [ ] **Step 6: Write the failing integration test**

In `backend/internal/masterdata/employee/employee_integration_test.go`, add a `strptr` helper near the top (if not present) and this test (sibling to `TestEmployeeDataScope`):
```go
func strptr(s string) *string { return &s }

func TestEmployeePhone(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := employee.NewService(q)
	ctx := context.Background()

	testsupport.Reset(t, pool)
	tree := testsupport.SeedOfficeTree(t, pool)

	created, err := svc.Create(ctx, true, nil, employee.CreateInput{
		Code: "EP-1", Name: "Phone Emp", OfficeID: tree.Cabang,
		Status: sqlc.SharedUserStatus("active"), Phone: strptr("0812-1111"),
	})
	require.NoError(t, err)
	require.NotNil(t, created.Phone)
	assert.Equal(t, "0812-1111", *created.Phone)

	_, after, err := svc.Update(ctx, created.ID, true, nil, employee.UpdateInput{CreateInput: employee.CreateInput{
		Code: "EP-1", Name: "Phone Emp", OfficeID: tree.Cabang,
		Status: sqlc.SharedUserStatus("active"), Phone: strptr("0813-2222"),
	}})
	require.NoError(t, err)
	require.NotNil(t, after.Phone)
	assert.Equal(t, "0813-2222", *after.Phone)

	created2, err := svc.Create(ctx, true, nil, employee.CreateInput{
		Code: "EP-2", Name: "No Phone", OfficeID: tree.Cabang, Status: sqlc.SharedUserStatus("active"),
	})
	require.NoError(t, err)
	assert.Nil(t, created2.Phone)
}
```
(If `strptr` or the `require`/`assert` imports already exist in the file, reuse them — don't duplicate.)

- [ ] **Step 7: Run build + tests**

Run (from `backend/`):
```bash
go build ./... && go vet ./...
go test -tags=integration ./internal/masterdata/employee/ -run 'TestEmployeePhone|TestEmployeeDataScope' -v
go test ./...
```
Expected: build clean; phone + existing scope tests PASS; full suite green.

- [ ] **Step 8: Update OpenAPI + Spectral**

In `backend/api/openapi.yaml`, add `phone` (type string, nullable) to the employee request schema AND the employee response schema (locate the existing employee schemas used by the `/employees` paths).

Run (from repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add backend/db/migrations/000019_employee_phone.up.sql backend/db/migrations/000019_employee_phone.down.sql backend/db/queries/employees.sql backend/db/sqlc/ backend/internal/masterdata/employee/ backend/api/openapi.yaml
git commit -m "feat(masterdata): add employee phone column + DTO/query"
```

---

### Task 2: Frontend — `Employee` type + `useEmployees` HTTP rewrite

**Files:**
- Modify: `frontend/app/types/index.ts` (rewrite `Employee`)
- Modify (full rewrite): `frontend/app/composables/api/useEmployees.ts`
- Delete: `frontend/test/unit/employees-mock.spec.ts`
- Test: `frontend/test/unit/use-employees.spec.ts` (NEW)

**Interfaces:**
- Produces: `Employee` (English shape below); `EmployeeStatus = 'active'|'inactive'|'suspended'`; `useEmployees()` → `{ list, get, create, update, remove }`; `EmployeeInput`. Task 3 (page) consumes these.

- [ ] **Step 1: Rewrite the `Employee` type**

In `frontend/app/types/index.ts`, replace the `Employee` interface (and any old `Employee['status']` union it relied on) with:
```ts
export type EmployeeStatus = 'active' | 'inactive' | 'suspended'

export interface Employee {
  id: string
  code: string
  name: string
  email: string | null
  phone: string | null
  department_id: string | null
  position_id: string | null
  office_id: string
  status: EmployeeStatus
  avatar_key?: string | null
  created_at: string | null
  updated_at: string | null
}
```
(Grep confirmed `Employee` is used only by `employees.vue` + `useEmployees` — no other screen breaks. If `grep -rn "import type.*Employee\b" frontend/app` reveals a new consumer, stop and report.)

- [ ] **Step 2: Write the failing unit test**

Create `frontend/test/unit/use-employees.spec.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useEmployees } from '~/composables/api/useEmployees'

beforeEach(() => request.mockReset())

describe('useEmployees', () => {
  it('list builds the query (omits empty search) and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'e1' }], total: 1, limit: 20, offset: 0 })
    const res = await useEmployees().list({ limit: 20, offset: 0 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/employees?')
    expect(path).toContain('limit=20')
    expect(path).not.toContain('search=')
    expect(res.total).toBe(1)
  })

  it('create POSTs /employees with UUID FKs + phone, omitting empty optionals', async () => {
    request.mockResolvedValueOnce({ id: 'e1' })
    await useEmployees().create({ code: '199001', name: 'Andi', office_id: 'o1', status: 'active', department_id: 'd1', position_id: 'p1', email: 'a@x.id', phone: '0812' })
    expect(request).toHaveBeenCalledWith('/employees', { method: 'POST', body: { code: '199001', name: 'Andi', office_id: 'o1', status: 'active', email: 'a@x.id', phone: '0812', department_id: 'd1', position_id: 'p1' } })
  })

  it('create omits empty email/phone/department_id/position_id', async () => {
    request.mockResolvedValueOnce({ id: 'e2' })
    await useEmployees().create({ code: 'X', name: 'B', office_id: 'o1', status: 'active' })
    expect(request).toHaveBeenCalledWith('/employees', { method: 'POST', body: { code: 'X', name: 'B', office_id: 'o1', status: 'active' } })
  })

  it('update PUTs /employees/:id', async () => {
    request.mockResolvedValueOnce({ id: 'e1' })
    await useEmployees().update('e1', { code: 'X', name: 'B', office_id: 'o1', status: 'inactive' })
    expect(request).toHaveBeenCalledWith('/employees/e1', { method: 'PUT', body: { code: 'X', name: 'B', office_id: 'o1', status: 'inactive' } })
  })

  it('get GETs /employees/:id; remove DELETEs', async () => {
    request.mockResolvedValueOnce({ id: 'e1' })
    await useEmployees().get('e1')
    expect(request).toHaveBeenCalledWith('/employees/e1')
    request.mockResolvedValueOnce(undefined)
    await useEmployees().remove('e1')
    expect(request).toHaveBeenCalledWith('/employees/e1', { method: 'DELETE' })
  })
})
```

- [ ] **Step 3: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-employees`
Expected: FAIL (current `useEmployees` is mock-backed; English shape undefined).

- [ ] **Step 4: Rewrite `useEmployees.ts`**

Replace `frontend/app/composables/api/useEmployees.ts` entirely with:
```ts
import type { Employee, EmployeeStatus, ListQuery, Paginated } from '~/types'

export interface EmployeeInput {
  code: string
  name: string
  email?: string
  phone?: string
  department_id?: string
  position_id?: string
  office_id: string
  status: EmployeeStatus
}

/** Employees, wired to /api/v1/employees (server-enforced `employees` data-scope). */
export function useEmployees() {
  const { request } = useApiClient()

  async function list(query: ListQuery = {}): Promise<Paginated<Employee>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 20))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    return request<Paginated<Employee>>(`/employees?${q.toString()}`)
  }

  async function get(id: string): Promise<Employee> {
    return request<Employee>(`/employees/${id}`)
  }

  function toBody(input: EmployeeInput): Record<string, unknown> {
    const body: Record<string, unknown> = { code: input.code, name: input.name, office_id: input.office_id, status: input.status }
    if (input.email) body.email = input.email
    if (input.phone) body.phone = input.phone
    if (input.department_id) body.department_id = input.department_id
    if (input.position_id) body.position_id = input.position_id
    return body
  }

  async function create(input: EmployeeInput): Promise<Employee> {
    return request<Employee>('/employees', { method: 'POST', body: toBody(input) })
  }

  async function update(id: string, input: EmployeeInput): Promise<Employee> {
    return request<Employee>(`/employees/${id}`, { method: 'PUT', body: toBody(input) })
  }

  async function remove(id: string): Promise<void> {
    await request(`/employees/${id}`, { method: 'DELETE' })
  }

  return { list, get, create, update, remove }
}
```

- [ ] **Step 5: Delete the mock unit test + run**

Run (from `frontend/`):
```bash
git rm frontend/test/unit/employees-mock.spec.ts
pnpm test -- use-employees
pnpm lint
```
Expected: `use-employees` PASS; lint clean. NOTE: `pnpm typecheck` will now FAIL in `pages/master/employees.vue` (old Indonesian fields against the new `Employee` type) — EXPECTED, fixed in Task 3. The whole `pnpm test` suite is also red on `test/nuxt/master-employees.spec.ts` (page now calls real HTTP) — EXPECTED, fixed in Task 4. Do NOT touch the page or that component test here.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/types/index.ts frontend/app/composables/api/useEmployees.ts frontend/test/unit/use-employees.spec.ts
git commit -m "feat(employees): English Employee type + useEmployees wired to /api/v1/employees"
```

---

### Task 3: Frontend — rewrite `employees.vue` (FK UUID pickers, name resolution, phone)

**Files:**
- Modify (script + template): `frontend/app/pages/master/employees.vue`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json` (`status.suspended`, `loadError`)

**Interfaces:**
- Consumes: `useEmployees` + `Employee`/`EmployeeStatus`/`EmployeeInput` (Task 2); `useReference` (already wired); `useApiClient` (inline offices).

- [ ] **Step 1: Add i18n keys**

In `frontend/i18n/locales/id.json` + `en.json`, under `masterdata.employees`:
- `status.suspended`: id `"Ditangguhkan"`, en `"Suspended"`.
- `loadError`: id `"Gagal memuat pegawai."`, en `"Failed to load employees."`.
Reuse `common.retry` (already exists). Keep valid JSON, no trailing commas.

- [ ] **Step 2: Rewrite the page `<script setup>`**

Replace the `<script setup lang="ts">` block of `frontend/app/pages/master/employees.vue` with:
```ts
import type { Employee, EmployeeStatus, RowAction, TableSorting } from '~/types'
import type { EmployeeInput } from '~/composables/api/useEmployees'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
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
function officeName(id: string | null): string { return id ? (officeMap.value.get(id) ?? id) : '—' }
function deptName(id: string | null): string { return id ? (deptMap.value.get(id) ?? id) : '—' }
function positionName(id: string | null): string { return id ? (positionMap.value.get(id) ?? id) : '—' }

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<EmployeeInput>({
  code: '', name: '', email: '', phone: '', department_id: '', position_id: '', office_id: '', status: 'active'
})

const columns = [
  { accessorKey: 'code', header: t('masterdata.employees.columns.nip'), sortable: true },
  { accessorKey: 'name', header: t('masterdata.employees.columns.nama'), sortable: true },
  { accessorKey: 'departemen', header: t('masterdata.employees.columns.departemen'), sortable: true },
  { accessorKey: 'jabatan', header: t('masterdata.employees.columns.jabatan'), sortable: true },
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
  } catch { /* useApiClient surfaces the error toast */ }
  finally {
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
watch(sorting, () => { offset.value = 0 })

onMounted(() => {
  refresh()
  loadFkData()
})
```

- [ ] **Step 3: Update the page template**

Apply these edits to `employees.vue`:
- **Filter selects**: rename `v-model` bindings `filterKantor→filterOffice`, `filterDept` (keep), `filterJabatan→filterPosition`; their `:items` use `officeOptions`/`deptOptions`/`positionOptions` (now UUID-valued). Keep the status select as `active`/`inactive`. (The `allKantor`/`allDept`/`allJabatan`/`allStatus` i18n labels stay.)
- **ResourceTable cell slots** — rebind to English fields + resolved names:
  - `#code-cell` (was `#nip-cell`): `{{ (row as unknown as Employee).code }}`.
  - `#name-cell` (was `#nama-cell`): `initials((row as unknown as Employee).name)` + `{{ (row as unknown as Employee).name }}`.
  - `#departemen-cell`: `{{ deptName((row as unknown as Employee).department_id) }}`.
  - `#jabatan-cell`: `{{ positionName((row as unknown as Employee).position_id) }}`.
  - `#kantor-cell`: `{{ officeName((row as unknown as Employee).office_id) }}`.
  - `#kontak-cell`: email `{{ (row as unknown as Employee).email ?? '—' }}`, phone `{{ (row as unknown as Employee).phone ?? '—' }}`.
  - `#status-cell`: `:color="(row as unknown as Employee).status === 'active' ? 'success' : 'neutral'"` + `{{ t('masterdata.employees.status.' + (row as unknown as Employee).status) }}` (now also renders `suspended`).
  (The column `accessorKey`s stay `code`/`name`/`departemen`/`jabatan`/`kantor`/`kontak`/`status` as defined in the script — the slot names must match those accessor keys.)
- **Load-error block**: add, before `<ResourceTable>`, a `v-if="loadFailed"` panel with `{{ t('masterdata.employees.loadError') }}` + a retry `UButton` calling `refresh` + `{{ t('common.retry') }}`, and put `v-else` on the `<ResourceTable>` (mirror the categories.vue load-error block).
- **Form (FormSlideover)** — rebind to the new `form` keys + UUID pickers:
  - NIP `v-model="form.code"`.
  - Status toggle: `:model-value="form.status === 'active'"` `@update:model-value="form.status = $event ? 'active' : 'inactive'"` (unchanged logic).
  - Nama `v-model="form.name"`.
  - Departemen `<USelect v-model="form.department_id" :items="deptOptions" …/>`.
  - Jabatan `<USelect v-model="form.position_id" :items="positionOptions" …/>`.
  - Kantor `<USelect v-model="form.office_id" :items="officeOptions" …/>` (keep the scope-note hint).
  - Email `v-model="form.email"`; Telepon `v-model="form.phone"` (keep the field — phone is now wired).

- [ ] **Step 4: Add the `required` i18n key**

The script references `masterdata.employees.required`. Add it to both locales (id: `"required": "NIP, Nama, dan Kantor wajib diisi."`, en: `"required": "NIP, Name, and Office are required."`).

- [ ] **Step 5: Verify lint + typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: both exit 0. NOTE: `pnpm test` is still red on `test/nuxt/master-employees.spec.ts` — fixed in Task 4.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/pages/master/employees.vue frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "feat(employees): page on real API — UUID FK pickers, name resolution, phone, load-error"
```

---

### Task 4: Frontend — rewrite the component test

**Files:**
- Modify (full rewrite): `frontend/test/nuxt/master-employees.spec.ts`

**Interfaces:**
- Consumes: the wired page; mocks `~/composables/useApiClient` (covers `useEmployees`, `useReference`, the inline `/offices`).

- [ ] **Step 1: Study the harness**

Read the CURRENT `frontend/test/nuxt/master-employees.spec.ts` (it mock-stubbed `useReference` in #40 — to be OVERWRITTEN) AND `frontend/test/nuxt/settings-users.spec.ts` (the per-path `useApiClient` stub + `useAuthStore().setSession(token,user,['*'])` + `mountSuspended` + `wrapper.vm` ref-mutation harness). Read `frontend/app/pages/master/employees.vue` so the stub matches its calls.

- [ ] **Step 2: Write the rewritten test**

Overwrite `frontend/test/nuxt/master-employees.spec.ts`. Mock `~/composables/useApiClient` with a `request(path, opts)` handler that routes by path + captures mutation bodies. On mount the page calls: `GET /employees?...` (list), `GET /offices?limit=100`, `GET /departments?...`, `GET /positions?...`. Fixtures: 2-3 employees (with `department_id`/`position_id`/`office_id` referencing the stubbed offices/departments/positions; include one `suspended`); offices `[{id:'o1',name:'Kantor Pusat'},…]`; departments `[{id:'d1',name:'Umum'},…]`; positions `[{id:'p1',name:'Staf'},…]`. Cover (assert REAL rendered text / captured bodies — NO hollow checks):
- Loaded rows render with **resolved FK names** (department/position/office show the NAME, not the UUID); the table does NOT contain the raw `o1`/`d1`/`p1` UUIDs.
- A `suspended` employee renders the "Suspended"/"Ditangguhkan" status label.
- Office filter (`filterOffice='o1'` via `wrapper.vm`) narrows to that office's rows (assert an out-of-office row is ABSENT). Same for the department filter.
- Create: open form, set `form.code`/`form.name`/`form.office_id`/`form.department_id`/`form.position_id` via `wrapper.vm`, submit → captured `POST /employees` body has `{code,name,office_id,status,department_id,position_id,...}` with UUID values (not names).
- Required guard: submit with empty `code` → NO `POST` sent.
- Edit → `PUT /employees/:id`; delete (confirm) → `DELETE /employees/:id`.
- Load-error: `GET /employees` rejects → the page shows the load-error block + retry.

Assert real behavior; use the `wrapper.vm as unknown as {...}` technique. The page `defineExpose`s nothing by default — drive via the setupState proxy (`wrapper.vm.filterOffice`, `.form`, `.openCreate`, `.onSubmit`, etc.), as the other wired component tests do.

- [ ] **Step 3: Run the target test + full suite**

Run (from `frontend/`): `pnpm test -- master-employees` then `pnpm test`
Expected: target PASS; whole suite green. **Confirm `pnpm test` exits 0** (the #40 lesson — `useEmployees`'s only consumer is this page, now stubbed; `useGlobalSearch` uses the mock store, no network).

- [ ] **Step 4: Commit**

```bash
git add frontend/test/nuxt/master-employees.spec.ts
git commit -m "test(employees): component test against stubbed /employees + FK endpoints"
```

---

### Task 5: E2E + mockup + PROGRESS + full gate (mock kept)

**Files:**
- Create: `frontend/e2e/employees.spec.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Confirm the mock stays**

Run (from repo root): `grep -rn "mock/employees" frontend/app frontend/test`. Expected importers: `useGlobalSearch.ts` (still uses `employeeStore` for global search) — so `mock/employees.ts` is NOT deleted this sub-project. Confirm `useEmployees.ts` + `employees-mock.spec.ts` no longer reference it (the latter was deleted in Task 2). Report the importer list; do NOT delete the mock.

- [ ] **Step 2: Write the e2e**

Read `frontend/e2e/helpers.ts` (`login()`) + a robust wired e2e (`frontend/e2e/master-reference.spec.ts`). Create `frontend/e2e/employees.spec.ts` against the real backend (seeded admin = global scope, `masterdata.office.manage`): login → `/master/employees`; assert heading + Add button; create an employee — open the form, fill NIP (`code`) + Name, select an Office via its USelect (trigger-click + `role="option"`, NEVER `selectOption`; the office picker is required), optionally a Department + Position, submit; then **filter via the search box by the unique name/NIP** and assert the new row appears with the resolved office name.

**Apply the e2e robustness lessons (a prior PR failed on these):**
- Unique **name AND code/NIP per run** (the `code` is unique-constrained and rows persist): `const s = \`${Date.now()}\`` → name `E2E Pegawai ${s}`, code (NIP) `E2E${s}`.
- Assert created rows only **after** typing into the search box (the table paginates; a fresh row may be on a later page).
- Wait for the slideover to **close** (`toBeHidden`) before reopening Add.
- For any USelect whose accessible name includes extra text, target a `data-testid` (add a minimal one to the office/dept/position USelect in `employees.vue` if needed — report it). NO `selectOption`, NO `isVisible()` snapshot booleans driving control flow, NO silent `if(...)return`, NO `.first()`/`.last()` on broad `div`/`button` filters, NO `getByText(...,{exact:false})` ambiguous matches.

You likely CANNOT run `pnpm test:e2e` here (needs full stack); ensure it compiles + lints; CI runs it. State that in your report.

- [ ] **Step 3: Mockup fidelity comparison**

Read `docs/design/Master Data Pegawai.dc.html` + the built `frontend/app/pages/master/employees.vue`. Verify the table (7 columns incl. Email/Telepon), filter bar (search + 4 dropdowns + reset), and the slideover form (NIP+status, name, dept+position, office+scope-note, email+phone) match 1:1. Email/Telepon is retained (phone added to backend). No approved deviation expected. Fix any genuine deviation; report.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`: mark Pegawai ✅ wired to `/api/v1/employees` (server-enforced `employees` data-scope; FK pickers office/department/position with table name-resolution; backend `phone` column added). Add a TODO note: `mock/employees.ts` retained (still used by `useGlobalSearch` — delete when global search is wired). Refresh "▶ Next session — start here" → next master-data sub-project = **Kantor + Lantai + Ruangan** (last of the batch; will wire `useOffices` that this screen inline-fetches). Don't fabricate status for unrelated screens.

- [ ] **Step 5: Full gate (backend + frontend)**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./...`, then `go test -tags=integration ./...`, then Spectral.
Run (from `frontend/`): `pnpm lint && pnpm typecheck && pnpm test && pnpm build`.
Expected: all green. **Confirm `pnpm test` exits 0** — trace any unhandled rejection to its file (note: `assets-catalog`/`assets-label` have a pre-existing parallel-load timeout flake unrelated to this branch; if those are the only non-pass and they pass in isolation, that's the known flake, not a regression). (Integration needs dev Postgres. E2E runs in CI.)

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/employees.spec.ts docs/PROGRESS.md
git commit -m "test(employees): e2e + progress; wire Pegawai end-to-end"
```
(Include `frontend/app/pages/master/employees.vue` if Step 2 added a `data-testid`.)

---

## Self-Review

**Spec coverage:**
- §2 backend `phone` (migration 000019 + DTO/service/query + openapi + integration test) → Task 1. ✓
- §3.1 `Employee`/`EmployeeInput` English rewrite → Task 2. ✓
- §3.2 `useEmployees` HTTP rewrite → Task 2. ✓
- §3.3 page rewrite (FK UUID pickers, 3 name-resolution maps, inline `/offices`, phone, status toggle + suspended render, filters by UUID, load-error) → Task 3. ✓
- §3.4 i18n (`status.suspended`, `loadError`, `required`) → Task 3. ✓
- §3.5 mock kept (useGlobalSearch) → Task 5 Step 1. ✓
- §4 tests (backend phone+scope, unit, component, e2e) → Tasks 1/2/4/5. ✓
- §5 done (mock kept, mockup, PROGRESS, full-gate exit-0) → Task 5. ✓
- §6 risks (Employee cross-screen → none, verified; useOffices inline; #40 consumer-stub; picker-empty) → handled in Tasks 2/3/4 + constraints.

**Placeholder scan:** Tasks 4 & 5 give explicit assertion lists + "read X first" pointers (settings-users harness, master-reference e2e, helpers, mockup) and the e2e robustness rules verbatim. The OpenAPI step names the exact addition. No "TODO"/"add validation"/"similar to".

**Type consistency:** `Employee{id,code,name,email,phone,department_id,position_id,office_id,status,avatar_key?,created_at,updated_at}`, `EmployeeStatus='active'|'inactive'|'suspended'`, `EmployeeInput{code,name,email?,phone?,department_id?,position_id?,office_id,status}`, `useEmployees()→{list,get,create,update,remove}` consistent across Tasks 2/3/4. Page helpers `officeName/deptName/positionName` + `officeOptions/deptOptions/positionOptions` + `loadFkData` defined in Task 3's script and used in its template. Backend `employee.CreateInput.Phone`/`Request.Phone`/`Response.Phone` + sqlc `*string` consistent in Task 1. The page filter refs (`filterOffice`/`filterDept`/`filterPosition`/`filterStatus`) match the template `v-model`s. Column accessor keys (`code`/`name`/`departemen`/`jabatan`/`kantor`/`kontak`/`status`) match the cell slot names.
