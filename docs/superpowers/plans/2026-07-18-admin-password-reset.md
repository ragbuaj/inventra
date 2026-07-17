# Plan — Admin-initiated password reset

Spec: `docs/superpowers/specs/2026-07-18-admin-password-reset-design.md`. Branch
`feat/admin-password-reset`.

Setiap task diverifikasi hijau sebelum lanjut. Gate akhir: `go build/vet/test ./...`, Spectral,
`pnpm lint/typecheck/test/build`, dan e2e (butuh stack up + Mailpit + seeded admin).

## Task 1 — Backend: metode service `AdminInitiatePasswordReset`

- `internal/identity/service.go`: sentinel baru `ErrNoPasswordLogin`; metode
  `AdminInitiatePasswordReset(ctx, targetUserID uuid.UUID) (string, error)` (ambil user, cek
  password_hash, generate+save token, kirim email, kembalikan email). Cerminan `RequestPasswordReset`
  tapi by-id, tidak silent, permisif status.
- `internal/identity/service_test.go`: 3 kasus (sent / Google-only / not-found).
- Verifikasi: `go test ./internal/identity/`.

## Task 2 — Backend: endpoint + handler + wiring + audit + OpenAPI

- `internal/user/handler.go`: field `reset passwordResetInitiator`; interface `passwordResetInitiator`;
  metode `resetPassword` (parse id, panggil initiator, map error identity.ErrNotFound->404 /
  identity.ErrNoPasswordLogin->422 / else 500, audit ActionUpdate, respons 200 `{status,email}`).
- `internal/user/handler.go` `NewHandler`: parameter baru `reset passwordResetInitiator`.
- `internal/user/routes.go`: `g.POST("/:id/reset-password", h.resetPassword)`.
- `internal/server/router.go`: oper `identitySvc` ke `user.NewHandler`.
- `internal/user/handler_integration_test.go`: fake initiator + 3 kasus (200/422/404); tambahkan
  parameter fake ke pemanggilan `user.NewHandler` yang sudah ada.
- `backend/api/openapi.yaml`: path `POST /users/{id}/reset-password` + skema respons.
- Verifikasi: `go build/vet ./...`, `go test -tags=integration ./internal/user/`, Spectral lint.

## Task 3 — Frontend: composable + aksi UI + i18n

- `frontend/app/composables/api/useUsers.ts`: `resetPassword(id)`.
- `frontend/app/pages/settings/users.vue`: aksi baris "Reset Password" (ikon key-round, urutan
  mockup), `onResetPassword` dengan `useConfirm` + toast sukses + cabang 422.
- `frontend/i18n/locales/{id,en}.json`: kunci baru.
- Verifikasi: `pnpm lint`, `pnpm typecheck`.

## Task 4 — Frontend: component test

- `frontend/test/nuxt/`: aksi tampil (user.manage), konfirmasi->panggil API, toast sukses, cabang
  422 toast Google-only, batal->tak ada panggilan.
- Verifikasi: `pnpm test` (subset spec), lalu `pnpm build`.

## Task 5 — Frontend: e2e (real backend + Mailpit)

- `frontend/e2e/admin-password-reset.spec.ts`: buat user throwaway via API (dengan password), buka
  User Management, klik Reset Password, verifikasi Mailpit menerima email + tautan reset; cleanup
  failure-safe (soft-delete). Reuse helper mailpit dari `password-reset.spec.ts`.
- Verifikasi: `pnpm test:e2e` (spec ini), lalu jalankan suite penuh.

## Task 6 — Dokumentasi + finalisasi

- `docs/PROGRESS.md`: centang follow-up "admin-initiated password reset" (item 69 carried candidate +
  TODO di item 12/35); tulis catatan + deviasi (permisif status).
- Vault Obsidian: catat keputusan produk (mekanisme email-based) di `Keputusan/Produk/` + update
  status roadmap bila relevan.
- Gate penuh hijau: backend build/vet/test(+integration), Spectral, frontend
  lint/typecheck/test/build, e2e suite penuh.
