# Tech-Debt Sweep #2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the documented tech-debt items in PROGRESS.md candidate (c): a `GET /offices/tree` endpoint that kills the office-hierarchy `limit:100` truncations, server-side user-list filters, a real approval badge count, and a batch of polish fixes.

**Architecture:** Four independent parts (A offices/tree, B user filters, C badge count, D polish), mirroring tech-debt sweep #1. Backend is Go/Gin modular monolith with sqlc; frontend is Nuxt 4 SPA with Pinia + composables. Each task is TDD, small, independently committable/reviewable.

**Tech Stack:** Go 1.25 + Gin + pgx + sqlc; Nuxt 4 + Vue 3 + Pinia + Nuxt UI (`U*`); Vitest + Playwright; OpenAPI 3.1 (hand-maintained, Spectral-linted).

**Spec:** `docs/superpowers/specs/2026-07-13-tech-debt-sweep-2-design.md`

## Global Constraints

- Branch: `feat/tech-debt-sweep-2` (already created; spec already committed there).
- No `Co-Authored-By` / AI attribution in commits.
- Conventional Commits with scope: `feat(offices):`, `feat(user):`, `feat(approval):`, `fix(a11y):`, `fix(security):`, `test(user):`, `docs(...)`, etc.
- Backend: don't hand-edit `backend/db/sqlc/` — edit `db/queries/*.sql` then run `sqlc generate` (from `backend/`).
- Backend authz: every endpoint enforces permission + data scope on read **and** write. New `/offices/tree` and `/requests/inbox/count` mirror the scope/permission posture of their sibling list endpoints.
- Money/numeric columns are Go `string` (sqlc override).
- Frontend: `U*` Nuxt UI components only; theme via semantic tokens; i18n mandatory — every new user-facing string goes in `i18n/locales/{id,en}.json` and is referenced via `$t`/`t`. ESLint: no trailing commas, 1tbs.
- Frontend API access via `useApiClient()` / composables — never hardcode backend URL.
- Keep `backend/api/openapi.yaml` in sync with every route change (Spectral must pass).
- List endpoints return `{data, total, limit, offset}` with `limit` clamped 1–100.
- Verification gate (run before declaring the branch done): backend `go build ./... && go vet ./... && go test ./...` and `go test -tags=integration ./... -p 1` (from `backend/`); Spectral lint; frontend `pnpm lint && pnpm typecheck && pnpm test && pnpm build` (from `frontend/`); affected e2e specs.
- On completion, update `docs/PROGRESS.md` (item 51 candidate (c)) with a one-line note + deferrals.

---

## Task 1 — D3: `useReference.get(key, id)`

Single-item reference fetch. Consumed by Task 4's brand/model picker `resolveFn`.

**Files:**
- Modify: `frontend/app/composables/api/useReference.ts`
- Test: `frontend/test/nuxt/use-reference.spec.ts` (create if absent; else add a case)

**Interfaces:**
- Produces: `useReference().get(key: ReferenceKey, id: string): Promise<ReferenceRow>` → `GET /{key}/{id}`.

- [ ] **Step 1: Write the failing test.** In `frontend/test/nuxt/use-reference.spec.ts`, add a spec that mocks `useApiClient().request` and asserts `get('brands','abc')` calls `request('/brands/abc')` and returns the row. Follow the existing composable-test style in `frontend/test/` (spy on `useApiClient`).

```ts
// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
// mock useApiClient to capture the path; assert get('brands', 'abc') → request('/brands/abc')
```

- [ ] **Step 2: Run it, verify it fails.** Run: `pnpm test -- use-reference` → FAIL (`get` is not a function).
- [ ] **Step 3: Implement.** Add to `useReference.ts` before the `return`:

```ts
async function get(key: ReferenceKey, id: string): Promise<ReferenceRow> {
  return request<ReferenceRow>(`/${key}/${id}`)
}
```

  and add `get` to the returned object: `return { list, get, create, update, remove }`.

- [ ] **Step 4: Run test, verify pass.** Run: `pnpm test -- use-reference` → PASS.
- [ ] **Step 5: Commit.** `git add frontend/app/composables/api/useReference.ts frontend/test/nuxt/use-reference.spec.ts && git commit -m "feat(reference): add useReference.get(key, id) single-item fetch"`

