# Wire Data Scope screen to `/authz` API — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mock-backed `useDataScope` composable + Data Scope settings screen with the real `/api/v1/authz` backend (catalog scope_modules, roles, per-role scope policies), adopting English DTO keys + UUID id identity.

**Architecture:** `useDataScope` is rewritten to call `useApiClient().request` against `/authz/catalog` (module columns), `/authz/roles` + `/authz/roles/:id/scope` (per-role default+overrides). Scope-level presentation (tone + i18n descriptions) and module-column labels move to a frontend constants file + i18n with fallback. The page tracks per-role dirty ids and PUTs only changed roles.

**Tech Stack:** Nuxt 4 (SPA), Nuxt UI (`U*`), `@nuxtjs/i18n` (id default + en), Vitest + `@nuxt/test-utils` (`mountSuspended`), Playwright e2e.

## Global Constraints

- Wire ONLY the Data Scope screen: `pages/settings/data-scope.vue`, `composables/api/useDataScope.ts`, `components/scope/ScopeCell.vue`, new `constants/dataScope.ts`. Do NOT touch other composables/screens.
- English DTO + UUID `id` identity: `ScopeRoleView { id, code, name, sub, def, ov }` (drop `key`/`nama`). `ScopeModuleView { key }` (label resolved in the page via i18n).
- Module columns come from `GET /authz/catalog` `scope_modules` minus `'*'` (real modules: offices/employees/assets/requests/audit). The `'*'` policy is the "Default" column.
- Scope levels: `global | office_subtree | office | own`. Tone map (mockup-faithful): `global=info, office_subtree=primary, office=warning, own=neutral`.
- `def` = the `'*'` policy's `scope_level` (fallback `'own'` when absent). `ov` = non-`'*'` policies. On save, `policies` ALWAYS includes `{module:'*', scope_level:def}` (replace-set; omitting it drops the default).
- Save only changed roles (track `dirtyIds`), PUT in parallel.
- All API calls via `useApiClient().request<T>(path, opts)`; never hardcode the backend URL.
- i18n mandatory: level descriptions + module labels live in `i18n/locales/{id,en}.json`; module labels resolve via `te()/t()` with fallback to the module key.
- Match `docs/design/Data Scope.dc.html` (layout/legend/states); the module column SET intentionally differs (real backend modules) per the approved decision.
- Gates: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green. Run from `frontend/`.

---

### Task 1: Scope-level constants + i18n

**Files:**
- Create: `frontend/app/constants/dataScope.ts`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`
- Test: `frontend/test/unit/data-scope-constants.spec.ts`

**Interfaces:**
- Produces: `SCOPE_LEVEL_KEYS` (readonly tuple `['global','office_subtree','office','own']`), `type ScopeLevel`, `type ScopeTone`, `SCOPE_LEVEL_TONE: Record<ScopeLevel, ScopeTone>`.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/unit/data-scope-constants.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { SCOPE_LEVEL_KEYS, SCOPE_LEVEL_TONE } from '~/constants/dataScope'

describe('data-scope constants', () => {
  it('has the 4 scope levels in order', () => {
    expect(SCOPE_LEVEL_KEYS).toEqual(['global', 'office_subtree', 'office', 'own'])
  })
  it('maps every level to a tone (mockup-faithful)', () => {
    expect(SCOPE_LEVEL_TONE).toEqual({
      global: 'info', office_subtree: 'primary', office: 'warning', own: 'neutral'
    })
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- data-scope-constants`
Expected: FAIL — cannot resolve `~/constants/dataScope`.

- [ ] **Step 3: Create the constants file**

Create `frontend/app/constants/dataScope.ts`:

```ts
// Scope-level presentation metadata. The backend supplies the authoritative
// scope_modules + scope_levels via /authz/catalog; tone (color) and i18n
// descriptions are a frontend concern.
export const SCOPE_LEVEL_KEYS = ['global', 'office_subtree', 'office', 'own'] as const
export type ScopeLevel = typeof SCOPE_LEVEL_KEYS[number]
export type ScopeTone = 'info' | 'primary' | 'warning' | 'neutral'

export const SCOPE_LEVEL_TONE: Record<ScopeLevel, ScopeTone> = {
  global: 'info',
  office_subtree: 'primary',
  office: 'warning',
  own: 'neutral'
}
```

