# Modul Depresiasi Dual-Basis (PSAK 16 + PMK 72/2023) — Design

**Tanggal:** 2026-07-05 · **Status:** Disetujui user (brainstorming session)
**Referensi:** PRD bagian 3.5 (FR-5.1–5.8) + Lampiran A.1 (parameter PMK 72/2023 terverifikasi) ·
DATABASE.md bagian 4.4/bagian 7/DB-Q7 · mockup `docs/design/Depresiasi.dc.html` · DESIGN_BRIEF bagian 6.4 ·
ADR-0010 (background job — staged adoption) · pola modul PR #48/#49

## Tujuan

Menghidupkan engine penyusutan dua basis (komersial PSAK 16 + fiskal PMK 72/2023) yang mengisi
read model `depreciation.depreciation_entries` (sudah ada, belum pernah terisi), membangun layar
`/depreciation` 1:1 dari mockup, dan menyalakan integrasi yang menunggunya: nilai buku server-side
di disposal (menghapus caveat maker-supplied), nilai fiskal di layar Disposal, dan tab Depresiasi
di Detail Aset. `assets.book_value`/`accumulated_depreciation` — yang hari ini tidak pernah
ditulis kode mana pun — mulai dipelihara oleh engine.

## Keputusan produk (dikonfirmasi user)

1. **Run model: manual** — Superadmin "Hitung Periode" (idempotent, boleh diulang selagi belum
   ditutup) → review → "Tutup Periode" (final). Banner pengingat saat periode berjalan belum
   dihitung. Otomatisasi = tahap 2/3 ADR-0010 (bukan fase ini).
2. **Ekspor jurnal penuh sekarang** — `.xlsx` (dependensi baru `excelize`) + PDF (reuse infra
   gofpdf label).
3. **Integrasi disposal keduanya** — `book_value_at_disposal` dihitung server (input maker
   dihapus dari kontrak) DAN basis amount approval disposal → nilai buku komersial server-side
   (fallback `purchase_cost` bila aset belum punya entri — konservatif). Transfer tetap
   `purchase_cost`.
4. **Kartu depresiasi di layar Laporan tetap mock** (fase Reporting sendiri).
5. **Read model Postgres, bukan OLAP** — run depresiasi adalah proses akuntansi transaksional
   (atomik, immutable pasca-tutup, auditable) → wilayah OLTP; OLAP/fact tables diturunkan darinya
   di fase Analytics (sudah terencana di PROGRESS).
6. **Aset tersusut penuh tetap tampil** di jadwal (beban 0, nilai akhir = residu) agar KPI jujur;
   preview "Aset disusutkan: n" hanya menghitung beban > 0.
7. **Nilai memorial Rp 1 didukung** via `salvage_value` per aset (floor mekanisme yang sama);
   perubahan estimasi residu berlaku **prospektif by construction** (algoritme iteratif). Nilai
   buku tidak pernah bisa diedit langsung — satu-satunya penurunan manual adalah impairment.

## Batasan yang jujur dicatat (follow-up, bukan fase ini)

- **Revisi masa manfaat via UI** (perubahan estimasi PSAK — prospektif): engine iteratif sudah
  siap menghitungnya, tapi UI/endpoint pengubah parameter aset belum dibangun.
- **Impor saldo awal migrasi**: aset lama di-backfill penuh seolah sistem ada sejak tanggal beli —
  benar bila parameter = pembukuan lama; impor akumulasi historis yang berbeda belum didukung.
- **Kebijakan Rp-1 flat di level kategori**: `default_salvage_rate` berbentuk rasio; nilai-tetap
  kategori menyusul bila kebijakan BTN memintanya (masuk item "Confirm with bank policy").

## 1. Backend — migrasi `000023_depreciation_periods`

