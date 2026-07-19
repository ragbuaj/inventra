# Kesiapan iOS — Inventra Field Companion

Rencana lengkap untuk mengaktifkan target **iOS**. Status saat ini: **iOS belum menjadi target
rilis** — fokus rilis adalah Android (ADR-0015, keputusan ditegaskan ulang pemilik produk
2026-07-18). Dokumen ini memastikan dua hal: (1) kode yang ditulis sejak M0 **tidak pernah
menutup pintu iOS**, dan (2) saat iOS diaktifkan, semua prasyarat, langkah, dan risikonya sudah
terpetakan sehingga aktivasi tinggal eksekusi.

## 1. Status dan pemicu aktivasi

- Keputusan berlaku: **Android dulu** (ADR-0015). Dokumen ini bukan perubahan keputusan itu.
- Aktivasi iOS adalah **keputusan produk terpisah** nanti (dicatat di vault
  `Keputusan/Produk/`); tidak butuh ADR baru selama arsitektur tidak berubah — semua library
  terpilih sudah lintas platform (bagian 3).
- Pemicu yang masuk akal: ada pengguna ber-iPhone di lapangan, atau kebutuhan demo/portfolio
  iOS. Prasyarat teknisnya di bagian 4 — **tanpa akses macOS, aktivasi tidak mungkin dimulai**.
- Estimasi aktivasi: **1-2 sesi kerja** setelah prasyarat bagian 4 tersedia; dapat dikerjakan
  kapan saja setelah M6 (atau paralel dengannya).

## 2. Aturan "iOS-ready" yang berlaku SEJAK M0 (wajib, walau rilisnya Android)

Aturan berikut mengikat semua kode `mobile/` sejak scaffold, agar aktivasi iOS tidak menjadi
proyek refactor:

1. **Folder `ios/` dibuat sejak scaffold dan di-commit** (hasil `flutter create` standar) —
   jangan dihapus, ikut ter-update saat upgrade Flutter.
2. **Tidak ada API platform Android-only di kode bersama.** Bila suatu saat butuh kode
   per-platform, pisahkan lewat abstraksi di `core/` dengan implementasi per platform — bukan
   `Platform.isAndroid` bertebaran di fitur.
3. **SafeArea sejak awal.** Shell bottom-nav (termasuk tombol Scan tengah), banner offline, dan
   bar aksi sticky bawah selalu menghormati `SafeArea`/`viewPadding` — notch dan home indicator
   iOS bukan pemikiran ulang, melainkan sudah ditangani dari hari pertama.
4. **Jangan mematikan back-swipe.** Navigasi memakai perilaku pop default go_router; tidak ada
   override `WillPopScope`/`PopScope` yang mengubur gesture kembali khas iOS, kecuali di layar
   yang memang butuh konfirmasi keluar (counting opname) — dan itu pun lewat dialog, bukan
   memblokir diam-diam.
5. **Izin runtime lewat satu flow platform-aware** (kamera, notifikasi): string alasan izin
   disiapkan untuk kedua platform sejak awal (bagian 6), prompt dimintakan pada momen yang tepat
   (kamera saat pertama membuka Scan; notifikasi setelah login — bukan saat cold start).
6. **Konfigurasi build netral platform** via `--dart-define`; tidak ada path/asumsi
   Android-specific di kode bersama.
7. **CI tidak menguji iOS dulu** (biaya runner macOS, bagian 7) — tetapi pelanggaran aturan
   1-6 diperlakukan sebagai bug review, bukan "nanti saja".

## 3. Kompatibilitas library (sudah beres — tidak ada penggantian)

Semua pilihan ADR-0015/ARCHITECTURE sudah lintas platform; kolom kanan adalah implementasi
di sisi iOS:

| Library | Backend iOS-nya |
|---|---|
| Riverpod, go_router, freezed, json_serializable, intl | Dart murni — tidak ada kode platform |
| Dio | HTTP Dart murni; refresh token via body (ADR-0017), tanpa kode platform |
| drift | SQLite via `sqlite3_flutter_libs` (FFI) — bundel SQLite sendiri, tidak bergantung versi OS |
| mobile_scanner | AVFoundation + Vision (kamera dan dekode native iOS) |
| flutter_secure_storage | **Keychain** (setel accessibility `first_unlock` agar token terbaca saat app bangun di background) |
| firebase_messaging | FCM menunggangi **APNs** (butuh setup bagian 5) |
| Crashlytics/Sentry (M6) | dSYM upload untuk symbolication crash iOS |

Target OS minimum saat aktivasi: **iOS 13** (minimum Flutter saat ini; naikkan mengikuti versi
Flutter yang dipakai saat itu).

## 4. Prasyarat aktivasi (checklist sekali jalan)

- [ ] **Akses macOS + Xcode** — build dan signing iOS hanya bisa dari macOS. Dua pola kerja:
      Mac lokal (pengembangan + debug penuh) atau **CI macOS saja** (cukup untuk build/rilis,
      debug terbatas). Tentukan pola sebelum mulai. *(Pertanyaan terbuka QM3 — belum diputuskan.)*
- [ ] **Apple Developer Program** — 99 USD/tahun, atas nama pemilik proyek; memberi akses
      signing, APNs, TestFlight, App Store Connect.
- [ ] **Bundle ID** didaftarkan (usulan: `id.web.ragilbuaj.inventra` — cermin domain produksi).
- [ ] **Signing**: mulai dengan Xcode *automatically manage signing* (paling sederhana untuk
      satu app satu developer); pindah ke fastlane `match` hanya bila kelak ada lebih dari satu
      mesin/orang yang menandatangani.
