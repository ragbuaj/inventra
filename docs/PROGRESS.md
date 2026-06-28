# Inventra — Progress & Remaining Work

Living checklist of what's built vs. what's left. See [PRD.md](PRD.md) for scope,
[DATABASE.md](DATABASE.md) for the schema, and [ERD.md](ERD.md) for entity relationships.

> **Scope update — PRD v1.1 (Bank Fixed Asset Management).** The product was reframed to a **bank**
> fixed-asset system (context: Bank BTN) and enriched with: inter-office **mutasi**, **stock opname**,
> **BAST** documents, **dual-basis depreciation** (commercial PSAK 16 + fiscal PMK 72/2023), **disposal**
> with gain/loss, **intangible** assets (PSAK 19, fields prepared), **capitalization threshold**, and
> **value-tiered maker-checker** (`approval_thresholds`, SoD per POJK 17/2023 & 18/POJK.03/2016).
> Design docs (PRD/DATABASE/ERD) are updated, and the **bank-FAM schema is now built** — v1.1
> enums/columns are **baked into the initial migrations** (greenfield) + new tables in
> `000015_fam_tables` (see *Database* below and DATABASE.md §6). Architecture decisions from the pivot
> are recorded as ADRs in [adr/](adr/) (**ADR-0001–0009**: testing, logging, config, rate-limit, authz
> build-vs-buy, map, frontend API convention, masterdata split, third-party sign-in). What's already ✅
> predates the pivot and remains valid — the office hierarchy + 3-layer authorization are the foundation
> the bank scope builds on.

> ## ▶ Next session — start here
> 1. ~~**Bring the dev stack up, reset & migrate**~~ ✅ **DONE (2026-06-27).**
> 2. ~~**#6 Kategori Aset screen**~~ ✅ **DONE.**
> 3. ~~**Approval engine + Asset core backend**~~ ✅ **DONE (2026-06-28).**
> 4. ~~**Asset attachments (MinIO)**~~ ✅ **DONE (2026-06-28).**
> 5. ~~**Barcode/QR + label PDF**~~ ✅ **DONE (2026-06-28).**
> 6. ~~**Asset documents (BAST)**~~ ✅ **DONE (2026-06-28).**
> 7. ~~**Authorization admin endpoints**~~ ✅ **DONE (2026-06-28).** `internal/authzadmin` — role CRUD, replace-set permissions/scope/fields, Redis cache invalidation, permission catalog, seed RBAC drift fix, integration tests, OpenAPI spec.
> 8. ~~**Wire Peran & RBAC screen to real `/authz` APIs**~~ ✅ **DONE (2026-06-28).** `useRbac` composable rewritten to `/authz/catalog` + `/authz/roles` + `/authz/roles/:id/permissions`; English DTO; UUID `id` identity; system-role permissions now editable (product decision — lock note reworded, switches active); e2e spec updated against real seeded backend.
> 9. ~~**Wire frontend Data Scope screen** (`/settings/data-scope`) to real `/authz` APIs~~ ✅ **DONE (2026-06-28).** `useDataScope` composable rewritten to `/authz/catalog` (scope_modules, filters `*`) + `/authz/roles` + `/authz/roles/:id/scope`; English DTO; UUID `id` identity; save only changed roles (dirtyIds set); e2e spec added against real seeded backend; orphaned `mock/dataScope.ts` deleted.
> 10. ~~**Wire frontend Field Permission screen** (`/settings/field-permission`) to real `/authz` APIs~~ ✅ **DONE (2026-06-28).** `useFieldPermission` composable rewritten to `/authz/roles` + `/authz/roles/:id/fields`; catalog `assets`+`users` (English field keys); UUID `id` identity; default-allow (no stored policy = view+edit); save preserves other-entity rows + only PUTs changed roles; e2e spec added against real seeded backend; orphaned `mock/fieldPermission.ts` deleted. **Authz-screen wiring trio (RBAC + Data Scope + Field Permission) now complete.**
>    - **TODO — extend field-permission ENFORCEMENT (`FilterView`) beyond `assets`+`users`:** `requests` (approval handler already injects `fieldSvc` + has `requestToMap`; add `ForEntity`/`FilterView` calls), `employees` (needs `fieldSvc`+map wiring), and other masterdata modules. Until then the Field Permission screen configures rules but they only take effect for `assets`+`users`. Add each new entity to `frontend/app/constants/fieldCatalog.ts` once its backend enforcement lands.
> 11. ~~**Wire Audit Trail screen** (`/settings/audit`) to real `GET /api/v1/audit`~~ ✅ **DONE (2026-06-29).** `useAudit` composable rewritten to server-side list (`GET /api/v1/audit`, limit/offset/filter params); gate `audit.view`; entity-type filter from `AUDIT_ENTITY_TYPES` frontend catalog; expandable diff viewer unchanged; orphaned `mock/audit.ts` deleted; e2e spec updated against real seeded backend.
>    - **TODO — actor filter + role/summary/office-name columns are dropped:** the backend audit response has no `role` or `summary` fields, and resolving actor/office **names** needs `user.manage`/masterdata reads that an `audit.view`-only viewer may lack. Revisit if a viewer-accessible actor/office name lookup (or an enriched `GET /api/v1/audit` response) lands.
> 12. **Next: Wire User Management screen** (`/settings/users`) to real `/api/v1/users` — the last remaining Settings screen to wire in this batch.

