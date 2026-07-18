# Desain: Modul Import Massal (Aset + Master Data)

- **Tanggal:** 2026-07-12
- **Status:** Disetujui (brainstorming session)
- **Referensi:** PRD FR-2.11 (import massal aset), FR-7.5b (import massal master data),
  DATABASE.md bagian 4.5 (`import.import_jobs`), mockup `docs/design/Import Aset.dc.html`,
  ADR-0008 (konvensi modul backend).

## 1. Ringkasan

Membangun modul import massal CSV/XLSX end-to-end: backend `internal/importer` (engine generik +
`TargetImporter` per target) dan wiring frontend wizard `pages/assets/import.vue` (plus entry point
master data), menggantikan seam mock (`mock/assets.ts` dihapus).

**Cakupan target:** `asset` (FR-2.11), `employee`, `office`, `reference:provinces`,
`reference:cities` (FR-7.5b).

**Keputusan arsitektur kunci (disetujui user):**

1. **Batch aset = SATU approval request** — request type baru `asset_import`; threshold maker-checker
   dinilai dari **total nilai batch**; executor membuat semua aset saat disetujui. Master data
   dieksekusi langsung (CRUD master data memang tanpa approval).
2. **Pemrosesan async worker + polling** — job antri di DB, worker goroutine memproses, frontend
   poll status + progress (Redis).
3. **Engine generik + Importer per target** — plumbing (upload MinIO, parsing, job lifecycle,
   template, error report) ditulis sekali; tiap target implement interface `TargetImporter` di
   package domainnya (meniru pola approval-executor).
4. **Satu batch = satu kantor** — semua baris kolom `kantor` harus sama (routing approval
   `approval_requests.office_id` tunggal; pola operasional bank upload per cabang). Lintas kantor =
   error validasi.

## 2. Data model — migration `000030_import_module`

### 2.1 Perluasan enum

- `shared.import_status` += `validated`, `confirmed`, `executing`, `awaiting_approval`, `cancelled`
  (nilai lama `pending/processing/completed/failed` tetap).
- `shared.request_type` += `asset_import`.

### 2.2 Kolom baru `import.import_jobs`

| Kolom | Tipe | Fungsi |
|---|---|---|
| `office_id` | `uuid?` FK `masterdata.offices` | Kantor target batch (routing approval + scope) |
| `request_id` | `uuid?` FK `approval.requests` | Terisi saat batch aset diajukan approval |
| `confirmed_at` | `timestamptz?` | Waktu user mengkonfirmasi eksekusi |
| `error_key` | `text?` | Alasan gagal fatal (kunci i18n: file rusak, header salah, dst.) |

`error_report_key` yang sudah ada **dibiarkan null** — laporan error dibangun on-demand dari
`import_rows` (lihat bagian 4, endpoint error-report), tidak disimpan ke MinIO.

### 2.3 Tabel baru `import.import_rows`

```
id uuid PK · job_id uuid FK import_jobs · row_no int · data jsonb (nilai sel per kolom)
· valid bool · errors jsonb (array {column, error_key}) · result_ref text? (asset_tag/kode hasil)
· created_at/updated_at/deleted_at + trigger set_updated_at
· UNIQUE (job_id, row_no) WHERE deleted_at IS NULL · INDEX (job_id, valid)
```

`error_key` per sel = kunci i18n (frontend menerjemahkan) — konsisten dengan pola mock lama
(`assets.import.errors.*`).

### 2.4 State machine job

```
pending ──worker──▶ processing (parse+validasi) ──▶ validated ──confirm──▶ confirmed
   │                        └──▶ failed (fatal: file/header/oversize)
   │        validated/pending ──cancel──▶ cancelled
   ▼
confirmed ──worker──▶ executing ──┬─ master data: insert 1 tx ─▶ completed
                                  └─ asset: submit approval `asset_import`
                                       ─▶ awaiting_approval
                                            ├─ disetujui: executor buat aset ─▶ completed
                                            └─ ditolak: TIDAK ada transisi status —
                                               "rejected" di-derive dari status request_id
                                               saat GET /imports/:id
```

Transisi ilegal (mis. confirm saat `processing`) ditolak dengan sentinel error → 409.

