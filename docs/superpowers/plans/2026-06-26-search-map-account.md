# Global Search · Office Map · Account Profile — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build three mock-first frontend screens — a ⌘K global command palette, a Leaflet office-location map, and an account profile/settings page — each reproducing its `docs/design/*.dc.html` mockup 1:1.

**Architecture:** Nuxt 4 SPA. Each screen has a presentational layer (`U*` + semantic tokens, faithful to the mockup) backed by a mock-first composable (`fakeLatency` + in-memory/`localStorage` store) that can later swap to `$fetch` behind the same interface. State lives in pages/composables; components stay presentational. The map panel is the only deliberate deviation: a real Leaflet + OpenStreetMap map instead of the mockup's illustrative SVG (user-requested), inside the mockup's exact layout.

**Tech Stack:** Nuxt 4, `@nuxt/ui` v4, `@nuxtjs/i18n`, Pinia, `useColorMode`, Vitest + `@nuxt/test-utils` (`mountSuspended`), Playwright, Leaflet.

## Global Constraints

- **Mock-first only.** No backend calls. Data through `app/composables/api/use*` using `fakeLatency()`. Identity from `useAuthStore().user`; theme from `useColorMode()`; language from `useI18n()`.
- **Mockup fidelity 1:1.** Build exactly what each `.dc.html` shows. No self-initiated changes/simplifications/drops. The only deviation is the Leaflet map (explicitly approved). For a literal hex in a mockup, substitute the equivalent **semantic token** but keep structure/intent.
- **Office map categories follow the mockup exactly:** jenis `Pusat / Wilayah / Cabang / Outlet` with the mockup's four colors via tokens (`pin-pusat`→primary, `pin-wilayah`→info, `pin-cabang`→warning, `pin-outlet`→neutral). Self-contained map dataset (the mockup's 9 offices). Do **not** reuse `officeStore`.
- **i18n mandatory.** Every user-facing string in `i18n/locales/{id,en}.json`; reference via `$t`/`t`. Default locale `id`.
- **Tokens not hex.** Use semantic Nuxt UI colors / CSS vars (`text-muted`, `bg-default`, `border-default`, `text-primary`, `--ui-*`). No literal Tailwind colors.
- **Lint rules:** no trailing commas (`commaDangle: 'never'`), 1tbs brace style. `pnpm lint` + `pnpm typecheck` must pass.
- **Tests required & broad:** unit (node) for pure logic; `mountSuspended` runtime (`// @vitest-environment nuxt`) for components; Playwright e2e for flows. Assert real behavior (rendered text, resolved i18n, navigation, emitted events) — cover happy path **and** empty/loading/error/invalid/permission states. `pnpm test` gates CI.
- **Verify before commit:** run `pnpm lint`, `pnpm typecheck`, `pnpm test`. Branch: `feat/search-map-account` (already created).
- **Mockup line-range references** below point into the three `.dc.html` files; treat them as the markup source of truth and finish each screen with a side-by-side light+dark comparison.

---

# Part A — Global Search (⌘K Command Palette)

Mockup: `docs/design/Global Search.dc.html`. Topbar trigger (button, lines 62–68), palette overlay (99–187), states: initial recent+quick (122–141), loading skeleton (115–119), grouped results (144–163), no-results (166–172), footer (177–183). Data model & group order: lines 218–235.

### Task A1: Search types + i18n keys

**Files:**
- Modify: `frontend/app/types/index.ts` (append)
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`

**Interfaces:**
- Produces: `SearchEntityType`, `SearchItem`, `SearchGroup` types; `search.*` i18n namespace.

- [ ] **Step 1: Add types** — append to `frontend/app/types/index.ts`:

```ts
export type SearchEntityType = 'aset' | 'pegawai' | 'kantor' | 'user' | 'pengajuan'

export interface SearchItem {
  type: SearchEntityType
  title: string
  sub: string
  status: string | null
  icon: string
  to: string
}

export interface SearchGroup {
  type: SearchEntityType
  labelKey: string
  total: number
  items: SearchItem[]
}
```

- [ ] **Step 2: Add i18n keys** — in `frontend/i18n/locales/id.json`, add a top-level `"search"` object:

```json
  "search": {
    "topbarPlaceholder": "Cari aset, pegawai, kantor, pengajuan…",
    "placeholder": "Cari aset, pegawai, kantor, pengajuan…",
    "hint": "Tekan untuk pencarian cepat",
    "openNow": "Buka sekarang",
    "recentTitle": "Pencarian Terakhir",
    "quickTitle": "Aksi Cepat",
    "qAddAsset": "Tambah Aset",
    "qOpenReports": "Buka Laporan",
    "qCreateRequest": "Buat Pengajuan",
    "seeAll": "Lihat semua ({n})",
    "emptyTitle": "Tidak ada hasil untuk \"{q}\"",
    "emptySub": "Coba kata kunci lain atau periksa ejaannya.",
    "navHint": "navigasi",
    "openHint": "buka",
    "closeHint": "tutup",
    "scopeNote": "Hasil dibatasi lingkup & izin Anda",
    "group": {
      "aset": "Aset",
      "pegawai": "Pegawai",
      "kantor": "Kantor",
      "user": "User",
      "pengajuan": "Pengajuan"
    }
  },
```

- [ ] **Step 3: Add English** — in `frontend/i18n/locales/en.json`, mirror with: `topbarPlaceholder`/`placeholder` "Search assets, staff, offices, requests…", `hint` "Press for quick search", `openNow` "Open now", `recentTitle` "Recent Searches", `quickTitle` "Quick Actions", `qAddAsset` "Add Asset", `qOpenReports` "Open Reports", `qCreateRequest` "Create Request", `seeAll` "See all ({n})", `emptyTitle` "No results for \"{q}\"", `emptySub` "Try another keyword or check the spelling.", `navHint` "navigate", `openHint` "open", `closeHint` "close", `scopeNote` "Results limited to your scope & permissions", group → "Assets"/"Employees"/"Offices"/"Users"/"Requests".

- [ ] **Step 4: Verify typecheck**

Run: `cd frontend && pnpm typecheck`
Expected: PASS (no errors).

- [ ] **Step 5: Commit**

```bash
git add frontend/app/types/index.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "feat(search): add search types and i18n namespace"
```

---

### Task A2: `useGlobalSearch` composable (mock aggregator)

**Files:**
- Create: `frontend/app/composables/api/useGlobalSearch.ts`
- Test: `frontend/test/nuxt/useGlobalSearch.spec.ts`

**Interfaces:**
- Consumes: `SearchGroup`, `SearchItem` (A1); `assetStore`, `employeeStore`, `officeStore`, `userStore`, `approvalStore` from `~/mock/*`; `fakeLatency`.
- Produces: `useGlobalSearch()` → `{ search(query: string): Promise<SearchGroup[]> }`. Group order: `['aset','pegawai','kantor','user','pengajuan']`. Empty/whitespace query → `[]`.

- [ ] **Step 1: Write the failing test** — `frontend/test/nuxt/useGlobalSearch.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { useGlobalSearch } from '~/composables/api/useGlobalSearch'

describe('useGlobalSearch', () => {
  const { search } = useGlobalSearch()

  it('returns no groups for an empty or whitespace query', async () => {
    expect(await search('')).toEqual([])
    expect(await search('   ')).toEqual([])
  })

  it('matches assets by name and tag, case-insensitively', async () => {
    const groups = await search('latitude')
    const aset = groups.find(g => g.type === 'aset')
    expect(aset).toBeTruthy()
    expect(aset!.items.some(i => i.title.includes('Latitude'))).toBe(true)
    expect(aset!.items[0]!.to).toMatch(/^\/assets\//)
  })

  it('groups results in fixed order and reports a total per group', async () => {
    const groups = await search('a')
    const order = groups.map(g => g.type)
    const expected = ['aset', 'pegawai', 'kantor', 'user', 'pengajuan'].filter(t => order.includes(t))
    expect(order).toEqual(expected)
    for (const g of groups) expect(g.total).toBeGreaterThanOrEqual(g.items.length)
  })

  it('returns an empty array when nothing matches', async () => {
    expect(await search('zzzzzzz-no-match')).toEqual([])
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm test useGlobalSearch`
Expected: FAIL — cannot resolve `~/composables/api/useGlobalSearch`.

- [ ] **Step 3: Write minimal implementation** — `frontend/app/composables/api/useGlobalSearch.ts`:

```ts
import type { SearchGroup, SearchItem, SearchEntityType } from '~/types'
import { fakeLatency } from '~/mock/helpers'
import { assetStore } from '~/mock/assets'
import { employeeStore } from '~/mock/employees'
import { officeStore } from '~/mock/offices'
import { userStore } from '~/mock/users'
import { approvalStore } from '~/mock/approval'

const ORDER: SearchEntityType[] = ['aset', 'pegawai', 'kantor', 'user', 'pengajuan']
const ICON: Record<SearchEntityType, string> = {
  aset: 'i-lucide-package',
  pegawai: 'i-lucide-user',
  kantor: 'i-lucide-building',
  user: 'i-lucide-shield',
  pengajuan: 'i-lucide-check-square'
}

function match(q: string, ...fields: (string | null | undefined)[]): boolean {
  return fields.some(f => String(f ?? '').toLowerCase().includes(q))
}

export function useGlobalSearch() {
  async function search(query: string): Promise<SearchGroup[]> {
    const q = query.trim().toLowerCase()
    if (!q) return []
    await fakeLatency(220)

    const byType: Record<SearchEntityType, SearchItem[]> = {
      aset: [], pegawai: [], kantor: [], user: [], pengajuan: []
    }

    for (const a of assetStore.all()) {
      if (match(q, a.nama, a.tag)) {
        byType.aset.push({ type: 'aset', title: a.nama, sub: a.tag, status: a.status, icon: ICON.aset, to: `/assets/${a.tag}` })
      }
    }
    for (const e of employeeStore.all()) {
      if (match(q, e.nama, e.nip, e.jabatan)) {
        byType.pegawai.push({ type: 'pegawai', title: e.nama, sub: `${e.jabatan} · ${e.departemen}`, status: null, icon: ICON.pegawai, to: '/master/employees' })
      }
    }
    for (const o of officeStore.all()) {
      if (match(q, o.nama, o.kode, o.kota)) {
        byType.kantor.push({ type: 'kantor', title: o.nama, sub: `${o.kode} · ${o.provinsi}`, status: o.active ? 'aktif' : null, icon: ICON.kantor, to: '/master/offices' })
      }
    }
    for (const u of userStore.all()) {
      if (match(q, u.nama, u.email)) {
        byType.user.push({ type: 'user', title: u.email, sub: `${u.peran} · ${u.kantor}`, status: null, icon: ICON.user, to: '/settings/users' })
      }
    }
    for (const r of approvalStore.all()) {
      if (match(q, r.judul, r.id)) {
        byType.pengajuan.push({ type: 'pengajuan', title: r.judul, sub: r.id, status: r.status, icon: ICON.pengajuan, to: '/approval' })
      }
    }

    return ORDER
      .filter(t => byType[t].length > 0)
      .map(t => ({ type: t, labelKey: `search.group.${t}`, total: byType[t].length, items: byType[t].slice(0, 5) }))
  }

  return { search }
}
```

- [ ] **Step 4: Confirm `approvalStore.all()` exists** — open `frontend/app/mock/approval.ts` around line 150. If the store exposes rows under a different method (e.g. `list()`), use that method here instead of `all()`. Adjust the import/call to match the actual export.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && pnpm test useGlobalSearch`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add frontend/app/composables/api/useGlobalSearch.ts frontend/test/nuxt/useGlobalSearch.spec.ts
git commit -m "feat(search): mock global-search aggregator composable"
```

---

### Task A3: `useCommandPalette` state + recent searches

**Files:**
- Create: `frontend/app/composables/useCommandPalette.ts`
- Test: `frontend/test/nuxt/useCommandPalette.spec.ts`

**Interfaces:**
- Produces: `useCommandPalette()` → `{ isOpen: Ref<boolean>, open(), close(), toggle(), recent: Ref<string[]>, pushRecent(q: string) }`. `recent` capped at 5, de-duplicated (most-recent first), persisted to `localStorage` key `inventra.search.recent`.

- [ ] **Step 1: Write the failing test** — `frontend/test/nuxt/useCommandPalette.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { useCommandPalette } from '~/composables/useCommandPalette'

describe('useCommandPalette', () => {
  beforeEach(() => {
    localStorage.clear()
    const { close, recent } = useCommandPalette()
    close()
    recent.value = []
  })

  it('opens, closes, and toggles', () => {
    const p = useCommandPalette()
    expect(p.isOpen.value).toBe(false)
    p.open(); expect(p.isOpen.value).toBe(true)
    p.toggle(); expect(p.isOpen.value).toBe(false)
  })

  it('shares state across calls (singleton)', () => {
    useCommandPalette().open()
    expect(useCommandPalette().isOpen.value).toBe(true)
  })

  it('pushes recent searches, most-recent-first, de-duplicated, capped at 5', () => {
    const p = useCommandPalette()
    for (const q of ['a', 'b', 'c', 'd', 'e', 'f']) p.pushRecent(q)
    expect(p.recent.value).toEqual(['f', 'e', 'd', 'c', 'b'])
    p.pushRecent('e')
    expect(p.recent.value[0]).toBe('e')
    expect(p.recent.value.filter(x => x === 'e')).toHaveLength(1)
  })

  it('ignores blank recent entries', () => {
    const p = useCommandPalette()
    p.pushRecent('   ')
    expect(p.recent.value).toEqual([])
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm test useCommandPalette`
Expected: FAIL — module not found.

- [ ] **Step 3: Write implementation** — `frontend/app/composables/useCommandPalette.ts`:

```ts
const KEY = 'inventra.search.recent'

export function useCommandPalette() {
  const isOpen = useState('cmdp-open', () => false)
  const recent = useState<string[]>('cmdp-recent', () => {
    if (import.meta.client) {
      try {
        const raw = localStorage.getItem(KEY)
        if (raw) return JSON.parse(raw) as string[]
      } catch { /* ignore */ }
    }
    return []
  })

  function open() { isOpen.value = true }
  function close() { isOpen.value = false }
  function toggle() { isOpen.value = !isOpen.value }

  function pushRecent(q: string) {
    const term = q.trim()
    if (!term) return
    recent.value = [term, ...recent.value.filter(x => x !== term)].slice(0, 5)
    if (import.meta.client) {
      try { localStorage.setItem(KEY, JSON.stringify(recent.value)) } catch { /* ignore */ }
    }
  }

  return { isOpen, open, close, toggle, recent, pushRecent }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm test useCommandPalette`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/useCommandPalette.ts frontend/test/nuxt/useCommandPalette.spec.ts
git commit -m "feat(search): command-palette open state + recent searches"
```

