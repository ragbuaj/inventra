# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**Inventra** — a **bank fixed-asset management system** (manajemen aset tetap & inventaris for a
bank; reference context: Bank BTN). This is **fixed/physical asset** management (buildings, vehicles,
IT/ATM hardware, furniture) — **not** investment/wealth asset management. Go 1.25 + Gin backend
(`backend/`), Nuxt 4 frontend (`frontend/`), PostgreSQL 16, Redis 7, MinIO. Status: **foundation
scaffold** — backend feature modules are being built in phases per `docs/PRD.md`; the frontend
**foundation** (SPA shell, design system, real auth, reusable component library, Vitest + Playwright)
is built, with feature screens built per phase against the `docs/design` mockups. Go module path:
`github.com/ragbuaj/inventra`.

The product scope was enriched to **bank-grade** in **PRD v1.1**: asset lifecycle with BAST documents,
inter-office transfers (mutasi), physical stock-take (stock opname), disposal with gain/loss,
**dual-basis depreciation** (commercial PSAK 16 + fiscal PMK 72/2023), intangible assets (PSAK 19,
fields prepared), capitalization threshold, and **value-tiered maker-checker** (segregation of duties +
`approval_thresholds` per POJK 17/2023 & 18/POJK.03/2016). These map onto the **existing** office
hierarchy + 3-layer authorization. The core schema (migrations `000001`–`000014`) is implemented; the
v1.1 additions are planned migrations `000015`–`000021` (see DATABASE.md §6) — **not yet written**.

Authoritative design docs: `docs/PRD.md` (requirements, roles, FRs; bank-FAM regulatory citations in
its **Lampiran A**), `docs/DATABASE.md` (schema, conventions, data dictionary), and `docs/ERD.md`
(consolidated entity-relationship diagrams). All written in Indonesian. `docs/PROGRESS.md` tracks phase
status. `docs/DESIGN_BRIEF.md` holds the UI prompt kit / design brief used to generate frontend mockups.
`docs/adr/` holds Architecture Decision Records (MADR) — consult before changing stack choices
(testing, logging, config, rate limiting, authz); supersede rather than edit an Accepted ADR.

## Obsidian vault (knowledge base)

The project knowledge base lives in an Obsidian vault at `D:\Obsidian\inventra` (local Markdown,
also reachable via the `obsidian-vault` MCP server; plain file tools work too). Start from
`Beranda.md` (the MOC). **Consult it when you need**: product decisions & approved mockup deviations
(`Keputusan/Produk/`), the incident runbook (`Ops/Runbook Insiden.md`), regulation parameters —
PMK 72/2023 tariffs, POJK articles (`Referensi/Regulasi/`), the API endpoint/permission map, or the
Bank-FAM glossary. **Keep it maintained**: when a milestone lands, update `Proyek/Status & Roadmap.md`
and `Modul/Peta Modul.md`, record new product decisions as atomic notes in `Keputusan/Produk/`
(indexed in `Keputusan/Keputusan Produk.md`), and write a session note in `Catatan/` (prefix
`YYYY-MM-DD`). Repo docs (`docs/`) stay authoritative — on conflict, fix the vault.

## Commands

```bash
# Backend (from backend/)
go build ./...
go vet ./...
go test ./...
go test ./internal/authz/ -run TestEffectiveLevel   # single package / single test
sqlc generate                                        # regenerate db/sqlc after editing migrations or queries

# Migrations (golang-migrate). DATABASE_URL points at the dev Postgres on :5433.
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
migrate -path db/migrations -database "$DATABASE_URL" down 1

# OpenAPI spec is hand-maintained (backend/api/openapi.yaml) and linted in CI:
npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml

# Frontend (from frontend/, pnpm)
pnpm dev | pnpm build | pnpm lint | pnpm typecheck
pnpm test                 # Vitest unit + Nuxt-runtime tests (frontend/test/)
pnpm test:watch           # Vitest watch mode
pnpm test:e2e             # Playwright e2e (frontend/e2e/) — needs backend stack up + seeded admin

# Infra only (Postgres :5433, Redis :6379, MinIO :9000/:9001) — run app on host
docker compose -f docker-compose.dev.yml up -d
# Full stack in Docker WITH live reload (Go via Air, Nuxt via Vite HMR) — Docker
# Compose `watch` syncs source into the containers; no bind-mount polling.
docker compose -f docker-compose.dev.yml --profile app watch
# Production-like full stack (compiled images; what CI's e2e uses)
docker compose up --build
# Seed a superadmin (run from host while stack is up)
go run ./cmd/createadmin -email admin@inventra.local -password admin12345
```

