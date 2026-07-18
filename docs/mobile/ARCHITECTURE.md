# Arsitektur Aplikasi Mobile — Inventra Field Companion

Arsitektur klien Flutter untuk aplikasi mobile companion. Melengkapi [PRD.md](PRD.md) (kebutuhan)
dan [ADR-0015](adr/0015-mobile-companion-flutter.md)/[ADR-0016](adr/0016-stock-opname-offline-sync.md)
(keputusan stack dan offline sync). Konvensi penulisan kode ada di [CONVENTIONS.md](CONVENTIONS.md).

Prinsip yang menurunkan semua keputusan di dokumen ini:

1. **Klien tipis.** Mobile adalah konsumen `/api/v1` — tidak ada aturan bisnis, tidak ada logika
   otorisasi, tidak ada perhitungan domain di klien. Satu-satunya state kaya di sisi klien adalah
   **stock opname offline** (ADR-0016), dan itu pun sebatas antrean kiriman, bukan aturan.
2. **Sederhana dulu.** Lapisan hanya ditambah saat terbukti dibutuhkan; tidak ada layer "domain"
   terpisah, tidak ada abstraksi use-case, karena hampir semua alur adalah baca-tampilkan dan
   kirim-aksi.
3. **Selaras preseden repo.** DTO English snake_case mengikuti kontrak backend (paritas dengan
   konvensi web ADR-0007), file generated di-commit (preseden sqlc), verifikasi sebelum commit.

## 1. Struktur folder

Feature-first: kode dikelompokkan per fitur, bukan per jenis file. Kerangka `mobile/`:

```
mobile/
  lib/
    main.dart                 # entry point: init DI, tema, router, i18n
    app/
      router.dart             # tabel rute go_router + guard auth + deep-link
      theme.dart              # ThemeData Material 3 dari token Inventra (light + dark)
      shell.dart              # scaffold bottom-nav 5 slot (tombol Scan tengah)
    core/
      api/                    # Dio client, interceptor auth/refresh, error mapper
      auth/                   # session state, secure storage, cookie jar
      db/                     # inisialisasi drift database (dipakai fitur opname)
      i18n/                   # l10n.yaml output, helper locale
      utils/                  # formatter (tanggal, Rp), logger
      widgets/                # komponen lintas-fitur: OfflineBanner, SyncPill,
                              #   StatusChip, EmptyState, AppSkeleton, ConfirmDialog
    features/
      login/
        data/                 # repository + DTO (freezed) untuk /auth/*
        presentation/         # LoginScreen + controller Riverpod
      home/
      scan/                   # kamera (mobile_scanner) + input manual + hasil
      asset_detail/           # GET /assets/by-tag/:tag, masking field-permission
      approval/               # inbox + detail + putuskan (/requests)
      notifications/          # feed + unread + registrasi token FCM + deep-link
      stock_opname/
        data/                 # repository API + tabel drift + sync engine
        presentation/         # daftar sesi, counting, variance
      account/                # profil, sesi device, pengaturan
  test/                       # unit + widget test (struktur mencerminkan lib/)
  integration_test/           # alur end-to-end melawan backend compose
  android/                    # proyek platform (iOS menyusul)
```

Aturan ketergantungan: `features/*` boleh memakai `core/*` dan `app/*`; **antar-fitur tidak saling
impor** — kebutuhan bersama dinaikkan ke `core/`. `core/` tidak pernah mengimpor `features/`.

Setiap fitur memakai dua sub-lapisan saja:

- **`data/`** — repository (satu kelas per sumber API), DTO `freezed` + `json_serializable`
  (field English snake_case sama persis dengan kontrak OpenAPI backend), dan untuk opname: tabel
  drift + sync engine. Repository mengembalikan DTO atau melempar `AppFailure` — tidak tahu UI.