---

### Task A4: `CommandPalette.vue` component

**Files:**
- Create: `frontend/app/components/CommandPalette.vue`
- Test: `frontend/test/nuxt/CommandPalette.spec.ts`

**Interfaces:**
- Consumes: `useCommandPalette` (A3), `useGlobalSearch` (A2), `useCan`, `useI18n`, `navigateTo`.
- Produces: a self-contained overlay component (no props). Renders nothing when `isOpen` is false.

- [ ] **Step 1: Write the component** — `frontend/app/components/CommandPalette.vue`. Reproduce the mockup overlay (lines 99–187). Full `<script setup>`:

```vue
<script setup lang="ts">
import type { SearchGroup, SearchItem } from '~/types'

const { t } = useI18n()
const can = useCan()
const { isOpen, close, recent, pushRecent } = useCommandPalette()
const { search } = useGlobalSearch()

const query = ref('')
const loading = ref(false)
const groups = ref<SearchGroup[]>([])
const sel = ref(0)
let seq = 0

const quickActions = computed(() => [
  { key: 'a', labelKey: 'search.qAddAsset', icon: 'i-lucide-plus', to: '/assets/new', perm: 'masterdata.office.manage' },
  { key: 'l', labelKey: 'search.qOpenReports', icon: 'i-lucide-bar-chart-2', to: '/reports', perm: '' },
  { key: 'p', labelKey: 'search.qCreateRequest', icon: 'i-lucide-send', to: '/approval', perm: '' }
].filter(a => !a.perm || can(a.perm)))

const hasQuery = computed(() => query.value.trim().length > 0)
const showInitial = computed(() => isOpen.value && !hasQuery.value)
const showLoading = computed(() => isOpen.value && hasQuery.value && loading.value)
const showResults = computed(() => isOpen.value && hasQuery.value && !loading.value && groups.value.length > 0)
const showEmpty = computed(() => isOpen.value && hasQuery.value && !loading.value && groups.value.length === 0)

// flat list of items for keyboard navigation, in group order
const flat = computed<SearchItem[]>(() => groups.value.flatMap(g => g.items))

watch(query, async (q) => {
  sel.value = 0
  if (!q.trim()) { groups.value = []; loading.value = false; return }
  loading.value = true
  const mine = ++seq
  const res = await search(q)
  if (mine === seq) { groups.value = res; loading.value = false }
})

watch(isOpen, (v) => {
  if (!v) { query.value = ''; groups.value = []; sel.value = 0 }
})

function go(item: SearchItem) {
  pushRecent(item.title)
  close()
  navigateTo(item.to)
}

function runQuick(to: string) { close(); navigateTo(to) }
function useRecent(term: string) { query.value = term }

function onKey(e: KeyboardEvent) {
  if (!isOpen.value) return
  if (e.key === 'Escape') { e.preventDefault(); close() }
  else if (e.key === 'ArrowDown') { e.preventDefault(); if (flat.value.length) sel.value = (sel.value + 1) % flat.value.length }
  else if (e.key === 'ArrowUp') { e.preventDefault(); if (flat.value.length) sel.value = (sel.value - 1 + flat.value.length) % flat.value.length }
  else if (e.key === 'Enter') { e.preventDefault(); const it = flat.value[sel.value]; if (it) go(it) }
}

function onGlobalKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    isOpen.value = !isOpen.value
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKey))
onUnmounted(() => window.removeEventListener('keydown', onGlobalKey))

// index helper so the template can compare a group/item against the flat selection
function flatIndex(gi: number, ii: number): number {
  let n = 0
  for (let i = 0; i < gi; i++) n += groups.value[i]!.items.length
  return n + ii
}
</script>
```

Build the `<template>` faithful to mockup lines 99–187:
- Root `<Teleport to="body">` + `v-if="isOpen"` overlay: `position:fixed; inset:0; z-60` backdrop `bg-[rgba(2,6,23,0.45)] backdrop-blur-sm`, flex top-aligned, `padding:88px 20px 20px`. Click backdrop → `close()`; inner panel `@click.stop`.
- Panel: `max-w-[640px] w-full`, `bg-default border border-default rounded-2xl shadow-xl`, column flex, `max-h-[calc(100vh-130px)]`.
- Input row (104–109): search icon, `<input v-model="query" :placeholder="t('search.placeholder')" @keydown="onKey" autofocus>`, a clear button when `hasQuery`, and an `Esc` chip button → `close()`.
- Body (`flex-1 overflow-y-auto p-2`):
  - `showLoading` → 4 shimmer skeleton rows (mockup 115–119; reuse the `animate-[shimmer…]` style already used by `TableSkeleton`/`CardSkeleton` — copy that gradient class).
  - `showInitial` → Recent block: heading `t('search.recentTitle')`, `v-for` over `recent` (each row → `useRecent`); Quick block: heading `t('search.quickTitle')`, `v-for` over `quickActions` (each → `runQuick(a.to)`), trailing kbd chip showing `a.key` uppercased.
  - `showResults` → `v-for="(g, gi) in groups"`: group header `{{ t(g.labelKey) }}` + a "See all" button `{{ t('search.seeAll', { n: g.total }) }}`; then `v-for="(it, ii) in g.items"` row → `go(it)`, `@mouseenter="sel = flatIndex(gi, ii)"`, highlight when `flatIndex(gi,ii) === sel` (`bg-primary/10` + `shadow-[inset_3px_0_0_var(--ui-primary)]`), icon chip, title (wrap match highlight — see below), sub in mono, optional status badge via `<StatusBadge>` when `it.status`, and a `↵` chip when selected.
  - `showEmpty` → centered empty state (mockup 166–172): `{{ t('search.emptyTitle', { q: query.trim() }) }}` + `t('search.emptySub')`.
