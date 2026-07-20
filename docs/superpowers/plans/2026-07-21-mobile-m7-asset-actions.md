# Implementation Plan — Mobile fase M7: Katalog & aksi aset

Tanggal: 2026-07-21. Spec: [`2026-07-21-mobile-m7-asset-actions.md`](../specs/2026-07-21-mobile-m7-asset-actions.md).
Branch rencana `feat/mobile-m7`. Frontend/mobile-only, nol backend baru.

## Overview

Delapan irisan vertikal menambah katalog + aksi/pengajuan aset + view self-scoped ke aplikasi
Flutter yang ada (12 layar M0-M4 sudah jalan). Tiap irisan = satu jalur fitur lengkap (DTO + service
Dio + layar + tests), memakai endpoint yang sudah ada. Konvensi mobile: mockup layar di-generate
dulu (DESIGN_BRIEF 5.13-5.18 + edit 5.2/5.4), lalu bangun 1:1, lalu bandingkan side-by-side.

## Architecture decisions

- Reuse pola feature-first + Riverpod + Dio + freezed yang ada (ARCHITECTURE.md). Tiap fitur:
  `models/` (freezed DTO), `data/` (service Dio), `presentation/` (screen + controller).
- Visibilitas aksi berbasis **permission** (`/auth/permissions`, composable izin yang ada) x status
  aset — bukan peran hardcode. Penulisan divalidasi server.
- Katalog & Detail Aset reuse komponen kartu aset + field-permission masking dari M1.
- Nol dependensi baru diharapkan (image picker untuk lampiran Lapor Kerusakan: cek apakah sudah ada;
  bila belum, tambah — satu-satunya kandidat dependensi, tanya dulu).

## Prasyarat (Phase 0)

- [ ] Mockup di-generate dari DESIGN_BRIEF (5.13-5.18 baru; 5.2 & 5.4 edit) ke `docs/mobile/design/`.
- [ ] Konfirmasi `GET /assets` mendukung param filter (kategori/status/kantor) + search + paginasi
      yang dibutuhkan Katalog; bila kurang, itu jadi temuan (spec asumsi cukup).
- [ ] QM7-4: konfirmasi sumber **id assignment aktif** untuk Check-in (detail aset vs
      `GET /assets/:id/assignments`).

### Checkpoint: Phase 0
- [ ] Mockup keenam layar baru + dua edit tersedia; review 1:1 sebagai acuan.

## Task list

### Phase 1: Layar list read-only (paralel-aman)

#### Task M7-1: Katalog Aset
**Description:** Layar telusur aset dalam scope: search, filter chips (kategori/status/kantor),
paginasi server, tap lalu Detail Aset.
**Acceptance:**
- [ ] `GET /assets` dipanggil dengan search + filter + paginasi; infinite scroll + pull-to-refresh.
- [ ] Empty/loading/terisi sesuai mockup 5.13; field sensitif tak tampil di kartu.
- [ ] Tap kartu membuka Detail Aset yang ada.
**Verification:** `flutter analyze` + widget test (empty/filter/loading) + golden light/dark; banding 1:1 mockup.
**Dependencies:** Phase 0.
**Files:** `mobile/lib/features/catalog/` (model, service, screen, controller) + test.
**Scope:** M.

#### Task M7-2: Aset Saya
**Description:** Menu tersendiri berisi aset yang dipegang pengguna (`GET /assignments/mine`), penanda terlambat.
**Acceptance:**
- [ ] List `GET /assignments/mine`; kartu menampilkan status pinjam + jatuh tempo; item lewat tempo ditandai "Terlambat".
- [ ] Empty/loading/offline (banner + data terakhir) sesuai mockup 5.18; tap lalu Detail Aset.
**Verification:** widget test (terlambat, empty) + golden; 1:1 mockup.
**Dependencies:** Phase 0.
**Files:** `mobile/lib/features/my_assets/` + test.
**Scope:** S/M.

#### Task M7-3: Pengajuan Saya + detail + batal
**Description:** Lensa maker: `GET /requests?requested_by=diri`, filter status, detail read-only, batal pending.
**Acceptance:**
- [ ] List pengajuan sendiri + filter chips status; kartu tanpa aksi keputusan.
- [ ] Status `pending` punya Batalkan (`POST /requests/:id/cancel`, dialog konfirmasi).
- [ ] Tap lalu detail read-only (timeline jenjang; tanpa approve/reject).
**Verification:** widget test (filter, Batalkan hanya pending milik sendiri, empty) + golden; 1:1 mockup.
**Dependencies:** Phase 0.
**Files:** `mobile/lib/features/my_requests/` + test.
**Scope:** M.

### Checkpoint: Phase 1
- [ ] Tiga layar list jalan, analyze/test hijau, golden light+dark, 1:1 mockup. Review.

### Phase 2: Aksi dari Detail Aset

#### Task M7-4: Bar aksi Detail Aset (visibilitas per permission x status)
**Description:** Tambah bar aksi sticky di Detail Aset (edit layar M1) yang memunculkan tombol sesuai izin + status.
**Acceptance:**
- [ ] Matriks: `Tersedia`+`request.create` lalu "Pinjam"; `Tersedia`+`assignment.manage` lalu "Check-out"; `Dipinjam`+`assignment.manage` lalu "Check-in"; `request.create` lalu "Lapor Kerusakan"; tanpa izin lalu tanpa bar.
- [ ] Tidak mengubah perilaku in-opname (bar tandai hasil) yang ada.
**Verification:** unit test matriks visibilitas (permission x status) + widget test + golden tiga varian (Tersedia-Staf, Tersedia-Manager, Dipinjam-Manager); 1:1 mockup 5.4.
**Dependencies:** Phase 0.
**Files:** `mobile/lib/features/asset_detail/` (edit) + test.
**Scope:** S/M.