- [ ] **Step 4: Add i18n keys**

In `frontend/i18n/locales/id.json` and `en.json`, under the existing `settings.dataScope` object, add `level`, `module`, `loadError`, `retry`.

**id.json** `settings.dataScope`:
```json
"level": {
  "global": "Semua data lintas kantor",
  "office_subtree": "Kantor sendiri + seluruh turunannya",
  "office": "Hanya kantor sendiri",
  "own": "Hanya data miliknya"
},
"module": {
  "offices": "Kantor",
  "employees": "Pegawai",
  "assets": "Aset",
  "requests": "Pengajuan",
  "audit": "Audit"
},
"loadError": "Gagal memuat kebijakan data scope.",
"retry": "Coba lagi"
```

**en.json** `settings.dataScope`:
```json
"level": {
  "global": "All data across offices",
  "office_subtree": "Own office + all its descendants",
  "office": "Own office only",
  "own": "Only their own data"
},
"module": {
  "offices": "Offices",
  "employees": "Employees",
  "assets": "Assets",
  "requests": "Requests",
  "audit": "Audit"
},
"loadError": "Failed to load data-scope policies.",
"retry": "Retry"
```

Read the `settings.dataScope` section of each file first; insert as valid JSON (no trailing commas after the last member; commas between members).

- [ ] **Step 5: Run test + lint**

Run (from `frontend/`): `pnpm test -- data-scope-constants && pnpm lint`
Expected: PASS, lint clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/constants/dataScope.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/unit/data-scope-constants.spec.ts
git commit -m "feat(datascope): scope-level constants + i18n labels"
```

---

### Task 2: Rewrite `useDataScope` to the real API

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useDataScope.ts`
- Delete: `frontend/test/unit/data-scope-mock.spec.ts`
- Test: `frontend/test/unit/use-data-scope.spec.ts`

**Interfaces:**
- Consumes: `useApiClient().request`; `ScopeLevel` (Task 1).
- Produces: `ScopeModuleView{key}`, `ScopeRoleView{id,code,name,sub,def,ov}`; `useDataScope()` → `{ getModules, listRoles, saveRoleScope }`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/test/unit/use-data-scope.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

import { useDataScope } from '~/composables/api/useDataScope'

beforeEach(() => request.mockReset())