> **Two compose files** (the `watch.yml` overlay is gone): `docker-compose.yml` =
> production-like compiled images (used by CI e2e); `docker-compose.dev.yml` = dev — infra by
> default, full stack + live reload via `--profile app watch`.

CI (`.github/workflows/ci.yml`) runs the backend build/vet/test, the frontend
lint/typecheck/**test**/build, the Spectral lint, and a separate **e2e** job (docker-compose backend +
seeded admin → Playwright) — keep all of these green.

## Backend architecture

Modular monolith. `cmd/api/main.go` wires config + Postgres pool + Redis and starts the HTTP server;
`internal/server/router.go` (`NewRouter`) is the **single composition root** — it constructs the shared
services and every feature module registers its routes there under `/api/v1`. To add a module: write
its migration + sqlc queries, run `sqlc generate`, write the handler with a `RegisterRoutes`, then wire
it in `NewRouter`.

### Module file layout

A self-contained feature module is split into **four files** by responsibility (see `internal/identity/`
and `internal/user/` as the canonical examples). Follow this split for new modules:

- **`service.go`** — business logic + the package's sentinel errors (`ErrNotFound`, `ErrEmailExists`, …)
  and a `mapDBError` that translates Postgres codes (`23505`/`23503`) into them. Holds `*sqlc.Queries`,
  takes/returns plain domain structs (`CreateInput`, `UpdateInput`) and sqlc row types — **no Gin / no
  HTTP** in this layer.
- **`dto.go`** — request structs with `binding:"..."` validation tags, and the response serialization
  (`toXxxResponse` or a `xxxToMap`). Use the **map form** when the entity needs field-permission
  filtering (so `authz.FilterView` can drop fields); never serialize sensitive columns
  (`password_hash`, `google_id`).
- **`handler.go`** — the `Handler` struct holding the service plus any cross-cutting services
  (`*authz.FieldService`, …). Each method does: bind/validate → call service → serialize → respond, and
  routes service sentinel errors to HTTP status via a `svcError` helper. HTTP status mapping lives here,
  never in the service.
- **`routes.go`** — `RegisterRoutes(rg, handler, authMW, ...)` mounts the route group and attaches
  middleware (`RequireAuth`, `RequirePermission`, scope) per endpoint.

`internal/masterdata/` is a multi-resource aggregate: **each resource is its own sub-package** following
the same four-file split (`office/`, `category/`, `employee/`, `floor/`, `room/`), plus a generic
`reference/` engine for flat tables and a `common/` package of shared plumbing. A thin `masterdata.go`
aggregates them. See the masterdata patterns below (and [docs/adr/0008](docs/adr/0008-backend-masterdata-module-convention.md)).

### Authorization — the core abstraction (read this before touching any endpoint)

Three **orthogonal, configurable, Redis-cached** layers, all keyed by the caller's `role_id` (resolved
from the JWT). All are data-driven from `identity.*` tables — there is no hardcoded role/capability
matrix. Invalidate the relevant Redis cache after mutating these tables.

1. **Action permissions** (`internal/authz/permissions.go`, table `role_permissions`) — boolean
   permission keys like `masterdata.global.manage`, `masterdata.office.manage`, `user.manage`. Gate a
   route with `middleware.RequirePermission(permSvc, "key")` (must run after `RequireAuth`).
2. **Data scope** (`internal/authz/scope.go`, table `data_scope_policies`) — per-row visibility over the
   office hierarchy: `global` / `office_subtree` / `office` / `own`. Resolved per **module string** (e.g.
   `"offices"`), with a per-role default row (`module = '*'`) overridable per module. `office_subtree`
   expands via `GetOfficeSubtree` (also cached). Handlers call `common.ScopedDeps.CallerOfficeScope(c,
   module)` → `(allScope bool, officeIDs []uuid)` and pass those into scope-aware sqlc queries
   (`AllScope`/`OfficeIds` params), typically via their resource service. The module string passed here
   **must match** the `data_scope_policies.module` value. Conservative fallback is always `own`.