## ✅ Done

### Foundation & DevOps
- [x] Project scaffold (Go/Gin backend, Nuxt 4 frontend)
- [x] `docker compose up` full stack (Postgres + Redis + MinIO + migrate + backend + frontend)
- [x] GitHub Actions CI (backend build/vet/test · frontend lint/typecheck/build · Spectral)
- [x] PRD + DATABASE design docs

### Database (15 migrations · 12 schemas)
- [x] enums + `set_updated_at` + per-module schemas (`shared/identity/audit/masterdata/asset/import/approval/assignment/maintenance/depreciation` + v1.1 `transfer/stockopname/disposal`)
- [x] All tables incl. soft delete, partial-unique, FK indexes, seed (5 roles, 45 RBAC perms)
- [x] **Bank-FAM v1.1 schema baked in** (greenfield) — enums + columns folded into initial migrations
      (`000002`/`003`/`006`/`007`/`008`/`010`/`013`) + new tables `000015_fam_tables` (asset_transfers,
      disposals, stock_opname_*, asset_documents) + app_settings/approval_thresholds/request_approvals.
      `sqlc generate` + build/vet/test + Spectral green; ⚠️ full `migrate up` re-validate on next stack-up

### Backend — Data layer
- [x] pgx pool + Redis client + sqlc models (all tables)
- [x] `/health` (liveness) + `/health/ready` (Postgres + Redis)

### Backend — Identity & Authorization
- [x] Local auth: login, JWT access+refresh (Redis store + denylist), logout, `/auth/me`
- [x] Authorization 3-layer (configurable): RBAC (`role_permissions`), data scope (`data_scope_policies` + office subtree), field permission (`field_permissions`)
- [x] `/auth/permissions`, `/auth/scope/{module}`
- [x] User management (Superadmin): CRUD + field-permission filtering

### Backend — Master Data (all data-scoped & access-controlled)
- [x] Categories (enum/nullable/self-ref/numeric)
- [x] 11 reference resources via generic engine (office-types, departments, positions, units, maintenance/problem categories, brands, vendors, provinces, cities, models)
- [x] Offices (hierarchy) + floors + rooms + employees — **office-subtree scoping** on all ops, IDOR-hardened
- [x] **Masterdata convention refactor** (ADR-0008) — each resource is its own sub-package with the
      four-file split (`office/` · `category/` · `employee/` · `floor/` · `room/`), shared plumbing in
      `common/`, generic engine in `reference/`; thin `masterdata.go` aggregator. Build/vet/test green, no behavior change

