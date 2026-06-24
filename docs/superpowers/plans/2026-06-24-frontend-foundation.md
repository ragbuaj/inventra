# Frontend Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Inventra frontend foundation — design tokens, real authentication, the app shell, and the global reusable component library — so every feature screen can be built on top of it.

**Architecture:** Nuxt 4 + Nuxt UI v4. A per-module **service layer** is the seam between UI and data: services return backend-shaped payloads (`{ data, total, limit, offset }`); today fixtures back them, later real `$fetch` does. **Auth is real** — wired to the live Go backend `/auth/*` with token storage, auto-refresh on 401, route middleware, and permission-driven menus. Global components live in `app/components/` (auto-imported); module components live in `app/components/<module>/`.

**Tech Stack:** Nuxt 4.4, Nuxt UI 4.9, Pinia, `@nuxtjs/i18n` (id default/en), Tailwind v4 (`@theme`), lucide icons (`i-lucide-*`).

## Global Constraints

- **Build only on Nuxt UI `U*` components.** Never hand-roll buttons/inputs/modals or add another component library.
- **Theme via semantic tokens**, never hardcoded Tailwind colors. Primary = green, neutral = slate. Light + dark mode both work.
- **i18n mandatory** — every user-facing string lives in `i18n/locales/{id,en}.json`, referenced via `$t('key')`/`useI18n()`. Default locale `id`. Routing strategy `prefix_except_default`.
- **API base** comes from `runtimeConfig.public.apiBase` (default `http://localhost:8080/api/v1`) — never hardcode the backend URL.
- **Lint rules:** ESLint stylistic — **no trailing commas** (`commaDangle: 'never'`), 1tbs brace style. `pnpm lint` and `pnpm typecheck` must pass.
- **No frontend test runner exists.** Per-task verification cycle = `pnpm lint` + `pnpm typecheck`; `pnpm build` at milestones (Tasks 7, 13); plus visual verification at `/dev/components`. Do NOT add vitest in this phase (YAGNI). Pure-logic units (formatters, `paginate`, `useCan`) are kept side-effect-free so they're testable later.
- **Commits:** Conventional Commits, lowercase, imperative, scope `feat(frontend):` / `chore(frontend):`. No Co-Authored-By trailers. Work stays on branch `feat/fe-foundation`.
- **Run all commands from `frontend/`.**

---

## File structure (created in this plan)

```
frontend/app/
├── app.vue                         # MODIFY → UApp + NuxtLayout + NuxtPage
├── app.config.ts                   # MODIFY → confirm primary green / neutral slate
├── assets/css/main.css             # MODIFY → Inter font + slate/green token scale
├── types/index.ts                  # shared TS types (Paginated, AuthUser, etc.)
├── utils/format.ts                 # formatRupiah, formatDate
├── utils/statusMeta.ts             # asset/approval status → {color,label,i18nKey}
├── stores/auth.ts                  # token + user + permissions
├── stores/ui.ts                    # sidebar collapsed, (theme via Nuxt color-mode)
├── composables/useApiClient.ts     # $fetch base: Bearer + refresh-on-401 + error toast
├── composables/useAuthApi.ts       # REAL /auth/* calls
├── composables/useCan.ts           # permission check
├── composables/useConfirm.ts       # confirm-dialog state + promise
├── mock/helpers.ts                 # paginate / filterBy / fakeLatency
├── mock/index.ts                   # re-exports (future module fixtures land here)
├── middleware/auth.global.ts       # redirect unauthenticated → /login
├── middleware/can.ts               # named guard from route meta.permission
├── layouts/auth.vue                # login layout
├── layouts/default.vue            # app shell
├── components/                     # GLOBAL components (Tasks 8–12)
│   ├── StatusBadge.vue  EmptyState.vue  TableSkeleton.vue  CardSkeleton.vue
│   ├── StatCard.vue  EntityAvatar.vue  Can.vue  ConfirmDialog.vue
│   ├── PageHeader.vue  DataToolbar.vue  TablePagination.vue  ResourceTable.vue
│   ├── FormSlideover.vue  FormModal.vue  TreeView.vue
│   ├── ThemeToggle.vue  LangSwitcher.vue  NotificationBell.vue
│   ├── UserMenu.vue  GlobalSearch.vue  AppBreadcrumb.vue
│   ├── AppSidebar.vue  AppTopbar.vue
└── pages/
    ├── login.vue                   # real login
    ├── index.vue                   # MODIFY → dashboard placeholder
    └── dev/components.vue          # style-guide / verification page
```

---

## Task 1: Design tokens, fonts & app shell entry

**Files:**
- Modify: `frontend/app/assets/css/main.css`
- Modify: `frontend/app/app.config.ts`
- Modify: `frontend/app/app.vue`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`

**Interfaces:**
- Produces: global token scale (primary green-600 `#16a34a`, neutral slate), Inter font, and an `app.vue` that renders the active layout. i18n root keys `app.*`.

- [ ] **Step 1: Replace `main.css`** with the standard Tailwind green scale (so `primary: green` resolves to `#16a34a` at 600) and Inter:

```css
@import "tailwindcss";
@import "@nuxt/ui";

@theme static {
  --font-sans: 'Inter', ui-sans-serif, system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', ui-monospace, monospace;

  --color-green-50: #f0fdf4;
  --color-green-100: #dcfce7;
  --color-green-200: #bbf7d0;
  --color-green-300: #86efac;
  --color-green-400: #4ade80;
  --color-green-500: #22c55e;
  --color-green-600: #16a34a;
  --color-green-700: #15803d;
  --color-green-800: #166534;
  --color-green-900: #14532d;
  --color-green-950: #052e16;
}
```

- [ ] **Step 2: Load Inter + JetBrains Mono.** In `app.vue` `useHead`, add the Google Fonts links (preconnect + stylesheet) so the `--font-sans`/`--font-mono` resolve.

- [ ] **Step 3: Rewrite `app.vue`** to drive layouts instead of the stock header/footer:

```vue
<script setup>
useHead({
  htmlAttrs: { lang: 'id' },
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap' },
    { rel: 'icon', href: '/favicon.ico' }
  ]
})
useSeoMeta({ title: 'Inventra', description: 'Manajemen aset & inventaris' })
</script>

<template>
  <UApp>
    <NuxtLoadingIndicator />
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </UApp>
</template>
```

- [ ] **Step 4: Confirm `app.config.ts`** is `ui.colors.primary: 'green'`, `neutral: 'slate'` (already set — leave as-is).

- [ ] **Step 5: Seed i18n root keys.** Replace `id.json` / `en.json` with the foundation keys (extended in later tasks):

```json
{
  "app": { "name": "Inventra", "tagline": "Manajemen Aset" },
  "common": {
    "save": "Simpan", "cancel": "Batal", "delete": "Hapus", "edit": "Ubah",
    "add": "Tambah", "search": "Cari", "reset": "Reset", "actions": "Aksi",
    "loading": "Memuat…", "noData": "Belum ada data", "confirm": "Konfirmasi"
  }
}
```
(en.json mirrors with English values.)