- [ ] **APNs Auth Key (.p8)** dibuat di Apple Developer, diunggah ke project Firebase (satu key
      berlaku semua app di akun itu; tidak kedaluwarsa seperti sertifikat lama).
- [ ] **`GoogleService-Info.plist`** (konfigurasi Firebase iOS) ditambahkan ke target Xcode —
      dan masuk daftar berkas yang tidak boleh berisi secret tak perlu.

## 5. Push notification di iOS (perluasan fase M3)

Desain backend **tidak berubah**: backend hanya bicara ke FCM (ADR-0014 consumer push), FCM yang
meneruskan ke APNs. Yang perlu dikerjakan di sisi app/Apple saat aktivasi:

1. Capability **Push Notifications** + **Background Modes: Remote notifications** di Xcode.
2. APNs key terpasang di Firebase (bagian 4).
3. **Izin notifikasi iOS**: prompt sistem sekali seumur hidup app — minta setelah login dengan
   layar penjelas singkat (pola yang sama sudah ada di mockup Pengaturan: baris peringatan bila
   izin OS dimatikan, tautan ke pengaturan sistem).
4. Verifikasi perilaku foreground (tampilkan in-app, bukan banner OS ganda — paritas perilaku
   Android yang sudah dispesifikasikan di ARCHITECTURE bagian 7) dan tap-to-deep-link dari
   terminated state.

## 6. Deklarasi izin (Info.plist)

Disiapkan tekstnya sekarang (i18n id/en via `InfoPlist.strings` saat aktivasi):

| Kunci | Alasan yang ditampilkan |
|---|---|
| `NSCameraUsageDescription` | "Kamera dipakai untuk memindai label barcode/QR aset." |
| (Notifikasi — tanpa kunci plist, via prompt `firebase_messaging`) | Layar penjelas: "Terima pemberitahuan approval dan maintenance." |

Tidak ada izin lain: aplikasi tidak memakai lokasi, kontak, galeri (foto aset hanya
*ditampilkan* dari server, tidak diunggah dari galeri di v1).

## 7. Build dan CI

- Job CI baru `mobile-ios` di runner **macOS**: `flutter build ipa`. Karena menit macOS
  terhitung 10 kali lipat, job ini **tidak** berjalan per-PR — jalankan saat tag rilis dan
  jadwal mingguan (deteksi dini regresi build iOS), sementara analyze/test/APK Android tetap
  per-PR di Linux.
- **Golden test tetap dijalankan di satu platform CI (Linux) saja** — rendering font antar-OS
  berbeda piksel; golden lintas-platform adalah sumber flake klasik. Verifikasi visual iOS
  dilakukan manual di simulator saat aktivasi dan menjelang rilis.
- Integration test iOS (simulator di runner macOS) opsional; minimum yang wajib adalah smoke
  test manual checklist bagian 9.

## 8. Distribusi iOS

- **Jalur utama: TestFlight** — internal tester (sampai 100, tanpa review App Store) untuk
  pegawai; build diunggah dari CI macOS atau Mac lokal via `flutter build ipa` +
  App Store Connect. Rapi, tidak butuh registrasi UDID, dan jalur menuju App Store terbuka
  bila kelak dibutuhkan.
- Alternatif ditolak sebagai jalur utama: **Firebase App Distribution ad-hoc** (batas 100
  device dengan registrasi UDID manual per perangkat — merepotkan) dan **Apple Enterprise
  Program** (299 USD/tahun, syarat organisasi ketat — berlebihan untuk skala ini).
- Versioning mengikuti Android (satu `pubspec.yaml`: `version: x.y.z+build`); rilis iOS dan
  Android dari tag yang sama.

## 9. Checklist QA saat aktivasi (perbedaan platform yang wajib diverifikasi)

- [ ] Safe area: bottom-nav + tombol Scan tengah, banner offline, bar aksi sticky di perangkat
      ber-notch dan home indicator (simulator iPhone SE = layar terkecil, iPhone Pro Max =
      terbesar).
- [ ] Back-swipe di semua layar sekunder; konfirmasi keluar counting opname tetap muncul.
- [ ] Alur izin kamera: pertama kali, ditolak, lalu diaktifkan dari Settings (app harus pulih
      tanpa restart).
- [ ] Alur izin notifikasi + push end-to-end (approval_pending sampai deep-link) dari state
      foreground, background, dan terminated.
- [ ] Opname offline: airplane mode, scan, restart app (antrean bertahan), online, sinkron.
- [ ] Keychain: token bertahan setelah restart app dan reboot device; logout membersihkannya.
- [ ] Keyboard iOS tidak menutupi field catatan (bottom sheet hasil scan, bar catatan approval).
- [ ] Crash reporter menerima dSYM (crash iOS ter-symbolicate).

## 10. Ringkasan biaya dan keputusan yang masih terbuka

| Hal | Nilai |
|---|---|
| Apple Developer Program | 99 USD/tahun |
| Runner CI macOS | menit x10 — dimitigasi jadwal tag/mingguan |
| Estimasi kerja aktivasi | 1-2 sesi (setup + distribusi + QA sweep bagian 9) |
| **QM3 (terbuka)** | Pola kerja macOS: Mac lokal vs CI-saja — putuskan sebelum aktivasi |
| **QM4 (terbuka)** | Waktu aktivasi: setelah M6 Android, atau paralel — keputusan produk |