3. **Field permissions** (`internal/authz/fields.go`, table `field_permissions`) — per-`(entity, field,
   role)` view/edit flags. `FilterView` strips non-viewable fields from a serialized record;
   **default-allow** (a field with no explicit policy stays visible).

`RequireAuth` (`internal/middleware/auth.go`) validates the Bearer access token, checks Redis for
revocation, and sets `CtxUserID` / `CtxRoleID` on the Gin context. Auth lives in `internal/auth`
(JWT access/refresh, Argon2 passwords, Redis token store); the `internal/identity` module exposes
`/auth/login|refresh|logout|me|permissions|scope/:module`.

### masterdata module — two deliberate patterns

`internal/masterdata` serves reference data via **two** approaches; pick by entity shape:

- **Generic reference engine** (`reference/` package: `engine.go` + `resources.go`) — for *flat* tables
  (text/bool/uuid columns + id/timestamps/soft-delete). Adding a new reference resource is
  **declarative**: append a `resource{}` to the `referenceResources` slice in `reference/resources.go` —
  no SQL, no handler. The engine builds parameterized CRUD against `masterdata.<table>` using `pgx`
  directly (not sqlc). Table/column names come only from these literals, never from request input.
- **Per-resource sub-packages** (`office/`, `category/`, `employee/`, `floor/`, `room/`) — for complex
  entities needing enums, numerics, self-references, or office data-scoping. Each follows the four-file
  split: the **service** holds business rules + scope enforcement as sentinel errors (Gin-free), the
  **handler** resolves scope via `common.ScopedDeps.CallerOfficeScope(c, module)` and passes
  `(allScope, officeIDs)` into the service. `office/` is the reference example (a scoped caller may only
  place an office under a parent within their scope).

Shared plumbing (`common/` package): `common.MapDBError` turns pgx/Postgres errors (`23505`→conflict,
`23503`→invalid reference, no-rows→not found) into sentinel errors; `common.WriteError` maps those to
HTTP status codes; `common.ScopedDeps` resolves the caller's office scope.

## Database conventions (from docs/DATABASE.md)

- **Schema-per-module**: `shared` (enums + the `set_updated_at` trigger fn), `identity`, `masterdata`,
  `audit`, etc. Cross-schema FKs are added in a later migration once both tables exist (e.g. users →
  offices/employees). Money/numeric columns map to **Go `string`** (sqlc override) to avoid float
  precision loss — parse when computing.
- **Soft delete everywhere**: every table carries `created_at` / `updated_at` / `deleted_at`. All
  `UNIQUE` constraints are **partial indexes** `WHERE deleted_at IS NULL` so codes/emails can be reused
  after deletion. Every table gets a `BEFORE UPDATE` trigger calling `shared.set_updated_at()`.
- **Enums live in `shared`** (`shared.scope_level`, `shared.asset_status`, …). `role` is intentionally
  **not** an enum — roles are configurable rows in `identity.roles`.
- sqlc config (`sqlc.yaml`): `pgx/v5`, `emit_pointers_for_null_types`, generates into `db/sqlc/`.
  Migrations are the schema source; queries live per-module in `db/queries/*.sql`.

## Frontend (Nuxt 4)

The foundation is built — **SPA mode** (`ssr: false`), design tokens, real auth, the app shell, and a
global component library (see `docs/superpowers/specs/2026-06-24-frontend-foundation-design.md` and
`docs/superpowers/plans/2026-06-24-frontend-foundation.md`). Feature screens are built per phase on top
of it. When building UI, follow these conventions:

