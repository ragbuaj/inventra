# Account Security via Email (Spec A) — Design

**Date:** 2026-07-13
**Status:** Approved (design)
**Source:** PROGRESS.md item 53 — user-picked next step: "reset password dan sesi perangkat". This spec
is the **first** of a two-spec decomposition; **Spec B — Device Sessions** (session list / revoke /
logout-others) is a separate follow-up spec.

## 1. Overview

Add real, email-backed account-security flows, replacing the mock `useAccount` seam and the placeholder
"Lupa password?" link. Three cohesive, email-centric features that share one email sender + one
session-invalidation mechanism:

1. **Email infrastructure** — a provider-agnostic SMTP sender (`internal/email`) + branded Indonesian
   templates + a `LogSender` fallback for dev/CI; Mailpit as the local mail-catcher.
2. **Forgot-password** — unauthenticated reset via an emailed, single-use, short-lived token.
3. **Change-password** (authenticated) — verify the current password, apply the new one, and send a
   **notification** email (no email click-gate — industry standard).

**Cross-cutting:** a password change (via either flow) invalidates **all** sessions (full logout
everywhere, including the current device), plus rate-limiting, anti-enumeration, and audit logging.

**Explicitly out of scope (deferred):** device-session list/revoke UI (→ Spec B), profile-field editing
(telepon/kantor), admin-initiated reset + force-change-on-next-login, per-user email localization
(Indonesian default is used).

## 2. Key architecture decisions (alternatives → choice)