- Footer (177–183): `↑↓ {{ t('search.navHint') }}`, `↵ {{ t('search.openHint') }}`, `Esc {{ t('search.closeHint') }}`, spacer, right `{{ t('search.scopeNote') }}`.

For the match highlight, add a tiny inline helper component or use a computed that splits the title around the query and wraps the match in `<mark class="bg-[var(--hl)] …">`; simplest is a render in template using `v-html` is **disallowed** — instead split into three `<span>`s (pre / match / post) computed per item. Implement an `parts(title)` function in `<script setup>` returning `{ pre, mark, post }`.

- [ ] **Step 2: Add `parts()` to script** (used by the template highlight):

```ts
function parts(title: string): { pre: string, mark: string, post: string } {
  const q = query.value.trim()
  if (!q) return { pre: title, mark: '', post: '' }
  const i = title.toLowerCase().indexOf(q.toLowerCase())
  if (i < 0) return { pre: title, mark: '', post: '' }
  return { pre: title.slice(0, i), mark: title.slice(i, i + q.length), post: title.slice(i + q.length) }
}
```

- [ ] **Step 3: Write the component test** — `frontend/test/nuxt/CommandPalette.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import CommandPalette from '~/components/CommandPalette.vue'
import { useCommandPalette } from '~/composables/useCommandPalette'
import { useAuthStore } from '~/stores/auth'

function admin() {
  useAuthStore().setSession('t', { id: '1', name: 'A', email: 'a@e.com', role_id: 'r', role_name: 'Superadmin' }, ['*'])
}

describe('CommandPalette', () => {
  beforeEach(() => { useAuthStore().clear(); useCommandPalette().close() })

  it('renders nothing when closed', async () => {
    const w = await mountSuspended(CommandPalette)
    expect(w.find('input').exists()).toBe(false)
  })

  it('shows the initial state with quick actions when opened', async () => {
    admin()
    const w = await mountSuspended(CommandPalette)
    useCommandPalette().open()
    await flushPromises()
    expect(w.text()).toContain('Aksi Cepat')
    expect(w.text()).toContain('Tambah Aset')
  })

  it('searches and shows grouped results', async () => {
    admin()
    const w = await mountSuspended(CommandPalette)
    useCommandPalette().open()
    await flushPromises()
    await w.find('input').setValue('latitude')
    await new Promise(r => setTimeout(r, 350))
    await flushPromises()
    expect(w.text()).toContain('Aset')
    expect(w.text()).toContain('Latitude')
  })

  it('shows the empty state when nothing matches', async () => {
    admin()
    const w = await mountSuspended(CommandPalette)
    useCommandPalette().open()
    await flushPromises()
    await w.find('input').setValue('zzzzz-nomatch')
    await new Promise(r => setTimeout(r, 350))
    await flushPromises()
    expect(w.text()).toContain('Tidak ada hasil')
  })
})
```

- [ ] **Step 4: Run the test**

Run: `cd frontend && pnpm test CommandPalette`
Expected: PASS (4 tests). Fix markup/logic until green.

- [ ] **Step 5: Lint + typecheck**

Run: `cd frontend && pnpm lint && pnpm typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/components/CommandPalette.vue frontend/test/nuxt/CommandPalette.spec.ts
git commit -m "feat(search): command palette component with states + keyboard nav"
```

---

### Task A5: Wire trigger + mount palette + e2e

**Files:**
- Modify: `frontend/app/components/GlobalSearch.vue`
- Modify: `frontend/app/layouts/default.vue`
- Test: `frontend/test/nuxt/GlobalSearch.spec.ts`, `frontend/e2e/global-search.spec.ts`

**Interfaces:**
- Consumes: `useCommandPalette` (A3).

- [ ] **Step 1: Convert `GlobalSearch.vue` to a button trigger** — replace the `<input>` with a `<button @click="open">` keeping the search icon, the placeholder text `{{ $t('search.topbarPlaceholder') }}` (left-aligned muted), and the ⌘K badge. Script adds:

```vue
<script setup lang="ts">
const { open } = useCommandPalette()
</script>
```

Template (mockup lines 62–68): a `<button>` styled like the old input box (`w-full max-w-[420px] bg-muted rounded-[10px] py-2 px-3 flex items-center gap-2.5 cursor-text hover:border-strong`), `<UIcon name="i-lucide-search" class="size-4">`, a flex-1 left-aligned `<span class="text-dimmed text-[13.5px] truncate">{{ $t('search.topbarPlaceholder') }}</span>`, and the ⌘K `<span>` chip. Keep the outer `hidden md:flex` wrapper.

- [ ] **Step 2: Mount the palette globally** — in `frontend/app/layouts/default.vue`, add `<CommandPalette />` next to `<ConfirmDialog />`:

```vue
    <ConfirmDialog />
    <CommandPalette />
```

- [ ] **Step 3: Write component test** — `frontend/test/nuxt/GlobalSearch.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import GlobalSearch from '~/components/GlobalSearch.vue'
import { useCommandPalette } from '~/composables/useCommandPalette'

describe('GlobalSearch trigger', () => {
  beforeEach(() => useCommandPalette().close())

  it('opens the palette when the trigger is clicked', async () => {
    const w = await mountSuspended(GlobalSearch)
    expect(useCommandPalette().isOpen.value).toBe(false)
    await w.find('button').trigger('click')
    expect(useCommandPalette().isOpen.value).toBe(true)
  })

  it('shows the topbar placeholder and ⌘K hint', async () => {
    const w = await mountSuspended(GlobalSearch)
    expect(w.text()).toContain('⌘K')
  })
})
```

- [ ] **Step 4: Run component test**

Run: `cd frontend && pnpm test GlobalSearch`
Expected: PASS (2 tests).

- [ ] **Step 5: Write e2e** — `frontend/e2e/global-search.spec.ts`:

```ts
import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('opens the command palette and searches', async ({ page }) => {
  await login(page)
  await page.getByRole('button', { name: /⌘K|Cari aset/ }).first().click()
  const input = page.getByPlaceholder(/Cari aset, pegawai/)
  await expect(input).toBeVisible()
  await input.fill('latitude')
  await expect(page.getByText('Aset', { exact: false })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(input).toBeHidden()
})

test('toggles the palette with the keyboard shortcut', async ({ page }) => {
  await login(page)
  await page.keyboard.press('ControlOrMeta+k')
  await expect(page.getByPlaceholder(/Cari aset, pegawai/)).toBeVisible()
})
```

- [ ] **Step 6: Lint + typecheck + unit suite**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test`
Expected: PASS.

- [ ] **Step 7: Side-by-side check** — open the app, trigger ⌘K, and compare against `docs/design/Global Search.dc.html` in light + dark: initial, loading, results, empty, footer. Fix any gap.

- [ ] **Step 8: Commit**

```bash
git add frontend/app/components/GlobalSearch.vue frontend/app/layouts/default.vue frontend/test/nuxt/GlobalSearch.spec.ts frontend/e2e/global-search.spec.ts
git commit -m "feat(search): wire topbar trigger + ⌘K, mount palette, e2e"
```

---

# Part B — Office Location Map (Leaflet)

Mockup: `docs/design/Peta Lokasi.dc.html`. Layout: left list panel (104–130), right map panel header/legend (134–141), map area (143–220), zoom controls (193–197), reset (198), detail card (201–218). Office dataset & jenis meta: lines 254–271.

### Task B1: Add Leaflet dependency + global CSS

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/nuxt.config.ts`

- [ ] **Step 1: Install Leaflet**

Run: `cd frontend && pnpm add leaflet && pnpm add -D @types/leaflet`
Expected: both added to `package.json`.

- [ ] **Step 2: Register Leaflet CSS** — in `frontend/nuxt.config.ts`, add to the `css` array (create it if absent) `'leaflet/dist/leaflet.css'`. Example:

```ts
  css: ['leaflet/dist/leaflet.css'],
```

- [ ] **Step 3: Verify build resolves the import**

Run: `cd frontend && pnpm typecheck`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/package.json frontend/pnpm-lock.yaml frontend/nuxt.config.ts
git commit -m "build(map): add leaflet dependency + global css"
```

---

### Task B2: Map office dataset, jenis meta, types, i18n

**Files:**
- Modify: `frontend/app/types/index.ts` (append)
- Create: `frontend/app/mock/officeMap.ts`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`

**Interfaces:**
- Produces: `MapOffice`, `OfficeJenis` types; `mapOffices: MapOffice[]`; `jenisMeta` (label key + color token + pin/legend classes); `map.*` i18n namespace.

- [ ] **Step 1: Add types** — append to `frontend/app/types/index.ts`:

```ts
export type OfficeJenis = 'Pusat' | 'Wilayah' | 'Cabang' | 'Outlet'

export interface MapOffice {
  id: string
  nama: string
  kode: string
  jenis: OfficeJenis
  kota: string
  prov: string
  alamat: string
  aset: number
  lat: number
  lng: number
}
```

