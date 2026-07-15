# Authz Nav/Guard Consistency — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make menu visibility = page reachability = endpoint permission across all roles. Replace the binary
`superadminNav/staffNav` selection with a single per-permission nav model, align every page-guard with its
entry permission and its on-load endpoints, and (Opsi 1) loosen authz-admin **reads** so `scope.manage` /
`fieldperm.manage` can be delegated independently. No schema/seed changes.

**Architecture:** Mostly frontend (Nuxt 4 SPA): nav model, `can` middleware, page `definePageMeta`, in-page
fetch gating. One backend slice: a new `RequireAnyPermission` middleware + relaxed read gates on
`/authz/catalog`, `/authz/roles`, `/authz/roles/:id`. Companion spec:
`docs/superpowers/specs/2026-07-16-authz-nav-guard-consistency-design.md`.

**Tech Stack:** Go 1.25 + Gin + `authz.PermissionService`; Nuxt 4 + `@nuxt/ui` + Vitest/@nuxt/test-utils +
Playwright; i18n `id`/`en`.

## Global Constraints
- Frontend: compose `U*` only; keep pages thin; semantic tokens; i18n mandatory (`t('key')`, default `id`).
- Lint: ESLint stylistic — **no trailing commas**, 1tbs. `pnpm lint` + `pnpm typecheck` must pass.
- API via `useApiClient`/`runtimeConfig.public.apiBase`; never hardcode backend URL.
- Backend: keep **mutation** endpoints strict; only **read** gates are relaxed. OpenAPI hand-maintained + Spectral.
- CI gates: backend `go build ./... && go vet ./... && go test ./...` + Spectral; frontend `pnpm lint && pnpm typecheck && pnpm test && pnpm build`.
- Commits: Conventional Commits with scope; **no AI co-author trailers**.
- Update `docs/PROGRESS.md` when the batch lands.
- **Scope guard:** do NOT change seed/role→permission, schema, migrations, data-scope enforcement, or menu
  layout. Only visibility/gating alignment.

## File Structure
**Modified — backend**
- `backend/internal/middleware/permission.go` — add `RequireAnyPermission`.
- `backend/internal/middleware/permission_test.go` — add `TestRequireAnyPermission`.
- `backend/internal/authzadmin/routes.go` — relax read gates (catalog/roles/role-by-id).
- `backend/internal/authzadmin/integration_test.go` — delegation tests (scope-only role).
- `backend/api/openapi.yaml` — security notes for relaxed reads.

**Modified — frontend**
- `frontend/app/utils/nav.ts` — single `appNav`; per-item `permission: string | string[]`.
- `frontend/app/types/index.ts` — `NavItem.permission?: string | string[]`.
- `frontend/app/middleware/can.ts` — accept `string | string[]` (OR).
- `frontend/app/components/AppSidebar.vue` — single model, `hasAny`, parent auto-hide.
- `frontend/app/components/AppTopbar.vue` — breadcrumb from `appNav`.
- Page `definePageMeta` guards: `pages/assignment.vue`, `pages/maintenance.vue`, `pages/settings/rbac.vue`,
  `pages/settings/data-scope.vue`, `pages/settings/field-permission.vue`.
- In-page fetch gating: `pages/index.vue` (dashboard summary), `pages/assignment.vue` (available fetch).
- `frontend/i18n/locales/{id,en}.json` — any new/moved nav labels.

**New / extended tests — frontend**
- `frontend/test/unit/nav-model.spec.ts` — extend: per-role visible-set = permission-set.
- `frontend/test/unit/use-can.spec.ts` (or existing) — OR semantics.
- `frontend/test/nuxt/app-sidebar.spec.ts` — per-role sidebar render + parent auto-hide.
- `frontend/e2e/nav-access.spec.ts` — per-role: every visible menu opens without 403.

---

# PART A — Backend: relax authz-admin reads (Opsi 1)

## Task A1: `RequireAnyPermission` middleware
**Files:** `backend/internal/middleware/permission.go`, `permission_test.go`
**Steps:**
- [ ] Add `func RequireAnyPermission(checker authz.PermissionChecker, keys ...string) gin.HandlerFunc`.
      Mirror `RequirePermission`: resolve `CtxRoleID` (401 if missing); loop keys, call the checker's
      `Has` for each; **allow on first true**; 403 if none; 500 on checker error. Verify the exact
      `PermissionChecker` method name/signature in `internal/authz/permissions.go` before writing.