```sql
CREATE TYPE shared.depreciation_period_status AS ENUM ('open', 'computed', 'closed');

CREATE TABLE depreciation.depreciation_periods (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  period       date NOT NULL,            -- hari pertama bulan
  status       shared.depreciation_period_status NOT NULL DEFAULT 'open',
  computed_at  timestamptz,
  computed_by  uuid REFERENCES identity.users (id),
  closed_at    timestamptz,
  closed_by    uuid REFERENCES identity.users (id),
  asset_count  int NOT NULL DEFAULT 0,   -- ringkasan run terakhir: aset dgn beban > 0
  total_amount numeric(18,2) NOT NULL DEFAULT 0, -- total beban komersial run terakhir
  skipped_count int NOT NULL DEFAULT 0,  -- aset dilewati (parameter tak lengkap dsb.)
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz
);
CREATE UNIQUE INDEX uq_depr_period ON depreciation.depreciation_periods (period) WHERE deleted_at IS NULL;
-- + trigger set_updated_at (konvensi)
CREATE INDEX idx_depr_basis_period ON depreciation.depreciation_entries (basis, period);
```

Seed (idempotent, pola 000017): `app_settings` key `depreciation.accumulated_gl_account`
(placeholder `1.2.9.001`, description jelas) — akun kredit rekap jurnal. Seed RBAC: permission
**`depreciation.view`** ("Lihat depresiasi") + **`depreciation.manage`** ("Jalankan & tutup
periode depresiasi, catat impairment") — keduanya Superadmin-only sesuai PRD bagian 2.1; data-scope
module string `"depreciation"` (default per-role berlaku). Down migration membalik semuanya
(kolom/tabel/type; permission rows ikut pola down seed yang ada).

## 2. Backend — modul `internal/depreciation` (ADR-0008 + engine terpisah)

File: `engine.go` (kalkulasi murni, tanpa DB — unit-testable), `service.go`, `dto.go`,
`handler.go`, `routes.go`, ditambah `export.go` (xlsx/pdf) dan queries `db/queries/depreciation.sql`.

### engine.go — aturan kalkulasi (pure functions)

- **Input per aset**: `purchase_cost`, `purchase_date`, parameter komersial resolved
  (method = asset.depreciation_method ?? category.default_depreciation_method;
  life = asset.useful_life_months ?? category.default_useful_life_months;
  salvage = asset.salvage_value ?? round(cost × category.default_salvage_rate)),
  parameter fiskal (group = asset.fiscal_group ?? category.default_fiscal_group; life & tarif
  dari **tabel konstanta PMK 72/2023**), `impairment_loss`, entri existing terakhir per basis.
- **Konstanta PMK 72/2023** (Lampiran A.1): kelompok_1 48 bln (GL 25%/SM 50%), kelompok_2 96
  (12,5%/25%), kelompok_3 192 (6,25%/12,5%), kelompok_4 240 (5%/10%), bangunan_permanen 240
  (GL 5% saja), bangunan_non_permanen 120 (GL 10% saja), non_susut → tanpa entri fiskal.
  Fiskal **tanpa residu**; bangunan wajib garis lurus (method saldo menurun pada bangunan →
  **fallback ke garis lurus**, bukan skip — sesuai implementasi engine); metode fiskal mengikuti
  metode komersial aset bila valid untuk kelompoknya, else garis lurus. Bila `salvage_value` aset
  DAN `default_salvage_rate` kategori sama-sama kosong, residu komersial default 0.
- **Konvensi mulai**: bulan `purchase_date` (full-month).
- **Algoritme iteratif per (aset, basis)**: mulai dari bulan setelah entri terakhir (atau bulan
  perolehan), jalan bulan-demi-bulan hingga periode target:
  - opening = closing entri sebelumnya (atau cost).
  - **Garis lurus**: amount = (opening − salvage) / sisa_bulan — bentuk ini membuat perubahan
    estimasi (residu/impairment) otomatis prospektif.
  - **Saldo menurun**: amount = opening × (tarif_tahunan/12); komersial: tarif = 2/umur_tahun,
    floor di salvage; fiskal: tarif dari tabel PMK, **bulan terakhir masa manfaat menyerap seluruh
    sisa** (disusutkan sekaligus).
  - closing = opening − amount; bulan terakhir menyerap sisa pembulatan → closing akhir = salvage
    (komersial) / 0 (fiskal) **persis**.
  - Berhenti bila umur habis atau closing sudah = salvage (aset tersusut penuh → tidak ada entri
    baru).
  - Aritmetika `math/big.Rat`, dibulatkan half-up 2 desimal per bulan.
- **Dilewati (skipped, dilaporkan per run)**: `capitalized=false`, tanpa `purchase_cost` atau
  `purchase_date`, parameter komersial tak lengkap (method/life nil setelah fallback), status
  `disposed` (tidak menambah entri setelah bulan disposal). `excluded_from_valuation` TETAP
  disusutkan (flag itu soal valuasi laporan, bukan penyusutan). Intangible = jalur sama (istilah
  amortisasi di UI).

### service.go — orkestrasi

- `ComputePeriod(ctx, period, actor)`: guard periode tidak `closed`; **`pg_advisory_xact_lock`**
  (key konstanta modul) di dalam transaksi; hapus entri **non-closed** yang akan ditulis ulang
  (delete per aset per periode yang di-regenerate — periode `closed` tidak pernah disentuh, entri
  di dalamnya menjadi basis lanjutan); jalankan engine per aset (catch-up hingga `period`);
  insert entri; update ringkasan `assets.accumulated_depreciation` + `book_value`
  (= cost − akum − impairment, floor salvage) untuk basis komersial; upsert row periode →
  `computed` + ringkasan (asset_count/total/skipped); audit `depreciation.compute`.
- `ClosePeriod(ctx, period, actor)`: hanya dari `computed`; **sekuensial** — semua periode lebih
  awal yang punya entri harus `closed`; set `closed` + audit. Setelah closed, compute untuk
  periode itu ditolak `ErrPeriodClosed` (409).
- `Schedule(ctx, scope, period, basis, filter)`: baris per aset (semua aset terkapitalisasi
  ber-parameter dalam scope — **termasuk tersusut penuh**, beban 0) + KPI (total perolehan/akum/
  nilai buku/beban periode) + total footer; join nama kategori/kantor (pola enriched PR #48).
- `Journal(ctx, scope, period, basis)`: agregasi entri periode per `categories.gl_account_code`
  (debit "Beban Penyusutan — {kategori}"; aset tanpa GL → baris "(tanpa akun GL)") + satu baris
  kredit "Akumulasi Penyusutan" (akun dari app_settings) = total; balanced by construction.
- `AssetSchedule(ctx, assetID)`: seluruh entri kedua basis (untuk tab Detail Aset).
- `RecordImpairment(ctx, assetID, recoverable, reason, actor)`: guard `recoverable >= 0` dan
  `< book_value` saat ini (404/422/409 sesuai sentinel); dalam transaksi: `impairment_loss +=
  (book_value − recoverable)`, `book_value = recoverable`; audit dengan diff; **hanya basis
  komersial yang terpengaruh** (fiskal tidak mengakui impairment); periode berjalan yang sudah
  `computed` (belum closed) boleh di-recompute setelahnya.
- `Periods(ctx)`: daftar periode + status; sisipkan periode berjalan (bulan kalender saat ini)
  bila belum ada row → status virtual `open` (banner pengingat frontend membaca ini).

### Endpoint & permission

| Route | Gate |
|---|---|
| `GET /depreciation/periods` | `depreciation.view` |
| `POST /depreciation/periods/{YYYY-MM}/compute` | `depreciation.manage` |
| `POST /depreciation/periods/{YYYY-MM}/close` | `depreciation.manage` |
| `GET /depreciation/schedule?period=&basis=&search=&category_id=&office_id=` | `depreciation.view` + scope `depreciation` |
| `GET /depreciation/journal?period=&basis=` | `depreciation.view` + scope |
| `GET /depreciation/journal/export?period=&basis=&format=xlsx\|pdf` | `depreciation.view` + scope |
| `GET /assets/{id}/depreciation` | `asset.view` + scope aset; bila field-permission men-deny `book_value` untuk role → 200 `{masked:true, entries:[]}` |
| `POST /assets/{id}/impairment` | `depreciation.manage` + scope |

Ekspor: xlsx via `github.com/xuri/excelize/v2` (dependensi baru); PDF reuse pola gofpdf modul
label (header bank + tabel D/K + total + catatan seimbang).

## 3. Integrasi disposal (perubahan kontrak)

- `POST /disposals`: field `book_value_at_disposal` **dihapus dari SubmitRequest** (OpenAPI ikut).
  Server menghitung: nilai buku komersial as-of tanggal pelepasan = closing entri komersial
  terakhir ≤ bulan pelepasan (fallback: `purchase_cost` bila belum ada entri sama sekali — mis.
  `non_susut`/belum pernah di-run). Nilai itu masuk payload → executor → row (gain_loss SQL tetap).
- **Amount approval disposal** = nilai buku hasil hitungan yang sama (fallback purchase_cost) —
  band otorisasi kini mencerminkan dampak penghapusan riil. Kartu "Jenjang Persetujuan" di layar
  Disposal menyesuaikan: subtitle → **"berdasar nilai buku"** (i18n diperbarui) dan preview
  memakai nilai buku dari endpoint baru `GET /assets/{id}/depreciation` ringkas — sederhananya:
  frontend memanggil preview dengan nilai yang SAMA yang akan dipakai server (tambahkan
  `book_value_commercial` ke response `GET /assets/{id}/depreciation` atau field ringkas pada
  asset response — keputusan implementasi: **expose `computed_book_value` di response
  `GET /assets/{id}/depreciation`** dan layar disposal memakainya untuk preview + tampilan).
- Layar Disposal: Ringkasan Valuasi FISKAL & "Laba/rugi fiskal" jadi riil (nilai buku fiskal =
  cost − Σ entri fiskal, dari endpoint yang sama); tooltip "menunggu modul depresiasi" dihapus.
- Transfer TIDAK berubah (amount tetap purchase_cost — dicatat).

## 4. Frontend — layar `/depreciation` (1:1 mockup)

- Nav "Depresiasi" (en "Depreciation", ikon `i-lucide-trending-down`,
  `permission: 'depreciation.view'`) setelah "Penghapusan", sebelum Maintenance.
- Header + subtitle mockup; **toggle basis** segmented (Komersial chip "PSAK 16" / Fiskal chip
  "PMK 72/2023") — semua data di halaman mengikuti basis.
- **4 KPI**: Total Nilai Perolehan (sub: jumlah aset), Akumulasi Penyusutan (sub: referensi
  basis), Nilai Buku (sub: bulan berjalan), Beban Periode Berjalan (merah) — dari endpoint
  schedule.
- **Panel Jalankan Periode**: select periode (dari `GET /periods`; disabled bila bukan open),
  badge status (Terbuka warning / Terhitung info / Ditutup neutral), tombol Hitung Periode (open)
  / Tutup Periode (computed) / badge Periode Ditutup (closed), preview "Aset disusutkan: n" +
  "Total beban: Rp" + note hijau "Sudah dihitung — tinjau lalu tutup periode.", **banner
  pengingat** bila periode berjalan berstatus open & belum pernah dihitung. Aksi digate
  `depreciation.manage` (disabled + note bila tidak punya, pola layar transfer).
- **Tab Jadwal per Aset**: filter search/kategori/kantor; kolom persis mockup (Aset+tag+ikon
  impaired violet, Metode badge GL/SM, Masa (bln), Nilai Awal, Beban Periode merah, Akumulasi,
  Nilai Akhir, Aksi); baris tersusut penuh beban 0; footer TOTAL; empty state; aksi baris
  "Catat Penurunan Nilai" (gate manage) → **modal Impairment** persis mockup (nilai buku saat
  ini, input Nilai Terpulihkan, baris Rugi Penurunan Nilai merah, Alasan, Simpan & Sesuaikan) →
  `POST /assets/:id/impairment` → refresh. Impairment hanya relevan basis komersial — saat toggle
  Fiskal, kolom Aksi tetap ada namun aksi disabled dengan tooltip (fiskal tidak mengakui
  impairment).
- **Tab Rekap Siap-Jurnal**: subtitle + tombol Ekspor PDF/Excel (hidup → download); kartu Jurnal
  Penyusutan (subtitle basis · bulan); tabel Kode Akun/Nama Akun/Debit/Kredit + TOTAL + banner
  hijau "Jurnal seimbang — debit = kredit."
- String i18n id/en dari tabel CH mockup; istilah "Amortisasi" dipakai pada baris aset intangible
  (kolom metode badge tetap).
- **Tab Depresiasi di Detail Aset** (`assets/[tag]/index.vue`): ganti empty-state generik dengan
  tabel jadwal (kolom periode/nilai awal/beban/nilai akhir/metode) + toggle basis kecil; sumber
  `GET /assets/:id/depreciation`; `masked:true` → tampilan masked (konsisten masking money);
  empty (belum ada entri) → keterangan "belum pernah dihitung".
- Composables: `useDepreciation` (periods/compute/close/schedule/journal/exportJournal/
  assetSchedule/recordImpairment); konstanta `depreciationMeta` (status periode tone, basis meta).

## 5. Testing

- **Unit engine (paling ekstensif di fase ini)** — table-driven: GL/SM × komersial/fiskal;
  konvensi bulan mulai; catch-up multi-tahun (aset 2020); bulan terakhir menyerap pembulatan
  (closing = salvage persis); floor salvage; fiskal tanpa residu + SM sekaligus di akhir;
  bangunan GL-only (SM → skipped); non_susut; capitalized=false; parameter tak lengkap → skipped;
  intangible; impairment mid-life (prospektif, sisa disebar ulang); perubahan residu mid-life
  (prospektif); nilai memorial Rp 1; aset tersusut penuh → tidak ada entri baru; presisi Rat/
  rounding half-up.
- **Integration**: state machine periode (open→computed→closed; compute ditolak setelah closed;
  close sekuensial; close butuh computed); idempotensi recompute (jalankan 2× → entri identik,
  UNIQUE tidak bentrok); advisory lock (2 goroutine compute bersamaan → satu menunggu, hasil
  konsisten); ringkasan assets ter-update; disposal book-value server-side (as-of date; fallback
  purchase_cost; maker tidak bisa menyuntik nilai — field dihapus); amount approval = nilai buku;
  journal balanced + grup GL + baris tanpa-GL; asset schedule + masked response; impairment
  (guard, akumulasi, audit, fiskal tak berubah); permission gates + scope pada semua endpoint;
  export xlsx/pdf menghasilkan file valid (magic bytes + isi minimal).
- **Component** (`mountSuspended`): semua state mockup — 3 status periode × 2 basis, banner
  pengingat, KPI, jadwal (impaired icon, tersusut penuh beban 0, filter, empty), jurnal + ekspor,
  modal impairment (validasi, loss preview, submit), gate manage (disabled+note), loading/error
  di tiap fetch; tab Detail Aset (entri, masked, kosong); layar Disposal yang diperbarui (FISKAL
  riil, subtitle jenjang "berdasar nilai buku", book_value tak lagi dikirim).
- **E2E real backend**: seed aset ber-cost via alur maker-checker → `/depreciation`: compute
  periode berjalan → KPI & jadwal terisi → jurnal seimbang → ekspor xlsx respon file → close
  periode → status Ditutup + compute ditolak; impairment pada satu aset → nilai buku turun +
  ikon; disposal aset ber-entri → nilai buku & gain/loss riil di riwayat. Update
  `disposals.spec.ts` yang ada bila asersinya menyentuh kontrak yang berubah.
- Side-by-side `Depresiasi.dc.html` light+dark; OpenAPI penuh (schemas DepreciationPeriod/
  ScheduleRow/JournalRow + paths + perubahan DisposalSubmitRequest); Spectral 0 error.

## 6. Deviasi mockup (butuh catatan PROGRESS)

(a) baris aset tersusut penuh ditampilkan dengan beban 0 (mockup tak mencontohkan kasus ini —
penambahan demi KPI jujur); (b) aksi impairment disabled saat basis Fiskal aktif (mockup tidak
membedakan); (c) banner pengingat "periode belum dihitung" (penambahan, bagian keputusan run
manual); (d) kolom `book_value_at_disposal` hilang dari form Disposal (server-computed — kontrak
berubah); (e) subtitle kartu Jenjang Persetujuan disposal berubah "berdasar nilai perolehan" →
"berdasar nilai buku".

## 7. Definition of done

1. Gates penuh hijau (backend build/vet/test + full integration; Spectral; frontend
   lint/typecheck/test/build; full e2e workers=1).
2. Side-by-side mockup light+dark 1:1 kecuali deviasi bagian 6.
3. PROGRESS.md: item Dual-basis depreciation + Journal-ready export dicentang; follow-up disposal
   basis-switch DITUTUP; batasan (revisi masa manfaat UI, saldo awal migrasi, Rp-1 kategori)
   tercatat; item Scheduler diperbarui merujuk ADR-0010 tahap 2/3; deviasi bagian 6; pointer next
   session di-refresh.
4. ADR-0010 committed; OpenAPI sinkron; tidak ada dead query/i18n yatim baru.
