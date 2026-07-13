# Tech-Debt Sweep #2 — Design

**Date:** 2026-07-13
**Status:** Approved (design)
**Predecessor:** `2026-07-12-tech-debt-sweep-design.md` (sweep #1 — field-perm enforcement + audit enrichment + async pickers)
**Source:** PROGRESS.md item 51 candidate (c) — "the rest of the standing tech-debt list".

## 1. Overview

A second tech-debt sweep bundling four independent parts, mirroring sweep #1's structure
(independent parts, subagent-driven, each task-reviewed). Nothing here is a new product feature;
every part closes a documented gap or follow-up.

- **Part A** — new `GET /offices/tree` endpoint; use it to remove the office-hierarchy `limit:100`
  truncations (master/offices tree completeness + transfer/disposal office name-maps), and migrate the
  `assets/index` brand/model filters off their client-capped `limit:100` selects onto `AsyncSearchPicker`.
- **Part B** — server-side filter params (role / office / status) on `GET /users` + the frontend filter
  bar. **Reset-password is deferred** (product decision — no email infra; out of this sweep).
- **Part C** — a real pending-approval badge count: lightweight `GET /requests/inbox/count` + a global
  inbox store + sidebar wiring, replacing the last hardcoded `badgeCount`. Event-driven refresh, no poll.
- **Part D** — a polish grab-bag: `AsyncSearchPicker` a11y, failure-safe Data-Scope e2e cleanup,
  `useReference.get()`, `assets/index` resetFilters double-fetch, `user` write-path handler tests, and
  audit-summary entity localization.

**Explicitly out of scope (deferred):** reset-password endpoint/UI, force-change-on-next-login, badge
polling. **Resolved-by-restructuring:** item 50's "master/offices *table* server-side pagination"
follow-up — the page is tree-only, so the fix is an unbounded scope-filtered `/offices/tree`, not table
pagination.

## 2. Part A — `GET /offices/tree` + kill office-class `limit:100`

### Problem
`master/offices.vue:209` loads the office hierarchy via `api.list({ limit: 100 })` and builds the tree
client-side (`buildTree`, `offices.vue:135`). Past 100 offices the tree is silently truncated. The same
class of truncation shows up as display-only office name-maps in `transfers.vue:133` and
`disposals.vue:68` (offices beyond 100 render blank/id). Separately, `assets/index.vue:194-195` fetch
`brands`/`models` reference options at `limit:100` into plain client-capped `USelect` filters.

### Backend
- **Query** `db/queries/offices.sql`: add `ListOfficesTree :many`, modeled on `ListOfficesMap`
  (`offices.sql:68-86`) — select all non-deleted offices where
  `sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[])`, `ORDER BY name`, **no LIMIT/OFFSET**.
  Return the full row (same columns as `MasterdataOffice`, incl. `parent_id`) so `toResponse`
  (`dto.go:106`) serializes it unchanged.
- **Service** `office/service.go`: add `Tree(ctx, all bool, ids []uuid.UUID) ([]MasterdataOffice, error)`.
- **Handler** `office/handler.go`: add `tree` — resolve scope via `h.scoped.CallerOfficeScope(c, "offices")`
  (`handler.go:64` pattern), call `Tree`, respond `{ "data": [...], "total": len }` (flat list). No
  pagination. Same auth posture as `list`/`map`.
- **Route** `office/routes.go`: `g.GET("/tree", authMW, h.tree)` alongside `/map`.
- **OpenAPI** `backend/api/openapi.yaml`: add `/api/v1/offices/tree` (GET) returning the office list
  envelope; reuse the existing Office schema.

### Frontend
- `useOffices.ts`: add `tree(): Promise<Office[]>` → `GET /offices/tree`, returning the flat `data` array.
- `master/offices.vue:209`: replace `api.list({ limit: 100 })` with `api.tree()`. `buildTree`,
  `parentOptions`, `childCount`, and `filteredNodes` are unchanged — they already operate on a flat list.
- `transfers.vue:133` and `disposals.vue:68`: repoint the office **name-map** fetch from
  `officesApi.list({ limit: 100 })` to `officesApi.tree()` so office names resolve past 100. (The from/to
  office *pickers* in transfers are already `AsyncSearchPicker` — only the display name-map changes.)
- `assets/index.vue:194-195`: migrate the `brands`/`models` filter selects to `AsyncSearchPicker`, backed
  by a reference picker adapter (server-side `search`, `resolveFn` via `useReference.get()` — Part D3).
  Removes both `limit:100` reference fetches on this page. The office filter here is already async.

### Tests
- Backend integration: `Tree` returns the full scoped set with **no 100-row cap** (seed >100 offices in
  one scope, assert count), respects `office_subtree` expansion (child seeded under caller's office is
  present; out-of-scope office is absent — mutation-testable assertion), and `global` scope returns all.
- Frontend: `useOffices.tree` unit; `master/offices` still renders the tree from `tree()`;
  `assets/index` brand/model `AsyncSearchPicker` component test (search + resolve). Existing
  `master-offices.spec.ts` e2e continues to pass.

## 3. Part B — Users list server-side filters (reset-password deferred)

### Problem
`GET /users` accepts only `search/limit/offset` (`users.sql:3-12`, `handler.go:29-49`). The frontend
filter bar (`users.vue:250-258`) has only a search input — role/office/status filters were removed in an
earlier phase, not stubbed.

### Backend
- **Queries** `db/queries/users.sql`: extend `ListUsers` **and** `CountUsers` (keep the two WHERE clauses
  in sync) with nullable-arg predicates:
  `(sqlc.narg(role_id)::uuid IS NULL OR role_id = sqlc.narg(role_id))`, same for `office_id (uuid)` and
  `status (shared.user_status)`. `sqlc generate`.
- **Service/handler** `user/service.go` `List` + `user/handler.go` `list`: read `c.Query("role_id")`,
  `c.Query("office_id")` (parse to `uuid`, empty → nil), `c.Query("status")` (validate against
  `active|inactive|suspended`, invalid → 400 or ignored — pick **ignore-empty, 400-on-malformed-uuid**).
  Thread the three optional args through to the query.
- **OpenAPI**: add `role_id`, `office_id`, `status` query params to `listUsers` (lines 431-441).

### Frontend
- `useUsers.ts` `list`: add optional `roleId` / `officeId` / `status` params to the query string.
- `users.vue`: add to the filter bar — role `USelect` (reuse `roleFormOptions` from `useUsers.lookups()`
  → `/authz/roles`), office `AsyncSearchPicker` (`useOfficePicker()` already imported for the form),
  status `USelect` (active/inactive/suspended, i18n labels). Each filter has a watcher that resets
  `offset` and refetches (mirror the existing `search` watcher at `users.vue:224-227`).

### Tests
- Backend: covered together with Part D5 (`user` handler write-path tests) plus list-filter assertions
  (filter by role, by office, by status; combined).
- Frontend: `users.vue` filter component test (selecting a filter issues the right query params); e2e
  extends `employees`/users-style flow to assert a role/status filter narrows the list.

## 4. Part C — Real approval badge count

### Problem
The only remaining hardcoded badge is `nav.ts:175` (`badgeCount: 2` on the disabled `approvalStaff`
item). No live count exists. The inbox count is expensive: `approval/service.go:486-524` `Inbox` computes
SoD/tier eligibility per row in Go (N+1 by design), so there is no cheap SQL `COUNT`.

### Backend
- **Route** `approval/routes.go`: add `g.GET("/inbox/count", authMW, decide, h.inboxCount)` (gate
  `request.decide`, same as `/inbox`).
- **Handler** `approval/handler.go`: `inboxCount` calls the **same** `h.svc.Inbox(c, caller)` and returns
  `{ "count": len(rows) }` — skipping the per-row enrichment/field-filter/serialization the full `/inbox`
  does. Guarantees the badge equals what the inbox would show. (No new service method needed; the DB cost
  is shared, only the payload shrinks.)
- **OpenAPI**: add `/api/v1/requests/inbox/count`.

### Frontend
- `stores/inbox.ts`: new Pinia **options-API** store (mirror `stores/auth.ts`), state
  `{ pendingCount: 0 }`, action `async refresh()` calling `useApproval().inboxCount()` (guarded — only
  when `useCan()('request.decide')`, else set 0).
- `useApproval.ts`: add `inboxCount(): Promise<number>` → `GET /requests/inbox/count`, returns `res.count`.
- `AppSidebar.vue`: for the `nav.approval` (superadmin) and `nav.approvalStaff` (staff) leaves, render the
  badge from the store's `pendingCount` (0 → no badge) instead of the static `item.badgeCount`. Remove the
  hardcoded `badgeCount: 2` from `nav.ts:175`.
- **Refresh triggers** (event-driven, no poll): on app mount / after login (near `useAuthApi` session
  set), after each `decide()` in `approval.vue:267-284` (alongside the existing `loadTab()`), and on
  entering `/approval`.

### Tests
- Backend integration: `inbox/count` `count` equals `len(inbox.data)` for the same caller/seed;
  permission gate (a non-`request.decide` caller is rejected).
- Frontend: `stores/inbox` unit (`refresh` writes count, guarded when lacking permission); `AppSidebar`
  renders the store count and hides the badge at 0; `approval.vue` `decide()` calls `inboxStore.refresh()`.

## 5. Part D — Polish grab-bag

### D1 — `AsyncSearchPicker` a11y (`components/AsyncSearchPicker.vue`)
Add proper combobox/listbox semantics + keyboard nav. The `<input>` (line 119) gets
`role="combobox"`, `aria-expanded`, `aria-controls`, `aria-haspopup="listbox"`, `aria-activedescendant`;
the `<ul>` (line 164) gets `role="listbox"` + an `id`; each `<li>` (line 168) gets `role="option"`, a
stable `id`, and `aria-selected`. Add an `activeIndex` ref and `@keydown` handling: ArrowDown/ArrowUp
move the active option (open the popover if closed), Enter selects the active option, Escape closes and
returns focus to the input, Home/End jump to first/last. Highlight the active option visually (not just
hover). Add `role="status"`/`aria-live="polite"` to the loading (line 148) and empty (line 157) states so
async result changes are announced. Component tests assert keyboard selection and the aria wiring.

### D2 — Failure-safe Data-Scope e2e cleanup (`e2e/settings.spec.ts` + `e2e/helpers.ts`)
The Data-Scope test flips the Superadmin **Default** scope policy `global → own` and saves
(`settings.spec.ts:295-309`); the in-body revert (`323-331`) is skipped if any assertion in between fails,
leaving the shared dev-DB Superadmin `*` policy corrupted to `own` (the recurring failure documented in
PROGRESS items 25/26/28). Fix: add a small **authenticated API helper** to `e2e/helpers.ts` (log in, reuse
the session cookie/token via a Playwright `request` context) and move the revert into an **`afterEach`**
that restores the Superadmin default scope to `global` via `/api/v1/authz`, independent of test outcome.
Apply the same `afterEach` restore to the field-permission test's `purchase_cost` toggle (`452-465`).

### D3 — `useReference.get(key, id)` (`composables/api/useReference.ts`)
Add the missing single-item fetch: `get(key, id): Promise<ReferenceRow>` → `GET /{key}/{id}` (the generic
engine already serves this; `update`/`remove` already hit `/{key}/{id}`). Consumed by Part A's brand/model
`AsyncSearchPicker` `resolveFn`.

### D4 — `assets/index` resetFilters double-fetch (`pages/assets/index.vue:135-143,200-216`)
`resetFilters` mutates the filter refs and `page` together, causing the filter-watcher (line 207) and the
`page`-watcher (line 216) to both schedule a `load()` for one user action (the `seq` guard makes it
correct but wastes a round-trip). Dedupe so one reset performs exactly one fetch — e.g. a `resetting`
guard flag consumed by the watchers, or consolidate the reset to set state then call `load()` once while
suppressing the watchers for that tick. Assert single `load()` in a component test.

### D5 — `user` write-path handler tests (`backend/internal/user/handler_integration_test.go`)
Only field-masking GET tests exist. Add integration tests for `create` (POST), `update` (PUT), `delete`
(DELETE): success, validation (400 on bad body), error mapping via `svcError` (e.g. duplicate email →
409), and authz. Add a POST/PUT/DELETE-with-body request helper (current `doRequest` is GET-only). These
tests also cover Part B's list filters (role/office/status).

### D6 — Audit summary entity localization (`composables/api/useAudit.ts` + `pages/settings/audit.vue`)
`toSummary` (`useAudit.ts:79-85`) interpolates the raw backend `entity_type` key (and raw `entity_id`
UUID) into the summary string. Route the entity through the same `entityLabel(key)` localization the page
already uses for the Entity column (`audit.vue:37-40`, `settings.audit.entity.*`) so the summary shows a
localized entity label. Options: pass a translator/label fn into the composable, or compute the summary in
the page where `entityLabel` is in scope. Keep (or shorten) the id. Update the `useAudit` unit tests.

## 6. Dependencies & sequencing

- **D3 (`useReference.get`)** must land before **Part A's** brand/model picker migration (it's the
  `resolveFn`).
- Everything else is independent and parallelizable (subagent-driven), grouped by part.
- Suggested task grouping for the plan: backend-A, frontend-A (after D3), backend-B, frontend-B,
  backend-C, frontend-C, D1, D2, D3, D4, D5 (folds in B's backend list-filter assertions), D6.

## 7. Verification gate (task-13 sweep)

- Backend: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./... -p 1`
  (all packages — required after shared-signature changes).
- Spectral: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`.
- Frontend: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
- E2E: the affected specs (`master-offices`, users/settings, `approval`) against the real backend; note
  that D2 should reduce the recurring shared-dev-DB Data-Scope corruption.

## 8. PROGRESS.md updates on completion

Tick item 51's candidate (c) work; record approved deviations (if any) per the catat-deviasi convention;
note the deferred reset-password / force-change / badge-poll items and any remaining follow-ups (e.g.
`AsyncSearchPicker` a11y edge cases, `assets/index` reference filters now async).
