# Authorization Admin (roles / permissions / scope / fields) — Design

| | |
|---|---|
| **Tanggal** | 2026-06-28 |
| **Modul** | `internal/authzadmin` (baru) + 2 method di `internal/authz` |
| **Schema** | `identity.roles`, `identity.role_permissions`, `identity.data_scope_policies`, `identity.field_permissions` (sudah ada) |
| **Status** | Disetujui — siap implementasi |

## 1. Konteks & tujuan

Mesin otorisasi 3-lapis (`internal/authz`) **sudah lengkap di sisi penegakan** (runtime): `PermissionService` (RBAC), `ScopeService` (data scope), `FieldService` (field permission), semua Redis-cached, dipakai via `middleware.RequirePermission` + `CallerOfficeScope` + `FilterView`. Yang **belum ada** adalah sisi **pengelolaan/konfigurasi via API**: Superadmin tidak bisa CRUD peran, role_permissions, data_scope_policies, atau field_permissions lewat aplikasi — saat ini hanya bisa lewat SQL manual/seed. PROGRESS.md menandainya `[ ]` ("Authorization admin endpoints … + Redis cache invalidation").

Tujuan: bangun modul admin Superadmin untuk keempat tabel itu, dengan **invalidasi cache** yang benar, **katalog permission kanonik** sebagai sumber kebenaran, dan **perbaikan drift seed** agar peran bawaan benar-benar berfungsi.

### Temuan yang melatari (diverifikasi dari kode)

- **Tidak ada bypass superadmin**: `RequirePermission` murni `Has(roleID, key)`.
- **Drift key**: kode menegakkan `asset.view`/`asset.manage`, `request.decide`, `approval.config.manage`; tetapi seed `000005` memberi `asset.read/create/update/delete/checkout`, `request.approve`, dan tak punya `approval.config.manage`. Akibatnya, dengan seed produksi **tak ada peran** yang punya `asset.view`/`asset.manage` → endpoint aset tertutup untuk semua (integration test lolos karena seed izin ad-hoc). Bug laten ini diperbaiki di sini (bagian 4).
- Permission key yang **benar-benar ditegakkan** saat ini (grep `RequirePermission`): `approval.config.manage`, `asset.manage`, `asset.view`, `audit.view`, `masterdata.global.manage`, `masterdata.office.manage`, `request.create`, `request.decide`, `user.manage`. Modul ini menambah penegakan `role.manage`/`scope.manage`/`fieldperm.manage` (gerbang endpoint admin).

### Skema tabel (sudah ada)

- `roles(id, code, name, description?, is_system, ts, deleted_at)` — 5 baris `is_system=true` bawaan.
- `role_permissions(id, role_id, permission_key, ts, deleted_at)` — partial-unique `(role_id, permission_key) WHERE deleted_at IS NULL`.
- `data_scope_policies(id, role_id, module, scope_level, ts, deleted_at)` — partial-unique `(role_id, module) WHERE deleted_at IS NULL`; `module='*'` = default per-role.
- `field_permissions(id, entity, field, role_id, can_view, can_edit, ts, deleted_at)` — partial-unique `(entity, field, role_id) WHERE deleted_at IS NULL`.

### Cache key (untuk invalidasi)

- `authz:perms:{roleID}` — `PermissionService` (sudah punya `Invalidate`).
- `authz:scope:{roleID}` — `ScopeService.policies` (**belum** punya Invalidate → ditambah).
- `authz:fields:{roleID}` — `FieldService.forRole` (**belum** punya Invalidate → ditambah).
- `authz:subtree:{officeID}` — di-key per office, **bukan** konfigurasi peran → tidak terpengaruh perubahan policy peran (tak perlu di-invalidate di sini).

## 2. Penempatan & struktur

Package baru `internal/authzadmin/` (konvensi four-file):
- `service.go` — logika + sentinel error (`ErrNotFound`, `ErrConflict`, `ErrSystemRole`, `ErrRoleInUse`, `ErrUnknownPermission`, `ErrInvalidScope`, `ErrValidation`) + `mapDBError`; pegang `*sqlc.Queries`, `*pgxpool.Pool` (untuk tx replace-set), dan `*authz.PermissionService`/`*authz.ScopeService`/`*authz.FieldService` (untuk invalidasi).
- `dto.go` — request structs (`binding`) + serialisasi (`roleToMap`, dst.) + katalog (bagian 3).
- `handler.go` — `Handler` (bind→service→audit→respond; map sentinel→HTTP via `svcError`).
- `routes.go` — `RegisterRoutes(rg, h, authMW, requireRole, requireScope, requireField)` (gerbang per-endpoint).

