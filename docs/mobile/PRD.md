# PRD Mobile — Inventra Field Companion

Dokumen kebutuhan produk untuk **aplikasi mobile companion** Inventra (Flutter). Dipisah dari
[PRD web](../PRD.md) agar mudah dibaca; PRD web tetap **otoritatif untuk domain** (peran, aturan
bisnis, otorisasi, regulasi) — dokumen ini hanya mencakup klien mobile. Keputusan arsitektur:
[ADR-0015](adr/0015-mobile-companion-flutter.md) (Flutter) dan
[ADR-0016](adr/0016-stock-opname-offline-sync.md) (offline sync). Roadmap fase:
`docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`. Design brief mockup:
[DESIGN_BRIEF.md](DESIGN_BRIEF.md) (hasil di `docs/mobile/design/`).

Versi: **v1.0 (2026-07-18)** — scope dibuka via PRD web v1.2 (non-goal v1.1 dicabut).

---

## 1. Ringkasan

**Field companion** — pendamping lapangan untuk sistem manajemen aset tetap bank Inventra, bukan
pengganti aplikasi web. Web tetap aplikasi utama administrasi (master data, konfigurasi, laporan);
mobile melayani pekerjaan yang terjadi **jauh dari meja**: memindai label aset dengan kamera,
menjalankan stock opname (termasuk di lokasi tanpa sinyal), memutus approval saat mobile, dan
menerima push notification.

### 1.1 Masalah yang dipecahkan

Web sudah mobile-responsive, tetapi tiga kebutuhan lapangan tidak terlayani baik oleh browser:

1. **Opname di lokasi tanpa sinyal** — gudang, basement, lokasi ATM; browser butuh koneksi dan
   tidak punya penyimpanan lokal yang andal untuk antrean scan.
2. **Kecepatan scan** — petugas memindai ratusan label per sesi; pipeline kamera native jauh
   lebih cepat dan andal daripada scan via browser.
3. **Push notification** — approver perlu tahu ada pengajuan menunggu tanpa membuka aplikasi;
   web push tidak andal lintas platform untuk aplikasi internal.

### 1.2 Tujuan (Goals)

- GM1 — Stock opname dapat diselesaikan penuh dari device, **termasuk offline**, tanpa kehilangan
  data scan.
- GM2 — Identifikasi aset di lapangan dalam hitungan detik: scan label lalu detail tampil.
- GM3 — Approval tidak menunggu approver kembali ke meja: push masuk, buka, putuskan.
- GM4 — Nol kompromi kontrol: semua otorisasi (permission, data scope, field permission, SoD,
  threshold) tetap ditegakkan server — klien mobile murni konsumen `/api/v1`.

### 1.3 Non-Goals (di luar lingkup mobile v1)

- CRUD master data, administrasi user/RBAC/data-scope/field-permission, import massal, laporan
  dan dashboard penuh — tetap di web.
- **Pembuatan** pengajuan modul non-opname (registrasi aset, mutasi, disposal, dsb.) — diajukan
  dari web; mobile hanya **memutus** approval-nya.
- Mode offline untuk fitur selain stock opname (approval/scan/notifikasi butuh koneksi).
- Login Google (menyusul; v1 email + password).
- iOS build (disiapkan strukturnya, rilis Android dulu).

## 2. Pengguna

Peran mengikuti PRD web bagian 2. Pengguna utama mobile:

| Persona | Pekerjaan di mobile |
|---|---|
| Petugas opname / GA cabang (Manager) | Unduh snapshot sesi, scan label per ruangan, tandai hasil, pantau antrean sync, lihat variance |
| Pejabat pemutus (Kepala Unit / Kepala Kanwil) | Terima push, buka inbox, tinjau detail pengajuan, approve/reject dengan catatan |
| Semua pengguna | Login, scan label untuk lihat detail aset, baca notifikasi, kelola profil dan sesi device |

Menu dan data yang tampil mengikuti permission + data scope pengguna, sama seperti web.

## 3. Kebutuhan Fungsional

### 3.1 Autentikasi & sesi (FR-M1)

- **FR-M1.1** Login email + password memakai akun yang sama dengan web; Google menyusul.
- **FR-M1.2** Access token disimpan di secure storage; refresh memakai mekanisme cookie httpOnly
  yang ada via cookie jar persisten — tanpa perubahan backend (ADR-0015).
