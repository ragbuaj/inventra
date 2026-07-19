# Spec — Mobile v1: fondasi Flutter + auth per-klien (ADR-0017) + 12 layar 1:1

Tanggal: 2026-07-19. Status: disetujui, siap implementasi. Branch `feat/mobile-m0`.

Revisi 2026-07-19 (keputusan pemilik produk): scope diperluas dari M0 menjadi **seluruh 12
layar mockup dibangun 1:1 dan fungsional dengan API nyata** dalam satu PR (setara
M0+M1+M2+M4). Batasnya: hanya endpoint yang sudah ada — push FCM (M3) dan offline sync opname
(M5) tetap fase backend tersendiri. Konsekuensi yang disetujui: layar Opname Counting
menampilkan elemen offline/sync sesuai mockup namun berperilaku **online-only** (saat offline,
scan dinonaktifkan dengan pesan jelas); deviasi perilaku ini dicatat di PROGRESS.md.

## Konteks & masalah

Roadmap mobile (`docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`) mendefinisikan M0:
scaffold `mobile/`, tema + i18n, navigasi shell, login/refresh/logout, secure storage, CI job
Flutter. Prasyarat mockup terpenuhi 2026-07-19 — 12 layar + component library di
`docs/mobile/design/` (commit 79bc948).