- [ ] **Step 2: Create the dataset** — `frontend/app/mock/officeMap.ts` (real Jabodetabek lat/lng for the mockup's 9 offices):

```ts
import type { MapOffice, OfficeJenis } from '~/types'

/**
 * Jenis → i18n label key, semantic color token, and pin/legend Tailwind classes.
 * Colors map the mockup's pins to semantic tokens:
 *   Pusat→primary, Wilayah→info, Cabang→warning, Outlet→neutral.
 */
export const jenisMeta: Record<OfficeJenis, {
  labelKey: string
  color: string // hex resolved from CSS var at runtime for the Leaflet divIcon
  pinVar: string
  softBg: string
  softText: string
  icon: string
}> = {
  Pusat: { labelKey: 'map.jenis.pusat', color: '', pinVar: '--pin-pusat', softBg: 'bg-primary/10', softText: 'text-primary', icon: 'i-lucide-landmark' },
  Wilayah: { labelKey: 'map.jenis.wilayah', color: '', pinVar: '--pin-wilayah', softBg: 'bg-info/10', softText: 'text-info', icon: 'i-lucide-building-2' },
  Cabang: { labelKey: 'map.jenis.cabang', color: '', pinVar: '--pin-cabang', softBg: 'bg-warning/10', softText: 'text-warning', icon: 'i-lucide-building' },
  Outlet: { labelKey: 'map.jenis.outlet', color: '', pinVar: '--pin-outlet', softBg: 'bg-neutral/10', softText: 'text-dimmed', icon: 'i-lucide-store' }
}

export const JENIS_ORDER: OfficeJenis[] = ['Pusat', 'Wilayah', 'Cabang', 'Outlet']

export const mapOffices: MapOffice[] = [
  { id: 'o1', nama: 'Kantor Pusat', kode: 'PST', jenis: 'Pusat', kota: 'Jakarta Pusat', prov: 'DKI Jakarta', alamat: 'Jl. Medan Merdeka Barat No. 1, Jakarta Pusat', aset: 94, lat: -6.1754, lng: 106.8272 },
  { id: 'o2', nama: 'Kanwil DKI Jakarta', kode: 'KW-DKI', jenis: 'Wilayah', kota: 'Jakarta Pusat', prov: 'DKI Jakarta', alamat: 'Jl. Jend. Sudirman Kav. 5, Jakarta Pusat', aset: 56, lat: -6.2088, lng: 106.8200 },
  { id: 'o3', nama: 'Cabang Jakarta Selatan', kode: 'JKT01', jenis: 'Cabang', kota: 'Jakarta Selatan', prov: 'DKI Jakarta', alamat: 'Jl. TB Simatupang No. 22, Jakarta Selatan', aset: 96, lat: -6.2920, lng: 106.8000 },
  { id: 'o4', nama: 'Cabang Jakarta Pusat', kode: 'JKT02', jenis: 'Cabang', kota: 'Jakarta Pusat', prov: 'DKI Jakarta', alamat: 'Jl. M.H. Thamrin No. 10, Jakarta Pusat', aset: 112, lat: -6.1944, lng: 106.8229 },
  { id: 'o5', nama: 'Outlet Blok M', kode: 'JKT01-BM', jenis: 'Outlet', kota: 'Jakarta Selatan', prov: 'DKI Jakarta', alamat: 'Blok M Square Lt. 2, Jakarta Selatan', aset: 28, lat: -6.2443, lng: 106.7992 },
  { id: 'o6', nama: 'Outlet Kemang', kode: 'JKT01-KM', jenis: 'Outlet', kota: 'Jakarta Selatan', prov: 'DKI Jakarta', alamat: 'Jl. Kemang Raya No. 8, Jakarta Selatan', aset: 19, lat: -6.2601, lng: 106.8140 },
  { id: 'o7', nama: 'Cabang Bekasi', kode: 'BKS01', jenis: 'Cabang', kota: 'Bekasi', prov: 'Jawa Barat', alamat: 'Jl. Ahmad Yani No. 1, Bekasi', aset: 64, lat: -6.2383, lng: 106.9756 },
  { id: 'o8', nama: 'Cabang Tangerang', kode: 'TGR01', jenis: 'Cabang', kota: 'Tangerang', prov: 'Banten', alamat: 'Jl. Jend. Sudirman No. 3, Tangerang', aset: 48, lat: -6.1783, lng: 106.6319 },
  { id: 'o9', nama: 'Outlet Depok', kode: 'DPK01', jenis: 'Outlet', kota: 'Depok', prov: 'Jawa Barat', alamat: 'Jl. Margonda Raya No. 100, Depok', aset: 22, lat: -6.3833, lng: 106.8167 }
]
```

- [ ] **Step 3: Add `--pin-*` CSS vars** — these tokens come from the mockup (lines 35, 48). Add them to the global stylesheet used by the app (find `frontend/app/assets/css/main.css` or the file referenced in `nuxt.config.ts` `css`). Add, inside `:root` and the dark selector respectively:

```css
:root { --pin-pusat:#16a34a; --pin-wilayah:#2563eb; --pin-cabang:#d97706; --pin-outlet:#64748b; }
.dark { --pin-pusat:#22c55e; --pin-wilayah:#3b82f6; --pin-cabang:#f59e0b; --pin-outlet:#94a3b8; }
```

(If the project's dark selector is `[data-theme="dark"]` or `html.dark`, match the existing convention used by the other tokens in that file.)

- [ ] **Step 4: Add i18n** — add a top-level `"map"` object to `id.json`:

```json
  "map": {
    "title": "Peta Lokasi Kantor",
    "breadcrumb": "Peta Lokasi",
    "usageNote": "Provinsi & Kota dikelola di Referensi; titik lokasi kantor diatur pada form kantor (Master Data Kantor).",
    "searchPlaceholder": "Cari kantor / kode…",
    "jenisAll": "Semua Jenis",
    "provAll": "Semua Provinsi",
    "emptyListTitle": "Tidak ada kantor",
    "emptyListSub": "Tidak ada kantor cocok dengan filter.",
    "emptyMapTitle": "Tidak ada titik untuk ditampilkan",
    "emptyMapSub": "Sesuaikan filter untuk melihat pin kantor.",
    "resetLabel": "Reset Tampilan",
    "resetTip": "Pusatkan & reset zoom",
    "viewOffice": "Lihat Kantor",
    "openMaps": "Buka di Maps",
    "registeredAssets": "aset terdaftar",
    "summary": "{o} kantor · {k} kota · {p} provinsi",
    "jenis": { "pusat": "Pusat", "wilayah": "Wilayah", "cabang": "Cabang", "outlet": "Outlet" }
  },
```

- [ ] **Step 5: Add English** — mirror in `en.json`: title "Office Location Map", breadcrumb "Location Map", usageNote "Provinces & Cities are managed in Reference; office coordinates are set on the office form (Office Master Data).", searchPlaceholder "Search office / code…", jenisAll "All Types", provAll "All Provinces", emptyListTitle "No offices", emptyListSub "No office matches the filter.", emptyMapTitle "No points to display", emptyMapSub "Adjust the filter to see office pins.", resetLabel "Reset View", resetTip "Recenter & reset zoom", viewOffice "View Office", openMaps "Open in Maps", registeredAssets "registered assets", summary "{o} offices · {k} cities · {p} provinces", jenis → "HQ"/"Regional"/"Branch"/"Outlet".

- [ ] **Step 6: Typecheck + commit**

Run: `cd frontend && pnpm typecheck`
Expected: PASS.

```bash
git add frontend/app/types/index.ts frontend/app/mock/officeMap.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/app/assets/css/main.css
git commit -m "feat(map): office dataset, jenis meta, pin tokens, i18n"
```

---

### Task B3: `useOfficeMap` composable + maps URL util

**Files:**
- Create: `frontend/app/composables/api/useOfficeMap.ts`
- Create: `frontend/app/utils/googleMapsUrl.ts`
- Test: `frontend/test/nuxt/useOfficeMap.spec.ts`, `frontend/test/unit/googleMapsUrl.spec.ts`

**Interfaces:**
- Produces: `useOfficeMap()` → `{ list(): Promise<MapOffice[]> }`; `googleMapsUrl(lat: number, lng: number): string`.

- [ ] **Step 1: Write the util test** — `frontend/test/unit/googleMapsUrl.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { googleMapsUrl } from '~/utils/googleMapsUrl'

describe('googleMapsUrl', () => {
  it('builds a maps search URL from lat/lng', () => {
    expect(googleMapsUrl(-6.1754, 106.8272)).toBe('https://www.google.com/maps/search/?api=1&query=-6.1754%2C106.8272')
  })
})
```

- [ ] **Step 2: Run to fail**

Run: `cd frontend && pnpm test googleMapsUrl`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement util** — `frontend/app/utils/googleMapsUrl.ts`:

```ts
export function googleMapsUrl(lat: number, lng: number): string {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(`${lat},${lng}`)}`
}
```

- [ ] **Step 4: Write the composable test** — `frontend/test/nuxt/useOfficeMap.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { useOfficeMap } from '~/composables/api/useOfficeMap'

describe('useOfficeMap', () => {
  it('lists the 9 mockup offices with coordinates', async () => {
    const rows = await useOfficeMap().list()
    expect(rows).toHaveLength(9)
    expect(rows.every(r => typeof r.lat === 'number' && typeof r.lng === 'number')).toBe(true)
    expect(rows.map(r => r.jenis)).toContain('Pusat')
  })
})
```

- [ ] **Step 5: Implement composable** — `frontend/app/composables/api/useOfficeMap.ts`:

```ts
import type { MapOffice } from '~/types'
import { fakeLatency } from '~/mock/helpers'
import { mapOffices } from '~/mock/officeMap'

export function useOfficeMap() {
  async function list(): Promise<MapOffice[]> {
    await fakeLatency(500)
    return mapOffices
  }
  return { list }
}
```

- [ ] **Step 6: Run both tests**

Run: `cd frontend && pnpm test googleMapsUrl useOfficeMap`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/app/composables/api/useOfficeMap.ts frontend/app/utils/googleMapsUrl.ts frontend/test/nuxt/useOfficeMap.spec.ts frontend/test/unit/googleMapsUrl.spec.ts
git commit -m "feat(map): office-map composable + google maps url util"
```

---

### Task B4: `OfficeMap.client.vue` Leaflet component

**Files:**
- Create: `frontend/app/components/OfficeMap.client.vue`

**Interfaces:**
- Consumes: `MapOffice` (B2), `jenisMeta` (B2), `leaflet`.
- Produces: component with props `{ offices: MapOffice[], selectedId: string | null }` and emits `{ (e: 'select', id: string): void }`. Builds colored `divIcon` teardrop markers; `flyTo` on `selectedId` change; exposes `resetView()` and `zoomIn()/zoomOut()` via `defineExpose` for the page's controls.

- [ ] **Step 1: Write the component** — `frontend/app/components/OfficeMap.client.vue`. `.client.vue` ⇒ client-only (no SSR; SPA already, but this guards the Leaflet import). Full script:

```vue
<script setup lang="ts">
import L from 'leaflet'
import type { MapOffice } from '~/types'
import { jenisMeta } from '~/mock/officeMap'

const props = defineProps<{ offices: MapOffice[], selectedId: string | null }>()
const emit = defineEmits<{ (e: 'select', id: string): void }>()

const el = ref<HTMLElement | null>(null)
let map: L.Map | null = null
let markers = new Map<string, L.Marker>()

function pinHtml(o: MapOffice, selected: boolean): string {
  const color = `var(${jenisMeta[o.jenis].pinVar})`
  const size = selected ? 34 : 27
  return `<div style="position:relative;display:flex;flex-direction:column;align-items:center;">
    <div style="display:flex;align-items:center;justify-content:center;width:${size}px;height:${size}px;border-radius:50% 50% 50% 0;background:${color};transform:rotate(-45deg);box-shadow:0 3px 8px rgba(0,0,0,.3);border:2px solid var(--ui-bg);"></div>
  </div>`
}

function icon(o: MapOffice, selected: boolean): L.DivIcon {
  const size = selected ? 34 : 27
  return L.divIcon({ html: pinHtml(o, selected), className: 'office-pin', iconSize: [size, size], iconAnchor: [size / 2, size] })
}

function render() {
  if (!map) return
  for (const m of markers.values()) m.remove()
  markers = new Map()
  for (const o of props.offices) {
    const selected = o.id === props.selectedId
    const m = L.marker([o.lat, o.lng], { icon: icon(o, selected), zIndexOffset: selected ? 1000 : 0 })
    m.on('click', () => emit('select', o.id))
    m.addTo(map)
    markers.set(o.id, m)
  }
}

function fitAll() {
  if (!map || props.offices.length === 0) return
  map.fitBounds(L.latLngBounds(props.offices.map(o => [o.lat, o.lng])), { padding: [48, 48] })
}

function resetView() { fitAll() }
function zoomIn() { map?.zoomIn() }
function zoomOut() { map?.zoomOut() }
defineExpose({ resetView, zoomIn, zoomOut })

onMounted(() => {
  if (!el.value) return
  map = L.map(el.value, { zoomControl: false, attributionControl: true })
  L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', { maxZoom: 19, attribution: '© OpenStreetMap' }).addTo(map)
  render()
  fitAll()
})

onUnmounted(() => { map?.remove(); map = null })

watch(() => props.offices, () => { render(); fitAll() }, { deep: true })
watch(() => props.selectedId, (id) => {
  render()
  const o = props.offices.find(x => x.id === id)
  if (o && map) map.flyTo([o.lat, o.lng], Math.max(map.getZoom(), 12), { duration: 0.5 })
})
</script>

<template>
  <div
    ref="el"
    class="absolute inset-0"
  />
</template>
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend && pnpm typecheck`
Expected: PASS. (Leaflet is browser-only; the `.client.vue` suffix prevents SSR evaluation. No unit test for the Leaflet component itself — it is exercised via the page test with the component stubbed and via e2e.)

- [ ] **Step 3: Commit**

```bash
git add frontend/app/components/OfficeMap.client.vue
git commit -m "feat(map): leaflet office-map client component with colored pins"
```

---

### Task B5: Map page + nav entry + tests

**Files:**
- Create: `frontend/app/pages/master/map.vue`
- Modify: `frontend/app/utils/nav.ts`
- Modify: `frontend/i18n/locales/id.json`, `en.json` (nav key)
- Test: `frontend/test/nuxt/master-map.spec.ts`, `frontend/e2e/office-map.spec.ts`

**Interfaces:**
- Consumes: `useOfficeMap` (B3), `jenisMeta`/`JENIS_ORDER`/`mapOffices` types (B2), `OfficeMap.client.vue` (B4), `googleMapsUrl` (B3).

- [ ] **Step 1: Update nav** — in `frontend/app/utils/nav.ts`, replace the disabled `geography` child in the Master Data group:

```ts
          {
            labelKey: 'nav.officeMap',
            to: '/master/map'
          },
```

(Place it where `{ labelKey: 'nav.geography', disabled: true }` was, keeping order offices → officeMap → reference.)

- [ ] **Step 2: Add nav i18n key** — add `"officeMap": "Peta Lokasi"` to the `nav` object in `id.json` and `"officeMap": "Location Map"` in `en.json` (next to the existing `geography` key, which can remain unused or be removed).

- [ ] **Step 3: Build the page** — `frontend/app/pages/master/map.vue`. Reproduce the mockup layout (lines 95–224). Full script:

```vue
<script setup lang="ts">
import type { MapOffice, OfficeJenis } from '~/types'
import { jenisMeta, JENIS_ORDER } from '~/mock/officeMap'
import { googleMapsUrl } from '~/utils/googleMapsUrl'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const { list } = useOfficeMap()

const all = ref<MapOffice[]>([])
const loading = ref(true)
const q = ref('')
const fJenis = ref<'all' | OfficeJenis>('all')
const fProv = ref<'all' | string>('all')
const selId = ref<string | null>(null)
const mapRef = ref<{ resetView(): void, zoomIn(): void, zoomOut(): void } | null>(null)

onMounted(async () => { all.value = await list(); loading.value = false })

const provinces = computed(() => Array.from(new Set(all.value.map(o => o.prov))))
const filtered = computed(() => all.value.filter((o) => {
  const term = q.value.trim().toLowerCase()
  if (term && !(o.nama.toLowerCase().includes(term) || o.kode.toLowerCase().includes(term))) return false
  if (fJenis.value !== 'all' && o.jenis !== fJenis.value) return false
  if (fProv.value !== 'all' && o.prov !== fProv.value) return false
  return true
}))
const cities = computed(() => new Set(filtered.value.map(o => o.kota)).size)
const provShown = computed(() => new Set(filtered.value.map(o => o.prov)).size)
const summary = computed(() => t('map.summary', { o: filtered.value.length, k: cities.value, p: provShown.value }))
const selected = computed(() => filtered.value.find(o => o.id === selId.value) ?? null)

const legend = computed(() => JENIS_ORDER.map(j => ({ jenis: j, label: t(jenisMeta[j].labelKey), pinVar: jenisMeta[j].pinVar })))
const jenisOptions = computed(() => [{ value: 'all', label: t('map.jenisAll') }, ...JENIS_ORDER.map(j => ({ value: j, label: t(jenisMeta[j].labelKey) }))])
const provOptions = computed(() => [{ value: 'all', label: t('map.provAll') }, ...provinces.value.map(p => ({ value: p, label: p }))])

function selectOffice(o: MapOffice) { selId.value = o.id }
function clearSel() { selId.value = null }
function resetView() { selId.value = null; mapRef.value?.resetView() }
watch([fJenis, fProv], () => { selId.value = null })

function openMaps(o: MapOffice) { window.open(googleMapsUrl(o.lat, o.lng), '_blank', 'noopener') }
function viewOffice() { navigateTo('/master/offices') }
</script>
```

Build the `<template>` faithful to mockup 95–224 using `U*` + tokens:
- `PageHeader`-style title `{{ t('map.title') }}` + the usage note row (mockup 96–99).
- Two-column flex `flex-1 gap-4 min-h-0`:
  - **Left list panel** `w-[312px]`: search `<UInput v-model="q" :placeholder="t('map.searchPlaceholder')">` with leading search icon; two `<USelect>`s bound to `fJenis`/`fProv` with `jenisOptions`/`provOptions`; scrollable list — when `loading` show 5 shimmer rows; else `v-for="o in filtered"` row (mockup 117–125): a colored pin chip (`:style="{ background: 'var(' + jenisMeta[o.jenis].pinVar + ')' }"` glyph or the `jenisMeta[o.jenis].softBg`/`softText` chip), name, jenis badge `{{ t(jenisMeta[o.jenis].labelKey) }}`, mono code, `{{ o.kota }}, {{ o.prov }}`; row `@click="selectOffice(o)"`, selected → primary ring/bg. When `filtered.length === 0` render the empty state (mockup 127) with `t('map.emptyListTitle')` / `Sub`.
  - **Right map panel** `flex-1`: header with summary strip `{{ summary }}` + legend (`v-for="l in legend"` → dot `:style="{ background: 'var(' + l.pinVar + ')' }"` + `l.label`). Map area `relative flex-1 overflow-hidden`:
    - `<ClientOnly>` → `<OfficeMap ref="mapRef" :offices="filtered" :selected-id="selId" @select="(id) => selId = id" />`.
    - When `loading`, overlay a shimmer block.
    - Zoom controls (top-right): two buttons → `mapRef?.zoomIn()` / `zoomOut()`.
    - Reset button (bottom-right) → `resetView()`, label `{{ t('map.resetLabel') }}`.
    - When `filtered.length === 0`, the empty map overlay (mockup 188–190) with `t('map.emptyMapTitle')`/`Sub`.
    - **Detail card** (mockup 201–218) when `selected`: pin chip + `selected.nama` + jenis badge + mono `selected.kode`; close button → `clearSel()`; address row (`selected.alamat`), city·prov row, an assets pill `{{ selected.aset }} {{ t('map.registeredAssets') }}`; footer two buttons: primary `{{ t('map.viewOffice') }}` → `viewOffice()`, secondary `{{ t('map.openMaps') }}` → `openMaps(selected)`.

- [ ] **Step 4: Write the page test** — `frontend/test/nuxt/master-map.spec.ts` (stub the client-only Leaflet component):

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import MapPage from '~/pages/master/map.vue'
import { useAuthStore } from '~/stores/auth'

const stubs = { OfficeMap: { template: '<div class="office-map-stub" />' } }

function admin() {
  useAuthStore().setSession('t', { id: '1', name: 'A', email: 'a@e.com', role_id: 'r', role_name: 'Superadmin' }, ['*'])
}

describe('Office map page', () => {
  beforeEach(() => { useAuthStore().clear(); admin() })

  it('renders office rows after load', async () => {
    const w = await mountSuspended(MapPage, { global: { stubs } })
    await new Promise(r => setTimeout(r, 600))
    await flushPromises()
    expect(w.text()).toContain('Kantor Pusat')
    expect(w.text()).toContain('Cabang Jakarta Selatan')
  })

  it('filters the list by search query', async () => {
    const w = await mountSuspended(MapPage, { global: { stubs } })
    await new Promise(r => setTimeout(r, 600))
    await flushPromises()
    await w.find('input').setValue('bekasi')
    await flushPromises()
    expect(w.text()).toContain('Cabang Bekasi')
    expect(w.text()).not.toContain('Kantor Pusat')
  })

  it('opens the detail card when a row is selected', async () => {
    const w = await mountSuspended(MapPage, { global: { stubs } })
    await new Promise(r => setTimeout(r, 600))
    await flushPromises()
    const rows = w.findAll('button')
    const row = rows.find(b => b.text().includes('Kantor Pusat'))
    await row!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('aset terdaftar')
    expect(w.text()).toContain('Lihat Kantor')
  })

  it('shows the empty state when nothing matches', async () => {
    const w = await mountSuspended(MapPage, { global: { stubs } })
    await new Promise(r => setTimeout(r, 600))
    await flushPromises()
    await w.find('input').setValue('zzz-nomatch')
    await flushPromises()
    expect(w.text()).toContain('Tidak ada kantor')
  })
})
```

- [ ] **Step 5: Run page test**

Run: `cd frontend && pnpm test master-map`
Expected: PASS (4 tests). Adjust selectors/markup until green.

- [ ] **Step 6: Write e2e** — `frontend/e2e/office-map.spec.ts`:

```ts
import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('office map: filter and select an office', async ({ page }) => {
  await login(page)
  await page.goto('/master/map')
  await expect(page.getByText('Kantor Pusat')).toBeVisible()
  await page.getByPlaceholder(/Cari kantor/).fill('bekasi')
  await expect(page.getByText('Cabang Bekasi')).toBeVisible()
  await expect(page.getByText('Kantor Pusat')).toBeHidden()
})
```

- [ ] **Step 7: Lint + typecheck + unit suite**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test`
Expected: PASS.

- [ ] **Step 8: Side-by-side check** — open `/master/map` and compare to `docs/design/Peta Lokasi.dc.html` in light + dark: list rows, filters, summary strip, legend, detail card, zoom/reset controls, empty states. (Leaflet tiles replace the illustrative SVG by design.) Fix gaps.

- [ ] **Step 9: Commit**

```bash
git add frontend/app/pages/master/map.vue frontend/app/utils/nav.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/nuxt/master-map.spec.ts frontend/e2e/office-map.spec.ts
git commit -m "feat(map): office location map page + nav entry + tests"
```

---

# Part C — Account Profile (Profil Akun)

Mockup: `docs/design/Profil Akun.dc.html`. Header (108–120), tabs (122–127), Profil tab (130–183), Keamanan tab (186–240), Preferensi tab (243–278), toast (286–296). Strings: lines 334–367. Strength/logic: 393–398, 411–427.

### Task C1: `passwordStrength` util

**Files:**
- Create: `frontend/app/utils/passwordStrength.ts`
- Test: `frontend/test/unit/passwordStrength.spec.ts`

**Interfaces:**
- Produces: `passwordStrength(pw: string): { score: 0|1|2|3|4, labelKey: string }`. `score` 0–4 from length≥8 / upper+lower / digit / symbol. `labelKey` = `''` for score 0, else `account.strength.{weak|fair|strong|veryStrong}` mapped from score 1→4.

- [ ] **Step 1: Write the test** — `frontend/test/unit/passwordStrength.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { passwordStrength } from '~/utils/passwordStrength'

describe('passwordStrength', () => {
  it('scores 0 for empty', () => {
    expect(passwordStrength('')).toEqual({ score: 0, labelKey: '' })
  })
  it('scores low for a short simple password', () => {
    expect(passwordStrength('abc').score).toBe(0)
  })
  it('rewards length, case mix, digit, and symbol', () => {
    expect(passwordStrength('abcdefgh').score).toBe(1)
    expect(passwordStrength('Abcdefgh').score).toBe(2)
    expect(passwordStrength('Abcdefg1').score).toBe(3)
    expect(passwordStrength('Abcdefg1!').score).toBe(4)
  })
  it('maps score to a label key', () => {
    expect(passwordStrength('Abcdefg1!').labelKey).toBe('account.strength.veryStrong')
    expect(passwordStrength('abcdefgh').labelKey).toBe('account.strength.weak')
  })
})
```

- [ ] **Step 2: Run to fail**

Run: `cd frontend && pnpm test passwordStrength`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement** — `frontend/app/utils/passwordStrength.ts`:

```ts
const LABELS = ['', 'account.strength.weak', 'account.strength.fair', 'account.strength.strong', 'account.strength.veryStrong']

export function passwordStrength(pw: string): { score: 0 | 1 | 2 | 3 | 4, labelKey: string } {
  let s = 0
  if (pw.length >= 8) s++
  if (/[A-Z]/.test(pw) && /[a-z]/.test(pw)) s++
  if (/\d/.test(pw)) s++
  if (/[^A-Za-z0-9]/.test(pw)) s++
  const score = Math.min(s, 4) as 0 | 1 | 2 | 3 | 4
  return { score, labelKey: LABELS[score]! }
}
```

- [ ] **Step 4: Run to pass**

Run: `cd frontend && pnpm test passwordStrength`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/app/utils/passwordStrength.ts frontend/test/unit/passwordStrength.spec.ts
git commit -m "feat(account): password strength utility"
```

---

### Task C2: Account types + `useAccount` composable + i18n

**Files:**
- Modify: `frontend/app/types/index.ts` (append)
- Create: `frontend/app/composables/api/useAccount.ts`
- Modify: `frontend/i18n/locales/id.json`, `en.json`
- Test: `frontend/test/nuxt/useAccount.spec.ts`

**Interfaces:**
- Produces: types `AccountProfile`, `AccountSession`, `NotifPrefs`; `useAccount()` → `{ getProfile(): Promise<AccountProfile>, updateProfile(input): Promise<void>, changePassword(input): Promise<void>, listSessions(): Promise<AccountSession[]>, revokeSession(id): Promise<void>, logoutAllOthers(): Promise<void>, getNotifPrefs(): NotifPrefs, setNotifPrefs(p): void }`. `changePassword` throws `Error('account.errConfirmMismatch')` when confirm ≠ new, and `Error('account.errRequired')` when a field is blank.

- [ ] **Step 1: Add types** — append to `frontend/app/types/index.ts`:

```ts
export interface AccountProfile {
  nama: string
  email: string
  telepon: string
  peran: string
  kantor: string
  pegawai: string
  loginMethod: 'email' | 'google'
  joinDate: string
}

export interface AccountSession {
  id: string
  device: string
  meta: string
  icon: string
  current: boolean
}

export interface NotifPrefs {
  approval: boolean
  maint: boolean
  assign: boolean
}
```

- [ ] **Step 2: Write the composable test** — `frontend/test/nuxt/useAccount.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { useAccount } from '~/composables/api/useAccount'
import { useAuthStore } from '~/stores/auth'

describe('useAccount', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager' }, ['*'])
  })

  it('builds a profile from the auth user merged with mock fields', async () => {
    const p = await useAccount().getProfile()
    expect(p.nama).toBe('Andi Saputra')
    expect(p.email).toBe('andi@inventra.local')
    expect(p.peran).toBe('Asset Manager')
    expect(p.loginMethod).toBe('email')
  })

  it('rejects a password change with mismatched confirmation', async () => {
    await expect(useAccount().changePassword({ oldPass: 'x', newPass: 'Abcdefg1!', confirmPass: 'nope' }))
      .rejects.toThrow('account.errConfirmMismatch')
  })

  it('rejects a password change with a blank field', async () => {
    await expect(useAccount().changePassword({ oldPass: '', newPass: 'Abcdefg1!', confirmPass: 'Abcdefg1!' }))
      .rejects.toThrow('account.errRequired')
  })

  it('lists sessions with exactly one current session', async () => {
    const s = await useAccount().listSessions()
    expect(s.length).toBeGreaterThanOrEqual(1)
    expect(s.filter(x => x.current)).toHaveLength(1)
  })

  it('persists notification preferences', () => {
    const a = useAccount()
    a.setNotifPrefs({ approval: false, maint: true, assign: true })
    expect(a.getNotifPrefs()).toEqual({ approval: false, maint: true, assign: true })
  })
})
```

- [ ] **Step 3: Run to fail**

Run: `cd frontend && pnpm test useAccount`
Expected: FAIL — module not found.

- [ ] **Step 4: Implement composable** — `frontend/app/composables/api/useAccount.ts`:

```ts
import type { AccountProfile, AccountSession, NotifPrefs } from '~/types'
import { fakeLatency } from '~/mock/helpers'

