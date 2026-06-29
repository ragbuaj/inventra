# Wire Kategori Aset to `/api/v1/categories` (+ tree endpoint) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the Kategori Aset screen from mock to the real `/api/v1/categories` backend, adding a `GET /categories/tree` endpoint that returns the full category set (no 100-row cap) for client-side tree building.

**Architecture:** Backend adds an unpaginated `ListCategoryTree` query + `GET /categories/tree` (flat `{data:[...]}`). Frontend rewrites `useCategories` to HTTP and adds `tree()`; the page loads via `tree()` (full set) and keeps its existing client-side DFS/pagination/filters. The mock's co-located utilities move to a constants file; the mock is deleted.

**Tech Stack:** Go 1.25 + Gin + pgx/sqlc (backend); Nuxt 4 SPA + Nuxt UI + @nuxtjs/i18n + Vitest + Playwright (frontend).

## Global Constraints

- **The frontend `Category` type already matches the backend JSON keys exactly** (English snake_case) — NO rename. The 12 fields + enums are already aligned. The only type change is adding `updated_at?: string | null`.
- **Numeric fields are JSON strings** (`default_salvage_rate` `numeric(5,4)`, `capitalization_threshold` `numeric(18,2)` → e.g. `"1000000.00"`); the frontend already treats them as strings (`formatThousands`/`parseThousands` strip non-digits).
- **`GET /categories/tree`** returns the FULL non-deleted set, **flat** `{ "data": [ ...Category ] }`, NO pagination. Gated `authMW` only (read parity with `GET /categories`). Register `/tree` BEFORE `/:id` (Gin v1.12 allows static + param coexistence).
- CRUD stays gated `masterdata.global.manage`; reads `authMW`; categories are **global (no data-scope)**. Page guard stays `definePageMeta({ middleware:'can', permission:'masterdata.global.manage' })`.
- All frontend HTTP via `useApiClient().request`; i18n mandatory in BOTH `id.json` + `en.json`; no hardcoded user-facing strings; ESLint no-trailing-commas + 1tbs.
- **#40 lesson:** wiring `useCategories` mock→HTTP makes any test that mounts a consumer (the categories page) hit the real backend (`:8080`) → `ECONNREFUSED` unhandled rejection → `pnpm test` exits 1. The ONLY consumer is `pages/master/categories.vue` (its component test `master-categories.spec.ts` must stub the API). **Verify the FULL `pnpm test` exit code is 0**, not just "N passed".
- Backend gates (from `backend/`): `go build ./...`, `go vet ./...`, `go test ./...`, **and `go test -tags=integration ./...`**, plus Spectral lint on `backend/api/openapi.yaml`.
- Frontend gates (from `frontend/`): `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
- After editing migrations/queries: `sqlc generate` (from `backend/`). Don't hand-edit `backend/db/sqlc/`.
- These pre-existing files must be REWRITTEN/handled, not created fresh: `frontend/test/nuxt/master-categories.spec.ts` (mock-based component test), `frontend/test/nuxt/CategoryFormSlideover.spec.ts` (may import mock utils), `frontend/e2e/categories.spec.ts` (mock-based e2e), `frontend/test/unit/categories-mock.spec.ts` (mock unit test → delete).

---

### Task 1: Backend — `GET /categories/tree` (full, unpaginated)

**Files:**
- Modify: `backend/db/queries/categories.sql` (add `ListCategoryTree`)
- Regenerate: `backend/db/sqlc/`
- Modify: `backend/internal/masterdata/category/service.go` (add `Tree`)
- Modify: `backend/internal/masterdata/category/handler.go` (add `tree`)
- Modify: `backend/internal/masterdata/category/routes.go` (register `/tree`)
- Modify: `backend/api/openapi.yaml` (document `GET /categories/tree`)
- Test: `backend/internal/masterdata/category/category_integration_test.go` (NEW, //go:build integration)

**Interfaces:**
- Produces: `GET /api/v1/categories/tree` → `{ "data": [ {id,name,code,parent_id,default_depreciation_method,default_useful_life_months,default_salvage_rate,asset_class,default_fiscal_group,default_fiscal_life_months,gl_account_code,capitalization_threshold,is_active,created_at,updated_at} ] }`. `category.Service.Tree(ctx) ([]sqlc.MasterdataCategory, error)`. Frontend Task 2 consumes this JSON.

- [ ] **Step 1: Add the tree query**

Append to `backend/db/queries/categories.sql`:
```sql
-- name: ListCategoryTree :many
-- The full non-deleted category set (no pagination) for client-side tree building.
SELECT * FROM masterdata.categories
WHERE deleted_at IS NULL
ORDER BY name;
```

- [ ] **Step 2: Regenerate sqlc**

Run (from `backend/`): `sqlc generate`
Expected: no error; `ListCategoryTree(ctx) ([]MasterdataCategory, error)` appears in `db/sqlc/categories.sql.go` + the `Querier` interface.

- [ ] **Step 3: Add the service method**

In `backend/internal/masterdata/category/service.go`, add after `List`:
```go
// Tree returns the full non-deleted category set (no pagination) for the tree view.
func (s *Service) Tree(ctx context.Context) ([]sqlc.MasterdataCategory, error) {
	return s.q.ListCategoryTree(ctx)
}
```

- [ ] **Step 4: Add the handler**

In `backend/internal/masterdata/category/handler.go`, add after `list` (before `get`):
```go
func (h *Handler) tree(c *gin.Context) {
	rows, err := h.svc.Tree(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list category tree"})
		return
	}
	data := make([]Response, 0, len(rows))
	for _, cat := range rows {
		data = append(data, toResponse(cat))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
```

- [ ] **Step 5: Register the route (before `/:id`)**

In `backend/internal/masterdata/category/routes.go`, replace the route block with:
```go
	g := rg.Group("/categories")
	g.GET("/tree", authMW, h.tree)
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
```

- [ ] **Step 6: Write the failing integration test**

Create `backend/internal/masterdata/category/category_integration_test.go`:
```go
//go:build integration

package category_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/category"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func mkInput(name string) category.CreateInput {
	return category.CreateInput{Name: name, AssetClass: sqlc.SharedAssetClass("tangible"), IsActive: true}
}

func TestCategoryTree(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := category.NewService(q)
	ctx := context.Background()

	t.Run("returns non-deleted incl. parent/child, excludes soft-deleted", func(t *testing.T) {
		testsupport.Reset(t, pool)

		parent, err := svc.Create(ctx, mkInput("Perangkat IT"))
		require.NoError(t, err)
		childIn := mkInput("Laptop")
		childIn.ParentID = &parent.ID
		child, err := svc.Create(ctx, childIn)
		require.NoError(t, err)
		gone, err := svc.Create(ctx, mkInput("Dihapus"))
		require.NoError(t, err)
		_, err = svc.Delete(ctx, gone.ID)
		require.NoError(t, err)

		rows, err := svc.Tree(ctx)
		require.NoError(t, err)
		ids := map[string]bool{}
		var childRow sqlc.MasterdataCategory
		for _, r := range rows {
			ids[r.ID.String()] = true
			if r.ID == child.ID {
				childRow = r
			}
		}
		assert.True(t, ids[parent.ID.String()], "parent present")
		assert.True(t, ids[child.ID.String()], "child present")
		assert.False(t, ids[gone.ID.String()], "soft-deleted excluded")
		require.NotNil(t, childRow.ParentID)
		assert.Equal(t, parent.ID, *childRow.ParentID, "child parent_id passthrough")
	})

	t.Run("no pagination cap — returns more than the list's 100-row limit", func(t *testing.T) {
		testsupport.Reset(t, pool)
		for i := 0; i < 101; i++ {
			_, err := svc.Create(ctx, mkInput(fmt.Sprintf("Kategori %03d", i)))
			require.NoError(t, err)
		}
		rows, err := svc.Tree(ctx)
		require.NoError(t, err)
		assert.Equal(t, 101, len(rows), "tree returns all rows, not capped at 100")
	})
}
```

- [ ] **Step 7: Run build + tests**

Run (from `backend/`):
```bash
go build ./... && go vet ./...
go test -tags=integration ./internal/masterdata/category/ -run TestCategoryTree -v
go test ./...
```
Expected: build clean; `TestCategoryTree` PASS (2 subtests); full suite green. (Integration needs the dev Postgres on :5433, already running.)

- [ ] **Step 8: Update OpenAPI + Spectral**

In `backend/api/openapi.yaml`, find the `/categories` paths + the category response schema (the existing `GET /categories` documents it — note the schema name). Add a `GET /categories/tree` path: summary "Category tree (full)", bearer auth, 200 response `{ data: array of <the existing Category schema> }` (no pagination fields). Reuse the existing category schema `$ref`; do not duplicate it.

Run (from repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add backend/db/queries/categories.sql backend/db/sqlc/ backend/internal/masterdata/category/ backend/api/openapi.yaml
git commit -m "feat(masterdata): add GET /categories/tree (full unpaginated category set)"
```

---

### Task 2: Frontend — `useCategories` HTTP rewrite + `tree()`

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useCategories.ts`
- Delete: `frontend/test/unit/categories-mock.spec.ts`
- Test: `frontend/test/unit/use-categories.spec.ts` (NEW)

**Interfaces:**
- Consumes: `GET /categories` + `GET /categories/tree` (Task 1).
- Produces: `useCategories()` → `{ list, get, create, update, remove, tree }`. `CategoryInput = Omit<Category,'id'|'created_at'|'updated_at'>`. `tree(): Promise<Category[]>`. Task 3 (page) consumes `tree`.

- [ ] **Step 1: Write the failing unit test**

Create `frontend/test/unit/use-categories.spec.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useCategories } from '~/composables/api/useCategories'

