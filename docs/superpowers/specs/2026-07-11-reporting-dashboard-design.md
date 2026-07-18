# Modul Reporting & Dashboard — Design

**Tanggal:** 2026-07-11 · **Status:** Disetujui user (brainstorming session)
**Referensi:** PRD bagian 3.5 (FR-5.5–5.7), FR-2.10/FR-7.6 (pengecualian valuasi), FR-3.5 (overdue),
matriks peran bagian 2.2 ("Lihat laporan & dashboard" semua role; "Ekspor laporan" tanpa Staf) ·
mockup `docs/design/Dashboard.dc.html` + `docs/design/Laporan.dc.html` · PROGRESS.md bagian Analytics
(agregasi OLTP dulu; star-schema/CQRS level 2 ditunda) · pola `internal/depreciation/export.go` +
`internal/stockopname/report.go` · ADR-0008 (split modul)

## Tujuan

Menghidupkan halaman Dashboard (`/`) dan Laporan (`/reports`) yang hari ini masih mock-backed:
backend `internal/report` (read-only, agregat + ekspor) di atas tabel OLTP yang sudah ada, wiring
`useDashboard`/`useReports` ke API nyata, dan menghapus `mock/dashboard.ts` + `mock/reports.ts`.
Menutup juga deferral lama: rekap jurnal GL laba/rugi pelepasan (dari modul disposal) dan guard
route `/reports` yang masih memakai permission placeholder.

## Keputusan produk (dikonfirmasi user)

1. **Cakupan penuh 7 jenis laporan** (FR-5.6): 4 dari mockup (aset, depresiasi, utilisasi, biaya
   maintenance) + laporan **mutasi** + laporan **penghapusan/laba-rugi** (termasuk rekap jurnal GL
   pelepasan) + akses **berita acara stock opname** dari halaman Laporan (memakai ekspor per-sesi
   yang sudah ada di modul stockopname).
2. **Periode = preset + rentang kustom.** Preset (30 hari terakhir / bulan ini / kuartal ini /
   tahun berjalan) ditambah opsi rentang tanggal manual (date-range picker berbasis UCalendar).
   Semantik parsial yang benar: metrik periodik (biaya maintenance, maintenance jatuh tempo dalam
   window, tren) mengikuti periode; metrik point-in-time (total aset, nilai perolehan/buku, donut
   status, bar kategori/lokasi) selalu kondisi saat ini.
3. **Select "Scope" mockup → select "Kantor" dalam scope.** Pilihan hanya kantor-kantor di dalam
   data scope caller (default: seluruh scope); backend memvalidasi kantor terpilih ⊆ scope (403
   bila di luar). Role satu kantor praktis tidak melihat pilihan.
4. **Tombol Ekspor dashboard = dropdown PDF / Excel** — snapshot ringkasan dashboard (KPI +
   komposisi + daftar jatuh tempo) sesuai scope & filter aktif.
5. **Panel approval dashboard = aksi nyata inline.** Sumber `GET /requests/inbox` (top 5); ✓
   approve langsung, ✕ membuka modal catatan penolakan; memakai endpoint decide yang sudah ada.
6. **Arsitektur A — agregasi OLTP langsung + cache tipis** (lihat bagian Arsitektur): laporan & semua
   ekspor **selalu fresh** (tanpa cache); hanya `GET /dashboard/summary` yang di-cache Redis TTL
   90 detik. Kontrak API stabil agar backing store bisa naik ke MV/star-schema kelak tanpa
   mengubah frontend.
7. **Aturan nilai:** aset `excluded_from_valuation = true` dikeluarkan dari **semua total nilai**
   (perolehan, buku, akumulasi) tapi tetap dihitung dalam jumlah unit (FR-2.10/FR-7.6). Semua
   penjumlahan uang dilakukan di SQL (`numeric` → `::text`), tidak pernah float di Go.

## Arsitektur — kenapa agregasi OLTP langsung (bukan MV/star-schema)

