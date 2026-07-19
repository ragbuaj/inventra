# Plan — Mobile v1: fondasi Flutter + auth per-klien + 12 layar 1:1

Spec: `docs/superpowers/specs/2026-07-19-mobile-m0-foundation-design.md` (revisi scope
2026-07-19: seluruh 12 layar mockup, fungsional dengan API nyata, satu PR). Branch
`feat/mobile-m0`.

Setiap task diverifikasi hijau sebelum lanjut. Task layar (8-12) dikerjakan berurutan (satu
working tree; file bersama: router, ARB, core/widgets). Gate akhir: backend
`go build/vet/test ./...` + `go test -tags=integration ./...` + Spectral; mobile
`flutter analyze` (nol warning) + `flutter test`; CI job mobile hijau di PR.

## Task 1 — Backend: klaim `aud` + validasi issuer/audience

- `internal/auth/jwt.go`: audience di `Claims`; issue access+refresh dengan `aud`; `Parse`
  validasi issuer `inventra` + tolak `aud` tak dikenal; token tanpa `aud` diperlakukan `web`.
- `internal/auth/jwt_test.go`: roundtrip per audience, issuer salah, `aud` asing, `aud` absen.
- Verifikasi: `go test ./internal/auth/`.

## Task 2 — Backend: login/refresh/logout per-klien + SessionAlive

- `internal/identity/handler.go`: baca `X-Client-Type` saat login; `aud=mobile` berarti
  `refresh_token` di body + tanpa `Set-Cookie`; refresh/logout menerima `refresh_token` dari
  body bila cookie absen.
- `internal/identity/service.go`: `Refresh` cek `SessionAlive(claims.SID)` sebelum rotasi;
  propagasi audience saat rotasi.
- `internal/identity/dto.go`: field `refresh_token` omitempty (hanya terisi untuk mobile).
- Revisi `handler_cookie_test.go` per-klien; tambah test handler login/refresh/logout mobile.
- Verifikasi: `go test ./internal/identity/`.

## Task 3 — Backend: RequireAudience + wiring + OpenAPI + integration

- `internal/middleware/auth.go`: set `CtxAudience`. `internal/middleware/audience.go` (baru):
  `RequireAudience(allowed ...string)` + unit test.
- `internal/server/router.go`: deny `aud=mobile` pada grup `authzadmin` + `importer`.
- `backend/api/openapi.yaml`: header `X-Client-Type`, varian body refresh/logout.
- Integration: login-refresh-logout mobile e2e; deny authzadmin utk mobile; regresi web.
- Verifikasi: `go build/vet ./...`, `go test -tags=integration ./...` (penuh — shared
  signature), Spectral.

## Task 4 — Amendemen dokumen cookie jar (ADR-0017)

- `docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md` (bagian 3 no. 2, baris M0):
  Opsi A/B diganti rujukan ADR-0017 (refresh body per-klien).
- `docs/mobile/ARCHITECTURE.md` bagian 4 (interceptor: hapus cookie jar, tambah bearer +
  refresh body + `X-Client-Type`) dan bagian 6 (secure storage, cold start via body).
- `docs/mobile/CONVENTIONS.md` bagian 8 (cookie jar terenkripsi diganti refresh token di
  secure storage).
- Verifikasi: grep `cookie jar|dio_cookie_manager` di docs/mobile + roadmap hanya tersisa
  dalam konteks historis ADR.

## Task 5 — Flutter: scaffold + tema + i18n + CI

- `flutter create mobile` (org `id.web.ragilbuaj`, platforms `android,ios`); rapikan ke
  struktur ARCHITECTURE.md; `analysis_options.yaml` sesuai CONVENTIONS bagian 1; pubspec
  dependensi M0; bundle font Inter; `.gitignore` Flutter standar.
- `app/theme.dart`: ThemeData M3 light+dark dari token mockup + `ThemeExtension` warna status
  domain. `core/i18n`: ARB `id` + `en` via gen_l10n.
- `.github/workflows/ci.yml`: job `mobile` (analyze, test, build apk debug) dengan path filter
  `mobile/**`.
- Verifikasi: `flutter analyze` nol warning, `flutter test` (smoke), commit file codegen.

## Task 6 — Flutter: core/api + core/auth + unit test

- `core/api`: Dio + interceptor auth (Bearer, 401 refresh single-flight, `X-Client-Type:
  mobile`) + error mapper `AppFailure` (sealed).