const NOTIF_KEY = 'inventra.account.notif'
const DEFAULT_NOTIF: NotifPrefs = { approval: true, maint: true, assign: false }

export interface ProfileInput { nama: string, telepon: string }
export interface PasswordInput { oldPass: string, newPass: string, confirmPass: string }

export function useAccount() {
  const auth = useAuthStore()

  async function getProfile(): Promise<AccountProfile> {
    await fakeLatency(400)
    return {
      nama: auth.user?.name ?? '',
      email: auth.user?.email ?? '',
      telepon: '0812-3456-7890',
      peran: auth.user?.role_name ?? '',
      kantor: 'Cabang Jakarta Selatan',
      pegawai: auth.user?.name ?? '',
      loginMethod: 'email',
      joinDate: '2024-03-12'
    }
  }

  async function updateProfile(input: ProfileInput): Promise<void> {
    if (!input.nama.trim()) throw new Error('account.errRequired')
    await fakeLatency()
  }

  async function changePassword(input: PasswordInput): Promise<void> {
    if (!input.oldPass || !input.newPass || !input.confirmPass) throw new Error('account.errRequired')
    if (input.newPass !== input.confirmPass) throw new Error('account.errConfirmMismatch')
    await fakeLatency()
  }

  async function listSessions(): Promise<AccountSession[]> {
    await fakeLatency(300)
    return [
      { id: 's1', device: 'Chrome · macOS', meta: 'Jakarta, Indonesia · Sekarang', icon: 'i-lucide-laptop', current: true },
      { id: 's2', device: 'Safari · iPhone 15', meta: 'Jakarta, Indonesia · 2 jam lalu', icon: 'i-lucide-smartphone', current: false },
      { id: 's3', device: 'Edge · Windows 11', meta: 'Bandung, Indonesia · kemarin', icon: 'i-lucide-monitor', current: false }
    ]
  }

  async function revokeSession(_id: string): Promise<void> { await fakeLatency() }
  async function logoutAllOthers(): Promise<void> { await fakeLatency() }

  function getNotifPrefs(): NotifPrefs {
    if (import.meta.client) {
      try {
        const raw = localStorage.getItem(NOTIF_KEY)
        if (raw) return JSON.parse(raw) as NotifPrefs
      } catch { /* ignore */ }
    }
    return { ...DEFAULT_NOTIF }
  }

  function setNotifPrefs(p: NotifPrefs): void {
    if (import.meta.client) {
      try { localStorage.setItem(NOTIF_KEY, JSON.stringify(p)) } catch { /* ignore */ }
    }
  }

  return { getProfile, updateProfile, changePassword, listSessions, revokeSession, logoutAllOthers, getNotifPrefs, setNotifPrefs }
}
```

- [ ] **Step 5: Add i18n** — add a top-level `"account"` object to `id.json` (and English mirror to `en.json`). Indonesian (values from mockup lines 334–350):

```json
  "account": {
    "title": "Profil & Pengaturan Akun",
    "changePhoto": "Ganti foto",
    "tabProfil": "Profil",
    "tabKeamanan": "Keamanan",
    "tabPref": "Preferensi",
    "secFoto": "Foto Profil",
    "upload": "Unggah Foto",
    "remove": "Hapus",
    "fotoHint": "JPG atau PNG, maks. 2 MB.",
    "secDiri": "Data Diri",
    "lNama": "Nama Lengkap",
    "lTelepon": "Telepon",
    "lEmail": "Email",
    "emailLockNote": "Email dikelola oleh akun Google dan tidak dapat diubah.",
    "required": "Wajib diisi.",
    "secAkun": "Informasi Akun",
    "secAkunHint": "Data berikut dikelola oleh administrator dan tidak dapat diubah dari sini.",
    "iPeran": "Peran",
    "iKantor": "Kantor Penempatan",
    "iPegawai": "Pegawai Tertaut",
    "iLogin": "Metode Login",
    "iJoin": "Tanggal Bergabung",
    "loginEmail": "Email",
    "loginGoogle": "Google",
    "save": "Simpan Perubahan",
    "secPassword": "Ganti Password",
    "lOldPass": "Password Lama",
    "lNewPass": "Password Baru",
    "lConfirmPass": "Konfirmasi Password Baru",
    "changePass": "Ganti Password",
    "confirmMismatch": "Konfirmasi password tidak cocok.",
    "googleTitle": "Akun ini masuk via Google",
    "googleNote": "Password tidak dikelola di Inventra. Kelola kata sandi & keamanan di akun Google Anda.",
    "secSesi": "Sesi & Perangkat",
    "logoutAll": "Keluar dari semua perangkat",
    "current": "Sesi ini",
    "revoke": "Keluar",
    "secTampilan": "Tampilan",
    "lBahasa": "Bahasa",
    "lBahasaHint": "Bahasa antarmuka aplikasi.",
    "lTema": "Tema",
    "lTemaHint": "Pilih tampilan terang, gelap, atau ikuti sistem.",
    "themeLight": "Terang",
    "themeDark": "Gelap",
    "themeSystem": "Sistem",
    "secNotif": "Notifikasi",
    "secNotifHint": "Atur pemberitahuan yang Anda terima.",
    "notifApproval": "Keputusan Approval",
    "notifApprovalDesc": "Saat pengajuan Anda disetujui atau ditolak",
    "notifMaint": "Pengingat Maintenance",
    "notifMaintDesc": "Saat aset jatuh tempo perawatan",
    "notifAssign": "Penugasan Aset",
    "notifAssignDesc": "Saat aset di-check-out ke Anda",
    "strength": { "weak": "Lemah", "fair": "Sedang", "strong": "Kuat", "veryStrong": "Sangat Kuat" },
    "toastProfilTitle": "Profil diperbarui",
    "toastProfilMsg": "Perubahan data diri Anda telah disimpan.",
    "toastPassTitle": "Password diganti",
    "toastPassMsg": "Kata sandi Anda berhasil diperbarui.",
    "toastLogoutTitle": "Sesi diakhiri",
    "toastLogoutMsg": "Anda telah keluar dari semua perangkat lain."
  },