### API Documentation
- [x] OpenAPI 3.1 spec + self-hosted Scalar at `/docs` + Spectral lint in CI
- [x] Bruno collection (git-tracked)

### Frontend — foundation & screens (mock-first, built 1:1 with `docs/design`)
- [x] Foundation: SPA shell (`AppSidebar`/`AppTopbar`/`layouts`), design tokens, real backend auth (login + route middleware `can` + `useCan`/`<Can>`), `U*` component library, i18n (id/en), Vitest + Playwright harness
- [x] Dashboard
- [x] **Assets cluster** — Catalog, Detail, Form (new/edit), Import wizard, Label/Barcode
- [x] **Settings cluster** — User Management, Peran & RBAC, Data Scope, Field Permission, Audit Trail
- [x] **Master Data** — Offices, Employees, Reference
- [x] **Operasional cluster** — Penugasan (assignment), Maintenance, Pengajuan & Approval, Laporan (reports)
- [x] **Global Search** — ⌘K command palette (mock multi-entity aggregator, keyboard nav, recent + quick actions)
- [x] **Peta Lokasi** — office-location map (real Leaflet + OSM, colored pins, list/filter/detail) under Master Data
- [x] **Profil Akun** — `/akun` profile & settings (Profil / Keamanan / Preferensi tabs)
- [x] Mock-first data seam (`mock/*` + `composables/api/use*`) ready to swap to real `$fetch` behind the same interface
- [x] Tests: 387 Vitest unit + `mountSuspended` runtime specs green; lint/typecheck/build gate CI

