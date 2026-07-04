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
> 12. ~~**Wire User Management screen** (`/settings/users`) to real `/api/v1/users`~~ ✅ **DONE (2026-06-29).** `useUsers` composable rewritten to server-side CRUD (`GET/POST/PUT/DELETE /api/v1/users`, limit/offset/search params); gate `user.manage`; role/office/employee pickers from real API lookups; employee picker filtered by selected office; orphaned mock NOT deleted (still imported by `useGlobalSearch` — see §TODO below); e2e spec updated against real seeded backend. **Authz/settings screen wiring batch now complete (RBAC + Data Scope + Field Permission + Audit Trail + User Management).**
>    - **TODO — server-side role/office/status filter dropdowns** dropped pending backend filter-param support on `GET /api/v1/users`; **reset-password** action dropped pending a backend reset endpoint. The office/employee lookup is capped at 100 entries (a searchable async picker is a follow-up if user/employee counts grow).
>    - **TODO — `mock/users.ts` cleanup**: still imported by `useGlobalSearch.ts` for the mock global search. Delete it when `useGlobalSearch` is wired to the real backend `/search` endpoint.
> 13. ~~**Wire Peta Lokasi screen** (`/master/map`) to real `GET /api/v1/offices/map`~~ ✅ **DONE (2026-06-29).** First of the master-data screen wiring batch. `useOfficeMap` rewritten to `GET /offices/map`; types migrated to `MapOffice`/`OfficeTier` (English snake_case DTO); `officeMapMeta` constants (3 tiers: pusat/wilayah/office → Pusat/Wilayah/Cabang); page rebound (lat/lng null-guard, load-error/retry, data-scoped); Leaflet `OfficeMap` component field-rename; e2e spec (`frontend/e2e/master-map.spec.ts`) + component test added; orphaned `mock/officeMap.ts` deleted.
>    - ~~**TODO — `office_types.tier` not yet editable:**~~ ✅ resolved — tier is now editable via Master Data Referensi (`/master/reference`, `office-types` resource: tier select pusat/wilayah/office). Offices whose type has `tier = NULL` still render as Cabang (conservative fallback in `toTier`) until a tier is assigned.
>    - **TODO — map shows empty-state until offices have lat/lng coordinates:** no production seed supplies coordinates. Office coordinates must be entered manually on the Office form (`/master/offices`). Per-office asset count is live (real `asset_count` from `GET /offices/map`) but returns 0 until the asset module is populated.
> 14. ~~**Wire Referensi screen** (`/master/reference`) to the generic reference engine~~ ✅ **DONE (2026-06-29).** `useReference` composable rewritten to real `$fetch` (`/api/v1/masterdata/reference/:resource`); 11 resources fully described (FK pickers: cities→provinces, models→brands; `office-types` `tier` editable via select; `vendors` gains `contact_name` + `address` fields; `is_active` toggle/column hidden for provinces & cities which lack the column; `departments` `code` field restored); `brands` label corrected to "Brand" (matching mockup); engine `typeEnum` + `tier` column wired on backend; orphaned `mock/reference.ts` deleted. **The office map is now meaningful** — office-type tier can be set (pusat/wilayah/office) via Referensi, so the map renders real Pusat/Wilayah/Cabang pins once tiers are assigned. **TODO:** cities and models need at least one province / brand created first (no production seed); empty FK picker shows a warning message.
> 15. ~~**Wire Kategori Aset screen** (`/master/categories`) to `GET/POST/PUT/DELETE /api/v1/masterdata/categories`~~ ✅ **DONE (2026-06-29).** `useCategories` composable rewritten to real `$fetch` (CRUD + `GET /categories/tree` for full unpaginated set); client-side tree ordering, filter/search/pagination retained; `CategoryFormSlideover` repointed to `~/constants/categoryMeta`; load-error/retry; `data-testid="category-parent-select"` added to parent picker; orphaned `mock/categories.ts` deleted; e2e spec rewritten against real seeded backend; mockup comparison verified (8 columns, filter bar with search + 2 selects + active toggle, 4-section slideover — 1:1 match). **Master-data wiring batch complete (Peta Lokasi + Referensi + Kategori Aset).**
> 16. ~~**Wire Pegawai screen** (`/master/employees`) to `GET/POST/PUT/DELETE /api/v1/masterdata/employees`~~ ✅ **DONE (2026-06-30).** `useEmployees` composable rewritten to real `$fetch` (CRUD `/api/v1/employees`, server-enforced `employees` data-scope); `Employee`/`EmployeeInput` English DTO; UUID FK pickers for office (required), department, position with table name-resolution (`officeMap`/`deptMap`/`positionMap`); inline `GET /offices?limit=100` for office options (data-scoped); backend `phone` column added (migration + DTO + query + OpenAPI); `data-testid` added to office/dept/position USelects; e2e spec created (`frontend/e2e/employees.spec.ts`); mockup comparison verified (7 columns, filter bar with search + 4 selects + reset, slideover NIP+status/name/dept+position/office+scope-note/email+phone — 1:1 match). `mock/employees.ts` retained (still imported by `useGlobalSearch` — delete when global search is wired to real `/search` endpoint).
> 17. ~~**Wire Kantor + Lantai + Ruangan screens**~~ ✅ **DONE (2026-07-02).** `/master/offices` (split-panel tree + detail + floors/rooms) wired end-to-end: `useOffices` → `GET/POST/PUT/DELETE /api/v1/offices` (tree built client-side from the flat scoped list), `useFloors` → `/api/v1/floors` (`?office_id=`) + `/api/v1/rooms` (`?floor_id=`) with floor/room updates resending the required `office_id`/`floor_id`. English DTO (`name`/`code`/`is_active`, FK `office_type_id`/`province_id`/`city_id`, `latitude`/`longitude`); form now uses **FK pickers** (office-type/province/city from `useReference`, city filtered by province) + **optional lat/lng inputs** (product decision — enables Peta Lokasi pins; mockup had none); tree icon/colour derived from the office type's **tier** (`tierMeta`). Detail resolves FK ids → names + parent name. Load-error/retry added. `mock/floors.ts` + `floors-mock.spec.ts` + `offices-mock.spec.ts` deleted; `mock/offices.ts` retained (decoupled `MockOffice` type — still used by `useGlobalSearch`). Unit specs (`use-offices`/`use-floors`) + 20-case component spec + real-backend e2e (creates an office type via Referensi, then an office). **Master-data screen-wiring batch now COMPLETE (Peta Lokasi + Referensi + Kategori + Pegawai + Kantor/Lantai/Ruangan).**
>    - **TODO — `mock/offices.ts`:** delete when `useGlobalSearch` is wired to the real `/search` endpoint.
> 18. ~~**Asset transfer (mutasi) — backend module**~~ ✅ **DONE (2026-07-02).** `internal/transfer` (service/executor/handler/routes) wired end-to-end: `asset_transfer` approval executor creates the `transfers` row on approval; `approved → in_transit → received` state machine (`POST /transfers`, `GET /transfers`, `GET /transfers/:id`, `POST /transfers/:id/ship`, `POST /transfers/:id/receive`, `GET /assets/:id/transfers`); receive atomically relocates the asset + records a `bast_transfer` asset-document (optional MinIO file); `transfer.view`/`transfer.manage` + `transfers` data-scope enforced on every verb; OpenAPI documented; 15 tests green. **Frontend Mutasi screen not started — mockup now available at `docs/design/Mutasi Aset.dc.html` (2026-07-03).**
> 19. ~~**Asset disposal — backend module**~~ ✅ **DONE (2026-07-02).** `internal/disposal` (service/executor/handler/routes) wired end-to-end; the `asset_disposal` approval executor was moved out of the asset package into this module, creating the `disposal.disposals` row only on approval with SQL-computed `gain_loss`; BAST attached via the shared asset-documents mechanism (`bast_disposal` doc type, `related_disposal_id`); `disposal.view`/`disposal.manage` + `disposals` data-scope enforced on every verb; `POST /disposals`, `GET /disposals`, `GET /disposals/:id`, `POST /disposals/:id/document`, `GET /assets/:id/disposal`; OpenAPI documented (`Disposal` schema + 5 paths); 9 tests green. **Deferred:** gain/loss GL export + depreciation-derived `book_value_at_disposal` (both wait on the depreciation module). **Frontend Disposal screen not started — mockup now available at `docs/design/Penghapusan Aset.dc.html` (2026-07-03).**
> 20. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-04): wire the Assets cluster.**
> 21. ~~**Wire Assets cluster** (Katalog/Detail/Form/Label) to real `/api/v1` + real-backend e2e~~ ✅ **DONE
>     (2026-07-04).** `AssetCreatePayload` widened to the full create-form field set; Katalog → real
>     `GET /assets` (server-side list/search/filter, FK name resolution); Detail → `GET
>     /assets/by-tag/:tag` (field-permission money masking, attachments gallery, tab empty-states for
>     Assignment/Maintenance/Depreciation — none of those modules exist yet); Form → create submits
>     `POST /requests` type `asset_create` (maker-checker — **no direct create**), edit is restricted to
>     mutable fields via `PUT /assets/:id`; Label/Barcode → real barcode/label-PDF endpoints. Real-backend
>     e2e (`frontend/e2e/assets.spec.ts`, rewritten): API setup (office/floor/room/category prereqs,
>     unique per run) → submit `asset_create` → approve as a second SoD-eligible user (maker ≠ checker)
>     → UI assertions across Katalog/Detail/Edit/Label + the `/assets/new` form flow (verified via a
>     follow-up API call, left pending/unapproved) + a negative empty-state search. All gates green:
>     backend build/vet/test/integration, Spectral, frontend lint/typecheck/test/build, full e2e suite
>     (57/57).
>     - **Bug found + fixed during e2e verification:** `pages/assets/[tag].vue` co-existed with the
>       `pages/assets/[tag]/` folder, which made Nuxt treat `[tag].vue` as the **parent route** for
>       `[tag]/edit.vue` — without a `<NuxtPage/>` in `[tag].vue`, `/assets/:tag/edit` silently rendered
>       the Detail page instead of the edit form (both via `page.goto` and via clicking "Ubah"). Fixed by
>       moving `[tag].vue` → `[tag]/index.vue` so Detail and Edit are sibling routes. This was a real,
>       previously-unnoticed regression — not a test artifact — so it's called out explicitly here.
>     - **Deliberate deferrals:** Import wizard still mock (no backend bulk-import endpoint yet);
>       Approval screen still mock (only the **submit** side is wired, via this task's `POST /requests`
>       call — the inbox/decide UI is still fixture-backed); asset **delete** is out of scope for this
>       screen — deletion goes through the Disposal screen/module instead; **field "Pemegang" (holder)
>       was dropped from the Form** per user decision — the holder is set via the future **Penugasan**
>       (assignment) module, not at asset creation/edit — Katalog's "Pemegang" column renders "—" until
>       that module lands.
> 22. ~~**Security follow-up (from Task 21 review)** — server-side `amount == payload.purchase_cost`
>     cross-check for `asset_create` on `POST /requests`~~ ✅ **DONE (2026-07-04).** Enforced in
>     `SubmitRequest.validate()` (numeric big.Rat equality; zero when payload has no cost; malformed
>     payload/amount rejected). Unit tests + OpenAPI updated. See the resolved note under *Assets
>     cluster* in §Remaining.
> 23. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-04): wire the Pengajuan &
>     Approval screen** (candidate (a) below — see item 24). Remaining candidates (see *Remaining*
>     below) for the *next* session: **(b)** build the last core Bank-FAM backend module — **Stock
>     opname** (session + item reconciliation: found/not_found/damaged/misplaced + report), following the
>     same pattern as `internal/transfer`/`internal/disposal` (approval executor where applicable +
>     scoped module + OpenAPI); **(c)** start the **depreciation** module, which several deferred items
>     (disposal book_value, GL export) depend on; **(d)** build the frontend **Mutasi** and/or
>     **Disposal** screens against the now-complete backends — mockups prepared 2026-07-03 for all six
>     v1.1 screens; **(e)** build **Assignment** (check-out/in, and the natural home for the deferred
>     "Pemegang" field) or **Maintenance**; **(f)** wire **global search** backend (`/search`) + drop the
>     last `mock/*` files. Confirm priority before starting.
> 24. ~~**Wire Pengajuan & Approval screen** (`/approval`) to real `/api/v1/requests`~~ ✅ **DONE
>     (2026-07-04).** `app/pages/approval.vue` rewritten off `~/mock/approval` onto `useApproval`
>     (inbox/list/get/approve/reject), `~/constants/approvalMeta` (`TYPE_META`/`STATUS_TONE`/
>     `REQUEST_TYPE_KEYS`/`STATUS_FILTERS`), and `payloadToView` for the Data section (summary rows for
>     `asset_create`/`asset_transfer`, before→after diff rows for `asset_disposal`/`valuation_exclusion`,
>     with an empty-state when the payload is masked/absent). Category/office FK names resolved via the
>     real `useCategories().tree()` + `useOffices().list({limit:100})` composables (best-effort — a
>     lookup failure falls back to raw ids, never blocks the screen). Pending tab reads the caller's
>     scoped inbox (`GET /requests/inbox`); other tabs (approved/rejected/cancelled/all) read `GET
>     /requests` with a `status` filter; a pending request **not** in the caller's inbox (SoD/scope
>     ineligible) renders a disabled "not eligible" lock instead of decide buttons — `view.eligible`
>     checks membership in the inbox id set, not just `status === 'pending'`. Approve/reject call
>     `POST /requests/:id/approve|reject`, then reload the tab + re-fetch the detail (the decide
>     response is the plain unenriched request row, so the page never renders off it directly).
>     Load-error + retry state added for the inbox/list fetch; a `detailLoading` skeleton covers the
>     detail-pane fetch. `nav.ts`'s `approval` item: removed the mock-era hardcoded `badgeCount: 8` and
>     added `permission: 'request.decide'` — confirmed `AppSidebar.vue` filters nav items on `.permission`
>     via `can()`, so the item now hides for roles without decide rights. Component test rewritten
>     (`test/nuxt/approval.spec.ts`, 14 cases: inbox load/empty/error, tab switching incl. cancelled,
>     detail fetch + payload/timeline rendering, approve/reject with note, not-eligible lock, cancelled
>     neutral result banner, sensitive-type warning banner, approved result banner hides decide buttons,
>     tab switch clears selection). Orphaned mock-era i18n keys (`approval.type.registrasi/penghapusan/
>     peminjaman/maintenance/valuasi`) deleted from both locale files — confirmed nothing (`mock/approval.ts`
>     itself, `useGlobalSearch.ts`, `approval-mock.spec.ts`) does an i18n lookup against them. `test/unit/
>     nav-model.spec.ts`'s stale `badgeCount: 8` assertion updated to assert the new `permission` gate
>     instead. `mock/approval.ts` **retained** — still imported by `useGlobalSearch.ts` for the mock
>     global-search result list (same pattern as the other pending `mock/*` cleanups; drop when global
>     search wires to a real `/search` endpoint, item (f) above).
>     - **Real badge count is still out of scope:** the sidebar no longer shows a hardcoded pending count;
>       a live one needs a global inbox-count store/poll, deferred until there's a cross-page need for it.

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