- [ ] **Step 6: Verify**

Run: `pnpm lint && pnpm typecheck`
Expected: PASS (no errors).

- [ ] **Step 7: Commit**

```bash
git add app/assets/css/main.css app/app.vue app/app.config.ts i18n/locales
git commit -m "feat(frontend): set design tokens, fonts, and layout-driven app shell"
```

---

## Task 2: Shared types + formatters + status metadata

**Files:**
- Create: `frontend/app/types/index.ts`
- Create: `frontend/app/utils/format.ts`
- Create: `frontend/app/utils/statusMeta.ts`

**Interfaces:**
- Produces:
  - `Paginated<T> = { data: T[]; total: number; limit: number; offset: number }`
  - `ListQuery = { search?: string; limit?: number; offset?: number; [k: string]: unknown }`
  - `AuthUser = { id: string; name: string; email: string; role_id: string; role_name: string }`
  - `formatRupiah(value: string | number | null): string`
  - `formatDate(iso: string | null, opts?: { withTime?: boolean }): string`
  - `assetStatusMeta: Record<string, { color: BadgeColor; labelKey: string }>` and `approvalStatusMeta` (same shape); `BadgeColor = 'primary'|'success'|'warning'|'error'|'neutral'|'info'`.

- [ ] **Step 1: Write `types/index.ts`**

```ts
export interface Paginated<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

export interface ListQuery {
  search?: string
  limit?: number
  offset?: number
  [key: string]: unknown
}

export interface AuthUser {
  id: string
  name: string
  email: string
  role_id: string
  role_name: string
}

export type BadgeColor = 'primary' | 'success' | 'warning' | 'error' | 'neutral' | 'info'
```

- [ ] **Step 2: Write `utils/format.ts`**

```ts
export function formatRupiah(value: string | number | null): string {
  if (value === null || value === '') return '—'
  const n = typeof value === 'string' ? Number(value) : value
  if (Number.isNaN(n)) return '—'
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0
  }).format(n)
}

export function formatDate(iso: string | null, opts: { withTime?: boolean } = {}): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    ...(opts.withTime ? { timeStyle: 'short' } : {})
  }).format(d)
}
```

- [ ] **Step 3: Write `utils/statusMeta.ts`**

```ts
import type { BadgeColor } from '~/types'

interface StatusMeta { color: BadgeColor; labelKey: string }

// Asset statuses from PRD: tersedia/dipinjam/maintenance/dilepas/hilang
export const assetStatusMeta: Record<string, StatusMeta> = {
  available: { color: 'success', labelKey: 'status.asset.available' },
  assigned: { color: 'info', labelKey: 'status.asset.assigned' },
  under_maintenance: { color: 'warning', labelKey: 'status.asset.under_maintenance' },
  disposed: { color: 'neutral', labelKey: 'status.asset.disposed' },
  lost: { color: 'error', labelKey: 'status.asset.lost' }
}

// Approval statuses: pending/approved/rejected
export const approvalStatusMeta: Record<string, StatusMeta> = {
  pending: { color: 'warning', labelKey: 'status.approval.pending' },
  approved: { color: 'success', labelKey: 'status.approval.approved' },
  rejected: { color: 'error', labelKey: 'status.approval.rejected' }
}
```

- [ ] **Step 4: Add status i18n keys** to `id.json`/`en.json` under a `status` object (`status.asset.*`, `status.approval.*`) with natural labels (id: "Tersedia", "Dipinjam", "Maintenance", "Dilepas", "Hilang"; "Menunggu", "Disetujui", "Ditolak").

- [ ] **Step 5: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 6: Commit**

```bash
git add app/types app/utils i18n/locales
git commit -m "feat(frontend): add shared types, rupiah/date formatters, status metadata"
```

---

## Task 3: Pinia stores (auth + ui)

**Files:**
- Create: `frontend/app/stores/auth.ts`
- Create: `frontend/app/stores/ui.ts`

**Interfaces:**
- Produces:
  - `useAuthStore()` → state `{ accessToken: string|null, user: AuthUser|null, permissions: string[] }`; getters `isAuthenticated`; actions `setSession(token, user, permissions)`, `setToken(token)`, `clear()`.
  - `useUiStore()` → state `{ sidebarCollapsed: boolean }`; action `toggleSidebar()`.

- [ ] **Step 1: Write `stores/auth.ts`**

```ts
import { defineStore } from 'pinia'
import type { AuthUser } from '~/types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    accessToken: null as string | null,
    user: null as AuthUser | null,
    permissions: [] as string[]
  }),
  getters: {
    isAuthenticated: state => !!state.accessToken
  },
  actions: {
    setSession(token: string, user: AuthUser, permissions: string[]) {
      this.accessToken = token
      this.user = user
      this.permissions = permissions
    },
    setToken(token: string) {
      this.accessToken = token
    },
    clear() {
      this.accessToken = null
      this.user = null
      this.permissions = []
    }
  }
})
```

- [ ] **Step 2: Write `stores/ui.ts`**

```ts
import { defineStore } from 'pinia'

export const useUiStore = defineStore('ui', {
  state: () => ({
    sidebarCollapsed: false
  }),
  actions: {
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    }
  }
})
```

- [ ] **Step 3: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 4: Commit**

```bash
git add app/stores
git commit -m "feat(frontend): add auth and ui pinia stores"
```

---

## Task 4: API client + real auth composable + permission check

**Files:**
- Create: `frontend/app/composables/useApiClient.ts`
- Create: `frontend/app/composables/useAuthApi.ts`
- Create: `frontend/app/composables/useCan.ts`

**Interfaces:**
- Consumes: `useAuthStore` (Task 3), `Paginated` (Task 2), Nuxt `useRuntimeConfig`, `useToast` (Nuxt UI), `useCookie`.
- Produces:
  - `useApiClient()` → `{ request<T>(path: string, opts?): Promise<T> }` — injects `Authorization: Bearer`, retries once after `/auth/refresh` on 401, toasts on other errors.
  - `useAuthApi()` → `{ login(email, password): Promise<void>; logout(): Promise<void>; fetchMe(): Promise<void>; refresh(): Promise<boolean> }`.
  - `useCan()` → `(permission: string) => boolean` (Superadmin wildcard `*` allowed).

- [ ] **Step 1: Write `composables/useApiClient.ts`**