- **Match the prepared mockups — design fidelity is mandatory.** Every screen has a high-fidelity
  reference in `docs/design/<Screen>.dc.html` (the set in `docs/DESIGN_BRIEF.md` §2 — e.g.
  `Katalog Aset.dc.html`, `Dashboard.dc.html`, `Detail Aset.dc.html`). **Before building a screen, open
  its mockup in a browser** (the `.dc.html` files render standalone) and treat it as the source of truth
  for layout, spacing, visual hierarchy, every state (loading/empty/error), and which components appear.
  Reproduce it faithfully with `U*` components — don't improvise a different layout. If a mockup detail
  conflicts with these conventions (e.g. a literal hex color), keep the convention (semantic tokens,
  i18n) but preserve the mockup's structure and intent. The aim is a UI that matches what's in
  `docs/design` 1:1.
  - **Never change, simplify, drop, or substitute the provided design on your own initiative.** Build
    exactly what the mockup shows. If you think a deviation is warranted, ask first and wait — only
    deviate when the user explicitly requests a customization. Deferring part of a design ("later
    phase") also counts as a change: do it only when the user agrees.
  - **Finish every screen with a side-by-side comparison against its mockup.** Open the built screen
    and `docs/design/<Screen>.dc.html` together and verify a 1:1 match — layout, spacing, visual
    hierarchy, every state (loading/empty/error/populated), and every component/field present in the
    mockup. Fix any gap before claiming the screen done; report the comparison result.
- **Always build on Nuxt UI components** (`@nuxt/ui`, the `U*` prefix: `UApp`, `UButton`, `UCard`,
  `UTable`, `UForm`, `UModal`, `UInput`, …). Don't hand-roll buttons/inputs/modals or pull in another
  component library — compose the `U*` primitives. The app shell is `layouts/default.vue`
  (`AppSidebar` + `AppTopbar` + `UMain`); the login screen uses `layouts/auth.vue`.
- **Extract reusable components** into `app/components/` (auto-imported, no manual import). Prefer a
  small wrapper component over repeating the same `U*` markup across pages — e.g. a `ResourceTable`,
  `FormField`, or entity-specific card that encapsulates a Nuxt UI composition. Keep pages thin; push
  shared structure into components.
- **Component-first form inputs** — standard input behaviors live in shared wrapper components, not
  per-page ad-hoc markup:
  - **Number inputs only accept numeric input** (reject/strip non-numeric keystrokes, not just
    validate on submit) and expose a prop that controls whether negative (minus) values are allowed
    — default to non-negative unless the field genuinely needs negatives.
  - **Dropdown / autocomplete inputs must render a "No Data" empty state** when there are no options
    or no search matches (reference behavior: the category input on the maintenance form) — never an
    empty silent popover.
  - **Date inputs use the Nuxt UI calendar** (`UCalendar`-based picker) — no plain text/native date
    inputs.
- **Theme via tokens, not hardcoded colors.** Brand colors are set in `app/app.config.ts`
  (`ui.colors.primary: 'green'`, `neutral: 'slate'`); use the semantic Nuxt UI color props
  (`color="primary"`, `text-muted`) and CSS vars (`--ui-primary`) instead of literal Tailwind colors so
  light/dark mode and rebrands work automatically.
- **i18n is mandatory** — default locale is `id` (Indonesian), with `en`. Put every user-facing string
  in `i18n/locales/{id,en}.json` and reference via `$t('key')` / `useI18n()`; don't hardcode UI text.
  Routing strategy is `prefix_except_default`.
- **API access** goes through `runtimeConfig.public.apiBase` (default `http://localhost:8080/api/v1`,
  override with `NUXT_PUBLIC_API_BASE`) — don't hardcode the backend URL.
- **Lint matters**: ESLint stylistic config enforces no trailing commas (`commaDangle: 'never'`) and
  1tbs brace style; `pnpm lint` and `pnpm typecheck` must pass (CI gates on them).
- **Testing is required.** Unit + Nuxt-runtime tests use **Vitest + @nuxt/test-utils** (`frontend/test/`,
  `pnpm test`); E2E uses **Playwright** (`frontend/e2e/`, `pnpm test:e2e`). Pure logic (formatters, mock
  helpers, composables like `useCan`) gets unit tests (node env); components get runtime mount tests via
  `mountSuspended` (add `// @vitest-environment nuxt` to the file); user-facing flows get an e2e against
  the real backend. **Assert real behavior** (rendered text, resolved i18n, emitted events) — never a
  hollow `expect(html.length).toBeGreaterThan(0)`. New screens land with tests; `pnpm test` gates CI.
  - **Be proactive and expansive with test cases — this initiative is wanted.** Don't stop at the happy
    path: cover edge and boundary conditions, empty/error/loading states, invalid input, permission
    variations, and failure modes, so hard-to-find bugs surface early. Err on the side of *more*
    coverage rather than less; broad, thorough test suites are explicitly preferred here.
  - **At every completion, re-check that the test suite is complete** for what you built — every state,
    branch, edge case, and failure mode that can be tested has a test. Treat missing coverage as
    unfinished work, not a follow-up.

## Development workflow

Work is **phased** per `docs/PRD.md` §10 (Fondasi → Identity & Otorisasi → Master data → Asset core →
Approval → Assignment → Maintenance → Depreciation/Reporting → Polish). Each feature phase gets its own
spec + implementation plan before code. The repo is currently in the **Master data** phase; commits land
roughly one resource/sub-feature at a time (see git history: offices, then floors+rooms, etc.).

- **Branch per feature**, named `feat/<short-topic>` (e.g. `feat/md-floors-rooms`); `main` is the
  integration branch. Don't commit feature work directly to `main`.
- **Conventional Commits with a scope** matching the module/area, lowercase, imperative:
  `feat(masterdata): ...`, `fix(security): ...`, `feat(authz): ...`, `feat(db): ...`,
  `refactor(db): ...`. Security/authorization fixes use `fix(security):` and are treated as first-class
  (e.g. `enforce data scope on office/employee ...`).
- **Adding a backend feature module — standard order:**
  1. Migration (`db/migrations/NNNNNN_*.up.sql` + matching `.down.sql`) — new schema/tables follow the
     soft-delete + partial-unique + `set_updated_at` conventions.
  2. Queries (`db/queries/<module>.sql`), then `sqlc generate`.
  3. Handler(s) + `RegisterRoutes` in `internal/<module>/`, reusing the masterdata patterns
     (generic engine vs. sqlc+scope) where applicable.
  4. **Enforce authorization explicitly** on every endpoint: `RequirePermission` for the action, and
     thread `callerOfficeScope` through scoped queries for per-row data scope. Scope must be enforced on
     **read *and* write** (get/list/create/update/delete) — missing scope on any verb is a security bug.
  5. Wire the module into `NewRouter` (or the module's own `RegisterRoutes`).
  6. Update `backend/api/openapi.yaml` to match.
- **Building a frontend screen — standard order:**
  1. **Open the matching `docs/design/<Screen>.dc.html` mockup** in a browser; identify the layout,
     components, and every state. It is the visual source of truth — build to match it.
  2. Reuse/extend global components (`app/components/`) before writing page markup; keep pages thin.
  3. Back data with a module service (mock fixtures today, real `$fetch` later behind the **same**
     interface — `composables/api/`); never call the backend URL directly.
  4. Put every string in `i18n/locales/{id,en}.json`; gate role-specific UI with `useCan`/`<Can>`.
  5. Write tests proactively and broadly: unit for logic, a `mountSuspended` runtime test for the
     component, an e2e for the flow — and beyond the happy path cover edge/boundary cases,
     empty/error/loading states, invalid input, and permission variations. Then re-check the suite is
     complete for every state and branch before moving on.
  6. Match the mockup in light **and** dark mode, then do a final **1:1 side-by-side comparison** of the
     built screen against `docs/design/<Screen>.dc.html`. Fix any deviation — never redesign or defer
     part of the mockup on your own initiative (ask first) — before committing.
- **Verify before committing** — run and confirm green: `go build ./...`, `go vet ./...`,
  `go test ./...`, and the Spectral lint. For frontend changes: `pnpm lint`, `pnpm typecheck`,
  `pnpm test`, `pnpm build`. These are exactly what CI enforces; don't claim done without running them.
  (E2E `pnpm test:e2e` needs the backend stack up + a seeded admin; CI runs it in the separate `e2e` job.)
- **Always update `docs/PROGRESS.md` when a task lands** — part of "done", not optional. After completing
  and verifying a feature/task (in the same commit/PR that finishes it), tick the matching checkbox
  (`[ ]` → `[x]`) with a one-line note and PR number where relevant, and refresh the **"▶ Next session —
  start here"** block to point at the next real step. PROGRESS.md is the source of truth for *what's built
  vs. left*; never let it drift from the actual code. Before claiming a screen/module already exists,
  verify against the repo (files/routes/migrations), not the checklist — the checklist can lag.

## Conventions

- Don't hand-edit `backend/db/sqlc/` — it is generated; change `db/queries/*.sql` or migrations and
  rerun `sqlc generate`.
- Keep `backend/api/openapi.yaml` in sync with route changes (it is hand-written and Spectral-linted).
- Backend logging/HTTP is Gin; responses are JSON `gin.H`. List endpoints return
  `{data, total, limit, offset}` with `limit` clamped 1–100 (`clampInt`).