- **`presentation/`** — screen (widget), widget lokal fitur, dan controller Riverpod
  (`AsyncNotifier`) yang memanggil repository dan memegang state layar. Tidak ada `Dio`/`drift`
  yang disentuh langsung dari widget.

## 2. State management — Riverpod

- **Riverpod tanpa codegen provider** (deklarasi manual): jumlah provider aplikasi ini kecil;
  satu lapisan codegen lebih sedikit berarti build lebih cepat dan lebih mudah dipahami.
  (`freezed`/`json_serializable`/`drift` tetap memakai `build_runner`.)
- State layar dimodelkan `AsyncValue<T>` dari `AsyncNotifier` — loading/error/data gratis dan
  konsisten; widget cukup `switch` di ketiganya (paritas dengan konvensi loading/empty/error web).
- State global hanya tiga: **sesi auth** (`authControllerProvider` — status login, user, logout),
  **konektivitas** (stream online/offline untuk banner dan pemicu sync), dan **unread count**
  notifikasi. Selain itu state hidup di fitur masing-masing.
- Controller tidak menyimpan hasil yang bisa diminta ulang murah — daftar approval, feed
  notifikasi, dan detail aset di-fetch per tampil (dengan `ref.invalidate` untuk refresh), bukan
  di-cache manual.

## 3. Navigasi — go_router

- Satu tabel rute di `app/router.dart`. Shell route membungkus 5 tab bottom-nav (Beranda,
  Opname, Scan, Approval, Notifikasi); layar sekunder (detail, counting) berada di atas shell
  tanpa bottom-nav — sesuai design brief.
- **Guard auth**: redirect ke `/login` bila tidak ada sesi; kembali ke tujuan semula setelah
  login.
- **Deep-link** adalah alasan utama memilih go_router: payload data FCM membawa path rute
  (mis. `/approval/:id`, `/stock-opname/:id`) dan handler notifikasi cukup `push` path itu —
  satu mekanisme untuk cold start maupun app berjalan.

## 4. Jaringan dan kontrak API

- **Dio** tunggal dari `core/api`, dengan tiga interceptor berurutan:
  1. **Cookie jar persisten** (`dio_cookie_manager` + `cookie_jar`) — memegang refresh cookie
     httpOnly; inilah mekanisme refresh v1 tanpa perubahan backend (ADR-0015).
  2. **Auth**: menempelkan `Authorization: Bearer <access>` dari memori; saat 401, melakukan
     refresh **single-flight** (request lain menunggu satu refresh yang sama) lalu mengulang
     request; bila refresh gagal, sesi dinyatakan mati dan router mengarahkan ke login.
  3. **Error mapper**: mengubah error Dio/HTTP menjadi `AppFailure` yang seragam.
- **`AppFailure`** adalah sealed class kecil: `network` (offline/timeout), `unauthorized`,
  `forbidden`, `notFound`, `validation(message)`, `conflict`, `server`, `unknown`. Widget
  menampilkan pesan i18n per jenis — tidak pernah menampilkan string error mentah backend.
- DTO dibuat manual per endpoint yang dipakai (bukan generate seluruh OpenAPI) — permukaan API
  yang dikonsumsi mobile kecil; nama field mengikuti OpenAPI persis supaya perbandingan kontrak
  trivially diff-able.
- Field permission dihormati apa adanya: field yang tidak dikirim backend dirender "—" dengan
  penanda dibatasi — klien tidak menebak.

## 5. Offline stock opname (implementasi ADR-0016)

Satu-satunya bagian stateful. Semua berada di `features/stock_opname/data/`:

- **Tabel drift**: `opname_sessions` (snapshot metadata sesi yang diunduh),
  `opname_items` (item snapshot + hasil lokal), `scan_queue` (antrean kiriman:
  `client_scan_id` UUID, asset_tag, hasil, catatan, `scanned_at`, status
  `pending|sent|conflict`).