Data aset tetap bank bersifat low-velocity & bounded (ratusan ribu baris, bukan miliaran event).
Agregat Postgres pada kolom yang **sudah ter-index** (`status`, `category_id`, `office_id`,
`next_due_date`) selesai dalam milidetik. Tangga eskalasi standar: (1) indexed OLTP aggregate →
(2) cache → (3) summary table/MV → (4) OLAP — naik hanya saat pengukuran membuktikan perlu.
MV/star-schema justru memperburuk konsistensi (basi antar-refresh; tidak bisa di-precompute per
kombinasi data-scope role tanpa ledakan varian). Disiplin konsistensi fase ini:

- **Laporan & ekspor (PDF/Excel/jurnal) tidak pernah di-cache** — dokumen akuntansi/audit harus
  dihitung fresh dari satu sumber kebenaran (OLTP).
- **Hanya ringkasan dashboard di-cache** (TTL 90 dtk, self-healing, tanpa logika invalidasi
  lintas modul yang rapuh). Key: `report:dash:<role_id>:<hash officeIDs scope>:<office-filter>:<period>`.
  Pola `cacheGetJSON`/`cacheSetJSON` di-mirror ke package report; Redis bukan source of truth
  (miss/error → hitung DB).
- Dashboard & laporan diturunkan dari query dasar yang sama sehingga tidak saling bertentangan di
  luar jendela TTL.

## 1. Backend — modul `internal/report`

Read-only; **tidak ada tabel baru**. Split ADR-0008: `service.go` / `dto.go` / `handler.go` /
`routes.go` + `export.go` (pure builder xlsx/pdf, pola `depreciation/export.go` +
`stockopname/report.go`; `excelize` + `gofpdf` sudah di dependency tree).

### Migrasi `000029_report_scope_seed`

Hanya seed `identity.data_scope_policies` untuk module `report` (pola migrasi maintenance):
Superadmin→`global`, Kepala Kanwil→`office_subtree`, Kepala Unit→`office_subtree`,
Manager→`office`, Staf→`own`. Permission `report.view`/`report.export` **sudah ter-seed** di
`000005` (Staf punya `view`, tidak punya `export`) dan sudah terdaftar di katalog authzadmin —
tidak ada key baru.

### Endpoint

| Endpoint | Permission | Cache | Isi |
|---|---|---|---|
| `GET /dashboard/summary` | `report.view` | 90 dtk | Semua angka dashboard satu respons |
| `GET /dashboard/export?format=xlsx\|pdf` | `report.export` | — | Snapshot ringkasan |
| `GET /reports/:type` | `report.view` | — | KPI + chart + rows + total per jenis |
| `GET /reports/:type/export?format=xlsx\|pdf` | `report.export` | — | Ekspor laporan |

Query param bersama: `office_id` (opsional, divalidasi ⊆ scope), `period` (preset enum) **atau**
`date_from`+`date_to` (rentang kustom, keduanya wajib bila dipakai, `from ≤ to`); per-jenis:
`category_id`, `status` (hanya `aset`), `basis` (hanya `depresiasi`, default `commercial`).
`:type` divalidasi terhadap whitelist konstanta — bukan input bebas. Scope via
`common.ScopedDeps.CallerOfficeScope(c, "report")`; fallback konservatif `own`. Panel approval
dan BA opname **tidak** menambah endpoint (reuse `GET /requests/inbox`, decide endpoints, dan
ekspor BA stockopname per-sesi).

### `GET /dashboard/summary` — bentuk respons

```
{
  "office_name": string|null,        // nama kantor filter aktif, null = seluruh scope
  "kpi": {
    "total_assets": int,
    "acquisition_value": string,     // SUM purchase_cost, excl. excluded_from_valuation
    "book_value": string,            // SUM book_value, excl. excluded_from_valuation
    "overdue_assets": int,           // assignments aktif dgn due_date < today (FR-3.5)
    "maintenance_due": int,          // schedules aktif dgn next_due_date dlm window periode
    "maintenance_cost": string,      // SUM records.cost completed dlm periode
    "trends": { ... }                // lihat aturan tren di bawah
  },
  "by_status":   [{status, count}],          // 5 status enum
  "by_category": [{category_id, name, count}],  // top 5 by count
  "by_location": [{office_id|room_id, name, count}],  // per kantor (scope >1 kantor) / per lantai-ruang (1 kantor)
  "maintenance_due_list": [{record/schedule id, asset_name, task, due_date}],  // top 3, urut due_date
  "excluded_count": int              // aset terkecuali valuasi (transparansi angka nilai)
}
```

