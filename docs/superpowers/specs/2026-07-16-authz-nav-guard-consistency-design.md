# Design — Konsistensi Authz: Nav, Page-Guard, Endpoint

**Tanggal:** 2026-07-16
**Status:** Disetujui (2026-07-16) — Opsi 1 (bagian C) & dua entri menu terpisah (bagian A) dipilih
**Cakupan:** Perbaikan lintas-cutting pada tiga lapis otorisasi frontend/backend agar **visibilitas menu =
keterjangkauan halaman = izin endpoint**. Frontend (nav model + page-guard + item permission) dan sedikit
backend (pelonggaran read authz-admin). Diikuti satu rencana implementasi bertahap.

## Ringkasan

Audit menemukan bahwa role selain `superadmin` tidak dapat mencapai menu yang izinnya mereka miliki, dan —
bila diberi `user.manage` — justru melihat menu yang diklik menghasilkan **403**. Akarnya: tiga lapis gating
tidak sinkron.

| Bug | Ringkas | Area |
|-----|---------|------|
| A | Pemilihan nav biner `can('user.manage') ? superadminNav : staffNav`, sehingga kanwil/kepala_unit/manager jatuh ke menu staf | Frontend |
| B | Item nav grup Aset/Master/Settings **tanpa** `permission`, sehingga tampil tanpa syarat, lalu page-guard 403 saat diklik | Frontend |
| C | Page-guard tidak sama dengan endpoint pada layar authz (`rbac`/`data-scope`/`field-permission` guard `user.manage`, endpoint butuh `role`/`scope`/`fieldperm.manage`) + Dashboard tanpa guard | Frontend + Backend |
| D | Inkonsistensi key: Assignment di-gate `assignment.manage` padahal list butuh `assignment.view`; Maintenance nav pakai `maintenance.view` tapi page-guard `request.create` | Frontend |

**Tiga lapis** (harus konsisten):
1. **Nav visibility** — `AppSidebar.isVisible`: tampil bila `!item.permission || can(item.permission)`.
2. **Page guard** — `middleware/can.ts` memanggil `abortNavigation(403)` bila `!can(to.meta.permission)`.
3. **Endpoint** — `middleware.RequirePermission` di backend.

Prinsip lintas-perbaikan: ikuti CLAUDE.md (komponen `U*`, token tema, i18n `id`/`en`, test proaktif per-role,
least-privilege / SoD sesuai konteks bank).

---

## Matriks permission per role (dari seed, otoritatif)

| Permission | superadmin | kepala_kanwil | kepala_unit | manager | staf |
|---|:--:|:--:|:--:|:--:|:--:|
| user.manage / role.manage / scope.manage / fieldperm.manage | ✅ | — | — | — | — |
| audit.view | ✅ | ✅ | ✅ | — | — |
| masterdata.global.manage | ✅ | — | — | — | — |
| masterdata.office.manage / employee.manage | ✅ | ✅ | — | — | — |
| asset.view | ✅ | ✅ | ✅ | ✅ | ✅ |
| asset.manage | ✅ | — | — | ✅ | — |
| request.create | ✅ | ✅ | ✅ | ✅ | ✅ |
| request.decide | ✅ | ✅ | ✅ | ✅ | — |
| approval.config.manage | ✅ | — | — | — | — |
| valuation.exclude.approve | ✅ | ✅ | — | — | — |
| report.view | ✅ | ✅ | ✅ | ✅ | ✅ |
| report.export | ✅ | ✅ | ✅ | ✅ | — |
| transfer.view / transfer.manage | ✅ | ✅ | ✅ | ✅ | — |
| disposal.view / disposal.manage | ✅ | ✅ | ✅ | ✅ | — |
| stockopname.view / stockopname.manage | ✅ | ✅ | ✅ | ✅ | — |
| assignment.view | ✅ | ✅ | ✅ | ✅ | — |
| assignment.manage | ✅ | — | — | ✅ | — |
| maintenance.view | ✅ | ✅ | ✅ | ✅ | — |
| maintenance.manage | ✅ | — | — | ✅ | — |
| depreciation.view / depreciation.manage | ✅ | — | — | — | — |