---

## Task 2 — A backend: `GET /offices/tree`

Unbounded, scope-filtered office list so the frontend can build a complete tree past 100 rows.

**Files:**
- Modify: `backend/db/queries/offices.sql` (add `ListOfficesTree`)
- Regenerate: `backend/db/sqlc/` via `sqlc generate`
- Modify: `backend/internal/masterdata/office/service.go` (add `Tree`)
- Modify: `backend/internal/masterdata/office/handler.go` (add `tree`)
- Modify: `backend/internal/masterdata/office/routes.go` (add route)
- Modify: `backend/api/openapi.yaml` (add `/offices/tree`)
- Test: `backend/internal/masterdata/office/office_integration_test.go`

**Interfaces:**
- Produces: `Service.Tree(ctx, all bool, ids []uuid.UUID) ([]sqlc.MasterdataOffice, error)`; HTTP `GET /api/v1/offices/tree` → `{ "data": Response[], "total": int }` (flat, no pagination), auth-only read, office data-scope enforced.

- [ ] **Step 1: Write the failing integration test.** In `office_integration_test.go` (build tag `//go:build integration`), add `TestOffice_Tree_ReturnsFullScopedSetNoLimit`: seed >100 offices in one scope (e.g. 105 under the caller's subtree), call `GET /api/v1/offices/tree` as a global-scope caller, assert `len(data) >= 105` (proves no 100 cap). Add `TestOffice_Tree_RespectsSubtreeScope`: seed a child office under the caller's office and one office outside; call as a subtree-scoped caller; assert the in-scope child is present and the out-of-scope office is absent (mutation-testable). Follow the existing test harness in this file for seeding + request helpers.
- [ ] **Step 2: Run, verify fail.** Run (from `backend/`): `go test -tags=integration ./internal/masterdata/office/ -run TestOffice_Tree` → FAIL (route 404 / undefined).
- [ ] **Step 3a: Add the query.** Append to `backend/db/queries/offices.sql`:

```sql
-- name: ListOfficesTree :many
-- Full scoped office set (no pagination) for building the office hierarchy tree
-- client-side. Mirrors ListOffices' scope filter but without LIMIT/OFFSET/search.
SELECT * FROM masterdata.offices
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY name;
```

- [ ] **Step 3b: Regenerate sqlc.** Run (from `backend/`): `sqlc generate`. Confirm `ListOfficesTree` + `ListOfficesTreeParams` appear in `db/sqlc/`.
- [ ] **Step 3c: Service.** Add to `office/service.go`:

```go
// Tree returns the full scoped office set (unbounded) for building the hierarchy tree.
func (s *Service) Tree(ctx context.Context, all bool, ids []uuid.UUID) ([]sqlc.MasterdataOffice, error) {
	return s.q.ListOfficesTree(ctx, sqlc.ListOfficesTreeParams{AllScope: all, OfficeIds: ids})
}
```

- [ ] **Step 3d: Handler.** Add to `office/handler.go` (mirror `mapList`):

```go
func (h *Handler) tree(c *gin.Context) {
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	rows, err := h.svc.Tree(c.Request.Context(), all, ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list office tree"})
		return
	}
	data := make([]Response, 0, len(rows))
	for _, o := range rows {
		data = append(data, toResponse(o))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}
```

- [ ] **Step 3e: Route.** In `office/routes.go`, add after the `/map` line: `g.GET("/tree", authMW, h.tree)`.
- [ ] **Step 3f: OpenAPI.** In `backend/api/openapi.yaml`, add a `/api/v1/offices/tree` GET path (auth, tag Offices) returning an object `{ data: array of Office, total: integer }`. Reuse the existing `Office` schema.
- [ ] **Step 4: Run tests, verify pass.** Run: `go test -tags=integration ./internal/masterdata/office/ -run TestOffice_Tree` → PASS. Then `go build ./... && go vet ./...` → clean. Spectral: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` → 0 errors.
- [ ] **Step 5: Commit.** `git add backend/db/queries/offices.sql backend/db/sqlc backend/internal/masterdata/office backend/api/openapi.yaml && git commit -m "feat(offices): add GET /offices/tree (unbounded scoped set for hierarchy)"`

---

## Task 3 — A frontend: consume `/offices/tree`, fix office name-maps

**Files:**
- Modify: `frontend/app/composables/api/useOffices.ts` (add `tree`)
- Modify: `frontend/app/pages/master/offices.vue:209`
- Modify: `frontend/app/pages/transfers.vue:133`
- Modify: `frontend/app/pages/disposals.vue:68`
- Test: `frontend/test/nuxt/use-offices.spec.ts` (add a `tree` case), and the existing master-offices component/e2e specs must stay green.

**Interfaces:**
- Consumes: `GET /offices/tree` from Task 2.
- Produces: `useOffices().tree(): Promise<Office[]>`.

- [ ] **Step 1: Write the failing test.** In `use-offices.spec.ts`, add a case asserting `tree()` calls `request('/offices/tree')` and returns the `data` array.
- [ ] **Step 2: Run, verify fail.** Run: `pnpm test -- use-offices` → FAIL.
- [ ] **Step 3a: Add `tree` to `useOffices.ts`:**

```ts
async function tree(): Promise<Office[]> {
  const res = await request<{ data: Office[], total: number }>('/offices/tree')
  return res.data
}
```

  add `tree` to the returned object.

- [ ] **Step 3b: master/offices.vue.** At line ~209 replace `const res = await api.list({ limit: 100 })` + `offices.value = res.data` with `offices.value = await api.tree()`. Keep everything else (`buildTree`, `parentOptions`, `filteredNodes`) unchanged.
- [ ] **Step 3c: transfers.vue:133.** Replace `officesApi.list({ limit: 100 })` (the name-map load in `loadLookups`) with `officesApi.tree()`; adapt to the returned flat array (it currently reads `.data` — `tree()` already returns the array, so drop the `.data`).
- [ ] **Step 3d: disposals.vue:68.** Same change for the `loadOffices` name-map: `officesApi.tree()` returning the flat array.
- [ ] **Step 4: Run tests + typecheck.** Run: `pnpm test -- use-offices` → PASS; `pnpm typecheck` → clean; `pnpm test -- offices` (component/page specs) → PASS.
- [ ] **Step 5: Commit.** `git add frontend/app/composables/api/useOffices.ts frontend/app/pages/master/offices.vue frontend/app/pages/transfers.vue frontend/app/pages/disposals.vue frontend/test && git commit -m "feat(offices): build office tree + name-maps from GET /offices/tree (no 100 cap)"`

---

## Task 4 — A frontend: migrate `assets/index` brand/model filters to `AsyncSearchPicker`

Depends on Task 1 (`useReference.get`).

**Files:**
- Modify: `frontend/app/composables/usePickerSource.ts` (add a reference picker adapter if none exists)
- Modify: `frontend/app/pages/assets/index.vue` (replace brand/model `USelect` filters + drop `limit:100` fetches at lines ~194-195)
- Test: `frontend/test/nuxt/assets-index-filters.spec.ts` (create) or extend the existing assets/index spec

**Interfaces:**
- Consumes: `useReference().get` (Task 1), `AsyncSearchPicker` (`modelValue`/`searchFn`/`resolveFn`/`placeholder`).
- Produces: `useReferencePicker(key: ReferenceKey)` in `usePickerSource.ts` returning `{ searchFn, resolveFn }`.

- [ ] **Step 1: Write the failing test.** Add a component test mounting `assets/index` (or a focused harness) asserting the brand filter is an `AsyncSearchPicker` whose `searchFn` hits `reference.list('brands', {search, limit:20})` and `resolveFn` hits `reference.get('brands', id)`; selecting a brand sets the filter and triggers a list reload with `brand_id`.
- [ ] **Step 2: Run, verify fail.** Run: `pnpm test -- assets-index` → FAIL.
- [ ] **Step 3a: Reference adapter.** In `usePickerSource.ts`, add (mirroring `useOfficePicker`):

```ts
export function useReferencePicker(key: ReferenceKey) {
  const api = useReference()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list(key, { search: term, limit: 20 })
      return res.data.map(r => ({ id: r.id, label: r.name }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      try {
        const r = await api.get(key, id)
        return { id: r.id, label: r.name }
      } catch {
        return null
      }
    }
  }
}
```

  (Adjust `r.name` to the actual `ReferenceRow` label field.)

- [ ] **Step 3b: assets/index.vue.** Remove the `referenceApi.list('brands', {limit:100})` and `('models', {limit:100})` fetches (~lines 194-195) and the `brandOptions`/`modelOptions` computed. Replace the brand and model filter `USelect`s with `<AsyncSearchPicker>` bound to `fKat`/`fClass` (brand/model filter refs), each fed by `useReferencePicker('brands')` / `useReferencePicker('models')`, `clearable`, with `data-testid`. Keep the existing filter-watcher wiring so selecting reloads the list.
- [ ] **Step 4: Run tests + lint.** Run: `pnpm test -- assets-index` → PASS; `pnpm lint && pnpm typecheck` → clean.
- [ ] **Step 5: Commit.** `git add frontend/app/composables/usePickerSource.ts frontend/app/pages/assets/index.vue frontend/test && git commit -m "feat(assets): brand/model filters as AsyncSearchPicker (drop limit:100)"`

---

## Task 5 — B backend: user list server-side filters (role/office/status)

**Files:**
- Modify: `backend/db/queries/users.sql` (`ListUsers` + `CountUsers`)
- Regenerate: `backend/db/sqlc/`
- Modify: `backend/internal/user/service.go` (`List` signature)
- Modify: `backend/internal/user/handler.go` (`list`)
- Modify: `backend/api/openapi.yaml` (listUsers params)
- Test: `backend/internal/user/handler_integration_test.go` (filter cases)

**Interfaces:**
- Produces: `Service.List(ctx, search string, roleID, officeID *uuid.UUID, status *string, limit, offset int32) ([]sqlc.IdentityUser, int64, error)`; `GET /users` accepts `role_id`, `office_id`, `status` query params (empty = no filter; malformed uuid = 400).

- [ ] **Step 1: Write the failing tests.** In `handler_integration_test.go`, seed users with distinct role/office/status, then assert `GET /users?role_id=…` / `?office_id=…` / `?status=inactive` each narrow the result set, and a combined filter works. (You may need the write-path/request helpers from Task 6 — if Task 6 lands first, reuse them; otherwise seed via raw SQL like `seedUserDirect`.)
- [ ] **Step 2: Run, verify fail.** Run: `go test -tags=integration ./internal/user/ -run Filter` → FAIL.
- [ ] **Step 3a: Queries.** In `users.sql`, add nullable-arg predicates to BOTH `ListUsers` and `CountUsers` WHERE clauses:

```sql
  AND (sqlc.narg(role_id)::uuid IS NULL OR role_id = sqlc.narg(role_id))
  AND (sqlc.narg(office_id)::uuid IS NULL OR office_id = sqlc.narg(office_id))
  AND (sqlc.narg(status)::shared.user_status IS NULL OR status = sqlc.narg(status))
```

- [ ] **Step 3b: Regenerate.** Run: `sqlc generate`. `ListUsersParams`/`CountUsersParams` gain `RoleID *uuid.UUID`, `OfficeID *uuid.UUID`, `Status shared.NullUserStatus` (or the generated nullable type — check).
- [ ] **Step 3c: Service.** Update `List` to accept + pass `roleID, officeID *uuid.UUID, status *string`, converting `status` into the generated nullable enum type. Pass through to both `ListUsers` and `CountUsers`.
- [ ] **Step 3d: Handler.** In `handler.go` `list`, parse the three query params: `role_id`/`office_id` via `uuid.Parse` (empty → nil; non-empty invalid → `c.JSON(400, …)` and return); `status` validated against `active|inactive|suspended` (invalid → 400). Pass to `svc.List`.
- [ ] **Step 3e: OpenAPI.** Add `role_id` (uuid), `office_id` (uuid), `status` (enum) query params to `listUsers`.
- [ ] **Step 4: Run tests.** Run: `go test -tags=integration ./internal/user/ -run Filter` → PASS; `go build ./... && go vet ./...` clean; Spectral 0 errors.
- [ ] **Step 5: Commit.** `git add backend/db/queries/users.sql backend/db/sqlc backend/internal/user backend/api/openapi.yaml && git commit -m "feat(user): server-side role/office/status filters on GET /users"`

---

## Task 6 — D5: user write-path handler tests (create/update/delete)

**Files:**
- Modify: `backend/internal/user/handler_integration_test.go`

**Interfaces:**
- Consumes: existing `user` handlers (`create`/`update`/`delete`) + the test harness.
- Produces: a POST/PUT/DELETE-with-body request helper reusable within the test file.

- [ ] **Step 1: Write the tests.** Add:
  - `doJSON(method, path, body)` helper (the current `doRequest` is GET-only) returning `(*httptest.ResponseRecorder)`.
  - `TestUser_Create_Success` (POST valid → 201, row exists), `TestUser_Create_ValidationError` (missing required → 400), `TestUser_Create_DuplicateEmail` (→ 409 via `svcError`).
  - `TestUser_Update_Success` (PUT → 200, fields changed), `TestUser_Update_NotFound` (→ 404).
  - `TestUser_Delete_Success` (DELETE → 204, soft-deleted), `TestUser_Delete_NotFound` (→ 404).
- [ ] **Step 2: Run, verify they fail first (red) where behavior is missing**, then confirm they pass against current handlers. Run: `go test -tags=integration ./internal/user/ -run 'TestUser_(Create|Update|Delete)'`.
- [ ] **Step 3: Adjust** any handler/service error-mapping only if a test surfaces a real bug (else no prod change — this task is coverage).
- [ ] **Step 4: Run tests, verify pass.** Run: `go test -tags=integration ./internal/user/` → PASS.
- [ ] **Step 5: Commit.** `git add backend/internal/user/handler_integration_test.go && git commit -m "test(user): cover create/update/delete handler write paths"`

---

## Task 7 — B frontend: users filter bar

Depends on Task 5.

**Files:**
- Modify: `frontend/app/composables/api/useUsers.ts` (`list` params)
- Modify: `frontend/app/pages/settings/users.vue` (filter bar)
- Modify: `frontend/i18n/locales/{id,en}.json` (filter labels)
- Test: `frontend/test/nuxt/users-filters.spec.ts` (create) + e2e assertion

**Interfaces:**
- Consumes: `GET /users?role_id&office_id&status` (Task 5), `useOfficePicker` + `AsyncSearchPicker`.
- Produces: `useUsers().list({ search, roleId, officeId, status, limit, offset })`.

- [ ] **Step 1: Write the failing test.** Component test: mounting `users.vue`, selecting a status in the status `USelect` issues a `list` call with `status` set and `offset` reset to 0; selecting a role issues `role_id`.
- [ ] **Step 2: Run, verify fail.** Run: `pnpm test -- users-filters` → FAIL.
- [ ] **Step 3a: useUsers.ts.** Extend `list` params + query string with `roleId → role_id`, `officeId → office_id`, `status`.
- [ ] **Step 3b: users.vue.** Add to the filter bar (`~lines 250-258`): role `USelect` (reuse `roleFormOptions` from `lookups()`), office `AsyncSearchPicker` (`useOfficePicker`, clearable), status `USelect` (options active/inactive/suspended with i18n labels). Add reactive refs `fRole`/`fOffice`/`fStatus`, watchers that reset `offset` and refetch (mirror the search watcher). Add `data-testid`s.
- [ ] **Step 3c: i18n.** Add `settings.users.filter.role|office|status|allRoles|allStatuses` (and status value labels) to both locales.
- [ ] **Step 4: Run tests + lint.** Run: `pnpm test -- users-filters` PASS; `pnpm lint && pnpm typecheck` clean.
- [ ] **Step 5: Commit.** `git add frontend/app/composables/api/useUsers.ts frontend/app/pages/settings/users.vue frontend/i18n/locales frontend/test && git commit -m "feat(user): users list filter bar (role/office/status)"`

---

## Task 8 — C backend: `GET /requests/inbox/count`

**Files:**
- Modify: `backend/internal/approval/routes.go`
- Modify: `backend/internal/approval/handler.go` (add `inboxCount`)
- Modify: `backend/api/openapi.yaml`
- Test: `backend/internal/approval/*_integration_test.go`

**Interfaces:**
- Produces: `GET /api/v1/requests/inbox/count` (gate `request.decide`) → `{ "count": int }`, equals `len` of `GET /requests/inbox` data for the same caller.

- [ ] **Step 1: Write the failing test.** In the approval integration tests, seed an inbox for a decider, assert `GET /requests/inbox/count` `count` equals the length of `GET /requests/inbox` `data`; assert a caller without `request.decide` is rejected (403).
- [ ] **Step 2: Run, verify fail.** Run: `go test -tags=integration ./internal/approval/ -run InboxCount` → FAIL.
- [ ] **Step 3a: Handler.** Add to `approval/handler.go`:

```go
// inboxCount handles GET /requests/inbox/count — the pending count for the sidebar badge.
func (h *Handler) inboxCount(c *gin.Context) {
	caller, err := h.callerFromCtx(c)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	rows, err := h.svc.Inbox(c, caller)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(rows)})
}
```

- [ ] **Step 3b: Route.** In `routes.go`, add after the `/inbox` line: `g.GET("/inbox/count", authMW, decide, h.inboxCount)`.
- [ ] **Step 3c: OpenAPI.** Add `/api/v1/requests/inbox/count` GET returning `{ count: integer }`.
- [ ] **Step 4: Run tests.** Run: `go test -tags=integration ./internal/approval/ -run InboxCount` PASS; `go build ./... && go vet ./...` clean; Spectral 0 errors.
- [ ] **Step 5: Commit.** `git add backend/internal/approval backend/api/openapi.yaml && git commit -m "feat(approval): add GET /requests/inbox/count for the sidebar badge"`

---

## Task 9 — C frontend: inbox store + live sidebar badge

Depends on Task 8.

**Files:**
- Create: `frontend/app/stores/inbox.ts`
- Modify: `frontend/app/composables/api/useApproval.ts` (add `inboxCount`)
- Modify: `frontend/app/components/AppSidebar.vue` (badge from store)
- Modify: `frontend/app/utils/nav.ts` (drop hardcoded `badgeCount: 2`)
- Modify: `frontend/app/pages/approval.vue` (refresh on decide + mount)
- Modify: the app-mount/login hook (where `useAuthApi` sets the session) to call `refresh()`
- Test: `frontend/test/nuxt/inbox-store.spec.ts` (create), `AppSidebar` badge test, `approval.vue` decide-refresh test

**Interfaces:**
- Consumes: `GET /requests/inbox/count`, `useCan()`.
- Produces: `useInboxStore()` (Pinia, `pendingCount: number`, `refresh(): Promise<void>`); `useApproval().inboxCount(): Promise<number>`.

- [ ] **Step 1: Write the failing tests.** (a) inbox-store: `refresh()` calls `inboxCount()` and sets `pendingCount`; when `can('request.decide')` is false, `refresh()` sets 0 without calling the API. (b) AppSidebar: with store `pendingCount = 3`, the `nav.approval` leaf renders a badge "3"; at 0 no badge. (c) approval decide: after `decide()`, `inboxStore.refresh()` is called.
- [ ] **Step 2: Run, verify fail.** Run: `pnpm test -- inbox` → FAIL.
- [ ] **Step 3a: Store.** Create `stores/inbox.ts`:

```ts
import { defineStore } from 'pinia'

export const useInboxStore = defineStore('inbox', {
  state: () => ({ pendingCount: 0 }),
  actions: {
    async refresh() {
      const can = useCan()
      if (!can('request.decide')) { this.pendingCount = 0; return }
      try { this.pendingCount = await useApproval().inboxCount() } catch { /* keep last */ }
    }
  }
})
```

- [ ] **Step 3b: useApproval.ts.** Add:

```ts
async function inboxCount(): Promise<number> {
  const res = await request<{ count: number }>('/requests/inbox/count')
  return res.count
}
```

  and export it.

- [ ] **Step 3c: AppSidebar.vue.** For the `nav.approval` and `nav.approvalStaff` leaves, render the badge from `useInboxStore().pendingCount` (via a helper e.g. `badgeFor(item)` that returns the store count when `item.labelKey` is `'nav.approval'`/`'nav.approvalStaff'`, else `item.badgeCount`). Hide the badge when the value is 0.
- [ ] **Step 3d: nav.ts.** Remove `badgeCount: 2` from the `approvalStaff` item (line ~175).
- [ ] **Step 3e: Refresh triggers.** In `approval.vue` `decide()` add `await useInboxStore().refresh()` after `loadTab()`; in `onMounted` call it. Add a `refresh()` call at app mount / after login where the session is established (near `useAuthApi` `setSession`), guarded by the store's own permission check.
- [ ] **Step 4: Run tests + lint.** Run: `pnpm test -- inbox approval AppSidebar` PASS; `pnpm lint && pnpm typecheck` clean.
- [ ] **Step 5: Commit.** `git add frontend/app/stores/inbox.ts frontend/app/composables/api/useApproval.ts frontend/app/components/AppSidebar.vue frontend/app/utils/nav.ts frontend/app/pages/approval.vue frontend/test && git commit -m "feat(approval): live pending-approval sidebar badge via inbox store"`

---

## Task 10 — D1: `AsyncSearchPicker` a11y (combobox + keyboard nav)

**Files:**
- Modify: `frontend/app/components/AsyncSearchPicker.vue`
- Test: `frontend/test/nuxt/async-search-picker.spec.ts` (create or extend)

**Interfaces:** unchanged public props/emits; adds internal keyboard handling + aria.

- [ ] **Step 1: Write the failing tests.** Assert: input has `role="combobox"`, `aria-expanded` reflects `isOpen`; `<ul>` has `role="listbox"`, each `<li>` `role="option"` + `aria-selected`; ArrowDown moves `activeIndex` and sets `aria-activedescendant`; Enter selects the active option (emits `update:modelValue`); Escape closes; loading/empty containers have `role="status"`.
- [ ] **Step 2: Run, verify fail.** Run: `pnpm test -- async-search-picker` → FAIL.
- [ ] **Step 3: Implement.** Add an `activeIndex` ref (reset to -1 on new results/open). On the `UInput`: `role="combobox"`, `:aria-expanded="isOpen"`, `aria-haspopup="listbox"`, `:aria-controls="listboxId"`, `:aria-activedescendant="activeIndex>=0 ? optionId(activeIndex) : undefined"`, and `@keydown` handler: ArrowDown/ArrowUp move `activeIndex` within `results` (open if closed), Enter → `select(results[activeIndex])` (guard bounds), Escape → close + refocus input, Home/End → first/last. On `<ul>`: `:id="listboxId"`, `role="listbox"`. On each `<li>`: `:id="optionId(i)"`, `role="option"`, `:aria-selected="i===activeIndex"`, active-highlight class when `i===activeIndex`. Add `role="status" aria-live="polite"` to the loading and empty `<div>`s. Generate `listboxId` via `useId()`.
- [ ] **Step 4: Run tests + existing picker consumers.** Run: `pnpm test -- async-search-picker` PASS; `pnpm test` (full) → the transfers/employees/assets picker tests still pass; `pnpm lint && pnpm typecheck` clean.
- [ ] **Step 5: Commit.** `git add frontend/app/components/AsyncSearchPicker.vue frontend/test && git commit -m "fix(a11y): keyboard nav + ARIA combobox/listbox for AsyncSearchPicker"`

---

## Task 11 — D2: failure-safe Data-Scope e2e cleanup

**Files:**
- Modify: `frontend/e2e/helpers.ts` (authenticated API helper)
- Modify: `frontend/e2e/settings.spec.ts` (afterEach restore)

**Interfaces:**
- Produces: `apiContext(playwright): Promise<APIRequestContext>` (or a token-bearing helper) usable to `PUT`/read `/api/v1/authz` in `afterEach`.

- [ ] **Step 1: Write the helper + afterEach.** In `helpers.ts`, add a helper that logs in via the API (`POST /api/v1/auth/login` with `EMAIL`/`PASSWORD`) and returns an `APIRequestContext` (or bearer token) for authenticated requests. In `settings.spec.ts`, add `afterEach` to the Data-Scope describe that restores the Superadmin **Default** (`*`) scope policy to `global` via `/api/v1/authz` regardless of test outcome; apply the same pattern to restore the field-permission `purchase_cost` toggle in its describe.
- [ ] **Step 2: Verify the revert runs on failure.** Temporarily make an assertion in the Data-Scope test fail; confirm the `afterEach` still restores `global` (check via API); then restore the assertion.
- [ ] **Step 3: Keep the in-body revert only as a no-op-safe fast path** or remove it (the `afterEach` is now authoritative). Ensure no double-toggle leaves the wrong state.
- [ ] **Step 4: Run the spec.** Run (backend stack up, seeded admin, `RATELIMIT_ENABLED=false`): `pnpm test:e2e -- settings` → PASS, and the Superadmin `*` policy is `global` afterward.
- [ ] **Step 5: Commit.** `git add frontend/e2e/helpers.ts frontend/e2e/settings.spec.ts && git commit -m "test(e2e): failure-safe afterEach revert for data-scope/field-perm settings"`

---

## Task 12 — D4: `assets/index` resetFilters double-fetch

**Files:**
- Modify: `frontend/app/pages/assets/index.vue` (`resetFilters` + watchers, ~lines 135-143, 200-216)
- Test: `frontend/test/nuxt/assets-index-reset.spec.ts` (create) or extend

- [ ] **Step 1: Write the failing test.** Mount `assets/index`, spy on the list fetch, navigate to page ≥2 with a filter set, call `resetFilters()`, assert the list fetch fires **exactly once**.
- [ ] **Step 2: Run, verify fail.** Run: `pnpm test -- assets-index-reset` → FAIL (fires twice).
- [ ] **Step 3: Implement.** Introduce a `resetting` guard: set it true at the start of `resetFilters`, mutate all refs, then in `nextTick` set it false and call `load()` once; make the filter-watcher and the `page`-watcher early-return while `resetting` is true. (Or equivalent single-fetch consolidation.) Keep the `seq` guard.
- [ ] **Step 4: Run tests.** Run: `pnpm test -- assets-index` PASS (reset + filter specs); `pnpm typecheck` clean.
- [ ] **Step 5: Commit.** `git add frontend/app/pages/assets/index.vue frontend/test && git commit -m "fix(assets): single fetch on resetFilters (was double)"`

---

## Task 13 — D6: audit summary entity localization

**Files:**
- Modify: `frontend/app/composables/api/useAudit.ts` (`toSummary`) and/or `frontend/app/pages/settings/audit.vue`
- Test: `frontend/test/unit/use-audit.spec.ts` (or the existing audit spec)

- [ ] **Step 1: Write the failing test.** Assert the summary for an `update` on `entity_type='assets'` renders the localized entity label (e.g. `settings.audit.entity.assets`), not the raw key `assets`.
- [ ] **Step 2: Run, verify fail.** Run: `pnpm test -- audit` → FAIL.
- [ ] **Step 3: Implement.** Make `toSummary` localize the entity: pass an `entityLabel(key)` translator into the composable (or move summary derivation into `audit.vue` where `entityLabel` at lines 37-40 is in scope). Replace `entity: d.entity_type` with the localized label; keep (or shorten) `id`. Ensure `entityLabel` falls back to the raw key when no i18n entry exists (via `te`).
- [ ] **Step 4: Run tests + lint.** Run: `pnpm test -- audit` PASS; `pnpm lint && pnpm typecheck` clean.
- [ ] **Step 5: Commit.** `git add frontend/app/composables/api/useAudit.ts frontend/app/pages/settings/audit.vue frontend/test && git commit -m "fix(audit): localize entity label in derived summary"`

---

## Final: whole-branch verification + PROGRESS

- [ ] **Backend gate (from `backend/`):** `go build ./... && go vet ./... && go test ./...` and `go test -tags=integration ./... -p 1` → all green. Spectral: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` → 0 errors.
- [ ] **Frontend gate (from `frontend/`):** `pnpm lint && pnpm typecheck && pnpm test && pnpm build` → all green.
- [ ] **E2E (affected):** with the backend stack up + seeded admin + `RATELIMIT_ENABLED=false`: `pnpm test:e2e -- settings master-offices` (and users/approval if specs were added) → green.
- [ ] **PROGRESS.md:** tick item 51 candidate (c); note deferrals (reset-password, force-change-on-next-login, badge polling) and any remaining follow-ups; add a new "Next session — pick the next real step" block (remaining item 51 candidates: (a) notifications, (b) room/floor import targets, (d) Analytics/OLAP). Commit `docs(progress): tech-debt sweep #2 done`.
- [ ] **Finish the branch** via superpowers:finishing-a-development-branch (PR or merge per user preference).