beforeEach(() => request.mockReset())

const sample = { id: 'c1', name: 'IT', code: 'ITX', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3', capitalization_threshold: '1000000.00', is_active: true, created_at: '2026-01-01', updated_at: '2026-01-02' }

describe('useCategories', () => {
  it('tree GETs /categories/tree and returns the data array', async () => {
    request.mockResolvedValueOnce({ data: [sample] })
    const rows = await useCategories().tree()
    expect(request).toHaveBeenCalledWith('/categories/tree')
    expect(rows).toHaveLength(1)
    expect(rows[0].id).toBe('c1')
  })

  it('list builds the query (omits empty search) and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [sample], total: 1, limit: 20, offset: 0 })
    const res = await useCategories().list({ limit: 20, offset: 0 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/categories?')
    expect(path).toContain('limit=20')
    expect(path).not.toContain('search=')
    expect(res.total).toBe(1)
  })

  it('get GETs /categories/:id', async () => {
    request.mockResolvedValueOnce(sample)
    await useCategories().get('c1')
    expect(request).toHaveBeenCalledWith('/categories/c1')
  })

  it('create POSTs /categories with the body verbatim', async () => {
    request.mockResolvedValueOnce(sample)
    const input = { name: 'IT', code: 'ITX', parent_id: null, default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1', default_fiscal_life_months: 48, gl_account_code: '1.2.3', capitalization_threshold: '1000000', is_active: true } as const
    await useCategories().create(input)
    expect(request).toHaveBeenCalledWith('/categories', { method: 'POST', body: input })
  })

  it('update PUTs /categories/:id', async () => {
    request.mockResolvedValueOnce(sample)
    await useCategories().update('c1', { name: 'IT2' } as never)
    expect(request).toHaveBeenCalledWith('/categories/c1', { method: 'PUT', body: { name: 'IT2' } })
  })

  it('remove DELETEs /categories/:id', async () => {
    request.mockResolvedValueOnce(undefined)
    await useCategories().remove('c1')
    expect(request).toHaveBeenCalledWith('/categories/c1', { method: 'DELETE' })
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-categories`
Expected: FAIL (current `useCategories` is mock-backed; `tree` undefined).

- [ ] **Step 3: Rewrite `useCategories.ts`**

Replace `frontend/app/composables/api/useCategories.ts` entirely with:
```ts
import type { Category, ListQuery, Paginated } from '~/types'

export type CategoryInput = Omit<Category, 'id' | 'created_at' | 'updated_at'>

/** Asset categories, wired to /api/v1/categories. `tree()` loads the full set. */
export function useCategories() {
  const { request } = useApiClient()

  async function list(query: ListQuery = {}): Promise<Paginated<Category>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 20))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    return request<Paginated<Category>>(`/categories?${q.toString()}`)
  }

  async function tree(): Promise<Category[]> {
    const res = await request<{ data: Category[] }>('/categories/tree')
    return res.data
  }

  async function get(id: string): Promise<Category> {
    return request<Category>(`/categories/${id}`)
  }

  async function create(input: CategoryInput): Promise<Category> {
    return request<Category>('/categories', { method: 'POST', body: input })
  }

  async function update(id: string, input: CategoryInput): Promise<Category> {
    return request<Category>(`/categories/${id}`, { method: 'PUT', body: input })
  }

  async function remove(id: string): Promise<void> {
    await request(`/categories/${id}`, { method: 'DELETE' })
  }

  return { list, get, create, update, remove, tree }
}
```

- [ ] **Step 4: Delete the mock unit test + run**

Run (from `frontend/`):
```bash
git rm frontend/test/unit/categories-mock.spec.ts
pnpm test -- use-categories
pnpm lint
```
Expected: `use-categories` PASS; lint clean. NOTE: `pnpm typecheck` may flag `CategoryInput` usage if the page passes `updated_at` — it does not (the page builds input without it); typecheck should pass. The whole `pnpm test` suite is NOT green yet — `test/nuxt/master-categories.spec.ts` (mock-based) now fails because the page's `useCategories` calls real HTTP with no stub. EXPECTED, fixed in Task 4. Do NOT touch the page or that test here.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useCategories.ts frontend/test/unit/use-categories.spec.ts
git commit -m "feat(categories): wire useCategories to /api/v1/categories + add tree()"
```

---

### Task 3: Frontend — page → `tree()`, move utils to constants, type + i18n

**Files:**
- Create: `frontend/app/constants/categoryMeta.ts`
- Modify: `frontend/app/components/category/CategoryFormSlideover.vue` (import from constants)
- Modify: `frontend/app/pages/master/categories.vue` (use `tree()` + load-error/retry)
- Modify: `frontend/app/types/index.ts` (`Category.updated_at?`)
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json` (`fiscalGroup.non_susut`)

**Interfaces:**
- Consumes: `useCategories().tree` (Task 2).
- Produces: `~/constants/categoryMeta` exports `FISCAL_GROUPS`, `isBuildingGroup`, `formatThousands`, `parseThousands`. `Category.updated_at?: string | null`.

- [ ] **Step 1: Create the constants file**

`frontend/app/constants/categoryMeta.ts` (moved verbatim from `mock/categories.ts`, minus the seed/store):
```ts
import type { FiscalGroup } from '~/types'

// Form-select order (mockup); excludes non_susut.
export const FISCAL_GROUPS: FiscalGroup[] = [
  'kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen'
]

export function isBuildingGroup(g: FiscalGroup | null | undefined): boolean {
  return g === 'bangunan_permanen' || g === 'bangunan_non_permanen'
}

// Display a numeric string with id-ID thousands grouping ('1000000' → '1.000.000').
export function formatThousands(v: string | number | null | undefined): string {
  const s = String(v ?? '').replace(/\D/g, '')
  if (!s) return ''
  return Number(s).toLocaleString('id-ID')
}

// Strip grouping back to a bare digit string ('1.000.000' → '1000000').
export function parseThousands(v: string | null | undefined): string {
  return String(v ?? '').replace(/\D/g, '')
}
```

- [ ] **Step 2: Repoint `CategoryFormSlideover.vue` import**

In `frontend/app/components/category/CategoryFormSlideover.vue` line 4, change:
```ts
import { FISCAL_GROUPS, isBuildingGroup, formatThousands, parseThousands } from '~/mock/categories'
```
to:
```ts
import { FISCAL_GROUPS, isBuildingGroup, formatThousands, parseThousands } from '~/constants/categoryMeta'
```
(No other change to that component.)

- [ ] **Step 3: Add `updated_at` to the `Category` type**

In `frontend/app/types/index.ts`, in the `Category` interface, add after `created_at: string`:
```ts
  updated_at?: string | null
```

- [ ] **Step 4: Add the `non_susut` i18n label (both locales)**

In `frontend/i18n/locales/id.json` and `en.json`, under `masterdata.categories.fiscalGroup`, add `non_susut` (keep existing keys, no trailing commas):
- id: `"non_susut": "Non-Penyusutan"`
- en: `"non_susut": "Non-Depreciable"`

- [ ] **Step 5: Switch the page to `tree()` + add load-error/retry**

In `frontend/app/pages/master/categories.vue`:
- Add a `loadFailed` ref near the other refs (after `const loading = ref(true)`):
```ts
const loadFailed = ref(false)
```
- Replace `refresh()` with:
```ts
async function refresh() {
  loading.value = true
  loadFailed.value = false
  try {
    allRows.value = await api.tree()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}
```
- In the template, render a load-error block with a retry button when `loadFailed`, before the `<ResourceTable>` (and keep the table rendering otherwise):
```vue
    <div
      v-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-16 text-muted"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-6"
      />
      <span class="text-sm">{{ t('masterdata.categories.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="refresh"
      >
        {{ t('common.retry') }}
      </UButton>
    </div>
    <ResourceTable
      v-else
      ...existing props...
    >
```
- Add the i18n key `masterdata.categories.loadError` to BOTH locales (id: `"loadError": "Gagal memuat kategori."`, en: `"loadError": "Failed to load categories."`). If `common.retry` does not already exist in the locales, add it (id: `"retry": "Coba lagi"`, en: `"retry": "Retry"`) — grep first; reuse if present.

- [ ] **Step 6: Verify lint + typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: both exit 0. NOTE: `pnpm test` is still red on `test/nuxt/master-categories.spec.ts` (mock-based) — fixed in Task 4.

- [ ] **Step 7: Commit**

```bash
git add frontend/app/constants/categoryMeta.ts frontend/app/components/category/CategoryFormSlideover.vue frontend/app/pages/master/categories.vue frontend/app/types/index.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "feat(categories): page loads via tree(), utils to constants, +updated_at/non_susut"
```

---

### Task 4: Frontend — rewrite the component test (+ confirm form test)

**Files:**
- Modify (full rewrite): `frontend/test/nuxt/master-categories.spec.ts`
- Possibly modify: `frontend/test/nuxt/CategoryFormSlideover.spec.ts` (only if it imports from `~/mock/categories`)

**Interfaces:**
- Consumes: the wired page + composable; mocks the HTTP layer.

- [ ] **Step 1: Study the harness + current tests**

Read the CURRENT `frontend/test/nuxt/master-categories.spec.ts` (to be overwritten) AND a wired-screen test that stubs `useApiClient` per-path + `useAuthStore().setSession(token, user, ['*'])` + `mountSuspended` (e.g. `frontend/test/nuxt/settings-users.spec.ts`). Read `frontend/app/pages/master/categories.vue` so stubs match the real calls. On mount the page calls `tree()` → `GET /categories/tree` (single call; no counts, no list). Mutations: `POST /categories`, `PUT /categories/:id`, `DELETE /categories/:id`.

- [ ] **Step 2: Confirm `CategoryFormSlideover.spec.ts`**

Run `grep -n "mock/categories" frontend/test/nuxt/CategoryFormSlideover.spec.ts`. If it imports `FISCAL_GROUPS`/`isBuildingGroup`/`formatThousands`/`parseThousands` from `~/mock/categories`, change that import to `~/constants/categoryMeta` (the functions are identical). If it has no such import, leave it untouched. Run `pnpm test -- CategoryFormSlideover` → must stay green (the form component doesn't call `useCategories`, so it needs no API stub).

- [ ] **Step 3: Rewrite `master-categories.spec.ts`**

Overwrite `frontend/test/nuxt/master-categories.spec.ts` entirely. Mock `~/composables/useApiClient` with a `request(path, opts)` handler that returns `{ data: CATEGORIES }` for `GET /categories/tree` and captures create/update/delete (method+path+body). Use `// @vitest-environment nuxt` + an admin session with `['*']`. Fixtures: a parent + child (child has `parent_id` = parent.id) + a couple more, covering tangible/intangible + a building group. Cover (assert REAL rendered text / captured bodies — NO hollow checks):
- Loaded rows render; the child row is **indented** (its `name-cell` carries the `ps-6` indent / corner-down-right icon — assert the child appears AND that the page's `orderedRows` places the child immediately after its parent, or assert the indent marker is present for the child row).
- Class filter (`filterClass='intangible'` via `wrapper.vm`) narrows the table to intangible rows only.
- Fiscal-group filter narrows correctly.
- `activeOnly` hides inactive rows.
- Parent picker (`parentOptions` via `wrapper.vm` when editing a parent) EXCLUDES the row itself and its descendants.
- Create → captured `POST /categories` body has the expected fields (name/asset_class/parent_id/…); edit → `PUT /categories/:id`; delete (confirm) → `DELETE /categories/:id`.
- Load-error: `GET /categories/tree` rejects → the page shows the load-error block + retry.

Use the `wrapper.vm as unknown as {...}` ref-mutation technique (the page `defineExpose`s `filterClass`, `filterGroup`, `activeOnly`, `orderedRows`, `parentOptions`, `openEdit`, etc.). Assert real behavior.

- [ ] **Step 4: Run the target test + full suite**

Run (from `frontend/`): `pnpm test -- master-categories` then `pnpm test`
Expected: target PASS; whole suite green. **Confirm the exit code is 0** (the #40 lesson — the only `useCategories` consumer is this page, now stubbed).

- [ ] **Step 5: Commit**

```bash
git add frontend/test/nuxt/master-categories.spec.ts
git commit -m "test(categories): component test against stubbed /categories/tree"
```
(Include `CategoryFormSlideover.spec.ts` in the add if Step 2 changed it.)

---

### Task 5: E2E + delete mock + mockup + PROGRESS + full gate

**Files:**
- Modify (rewrite): `frontend/e2e/categories.spec.ts` (pre-existing, mock-based)
- Delete: `frontend/app/mock/categories.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Delete the orphaned mock + verify no importers**

Run (from repo root): `grep -rn "mock/categories" frontend/app frontend/test` (exclude the file itself).
After Tasks 2-4 the importers (`useCategories.ts` rewritten, `CategoryFormSlideover.vue` repointed to constants, `categories-mock.spec.ts` deleted) no longer reference it. If ZERO importers remain, `git rm frontend/app/mock/categories.ts`. (The `mock/index.ts` barrel does NOT export categories — confirmed — so no barrel edit.) If anything still imports it, repoint to `~/constants/categoryMeta` and report. Run `pnpm typecheck` to confirm no dangling import.

- [ ] **Step 2: Rewrite the e2e**

Read `frontend/e2e/helpers.ts` (`login()`) + the CURRENT `frontend/e2e/categories.spec.ts` (mock-based, to be replaced) + a robust wired e2e (`frontend/e2e/master-reference.spec.ts`). Rewrite `frontend/e2e/categories.spec.ts` against the real backend (seeded admin has `masterdata.global.manage`): login → `/master/categories`; create a parent category (Add → fill name + required fields → Simpan) and assert the row appears; create a child category selecting the parent via the parent USelect (trigger-click + `role="option"` — NEVER `selectOption`), assert the child row appears indented under the parent. ROBUST locators only: text/role + `data-testid`; NO `selectOption`, NO `isVisible()`/`isEnabled()` snapshot booleans driving control flow, NO silent `if(...)return`, NO `.first()`/`.last()` on broad `div`/`button` filters, NO `getByText(...,{exact:false})` that can match multiple elements (use `{exact:true}` for short labels; if a control's accessible name includes extra text like a badge, target a `data-testid` instead). You likely CANNOT run `pnpm test:e2e` here (needs full stack); ensure it compiles + lints; CI runs it. State that in your report.

- [ ] **Step 3: Mockup fidelity comparison**

Read `docs/design/Kategori Aset.dc.html` + the built `frontend/app/pages/master/categories.vue` + `CategoryFormSlideover.vue`. Verify the indented table (8 columns), filter bar (search + 2 selects + active toggle), and the 4-section slideover form match 1:1. All fields are already aligned to the backend, so NO field is dropped — there should be no approved deviation. Fix any genuine deviation; report the result.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`: mark Kategori Aset ✅ wired to `/api/v1/categories` (CRUD + `GET /categories/tree` for the full unpaginated set; client-side tree/filter/pagination retained). Refresh "▶ Next session — start here" → next master-data sub-project = **Pegawai** (scope `employees` + FK pickers; the employees page is a `useReference` consumer already stubbed in #40). Don't fabricate status for unrelated screens.

- [ ] **Step 5: Full gate (backend + frontend)**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./...`, then `go test -tags=integration ./...`, then `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` (adjust ruleset path to repo-root `.spectral.yaml`).
Run (from `frontend/`): `pnpm lint && pnpm typecheck && pnpm test && pnpm build`.
Expected: all green. **Confirm `pnpm test` exits 0** (not just "N passed") — trace any unhandled rejection to its file. (Integration needs dev Postgres. E2E runs in CI.)

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/categories.spec.ts docs/PROGRESS.md
git rm frontend/app/mock/categories.ts
git commit -m "test(categories): e2e + drop mock; wire Kategori Aset end-to-end"
```

---

## Self-Review

**Spec coverage:**
- §2.1 `ListCategoryTree` query → Task 1. ✓
- §2.2 service/handler/route `GET /categories/tree` (flat `{data}`, `/tree` before `/:id`) + openapi + tests (>100 no-cap, soft-delete exclusion, parent passthrough) → Task 1. ✓
- §3.1 `useCategories` HTTP rewrite + `tree()` → Task 2. ✓
- §3.2 page → `tree()` + load-error/retry → Task 3. ✓
- §3.3 utils → `constants/categoryMeta` + repoint `CategoryFormSlideover` → Task 3 (+ Task 4 confirms the form test). ✓
- §3.4 `Category.updated_at?` + i18n `non_susut` → Task 3. ✓
- §4 tests (backend tree, unit, component, e2e) → Tasks 1/2/4/5. ✓
- §5 done (delete mock, mockup, PROGRESS, full-gate exit-0) → Task 5. ✓
- §6 risks (route order, #40 consumer-stub, numeric string, non_susut) → handled in Tasks 1/3/4 + Global Constraints.

**Placeholder scan:** Tasks 4 & 5 give explicit assertion lists + "read X first" pointers (settings-users harness, helpers, master-reference e2e, mockup) with concrete scenarios; the OpenAPI step names the exact addition + says reuse the existing schema `$ref`. No "TODO"/"add validation"/"similar to".

**Type consistency:** `useCategories()→{list,get,create,update,remove,tree}`; `tree():Promise<Category[]>`; `CategoryInput=Omit<Category,'id'|'created_at'|'updated_at'>`; `Category.updated_at?:string|null`; constants `FISCAL_GROUPS`/`isBuildingGroup`/`formatThousands`/`parseThousands` consistent across Tasks 2/3/4. Backend `category.Service.Tree(ctx)→[]sqlc.MasterdataCategory`; handler `tree` → `{data:[]Response}`; route `/tree` before `/:id`. The page consumes `tree()` (Task 2) and `defineExpose`s the refs the component test (Task 4) drives. All consistent.