```

- [ ] **Step 6: Add English mirror** — same keys in `en.json` using the mockup's English strings (lines 351–366): title "Profile & Account Settings", tabs "Profile"/"Security"/"Preferences", "Profile Photo", "Upload Photo", "Remove", "JPG or PNG, max. 2 MB.", "Personal Data", "Full Name", "Phone", "Email", emailLockNote "Email is managed by your Google account and cannot be changed.", required "Required.", "Account Information", secAkunHint "The following data is managed by an administrator and cannot be changed here.", "Role"/"Assigned Office"/"Linked Employee"/"Login Method"/"Joined Date", "Save Changes", "Change Password"/"Current Password"/"New Password"/"Confirm New Password", confirmMismatch "Password confirmation does not match.", googleTitle "This account signs in via Google", googleNote "Password is not managed in Inventra. Manage your password & security in your Google account.", "Sessions & Devices", "Log out of all devices", "This session", "Log out", "Appearance"/"Language"/…/"Light"/"Dark"/"System", notifications labels (Approval Decisions / Maintenance Reminders / Asset Assignments + descriptions), strength "Weak"/"Fair"/"Strong"/"Very Strong", toast titles/msgs (Profile updated / Password changed / Sessions ended).

- [ ] **Step 7: Run the composable test + typecheck**

Run: `cd frontend && pnpm test useAccount && pnpm typecheck`
Expected: PASS (5 tests + typecheck clean).

- [ ] **Step 8: Commit**

```bash
git add frontend/app/types/index.ts frontend/app/composables/api/useAccount.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/nuxt/useAccount.spec.ts
git commit -m "feat(account): account types, mock composable, i18n namespace"
```

---

### Task C3: `/akun` page — Profil tab + UserMenu wiring

**Files:**
- Create: `frontend/app/pages/akun.vue`
- Modify: `frontend/app/components/UserMenu.vue`
- Test: `frontend/test/nuxt/akun-profil.spec.ts`

**Interfaces:**
- Consumes: `useAccount` (C2), `useToast`, `useI18n`, `useColorMode`, `passwordStrength` (C1), `googleIcon` (inline).

- [ ] **Step 1: Scaffold the page with the Profil tab** — `frontend/app/pages/akun.vue`. Tab state from `?tab=`. Full script (covers all three tabs; tabs C4/C5 add their template blocks):

```vue
<script setup lang="ts">
import type { AccountProfile, AccountSession, NotifPrefs } from '~/types'
import { passwordStrength } from '~/utils/passwordStrength'

