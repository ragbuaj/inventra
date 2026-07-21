# Spec — Mobile fase M7: Katalog & aksi aset di lapangan

Tanggal: 2026-07-21. Status: draf, menunggu review pemilik produk. Branch rencana `feat/mobile-m7`.

Dasar scope: keputusan produk 2026-07-21 (vault `Keputusan/Produk/Mobile v1 Tambah Katalog
Registrasi Maintenance Peminjaman.md`), PRD mobile v1.1 FR-M7, roadmap fase M7. Mockup: prompt
`docs/mobile/DESIGN_BRIEF.md` bagian 5.13-5.18 + edit 5.4 (hasil ke `docs/mobile/design/`).

## 1. Objective

Menjadikan aplikasi mobile mampu **menelusuri katalog aset** dan **membuat aksi/pengajuan aset di
titik penemuan lapangan** — peminjaman/check-out, lapor kerusakan, registrasi — serta melihat
**pengajuan sendiri** dan **aset yang dipegang**. Sebelumnya Detail Aset read-only dan pembuatan
pengajuan hanya di web.

Pengguna: petugas lapangan & Manager aset (isi mengikuti permission + data scope + field permission
server). Sukses = seorang petugas dapat, dari HP: mencari aset di katalog, memindai lalu mengajukan
peminjaman/lapor kerusakan/registrasi, memantau status pengajuannya, dan melihat aset yang
dipegangnya — semua tanpa membuka web, dengan seluruh otorisasi tetap ditegakkan server.

Non-goal M7: mutasi dan penghapusan/disposal (tetap di web); mode offline (M7 online-only, konsisten
kebijakan Detail Aset v1); backend baru (nol — semua endpoint sudah ada).

## 2. Tech stack & commands

Flutter (Dart 3), `mobile/`, arsitektur feature-first (lihat `docs/mobile/ARCHITECTURE.md`):
Riverpod, Dio (interceptor auth ADR-0017), freezed + json_serializable, go_router,
material_symbols_icons, mobile_scanner (sudah ada dari M1). Nol dependensi baru diharapkan.

```
Analyze : (cd mobile) flutter analyze
Test    : flutter test
Build   : flutter build apk --debug
Codegen : dart run build_runner build --delete-conflicting-outputs
```

## 3. Kontrak API yang dikonsumsi (semua sudah ada — nol backend baru)

| Kemampuan | Endpoint | Permission | Catatan |
|---|---|---|---|
| Katalog aset (FR-M7.1) | `GET /assets` (list, filter, paginasi) | data-scoped | search + filter kategori/status/kantor; field sensitif tunduk field permission |
| Peminjaman Staf (FR-M7.2) | `POST /assignments/borrow` | `request.create` | membuat pengajuan `assignment` (maker-checker) |
| Check-out Manager (FR-M7.2) | `POST /assignments` | `assignment.manage` | penugasan langsung; aset jadi `assigned` |
| Check-in Manager (FR-M7.2) | `POST /assignments/:id/checkin` | `assignment.manage` | pengembalian; aset `assigned` jadi `available` (atau `under_maintenance`); butuh id assignment aktif |
| Picker pegawai check-out | `GET /assignments/available` / picker pegawai | `request.create`/scope | custodian dipilih dalam scope (bukan office_id mentah) |
| Lapor kerusakan (FR-M7.3) | `POST /maintenance/reports` | `request.create` | membuat pengajuan `maintenance` |
| Registrasi aset (FR-M7.4) | `POST /requests` (type `asset_create`) | `request.create` | payload = `AssetCreatePayload` (lihat 4.5); `amount` wajib == `payload.purchase_cost`; hanya type asset_create/asset_disposal/valuation_exclusion diterima endpoint ini |
| Pengajuan saya (FR-M7.5) | `GET /requests?requested_by=<diri>` | authed | lensa maker; filter status |
| Batal pengajuan sendiri | `POST /requests/:id/cancel` | authed (cek `requested_by` di server) | hanya status `pending` |
| Detail pengajuan | `GET /requests/:id` | authed | read-only, timeline jenjang |
| Aset saya (FR-M7.6) | `GET /assignments/mine` | `request.create` | aset yang dipegang pengguna |

Verifikasi kontrak (pelajaran "verifikasi klaim kontrak API"): sebelum menulis DTO Flutter,
konfirmasi bentuk request/response tiap endpoint terhadap handler backend + `openapi.yaml`, terutama
payload `POST /requests` type `asset_create` (field valuasi/penyusutan/kapitalisasi) agar identik
dengan yang dikirim form registrasi web.

## 4. Perilaku per layar

Navigasi: bottom nav tetap 5 slot (tidak berubah). Katalog Aset, Aset Saya, Pengajuan Saya =
destinasi sekunder (AppBar + kembali) dari aksi cepat Beranda / area Profil. Peminjaman, Lapor
Kerusakan dari Detail Aset (bottom sheet). Registrasi = form multi-langkah.

### 4.1 Katalog Aset (5.13)
- List `GET /assets` dengan search (nama/kode), filter chips (kategori, status, kantor), paginasi
  server (infinite scroll), pull-to-refresh. Field sensitif tidak tampil di kartu.