**Aturan tren (jujur, tidak dikarang):** hanya dihitung bila murah & benar —
`maintenance_cost` vs periode sebelumnya sama panjang; `acquisition_value` via `purchase_date`
dalam periode; `book_value` = beban depresiasi periode berjalan dari `depreciation_entries`
(basis komersial). KPI lain memakai teks deskriptif statis seperti mockup ("perlu tindakan",
"dalam 7 hari"). Tren yang pembandingnya kosong (mis. periode sebelumnya tanpa data) → null,
frontend menyembunyikan baris tren.

### Query agregat (`db/queries/report.sql`)

Semua memakai klausa scope standar `(sqlc.arg(all_scope)::boolean OR office_id =
ANY(sqlc.arg(office_ids)::uuid[]))` + `deleted_at IS NULL`; pola `count(*) FILTER (WHERE ...)`
(contoh yang ada: `SessionKpis` stockopname). Kelompok query:

- **Dashboard:** `DashboardAssetKpis` (total, sum perolehan/buku dgn guard exclusion, per status
  via FILTER), `DashboardAssetsByCategory` (JOIN kategori, GROUP BY, top 5), 
  `DashboardAssetsByOffice` / `DashboardAssetsByRoom` (dua granularitas lokasi),
  `DashboardOverdueCount` (assignments aktif lewat due), `DashboardMaintenanceDue` (count +
  list top 3 join asset), `DashboardMaintenanceCost` (periode berjalan + periode pembanding).
- **Laporan:** `ReportAssetRows` (aset + kategori + nilai; filter status/kategori),
  `ReportDepreciationRows` (agregat `depreciation_entries` per periode per basis:
  SUM opening/amount/closing), `ReportUtilizationRows` (per aset: SUM hari pinjam — interval
  `checkout_date`→`checkin_date`/now dipotong ke periode, COUNT peminjaman, % utilisasi =
  hari-pinjam ÷ hari-periode), `ReportMaintenanceCostRows` (per aset: tipe, jml tindakan, total
  biaya; hanya `completed` dlm periode), `ReportTransferRows` (join kantor asal/tujuan + aset),
  `ReportDisposalRows` (join aset; kolom laba/rugi) + `ReportDisposalGlRecap` (rekap per akun GL:
  laba vs rugi pelepasan — menutup deferral modul disposal), `ReportOpnameSessions` (sesi closed
  + KPI selisih, scoped).
- Setiap query laporan punya pasangan KPI/total (`:one`) supaya baris `TOTAL` tfoot dan tiles KPI
  dihitung SQL, bukan dijumlah ulang di Go/frontend.

### `export.go`

Builder murni per jenis (tanpa Gin): `BuildReportXLSX(type, data)` /
`BuildReportPDF(type, data)` + `BuildDashboardXLSX/PDF(summary)`. PDF: header nama perusahaan
(`GetAppSetting("label.company_name")`, fallback konstanta), judul + meta filter (periode/kantor/
kategori), tabel ber-border, baris TOTAL, footer waktu cetak + nama pencetak. XLSX: satu sheet
data tabular (+ sheet "Ringkasan" untuk dashboard & disposal-GL). Handler menyetel
`X-Content-Type-Options: nosniff` + `Content-Disposition: attachment; filename="..."` (pola
`journalExport`). Format hanya `xlsx|pdf` (`parseExportFormat` di-reuse/di-mirror).

## 2. Tujuh jenis laporan

| # | `:type` | Kolom tabel | KPI (3 tiles) | Chart |
|---|---|---|---|---|
| 1 | `assets` | Kode · Nama · Kategori · Harga Beli · Akum. Penyusutan · Nilai Buku | Total Aset · Total Perolehan · Total Nilai Buku | Nilai buku per kategori |
| 2 | `depreciation` | Periode · Nilai Awal · Penyusutan · Nilai Akhir | Penyusutan periode · Akumulasi · Sisa Nilai Buku | Penyusutan per periode |
| 3 | `utilization` | Nama Aset · Kategori · Hari Dipinjam · Jml Peminjaman · Utilisasi | Rata utilisasi · Aset aktif dipinjam · Total hari pinjam | Rata utilisasi per kategori |
| 4 | `maintenance` | Aset · Kategori · Tipe · Jml Tindakan · Total Biaya | Total biaya · Preventive · Corrective | Biaya per kategori |
| 5 | `transfers` | Aset · Dari → Ke Kantor · Status · Tgl Kirim · Tgl Terima · No. BAST | Total mutasi · Dalam pengiriman · Selesai | Mutasi per kantor tujuan |
| 6 | `disposals` | Aset · Metode · Tanggal · Nilai Buku · Hasil · Laba/Rugi | Total pelepasan · Total hasil · Total laba/rugi | Laba/rugi per metode |
| 7 | `opname` | Sesi · Kantor · Periode · Total Item · Selisih · Status | Sesi selesai · Total item dicek · Total selisih | Selisih per sesi |