- **Alur tulis**: scan menghasilkan satu transaksi drift — update `opname_items` + insert
  `scan_queue`. UI membaca stream drift, jadi progres dan pill sync selalu konsisten dengan isi
  antrean walau app di-restart (FR-M5.3).
- **Sync engine**: worker tunggal yang terbangun oleh (a) perubahan konektivitas ke online,
  (b) insert antrean saat online, (c) buka layar sesi. Ia mengirim batch
  `POST /stock-opname/sessions/:id/scans/batch`, menandai `sent` per item yang diterima, dan
  menandai `conflict` untuk item yang kalah first-write-wins (hasil pemenang disimpan untuk
  ditampilkan). Retry memakai backoff sederhana; idempotensi dijamin server via `client_scan_id`,
  jadi kirim ulang selalu aman.
- **Lifecycle data**: snapshot dan antrean sesi dihapus setelah sesi selesai dan antrean kosong
  (FR-M5.8). Drift bukan arsip; tidak ada data aset lain yang disimpan lokal.

## 6. Autentikasi dan sesi

- Access token hanya di memori; refresh cookie di cookie jar persisten yang file-nya disimpan
  terenkripsi via `flutter_secure_storage` (kunci enkripsi) — token tidak pernah menyentuh
  SharedPreferences.
- Cold start: baca cookie jar, panggil `/auth/refresh`; sukses berarti sesi hidup (skip login),
  gagal berarti ke login. Logout memanggil `/auth/logout` lalu membersihkan jar + memori +
  deregistrasi token FCM.
- Pencabutan sesi dari server (device sessions) terlihat sebagai 401 yang gagal refresh —
  ditangani jalur yang sama, tanpa kode khusus.

## 7. Push notification (FCM)

- Token FCM diregistrasikan ke backend setelah login dan dideregistrasi saat logout
  (endpoint fase M3). Refresh token FCM (rotasi oleh Google) memicu registrasi ulang.
- Payload push memuat `type` + `params` + path deep-link. Teks final dirender klien dari i18n
  (konsisten ADR-0014 — server tidak pernah mengirim kalimat jadi).
- Foreground: tampilkan SnackBar/inbox update tanpa notifikasi OS ganda; background/terminated:
  notifikasi OS, tap membuka path via go_router.

## 8. Tema dan i18n

- `app/theme.dart` menurunkan `ThemeData` Material 3 dari token Inventra: seed primary green,
  neutral slate, radius/spacing mengikuti design brief; light + dark dari satu sumber. Tidak ada
  warna literal di widget — semua lewat `Theme.of(context)`/ekstensi token.
- i18n memakai ARB (`intl`) — `id` default + `en`, paritas istilah domain dengan
  `frontend/i18n/locales/*.json` (glosarium vault sebagai acuan). Tidak ada string UI hardcode.

## 9. Observability

- Crash reporting (Crashlytics atau Sentry — diputuskan di fase M6) dipasang sejak rilis internal
  pertama; error `AppFailure.unknown` dan kegagalan sync tercatat sebagai non-fatal ber-konteks
  (fitur, endpoint) tanpa PII/token.
- Logger `core/utils` membungkus `dart:developer log` — `print` dilarang (lint).

## 10. Testing (peta ke arsitektur)

| Lapisan | Jenis tes | Contoh |
|---|---|---|
| `core/api` | unit | interceptor refresh single-flight; mapping error ke `AppFailure` |
| `features/*/data` | unit | repository (Dio di-mock); sync engine: retry, konflik, idempoten |
| `features/stock_opname` | unit + drift in-memory | transaksi scan, stream progres, lifecycle hapus |
| `features/*/presentation` | widget + golden | tiap state `AsyncValue`; golden light + dark |
| Alur lintas-fitur | `integration_test/` | login, scan-ke-detail, approval, opname offline-sync melawan backend compose |

Detail konvensi penulisan tes (penamaan, cakupan wajib) ada di [CONVENTIONS.md](CONVENTIONS.md).