> **All 23 `docs/design/*.dc.html` mockups are now implemented.** Frontend screens currently
> render mock fixtures; they need wiring to real backend modules as those land (below).
> (Peta Lokasi uses a real Leaflet map per an explicit product decision, in place of the
> mockup's illustrative SVG; everything else matches its mockup 1:1.)

---

## ⛔ Remaining

### Bank-FAM (PRD v1.1) — schema done, modules to build

> New scope from the bank pivot. **Schema is built** (see *Database* above); what remains is the
> **backend modules/handlers** + frontend for these features. Enforce data-scope + field-permission on
> every new endpoint (read **and** write); follow the masterdata 4-file split (ADR-0008).

- [x] **Bank-FAM schema** — DONE (greenfield bake-in). New enums + columns folded into the initial
      migrations (`000002`/`000003`/`000006`/`000007`/`000008`/`000010`/`000013`); genuinely-new tables in
      `000015_fam_tables` (`transfer.asset_transfers`, `disposal.disposals`, `stockopname.stock_opname_*`,
      `asset.asset_documents`) + `app_settings`/`approval_thresholds`/`request_approvals`. `sqlc generate`
      + `go build/vet/test` green; `migrate up` validated live (reset via drop-schemas, not `down -all`).
      **Backend handlers** for the new tables (transfer/opname/disposal/documents) still to build.
- [x] **Category enrichment — backend** — `categories` columns (GL account, fiscal group, commercial+
      fiscal useful life, capitalization threshold, asset_class) baked in; `category` service/dto + sqlc +
      OpenAPI wired (build green). **Frontend Kategori screen** still to build (#6 — see *Next session*).
- [ ] **Dual-basis depreciation** — commercial (PSAK 16) + fiscal (PMK 72/2023, kelompok 1–4 / bangunan)
      `depreciation_entries` per basis; intangible amortization (PSAK 19); impairment (PSAK 48) write-down
- [x] **Value-tiered approval** — `approval_thresholds` (configurable bands per request_type/min-max
      amount/approval_level) + `request_approvals` chain; SoD (maker ≠ checker per step); seeded
      placeholder bands; authz-admin CRUD endpoints for thresholds included. **Done — (2026-06-28).**
- [ ] **Asset transfer (mutasi)** — inter-office transfer + BAST + history; updates `assets.office_id`
- [ ] **Stock opname** — sessions + item reconciliation (found/not_found/damaged/misplaced) + report
- [ ] **Disposal** — status transition (`assets.status → disposed`) implemented via approval executor
      (asset_disposal flow). Gain/loss accounting + journal entries still pending (requires depreciation
      to derive server-side `book_value`; currently disposal `amount` is maker-supplied — ⚠️ value-tier
      hardening needed once depreciation lands).
- [x] **Asset documents (BAST)** — metadata CRUD + optional MinIO file; scope-gated + audited; integration tests (10 cases). **Done — (2026-06-28).**
- [ ] **Journal-ready export** — GL-account rollup (depreciation expense, disposal gain/loss)
- [ ] **Capitalization threshold** — `app_settings` global default + per-category override; below
      threshold → expensed, not capitalized
- [ ] **Confirm with bank policy** — office-tier naming, capitalization amount, approval-limit bands,
      cost-model vs revaluation, exact PSAK paragraphs (PRD ⚠️ items / DATABASE DB-Q6–Q8)

### Backend — Feature modules
- [x] **Asset core** — CRUD read/update (direct, data-scoped + field-permission masking of
      `purchase_cost`/`book_value`/`accumulated_depreciation`); `asset_tag` generator (atomic
      per-office/category/year, Postgres advisory lock); status state machine (valid transitions
      enforced); valuation-exclusion flag. Asset create/disposal/exclusion go through the approval
      engine (not direct write). **Done — (2026-06-28).**
- [x] **Asset attachments (MinIO)** — Storage interface; upload + size/type validation; image thumbnail (original preserved); proxy download/thumbnail; integration-test coverage (MinIO round-trip + scope + rollback). **Done — (2026-06-28).**
- [x] **Barcode / QR** — Code128 + QR PNG from `asset_tag`; scan-lookup `GET /assets/by-tag/:tag`; barcode PNG `GET /assets/:id/barcode`; label PDF `POST /assets/labels` — **BTN template** (QR+logo + bank header + asset code + office/category/name/TP + disclaimer; `company_name`/`disclaimer` from `app_settings`; logo via `LABEL_LOGO_PATH`) + **generic** template; layout **roll** (page-per-label, default 60×24 mm on 64 mm media for Epson C4050) + **sheet** (A4 grid); scope-gated; integration tests. **Done — (2026-06-28).**
- [x] **Approval (maker-checker)** — generic `request_approvals` table; threshold-driven chain
      construction; SoD enforcement (maker cannot approve own request); pull-model eligibility
      (pending step scoped to checker's office); executors: `asset_create`, `asset_disposal`,
      `valuation_exclusion`; authz-admin CRUD endpoints for `approval_thresholds` (Superadmin-gated).
      **Done — (2026-06-28).**
- [ ] **Assignment** — check-out/check-in; assignment requests (Staf → approve); one-active-per-asset; overdue; history
- [ ] **Maintenance** — schedules (interval/next_due); records (preventive/corrective, cost, vendor); damage reports (Staf + problem category); `under_maintenance` status
- [ ] **Depreciation** — book value (straight-line / declining-balance); monthly `depreciation_entries` read model
- [ ] **Reporting & Dashboard** — aggregates (totals/value/by status·category·office, overdue, maintenance due, costs); **PDF + Excel export**; scoped — reading from the pre-aggregated OLAP tables (see *Analytics / OLAP* below)
- [ ] **Bulk import** — CSV/XLSX (assets + master data); `import_jobs`; per-row validation + error report

### Analytics / OLAP (large-data plan)

> Dashboard & Reporting currently aggregate **directly over the OLTP tables**. As assets,
> assignments, maintenance records, depreciation entries, and audit logs grow, those scans get
> slow and contend with transactional writes. Plan: add a dedicated **analytical read layer**
> kept separate from the write path (OLTP stays the source of truth; OLAP is a derived read model).

- [ ] **`analytics` schema (star schema)** — dimension tables (`dim_office`, `dim_category`, `dim_status`, `dim_date`) + fact tables (`fact_asset_snapshot`, `fact_assignment`, `fact_maintenance_cost`, `fact_depreciation`). `depreciation.depreciation_entries` is the first instance of such a derived read model and sets the pattern.
- [ ] **Population via the in-process scheduler** — periodic rollups (nightly/hourly) transform OLTP → facts, incremental where possible. Start with **materialized views** (scheduled `REFRESH`) for moderate scale; graduate to maintained fact tables once volume warrants it.
- [ ] **Reporting/Dashboard read from OLAP** — scoped by office (reuse data-scope on dimension keys), keeping report queries cheap and OLTP writes fast. Keep the read API stable so the backing store can change transparently.
- [ ] **Escalation path (only if needed)** — a column-store / external OLAP engine (e.g. DuckDB or ClickHouse) for very large volumes; introduce only when materialized views + fact tables on Postgres stop scaling.

### Global search (topbar)

> The topbar has a global-search input (placeholder wired in the app shell) but no backend. Plan a
> cross-entity **command palette** (⌘K) that searches assets, employees, offices, users, and requests,
> **respecting the caller's data-scope + field-permission**, returning typed/grouped results that
> deep-link to the record.

- [ ] **Frontend — command palette** — overlay opened by ⌘K or the topbar input: debounced query, results grouped by type (Aset, Pegawai, Kantor, User, Pengajuan) each with icon + deep link, keyboard navigation, recent searches, empty/loading states. Backed by `composables/api/useSearch` (mock first, then real). Design prompt at `DESIGN_BRIEF.md` §5.23.
- [ ] **Backend `/search?q=&types=`** — fan-out across modules, **scope-filtered** (reuse `callerOfficeScope`) and **field-permission-aware**; return typed hits `{ type, id, title, subtitle, url }` with a small per-type limit + "more" counts.
- [ ] **Indexing / scale** — start with Postgres full-text search (`tsvector` columns + GIN indexes, `unaccent` for accent-insensitive matching) per searchable entity; graduate to a dedicated engine (Meilisearch / Typesense / Elasticsearch) — populated by the scheduler/CDC — when volume, ranking, and typo-tolerance demand it (shares the indexing story with *Analytics / OLAP* above).

### Backend — Cross-cutting
- [x] **Audit logging** — `internal/audit` writer wired into every masterdata + user mutation (create/update/delete) with before/after diffs; office-scoped, filterable `GET /api/v1/audit` (gated by `audit.view`); migration 000014 adds `audit_logs.office_id`. (This is the **business audit trail** — distinct from application/observability logging below.)
- [x] **Structured logging & request correlation (ADR-0002)** — `log/slog` logger (JSON in prod, text in dev),
      slog-backed request middleware (method/path/status/latency) replacing `gin.Logger()`, a **request-id**
      middleware reading/echoing `X-Request-ID` (CORS allow/expose-listed) and binding `request_id`/`user_id`/`role_id`
      to every line, a context-carried logger, and a `safeAttrs` redaction helper (`password_hash`/tokens/`google_id`).
      Frontend `useLogger` propagates `X-Request-ID` per API call and ships client errors. **Done — PR #18.**
- [x] **Google OAuth2 login (ADR-0009, link-only)** — `/auth/google` + callback via `golang.org/x/oauth2` +
      `coreos/go-oidc/v3`: OIDC authorization-code + **PKCE (S256)**, single-use Redis state, ID-token verify
      (audience pinned, `email_verified` required), **link-only** account linking by verified email (no
      auto-provision), mints the same app JWT (refresh in **httpOnly cookie**). Feature-gated off without
      `GOOGLE_CLIENT_ID`. **Done — PR #21** (setup guide #22, Docker env fix #23; see `docs/google-oauth-setup.md`).
- [x] **Refresh token in httpOnly cookie (C1)** — refresh moved out of the JS-readable body into an
      HttpOnly/SameSite cookie scoped to `/api/v1/auth`; access token stays in memory. **Done — PR #20.**
- [ ] **Password reset / email verification** — Redis-TTL tokens (+ email later)
- [x] **Rate limiting (ADR-0004)** — Redis token-bucket (`go-redis/redis_rate`): per-IP + per-account login
      bands, global + refresh throttles, trusted-proxy client-IP hardening; configurable, fail-open. **Done — PR #19.**
- [ ] **Notifications (in-app)** — store + endpoints (approval decisions, maintenance reminders)
- [ ] **Scheduler (cron in-process)** — monthly depreciation; maintenance-due reminders
- [x] **Authorization admin endpoints** — `internal/authzadmin` — role CRUD (system-role protected), replace-set role_permissions/data_scope/field_permissions with Redis cache invalidation (ScopeService/FieldService gained `Invalidate`), canonical permission catalog (`GET /authz/catalog`). **Done — (2026-06-28).**
- [x] **Seed RBAC drift fix** — stale permission keys (`asset.read`/`asset.create`/`request.approve`) realigned to the canonical catalog (`asset.view`/`asset.manage`, `request.decide`, `approval.config.manage`); seed script and migration re-verified against `permissionCatalog`. **Done — (2026-06-28).**

### Frontend (screens built mock-first — remaining work)
- [ ] **API composable convention refactor** (ADR-0007) — (a) rename Indonesian DTO field keys to the
      backend's English `snake_case` contract (start `useOffices`/`Office`/mock store), (b) regroup
      `composables/api/` + `mock/` into module subfolders (masterdata/asset/identity/operational/reporting).
      Do before wiring screens to real APIs to avoid a mapping shim; keep lint/typecheck/test green.
- [x] **Kategori Aset screen** (#6) — built mock-first 1:1 from `docs/design/Kategori Aset.dc.html`:
      `app/pages/master/categories.vue` + `useCategories` + `mock/categories` + `components/category/`
      `CategoryFormSlideover.vue` + i18n + tests. Rich form carries the bank-FAM fields (asset_class,
      commercial+fiscal depreciation, GL account, fiscal group, capitalization threshold). **Done.**
- [ ] **Wire screens to real backend APIs** — replace `mock/*` fixtures with real `$fetch` behind the
      existing `composables/api/use*` interface, as each backend module lands; field-permission-aware forms
  - [x] **Peran & RBAC** (`/settings/rbac`) → wired to `/authz` (catalog + roles + role-permissions);
        English DTO; UUID `id` identity; system-role permissions editable per product decision; e2e updated. **Done (2026-06-28).**
  - [x] **Data Scope** (`/settings/data-scope`) → wired to `/authz` (catalog scope_modules + per-role scope policies);
        English DTO; UUID `id` identity; save only changed roles (dirtyIds); e2e spec updated against real seeded backend; orphaned mock deleted. **Done (2026-06-28).**
  - [x] **Field Permission** (`/settings/field-permission`) → wired to `/authz/roles` + `/authz/roles/:id/fields`; catalog
        `assets`+`users` (English field keys); UUID `id` identity; default-allow; save preserves other-entity rows + only PUTs changed roles; e2e spec added against real seeded backend; orphaned `mock/fieldPermission.ts` deleted. **Done (2026-06-28).** ⚠️ TODO: extend `FilterView` enforcement to `requests`/`employees`/other modules (see *Next session* §10 TODO).
  - [x] **Audit Trail** (`/settings/audit`) ✅ wired to `GET /api/v1/audit` — server-side filter + pagination (limit/offset); gate `audit.view`; entity-type filter from frontend `AUDIT_ENTITY_TYPES` catalog; expandable diff viewer; e2e spec against real backend; orphaned `mock/audit.ts` deleted. **Done (2026-06-29).** ⚠️ TODO: actor filter + role/summary/office-name columns dropped — backend response has no role/summary; resolving actor/office names requires `user.manage`/masterdata reads that an `audit.view`-only viewer may lack. Revisit if a viewer-accessible name lookup or enriched audit response lands.
- [ ] **Lokasi & Geografi** — office-location **map** screen (`nav.geography`); provinces/cities already live in Referensi, so this just plots offices on a map. No mockup yet; design prompt at `DESIGN_BRIEF.md` §5.21
- [ ] **Staff role menus** — wire staff nav (`myAssets`, staff `assignment`/`approval`) to pages/variants
- [x] **Google OAuth login** button + flow (UI) — login redirect + `?oauth=success/error` landing
      (refresh → fetchMe → navigate; i18n error reasons). **Done — PR #21.**
- [ ] **Profil & Pengaturan Akun** (`nav.profile` + `nav.accountSettings`) — no mockup yet; design prompt at `DESIGN_BRIEF.md` §5.22
- [ ] **E2E coverage** — Playwright specs for Dashboard, Assets, Settings, RBAC, Operasional clusters
      (currently only `login` + `master-offices`)
- [ ] Live light/dark visual pass for auth-gated screens (pending a stable backend to log in)

### Quality
- [x] Backend testing stack (ADR-0001): testify + testcontainers-go; `internal/testsupport` (Postgres/Redis containers, migration apply, `Reset`, seed helpers) + `backend-integration` CI job (`-tags=integration`, runs every PR; default `go test ./...` stays unit-only via the build tag).
- [x] Backend integration suites (real Postgres/Redis, behind `//go:build integration`):
      - **Masterdata data-scope:** office (#24), employee (#25), floor (#26), room — transitive floor→office scope (#26).
      - **Authz:** `ScopeService.Resolve` — 4 levels + fallback + Redis caching (#25); field-permission `ForEntity`/`FilterView` + caching (#26).
      - **Cross-module:** audit office-scoped `List` + `Log`/`Diff` round-trip (#27); reference engine generic CRUD + `coerce` (white-box) (#27).
      - **Approval engine + asset core** (#28 ← task-21): 11 approval scenarios (3-step chain, SoD, reject mid-chain, disposal/exclusion with cross-office security bypass, cancel, scope filter, threshold edit, executor atomicity/rollback) + 4 asset scenarios (field masking by role, tag atomicity sequential + per-year, read scope). 15 integration tests, all PASS.
      - **Asset attachments (MinIO)** (task-11): image round-trip, PDF upload, oversize rejection, disallowed type, scope enforcement, DB rollback (no orphan in MinIO). 6 integration tests (MinIO testcontainer), all PASS.
      - **Barcode / QR + label PDF** (task-9): Code128 PNG, QR PNG, BTN + generic label PDF (roll + sheet), scan-lookup, scope gate. Integration tests (`-tags=integration`) green.
      - **Asset documents (BAST)** (task-5): list, create, get, update, delete, file-upload (multipart), file-download; scope-gated + audited; rollback on MinIO failure. 10 integration tests (MinIO testcontainer), all PASS.
      - Remaining backend targets (minor): category sub-package, full HTTP+JWT request path.
- [ ] Optional seed data (provinces/cities, office types, etc.)

---

## Suggested order
1. **Audit logging** (cross-cutting — wire before more mutations accrue)
2. **Asset core + attachments (MinIO) + barcode**
3. **Approval (maker-checker)** → **Assignment** → **Maintenance**
4. **Depreciation** → **Reporting/Dashboard (+ PDF/Excel)** → **Import** — add the **Analytics / OLAP** read layer (materialized views → fact tables) once report data volume warrants it
5. ~~Structured logging (ADR-0002) + Google OAuth2 (ADR-0009) + rate limiting (ADR-0004)~~ ✅ **done (PR #18/#19/#21)** — remaining cross-cutting: **notifications + scheduler + authz admin endpoints**
6. **Wire the (already-built) frontend screens to real APIs** as each backend module lands —
   swap `mock/*` for real `$fetch` behind the same `composables/api/use*` interface
