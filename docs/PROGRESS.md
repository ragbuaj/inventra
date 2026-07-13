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
>     - **Approved deviations from the mockup** (per the *catat-deviasi* convention), confirmed 1:1
>       against `docs/design/Pengajuan Approval.dc.html` in both light and dark mode: **(a)** a 5th
>       **Dibatalkan (Cancelled)** tab was added (mockup has 4: Menunggu/Disetujui/Ditolak/Semua) —
>       `cancelled` is a real backend `request_status`, so it needed a home in the UI; **(b)** the
>       **Lampiran (attachments)** section is a permanent empty-state ("Tidak ada lampiran.") — the
>       request payload has no attachment/file list yet, so the mockup's file-chip UI has nothing to
>       bind to; **(c)** the type filter lists only the **4 real backend types** (`asset_create`,
>       `asset_disposal`, `asset_transfer`, `valuation_exclusion` → Registrasi/Penghapusan/Mutasi/
>       Pengecualian Valuasi) instead of the mockup's 5 fictional ones — `peminjaman`/`maintenance`
>       have no submit path yet; **(d)** card/detail **titles are built from `type + office`**
>       (`rowTitle()`) rather than an asset/item name, because the list row payload is absent on
>       `GET /requests`/`GET /requests/inbox` (only the detail fetch resolves the full payload).
> 25. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-04): wire the frontend
>     Mutasi + Penghapusan screens** (candidate (d) — see item 26). Remaining candidates from that
>     session (see *Remaining* below): **(b)** Stock opname backend module; **(c)** Depreciation module;
>     **(e)** Assignment/Maintenance; **(f)** global search backend + drop the last `mock/*` files.
>     **Dev-stack notes from this session's e2e verification (both issues found, fixed, and
>     re-verified — full suite 61/61 green after):**
>     (1) the backend container had drifted to **stale source** (`docker compose ... watch` wasn't
>     actively syncing after a container recreate) — fixed by rebuilding + redeploying the backend
>     image; if e2e results ever look impossible, check container source freshness first.
>     (2) the **Superadmin default (`*`) data-scope policy** was found corrupted to `own` — a
>     parallel-worker run of the Data Scope settings e2e failed mid-mutation and never reverted it
>     (that test mutates live RBAC config as its subject; its cleanup is not failure-safe —
>     **follow-up: make that e2e revert via `afterEach`/API instead of in-test steps**). Restored
>     to `global` (user-approved). (3) local full `pnpm test:e2e` needs `RATELIMIT_ENABLED=false`
>     on the backend (now set via `backend/.env`; CI's e2e job already sets it). (4) the legacy
>     mock-backed approval test in `e2e/operasional.spec.ts` was deleted — superseded by the
>     real-backend `e2e/approval.spec.ts`.
> 26. ~~**Wire the frontend Mutasi Aset + Penghapusan Aset screens**~~ ✅ **DONE (2026-07-05).**
>     Backend additions on this branch first: migration `000022_transfer_condition_return`
>     (`condition_sent`/`transfer_date`/`returned` + `asset_transfers.return_note`); new
>     `POST /transfers/:id/reject-receive` (destination rejects an in-transit shipment → `returned`,
>     asset stays at the origin office, `return_note` recorded); enriched reads for both
>     `internal/transfer` and `internal/disposal` (asset/office/actor names resolved server-side);
>     `GET /approval-thresholds/preview` (submit-side UIs render the approval chain before
>     submitting) plus a plain-decimal hardening fix in the threshold validator that also tightened
>     PR #47's amount validator; OpenAPI synced for all of the above.
>     Frontend: `/transfers` (`app/pages/transfers.vue`) — Ajukan/Kotak Masuk/Riwayat tabs, asset
>     picker restricted to `status=tersedia`, inter-office/inter-region banner, ship + receive (with
>     BAST no.) + reject-receive actions, merged request+transfer history feed; `/disposals`
>     (`app/pages/disposals.vue`) — Ajukan/Riwayat tabs, valuation summary + laba/rugi card, approval
>     chain preview card (via the new preview endpoint), post-submit timeline, Lampirkan BAST on
>     completed rows. Shared: `AssetSearchPicker` component, `officeRegion`/`transferHistory`/
>     `disposalHistory` merge utilities (framework-free, unit-tested), meta constants
>     (`transferMeta`/`disposalMeta`), ~161 new i18n keys (id/en), the caller's `office_id` added to
>     auth state (needed for the inter-region check), and 2 new nav items. Tests: unit (meta,
>     region/merge helpers), `mountSuspended` component specs covering every mockup state (empty/
>     filled/invalid form, inbox empty/populated, gain/loss green/red/masked, chain-card + fallback,
>     post-submit timeline, loading/error/empty on every fetch), and 2 new real-backend e2e specs
>     (`transfers.spec.ts`, `disposals.spec.ts`, 8 tests: full transfer lifecycle incl.
>     reject-receive, full disposal lifecycle incl. BAST attach).
>     **Approved mockup deviations** (catat-deviasi convention, confirmed 1:1 against
>     `docs/design/Mutasi Aset.dc.html` and `docs/design/Penghapusan Aset.dc.html` in both light and
>     dark mode) — **(a)** a **Kirim** button was added to Riwayat (the mockup has no ship UI at
>     all); **(b)** the backend's `in_transit` enum value is localized as **"Dalam Pengiriman"** (the
>     mockup's own alur/status chips show the raw placeholder `in_transfer`, not real i18n);
>     **(c)** disposal method is the real 4-value backend enum `sale`/`auction`/`donation`/
>     `write_off` → **Dijual/Lelang/Hibah/Musnah** (the mockup's fictional "Scrap" is dropped,
>     "Lelang" added; subtitle copy updated to "jual/lelang/hibah/musnah"); **(d)** the fiscal
>     book-value line always renders "—" with its chip (the depreciation module doesn't exist yet,
>     so there is no fiscal value to show); **(e)** history rows that are still approval-request-only
>     (Diajukan/Menunggu/Ditolak/Dibatalkan — no `asset_transfers`/`disposals` row exists yet) render
>     limited info: the asset/method/value columns show "—". Noted during this task's re-verification:
>     the `assetName` resolver for these rows is currently a hard `() => null` stub (no lookup is
>     actually attempted, even though `target_id` is present on the request) — it always falls back
>     to "—" rather than resolving the name "when possible" as originally intended; a real
>     `GET /assets/:id` lookup for request-only rows would be a small, welcome follow-up;
>     **(f)** the transfer history's No. BAST is **plain mono text**, not a clickable link (the
>     Dokumen BAST screen doesn't exist yet — nothing to link to); **(g)** the Mutasi Aset nav item's
>     badge count is deferred (needs the same global inbox-count store as the Approval screen's
>     item 24 deferral); **(h)** the disposal Riwayat status filter drops **"Disetujui"** as a
>     standalone option (a disposal never rests there for the bands used in practice — it completes
>     atomically to `disposed` on the approval that satisfies the last chain step); **(i)**
>     `transfer_date` is backend-optional but the UI form requires it (contract compatibility — some
>     non-UI submitters may omit it).
>     **Follow-ups (tracked, not yet done):** ~~switch the disposal approval-amount basis from
>     `purchase_cost` (server-derived and conservative — the only tamper-proof value today) to the
>     **server-computed commercial book value** once the depreciation module lands (the
>     maker-supplied `book_value_at_disposal` caveat also disappears then)~~ ✅ **DONE — see item 28
>     (depreciation module, 2026-07-05)**; add the BAST-link
>     behavior to Mutasi history once the Dokumen BAST screen is built; fold
>     the transfer/disposal money fields (`proceeds`, `book_value_at_disposal`, transfer
>     `condition`/`reason`) into the field-permission catalog if a future role needs them masked.
>     From the final whole-branch review (all non-gating): apply `parsePlainDecimal` to
>     `ThresholdRequest.amount_from/to` (admin CRUD still accepts non-numeric → DB 500); client-side
>     from-office check on the Riwayat **Kirim** button (backend enforces; UX-only gap);
>     distinguish 422 from network/500 in the disposal chain-preview card's fallback message;
>     delete ~11 dead mockup-era i18n key pairs (`disposal.chain.role.*`, `*.form.noAvailable`, …).
>     **Gate sweep (task-13):** backend build/vet/test + full integration, Spectral, frontend
>     lint/typecheck/test (826 unit)/build all green. Full e2e: a first run (auto-parallel workers,
>     matching Task 12's mode) was **69/69 green**; a later same-session rerun at `--workers=1`
>     (matching CI) hit **1 failure** in `transfers.spec.ts` — not a code regression, but this
>     long-lived local dev DB (never reset between manual e2e runs across many sessions on this
>     branch) had accumulated **101 office rows**, one past the existing `officesApi.list({limit:
>     100})` cap already flagged for Pegawai — so a freshly created destination office fell outside
>     the picker's page and the UI couldn't select it. CI is unaffected (its e2e job starts every run
>     against a fresh `docker compose up` Postgres). Left as-is (no destructive cleanup of shared
>     dev-DB history without explicit approval); **follow-up:** either a periodic dev-DB reset for
>     this stack, or upgrade office/employee-style pickers to a searchable/paginated async lookup
>     (already a standing TODO elsewhere in this doc).
> 27. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-05): the Depreciation
>     module** (candidate (c) — see item 28). Remaining candidates from that session (see *Remaining*
>     below): **(b)** Stock opname backend module; **(e)** Assignment/Maintenance; **(f)** global
>     search backend + drop the last `mock/*` files.
> 28. ~~**Dual-basis depreciation module (PSAK 16 + PMK 72/2023) — backend + frontend + disposal
>     integration**~~ ✅ **DONE (2026-07-05).** Design: `docs/superpowers/specs/
>     2026-07-05-depreciation-module-design.md` (brainstormed + approved). Backend: migration
>     `000023_depreciation_periods` (state machine `open`/`computed`/`closed` + enum
>     `shared.depreciation_period_status`; seed `app_settings.depreciation.accumulated_gl_account` +
>     permissions `depreciation.view`/`depreciation.manage`, Superadmin-only); `internal/depreciation`
>     (ADR-0008 file split) — `engine.go` is a pure, DB-free, unit-tested dual-basis calculator
>     (commercial PSAK 16 straight-line/declining-balance + fiscal PMK 72/2023 kelompok 1–4/bangunan,
>     iterative month-by-month `math/big.Rat` half-up rounding, prospective-by-construction estimate/
>     impairment changes, salvage floor incl. Rp 1 memorial value, fiscal has no residual and absorbs
>     the final month's rounding remainder); `service.go` orchestrates `ComputePeriod` (idempotent,
>     `pg_advisory_xact_lock`-serialized, regenerates non-closed entries, updates
>     `assets.accumulated_depreciation`/`book_value` summary), `ClosePeriod` (sequential, immutable
>     after close), `Schedule`/`Journal`/`AssetSchedule`/`RecordImpairment`/`Periods`; 8 endpoints (`GET
>     /depreciation/periods`, `POST .../compute`, `POST .../close`, `GET /depreciation/schedule`, `GET
>     /depreciation/journal` (+ `/export?format=xlsx|pdf` via `excelize` + the existing gofpdf pattern),
>     `GET /assets/:id/depreciation` (masked when field-permission denies `book_value`), `POST
>     /assets/:id/impairment`) — all Superadmin-gated (`depreciation.view`/`manage`) + `depreciation`
>     data-scope module. Disposal integration: `POST /disposals` no longer accepts
>     `book_value_at_disposal` from the maker (removed from `SubmitRequest`/OpenAPI) — the server
>     computes it as the commercial book value as-of the disposal month (closing of the last
>     commercial entry ≤ that month; falls back to `purchase_cost` if the asset has no entries yet,
>     e.g. `non_susut` or never-computed) and that same value is now the disposal **approval amount**
>     (band routing reflects real write-off impact, not historical cost); `GET
>     /assets/:id/depreciation` exposes `computed_book_value` (commercial) so the frontend preview uses
>     the exact value the server will submit. Frontend: `/depreciation` screen (1:1 against
>     `docs/design/Depresiasi.dc.html` — header + basis toggle, 4 KPI tiles, Jalankan-Periode panel
>     with all 3 status states + the "belum dihitung" reminder banner, Jadwal-per-Aset tab incl.
>     impaired icon + fully-depreciated rows, Rekap Siap-Jurnal tab with the balanced-journal banner +
>     xlsx/pdf export, the impairment modal with live loss preview); asset-detail Depreciation tab
>     (`assets/[tag]/index.vue`) replaces the old empty-state with a real schedule + basis toggle,
>     backed by `GET /assets/:id/depreciation` (masked-response handling); Disposal screen's fiscal
>     valuation card and "berdasar nilai buku" approval-chain subtitle are now real (no more "menunggu
>     modul depresiasi" placeholder). `useDepreciation` composable + `depreciationMeta` constants +
>     ~full i18n id/en coverage; nav item "Depresiasi" (`depreciation.view`-gated) between Penghapusan
>     and Maintenance.
>     **Approved mockup deviations** (catat-deviasi convention, confirmed 1:1 against
>     `docs/design/Depresiasi.dc.html` in both light and dark mode; see spec §6) — **(a)** fully
>     depreciated assets are still shown in the schedule with a Rp 0 expense row (the mockup has no
>     example of this state — added so the KPI/"aset disusutkan: n" preview stays honest, only counting
>     assets with expense > 0); **(b)** the impairment row-action is disabled with a tooltip when the
>     Fiskal basis is active (the mockup doesn't distinguish — PSAK 48 impairment only applies to the
>     commercial basis, fiscal never recognizes it); **(c)** the "periode berjalan belum dihitung"
>     reminder banner is a new element (not in the mockup — a direct consequence of the manual
>     run-model product decision, so operators don't forget to run the period); **(d)** the
>     `book_value_at_disposal` field is gone from the Disposal submit form (server-computed now — a
>     real contract change, not a visual one); **(e)** the Disposal screen's "Jenjang Persetujuan" card
>     subtitle changed from "berdasar nilai perolehan" to **"berdasar nilai buku"** (i18n updated to
>     match the new approval-amount basis).
>     **Honest limitations (follow-up, not this phase)** — **useful-life revision via UI**: the
>     iterative engine already computes prospective effects of a changed estimate correctly, but no
>     endpoint/UI exists yet to edit an asset's useful-life/method/salvage after creation; **opening-
>     balance import**: pre-existing assets are backfilled in full as if the system had run since their
>     purchase date (correct if that matches historical books; importing a *different* historical
>     accumulated balance is not supported); **category-level flat Rp 1 policy**: `default_salvage_rate`
>     is a ratio only — a category-level flat-value override is deferred pending bank policy (existing
>     "Confirm with bank policy" backlog item).
>     **Disposed-asset regeneration rule** (recorded for auditors): once an asset's status flips to
>     `disposed`, `ComputePeriod` stops adding new entries for it after the disposal month, but a
>     **recompute of an already-non-closed period still deletes and does not regenerate** that asset's
>     entries for periods after disposal — history for a disposed asset survives only in periods that
>     were already `closed` before the disposal; a non-closed period's schedule reflects the asset's
>     current (disposed) reality, not a frozen pre-disposal snapshot.
>     **Accounting-policy note (possible future refinement):** the commercial declining-balance method
>     absorbs the entire remainder in the asset's final life month (so closing lands exactly on
>     salvage) — some PSAK declining-balance practice switches to straight-line for the last stretch of
>     an asset's life instead; current behavior is a deliberate, documented simplification, not a bug.
>     **Security follow-up (recorded, not a live gap):** `GET /depreciation/periods` is a global
>     (non-office-scoped) read, safe today because `depreciation.view` is Superadmin-only (global scope
>     by definition) — documented in `handler.go` with a SECURITY NOTE and in the OpenAPI tag. **If**
>     this permission is ever delegated to a non-global/scoped role in the future, the aggregate
>     `asset_count`/`total_amount` summary fields must be scoped or stripped for that role first (they
>     currently reflect the whole fleet, not the caller's office subtree).
>     **Gate sweep (task-13, 2026-07-05):** backend build/vet/test + full `-tags=integration` all green
>     (one `internal/masterdata/floor` testcontainers failure was transient Docker resource contention
>     under concurrent container churn — reran in isolation and it passed); Spectral 0 errors; frontend
>     lint/typecheck/test (882 unit, 84 files)/build all green. Full e2e was run twice: the first pass
>     accidentally auto-parallelized (a pnpm/script quirk swallowed `--workers=1`) and hit only the
>     already-known environmental issues (>100 accumulated dev-DB offices breaking the `limit:100`
>     office picker in `master-offices`/`transfers`, plus the depreciation spec's "reminder banner"
>     assertion failing because this month's period was already computed/closed from earlier manual
>     verification in this same session — both anticipated, not regressions); a second, forced
>     single-worker rerun (after truncating `depreciation.depreciation_entries`/`depreciation_periods`
>     to reset the monthly singleton) surfaced **one more** environmental issue of the same known
>     species as the item-25/26 note: the shared dev-DB's Superadmin default (`*`) data-scope policy
>     was again left at `own` by an incomplete revert in the first run's Data-Scope settings test,
>     which then 403'd every other spec's office-creation setup step (`approval`, `assets`,
>     `depreciation`, `disposals`, `transfers`) plus the pre-existing `master-offices` case — **not**
>     fixed in this task (a direct DB/API mutation to restore it was outside this task's docs-only
>     scope and was correctly declined), so it's recorded here rather than worked around. **Net read:
>     zero e2e failures are attributable to this branch's code** — every failure traces to one of the
>     two pre-existing, previously-documented dev-DB fragilities (office-count debris; the Data-Scope
>     test's non-atomic cleanup) that CI's fresh-database-per-run avoids entirely. Side-by-side against
>     `docs/design/Depresiasi.dc.html` (Playwright MCP, real seeded data, light + dark): header,
>     basis toggle, all 4 KPI tiles, the run panel in all 3 states (open + reminder banner, computed +
>     green "sudah dihitung" note, closed), schedule table anatomy (impaired icon, disabled-fiskal-
>     impairment tooltip, empty-search state), the journal tab (per-GL-account rows + "(tanpa akun
>     GL)" + balanced banner), and the impairment modal (loss preview, violet confirm action matching
>     the mockup) all matched 1:1 — only the approved (a)–(e) deviations above were present.
> 29. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-05): production ops
>     hardening (WAF → IaC → observability), see** `docs/superpowers/specs/` **ops-hardening design.**
> 30. ~~**Ops hardening Phase 1 — WAF (Coraza + OWASP CRS)**~~ ✅ **DONE (2026-07-06).** Custom Caddy
>     image + isolated smoke-test harness + prod Caddyfile rolled out DetectionOnly → tuned →
>     **Blocking**; ADR-0012 + `docs/DEPLOYMENT.md` WAF section. See *Foundation & DevOps* above.
> 31. ~~**Ops hardening Phase 2 — IaC (Ansible)**~~ ✅ **DONE (2026-07-06).** `ops/ansible/` playbook
>     (`base` + `docker` + `app` roles, idempotent, containerized tooling — host needs only Docker),
>     secrets via Ansible Vault (`*.example` committed, real `inventory.ini`/`vault.yml` gitignored);
>     ADR-0013 + `docs/DEPLOYMENT.md` §15 IaC sub-section. See *Foundation & DevOps* above.
> 32. ~~**Ops hardening Phase 3 — Monitoring/observability**~~ ✅ **DONE (2026-07-06).** Self-hosted
>     stack as a toggleable compose overlay (`docker-compose.monitoring.yml`): backend RED metrics
>     (`/metrics`, internal-only), Prometheus (15d retention + `mem_limit`) + exporters (node/cAdvisor/
>     postgres/redis/blackbox), Alertmanager → Telegram, Loki+Promtail (log), Grafana (datasource+
>     dashboard as-code) — only Grafana public, via its own subdomain, no WAF/no login bypass; secrets
>     via `*.example` + gitignore. Ansible `monitoring` role (`ops/ansible/roles/monitoring/`) brings the
>     overlay up idempotently, appended after `app` in `site.yml` — completes the ops-hardening trilogy.
>     ADR-0011 + `docs/DEPLOYMENT.md` §16. **Ops hardening (WAF → IaC → Monitoring) is now fully
>     complete** — see *Foundation & DevOps* below.
> 33. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-06): Stock opname (candidate
>     (b) — see item 34).**
> 34. ~~**Stock opname — backend + frontend + e2e**~~ ✅ **DONE (2026-07-07).** `internal/stockopname`
>     (service/dto/handler/routes, ADR-0008 4-file split + `report.go` for the on-the-fly Berita Acara
>     PDF/Excel render) wired end-to-end on migration `000025_stockopname` (`stock_opname_sessions`/
>     `stock_opname_items`, `followup_request_id` link, `stockopname.view`/`stockopname.manage`
>     permissions + `stockopname` data-scope module). Session lifecycle `open → counting →
>     reconciling → closed`: create snapshots every in-scope asset as a `pending` item; scan/manual
>     code entry and per-item result-setting (`found`/`damaged`/`misplaced`/`not_found`) drive the
>     count; `reconcile` locks editing; variance follow-up auto-generates the linked action
>     (`not_found` → `disposal.Submit`, `misplaced` → `transfer.Submit`, no new request-type/executor —
>     reuses the existing submit paths); `close` finalizes and unlocks the Berita Acara export. 11
>     endpoints, scope enforced read **and** write (`common.InScope` + `AllScope`/`OfficeIds`),
>     OpenAPI documented, backend integration/unit tests green (96s on a fresh Docker run, no flakes).
>     Frontend: `/stock-opname` (`app/pages/stock-opname.vue`) — 1:1 against
>     `docs/design/Stock Opname.dc.html` in light + dark — list (empty/loading/error+retry/populated
>     with per-session progress bar), detail toggle (no dedicated route) covering all 4 session
>     states, scan bar + manual code entry, segmented per-item result buttons (counting only,
>     read-only badges once reconciling/closed), variance panel with follow-up buttons, create/finish
>     (Berita Acara preview) modals; `SessionCard`/`StockopnameCreateSessionModal`/
>     `StockopnameFollowupModal` components; `useStockOpname` composable; `stockOpnameMeta` constants
>     (status/result tone maps); full i18n id/en. Real-backend e2e (`frontend/e2e/stock-opname.spec.ts`,
>     1/1) covers the full lifecycle + follow-up + a duplicate-follow-up guard assertion (via request
>     count, since the UI has no "already submitted" indicator yet — see follow-up below).
>     **Approved deviations from the mockup** (catat-deviasi convention, confirmed 1:1 against
>     `docs/design/Stock Opname.dc.html` in both light and dark mode) — **(a)** session status
>     `reconciling` renders as its own **"Rekonsiliasi"** chip (the mockup shows 3 statuses; the
>     backend's real state machine has 4, so `reconciling` needed a home); **(b)** the `damaged`
>     variance's follow-up button is **disabled with a "coming soon" tooltip** (the Maintenance module
>     doesn't exist yet, so there is no `→ maintenance` request path to submit); **(c)** item-result
>     labels follow **DB enum semantics** (`pending` = "Belum dicek", `not_found` = "Tidak ditemukan",
>     etc.) rather than the mockup's own copy where it differs; **(d)** the mockup's green "tap to
>     simulate" scan tile is **omitted** — the real manual/scan-gun code-entry path is kept instead
>     (camera scanning itself is deferred per spec §9, user-approved; simulate-tap has no real backend
>     analog to bind to).
>     **Follow-ups (tracked, not yet done):** the follow-up button has no "sudah diajukan"
>     submitted-state indicator in the UI (the backend safely rejects a duplicate follow-up request,
>     so this is a UX polish gap, not a correctness bug); session creation in the e2e goes via a direct
>     API call rather than the office-picker dropdown, because of the same documented office-picker
>     `limit:100` cap noted elsewhere in this doc (dev-DB office-count debris) — not a stock-opname-
>     specific issue.
>     **Gate sweep (task-13, 2026-07-07):** backend build/vet/test + full `-tags=integration` (all
>     packages, fresh run, no flakes, `internal/stockopname` 96.5s) all green; Spectral 0 errors (the
>     pre-existing `AssetCreatePayload` unused-component warning persists, unrelated); frontend
>     lint/typecheck/test (87 files, 926 unit incl. the new view-only follow-up-button negative
>     test)/build all green. Full `pnpm test:e2e` intentionally **not** re-run in this task (the
>     stock-opname e2e already passed 1/1 and is committed; the full local suite hits the
>     already-documented dev-DB office-count debris on *other* specs — CI runs the full suite against
>     a fresh database).
> 35. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-07): Assignment
>     (Penugasan/Peminjaman) — backend + frontend + e2e.** With Stock opname complete, the Bank-FAM core
>     module set was transfer + disposal + depreciation + stock opname; Assignment (candidate **(e)**)
>     was chosen next as the natural home for the deferred "Pemegang" field. Remaining after this:
>     **(e′)** Maintenance; **(f)** global search backend (`/search`) + drop the last `mock/*` files;
>     **(g)** Reporting & Dashboard.
> 36. ~~**Assignment (Penugasan/Peminjaman) — backend + frontend + e2e**~~ ✅ **DONE (2026-07-08,
>     branch `feat/assignment-module`).** `internal/assignment` (service/dto/handler/routes + `executor.go`,
>     ADR-0008 split) wired end-to-end on migration `000011_assignment` (`assignment.assignments`,
>     one-active-per-asset partial-unique index) + seed `000026_assignment_seed` (`assignment.view`
>     permission, `assignments` data-scope module, single office-level `assignment` approval band).
>     Two submission paths: **direct Manager check-out/check-in** (`POST /assignments`,
>     `POST /assignments/:id/checkin` — gated `assignment.manage`, atomically flips the asset
>     `available ↔ assigned`/`under_maintenance`) and **Staf peminjaman** (`POST /assignments/borrow`
>     → assignment-type approval request; the executor performs the check-out on approval). Reads:
>     `GET /assignments` (scoped+enriched list), `GET /assignments/:id`, `GET /assignments/available`
>     (own-office available-asset picker for Staf), `GET /assets/:id/assignments` (per-asset history).
>     Scope enforced read **and** write. OpenAPI documented (Task 8); backend integration + unit tests
>     green (Task 8 verified the full `-tags=integration` run for the branch-touched packages). Frontend:
>     **`/assignment`** (`app/pages/assignment.vue`) — Manager screen, 1:1 against
>     `docs/design/Penugasan Aset.dc.html` (Check-out / Check-in / Riwayat tabs, active-count badge,
>     colored condition column, load-error/retry); **`/peminjaman`** (`app/pages/peminjaman.vue`) — Staf
>     page (inline Ajukan Peminjaman form + "Pengajuan Saya" list with expandable approval timeline +
>     cancel); **Detail-Aset "Ajukan Peminjaman"** button + `AssignmentAjukanPeminjamanModal`
>     (locked-asset variant). `useAssignment` composable, `assignmentMeta` constants (status/condition/
>     request-status tone maps), full i18n id/en. 963 unit/component tests green across Tasks 7–14. Real-
>     backend e2e (`frontend/e2e/assignment.spec.ts`): direct Manager check-out → Riwayat Aktif +
>     Detail "Digunakan" + borrow disabled → check-in → Dikembalikan + available; Staf peminjaman submit
>     via UI → Menunggu → approve via API as a second office-level Manager (maker ≠ checker) → Disetujui
>     + assignment created; negative empty-Alasan guard. **Approved deviations (catat-deviasi convention):**
>     **(a)** borrow submit is a dedicated `POST /assignments/borrow` (not generic `POST /requests`) —
>     consistent with how transfer/disposal submit (the generic `SubmitRequest.Type` binding excludes
>     `assignment` and needs an `office_id` a Staf would not supply); **(b)** the Staf borrow asset-picker
>     uses `GET /assignments/available` (own-office scoped) because a Staf's `own` data scope makes
>     `GET /assets` empty; **(c)** the "Pengajuan Saya" asset name is a **best-effort client lookup**
>     (`useAssets().get(id)`; a 403 out-of-scope falls back to showing the id/tag) — the `mine` request
>     list/payload never snapshots the asset name server-side; **(d)** check-in "perlu maintenance" only
>     flips the asset to `under_maintenance` (no maintenance record is created — the Maintenance module
>     does not exist yet); **(e)** the Detail-Aset "Ajukan Peminjaman" button shows for **all**
>     `request.create` roles (incl. Manager), not Staf-only; **(f)** the disabled-button hint uses the
>     native `title` attribute, not a styled UTooltip popover (**user-approved** — the whole app uses
>     native `title`, there is no UTooltip infra); **(g)** the Riwayat "Kondisi" column renders as
>     **colored text** (baik/ringan/berat) per the Penugasan mockup, not a badge. **Honest limitations
>     (tracked, not yet done):** the frontend `RequestType` union does **not** yet include `'assignment'`
>     (the `myRequests`/peminjaman path uses a local test cast — small follow-up to add the member); and
>     the nav "Peminjaman" item has **no real pending-count badge** (static). **Gate sweep (task-13):**
>     backend build/vet/test + Spectral (0 errors, the pre-existing unrelated `AssetCreatePayload`
>     warning persists) + frontend lint/typecheck/test/build — see the task-15 report for counts. The
>     `assignment.spec.ts` e2e was **written + committed but not run locally**: this shared dev DB is
>     missing the `assignment.manage` grant for Superadmin/Manager (migration `000005` was amended to
>     add it *after* this DB had already applied `000005`, so the INSERT never re-ran — a pre-existing
>     dev-DB/seed drift, NOT a branch bug). Per user decision the dev DB was **not** mutated; CI runs the
>     full e2e suite against a fresh database where `000005` seeds the grant correctly.
> 37. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-08): Maintenance** (candidate
>     **(e′)** — check-in "perlu maintenance" already flags assets; this module is the natural consumer
>     of that signal). Remaining candidates after this: **(f)** global search backend (`/search`) + drop
>     the last `mock/*` files; **(g)** Reporting & Dashboard.
> 38. ~~**Maintenance (Jadwal/Catatan/Laporan Kerusakan) — backend + frontend + e2e**~~ ✅ **DONE
>     (2026-07-11, branch `feat/maintenance-module`).** `internal/maintenance` (service/dto/executor/
>     handler/routes) on migration `000027_maintenance_module` (adds `maintenance_records.schedule_id`
>     link + `stock_opname_items.followup_record_id`, seeds `maintenance.view` + the `maintenance`
>     data-scope module + a single office-level `maintenance` approval band — `maintenance.manage` was
>     already seeded in `000005`). **11 endpoints**: schedule CRUD (`/maintenance/schedules*`, interval +
>     `next_due_date`), record CRUD (`/maintenance/records*`, preventive/corrective, cost, vendor, status
>     state machine `scheduled → in_progress → completed/cancelled`, atomically flips the asset
>     `available/assigned ↔ under_maintenance` and releases it once no active record remains), the
>     **attention queue** (`GET /maintenance/attention` — `under_maintenance` assets with no active
>     record, "Perlu Tindak Lanjut"), the Staf **damage-report** submit (`POST /maintenance/reports`,
>     multipart with an optional photo attachment) which opens a `maintenance`-type approval request
>     (duplicate-guarded: one pending report per asset+maker via `CountPendingMaintRequests`/
>     `ErrDuplicatePending`) whose executor creates the corrective `scheduled` record on approval, and
>     per-asset history (`GET /assets/:id/maintenance`). Stock-opname **damaged followup** now creates a
>     corrective maintenance record **directly** (no approval step — explicit product decision, see
>     deviation (h)), idempotent via `stock_opname_items.followup_record_id`. Scope enforced read *and*
>     write; OpenAPI documented; integration tests green (11 scenarios incl. duplicate-pending guard +
>     followup idempotency). Frontend: **`/maintenance`** (`app/pages/maintenance.vue`), 1:1 against
>     `docs/design/Maintenance.dc.html` (due banner, 3 tabs — Jadwal/Catatan/Laporan Kerusakan, light +
>     dark verified) plus the approved additions below; `MaintenanceScheduleSlideover` +
>     `MaintenanceRecordSlideover` components; **Detail-Aset "Riwayat Maintenance" tab** (Task 12, lazy-
>     loaded like the Depreciation tab); the Approval screen's `RequestType`/`TYPE_META` already carried
>     `'maintenance'` (Task 8); Stock Opname's damaged-item follow-up button now calls this module instead
>     of showing "coming soon". `useMaintenance` composable; `maintenanceMeta` constants (status/type
>     tone maps, due-date helpers, Rupiah formatter). Real-backend e2e (`frontend/e2e/maintenance.spec.ts`):
>     Manager creates a schedule (due-today badge + banner) → "Buat Catatan" prefilled → save
>     `in_progress` (asset → "Maintenance") → edit → `completed` + biaya (Catatan row "Selesai" + `Rp
>     150.000`; schedule's next-due shifts past the banner; asset → "Tersedia"); Staf submits a damage
>     report → "Menunggu Review" → approve via API as a second office-level Manager (maker ≠ checker) →
>     "Disetujui" + a corrective `scheduled` record appears in the Manager's Catatan; negative: submit
>     disabled with empty kategori, and a completed record reopens read-only (no save button). **Approved
>     deviations (catat-deviasi convention):** **(a)** a "Tambah Jadwal" button + full create/edit
>     slideover for schedules (the mockup has no schedule-creation UI at all — schedules are display-only
>     there); **(b)** Catatan rows are clickable to open an edit slideover (the mockup has no row actions);
>     **(c)** a "Perlu Tindak Lanjut" section surfaces the attention queue (not in the mockup) — reflects
>     the explicit product decision that check-in "needs maintenance" does **not** auto-create a record
>     (spec decision #3); **(d)** the record form's "Vendor / Teknisi" field is a `vendor_id` select
>     (`vendors` reference resource) — the free-text `performed_by` column exists server-side but is not
>     exposed in the form (no UI need yet); **(e)** the due banner + due-badge coloring (overdue/today/
>     ≤7 days) is computed **client-side** from `GET /maintenance/schedules?limit=100` — no dedicated
>     backend "due soon" endpoint; **(f)** "reminder" is an in-page banner only — no push/email
>     notification channel exists; **(g)** schedule cards always render a fixed **"Preventive"** type badge
>     and a vendor-less task line — `maintenance_schedules` has no `type`/`vendor_id` column (only
>     records do), a data-model consequence, not a UI choice; **(h)** the stock-opname "damaged" followup
>     creates the corrective record **directly**, with no approval step (mirrors the `not_found`→disposal
>     and `misplaced`→transfer followups, none of which go through maker-checker either), idempotent via
>     `followup_record_id`; **(i)** Staf's `maintenance` data-scope is seeded **`office`** (mirroring the
>     `assignments` module precedent), not the spec's literal `own` wording — via direct API a Staf can
>     submit a damage report for any asset in their office (still maker-checker-gated + duplicate-guarded);
>     the UI picker limits the choice to assets the Staf actually holds. Intentional, consistent with the
>     borrow-request precedent. **Honest limitations:** the photo-upload path (`POST /maintenance/reports`
>     multipart) is exercised by unit/component tests but **not** integration-tested end-to-end (would
>     need a MinIO testcontainer harness like the asset-attachments suite); the Laporan tab's "Riwayat
>     Laporan Saya" resolves the reported asset's name via a best-effort client-side `useAssets().get(id)`
>     lookup (same pattern as Assignment's "Pengajuan Saya"), not a server-side snapshot. **Closed
>     follow-up:** the frontend `RequestType` union already included `'assignment'` **and** `'maintenance'`
>     going into this module (the old local test cast is gone). **Bug found while building the e2e, then
>     fixed with a different design after code review:** the Laporan tab's "Aset yang Anda pegang" picker
>     called `GET /assignments?status=active&employee_id=...` with a **client-supplied** `employee_id`. A
>     first fix attempt (migration `000028_assignment_staf_view`) granted Staf `assignment.view` so that
>     call would stop 403ing. Code review caught that this reopened the door wider than intended: with
>     `assignment.view` + the existing office-level data scope, `employee_id` being client-supplied and
>     optional meant any Staf could simply omit it and read **every coworker's assignments in the office**
>     — a regression against PRD §2.2 (Staf = data miliknya) and against 000026's own recorded decision to
>     withhold `assignment.view` from Staf. **Final design (mirrors the `/assignments/available`
>     precedent):** a dedicated `GET /assignments/mine` endpoint, gated by `request.create` (already
>     seeded for Staf in `000005` — **no new permission grant**), which resolves the caller's employee id
>     **server-side** from the JWT-resolved user record (never from the request), so the response can only
>     ever contain the caller's own rows; a caller with no linked employee gets back an empty list (200).
>     Migration `000028` was **deleted** (it had never been applied to the shared dev DB — confirmed via
>     `migrate ... version` = 27 before deletion — so no `down` run was needed). Frontend: `useAssignment().mine()`
>     replaces the `list({ employee_id })` call in `maintenance.vue`; the local employee-id branch/early-return
>     is gone since the server now always resolves it. **New integration coverage**
>     (`internal/assignment/assignment_integration_test.go`): `TestAssignment_Mine_returns_only_caller_own_rows`
>     (two Staf in the same office, each with an active assignment — `Mine` returns only the caller's own
>     row) and `TestAssignment_Staf_role_lacks_assignment_view` (seed-level guard asserting the Staf role
>     still has zero `assignment.view` rows after all migrations — the general `GET /assignments` list
>     route stays 403 for Staf). **Gate sweep:** backend build/vet/test green; `go vet -tags=integration
>     ./...` green; full `-tags=integration ./... -count=1 -p 1` green (all packages); Spectral 0 errors
>     (pre-existing `AssetCreatePayload` warning only); frontend lint/typecheck/test green (1021
>     unit/component tests, same count — the picker tests were updated in place, not added/removed).
>     **E2E status:** the scenario-2 `beforeAll`'s try/catch (previously wrapping both the borrow+approve
>     seeding **and** a permission probe in one block, which could silently self-skip a real regression)
>     was narrowed: since `/assignments/mine` needs only the already-stable `request.create` grant (no
>     migration-timing gap like `assignment.manage`'s — see item 36), the probe is gone and the seeding now
>     runs unwrapped, so a genuine borrow/approve regression fails the suite loudly instead of skipping.
>     Full local run against the dev stack: **3/3 PASS** (after one harness fix: the mid-test switch from
>     the Staf session to the admin login now clears cookies/localStorage first — `/login` redirects
>     authenticated users because the httpOnly refresh cookie silently restores the session).
> 39. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-11): global search (candidate
>     (f) — see item 40).** Remaining candidate after this: **(g)** Reporting & Dashboard.
> 40. ~~**Global search — backend `GET /search` + wire `useGlobalSearch` + drop orphaned mocks**~~
>     ✅ **DONE (2026-07-11, branch `feat/global-search`).** Design:
>     `docs/superpowers/specs/2026-07-11-global-search-design.md` (brainstormed + approved; includes the
>     CQRS options analysis — **Opsi A "CQRS level kode"** chosen: uniform read model built on-the-fly by
>     5 per-entity scope-gated queries; view-`UNION ALL` and projection-table variants rejected for
>     search, with **CQRS level 2 (pre-aggregated projections) explicitly reserved for the Reporting
>     phase**). Backend: migration `000028_search_trgm` (`pg_trgm` extension + 9 partial GIN trigram
>     indexes on the searched columns — also accelerates the existing list-endpoint ILIKEs;
>     `users.email` deliberately unindexed, citext); `db/queries/search.sql` (5 queries, `count(*)
>     OVER()` totals, `LIMIT 5`); `internal/search` (ADR-0008 split) — handler resolves per-entity gates
>     (assets = `asset.view` + scope `assets`; employees/offices/requests = auth-only + their scope
>     modules; users = `user.manage`, unscoped — mirroring each entity's existing list endpoint), a
>     failed gate **silently omits the group** (never 403), infra errors 500; service fans out via
>     `errgroup` into fixed-order groups (assets→employees→offices→users→requests), `q` < 2 runes →
>     empty groups without querying. Integration tests: 7 scenarios incl. real **subtree-expansion**
>     coverage (child office seeded; assertion mutation-tested), permission-gating absence checks, and
>     limit-vs-total window semantics; full `-tags=integration ./... -p 1` gate 29 pkgs green. OpenAPI:
>     `Search` tag + path + schemas (3.1-style nullability). Frontend: `useGlobalSearch` rewritten to
>     `GET /search` (same `search(q)` signature; type→route/icon/labelKey mapping client-side; requests
>     title composed `t('approval.type.*') · office`); `CommandPalette.vue` gains a **250 ms debounce**
>     (seq guard retained) + `StatusBadge kind="approval"` for pengajuan rows; specs rewritten (6
>     composable cases + 10 palette cases incl. debounce single-call + resolved-badge-label); orphaned
>     `mock/offices.ts`/`mock/employees.ts`/`mock/users.ts`/`mock/approval.ts` + `approval-mock.spec.ts`
>     **deleted** (barrel `mock/index.ts` trimmed; retained: `mock/helpers.ts` → useDashboard/useReports/
>     useAccount, `mock/assets.ts` → import wizard, `mock/dashboard.ts`/`mock/reports.ts`/
>     `mock/notifications.ts` → consumers not yet wired). Real-backend e2e
>     (`frontend/e2e/global-search.spec.ts`, 2/2): API-created unique office → Ctrl+K → search → navigate
>     to `/master/offices`; empty-state negative.
>     **Approved deviations from the mockup/mock behavior** (catat-deviasi convention): **(a)** 250 ms
>     **debounce** on palette queries (mock fired per keystroke — wasteful against a real backend);
>     **(b)** Pengajuan result **title = request type + office name** (the real schema has no title
>     column; the old mock searched a synthetic `judul`), matching via `reason` + id-prefix instead;
>     **(c)** Kantor rows show **no status badge** (mock showed a hardcoded Indonesian "aktif" chip —
>     an i18n violation to reproduce); **(d)** User rows render **name + email** (mock rendered
>     email + "role · office" — role/office enrichment isn't worth a 3-table join for a palette row).
>     **Follow-ups (tracked, not done):** "Lihat semua (n)" group buttons remain non-functional (as in
>     the mockup-era UI — no list-page-with-query target defined); OpenAPI documents `q` `minLength: 2`
>     though the handler returns 200-empty (not 400) below it — description discloses the real
>     behavior; e2e `getByText('Kantor')` could be scoped to the palette overlay for extra robustness.
> 41. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-11): Reporting & Dashboard
>     (candidate (g)) — see item 42.** Design:
>     `docs/superpowers/specs/2026-07-11-reporting-dashboard-design.md` + plan
>     (`.superpowers/sdd/` task briefs 1–15).
> 42. ~~**Reporting & Dashboard — backend `internal/report` module + both frontend pages + e2e**~~
>     ✅ **DONE (2026-07-12, branch `feat/reporting-dashboard`).** Backend `internal/report` (ADR-0008
>     split: `service`/`dto`/`handler`/`routes` + `export.go`): migration `000029_report_scope_seed`
>     (seeds the `report` data-scope-policy module rows only — the `report.view`/`report.export`
>     permissions were already seeded back in migration `000005`);
>     **4 endpoints** — `GET /dashboard/summary`, `GET /dashboard/export`, `GET /reports/:type`,
>     `GET /reports/:type/export` (`report.view`/`report.export` + `report` data-scope enforced on
>     every verb, office filter ⊆ caller scope via `CallerOfficeScope`). Aggregates run **directly over
>     the OLTP tables** (no OLAP/MV yet — reserved CQRS level 2). **Cache policy:** only the dashboard
>     summary is Redis-cached (`dashboardCacheTTL = 90 * time.Second`, get-or-compute per
>     scope+period+office key); **reports and all exports always compute fresh**. **7 report types** —
>     assets (Daftar Aset & Nilai Buku), depreciation (commercial/fiscal basis toggle), utilization,
>     maintenance-cost, transfers, disposals (**incl. the gain/loss GL recap — closes the disposal-module
>     deferral**), stock-opname (+ Berita Acara). `excluded_from_valuation` assets are **still counted**
>     in headcounts but **excluded from money totals** (with a transparency note on the dashboard).
>     Exports (xlsx via excelize, pdf via gofpdf) go through the single **`columnsFor` DRY seam** so every
>     report type serializes uniformly. Tests: unit (`dto`/`export`/`service_helpers`) + integration
>     (`report_integration`/`report_fam`/`report_http`/`report_run`, incl. 403-gating, TTL-bound cache
>     keys, exclusion rule); full `-tags=integration ./... -p 1` gate green (30 pkgs). OpenAPI: a single
>     `Report` tag covering all dashboard + report paths/schemas. Frontend: **both pages wired** — `useDashboard`/`useReports`
>     rewritten to real `$fetch`; `pages/index.vue` (dashboard) + `pages/reports.vue` (7 cards) rebuilt;
>     `mock/dashboard.ts` + `mock/reports.ts` **deleted** (only `mock/helpers.ts`, `mock/assets.ts`,
>     `mock/notifications.ts` remain — see item 43). New shared component **`PeriodFilter`** — the repo's
>     **first `UCalendar`-range** date component (preset + "Rentang kustom…"). Real-backend e2e
>     (`frontend/e2e/dashboard.spec.ts` + `reports.spec.ts`, 9/9 green). Full gate sweep (Task 15):
>     backend build/vet/test + integration, Spectral (0 errors, 9 known warnings), frontend
>     lint/typecheck/test/build all green; **side-by-side mockup comparison** (Dashboard.dc.html +
>     Laporan.dc.html, light + dark) verified 1:1 — screenshots in `.superpowers/sdd/task-15-*.png`.
>     **Approved deviations from the mockup** (catat-deviasi convention): **(a)** the mockup's plain
>     "Scope" select → a **Kantor-dalam-scope** select (hidden when the caller holds ≤1 office);
>     **(b)** Periode gains a **"Rentang kustom…"** option (UCalendar range); **(c)** the dashboard
>     Ekspor button is a **PDF/Excel dropdown** (mockup had a single button); **(d)** **3 extra report
>     cards** (transfers/disposals/opname) with no mockup — built to the same card anatomy (7 cards
>     total); **(e)** the depreciation report gains a **Basis komersial/fiskal toggle**; **(f)** the
>     dashboard inline **reject opens a note modal** (mockup rejected inline); **(g)** **error + retry**
>     states added (mockup has none); **(h)** KPI trends show a **real %** when computable, else a static
>     descriptor ("Relatif stabil"/"+ aktif bertambah"); **(i)** the status donut renders **all 7 real
>     statuses** (user-approved — mockup showed 5); **(j)** the maintenance-due KPI uses a **fixed
>     ≤ today+7d window** (user-approved). Plus a minor **"Atur Ulang Filter"** reset button on the
>     reports filter bar (sensible addition; mockup had only Terapkan).
>     **Honest limitations / follow-ups (tracked, not done):** grouped reports cap at **>1000 groups →
>     KPIs silently understate** (follow-up: a separate COUNT query); `report.gl.*_account` app-settings
>     are **unseeded**, so GL account codes render `""` in the disposal recap until configured;
>     **historical value snapshots deferred** to the Analytics/OLAP phase (trends compare against a
>     computed prior window, not a stored snapshot); the donut's colour/label index-mapping **relies on
>     the backend status order**; the transfers/disposals/opname report types **have no mockup** (built
>     to card anatomy); route **badge counts still deferred**. **Note — dev-stack image staleness:** the
>     running `asset-management-frontend` Docker image predates this branch, so `:3000` served the old
>     mock dashboard until rebuilt; the mockup comparison was run against a fresh host build of the
>     current branch (verified real backend data: 185 assets, live E2E categories/offices) — the deployed
>     image needs a rebuild to reflect the new wiring.
> 43. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-12): Import module (bulk
>     CSV/XLSX) — see item 44.** Design: `docs/superpowers/specs/2026-07-12-import-module-design.md`
>     + plan `docs/superpowers/plans/2026-07-12-import-module.md`.
> 44. ~~**Import module — backend `internal/importer` engine + all 5 targets + frontend wizard + e2e**~~
>     ✅ **DONE (2026-07-12, branch `feat/import-module`).** Generic bulk-import engine (ADR-0008 split
>     + `parser`/`template`/`errreport`/`worker`/`target`): migration `000030` (import_rows table, job
>     routing office, batch enums `validated/confirmed/executing/awaiting_approval/cancelled` +
>     `asset_import` request type), `000031` (seed `asset_import` approval thresholds mirroring
>     `asset_create`), `000032` (seed `masterdata.employee.manage` to superadmin+kepala_kanwil — was
>     never granted). **`TargetImporter` interface** with 5 targets: **asset** (batch = ONE
>     `asset_import` approval request, executor creates all on approval), **employee**, **office**,
>     **reference:provinces**, **reference:cities** — each in its own domain package. **Async DB-queue
>     worker** (`FOR UPDATE SKIP LOCKED`, validate→execute phases, Redis progress, startup recovery,
>     started in `main.go` on a cancellable ctx). **8 endpoints** `/imports` (template, upload, list,
>     get, rows, confirm, cancel, error-report) — per-target permission gate + `imports` data-scope +
>     owner-only job access; maker scope resolved from role (fail-closed). Exports/parsing via excelize;
>     money summed with `big.Rat`. Frontend: **reusable `ImportWizard.vue`** (props target/permission)
>     driven by real `useImports` composable — 3-step wizard (upload→validate→result) + asset
>     approval-pending state + resume; `pages/assets/import.vue` rewired, new `pages/master/import.vue`
>     (per-target `?target=`) + Import buttons on Pegawai/Kantor/Referensi; **`mock/assets.ts` DELETED**
>     (last non-notification app-shell mock). Tests: backend unit + 12 integration (incl. **mid-batch
>     tag-collision regression** proving the tx-poisoning fix), frontend 1156 unit, **Playwright e2e
>     3/3 green against real backend** (asset maker-checker happy path + employee cycle + validation
>     rejection). Full gate green: backend build/vet/test + `-tags=integration ./... -p 1` (31 pkgs),
>     Spectral (0 err/9 known warn), frontend lint/typecheck/test/build; **side-by-side vs
>     `Import Aset.dc.html` verified 1:1** (upload screen — stepper, template card, drop-zone, column
>     badges, validate button all match). OpenAPI `Import` tag added.
>     **Approved deviations from the mockup** (catat-deviasi convention): **(a)** asset step-3 shows a
>     **"Diajukan untuk persetujuan"** state before the final result (consequence of batch approval —
>     the mockup assumed direct creation); **(b)** the master-data import entry point + wizard have **no
>     mockup** (built to the Import Aset anatomy); **(c)** the progress bar is **real polling** progress
>     (mockup was cosmetic); **(d)** the preview is **server-validated + paginated** (mockup was a static
>     12-row table); **(e)** the wizard **resumes an active job** when reopened. (The asset-specific cell
>     formatting — `Rp` prefix, monospaced tag — was restored so the asset table matches the mockup; the
>     generic renderer serves the non-asset targets.)
>     **Bugs found by the e2e that unit/integration/component tests all missed (fixed):** row-data
>     nested-vs-flat contract mismatch; Go nil-slice `Errors` → JSON `null` (frontend crash); completed
>     job `failed_rows` overwritten by the execution-only count (validation failures discarded);
>     `masterdata.employee.manage` never seeded.
>     **Honest limitations / follow-ups — original list, now largely cleared (see item 48):**
>     ~~employee/office importers cover only the core columns (no dept/position lookup yet)~~ **done —
>     employee dept/position lookup landed (item 48); office import remains core-columns-only (not
>     planned)**; ~~only 2 reference targets wired (provinces/cities)~~ **done — brand/unit/model added
>     (item 48); room/floor targets remain genuinely open (deferred, not implemented)**; ~~error reports
>     are generated on-demand (not stored to MinIO — `error_report_key` stays null)~~ **done — persisted
>     to MinIO on validate+execute completion (item 48); the asset target still always rebuilds
>     on-demand by design (approval-gated, content can change between confirm and view)**;
>     ~~office/reference import paths have no e2e~~ **done — covered (item 48)**. **Still open:** the
>     dup-code DB check is **case-sensitive** while validation is case-insensitive (narrow TOCTOU under
>     concurrency, inherited from the asset-tag pattern); Redis validate progress is written **once at
>     100%** (no incremental); the importer maps `employee`→`masterdata.employee.manage` while employee
>     **CRUD** uses `office.manage` (000032 aligns the grants). **Note — dev-stack image staleness:** the
>     running `asset-management-frontend` container predates this branch and was stopped for the
>     e2e/mockup run (served by a fresh host build) — it needs a rebuild for normal dev use.
> 48. ~~**Import follow-ups (2026-07-12): dept/position + brand/model/unit targets + MinIO error reports
>     + office/reference e2e**~~ ✅ **DONE (this PR, branch `feat/import-followups`).** Consolidates
>     what shipped across 6 sub-tasks on this branch:
>     - **Employee dept/position lookup**: optional `departemen` (name OR code) / `jabatan` (name)
>       columns on the employee import template; `buildEmployeeLookups` loads
>       `ListDepartmentsLookup`/`ListPositionsLookup` (new `db/queries/reference_import.sql` queries),
>       `validateEmployeeRows` resolves them case-insensitively (error keys `departemen`/`jabatan`, stamps
>       `_department_id`/`_position_id`, dropped on invalid rows), `Execute` passes the resolved ids into
>       `CreateEmployee` instead of hardcoded `nil`. Office import stays core-columns-only (no dept/position
>       concept there) — not a gap, just out of scope.
>     - **3 new reference targets**: `internal/masterdata/reference/importer.go` gained **brands**
>       (`nama`, dupNama vs. `uq_brands_name`), **units** (`nama`+optional `simbol`, dupNama vs.
>       `uq_units_name`), **models** (`merek`+`nama`, brand resolved by name → `merek` error on miss,
>       dupNama on the composite `uq_models_brand_name`) — each on the same validate/Execute
>       anti-poisoning split as provinces/cities. Registered in `router.go`
>       (`reference.NewImporter(refSvc, "brands"|"models"|"units")`, no new permission — `reference:*`
>       already maps to `masterdata.global.manage`); `master/import.vue` target union/permission/label
>       maps and `master/reference.vue` `IMPORTABLE_RESOURCES` extended so Import appears for
>       Brand/Model/Satuan; i18n `masterdata.import.targets.{brands,models,units}` +
>       `import.wizard.cellErrors.{departemen,jabatan,merek}` (id+en); OpenAPI `target` enum (4
>       occurrences) + Import tag description updated. Room/floor targets **deferred** — genuinely not
>       implemented, no follow-up scheduled yet.
>     - **MinIO-persisted error reports**: the worker now builds an error report at validate *and*
>       execute completion and uploads it to MinIO, setting `error_report_key` (new `SetJobErrorReportKey`
>       query) instead of leaving it null. The handler streams the stored object for non-approval
>       targets; for the **asset** target (approval-gated) it **always rebuilds on-demand** instead —
>       deliberate, not a gap: an asset batch's error surface can still change between confirm and
>       later views (approval outcome, re-validation), so a cached MinIO snapshot could go stale.
>     - **e2e for office/reference (+brand/model) import**: `frontend/e2e/import-masterdata.spec.ts`
>       covers office + provinces + cities + brands + models against the real backend, 5/5 green.
>       Deviation: the office case asserts creation via the API (not a UI list-search) because
>       `pages/master/offices.vue` lists with a hard `limit:100` and no server-side search, so a newly
>       imported office isn't reliably visible in the UI list within e2e's seeded dataset — tracked in
>       the standing tech-debt list below, not re-solved here.
>     Tests: 3 new backend integration subtests (`TestImport_Reference{Brand,Unit}Cycle_DuplicateNameMarkedFailed`,
>     `TestImport_ReferenceModelCycle_UnknownBrandAndDuplicatePair`), 1 new employee integration test
>     (`TestImport_EmployeeCycle_DeptPositionResolved`), unit tests (`importer_test.go`, 3 new cases),
>     frontend `master-import.spec.ts` (+3 cases), Playwright `import-masterdata.spec.ts` (5/5 green).
>     **Full verification gate (this PR, 2026-07-12):** backend `go build`/`go vet`/`go test` all green;
>     full `go test -tags=integration ./... -p 1` (all packages) green; Spectral 0 errors/9 known
>     warnings; frontend lint/typecheck green, `pnpm test` 95 files/1159 tests green, `pnpm build` green.
> 49. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-12): tech-debt sweep —
>     item (c) subset: field-permission enforcement + enriched audit response + async searchable pickers.**
>     Design: `docs/superpowers/specs/2026-07-12-tech-debt-sweep-design.md` + plan
>     `docs/superpowers/plans/2026-07-12-tech-debt-sweep.md`. See item 50.
> 50. ~~**Tech-debt sweep (field-perm enforcement + audit enrichment + async pickers)**~~ ✅ **DONE
>     (2026-07-13, branch `feat/tech-debt-sweep`, 19 commits, base 3b0a483).** Three independent parts,
>     subagent-driven (13 tasks + final gate + whole-branch review, all task-reviewed):
>     - **Part 3 — async searchable pickers (item 49(c) "async pickers"):** new resource-agnostic
>       `AsyncSearchPicker.vue` (hand-rolled `UInput`+`<ul>` dropdown — deliberately NOT `USelectMenu`, to
>       dodge the focus-trap; 300ms debounce, seq-guard, `clearable`) replaces EVERY `{limit:100}`
>       client-side office/employee/reference/category picker across forms **and** filters
>       (AssetForm category/brand/model/unit/vendor, employees, assignment, transfers, disposals,
>       stock-opname, users, reference, maintenance, + the assets/reports/depreciation/dashboard office
>       filters). `AssetSearchPicker` refactored to wrap it; `usePickerSource.ts` adapters
>       (office/employee/reference/category/user) + `useResolveCache`. `master/employees` table → real
>       server-side pagination; **`master/offices` table deferred** (its flat list feeds the office tree —
>       needs a `GET /offices/tree` endpoint, out of frontend-only scope). e2e now drive pickers via a
>       `pickAsync` helper (assignment recipient UI-driven). **No backend change** — the server already
>       supported `search` for all these resources.
>     - **Part 2 — enriched audit response (item 49(c) "enriched audit response"):** `ListAuditLogs`
>       gained `LEFT JOIN identity.roles` (actor role) + `LEFT JOIN masterdata.offices` (office name),
>       resolution kept **inside the SQL** so an `audit.view`-only viewer isn't blocked by
>       `user.manage`/masterdata perms. Frontend adds Role (sub-line under Aktor) + Office columns and a
>       **client-side derived localized summary** (per `Audit Trail.dc.html`), plus an **actor filter**
>       (`AsyncSearchPicker` on users) **gated on `user.manage`** (the picker hits `/users`, which the
>       other `audit.view` roles can't). Actor role is the *current* role (not snapshotted).
>     - **Part 1 — field-permission enforcement (item 49(c) "field-permission enforcement beyond
>       assets+users"):** new fail-closed `authz.FieldService.FilterEntity` helper standardized
>       user/asset/approval masking (**`user` changed fail-open → fail-closed**); **`employees` now
>       field-masked** (map DTO + `fieldSvc` threaded through `masterdata.RegisterRoutes`, entity key
>       `"employees"`); **depreciation impairment leak closed** (`book_value`/`accumulated_depreciation`
>       coupled masking, mirroring the schedule path; attachment/document maps verified NOT leaks —
>       metadata only); frontend `fieldCatalog` gains the `employees` entity. A post-review
>       `fix(security)` made all four masking wrappers fail-closed on an unparseable `CtxRoleID` too.
>     - **Approved deviations (catat-deviasi):** **(a)** audit summary derived **client-side** (i18n) not
>       a stored column — no migration, works for existing rows; **(b)** audit Role/Office rendered per
>       the mockup's actual layout (role sub-line + "Kantor / IP" combined col), not two literal new
>       columns; **(c)** actor role is current-role (JOIN), not snapshotted; **(d)** `master/offices`
>       table left on `limit:100` (deferred, tree structural blocker); **(e)** the users employee picker
>       lost client-side office-narrowing (backend has no `office_id` filter param — safety-net clears the
>       employee on office change; server-side data-scope intact).
>     - **Gate:** backend build/vet/test + full `-tags=integration ./... -p 1` (all pkgs) + Spectral
>       0err/9warn; frontend lint/typecheck/**test 1260**/build; **e2e 29/0** vs real backend. Whole-branch
>       review (opus): ready to merge. **Honest follow-ups (open):** `master/offices` tree >100 (needs
>       `GET /offices/tree`); `AsyncSearchPicker` a11y (aria/keyboard nav); `transfers`/`disposals` office
>       maps + `assets/index` brand/model filters still `limit:100` (same office-tree class);
>       `useReference.get()`; `assets/index` resetFilters double-fetch (pre-existing); `user`
>       create/update/delete handler tests; audit summary echoes raw `entity_type`/`entity_id`.
> 51. **Next session — pick the next real step.** Remaining item 49 candidates: **(a) notifications**
>     (`mock/notifications.ts` — the last app-shell mock; needs a backend feed) — biggest remaining "real
>     feature" gap; **(b) room/floor import targets**; **(c)** the rest of the standing tech-debt list
>     (Users screen server-side filters + reset-password; approval/route badge counts; failure-safe
>     Data-Scope e2e cleanup; + the sweep's own follow-ups above — notably a `GET /offices/tree` endpoint
>     to unblock `master/offices` pagination and the remaining office-map `limit:100` truncations);
>     **(d)** the **Analytics/OLAP** read layer. Confirm priority before starting.

## ✅ Done

### Foundation & DevOps
- [x] Project scaffold (Go/Gin backend, Nuxt 4 frontend)
- [x] `docker compose up` full stack (Postgres + Redis + MinIO + migrate + backend + frontend)
- [x] GitHub Actions CI (backend build/vet/test · frontend lint/typecheck/build · Spectral)
- [x] PRD + DATABASE design docs
- [x] Production Docker Compose stack + VPS deployment guide (`docker-compose.prod.yml`,
      `docs/DEPLOYMENT.md`) — PR #51
- [x] **Ops hardening Phase 1 — WAF (Coraza + OWASP CRS)** ✅ custom Caddy image
      (`ops/caddy/`, `xcaddy --with coraza-caddy/v2`), isolated smoke-test harness
      (`ops/caddy/test/`, `ops/waf-smoketest.sh`, host port 18080) proving CRS blocks
      SQLi/XSS/path-traversal while legit traffic passes, prod Caddyfile wired
      DetectionOnly → tuned → **Blocking** (`SecRuleEngine On`, default produksi).
      ADR-0012 + `docs/DEPLOYMENT.md` WAF sub-section. **Done (2026-07-06).**
- [x] **Ops hardening Phase 2 — IaC (Ansible)** ✅ `ops/ansible/` playbook: `base`
      (users/ufw/swap/hardening), `docker` (Engine + Compose plugin), `app`
      (`.env.prod` template + `docker compose up --build`, WAF included via the
      `app`-role compose stack — no separate WAF role) roles; idempotent
      (`changed=0` on second run); tooling containerized (`ops/ansible/tools/`,
      host needs only Docker); secrets via Ansible Vault (`*.example` files
      committed, real `inventory.ini`/`vault.yml` gitignored); `ops/ansible/lint.sh`
      (`--syntax-check` + `ansible-lint`) green in CI-less dev. ADR-0013 +
      `docs/DEPLOYMENT.md` §15 IaC sub-section. **Done (2026-07-06).**
- [x] **Ops hardening Phase 3 — Monitoring/observability** ✅ self-hosted stack as
      a toggleable compose overlay (`docker-compose.monitoring.yml`): backend RED
      metrics (`/metrics`, internal-only), Prometheus (15d retention + `mem_limit`)
      + exporters (node/cAdvisor/postgres/redis/blackbox), Alertmanager → Telegram,
      Loki+Promtail (log), Grafana (datasource+dashboard as-code) — only Grafana
      public (own subdomain); secrets via `*.example` + gitignore
      (`alertmanager.yml`, `grafana.env`). Ansible `monitoring` role
      (`ops/ansible/roles/monitoring/`) brings the overlay up idempotently via
      `community.docker.docker_compose_v2`, appended after `app` in `site.yml`.
      ADR-0011 + `docs/DEPLOYMENT.md` §16. **Done (2026-07-06). Ops-hardening
      trilogy (WAF → IaC → Monitoring) now COMPLETE.**

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
- [x] **Dual-basis depreciation** — commercial (PSAK 16) + fiscal (PMK 72/2023, kelompok 1–4 / bangunan)
      `depreciation_entries` per basis; intangible amortization (PSAK 19); impairment (PSAK 48)
      write-down. `internal/depreciation` module (engine + service + 8 endpoints, migration `000023`) +
      `/depreciation` frontend screen + asset-detail tab + disposal integration. **Done — see item 28
      (2026-07-05).**
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
      **Done — (2026-07-02).** Follow-up additions for the frontend (migration `000022`, reject-receive,
      enrichment, threshold preview) and the **Frontend Mutasi screen** (`/transfers`, 1:1 against
      `docs/design/Mutasi Aset.dc.html`, deviations (a)–(i)) are **done — see item 26** in *Next session*
      above.
- [x] **Stock opname** — sessions + item reconciliation (found/not_found/damaged/misplaced) + report.
      `internal/stockopname` backend (migration `000025`, ADR-0008 4-file split, 11 endpoints, scoped
      read+write, Berita Acara PDF/Excel) + `/stock-opname` frontend screen + real-backend e2e. **Done —
      see item 34 below** (PR pending — branch `feat/stock-opname` not yet merged).
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
      depreciation module. The **Frontend Disposal screen** (`/disposals`, 1:1 against
      `docs/design/Penghapusan Aset.dc.html`, deviations (a)–(i)) is **done — see item 26** in
      *Next session* above.
- [x] **Asset documents (BAST)** — metadata CRUD + optional MinIO file; scope-gated + audited; integration tests (10 cases). **Done — (2026-06-28).**
- [x] **Journal-ready export (depreciation)** — GL-account rollup of the period's depreciation expense
      (per-category `gl_account_code` debit rows + one accumulated-depreciation credit row, balanced by
      construction) with xlsx (`excelize`) + PDF export. **Done — see item 28 (2026-07-05).**
      ⚠️ **Remaining:** a disposal **gain/loss** GL-account rollup is a separate, not-yet-built export —
      `disposals.gain_loss` exists per-row but there is no journal-recap endpoint/screen for it yet
      (candidate for the Reporting phase or a small disposal follow-up).
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
- [x] **Assignment** — `internal/assignment` (service/dto/handler/routes + executor, ADR-0008 split) on
      migration `000011_assignment` (one-active-per-asset partial-unique index) + seed `000026`
      (`assignment.view`, `assignments` data-scope, single office-level `assignment` approval band).
      Direct Manager check-out/check-in (`POST /assignments`, `/assignments/:id/checkin` — gated
      `assignment.manage`, atomic asset `available ↔ assigned`/`under_maintenance`) + Staf peminjaman
      (`POST /assignments/borrow` → assignment-type approval request; executor checks out on approval);
      reads `GET /assignments`(+`/available`,`/:id`) & `GET /assets/:id/assignments`; scope enforced read
      **and** write; OpenAPI documented; integration + unit tests green. **Done — (2026-07-08, branch
      `feat/assignment-module`; see item 36 in *Next session* for frontend screens + e2e + deviations).**
- [x] **Maintenance** — `internal/maintenance` (service/dto/executor/handler/routes) on migration
      `000027_maintenance_module` (seeds `maintenance.view` + `maintenance` data-scope + a single
      office-level `maintenance` approval band). 11 endpoints: schedule CRUD (interval/`next_due_date`),
      record CRUD (preventive/corrective, cost, vendor, status state machine, atomic asset
      `available/assigned ↔ under_maintenance`), the "Perlu Tindak Lanjut" attention queue, the Staf
      damage-report submit (maker-checker, duplicate-guarded) whose executor creates the corrective
      record on approval, and per-asset history. Stock-opname "damaged" followup creates a corrective
      record directly (no approval), idempotent via `followup_record_id`. Scope enforced read **and**
      write; OpenAPI documented; integration tests green. Frontend: **`/maintenance`** (Jadwal/Catatan/
      Laporan Kerusakan tabs) + Detail-Aset "Riwayat Maintenance" tab; real-backend e2e. **Done —
      (2026-07-11, branch `feat/maintenance-module`; see item 38 in *Next session* for the full
      deviation list, honest limitations, and the `assignment.view`-for-Staf bug found + fixed.)**
- [ ] **Depreciation** — book value (straight-line / declining-balance); monthly `depreciation_entries` read model
- [x] **Reporting & Dashboard** — `internal/report` (migration `000029`; 4 endpoints; dashboard summary Redis-cached 90s, reports/exports always fresh); **7 report types** incl. the disposal gain/loss **GL recap** (closes the old disposal deferral); `excluded_from_valuation` counted-but-not-valued; xlsx/pdf export via the `columnsFor` seam; both frontend pages wired (`mock/dashboard.ts`+`mock/reports.ts` deleted); `PeriodFilter` = first `UCalendar`-range component. **Done (2026-07-12, branch `feat/reporting-dashboard`) — see items 42/43 in *Next session*** for the full deviation list (a)–(j) + honest limitations. Aggregates still run **directly over the OLTP tables** — the pre-aggregated OLAP read layer stays deferred (see *Analytics / OLAP* below).
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

- [x] **Frontend — command palette** — overlay (⌘K/topbar), grouped results, keyboard nav, recent
      searches, empty/loading states (built in the foundation phase, mock-backed); **wired to the real
      `GET /search` with a 250 ms debounce** — item 40. **Done (2026-07-11).**
- [x] **Backend `GET /search?q=`** — `internal/search`: per-entity fan-out (assets/employees/offices/
      users/requests), **scope-filtered** (per-module `CallerOfficeScope`) + permission-gated (groups
      silently omitted), 5 items + total per group. `types=` param dropped (UI has no per-type filter);
      field-permission filtering not needed — items expose only non-sensitive columns (name/code/tag/
      email). **Done (2026-07-11) — item 40.**
- [x] **Indexing / scale (phase 1)** — `pg_trgm` GIN indexes (migration `000028`) chosen over
      `tsvector` (substring match on short names/codes, not document search). Deferred until volume/
      ranking demand it: `unaccent`, cross-entity ranking, dedicated engine (Meilisearch/Typesense/
      Elasticsearch via scheduler/CDC — shares the indexing story with *Analytics / OLAP* above).

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
- [ ] **Scheduler** — automated triggers for periodic jobs (monthly depreciation compute/close,
      maintenance-due reminders). Superseded by **ADR-0010** (staged adoption, 2026-07-05): stage 1
      (manual HTTP/UI trigger — "Hitung Periode"/"Tutup Periode", idempotent + `pg_advisory_xact_lock`
      + audit-logged) is **done** as part of the depreciation module (item 28); this checklist item now
      tracks **stage 2** (`cmd/jobs` binary + external scheduler — Task Scheduler/cron/K8s CronJob — +
      a `job_runs` table) and **stage 3** (in-process advisory-locked scheduler or a Redis job queue for
      multi-replica scale), neither built yet. Period **close** stays manual by product decision
      (accounting discipline, not a technical gap) regardless of stage.
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
        `assets`+`users` (English field keys); UUID `id` identity; default-allow; save preserves other-entity rows + only PUTs changed roles; e2e spec added against real seeded backend; orphaned `mock/fieldPermission.ts` deleted. **Done (2026-06-28).** ⚠️ TODO: `FilterView` enforcement now also covers `requests` (see the Pengajuan & Approval entry below); remaining: `employees` + other masterdata entities.
  - [x] **Audit Trail** (`/settings/audit`) ✅ wired to `GET /api/v1/audit` — server-side filter + pagination (limit/offset); gate `audit.view`; entity-type filter from frontend `AUDIT_ENTITY_TYPES` catalog; expandable diff viewer; e2e spec against real backend; orphaned `mock/audit.ts` deleted. **Done (2026-06-29).** ⚠️ TODO: actor filter + role/summary/office-name columns dropped — backend response has no role/summary; resolving actor/office names requires `user.manage`/masterdata reads that an `audit.view`-only viewer may lack. Revisit if a viewer-accessible name lookup or enriched audit response lands.
  - [x] **User Management** (`/settings/users`) ✅ wired to `/api/v1/users` — CRUD (GET list with server-side search+pagination, POST create, PUT update, DELETE remove); gate `user.manage`; role/office/employee pickers from real API lookups; employee picker filtered by selected office (office_id-aware `employeeFormOptions`); e2e spec against real seeded backend; status toggled via update endpoint. **Done (2026-06-29). Authz/settings screen wiring batch complete (RBAC + Data Scope + Field Permission + Audit Trail + User Management).** Filter bar now has server-side role/office/status filter controls (role `USelect`, office `AsyncSearchPicker`, status `USelect`, reset button matching the mockup) driving `GET /users?role_id&office_id&status`; `useUsers().list()` extended; 12-case component spec (`users-filters.spec.ts`); verified live against the real backend. **Done (2026-07-13, Tech-Debt Sweep #2 Task 7).** ⚠️ TODO: reset-password action still dropped pending backend support; office/employee lookup capped at 100 (searchable async picker is a follow-up if counts grow); `mock/users.ts` retained until `useGlobalSearch` is wired to the real `/search` endpoint; no dedicated e2e assertion added yet for the new filter controls (component-test only).
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
      ✅ **Partly resolved (2026-07-12, item 42):** the mock Laporan screen is gone — `pages/reports.vue`
      is wired to the real `/reports` API and `mock/reports.ts` is **deleted**, so the old-key coupling no
      longer blocks a wiring. **Remaining standalone cleanup:** tighten `AssetStatusBadge`'s prop from
      `AssetStatus | string` to `AssetStatus` and drop the badge's `?? assets.status.${status}`
      legacy-status fallback; and extract a shared `moneyCell`/rupiah formatter util (now duplicated across
      Katalog/Detail/AssetForm/reports — the reports page reuses `formatMoneyShort` from `reportMeta`).
      🐛 **Bug fixed during verification:** `pages/assets/[tag].vue` + the `pages/assets/[tag]/` folder
      made `[tag].vue` an unintended parent route for `[tag]/edit.vue` (no `<NuxtPage/>` to render the
      child), so `/assets/:tag/edit` silently showed the Detail page. Fixed by moving `[tag].vue` →
      `[tag]/index.vue` (siblings).
- [x] **Pengajuan & Approval** (`/approval`) ✅ wired to real `/api/v1/requests` — inbox
      (`GET /requests/inbox`, scoped to the caller's pending steps) + list-by-status
      (`GET /requests` with a `status` filter) for the other four tabs; detail fetch resolves the
      full payload + approval-step timeline; approve/reject via `POST /requests/:id/approve|reject`;
      a pending request outside the caller's inbox (SoD/scope-ineligible) renders a disabled
      "not eligible" lock instead of decide buttons. Category/office FK names resolved via
      `useCategories().tree()` + `useOffices().list()` (best-effort, falls back to raw ids).
      Backend: `internal/approval` responses enriched with maker/office names + the full payload
      (`enrichRequestMap`), and **field-permission `FilterView` now covers the `requests` entity**
      (first entity beyond `assets`/`users`). `nav.ts`'s `approval` item: removed the mock-era
      hardcoded `badgeCount: 8`, gated on `permission: 'request.decide'`. Component test suite
      rewritten (`test/nuxt/approval.spec.ts`, 14 cases). `mock/approval.ts` **retained** — still
      imported by `useGlobalSearch.ts` for the mock global-search result list (drop when global
      search wires to a real `/search` endpoint, item (f) in *Next session*). **Done (2026-07-04).**
      See item 24/25 in *Next session* above for the full deviation list (a)–(d) and the dev-stack
      issues found & fixed during this task's e2e verification (stale container source; corrupted
      Superadmin data-scope from a flaky settings e2e; RATELIMIT_ENABLED=false for local e2e).
      Full local e2e re-verified green (61/61) after the fixes.
- [x] **Mutasi Aset** (`/transfers`) + **Penghapusan Aset** (`/disposals`) ✅ wired to real
      `/api/v1/transfers` + `/api/v1/disposals` (+ `/api/v1/requests` for submit/approve and the new
      `/api/v1/approval-thresholds/preview` for the pre-submit chain preview). Mutasi: Ajukan/Kotak
      Masuk/Riwayat tabs, asset picker restricted to available assets, inter-office/inter-region
      banner, ship + receive (BAST no.) + reject-receive actions, merged request+transfer history.
      Disposal: Ajukan/Riwayat tabs, valuation summary + laba/rugi card, approval-chain preview card,
      post-submit timeline, Lampirkan BAST on completed rows. Shared `AssetSearchPicker` +
      `transferHistory`/`disposalHistory`/`officeRegion` utilities; caller `office_id` added to auth
      state. Backend companions on this branch: migration `000022` (condition/transfer_date/return),
      `POST /transfers/:id/reject-receive`, enriched transfer+disposal reads, threshold preview
      endpoint. 2 new real-backend e2e specs (8 tests). **Done (2026-07-05).** See item 26 in
      *Next session* above for the full deviation list (a)–(i) and follow-ups (disposal amount basis
      → server-computed book value; BAST link once Dokumen BAST exists; money fields into the
      field-permission catalog if needed).
- [x] **Depresiasi** (`/depreciation`) ✅ wired to real `/api/v1/depreciation/*` +
      `/api/v1/assets/:id/depreciation` + `/api/v1/assets/:id/impairment` — basis toggle (Komersial
      PSAK 16 / Fiskal PMK 72/2023), 4 KPI tiles, Jalankan-Periode panel (open/computed/closed +
      reminder banner), Jadwal-per-Aset tab (impaired icon, fully-depreciated rows, filters), Rekap
      Siap-Jurnal tab (balanced banner + xlsx/pdf export), impairment modal (live loss preview); asset
      detail's Depreciation tab now shows a real schedule instead of an empty state; Disposal screen's
      fiscal valuation + approval-chain subtitle are real. **Done (2026-07-05).** See item 28 in
      *Next session* above for the full deviation list (a)–(e), limitations, and follow-ups.
- [x] **Stock Opname** (`/stock-opname`) ✅ wired to real `/api/v1/stockopname/*` — list (empty/loading/
      error+retry/populated with per-session progress) + detail toggle covering all 4 session states
      (`open`/`counting`/`reconciling`/`closed`), scan bar + manual code entry, segmented per-item
      result buttons while counting (read-only badges once reconciling/closed), variance panel with
      follow-up buttons (`not_found` → disposal, `misplaced` → transfer, `damaged` disabled/coming-soon),
      create/finish (Berita Acara PDF/Excel preview) modals. `useStockOpname` composable;
      `stockOpnameMeta` constants; `SessionCard`/`StockopnameCreateSessionModal`/
      `StockopnameFollowupModal` components. Real-backend e2e (`frontend/e2e/stock-opname.spec.ts`,
      1/1). **Done (2026-07-07).** See item 34 in *Next session* above for the full deviation list
      (a)–(d) and follow-ups (no submitted-state indicator on the follow-up button; e2e session
      creation goes via API due to the office-picker `limit:100` cap).
- [x] **Assignment (Penugasan/Peminjaman)** (`/assignment` + `/peminjaman` + Detail-Aset borrow) ✅ wired
      to real `/api/v1/assignments/*` + `/api/v1/requests?type=assignment`. **`/assignment`** (Manager,
      gated `assignment.manage`) — Check-out / Check-in / Riwayat tabs (active-count badge, colored
      condition column, load-error/retry), 1:1 against `docs/design/Penugasan Aset.dc.html`;
      check-out/check-in atomically flip the asset (`available ↔ Digunakan`/`Maintenance`).
      **`/peminjaman`** (Staf, gated `request.create`) — inline Ajukan Peminjaman form + "Pengajuan Saya"
      list (expandable approval timeline + cancel); asset picker from `GET /assignments/available`
      (own-office scoped). **Detail-Aset "Ajukan Peminjaman"** button + `AssignmentAjukanPeminjamanModal`
      (locked-asset variant), shown for all `request.create` roles, disabled (native `title` hint) when
      the asset is not available. `useAssignment` composable; `assignmentMeta` constants; full i18n id/en.
      963 unit/component tests green. Real-backend e2e (`frontend/e2e/assignment.spec.ts`): direct Manager
      check-out → Aktif + Detail "Digunakan" + borrow disabled → check-in → Dikembalikan; Staf peminjaman
      submit → Menunggu → approve via API (maker ≠ checker, office-level) → Disetujui + assignment created;
      empty-Alasan negative. **Done (2026-07-08, branch `feat/assignment-module`).** See item 36 in
      *Next session* for the full deviation list (a)–(g), honest limitations (frontend `RequestType` union
      lacks `'assignment'` — local test cast, follow-up; no real nav badge count), and the note that the
      e2e is written + committed but not run locally (stale dev-DB missing the `assignment.manage` seed
      grant — CI's fresh-DB e2e covers it).
- [x] **Maintenance** (`/maintenance` + Detail-Aset "Riwayat Maintenance" tab) ✅ wired to real
      `/api/v1/maintenance/*` + `/api/v1/requests?type=maintenance`, 1:1 against
      `docs/design/Maintenance.dc.html` (due banner, Jadwal/Catatan/Laporan Kerusakan tabs, light + dark
      verified). Jadwal — schedule cards with due badges + "Buat Catatan" quick-create; Catatan — 7-column
      table (Aset/Tipe/Kategori/Tanggal/Status/Biaya/Vendor), row-click edit; Laporan Kerusakan (Staf) —
      asset + kategori masalah picker, photo upload, "Riwayat Laporan Saya". `MaintenanceScheduleSlideover`
      + `MaintenanceRecordSlideover`; `useMaintenance` composable; `maintenanceMeta` constants; full i18n
      id/en. 1021 unit/component tests green. Real-backend e2e (`frontend/e2e/maintenance.spec.ts`):
      Manager schedule→"Buat Catatan"→`in_progress`→`completed` (+biaya, next-due shift, asset
      Maintenance→Tersedia); Staf report→approve (maker ≠ checker)→corrective record; negative
      empty-kategori + read-only-completed-record. **Done (2026-07-11, branch
      `feat/maintenance-module`).** See item 38 in *Next session* for the full deviation list (a)–(h),
      honest limitations, and a real permission bug found + fixed (`assignment.view` missing for Staf,
      blocking the Laporan asset picker — migration `000028` + an `employee_id`-scoped frontend fix). The
      e2e's Staf→approve scenario self-skips locally (this shared dev DB predates migration `000028`);
      the other two scenarios pass locally; CI's fresh DB runs all three.
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