```ts
export function useApiClient() {
  const config = useRuntimeConfig()
  const auth = useAuthStore()
  const toast = useToast()
  const base = config.public.apiBase as string

  async function refreshToken(): Promise<boolean> {
    const refresh = useCookie<string | null>('inventra_refresh')
    if (!refresh.value) return false
    try {
      const res = await $fetch<{ access_token: string }>(`${base}/auth/refresh`, {
        method: 'POST',
        body: { refresh_token: refresh.value }
      })
      auth.setToken(res.access_token)
      return true
    } catch {
      return false
    }
  }

  async function request<T>(path: string, opts: Record<string, unknown> = {}): Promise<T> {
    const headers: Record<string, string> = { ...(opts.headers as Record<string, string> || {}) }
    if (auth.accessToken) headers.Authorization = `Bearer ${auth.accessToken}`
    try {
      return await $fetch<T>(`${base}${path}`, { ...opts, headers })
    } catch (err: unknown) {
      const status = (err as { statusCode?: number }).statusCode
      if (status === 401 && await refreshToken()) {
        headers.Authorization = `Bearer ${auth.accessToken}`
        return await $fetch<T>(`${base}${path}`, { ...opts, headers })
      }
      if (status === 401) {
        auth.clear()
        await navigateTo('/login')
      } else {
        toast.add({ title: 'Terjadi kesalahan', description: String(status ?? ''), color: 'error' })
      }
      throw err
    }
  }

  return { request, refreshToken }
}
```

- [ ] **Step 2: Write `composables/useAuthApi.ts`** (calls the live backend `/auth/*`):

```ts
import type { AuthUser } from '~/types'

export function useAuthApi() {
  const config = useRuntimeConfig()
  const auth = useAuthStore()
  const base = config.public.apiBase as string
  const refreshCookie = useCookie<string | null>('inventra_refresh', { sameSite: 'lax' })

  async function login(email: string, password: string): Promise<void> {
    const res = await $fetch<{ access_token: string; refresh_token: string }>(`${base}/auth/login`, {
      method: 'POST',
      body: { email, password }
    })
    auth.setToken(res.access_token)
    refreshCookie.value = res.refresh_token
    await fetchMe()
  }

  async function fetchMe(): Promise<void> {
    const client = useApiClient()
    const me = await client.request<AuthUser>('/auth/me')
    const perms = await client.request<{ permissions: string[] }>('/auth/permissions')
    auth.setSession(auth.accessToken as string, me, perms.permissions)
  }

  async function logout(): Promise<void> {
    try {
      await useApiClient().request('/auth/logout', { method: 'POST' })
    } finally {
      auth.clear()
      refreshCookie.value = null
      await navigateTo('/login')
    }
  }

  return { login, fetchMe, logout }
}
```

> Confirm exact response field names (`access_token`/`refresh_token`, `/auth/permissions` shape) against `backend/api/openapi.yaml` during execution; adjust the typed bodies if they differ. `pnpm typecheck` will not catch a runtime field-name mismatch, so cross-check the spec.

- [ ] **Step 3: Write `composables/useCan.ts`**

```ts
export function useCan() {
  const auth = useAuthStore()
  return (permission: string): boolean => {
    if (auth.permissions.includes('*')) return true
    return auth.permissions.includes(permission)
  }
}
```

- [ ] **Step 4: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 5: Commit**

```bash
git add app/composables
git commit -m "feat(frontend): add api client with refresh, real auth api, and permission check"
```

---

## Task 5: Mock service helpers

**Files:**
- Create: `frontend/app/mock/helpers.ts`
- Create: `frontend/app/mock/index.ts`

**Interfaces:**
- Consumes: `Paginated`, `ListQuery` (Task 2).
- Produces:
  - `fakeLatency(ms?: number): Promise<void>`
  - `filterBy<T>(rows: T[], query: ListQuery, fields: (keyof T)[]): T[]` — case-insensitive search across `fields`.
  - `paginate<T>(rows: T[], query: ListQuery): Paginated<T>` — applies `limit` (clamped 1–100, default 20) + `offset`.

- [ ] **Step 1: Write `mock/helpers.ts`**

```ts
import type { Paginated, ListQuery } from '~/types'

export function fakeLatency(ms = 300): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

export function filterBy<T>(rows: T[], query: ListQuery, fields: (keyof T)[]): T[] {
  const term = (query.search ?? '').toString().trim().toLowerCase()
  if (!term) return rows
  return rows.filter(row =>
    fields.some(f => String(row[f] ?? '').toLowerCase().includes(term))
  )
}

export function paginate<T>(rows: T[], query: ListQuery): Paginated<T> {
  const limit = Math.min(Math.max(Number(query.limit) || 20, 1), 100)
  const offset = Math.max(Number(query.offset) || 0, 0)
  return {
    data: rows.slice(offset, offset + limit),
    total: rows.length,
    limit,
    offset
  }
}
```

- [ ] **Step 2: Write `mock/index.ts`**

```ts
// Module fixtures (assets, employees, …) are re-exported here in later phases.
export * from './helpers'
```

- [ ] **Step 3: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 4: Commit**

```bash
git add app/mock
git commit -m "feat(frontend): add mock service helpers (paginate, filterBy, fakeLatency)"
```

---

## Task 6: Route middleware (auth guard + permission guard)

**Files:**
- Create: `frontend/app/middleware/auth.global.ts`
- Create: `frontend/app/middleware/can.ts`

**Interfaces:**
- Consumes: `useAuthStore`, `useCan`.
- Produces: global redirect to `/login` for unauthenticated users (allow-list `/login`); named middleware `can` reading `to.meta.permission`.

- [ ] **Step 1: Write `middleware/auth.global.ts`**

```ts
export default defineNuxtRouteMiddleware((to) => {
  const auth = useAuthStore()
  const publicPaths = ['/login']
  const path = to.path.replace(/^\/(en)(?=\/|$)/, '') || '/'
  if (publicPaths.includes(path)) {
    if (auth.isAuthenticated && path === '/login') return navigateTo('/')
    return
  }
  if (!auth.isAuthenticated) {
    return navigateTo('/login')
  }
})
```

- [ ] **Step 2: Write `middleware/can.ts`**

```ts
export default defineNuxtRouteMiddleware((to) => {
  const permission = to.meta.permission as string | undefined
  if (!permission) return
  const can = useCan()
  if (!can(permission)) {
    return abortNavigation({ statusCode: 403, statusMessage: 'Akses ditolak' })
  }
})
```

- [ ] **Step 3: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 4: Commit**

```bash
git add app/middleware
git commit -m "feat(frontend): add auth and permission route middleware"
```

---

## Task 7: Auth layout + real Login page  *(milestone — runs `pnpm build`)*

**Files:**
- Create: `frontend/app/layouts/auth.vue`
- Create: `frontend/app/pages/login.vue`

**Interfaces:**
- Consumes: `useAuthApi` (Task 4), Nuxt UI `UForm`/`UFormField`/`UInput`/`UButton`/`UCard`/`UAlert`.
- Produces: `/login` route using `layout: 'auth'`; on success navigates to `/`.

- [ ] **Step 1: Write `layouts/auth.vue`** (two-column: brand panel + slot):

