# ADR-0015 — Aplikasi mobile companion: Flutter (field companion, Android dulu)

- Status: Accepted
- Date: 2026-07-18
- Deciders: pemilik proyek + sesi perencanaan mobile (lihat
  `docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`)
- Terkait: [ADR-0016](0016-stock-opname-offline-sync.md) (strategi offline sync opname).
  Men-supersede butir non-goal PRD v1.1 "Aplikasi mobile native" (PRD dinaikkan ke v1.2).

## Konteks

PRD v1.1 mengecualikan aplikasi mobile native dengan alasan web responsif sudah cukup. Sejak itu
seluruh menu web memang sudah mobile-responsive (PR #95), tetapi tiga kebutuhan lapangan tetap tidak
terlayani baik oleh web di perangkat genggam:

1. **Stock opname di lokasi tanpa sinyal** (gudang, basement, lokasi ATM) — web butuh koneksi;
   tidak ada penyimpanan lokal yang andal untuk antrean scan.
2. **Scan kamera** — pemindaian barcode/QR via browser (getUserMedia) berfungsi tapi lambat dan
   rapuh dibanding pipeline kamera native; petugas opname memindai ratusan label per sesi.
3. **Push notification** — approver perlu tahu ada pengajuan menunggu tanpa membuka aplikasi;
   web push tidak andal lintas platform (khususnya iOS) untuk aplikasi internal.

Pada 2026-07-18 pemilik produk memutuskan membuka scope mobile dengan bentuk **field companion**
(scan aset, approval on-the-go, push, stock opname offline) — bukan paritas penuh dengan web.
Prinsip proyek (README ADR): pilih standar industri yang matang, bukan shortcut diferensiasi.

## Keputusan

**Flutter (Dart 3), Android dulu, satu folder `mobile/` di monorepo ini.**

- **Framework: Flutter** — satu codebase Android + iOS; praktik yang lazim di perbankan Indonesia
  (BTN Mobile, BCA, BRI memakai Flutter); ekosistem plugin kamera dan storage lokal matang.
- **Target rilis: Android dulu** (device operasional bank umumnya Android); proyek disiapkan agar
  build iOS tinggal diaktifkan tanpa restrukturisasi.
- **Pustaka inti**: Riverpod (state), Dio (HTTP + interceptor auth), `freezed` +
  `json_serializable` (model DTO), `drift` (SQLite untuk offline opname), `mobile_scanner`
  (kamera QR/barcode), `flutter_secure_storage` (token), `intl`/ARB (i18n id + en).
- **Auth**: memakai JWT backend yang ada. Refresh token tetap cookie httpOnly — klien memakai
  cookie jar persisten (`dio_cookie_manager`), sehingga **tidak ada perubahan backend** untuk auth
  di v1. Bila cookie jar terbukti rapuh di OEM Android tertentu, fallback yang direncanakan adalah
  jalur refresh khusus mobile yang mengembalikan `refresh_token` di body (disimpan di secure
  storage) — perpindahan itu akan dicatat sebagai ADR baru.
- **CI**: job `flutter analyze` + `flutter test` + build APK di workflow yang ada; integration
  test melawan docker-compose backend meniru pola job e2e Playwright.

## Alternatif yang ditolak

- **Capacitor membungkus Nuxt yang ada.** Reuse maksimal dan paling cepat, tetapi UX kamera dan
  offline bergantung pada WebView + plugin bridge (persis titik lemah yang mendorong kebutuhan
  mobile), dan aplikasi admin penuh ikut terbawa padahal scope-nya field companion.
- **PWA murni.** Tidak butuh distribusi APK, tapi push di iOS terbatas, storage lokal bisa
  di-evict OS, dan scan kamera tetap jalur browser — tiga-tiganya adalah alasan scope ini dibuka.
- **React Native / Expo.** Dekat dengan skill JS/TS tim web dan ekosistem Expo bagus, tetapi kurang
  lazim di perbankan Indonesia dibanding Flutter; pipeline kamera/scanner dan tooling offline-nya
  tidak lebih matang.
- **Native Kotlin (Android saja).** Kontrol platform paling penuh, tetapi effort per fitur paling
  tinggi, menutup jalur iOS, dan tidak selaras dengan kebutuhan "companion ringan di atas API yang
  sudah ada".

## Konsekuensi

- Stack ketiga di repo (Go, TypeScript/Vue, kini Dart) — biaya belajar diterima; mitigasi lewat
  fase M0 kecil dan source-driven development (dokumentasi resmi Flutter/Riverpod/drift).
- Konvensi design-fidelity berlaku juga di mobile: **mockup mobile dibuat lebih dulu**
  (`docs/design/mobile/`, perluasan `docs/DESIGN_BRIEF.md`) sebelum layar dibangun.
- Backend perlu dua kemampuan baru: **push FCM** (tabel device token + dispatcher sebagai consumer
  tambahan di pipeline notifikasi ADR-0014) dan **endpoint batch sync opname** (ADR-0016).
- Otorisasi tidak berubah: semua enforcement (permission, data scope, field permission, SoD)
  tetap di server; klien mobile hanya konsumen `/api/v1`.
- Distribusi awal internal (Firebase App Distribution atau APK langsung); Play Store internal
  track menyusul. Crash reporting (Crashlytics/Sentry) masuk fase rilis.
- Fase implementasi (M0 fondasi sampai M6 rilis) dirinci di
  `docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`; tiap fase tetap mendapat spec + plan
  sendiri.