## 3. Otorisasi

- **Membuat import** digate permission `manage` target — tanpa permission `import.*` baru:
  `asset` → `asset.manage`; `employee` → `masterdata.employee.manage`; `office` →
  `masterdata.office.manage`; `reference:*` → `masterdata.global.manage`. Gate di-resolve per
  request dari parameter `target` (handler memetakan target → permission key lalu memanggil
  layanan permission; bukan middleware statis per route).
- **Data scope**: kantor batch (kolom `kantor`) harus dalam `CallerOfficeScope` pengunggah —
  di luar scope = error validasi per-baris. Berlaku juga untuk employee/office import (baris
  menunjuk kantor di luar scope = error).
- **Akses job** (`GET /imports/:id`, `/rows`, `/error-report`, `confirm`, `cancel`): hanya
  **pembuat job**; khusus baca (`GET :id` + `/rows`) juga diizinkan bagi caller yang berhak
  melihat approval request terkait (approver perlu memeriksa isi batch) — cek reuse mekanisme
  scope modul approval.
- `GET /imports` (riwayat) hanya mengembalikan job milik caller.

## 4. Backend — modul `internal/importer`

Folder `internal/importer` (bukan `import` — keyword Go). ADR-0008 four-file split + file khusus:

| File | Isi |
|---|---|
| `service.go` | Job lifecycle (create/get/list/confirm/cancel), sentinel errors, `mapDBError` — Gin-free |
| `dto.go` | Request/response structs, serialisasi job & row |
| `handler.go` | bind → service → respond; sentinel → HTTP (`svcError`) |
| `routes.go` | Mount `/imports` |
| `worker.go` | Worker pool (default 1 goroutine, configurable); poll DB `pending`/`confirmed` dengan `FOR UPDATE SKIP LOCKED` tiap ~2 dtk; recovery startup: `processing`→`pending`, `executing`→`confirmed`; progress ke Redis `import:progress:<job_id>` `{phase, done, total}` TTL 1 jam |
| `parser.go` | CSV (`encoding/csv`) + XLSX (excelize) dari MinIO → `[]RawRow`; header wajib cocok `Columns()`; cap **10.000 baris**, file max **10 MB**, format hanya `.csv`/`.xlsx` |
| `target.go` | Interface `TargetImporter` + registry |
| `template.go` | Generator template CSV/XLSX dari `Columns()` |
| `errreport.go` | Generator laporan baris gagal on-demand (format = format file asal + kolom keterangan) |

### 4.1 Interface

```go
type TargetImporter interface {
    Target() string               // "asset", "employee", "office", "reference:provinces", ...
    Columns() []ColumnSpec        // nama, wajib?, tipe — sumber tunggal template + validasi header
    ValidateRows(ctx context.Context, rows []RawRow, scope Scope) ([]RowResult, error)
    Execute(ctx context.Context, qtx *sqlc.Queries, job Job, validRows []Row) (created int, err error)
    NeedsApproval() bool          // true hanya "asset"
}
```

Implementasi tinggal di package domain, registrasi di `NewRouter`:
`importerSvc.RegisterTarget(assetSvc.Importer())`, `employee.NewImporter(...)`,
`office.NewImporter(...)`, `reference.NewImporter("provinces")`, `reference.NewImporter("cities")`.

### 4.2 Alur eksekusi

- **Master data** (`NeedsApproval() == false`): worker menjalankan `Execute` dalam **satu
  transaksi**; race unik saat insert (kode diserobot pasca-validasi) → baris itu dicatat gagal
  (update `import_rows.valid=false` + `errors`), batch lanjut; counter job diperbarui; job
  `completed`.
- **Asset** (`NeedsApproval() == true`): worker TIDAK membuat aset — submit **satu** approval
  request `asset_import` payload `{job_id, filename, total_rows, total_value, office_id}`;
  `office_id` request = kantor batch; threshold = total `harga` batch; job → `awaiting_approval`
  + `request_id` terisi.