```vue
<template>
  <div class="min-h-screen grid lg:grid-cols-2">
    <div class="hidden lg:flex flex-col justify-between p-12 bg-primary text-inverted">
      <div class="flex items-center gap-3">
        <UIcon name="i-lucide-package" class="size-8" />
        <span class="text-2xl font-bold">{{ $t('app.name') }}</span>
      </div>
      <p class="text-lg opacity-90">{{ $t('app.tagline') }}</p>
      <span class="text-sm opacity-70">© {{ year }} Inventra</span>
    </div>
    <div class="flex items-center justify-center p-6">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
const year = new Date().getFullYear()
</script>
```

- [ ] **Step 2: Write `pages/login.vue`**

```vue
<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const { login } = useAuthApi()

const state = reactive({ email: '', password: '' })
const loading = ref(false)
const errorMsg = ref('')

async function onSubmit() {
  loading.value = true
  errorMsg.value = ''
  try {
    await login(state.email, state.password)
    await navigateTo('/')
  } catch {
    errorMsg.value = t('auth.invalidCredentials')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UCard class="w-full max-w-sm">
    <template #header>
      <h1 class="text-xl font-semibold">{{ $t('auth.signInTitle') }}</h1>
      <p class="text-sm text-muted">{{ $t('auth.signInSubtitle') }}</p>
    </template>

    <UAlert
      v-if="errorMsg"
      color="error"
      variant="subtle"
      :description="errorMsg"
      class="mb-4"
    />

    <UForm :state="state" class="space-y-4" @submit="onSubmit">
      <UFormField :label="$t('auth.email')" name="email" required>
        <UInput v-model="state.email" type="email" class="w-full" autocomplete="email" />
      </UFormField>
      <UFormField :label="$t('auth.password')" name="password" required>
        <UInput v-model="state.password" type="password" class="w-full" autocomplete="current-password" />
      </UFormField>
      <UButton type="submit" block :loading="loading">{{ $t('auth.signIn') }}</UButton>
    </UForm>
  </UCard>
</template>
```

- [ ] **Step 3: Add `auth.*` i18n keys** (`signInTitle`, `signInSubtitle`, `email`, `password`, `signIn`, `invalidCredentials`) to `id.json`/`en.json`.

- [ ] **Step 4: Verify** — `pnpm lint && pnpm typecheck && pnpm build` → PASS.

- [ ] **Step 5: Manual check (optional but recommended).** With the backend stack up (`docker compose -f docker-compose.dev.yml up -d` + `cmd/api`) and a seeded admin, run `pnpm dev`, open `/login`, sign in, confirm redirect to `/`. If backend isn't running, skip and rely on build.

- [ ] **Step 6: Commit**

```bash
git add app/layouts/auth.vue app/pages/login.vue i18n/locales
git commit -m "feat(frontend): add auth layout and real login page"
```

---

## Task 8: Presentational primitives (batch 1)

**Files:**
- Create: `StatusBadge.vue`, `EmptyState.vue`, `TableSkeleton.vue`, `CardSkeleton.vue`, `StatCard.vue`, `EntityAvatar.vue`, `Can.vue` (all in `frontend/app/components/`)

**Interfaces:**
- Consumes: `assetStatusMeta`/`approvalStatusMeta` (Task 2), `useCan` (Task 4), Nuxt UI `UBadge`/`USkeleton`/`UCard`/`UAvatar`/`UIcon`.
- Produces:
  - `<StatusBadge :status="string" kind="asset|approval" />`
  - `<EmptyState :title icon? :description? />` + `#action` slot
  - `<TableSkeleton :rows? :cols? />`, `<CardSkeleton />`
  - `<StatCard :label :value :icon? :trend? />`
  - `<EntityAvatar :name :sub? :src? />`
  - `<Can :permission="string">…</Can>` (renders slot only if permitted)

- [ ] **Step 1: `StatusBadge.vue`**

```vue
<script setup lang="ts">
import { assetStatusMeta, approvalStatusMeta } from '~/utils/statusMeta'

const props = withDefaults(defineProps<{ status: string; kind?: 'asset' | 'approval' }>(), {
  kind: 'asset'
})
const { t } = useI18n()
const meta = computed(() => {
  const map = props.kind === 'approval' ? approvalStatusMeta : assetStatusMeta
  return map[props.status] ?? { color: 'neutral' as const, labelKey: '' }
})
const label = computed(() => meta.value.labelKey ? t(meta.value.labelKey) : props.status)
</script>

<template>
  <UBadge :color="meta.color" variant="subtle">{{ label }}</UBadge>
</template>
```

- [ ] **Step 2: `EmptyState.vue`**

```vue
<script setup lang="ts">
withDefaults(defineProps<{ title: string; description?: string; icon?: string }>(), {
  icon: 'i-lucide-inbox'
})
</script>

<template>
  <div class="flex flex-col items-center justify-center text-center py-12 px-4">
    <UIcon :name="icon" class="size-10 text-dimmed mb-3" />
    <p class="font-medium">{{ title }}</p>
    <p v-if="description" class="text-sm text-muted mt-1">{{ description }}</p>
    <div class="mt-4"><slot name="action" /></div>
  </div>
</template>
```

- [ ] **Step 3: `TableSkeleton.vue`**

```vue
<script setup lang="ts">
withDefaults(defineProps<{ rows?: number; cols?: number }>(), { rows: 5, cols: 4 })
</script>

<template>
  <div class="space-y-2">
    <div v-for="r in rows" :key="r" class="flex gap-3">
      <USkeleton v-for="c in cols" :key="c" class="h-8 flex-1" />
    </div>
  </div>
</template>
```

- [ ] **Step 4: `CardSkeleton.vue`**

```vue
<template>
  <UCard>
    <USkeleton class="h-4 w-24 mb-3" />
    <USkeleton class="h-8 w-32" />
  </UCard>
</template>
```

- [ ] **Step 5: `StatCard.vue`**

```vue
<script setup lang="ts">
defineProps<{ label: string; value: string | number; icon?: string; trend?: string }>()
</script>

<template>
  <UCard>
    <div class="flex items-start justify-between">
      <div>
        <p class="text-sm text-muted">{{ label }}</p>
        <p class="text-2xl font-semibold mt-1">{{ value }}</p>
        <p v-if="trend" class="text-xs text-muted mt-1">{{ trend }}</p>
      </div>
      <UIcon v-if="icon" :name="icon" class="size-6 text-primary" />
    </div>
  </UCard>
</template>
```

- [ ] **Step 6: `EntityAvatar.vue`**

```vue
<script setup lang="ts">
defineProps<{ name: string; sub?: string; src?: string }>()
</script>

<template>
  <div class="flex items-center gap-2">
    <UAvatar :src="src" :alt="name" size="sm" />
    <div class="min-w-0">
      <p class="text-sm font-medium truncate">{{ name }}</p>
      <p v-if="sub" class="text-xs text-muted truncate">{{ sub }}</p>
    </div>
  </div>
</template>
```