`internal/authz`: tambah `ScopeService.Invalidate(ctx, roleID) error` (`Del authz:scope:{roleID}`) dan `FieldService.Invalidate(ctx, roleID) error` (`Del authz:fields:{roleID}`), meniru `PermissionService.Invalidate`.

Wiring `NewRouter`: konstruksi `authzadmin` service+handler; `authzadmin.RegisterRoutes(api, h, requireAuth, RequirePermission(permSvc,"role.manage"), RequirePermission(permSvc,"scope.manage"), RequirePermission(permSvc,"fieldperm.manage"))`.

## 3. Katalog permission kanonik

Konstanta Go di `dto.go` (atau `catalog.go`) — satu sumber kebenaran untuk key yang boleh di-assign:

```
Sistem:        user.manage, role.manage, scope.manage, fieldperm.manage, audit.view
Master Data:   masterdata.global.manage, masterdata.office.manage
Aset:          asset.view, asset.manage
Persetujuan:   request.create, request.decide, approval.config.manage
Cadangan:      report.view, report.export, maintenance.manage, depreciation.manage,
               valuation.exclude.approve, assignment.manage
```

Struktur: grup → `{key, label}`. Helper `IsKnownPermission(key) bool` untuk validasi. Grup "Cadangan" = key yang akan ditegakkan modul mendatang; tetap valid untuk di-assign agar seed forward-looking tak gugur validasi.

`GET /authz/catalog` → `{ permissions: [{group, items:[{key,label}]}], scope_levels: ["global","office_subtree","office","own"], scope_modules: ["*","offices","employees","floors","rooms","assets", …] }`. `scope_modules` diturunkan dari call-site `CallerOfficeScope` (dikonfirmasi saat implementasi via grep) + `'*'`.

## 4. Perbaikan drift seed (`000005_seed_identity.up.sql`, greenfield in-place)

Tulis ulang blok `role_permissions` agar **hanya** memakai key katalog dan mencerminkan key yang ditegakkan:
- `asset.read/create/update/delete/checkout` → `asset.view` (read) + `asset.manage` (write) sesuai kapabilitas peran (mis. Manager dapat `asset.view`+`asset.manage`; Kanwil/Unit/Staf dapat `asset.view`).
- `request.approve` → `request.decide`.
- Tambah `approval.config.manage` ke Superadmin.
- Pertahankan key cadangan yang sudah diberikan (report.*, maintenance.manage, depreciation.manage, valuation.exclude.approve) — semuanya ada di katalog.
- Superadmin = **seluruh** katalog (termasuk `role.manage`/`scope.manage`/`fieldperm.manage` yang sudah ada).
`.down.sql` disesuaikan bila perlu. Karena greenfield (DATABASE.md bagian 6), edit seed langsung; DB dev di-reset.

## 5. Endpoint (semua `authMW` + gerbang izin)

| Resource | Method + Path | Izin | Catatan |
|---|---|---|---|
| Katalog | `GET /authz/catalog` | role.manage | metadata untuk UI |
| Roles | `GET /authz/roles` | role.manage | list (pakai ulang `ListRoles`) |
| | `POST /authz/roles` | role.manage | buat peran kustom (`is_system=false`) → 201 |
| | `GET /authz/roles/:id` | role.manage | detail |
| | `PUT /authz/roles/:id` | role.manage | edit name/description (+ code bila bukan sistem) |
| | `DELETE /authz/roles/:id` | role.manage | 204; lihat aturan bagian 6 |
| Role permissions | `GET /authz/roles/:id/permissions` | role.manage | `{permissions:[key…]}` |
| | `PUT /authz/roles/:id/permissions` | role.manage | replace-set |
| Data scope | `GET /authz/roles/:id/scope` | scope.manage | `{policies:[{module,scope_level}…]}` |
| | `PUT /authz/roles/:id/scope` | scope.manage | replace-set |
| Field permissions | `GET /authz/roles/:id/fields` | fieldperm.manage | `{fields:[{entity,field,can_view,can_edit}…]}` |
| | `PUT /authz/roles/:id/fields` | fieldperm.manage | replace-set |

## 6. Aturan bisnis & data

