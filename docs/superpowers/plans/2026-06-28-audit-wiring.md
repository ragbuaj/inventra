# Wire Audit Trail screen to `/api/v1/audit` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mock-backed `useAudit` composable + Audit Trail screen with the real `GET /api/v1/audit` backend, moving filtering + pagination from client-side to server-side.

**Architecture:** `useAudit.list(params)` calls `useApiClient` against `/api/v1/audit` with `search/entity_type/action/from/to/limit/offset` and maps the response (`actor:{}`, `changes:{}`, `entity_type`) to display rows. The page becomes a reactive server-driven list: every filter change / page turn refetches. Entity-type filter options come from a frontend catalog of the real recorded entity types; actor + role + summary + office are dropped (no backend data / gated lookups).

**Tech Stack:** Nuxt 4 (SPA), Nuxt UI (`U*`), `@nuxtjs/i18n` (id default + en), Vitest + `@nuxt/test-utils`, Playwright e2e.

## Global Constraints

- Wire ONLY the Audit Trail screen: `pages/settings/audit.vue`, `composables/api/useAudit.ts`, new `constants/auditCatalog.ts`. Do NOT touch other screens.
- Backend `GET /api/v1/audit` (gated `audit.view`, office-scoped). Response `auditToMap`: `{id, entity_type, entity_id, action, ip, changes:{field:{before,after}}, actor:{id,name,email}, office_id, created_at}`; envelope `{data,total,limit,offset}`.
- Query params: `search` (matches `entity_type`/`entity_id` ILIKE — NOT actor), `entity_type`, `action`, `from` (RFC3339), `to` (RFC3339), `limit` (default 20, clamp 1–100), `offset`.
- Server-side filter + pagination: page size 20; filter change resets to page 1; both refetch.
- Drop columns/filters with no backend data or gated lookups: role, summary, office name, actor dropdown filter. Keep: time, actor name, action, entity (i18n label), IP, and the expandable `changes` diff.
- Permission gate fixed to `audit.view` (was `user.manage`).
- All API calls via `useApiClient().request<T>`; never hardcode the backend URL.
- i18n mandatory: entity labels (`settings.audit.entity.<key>`) resolve via `te()/t()` with fallback to the raw key; reuse existing `action.*` labels.
- Match `docs/design/Audit Trail.dc.html` for layout/filter-bar/diff-viewer/pagination; the dropped columns/filters are an approved deviation.
- PROGRESS.md records the deviation + the actor/office follow-up TODO.
- Gates: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green. Run from `frontend/`.

---

### Task 1: Audit entity catalog + i18n

**Files:**
- Create: `frontend/app/constants/auditCatalog.ts`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`
- Test: `frontend/test/unit/audit-catalog.spec.ts`

**Interfaces:**
- Produces: `const AUDIT_ENTITY_TYPES: readonly string[]`.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/audit-catalog.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { AUDIT_ENTITY_TYPES } from '~/constants/auditCatalog'

describe('AUDIT_ENTITY_TYPES', () => {
  it('lists the real recorded entity types', () => {
    expect(AUDIT_ENTITY_TYPES).toContain('assets')
    expect(AUDIT_ENTITY_TYPES).toContain('users')
    expect(AUDIT_ENTITY_TYPES).toContain('roles')
    expect(AUDIT_ENTITY_TYPES).toContain('field_permissions')
  })
  it('has no duplicates', () => {
    expect(new Set(AUDIT_ENTITY_TYPES).size).toBe(AUDIT_ENTITY_TYPES.length)
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- audit-catalog`
Expected: FAIL — cannot resolve `~/constants/auditCatalog`.

- [ ] **Step 3: Create the constants file**

Create `frontend/app/constants/auditCatalog.ts`:

```ts
// The real entity_type values recorded by the backend's audit.Record(...) calls.
// Used to populate the Audit Trail entity-type filter dropdown.
export const AUDIT_ENTITY_TYPES = [
  'assets', 'users', 'roles', 'role_permissions', 'data_scope_policies', 'field_permissions',
  'offices', 'employees', 'categories', 'floors', 'rooms', 'requests',
  'asset_attachments', 'asset_documents'
] as const
```

- [ ] **Step 4: Add i18n keys**

In `frontend/i18n/locales/id.json` and `en.json`, under the existing `settings.audit` object, add `entity` (14 labels), `loadError`, `retry`, and reword `searchPlaceholder` to mention entity/ID. Read the `settings.audit` section first; insert as valid JSON.

