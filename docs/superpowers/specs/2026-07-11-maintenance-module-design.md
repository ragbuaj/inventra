# Modul Maintenance (Jadwal · Catatan · Laporan Kerusakan) — Design

**Tanggal:** 2026-07-11 · **Status:** Disetujui user (brainstorming session)
**Referensi:** PRD bagian 3.4 (FR-4.1–4.6) + bagian 3.6 (maker-checker) + RBAC matrix bagian 2.1 + state machine bagian 5 ·
DATABASE.md bagian 4.4 (schema `maintenance`) · migrasi `000012_maintenance` (tabel sudah ada) ·
enum `shared.maintenance_type('preventive','corrective')` +
`shared.maintenance_status('scheduled','in_progress','completed','cancelled')` +
`shared.request_type` (nilai `maintenance` sudah ada) · mockup `docs/design/Maintenance.dc.html` ·
master data `maintenance_categories` / `problem_categories` / `vendors` (reference engine, sudah ada) ·
pola modul `internal/assignment` (PR #57), `internal/stockopname` (PR #56) · ADR-0008 (4-file split)

## Tujuan

Menghidupkan lifecycle maintenance aset (FR-4.1–4.6) dan **mengonsumsi sinyal `under_maintenance`**
yang selama ini hanya di-flag tanpa jejak:

1. **Jadwal perawatan berkala** per aset (interval bulan → `next_due_date`), tampil dengan badge
   jatuh tempo + banner reminder in-app (FR-4.1, FR-4.5 — asumsi PRD A2: in-app dulu).
2. **Catatan maintenance** (preventive/corrective, biaya, vendor) dengan transisi status yang
   menggerakkan status aset: mulai → `under_maintenance`, selesai → `available` (FR-4.2, FR-4.3).
3. **Laporan kerusakan oleh Staf** → request approval type `maintenance`; persetujuan checker
   memicu pembuatan catatan corrective `scheduled` via executor (FR-4.4, FR-6.1).
4. **Tindak lanjut eksplisit** untuk aset ter-flag: daftar "Perlu Tindak Lanjut" (aset
   `under_maintenance` tanpa catatan aktif) + tindak lanjut item stock-opname `damaged` →
   membuat catatan maintenance (menutup deviasi (d) modul Assignment & gap `damaged` stock opname).

`maintenance.maintenance_schedules` dan `maintenance.maintenance_records` (migrasi `000012`,
belum pernah terisi) mulai dipelihara modul ini.

## Keputusan produk (dikonfirmasi user)

1. **Lingkup: full-stack** — backend module + frontend `/maintenance` (3 tab sesuai mockup) +
   tab riwayat di Detail Aset + e2e. Pola sesi assignment/stock-opname.
2. **Jadwal dapat dibuat/diedit dari UI** — tombol "Tambah Jadwal" + slideover (aset, kategori
   perawatan, interval bulan, tanggal mulai) di tab Jadwal. **Deviasi mockup yang disetujui**
   (mockup hanya menampilkan jadwal, tanpa UI pembuatan).
3. **Tanpa auto-create record.** Check-in assignment "perlu maintenance" **tetap hanya flip
   status** (modul assignment tidak diubah). Tindak lanjut eksplisit oleh Manager lewat daftar
   **"Perlu Tindak Lanjut"** di halaman Maintenance: aset `under_maintenance` yang tidak punya
   catatan aktif (`scheduled`/`in_progress`); tombol per item membuka slideover catatan
   ter-prefill → Manager membuat catatan `scheduled` secara sadar.
4. **Stock opname `damaged` → tindak lanjut membuat catatan langsung** (tanpa approval), lewat
   tombol tindak lanjut yang sudah ada di layar Stock Opname (eksplisit, bukan otomatis).
   Idempoten via kolom baru `followup_record_id`.
5. **Foto laporan kerusakan = asset attachment** (kind `photo`, MinIO) di-upload saat submit;
   `attachment_id` disimpan di payload request sehingga checker bisa melihatnya saat review.
6. **Update catatan via slideover edit** — klik baris tabel Catatan membuka slideover yang sama
   dengan Tambah Catatan dalam mode edit (ubah status, tanggal selesai, biaya final, vendor).
   **Deviasi mockup yang disetujui** (mockup tanpa aksi per baris).
7. **Arsitektur: satu paket `internal/maintenance`** (ADR-0008 4-file split + `executor.go`),
   gaya `internal/assignment` — jadwal & catatan berpasangan erat, tidak dipecah sub-paket.
8. **Link catatan → jadwal eksplisit**: kolom baru nullable `schedule_id` di
   `maintenance_records` (migrasi `000027`), bukan pencocokan implisit aset+kategori.
9. **Laporan kerusakan bukan value-tiered** — 1 langkah approval office-level
   (`required_level='office'`, `step_order=1`, `amount=0`), sama seperti peminjaman assignment.

## Batasan yang jujur dicatat (deviasi mockup + follow-up, bukan blocker)

- **"Tambah Jadwal" + slideover jadwal** dan **slideover edit catatan (klik baris)** adalah
  penambahan di luar mockup — disetujui user, dicatat di PROGRESS.md (konvensi catat-deviasi).
- **Seksi "Perlu Tindak Lanjut"** di halaman Maintenance tidak ada di mockup — disetujui user.
- **Vendor/Teknisi di UI = select `vendor_id`** (master data vendors; teknisi internal
  didaftarkan sebagai vendor). Kolom `performed_by` (teks bebas) tetap ada di API/DB tapi tidak
  diekspos di form (mockup hanya punya satu select).
- **Banner jatuh tempo dihitung client-side** dari list jadwal (due ≤ 3 hari, seperti mockup);
  list jadwal per kantor diasumsikan < clamp `limit=100`. Bila kelak melebihi, tambah filter
  server-side (follow-up, bukan fase ini).
- **Reminder = banner in-app di halaman Maintenance saja** (sesuai mockup); notifikasi
  dashboard/pusat notifikasi menyusul di fase Reporting (asumsi PRD A2).
- **Upload foto oleh Staf**: endpoint asset-attachment saat ini di-gate permission asset —
  perlu penyesuaian gate agar pelapor ber-`request.create` dapat meng-upload foto ke aset dalam
  scope-nya (`own`). Detail gate dipastikan saat implementasi; bila berubah dari sekali jalan,
  dicatat sebagai deviasi.

---

## 1. Backend — modul `internal/maintenance` (ADR-0008 4-file split + `executor.go`)

Pola persis `internal/assignment`. Tabel sudah ada (migrasi `000012`):

- `maintenance_schedules`: `id, asset_id FK, maintenance_category_id FK?, interval_months int,
  last_done_date date?, next_due_date date?, is_active bool, timestamps, deleted_at`.
- `maintenance_records`: `id, asset_id FK, maintenance_category_id FK?, problem_category_id FK?,
  type, status (default 'scheduled'), scheduled_date?, completed_date?, cost numeric(18,2)?,
  vendor_id FK?, performed_by text?, description NOT NULL, reported_by_id FK?, timestamps,
  deleted_at`.

### 1.1 Migrasi `000027_maintenance_module` (up + down)

```sql
-- Link eksplisit catatan → jadwal (keputusan #8)
ALTER TABLE maintenance.maintenance_records
  ADD COLUMN schedule_id uuid REFERENCES maintenance.maintenance_schedules (id);
CREATE INDEX idx_mrec_schedule_id ON maintenance.maintenance_records (schedule_id);

-- Idempotensi tindak lanjut damaged (keputusan #4) — pola followup_request_id (000025)
ALTER TABLE stockopname.stock_opname_items
  ADD COLUMN followup_record_id uuid REFERENCES maintenance.maintenance_records (id);

-- Permission baru: maintenance.view (maintenance.manage sudah di-seed 000005
-- untuk superadmin + manager). Kepala dapat view (preseden assignment.view).
INSERT INTO identity.role_permissions (role_id, permission)
SELECT r.id, 'maintenance.view' FROM identity.roles r
 WHERE r.code IN ('superadmin','kepala_kanwil','kepala_unit','manager');

-- Data scope module 'maintenance' — mengikuti pola baris modul 'assignments' (000026).

-- Approval threshold utk request_type 'maintenance': 1 langkah office-level (keputusan #9)
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order)
VALUES ('maintenance', 0, NULL, 'office', 1);
```

`.down.sql` membalik semuanya. `catalog.go`: tambah `maintenance.view` ke `permissionCatalog` dan
`"maintenance"` ke `ScopeModules()` (+ update test penghitung bila ada). Bentuk INSERT final
mengikuti bentuk aktual seed `000026` (kolom/penulisan role id vs code disamakan saat implementasi).

### 1.2 sqlc — `db/queries/maintenance.sql`

Semua list/get discope via `JOIN asset.assets a ON a.id = asset_id` lalu
`(AllScope OR a.office_id = ANY(OfficeIds))` (pola assignment/transfer). Query:

- Jadwal: `CreateSchedule`, `GetSchedule`, `ListSchedules` (+`Count`; filter `is_active`,
  pagination, sort `next_due_date ASC NULLS LAST`), `UpdateSchedule`, `SoftDeleteSchedule`,
  `TouchScheduleDone` (set `last_done_date`, `next_due_date`).
- Catatan: `CreateRecord`, `GetRecord`, `ListRecords` (+`Count`; search `q` pada asset
  tag/name/vendor name, filter `status`/`type`, pagination), `UpdateRecord`,
  `ListRecordsByAsset` (riwayat per aset), `CountActiveRecordsByAsset`
  (status IN scheduled,in_progress — untuk guard release & daftar attention).
- Attention: `ListAttentionAssets` (+`Count`) — aset `status='under_maintenance'` tanpa record
  aktif, scoped, enriched (tag, nama, kantor).

Enrichment via JOIN ke `asset.assets`, `masterdata.maintenance_categories`,
`masterdata.problem_categories`, `masterdata.vendors`, `identity.users` (reported_by) →
kembalikan `asset_tag/asset_name/office_name/category_name/problem_name/vendor_name/
reported_by_name` di row types (pola enriched assignment).

### 1.3 `service.go` — aturan bisnis (Gin-free, sentinel errors + `mapDBError`)

**Jadwal:**
- `CreateSchedule(in)`: aset wajib dalam scope caller; aset `disposed`/`lost` ditolak
  (`ErrAssetNotMaintainable`); `interval_months >= 1`; `next_due_date` = tanggal mulai input
  (first due). `last_done_date` awal null.
- `UpdateSchedule`: interval/kategori/`is_active`; bila interval berubah dan `last_done_date`
  ada → `next_due_date = last_done_date + interval` dihitung ulang.
- `DeleteSchedule`: soft delete.

**Catatan — transisi status (satu transaksi DB per operasi):**

```
scheduled ──mulai──▶ in_progress ──selesai──▶ completed
    │                    │
    └──batal──▶ cancelled◀──batal┘            (completed/cancelled = terminal)
```

- `CreateRecord(in)`: aset dalam scope; aset `disposed`/`lost` ditolak; `description` wajib;
  `schedule_id` (bila ada) harus milik aset yang sama (`ErrScheduleMismatch`). Status awal boleh
  `scheduled`/`in_progress`/`completed` (slideover mockup mengizinkan memilih status); efek
  samping status dijalankan sama seperti transisi di bawah.
- `UpdateRecord(id, in)`: transisi hanya sesuai diagram (`ErrInvalidTransition` untuk lainnya;
  record `completed`/`cancelled` tidak bisa diedit lagi).
- Efek samping (dalam tx yang sama):
  - → `in_progress`: aset `available`/`assigned` → `under_maintenance` (state machine PRD bagian 5);
    aset `in_transfer` ditolak (`ErrAssetBusy`); aset sudah `under_maintenance` → no-op.
  - → `completed`: set `completed_date` (default hari ini) + `cost` final; **release** aset:
    bila aset `under_maintenance` **dan** `CountActiveRecordsByAsset == 0` (setelah record ini)
    → aset `available`. Bila `schedule_id` terisi → `TouchScheduleDone(last_done_date =
    completed_date, next_due_date = completed_date + interval_months)`.
  - → `cancelled`: release aset dengan aturan yang sama (tanpa sentuh jadwal).
- **Scope ditegakkan pada read dan write di semua verb** (aturan repo; missing scope = security bug).

**Laporan kerusakan (Staf):**
- `SubmitReport(caller, in)`: `asset_id` + `problem_category_id` wajib, `description` +
  `attachment_id` opsional. Aset harus dalam scope caller (`own` utk Staf = aset yang
  dipegangnya). Buat request approval type `maintenance`, `office_id` = kantor aset
  (server-resolved, pola borrow), `amount = 0`, payload JSONB
  `{asset_id, problem_category_id, description, attachment_id}`.
  Guard: aset `disposed`/`lost` ditolak; duplikat laporan pending untuk aset yang sama oleh
  requester yang sama ditolak (`ErrDuplicatePending`).

**Attention & follow-up:**
- `ListAttention(scope)`: daftar aset `under_maintenance` tanpa record aktif (untuk UI
  "Perlu Tindak Lanjut").
- `CreateFromOpname(...)`: dipanggil stockopname (lihat bagian 1.6) — buat record corrective
  `scheduled` untuk item `damaged` (deskripsi dari catatan opname, `reported_by_id` = caller).

### 1.4 `executor.go` — approval executor

`RegisterExecutor(sqlc.SharedRequestTypeMaintenance, maintenanceSvc.Executor())` di `router.go`.
Saat request `maintenance` disetujui penuh: parse payload → `CreateRecord` corrective status
`scheduled` (`problem_category_id`, `description` payload, `reported_by_id` = maker/requested_by,
`scheduled_date` = tanggal approve). Aset **tidak** di-flip di titik ini — flip saat Manager
memulai pekerjaan (`in_progress`), sesuai FR-4.3. Reject → tidak ada row (pola disposal/transfer).

### 1.5 `handler.go` + `routes.go` — endpoint `/api/v1`

| Method & path | Gate | Fungsi |
|---|---|---|
| `POST /maintenance/schedules` | `maintenance.manage` + scope | Buat jadwal |
| `GET /maintenance/schedules` | `maintenance.view` + scope | List jadwal enriched |
| `PATCH /maintenance/schedules/:id` | manage + scope | Ubah interval/kategori/aktif |
| `DELETE /maintenance/schedules/:id` | manage + scope | Soft delete |
| `POST /maintenance/records` | manage + scope | Buat catatan |
| `GET /maintenance/records` | view + scope | List + search `q` + filter status/type |
| `GET /maintenance/records/:id` | view + scope | Detail (slideover edit) |
| `PATCH /maintenance/records/:id` | manage + scope | Edit + transisi status |
| `GET /maintenance/attention` | view + scope | Daftar "Perlu Tindak Lanjut" |
| `POST /maintenance/reports` | `request.create` | Laporan kerusakan → request approval |
| `GET /assets/:id/maintenance` | view + scope | Riwayat maintenance per aset (FR-2.8) |

List mengembalikan `{data,total,limit,offset}`, `limit` clamp 1–100. HTTP mapping via `svcError`
di handler (bukan service). Module string data-scope: `"maintenance"` — konsisten dengan baris
`data_scope_policies` yang di-seed. `GET /assets/:id/maintenance` didaftarkan dari `routes.go`
maintenance (pola `GET /assets/:id/assignments` milik assignment).

### 1.6 Perubahan `internal/stockopname` (kecil)

`GenerateFollowup`: tambah cabang `damaged` → panggil interface kecil yang didefinisikan **di
paket stockopname** (mis. `CorrectiveCreator { CreateFromOpname(...) (uuid.UUID, error) }`),
diimplementasikan `*maintenance.Service`, disuntik via `NewRouter` (tanpa import cycle; pola
komposisi root). Simpan `followup_record_id` di item; item yang sudah punya
`followup_request_id` **atau** `followup_record_id` ditolak (idempoten). Respons mengembalikan
`record_id` + jenis follow-up. `internal/assignment` **tidak diubah** (keputusan #3).

### 1.7 OpenAPI (`backend/api/openapi.yaml`)

Schemas `MaintenanceSchedule`, `MaintenanceRecord`, `MaintenanceAttentionItem` + request bodies;
11 path di atas + perubahan respons followup stock-opname. Spectral wajib 0 error baru.

---

## 2. Frontend — halaman `/maintenance` + integrasi

### 2.1 Halaman `/maintenance` (`app/pages/maintenance.vue`) — 1:1 mockup + deviasi disetujui

- **Banner jatuh tempo** (atas tab): jadwal due ≤ 3 hari termasuk overdue, badge merah
  (terlambat N hari / hari ini) · kuning (≤ 7 hari), tombol "Lihat Jadwal" → pindah tab.
  Hanya tampil bila ada item (sesuai mockup).
- **Seksi "Perlu Tindak Lanjut"** (deviasi disetujui): kartu aset `under_maintenance` tanpa
  catatan aktif (`GET /maintenance/attention`), tombol per item membuka slideover catatan
  ter-prefill (aset terkunci, tipe corrective). Hanya tampil bila ada item dan caller punya
  `maintenance.manage`.
- **Tab Jadwal**: kartu jadwal sesuai mockup (ikon, aset + badge tipe, tugas · vendor, label due
  berwarna + tanggal, "Buat Catatan" ter-prefill aset+kategori+`schedule_id`) + tombol
  "Tambah Jadwal" + slideover jadwal (deviasi disetujui; termasuk edit/nonaktifkan).
- **Tab Catatan**: search (aset/tag/vendor), "Tambah Catatan" → slideover (Aset*, Tipe, Kategori
  Perawatan, Tanggal*, Status, Biaya Rp, Vendor/Teknisi, Deskripsi*), tabel 7 kolom mockup
  (Aset+tag, Tipe badge, Kategori, Tanggal, Status badge+dot, Biaya rata-kanan, Vendor) +
  empty state. Klik baris → slideover mode edit (deviasi disetujui); record terminal
  (completed/cancelled) tampil read-only.
- **Tab Laporan Kerusakan** (gate `request.create`): badge "Tampilan Staf", form (Aset yang Anda
  pegang* — dari assignment aktif milik caller, Kategori Masalah* — reference `problem_categories`,
  Deskripsi, Foto opsional → upload asset-attachment lalu kirim `attachment_id`), tombol submit
  disabled hingga valid, alert sukses 4 detik, catatan info antrean; kolom kanan "Riwayat Laporan
  Saya" (requests `mine` type `maintenance`, nama aset best-effort lookup — pola peminjaman) +
  empty state dashed.
- **Gating**: tab Jadwal+Catatan tampil dengan `maintenance.view`; aksi tulis (tombol tambah,
  edit, tindak lanjut) dengan `maintenance.manage` via `useCan`; tab Laporan dengan
  `request.create`. Item nav "Maintenance" (ikon wrench, grup Operasional) diaktifkan.
- `useMaintenance` composable (`composables/api/maintenance.ts`), `maintenanceMeta` constants
  (tone map status/tipe/due), i18n penuh `id`/`en`, dark mode, format tanggal & Rupiah pakai
  util yang ada.

### 2.2 Integrasi layar lain

- **Detail Aset**: tab "Riwayat Maintenance" (`GET /assets/:id/maintenance`) — tabel ringkas
  tanggal/tipe/kategori/status/biaya/vendor (FR-2.8; mockup Detail Aset).
- **Stock Opname**: tombol tindak lanjut item `damaged` diaktifkan → panggil followup existing;
  toast sukses menyebut catatan maintenance dibuat; item yang sudah ditindaklanjuti disabled.
- **Pengajuan/Approval**: render payload request `maintenance` (aset, kategori masalah,
  deskripsi, foto bila ada) — pola render payload assignment/transfer/disposal.
- **`RequestType` union** ditambah `'maintenance'` **dan** `'assignment'` (melunasi follow-up
  PROGRESS.md; hapus cast lokal di jalur peminjaman).

---

## 3. Testing

**Backend (unit + integration, `-tags=integration`):**
- Jadwal: CRUD happy path; interval < 1 ditolak; recompute `next_due_date` saat interval berubah;
  scope read+write (out-of-scope 404/403); aset disposed ditolak.
- Catatan: create per status awal (scheduled/in_progress/completed) + efek status aset;
  transisi valid semua jalur + invalid (`completed → in_progress`, edit record terminal) ditolak;
  release logic (dua record in_progress → complete satu → aset tetap `under_maintenance`;
  complete keduanya → `available`); `schedule_id` mismatch ditolak; complete dengan `schedule_id`
  → `last_done_date`/`next_due_date` jadwal bergeser; aset `in_transfer` → mulai ditolak;
  scope read+write; search & filter list.
- Laporan + executor: submit happy path (request tercipta, office server-resolved); duplikat
  pending ditolak; aset out-of-scope ditolak; approve (maker ≠ checker, SoD) → record corrective
  `scheduled` tercipta dengan payload benar; reject → tidak ada record.
- Attention: aset ter-flag tanpa record aktif muncul; setelah record dibuat hilang; scoped.
- Stock opname: followup `damaged` sukses → record + `followup_record_id`; followup kedua
  ditolak; out-of-scope ditolak; `not_found`/`misplaced` tidak berubah perilaku (regresi).
- **Full integration gate**: `go test -tags=integration ./...` semua paket (memori repo:
  perubahan lintas paket wajib full run).

**Frontend (Vitest + @nuxt/test-utils):**
- Unit: `maintenanceMeta` tone maps, kalkulasi label due (terlambat/hari ini/N hari), format
  biaya, composable `useMaintenance` (URL & param).
- Runtime mount per tab: loading/empty/error/populated; banner muncul hanya bila ada due ≤ 3 hari;
  seksi attention hanya utk `maintenance.manage`; slideover create (submit disabled sampai valid,
  prefill dari jadwal/attention) & edit (status terminal read-only); form laporan (disabled
  submit, alert sukses, riwayat laporan); variasi permission (tanpa `maintenance.view` → hanya
  tab Laporan; view-only → tanpa tombol tulis); union `RequestType` (typecheck).
- Konsumen API lain yang ter-rewire di-stub (memori repo: wiring composable memecahkan test
  konsumen); exit code full suite diperiksa.

**E2E (Playwright, real backend, uniqueness per run + assert-after-search + wait-modal-closed):**
1. Manager: buat jadwal → tampil di tab Jadwal → "Buat Catatan" dari jadwal → complete →
   `next_due_date` bergeser & aset kembali `available`.
2. Staf: lapor kerusakan via UI → "Riwayat Laporan Saya" berisi → approve via API sebagai
   Manager office-level kedua (maker ≠ checker) → record muncul di tab Catatan.
3. Negative: submit laporan tanpa kategori ditolak (button disabled).

**Gate akhir:** `go build/vet/test` + full integration + Spectral + `pnpm lint/typecheck/test/build`
+ e2e; side-by-side 1:1 vs `Maintenance.dc.html` (light & dark); update `PROGRESS.md` (checkbox
Maintenance + daftar deviasi sesuai bagian "Batasan yang jujur dicatat" + blok "Next session") dalam
PR yang sama.

## Urutan implementasi (ringkas)

1. Migrasi `000027` + sqlc + `catalog.go`.
2. `internal/maintenance` service + dto + handler + routes + executor; wire `NewRouter`.
3. Perubahan `internal/stockopname` (followup damaged) via interface.
4. OpenAPI + Spectral.
5. Backend tests (unit + integration penuh).
6. Frontend: composable + meta + i18n + halaman `/maintenance` + Detail Aset tab + stock-opname
   wiring + union `RequestType`.
7. Frontend tests (unit + runtime) + e2e.
8. Gate sweep + side-by-side mockup + PROGRESS.md + PR.
