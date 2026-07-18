# Layar Mutasi Aset + Penghapusan Aset (wired) — Design

**Tanggal:** 2026-07-05 · **Status:** Disetujui user (brainstorming session)
**Referensi:** mockup `docs/design/Mutasi Aset.dc.html` + `docs/design/Penghapusan Aset.dc.html` ·
DESIGN_BRIEF bagian 6.1 & bagian 6.3 · backend `internal/transfer` + `internal/disposal` (PR #44/#45) ·
pola wiring approval (PR #48)

## Tujuan

Bangun dua layar frontend baru 1:1 dari mockup dan wire ke backend yang sudah lengkap:
**Mutasi Aset** (`/transfers`) dan **Penghapusan Aset** (`/disposals`). Alur maker-checker
end-to-end sudah hidup (PR #48), jadi kedua layar bisa diuji penuh submit→approve→aksi lanjutan.

## Keputusan produk (semua sudah dikonfirmasi user)

1. **`condition_sent` + `transfer_date` ditambahkan ke backend** (migrasi + payload + executor) —
   bukan di-drop dari form.
2. **Endpoint `POST /transfers/:id/reject-receive` ditambahkan** — status baru `returned`; aset
   tidak berpindah.
3. **Tombol "Kirim" (ship) ditambahkan di tab Riwayat** untuk baris Disetujui milik kantor asal —
   penambahan atas mockup (mockup tidak punya UI ship), deviasi disetujui.
4. **Kartu Jenjang Persetujuan dirender dari endpoint preview backend**
   (`GET /approval-thresholds/preview`, gate `request.create`), basis amount = **purchase_cost**
   (nilai perolehan — satu-satunya nilai server-trustable saat ini; keputusan keamanan, bukan
   akuntansi). Kartu berlabel eksplisit "berdasar nilai perolehan". **Follow-up PROGRESS.md:**
   alihkan basis amount disposal ke nilai buku komersial hasil hitungan server saat modul
   depresiasi jadi.
5. **Metode pelepasan = 4 enum backend**: Dijual (`sale`), Lelang (`auction`), Hibah (`donation`),
   Musnah (`write_off`). "Scrap" mockup di-drop (akuntansi: scrap = sale dengan proceeds); Lelang
   ditambah. Deviasi dicatat.
6. **Nilai fiskal ditampilkan "—" dengan chip FISKAL tetap dirender** (tooltip "menunggu modul
   depresiasi") — struktur mockup utuh, otomatis hidup saat depresiasi jadi.
7. **Lampiran hybrid**: dropzone di form submit mengunggah **foto bukti sebagai attachment aset**
   (`POST /assets/:id/attachments`, terlihat approver di Detail Aset PRA-approval); **BAST formal**
   dilampirkan pasca-approval via aksi "Lampirkan BAST" pada baris riwayat Selesai
   (`POST /disposals/:id/document`).
8. Label mockup `in_transfer` yang mentah **dilokalkan** ("Dalam Pengiriman"/"In Transit") —
   konvensi i18n menang atas literal mockup (CLAUDE.md); dicatat.

## 1. Backend — modul `transfer`

### Migrasi `000022_transfer_condition_return` (up + down)

- Enum baru `shared.transfer_condition`: `baik | rusak_ringan | rusak_berat`.
- `ALTER TYPE` enum status transfer (nilai `returned` ditambahkan; cek nama enum riil di migrasi
  000015 — kemungkinan `shared.transfer_status`).
- `transfer.asset_transfers` + kolom `condition_sent shared.transfer_condition NULL`,
  `transfer_date date NULL`, `return_note text NULL`.
- `sqlc generate` setelah query diperbarui.

### Perubahan kontrak

- `SubmitRequest` (`POST /transfers`) + `condition_sent` (binding
  `omitempty,oneof=baik rusak_ringan rusak_berat`) + `transfer_date` (`YYYY-MM-DD`, opsional —
  mockup menandainya wajib; UI mewajibkan, backend menerima opsional agar kompatibel mundur).
- `TransferPayload` + kedua field; executor menulisnya ke row transfer saat approval.
- **`POST /transfers/:id/reject-receive`** (gate `transfer.manage`): body `{note?: string}`;
  guard `status == in_transit` (else 409 `ErrInvalidState`); scope kantor **tujuan** (else 403
  `ErrOutOfScope`); efek: `status = returned`, `return_note`, `received_by_id = caller` (pencatat
  keputusan); aset TIDAK berpindah (tetap tercatat di kantor asal — `SetAssetOffice` tidak
  dipanggil). Response 200 `Transfer`.
- `toResponse` + `condition_sent`, `transfer_date`, `return_note`.

### Enrichment reads (pola PR #48: `sqlc.embed` + LEFT JOIN, filter `deleted_at` di ON)

`ListTransfers`/`GetTransfer`/`ListTransfersByAsset` → varian enriched dengan:
`asset_name`, `asset_tag` (JOIN `asset.assets`), `from_office_name`, `to_office_name`
(JOIN `masterdata.offices` ×2 alias), `to_room_name` (JOIN `masterdata.rooms`),
`requested_by_name`, `received_by_name` (JOIN `identity.users` ×2 alias). Semua `*string`
(LEFT JOIN). Query lama yang tak lagi dipanggil dihapus (jangan ulangi dead-query PR #48).
Ship/receive/reject-receive tetap return bentuk non-enriched (`toResponse`) — halaman refresh
via list (pola sama dengan decide di approval).

## 2. Backend — modul `disposal` + approval preview

- **Enrichment**: `ListDisposals`/`GetDisposal`/`ListDisposalsByAsset` → + `asset_name`,
  `asset_tag`, `office_name` (via JOIN asset→office), `created_by_name`.
- **`GET /approval-thresholds/preview`** di modul approval, gate **`request.create`** (bukan
  `approval.config.manage`): query `request_type` (validasi terhadap `validRequestTypes`) +
  `amount` (string desimal; validasi numerik). Return
  `{"steps":[{"step_order":1,"required_level":"office"}, …]}` hasil `MatchThresholdSteps` +
  `buildChain` — persis yang dipakai engine. 422 bila tidak ada threshold cocok (konsisten
  `ErrNoThreshold`); TIDAK mengekspos band nominal (amount_from/to) — hanya urutan step + level,
  cukup untuk kartu dan tidak membocorkan konfigurasi penuh ke non-admin.

## 3. OpenAPI

Semua perubahan didokumentasikan: `Transfer` schema (+3 field + enum status `returned` + field
enrichment), `TransferSubmitRequest` (+2 field), path `reject-receive`, `Disposal` (+enrichment),
`ThresholdPreview` schema + path preview. Spectral 0 error.

## 4. Frontend — layar Mutasi (`/transfers`)

- **Nav**: item "Mutasi Aset" (en "Transfers") di grup Operasional antara Penugasan dan
  Maintenance, ikon `i-lucide-arrow-right-left`, `permission: 'transfer.view'`. Badge count
  kotak-masuk di-defer (konsisten dengan keputusan badge layar Approval) — deviasi dicatat.
- **Gate halaman**: `middleware: 'can', permission: 'transfer.view'`; aksi tulis (submit/ship/
  terima/tolak) di-gate `useCan('transfer.manage')` di UI.
- **Legend alur** (selalu tampil): Diajukan → Disetujui → Dalam Pengiriman → Diterima
  (pill + panah, persis mockup; label dilokalkan per keputusan #8).
- **Tab 1 — Ajukan Mutasi** (kartu form max-w 640):
  - `AssetSearchPicker` (komponen bersama, lihat bagian 6): search `GET /assets?search=&status=available&limit=20`,
    dropdown nama + `tag · kantor` (butuh nama kantor — resolve via lookup offices), hint mockup.
  - Kantor Asal: read-only dashed box = kantor aset terpilih (office_name dari row aset via lookup).
  - Kantor Tujuan: select semua kantor scope-visible ≠ kantor asal (`GET /offices?limit=100`).
  - **Alert antar-wilayah / dalam-wilayah**: dihitung klien — naikkan rantai `parent_id` kedua
    kantor ke ancestor ber-tier `wilayah` (data `GET /offices` + office_type tier via
    `useReference`); ancestor wilayah beda → alert violet "Mutasi antar-wilayah", sama → note
    hijau "dalam satu wilayah". Bila tier tak ter-resolve (data tak lengkap) → tidak menampilkan
    alert apa pun (fail-quiet).
  - Ruangan Tujuan opsional: `GET /floors?office_id=` → `GET /rooms?floor_id=` (kantor tujuan),
    flatten "— (Belum ditentukan)" default.
  - Tanggal Mutasi (wajib di UI), Kondisi Saat Dikirim (baik/rusak_ringan/rusak_berat, wajib UI),
    Alasan textarea. Reset + Ajukan (disabled sampai aset+tujuan+tanggal terisi; error banner
    mockup). Submit → `POST /transfers` → banner sukses (template mockup) → reset form.
- **Tab 2 — Kotak Masuk** (badge count = jumlah item): baris dari
  `GET /transfers?status=in_transit&limit=100` difilter klien `to_office_id == officeId` milik
  caller (`/auth/me`); bila caller tanpa office (mis. Superadmin global) → tampilkan semua
  in_transit dalam scope. Kartu persis mockup: nama+tag aset, badge "Dalam Pengiriman", badge
  "Antar-Wilayah" bila lintas wilayah (hitung klien), rute asal→tujuan, "Diajukan oleh
  {requested_by_name}", badge kondisi kirim, blok alasan. Aksi:
  - **Terima** → modal: tanggal terima (default hari ini), ruangan tujuan (opsional, dari kantor
    saya), No. BAST, file BAST opsional → multipart `POST /transfers/:id/receive` → toast sukses
    template mockup → refresh inbox+riwayat.
  - **Tolak Terima** → modal alasan → `POST /transfers/:id/reject-receive` → toast template
    mockup → refresh.
  - Empty state persis mockup.
- **Tab 3 — Riwayat**: **gabungan dua sumber** —
  (a) `GET /requests?type=asset_transfer` status pending/rejected/cancelled → baris status
  **Diajukan / Ditolak / Dibatalkan** (payload tidak tersedia di list → kolom aset memakai
  `target_id` di-resolve nama via lookup aset ATAU tampil office+tanggal; lihat batasan di
  Deviasi (e)); (b) `GET /transfers?limit=100` → baris **Disetujui / Dalam Pengiriman / Diterima /
  Dikembalikan** dengan nama dari enrichment. Filter: search klien (nama/tag/kantor) + select
  status (semua nilai di atas). Kolom mockup: Aset (nama+tag, ikon globe violet bila antar-
  wilayah), Asal → Tujuan, Tanggal (`transfer_date` → fallback created_at), Pelaku (inisial +
  requested_by_name), Status (badge), No. BAST (teks mono; **bukan link** — layar Dokumen BAST
  belum ada, deviasi dicatat). Baris in_transit di-highlight info. **Aksi "Kirim"** pada baris
  Disetujui yang `from_office` dalam scope caller + `transfer.manage`: modal tanggal kirim
  (default hari ini) → `POST /transfers/:id/ship` → refresh. Footer "Total {n} mutasi".

## 5. Frontend — layar Penghapusan (`/disposals`)

- **Nav**: "Penghapusan" (en "Disposal") tepat setelah "Mutasi Aset", ikon `i-lucide-trash-2`,
  `permission: 'disposal.view'`. Gate halaman `disposal.view`; aksi tulis `disposal.manage`.
- **Tab 1 — Ajukan Penghapusan** (grid 1fr/340px, kolom kanan sticky):
  - `AssetSearchPicker`: status yang backend izinkan → `available`, `under_maintenance`
    (`assigned` ikut valid secara backend tapi modul Assignment belum ada — praktis tak ada aset
    assigned; picker memfilter `status=available` + `status=under_maintenance` via dua query atau
    filter klien).
  - **Ringkasan Valuasi** (muncul saat aset terpilih): Nilai Perolehan (`purchase_cost`),
    Akumulasi Penyusutan (`accumulated_depreciation`, tampil "− {v}"), Nilai Buku chip **PSAK**
    (`book_value`), Nilai Buku chip **FISKAL** = "—" + tooltip (keputusan #6). Field money bisa
    dimask FilterView → tampil "•••" bila absen dari response.
  - **Detail Pelepasan**: Metode (4 enum, label id/en), Nilai Jual/Terima (input Rp, hint mockup),
    Tanggal Pelepasan, No. BAST Penghapusan (mono), Alasan, **dropzone foto bukti** → langsung
    `POST /assets/:id/attachments` per file (chip file terunggah + hapus via DELETE attachment;
    hint "terlihat approver di Detail Aset").
  - **Kartu Laba/Rugi** (sticky kanan): klien menghitung `proceeds − book_value` (komersial);
    varian laba (hijau)/rugi (merah)/impas (netral) + breakdown persis mockup; baris
    "Laba/rugi fiskal: —". Empty state mockup bila belum ada aset/nilai. `book_value_at_disposal`
    yang dikirim = `asset.book_value` (read-only; null bila masked/absen → gain_loss null di
    server, kartu menampilkan "—" + catatan).
  - **Kartu Jenjang Persetujuan**: `GET /approval-thresholds/preview?request_type=asset_disposal&amount={purchase_cost}`
    → render baris Maker (selalu, "Pengaju") + tiap step: label level via i18n `approval.level.*`
    + nomor step; subjudul kartu "berdasar nilai perolehan: {rp}"; banner amber sensitif persis
    mockup. 422/error → kartu menampilkan pesan "band approval belum dikonfigurasi".
    Bila `purchase_cost` dimask untuk role caller → kartu menampilkan keterangan "nilai perolehan
    tersembunyi untuk peran Anda" tanpa memanggil preview.
  - Submit disabled sampai aset+tanggal+metode terisi → `POST /disposals` → **post-submit view**:
    banner sukses + kartu ringkasan (metode, nilai jual, laba/rugi, badge "Menunggu Approval") +
    **Timeline Approval Berlapis** dari `GET /requests/{request_id}` (steps: done=Disetujui hijau
    dengan approver_name+decided_at, current=Menunggu amber, sisanya=Antre muted — mapping
    `step_order` vs `current_step`) + tombol "Ajukan Penghapusan Lain" (reset ke form).
- **Tab 2 — Riwayat**: gabungan (a) `GET /requests?type=asset_disposal` pending/rejected/cancelled
  → **Menunggu Approval / Ditolak / Dibatalkan**; (b) `GET /disposals?limit=100` → **Selesai**
  (row disposal tercipta = selesai; opsi filter "Disetujui" mockup di-drop — state itu tak pernah
  berdiri sendiri karena executor atomik; deviasi dicatat). Kolom mockup: Aset (nama+tag dari
  enrichment / lookup), Metode (badge warna per metode; baris request memakai `payload`? — payload
  tidak ada di list → kolom metode baris request tampil "—"; lihat Deviasi (e)), Nilai Jual,
  Laba/Rugi (±warna), Tanggal, Status. Aksi **"Lampirkan BAST"** pada baris Selesai
  (`disposal.manage`): modal `bast_no`/`doc_no`/`doc_date`/`counterparty` + file → multipart
  `POST /disposals/:id/document` → toast + refresh. Footer "Total {n} pengajuan".

## 6. Shared frontend

- **`AssetSearchPicker.vue`** (components): props `statuses[]`, `placeholder`, `hint`; debounce
  search ke `GET /assets`; emit asset row lengkap; dropdown item = dot hijau + nama +
  `tag · office`; empty "Tidak ada aset tersedia". Dipakai kedua layar (dan kandidat reuse layar
  Assignment kelak).
- Composables: `useTransfers` (list/get/submit/ship/receive/rejectReceive — receive/reject
  multipart-aware), `useDisposals` (list/get/submit/attachDocument), `useApprovalPreview`
  (preview(type, amount)). DTO Inggris snake_case sesuai kontrak; error propagate (toast global
  useApiClient).
- Konstanta `transferMeta.ts` (status/condition badge tone + ikon) & `disposalMeta.ts`
  (method/status meta). i18n `id`/`en` lengkap — string mockup dipakai sebagai nilai id.
- Rute `/transfers` + `/disposals` (halaman tunggal per layar, tab internal state).

## 7. Testing

- **Backend integration** (`-tags=integration`): migrasi round-trip (condition/transfer_date
  tersimpan via submit→approve→row); reject-receive (guard status, scope tujuan, aset tak
  berpindah, returned + note); enrichment transfer+disposal (nama benar, soft-delete aman);
  preview endpoint (band cocok, 422 tanpa band, gate permission).
- **Frontend unit**: meta konstanta; helper antar-wilayah (ancestor tier — kasus: sama wilayah,
  beda wilayah, tier tak ter-resolve); mapper status gabungan riwayat (request+row → label).
- **Component (`mountSuspended`, stub API)** per layar: semua state mockup — form kosong/terisi/
  invalid, alert antar-wilayah, inbox berisi/kosong, aksi terima/tolak memanggil endpoint benar,
  riwayat gabungan + filter + tombol Kirim hanya pada baris eligible; disposal: ringkasan valuasi,
  laba/rugi hijau/merah/impas, kartu chain (+ fallback masked/422), post-submit timeline, riwayat
  + Lampirkan BAST, dropzone upload attachment. Loading/error/empty di semua fetch.
- **E2E real backend** (2 spec, data unik per run, RATELIMIT off): transfer penuh
  (submit → approve sebagai checker SoD → Kirim → Terima dengan BAST → aset pindah kantor → badge
  Diterima; plus alur Tolak Terima → Dikembalikan + aset tidak pindah) dan disposal penuh
  (submit dengan foto bukti → chain card tampil → approve → status aset disposed → riwayat
  Selesai → Lampirkan BAST). Suite penuh tetap hijau.
- Side-by-side kedua mockup light+dark 1:1 kecuali deviasi di bawah.

## 8. Deviasi mockup (disetujui user — catat di PROGRESS.md)

(a) tombol **Kirim** ditambahkan di Riwayat (mockup tak punya UI ship);
(b) label `in_transfer` dilokalkan "Dalam Pengiriman";
(c) metode disposal = 4 enum backend (Scrap di-drop, Lelang ditambah);
(d) nilai fiskal "—" + chip tetap (menunggu modul depresiasi);
(e) baris riwayat yang masih berupa approval request (Diajukan/Menunggu/Ditolak/Dibatalkan)
    menampilkan info terbatas (payload tak diekspos di list `GET /requests`) — kolom
    aset di-resolve dari `target_id` via lookup bila memungkinkan, kolom metode/nilai "—";
(f) No. BAST di riwayat mutasi = teks mono, bukan link (layar Dokumen BAST belum dibangun);
(g) badge count nav Mutasi di-defer (butuh store inbox global — konsisten layar Approval);
(h) status filter "Disetujui" di riwayat disposal di-drop (tak pernah berdiri sendiri);
(i) tanggal mutasi backend menerima opsional (UI mewajibkan) demi kompatibilitas kontrak.

## 9. Definition of done

1. Semua gates hijau: backend build/vet/test + full integration, Spectral, frontend
   lint/typecheck/test/build, full e2e (stack Docker + RATELIMIT off).
2. Side-by-side kedua mockup (light+dark) 1:1 kecuali deviasi (a)–(i).
3. PROGRESS.md: kandidat (d) ditandai selesai + deviasi + follow-up "basis amount disposal →
   nilai buku server-side pasca-depresiasi" + follow-up "BAST link saat layar Dokumen BAST jadi".
4. OpenAPI sinkron; tidak ada dead sqlc query yang tersisa; migrasi punya down yang benar.