- `core/auth`: secure storage refresh token, access di memori, `authControllerProvider`
  (cold start refresh via body, login, logout end-point + bersih-bersih).
- Repository + DTO freezed `features/login/data/` (`/auth/login`, `/auth/refresh`,
  `/auth/logout`).
- Unit test: interceptor (single-flight, gagal berarti logout), mapper per `AppFailure`,
  controller cold start, repository (Dio mock).
- Verifikasi: `flutter analyze`, `flutter test`.

## Task 7 — Flutter: router + shell + komponen bersama + LoginScreen 1:1

- `app/router.dart`: go_router + guard auth + **seluruh tabel rute v1** (login, 5 tab shell,
  detail aset `/assets/:tag`, approval `/approval/:id`, opname `/stock-opname/:id` +
  `/stock-opname/:id/variance`, profil, pengaturan) — layar belum dibangun sementara diisi
  placeholder; task 8-12 mengganti placeholder-nya masing-masing.
- `app/shell.dart`: bottom-nav 5 slot + FAB Pindai tengah 1:1 mockup (pill aktif, badge
  unread di Notif).
- `core/widgets/`: komponen Component Library yang dipakai lintas layar — StatusChip (aset/
  opname/pengajuan via ThemeExtension), EmptyState, AppSkeleton, OfflineBanner, SyncPill,
  ConfirmDialog.
- `features/login/presentation/`: LoginScreen 1:1 mockup (3 state), controller Riverpod.
- Widget test per state via kunci i18n; golden Login light+dark; test shell 5 slot; test
  komponen bersama.
- Verifikasi: `flutter analyze`, `flutter test`.

## Task 8 — Layar: Scan + Detail Aset (FR-M2)

- Dep baru `mobile_scanner`. `features/scan/`: kamera full screen + torch + input tag manual
  1:1 mockup; hasil scan/submit menuju `/assets/:tag`.
- `features/asset_detail/`: `GET /assets/by-tag/:tag` — header foto/status, seksi info 1:1;
  field absen (field permission) dirender "—" + penanda dibatasi; state loading/error/
  not-found.
- Unit test repository; widget test per state; golden light+dark kedua layar.

## Task 9 — Layar: Approval Inbox + Detail (FR-M3)

- `features/approval/`: inbox `/requests` (filter status, kartu 1:1), detail + approve/reject
  dengan catatan (ConfirmDialog), guard SoD/permission dari respons API (403 dirender sopan).
- Unit test repository (termasuk cabang 403/konflik); widget test aksi approve/reject +
  state; golden light+dark kedua layar.

## Task 10 — Layar: Opname (Daftar Sesi + Counting + Variance) (FR-M4, online-only)

- `features/stock_opname/`: daftar sesi, counting (scan bar via kamera + manual, progress,
  daftar item, hasil per item), variance — semua endpoint online yang ada.
- Elemen offline/sync dirender 1:1 (SyncPill, OfflineBanner) dengan perilaku online-only:
  offline berarti scan dinonaktifkan + pesan; TANPA drift/antrean (M5). Deviasi dicatat.
- Unit + widget test per state; golden light+dark tiga layar.

## Task 11 — Layar: Beranda + Notifikasi

- `features/home/`: ringkasan 1:1 (opname aktif, approval menunggu, notifikasi terbaru,
  quick actions) dari endpoint list yang ada — panggilan supplementary non-fatal (catch,
  jangan blokir halaman).
- `features/notifications/`: feed + unread + tandai dibaca; badge unread shell tersambung.
- Unit + widget test; golden light+dark kedua layar.

## Task 12 — Layar: Profil & Sesi Device + Pengaturan

- `features/account/`: profil (data diri), sesi device (daftar + revoke, sesi ini ditandai),
  logout (pindah dari penanda sementara task 7); pengaturan bahasa (id/en) + tema
  (light/dark/system) persist lokal.
- Unit + widget test; golden light+dark kedua layar.

## Task 13 — Finalisasi

- Perbandingan 1:1 seluruh 12 layar (light + dark) terhadap mockup — dilaporkan di PR;
  deviasi tercatat (opname online-only).
- `docs/PROGRESS.md`: centang M0 + layar v1 dengan nomor PR; catat deviasi.
- Vault Obsidian: Status & Roadmap + catatan sesi.
- Gate penuh hijau (backend + mobile + Spectral + CI); PR `feat/mobile-m0` ke `main`.