- Tap kartu lalu Detail Aset. States: terisi, hasil-filter, empty ("Tidak ada aset yang cocok" +
  reset filter), loading skeleton. Aset di luar scope tidak muncul (server).

### 4.2 Detail Aset — bar aksi FR-M7 (edit 5.4)
- Di luar sesi opname, bar sticky bawah menampilkan aksi **sesuai permission x status aset**:
  - `Tersedia` + `request.create`: **Pinjam** (buka sheet Ajukan Peminjaman).
  - `Tersedia` + `assignment.manage`: **Check-out** (buka sheet Check-out langsung).
  - `Dipinjam` (`assigned`) + `assignment.manage`: **Check-in** (buka sheet Check-in).
  - `request.create`: **Lapor Kerusakan** (buka sheet).
  - Tanpa izin aksi: tanpa bar (murni read-only — perilaku lama tetap).
- Aksi ditentukan dari `/auth/permissions` (composable izin yang ada) + status aset; jangan
  hardcode peran.

### 4.3 Peminjaman / Check-out / Check-in (5.14)
- **Staf — Ajukan Peminjaman** (`POST /assignments/borrow`): tanggal pinjam, jatuh tempo opsional
  (UCalendar/date picker), catatan/alasan; sukses lalu SnackBar "Pengajuan peminjaman dikirim".
- **Manager — Check-out** (`POST /assignments`): pilih pegawai/custodian (autocomplete dengan empty
  state "Tidak ada data"), tanggal pinjam, jatuh tempo opsional, catatan kondisi keluar; sukses lalu
  aset jadi `Dipinjam`, SnackBar sukses, Detail di-refresh.
- **Manager — Check-in** (`POST /assignments/:id/checkin`): untuk aset `Dipinjam`; tampilkan pemegang
  saat ini, kondisi masuk (Baik / Perlu Servis), catatan opsional; sukses lalu aset kembali
  `Tersedia` (atau `under_maintenance`), Detail di-refresh. Klien perlu **id assignment aktif** aset
  (dari detail aset atau `GET /assets/:id/assignments`) — lihat QM7-4.
- Validasi inline (custodian wajib untuk check-out). Error server dipetakan ke pesan jelas.

### 4.4 Lapor Kerusakan (5.15)
- `POST /maintenance/reports`: deskripsi (wajib), severity opsional, lampiran foto opsional
  (kamera/galeri). Sukses lalu SnackBar "Laporan kerusakan dikirim". Diproses sebagai pengajuan.

### 4.5 Form Registrasi Aset (5.16)
- `POST /requests` type `asset_create`. Payload = `AssetCreatePayload` (verifikasi
  `internal/asset/executor.go`): `name`, `category_id`, `office_id`, `room_id?`, `asset_class`,
  `purchase_cost?`, `purchase_date?` ("2006-01-02"), `serial_number?`, `brand_id?`, `model_id?`,
  `unit_id?`, `vendor_id?`, `po_number?`, `funding_source?`, `warranty_expiry?`, `notes?`. Field
  wajib: `name`, `category_id`, `office_id`, `asset_class`. **`amount` request WAJIB sama dengan
  `payload.purchase_cost`** (server menolak bila beda; nol bila cost absen) — klien menyetel `amount`
  dari harga perolehan.
- Stepper 3 langkah: Identitas lalu Penempatan & Perolehan lalu Tinjau & Kirim.
- **TIDAK ada cek ambang kapitalisasi** — form web tidak punya itu dan executor selalu
  `Capitalized: true`; ambang kapitalisasi adalah fitur v1.1 yang belum diimplementasi (migrasi
  000015-000021 belum ditulis). Menambahkannya = perubahan produk + backend baru, di luar scope M7.
- **Input harga numerik-only** (tolak keystroke non-numerik, non-negatif, format desimal polos
  seperti `parsePlainDecimal` server). Validasi per langkah sebelum boleh lanjut.
- Sukses lalu arahkan ke Pengajuan Saya + SnackBar.

### 4.6 Pengajuan Saya (5.17)
- `GET /requests?requested_by=<diri>`; filter chips status (Menunggu/Disetujui/Ditolak/Semua).
- Kartu tanpa aksi keputusan; status `pending` punya tombol **Batalkan** (`POST /requests/:id/cancel`,
  dialog konfirmasi). Tap lalu detail read-only (timeline jenjang, tanpa approve/reject).
- States: campuran status, filter Menunggu + Batalkan, konfirmasi batal, empty, loading.

### 4.7 Aset Saya (5.18)
- Menu tersendiri; `GET /assignments/mine`. Kartu: aset, status pinjam, jatuh tempo; item lewat
  tempo diberi penanda "Terlambat". Tap lalu Detail Aset. States: terisi (+terlambat), empty,
  loading, offline (banner + data terakhir).

## 5. Otorisasi (ditegakkan server; klien hanya menyesuaikan UI)

- Tombol aksi muncul berdasar permission pengguna (`/auth/permissions`) + status aset — bukan peran
  hardcode. Semua penulisan tetap divalidasi server (permission, data scope, SoD, threshold).