ADR-0017 (merged PR #105) mengubah desain auth M0: rencana cookie jar persisten di-supersede
oleh jalur refresh per-klien (klaim `aud`, refresh token di body untuk mobile, disimpan
`flutter_secure_storage`). Konsekuensinya M0 kini punya irisan backend yang dulu tidak ada,
dan tiga dokumen masih menyebut desain lama (roadmap bagian 3 no. 2 dan baris M0;
`ARCHITECTURE.md` bagian 4 & 6; `CONVENTIONS.md` bagian 8) — diamendemen di PR ini.

Lingkungan: Flutter 3.44.6 stable di `C:\flutter` (tanpa Android SDK lokal — build APK di CI;
runner ubuntu-latest sudah menyertakan Android SDK).

## Perilaku backend (implementasi ADR-0017 keputusan 2, 3, 4, dan 6-M1)

### Klaim `aud` + validasi (internal/auth)

- `Claims` mendapat audience (`"web"` / `"mobile"`); access dan refresh token di-issue dengan
  `aud` yang sama. `Parse` memvalidasi issuer `inventra` (`jwt.WithIssuer` — selama ini di-set
  tapi tidak diverifikasi) dan menolak `aud` di luar himpunan dikenal.
- **Kompatibilitas rollout**: token tanpa `aud` (sesi hidup pra-deploy) diperlakukan sebagai
  `web` — refresh berikutnya menerbitkan token ber-`aud`. Tanpa ini seluruh sesi web produksi
  ter-logout saat deploy.

### Login/refresh/logout per-klien (internal/identity)

- Login membaca header `X-Client-Type`; nilai `mobile` menghasilkan `aud=mobile`, selain itu
  (termasuk absen) `web`. Header bukan otorisasi — mengaku mobile mempersempit akses.
- **Web: perilaku sekarang, tidak berubah** (refresh via cookie httpOnly; body tidak pernah
  memuat `refresh_token`).
- **Mobile**: respons login/refresh memuat `refresh_token` di body dan **tidak** men-set
  cookie. `POST /auth/refresh` dan `POST /auth/logout` menerima refresh token dari body
  (`{"refresh_token": "..."}`) bila cookie absen. Rotasi refresh tetap identik.
- `Refresh` menambah cek `SessionAlive(claims.SID)` sebelum rotasi (sejajar `RequireAuth`) —
  defense-in-depth ADR-0017 M-3.
- Test invariant `handler_cookie_test.go` direvisi per-klien: klien web tetap tidak boleh
  menerima `refresh_token` di body; klien mobile wajib menerimanya dan tanpa `Set-Cookie`.

### RequireAudience (internal/middleware + router)

- `RequireAuth` menaruh audience ke context (`CtxAudience`). Middleware baru
  `RequireAudience(allowed ...string)` menolak 403 bila audience tidak termasuk.
- Daftar deny `aud=mobile` awal (eksplisit di `router.go`, perubahan lewat review PR):
  grup `authzadmin` dan `importer`. Belum ada rute mobile-only di M0.

### OpenAPI

Header `X-Client-Type` pada login; varian body `refresh_token` pada login/refresh/logout;
Spectral hijau.

## Perilaku klien Flutter (mobile/)

Scaffold `flutter create` (org `id.web.ragilbuaj`, platform `android,ios`), struktur
feature-first persis `ARCHITECTURE.md` bagian 1. Dependensi M0 saja: `flutter_riverpod`,
`go_router`, `dio`, `flutter_secure_storage`, `freezed` + `json_serializable` (+
`build_runner`), `intl`/ARB via `gen_l10n`, `material_symbols_icons` (ikon mockup), dev:
`flutter_lints`, `mocktail`. (`drift`, `mobile_scanner`, FCM menyusul di fasenya.)

- **Tema** (`app/theme.dart`): `ThemeData` Material 3 light + dark dari token hasil ekstraksi
  mockup (primary green `#16a34a`/`#22c55e`, slate, semantik success/warning/error/info,
  radius 12-24, Inter di-bundle sebagai asset). Warna status domain (aset/opname/pengajuan)
  masuk `ThemeExtension` — tidak ada `Color(0xFF...)` literal di widget (CONVENTIONS bagian 3).
- **i18n**: ARB `id` (default) + `en`, kunci camelCase berprefix layar; tanpa string hardcode.
- **`core/api`**: Dio tunggal + interceptor berurutan: (1) auth — Bearer access token dari
  memori; 401 memicu refresh single-flight lalu ulang request; refresh gagal berarti sesi mati;
  (2) error mapper ke sealed `AppFailure`. **Tanpa cookie jar.** Login/refresh mengirim
  `X-Client-Type: mobile`.
- **`core/auth`**: refresh token hanya di `flutter_secure_storage`; access token hanya di
  memori; `authControllerProvider` (status sesi, user, logout). Cold start: baca refresh dari
  secure storage, panggil `/auth/refresh` (body); sukses berarti skip login. Logout memanggil
  `/auth/logout` (body) lalu membersihkan storage + memori.
- **Router + shell** (`app/router.dart`, `app/shell.dart`): go_router, guard auth redirect ke
  `/login`; shell bottom-nav 5 slot sesuai mockup Beranda/Component Library: Beranda (`home`),
  Opname (`fact_check`), tombol Pindai tengah menonjol (FAB kotak primary, menjorok atas,
  border surface), Approval (`approval`), Notif (`notifications`).
- **Layar v1 — seluruh 12 mockup dibangun 1:1 dan fungsional** (revisi scope). Per layar:
  - **Login** (`/auth/login` per-klien): logo + wordmark + badge MOBILE, card form, tiga state
    (normal/error banner inline/loading), switch bahasa ID/EN, teks versi.
  - **Beranda**: ringkasan tugas (sesi opname aktif, approval menunggu, notifikasi terbaru)
    dari endpoint list yang ada; quick actions; header profil.
  - **Scan** (`mobile_scanner`): kamera full screen + input tag manual (fallback wajib —
    paritas web), hasil mengarah ke Detail Aset.
  - **Detail Aset** (`GET /assets/by-tag/:tag`): read-only; field yang tidak dikirim backend
    (field permission) dirender "—" dengan penanda dibatasi — klien tidak menebak.
  - **Inbox Approval** + **Detail Approval** (`/requests`): daftar + filter status, detail,
    approve/reject dengan catatan; guard SoD/permission sepenuhnya dari respons API.
  - **Daftar Sesi Opname**, **Opname Counting**, **Variance Opname** (endpoint stock opname
    online yang ada): scan bar + progress + daftar item; elemen offline/sync dirender sesuai
    mockup dengan perilaku online-only (deviasi M5 tercatat).
  - **Notifikasi**: feed + unread count (endpoint in-app yang ada); tanpa push (M3).
  - **Profil & Sesi Device**: data diri, daftar sesi login per device + revoke, logout.
  - **Pengaturan**: bahasa (id/en) + tema (light/dark/system), persist lokal.
  - Komponen bersama dari Component Library (StatusChip, EmptyState, SyncPill, OfflineBanner,
    AppSkeleton, ConfirmDialog, dsb.) dibangun sekali di `core/widgets/`.
  - Tiap layar ditutup **perbandingan 1:1 light + dark terhadap mockup-nya** (konvensi
    design-fidelity), dilaporkan di PR.

## Pengujian

- **Backend unit**: jwt (roundtrip `aud`, issuer salah ditolak, `aud` tak dikenal ditolak,
  token tanpa `aud` = web); identity (login web vs mobile: cookie vs body; refresh dari body;
  refresh sesi revoked ditolak; logout body); middleware `RequireAudience` (allow/deny/absen).
- **Backend integration**: jalur login-refresh-logout mobile end-to-end; deny `aud=mobile` di
  rute authzadmin; regresi jalur web. Gate `go test -tags=integration ./...` penuh
  (shared-signature berubah — memory full-integration-gate).
- **Flutter unit**: interceptor auth (401 memicu satu refresh single-flight; gagal berarti
  logout), error mapper per jenis `AppFailure`, `authControllerProvider` cold start
  (sukses/gagal).
- **Flutter widget + golden**: LoginScreen per state (loading/error/data) via kunci i18n;
  golden Login light + dark; shell menampilkan 5 slot + FAB.
- **CI**: job `mobile` baru di `.github/workflows/ci.yml` — `flutter analyze` (nol warning),
  `flutter test`, `flutter build apk --debug` (subosito/flutter-action, channel stable).

## Non-tujuan

- Layar mockup selain Login (Beranda dkk. dibangun di fasenya masing-masing).
- `drift`, `mobile_scanner`, FCM/push, deep-link handler (fase M3-M5).
- Endpoint mobile-only (belum ada kebutuhan di M0).
- `role_epoch` (opsional ADR-0017, tidak memblokir).
- Android Studio/emulator lokal; distribusi APK (fase M6).
- Integration test Flutter melawan compose (menyusul bersama fase berlayar-data; M0 cukup
  unit + widget + golden + integration backend).