- [ ] `TestRequireAnyPermission`: allow when one of N granted; 403 when none; 401 when no role; 500 on error.
**Acceptance:** `go test ./internal/middleware/` green.

## Task A2: Relax read gates in authzadmin routes
**Files:** `backend/internal/authzadmin/routes.go`
**Steps:**
- [ ] `GET /authz/catalog` → `RequireAnyPermission(permSvc, "role.manage","scope.manage","fieldperm.manage")`.
- [ ] `GET /authz/roles` + `GET /authz/roles/:id` → `RequireAnyPermission(permSvc,
      "role.manage","scope.manage","fieldperm.manage","user.manage")` (user.manage for the users-screen role picker).
- [ ] Leave `POST/PUT/DELETE /roles`, `PUT /roles/:id/permissions` on `role.manage`; `.../scope` on
      `scope.manage`; `.../fields` on `fieldperm.manage` — **unchanged**.
- [ ] `RegisterRoutes` signature may need the extra permSvc-derived middlewares threaded from `router.go`;
      keep the existing `requireRole/requireScope/requireField` for mutations.
**Acceptance:** routes compile; mutations still strict.

## Task A3: Delegation integration tests + OpenAPI
**Files:** `backend/internal/authzadmin/integration_test.go`, `backend/api/openapi.yaml`
**Steps:**
- [ ] Role with **only** `scope.manage`: `GET /catalog` 200, `GET /roles` 200, `GET /roles/:id/scope` 200,
      `PUT /roles/:id/scope` 200; but `PUT /roles/:id/permissions` → 403.
- [ ] Role with only `user.manage`: `GET /roles` 200 (picker); `GET /catalog` → 403.
- [ ] Update OpenAPI security descriptions for the three relaxed read endpoints.
- [ ] `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` clean.
**Acceptance:** `go test ./internal/authzadmin/` + Spectral green.

---

# PART B — Frontend: single nav model + OR-aware gating

## Task B1: Types + `can` middleware OR support
**Files:** `frontend/app/types/index.ts`, `frontend/app/middleware/can.ts`, `frontend/test/unit/use-can.spec.ts`
**Steps:**
- [ ] `NavItem.permission?: string | string[]`.
- [ ] `can.ts`: read `to.meta.permission` as `string | string[]`; allow if `!permission` or (array → some /
      string → single) `can(...)` true; else `abortNavigation(403)`.
- [ ] Unit test the OR branch (string allow/deny; array allow-if-any; missing → allowed).
**Acceptance:** `pnpm test` for the middleware/unit specs green.

## Task B2: Single `appNav` model
**Files:** `frontend/app/utils/nav.ts`
**Steps:**
- [ ] Replace `superadminNav` + `staffNav` with one exported `appNav: NavGroup[]` using the §A permission
      map from the spec (Operasional + Administrasi groups). Set `permission` per item; Maintenance =
      `['maintenance.view','request.create']`; Master ▸ Impor = the 3-key array; Dashboard = none.
- [ ] Remove disabled placeholders (`My Assets`, staff `Approval`).
- [ ] Keep `labelKey`/`icon`/`to`/`children` shape unchanged so templates need no structural change.
**Acceptance:** typecheck passes; no remaining imports of `superadminNav`/`staffNav` except updated ones.

## Task B3: AppSidebar + AppTopbar rewire
**Files:** `frontend/app/components/AppSidebar.vue`, `frontend/app/components/AppTopbar.vue`
**Steps:**
- [ ] Sidebar: `const nav = appNav` (drop the `user.manage ? ... : ...` ternary). Add `hasAny(perm)` helper.
- [ ] `isVisible(item)`: leaf → `!perm || hasAny(perm)`; parent (has `children`) → some child visible.
- [ ] Ensure the render loop hides a parent group and a top-level group whose visible children are empty.
- [ ] Topbar: build breadcrumb from `appNav` (replace `superadminNav` import).
**Acceptance:** `pnpm typecheck` + existing sidebar tests updated green.

---

# PART C — Page-guard + in-page fetch alignment