- [ ] **Step 7: `Can.vue`**

```vue
<script setup lang="ts">
const props = defineProps<{ permission: string }>()
const can = useCan()
const allowed = computed(() => can(props.permission))
</script>

<template>
  <slot v-if="allowed" />
</template>
```

- [ ] **Step 8: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 9: Commit**

```bash
git add app/components/StatusBadge.vue app/components/EmptyState.vue app/components/TableSkeleton.vue app/components/CardSkeleton.vue app/components/StatCard.vue app/components/EntityAvatar.vue app/components/Can.vue
git commit -m "feat(frontend): add presentational primitive components"
```

---

## Task 9: Confirm dialog + useConfirm

**Files:**
- Create: `frontend/app/composables/useConfirm.ts`
- Create: `frontend/app/components/ConfirmDialog.vue`

**Interfaces:**
- Produces:
  - `useConfirm()` → `{ open(opts: { title: string; description?: string; confirmLabel?: string; color?: 'error'|'primary' }): Promise<boolean>; state }` — shared singleton state via `useState`.
  - `<ConfirmDialog />` — mounted once in `default.vue`; resolves the promise on confirm/cancel.

- [ ] **Step 1: Write `composables/useConfirm.ts`**

```ts
interface ConfirmOptions {
  title: string
  description?: string
  confirmLabel?: string
  color?: 'error' | 'primary'
}

interface ConfirmState extends ConfirmOptions {
  open: boolean
}

let resolver: ((value: boolean) => void) | null = null

export function useConfirm() {
  const state = useState<ConfirmState>('confirm-dialog', () => ({
    open: false,
    title: ''
  }))

  function open(opts: ConfirmOptions): Promise<boolean> {
    state.value = { ...opts, open: true }
    return new Promise<boolean>((resolve) => {
      resolver = resolve
    })
  }

  function resolve(value: boolean) {
    state.value = { ...state.value, open: false }
    resolver?.(value)
    resolver = null
  }

  return { state, open, resolve }
}
```

- [ ] **Step 2: Write `components/ConfirmDialog.vue`**

```vue
<script setup lang="ts">
const { state, resolve } = useConfirm()
const isOpen = computed({
  get: () => state.value.open,
  set: v => { if (!v) resolve(false) }
})
</script>

<template>
  <UModal v-model:open="isOpen" :title="state.title" :description="state.description">
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton color="neutral" variant="ghost" @click="resolve(false)">
          {{ $t('common.cancel') }}
        </UButton>
        <UButton :color="state.color ?? 'error'" @click="resolve(true)">
          {{ state.confirmLabel ?? $t('common.delete') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
```

- [ ] **Step 3: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 4: Commit**

```bash
git add app/composables/useConfirm.ts app/components/ConfirmDialog.vue
git commit -m "feat(frontend): add confirm dialog with promise-based useConfirm"
```

---

## Task 10: Data-layout components (PageHeader, DataToolbar, TablePagination, ResourceTable)

**Files:**
- Create: `PageHeader.vue`, `DataToolbar.vue`, `TablePagination.vue`, `ResourceTable.vue` (in `frontend/app/components/`)

**Interfaces:**
- Consumes: Nuxt UI `UTable`, `UPagination`, `UInput`, `UButton`, `UButtonGroup`; `EmptyState`, `TableSkeleton` (Task 8).
- Produces:
  - `<PageHeader :title :subtitle?>` + `#actions` slot.
  - `<DataToolbar v-model:search="string">` + `#filters` slot + `#view` slot; emits `reset`.
  - `<TablePagination :total :limit :offset @update:offset />` with "menampilkan X–Y dari N".
  - `<ResourceTable :rows :columns :loading :total :limit :offset @update:offset>` — `columns: { accessorKey: string; header: string; }[]`; named cell slots forwarded as `#<accessorKey>-cell`; `#row-actions` slot; loading→`TableSkeleton`, empty→`EmptyState`.

- [ ] **Step 1: `PageHeader.vue`**

```vue
<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="flex items-center justify-between gap-4 mb-4">
    <div>
      <h1 class="text-xl font-semibold">{{ title }}</h1>
      <p v-if="subtitle" class="text-sm text-muted">{{ subtitle }}</p>
    </div>
    <div class="flex items-center gap-2"><slot name="actions" /></div>
  </div>
</template>
```

- [ ] **Step 2: `DataToolbar.vue`**

```vue
<script setup lang="ts">
defineProps<{ search?: string }>()
const emit = defineEmits<{ 'update:search': [string]; reset: [] }>()
</script>

<template>
  <div class="flex flex-wrap items-center gap-2 mb-4">
    <UInput
      :model-value="search"
      icon="i-lucide-search"
      :placeholder="$t('common.search')"
      class="w-64"
      @update:model-value="emit('update:search', String($event))"
    />
    <slot name="filters" />
    <UButton color="neutral" variant="ghost" icon="i-lucide-rotate-ccw" @click="emit('reset')">
      {{ $t('common.reset') }}
    </UButton>
    <div class="ms-auto"><slot name="view" /></div>
  </div>
</template>
```

- [ ] **Step 3: `TablePagination.vue`**

```vue
<script setup lang="ts">
const props = defineProps<{ total: number; limit: number; offset: number }>()
const emit = defineEmits<{ 'update:offset': [number] }>()

const page = computed({
  get: () => Math.floor(props.offset / props.limit) + 1,
  set: (p: number) => emit('update:offset', (p - 1) * props.limit)
})
const from = computed(() => props.total === 0 ? 0 : props.offset + 1)
const to = computed(() => Math.min(props.offset + props.limit, props.total))
</script>

<template>
  <div class="flex items-center justify-between gap-4 mt-4">
    <p class="text-sm text-muted">
      {{ $t('common.showingRange', { from, to, total }) }}
    </p>
    <UPagination
      v-model:page="page"
      :total="total"
      :items-per-page="limit"
    />
  </div>
</template>
```

- [ ] **Step 4: `ResourceTable.vue`**