> **All 23 original `docs/design/*.dc.html` mockups are implemented.** Frontend screens currently
> render mock fixtures; they need wiring to real backend modules as those land (below).
> (Peta Lokasi uses a real Leaflet map per an explicit product decision, in place of the
> mockup's illustrative SVG; everything else matches its mockup 1:1.)
> **Six new v1.1 bank-grade mockups added 2026-07-03** (DESIGN_BRIEF §6: `Mutasi Aset`,
> `Stock Opname`, `Penghapusan Aset`, `Depresiasi`, `Dokumen BAST`, `Limit Otorisasi`) —
> screens **not yet built**; see *Remaining* below.

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
- [x] **Asset transfer (mutasi) — backend** — `internal/transfer` module (service/dto/executor/handler/
      routes, ADR-0008 4-file split); `asset_transfer` approval executor creates the `transfers` row only
      on approval (`approved` → `in_transit` → `received` state machine via `POST /transfers/:id/ship`
      and `/receive`); receive atomically relocates the asset (`assets.office_id`/`room_id`) and records a
      `bast_transfer` asset-document (optional MinIO file, best-effort); `transfer.view`/`transfer.manage`
      permissions + `transfers` data-scope module wired; `GET /assets/:id/transfers` history endpoint;
      OpenAPI documented (`Transfer` schema + 6 paths); 15 integration/unit tests (happy path, reject
      leaves no row, submit guards, scope + state-machine, BAST doc creation, asset history), all green.
      **Done — (2026-07-02).** **Frontend Mutasi screen still to build** — mockup available at
      `docs/design/Mutasi Aset.dc.html` (2026-07-03); build 1:1 per the mandatory-mockup convention.
- [ ] **Stock opname** — sessions + item reconciliation (found/not_found/damaged/misplaced) + report
- [x] **Disposal — backend** — `internal/disposal` module (service/dto/executor/handler/routes, ADR-0008
      4-file split); the `asset_disposal` executor was moved out of the asset package into this module's
      own `Executor()`; creates the `disposal.disposals` row only on approval (`assets.status → disposed`),
      with `gain_loss` computed in SQL (`proceeds` − `book_value_at_disposal`); BAST is attached via the
      shared asset-documents mechanism (`bast_disposal` doc type, `related_disposal_id` FK, optional MinIO
      file, best-effort); `disposal.view`/`disposal.manage` permissions + `disposals` data-scope module
      wired on every verb; `GET /assets/:id/disposal` history endpoint; OpenAPI documented (`Disposal`
      schema + 5 paths); 9 integration/unit tests (happy path + gain/loss, gain/loss null when book value
      absent, reject leaves no row, submit guards incl. already-disposed/duplicate/out-of-scope, scoped
      reads, BAST doc + bast_no persistence), all green. **Done — (2026-07-02).** **Deferred:** gain/loss
      GL-account export (journal-ready) and deriving `book_value_at_disposal` server-side from
      depreciation (currently maker-supplied, per the same value-tier caveat as before) — both wait on the
      depreciation module. **Frontend Disposal screen not started** — mockup available at
      `docs/design/Penghapusan Aset.dc.html` (2026-07-03); build 1:1 per the mandatory-mockup convention.
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
      `app/pages/master/categories.vue` + `useCategories` + `components/category/CategoryFormSlideover.vue`
      + i18n + tests. Rich form carries the bank-FAM fields (asset_class, commercial+fiscal depreciation,
      GL account, fiscal group, capitalization threshold). ✅ **Wired to `/api/v1/masterdata/categories`** (CRUD + `GET /categories/tree` for full unpaginated set; client-side tree/filter/pagination retained; orphaned `mock/categories.ts` deleted). **Done (2026-06-29).**
- [ ] **Wire screens to real backend APIs** — replace `mock/*` fixtures with real `$fetch` behind the
      existing `composables/api/use*` interface, as each backend module lands; field-permission-aware forms
  - [x] **Peran & RBAC** (`/settings/rbac`) → wired to `/authz` (catalog + roles + role-permissions);
        English DTO; UUID `id` identity; system-role permissions editable per product decision; e2e updated. **Done (2026-06-28).**
  - [x] **Data Scope** (`/settings/data-scope`) → wired to `/authz` (catalog scope_modules + per-role scope policies);
        English DTO; UUID `id` identity; save only changed roles (dirtyIds); e2e spec updated against real seeded backend; orphaned mock deleted. **Done (2026-06-28).**
  - [x] **Field Permission** (`/settings/field-permission`) → wired to `/authz/roles` + `/authz/roles/:id/fields`; catalog
        `assets`+`users` (English field keys); UUID `id` identity; default-allow; save preserves other-entity rows + only PUTs changed roles; e2e spec added against real seeded backend; orphaned `mock/fieldPermission.ts` deleted. **Done (2026-06-28).** ⚠️ TODO: extend `FilterView` enforcement to `requests`/`employees`/other modules (see *Next session* §10 TODO).
  - [x] **Audit Trail** (`/settings/audit`) ✅ wired to `GET /api/v1/audit` — server-side filter + pagination (limit/offset); gate `audit.view`; entity-type filter from frontend `AUDIT_ENTITY_TYPES` catalog; expandable diff viewer; e2e spec against real backend; orphaned `mock/audit.ts` deleted. **Done (2026-06-29).** ⚠️ TODO: actor filter + role/summary/office-name columns dropped — backend response has no role/summary; resolving actor/office names requires `user.manage`/masterdata reads that an `audit.view`-only viewer may lack. Revisit if a viewer-accessible name lookup or enriched audit response lands.
  - [x] **User Management** (`/settings/users`) ✅ wired to `/api/v1/users` — CRUD (GET list with server-side search+pagination, POST create, PUT update, DELETE remove); gate `user.manage`; role/office/employee pickers from real API lookups; employee picker filtered by selected office (office_id-aware `employeeFormOptions`); e2e spec against real seeded backend; status toggled via update endpoint. **Done (2026-06-29). Authz/settings screen wiring batch complete (RBAC + Data Scope + Field Permission + Audit Trail + User Management).** ⚠️ TODO: server-side role/office/status filter dropdowns + reset-password action dropped pending backend support; office/employee lookup capped at 100 (searchable async picker is a follow-up if counts grow); `mock/users.ts` retained until `useGlobalSearch` is wired to the real `/search` endpoint.
- [x] **Peta Lokasi** (`/master/map`) ✅ wired to `GET /api/v1/offices/map` — office lat/lng columns + geo endpoint with resolved type/province/city names + per-office asset count; data-scoped. `useOfficeMap` rewritten (real `$fetch`); types `MapOffice`/`OfficeTier`; 3-tier legend (Pusat/Wilayah/Cabang; Outlet folded into Cabang — `office_types.tier` not yet editable); coord-filtered Leaflet pins; load-error/retry; e2e spec added; orphaned `mock/officeMap.ts` deleted. **Done (2026-06-29).** ⚠️ TODO: map shows empty-state until offices have coordinates (no production seed); asset count real but 0 until asset module populated. (`office_types.tier` now editable via Referensi screen — resolved as part of §Referensi wiring below.)
- [x] **Master Data Referensi** (`/master/reference`) ✅ wired to generic reference engine (`GET/POST/PUT/DELETE /api/v1/masterdata/reference/:resource`) — 11 resources (office-types, departments, positions, units, maintenance-categories, problem-categories, brands, vendors, provinces, cities, models); FK pickers (cities→provinces, models→brands); `office-types` `tier` editable (select: pusat/wilayah/office) — **office map now meaningful** (tier settable → real Pusat/Wilayah/Cabang pins); `vendors` gains `contact_name` + `address` fields; `is_active` toggle/column hidden for provinces & cities (no `is_active` column); `departments` `code` field restored; `brands` label corrected to "Brand". Backend: `typeEnum` + `tier` column in reference engine. Orphaned `mock/reference.ts` deleted; e2e spec added (`frontend/e2e/master-reference.spec.ts`). **Done (2026-06-29).** ⚠️ TODO: cities and models need at least one province/brand created first (no production seed); empty FK picker shows a warning.
- [x] **Pegawai** (`/master/employees`) ✅ wired to `GET/POST/PUT/DELETE /api/v1/employees` — server-enforced `employees` data-scope; `useEmployees` composable rewritten (real `$fetch`, CRUD); `Employee`/`EmployeeInput` English DTO; UUID FK pickers for office (required), department, position with table name-resolution; inline `GET /offices?limit=100` for office options; backend `phone` column added (migration + DTO + query + OpenAPI); `data-testid` on office/dept/position USelects; e2e spec (`frontend/e2e/employees.spec.ts`); mockup comparison 1:1 (7 cols, 4-filter bar, slideover); `mock/employees.ts` retained (still used by `useGlobalSearch`). **Done (2026-06-30).** ⚠️ TODO: `mock/employees.ts` — delete when `useGlobalSearch` is wired to real `/search` endpoint.
- [x] **Kantor + Lantai + Ruangan** (`/master/offices`) ✅ wired to `/api/v1/offices` + `/api/v1/floors` (`?office_id=`) + `/api/v1/rooms` (`?floor_id=`) — split-panel tree (built client-side from the flat scoped list) + detail + inline floor/room CRUD; server-enforced `offices` data-scope. `useOffices`/`useFloors` rewritten (real `$fetch`); `Office`/`Floor`/`Room` English DTO; FK pickers (office-type/province/city via `useReference`, city filtered by province) + optional `latitude`/`longitude` inputs (product decision → Peta Lokasi pins); tree icon/colour from office-type **tier** (`tierMeta`); FK id → name resolution in detail; floor/room updates resend required `office_id`/`floor_id`; load-error/retry; `data-testid` on office-type/province/city USelects. Deleted `mock/floors.ts` + `floors-mock.spec.ts` + `offices-mock.spec.ts`; `mock/offices.ts` retained (decoupled `MockOffice`, used by `useGlobalSearch`). Unit + 20-case component spec + real-backend e2e (create office-type via Referensi → create office). **Done (2026-07-02). Master-data screen-wiring batch complete.** ⚠️ TODO: delete `mock/offices.ts` when `useGlobalSearch` is wired to real `/search`.
- [x] **Assets cluster** (`/assets`, `/assets/:tag`, `/assets/:tag/edit`, `/assets/label`, `/assets/new`) ✅
      wired to real `/api/v1/assets` + `/api/v1/requests` — Katalog: server-side list/search/filter +
      FK name resolution (category/office/brand/model); Detail: `GET /assets/by-tag/:tag`,
      field-permission money masking (`purchase_cost`/`accumulated_depreciation`/`book_value`), real
      attachments gallery, tab empty-states for the not-yet-built Assignment/Maintenance/Depreciation
      modules; Form: **create submits `POST /requests` type `asset_create`** (maker-checker — no direct
      create endpoint), edit is restricted to mutable fields via `PUT /assets/:id` (only `office_id`,
      `purchase_cost`, `asset_class`, `status` and `tag` stay read-only post-creation — `purchase_date`
      IS editable in edit mode); Label/Barcode: real barcode/label-PDF endpoints
      (`GET /assets/:id/barcode`, `POST /assets/labels`). `AssetCreatePayload` (backend) widened to the
      full create-form field set. Real-backend e2e rewritten (`frontend/e2e/assets.spec.ts`): API setup
      (office/floor/room/category prereqs, unique per run) → submit `asset_create` → approve as a second
      SoD-eligible user (maker ≠ checker) → UI assertions across Katalog/Detail/Edit/Label + the
      `/assets/new` form flow (verified via a follow-up API call) + a negative empty-state search.
      **Done (2026-07-04).**
      ⚠️ **Deliberate deferrals:** Import wizard still mock (no backend bulk-import endpoint); Approval
      screen still mock (only the submit call is wired — see *Next session* above); asset delete is out
      of scope here — deletion goes through the Disposal screen/module; **field "Pemegang" (holder)
      dropped from the Form** per user decision (holder assignment belongs to the future Penugasan
      module) — Katalog's "Pemegang" column shows "—" until that module lands.
      ✅ **RESOLVED (2026-07-04) — security follow-up from the branch's final review:** the approval
      submit used to trust the client-supplied `amount` without cross-checking `payload.purchase_cost` —
      a maker could send `amount: "0"` with a huge `purchase_cost` and route an `asset_create` through
      the lowest approval band. Fixed in `SubmitRequest.validate()` (`internal/approval/dto.go`): for
      `asset_create`, `amount` must **numerically** equal `payload.purchase_cost` (big.Rat comparison, so
      `"1000"` == `"1000.00"`), or equal 0 when the payload carries no `purchase_cost`; malformed
      payload/amount/cost strings are rejected too → 400. Unit-tested (12 table cases + other-types
      passthrough); OpenAPI `SubmitRequest.amount` description updated. (The disposal
      `book_value_at_disposal` sibling caveat still waits on the depreciation module.)
      ⚠️ **TODO (cleanup when Reports screen is wired):** old Indonesian `assets.status.*` i18n keys are
      still consumed by the mock Laporan screen (`pages/reports.vue` + `mock/reports.ts`) — delete them,
      tighten `AssetStatusBadge`'s prop from `AssetStatus | string` to `AssetStatus`, and drop the badge's
      legacy-status fallback in the same sweep. Also extract a shared `moneyCell`/rupiah formatter util
      (now duplicated across Katalog/Detail/AssetForm) before the Disposal/Depresiasi screens add copies.
      🐛 **Bug fixed during verification:** `pages/assets/[tag].vue` + the `pages/assets/[tag]/` folder
      made `[tag].vue` an unintended parent route for `[tag]/edit.vue` (no `<NuxtPage/>` to render the
      child), so `/assets/:tag/edit` silently showed the Detail page. Fixed by moving `[tag].vue` →
      `[tag]/index.vue` (siblings).
- [ ] **Staff role menus** — wire staff nav (`myAssets`, staff `assignment`/`approval`) to pages/variants
- [x] **Google OAuth login** button + flow (UI) — login redirect + `?oauth=success/error` landing
      (refresh → fetchMe → navigate; i18n error reasons). **Done — PR #21.**
- [x] **Profil & Pengaturan Akun** (`nav.profile` + `nav.accountSettings`) — built at `app/pages/account.vue` (`/akun`, Profil / Keamanan / Preferensi tabs) from `docs/design/Profil Akun.dc.html`; see *Done → Frontend*. (Checkbox was stale — screen and mockup both exist.)
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
