# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**Inventra** — asset/inventory management system. Go 1.25 + Gin backend (`backend/`), Nuxt 4
frontend (`frontend/`), PostgreSQL 16, Redis 7, MinIO. Status: **foundation scaffold** — backend
feature modules are being built in phases per `docs/PRD.md`; the frontend is still the stock Nuxt UI
template (real UI not yet built). Go module path: `github.com/ragbuaj/inventra`.

Authoritative design docs: `docs/PRD.md` (requirements, roles, FRs) and `docs/DATABASE.md` (schema,
conventions, data dictionary). Both are written in Indonesian. `docs/PROGRESS.md` tracks phase status.
`docs/DESIGN_BRIEF.md` holds the UI prompt kit / design brief used to generate frontend mockups.

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

# Infra only (Postgres :5433, Redis :6379, MinIO :9000/:9001)
docker compose -f docker-compose.dev.yml up -d
# Full stack incl. migrate + api + frontend
docker compose up --build
# Seed a superadmin (run from host while stack is up)
go run ./cmd/createadmin -email admin@inventra.local -password admin12345
```

CI (`.github/workflows/ci.yml`) runs the backend build/vet/test, the frontend lint/typecheck/build,
and the Spectral lint — keep all of these green.

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

`internal/masterdata/` is the deliberate exception: it's a multi-resource aggregate (many small reference
entities in one package, one file per entity like `offices.go` / `employees.go`) rather than the strict
four-file split — see the two masterdata patterns below.

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
   expands via `GetOfficeSubtree` (also cached). Handlers call `scopedDeps.callerOfficeScope(c, module)`
   → `(allScope bool, officeIDs []uuid)` and pass those into scope-aware sqlc queries (`AllScope`/
   `OfficeIds` params). The module string passed here **must match** the `data_scope_policies.module`
   value. Conservative fallback is always `own`.
3. **Field permissions** (`internal/authz/fields.go`, table `field_permissions`) — per-`(entity, field,
   role)` view/edit flags. `FilterView` strips non-viewable fields from a serialized record;
   **default-allow** (a field with no explicit policy stays visible).

`RequireAuth` (`internal/middleware/auth.go`) validates the Bearer access token, checks Redis for
revocation, and sets `CtxUserID` / `CtxRoleID` on the Gin context. Auth lives in `internal/auth`
(JWT access/refresh, Argon2 passwords, Redis token store); the `internal/identity` module exposes
`/auth/login|refresh|logout|me|permissions|scope/:module`.

### masterdata module — two deliberate patterns

`internal/masterdata` serves reference data via **two** approaches; pick by entity shape:

- **Generic reference engine** (`ref.go` + `resources.go`) — for *flat* tables (text/bool/uuid columns +
  id/timestamps/soft-delete). Adding a new reference resource is **declarative**: append a `resource{}`
  to the `referenceResources` slice in `resources.go` — no SQL, no handler. The engine builds
  parameterized CRUD against `masterdata.<table>` using `pgx` directly (not sqlc). Table/column names
  come only from these literals, never from request input.
- **sqlc-backed handlers** — for complex entities (`categories`, `offices`, `floors`, `rooms`,
  `employees`) needing enums, numerics, self-references, or office data-scoping. `offices.go` is the
  reference example: it threads `callerOfficeScope` through scope-aware queries and enforces scope on
  create/update/delete (e.g. a scoped caller may only place an office under a parent within their scope).

Shared error mapping (`common.go`): `mapDBError` turns pgx/Postgres errors (`23505`→conflict,
`23503`→invalid reference, no-rows→not found) into sentinel errors; `writeError` maps those to HTTP
status codes.

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

The frontend is still the stock Nuxt UI starter — real screens are not built yet. When building UI,
follow these conventions:

- **Always build on Nuxt UI components** (`@nuxt/ui`, the `U*` prefix: `UApp`, `UButton`, `UCard`,
  `UTable`, `UForm`, `UModal`, `UInput`, …). Don't hand-roll buttons/inputs/modals or pull in another
  component library — compose the `U*` primitives. The app shell uses `UApp` > `UHeader`/`UMain`/
  `UFooter`.
- **Extract reusable components** into `app/components/` (auto-imported, no manual import). Prefer a
  small wrapper component over repeating the same `U*` markup across pages — e.g. a `ResourceTable`,
  `FormField`, or entity-specific card that encapsulates a Nuxt UI composition. Keep pages thin; push
  shared structure into components.
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
- **Verify before committing** — run and confirm green: `go build ./...`, `go vet ./...`,
  `go test ./...`, and the Spectral lint. For frontend changes: `pnpm lint`, `pnpm typecheck`,
  `pnpm build`. These are exactly what CI enforces; don't claim done without running them.

## Conventions

- Don't hand-edit `backend/db/sqlc/` — it is generated; change `db/queries/*.sql` or migrations and
  rerun `sqlc generate`.
- Keep `backend/api/openapi.yaml` in sync with route changes (it is hand-written and Spectral-linted).
- Backend logging/HTTP is Gin; responses are JSON `gin.H`. List endpoints return
  `{data, total, limit, offset}` with `limit` clamped 1–100 (`clampInt`).