- **Executor `asset_import`** (`internal/asset/executor.go`, register seperti `asset_create`):
  dalam transaksi commit approval — load `import_rows` valid → per baris: pakai `asset_tag` dari
  file bila ada (sudah lolos validasi), selain itu `GenerateAssetTag` → `CreateAsset` → tulis
  `result_ref`; konflik unik saat eksekusi → baris dicatat gagal, batch lanjut (tidak
  membatalkan transaksi); update counter; job `completed`.
- Payload executor memuat `job_id` — defense-in-depth: verifikasi `office_id` payload = office
  request (pola executor yang ada) dan job masih `awaiting_approval`.

### 4.3 Endpoint (`/api/v1/imports`, semua `RequireAuth`)

| Method | Path | Gate | Fungsi |
|---|---|---|---|
| GET | `/imports/template?target=&format=` | permission target | Unduh template csv/xlsx |
| POST | `/imports` (multipart `file`,`target`) | permission target | Upload → MinIO `imports/<job_id>/<filename>` → job `pending` |
| GET | `/imports` `?target=&limit=&offset=` | login | Riwayat job milik caller (`{data,total,limit,offset}`, clamp 1–100) |
| GET | `/imports/:id` | pembuat / approver terkait | Status + counter + progress Redis + status approval derived |
| GET | `/imports/:id/rows?only_errors=&limit=&offset=` | pembuat / approver terkait | Preview paginasi server-side |
| POST | `/imports/:id/confirm` | pembuat | `validated` → `confirmed` |
| POST | `/imports/:id/cancel` | pembuat | `pending/validated` → `cancelled` |
| GET | `/imports/:id/error-report` | pembuat / approver terkait | Unduh baris gagal (on-demand) |

`backend/api/openapi.yaml` diperbarui (tag `Import`).

### 4.4 Kolom & validasi target asset

Kolom (identik dengan mock/template sekarang):
`asset_tag?` · `nama*` · `kategori*` · `kantor*` · `tgl_beli*` · `harga*` · `vendor?` · `lokasi?`

| Aturan | error_key (contoh) |
|---|---|
| Kolom wajib kosong | `required` |
| `kategori`/`kantor`/`vendor` lookup **by kode ATAU nama, case-insensitive**, tidak ketemu | `kat` / `kantor` / `vendor` |
| `lokasi` = nama ruang dalam kantor tsb, tidak ketemu | `lokasi` |
| `tgl_beli` bukan `YYYY-MM-DD` | `tgl` |
| `harga` bukan angka desimal ≥ 0 (string decimal, konvensi money) | `harga` |
| `asset_tag` diisi: format salah / sudah ada di DB / duplikat dalam file | `dupTag` |
| `kantor` berbeda antar baris (aturan satu batch satu kantor) | `multiOffice` |
| `kantor` di luar scope pengunggah | `scope` |

Kelas aset selalu `tangible` (aset intangible dibuat via form satuan — di luar cakupan template).
Kolom validasi target employee/office/reference didefinisikan analog dari field wajib
masing-masing service saat implementasi (sumber kebenaran: input service yang ada), dengan aturan
lookup dan unik yang sama polanya.

## 5. Frontend

- **Composable `composables/api/useImports.ts`** (real `$fetch` via `apiBase`): `uploadImport`,
  `getJob` (poll ~1,5 dtk selama `processing/executing`), `getRows`, `confirmJob`, `cancelJob`,
  `templateUrl`, `downloadErrorReport`, `listJobs`.
- **`app/components/import/ImportWizard.vue`** — wizard 3 langkah reusable (props `target`,
  `permission`); `pages/assets/import.vue` jadi pemakai pertama; anatomi visual identik mockup
  `Import Aset.dc.html`.
- **Step 1**: input file asli (`.csv/.xlsx`, max 10 MB) + Unduh Template wired; setelah upload →
  progress nyata (poll job + Redis progress).
- **Step 2**: preview dari `/rows`, **paginasi server-side**, filter "hanya error", ringkasan
  valid/error; tombol konfirmasi.
- **Step 3**: master data → hasil final (dibuat X / gagal Y, unduh baris gagal). Asset → state
  **"Diajukan untuk persetujuan"** (nomor pengajuan + status) → berubah hasil final setelah
  disetujui (halaman terus poll); ditolak → state ditolak + unduh error report.