- **FR-M1.3** Sesi login tercatat sebagai **device session** (terlihat & dapat dicabut dari web
  maupun mobile); pencabutan sesi berlaku seketika (perilaku `RequireAuth` yang ada).
- **FR-M1.4** Logout menghapus token lokal dan mencabut sesi di server.

### 3.2 Scan & identifikasi aset (FR-M2)

- **FR-M2.1** Scan barcode/QR label aset via kamera (Code128 + QR dari `asset_tag`); fallback
  **input tag manual** selalu tersedia (label pudar / kamera buruk).
- **FR-M2.2** Hasil scan membuka **detail aset** (`GET /assets/by-tag/:tag`) — read-only.
- **FR-M2.3** Field permission dan data scope berlaku sama dengan web (field tersembunyi tampil
  "—"; aset di luar scope = tidak ditemukan).
- **FR-M2.4** Tag tak dikenal atau di luar scope menghasilkan pesan jelas, bukan layar kosong.

### 3.3 Approval on-the-go (FR-M3)

- **FR-M3.1** Inbox pengajuan (`GET /requests`) dengan filter status (menunggu / disetujui /
  ditolak) dan badge jumlah menunggu.
- **FR-M3.2** Detail pengajuan: ringkasan data (before/after bila relevan), pengaju & kantor,
  nilai, lampiran, timeline approval.
- **FR-M3.3** Approve/reject dengan catatan; maker-checker, SoD, dan `approval_thresholds`
  ditegakkan server — mobile tidak menduplikasi logika kelayakan.
- **FR-M3.4** Pengajuan sensitif (disposal, pengecualian valuasi) menampilkan penanda peringatan
  yang sama dengan web.

### 3.4 Notifikasi (FR-M4)

- **FR-M4.1** **Push notification (FCM)** untuk empat jenis notifikasi yang ada
  (`approval_pending`, `approval_decided`, `maintenance_due`, `asset_returned`); dispatcher push
  adalah consumer tambahan pada pipeline outbox ADR-0014 (backend, fase M3).
- **FR-M4.2** Registrasi/deregistrasi device token saat login/logout (endpoint baru; tabel
  `device_tokens`).
- **FR-M4.3** Feed notifikasi in-app (paritas layar web) + unread count; tap notifikasi
  **deep-link** ke layar terkait (approval detail, opname).
- **FR-M4.4** Teks notifikasi dirender klien dari `type` + `params` (i18n id/en) — konsisten
  keputusan ADR-0014.

### 3.5 Stock opname offline-first (FR-M5)

Mengacu aturan domain PRD web bagian 3.9; strategi sync di ADR-0016.

- **FR-M5.1** Daftar sesi opname dalam scope; membuka sesi mengunduh **snapshot** item ke
  penyimpanan lokal device.
- **FR-M5.2** Mode counting: scan (atau input manual) menandai hasil per aset — `found` /
  `not_found` / `damaged` / `misplaced` — dan bekerja penuh **tanpa koneksi**; scan tercatat ke
  antrean lokal dengan `client_scan_id`.
- **FR-M5.3** Antrean tahan restart/crash aplikasi; tidak ada scan yang hilang sebelum
  tersinkron.
- **FR-M5.4** Saat online, antrean disetor via endpoint batch idempoten; retry aman
  (`client_scan_id` dedup di server).
- **FR-M5.5** Konflik antar-device (aset sama, hasil beda) diselesaikan server
  **first-write-wins per aset per sesi** dan dilaporkan per item; device menampilkan konflik
  agar petugas bisa mengoreksi manual.
- **FR-M5.6** Status antrean selalu terlihat (jumlah belum tersinkron + indikator offline);
  kegagalan sync tidak pernah senyap.
- **FR-M5.7** Aset dalam-scope yang di luar snapshot dapat discan (server menambah item
  `expected=false` — perilaku existing); variance sesi dapat dilihat dari mobile.
- **FR-M5.8** Data lokal sesi dihapus setelah sesi selesai dan tersinkron penuh.

### 3.6 Profil, sesi & preferensi (FR-M6)

- **FR-M6.1** Profil ringkas (nama, peran, kantor) + daftar **sesi device** dengan sesi saat ini
  ditandai; cabut sesi lain / logout semua.