- **Session invalidation on password change** — (a) *token-epoch* via a `password_changed_at` column:
  reject any token issued before that timestamp; (b) per-user session index + delete every JTI;
  (c) rotate the global `JWT_SECRET`. **Chosen: (a)** — durable, simple, needs no session index (that is
  Spec B's job), and the column doubles as "password last changed" metadata. Both the forgot-reset flow
  **and** the authenticated change-password flow revoke **all** sessions (user decision 2026-07-13), so
  the mechanism is uniform: bump `password_changed_at = now()`.
- **SMTP library** — (a) `github.com/wneessen/go-mail` (modern, STARTTLS/implicit-TLS/none, actively
  maintained, no CGO); (b) stdlib `net/smtp` (no dependency but manual, rigid TLS). **Chosen: (a)**.
- **Reset-token transport** — the token travels in the reset-link URL query (`/reset-password?token=…`).
  This is standard practice and is safe here: the token is single-use, 30-min TTL, and only its SHA-256
  hash is stored server-side. (The platform rule against putting sensitive data in URLs concerns *us*
  placing user PII into third-party URLs; a self-issued, single-use reset token in our own link is the
  established pattern.)

## 3. Backend

### 3.1 New package `internal/email`

- `Sender` interface: `Send(ctx context.Context, to, subject, htmlBody, textBody string) error`.
- `SMTPSender` — go-mail client built from config (host/port/username/password/from/from-name/TLS mode).
- `LogSender` — used when `MAIL_ENABLED=false` or `SMTP_HOST` is empty: logs the subject + recipient +
  (for reset) the link, so dev/CI without Mailpit still exercises the full flow. `NewSender(cfg, logger)`
  picks the impl.
- Templates via `html/template` + a plain-text alternative, Indonesian, Inventra-branded, embedded with
  `//go:embed`:
  - `password_reset` — greeting, reset link (`FRONTEND_URL/reset-password?token=<token>`), 30-min expiry
    note, "abaikan jika bukan Anda" line.
  - `password_changed` — notification that the password was just changed, with a "jika bukan Anda,
    segera hubungi admin" line.
- A small `Mailer` helper wraps `Sender` + templates: `SendPasswordReset(ctx, to, name, link)` and
  `SendPasswordChanged(ctx, to, name)`.

### 3.2 Config (`internal/config/config.go` + `backend/.env.example`)

New fields (env → default):
- `MAIL_ENABLED` (`false`) — when false (or host empty), use `LogSender`.
- `SMTP_HOST` (`""`), `SMTP_PORT` (`1025` — Mailpit default), `SMTP_USERNAME` (`""`),
  `SMTP_PASSWORD` (`""`), `SMTP_FROM` (`no-reply@inventra.local`), `SMTP_FROM_NAME` (`Inventra`),
  `SMTP_TLS` (`none` — one of `none|starttls|tls`).
- Reuse existing `FRONTEND_URL` for the reset link and `RATELIMIT_*` plumbing.
- `PASSWORD_RESET_TTL` (`30m`) via `getEnvDuration`.

### 3.3 Migration `000028_password_reset`

- `ALTER TABLE identity.users ADD COLUMN password_changed_at timestamptz;` (nullable; NULL = never
  changed → no token is rejected on epoch grounds). Down: drop the column.
- No new table — reset tokens live in Redis only (ephemeral by nature).

### 3.4 Reset-token store (Redis)

Extend `internal/auth` (new `pwreset.go` or methods on `TokenStore`):
- `SavePasswordReset(ctx, tokenHash, userID string, ttl)` → `SET auth:pwreset:<tokenHash> <userID> EX ttl`.
- `ConsumePasswordReset(ctx, tokenHash) (userID string, ok bool)` → atomic `GETDEL` (single-use).
- Token = 32 random bytes (`crypto/rand`) → base64url string; stored key uses `sha256(token)` hex. The
  raw token is only ever in the email link.

### 3.5 Service (`internal/identity/service.go`) additions

- `RequestPasswordReset(ctx, email)` — look up the user; if it exists, is `active`, and is an
  email-login account (`password_hash != nil`), generate a token, store its hash, and send the reset
  email via the `Mailer`. **Always returns nil** (anti-enumeration) — a missing/ineligible user is a
  silent no-op. Google-only accounts get no token.
- `ResetPassword(ctx, token, newPassword)` — `ConsumePasswordReset`; on miss → `ErrInvalidToken`. Load
  the user, `UpdateUserPassword` (bcrypt hash + `password_changed_at = now()`), send the
  `password_changed` notification, return the user for audit. All sessions are now invalid by epoch.
- `ChangePassword(ctx, userID, oldPassword, newPassword)` — load user; verify `oldPassword` (bcrypt);
  reject Google-only/no-hash with `ErrInvalidCredentials`; `UpdateUserPassword` (+ epoch bump); send
  `password_changed`; return the user. **All** sessions (including the caller's) are invalidated — no
  token re-issue.
- `Refresh` — add an epoch check: after loading the user, reject when the refresh token's `IssuedAt` is
  before `user.PasswordChangedAt` (→ `ErrInvalidToken`). Access tokens (15-min TTL) expire naturally;
  the middleware denylist is unchanged.
- New sentinel: `ErrWeakPassword` (min length enforced at the DTO layer; service double-checks length as
  defense-in-depth).

### 3.6 Query (`db/queries/identity.sql` → `sqlc generate`)

- `UpdateUserPassword :exec` — `UPDATE identity.users SET password_hash = $2, password_changed_at = now()
  WHERE id = $1 AND deleted_at IS NULL`.
- `GetUserByEmail` / `GetUserByID` already exist; ensure the generated `IdentityUser` row exposes
  `password_changed_at` (add to any narrowed SELECTs used by `Refresh`).

### 3.7 Endpoints (`internal/identity/handler.go` + `routes.go`)

Unauthenticated (rate-limited via existing `middleware.PerIP` + per-account key, mirroring `login`):
- `POST /auth/password/forgot` `{ "email": "..." }` → **always 200** `{ "status": "ok" }`. Per-IP limit
  (`auth_pwforgot`) + per-account limit (`pwforgot:acct:<email>`).
- `POST /auth/password/reset` `{ "token": "...", "new_password": "..." }` → 200 on success; 400 on
  invalid/expired/used token or weak password.

Authenticated (`authMW`):
- `PUT /auth/password` `{ "old_password": "...", "new_password": "..." }` → 200
  `{ "status": "password_changed" }`; 400/401 on wrong old password or weak new password. The caller's
  session is now invalid — the client must discard tokens and re-login.

DTO validation: `new_password` `binding:"required,min=8"`; `email` `binding:"required,email"`.

### 3.8 Wiring + audit

- `internal/server/router.go` (`NewRouter`): construct `email.NewSender(cfg, logger)` + `Mailer`, pass
  into the identity `Service`; register the three routes with rate-limit params.
- Audit: record `password_reset` (actor = the reset user) and `password_changed` (actor = the user) via
  the existing `audit` service, no diff payload (never log password material).

## 4. Frontend

- **`/forgot-password`** (layout `auth`): email `UInput` + `UButton` → `POST /auth/password/forgot`;
  regardless of outcome show the success panel ("Jika email terdaftar, tautan reset telah dikirim.").
  Handle 429 with a "coba lagi nanti" message.
- **`/reset-password`** (layout `auth`): read `?token=` from the route; new-password + confirm inputs +
  the existing `passwordStrength` meter → `POST /auth/password/reset`; success → toast + redirect to
  `/login`; invalid/expired token → inline error with a link back to `/forgot-password`.
- **`login.vue`**: point the existing "Lupa password?" link (currently a placeholder, `login.vue:139`)
  at `/forgot-password`; drop the "not wired" comment.
- **`account.vue` "Ganti Password" card**: wire `useAccount().changePassword` to `PUT /auth/password`;
  on success, **log out** (clear auth state) + toast "Password diubah, silakan login kembali" + redirect
  to `/login` (all sessions were revoked).
- **`useAccount.ts`**: replace the mock `changePassword` with a real `$fetch` (`apiBase` +
  `PUT /auth/password`) mapping backend errors to the existing i18n error keys. `getProfile` /
  `listSessions` / `revokeSession` / `logoutAllOthers` stay mock in this spec (→ Spec B / profile spec).
- **i18n**: add id/en keys for both new pages, the login link, and the change-password result toasts.

## 5. Error handling

- Forgot: always 200 (anti-enumeration); 429 on rate-limit.
- Reset: invalid/expired/used token → 400 generic ("Tautan tidak valid atau kedaluwarsa"); weak password
  → 400 validation.
- Change: wrong old password → 400/401; weak new password → 400.
- SMTP failure: logged; **does not** change the HTTP outcome — forgot still returns 200 (no leak); a
  change/reset still succeeds (the notification email is best-effort). Reset-link send failure is logged
  at error level.

## 6. Security

Token: 32 random bytes, SHA-256-hashed at rest, single-use (`GETDEL`), 30-min TTL. Anti-enumeration on
forgot. Per-IP + per-account rate limits on forgot/reset. Google-only accounts (no `password_hash`) can
never reset (still 200). Password change/reset revokes all sessions by epoch. Min password length 8
enforced at the DTO. No password material ever logged or audited. All behind the existing WAF + HTTPS.

## 7. Testing

- **Backend unit**: token generate/verify + hash; `RequestPasswordReset` (existing/missing/inactive/
  google-only all silent-ok); `ResetPassword` (happy, expired, reused, weak pw); `ChangePassword`
  (happy, wrong old pw, google-only, weak pw); `Refresh` epoch rejection (token issued before
  `password_changed_at`); email templating; `LogSender` fallback.
- **Backend integration**: full forgot→reset and change flows against Postgres + Redis with a fake
  `Sender` capturing the sent email/link; assert `password_changed_at` set and a pre-change refresh
  token is rejected afterward.
- **Frontend**: `useAccount.changePassword` unit; `mountSuspended` for `/forgot-password`,
  `/reset-password` (valid + invalid-token states), and the account card (success → logout/redirect,
  error states); the login "Lupa password?" link target.
- **E2E (real backend + Mailpit)**: request a reset in the UI → fetch the email via **Mailpit's HTTP
  API** → open the reset link → set a new password → log in with it; negative: an expired/garbage token
  shows the error state. CI's e2e job gains a Mailpit service and `MAIL_ENABLED=true` +
  `SMTP_HOST=mailpit`.

## 8. Deliverables checklist

- [ ] `internal/email` (sender + LogSender + templates + Mailer) with unit tests
- [ ] Config + `.env.example` SMTP block; docker-compose dev **Mailpit** service; CI e2e Mailpit
- [ ] Migration `000028_password_reset` (+ down) and `sqlc` regen for `UpdateUserPassword`
- [ ] Redis reset-token store (`auth/pwreset.go`)
- [ ] Identity service: `RequestPasswordReset` / `ResetPassword` / `ChangePassword` + `Refresh` epoch
- [ ] Three endpoints + rate-limit + audit + `NewRouter` wiring
- [ ] OpenAPI: three paths + request/response schemas
- [ ] Frontend: `/forgot-password`, `/reset-password`, login link, account-card wiring, `useAccount`
- [ ] i18n id/en; unit + component + e2e (Mailpit) tests
- [ ] `docs/PROGRESS.md` updated (item 53 → done for Spec A; Spec B queued)