---

## A. Nav model tunggal per-permission

### Masalah
`AppSidebar.vue:14` memilih seluruh nav dari satu bit `user.manage`. Semua role non-superadmin memakai
`staffNav` (Dashboard, Peminjaman, Maintenance), sehingga menu yang izinnya dimiliki (Mutasi, Penghapusan,
Stock Opname, Approval, Laporan, Master Data, Audit) tidak terjangkau.

### Desain
- **Hapus** `superadminNav` / `staffNav`. Ganti dengan **satu** `appNav: NavGroup[]` di `utils/nav.ts`.
- **Tipe `NavItem.permission` diperluas ke `string | string[]`** dengan semantik **OR** (tampil bila punya
  salah satu). Diperlukan karena beberapa halaman punya dua pintu masuk (mis. Maintenance).
- `AppSidebar.isVisible(item)`:
  - item daun: `true` bila `!permission` atau `hasAny(permission)`.
  - item parent (punya `children`): `true` bila **ada** anak yang `isVisible`, sehingga grup/parent otomatis
    tersembunyi ketika semua anaknya tersembunyi.
- `AppTopbar` breadcrumb memakai `appNav` yang sama (hapus import `superadminNav`).
- Hapus placeholder mati (`My Assets` disabled, `Approval (staff)` disabled) — digantikan item ber-permission.

### Peta permission per item nav (final)

**Grup Operasional**
| Item | `to` | permission |
|---|---|---|
| Dashboard | `/` | *(tanpa — semua authenticated)* |
| Aset > Katalog | `/assets` | `asset.view` |
| Aset > Impor | `/assets/import` | `asset.manage` |
| Aset > Cetak Label | `/assets/label` | `asset.view` |
| Peminjaman | `/peminjaman` | `request.create` |
| Penugasan Aset | `/assignment` | `assignment.view` |
| Stock Opname | `/stock-opname` | `stockopname.view` |
| Mutasi Aset | `/transfers` | `transfer.view` |
| Penghapusan Aset | `/disposals` | `disposal.view` |
| Penyusutan | `/depreciation` | `depreciation.view` |
| Maintenance | `/maintenance` | `['maintenance.view', 'request.create']` |
| Approval | `/approval` | `request.decide` |
| Laporan | `/reports` | `report.view` |

**Grup Administrasi**
| Item | `to` | permission |
|---|---|---|
| Master > Kantor | `/master/offices` | `masterdata.office.manage` |
| Master > Pegawai | `/master/employees` | `masterdata.office.manage` |
| Master > Kategori | `/master/categories` | `masterdata.global.manage` |
| Master > Peta Lokasi | `/master/map` | `masterdata.office.manage` |
| Master > Referensi | `/master/reference` | `masterdata.global.manage` |
| Master > Impor | `/master/import` | `['masterdata.employee.manage','masterdata.office.manage','masterdata.global.manage']` |
| Settings > Users | `/settings/users` | `user.manage` |
| Settings > RBAC | `/settings/rbac` | `role.manage` |
| Settings > Data Scope | `/settings/data-scope` | `scope.manage` |
| Settings > Field Permission | `/settings/field-permission` | `fieldperm.manage` |
| Settings > Audit Trail | `/settings/audit` | `audit.view` |

> Catatan Peminjaman vs Penugasan: keduanya sengaja dipertahankan sebagai entri berbeda — Peminjaman
> (`request.create`, self-service ajukan pinjam) dan Penugasan (`assignment.view`, manajemen check-out/in).
> Superadmin/manager bisa melihat keduanya; itu diterima.

---

## B. Samakan page-guard dengan permission entry nav

### Desain
`middleware/can.ts` juga diperluas menerima `permission: string | string[]` (OR). Lalu set
`definePageMeta.permission` tiap halaman = permission entry nav-nya (lihat tabel A), agar
**visibilitas = keterjangkauan**:

