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
> `000015_fam_tables` (see *Database* below and DATABASE.md bagian 6). Architecture decisions from the pivot
> are recorded as ADRs in [adr/](adr/) (**ADR-0001–0009**: testing, logging, config, rate-limit, authz
> build-vs-buy, map, frontend API convention, masterdata split, third-party sign-in). What's already ✅
> predates the pivot and remains valid — the office hierarchy + 3-layer authorization are the foundation
> the bank scope builds on.

> ## ▶ Next session — start here
> **AKTIF (2026-07-23): Legacy-parity data model** — penyelarasan model data agar Inventra
> menggantikan sistem lama (Vue2/Yii2/PG12) sebagai sole asset system anak-perusahaan bank. Spec:
> `docs/superpowers/specs/2026-07-23-legacy-parity-data-model-design.md` (disetujui user). Branch
> `feat/legacy-parity-data-model` (worktree). 8 fase (migrasi `000038`–`000045`).
> - **Fase 1 (kolom aset + constraint lokasi) — kode selesai.** Migrasi `000038` (enum office/lokasi) +
>   `000039` (kolom `capacity`/`lease_date`/`installation_date`/`warranty_start`/`floor_id`/
>   `pic_employee_id`, constraint tangible = floor ATAU room); sqlc + DTO + executor + `assetToMap` +
>   OpenAPI; form Aset (kapasitas, PIC picker, 3 tanggal, lokasi boleh berhenti di lantai) + i18n
>   id/en + test komponen. **Gate backend hijau** (build/vet/test/spectral). **Gate frontend: lint
>   hijau; typecheck/vitest/build tertunda ke CI** (blocker environmental Node v24 + vite8/babel8 —
>   `nuxt prepare` gagal lokal, analog integration-test CI-only). Deviasi mockup disetujui: field baru
>   (kapasitas/PIC/tanggal instalasi-sewa-garansi mulai) di luar `Form Aset.dc.html` — permintaan user.
> - **Fase 2 (penomoran kode aset baru) — kode selesai.** Migrasi `000040`: kolom `assets.tag_seq`
>   (NULLABLE), backfill + re-tag semua aset ke format `{KANTOR}{KATEGORI}{TAHUN}{NNNNN}` tanpa dash,
>   hapus `asset_tag_counters`. `formatAssetTag` tanpa pemisah; `GenerateAssetTag`/`NextTagSeq` pakai
>   `MAX(tag_seq)+1` + `pg_advisory_xact_lock` per-kantor (sequence per-KANTOR, tak reset per tahun,
>   tak dipakai ulang kecuali hard-delete teratas). Executor + importer + 2 integration test tag
>   ditulis ulang; DATABASE.md bagian 4.7 + DB-Q5 diperbarui. **Gate:** backend build/vet/unit (31 pkg)
>   hijau + `go vet -tags=integration` compile-check lolos (run integrasi via CI). Frontend tak berubah
>   (hanya menampilkan string tag). **Sisa doc-sweep:** referensi `asset_tag_counters` di daftar
>   skema/indeks/migrasi DATABASE.md (baris ~62/725/807) belum dibersihkan — kerjakan di sweep akhir.
> - **Fase 3 (history aset) — kode selesai.** Migrasi `000041`: tabel `asset_location_history` +
>   `asset_pic_history` (partial-unique PIC aktif) + backfill lokasi awal & PIC. Penulis history:
>   create executor (registration + PIC), `Update` (tx + actor; lokasi saat floor/room berubah, PIC
>   saat berubah — close aktif + buka baru), transfer `Receive` (source=transfer, dari state aktual
>   pasca-relokasi). Read: `GET /assets/:id/location-history` + `/pic-history` (scope-gated),
>   serializer + service list, OpenAPI (2 path + 2 skema). Frontend: tab **Riwayat Lokasi** & **Riwayat
>   PIC** di Detail Aset (lazy-load), composable `useAssets.locationHistory/picHistory`, i18n id/en,
>   test komponen. **Gate:** backend build/vet/unit + serializer unit test + Spectral 0-error +
>   `go vet -tags=integration` compile-check (incl. integration test baru Update-history) hijau;
>   frontend lint + JSON hijau (typecheck/vitest/build CI). **Pemegang** = tab `assign` existing
>   (masih placeholder — wiring assignment-history di luar scope Fase 3).
> - **Review fix (code-review 5-sumbu) — SELESAI.** #1 (Important): `SetAssetOffice` kini menurunkan
>   `floor_id` dari lantai room tujuan (NULL bila tanpa room) — aset tak lagi menyimpan floor kantor
>   asal pasca-mutasi; assert floor+history transfer ditambah. #2: `resolveLocation` memvalidasi &
>   memaksa floor = lantai room (konsistensi) di create + Update. #3: `LIMIT 200` pada query history.
>   #5: `formatDateTime` ikut locale (id/en). #4 (respons `{data}`) sengaja dipertahankan (sub-resource
>   read-only tak berpaginasi; LIMIT jadi pengaman) — bukan memalsukan pagination.
> - **Fase 4 (master baru) — SELESAI.** Migrasi `000042`: `office_classes`, `executor_divisions`
>   (+seed 5), `companies` (semua datar → generic reference engine deklaratif), dan
>   `building_classifications` (numerik min/max lantai + check-constraint). Engine diberi tipe kolom
>   **`typeInt`** (bukan sub-package) sehingga keempatnya deklaratif; route auto-mount. Frontend:
>   4 resource + tipe field **`number`** (NumberInput) di layar Referensi + i18n id/en. **Gate:**
>   backend build/vet + Spectral 0-error + unit (`TestCoerceInt`) + **integrasi Docker** (reference
>   pkg + `TestBuildingClassificationRoundTrip`: int round-trip, max nullable, check-constraint,
>   required) hijau; frontend eslint + JSON + unit config-test hijau (vitest/build via CI).
> - **Fase 5 (kolom kantor) — SELESAI.** Migrasi `000043`: 9 kolom (ownership_status enum,
>   office_class_id, building_classification_id, floor_count, building_area, office_kind enum
>   default konvensional, description, head_employee_id, contact). Query CreateOffice dikonversi ke
>   named-args + 9 kolom; UpdateOffice +9; DTO Request/Response + toInput/toResponse; service default
>   office_kind=konvensional (robust untuk zero-value CreateInput — memperbaiki test lama). Frontend:
>   9 field di form Kantor (select ownership/kind, USelect kelas/klasifikasi dari reference, NumberInput
>   floor_count/building_area, picker kepala kantor, textarea deskripsi) + **auto-saran klasifikasi
>   gedung dari floor_count** (watcher, hanya mengisi bila kosong) + i18n id/en. **Gate:** backend
>   build/vet + Spectral 0-error + **integrasi Docker** (office pkg + `TestOffice_LegacyParityFields_RoundTrip`)
>   hijau; frontend eslint + JSON hijau (typecheck/vitest/build via CI). Follow-up: test frontend form
>   Kantor (mount) belum ditambah.
> - **Fase 6 (pegawai + divisi per-kantor) — SELESAI.** Migrasi `000044`: employees +`company_id`
>   +`executor_division_id`; departments +`office_id` (NULLABLE — wajib di app layer, DB NOT NULL
>   menyusul) + unik code jadi per-kantor. Backend: CreateEmployee named-args +2; validasi
>   **department-in-office** (`GetDepartmentOffice`; departemen legacy office-null exempt) → sentinel
>   `ErrDepartmentOfficeMismatch` (400); DTO/Response/map +2; OpenAPI Employee/EmployeeRequest +2;
>   departments reference resource +`office_id` (typeUUID nullable). Frontend: form Pegawai +
>   select Perusahaan/Divisi Pelaksana (dari reference) + **department jadi USelect ter-filter kantor**
>   (client-side; watcher membersihkan department bila kantor ganti); Referensi departments + field
>   `office` (tipe field baru 'office' pakai `useOfficePicker`) + i18n id/en. **Gate:** backend
>   build/vet + Spectral 0-error + **integrasi Docker** (employee pkg + `TestEmployee_DepartmentOffice
>   ValidationAndParityFields`, reference pkg) hijau; frontend eslint + JSON hijau (typecheck/vitest/
>   build via CI). Catatan: `testsupport.Reset` menghapus seed executor_divisions — test insert sendiri.
> - **Fase 7 (login NIP-atau-email) — SELESAI.** Migrasi `000045`: kolom `identity.users.username`
>   (unik partial) + backfill dari NIP (`employees.code`). Query `GetUserByLogin` (email citext ATAU
>   username); `Login` service pakai itu (jalur lain—forgot/email-change—tetap `GetUserByEmail`);
>   loginRequest binding `email` direlaksasi jadi `required` (terima NIP), field JSON tetap `email`;
>   fake store test +`GetUserByLogin`. Frontend: field login jadi **NIP atau Email** (label/placeholder,
>   `type="text"`, autocomplete username). **PENTING—e2e:** 12 selector `input[type="email"]` (login)
>   di 9 spec + helpers.ts diganti `input[name="email"]` (name tetap) — field email asli (change-email/
>   forgot) pakai selector lain. **Gate:** backend build/vet + Spectral 0-error + **integrasi Docker**
>   (identity pkg penuh + `TestGetUserByLogin_EmailOrUsername`) hijau; frontend eslint + JSON hijau.
> - **Berikutnya: Fase 8 (TERAKHIR)** — batch registrasi aset: satu request `asset_create` +
>   `quantity` → N aset (executor buat N baris, tag berurutan via advisory lock); amount = cost*qty;
>   form Aset field "Jumlah".
>   **Doc-sweep tersisa** (kumulatif): bersihkan `asset_tag_counters` + dokumentasikan 2 tabel history
>   + 4 master + 9 kolom kantor + kolom pegawai/departemen + username di DATABASE.md.
>
> 1. ~~**Bring the dev stack up, reset & migrate**~~ ✅ **DONE (2026-06-27).**
> 2. ~~**#6 Kategori Aset screen**~~ ✅ **DONE.**
> 3. ~~**Approval engine + Asset core backend**~~ ✅ **DONE (2026-06-28).**
> 4. ~~**Asset attachments (MinIO)**~~ ✅ **DONE (2026-06-28).**
> 5. ~~**Barcode/QR + label PDF**~~ ✅ **DONE (2026-06-28).**
> 6. ~~**Asset documents (BAST)**~~ ✅ **DONE (2026-06-28).**
> 7. ~~**Authorization admin endpoints**~~ ✅ **DONE (2026-06-28).** `internal/authzadmin` — role CRUD, replace-set permissions/scope/fields, Redis cache invalidation, permission catalog, seed RBAC drift fix, integration tests, OpenAPI spec.
>    - ~~**Catalog-drift fix (permission keys enforced/seeded but not assignable)**~~ ✅ **DONE (2026-07-16, branch `fix/authz-catalog-gaps`).** `stockopname.view`/`stockopname.manage` (router-enforced, seeded 000025) and `masterdata.employee.manage` (importer-enforced, seeded 000032) were missing from `authzadmin.permissionCatalog`, so `SetRolePermissions → dedupePermissions` rejected any role edit containing them (`ErrUnknownPermission`) — Superadmin could not re-save those roles via the RBAC UI. Added a "Stock Opname" group + `masterdata.employee.manage` to the catalog, `stockopname` to `ScopeModules()`, frontend group icon + id/en i18n labels, and a `TestCatalog_CoversEveryEnforcedKey` regression test enumerating every `RequirePermission`/`PermissionKey` key so the class of gap can't recur.
> 8. ~~**Wire Peran & RBAC screen to real `/authz` APIs**~~ ✅ **DONE (2026-06-28).** `useRbac` composable rewritten to `/authz/catalog` + `/authz/roles` + `/authz/roles/:id/permissions`; English DTO; UUID `id` identity; system-role permissions now editable (product decision — lock note reworded, switches active); e2e spec updated against real seeded backend.
> 9. ~~**Wire frontend Data Scope screen** (`/settings/data-scope`) to real `/authz` APIs~~ ✅ **DONE (2026-06-28).** `useDataScope` composable rewritten to `/authz/catalog` (scope_modules, filters `*`) + `/authz/roles` + `/authz/roles/:id/scope`; English DTO; UUID `id` identity; save only changed roles (dirtyIds set); e2e spec added against real seeded backend; orphaned `mock/dataScope.ts` deleted.
> 10. ~~**Wire frontend Field Permission screen** (`/settings/field-permission`) to real `/authz` APIs~~ ✅ **DONE (2026-06-28).** `useFieldPermission` composable rewritten to `/authz/roles` + `/authz/roles/:id/fields`; catalog `assets`+`users` (English field keys); UUID `id` identity; default-allow (no stored policy = view+edit); save preserves other-entity rows + only PUTs changed roles; e2e spec added against real seeded backend; orphaned `mock/fieldPermission.ts` deleted. **Authz-screen wiring trio (RBAC + Data Scope + Field Permission) now complete.**
>    - **TODO — extend field-permission ENFORCEMENT (`FilterView`) beyond `assets`+`users`:** `requests` (approval handler already injects `fieldSvc` + has `requestToMap`; add `ForEntity`/`FilterView` calls), `employees` (needs `fieldSvc`+map wiring), and other masterdata modules. Until then the Field Permission screen configures rules but they only take effect for `assets`+`users`. Add each new entity to `frontend/app/constants/fieldCatalog.ts` once its backend enforcement lands.
> 11. ~~**Wire Audit Trail screen** (`/settings/audit`) to real `GET /api/v1/audit`~~ ✅ **DONE (2026-06-29).** `useAudit` composable rewritten to server-side list (`GET /api/v1/audit`, limit/offset/filter params); gate `audit.view`; entity-type filter from `AUDIT_ENTITY_TYPES` frontend catalog; expandable diff viewer unchanged; orphaned `mock/audit.ts` deleted; e2e spec updated against real seeded backend.
>    - **TODO — actor filter + role/summary/office-name columns are dropped:** the backend audit response has no `role` or `summary` fields, and resolving actor/office **names** needs `user.manage`/masterdata reads that an `audit.view`-only viewer may lack. Revisit if a viewer-accessible actor/office name lookup (or an enriched `GET /api/v1/audit` response) lands.
> 12. ~~**Wire User Management screen** (`/settings/users`) to real `/api/v1/users`~~ ✅ **DONE (2026-06-29).** `useUsers` composable rewritten to server-side CRUD (`GET/POST/PUT/DELETE /api/v1/users`, limit/offset/search params); gate `user.manage`; role/office/employee pickers from real API lookups; employee picker filtered by selected office; orphaned mock NOT deleted (still imported by `useGlobalSearch` — see bagian TODO below); e2e spec updated against real seeded backend. **Authz/settings screen wiring batch now complete (RBAC + Data Scope + Field Permission + Audit Trail + User Management).**
>    - **TODO — server-side role/office/status filter dropdowns** dropped pending backend filter-param support on `GET /api/v1/users`; ~~**reset-password** action dropped pending a backend reset endpoint~~ ✅ **done (item 70, 2026-07-18)** — `POST /users/:id/reset-password` emails the target a reset link. The office/employee lookup is capped at 100 entries (a searchable async picker is a follow-up if user/employee counts grow).
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
>     cluster* in bagian Remaining.
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
>     (that test mutates live RBAC config as its subject; its cleanup was not failure-safe —
>     ~~**follow-up: make that e2e revert via `afterEach`/API instead of in-test steps**~~ ✅ **DONE
>     (2026-07-13) — see item 52 Part D (tech-debt sweep #2, PR #64).** The mutating test now sits in
>     its own nested describe in `frontend/e2e/settings.spec.ts` whose `afterEach` restores the policy
>     unconditionally via `restoreDefaultScope` (`frontend/e2e/helpers.ts`) — idempotent, and it heals
>     a policy left corrupted by an earlier interrupted run. The same pattern covers the
>     field-permission `purchase_cost` test). Restored to `global` (user-approved).
>     (3) local full `pnpm test:e2e` needs `RATELIMIT_ENABLED=false`
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
>     `docs/design/Depresiasi.dc.html` in both light and dark mode; see spec bagian 6) — **(a)** fully
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
>     ADR-0013 + `docs/DEPLOYMENT.md` bagian 15 IaC sub-section. See *Foundation & DevOps* above.
> 32. ~~**Ops hardening Phase 3 — Monitoring/observability**~~ ✅ **DONE (2026-07-06).** Self-hosted
>     stack as a toggleable compose overlay (`docker-compose.monitoring.yml`): backend RED metrics
>     (`/metrics`, internal-only), Prometheus (15d retention + `mem_limit`) + exporters (node/cAdvisor/
>     postgres/redis/blackbox), Alertmanager → Telegram, Loki+Promtail (log), Grafana (datasource+
>     dashboard as-code) — only Grafana public, via its own subdomain, no WAF/no login bypass; secrets
>     via `*.example` + gitignore. Ansible `monitoring` role (`ops/ansible/roles/monitoring/`) brings the
>     overlay up idempotently, appended after `app` in `site.yml` — completes the ops-hardening trilogy.
>     ADR-0011 + `docs/DEPLOYMENT.md` bagian 16. **Ops hardening (WAF → IaC → Monitoring) is now fully
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
>     (camera scanning itself is deferred per spec bagian 9, user-approved; simulate-tap has no real backend
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
>     — a regression against PRD bagian 2.2 (Staf = data miliknya) and against 000026's own recorded decision to
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
> 51. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-13): candidate (c) — tech-debt
>     sweep #2 (see item 52).** Remaining item 49/51 candidates for the *next* session: **(a) notifications**
>     (`mock/notifications.ts` — the last app-shell mock; needs a backend feed) — biggest remaining "real
>     feature" gap; **(b) room/floor import targets**; **(d)** the **Analytics/OLAP** read layer.
> 52. ~~**Tech-debt sweep #2 (offices/tree + kill office-class `limit:100` · user list filters · live
>     approval badge · a11y/e2e/audit polish)**~~ ✅ **DONE (2026-07-13, branch `feat/tech-debt-sweep-2`,
>     20 commits, base 534a9a3).** Design: `docs/superpowers/specs/2026-07-13-tech-debt-sweep-2-design.md`
>     + plan `docs/superpowers/plans/2026-07-13-tech-debt-sweep-2.md`. Four independent parts, subagent-driven
>     (13 tasks + final gate + whole-branch review, all task-reviewed):
>     - **Part A — `GET /offices/tree` + kill office-class `limit:100`:** new unbounded scope-filtered
>       `ListOfficesTree`/`Service.Tree`/`GET /offices/tree` (byte-identical scope filter to `ListOffices`,
>       no LIMIT; 2 integration tests incl. 105-row no-cap + subtree presence/absence). Frontend
>       `useOffices.tree()` now builds the office **tree** (complete past 100) and the transfer/disposal
>       office **name-maps**; assets **Katalog + Detail** brand/model resolution moved off eager `limit:100`
>       fetches to on-demand `useResolveCache` (the `assets/index` "brand/model filter" the plan named
>       **does not exist** — no such filter control in page/backend/mockup, verified; reduced to the real
>       goal of killing the `limit:100`). `useReference.get()` added; `useReferencePicker.resolveFn`
>       switched to it (dropped the `request()` reach-around).
>     - **Part B — user list server-side filters:** `role_id`/`office_id`/`status` narg predicates on
>       `ListUsers`+`CountUsers` (in sync), 400 on malformed uuid/status; frontend filter bar (role USelect
>       reusing `/authz/roles`, office `AsyncSearchPicker`, status USelect) + status-filter e2e. **Reset-password
>       DEFERRED** (no email infra — product decision).
>     - **Part C — live approval badge:** lightweight `GET /requests/inbox/count` (shares `svc.Inbox`,
>       `request.decide`-gated, count == `/inbox` len); Pinia `stores/inbox.ts` + `AppSidebar` badge from
>       the store (hides at 0), hardcoded `nav.ts` `badgeCount:2` removed; **event-driven** refresh
>       (fetchMe choke-point + approval mount + post-decide), **no polling**.
>     - **Part D — polish:** `AsyncSearchPicker` a11y (combobox/listbox roles, ARIA on the real `<input>`,
>       Arrow/Enter/Escape/Home-End keyboard nav, Enter guarded against leaking into wrapping `UForm`
>       submit); **failure-safe Data-Scope/field-perm e2e** (`afterEach` API restore of Superadmin
>       `*`-scope→`global`, healing the recurring shared-dev-DB corruption); `assets/index` `resetFilters`
>       single-fetch; `user` create/update/delete handler tests; audit summary **localizes** the entity
>       label (was raw `entity_type`).
>     - **Approved deviation (catat-deviasi):** the plan's "migrate brand/model **filters** to
>       AsyncSearchPicker" (Part A) was a factually-wrong premise — no such filter exists (verified against
>       page/`GET /assets` params/`Katalog Aset.dc.html`, which shows Brand/Model as a **column** only);
>       reduced to killing the `limit:100` name-resolution fetch. Adding a filter would need a backend
>       `brand_id`/`model_id` param + a mockup change — not done (out of scope).
>     - **Gate:** backend build/vet/test + full `go test -tags=integration ./... -p 1` (32 pkgs, no flakes)
>       + Spectral 0err/9warn; frontend lint/typecheck/**test 1314**/build; whole-branch review (opus):
>       **ready to merge**. E2E: all local failures traced to pre-existing dev-DB debris (none touch this
>       branch's source).
>     - **Follow-ups (open, non-blocking):** `settings/users.vue` `resetFilters` fires up to 4 identical
>       `loadList()` (same double-fetch class as `assets/index`, benign); `master/offices.vue:219`
>       office-**types** select still `limit:100` (a reference picker, different from the office-tree class);
>       `useUserPicker`/`useUsers` still lack a `get()` (reach-around); disposals `officesTreeMock` has no
>       asserting test; a11y edge polish. Deferred by decision: reset-password, force-change-on-next-login,
>       badge polling.
> 53. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-13): Account Security via
>     Email (Spec A) — see item 54.** (User request: "reset password dan sesi perangkat"; scoped to
>     email-based flows first, device sessions deferred to Spec B.)
> 54. ~~**Account Security via Email (Spec A) — forgot-password + authenticated change-password + email
>     infra**~~ ✅ **DONE (2026-07-13, branch `feat/account-security-email`).** Design:
>     `docs/superpowers/specs/2026-07-13-account-security-email-design.md`; plan:
>     `docs/superpowers/plans/2026-07-13-account-security-email.md` (13 tasks, subagent-driven, each
>     task-reviewed). Backend: migration **`000033_password_reset`** (`identity.users.password_changed_at`)
>     + `UpdateUserPassword`; new `internal/email` (provider-agnostic SMTP via `github.com/wneessen/go-mail`
>     + `LogSender` fallback + embedded Indonesian templates + `Mailer`); Redis single-use hashed
>     reset-token store (`auth/pwreset.go`, 32-byte→base64url raw, SHA-256-at-rest, 30-min TTL, GETDEL);
>     identity service `RequestPasswordReset`/`ResetPassword`/`ChangePassword` + a **token-epoch** check in
>     `Refresh` (rejects any refresh token issued before `password_changed_at`); 3 endpoints
>     (`POST /auth/password/forgot` always-200 anti-enumeration + rate-limited, `POST /auth/password/reset`,
>     `PUT /auth/password` authed) with audit (`ActionUpdate` + `{"event":...}`, no password material) +
>     OpenAPI. Frontend: `/forgot-password` + `/reset-password` pages (public routes), login "Lupa password?"
>     wired, `useAccount` real `changePassword`/`requestPasswordReset`/`resetPassword`, account "Ganti
>     Password" card now logs out + redirects to `/login`. Dev/CI **Mailpit** mail-catcher + real-backend
>     e2e (`password-reset.spec.ts`, Mailpit HTTP API, failure-safe admin-password restore).
>     **Approved deviations / decisions:** **(a)** change-password revokes **ALL** sessions incl. the
>     current device (user decision — no email click-gate on #3; instead verify-old-password + a
>     notification email, per industry standard); **(b)** migration numbered **000033** (000028 was
>     already taken); **(c)** forgot always returns 200 and Google-only/inactive accounts silently no-op
>     (anti-enumeration); **(d)** email transport is provider-agnostic SMTP + `LogSender` when
>     `MAIL_ENABLED=false`, Mailpit for dev/CI; **(e)** the full email-flow e2e is validated in **CI**
>     (Mailpit wired into the e2e job), the invalid-token path passes locally — the shared local :8080
>     backend was stale so the full local run was CI-deferred (repo convention).
>     **Follow-ups:** ~~device-session list/revoke/logout-others is **Spec B**~~ ✅ **DONE — see item 62
>     (2026-07-15).** Still open (non-blocking): admin-initiated reset + force-change-on-next-login not
>     built; per-user email localization (Indonesian default used).
> 55. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-13): UX fixes batch — see item 56.**
>     (User request: 7 UX/correctness fixes bundled — number input, profile/email, security, forgot-resend,
>     search focus, map z-index, PDF/CSV mojibake.)
> 56. ~~**UX Fixes Batch (7 fixes)**~~ ✅ **DONE (2026-07-14, branch `feat/ux-fixes-batch`).** Design:
>     `docs/superpowers/specs/2026-07-13-ux-fixes-batch-design.md`; plan:
>     `docs/superpowers/plans/2026-07-13-ux-fixes-batch.md` (21 tasks, subagent-driven, each task-reviewed).
>     **(1) NumberInput** — reusable `frontend/app/components/NumberInput.vue` (numeric-only keystrokes,
>     `allowNegative`/`thousandSeparator`/`decimals`/`money`-Rp props; id-ID comma decimal when grouping)
>     rolled out to all 8 number fields (asset harga, category life/salvage/fiscal/threshold, maintenance
>     cost+interval, disposals proceeds, depreciation impairment, offices lat/lng); 3 duplicate format
>     patterns removed. **(2) Profil tab** — view↔edit state, wired to new backend (`useAccount` un-mocked:
>     `GET/PUT /auth/profile`), telepon → `masterdata.employees.phone` (disabled when no employee), **email
>     change with verification link to the new address** (`POST /auth/email/change-request` + password →
>     `POST /auth/email/confirm` public + `pages/verify-email.vue`). **(3) Keamanan tab** — inline password
>     fields removed; "Ganti Password" modal verifies old password → emails a reset link
>     (`POST /auth/password/change-request`, reuses the reset infra). **(4) Forgot password** — full-width
>     input + `useResendCooldown` (exponential 30/60/120s), resend errors shown, cooldown only advances on
>     success. **(5) Global search** — programmatic `.focus()` on open (replaces flaky `autofocus`).
>     **(6) Office map** — detail card `z-[1100]` + controls `z-[1000]` above Leaflet panes. **(7) Mojibake**
>     — PDFs now embed **DejaVuSansCondensed** UTF-8 font (`internal/pdfutil`, `//go:embed` +
>     `AddUTF8FontFromBytes`) across all 4 generators (`·`/`—`/accents render correctly); CSV importer
>     exports prepend a UTF-8 BOM (parser strips it on re-upload). Backend: new `auth/emailchange.go`
>     token store (mirrors pwreset), 2 email templates + mailer methods, identity service + 5 endpoints +
>     OpenAPI, audit for all mutating flows. **Approved deviations / decisions:** **(a)** "format limiter"
>     = `thousandSeparator` prop (per user); **(b)** email-change link → **new** address + **requires
>     current password** (industry std); **(c)** password change is **link-based** (verify old pw → email
>     reset link), so the account page no longer changes the password inline; **(d)** resend cooldown is
>     **exponential** (user choice); **(e)** PDF fix embeds a font (robust for dynamic DB data) vs. char
>     sanitization; **(f)** telepon stored on `masterdata.employees.phone` (**not** `users`), so users
>     without a linked employee can't edit it (disabled + hint); **(g)** the **account Profil/Keamanan tabs
>     deviate from `docs/design/Profil Akun.dc.html`** — the edit-toggle, "Ubah Email" modal, and
>     link-based "Ganti Password" modal are **net-new, user-requested** interactions the static mockup
>     predates; `verify-email.vue` is a net-new page styled after `forgot-password`/`reset-password`;
>     **(h)** wrong-current-password returns **400** (not 401) on the change-request endpoints to avoid the
>     frontend's 401 auto-logout interceptor. **Wrinkles resolved in-flight:** the planned `000034` phone
>     migration was a **duplicate** of `000019` and removed. **Verification:** backend `go build/vet/test`
>     ✅, Spectral 0 errors ✅; frontend `pnpm lint/typecheck/test` (1383) ✅. **Follow-ups (open,
>     non-blocking):** live browser 1:1 mockup pass (account/forgot-password/map, both themes) not run —
>     stack unreachable in the build env; e2e `account-security.spec.ts` (email + password change via
>     Mailpit) is **CI-deferred**; **`UpdateProfile` is not wrapped in a single transaction** — name and phone are
>     two separate writes, so a phone-write failure after the name commits leaves a partial update (spec
>     bagian 2 asked for one tx; low harm — caller's own data, retriable — deferred pending tx wiring); the two
>     authed send endpoints now have a **server-side per-IP rate limit** (fix #2) on top of the client
>     cooldown; NumberInput's paste-of-`.`-decimal into a grouping+`decimals>0`
>     field is latent-only (no field combines them); ~~`kantor`/`pegawai` display names show `—` (API returns
>     only IDs — needs a masterdata join or profile enrichment)~~ ✅ **RESOLVED (2026-07-16) — see item 64**
>     (profile enrichment: `role_name`/`office_name`/`employee_name`); the live browser 1:1 mockup pass
>     (account/forgot-password/map, both themes) was also run in that session.
> 57. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-14): UX fixes batch #2 — see item 58.**
> 58. ~~**UX Fixes Batch #2 (7 fixes)**~~ ✅ **DONE (2026-07-14, branch `feat/ux-fixes-batch`).** User request:
>     7 UX/correctness fixes. **(1) Date inputs → Nuxt UI calendar** — new reusable
>     `frontend/app/components/DateField.vue` (ISO `YYYY-MM-DD` v-model, `UInput` + `UCalendar` popover
>     picker; typeable ISO **and** calendar-pick) replaces every native `<UInput type="date">` across 9
>     files (asset form ×2, maintenance record/schedule slideovers, disposals ×2, transfers ×3, assignment
>     ×2, peminjaman, ajukan-peminjaman modal, audit filters ×2). `assignment.vue` date fields gained
>     testids; e2e `assignment.spec` switched off the removed `input[type="date"]` selector. **(2) Pagination
>     — max 3 page buttons** — `TablePagination.vue` rebuilt as a compact sliding window (≤3 numbered
>     buttons centred on current + prev/next chevrons + range text) replacing `UPagination` (reka-ui always
>     renders first+last, so it couldn't be capped). **Follow-up (2026-07-14, branch `fix/unify-pagination`):**
>     the **Katalog Aset** (`pages/assets/index.vue`, table **and** grid views) and **Audit Trail**
>     (`pages/settings/audit.vue`) had their own hand-rolled paginators that enumerated *every* page (Katalog
>     showed 22+ buttons over 63k rows); both now use the shared `TablePagination` via a `pageOffset`
>     writable-computed bridging their 1-based `page` ref to the component's 0-based offset contract. All
>     list screens now share one paginator (master screens already did via `ResourceTable`); the
>     `audit-next-page` testid was replaced by the component's `pagination-next` (spec updated). **Second
>     follow-up (2026-07-14, branch `fix/spa-cache-headers`):** after the unify deploy the catalog still
>     showed the old pagination until a manual hard refresh — the SPA `index.html` shell was served with
>     **no `Cache-Control`**, so the browser heuristically cached a stale shell pointing at the previous
>     build's chunk URLs. Added Nitro `routeRules` in `nuxt.config.ts`: `/_nuxt/**` stays
>     `immutable` (content-hashed), `/**` (the HTML shell) is `no-cache` (always revalidate) — verified via
>     `node .output/server/index.mjs` + curl. New deploys now land without a hard refresh. **(3) i18n `assets.import`** — the Katalog toolbar
>     button used `t('assets.import')`, which resolves to the wizard **namespace object** (a JSON duplicate
>     key: string at top, object at `assets.import.*`) → raw key rendered; renamed the button string to
>     `assets.importBtn` (id/en). **(4) Dashboard refresh icon** — the `:loading` button swapped the
>     refresh glyph for a spinner ("ikon muncul tapi tidak sesuai"); now the `i-lucide-refresh-cw` icon
>     always shows and spins via `animate-spin` while loading. **(5) Collapsed submenu click** — clicking a
>     parent-with-children while the sidebar is collapsed did nothing (children only render when expanded);
>     `onParentClick` now opens the rail (`sidebarCollapsed = false`) **and** expands that group. **(6) Labels
>     under icons when collapsed** — collapsed nav items now stack icon-over-label (`flex-col` + `text-[10px]`
>     truncated label) for leaf, disabled, and parent items. **(7) Thousand separators in reports** — new
>     `formatInt()` in `utils/format.ts` (id-ID grouping, sign-preserving, `—` on invalid) applied to the
>     plain counts in `reports.vue` (KPI default + chart bars + utilization days/loans + maintenance actions
>     + opname total-items/variance) and the 4 stock-opname detail KPIs (total/found/pending/variance);
>     money already grouped via `formatRupiah`/`formatMoneyShort`, dashboard already via `formatCount`.
>     **Notable bug found + fixed during live verification (#6):** adding `truncate` (nowrap) labels made
>     each collapsed item's min-content as wide as its full label; the `<aside>` is a **flex item** with
>     default `min-width:auto`, so it refused to shrink to `w-[76px]` and stayed **264px**. Root cause
>     confirmed empirically in-browser: a bare `width`/`flex-basis` is overridable by the flex row, and the
>     `transition-all` on the rail left the size **stuck at the start value** in the preview pane. Fixed by
>     pinning the rail with inline **`width` + `minWidth` + `maxWidth`** (`sidebarWidth` computed;
>     `max-width` is a hard cap the flex algorithm honours) and narrowing the transition to
>     `transition-colors` so the width change applies instantly (no stuck animation). **Approved deviations /
>     decisions:** **(a)** `DateField` keeps a **typeable ISO text field paired with** the `UCalendar` picker
>     (industry-standard combo; preserves all existing `.fill()`/`.setValue()` tests) rather than a
>     calendar-only read-only trigger; **(b)** collapsed rail width is pinned via inline min/max/width, so
>     `AppSidebar` no longer relies on the `w-[76px]`/`w-[264px]` class for sizing (tests updated to assert
>     the inline style). **Verification:** `pnpm typecheck`/`lint`/`build` ✅; **full `pnpm test` — 108 files,
>     1413 tests, exit 0** ✅ (new: `formatInt` unit cases, `table-pagination` windowing spec, `date-field`
>     spec, AppSidebar collapsed-label + collapsed-parent-opens-rail specs). **Live-verified in-browser**
>     (seeded demo, admin login): date-pick fills ISO, pagination window slides `[1,2,3]→[2,3,4]` and caps
>     at 3, Import label renders, sidebar collapses to 76px with under-icon labels, collapsed parent opens
>     the rail + group, reports KPI count renders `63.084`. **Follow-up (open):** screenshots time out in the
>     in-app preview pane, so the collapsed-rail look was verified via DOM measurement, not a visual capture.
> 59. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-14): live-app fix batch —
>     depreciation schedule perf + table-action UX (user-reported against the live deploy, not from the
>     candidate list below) — see item 60.**
> 60. ~~**Depreciation schedule perf + table-action UX batch**~~ ✅ **DONE (2026-07-14, branch
>     `feat/depreciation-perf-and-table-ux`, not yet merged).** Live-app fix batch against
>     `https://inventra.ragilbuaj.web.id`. **`/depreciation/schedule` rewrite (perf + double-call):**
>     replaced 3 unbounded queries + a full-set Go loop with 3 SQL-aggregated/paginated queries —
>     `ScheduleRows` (one asset-based, filtered, `LIMIT`/`OFFSET` page — assets `LEFT JOIN` this-period
>     entry `LEFT JOIN` lateral accumulated-sum, with the parameterizable-union predicate mirroring
>     `engine.go`'s `ResolveCommercial`/`ResolveFiscal` Skip checks in SQL), `ScheduleTotals` (filtered
>     tfoot + `total` count), `ScheduleKpi` (UNFILTERED KPI tiles + `asset_count`). Removed
>     `ListAssetsForScheduleUnion` + `SumAmountsThroughPeriodByAsset`; kept `ListEntriesForPeriod`
>     (journal). Handler gained `limit`/`offset` (default 10, clamp 1–100) + `total`/`limit`/`offset` in
>     the response; OpenAPI updated. Engine now resolves method/life only for the ≤10 page rows. The
>     frontend dropped the separate unfiltered KPI fetch (`loadKpis`) — the single `/schedule` response now
>     carries `kpi` — so the endpoint is called **once** per change (was twice), and server pagination
>     (10/page) bounds the table. **KPI-card overflow (#3):** new `formatRupiahCompact` util (`Rp 1,23 M` /
>     `Rp 3,4 T`) + `min-w-0 truncate` + `:title` full-precision tooltip on the 4 KPI tiles; table totals
>     keep full precision. **Row actions standardized (#1):** new shared `RowActionsMenu` (kebab `⋮`
>     dropdown) + `buildActionGroups` util, extracted from `ResourceTable`, applied with a wrapper-level
>     right-click `UContextMenu` (reset-`contextItems`-on-non-row-target) to all 8 action-bearing tables:
>     assets catalog, transfers, disposals, peminjaman, stock-opname items, reports opname, maintenance
>     records, and depreciation schedule (Impair). Read-only (journal, audit, assignment history, detail
>     sub-tables) + matrix/segmented (data-scope, field-permission) tables intentionally left as-is.
>     **Pagination = 10 (#5):** every list-screen page size set to 10 (`ResourceTable` default, assets,
>     audit, reference, employees, categories 7→10, ImportWizard preview) + composable `?? 20 → ?? 10`
>     fallbacks. Picker/lookup/existence-check limits untouched. **Deviations (recorded):** **(a)** the
>     unified schedule query applies `a.deleted_at IS NULL` uniformly, so entries for **soft-deleted assets
>     no longer appear in the schedule** (the old entry-row query lacked this filter; consistent with the
>     union path) — a deliberate consistency tightening; **(b)** the peminjaman **cancel** action lost its
>     per-row loading spinner (`RowAction` has no `loading` field) — now disabled-while-in-flight,
>     re-entrancy guard intact; **(c)** added **unfiltered** `kpi.asset_count` so the acquisition tile's
>     "n aset" sub-label stays full-set under table filters (matches the documented "tiles never shrink"
>     intent). **Known minor/follow-ups:** orphaned i18n key `impairDisabledFiscalTooltip` now dead (Impair
>     tooltip dropped in the menu conversion); `formatRupiahCompact` renders `Rp 1.000 rb` at 999.999
>     (bucket chosen pre-rounding — documented/tested); a flaky `EnvironmentTeardownError` in
>     `assets-index.spec.ts` teardown is a pre-existing Nuxt UI overlay-teardown artifact (not introduced;
>     full suite passes 1466/1466 with exit 0 on a clean run). **Verification gate (2026-07-14):** backend
>     `go build`/`go vet`/`go test` green; `go test -tags=integration ./internal/depreciation/` green
>     (parity + pagination + filter-shrinks-rows-not-KPI + scope); Spectral 0 errors; frontend
>     lint/typecheck green, `pnpm test` 1466/1466 green, `pnpm build` green.
> 61. ~~**Merge the depreciation-perf batch + pick the next step.**~~ ✅ Branch
>     `feat/depreciation-perf-and-table-ux` **merged (PR #72, `a7fc370`, 2026-07-14)**; next step picked:
>     **Spec B — Device Sessions**.
> 62. ~~**Device Sessions (Spec B) — session list / revoke-per-device / logout-all-others**~~ ✅ **DONE
>     (2026-07-15).** Replaces the last `useAccount` mock (`listSessions`/`revokeSession`/`logoutAllOthers`).
>     **Backend:** a stable `sid` claim minted at login and carried unchanged through refresh rotation
>     (embedded in both access + refresh JWTs); a **Redis** session store (`internal/auth/sessionstore.go`:
>     `auth:session:<sid>` hash + `auth:usessions:<uid>` per-user index, TTL = refresh TTL) recording
>     device metadata; **instant revocation** via a `RequireAuth` session-alive check (a revoked session's
>     still-unexpired access token now 401s on its next request; pre-`sid` legacy tokens skip the check and
>     age out); **GeoIP** (`internal/geoip`, MaxMind GeoLite2 via `oschwald/geoip2-golang`, config
>     `GEOIP_DB_PATH`) resolved once at login with a **no-op fallback** so dev/CI run without a DB (location
>     degrades to the raw IP — decision [[Lokasi Sesi via GeoIP]]); a pure-Go UA parser
>     (`internal/identity/useragent.go`); 3 endpoints `GET /auth/sessions`, `DELETE /auth/sessions/:id`
>     (SoD-gated: a foreign sid → 404), `POST /auth/sessions/revoke-others`; a password change/reset now
>     also **clears all sessions** (uniform logout everywhere). **Frontend:** real `useAccount` sessions
>     (mapped to the existing `AccountSession` shape via `useNuxtApp().$i18n` so the composable stays
>     callable outside setup), a `formatRelativeTime` util (built-in `Intl.RelativeTimeFormat`, id/en), and
>     `account.vue`'s "logout all" re-fetches so the list collapses to the current device. **Verification
>     gate (2026-07-15):** backend `go build`/`go vet`/`go test` green; `go test -tags=integration -p 1
>     ./internal/auth/ ./internal/identity/ ./internal/middleware/` green (session store CRUD/prune/order,
>     login-creates-session, refresh-keeps-sid, revoke→SessionAlive-false + 401, revoke-others-keeps-current,
>     password-change-clears-all, handler current-flag + 404); Spectral 0 errors; frontend
>     lint/typecheck/test/build green. Spec `docs/superpowers/specs/2026-07-15-device-sessions-design.md`,
>     plan `docs/superpowers/plans/2026-07-15-device-sessions.md`.
> 63. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-16): item-56 follow-ups —
>     profile office/employee/role name enrichment + live mockup visual pass — see item 64.**
> 64. ~~**Profile enrichment (`role_name`/`office_name`/`employee_name`) + live visual pass**~~ ✅ **DONE
>     (2026-07-16, branch `feat/profile-enrichment`).** The account **Profil** tab showed `—` for Peran /
>     Kantor Penempatan / Pegawai Tertaut because `GET /auth/profile` returned only `role_id`/`office_id`/
>     `employee_id`, and `peran` fell back to the auth store's `role_name` which is **always `''`**
>     (`/auth/me` never populates it). **Backend:** `GetUserProfile` now LEFT JOINs `identity.roles` +
>     `masterdata.offices` (employees already joined for phone) and selects `role_name`/`office_name`/
>     `employee_name` (all `*string` — nullable via LEFT JOIN + `deleted_at IS NULL`); `ProfileView` DTO +
>     `profileFromRow` + OpenAPI `Profile` schema extended; `sqlc generate` (no migration — additive SELECT
>     only). **Frontend:** `useAccount.mapProfile` now uses `raw.role_name || auth-store fallback`,
>     `raw.office_name ?? ''`, `raw.employee_name ?? ''` (was hardcoded `kantor:''` and `pegawai: raw.name`
>     — the **user** name, not the employee's). **Tests:** backend service/handler (enriched + null
>     office/employee cases, no-`google_id`/`password_hash` leak); frontend `useAccount.spec` (API-first,
>     store fallback when empty, null→'', employee_name≠user-name) + `account-profile.spec` (renders
>     enriched rows). **Gates:** `go build/vet/test` + `-tags=integration ./internal/identity/` ✅; Spectral
>     0 errors ✅; frontend lint/typecheck ✅, full `pnpm test` **1484 passed** (114 files; the lone
>     `assets-index.spec.ts` teardown `EnvironmentTeardownError` is the pre-existing Nuxt-UI flake noted in
>     item 60, unrelated), `pnpm build` ✅. **Runtime-verified** against the seeded demo DB: `GET
>     /auth/profile` for a staff user returned `role_name:"Staf"`, `office_name:"Kantor Cabang Pekanbaru"`,
>     `employee_name:"Wahyu Siregar"`; the browser account page renders those 1:1 in **light + dark** (header
>     + "Informasi Akun" rows), and the Superadmin (no office/employee) correctly shows Peran "Superadmin" +
>     `—`/`—`. **Live visual pass** also covered `/forgot-password` (full-width input, split-panel) and
>     `/master/map` (pins + legend + controls above Leaflet panes) — both match their mockup/intent.
>     **Known (pre-existing, not a regression):** a user linked to a **soft-deleted** employee still reports
>     `hasEmployee=true` (telepon editable) while `pegawai`/phone resolve blank — the `employee_phone` join
>     and `hasEmployee` already behaved this way; the pegawai fix (user-name → employee_name) only improves
>     the display. **Not in scope:** filling `role_name` on `/auth/me` / login (drives `UserMenu`/
>     `AppSidebar` badge via `auth.user.role_name`) — a separate path; follow-up if a consistent shell badge
>     is wanted.
> 65. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-16): authz nav/guard consistency
>     bug** — reported as "kanwil role has a menu but cannot open it". Audit found three misaligned layers
>     (nav visibility, page can-guard, backend endpoint permission).
> 66. ~~**Authz nav/guard/endpoint consistency**~~ ✅ **DONE (2026-07-16, branch `feat/authz-nav-guards`).**
>     Root cause: the sidebar picked the whole nav from a single `user.manage` bit
>     (`can('user.manage') ? superadminNav : staffNav`), so kepala_kanwil/kepala_unit/manager fell to the
>     staff menu and could not reach menus their permissions grant; several nav items had no `permission` and
>     403'd on click; the authz-admin screens guarded on `user.manage` while their endpoints need
>     `role`/`scope`/`fieldperm.manage`. Fix: one per-permission `appNav` (`NavItem.permission: string |
>     string[]`, OR semantics), `can` middleware accepts arrays, AppSidebar/AppTopbar rewired with
>     empty-group auto-hide + per-child visibility, five page-guards aligned to their entry permission
>     (assignment→view, maintenance→view-or-create, rbac→role.manage, data-scope→scope.manage,
>     field-permission→fieldperm.manage), dashboard summary fetch gated on `report.view`, assignment
>     available-fetch gated on `request.create`. Backend (Opsi 1): new `RequireAnyPermission`; authz-admin
>     READS loosened via two gates (`GET /catalog` = any of role/scope/fieldperm.manage; `GET /roles` +
>     `GET /roles/:id` = those + `user.manage` for the Users role picker), all mutations left strict — so
>     `scope.manage`/`fieldperm.manage` can now be delegated independently. Tests: per-role unit
>     (visible-set = permission-set), runtime AppSidebar per role, `can` OR unit, backend
>     `TestRequireAnyPermission` + delegation integration (scope-only / user-only), e2e Superadmin full-nav
>     reachability sweep. No schema/seed/role-permission changes. Spec
>     `docs/superpowers/specs/2026-07-16-authz-nav-guard-consistency-design.md`, plan
>     `docs/superpowers/plans/2026-07-16-authz-nav-guard-consistency.md`.
> 67. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-17): notifications.**
> 68. ~~**Notification module — full pipeline** (`feat/notifications`)~~ ✅ **DONE (2026-07-18).**
>     Modul notifikasi in-app dari nol: **transactional outbox (Postgres) lalu Redis Stream lalu
>     consumer fan-out lalu feed per-user**, plus sweeper `maintenance_due` + purge retensi. Mengganti
>     `frontend/app/mock/notifications.ts` — **mock terakhir di app-shell terhapus seluruhnya**
>     (`app/mock/` tidak ada lagi). Spec `docs/superpowers/specs/2026-07-17-notifications-design.md`
>     + plan `docs/superpowers/plans/2026-07-17-notifications.md`; keputusan arsitektur di
>     **[ADR-0014](adr/0014-notification-delivery.md)**. 19 task, subagent-driven, tiap task diverifikasi
>     + di-commit terpisah.
>     - **Backend:** migrasi `000034` (skema `notification`: outbox + feed, `uq_notif_dedup` menopang
>       consumer at-least-once) + `000035` (index lookup outbox). Empat jenis: `approval_pending` (ke
>       approver berhak — **invers kelayakan** `NotifiableApprovers` yang menyaring lewat
>       `eligibleToDecide` yang sudah ada, aturan SoD **tidak** diduplikasi di SQL), `approval_decided`
>       (ke maker), `asset_returned` (ke yang meng-check-out — `assigned_by_id`), `maintenance_due`
>       (sweeper, `pg_advisory_xact_lock`). Enqueue **se-transaksi** di service (bukan handler
>       best-effort) — rollback tidak meninggalkan event yatim. Auto-resolve notifikasi basi saat
>       giliran step lewat. 4 endpoint feed (`RequireAuth`, per-user, mark-read milik orang lain jadi 404).
>       Relay/consumer/sweeper dijalankan dari `main.go` (`NOTIFICATION_WORKER_ENABLED`), diverifikasi
>       dengan **menjalankan binary sungguhan** + menyuntik event dan mengamatinya mengalir.
>     - **Frontend:** `useNotifications` nyata (async), Pinia store (refresh event-driven dari
>       choke-point fetchMe, **tanpa polling**), `notificationMeta` catalog, bel `NotificationBell.vue`
>       lengkap, halaman `/notifications` penuh (paginasi/filter/state), `NotificationItem` +
>       `useNotificationLink` bersama (bel & halaman satu sumber). E2E backend-nyata
>       (`e2e/notifications.spec.ts`, 2 tes, async-aware).
>     - **Deviasi disetujui (catat-deviasi):** (a) **storage Postgres bukan Redis** — menyimpang PRD A1b,
>       di-supersede ADR-0014; (b) **lock Postgres advisory bukan Redis** (PRD baris 458); (c) **Redis
>       Streams sebagai transport + AOF dinyalakan** (`--appendonly yes` di 3 compose); (d) **halaman
>       `/notifications` tanpa mockup** — dirancang mengikuti bahasa visual App Shell; (e) **subtree i18n
>       `notifications.item.*`/`time.*` diganti** (kunci lama meng-hardcode nama); (f) **panel dropdown
>       bel dark-mode pakai bg-default bukan bg-elevated** (konsistensi dengan semua panel mengambang);
>       (g) **baris unread bertint bg-primary/5** (mockup tak menggambarkan state dibaca). Deviasi
>       peminjaman: `asset_returned` dinotifikasi ke **approver** (pemegang `assigned_by_id`), bukan Staf
>       peminjam — `employee_id` bukan user login.
>     - **Follow-up (tercatat, tidak memblokir):** SSE untuk push real-time (perlu verifikasi buffering
>       di balik Caddy/Coraza); job arsip/partisi notifikasi lama ke cold storage; penyelarasan retensi
>       90 hari ke kebijakan records-retention korporat; `importer.resolveMakerScope` adalah salinan
>       terakhir aturan scope yang belum dilipat ke `common.OfficeScopeFor`; celah rute maker
>       (`approval_decided` menaut ke `/approval` yang digerbangi `request.decide`, permission yang
>       mungkin tak dimiliki maker — klik sudah digerbangi, tapi rute detail-request menghadap maker
>       adalah keputusan produk); **`EnvironmentTeardownError` pre-existing** membuat `pnpm test` lokal
>       exit 1 meski semua tes lulus (race teardown modul lazy Nuxt di bawah beban paralel; menunjuk
>       file berbeda tiap run; **CI hijau di main** — bukan dari fitur ini).
> 69. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-18): admin-initiated password
>     reset + Resend email transport — see item 70.**
> 70. ~~**Admin-initiated password reset (User Management) + Resend email transport**~~ ✅ **DONE
>     (2026-07-18, branch `feat/admin-password-reset`).** Spec/plan
>     `docs/superpowers/{specs,plans}/2026-07-18-admin-password-reset*`. Two coherent commits in one PR.
>     - **Password reset (feature):** a **Reset Password** row action on `/settings/users` (mockup
>       `Manajemen User.dc.html` had the menu item; the action was dropped in item 12/35 "pending a
>       backend reset endpoint" — the email blocker is gone since item 54). New endpoint
>       `POST /users/:id/reset-password` (gated `user.manage`) → `identity.AdminInitiatePasswordReset`
>       generates a single-use TTL token and **emails the reset link to the target user** via the
>       existing pipeline; the admin never sees/sets the password (PRD FR-1.5 "reset via token";
>       best-practice). New sentinel `identity.ErrNoPasswordLogin` → **422** for Google-only accounts
>       (nothing to reset); missing user → 404; audited as `update` on `users`. Handler depends on a
>       narrow `passwordResetInitiator` interface (satisfied by `*identity.Service`, wired in
>       `NewRouter`) so it stays testable. Frontend: `useUsers().resetPassword` (suppresses the generic
>       error toast — the page owns its toasts), confirm dialog (primary, non-destructive), success toast
>       names the address, 422 → warning toast. Tests: identity unit (not-found/Google-only) + integration
>       (success saves+sends), user handler integration (200/422/404 + audit), 5 component tests, and a
>       real-backend e2e (`e2e/admin-password-reset.spec.ts`, Mailpit link assertion + Google-only 422).
>       OpenAPI synced (Spectral clean).
>       - **Deviation (recorded):** the admin path is **permissive about status** — an inactive/suspended
>         user that still has a password is emailed anyway (status is a login-time gate; an admin may reset
>         before reactivating). This differs from the self-service `RequestPasswordReset`, which blocks
>         non-active accounts (anti-enumeration silence is irrelevant to an admin who already knows the user).
>     - **Resend email transport (separate commit, same branch):** new `email.ResendSender` delivering via
>       the **Resend HTTP API** (`POST api.resend.com/emails`, Bearer key, JSON `{from,to,subject,html,text}`,
>       10s timeout, non-2xx → error with status + parsed message, **never the API key**). `NewSender`
>       selects transport by **`EMAIL_PROVIDER`** (`smtp` default | `resend` | `log`); the pre-Resend
>       SMTP/log behavior is unchanged when unset, so **dev/e2e keep using Mailpit over SMTP**. Config
>       `EMAIL_PROVIDER` + `RESEND_API_KEY`; `.env.example` + `DEPLOYMENT.md` email section updated.
>       Contract grounded in the official Resend API reference (source-driven). Tests: httptest
>       success/error, api-key-not-leaked, provider-selection matrix. **Recommended for production**
>       (Biznetgio VPS): tolerant of restrictive SMTP egress.
>     - **Verification note:** the local full e2e first ran against a **stale host backend on :8080**
>       (a leftover `go run ./cmd/api` from a prior session shadowing the freshly-built container — same
>       class as item 25's stale-source note) and against a dev DB with **845 accumulated test users**
>       (pushing the seeded admin off `GET /users` page 1). After killing the stale process + a clean
>       `docker compose down -v` reset, the fresh-DB serial suite is **102 passed**; the 2 remaining
>       failures (`account` change-password modal, `maintenance` "jatuh tempo hari ini" date-boundary) are
>       **pre-existing and unrelated** — different screens, deterministic, and the e2e backend didn't even
>       carry the Resend change. `admin-password-reset` + `settings` both pass.
> 71. ~~**Paginate the Stock Opname item table + the Laporan result table**~~ ✅ **DONE (2026-07-18).**
>     Both tables loaded their full row set in memory and rendered every row (stock-opname `listItems`
>     returns `total=len, limit=len`; `GET /reports/:type` returns all rows capped by a truncation guard),
>     so the fix is **client-side pagination** via the shared `TablePagination` component (0-based offset,
>     PAGE_SIZE 10 — the same paginator as every other list screen). `stock-opname.vue`: paginate over
>     `filteredItems` (search/room/result stay client-side), reset to page 1 when the filtered set changes.
>     `reports.vue`: paginate over `view.rows` for both the generic and opname result tables, reset on
>     `apply()` and report-type switch; the TOTAL `tfoot` still reflects the whole dataset and the
>     truncation notice is unchanged. **Approved user-requested deviation (catat-deviasi convention):** the
>     `docs/design/Stock Opname.dc.html` and `docs/design/Laporan.dc.html` mockups have no paginator — the
>     user asked for one, so the shared design-system paginator was added at each table's foot. A chosen
>     opname item result survives page navigation because it lives in `allItems` (persisted server-side via
>     `setItemResult`), not in page-local state — the paginator only slices the derived view, with a
>     regression test locking it. 11 new runtime tests (stock-opname + reports), full suite green
>     (1609 passed; the one `assets-index` teardown unhandled-rejection is a pre-existing @nuxt/test-utils
>     flake, unrelated); `pnpm typecheck`/`lint` clean.
> 72. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-18): mobile-responsive across all
>     menus — see item 73.** Carried candidates: **room/floor import targets**;
>     **Analytics/OLAP** read layer; **`/auth/me` `role_name`** (shell badge consistency); **admin
>     reset-password: audit as a dedicated `shared.audit_action` value** instead of `update` (needs a
>     migration); **pre-existing e2e failures** (`account` change-password modal, `maintenance`
>     date-boundary "jatuh tempo hari ini") worth a separate fix. **GeoIP DB provisioning** in prod
>     (drop a GeoLite2-City.mmdb + set `GEOIP_DB_PATH`) is an ops follow-up. Notification follow-ups
>     (SSE, retention archival, maker-route gap). Confirm priority before starting.
>     **Follow-up (optional):** a per-limited-role UI e2e (log in as a created `scope.manage`-only user and
>     confirm Data Scope opens while RBAC is hidden) — the seeded-superadmin CI e2e can't log in as demo
>     roles, so this needs an API-created user; the per-role guarantee is already covered by unit/runtime/
>     integration tests.
> 73. ~~**Mobile-responsive across all menus**~~ ✅ **DONE (2026-07-18, branch `feat/mobile-responsive`).**
>     The app shell was desktop-only (the `flex` layout in `layouts/default.vue` had no breakpoint, and
>     `AppSidebar` pinned its width in px with no drawer), so every screen broke below `lg`. Fix, in four
>     phases, preserving the desktop look 1:1 (all changes only add behavior below `lg`/`sm`):
>     - **Shell (biggest lever):** `stores/ui.ts` gains `mobileNavOpen` + `open/close/toggleMobileNav`.
>       `AppSidebar` becomes an **off-canvas drawer below `lg`** (`fixed` panel, `-translate-x-full`
>       hidden → `translate-x-0` open, backdrop scrim, `inert` while closed, Escape/scrim/route-change +
>       nav-click auto-close) and stays the **in-flow collapsible rail at `lg+`** (width via responsive
>       classes `lg:w-[76px]`/`lg:w-[264px]` — the old inline min/max px style is gone). A new
>       `useMediaQuery`/`useIsDesktop` composable drives the "drawer is always fully expanded on mobile"
>       logic (defaults to desktop when `matchMedia` is absent, so the jsdom test runtime still exercises
>       rail-collapse). `AppTopbar` gets a **hamburger (`<lg`, opens the drawer)** distinct from the
>       desktop panel-left rail toggle; the inline global search is hidden below `md` (still reachable via
>       `Ctrl+K`). Content padding is now responsive (`px-4 py-5 sm:px-6 lg:px-8`).
>     - **Pagination (one lever, every list):** `TablePagination` wrapper stacks vertically below `sm`
>       (`flex-col … sm:flex-row sm:justify-between`) so the range text + page buttons never crowd on a
>       narrow screen. Tables themselves **keep horizontal scroll** (user's explicit choice — the wide
>       `<table>` stays inside its `overflow-x-auto` wrapper; the page body never overflows).
>     - **Fixed-grid sweep:** stat grids `grid-cols-4`/`grid-cols-3` → `grid-cols-2 lg:grid-cols-4` /
>       `grid-cols-1 sm:grid-cols-3` (stock-opname, disposals, reports); the repeated `grid grid-cols-2`
>       form-field pairs inside modals/slideovers → `grid-cols-1 sm:grid-cols-2` (offices, employees,
>       users, transfers, assignment, disposals, account, category/record/peminjaman modals);
>       `PageHeader` wraps its actions; `DataToolbar` search is `w-full sm:w-64`. Left as-is (already fine
>       at 320px): the assets filter bar, the account theme picker (3 small buttons), and the import
>       result 2-card row.
>     - **Tests + live verification:** new `test/unit/ui-store.spec.ts` (drawer/rail action independence)
>       + updated `AppSidebar`/`AppTopbar` runtime specs (drawer classes, scrim, hamburger vs rail toggle,
>       width classes) — 53 targeted tests green; full suite **1617 passed** (the lone `assets-index`
>       teardown unhandled-rejection is the same pre-existing @nuxt/test-utils flake noted in item 71 —
>       reproduced on a clean tree). `lint`/`typecheck`/`build` clean. **Live sweep** (dev stack + seeded
>       admin, browser preview): all **25 menu routes at 375px show zero body overflow**; the drawer
>       opens with the full expanded nav + scrim and auto-closes on navigation; at 1280px the sidebar is
>       the in-flow 264px rail with the hamburger hidden and search/panel-left visible — desktop unchanged.
>     - **Deviation note (catat-deviasi):** the `docs/design` mockups are desktop-only (no mobile
>       comps); the mobile drawer + responsive grids are a **user-requested** capability, and desktop
>       (`lg+`) remains a 1:1 match to the mockups.
> 74. ~~**Fix: env email tidak diteruskan ke backend produksi**~~ ✅ **DONE (2026-07-18, branch
>     `fix/prod-email-env`).** Email transaksional (reset password, notifikasi ganti password/email) tidak
>     pernah terkirim di produksi meski `.env.prod` sudah diisi kredensial Resend. Sebabnya
>     `docker-compose.prod.yml` mendaftarkan env backend satu per satu dan **tidak memuat** `MAIL_ENABLED`,
>     `EMAIL_PROVIDER`, `RESEND_API_KEY`, `SMTP_FROM`, `SMTP_FROM_NAME` — Compose hanya meneruskan variabel
>     yang disebut eksplisit, jadi backend memakai default `MAIL_ENABLED=false` dan jatuh ke `LogSender`,
>     yang mengembalikan `nil` (sukses semu) sambil hanya mencatat ke log. Dikonfirmasi di VPS:
>     `docker exec inventra-backend env | grep -E "MAIL_|EMAIL_|RESEND_"` kosong total. Perbaikan: kelima
>     variabel ditambahkan ke service `backend` (default `MAIL_ENABLED=true`, `EMAIL_PROVIDER=resend`),
>     blok email ditambahkan ke `.env.prod.example`, dan tabel env di `docs/DEPLOYMENT.md` dilengkapi
>     `MAIL_ENABLED` + `SMTP_FROM` (sebelumnya hanya menyebut `EMAIL_PROVIDER`/`RESEND_API_KEY`, sehingga
>     dokumen tampak lengkap padahal compose-nya bocor). Diverifikasi via `docker compose config`. Catatan
>     operasional: domain `SMTP_FROM` harus sudah diverifikasi di dashboard Resend — tanpa itu Resend
>     menolak 403 dan hanya mengizinkan pengirim `onboarding@resend.dev` ke email pemilik akun.
> 75. ~~**Next session — pick the next real step.**~~ ✅ **Picked (2026-07-18): mobile companion app —
>     dokumentasi perencanaan dulu, lihat item 76.**
> 76. ~~**Mobile companion app — dokumen perencanaan & keputusan**~~ ✅ **DONE (2026-07-18, branch
>     `feat/mobile-docs`, PR #100).** Pemilik produk membuka scope mobile (PRD v1.1 semula mengecualikannya):
>     bentuk **field companion** (scan aset via kamera, approval on-the-go, push notification, stock
>     opname offline-first), teknologi **Flutter** (Android dulu, folder `mobile/` di monorepo),
>     **offline-first** untuk opname. Dokumen yang landed: roadmap
>     `docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md` (fase M0-M6, kesenjangan backend: push
>     FCM, batch sync opname, refresh via cookie jar tanpa perubahan backend); **ADR-0015** (Flutter;
>     alternatif Capacitor/PWA/React Native/Kotlin ditolak); **ADR-0016** (offline sync: batch idempoten
>     `client_scan_id`, konflik first-write-wins per aset per sesi + dilaporkan per-item); **PRD web
>     v1.2** (non-goal mobile dicabut, bagian 3.11 jadi penunjuk, baris stack + tahap 11 roadmap,
>     changelog); bagian *Mobile companion* di bagian Remaining; vault Obsidian (keputusan produk,
>     indeks ADR, Status & Roadmap, catatan sesi). **Dokumentasi mobile dipisah ke `docs/mobile/`**
>     (permintaan pemilik produk, agar dokumen web vs mobile mudah dibaca): **PRD mobile**
>     `docs/mobile/PRD.md` (FR-M1..M6 + NFR + asumsi), **ADR** pindah ke `docs/mobile/adr/`
>     (0015+0016, penomoran global tetap; `docs/adr/README.md` tetap indeks induk), **design brief +
>     prompt kit mockup** `docs/mobile/DESIGN_BRIEF.md` (master brief mobile + component library +
>     12 prompt per-layar, siap di-generate di Claude design; hasil ke `docs/mobile/design/`).
>     **Belum ada kode** — pengembangan mulai dari fase M0 setelah dokumen di-merge.
> 77. ~~**Next session — start here: Mobile M0 prep**~~ ✅ **Disela (2026-07-18): dokumentasi
>     arsitektur & konvensi mobile dilengkapi dulu — lihat item 78; M0 prep pindah ke item 79.**
>     Rincian lama: generate mockup mobile dari prompt kit
>     `docs/mobile/DESIGN_BRIEF.md` (13 artifact, hasil ke `docs/mobile/design/`) lalu spec + plan
>     fase M0 (scaffold Flutter `mobile/`, tema + i18n,
>     navigasi shell, login/refresh/logout cookie-jar, CI analyze/test/APK). Kandidat lain yang dibawa:
>     **room/floor import targets**; **Analytics/OLAP** read layer; **`/auth/me` `role_name`**; **admin
>     reset-password audit action** (migration); **pre-existing e2e failures** (`account` change-password
>     modal, `maintenance` date-boundary); **GeoIP DB provisioning** (ops); notification follow-ups (SSE,
>     retention archival, maker-route gap). Confirm priority before starting.
> 78. ~~**Mobile companion — dokumentasi arsitektur & konvensi**~~ ✅ **DONE (2026-07-18, branch
>     `feat/mobile-arch-docs`, PR #102).** Melengkapi dokumen mobile sebelum kode, di `docs/mobile/`:
>     **ARCHITECTURE.md** — klien tipis, struktur folder feature-first (`lib/features/*` dengan
>     `data/` + `presentation/`, aturan tanpa impor antar-fitur), Riverpod `AsyncNotifier` (tanpa
>     codegen provider), go_router + deep-link push, Dio tiga interceptor (cookie jar, refresh
>     single-flight, error mapper `AppFailure` sealed), drift `scan_queue` + sync engine (ADR-0016),
>     auth/token lifecycle, FCM, tema token Material 3, observability, peta testing per lapisan;
>     **CONVENTIONS.md** — Effective Dart + `flutter_lints` diperketat (`avoid_print` dkk., analyze
>     nol warning), tabel penamaan (file/provider/DTO/tabel drift/kunci ARB), file generated
>     di-commit (preseden sqlc), error handling wajib 3-cabang `AsyncValue`, konvensi tes ekspansif
>     (unit/widget/golden light+dark/integration vs compose), git scope `feat(mobile):`, keamanan
>     (secret via dart-define, larangan log token/PII); **README.md** — indeks peta dokumen mobile +
>     dokumen menyusul yang disarankan (SETUP.md di M0, TESTING.md evaluasi M1, RELEASE.md di M6,
>     katalog error API level repo). PRD mobile diberi penunjuk ke ketiganya. Belum ada kode.
> 79. ~~**Next session — start here: Mobile M0 prep**~~ ✅ **Disela (2026-07-18): dokumentasi
>     kesiapan iOS — lihat item 80; M0 prep pindah ke item 81.** Rincian lama: generate mockup
>     mobile dari prompt kit
>     `docs/mobile/DESIGN_BRIEF.md` (13 artifact, hasil ke `docs/mobile/design/`) lalu spec + plan
>     fase M0 (scaffold Flutter `mobile/`, tema + i18n, navigasi shell, login/refresh/logout
>     cookie-jar, CI analyze/test/APK). Kandidat lain yang dibawa: room/floor import targets;
>     Analytics/OLAP read layer; `/auth/me` `role_name`; admin reset-password audit action
>     (migration); pre-existing e2e failures (`account` change-password modal, `maintenance`
>     date-boundary); GeoIP DB provisioning (ops); notification follow-ups (SSE, retention archival,
>     maker-route gap); sapuan simbol section di komentar kode backend (task chip terpisah).
>     Confirm priority before starting.
> 80. ~~**Mobile companion — dokumentasi kesiapan iOS**~~ ✅ **DONE (2026-07-18, branch
>     `feat/mobile-ios-docs`, PR #103).** Keputusan produk: **fokus rilis tetap Android** (ADR-0015 tidak
>     berubah), tetapi kesiapan iOS didokumentasikan lengkap di **`docs/mobile/IOS.md`**: aturan
>     "iOS-ready sejak M0" yang mengikat kode sejak scaffold (folder `ios/` di-commit, tanpa API
>     Android-only di kode bersama, SafeArea sejak awal, back-swipe tidak dimatikan, izin
>     platform-aware, dart-define netral); tabel kompatibilitas library sisi iOS (semua pilihan
>     ADR-0015 sudah lintas platform — Keychain, AVFoundation, sqlite3 FFI, APNs via FCM);
>     checklist prasyarat aktivasi (macOS/Xcode, Apple Developer 99 USD/tahun, bundle id, APNs
>     key, GoogleService-Info.plist); setup push APNs (desain backend tidak berubah); Info.plist;
>     CI macOS (build ipa hanya di tag/mingguan — menit x10); distribusi **TestFlight** (jalur
>     utama; ad-hoc UDID & enterprise ditolak); checklist QA aktivasi; estimasi aktivasi 1-2 sesi;
>     pertanyaan terbuka QM3 (pola macOS) + QM4 (waktu aktivasi). Rujukan silang: baris IOS.md di
>     README mobile, penunjuk di PRD non-goal iOS, baris `ios/` di tree ARCHITECTURE, aturan
>     "jaga iOS-ready" di CONVENTIONS. Belum ada kode.
> 81. ~~**Profil: detail pegawai di kartu Data Diri + tombol Ubah pindah ke kartu + upload foto profil**~~
>     ✅ **DONE (2026-07-18, branch `feat/account-personal-data-card`).** Tiga permintaan user.
>     **(a) Detail pegawai.** `GetUserProfile` sebelumnya hanya men-join `e.name` + `e.phone`; sekarang
>     ikut mengambil `e.code`, `e.status`, dan nama departemen/jabatan lewat join baru ke
>     `masterdata.departments` + `masterdata.positions`. `ProfileView` bertambah `employee_code`,
>     `employee_status`, `department_name`, `position_name`. Di UI, blok "Detail Pegawai" (read-only,
>     bersumber dari master data) ada **di dalam** kartu Data Diri; baris "Pegawai Tertaut" dikeluarkan
>     dari kartu Informasi Akun sehingga kartu itu murni berisi metadata akun (Peran, Kantor, Metode
>     Login, Tanggal Bergabung). Akun tanpa tautan pegawai menampilkan catatan, bukan grid kosong.
>     **(b) Tombol Ubah.** Dipindah dari blok footer di luar kartu ke header kartu Data Diri (Ubah →
>     Simpan/Batal), karena editing hanya menyentuh isi kartu itu. `saveProfil` sekarang mengadopsi
>     respons server agar tampilan read-only ikut segar.
>     **(c) Upload foto profil.** Sebelumnya tombol Unggah/Hapus **inert** — tanpa `@click`, tanpa
>     backend, dan `users.avatar_url` tidak pernah ditulis kode mana pun. Migrasi `000036` mengganti nama
>     kolom itu jadi `avatar_key` (isinya object key MinIO, bukan URL — selaras dengan
>     `employees.avatar_key`). Endpoint baru `GET/POST/DELETE /auth/avatar`: upload divalidasi JPG/PNG
>     (`AVATAR_MAX_BYTES`, default 2 MB), di-decode sebagai cek isi sebenarnya, dipotong persegi tengah
>     dan di-encode ulang ke JPEG 512x512 via `imaging` — yang sekaligus **membuang metadata EXIF
>     (termasuk GPS)**. Object key diturunkan dari user id saja (`users/<id>/avatar.jpg`), tidak pernah
>     dari nama file, jadi nama file bermusuhan tidak bisa traversal maupun tabrakan; karena key stabil,
>     unggah ulang menimpa objek lama alih-alih menumpuk orphan. Tulis DB gagal → objek di-rollback.
>     Key **tidak pernah** diserialisasi: API mengekspos `has_avatar` (boolean) menggantikan `avatar_url`
>     di ProfileView, `/auth/me`, dan daftar user. Endpoint GET ter-autentikasi, jadi frontend
>     mengambilnya sebagai blob + `createObjectURL` (object URL di-revoke saat diganti/unmount agar tidak
>     bocor). Gagal ambil foto degradasi ke inisial, tidak memblokir halaman.
>     **Deviasi mockup (disetujui):** struktur kartu Data Diri berubah dari `docs/design/Profil
>     Akun.dc.html` karena diminta user secara eksplisit. Deviasi kedua, **disetujui user (2026-07-18)**:
>     tombol **Hapus** foto disembunyikan saat belum ada foto (mockup menampilkannya selalu), supaya
>     kontrol itu tidak pernah jadi no-op diam-diam.
>     **Belum dikerjakan (sengaja):** avatar belum muncul di `UserMenu` (sidebar/topbar) — mockup
>     `UserMenu` memang menampilkan inisial, jadi dibiarkan sesuai desain.
>     Verifikasi: `go build`/`vet`/`test ./...` hijau; Spectral 0 error; frontend `lint`/`typecheck`/
>     `build` hijau; `pnpm test` 122 file / **1666** tes lulus (+36 tes baru: 15 backend avatar, 21
>     frontend). Diverifikasi end-to-end di stack nyata (Postgres + MinIO + backend + Nuxt): PNG 900x300
>     jadi JPEG 512x512 tanpa EXIF, PDF/file rusak ditolak 415, 3 MB ditolak 413, DELETE idempoten dan
>     objek MinIO ikut terhapus, plus cek visual light dan dark mode. Catatan: `pnpm test` keluar dengan exit 1 karena satu `EnvironmentTeardownError` di
>     `assets-index.spec.ts` — **sudah ada sebelum perubahan ini**, dikonfirmasi dengan menjalankan suite
>     penuh di `main` bersih (juga exit 1, error yang sama). Perlu dibereskan terpisah.
> 82. ~~**Perbaikan layout label QR BTN 60x24 + penyelarasan halaman Label frontend**~~ ✅ **DONE
>     (2026-07-20, branch `fix/label-btn-layout`).** Permintaan user berawal dari hasil cetak label yang
>     tidak sesuai mockup BTN.
>     **(a) Backend PDF (`internal/asset/barcode.go`).** Akar masalah: auto page break bawaan fpdf
>     (margin bawah default 20 mm) memecah halaman label setinggi 24 mm menjadi ~8 halaman — tiap sel
>     teks terlempar ke halaman baru; kini `SetAutoPageBreak(false)` + margin halaman dan cell margin
>     dinolkan (cell margin 1 mm bawaan diam-diam menggeser semua teks). Layout template `btn` ditata
>     ulang sesuai mockup: margin 1 mm kertas-ke-border, border luar rounded (garis 0.3 mm), garis
>     pemisah vertikal antara QR dan kolom ringkasan (menyambung ke border), logo kecil + nama
>     perusahaan bold di header, kode kantor + `TP: <tahun>` bold, disclaimer merah bold rata tengah
>     dijangkar ke border bawah (sisa ruang jadi jarak dari nama aset), font menyusut otomatis agar muat
>     lebar kolom (`setFontFit`), baris disclaimer dipotong agar tak pernah keluar border. `prepLogo`
>     baru: file logo 4500x4500 dengan whitespace besar di-downscale dan di-trim (bbox non-putih);
>     overlay QR memakai `imaging.Fit` (aspek terjaga — sebelumnya dipaksa persegi hingga gepeng) di
>     atas alas putih quiet-zone, ukuran 32 persen sisi QR (EC High; area tertutup kira-kira 17 persen,
>     aman). Default lebar media kini = lebar label sehingga halaman roll persis 60x24 mm (`media_w_mm`
>     tetap bisa override); `TP` bersumber dari `purchase_date` (query `GetAssetLabelByID/ByTag`).
>     **(b) Frontend halaman Label (`/assets/label`).** Keputusan user: fokus template BTN. Preset
>     `60x24` ditambahkan dan jadi default (sebelumnya tidak ada; default lama 70x40); kontrol
>     "Tampilkan" (barcode/qr/keduanya) dan "Field dicetak" **dihapus** karena template btn
>     mengabaikannya di server — body cetak kini hanya `asset_ids`/`template`/`layout`/`size`/`columns`;
>     preview di layar diganti replika label BTN (`AssetLabel.vue` ditulis ulang, skala 5 px/mm) dengan
>     kode kantor + nama kategori di-resolve on-demand (`useResolveCache`), tahun dari `purchase_date`,
>     dan logo hasil trim di `public/logo-btn.png` (header + overlay tengah QR); fetch preview kini
>     hanya `type=qr`. i18n id/en dirapikan (key mode/field dihapus, hint format bank ditambahkan).
>     `assets-label.spec.ts` ditulis ulang — 23 tes: default 60x24, konten preview BTN (perusahaan/kode
>     kantor bukan nama/TP/kategori/disclaimer), TP kosong tanpa `purchase_date`, QR-only fetch + cache,
>     body cetak sheet/roll, clamp A4 semua kombinasi ukuran kali kolom, batas 500 aset.
>     Verifikasi: backend `go build`/`vet`/`test` hijau; frontend `lint`/`typecheck`/`build` hijau;
>     full vitest 122 file / 1680 tes lulus (dua run paralel sempat flake beban mesin — file gagal
>     berbeda tiap run dan lulus saat diulang; run bersih terakhir 0 gagal); verifikasi visual di stack
>     dev berjalan (QR + logo ter-load, kotak label 300x120 px). E2E label existing (`assets.spec.ts`)
>     tetap kompatibel — heading, "1 dipilih", tombol Cetak, dan endpoint tidak berubah.
> 83. **Next session — start here: Mobile M0 prep** — generate mockup mobile dari prompt kit
>     `docs/mobile/DESIGN_BRIEF.md` (13 artifact, hasil ke `docs/mobile/design/`) lalu spec + plan
>     fase M0 (scaffold Flutter `mobile/`, tema + i18n, navigasi shell, login/refresh/logout
>     cookie-jar, CI analyze/test/APK; scaffold menyertakan folder `ios/` per aturan IOS.md).
>     Kandidat lain yang dibawa: room/floor import targets; Analytics/OLAP read layer; `/auth/me`
>     `role_name`; admin reset-password audit action (migration); pre-existing e2e failures
>     (`account` change-password modal, `maintenance` date-boundary); GeoIP DB provisioning (ops);
>     notification follow-ups (SSE, retention archival, maker-route gap); sapuan simbol section di
>     komentar kode backend (task chip terpisah). Confirm priority before starting.
> 84. **Keputusan scope (planning, 2026-07-21, branch `feat/mobile-scope-expansion`)** — Pemilik
>     produk memperluas scope mobile v1 dengan **enam kemampuan baru (FR-M7)**: (a) **katalog aset**
>     browse read-only (bukan hanya scan-to-detail); (b) **peminjaman/check-out/check-in dari detail
>     aset** per permission x status: Manager check-out langsung (aset available, FR-3.1) + check-in
>     (aset assigned, FR-3.2) + Staf ajukan peminjaman via maker-checker (FR-3.3); (c) **lapor
>     kerusakan/maintenance** dari detail aset (pengajuan ringan);
>     (d) **registrasi aset** form penuh; (e) **Pengajuan saya** — lensa maker atas pengajuan yang
>     dibuat sendiri (`/requests?requested_by=diri`, filter status) + batal pengajuan `pending`
>     sendiri, terpisah dari inbox checker FR-M3.1; (f) **Aset saya** — aset yang dipegang pengguna
>     (`/assignments/mine`). **Mutasi + penghapusan/disposal tetap di web** (form berat,
>     keputusan meja). Alasan: keenamnya ringan/self-scoped dan ter-anchor ke alur scan/aksi yang mobile sudah punya
>     (Detail Aset ada sejak M1), beda kelas dari mutasi/disposal. Endpoint sudah ada (dipakai web) —
>     tak ada backend baru; pengajuan tetap lewat maker-checker/SoD server. Dokumen diperbarui: **PRD
>     mobile v1.1** (`docs/mobile/PRD.md` — non-goal bagian 1.3 direvisi, FR-M2.2 jadi actionable,
>     FR-M7 baru, changelog), **roadmap** (fase M7 katalog + aksi aset, prasyarat M1; daftar layar +
>     non-scope disesuaikan), **keputusan produk vault** (`Keputusan/Produk/Mobile v1 Tambah Katalog
>     Registrasi Maintenance Peminjaman.md` + indeks). **Spec + plan + prompt mockup sudah ditulis**
>     (2026-07-21): `docs/superpowers/specs/2026-07-21-mobile-m7-asset-actions.md`,
>     `.../plans/2026-07-21-mobile-m7-asset-actions.md`, prompt mockup DESIGN_BRIEF 5.13-5.18 + edit
>     5.2/5.4. Verifikasi kode menyingkap dua koreksi: registrasi memakai payload `AssetCreatePayload`
>     dengan `amount==purchase_cost`, dan **cek ambang kapitalisasi DIBUANG** (web tak punya, executor
>     hardcode Capitalized=true, fitur v1.1 belum ada). **Belum ada kode** — berikutnya: generate
>     mockup lalu implementasi fase M7 (8 irisan).
> 85. **Keputusan scope kedua (planning, 2026-07-21, branch `feat/mobile-scope-expansion`)** —
>     Perluasan **profil + keamanan akun** mobile (FR-M6/FR-M1.5), keputusan sejawat item 84:
>     (a) **profil lengkap** (metadata akun + detail pegawai read-only, `/auth/profile`+`/auth/me`);
>     (b) **ubah data diri + avatar** (`PUT /auth/profile`, `GET/POST/DELETE /auth/avatar`);
>     (c) **keamanan akun** ganti password/email **berbasis link email** (`/auth/password/change-request`,
>     `/auth/email/change-request` lalu konfirmasi via link) — mengikuti keputusan web "Keamanan Akun
>     via Email" + "Ganti Password Berbasis Link"; (d) **lupa password** dari login
>     (`/auth/password/forgot`, anti-enumerasi). **"Aset saya" ditetapkan sebagai menu tersendiri.**
>     Semua endpoint sudah ada & tidak di-deny untuk `aud=mobile` — **nol backend baru**; penetapan
>     password/email diselesaikan di halaman web (tanpa deep-link v1); ganti password mencabut semua
>     sesi (token-epoch) lalu klien logout ke Login. Dokumen: **PRD mobile v1.1** (FR-M6.1-M6.6 +
>     FR-M1.5), **roadmap** (fase M8 profil & keamanan, prasyarat M0; daftar layar), **keputusan
>     produk vault** (`Keputusan/Produk/Mobile v1 Profil Lengkap dan Keamanan Akun.md` + indeks).
>     **Spec + plan + prompt mockup sudah ditulis** (2026-07-21):
>     `docs/superpowers/specs/2026-07-21-mobile-m8-profile-security.md`,
>     `.../plans/2026-07-21-mobile-m8-profile-security.md`, prompt mockup DESIGN_BRIEF 5.19-5.20 +
>     edit 5.1/5.11. Verifikasi kode: `PUT /auth/profile` = {name, phone}. **Belum ada kode** —
>     berikutnya: generate mockup lalu implementasi fase M8 (6 irisan).
> 86. **Mobile M7 — mockup 12 layar di-generate + Task M7-1 (Katalog Aset) LANDED**
>     (branch `feat/mobile-scope-expansion`). Mockup FR-M7/M8 lengkap di `docs/mobile/design/`
>     (commit 6fab887). **Task M7-1 Katalog Aset** (`mobile/lib/features/catalog/`): `GET /assets`
>     dengan pencarian (debounce) + filter Kategori/Status/Kantor (picker bottom sheet; Kategori/Kantor
>     via `GET /categories`/`GET /offices`, "Tidak ada data" empty state) + paginasi infinite-scroll +
>     pull-to-refresh; kartu aset (nama/kode/chip status/kantor via reference lookup non-fatal);
>     empty/loading/error/403 state; navigasi ke Detail Aset; rute `/catalog`. Nol backend baru.
>     Tes: 6 unit repository + 10 widget + 4 widget filter + golden light/dark; `flutter analyze`
>     bersih. **Task M7-2 (Aset Saya) LANDED** (`mobile/lib/features/my_assets/`): `GET
>     /assignments/mine?status=active` (flat `{data:[...]}`, enum status active/returned), menu
>     tersendiri (rute `/my-assets`), kartu aset dipegang (nama/kode/chip Dipinjam/dipinjam sejak/
>     jatuh tempo) dengan penanda **Terlambat** (due_date < hari ini via clockProvider);
>     empty/loading/error/403 state; tap ke Detail Aset. Nol backend baru. Tes: 4 unit repository +
>     6 widget + golden light/dark; suite mobile 444 lulus. **Task M7-3 (Pengajuan Saya) — list +
>     filter + Batalkan LANDED** (`mobile/lib/features/my_requests/`): lensa maker `GET
>     /requests?mine=true` (server filter by JWT user, bypass office scope) + filter status +
>     Batalkan pengajuan `pending` sendiri (`POST /requests/:id/cancel`, ConfirmDialog destruktif,
>     reload). Reuse RequestDto/RequestListDto/ApprovalStatusFilter + request_presentation. Rute
>     `/my-requests`. Nol backend baru. Tes: 6 unit repository + 9 widget + golden; suite mobile 461
>     lulus. **Detail read-only LANDED** (keputusan pemilik produk: opsi bypass-maker). Backend
>     `fix(security)`: `GET /requests/:id` kini memperbolehkan **maker melihat pengajuannya sendiri
>     lepas dari office scope** (paritas bypass `mine=true` list; non-maker tetap scope-gated 403);
>     field-permission masking tetap berlaku. Handler + test integrasi (maker out-of-scope lihat own
>     lalu 200) + OpenAPI diperbarui. Mobile: rute `/my-requests/:id` reuse ApprovalDetailScreen
>     (maker-mode read-only, banner SoD, tanpa approve/reject); kartu Pengajuan Saya kini tappable.
>     Tes mobile +1 nav; suite mobile 462 lulus; backend vet/unit/spectral hijau (integrasi via CI —
>     testcontainers tak jalan di Windows lokal). **Task M7-4 (Detail Aset bar aksi + Peminjaman)
>     LANDED**: **infra permissions baru** (`core/authz/permissions_provider.dart` — `GET
>     /auth/permissions` -> Set, autoDispose anti bocor lintas sesi; sebelumnya mobile tak punya);
>     `assetActionsFor(perms, status)` (matriks FR-M7.2/3: available+manage->Check-out /
>     available+create->Pinjam / assigned+manage->Check-in / any+create->Lapor Kerusakan);
>     `AssetActionBar` sticky di kaki Detail Aset (bottomNavigationBar), render aksi yang SUDAH
>     terpasang (kini: Peminjaman/Pinjam) via `_implementedActions` (bertambah tiap fase); sheet
>     **Ajukan Peminjaman** (`POST /assignments/borrow`, jatuh tempo opsional + catatan -> approval).
>     Nol backend baru. Wiring permissionsProvider ke Detail Aset memaksa stub di 3 konsumen tes
>     (screen/router/golden) — pelajaran "wiring composable breaks consumer tests". Golden Detail
>     Aset kini menampilkan bar Pinjam. Tes: 8 unit matriks + 3 permissions repo + 3 borrow repo + 4
>     widget bar/sheet; suite mobile 480 lulus. **Task M7-5 (Check-out + Check-in) LANDED**: bar aksi
>     kini menyalakan Check-out (Manager, aset available) & Check-in (Manager, aset assigned).
>     `AssetActionRepository` diperluas: `checkout` (`POST /assignments` + picker custodian via
>     `GET /employees?search=`), `activeAssignment` (`GET /assets/:id/assignments` -> pilih
>     `status=active` untuk id + pemegang), `checkin` (`POST /assignments/:id/checkin`, kondisi masuk
>     Baik/Perlu Servis -> needs_maintenance). Sheet Check-out (autocomplete pegawai + tanggal + jatuh
>     tempo + kondisi) & Check-in (resolusi penugasan aktif + chips kondisi). Nol backend baru. Tes:
>     +5 repo (checkout/checkin/activeAssignment/searchEmployees) + 4 widget (Manager checkout/checkin
>     button, alur checkout/validasi, alur checkin); suite mobile 489 lulus. Catatan: golden khusus
>     sheet Check-out/Check-in belum dibuat (perilaku tercakup widget test). **Task M7-6 (Lapor
>     Kerusakan) LANDED** — bar aksi Detail Aset kini LENGKAP (4 aksi). `AssetActionRepository`:
>     `problemCategories` (`GET /problem-categories`) + `reportDamage` (`POST /maintenance/reports`
>     multipart FormData: asset_id + problem_category_id wajib + description opsional). Sheet Lapor
>     Kerusakan (dropdown kategori masalah wajib + deskripsi). **Foto DITUNDA** (opsional per kontrak;
>     butuh dependensi image_picker — perlu konfirmasi). Bar tombol jadi teks-only + ellipsis (bisa 2
>     tombol, mis. Pinjam+Lapor Kerusakan); golden Detail Aset diregenerasi (2 tombol). Nol backend
>     baru. Tes: +2 repo + 2 widget; suite mobile 493 lulus. **Cluster M7-4/5/6 (Detail Aset) SELESAI.**
>     **Task M7-7 (Registrasi Aset) LANDED** (`mobile/lib/features/asset_register/`): form Stepper 3
>     langkah (Identitas: nama+kategori+kelas aset tangible/intangible+seri; Penempatan: kantor+harga
>     numerik-only+tanggal+catatan; Tinjau lalu kirim) -> `POST /requests` type `asset_create` dengan
>     `AssetCreatePayload` + **`amount == purchase_cost`** ('0' bila kosong). Reuse picker
>     kategori/kantor katalog; harga numerik-only via inputFormatters; sukses -> ke Pengajuan Saya.
>     **Tanpa cek ambang kapitalisasi** (sesuai temuan). Field referensi opsional
>     (brand/model/unit/vendor/ruangan) ditunda; golden layar registrasi belum dibuat. Rute
>     `/register-asset` (entry Katalog/Beranda = M7-8). Nol backend baru. Tes: 3 repo (payload +
>     amount==cost + tanpa harga) + 3 widget (alur lengkap, validasi nama, numerik-only); suite mobile
>     499 lulus. **Task M7-8 (titik masuk Beranda) LANDED** — `_QuickActions` jadi 2 baris (8 aksi):
>     4 lama (Scan/Opname/Approval/Notifikasi) + 4 FR-M7 (Katalog `/catalog`, Aset Saya `/my-assets`,
>     Pengajuan Saya `/my-requests`, Registrasi `/register-asset`). Golden Beranda diregenerasi; tes
>     label diperluas ke 8. Nol backend baru; suite mobile 499.
>     **>> FASE M7 (katalog + aksi aset) TUNTAS <<** (M7-1..M7-8). Deferral tercatat (sengaja, perlu
>     keputusan): (a) **foto** Lapor Kerusakan + **avatar** M8 -> butuh dependensi `image_picker`
>     (satu keputusan, dua tempat); (b) **golden** sheet check-out/check-in/lapor + layar registrasi;
>     (c) **field referensi opsional** registrasi (brand/model/unit/vendor/ruangan). Backend
>     `fix(security)` maker-view GET /requests/:id (M7-3) menunggu verifikasi CI (integrasi tak jalan
>     di Windows lokal). **Fase M8 dimulai. Task M8-1 (Profil lengkap) LANDED**
>     (`mobile/lib/features/account/`): `AccountRepository.getProfile` (`GET /auth/profile` ->
>     `ProfileDto` plain) + `accountProfileProvider`; layar Profil ditambah kartu **Detail Pegawai**
>     (kode/status/departemen/jabatan, atau catatan bila tak tertaut pegawai) + **Informasi Akun**
>     (email/telepon/metode login/tanggal bergabung) — additive di atas kartu identitas + sesi yang
>     ada. `FakeAccountRepository` diberi `getProfile`; golden Profil diregenerasi. Nol backend baru.
>     Tes: 2 repo (parse, offline) + 2 widget (kartu terisi, akun tanpa pegawai); suite mobile 503.
>     **Task M8-2 (ubah data diri) LANDED**: `AccountRepository.updateProfile` (`PUT /auth/profile`
>     {name, phone}); kartu **Data Diri** editable (mode Ubah -> Simpan/Batal; validasi nama wajib;
>     sukses -> invalidate `accountProfileProvider` + SnackBar). `FakeAccountRepository.updateProfile`
>     + `ProfileDto.copyWith`; golden Profil regen; viewport tes profil dinaikkan. Catatan: nama di
>     header identitas (dari sesi auth) baru segar setelah re-login (Data Diri + provider profil
>     langsung segar). Nol backend baru. Tes: +2 repo (update, offline) + 3 widget (Ubah->Simpan,
>     validasi nama, Batal); suite mobile 506. **Task M8-3 (avatar) LANDED** (keputusan pemilik
>     produk: pakai **image_picker** — dependensi baru, juga untuk foto Lapor Kerusakan M7-6):
>     `AccountRepository.uploadAvatar` (`POST /auth/avatar` multipart field `file`) + `deleteAvatar`
>     (`DELETE`); `accountAvatarProvider` tak lagi digate `has_avatar` sesi (langsung GET, segar
>     pasca-unggah/hapus); `_EditableAvatar` (badge kamera -> sheet Galeri/Kamera/Hapus, overlay
>     busy) menggantikan avatar statis di kartu identitas. Izin CAMERA sudah ada (mobile_scanner).
>     Golden Profil regen. Nol backend baru. Tes: +3 repo (upload multipart, delete, offline) + 2
>     widget (Hapus flow, opsi Hapus tersembunyi tanpa foto); suite mobile 511. **Foto Lapor Kerusakan
>     (M7-6) DI-BACKFILL** dengan image_picker: `reportDamage` menerima `photoBytes` opsional (field
>     multipart `photo`); sheet Lapor Kerusakan dapat tombol "Tambah foto" (galeri) + thumbnail +
>     hapus. Tes: +1 repo (FormData photo) + asersi tombol; suite mobile 512. Deferral foto M7-6
>     TERTUTUP. **Task M8-4 (Keamanan Akun) LANDED**: layar Keamanan Akun (rute `/account-security`,
>     tautan dari Profil) + `AccountSecurityRepository` (`POST /auth/password/change-request`
>     {current_password}; `POST /auth/email/change-request` {new_email, current_password}) — keduanya
>     **berbasis link email** (klien memulai, penetapan di web). Sheet ganti password (verifikasi
>     password lama + peringatan cabut-sesi) & ganti email, masing-masing state "Cek email Anda"
>     setelah sukses. **Password salah -> 400 ValidationFailure inline (bukan 401 -> tak auto-logout)**;
>     email dipakai -> 409 inline. Golden Profil regen. Nol backend baru. Tes: 4 repo + 5 widget; suite
>     mobile 521. **Task M8-5 (Lupa Password) LANDED**: `AuthRepository.forgotPassword`
>     (`POST /auth/password/forgot` {email}, anti-enumerasi — server SELALU 200); `ForgotPasswordScreen`
>     (rute publik `/forgot-password`, dikecualikan guard auth) input email lalu state konfirmasi
>     "Cek kotak masuk Anda" dengan pesan IDENTIK apa pun status akun (penetapan password baru via link
>     email di web); tautan "Lupa password?" ditambah di layar Login. Golden Login regen. Nol backend
>     baru. Tes: +2 repo (body email di-trim, offline) + 4 widget (validasi kosong, sukses, anti-enumerasi
>     pesan identik, gagal jaringan inline). **Task M8-6 (verifikasi logout pasca-ganti-password)
>     TERVERIFIKASI tanpa kode baru**: `test/core/api/auth_interceptor_test.dart` sudah menegaskan
>     `sessionExpiredCalls == 1` saat refresh 401 definitif (skenario sesi dicabut setelah ganti
>     password = logout bersih ke Login). Suite mobile 527. **Fase M8 SELESAI** (M8-1..M8-6).
>     **Perluasan scope mobile v1 (M7 + M8) TUNTAS di branch `feat/mobile-scope-expansion`.**
>     **Review berlapis (code-reviewer + security-auditor + test-engineer) DIJALANKAN + remediasi
>     must-fix LANDED.** Verdict: 0 Critical; code-reviewer APPROVE-dgn-catatan, security 0 High/Med
>     (maker-bypass GET /requests/:id dinyatakan benar & bukan IDOR, anti-enumerasi Lupa Password
>     konsisten). Diperbaiki: **(A)** harga registrasi jadi `digitsOnly` (titik ribuan dulu bikin
>     purchase_cost/amount malformed); **(B)** `catalogOffice/CategoryOptionsProvider` jadi `autoDispose`
>     (cegah daftar kantor basi lintas-user di perangkat sama); **(C, fix(security))** `uuid.Parse` caller
>     ID gagal kini = non-maker (defense-in-depth) + integration test assert body maker; **(D)** sheet
>     check-in bedakan gagal-muat vs tak-ada-penugasan (dulu error jaringan di-mask). Tes +14 (error
>     submit ke-4 sheet, no-active vs load-error, toggle maintenance, FilterOptionsRepository + guard
>     autoDispose, titik ribuan). Suite mobile 541. **SELURUH FOLLOW-UP REVIEW DITUTUP** (permintaan
>     pemilik "kerjakan semua followup"): **(1) UX Minor** — helper `actionFailureMessage`
>     (NetworkFailure/Forbidden/Conflict -> pesan spesifik `commonError*`, sisanya fallback aksi) dipakai
>     di 4 sheet aksi + registrasi; validasi format email klien di ganti-email (email salah-format dulu
>     tampil "password lama salah"). **(2) Backend foto** — `asset.UploadAttachment` dapat flag
>     `Normalize`: foto (mobile field) di-downscale ke maks 2048px + re-encode JPEG q85 (strip EXIF/GPS,
>     hemat storage, kualitas terjaga); diaktifkan di `POST /maintenance/reports` (dua lapis dgn cap
>     1600px klien). PDF/attachment web lain tak berubah. **(3) Kontrak** — tes handler unit menegaskan
>     `/auth/password/forgot` selalu 200 identik untuk email dikenal vs tak-dikenal (anti-enumerasi),
>     malformed tetap 400. **(4) Backfill tes Medium** — catalog nama param HTTP, my_requests
>     loadMore(sukses/gagal/no-op)+cancel-error, asset_register intangible+harga konkret, account_security
>     NetworkFailure kedua form. Suite mobile 553; backend +normalizeImage & forgot-contract tests (build/
>     vet/test hijau). Berikutnya: buka PR
>     merge ke `main`; fase mobile lanjutan (M3/M5) menyusul.
> 87. **Seed dev realistis + non-duplikat + suite e2e Lampiran A (API-driven, ketat)**
>     (branch `feat/seed-lampiran-a-e2e`). **Seed** (`backend/db/seed/seed_demo.sql`) ditulis ulang:
>     reset menyeluruh menyisakan HANYA superadmin kanonik `admin@inventra.local` (buang superadmin
>     bootstrap sisa e2e + role custom `e2e_*`); **normalisasi authz role sistem** deterministik
>     (e2e menyisakan baris `data_scope_policies` soft-deleted/duplikat -> superadmin jatuh ke 'own',
>     tak melihat data — dibangun ulang persis kondisi migrasi); **identitas pegawai/user UNIK global**
>     (indeks deterministik pool 64x32, bukan hash acak — 0 nama duplikat lintas kantor, assert di
>     seed); **coverage role per tier** (tiap cabang: kepala_unit+manager+staf; tiap wilayah:
>     kepala_kanwil; pusat: role custom baru **`pejabat_pusat`** = approver tier `pusat` +
>     delegasi `depreciation.manage`); ~300 aset/kantor (~13rb) + **lapisan transaksional** konsisten
>     status aset (assignment/maintenance/transfer/disposal + approval history & inbox pending +
>     periode depresiasi + notifikasi + audit). **WAJIB setelah seed: `redis-cli FLUSHALL`** (cache
>     authz Redis by role_id — seed SQL langsung tak menginvalidasinya). **E2E** (`frontend/e2e/
>     lampiran-a-*.spec.ts` + `lampiran-helpers.ts`): 9 file, 46 tes, **API-driven multi-user** (tiap
>     aktor konteks API sendiri), assert **status code persis** (403/422/409/400) untuk seluruh jalur
>     boleh/tidak-boleh Skenario 1-7 + A.7 opname — rantai berjenjang office->wilayah->pusat dengan
>     approver berbeda per tier (pertama di repo). Cast di-resolve dari API by (kantor, role), bukan
>     email hardcoded. Semua hijau; `pnpm lint`/`typecheck` bersih.
>     **Code review (agent code-reviewer) dijalankan; temuan penting diperbaiki:** (1) seed sempat
>     `DELETE` semua `field_permissions` -> membocorkan `purchase_cost`/`book_value`/
>     `accumulated_depreciation` ke Staf/Kepala; kini dibangun ulang kanonik (mirror migrasi 000016) —
>     diverifikasi Staf tak lihat kolom finansial. (2) Reset seed tak re-runnable setelah e2e opname
>     (FK `stock_opname_items.followup_record_id` -> `maintenance_records`); urutan DELETE diperbaiki.
>     (3) Pending `asset_create` seed dilengkapi `category_id` agar BENAR-BENAR bisa di-approve (bukan
>     landmine); pending `assignment` tak-executable dibuang. (4) Assertion longgar dipersempit ke kode
>     persis (maintenance dup 409, import confirm-failed 409 + cancel-non-owner 403, opname out-of-scope
>     403); uji tolak tier pusat kini pakai aktor ber-scope wilayah; gate keunikan nama dipindah ke
>     DALAM transaksi. **Review ulang (pasca-merge `main` bump-deps #113): APPROVE** — ke-7 perbaikan
>     dikonfirmasi benar; ditambah `lampiran-a-field-masking.spec.ts` (guard regresi otomatis: Staf
>     tak lihat `purchase_cost`/`book_value`/`accumulated_depreciation`, Manager lihat 2 pertama saja,
>     Superadmin lihat semua) + assertion positif self-borrow tertaut pegawai Staf (mine dari JWT).
>     **Kebijakan field-permission finansial dirapikan (migrasi `000037`):** dulu Manager lihat
>     `purchase_cost`+`book_value` tapi `accumulated_depreciation` masked — padahal accumulated =
>     purchase_cost - book_value - impairment (bisa diturunkan), jadi maskingnya bocor & tak konsisten.
>     Kini ketiga kolom = SATU tier: view untuk Superadmin + Manager + Pejabat Kantor Pusat, masked
>     untuk Kepala Unit/Kanwil + Staf (migrasi 000037 up/down teruji; seed & guard e2e diselaraskan;
>     46 tes hijau). **Temuan sisa (dicatat, bukan bug e2e):** alur transfer app tak pernah menyetel
>     status aset `in_transfer` (aset tetap `available` selama transit; guard dobel-mutasi lewat baris
>     transfer terbuka).

## ✅ Done

### Foundation & DevOps
- [x] Project scaffold (Go/Gin backend, Nuxt 4 frontend)
- [x] `docker compose up` full stack (Postgres + Redis + MinIO + migrate + backend + frontend)
- [x] GitHub Actions CI (backend build/vet/test · frontend lint/typecheck/build · Spectral)
- [x] CI per aplikasi via path filter (`dorny/paths-filter`): job hanya jalan di PR yang
      menyentuh path terkait (backend/frontend/mobile/api-docs/e2e); push ke main tetap
      full run sehingga pemicu Deploy tidak berubah; job skipped lolos required checks — PR #111
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
      `docs/DEPLOYMENT.md` bagian 15 IaC sub-section. **Done (2026-07-06).**
- [x] **Ops hardening Phase 3 — Monitoring/observability** ✅ self-hosted stack as
      a toggleable compose overlay (`docker-compose.monitoring.yml`): backend RED
      metrics (`/metrics`, internal-only), Prometheus (15d retention + `mem_limit`)
      + exporters (node/cAdvisor/postgres/redis/blackbox), Alertmanager → Telegram,
      Loki+Promtail (log), Grafana (datasource+dashboard as-code) — only Grafana
      public (own subdomain); secrets via `*.example` + gitignore
      (`alertmanager.yml`, `grafana.env`). Ansible `monitoring` role
      (`ops/ansible/roles/monitoring/`) brings the overlay up idempotently via
      `community.docker.docker_compose_v2`, appended after `app` in `site.yml`.
      ADR-0011 + `docs/DEPLOYMENT.md` bagian 16. **Done (2026-07-06). Ops-hardening
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
> **Six new v1.1 bank-grade mockups added 2026-07-03** (DESIGN_BRIEF bagian 6: `Mutasi Aset`,
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
      **Done — (2026-06-28).** Added lightweight `GET /requests/inbox/count` (gate `request.decide`,
      shares `Service.Inbox` — no per-row enrichment/field-filter) to unblock a real sidebar approval
      badge count (frontend wiring still pending — see item 51(c)). Integration test asserts
      `count == len(GET /requests/inbox.data)` for the same caller and 403 for a caller without
      `request.decide`. OpenAPI documented; Spectral 0 errors. **Done (2026-07-13, Tech-Debt Sweep #2
      Task 8).**
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
  - [x] **User Management** (`/settings/users`) ✅ wired to `/api/v1/users` — CRUD (GET list with server-side search+pagination, POST create, PUT update, DELETE remove); gate `user.manage`; role/office/employee pickers from real API lookups; employee picker filtered by selected office (office_id-aware `employeeFormOptions`); e2e spec against real seeded backend; status toggled via update endpoint. **Done (2026-06-29). Authz/settings screen wiring batch complete (RBAC + Data Scope + Field Permission + Audit Trail + User Management).** Filter bar now has server-side role/office/status filter controls (role `USelect`, office `AsyncSearchPicker`, status `USelect`, reset button matching the mockup) driving `GET /users?role_id&office_id&status`; `useUsers().list()` extended; 12-case component spec (`users-filters.spec.ts`); verified live against the real backend. **Done (2026-07-13, Tech-Debt Sweep #2 Task 7).** ⚠️ TODO: ~~reset-password action still dropped pending backend support~~ ✅ **done (item 70, 2026-07-18)** — Reset Password row action → `POST /users/:id/reset-password` emails the target user a reset link; office/employee lookup capped at 100 (searchable async picker is a follow-up if counts grow); `mock/users.ts` retained until `useGlobalSearch` is wired to the real `/search` endpoint; no dedicated e2e assertion added yet for the new filter controls (component-test only).
- [x] **Peta Lokasi** (`/master/map`) ✅ wired to `GET /api/v1/offices/map` — office lat/lng columns + geo endpoint with resolved type/province/city names + per-office asset count; data-scoped. `useOfficeMap` rewritten (real `$fetch`); types `MapOffice`/`OfficeTier`; 3-tier legend (Pusat/Wilayah/Cabang; Outlet folded into Cabang — `office_types.tier` not yet editable); coord-filtered Leaflet pins; load-error/retry; e2e spec added; orphaned `mock/officeMap.ts` deleted. **Done (2026-06-29).** ⚠️ TODO: map shows empty-state until offices have coordinates (no production seed); asset count real but 0 until asset module populated. (`office_types.tier` now editable via Referensi screen — resolved as part of bagian Referensi wiring below.)
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

### Mobile companion (PRD v1.2 — planned; docs landed 2026-07-18, no code yet)

> Scope dibuka pemilik produk 2026-07-18 (dulu non-goal v1.1). Bentuk: **field companion** Flutter
> (Android dulu, folder `mobile/`). **Dokumentasi mobile dipisah di `docs/mobile/`**: PRD mobile
> (`PRD.md`), ADR (`adr/0015` Flutter + `adr/0016` offline sync — penomoran global), design brief +
> prompt kit (`DESIGN_BRIEF.md`), arsitektur klien (`ARCHITECTURE.md`), konvensi kode
> (`CONVENTIONS.md`), dan indeks (`README.md`). Rencana penuh: `docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`.
> Konvensi design-fidelity berlaku — mockup mobile (`docs/mobile/design/`) dibuat **sebelum** layar
> dibangun; tiap fase dapat spec + plan.

- [x] **Mockup mobile** — 12 layar + component library di `docs/mobile/design/` (login, home, scan, detail aset, sesi opname, counting, variance, inbox approval, detail approval, notifikasi, profil/sesi, pengaturan). ✅ 2026-07-19.
- [x] **ADR-0017 identitas klien** — backend bersama (tanpa BFF), klaim `aud` web/mobile + `RequireAudience`, jalur refresh per-klien (men-supersede cookie jar ADR-0015), kontrak response endpoint shared. ✅ 2026-07-19, PR #105.
- [x] **M0+M1+M2+M4 — Mobile v1 (12 layar 1:1 fungsional, satu PR)** — branch `feat/mobile-m0`. **Backend auth per-klien ADR-0017**: klaim `aud`, login/refresh/logout per-klien (mobile refresh di body + `flutter_secure_storage`, web cookie tak berubah), `SessionAlive` di refresh, `RequireAudience` deny `aud=mobile` pada authzadmin + importer + **ekspor laporan**. **Flutter `mobile/`**: scaffold feature-first, tema token M3 light+dark + `InventraStatusColors`, i18n ARB id/en, Dio interceptor (single-flight refresh, error->AppFailure), 12 layar 1:1 mockup dengan golden light+dark tiap layar, 410 test (unit+widget+golden). CI job mobile (analyze/test/build APK debug). Review berlapis (code-reviewer APPROVE + security-auditor + test-engineer) — temuan HIGH (ekspor laporan) + Major (izin INTERNET, N+1) + bug lintas-user + redaksi token semua ditutup. **Deviasi mockup disetujui pemilik produk (catat-deviasi convention; semua dipaksa kontrak API atau scope fase) — daftar lengkap per layar:** ✅ 2026-07-19.
  - **Lintas-layar:** warna dark mode ikut token tema (bukan piksel mockup, aturan CONVENTIONS "warna dari tema"); golden test di-exclude dari CI (rendering font platform-dependent, gate lokal); nilai referensi di-resolve jadi nama via lookup non-fatal (get-by-id, cache 5 mnt — UUID tak pernah tampil ke pengguna); nama peran tidak dirender di mana pun (Beranda/Profil) karena endpoint roles menolak `aud=mobile` (ADR-0017) — kandidat `role_name` di `/auth/me`.
  - **Login/Shell:** FAB Pindai center-docked, overlap ~28px (mockup 32px) demi hit-test penuh; gradient latar login aproksimasi `lerp` token (mockup green-200 bukan token); versi footer dari pubspec (mockup dummy v1.4.2).
  - **Scan/Detail Aset:** baris "Kondisi" mockup → "No. seri" (kondisi tak ada di kontrak, klien tak menebak); foto/carousel → placeholder "Belum ada foto" (belum ada API foto aset); tag/kode pakai Inter (mockup JetBrains Mono, tak di-bundle); sheet sukses/error scan dilewati (deteksi/submit langsung ke detail); garis scan statis (deterministik tes).
  - **Approval Inbox/Detail:** judul kartu pakai `reason` (tak ada field judul di kontrak); seksi lampiran dilewati (tak ada API attachment requests); SnackBar sukses tanpa "Urungkan" (tak ada API undo); "Menunggu keputusan" tanpa kata "Anda" (kontrak tak beri field eligibility pemanggil); jenis Peminjaman indigo → keluarga `info` (tema tanpa indigo); chip status kartu pakai `StatusChip` (titik+label) bukan pill ber-ikon (konsistensi komponen inti); ruangan asal mutasi dilewati (payload hanya tujuan); grid nilai + "estimasi rugi" disposal → baris data (klien tak hitung rugi); `unit_id` tak di-resolve (tak ada lookup unit); badge tab Approval tersembunyi tanpa izin/offline (non-fatal).
  - **Opname (Daftar/Counting/Variance):** **online-only** — elemen offline (SyncPill/OfflineBanner) dirender sesuai mockup tetapi scan dinonaktifkan saat offline; label "snapshot tersimpan" + "N belum tersinkron" + antre per-baris + state konflik multi-device TIDAK diimplementasikan (drift/antrean = M5); KPI progress kartu daftar di-fetch detail per sesi paralel (kontrak list tak beri KPI); filter "Berjalan" disaring klien (kontrak 1 status/query); tombol tindak-lanjut variance tetap di web (mobile hanya tampilkan status dari `followup_*_id`); endpoint PATCH item ke-5 ditambah (sheet hasil scan tak fungsional tanpanya); entri ke Variance via ikon app-bar counting (mockup tanpa entri eksplisit); judul kartu = `name`/kantor·bulan (mockup lantai/rentang tanggal tak di kontrak).
  - **Beranda/Notifikasi:** subjudul header = nama kantor (fallback email), bukan "peran · kantor"; kartu "Notifikasi terbaru" tak ditambah (mockup hanya lonceng berbadge + quick action — diikuti 1:1); "N di antaranya > 3 hari" dihitung dari halaman pertama pending (limit 20, tanpa endpoint agregat); kartu contoh "Sinkronisasi selesai" tak dirender (bukan type kontrak notifikasi, sync = M5).
  - **Profil/Pengaturan:** judul sesi "OS · Browser" bukan model perangkat "Pixel 8" (kontrak `SessionView` tak beri model); kartu Pengaturan push/penyimpanan/bantuan dilewati (M3/M5/M6); pemilih bahasa pakai bottom sheet (mockup hanya baris) — langsung terapkan tanpa tombol; identitas tak di-skeleton (data sinkron dari sesi auth); dialog "keluar semua perangkat lain" pakai jumlah sesi, bukan enumerasi nama.
  - **Penanda sementara (bukan deviasi permanen, sudah bersih):** logout sempat di app bar Beranda placeholder selama Task 7-11, dipindah ke layar Profil (Task 12) sebelum PR — tak ada sisa.
- [ ] **M3 Push notification** — backend: tabel `device_tokens` + endpoint register + dispatcher FCM sebagai consumer tambahan pipeline ADR-0014 (env FCM wajib masuk `docker-compose.prod.yml`); mobile: deep-link (layar Notifikasi sudah ada dari v1)
- [ ] **M5 Opname offline-first** — drift snapshot + antrean lokal; backend: endpoint batch idempoten `client_scan_id` + first-write-wins + laporan konflik (ADR-0016); indikator status sync (elemen offline/SyncPill sudah dirender dari v1, tinggal difungsikan). Fondasi `connectivity_plus` (`isOnlineProvider`) sudah ada.
- [ ] **M6 Rilis internal** — icon/splash/signing, Firebase App Distribution (Play internal track menyusul), Crashlytics/Sentry, runbook rilis di vault
- [ ] **(Pasca-M6, opsional) Aktivasi iOS** — prasyarat, setup APNs/TestFlight, dan checklist QA lengkap di `docs/mobile/IOS.md`; menunggu keputusan produk (QM4) + akses macOS/Apple Developer (QM3)

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