const { t, setLocale, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const toast = useToast()
const colorMode = useColorMode()
const account = useAccount()

const tab = ref<'profil' | 'keamanan' | 'pref'>(['profil', 'keamanan', 'pref'].includes(route.query.tab as string) ? route.query.tab as 'profil' | 'keamanan' | 'pref' : 'profil')
watch(tab, t => router.replace({ query: { ...route.query, tab: t } }))

const loading = ref(true)
const profile = ref<AccountProfile | null>(null)
const fNama = ref('')
const fTelepon = ref('')
const nameErr = ref(false)
const isGoogle = computed(() => profile.value?.loginMethod === 'google')

// security
const oldPass = ref('')
const newPass = ref('')
const confirmPass = ref('')
const secErr = reactive<{ old?: boolean, newp?: boolean, confirm?: boolean }>({})
const strength = computed(() => passwordStrength(newPass.value))
const sessions = ref<AccountSession[]>([])

// preferences
const themePref = ref(colorMode.preference)
const notif = ref<NotifPrefs>(account.getNotifPrefs())

onMounted(async () => {
  profile.value = await account.getProfile()
  fNama.value = profile.value.nama
  fTelepon.value = profile.value.telepon
  sessions.value = await account.listSessions()
  loading.value = false
})

async function saveProfil() {
  nameErr.value = false
  try {
    await account.updateProfile({ nama: fNama.value, telepon: fTelepon.value })
    toast.add({ title: t('account.toastProfilTitle'), description: t('account.toastProfilMsg'), color: 'success' })
  } catch {
    nameErr.value = true
  }
}

async function changePassword() {
  secErr.old = !oldPass.value
  secErr.newp = !newPass.value
  secErr.confirm = !confirmPass.value || confirmPass.value !== newPass.value
  if (secErr.old || secErr.newp || secErr.confirm) return
  await account.changePassword({ oldPass: oldPass.value, newPass: newPass.value, confirmPass: confirmPass.value })
  oldPass.value = ''; newPass.value = ''; confirmPass.value = ''
  toast.add({ title: t('account.toastPassTitle'), description: t('account.toastPassMsg'), color: 'success' })
}

async function logoutAll() {
  await account.logoutAllOthers()
  toast.add({ title: t('account.toastLogoutTitle'), description: t('account.toastLogoutMsg'), color: 'success' })
}

function setTheme(pref: 'light' | 'dark' | 'system') { themePref.value = pref; colorMode.preference = pref }
function toggleNotif(k: keyof NotifPrefs) { notif.value = { ...notif.value, [k]: !notif.value[k] }; account.setNotifPrefs(notif.value) }

const initials = computed(() => {
  const n = (profile.value?.nama ?? '').trim().split(/\s+/)
  return ((n[0]?.[0] ?? '') + (n[1]?.[0] ?? '')).toUpperCase() || '?'
})
const joinDateLabel = computed(() => {
  if (!profile.value) return ''
  return new Date(profile.value.joinDate).toLocaleDateString(locale.value === 'en' ? 'en-GB' : 'id-ID', { day: 'numeric', month: 'long', year: 'numeric' })
})
</script>
```

Build the `<template>` for the **Profil tab** faithful to mockup 96–183 within the default layout's `<main>`, constrained to `max-w-[760px] mx-auto`:
- When `loading`, render the skeleton block (mockup 100–104).
- Profile header (108–120): avatar (`initials`), `{{ profile.nama }}`, role badge `{{ profile.peran }}` (warning soft), email + office rows.
- Tabs bar (122–127): three buttons bound to `tab`, active = primary bottom border.
- Profil tab body (`v-if="tab === 'profil'"`, mockup 130–183): photo block (Upload/Remove buttons — visual only), Data Diri form: `<UInput v-model="fNama">` (required; red border + `{{ t('account.required') }}` when `nameErr`), `<UInput v-model="fTelepon">`, email input bound to `profile.email` `:disabled="isGoogle"` with the lock note when `isGoogle`; Info Akun read-only grid (peran/kantor/pegawai/login method with icon/join date via `joinDateLabel`); Save button → `saveProfil()`.

- [ ] **Step 2: Wire UserMenu links** — in `frontend/app/components/UserMenu.vue`, change the two menu buttons' handlers:

```html
            @click="open = false; navigateTo('/akun')"
```
for "Profil Saya", and
```html
            @click="open = false; navigateTo('/akun?tab=pref')"
```
for "Pengaturan Akun".

- [ ] **Step 3: Write the Profil tab test** — `frontend/test/nuxt/akun-profil.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import Akun from '~/pages/akun.vue'
import { useAuthStore } from '~/stores/auth'

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager' }, ['*'])
}