- Field permission: field sensitif di katalog/detail tampil "—" atau disembunyikan (perilaku FR-M2.3
  yang ada). Aset di luar scope = tidak ditemukan.

## 6. Testing strategy

Konvensi proyek (proaktif & ekspansif — cakup happy path plus edge/empty/error/loading, input tak
valid, variasi permission):
- **Unit** (`flutter test`): mapping DTO tiap endpoint; validator input numerik harga registrasi
  (tolak huruf, non-negatif, desimal polos); penyetelan `amount == purchase_cost`; logika
  visibilitas bar aksi (matriks permission x status aset).
- **Widget** (`flutter test`): Katalog (empty/filter/loading), sheet Peminjaman/Check-out/Check-in
  (validasi custodian, tiga alur, kondisi masuk pada check-in), Lapor Kerusakan (deskripsi wajib),
  Form Registrasi (blok per langkah, field wajib, tolak keystroke non-numerik pada harga), Pengajuan
  Saya (filter, Batalkan hanya untuk pending milik sendiri), Aset Saya (penanda terlambat, empty).
- **Golden** light + dark untuk tiap layar baru + tiga varian bar aksi Detail Aset (Tersedia-Staf,
  Tersedia-Manager, Dipinjam-Manager).
- **Integration** (`integration_test/` vs docker-compose backend + seed): alur Staf ajukan
  peminjaman lalu tampil di Pengajuan Saya lalu Batalkan; Manager check-out lalu aset jadi Dipinjam
  lalu muncul di Aset Saya (pegawai target) lalu **Check-in lalu aset kembali Tersedia**; registrasi
  lalu pengajuan asset_create tampil. Data unik per run; rate-limit off lokal (pelajaran e2e).

## 7. Boundaries

- **Selalu**: enforce lewat endpoint yang ada; tampilkan aksi hanya sesuai permission; input finansial
  numerik-only + validasi ketat; i18n id/en semua string; match mockup 1:1 (bandingkan sisi-sisi di
  akhir tiap layar) light + dark; jalankan analyze/test/build hijau sebelum commit.
- **Tanya dulu**: setiap penambahan endpoint/perubahan backend (spec ini mengasumsikan nol); deviasi
  mockup apa pun (catat-deviasi); menambah dependensi Flutter.
- **Jangan**: menaruh logika kelayakan (SoD/threshold) di klien; membangun mutasi/disposal;
  menyimpan field sensitif; mode offline untuk M7.

## 8. Success criteria

- [ ] Katalog: cari + filter + paginasi jalan; empty/loading benar; field sensitif tidak bocor.
- [ ] Detail Aset: bar aksi muncul sesuai matriks permission x status; read-only murni bila tak berizin.
- [ ] Peminjaman (Staf ajukan), Check-out & Check-in (Manager) sukses via endpoint masing-masing;
      validasi custodian; aset berubah status pada check-out (Dipinjam) dan check-in (Tersedia).
- [ ] Lapor kerusakan & Registrasi membuat pengajuan yang benar; registrasi menolak input harga tak
      valid dan menyetel `amount == purchase_cost`.
- [ ] Pengajuan Saya menampilkan pengajuan sendiri + filter; Batalkan hanya untuk pending milik
      sendiri.
- [ ] Aset Saya menampilkan aset yang dipegang + penanda terlambat.
- [ ] Semua layar 1:1 dengan mockup (light + dark); analyze/test/build hijau; PROGRESS + OpenAPI
      (bila tersentuh — diharapkan tidak) sinkron.

## 9. Open questions (diselesaikan 2026-07-21)

- **QM7-1 (resolved)** — Payload `asset_create` = `AssetCreatePayload` (`internal/asset/executor.go`),
  lihat 4.5; wajib `name`/`category_id`/`office_id`/`asset_class`; `amount == purchase_cost`. Endpoint
  `POST /requests` hanya menerima type asset_create/asset_disposal/valuation_exclusion.
- **QM7-2 (resolved)** — TIDAK ada cek ambang kapitalisasi: form web tak punya, executor selalu
  `Capitalized: true`, ambang kapitalisasi fitur v1.1 belum diimplementasi. Dibuang dari scope M7
  (menambahkannya = keputusan produk + backend baru).
- **QM7-3 (resolved)** — Beranda (5.2) diupdate: aksi cepat menambah Katalog, Aset Saya, Pengajuan
  Saya (prompt edit DESIGN_BRIEF 5.2). Bottom nav tetap 5 slot.
- **QM7-4 (resolved)** — Detail aset hanya mengekspos `current_holder_employee_id`, **bukan** id
  assignment. Sheet Check-in mengambil `GET /assets/:id/assignments` (Manager punya `assignment.view`)
  lalu memilih penugasan aktif (`checkin_date` null) untuk `POST /assignments/:id/checkin`. Nol backend
  baru. Check-in ditambahkan ke scope M7 atas keputusan pemilik produk 2026-07-21.
- **QM7-catalog (resolved)** — `GET /assets` mendukung `search`/`category_id`/`office_filter`/
  `status`/`asset_class` + paginasi + data-scope; cukup untuk Katalog.