- **FR-M6.2** Preferensi: bahasa (id default / en), tema (terang / gelap / ikuti sistem).
- **FR-M6.3** Seluruh UI ter-i18n; istilah domain konsisten dengan web (opname, mutasi, BAST).

## 4. Alur utama

1. **Opname offline**: login, buka sesi, unduh snapshot; sinyal hilang; scan ratusan label dan
   hasil masuk antrean; kembali ke area bersinyal; antrean tersinkron otomatis; cek variance;
   selesai. Konflik (bila ada) tampil untuk dikoreksi.
2. **Approval dari push**: push masuk, tap, detail pengajuan tampil; tinjau lalu approve/reject
   dengan catatan; maker menerima push keputusan.
3. **Identifikasi aset**: buka Scan, arahkan kamera ke label, detail aset tampil, selesai
   (atau tandai hasil bila sedang dalam sesi opname).

## 5. Kebutuhan Non-Fungsional

- **Keamanan**: token hanya di secure storage; tidak ada kredensial tersimpan plaintext; build
  release ter-obfuscate; data opname lokal dihapus pasca-sync; pencabutan sesi server dihormati
  seketika; tidak ada logika otorisasi di klien.
- **Keandalan offline**: antrean scan persisten (SQLite/drift), tahan crash & restart; sync
  idempoten (retry tanpa duplikasi).
- **Performa**: scan-ke-hasil terasa instan pada label kondisi baik; daftar & feed memakai
  pagination server yang ada.
- **Kompatibilitas**: Android 8.0+ (API 26+) untuk v1; struktur proyek siap iOS.
- **Aksesibilitas**: target sentuh minimal 48dp, kontras memenuhi, dukung font scaling OS.
- **Observability**: crash reporting (Crashlytics/Sentry) sejak rilis internal pertama.

## 6. Arsitektur teknis (ringkas)

Detail dan alternatif di ADR-0015/0016; ringkasnya:

- Flutter (Dart 3), folder `mobile/` di monorepo; Riverpod, Dio (+ cookie jar), freezed +
  json_serializable, drift, mobile_scanner, flutter_secure_storage, intl/ARB.
- Konsumen `/api/v1` yang sama dengan web. **Endpoint baru yang dibutuhkan hanya dua kelompok**:
  device-token push (fase M3) dan batch sync opname (fase M5) — keduanya mengikuti urutan standar
  modul backend (migration, sqlc, handler 4-file, authz eksplisit, OpenAPI).
- CI: `flutter analyze` + `flutter test` + build APK; integration test melawan docker-compose
  backend (pola job e2e yang ada).

## 7. Tahapan

Fase M0 (fondasi) sampai M6 (rilis internal) dengan estimasi dan prasyarat dirinci di
`docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`. M1 (scan), M2 (approval), M3 (push)
independen setelah M0; rilis internal pertama yang bermakna = M0 + M1 + M2. Tiap fase mendapat
spec + plan implementasi tersendiri, dengan mockup `docs/mobile/design/` sebagai sumber kebenaran
visual sebelum layar dibangun.

## 8. Asumsi & Pertanyaan Terbuka

- **AM1** — Device operasional bank adalah Android; tidak ada kebutuhan MDM khusus di v1.
- **AM2** — FCM dapat dipakai (device ber-Google-Play-Services); env kredensial FCM wajib
  terdaftar di `docker-compose.prod.yml` (pelajaran kasus env Resend).
- **AM3** — Distribusi internal via Firebase App Distribution / APK langsung; Play Store internal
  track menyusul.
- **AM4** — Satu sesi opname bisa dikerjakan lebih dari satu device (dasar keputusan konflik
  ADR-0016).
- **QM1 (terbuka)** — Perlukah mode kiosk/shared-device (satu device dipakai bergantian banyak
  petugas)? Belum diasumsikan; login per-user biasa.
- **QM2 (terbuka)** — Certificate pinning untuk API produksi: dievaluasi saat fase M6 (rilis).

---

## Changelog

- **v1.0 (2026-07-18)** — Dokumen awal. Scope dibuka melalui PRD web v1.2 (non-goal "aplikasi
  mobile native" v1.1 dicabut); keputusan bentuk field companion + Flutter + offline-first opname.