| Halaman | page-guard sekarang | page-guard baru |
|---|---|---|
| `/assignment` | `assignment.manage` | `assignment.view` |
| `/maintenance` | `request.create` | `['maintenance.view','request.create']` |
| `/settings/rbac` | `user.manage` | `role.manage` |
| `/settings/data-scope` | `user.manage` | `scope.manage` |
| `/settings/field-permission` | `user.manage` | `fieldperm.manage` |
| `/` (dashboard) | *(tanpa guard)* | tetap tanpa guard; fetch summary di-gate `can('report.view')` |

Halaman lain (`/assets*`, `/transfers`, `/disposals`, `/stock-opname`, `/depreciation`, `/reports`,
`/approval`, `/master/*`, `/settings/users`, `/settings/audit`) sudah selaras — tidak diubah.

### Gating fetch/aksi di dalam halaman (defensif)
- **Dashboard** `/`: panggil `GET /dashboard/summary` hanya bila `can('report.view')`; jika tidak, tampilkan
  kartu ringkasan kosong/placeholder (tanpa 403). (Semua role seed punya `report.view`; ini untuk role kustom.)
- **Assignment** `/assignment`: guard `assignment.view`; fetch `GET /assignments/available` (checkout picker)
  di-gate `can('request.create')`; tombol checkout/checkin di-gate `can('assignment.manage')`. (Sudah ada
  sebagian; pastikan fetch `/available` tidak dipanggil tanpa `request.create`.)

---

## C. Keputusan: `role.manage` vs `scope.manage`/`fieldperm.manage` (butuh konfirmasi)

### Masalah
Ketiga layar authz memuat `GET /authz/catalog` + `GET /authz/roles` (keduanya kini `role.manage`), plus
mutasi spesifik (`/roles/:id/scope` butuh `scope.manage`, `/roles/:id/fields` butuh `fieldperm.manage`). Selain itu
`/settings/users` memuat `GET /authz/roles` (untuk dropdown peran) — juga `role.manage`. Akibatnya
`scope.manage` & `fieldperm.manage` **tidak bisa didelegasikan mandiri**: semuanya runtuh ke "harus punya
`role.manage`", dan admin-user (`user.manage`) tak bisa memuat daftar peran.

### Opsi
- **Opsi 1 (disarankan — least privilege / SoD):** longgarkan **read** authz-admin dengan middleware baru
  `RequireAnyPermission(permSvc, keys...)`:
  - `GET /authz/catalog` boleh salah satu dari `role.manage`, `scope.manage`, `fieldperm.manage`.
  - `GET /authz/roles` (list) + `GET /authz/roles/:id` boleh salah satu dari `role.manage`, `scope.manage`,
    `fieldperm.manage`, `user.manage`.
  - `GET /authz/roles/:id/permissions|scope|fields` tetap spesifik per manage-key.
  - **Mutasi** (`POST/PUT/DELETE`) tetap ketat: `role.manage` / `scope.manage` / `fieldperm.manage`.
  - Hasil: role bisa diberi **hanya** `scope.manage`, sehingga layar Data Scope berfungsi; `user.manage` bisa
    membaca daftar peran untuk assign. Cocok dengan pemisahan tugas (SoD) bank.
- **Opsi 2 (lebih sederhana):** jadikan `role.manage` payung untuk ketiga layar (page-guard ketiganya
  `role.manage`); `scope.manage`/`fieldperm.manage` hanya jadi gate mutasi endpoint, tak berdiri sendiri di UI.
  Lebih sedikit kode, tapi granularitas delegasi hilang.

**Rekomendasi:** Opsi 1. Menyelesaikan bug sekaligus membuat tiga permission yang sudah ada di katalog benar-benar
bermakna. Perubahan backend: tambah `middleware.RequireAnyPermission` + `authz.PermissionService` sudah punya
`Has`; wire ulang di `authzadmin.RegisterRoutes` dan `router.go` (users lookup). OpenAPI disesuaikan.