```vue
<script setup lang="ts">
interface Column { accessorKey: string; header: string }
const props = withDefaults(defineProps<{
  rows: Record<string, unknown>[]
  columns: Column[]
  loading?: boolean
  total?: number
  limit?: number
  offset?: number
  emptyTitle?: string
}>(), { loading: false, total: 0, limit: 20, offset: 0, emptyTitle: '' })

const emit = defineEmits<{ 'update:offset': [number] }>()
const { t } = useI18n()

// Append an actions column when the slot is provided.
const slots = useSlots()
const tableColumns = computed(() => {
  const cols = props.columns.map(c => ({ accessorKey: c.accessorKey, header: c.header }))
  if (slots['row-actions']) {
    cols.push({ accessorKey: '__actions', header: t('common.actions') })
  }
  return cols
})
</script>

<template>
  <div>
    <TableSkeleton v-if="loading" :cols="columns.length" />

    <EmptyState
      v-else-if="rows.length === 0"
      :title="emptyTitle || $t('common.noData')"
    />

    <template v-else>
      <UTable :data="rows" :columns="tableColumns">
        <template v-for="col in columns" #[`${col.accessorKey}-cell`]="{ row }" :key="col.accessorKey">
          <slot :name="`${col.accessorKey}-cell`" :row="row.original">
            {{ row.original[col.accessorKey] }}
          </slot>
        </template>
        <template #__actions-cell="{ row }">
          <slot name="row-actions" :row="row.original" />
        </template>
      </UTable>

      <TablePagination
        v-if="total > 0"
        :total="total"
        :limit="limit"
        :offset="offset"
        @update:offset="emit('update:offset', $event)"
      />
    </template>
  </div>
</template>
```

> `UTable` in Nuxt UI v4 is TanStack-based: cell slots are `#<accessorKey>-cell` and expose `{ row }` where the record is `row.original`. Confirm slot naming against the installed `@nuxt/ui` during execution; `pnpm typecheck` plus the `/dev/components` render (Task 13) will surface mismatches.

- [ ] **Step 5: Add i18n key** `common.showingRange` = "Menampilkan {from}–{to} dari {total}" (en: "Showing {from}–{to} of {total}").

- [ ] **Step 6: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 7: Commit**

```bash
git add app/components/PageHeader.vue app/components/DataToolbar.vue app/components/TablePagination.vue app/components/ResourceTable.vue i18n/locales
git commit -m "feat(frontend): add page header, data toolbar, pagination, and resource table"
```

---

## Task 11: Form overlays (FormSlideover, FormModal)

**Files:**
- Create: `frontend/app/components/FormSlideover.vue`, `frontend/app/components/FormModal.vue`

**Interfaces:**
- Consumes: Nuxt UI `USlideover`, `UModal`, `UButton`.
- Produces:
  - `<FormSlideover v-model:open="bool" :title :loading? @submit>` — default slot = form body, sticky footer Batal/Simpan.
  - `<FormModal v-model:open="bool" :title :loading? @submit>` — same contract, modal variant.

- [ ] **Step 1: `FormSlideover.vue`**

```vue
<script setup lang="ts">
defineProps<{ title: string; loading?: boolean }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ submit: [] }>()
</script>

<template>
  <USlideover v-model:open="open" :title="title">
    <template #body>
      <slot />
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton color="neutral" variant="ghost" @click="open = false">
          {{ $t('common.cancel') }}
        </UButton>
        <UButton :loading="loading" @click="emit('submit')">
          {{ $t('common.save') }}
        </UButton>
      </div>
    </template>
  </USlideover>
</template>
```

- [ ] **Step 2: `FormModal.vue`** — identical contract using `UModal`:

```vue
<script setup lang="ts">
defineProps<{ title: string; loading?: boolean }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ submit: [] }>()
</script>

<template>
  <UModal v-model:open="open" :title="title">
    <template #body>
      <slot />
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton color="neutral" variant="ghost" @click="open = false">
          {{ $t('common.cancel') }}
        </UButton>
        <UButton :loading="loading" @click="emit('submit')">
          {{ $t('common.save') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
```

- [ ] **Step 3: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 4: Commit**

```bash
git add app/components/FormSlideover.vue app/components/FormModal.vue
git commit -m "feat(frontend): add form slideover and modal wrappers"
```

---

## Task 12: TreeView + topbar widgets

**Files:**
- Create: `TreeView.vue`, `ThemeToggle.vue`, `LangSwitcher.vue`, `NotificationBell.vue`, `UserMenu.vue`, `GlobalSearch.vue`, `AppBreadcrumb.vue` (in `frontend/app/components/`)

**Interfaces:**
- Consumes: Nuxt UI `UColorModeButton`/`UButton`/`UDropdownMenu`/`UBadge`/`UInput`/`UBreadcrumb`/`UIcon`/`UAvatar`; `useAuthApi` (logout), `useAuthStore`.
- Produces:
  - `<TreeView :nodes :selectedId? @select>` — `TreeNode = { id: string; label: string; icon?: string; childCount?: number; children?: TreeNode[] }`; recursive (self-referencing component).
  - `<ThemeToggle />`, `<LangSwitcher />`, `<NotificationBell :count? />`, `<UserMenu />`, `<GlobalSearch />`, `<AppBreadcrumb :items />`.

- [ ] **Step 1: `TreeView.vue`** (recursive via its own name `TreeView`):

```vue
<script setup lang="ts">
export interface TreeNode {
  id: string
  label: string
  icon?: string
  childCount?: number
  children?: TreeNode[]
}
const props = defineProps<{ nodes: TreeNode[]; selectedId?: string }>()
const emit = defineEmits<{ select: [string] }>()
const expanded = ref<Record<string, boolean>>({})
function toggle(id: string) { expanded.value[id] = !expanded.value[id] }
</script>

<template>
  <ul class="space-y-0.5">
    <li v-for="node in nodes" :key="node.id">
      <div
        class="flex items-center gap-1.5 px-2 py-1.5 rounded-md cursor-pointer hover:bg-elevated"
        :class="node.id === selectedId ? 'bg-elevated text-primary font-medium' : ''"
        @click="emit('select', node.id)"
      >
        <UButton
          v-if="node.children?.length"
          color="neutral"
          variant="ghost"
          size="xs"
          :icon="expanded[node.id] ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
          @click.stop="toggle(node.id)"
        />
        <span v-else class="w-5" />
        <UIcon v-if="node.icon" :name="node.icon" class="size-4 text-muted" />
        <span class="text-sm truncate">{{ node.label }}</span>
        <UBadge v-if="node.childCount" color="neutral" variant="subtle" size="sm" class="ms-auto">
          {{ node.childCount }}
        </UBadge>
      </div>
      <div v-if="node.children?.length && expanded[node.id]" class="ms-4 border-s border-default ps-1">
        <TreeView :nodes="node.children" :selected-id="selectedId" @select="emit('select', $event)" />
      </div>
    </li>
  </ul>
</template>
```

- [ ] **Step 2: `ThemeToggle.vue`**

```vue
<template>
  <UColorModeButton />
</template>
```

- [ ] **Step 3: `LangSwitcher.vue`**

```vue
<script setup lang="ts">
const { locale, locales, setLocale } = useI18n()
const items = computed(() => (locales.value as { code: string; name: string }[]).map(l => ({
  label: l.name,
  onSelect: () => setLocale(l.code as 'id' | 'en')
})))
</script>

<template>
  <UDropdownMenu :items="items">
    <UButton color="neutral" variant="ghost" icon="i-lucide-languages" :label="locale.toUpperCase()" />
  </UDropdownMenu>
</template>
```

- [ ] **Step 4: `NotificationBell.vue`**