**id.json** `settings.audit`:
```json
"entity": {
  "assets": "Aset", "users": "User", "roles": "Peran", "role_permissions": "Hak Peran",
  "data_scope_policies": "Kebijakan Scope", "field_permissions": "Field Permission",
  "offices": "Kantor", "employees": "Pegawai", "categories": "Kategori",
  "floors": "Lantai", "rooms": "Ruangan", "requests": "Pengajuan",
  "asset_attachments": "Lampiran Aset", "asset_documents": "Dokumen Aset"
},
"loadError": "Gagal memuat audit trail.",
"retry": "Coba lagi"
```
And update the existing `settings.audit.searchPlaceholder` value to: `"Cari entity atau ID…"`.

**en.json** `settings.audit`:
```json
"entity": {
  "assets": "Assets", "users": "User", "roles": "Roles", "role_permissions": "Role permissions",
  "data_scope_policies": "Data scope policies", "field_permissions": "Field permissions",
  "offices": "Offices", "employees": "Employees", "categories": "Categories",
  "floors": "Floors", "rooms": "Rooms", "requests": "Requests",
  "asset_attachments": "Asset attachments", "asset_documents": "Asset documents"
},
"loadError": "Failed to load the audit trail.",
"retry": "Retry"
```
And update the existing `settings.audit.searchPlaceholder` (en) value to: `"Search by entity or ID…"`.

- [ ] **Step 5: Run test + lint**

Run (from `frontend/`): `pnpm test -- audit-catalog && pnpm lint`
Expected: PASS, lint clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/constants/auditCatalog.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/unit/audit-catalog.spec.ts
git commit -m "feat(audit): entity-type catalog + i18n labels"
```

---

### Task 2: Rewrite `useAudit` to the real API

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useAudit.ts`
- Delete: `frontend/test/unit/audit-mock.spec.ts`
- Test: `frontend/test/unit/use-audit.spec.ts`

**Interfaces:**
- Consumes: `useApiClient().request`.
- Produces: types `AuditAction`, `AuditDiffView{field,before,after,hasBefore,hasAfter,hasArrow}`, `AuditRow{id,created_at,date,time,actor,actor_email,initials,action,entity_type,entity_id,ip,diff}`, `AuditListParams{search?,entity_type?,action?,from?,to?,limit,offset}`; `useAudit()` → `{ list }`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/test/unit/use-audit.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

import { useAudit } from '~/composables/api/useAudit'

beforeEach(() => request.mockReset())

const apiRow = {
  id: 'a1', entity_type: 'assets', entity_id: 'e9', action: 'update', ip: '10.0.0.1',
  changes: { name: { before: 'Old', after: 'New' }, status: { after: 'available' } },
  actor: { id: 'u1', name: 'Bambang Sukasno', email: 'b@x.id' },
  office_id: 'o1', created_at: '2026-06-24T08:30:00Z'
}