- **Resume**: saat halaman dibuka, job aktif milik caller (dari `GET /imports?target=`) membuat
  wizard resume ke step yang sesuai.
- **Entry master data**: tombol Import di layar Pegawai, Kantor, Referensi → wizard dengan target
  masing-masing (halaman `pages/masterdata/import.vue?target=` atau modal — diputuskan saat
  implementasi mengikuti pola layar induk).
- **Approval UI**: entri `asset_import` di `constants/approvalMeta.ts` + `utils/approvalPayload.ts`;
  detail pengajuan menampilkan ringkasan batch + tabel baris (endpoint rows).
- **Permission halaman** `pages/assets/import.vue` diperbaiki: `asset.manage` (sekarang
  placeholder `masterdata.office.manage`).
- **`mock/assets.ts` dihapus** — semua konsumen (assetStore, IMPORT_*) digrep dan dialihkan;
  test konsumen diperbaiki agar tidak menghantam backend nyata tanpa stub.
- i18n `id`/`en` lengkap untuk semua string baru.

### Deviasi mockup yang disetujui (dicatat juga di PROGRESS.md saat landing)

- **(a)** Step 3 target asset menampilkan state "menunggu persetujuan" sebelum hasil final
  (konsekuensi batch-approval; mockup mengasumsikan pembuatan langsung).
- **(b)** Entry point + wizard master data tidak punya mockup — dibangun memakai anatomi wizard
  Import Aset.
- **(c)** Progress bar = progress nyata dari polling (mockup: animasi kosmetik).
- **(d)** Preview divalidasi server & dipaginasi (mockup: tabel statis 12 baris).
- **(e)** Resume job aktif saat halaman dibuka ulang.

## 6. Error handling

- File tak terbaca / header salah / oversize / format salah → job `failed` + `error_key`;
  wizard menampilkan pesan i18n spesifik (bukan toast generik).
- Race unik pasca-validasi (eksekusi) → baris gagal dicatat, batch lanjut (master data & asset).
- Request approval ditolak → job tampil "ditolak" (derived), error report tetap bisa diunduh.
- Worker mati di tengah → recovery reset saat startup (fase transaksional, idempoten).
- Transisi status ilegal → 409; job/target tidak dikenal → 404/422.

## 7. Testing

- **Backend unit**: parser (csv/xlsx, header salah, cap baris/ukuran, sel kosong, tipe salah);
  validator per target (setiap error_key bagian 4.4 termasuk `multiOffice`, `scope`, duplikat file+DB);
  state machine (semua transisi ilegal); template generator; error-report generator.
- **Backend integration** (pola repo, `-tags=integration`): siklus penuh per target
  (upload→validate→rows→confirm→execute→completed); asset: approve → aset tercipta; reject →
  derived rejected; 403 per permission target; scope enforcement (upload beda kantor);
  akses job lintas user (403); approver bisa baca rows; recovery job nyangkut; pagination rows;
  full gate `go test -tags=integration ./... -p 1` hijau.
- **Frontend**: unit `useImports` (stub fetch, semua verb + error); runtime mount `ImportWizard`
  per step + state loading/empty/error/failed-job/rejected; e2e Playwright real-backend: upload
  fixture xlsx (baris valid+invalid, nama/kode unik per run), preview error, confirm, switch ke
  approver (clear cookies+localStorage), approve, verifikasi aset di katalog; skenario employee;
  unduh template + error report.
- **Verifikasi akhir**: seluruh gate CI (backend build/vet/test+integration, Spectral, frontend
  lint/typecheck/test/build), side-by-side wizard vs mockup (light+dark), PROGRESS.md + vault
  Obsidian (Peta Modul, Status & Roadmap, keputusan produk batch-approval + satu-kantor, catatan
  sesi).

## 8. Di luar cakupan

- Import aset intangible (form satuan sudah melayani).
- Penyimpanan error report ke MinIO (`error_report_key`) — on-demand cukup; kolom dibiarkan null.
- Import entitas lain (brand/model/unit/ruang/lantai) — engine extensible, tinggal tambah target.
- Notifikasi (menunggu modul notifications).
- Partisi/arsip `import_jobs` (DB-Q3 PRD — ditinjau saat volume besar).