Jenis 5–7 tidak punya mockup → mengikuti anatomi kartu/KPI/chart/tabel jenis 1–4 persis
(deviasi disetujui user). Jenis 7: setiap baris sesi punya tombol unduh **BA PDF/Excel** yang
memanggil endpoint ekspor stockopname per-sesi yang sudah ada; ekspor level-laporan `opname`
mengekspor tabel daftar sesi itu sendiri. Jenis 6: tombol ekspor tambahan **"Rekap Jurnal GL"**
(xlsx/pdf) via `GET /reports/disposals/export?format=...&variant=gl_recap` (param `variant`
default `table`; `gl_recap` hanya valid untuk `disposals`, selainnya 422) dari
`ReportDisposalGlRecap`.

## 3. Frontend

### Dashboard (`app/pages/index.vue`)

- `useDashboard` di-rewire: `summary(officeId, period)` → `GET /dashboard/summary`; hapus
  `mock/dashboard.ts`; `utils/dashboard` (buildDonut/barWidths/formatCount) tetap dipakai.
- Select Scope → **select Kantor** (opsi dari `useOffices().list()` yang sudah scoped; item
  pertama "Seluruh scope saya"); disembunyikan bila hanya 1 kantor dalam scope.
- Select Periode: 4 preset + item "Rentang kustom…" → popover UCalendar **range**; label kontrol
  menampilkan rentang aktif.
- Tombol Ekspor → `UDropdownMenu` PDF/Excel → unduh dari `/dashboard/export` (gate
  `report.export` via `useCan`; tanpa permission → tombol disembunyikan).
- Panel maintenance: dari `maintenance_due_list`; "Lihat semua" → `/maintenance`.
- Panel approval: dari `useApproval().inbox()` top 5 + count badge; ✓ approve → decide endpoint +
  refresh panel + toast; ✕ → modal catatan penolakan (wajib isi) → reject; empty-state sesuai
  mockup; panel disembunyikan bila caller tanpa `request.decide`.
- Semua state: skeleton loading (sudah ada), error+retry (baru — mockup tidak punya, pola standar
  halaman lain), empty.

### Laporan (`app/pages/reports.vue`)

- `useReports` di-rewire: `run(type, filters)` → `GET /reports/:type`; hapus `mock/reports.ts`.
- Guard route diperbaiki: `permission: 'masterdata.office.manage'` → **`'report.view'`**.
- Grid kartu 4 → **7** (baris kedua, gaya identik); filter bar: Periode (preset + rentang kustom),
  Kantor (scoped), Kategori, + Status (hanya `assets`), + Basis komersial/fiskal (hanya
  `depreciation`).
- Tombol Ekspor PDF/Excel nyata (unduh blob + `Content-Disposition`); di-gate `report.export`
  (Staf tidak melihatnya — matriks PRD); tombol "Rekap Jurnal GL" khusus `disposals`.
- State: placeholder pra-Apply, loading, empty + "Atur Ulang Filter", error+retry, results.
- i18n id/en untuk semua string baru; tidak ada teks hardcode.

## 4. Keamanan & otorisasi

- `RequirePermission("report.view")` / `("report.export")` per endpoint; scope
  `CallerOfficeScope(c, "report")` dipaksakan di **setiap** query (read-only module — semua verb
  adalah read).
- `office_id` filter divalidasi ⊆ scope caller di handler (`common.InScope`) → 403 di luar scope.
- Angka nilai (perolehan/buku) tunduk pada aturan exclusion valuasi; **tidak ada** kolom sensitif
  lain yang terekspos (respons agregat, bukan row detail user).
