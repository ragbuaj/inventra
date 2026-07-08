# Modul Assignment (Penugasan / Peminjaman) — Design

**Tanggal:** 2026-07-07 · **Status:** Disetujui user (brainstorming session)
**Referensi:** PRD §3.3 (FR-3.1–3.4) + §3.6 (maker-checker) + RBAC matrix §2.2 ·
DATABASE.md §4.4 (schema `assignment`) · migrasi `000011_assignment` (tabel sudah ada) ·
enum `shared.assignment_status('active','returned')` + `shared.request_type` (nilai `assignment`
sudah ada) · mockup `docs/design/Penugasan Aset.dc.html` (Manager, sudah terbangun mock),
`docs/design/Peminjaman Aset.dc.html` (Staf, baru), `docs/design/Modal Ajukan Peminjaman.dc.html`
(baru) · DESIGN_BRIEF §5.9 + §5.25 · pola modul `internal/transfer` (PR #44/#49),
`internal/disposal` (PR #45), `internal/stockopname` (PR #56) · ADR-0008 (4-file split)

## Tujuan

Menghidupkan lifecycle penugasan aset (check-out → assigned → check-in → returned) yang menutup
utang desain eksplisit: field **"Pemegang"** yang di-drop dari form Aset (Katalog menampilkan "—"
sampai modul ini ada). Dua jalur PRD dibangun penuh:

1. **Check-out / check-in langsung** oleh Manager (aksi operasional dalam lingkup kantor, seperti
   CRUD aset — bukan maker-checker). 1:1 dari mockup `Penugasan Aset.dc.html` (layar mock sudah
   terbangun, tinggal di-wire ke API nyata).
2. **Peminjaman oleh Staf via approval engine** (FR-3.3): Staf ajukan → Manager/Kepala approve →
   persetujuan **otomatis memicu check-out** ke pegawai milik Staf. Request type `assignment`
   +
   executor didaftarkan (konsisten dengan transfer/disposal). Dua permukaan UI baru untuk Staf:
   halaman "Peminjaman Aset" (`/peminjaman`) + modal "Ajukan Peminjaman" di Detail Aset.

`assignment.assignments` (tabel sudah ada, belum pernah terisi kode mana pun) mulai dipelihara
oleh modul ini; `asset.assets.status` di-transisikan `available ↔ assigned` (+ opsi
`under_maintenance` saat check-in).

## Keputusan produk (dikonfirmasi user)

1. **Lingkup: penuh** — direct check-out/check-in + executor peminjaman + **screen Staf baru**
   (halaman `/peminjaman` DAN modal di Detail Aset). User memilih opsi terlengkap secara eksplisit;
   mockup untuk keduanya sudah dibuat user (`Peminjaman Aset.dc.html` + `Modal Ajukan
   Peminjaman.dc.html`) dan diverifikasi cocok 1:1 dengan prompt di DESIGN_BRIEF §5.25.
2. **Peminjaman bukan value-tiered** — 1 langkah approval (`required_level='office'`, `step_order=1`,
   `amount=0`) oleh Manager/Kepala kantor aset. Beda dari disposal/transfer yang bertingkat per nilai.
3. **Employee peminjaman = pegawai milik Staf** — di-resolve server-side dari `users.employee_id`
   requester; tidak ada pemilih penerima di UI Staf (itulah pembeda dari layar Penugasan Manager,
   di mana Manager memilih penerima siapa saja dalam scope). Requester tanpa employee tertaut →
   ditolak.
4. **`assigned_by_id`** = Manager pelaku (direct) / approver (peminjaman) — yakni pihak yang
   mengotorisasi check-out, bukan requester.
5. **Scope basis = kantor aset** — assignment discope lewat `asset.assets.office_id` (JOIN),
   sama seperti transfer/disposal. Read **dan** write discope.
6. **Kondisi = teks bebas** disimpan di `condition_out`/`condition_in`; UI memakai 3 nilai
   (baik/ringan/berat) yang dipetakan ke teks — mempertahankan komponen mockup tanpa menambah enum.

## Batasan yang jujur dicatat (deviasi mockup + follow-up, bukan blocker)

- **Checkbox "perlu maintenance" saat check-in** hanya mentransisikan `asset.status →
  under_maintenance`; **tidak** membuat record maintenance (modul Maintenance belum ada). Dicatat
  sebagai deviasi (konvensi catat-deviasi).
- **Tombol "Ajukan Peminjaman" di Detail Aset tampil untuk semua peran ber-`request.create`**
  (termasuk Manager) — Manager pun boleh meminjam untuk diri sendiri. Aktif hanya saat status aset
  `available`.
- **`due_date` di layar Penugasan Manager**: mockup Manager (`Penugasan Aset.dc.html`) **tidak**
  punya field jatuh tempo → layar Manager tetap tanpa `due_date` (1:1 mockup). `due_date` hanya
  dikumpulkan di jalur peminjaman Staf (mockup barunya punya field itu). Kolom `due_date` tetap
  nullable — konsisten kedua jalur.
- **Badge overdue** (assigned melewati `due_date`) tidak dibangun di fase ini (tidak ada di mockup
  manapun); `due_date` disimpan sehingga fitur overdue/Dashboard menyusul tanpa migrasi.

---

## 1. Backend — modul `internal/assignment` (ADR-0008 4-file split + `executor.go`)

Pola persis `internal/transfer`. Tabel `assignment.assignments` **sudah ada** (migrasi `000011`,
lihat kolom di bawah) — tidak ada migrasi tabel baru; hanya migrasi seed permission + threshold.

Kolom tabel (existing, referensi): `id, asset_id, employee_id, assigned_by_id, checkout_date,
due_date, checkin_date, condition_out, condition_in, status, notes, timestamps, deleted_at`;
unique partial `uq_assignments_active_asset (asset_id) WHERE status='active' AND deleted_at IS
NULL` (menjamin 1-aktif-per-aset).

### 1.1 Migrasi `000026_assignment_seed`

```sql
-- Permission baru: assignment.view (assignment.manage sudah di-seed 000005 utk superadmin+manager)
INSERT INTO identity.role_permissions (role_id, permission) VALUES
  ('superadmin',    'assignment.view'),
  ('kepala_kanwil', 'assignment.view'),
  ('kepala_unit',   'assignment.view'),
  ('manager',       'assignment.view');
-- (Staf melihat penugasan/peminjaman miliknya via scope 'own' + jalur request-nya sendiri.)

-- Approval threshold utk request_type 'assignment': 1 langkah, tidak value-tiered.
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order)
VALUES ('assignment', 0, NULL, 'office', 1);
```

`.down.sql` menghapus kedua insert. `catalog.go`: tambah `{"assignment.view", "Lihat penugasan
aset"}` ke `permissionCatalog` dan `"assignments"` ke `ScopeModules()` (+ update `catalog_test.go`
bila menghitung jumlah).

### 1.2 sqlc — `db/queries/assignments.sql`

Semua query list/get discope via `JOIN asset.assets a ON a.id = asset_id` lalu filter
`(AllScope OR a.office_id = ANY(OfficeIds))` (pola transfer). Query:

- `CheckoutAssignment` — INSERT (asset_id, employee_id, assigned_by_id, checkout_date, due_date,
  condition_out, notes).
- `CheckinAssignment` — UPDATE set checkin_date, condition_in, status='returned' WHERE id + active.
- `GetAssignment` — by id (enriched via join, lihat DTO).
- `GetActiveAssignmentByAsset` — untuk lookup check-in + guard.
- `ListAssignments` — filter status/employee_id + search (asset tag/name, employee name) +
  pagination + scope; `CountAssignments` pendamping.
- `ListAssignmentsByAsset` — riwayat per aset (FR-3.4), scope.

Enrichment (name resolution) via join ke `asset.assets`, `masterdata.employees`, `identity.users`
(assigned_by) — kembalikan `asset_tag`, `asset_name`, `employee_name`, `assigned_by_name`,
`office_id`/`office_name` di row types (pola read enriched transfer/disposal).

### 1.3 `service.go` — state machine (Gin-free, sentinel errors + `mapDBError`)

```
available ──Checkout──▶ assigned      (assignment.status='active'; asset.status: available→assigned)
assigned  ──Checkin───▶ returned      (assignment.status='returned'; asset.status: assigned→available|under_maintenance)
```

- **`Checkout(ctx, in, allScope, officeIDs)`** — dalam `pgx.Tx`:
  1. Load aset FOR UPDATE; wajib in-scope (else `ErrOutOfScope`) dan `status='available'`
     (else `ErrAssetNotAvailable` — blokir `under_maintenance`/`in_transfer`/`assigned`/dll, PRD §3.3).
  2. Validasi employee ada + (opsional) in-scope.
  3. INSERT assignment; `23505` pada unique index → `ErrAlreadyAssigned`.
  4. UPDATE `asset.status='assigned'`.
  Sentinels: `ErrNotFound, ErrOutOfScope, ErrAssetNotAvailable, ErrAlreadyAssigned, ErrEmployeeNotFound`.
- **`Checkin(ctx, id, in, scope)`** — dalam tx: load assignment in-scope + `status='active'`
  (else `ErrNotActive`/`ErrNotFound`); set checkin_date/condition_in/status='returned';
  `asset.status` → `under_maintenance` bila `in.NeedsMaintenance` else `available`.
- **`List/Get/ListByAsset`** — teruskan scope.
- **`Executor()`** (untuk approval type `assignment`) — dipanggil approval engine di dalam tx
  commit approve. Baca payload (`asset_id`, `due_date?`, `condition_out?`, `notes?`); resolve
  `employee_id` dari `users.employee_id` requester (tolak bila kosong); `assigned_by_id` = approver;
  jalankan logika checkout yang sama (aset harus masih `available` saat approve — else executor
  gagal → approval rollback, pola transfer/disposal). Tanda tangan executor mengikuti
  `transferSvc.Executor()` (verifikasi saat implementasi).

### 1.4 `dto.go` / `handler.go` / `routes.go`

- **dto**: `CheckoutRequest{asset_id*, employee_id*, due_date?, condition_out?, notes?}`,
  `CheckinRequest{checkin_date?, condition_in?, needs_maintenance bool}`, `assignmentToMap`
  (enriched; siap FieldService bila kelak). Payload peminjaman: struct `AssignmentPayload`.
- **handler**: bind → `common.ScopedDeps.CallerOfficeScope(c, "assignments")` → service →
  serialize; `svcError` memetakan sentinel → status (404/403/409/422).
- **routes** (`/api/v1`):

  | Method | Path | Gate |
  |---|---|---|
  | GET | `/assignments` | `assignment.view` |
  | GET | `/assignments/:id` | `assignment.view` |
  | POST | `/assignments` (check-out langsung) | `assignment.manage` |
  | POST | `/assignments/:id/checkin` | `assignment.manage` |
  | GET | `/assets/:id/assignments` (riwayat) | `assignment.view` |

  Scope ditegakkan read **dan** write.

### 1.5 Wiring + approval submit + OpenAPI

- `router.go` `NewRouter`: konstruksi `assignmentSvc` + `RegisterRoutes`, lalu
  `approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssignment, assignmentSvc.Executor())`.
- `internal/approval/dto.go` `SubmitRequest.validate()`: tambah case `assignment` — `amount`
  wajib 0 (bukan value-tiered), payload wajib `asset_id` non-kosong.
- **Tambahan kecil approval** (satu-satunya di luar modul assignment): `GET /requests` menerima
  param `mine=true` → filter `requested_by = caller` (untuk daftar "Pengajuan Saya" Staf).
  Alternatif bila list sudah men-scope requester-own untuk peran `own`: verifikasi saat
  implementasi; pilih jalur paling kecil.
- `backend/api/openapi.yaml`: schema `Assignment` + 5 path + param `mine` + payload `assignment`
  pada `SubmitRequest`.

---

## 2. Frontend — 3 permukaan

**Sidebar Staf** (dari mockup): grup "MENU SAYA" = Dashboard · Aset Saya · **Peminjaman Aset** ·
Pengajuan Saya · Lapor Kerusakan. Task ini menambah item nav **Peminjaman Aset**
(`/peminjaman`, gate `request.create`). Item Staf lain (Aset Saya, Lapor Kerusakan) di luar
scope — masuk item PROGRESS "Staff role menus" terpisah.

### 2.1 Layar Penugasan Manager (`/assignment`) — wire mock → real, 1:1 `Penugasan Aset.dc.html`

`app/pages/assignment.vue` sudah terbangun (3 tab: Check-out / Check-in / Riwayat). Perubahan:

- `useAssignment` ditulis ulang ke `$fetch` nyata: `list()→GET /assignments`,
  `available()→GET /assets?status=available` (scoped), `checkout()→POST /assignments`,
  `checkin(id)→POST /assignments/:id/checkin`.
- Pemilih penerima → **pegawai nyata** `GET /employees?limit=100` (scoped, name resolution),
  ganti `recipientSeed`.
- Kondisi UI 3-nilai → teks di `condition_out`/`condition_in`.
- Checkbox "perlu maintenance" → kirim `needs_maintenance:true` (aset → `under_maintenance`).
- Gate diganti `masterdata.office.manage` (placeholder mock) → **`assignment.manage`**.
- Load-error/retry + skeleton (pola layar lain).

### 2.2 Layar "Peminjaman Aset" Staf (`/peminjaman`) — BARU, 1:1 `Peminjaman Aset.dc.html`

- **Kartu "Ajukan Peminjaman"**: Aset (`USelectMenu` search, hanya `available` scoped) + Jatuh
  Tempo (date, opsional) + Alasan* (`UTextarea`) + banner hijau info + tombol Batal/Ajukan.
  Submit → `POST /requests` type `assignment`, payload `{asset_id, due_date, notes}`, `amount:0`,
  `reason`= Alasan. Toast sukses "Pengajuan peminjaman terkirim".
- **Section "Pengajuan Peminjaman Saya"**: tab Menunggu/Disetujui/Ditolak/Semua; tabel kolom
  Aset (nama+kode) · Diajukan · Jatuh Tempo · Status (badge: Menunggu=warning, Disetujui=success,
  Ditolak=error, Dibatalkan=neutral) · Catatan Keputusan · Aksi (Batalkan untuk Menunggu →
  `POST /requests/:id/cancel`). Baris expand → **timeline persetujuan** (Diajukan oleh saya →
  Disetujui/Ditolak oleh approver, nama+waktu+catatan). Empty state + skeleton + total count.
- Data list = `GET /requests?type=assignment&mine=true` (lihat §1.5). Resolusi nama aset di
  payload best-effort.
- Gate halaman: `middleware: 'can', permission: 'request.create'`.

### 2.3 Modal "Ajukan Peminjaman" di Detail Aset — BARU, 1:1 `Modal Ajukan Peminjaman.dc.html`

- Tombol "Ajukan Peminjaman" (ikon `i-lucide-hand`, primary) di header aksi `assets/[tag]/index.vue`
  — aktif hanya bila `status === 'available'` (else disabled + tooltip "Hanya aset tersedia yang
  bisa dipinjam"), gate `request.create` via `useCan`.
- Modal (`UModal`): blok aset terkunci read-only (nama/kode mono/kategori/kantor/lokasi + ikon
  gembok) + Jatuh Tempo (opsional) + Alasan* + banner hijau + Batal/Kirim Pengajuan (loading).
  Submit identik §2.2. Sukses → tutup modal + toast + tautan kecil "Lihat di Peminjaman Saya".

### 2.4 Shared + i18n + tests

- **Komponen** `components/assignment/AjukanPeminjamanModal.vue` — dipakai §2.2 (opsional, form
  di kartu bisa inline) **dan** §2.3 (modal). Satu sumber logika submit peminjaman.
- **`constants/assignmentMeta.ts`** — `STATUS_TONE` (assignment active/returned),
  `REQUEST_STATUS_TONE` (pending/approved/rejected/cancelled), `CONDITION_TONE/CONDITION_KEYS`
  (pindah dari `mock/assignment.ts`), formatter tanggal Indonesia.
- **i18n** id/en penuh (namespace `assignment.*` sudah ada sebagian dari mock — perluas; tambah
  `peminjaman.*` untuk layar Staf + modal).
- **Nav** `app/utils/nav.ts`: item `peminjaman` (gate `request.create`); pastikan `assignment`
  item ada + gate `assignment.manage`.
- **Cleanup**: `mock/assignment.ts` + `useAssignment` mock-store dihapus setelah wire (cek konsumen
  lain, mis. `useGlobalSearch` — bila ada, ikuti pola retain seperti mock lain).

### 2.5 Testing (proaktif & luas)

- **Unit**: `assignmentMeta` (tone maps, formatter), helper resolusi payload.
- **Component (`mountSuspended`)**:
  - `/assignment` (Manager): tab switch, check-out form kosong/valid/error, available-empty,
    check-in active-empty vs populated, needs-maintenance toggle, riwayat filter/search/empty,
    loading/error.
  - `/peminjaman` (Staf): form kosong/valid/error (Alasan wajib), submit sukses (toast), list
    kosong/isi, tab status, badge tone tiap status, expand timeline, tombol Batalkan hanya untuk
    Menunggu, loading/error.
  - `AjukanPeminjamanModal` di Detail Aset: tombol disabled saat status ≠ available (+ tooltip),
    modal buka/submit/tutup, blok aset terkunci render benar.
- **E2E real-backend** `frontend/e2e/assignment.spec.ts`:
  - Setup API (office/floor/room/category/asset unik per-run, pola assets.spec).
  - **Jalur direct**: Manager check-out aset → assert `assigned` + muncul di Riwayat aktif →
    check-in → assert `available`/returned.
  - **Jalur peminjaman**: Staf submit peminjaman → muncul di "Pengajuan Saya" (Menunggu) →
    approve sebagai approver SoD-eligible (maker ≠ checker) → assert assignment auto-terbuat +
    aset `assigned` + status pengajuan Disetujui.
  - Negatif: submit tanpa Alasan (validasi), tombol Detail-Aset disabled saat aset tidak tersedia.

---

## 3. Urutan implementasi (untuk plan)

1. Migrasi `000026_assignment_seed` (+down) → `catalog.go`/`catalog_test.go`.
2. `db/queries/assignments.sql` → `sqlc generate`.
3. `internal/assignment` (service→dto→handler→routes→executor) + integration tests.
4. Approval: `validate()` case `assignment` + `mine=true` param + unit tests.
5. Wiring `router.go` (routes + RegisterExecutor) + OpenAPI + Spectral.
6. Gate backend: build/vet/test + `-tags=integration` semua paket hijau.
7. FE: `useAssignment` real + `assignmentMeta` + nav.
8. FE: `/assignment` wire (Manager) + tests.
9. FE: `/peminjaman` (Staf) + `AjukanPeminjamanModal` + tests.
10. FE: tombol+modal di Detail Aset + tests.
11. i18n id/en; hapus mock; lint/typecheck/test/build.
12. E2E `assignment.spec.ts`; verifikasi 1:1 vs 3 mockup (light+dark); PROGRESS.md.

## Gate (task-13, wajib hijau sebelum "selesai")

Backend: `go build/vet/test` + `go test -tags=integration ./...` (semua paket). Spectral 0 error.
Frontend: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`. E2E `assignment.spec.ts`
(suite penuh via CI fresh-DB). PROGRESS.md item 35 → assignment ✅ + entri di §Remaining.