describe('useDataScope', () => {
  it('getModules drops the "*" sentinel', async () => {
    request.mockResolvedValueOnce({ scope_modules: ['*', 'offices', 'assets'] })
    const mods = await useDataScope().getModules()
    expect(request).toHaveBeenCalledWith('/authz/catalog')
    expect(mods).toEqual([{ key: 'offices' }, { key: 'assets' }])
  })

  it('listRoles maps policies to def + ov', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', code: 'manager', name: 'Manager', description: 'Ops' }], total: 1 })
      .mockResolvedValueOnce({ policies: [{ module: '*', scope_level: 'office' }, { module: 'assets', scope_level: 'office_subtree' }] })
    const roles = await useDataScope().listRoles()
    expect(request).toHaveBeenNthCalledWith(1, '/authz/roles')
    expect(request).toHaveBeenNthCalledWith(2, '/authz/roles/r1/scope')
    expect(roles[0]).toEqual({ id: 'r1', code: 'manager', name: 'Manager', sub: 'Ops', def: 'office', ov: { assets: 'office_subtree' } })
  })

  it('listRoles falls back to own when no "*" policy', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r2', code: 'staf', name: 'Staf' }], total: 1 })
      .mockResolvedValueOnce({ policies: [] })
    const roles = await useDataScope().listRoles()
    expect(roles[0].def).toBe('own')
    expect(roles[0].ov).toEqual({})
    expect(roles[0].sub).toBe('')
  })

  it('saveRoleScope always includes the "*" default plus overrides', async () => {
    request.mockResolvedValueOnce({ policies: [] })
    await useDataScope().saveRoleScope('r1', 'office', { assets: 'office_subtree' })
    expect(request).toHaveBeenCalledWith('/authz/roles/r1/scope', {
      method: 'PUT',
      body: { policies: [{ module: '*', scope_level: 'office' }, { module: 'assets', scope_level: 'office_subtree' }] }
    })
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-data-scope`
Expected: FAIL — new `useDataScope` shape undefined.

- [ ] **Step 3: Rewrite `useDataScope.ts`**

Replace `frontend/app/composables/api/useDataScope.ts` entirely with:

```ts
import type { ScopeLevel } from '~/constants/dataScope'

export interface ScopeModuleView { key: string }
export interface ScopeRoleView { id: string; code: string; name: string; sub: string; def: ScopeLevel; ov: Record<string, ScopeLevel> }

interface CatalogResponse { scope_modules: string[] }
interface RoleDTO { id: string; code: string; name: string; description?: string }
interface ScopeResponse { policies: { module: string; scope_level: ScopeLevel }[] }

/**
 * Data-scope policies, wired to /api/v1/authz. Module columns come from the
 * catalog's scope_modules; each role's default (module "*") + per-module
 * overrides come from /authz/roles/:id/scope.
 */
export function useDataScope() {
  const { request } = useApiClient()

  async function getModules(): Promise<ScopeModuleView[]> {
    const cat = await request<CatalogResponse>('/authz/catalog')
    return cat.scope_modules.filter(m => m !== '*').map(key => ({ key }))
  }

  async function listRoles(): Promise<ScopeRoleView[]> {
    const res = await request<{ data: RoleDTO[]; total: number }>('/authz/roles')
    return Promise.all(res.data.map(async (r) => {
      const sc = await request<ScopeResponse>(`/authz/roles/${r.id}/scope`)
      const def: ScopeLevel = sc.policies.find(p => p.module === '*')?.scope_level ?? 'own'
      const ov: Record<string, ScopeLevel> = {}
      for (const p of sc.policies) {
        if (p.module !== '*') ov[p.module] = p.scope_level
      }
      return { id: r.id, code: r.code, name: r.name, sub: r.description ?? '', def, ov }
    }))
  }

  async function saveRoleScope(id: string, def: ScopeLevel, ov: Record<string, ScopeLevel>): Promise<void> {
    const policies = [
      { module: '*', scope_level: def },
      ...Object.entries(ov).map(([module, scope_level]) => ({ module, scope_level }))
    ]
    await request(`/authz/roles/${id}/scope`, { method: 'PUT', body: { policies } })
  }

  return { getModules, listRoles, saveRoleScope }
}
```

- [ ] **Step 4: Run tests + typecheck**

Run (from `frontend/`): `pnpm test -- use-data-scope && pnpm lint`
Expected: PASS, lint clean. NOTE: `pnpm typecheck` will fail ONLY in `pages/settings/data-scope.vue` + `components/scope/ScopeCell.vue` (they still import the old shape / `~/mock/dataScope`) — that is EXPECTED and fixed in Task 3. Do NOT edit those here.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useDataScope.ts frontend/test/unit/use-data-scope.spec.ts
git rm frontend/test/unit/data-scope-mock.spec.ts
git commit -m "feat(datascope): wire useDataScope to /authz API (English DTO, id identity)"
```

---

### Task 3: Update the page + ScopeCell

**Files:**
- Modify (script + template bindings): `frontend/app/pages/settings/data-scope.vue`
- Modify: `frontend/app/components/scope/ScopeCell.vue`

**Interfaces:**
- Consumes: `useDataScope()` (Task 2); `SCOPE_LEVEL_KEYS`/`SCOPE_LEVEL_TONE`/`ScopeLevel`/`ScopeTone` (Task 1); i18n `settings.dataScope.level.*` + `.module.*`.

- [ ] **Step 1: Rewrite `ScopeCell.vue` imports + level descriptions**

In `frontend/app/components/scope/ScopeCell.vue`, replace the mock import + `SCOPE_LEVELS` usages:
- Change line 2–3 imports to:
```ts
import type { ScopeLevel, ScopeTone } from '~/constants/dataScope'
import { SCOPE_LEVEL_KEYS, SCOPE_LEVEL_TONE } from '~/constants/dataScope'
```
- Replace `const effTone = computed(() => SCOPE_LEVELS[props.effective].tone)` with `const effTone = computed(() => SCOPE_LEVEL_TONE[props.effective])`.
- Replace the `levelDesc` function body with an i18n lookup:
```ts
function levelDesc(level: ScopeLevel): string {
  return t(`settings.dataScope.level.${level}`)
}
```
- In the template, replace `:class="toneClasses[SCOPE_LEVELS[lvl].tone].dot"` with `:class="toneClasses[SCOPE_LEVEL_TONE[lvl]].dot"`.
- The `toneClasses` map, `isOverride`/`isInheriting`, and the rest stay as-is. Remove the now-unused `locale` from `useI18n()` if it is no longer referenced (it was only used by the old `levelDesc`).

- [ ] **Step 2: Rewrite the page `<script setup>`**

Replace the `<script setup>` block of `frontend/app/pages/settings/data-scope.vue` with:

```ts
import type { ScopeRoleView, ScopeModuleView } from '~/composables/api/useDataScope'
import { useDataScope } from '~/composables/api/useDataScope'
import type { ScopeLevel, ScopeTone } from '~/constants/dataScope'
import { SCOPE_LEVEL_KEYS, SCOPE_LEVEL_TONE } from '~/constants/dataScope'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

const { t, te } = useI18n()
const toast = useToast()
const { getModules, listRoles, saveRoleScope } = useDataScope()

const roles = ref<ScopeRoleView[]>([])
const modules = ref<ScopeModuleView[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const dirtyIds = ref(new Set<string>())
const dirty = computed(() => dirtyIds.value.size > 0)

const toneDot: Record<ScopeTone, string> = {
  info: 'bg-info',
  primary: 'bg-primary',
  warning: 'bg-warning',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}

const legend = computed(() => SCOPE_LEVEL_KEYS.map(k => ({
  key: k,
  dot: toneDot[SCOPE_LEVEL_TONE[k]],
  desc: t(`settings.dataScope.level.${k}`)
})))

function moduleLabel(key: string): string {
  const k = `settings.dataScope.module.${key}`
  return te(k) ? t(k) : key
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const [mods, roleList] = await Promise.all([getModules(), listRoles()])
    modules.value = mods
    roles.value = roleList
    dirtyIds.value = new Set()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function findRole(id: string) {
  return roles.value.find(r => r.id === id)
}
function setDefault(id: string, level: ScopeLevel) {
  const r = findRole(id)
  if (!r) return
  r.def = level
  dirtyIds.value.add(id)
}
function setOverride(id: string, mod: string, level: ScopeLevel) {
  const r = findRole(id)
  if (!r) return
  r.ov = { ...r.ov, [mod]: level }
  dirtyIds.value.add(id)
}
function clearOverride(id: string, mod: string) {
  const r = findRole(id)
  if (!r) return
  const { [mod]: _omit, ...rest } = r.ov
  r.ov = rest
  dirtyIds.value.add(id)
}

async function save() {
  if (!dirty.value) return
  saving.value = true
  try {
    const ids = [...dirtyIds.value]
    await Promise.all(ids.map((id) => {
      const r = findRole(id)
      return r ? saveRoleScope(id, r.def, r.ov) : Promise.resolve()
    }))
    dirtyIds.value = new Set()
    toast.add({ title: t('settings.dataScope.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

onMounted(() => load())
```

Notes: id identity throughout; `dirtyIds` (reactive Set — Vue 3 tracks `.add`/`.size`); the `locale` watch is removed (labels come from i18n reactively); module label resolved via `moduleLabel`.

- [ ] **Step 3: Update the page template**

In the `<template>` of `data-scope.vue`:
- Role rows: `:key="r.key"` → `:key="r.id"`; `{{ r.nama }}` → `{{ r.name }}`; `{{ r.sub }}` stays.
- Module header: `{{ m.label }}` → `{{ moduleLabel(m.key) }}`.
- `ScopeCell` events: `@select="setDefault(r.key, $event)"` → `setDefault(r.id, $event)`; `@select="setOverride(r.key, m.key, $event)"` → `setOverride(r.id, m.key, $event)`; `@clear="clearOverride(r.key, m.key)"` → `clearOverride(r.id, m.key)`.
- Legend desc binding `{{ l.desc }}` already resolved via the computed; no change beyond the script.
- Add a load-error state right after the `v-if="loading"` block (before `<template v-else>`), and make the populated content show only when not loading AND not failed:

```vue
    <div
      v-else-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-20 text-muted"
    >
      <UIcon name="i-lucide-circle-alert" class="size-6" />
      <span class="text-sm">{{ t('settings.dataScope.loadError') }}</span>
      <UButton color="neutral" variant="subtle" @click="load">
        {{ t('settings.dataScope.retry') }}
      </UButton>
    </div>

    <template v-else>
```

- [ ] **Step 4: Verify build/lint/typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: exit 0. NOTE: `pnpm test` will still FAIL on `test/nuxt/settings-data-scope.spec.ts` (stubs the old mock shape) — that is fixed in Task 4. Run `pnpm test -- use-data-scope data-scope-constants` to confirm Task 1/2 unit tests still pass.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/settings/data-scope.vue frontend/app/components/scope/ScopeCell.vue
git commit -m "feat(datascope): page + ScopeCell on real API (id identity, dirty-id save, load error)"
```

---

### Task 4: Nuxt component test for the wired page

**Files:**
- Modify (rewrite): `frontend/test/nuxt/settings-data-scope.spec.ts`

**Interfaces:**
- Consumes: the wired page; mock the HTTP layer the same way the RBAC component test does.

- [ ] **Step 1: Study the patterns**

Read the CURRENT `frontend/test/nuxt/settings-data-scope.spec.ts` (store-reset + `setSession(['*'])` + `mountSuspended`) AND `frontend/test/nuxt/settings-rbac.spec.ts` (the already-wired RBAC test — it uses `vi.mock('~/composables/useApiClient', ...)` with a per-test `setHandler` that routes by method+path and captures request bodies). Mirror that exact stub approach: the wired Data Scope page calls `GET /authz/catalog`, `GET /authz/roles`, `GET /authz/roles/:id/scope`, `PUT /authz/roles/:id/scope`.

- [ ] **Step 2: Write the rewritten test**

Rewrite `frontend/test/nuxt/settings-data-scope.spec.ts` to stub the real endpoints and assert real behavior. Provide a catalog fixture with `scope_modules: ['*','offices','employees','assets','requests','audit']`, a roles fixture (e.g. Superadmin def `global`, Manager def `office` with an `assets: office_subtree` override), and per-role scope responses. Cover:
- Loaded grid: module column headers render the i18n labels (e.g. "Kantor"/"Offices", "Aset"/"Assets"); role rows render seeded role names; the "Default" column + legend render the 4 levels with their i18n descriptions.
- Changing a role's default (via ScopeCell select) marks dirty + enables Save; clicking Save issues `PUT /authz/roles/:id/scope` whose body `policies` contains `{module:'*', scope_level:<new>}` (assert the captured body); dirty clears.
- Setting a module override puts `{module:<mod>, scope_level:<lvl>}` in the PUT body alongside `*`; clearing an override drops it from the body.
- Only changed roles are PUT: change exactly one role and assert exactly one PUT fired.
- Load-error: when `GET /authz/roles` returns 500, the error block + retry render.

Assert real rendered text + captured request bodies — no hollow checks. Use the harness default locale and assert the resolved i18n strings for that locale. Interacting with `ScopeCell`'s popover may require opening it (click the cell button) then clicking a level option — follow how the existing data-scope test drives the cell, adapting to id identity.

- [ ] **Step 3: Run the test + whole suite**

Run (from `frontend/`): `pnpm test -- settings-data-scope` then `pnpm test`
Expected: target PASS; whole suite green.

- [ ] **Step 4: Commit**

```bash
git add frontend/test/nuxt/settings-data-scope.spec.ts
git commit -m "test(datascope): component test against stubbed /authz endpoints"
```

---

### Task 5: E2E + mockup fidelity + delete mock + PROGRESS + full gate

**Files:**
- Modify: `frontend/e2e/settings.spec.ts`
- Delete (if orphaned): `frontend/app/mock/dataScope.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Update the e2e Data Scope assertions**

Read `frontend/e2e/settings.spec.ts` + `frontend/e2e/helpers.ts` (`login()`). Add/Update a `/settings/data-scope` spec that runs against the real backend with a seeded admin: assert the grid renders with the real module columns (e.g. "Aset"/"Kantor") and seeded role rows; change one role's scope cell + Save; reload → the change persists. Keep the RBAC + audit specs untouched. NOTE: you likely cannot RUN `pnpm test:e2e` here (needs the full backend stack); ensure the spec compiles + lints; it runs in CI. Use robust locators (by visible role name / scope level text), not brittle Tailwind class selectors.

- [ ] **Step 2: Delete the orphaned mock**

Check whether `frontend/app/mock/dataScope.ts` still has importers: `grep -rn "mock/dataScope" frontend/app frontend/test` (exclude the file itself). If zero importers remain (ScopeCell + page now import from `~/constants/dataScope`; the composable no longer imports it; the old mock spec was deleted), `git rm frontend/app/mock/dataScope.ts`. If something still imports it, do NOT delete — report what does.

- [ ] **Step 3: Mockup 1:1 comparison**

Open the built `/settings/data-scope` screen and `docs/design/Data Scope.dc.html` side by side. Verify layout/legend/table/sticky-columns/override-pill/dirty-indicator/states (loading/error/populated) match. The module column SET intentionally differs (real backend modules) — not a regression. Fix any other deviation in `data-scope.vue`/`ScopeCell.vue`. Report the comparison result.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`, under the frontend "Wire screens to real backend APIs" sub-list (which already lists RBAC ✅), mark **Data Scope ✅ wired to `/authz`** (catalog scope_modules + per-role scope policies; English DTO; id identity; save only changed roles). Refresh "▶ Next session — start here" → **Field Permission** next. Don't invent status for other screens.

- [ ] **Step 5: Full frontend gate**

Run (from `frontend/`):
```
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```
Expected: all green. (E2E `pnpm test:e2e` runs in CI's e2e job.)

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/settings.spec.ts docs/PROGRESS.md
git commit -m "test(datascope): e2e against real backend + progress; drop orphaned mock"
```
(If `mock/dataScope.ts` was deleted, the `git rm` is already staged — include it in this commit.)

---

## Self-Review

**Spec coverage:**
- §2 composable rewrite (getModules drops `*`; listRoles eager per-role scope → def/ov; saveRoleScope always-`*`) → Task 2. ✓
- §3 constants relocation + i18n (level desc + module labels + loadError/retry; tone map) → Task 1 + Task 3 (ScopeCell/page import from constants, i18n). ✓
- §4 page (id identity, dirtyIds per-role save, loadError, moduleLabel) → Task 3. ✓
- §5 tests (unit/component/e2e) → Tasks 2, 4, 5. ✓
- §6 done (mockup compare, delete mock, PROGRESS, gate) → Task 5. ✓

**Placeholder scan:** Task 4 + Task 5 give explicit assertion lists / steps (Step 1 says read the existing stub pattern first, since ScopeCell popover interaction + the stub helper differ across the repo). Concrete checklists, not "TODO"s.

**Type consistency:** `ScopeRoleView{id,code,name,sub,def,ov}`, `ScopeModuleView{key}`, `ScopeLevel`, `SCOPE_LEVEL_TONE`, `SCOPE_LEVEL_KEYS`, `saveRoleScope(id,def,ov)`, `getModules`, `listRoles` consistent across Tasks 1/2/3/4. Page `dirtyIds`/`findRole(id)`/`setDefault(id,...)`/`setOverride(id,mod,...)`/`clearOverride(id,mod)` consistent with the template bindings. ScopeCell consumes `ScopeLevel`/`ScopeTone`/`SCOPE_LEVEL_TONE`/`SCOPE_LEVEL_KEYS` from constants (Task 1).
