# Spec — Admin-initiated password reset (User Management)

Tanggal: 2026-07-18. Status: disetujui, siap implementasi.

## Konteks & masalah

Layar **User Management** (`/settings/users`, mockup `docs/design/Manajemen User.dc.html`) punya
aksi baris **"Reset Password"** di menu kebab. Aksi ini belum dibangun ("reset-password action
dropped pending a backend reset endpoint" — lihat PROGRESS item 12/35). Pemblokir aslinya adalah
tidak adanya pipeline email; itu **sudah hilang sejak item 54** (account-security-email: Mailer +
template + Mailpit di stack dev/e2e). Sekarang fitur ini bisa dibangun end-to-end.

## Keputusan mekanisme

Admin **memicu email reset** ke user target. Backend membuat token reset sekali-pakai (TTL Redis),
menyimpannya, lalu mengirim email berisi tautan `/reset-password?token=...`. User membuka tautan dan
menyetel password baru sendiri lewat halaman `/reset-password` yang **sudah ada**.

Alasan (bukan admin mengetik/melihat password):

- **Best-practice keamanan**: admin tidak boleh melihat/menyetel password milik user. Tautan reset
  ber-TTL adalah standar industri (selaras memory *prefer-industry-best-practice*).
- **PRD FR-1.5**: "reset password via token" — mekanisme token sudah jadi kontrak.
- **Reuse maksimal**: memakai ulang `auth.GenerateResetToken`/`TokenStore.SavePasswordReset`/
  `Mailer.SendPasswordReset` + halaman `/reset-password` + template email. Tidak ada migrasi, tidak
  ada kolom baru (mis. "must change on next login" tidak diperlukan).

Perbedaan dari flow self-service (`RequestPasswordReset`): jalur admin **tidak** silent
(anti-enumeration tidak relevan — admin sudah tahu usernya) dan mengembalikan hasil yang jelas
supaya UI bisa memberi umpan balik.

## Perilaku backend

Endpoint baru: `POST /api/v1/users/:id/reset-password` — digerbangi `RequireAuth` +
`RequirePermission(user.manage)` (satu grup dengan endpoint `/users` lain).

Metode service baru di `internal/identity`:
`AdminInitiatePasswordReset(ctx, targetUserID uuid.UUID) (email string, err error)`.

Logika:

1. Ambil user by id. Tidak ada lalu `ErrNotFound` (HTTP 404).
2. `password_hash == nil` (akun Google-only, tak punya login password) lalu sentinel baru
   `ErrNoPasswordLogin` (HTTP 422) — reset email tidak masuk akal untuk akun tanpa password.
3. Selain itu: buat + simpan token, kirim email reset, kembalikan `user.Email`.
   - **Permisif terhadap status**: user `inactive`/`suspended` yang punya password **tetap**
     dikirim (admin memilih aksi ini secara eksplisit; gerbang status adalah urusan saat login).
     Ini deviasi sadar dari `RequestPasswordReset` yang memblok non-active — dicatat.

Respons sukses `200`: `{ "status": "sent", "email": "<user email>" }`.

Audit: `audit.Record` sebagai `ActionUpdate` pada entity `users` (id target, office target) dengan
payload penanda `{"password_reset": "email_sent"}` — aksi sensitif keamanan wajib tercatat. Tidak
menambah nilai enum `shared.audit_action` (hindari migrasi); update-semantics konsisten dengan
email-change.

Wiring: `user.Handler` mendapat dependensi baru lewat **interface sempit** yang didefinisikan di
paket `user` (bukan import balik yang bikin siklus — `identity` tidak meng-import `user`):

```go
type passwordResetInitiator interface {
    AdminInitiatePasswordReset(ctx context.Context, targetUserID uuid.UUID) (string, error)
}
```

`*identity.Service` memenuhi interface ini; di `NewRouter`, `identitySvc` (sudah dibangun sebelum
`userHandler`) dioper ke `user.NewHandler`. Handler membedakan error lewat sentinel `identity.*`
(import `identity` untuk perbandingan error aman — tanpa siklus).

## Perilaku frontend

`useUsers().resetPassword(id)` lalu `POST /users/:id/reset-password`, mengembalikan `{ status, email }`.

`users.vue`: tambah aksi baris **"Reset Password"** (ikon `i-lucide-key-round`) di urutan mockup
(Edit, **Reset Password**, Aktif/Nonaktif, Hapus), digerbangi `can('user.manage')`. Klik lalu dialog
konfirmasi (`useConfirm`) yang menjelaskan "tautan reset akan dikirim ke email user". Sukses lalu
toast sukses menyebut email tujuan. `422` (Google-only) lalu toast khusus "akun ini login via Google".

i18n: kunci baru di `i18n/locales/{id,en}.json` (label aksi, judul/isi konfirmasi, toast sukses,
toast Google-only).

## Pengujian

- **Backend unit** (`identity/service_test.go`): sukses (kirim + email dikembalikan), Google-only
  (`ErrNoPasswordLogin`), not-found (`ErrNotFound`). Pakai fake mailer/token-store yang sudah ada di
  test identity.
- **Backend integration** (`user/handler_integration_test.go`): `200` sent (fake initiator), `422`
  Google-only, `404` missing; audit tercatat. Suntik `passwordResetInitiator` palsu (interface bikin
  ini mudah — tak perlu Redis/mailer nyata di test user).
- **Frontend component** (`test/nuxt/`): aksi muncul untuk `user.manage`; konfirmasi lalu panggil
  API; toast sukses; cabang 422 lalu toast Google-only; batal lalu tak ada panggilan.
- **Frontend e2e** (`e2e/`): login admin lalu buat user throwaway via API (dengan password) lalu buka
  User Management lalu klik Reset Password lalu verifikasi email masuk Mailpit + tautan
  `/reset-password?token=`. Cleanup failure-safe (soft-delete user). Reuse `latestResetLink` +
  purge-mailbox dari `password-reset.spec.ts`.

## Non-tujuan

- Admin mengetik/menyetel password langsung (ditolak — lihat keputusan mekanisme).
- Flag "wajib ganti password saat login berikut" (tak perlu; reset token sudah memaksa set baru).
- Reset password massal / bulk (di luar lingkup).