async function mountLoaded() {
  const w = await mountSuspended(Akun)
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Profil tab', () => {
  beforeEach(() => { useAuthStore().clear(); user() })

  it('renders the profile header and personal data', async () => {
    const w = await mountLoaded()
    expect(w.text()).toContain('Andi Saputra')
    expect(w.text()).toContain('Asset Manager')
    expect(w.text()).toContain('Data Diri')
  })

  it('shows the required error when saving with an empty name', async () => {
    const w = await mountLoaded()
    const nameInput = w.findAll('input')[0]!
    await nameInput.setValue('')
    const saveBtn = w.findAll('button').find(b => b.text().includes('Simpan Perubahan'))!
    await saveBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Wajib diisi')
  })
})
```

- [ ] **Step 4: Run the test**

Run: `cd frontend && pnpm test akun-profil`
Expected: PASS (2 tests).

- [ ] **Step 5: Lint + typecheck**

Run: `cd frontend && pnpm lint && pnpm typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/pages/akun.vue frontend/app/components/UserMenu.vue frontend/test/nuxt/akun-profil.spec.ts
git commit -m "feat(account): /akun page profil tab + user menu links"
```

---

### Task C4: Keamanan tab — password form + sessions

**Files:**
- Modify: `frontend/app/pages/akun.vue` (add Keamanan tab template)
- Test: `frontend/test/nuxt/akun-keamanan.spec.ts`

- [ ] **Step 1: Add the Keamanan tab template** (`v-if="tab === 'keamanan'"`, mockup 186–240). Uses the already-defined script state (C3). Build:
  - If `!isGoogle` → Change Password card: three password `<UInput type="password">` bound to `oldPass`/`newPass`/`confirmPass`; required errors when `secErr.old/newp/confirm`; **strength meter** under New Password (`v-if="newPass.length"`): 4 bars colored from `strength.score` (error→warning→primary→primary), label `{{ strength.labelKey ? t(strength.labelKey) : '' }}`; confirm-mismatch error `{{ t('account.confirmMismatch') }}` when `secErr.confirm` and confirm non-empty; "Ganti Password" button → `changePassword()`.
  - If `isGoogle` → the Google info card (mockup 216–224) with `t('account.googleTitle')`/`googleNote`.
  - Sessions card (226–238): header + "Keluar dari semua perangkat" → `logoutAll()`; `v-for="s in sessions"` row (icon, device, `current` badge `{{ t('account.current') }}`, meta, Revoke button on non-current → `account.revokeSession(s.id)` then remove from `sessions`).

- [ ] **Step 2: Write the test** — `frontend/test/nuxt/akun-keamanan.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import Akun from '~/pages/akun.vue'
import { useAuthStore } from '~/stores/auth'

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager' }, ['*'])
}
async function mountLoaded() {
  const w = await mountSuspended(Akun, { props: {} })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Keamanan tab', () => {
  beforeEach(() => { useAuthStore().clear(); user() })

  it('switches to the security tab and shows the password form', async () => {
    const w = await mountLoaded()
    const tabBtn = w.findAll('button').find(b => b.text().trim() === 'Keamanan')!
    await tabBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Ganti Password')
    expect(w.text()).toContain('Sesi & Perangkat')
  })

  it('shows the confirm-mismatch error', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Keamanan')!.trigger('click')
    await flushPromises()
    const pw = w.findAll('input[type="password"]')
    await pw[0]!.setValue('oldpass')
    await pw[1]!.setValue('Abcdefg1!')
    await pw[2]!.setValue('different')
    await w.findAll('button').find(b => b.text().includes('Ganti Password') && b.attributes('class')?.includes('bg-primary'))!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('tidak cocok')
  })

  it('updates the strength meter as the new password is typed', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Keamanan')!.trigger('click')
    await flushPromises()
    const pw = w.findAll('input[type="password"]')
    await pw[1]!.setValue('Abcdefg1!')
    await flushPromises()
    expect(w.text()).toContain('Sangat Kuat')
  })
})
```

- [ ] **Step 3: Run the test** (adjust the mismatch button selector if needed so it targets the submit button, not the tab/section heading)

Run: `cd frontend && pnpm test akun-keamanan`
Expected: PASS (3 tests).

- [ ] **Step 4: Lint + typecheck**

Run: `cd frontend && pnpm lint && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/akun.vue frontend/test/nuxt/akun-keamanan.spec.ts
git commit -m "feat(account): security tab — password change + sessions"
```

---

### Task C5: Preferensi tab — language, theme, notifications + e2e

**Files:**
- Modify: `frontend/app/pages/akun.vue` (add Preferensi tab template)
- Test: `frontend/test/nuxt/akun-pref.spec.ts`, `frontend/e2e/account.spec.ts`

- [ ] **Step 1: Add the Preferensi tab template** (`v-if="tab === 'pref'"`, mockup 243–278):
  - Tampilan card: Language row with two buttons "Indonesia"/"English" → `setLocale('id')`/`setLocale('en')`, active = `locale`; Theme row with three cards Light/Dark/System → `setTheme('light'|'dark'|'system')`, active highlighted when `themePref === key` (icons sun/moon/monitor; labels `t('account.themeLight'|'themeDark'|'themeSystem')`).
  - Notifikasi card: `v-for` over the three notif keys (approval/maint/assign) with icon + label `t('account.notifApproval'…)` + desc + a toggle switch bound to `notif[k]` → `toggleNotif(k)`.

- [ ] **Step 2: Write the test** — `frontend/test/nuxt/akun-pref.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import Akun from '~/pages/akun.vue'
import { useAuthStore } from '~/stores/auth'

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager' }, ['*'])
}
async function mountLoaded() {
  const w = await mountSuspended(Akun)
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Preferensi tab', () => {
  beforeEach(() => { useAuthStore().clear(); user(); localStorage.clear() })

  it('shows appearance + notification sections', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Preferensi')!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tampilan')
    expect(w.text()).toContain('Notifikasi')
    expect(w.text()).toContain('Keputusan Approval')
  })

  it('persists a notification toggle', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Preferensi')!.trigger('click')
    await flushPromises()
    const before = localStorage.getItem('inventra.account.notif')
    // toggle the first notification switch
    const toggles = w.findAll('button').filter(b => b.attributes('class')?.includes('rounded-full') || b.attributes('role') === 'switch')
    await toggles[0]!.trigger('click')
    await flushPromises()
    expect(localStorage.getItem('inventra.account.notif')).not.toBe(before)
  })
})
```

(If the toggle selector is brittle, give each switch a stable `data-testid="notif-<key>"` in the template and select by that.)

- [ ] **Step 3: Run the test**

Run: `cd frontend && pnpm test akun-pref`
Expected: PASS (2 tests).

- [ ] **Step 4: Write the e2e** — `frontend/e2e/account.spec.ts`:

```ts
import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('account: open from user menu and change password validation', async ({ page }) => {
  await login(page)
  await page.goto('/akun')
  await expect(page.getByText('Profil & Pengaturan Akun')).toBeVisible()
  await page.getByRole('button', { name: 'Keamanan' }).click()
  const pw = page.locator('input[type="password"]')
  await pw.nth(0).fill('oldpass')
  await pw.nth(1).fill('Abcdefg1!')
  await pw.nth(2).fill('different')
  await page.getByRole('button', { name: 'Ganti Password' }).last().click()
  await expect(page.getByText('tidak cocok')).toBeVisible()
})

test('account: switch language preference', async ({ page }) => {
  await login(page)
  await page.goto('/akun?tab=pref')
  await page.getByRole('button', { name: 'English' }).click()
  await expect(page.getByText('Appearance')).toBeVisible()
})
```

- [ ] **Step 5: Full verification suite**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: PASS across the board.

- [ ] **Step 6: Side-by-side check** — open `/akun` and compare all three tabs to `docs/design/Profil Akun.dc.html` in light + dark: header, tabs, profil form + lock state, security (password + strength + sessions, and the Google variant), preferences (language/theme/notifications), and the success toast. Fix gaps.

- [ ] **Step 7: Commit**

```bash
git add frontend/app/pages/akun.vue frontend/test/nuxt/akun-pref.spec.ts frontend/e2e/account.spec.ts
git commit -m "feat(account): preferences tab — language, theme, notifications + e2e"
```

---

## Final integration check

- [ ] **Update PROGRESS.md** — add the three screens under the frontend "Done" list (Global Search palette, Office Location Map, Account Profile) and note the new `docs/design` mockups are implemented. Commit:

```bash
git add docs/PROGRESS.md
git commit -m "docs: mark global search, office map, account profile as built"
```

- [ ] **Run the whole frontend gate once more**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all PASS. The e2e job (`pnpm test:e2e`) needs the backend stack + seeded admin; run it if the stack is up.

---

## Self-review notes (coverage map)

- **Global Search** spec bagian 1 → Tasks A1–A5 (types/i18n, aggregator, palette state+recent, component+keyboard+states, trigger+mount+e2e). ✓
- **Office Map** spec bagian 2 → Tasks B1–B5 (leaflet dep, dataset/meta/tokens/i18n, composable+url, Leaflet client component, page+nav+tests). ✓ Leaflet deviation honored; categories follow mockup (Pusat/Wilayah/Cabang/Outlet). ✓
- **Account Profile** spec bagian 3 → Tasks C1–C5 (strength util, types/composable/i18n, profil tab+menu wiring, keamanan tab, preferensi tab+e2e). ✓ Theme via `useColorMode`, language via `useI18n`, identity from `auth.user`, everything else mock. ✓
- Type consistency: `SearchGroup/SearchItem` (A1) used by A2/A4; `useCommandPalette` shape (A3) used by A4/A5; `MapOffice/jenisMeta/JENIS_ORDER` (B2) used by B3/B4/B5; `useOfficeMap.list` (B3) used by B5; `passwordStrength` (C1) used by C3/C4; `AccountProfile/AccountSession/NotifPrefs` + `useAccount` methods (C2) used by C3/C4/C5. ✓
- No placeholders: every code/test step carries real code; component templates reference exact mockup line ranges with explicit token/handler mappings (the mockup is the markup source of truth — not duplicated to stay DRY). ✓