```vue
<script setup lang="ts">
withDefaults(defineProps<{ count?: number }>(), { count: 0 })
</script>

<template>
  <UChip :show="count > 0" :text="count" size="2xl" color="error">
    <UButton color="neutral" variant="ghost" icon="i-lucide-bell" />
  </UChip>
</template>
```

- [ ] **Step 5: `UserMenu.vue`**

```vue
<script setup lang="ts">
const auth = useAuthStore()
const { logout } = useAuthApi()
const { t } = useI18n()
const items = computed(() => [[
  { label: t('nav.profile'), icon: 'i-lucide-user' },
  { label: t('auth.signOut'), icon: 'i-lucide-log-out', onSelect: () => logout() }
]])
</script>

<template>
  <UDropdownMenu :items="items">
    <button class="flex items-center gap-2">
      <UAvatar :alt="auth.user?.name ?? ''" size="sm" />
      <div class="hidden md:block text-start">
        <p class="text-sm font-medium">{{ auth.user?.name }}</p>
        <p class="text-xs text-muted">{{ auth.user?.role_name }}</p>
      </div>
    </button>
  </UDropdownMenu>
</template>
```

- [ ] **Step 6: `GlobalSearch.vue`** (static input for the shell; wired to command palette later):

```vue
<template>
  <UInput
    icon="i-lucide-search"
    :placeholder="$t('common.search')"
    class="w-full max-w-xs hidden md:block"
  />
</template>
```

- [ ] **Step 7: `AppBreadcrumb.vue`**

```vue
<script setup lang="ts">
defineProps<{ items: { label: string; to?: string }[] }>()
</script>

<template>
  <UBreadcrumb :items="items" />
</template>
```

- [ ] **Step 8: Add i18n keys** `nav.profile`, `auth.signOut`.

- [ ] **Step 9: Verify** — `pnpm lint && pnpm typecheck` → PASS.

- [ ] **Step 10: Commit**

```bash
git add app/components/TreeView.vue app/components/ThemeToggle.vue app/components/LangSwitcher.vue app/components/NotificationBell.vue app/components/UserMenu.vue app/components/GlobalSearch.vue app/components/AppBreadcrumb.vue i18n/locales
git commit -m "feat(frontend): add tree view and topbar widget components"
```

---

## Task 13: App shell (sidebar + topbar + default layout), pages & style guide  *(milestone — runs `pnpm build`)*

**Files:**
- Create: `frontend/app/components/AppSidebar.vue`, `frontend/app/components/AppTopbar.vue`
- Create: `frontend/app/layouts/default.vue`
- Modify: `frontend/app/pages/index.vue`
- Create: `frontend/app/pages/dev/components.vue`
- Delete: `frontend/app/components/TemplateMenu.vue` (stock template leftover)

**Interfaces:**
- Consumes: `useUiStore`, `useCan`, all Task 8–12 components.
- Produces: `default.vue` wrapping every page; `/` dashboard placeholder; `/dev/components` rendering each global component in both light/dark.

- [ ] **Step 1: `AppSidebar.vue`** — nav groups filtered by permission, collapsible:

```vue
<script setup lang="ts">
const ui = useUiStore()
const can = useCan()

interface NavItem { labelKey: string; icon: string; to: string; permission?: string }
interface NavGroup { labelKey: string; items: NavItem[] }

const groups: NavGroup[] = [
  { labelKey: 'nav.group.main', items: [
    { labelKey: 'nav.dashboard', icon: 'i-lucide-layout-dashboard', to: '/' }
  ] },
  { labelKey: 'nav.group.asset', items: [
    { labelKey: 'nav.assets', icon: 'i-lucide-package', to: '/assets', permission: 'asset.read' },
    { labelKey: 'nav.assignment', icon: 'i-lucide-arrow-left-right', to: '/assignment', permission: 'assignment.read' },
    { labelKey: 'nav.maintenance', icon: 'i-lucide-wrench', to: '/maintenance', permission: 'maintenance.read' },
    { labelKey: 'nav.approval', icon: 'i-lucide-check-square', to: '/approval', permission: 'approval.read' }
  ] },
  { labelKey: 'nav.group.masterdata', items: [
    { labelKey: 'nav.offices', icon: 'i-lucide-building-2', to: '/master/offices', permission: 'masterdata.office.manage' },
    { labelKey: 'nav.employees', icon: 'i-lucide-users', to: '/master/employees', permission: 'masterdata.office.manage' },
    { labelKey: 'nav.reference', icon: 'i-lucide-list', to: '/master/reference', permission: 'masterdata.global.manage' }
  ] },
  { labelKey: 'nav.group.settings', items: [
    { labelKey: 'nav.users', icon: 'i-lucide-user-cog', to: '/settings/users', permission: 'user.manage' },
    { labelKey: 'nav.audit', icon: 'i-lucide-scroll-text', to: '/settings/audit', permission: 'audit.read' }
  ] }
]

function visibleItems(items: NavItem[]) {
  return items.filter(i => !i.permission || can(i.permission))
}
</script>

<template>
  <aside
    class="flex flex-col border-e border-default bg-default transition-all"
    :class="ui.sidebarCollapsed ? 'w-16' : 'w-60'"
  >
    <div class="flex items-center gap-3 h-15 px-4 border-b border-default">
      <div class="size-9 rounded-lg bg-primary text-inverted flex items-center justify-center shrink-0">
        <UIcon name="i-lucide-package" class="size-5" />
      </div>
      <span v-if="!ui.sidebarCollapsed" class="font-bold text-lg">{{ $t('app.name') }}</span>
    </div>

    <nav class="flex-1 overflow-y-auto p-3 space-y-4">
      <template v-for="group in groups" :key="group.labelKey">
        <div v-if="visibleItems(group.items).length">
          <p v-if="!ui.sidebarCollapsed" class="px-3 pb-1 text-[10px] font-semibold uppercase tracking-wider text-dimmed font-mono">
            {{ $t(group.labelKey) }}
          </p>
          <NuxtLink
            v-for="item in visibleItems(group.items)"
            :key="item.to"
            :to="item.to"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm hover:bg-elevated"
            active-class="bg-primary/10 text-primary font-medium"
          >
            <UIcon :name="item.icon" class="size-5 shrink-0" />
            <span v-if="!ui.sidebarCollapsed">{{ $t(item.labelKey) }}</span>
          </NuxtLink>
        </div>
      </template>
    </nav>
  </aside>
</template>
```

- [ ] **Step 2: `AppTopbar.vue`**

```vue
<script setup lang="ts">
const ui = useUiStore()
</script>

<template>
  <header class="flex items-center gap-3 h-15 px-4 border-b border-default bg-default">
    <UButton color="neutral" variant="ghost" icon="i-lucide-panel-left" @click="ui.toggleSidebar()" />
    <GlobalSearch />
    <div class="ms-auto flex items-center gap-1">
      <LangSwitcher />
      <ThemeToggle />
      <NotificationBell :count="0" />
      <UserMenu />
    </div>
  </header>
</template>
```