## Task C1: Align `definePageMeta` guards
**Files:** `pages/assignment.vue`, `pages/maintenance.vue`, `pages/settings/rbac.vue`,
`pages/settings/data-scope.vue`, `pages/settings/field-permission.vue`
**Steps:**
- [ ] `/assignment` → `permission: 'assignment.view'`.
- [ ] `/maintenance` → `permission: ['maintenance.view','request.create']`.
- [ ] `/settings/rbac` → `'role.manage'`; `/settings/data-scope` → `'scope.manage'`;
      `/settings/field-permission` → `'fieldperm.manage'`.
**Acceptance:** each page opens for the intended role, 403 for roles lacking the permission (covered by e2e).

## Task C2: In-page defensive fetch gating
**Files:** `frontend/app/pages/index.vue`, `frontend/app/pages/assignment.vue`
**Steps:**
- [ ] Dashboard: call `GET /dashboard/summary` only when `can('report.view')`; otherwise render an
      empty/placeholder summary (no 403). Keep the inbox fetch behind `can('request.decide')` (already).
- [ ] Assignment: gate the `GET /assignments/available` (checkout picker) fetch behind `can('request.create')`;
      keep checkout/checkin actions behind `can('assignment.manage')`.
**Acceptance:** a role without `report.view`/`request.create` loads the page without a failed request.

---

# PART D — Tests (per-role) + i18n

## Task D1: Unit — nav visible-set = permission-set
**Files:** `frontend/test/unit/nav-model.spec.ts`
**Steps:**
- [ ] For each seed role (superadmin, kepala_kanwil, kepala_unit, manager, staf) with its permission set
      (from spec matrix), compute the flattened visible leaf routes and assert it matches the expected set.
      Explicitly assert kanwil sees Mutasi/Penghapusan/Stock Opname/Approval/Laporan/Audit and NOT
      RBAC/Data-scope/Field/Depreciation.
**Acceptance:** `pnpm test nav-model` green.

## Task D2: Runtime — AppSidebar per role + auto-hide
**Files:** `frontend/test/nuxt/app-sidebar.spec.ts` (new, `// @vitest-environment nuxt`)
**Steps:**
- [ ] Mount `AppSidebar` with each role's `permissions` (stub `useAuthStore`); assert rendered menu labels
      match; assert a group with no visible children is not rendered (e.g., staf → no Settings group).
**Acceptance:** `pnpm test app-sidebar` green.

## Task D3: E2E — per-role reachability (the class-of-bug closer)
**Files:** `frontend/e2e/nav-access.spec.ts` (new)
**Steps:**
- [ ] Using the demo seed logins, for each role: log in, enumerate visible sidebar links, click each,
      assert the page renders its primary content and **no 403** error boundary appears. Use unique-run
      hygiene per e2e memory; assert-after-navigation.
- [ ] One case: create a `scope.manage`-only custom role → Data Scope screen loads catalog+roles.
**Acceptance:** `pnpm test:e2e nav-access` green against the seeded stack.

## Task D4: i18n
**Files:** `frontend/i18n/locales/{id,en}.json`
**Steps:**
- [ ] Add/verify labels for any nav entry now surfaced that lacked a key (e.g., Master ▸ Impor, Penugasan).
**Acceptance:** no missing-key fallback for visible nav items; `pnpm lint`/`typecheck` green.

---

# PART E — Land it

## Task E1: Full verification + PROGRESS + commit
**Steps:**
- [ ] Backend: `go build ./... && go vet ./... && go test ./...` + Spectral.
- [ ] Frontend: `pnpm lint && pnpm typecheck && pnpm test && pnpm build`; e2e in the seeded stack.
- [ ] Update `docs/PROGRESS.md` (note the fix + PR).
- [ ] Commit on `feat/authz-nav-guards` with Conventional Commits (`fix(security): align nav/guard/endpoint
      authz across roles`), no AI co-author.
**Acceptance:** all CI-equivalent gates green; PROGRESS updated.

---

## Dependency order
A (backend, independent) ∥ B (nav model) → C (guards, needs B types) → D (tests, needs A+B+C) → E.
Parts A and B can proceed in parallel; C depends on B1 (type/middleware); D depends on all; E last.