- **Replace-set transaksional** (permissions/scope/fields): dalam 1 tx (`pool.Begin`) → soft-delete semua baris aktif peran (`SET deleted_at=now() WHERE role_id=$1 AND deleted_at IS NULL`), lalu insert set baru (id baru; partial-unique mengabaikan baris soft-deleted → tak konflik). Setelah commit → invalidasi cache service terkait untuk peran itu. Empty set diperbolehkan (mencabut semua).
- **Roles**:
  - Create: `code` (non-empty, unik; 23505→`ErrConflict`/409), `name` wajib, `is_system=false` selalu. `description` opsional.
  - Update: `name` wajib; `description` opsional; `code` boleh diubah **hanya** bila `is_system=false` (immutable untuk sistem).
  - Delete: `is_system=true` → `ErrSystemRole`/409; `CountUsersByRole>0` → `ErrRoleInUse`/409; selain itu soft-delete peran + cascade soft-delete role_permissions/scope/fields peran itu (dalam tx) + invalidasi ketiga cache.
- **Validasi**:
  - `permission_key` ∈ katalog (`IsKnownPermission`) → else `ErrUnknownPermission`/400.
  - `scope_level` ∈ {global,office_subtree,office,own}; `module` non-empty; dedupe per `module` (duplikat → 400).
  - field `entity`/`field` non-empty (free-form, default-allow); dedupe per `(entity,field)`.
- **Audit**: `audit.Record` setelah tiap mutasi sukses — entity `roles`/`role_permissions`/`data_scope_policies`/`field_permissions`, `office_id = nil` (konfigurasi global), `audit.Diff(before, after)`. Replace-set: before = state lama tergabung, after = state baru.
- **Cache invalidation** (wajib, inti fitur): permissions berubah → `permSvc.Invalidate(role)`; scope → `scopeSvc.Invalidate(role)`; fields → `fieldSvc.Invalidate(role)`; role delete → ketiganya.

## 7. Query sqlc baru (`db/queries/identity.sql`) + `sqlc generate`

- `GetRole :one` (by id, non-deleted).
- `CreateRole :one` (code, name, description, is_system=false).
- `UpdateRole :one` (name, description, code).
- `SoftDeleteRole :execrows`.
- `CountUsersByRole :one` (users non-deleted dengan role_id).
- `InsertRolePermission :one`; `SoftDeleteRolePermissionsByRole :execrows`.
- `InsertDataScopePolicy :one`; `SoftDeleteDataScopePoliciesByRole :execrows`.
- `InsertFieldPermission :one`; `SoftDeleteFieldPermissionsByRole :execrows`.
- `ListFieldPermissionsByRole` sudah ada; `ListRolePermissions`/`ListDataScopePolicies`/`ListRoles`/`GetRoleByCode` dipakai ulang.
- Query tx (insert/soft-delete) dipanggil lewat `qtx := queries.WithTx(tx)`.

## 8. Pengujian (proaktif & luas)

- **Unit** (`*_test.go`): `IsKnownPermission` + katalog non-empty/konsisten (tiap key punya label, tak ada duplikat); validasi DTO (key di luar katalog→error, scope_level invalid→error, module/entity/field kosong→error, dedupe); aturan is_system (code immutable, protect-delete) di level service dengan fake/nil di mana mungkin.
- **Integration** (`//go:build integration`, Postgres+Redis testcontainer):
  - Create/Get/Update/List/Delete peran kustom; `code` duplikat→409.
  - `is_system` tak bisa dihapus (409) & `code`-nya tak bisa diubah.
  - Delete ditolak saat dipakai user (409); sukses + cascade saat tak dipakai.
  - Replace-set permissions/scope/fields menulis set baru & menyoft-delete lama.
  - **Invalidasi cache terverifikasi**: panggil `Has`/`Resolve`/`ForEntity` (memanaskan cache) → ubah via endpoint → panggil lagi → hasil **langsung** berubah (bukan menunggu TTL).
  - Assign permission di luar katalog → 400.
  - Audit row tercatat untuk tiap mutasi.
- **Verifikasi**: `go build/vet/test`, `go test -tags=integration ./...`, Spectral lint hijau.

## 9. Sinkronisasi & "selesai"

- `backend/api/openapi.yaml` — tambah path `/authz/*` + schema (`Role`, `RoleCreateRequest`, `RoleUpdateRequest`, `PermissionSet`, `ScopePolicySet`, `FieldPermissionSet`, `AuthzCatalog`); lolos Spectral.
- `docs/PROGRESS.md` — centang **"Authorization admin endpoints"**; catat perbaikan **seed drift** (key aset/approval) di bagian yang sesuai; refresh blok "Next session".
- Semua gerbang CI hijau.