#### Task M7-5: Sheet Peminjaman / Check-out / Check-in
**Description:** Tiga alur dari bar aksi: Staf ajukan (`POST /assignments/borrow`), Manager check-out (`POST /assignments`) dengan picker pegawai, Manager check-in (`POST /assignments/:id/checkin`).
**Acceptance:**
- [ ] Sheet Ajukan Peminjaman: tanggal pinjam + jatuh tempo opsional (calendar) + catatan lalu sukses SnackBar.
- [ ] Sheet Check-out: picker pegawai (autocomplete + empty "Tidak ada data") + tanggal + kondisi; validasi custodian wajib; sukses lalu aset jadi Dipinjam, Detail refresh.
- [ ] Sheet Check-in (aset Dipinjam): pemegang saat ini + kondisi masuk (Baik/Perlu Servis) + catatan; sukses lalu aset Tersedia (atau under_maintenance), Detail refresh. Resolusi **id assignment aktif** (QM7-4) sebelum submit.
- [ ] Error server dipetakan ke pesan inline jelas.
**Verification:** widget test (tiga alur, validasi custodian, kondisi masuk, error) + golden; integration (Staf ajukan lalu Pengajuan Saya; Manager check-out lalu Aset Saya pegawai target lalu Check-in lalu aset Tersedia).
**Dependencies:** M7-4; QM7-4 diverifikasi.
**Files:** `mobile/lib/features/asset_detail/assignment/` + test.
**Scope:** M.

#### Task M7-6: Sheet Lapor Kerusakan
**Description:** Ajukan laporan kerusakan (`POST /maintenance/reports`) dari Detail Aset.
**Acceptance:**
- [ ] Deskripsi wajib, severity opsional, lampiran foto opsional; sukses SnackBar "Laporan kerusakan dikirim".
- [ ] Validasi deskripsi kosong ditolak inline.
**Verification:** widget test (deskripsi wajib, sukses) + golden; 1:1 mockup 5.15.
**Dependencies:** M7-4.
**Files:** `mobile/lib/features/asset_detail/damage_report/` + test.
**Scope:** S.

### Checkpoint: Phase 2
- [ ] Bar aksi + tiga aksi jalan; integration peminjaman/check-out hijau. Review.

### Phase 3: Registrasi + navigasi

#### Task M7-7: Form Registrasi Aset
**Description:** Stepper 3 langkah lalu `POST /requests` type `asset_create` dengan `AssetCreatePayload`.
**Acceptance:**
- [ ] Field sesuai `AssetCreatePayload` (wajib: name/category_id/office_id/asset_class); kategori autocomplete + empty state; kantor default scope.
- [ ] Harga numerik-only (tolak keystroke non-numerik, non-negatif); `amount` diset == `purchase_cost` (nol bila kosong).
- [ ] TIDAK ada cek kapitalisasi; sukses lalu arahkan ke Pengajuan Saya.
**Verification:** unit (validator numerik, amount==cost) + widget (blok per langkah, field wajib) + golden; integration (registrasi lalu pengajuan asset_create tampil).
**Dependencies:** M7-3 (arahkan ke Pengajuan Saya).
**Files:** `mobile/lib/features/asset_register/` (stepper, model, service) + test.
**Scope:** M/L — bila melewati satu sesi, pecah per langkah stepper.

#### Task M7-8: Titik masuk Beranda
**Description:** Wiring aksi cepat Beranda ke Katalog, Aset Saya, Pengajuan Saya (edit mockup 5.2).
**Acceptance:**
- [ ] Grid aksi cepat menambah 3 destinasi; navigasi benar; bottom nav tetap 5 slot.
**Verification:** widget test navigasi + golden Beranda; 1:1 mockup 5.2.
**Dependencies:** M7-1, M7-2, M7-3.
**Files:** `mobile/lib/features/home/` (edit) + test.
**Scope:** S.

### Checkpoint: Phase 3 (Complete)
- [ ] Semua acceptance terpenuhi; analyze/test/build APK hijau; golden light+dark; integration hijau.
- [ ] Semua layar 1:1 mockup (side-by-side). PROGRESS.md dicentang + PR number. Review akhir.

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| `GET /assets` kurang param filter yang dibutuhkan | Med | Verifikasi di Phase 0; bila kurang, backend jadi temuan (di luar asumsi nol-backend) lalu tanya |
| Payload `asset_create` meleset dari executor | Med | DTO diturunkan dari `AssetCreatePayload` + `amount==purchase_cost`; integration test approve lalu aset tercipta |
| Lampiran foto butuh dependensi image picker baru | Low | Cek dulu; bila perlu, tanya sebelum menambah |
| Regresi Detail Aset (bar in-opname vs FR-M7) | Med | Test matriks visibilitas; golden pembeda; jangan ubah jalur opname |

## Open questions

- QM7-1/2/3 resolved di spec. **QM7-4** (belum): sumber **id assignment aktif** untuk Check-in —
  verifikasi apakah detail aset sudah memuatnya atau perlu `GET /assets/:id/assignments`; selesaikan
  di Phase 0 sebelum Task M7-5 (sheet Check-in). Sisakan juga verifikasi param `GET /assets` di Phase 0.
