# Plan — Mobile M0: fondasi Flutter + auth per-klien

Spec: `docs/superpowers/specs/2026-07-19-mobile-m0-foundation-design.md`. Branch `feat/mobile-m0`.

Setiap task diverifikasi hijau sebelum lanjut. Gate akhir: backend `go build/vet/test ./...` +
`go test -tags=integration ./...` + Spectral; mobile `flutter analyze` (nol warning) +
`flutter test`; CI job mobile hijau di PR.

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

## Task 7 — Flutter: router + shell + LoginScreen 1:1

- `app/router.dart`: go_router + guard auth; `app/shell.dart`: bottom-nav 5 slot + FAB Pindai
  tengah sesuai mockup; placeholder EmptyState 4 tab; logout sementara di app bar Beranda.
- `features/login/presentation/`: LoginScreen 1:1 mockup (3 state), controller Riverpod.
- Widget test per state via kunci i18n; golden Login light+dark; test shell 5 slot.
- Verifikasi: `flutter analyze`, `flutter test` (termasuk golden); perbandingan 1:1 layar
  Login terhadap mockup dilaporkan di PR.

## Task 8 — Finalisasi

- `docs/PROGRESS.md`: centang M0 dengan nomor PR; catat penanda sementara (logout di app bar
  placeholder Beranda).
- Vault Obsidian: Status & Roadmap + catatan sesi M0.
- Gate penuh hijau (backend + mobile + Spectral + CI); PR `feat/mobile-m0` ke `main`.
