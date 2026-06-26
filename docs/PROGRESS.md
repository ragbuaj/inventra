# Inventra — Progress & Remaining Work

Living checklist of what's built vs. what's left. See [PRD.md](PRD.md) for scope and
[DATABASE.md](DATABASE.md) for the schema.

## ✅ Done

### Foundation & DevOps
- [x] Project scaffold (Go/Gin backend, Nuxt 4 frontend)
- [x] `docker compose up` full stack (Postgres + Redis + MinIO + migrate + backend + frontend)
- [x] GitHub Actions CI (backend build/vet/test · frontend lint/typecheck/build · Spectral)
- [x] PRD + DATABASE design docs

### Database (13 migrations · 9 schemas · 31 tables)
- [x] enums + `set_updated_at` + per-module schemas (`shared/identity/audit/masterdata/asset/import/approval/assignment/maintenance/depreciation`)
- [x] All tables incl. soft delete, partial-unique, FK indexes, seed (5 roles, 45 RBAC perms)

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
- [x] Mock-first data seam (`mock/*` + `composables/api/use*`) ready to swap to real `$fetch` behind the same interface
- [x] Tests: 343 Vitest unit + `mountSuspended` runtime specs green; lint/typecheck/build gate CI

> **All 20 `docs/design/*.dc.html` mockups are now implemented.** Frontend screens currently
> render mock fixtures; they need wiring to real backend modules as those land (below).

---

## ⛔ Remaining

### Backend — Feature modules
- [ ] **Asset core** — CRUD; `asset_tag` generator (atomic per office/category/year); status state machine; data-scoping + field-permission (mask `purchase_cost`/`book_value`); valuation-exclusion flag
- [ ] **Asset attachments (MinIO)** — Storage interface; upload + size/type validation; image compress + thumbnail; presigned/proxy access
- [ ] **Barcode / QR** — Code128 from `asset_tag` + QR; printable labels (single/batch); scan lookup
- [ ] **Approval (maker-checker)** — generic `requests`; routing (Manager/Kepala Unit/Kanwil/Superadmin by scope); segregation-of-duty; flows: asset_create, asset_delete, valuation_exclusion
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

### Backend — Cross-cutting (not yet implemented)
- [x] **Audit logging** — `internal/audit` writer wired into every masterdata + user mutation (create/update/delete) with before/after diffs; office-scoped, filterable `GET /api/v1/audit` (gated by `audit.view`); migration 000014 adds `audit_logs.office_id`
- [ ] **Google OAuth2 login** — `/auth/google` + callback + account linking (currently local-only)
- [ ] **Password reset / email verification** — Redis-TTL tokens (+ email later)
- [ ] **Rate limiting** — login anti-brute-force + throttling (Redis)
- [ ] **Notifications (in-app)** — store + endpoints (approval decisions, maintenance reminders)
- [ ] **Scheduler (cron in-process)** — monthly depreciation; maintenance-due reminders
- [ ] **Authorization admin endpoints** — Superadmin CRUD for roles, role_permissions, field_permissions, data_scope_policies (+ Redis cache invalidation)

### Frontend (screens built mock-first — remaining work)
- [ ] **Wire screens to real backend APIs** — replace `mock/*` fixtures with real `$fetch` behind the
      existing `composables/api/use*` interface, as each backend module lands; field-permission-aware forms
- [ ] **Lokasi & Geografi** — office-location **map** screen (`nav.geography`); provinces/cities already live in Referensi, so this just plots offices on a map. No mockup yet; design prompt at `DESIGN_BRIEF.md` §5.21
- [ ] **Staff role menus** — wire staff nav (`myAssets`, staff `assignment`/`approval`) to pages/variants
- [ ] **Google OAuth login** button + flow (UI; awaits backend `/auth/google`)
- [ ] **Profil & Pengaturan Akun** (`nav.profile` + `nav.accountSettings`) — no mockup yet; design prompt at `DESIGN_BRIEF.md` §5.22
- [ ] **E2E coverage** — Playwright specs for Dashboard, Assets, Settings, RBAC, Operasional clusters
      (currently only `login` + `master-offices`)
- [ ] Live light/dark visual pass for auth-gated screens (pending a stable backend to log in)

### Quality
- [ ] Broaden backend test coverage (services, handlers, integration)
- [ ] Optional seed data (provinces/cities, office types, etc.)

---

## Suggested order
1. **Audit logging** (cross-cutting — wire before more mutations accrue)
2. **Asset core + attachments (MinIO) + barcode**
3. **Approval (maker-checker)** → **Assignment** → **Maintenance**
4. **Depreciation** → **Reporting/Dashboard (+ PDF/Excel)** → **Import** — add the **Analytics / OLAP** read layer (materialized views → fact tables) once report data volume warrants it
5. **Google OAuth2 + rate limiting + notifications + scheduler + authz admin**
6. **Wire the (already-built) frontend screens to real APIs** as each backend module lands —
   swap `mock/*` for real `$fetch` behind the same `composables/api/use*` interface