describe('useAudit', () => {
  it('list builds the query from non-empty params and returns {rows,total}', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const res = await useAudit().list({ entity_type: 'assets', action: 'update', search: '', limit: 20, offset: 40 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/audit?')
    expect(path).toContain('entity_type=assets')
    expect(path).toContain('action=update')
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=40')
    expect(path).not.toContain('search=')   // empty search omitted
    expect(res.total).toBe(1)
  })

  it('maps the API row to a display AuditRow', async () => {
    request.mockResolvedValueOnce({ data: [apiRow], total: 1, limit: 20, offset: 0 })
    const { rows } = await useAudit().list({ limit: 20, offset: 0 })
    const r = rows[0]
    expect(r).toMatchObject({
      id: 'a1', actor: 'Bambang Sukasno', actor_email: 'b@x.id', initials: 'BS',
      action: 'update', entity_type: 'assets', entity_id: 'e9', ip: '10.0.0.1',
      date: '2026-06-24', time: '08:30'
    })
    // changes → diff view
    expect(r.diff).toContainEqual({ field: 'name', before: 'Old', after: 'New', hasBefore: true, hasAfter: true, hasArrow: true })
    expect(r.diff).toContainEqual({ field: 'status', before: '', after: 'available', hasBefore: false, hasAfter: true, hasArrow: false })
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-audit`
Expected: FAIL — new `useAudit` shape undefined.

- [ ] **Step 3: Rewrite `useAudit.ts`**

Replace `frontend/app/composables/api/useAudit.ts` entirely with:

```ts
export type AuditAction = 'create' | 'update' | 'delete'

export interface AuditDiffView {
  field: string
  before: string
  after: string
  hasBefore: boolean
  hasAfter: boolean
  hasArrow: boolean
}

export interface AuditRow {
  id: string
  created_at: string
  date: string
  time: string
  actor: string
  actor_email: string
  initials: string
  action: AuditAction
  entity_type: string
  entity_id: string
  ip: string
  diff: AuditDiffView[]
}

export interface AuditListParams {
  search?: string
  entity_type?: string
  action?: AuditAction
  from?: string
  to?: string
  limit: number
  offset: number
}

interface AuditChange { before?: unknown; after?: unknown }
interface AuditDTO {
  id: string
  entity_type: string
  entity_id: string
  action: AuditAction
  ip: string
  changes: Record<string, AuditChange> | null
  actor: { id: string; name: string; email: string } | null
  office_id: string | null
  created_at: string
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
}

function toDiff(changes: Record<string, AuditChange> | null): AuditDiffView[] {
  if (!changes) return []
  return Object.entries(changes).map(([field, c]) => {
    const hasBefore = c.before != null
    const hasAfter = c.after != null
    return {
      field,
      before: hasBefore ? String(c.before) : '',
      after: hasAfter ? String(c.after) : '',
      hasBefore,
      hasAfter,
      hasArrow: hasBefore && hasAfter
    }
  })
}

function toRow(d: AuditDTO): AuditRow {
  const name = d.actor?.name ?? ''
  return {
    id: d.id,
    created_at: d.created_at,
    date: (d.created_at ?? '').slice(0, 10),
    time: (d.created_at ?? '').slice(11, 16),
    actor: name,
    actor_email: d.actor?.email ?? '',
    initials: initials(name),
    action: d.action,
    entity_type: d.entity_type,
    entity_id: d.entity_id,
    ip: d.ip,
    diff: toDiff(d.changes)
  }
}

/**
 * Audit log reader (read-only), wired to GET /api/v1/audit. Filtering and
 * pagination are server-side; the actor name comes from the response (no lookup).
 */
export function useAudit() {
  const { request } = useApiClient()

  async function list(params: AuditListParams): Promise<{ rows: AuditRow[]; total: number }> {
    const q = new URLSearchParams()
    q.set('limit', String(params.limit))
    q.set('offset', String(params.offset))
    if (params.search) q.set('search', params.search)
    if (params.entity_type) q.set('entity_type', params.entity_type)
    if (params.action) q.set('action', params.action)
    if (params.from) q.set('from', params.from)
    if (params.to) q.set('to', params.to)
    const res = await request<{ data: AuditDTO[]; total: number; limit: number; offset: number }>(`/audit?${q.toString()}`)
    return { rows: res.data.map(toRow), total: res.total }
  }

  return { list }
}
```

- [ ] **Step 4: Run tests + lint**

Run (from `frontend/`): `pnpm test -- use-audit && pnpm lint`
Expected: PASS, lint clean. NOTE: `pnpm typecheck` will fail ONLY in `pages/settings/audit.vue` (old shape / `~/mock/audit`) — EXPECTED, fixed in Task 3. Do NOT edit the page here.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useAudit.ts frontend/test/unit/use-audit.spec.ts
git rm frontend/test/unit/audit-mock.spec.ts
git commit -m "feat(audit): wire useAudit to /api/v1/audit (server-side list)"
```

---

### Task 3: Rewrite the page (server-side, dropped columns/filters)

**Files:**
- Modify (script + template): `frontend/app/pages/settings/audit.vue`

**Interfaces:**
- Consumes: `useAudit().list` (Task 2), `AuditRow`/`AuditAction` (Task 2), `AUDIT_ENTITY_TYPES` (Task 1).

- [ ] **Step 1: Rewrite the page `<script setup>`**

Replace the `<script setup>` block of `frontend/app/pages/settings/audit.vue` with:

```ts
import type { AuditRow, AuditAction } from '~/composables/api/useAudit'
import { useAudit } from '~/composables/api/useAudit'
import { AUDIT_ENTITY_TYPES } from '~/constants/auditCatalog'

definePageMeta({ middleware: 'can', permission: 'audit.view' })

const PAGE_SIZE = 20
const ALL = '__all__'

const { t, te } = useI18n()
const { list } = useAudit()

const rows = ref<AuditRow[]>([])
const total = ref(0)
const loading = ref(true)
const loadFailed = ref(false)
const search = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const fAction = ref(ALL)
const fEntity = ref(ALL)
const page = ref(1)
const openId = ref<string | null>(null)

// Action display metadata (tone + icon), inlined (was imported from the mock).
const ACTION_META: Record<AuditAction, { tone: 'success' | 'warning' | 'error'; icon: string }> = {
  create: { tone: 'success', icon: 'i-lucide-plus' },
  update: { tone: 'warning', icon: 'i-lucide-pencil' },
  delete: { tone: 'error', icon: 'i-lucide-trash-2' }
}

function entityLabel(key: string): string {
  const k = `settings.audit.entity.${key}`
  return te(k) ? t(k) : key
}

const actionOptions = computed(() => [
  { value: ALL, label: t('settings.audit.filter.allActions') },
  { value: 'create', label: t('settings.audit.action.create') },
  { value: 'update', label: t('settings.audit.action.update') },
  { value: 'delete', label: t('settings.audit.action.delete') }
])
const entityOptions = computed(() => [
  { value: ALL, label: t('settings.audit.filter.allEntities') },
  ...AUDIT_ENTITY_TYPES.map(e => ({ value: e, label: entityLabel(e) }))
])

const anyFilter = computed(() =>
  !!(search.value.trim() || dateFrom.value || dateTo.value || fAction.value !== ALL || fEntity.value !== ALL)
)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const pageInfo = computed(() => {
  const from = total.value === 0 ? 0 : (page.value - 1) * PAGE_SIZE + 1
  const to = Math.min(page.value * PAGE_SIZE, total.value)
  return t('settings.audit.showing', { from, to, total: total.value })
})

// A 'YYYY-MM-DD' date input → an RFC3339 day bound for the backend from/to filter.
function toRfc(d: string, endOfDay: boolean): string | undefined {
  if (!d) return undefined
  return new Date(`${d}T${endOfDay ? '23:59:59' : '00:00:00'}Z`).toISOString()
}

function actionMeta(action: AuditAction) {
  return ACTION_META[action]
}
function toggle(id: string) {
  openId.value = openId.value === id ? null : id
}
function resetFilters() {
  search.value = ''
  dateFrom.value = ''
  dateTo.value = ''
  fAction.value = ALL
  fEntity.value = ALL
  page.value = 1
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const res = await list({
      search: search.value.trim() || undefined,
      entity_type: fEntity.value !== ALL ? fEntity.value : undefined,
      action: fAction.value !== ALL ? (fAction.value as AuditAction) : undefined,
      from: toRfc(dateFrom.value, false),
      to: toRfc(dateTo.value, true),
      limit: PAGE_SIZE,
      offset: (page.value - 1) * PAGE_SIZE
    })
    rows.value = res.rows
    total.value = res.total
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

watch([search, dateFrom, dateTo, fAction, fEntity], () => {
  page.value = 1
  load()
})
watch(page, () => load())
onMounted(() => load())
```

Notes: gate is now `audit.view`; the actor filter (`fActor`/`actors`/`actorOptions`) is gone; filtering + pagination are server-side (`load()` refetches; filter change resets `page`); action metadata is inlined (no mock import); entity labels via i18n fallback. `openId`/`id` are now strings.

- [ ] **Step 2: Update the page template**

In the `<template>` of `audit.vue`:
- **Filter bar**: DELETE the actor `<USelect v-model="fActor" :items="actorOptions" .../>` block entirely. Keep search, the date-from/to inputs, the action `<USelect v-model="fAction">`, the entity `<USelect v-model="fEntity">`, and the reset button.
- **Loading/error/content**: keep `v-if="loading"` spinner. After it, add a load-error block, and change the populated-table condition to also require not-failed. Insert before the `<div v-else-if="pageRows.length > 0">` table block:

```vue
    <div
      v-else-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-20 text-muted"
    >
      <UIcon name="i-lucide-circle-alert" class="size-6" />
      <span class="text-sm">{{ t('settings.audit.loadError') }}</span>
      <UButton color="neutral" variant="subtle" @click="load">
        {{ t('settings.audit.retry') }}
      </UButton>
    </div>
```
and change the table block guard `v-else-if="pageRows.length > 0"` → `v-else-if="rows.length > 0"`, and the empty-state `v-else` stays.

- **Table head**: remove the `summary` and `office` `<th>` columns; add an `IP` `<th>` after entity:
  - Keep: chevron (empty th), time, actor, action, entity. Remove the `columns.summary` th and the `columns.office` th. Add `<th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase text-muted">IP</th>` (no i18n key needed; "IP" is universal — or reuse a literal).
- **Table body** (`v-for="r in pageRows"` → `v-for="r in rows"`):
  - The actor cell: remove the role sub-line `<div ...>{{ r.role }}</div>` (no role data); keep initials + `{{ r.actor }}`.
  - The entity cell: `{{ r.entity }}` → `{{ entityLabel(r.entity_type) }}`.
  - Remove the summary `<td>` (`{{ r.summary }}`) and the office `<td>` (`{{ r.office }}` / `{{ r.ip }}`). Add an IP `<td>`: `<td class="px-3.5 py-3 font-mono text-[12px] text-dimmed whitespace-nowrap">{{ r.ip }}</td>`.
  - The expandable diff row: change `colspan="7"` → `colspan="6"` (chevron + time + actor + action + entity + ip = 6 columns); the diff header `{{ r.ref }}` → `{{ r.entity_id }}`; the diff loop `v-for="(df, i) in r.diff"` stays (AuditDiffView shape unchanged).
- **Pagination**: change `pageRows`→`rows` is already done; the prev/next + numbered buttons use `totalPages`/`page` (server-side) unchanged. Remove the now-unused `pageRows`/`filtered`/`total` computed-over-rows references (the script rewrite already replaced them with server-side `total` ref + `rows`).
- The Export button keeps calling `comingSoon` — KEEP the `comingSoon`/toast (re-add `const toast = useToast()` to the script if you reference it; the rewritten script above omitted it, so either restore the toast + `comingSoon` for the Export button, OR make the Export button a disabled stub). Simplest: keep Export as a disabled button: change it to `<UButton icon="i-lucide-download" color="neutral" variant="outline" disabled :label="t('settings.audit.export')" />` and drop `comingSoon`.

- [ ] **Step 3: Verify build/lint/typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: exit 0. NOTE: `pnpm test` will still FAIL on `test/nuxt/settings-audit.spec.ts` (old mock stub) — fixed in Task 4. Run `pnpm test -- use-audit audit-catalog` to confirm Task 1/2 units pass.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/pages/settings/audit.vue
git commit -m "feat(audit): page on real API (server-side filters, audit.view gate, dropped columns)"
```

---

### Task 4: Nuxt component test for the wired page

**Files:**
- Modify (rewrite): `frontend/test/nuxt/settings-audit.spec.ts`

**Interfaces:**
- Consumes: the wired page; mock the HTTP layer the way the Data Scope / Field Permission component tests do.

- [ ] **Step 1: Study the patterns**

Read the CURRENT `frontend/test/nuxt/settings-audit.spec.ts` AND `frontend/test/nuxt/settings-data-scope.spec.ts` (it uses `vi.mock('~/composables/useApiClient', ...)` + a per-test `setHandler` capturing the request path, plus `useAuthStore().setSession(token, user, ['*'])` + `mountSuspended`). Mirror that for `GET /api/v1/audit?...`. The page now requires permission `audit.view`; granting `['*']` satisfies it.

- [ ] **Step 2: Write the rewritten test**

Rewrite `frontend/test/nuxt/settings-audit.spec.ts` to stub `/audit` and assert real behavior. Fixture: a `data` array of 2–3 `auditToMap` rows (e.g. one `update` on `assets` with `changes:{purchase_cost:{before:'1000',after:'1200'}}`, one `create` on `users`), plus `total`. The handler should parse the request query and return the fixture (capture the query for assertions). Cover:
- Loaded rows render: actor name, action badge label (e.g. "Ubah"/"Update"), entity i18n label (e.g. "Aset"/"Assets"), IP.
- Changing the entity filter (select an entity) triggers a new `GET /audit` whose query contains `entity_type=<key>` (assert the captured query); changing the action filter sends `action=`.
- A filter change resets to page 1 (offset back to 0); clicking next page sends `offset=20`.
- Expanding a row renders the `changes` diff (a field's before/after, e.g. `purchase_cost` 1000 → 1200).
- Load-error: the `/audit` stub returns a 500 → error block + retry render; empty state when `data:[]`.

Assert real rendered text + captured query params — no hollow checks. Use the harness default locale for i18n strings.

- [ ] **Step 3: Run the test + whole suite**

Run (from `frontend/`): `pnpm test -- settings-audit` then `pnpm test`
Expected: target PASS; whole suite green.

- [ ] **Step 4: Commit**

```bash
git add frontend/test/nuxt/settings-audit.spec.ts
git commit -m "test(audit): component test against stubbed /audit endpoint"
```

---

### Task 5: E2E + delete mock + mockup + PROGRESS + gate

**Files:**
- Modify: `frontend/e2e/settings.spec.ts`
- Delete (if orphaned): `frontend/app/mock/audit.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Update the e2e Audit assertions**

Read `frontend/e2e/settings.spec.ts` + `frontend/e2e/helpers.ts` (`login()`). Replace the existing audit smoke test (`'Audit trail screen loads'`) with a real-backend spec: the seeded admin's recent actions produce audit rows (the seed + the admin login/setup write audit logs), so the table should render at least one row; assert the page heading + that a table or the empty-state renders without error; if rows are present, filter by an action and assert the table updates. Use ROBUST locators (text/role; the action filter is a Nuxt UI `USelect` — open it by clicking the trigger showing the current label, then click an option by `role="option"`/text, as established for the Data Scope fix; do NOT use `selectOption` on it since USelect is not a native `<select>`). Keep RBAC/Data Scope/Field Permission specs untouched. You likely cannot RUN `pnpm test:e2e` here — ensure the spec compiles + lints; it runs in CI. State that in the report.

- [ ] **Step 2: Delete the orphaned mock**

Run `grep -rn "mock/audit" frontend/app frontend/test` (exclude the file itself). After Tasks 2–4 the composable/page/old-tests no longer reference it. If ZERO importers remain, `git rm frontend/app/mock/audit.ts`. If something still imports it, do NOT delete — report what does.

- [ ] **Step 3: Mockup fidelity comparison**

Reference `docs/design/Audit Trail.dc.html`. Structural comparison (read the `.dc.html` + the built `pages/settings/audit.vue`): verify header/filter-bar/table/expandable-diff-viewer/pagination/empty-state match. The dropped columns (role, summary, office name) + the dropped actor filter are an APPROVED deviation (backend provides no role/summary and office/actor names need gated lookups) — not a regression. Fix any other genuine deviation; report the result.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`:
- Under the frontend "Wire screens to real backend APIs" sub-list, mark **Audit Trail ✅ wired to `/api/v1/audit`** (server-side filter + pagination; gate `audit.view`; entity-type filter from a frontend catalog).
- Add a TODO note: the **actor filter + role/summary/office-name columns are dropped** because the backend audit response provides no role/summary and resolving actor/office NAMES needs `user.manage`/masterdata reads an `audit.view`-only viewer may lack. Revisit if a viewer-accessible actor/office lookup (or an enriched audit response) lands.
- Refresh "▶ Next session — start here": Audit Trail done → **User Management** is the next screen to wire (the last of this batch). Don't invent status for other screens.

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
git commit -m "test(audit): e2e against real backend + progress; drop orphaned mock"
```
(If `mock/audit.ts` was deleted, the `git rm` is staged — include it.)

---

## Self-Review

**Spec coverage:**
- §2 composable rewrite (`list(params)` server-side, `AuditRow` mapping, `changes`→diff) → Task 2. ✓
- §3 entity catalog + i18n (entity labels, search reword, loadError/retry) → Task 1. ✓
- §4 page (gate `audit.view`, server-side reactive filters+pagination, dropped actor/role/summary/office, entity i18n, diff viewer, loadError) → Task 3. ✓
- §5 tests (unit/component/e2e) → Tasks 2, 4, 5. ✓
- §6 done (delete mock, mockup, PROGRESS + follow-up TODO, gate) → Task 5. ✓

**Placeholder scan:** Tasks 4 & 5 give explicit assertion lists / steps (read the existing stub pattern first; the USelect e2e interaction follows the established Data Scope fix). Concrete checklists, not "TODO"s.

**Type consistency:** `AuditRow{id:string,...,entity_type,entity_id,ip,diff:AuditDiffView[]}`, `AuditListParams{...,limit,offset}`, `AuditAction`, `useAudit().list(params)→{rows,total}` consistent across Tasks 2/3/4. Page uses `entityLabel(r.entity_type)`, `rows`/`total` (server-side), `openId:string`, `ACTION_META` inlined — all consistent with the Task-2 types. `AUDIT_ENTITY_TYPES` (Task 1) consumed by the page (Task 3). The template's `AuditDiffView` diff block is unchanged (composable produces the same shape from `changes`).