---

## D. Konsistensi lain
- **Assignment**: nav & page-guard turun ke `assignment.view` (lihat A/B) — kanwil & kepala_unit kini bisa
  melihat daftar penugasan; aksi manage tetap `assignment.manage`.
- **Maintenance**: satu entri nav + page-guard `['maintenance.view','request.create']`; konten internal tetap
  di-gate `canView`/`canManage`/`canReport` (sudah ada).

---

## Perubahan backend (ringkas)
1. `internal/middleware/permission.go`: tambah `RequireAnyPermission(checker, keys ...string)` (lolos bila
   `Has` true untuk salah satu key; 403 bila tidak; 401 bila tak ada role).
2. `internal/authzadmin/routes.go`: ganti `requireRole` pada `GET /catalog`, `GET /roles`, `GET /roles/:id`
   menjadi `RequireAnyPermission(...)` sesuai Opsi 1.
3. `internal/server/router.go`: `/users` lookup peran memakai read yang dilonggarkan (sudah otomatis bila
   `/authz/roles` dilonggarkan).
4. `backend/api/openapi.yaml`: perbarui deskripsi security untuk endpoint yang dilonggarkan.

Tidak ada perubahan skema/migrasi. Read master-data tetap auth-only (tak diubah).

---

## Strategi test (per-role, proaktif)

**Unit (Vitest, node):**
- `utils/nav` + `isVisible`: untuk tiap role seed (superadmin, kepala_kanwil, kepala_unit, manager, staf),
  hitung himpunan item nav yang terlihat dan **assert = himpunan permission yang dimiliki** (tabel A).
- `middleware/can` (logika OR): string tunggal & array; hasil allow/deny.

**Runtime (Vitest + `mountSuspended`):**
- `AppSidebar` per-role: render dengan `permissions` tiap role, lalu assert menu yang muncul & yang tersembunyi
  (mis. kanwil melihat Mutasi/Penghapusan/Stock Opname/Approval/Laporan/Audit; staf tidak).
- Auto-hide parent: role tanpa satu pun anak Settings, sehingga grup Settings tidak dirender.

**E2E (Playwright, backend nyata):**
- Login tiap role seed; untuk **setiap** item nav yang terlihat, klik, lalu assert halaman terbuka **tanpa 403**
  dan memuat konten utamanya (menutup seluruh kelas bug ini end-to-end). Seed sudah menyediakan login demo.
- Kasus Opsi 1: buat role kustom hanya `scope.manage`, sehingga layar Data Scope terbuka & memuat katalog/roles.

**Backend (Go):**
- `TestRequireAnyPermission` (allow bila salah satu, deny bila tak ada, 401 tanpa role).
- Integration authzadmin: role dengan hanya `scope.manage` bisa `GET /catalog` & `GET /roles`, `PUT /scope`;
  tapi `PUT /permissions` (role.manage) menghasilkan 403.

---

## Boundaries

**Selalu:** samakan ketiga lapis; least-privilege; i18n `id`/`en` untuk label/menu baru; test tiap role
sebelum klaim selesai; jalankan gate CI (go build/vet/test, spectral; pnpm lint/typecheck/test/build).

**Tanya dulu:** perubahan cakupan permission per role di seed (spec ini **tidak** mengubah seed/peran-permission —
hanya menyelaraskan gating agar sesuai izin yang sudah ada); perubahan desain menu di luar penyelarasan ini.

**Jangan:** mengubah skema/migrasi; menambah/menghapus permission key; mengubah data-scope enforcement;
melonggarkan **mutasi** endpoint; redesign layout menu (hanya visibilitas/gating).

---

## Keputusan (disetujui 2026-07-16)
1. **Bagian C — Opsi 1** — longgarkan read authz-admin via `RequireAnyPermission`; mutasi tetap ketat per-key.
2. **Bagian A — dua entri terpisah** — Peminjaman (`request.create`) & Penugasan (`assignment.view`) tetap terpisah.