- [ ] **Step 3: `layouts/default.vue`**

```vue
<template>
  <div class="flex h-screen overflow-hidden bg-muted">
    <AppSidebar />
    <div class="flex-1 flex flex-col min-w-0">
      <AppTopbar />
      <main class="flex-1 overflow-y-auto p-6">
        <slot />
      </main>
    </div>
    <ConfirmDialog />
  </div>
</template>
```

- [ ] **Step 4: Rewrite `pages/index.vue`** (dashboard placeholder using the shell + a few primitives):

```vue
<script setup lang="ts">
const { t } = useI18n()
</script>

<template>
  <div>
    <PageHeader :title="t('nav.dashboard')" :subtitle="t('app.tagline')" />
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <StatCard :label="t('nav.assets')" value="0" icon="i-lucide-package" />
      <StatCard :label="t('nav.maintenance')" value="0" icon="i-lucide-wrench" />
      <StatCard :label="t('nav.approval')" value="0" icon="i-lucide-check-square" />
      <StatCard :label="t('nav.assignment')" value="0" icon="i-lucide-arrow-left-right" />
    </div>
  </div>
</template>
```

- [ ] **Step 5: Write `pages/dev/components.vue`** — render every global component for visual verification:

```vue
<script setup lang="ts">
import type { TreeNode } from '~/components/TreeView.vue'

const { open } = useConfirm()
const rows = ref([
  { id: '1', name: 'Laptop Dell', status: 'available' },
  { id: '2', name: 'Proyektor Epson', status: 'under_maintenance' }
])
const columns = [
  { accessorKey: 'name', header: 'Nama' },
  { accessorKey: 'status', header: 'Status' }
]
const tree: TreeNode[] = [
  { id: 'p', label: 'Kantor Pusat', icon: 'i-lucide-building-2', childCount: 1, children: [
    { id: 'w', label: 'Kanwil Jakarta', icon: 'i-lucide-building', children: [
      { id: 'c', label: 'Cabang Jakarta Selatan', icon: 'i-lucide-store' }
    ] }
  ] }
]
const offset = ref(0)
async function askDelete() {
  await open({ title: 'Hapus data?', description: 'Tindakan ini tidak dapat dibatalkan.' })
}
</script>

<template>
  <div class="space-y-8 max-w-4xl">
    <PageHeader title="Component Library" subtitle="Style guide & verifikasi">
      <template #actions>
        <UButton @click="askDelete">Confirm dialog</UButton>
      </template>
    </PageHeader>

    <section class="space-y-2">
      <h2 class="font-semibold">Status badges</h2>
      <div class="flex flex-wrap gap-2">
        <StatusBadge status="available" />
        <StatusBadge status="under_maintenance" />
        <StatusBadge status="lost" />
        <StatusBadge status="pending" kind="approval" />
        <StatusBadge status="approved" kind="approval" />
      </div>
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">Stat cards</h2>
      <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
        <StatCard label="Total Aset" value="1.248" icon="i-lucide-package" trend="+3,2%" />
        <CardSkeleton />
      </div>
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">Resource table</h2>
      <ResourceTable :rows="rows" :columns="columns" :total="2" :offset="offset" @update:offset="offset = $event">
        <template #status-cell="{ row }">
          <StatusBadge :status="row.status as string" />
        </template>
        <template #row-actions>
          <UButton size="xs" color="neutral" variant="ghost" icon="i-lucide-pencil" />
        </template>
      </ResourceTable>
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">Empty state</h2>
      <EmptyState title="Belum ada aset" description="Tambahkan aset pertama Anda." />
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">Tree view</h2>
      <TreeView :nodes="tree" selected-id="c" />
    </section>
  </div>
</template>
```

- [ ] **Step 6: Delete the stock `TemplateMenu.vue`** (no longer referenced after `app.vue` rewrite):

```bash
git rm app/components/TemplateMenu.vue
```

- [ ] **Step 7: Add remaining i18n keys** — `nav.group.*`, `nav.dashboard`, `nav.assets`, `nav.assignment`, `nav.maintenance`, `nav.approval`, `nav.offices`, `nav.employees`, `nav.reference`, `nav.users`, `nav.audit` — in both `id.json` and `en.json`.

- [ ] **Step 8: Verify** — `pnpm lint && pnpm typecheck && pnpm build` → PASS.

- [ ] **Step 9: Manual visual check (recommended).** `pnpm dev`, open `/dev/components`, toggle light/dark and id/en, confirm every section renders correctly and the shell sidebar collapses. (Requires being authenticated — log in first, or temporarily relax `auth.global.ts` during dev.)

- [ ] **Step 10: Commit**

```bash
git add app/components/AppSidebar.vue app/components/AppTopbar.vue app/layouts/default.vue app/pages/index.vue app/pages/dev/components.vue i18n/locales
git commit -m "feat(frontend): add app shell, dashboard placeholder, and component style guide"
```

---

## Self-Review

**Spec coverage** (against `2026-06-24-frontend-foundation-design.md`):
- Design tokens / Inter / slate-green → Task 1. ✔
- Mock service layer + helpers → Task 5 (helpers); module services arrive in Phase 2 per spec. ✔
- Real auth (login, token, refresh-on-401, guard, /me + /permissions, role menu) → Tasks 3, 4, 6, 7, 13 (`useCan` menu filtering). ✔
- Stores (auth, ui) → Task 3. ✔
- Layouts (auth, default) → Tasks 7, 13. ✔
- Pages (`/login`, `/`, `/dev/components`) → Tasks 7, 13. ✔
- Global component inventory B1 — StatusBadge, EmptyState, skeletons, StatCard, EntityAvatar, Can (T8); ConfirmDialog (T9); PageHeader, DataToolbar, TablePagination, ResourceTable (T10); FormSlideover, FormModal (T11); TreeView + topbar widgets, AppBreadcrumb (T12); AppSidebar, AppTopbar (T13). ✔ All B1 components covered.
- i18n keys for all strings → seeded T1, extended each task. ✔
- Out of scope honored: no Google OAuth, no password reset, no charts, no feature screens, no module fixtures. ✔

**Placeholder scan:** No "TBD/TODO/implement later". The two "confirm against installed version" notes (Tasks 4, 10) are verification instructions with full code already supplied, not missing content.

**Type consistency:** `Paginated`/`ListQuery` (T2) used identically in T5; `AuthUser` (T2) used in T3/T4; `useApiClient().request` (T4) consumed by `useAuthApi`/`useConfirm` consumers; `useConfirm` `open()`/`resolve()` consistent T9↔T13; `ResourceTable` column shape `{accessorKey, header}` consistent T10↔T13; `TreeNode` defined T12, imported T13. No mismatches found.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-24-frontend-foundation.md`.