- Field-permission `book_value`/`purchase_cost` per-role **tidak** diberlakukan pada agregat fase
  ini (agregat bukan record individual; `report.view` + data-scope adalah gate-nya) — dicatat
  sebagai keputusan; bila kelak ada role yang nilai uangnya harus dibutakan, tambahkan
  `FilterView` pada blok KPI.
- Ekspor mencantumkan nama pencetak + waktu (jejak audit dokumen).

## 5. Pengujian

- **Backend unit:** builder ekspor (xlsx bisa dibuka ulang via excelize, isi sel diverifikasi;
  PDF non-kosong + parse header), resolusi periode (preset → rentang; kustom valid/invalid),
  whitelist `:type`, `parseExportFormat`.
- **Backend integration (testcontainers):** tiap query agregat dengan data seed terarah —
  scope subtree vs office vs own; `excluded_from_valuation` keluar dari nilai tapi masuk count;
  overdue & maintenance-due window; utilisasi terpotong periode; disposal GL recap; `office_id`
  luar scope → 403; Staf tanpa `report.export` → 403 di endpoint ekspor; cache dashboard (hit
  kedua tidak menyentuh DB — atau minimal hasil identik + TTL terpasang).
- **Frontend unit:** helper periode/format, meta laporan.
- **Frontend component (`mountSuspended`):** kedua halaman semua state (loading/error/empty/
  populated), modal tolak (wajib catatan), dropdown ekspor, gate `report.export` (Staf),
  select kantor tersembunyi saat 1 kantor, rentang kustom.
- **E2E real-backend:** dashboard — KPI terisi dari data seed, approve inline dari panel (maker ≠
  checker via API), export unduh; laporan — pilih jenis → Terapkan → rows + TOTAL, ekspor xlsx
  terunduh, jenis `opname` unduh BA sesi. Ikuti konvensi e2e persistent-data (nama unik per run,
  assert-after-search).
- **OpenAPI** disinkronkan (tag Report, 4 path + skema) + Spectral hijau; semua gate CI (backend
  build/vet/test/integration, frontend lint/typecheck/test/build) sebelum commit.

## 6. Deviasi mockup (disetujui user, konvensi catat-deviasi)

- **(a)** select "Scope" (alat simulasi demo) → select **"Kantor"** berisi kantor dalam scope.
- **(b)** select Periode mendapat opsi **"Rentang kustom…"** (UCalendar range) — permintaan user.
- **(c)** tombol Ekspor dashboard menjadi **dropdown PDF/Excel** (mockup: satu tombol tanpa
  perilaku terdefinisi).
- **(d)** 3 kartu laporan baru (mutasi/disposal/opname) **tanpa mockup** — mengikuti anatomi
  kartu 1–4; halaman Laporan jadi 7 kartu.
- **(e)** laporan depresiasi mendapat filter **Basis komersial/fiskal** (PRD FR-5.6 dua basis;
  mockup tidak memuatnya).
- **(f)** tolak inline di dashboard membuka **modal catatan** (mockup: tombol ✕ polos; catatan
  penolakan adalah kontrak decide yang ada).
- **(g)** state **error + retry** ditambahkan di kedua halaman (mockup tidak punya state error;
  konvensi seluruh app).
- **(h)** tren KPI yang tidak bisa dihitung jujur memakai teks deskriptif statis; yang bisa
  (biaya, perolehan, depresiasi) dihitung nyata — angka tren mockup adalah fixture.

## 7. Batasan jujur (follow-up, bukan fase ini)

- **Snapshot historis nilai aset** (nilai per akhir periode lampau) tidak didukung — metrik nilai
  selalu point-in-time; rekonstruksi historis menunggu fase Analytics/OLAP.
- **Badge count sidebar global** (Pengajuan/Mutasi) tetap deferral terpisah — panel approval
  dashboard punya count sendiri tapi tidak mengisi badge nav (butuh store global lintas halaman).
- **Utilisasi** dihitung dari assignment (check-out/in) saja; peminjaman yang ditolak/pending
  tidak dihitung.
- Kolom **Pemegang** dan detail per-aset lain di luar kolom laporan yang ditetapkan tidak
  ditambahkan (jaga paritas mockup).
