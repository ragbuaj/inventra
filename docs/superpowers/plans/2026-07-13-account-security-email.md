# Account Security via Email (Spec A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add real email-backed account security — forgot-password (emailed single-use token), authenticated change-password (verify old + notify), and the supporting SMTP email infrastructure — replacing the mock `useAccount` seam.

**Architecture:** New `internal/email` package (provider-agnostic SMTP via `go-mail` + a `LogSender` dev fallback + embedded Indonesian templates). Reset tokens live in Redis (SHA-256-hashed, single-use, 30-min TTL). Both password-change flows bump a new `identity.users.password_changed_at` column; `Refresh` rejects any refresh token issued before it, so a password change logs out **all** sessions. Frontend gains `/forgot-password` + `/reset-password` pages and wires the existing account "Ganti Password" card.

**Tech Stack:** Go 1.25 + Gin, pgx/sqlc, Redis (go-redis), `github.com/wneessen/go-mail`, Mailpit (dev/CI mail-catcher), Nuxt 4 + Pinia + Vitest + Playwright.

## Global Constraints

- Go module path: `github.com/ragbuaj/inventra`. Backend commands run from `backend/`.
- Migrations follow soft-delete + `set_updated_at` conventions; never hand-edit `backend/db/sqlc/` — edit `db/queries/*.sql` / migrations and run `sqlc generate`.
- Money/sensitive columns are never serialized; **password material is never logged or audited**.
- Backend list/HTTP responses are JSON `gin.H`; sentinel errors map to HTTP status in the handler, never the service.
- Frontend: build on Nuxt UI `U*` components; i18n mandatory (`i18n/locales/{id,en}.json`, default `id`); theme via tokens; ESLint stylistic — **no trailing commas**, 1tbs braces; API access via `useApiClient()`/`$fetch` against `runtimeConfig.public.apiBase` — never hardcode the backend URL.
- Audit `Action` is the enum `create|update|delete` only — reuse `audit.ActionUpdate` with a `{"event": ...}` changes marker; do **not** add a new enum value.
- Gates that must stay green: `go build ./...`, `go vet ./...`, `go test ./...`, Spectral lint; frontend `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
- Password minimum length: **8** (DTO `binding:"required,min=8"`).
- Reset token TTL default: **30m** (`PASSWORD_RESET_TTL`). Mailpit SMTP: host `mailpit`, port `1025`, HTTP API `8025`.

---

## File Structure

**Backend — create:**
- `backend/internal/email/sender.go` — `Sender` interface, `SMTPSender`, `LogSender`, `NewSender`, `Options`.
- `backend/internal/email/mailer.go` — `Mailer` (templates → `Sender`): `SendPasswordReset`, `SendPasswordChanged`.
- `backend/internal/email/templates/password_reset.html`, `password_reset.txt`, `password_changed.html`, `password_changed.txt` — embedded via `//go:embed`.
- `backend/internal/email/mailer_test.go` — template render + `LogSender` capture.
- `backend/internal/auth/pwreset.go` — token generate/hash + Redis reset-token store methods.
- `backend/internal/auth/pwreset_test.go` — pure token/hash unit tests.
- `backend/db/migrations/000028_password_reset.up.sql` / `.down.sql`.

**Backend — modify:**
- `backend/db/queries/identity.sql` — add `UpdateUserPassword`.
- `backend/internal/config/config.go` + `backend/.env.example` — SMTP config block.
- `backend/internal/identity/service.go` — new methods + interfaces + epoch check.
- `backend/internal/identity/dto.go` — request DTOs.
- `backend/internal/identity/handler.go` — new handlers + audit field.
- `backend/internal/identity/routes.go` — new routes.
- `backend/internal/identity/service_test.go` (new file alongside existing tests) — service unit tests.
- `backend/internal/server/router.go` — wire email + audit into identity.
- `backend/api/openapi.yaml` — three new paths.
- `docker-compose.yml`, `docker-compose.dev.yml`, `.github/workflows/ci.yml` — Mailpit.

**Frontend — create:**
- `frontend/app/pages/forgot-password.vue`, `frontend/app/pages/reset-password.vue`.
- `frontend/test/nuxt/forgot-password.spec.ts`, `frontend/test/nuxt/reset-password.spec.ts`.
- `frontend/e2e/password-reset.spec.ts`.

**Frontend — modify:**
- `frontend/app/composables/api/useAccount.ts` — real `changePassword`; add `requestPasswordReset`, `resetPassword`.
- `frontend/app/pages/login.vue` — point "Lupa password?" at `/forgot-password`.
- `frontend/app/pages/account.vue` — real change-password → logout + redirect.
- `frontend/app/middleware/auth.global.ts` — add the two public paths.
- `frontend/i18n/locales/id.json`, `en.json` — new strings.
- `frontend/test/nuxt/useAccount.spec.ts` — updated expectations.

---

## Task 1: Migration + `UpdateUserPassword` query

**Files:**
- Create: `backend/db/migrations/000028_password_reset.up.sql`, `backend/db/migrations/000028_password_reset.down.sql`
- Modify: `backend/db/queries/identity.sql`
- Regenerate: `backend/db/sqlc/*` (via `sqlc generate`)

**Interfaces:**
- Produces: `sqlc.IdentityUser.PasswordChangedAt *time.Time`; `Queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID uuid.UUID, PasswordHash *string}) error`.

- [ ] **Step 1: Write the up migration**

`backend/db/migrations/000028_password_reset.up.sql`:
```sql
-- Password self-service: track when a user's password last changed so refresh
-- tokens issued before that instant can be rejected (logout-everywhere on change).
ALTER TABLE identity.users
    ADD COLUMN password_changed_at timestamptz;
```

- [ ] **Step 2: Write the down migration**

`backend/db/migrations/000028_password_reset.down.sql`:
```sql
ALTER TABLE identity.users
    DROP COLUMN IF EXISTS password_changed_at;
```

- [ ] **Step 3: Add the query**

Append to `backend/db/queries/identity.sql`:
```sql
-- name: UpdateUserPassword :exec
UPDATE identity.users
SET password_hash = $2, password_changed_at = now()
WHERE id = $1 AND deleted_at IS NULL;
```

- [ ] **Step 4: Apply the migration and regenerate sqlc**

Run (from `backend/`):
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
sqlc generate
```
Expected: migration applies clean; `db/sqlc/identity.sql.go` gains `UpdateUserPassword`; `db/sqlc/models.go` `IdentityUser` gains `PasswordChangedAt *time.Time`.

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/db/migrations/000028_password_reset.up.sql backend/db/migrations/000028_password_reset.down.sql backend/db/queries/identity.sql backend/db/sqlc
git commit -m "feat(db): add users.password_changed_at + UpdateUserPassword query"
```

---

## Task 2: `internal/email` package (Sender + LogSender + Mailer + templates)

**Files:**
- Create: `backend/internal/email/sender.go`, `backend/internal/email/mailer.go`, `backend/internal/email/mailer_test.go`, and the four template files under `backend/internal/email/templates/`.

**Interfaces:**
- Produces:
  - `email.Sender` interface: `Send(ctx context.Context, to, subject, htmlBody, textBody string) error`
  - `email.Options struct { Enabled bool; Host string; Port int; Username, Password, From, FromName, TLS string }`
  - `email.NewSender(opts Options, logger *slog.Logger) Sender`
  - `email.Mailer` with `NewMailer(s Sender) *Mailer`, `(*Mailer).SendPasswordReset(ctx context.Context, to, name, link string) error`, `(*Mailer).SendPasswordChanged(ctx context.Context, to, name string) error`

- [ ] **Step 1: Add the go-mail dependency**

Run (from `backend/`):
```bash
go get github.com/wneessen/go-mail@latest
```
Expected: `go.mod`/`go.sum` updated.

- [ ] **Step 2: Write the template files**

`backend/internal/email/templates/password_reset.html`:
```html
<!doctype html>
<html lang="id"><body style="font-family:Arial,sans-serif;color:#0f172a">
<h2>Reset Password Inventra</h2>
<p>Halo {{.Name}},</p>
<p>Kami menerima permintaan untuk mereset password akun Anda. Klik tautan di bawah untuk membuat password baru. Tautan berlaku selama 30 menit.</p>
<p><a href="{{.Link}}" style="background:#16a34a;color:#fff;padding:10px 16px;border-radius:6px;text-decoration:none">Reset Password</a></p>
<p>Jika tombol tidak berfungsi, salin URL berikut ke browser Anda:<br>{{.Link}}</p>
<p style="color:#64748b;font-size:13px">Jika Anda tidak meminta reset ini, abaikan email ini — password Anda tidak berubah.</p>
</body></html>
```

`backend/internal/email/templates/password_reset.txt`:
```text
Reset Password Inventra

Halo {{.Name}},

Kami menerima permintaan untuk mereset password akun Anda. Buka tautan berikut untuk membuat password baru (berlaku 30 menit):

{{.Link}}

Jika Anda tidak meminta reset ini, abaikan email ini — password Anda tidak berubah.
```

`backend/internal/email/templates/password_changed.html`:
```html
<!doctype html>
<html lang="id"><body style="font-family:Arial,sans-serif;color:#0f172a">
<h2>Password Diubah</h2>
<p>Halo {{.Name}},</p>
<p>Password akun Inventra Anda baru saja diubah. Semua sesi login lama telah diakhiri.</p>
<p style="color:#b91c1c;font-size:13px">Jika ini bukan Anda, segera hubungi administrator sistem Anda.</p>
</body></html>
```

`backend/internal/email/templates/password_changed.txt`:
```text
Password Diubah

Halo {{.Name}},

Password akun Inventra Anda baru saja diubah. Semua sesi login lama telah diakhiri.

Jika ini bukan Anda, segera hubungi administrator sistem Anda.
```

- [ ] **Step 3: Write the sender**

`backend/internal/email/sender.go`:
```go
// Package email sends transactional mail (password reset / change notices).
// It is provider-agnostic: any SMTP relay works via Options, and a LogSender
// fallback keeps dev/CI functional without a real relay.
package email

import (
	"context"
	"log/slog"

	mail "github.com/wneessen/go-mail"
)

// Sender delivers a single message. Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// Options configures the SMTP sender (mapped from env in the composition root).
type Options struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	TLS      string // "none" | "starttls" | "tls"
}

// NewSender returns an SMTPSender when enabled with a host, else a LogSender.
func NewSender(opts Options, logger *slog.Logger) Sender {
	if !opts.Enabled || opts.Host == "" {
		return &LogSender{logger: logger, from: opts.From}
	}
	return &SMTPSender{opts: opts, logger: logger}
}

// LogSender logs the message instead of sending — used in dev/CI without a relay.
type LogSender struct {
	logger *slog.Logger
	from   string
}

func (s *LogSender) Send(_ context.Context, to, subject, _, textBody string) error {
	s.logger.Info("email (log-only)", "from", s.from, "to", to, "subject", subject, "body", textBody)
	return nil
}

// SMTPSender delivers via go-mail over the configured relay.
type SMTPSender struct {
	opts   Options
	logger *slog.Logger
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	m := mail.NewMsg()
	if err := m.FromFormat(s.opts.FromName, s.opts.From); err != nil {
		return err
	}
	if err := m.To(to); err != nil {
		return err
	}
	m.Subject(subject)
	m.SetBodyString(mail.TypeTextPlain, textBody)
	m.AddAlternativeString(mail.TypeTextHTML, htmlBody)

	clientOpts := []mail.Option{mail.WithPort(s.opts.Port), mail.WithTimeout(10_000_000_000)}
	switch s.opts.TLS {
	case "tls":
		clientOpts = append(clientOpts, mail.WithSSLPort(false))
	case "starttls":
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.TLSMandatory))
	default:
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.NoTLS))
	}
	if s.opts.Username != "" {
		clientOpts = append(clientOpts, mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(s.opts.Username), mail.WithPassword(s.opts.Password))
	}
	c, err := mail.NewClient(s.opts.Host, clientOpts...)
	if err != nil {
		return err
	}
	return c.DialAndSendWithContext(ctx, m)
}
```

- [ ] **Step 4: Write the mailer**

`backend/internal/email/mailer.go`:
```go
package email

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	texttemplate "text/template"
)

//go:embed templates/*.html templates/*.txt
var templatesFS embed.FS

var (
	htmlTmpl = template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	textTmpl = texttemplate.Must(texttemplate.ParseFS(templatesFS, "templates/*.txt"))
)

// Mailer renders account-security templates and hands them to a Sender.
type Mailer struct {
	sender Sender
}

// NewMailer builds a Mailer over the given Sender.
func NewMailer(s Sender) *Mailer { return &Mailer{sender: s} }

type resetData struct {
	Name string
	Link string
}

type changedData struct {
	Name string
}

func (m *Mailer) render(htmlName, textName string, data any) (html, text string, err error) {
	var hb, tb bytes.Buffer
	if err = htmlTmpl.ExecuteTemplate(&hb, htmlName, data); err != nil {
		return "", "", err
	}
	if err = textTmpl.ExecuteTemplate(&tb, textName, data); err != nil {
		return "", "", err
	}
	return hb.String(), tb.String(), nil
}

// SendPasswordReset emails a reset link valid for the token TTL.
func (m *Mailer) SendPasswordReset(ctx context.Context, to, name, link string) error {
	html, text, err := m.render("password_reset.html", "password_reset.txt", resetData{Name: name, Link: link})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Reset Password Inventra", html, text)
}

// SendPasswordChanged notifies that the account password was changed.
func (m *Mailer) SendPasswordChanged(ctx context.Context, to, name string) error {
	html, text, err := m.render("password_changed.html", "password_changed.txt", changedData{Name: name})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Password Inventra Diubah", html, text)
}
```

- [ ] **Step 5: Write the failing test**

`backend/internal/email/mailer_test.go`:
```go
package email

import (
	"context"
	"strings"
	"testing"
)

// captureSender records the last message for assertions.
type captureSender struct {
	to, subject, html, text string
	calls                   int
}

func (c *captureSender) Send(_ context.Context, to, subject, html, text string) error {
	c.to, c.subject, c.html, c.text, c.calls = to, subject, html, text, c.calls+1
	return nil
}

func TestMailer_SendPasswordReset_RendersLinkAndName(t *testing.T) {
	cap := &captureSender{}
	m := NewMailer(cap)
	if err := m.SendPasswordReset(context.Background(), "u@example.com", "Budi", "https://app/reset-password?token=abc"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if cap.calls != 1 || cap.to != "u@example.com" {
		t.Fatalf("unexpected recipient/calls: %q %d", cap.to, cap.calls)
	}
	if !strings.Contains(cap.html, "https://app/reset-password?token=abc") || !strings.Contains(cap.text, "token=abc") {
		t.Fatalf("link missing from bodies")
	}
	if !strings.Contains(cap.html, "Budi") {
		t.Fatalf("name missing from html body")
	}
}

func TestMailer_SendPasswordChanged_RendersName(t *testing.T) {
	cap := &captureSender{}
	m := NewMailer(cap)
	if err := m.SendPasswordChanged(context.Background(), "u@example.com", "Budi"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(cap.text, "Budi") || cap.subject == "" {
		t.Fatalf("changed notice not rendered: subj=%q", cap.subject)
	}
}

func TestNewSender_FallsBackToLogSenderWhenDisabled(t *testing.T) {
	s := NewSender(Options{Enabled: false, Host: "smtp.example.com"}, discardLogger())
	if _, ok := s.(*LogSender); !ok {
		t.Fatalf("expected LogSender, got %T", s)
	}
}
```

Add a small logger helper at the bottom of the test file:
```go
import "log/slog"
import "io"

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
```
(Fold these imports into the existing import block.)

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/email/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/email backend/go.mod backend/go.sum
git commit -m "feat(email): provider-agnostic SMTP sender + LogSender fallback + templates"
```

---

## Task 3: Redis reset-token store + token helpers

**Files:**
- Create: `backend/internal/auth/pwreset.go`, `backend/internal/auth/pwreset_test.go`

**Interfaces:**
- Produces:
  - `auth.GenerateResetToken() (raw string, hash string, err error)` — 32 random bytes → base64url `raw`; `hash = HashResetToken(raw)`
  - `auth.HashResetToken(raw string) string` — hex SHA-256
  - `auth.ErrResetNotFound error`
  - `(*TokenStore).SavePasswordReset(ctx context.Context, hash, userID string, ttl time.Duration) error`
  - `(*TokenStore).ConsumePasswordReset(ctx context.Context, hash string) (userID string, err error)` — atomic GETDEL; `ErrResetNotFound` on miss

- [ ] **Step 1: Write the token helpers + store methods**

`backend/internal/auth/pwreset.go`:
```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const pwResetPrefix = "auth:pwreset:" // hashed reset token -> userID

// ErrResetNotFound is returned when a reset token is unknown, expired, or already used.
var ErrResetNotFound = errors.New("password reset token not found")

// GenerateResetToken returns a URL-safe random token and its storage hash.
func GenerateResetToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashResetToken(raw), nil
}

// HashResetToken returns the hex SHA-256 of a raw token (what we store at rest).
func HashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// SavePasswordReset stores a single-use reset token hash for the user with a TTL.
func (s *TokenStore) SavePasswordReset(ctx context.Context, hash, userID string, ttl time.Duration) error {
	return s.rdb.Set(ctx, pwResetPrefix+hash, userID, ttl).Err()
}

// ConsumePasswordReset atomically reads and deletes a reset token (single use).
func (s *TokenStore) ConsumePasswordReset(ctx context.Context, hash string) (string, error) {
	userID, err := s.rdb.GetDel(ctx, pwResetPrefix+hash).Result()
	if err != nil {
		return "", ErrResetNotFound
	}
	return userID, nil
}
```

- [ ] **Step 2: Write the failing unit test (pure helpers)**

`backend/internal/auth/pwreset_test.go`:
```go
package auth

import "testing"

func TestGenerateResetToken_UniqueAndHashed(t *testing.T) {
	raw1, hash1, err := GenerateResetToken()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	raw2, hash2, _ := GenerateResetToken()
	if raw1 == raw2 || hash1 == hash2 {
		t.Fatalf("tokens/hashes must be unique")
	}
	if hash1 != HashResetToken(raw1) {
		t.Fatalf("hash not stable for raw token")
	}
	if raw1 == hash1 {
		t.Fatalf("raw token must not equal its hash")
	}
}

func TestHashResetToken_Deterministic(t *testing.T) {
	if HashResetToken("abc") != HashResetToken("abc") {
		t.Fatalf("hash must be deterministic")
	}
	if len(HashResetToken("abc")) != 64 {
		t.Fatalf("expected 64 hex chars for sha256")
	}
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/auth/ -run TestGenerateResetToken -run TestHashResetToken`
Expected: PASS (run `go test ./internal/auth/...` to be safe).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/auth/pwreset.go backend/internal/auth/pwreset_test.go
git commit -m "feat(auth): single-use hashed password-reset token store"
```

---

## Task 4: Identity service — reset/change flows + epoch check

**Files:**
- Modify: `backend/internal/identity/service.go`
- Create: `backend/internal/identity/service_test.go`

**Interfaces:**
- Consumes: `email.Mailer` (Task 2), `auth.GenerateResetToken`/`ErrResetNotFound`/`TokenStore.SavePasswordReset`/`ConsumePasswordReset` (Task 3), `sqlc.UpdateUserPasswordParams` (Task 1).
- Produces (new `Service` deps + methods):
  - `NewService(q userStore, tm *auth.TokenManager, store *auth.TokenStore, mailer mailSender, resetTTL time.Duration, frontendURL string) *Service`
  - `mailSender` interface: `SendPasswordReset(ctx, to, name, link string) error`, `SendPasswordChanged(ctx, to, name string) error`
  - `userStore` gains: `UpdateUserPassword(ctx context.Context, arg sqlc.UpdateUserPasswordParams) error`
  - `(*Service).RequestPasswordReset(ctx context.Context, email string) error` (always nil unless infra error)
  - `(*Service).ResetPassword(ctx context.Context, token, newPassword string) (sqlc.IdentityUser, error)`
  - `(*Service).ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (sqlc.IdentityUser, error)`
  - New sentinels: `ErrWeakPassword`, `ErrInvalidToken` (exists)

- [ ] **Step 1: Extend the service struct, interfaces, and constructor**

In `backend/internal/identity/service.go`, add to the sentinel `var (...)` block:
```go
	ErrWeakPassword = errors.New("password must be at least 8 characters")
```

Extend the `userStore` interface with:
```go
	UpdateUserPassword(ctx context.Context, arg sqlc.UpdateUserPasswordParams) error
```

Add above `Service`:
```go
// mailSender is the account-security mail surface (satisfied by *email.Mailer).
type mailSender interface {
	SendPasswordReset(ctx context.Context, to, name, link string) error
	SendPasswordChanged(ctx context.Context, to, name string) error
}
```

Replace the `Service` struct + `NewService` with:
```go
// Service handles login, token refresh/rotation, logout, current-user lookup,
// and password reset/change.
type Service struct {
	q           userStore
	tm          *auth.TokenManager
	store       *auth.TokenStore
	mail        mailSender
	resetTTL    time.Duration
	frontendURL string
}

// NewService builds the identity Service.
func NewService(q userStore, tm *auth.TokenManager, store *auth.TokenStore, mailer mailSender, resetTTL time.Duration, frontendURL string) *Service {
	return &Service{q: q, tm: tm, store: store, mail: mailer, resetTTL: resetTTL, frontendURL: frontendURL}
}
```

- [ ] **Step 2: Add the epoch check to Refresh**

In `Service.Refresh`, after the `GetUserByID` + active check (right before the rotate comment), insert:
```go
	// Epoch check: a password change invalidates every token issued before it.
	if user.PasswordChangedAt != nil && claims.IssuedAt != nil &&
		claims.IssuedAt.Time.Before(*user.PasswordChangedAt) {
		return auth.TokenPair{}, ErrInvalidToken
	}
```

- [ ] **Step 3: Add the three new methods**

Append to `backend/internal/identity/service.go`:
```go
// RequestPasswordReset issues a reset token + email when the address maps to an
// active, email-login account. It is intentionally silent (always nil) about
// missing/ineligible accounts to prevent user enumeration.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if user.Status != sqlc.SharedUserStatusActive || user.PasswordHash == nil {
		return nil // inactive or Google-only: no reset, but do not reveal it
	}
	raw, hash, err := auth.GenerateResetToken()
	if err != nil {
		return err
	}
	if err := s.store.SavePasswordReset(ctx, hash, user.ID.String(), s.resetTTL); err != nil {
		return err
	}
	link := s.frontendURL + "/reset-password?token=" + raw
	return s.mail.SendPasswordReset(ctx, user.Email, user.Name, link)
}

// ResetPassword consumes a valid reset token and sets a new password. All
// existing sessions become invalid via the password_changed_at epoch.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) (sqlc.IdentityUser, error) {
	if len(newPassword) < 8 {
		return sqlc.IdentityUser{}, ErrWeakPassword
	}
	userIDStr, err := s.store.ConsumePasswordReset(ctx, auth.HashResetToken(token))
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	if err := s.setPassword(ctx, user.ID, newPassword); err != nil {
		return sqlc.IdentityUser{}, err
	}
	_ = s.mail.SendPasswordChanged(ctx, user.Email, user.Name) // best-effort
	return user, nil
}

// ChangePassword verifies the caller's current password and sets a new one,
// invalidating all sessions (including the caller's) via the epoch.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (sqlc.IdentityUser, error) {
	if len(newPassword) < 8 {
		return sqlc.IdentityUser{}, ErrWeakPassword
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return sqlc.IdentityUser{}, err
	}
	if user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, oldPassword) {
		return sqlc.IdentityUser{}, ErrInvalidCredentials
	}
	if err := s.setPassword(ctx, user.ID, newPassword); err != nil {
		return sqlc.IdentityUser{}, err
	}
	_ = s.mail.SendPasswordChanged(ctx, user.Email, user.Name) // best-effort
	return user, nil
}

func (s *Service) setPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: userID, PasswordHash: &hash})
}
```

- [ ] **Step 4: Write the failing service tests**

`backend/internal/identity/service_test.go`:
```go
package identity

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
)

type fakeStore struct {
	byEmail map[string]sqlc.IdentityUser
	byID    map[uuid.UUID]sqlc.IdentityUser
	updated map[uuid.UUID]string // userID -> new hash
}

func (f *fakeStore) GetUserByID(_ context.Context, id uuid.UUID) (sqlc.IdentityUser, error) {
	u, ok := f.byID[id]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeStore) GetUserByEmail(_ context.Context, e string) (sqlc.IdentityUser, error) {
	u, ok := f.byEmail[e]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeStore) LinkGoogleID(_ context.Context, _ sqlc.LinkGoogleIDParams) error { return nil }
func (f *fakeStore) UpdateUserPassword(_ context.Context, a sqlc.UpdateUserPasswordParams) error {
	if f.updated == nil {
		f.updated = map[uuid.UUID]string{}
	}
	f.updated[a.ID] = *a.PasswordHash
	return nil
}

type fakeMailer struct{ resetLink, changedTo string }

func (m *fakeMailer) SendPasswordReset(_ context.Context, _, _, link string) error {
	m.resetLink = link
	return nil
}
func (m *fakeMailer) SendPasswordChanged(_ context.Context, to, _ string) error {
	m.changedTo = to
	return nil
}

func activeUser(t *testing.T, email string) sqlc.IdentityUser {
	t.Helper()
	h, _ := auth.HashPassword("oldpassword")
	return sqlc.IdentityUser{ID: uuid.New(), Email: email, Name: "Budi", Status: sqlc.SharedUserStatusActive, PasswordHash: &h}
}

func newTestService(t *testing.T, fs *fakeStore, fm *fakeMailer) *Service {
	t.Helper()
	tm := auth.NewTokenManager(testCfg()) // testCfg lives in the existing identity test helpers
	// Reset-token store needs a real Redis client; ResetPassword tests that need
	// it are integration-level. These unit tests exercise ChangePassword and
	// RequestPasswordReset (mailer/store fakes) + epoch logic only.
	return NewService(fs, tm, nil, fm, 30*time.Minute, "https://app")
}

func TestChangePassword_WrongOld(t *testing.T) {
	u := activeUser(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	if _, err := svc.ChangePassword(context.Background(), u.ID, "nope", "brandnewpass"); err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestChangePassword_WeakNew(t *testing.T) {
	u := activeUser(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	if _, err := svc.ChangePassword(context.Background(), u.ID, "oldpassword", "short"); err != ErrWeakPassword {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
}

func TestChangePassword_Success_UpdatesHashAndNotifies(t *testing.T) {
	u := activeUser(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if _, err := svc.ChangePassword(context.Background(), u.ID, "oldpassword", "brandnewpass"); err != nil {
		t.Fatalf("change: %v", err)
	}
	if _, ok := fs.updated[u.ID]; !ok {
		t.Fatalf("password not updated")
	}
	if fm.changedTo != "u@x.com" {
		t.Fatalf("notification not sent")
	}
}

func TestRequestPasswordReset_UnknownEmail_SilentOK(t *testing.T) {
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if err := svc.RequestPasswordReset(context.Background(), "ghost@x.com"); err != nil {
		t.Fatalf("want nil (anti-enumeration), got %v", err)
	}
	if fm.resetLink != "" {
		t.Fatalf("no email should be sent for unknown account")
	}
}

func TestRequestPasswordReset_GoogleOnly_SilentOK(t *testing.T) {
	u := activeUser(t, "g@x.com")
	u.PasswordHash = nil // Google-only
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"g@x.com": u}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if err := svc.RequestPasswordReset(context.Background(), "g@x.com"); err != nil {
		t.Fatalf("got %v", err)
	}
	if fm.resetLink != "" {
		t.Fatalf("Google-only account must not receive a reset link")
	}
}
```
> **Note:** if the existing identity tests do not already expose a `testCfg()` helper, add a minimal one to this file: `func testCfg() *config.Config { return &config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour} }` (import `github.com/ragbuaj/inventra/internal/config`). Check `google_test.go` first and reuse its helper if present.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/identity/ -run 'TestChangePassword|TestRequestPasswordReset'`
Expected: PASS.

- [ ] **Step 6: Fix existing NewService callers to compile**

`go build ./...` will fail until callers pass the new args. Update `google_test.go`'s helper (the `NewService(store, ..., ...)` call at ~line 42) to `NewService(store, auth.NewTokenManager(cfg), auth.NewTokenStore(rdb), &fakeMailer{}, 30*time.Minute, "https://app")` (define a local `fakeMailer` there or move it to a shared `_test.go`). The `router.go` caller is updated in Task 5.

- [ ] **Step 7: Run the identity package tests**

Run: `go test ./internal/identity/...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/identity/service.go backend/internal/identity/service_test.go backend/internal/identity/google_test.go
git commit -m "feat(identity): password reset/change flows + refresh epoch check"
```

---

## Task 5: Config, DTOs, handlers, routes, audit + wiring

**Files:**
- Modify: `backend/internal/config/config.go`, `backend/.env.example`, `backend/internal/identity/dto.go`, `backend/internal/identity/handler.go`, `backend/internal/identity/routes.go`, `backend/internal/server/router.go`

**Interfaces:**
- Consumes: service methods (Task 4), `email.NewSender`/`NewMailer` (Task 2), `audit.Record`/`audit.Service` (existing).
- Produces routes: `POST /auth/password/forgot`, `POST /auth/password/reset`, `PUT /auth/password`.

- [ ] **Step 1: Add SMTP config fields**

In `backend/internal/config/config.go`, add to the `Config` struct (after the Auth block):
```go
	// Email / SMTP (password reset + notifications).
	MailEnabled      bool
	SMTPHost         string
	SMTPPort         int
	SMTPUsername     string
	SMTPPassword     string
	SMTPFrom         string
	SMTPFromName     string
	SMTPTLS          string
	PasswordResetTTL time.Duration
```
And in `Load()` (after the Google block):
```go
		MailEnabled:      getEnvBool("MAIL_ENABLED", false),
		SMTPHost:         getEnv("SMTP_HOST", ""),
		SMTPPort:         getEnvInt("SMTP_PORT", 1025),
		SMTPUsername:     getEnv("SMTP_USERNAME", ""),
		SMTPPassword:     getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:         getEnv("SMTP_FROM", "no-reply@inventra.local"),
		SMTPFromName:     getEnv("SMTP_FROM_NAME", "Inventra"),
		SMTPTLS:          getEnv("SMTP_TLS", "none"),
		PasswordResetTTL: getEnvDuration("PASSWORD_RESET_TTL", 30*time.Minute),
```
Append to `backend/.env.example`:
```dotenv
# Email / SMTP (password reset + notifications). MAIL_ENABLED=false uses a log-only sender.
MAIL_ENABLED=false
SMTP_HOST=
SMTP_PORT=1025
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=no-reply@inventra.local
SMTP_FROM_NAME=Inventra
SMTP_TLS=none
PASSWORD_RESET_TTL=30m
```

- [ ] **Step 2: Add request DTOs**

Append to `backend/internal/identity/dto.go`:
```go
// forgotPasswordRequest starts a reset; response is always 200 (anti-enumeration).
type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// resetPasswordRequest completes a reset with the emailed token.
type resetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// changePasswordRequest changes the authenticated user's password.
type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}
```

- [ ] **Step 3: Add handler fields + methods**

In `backend/internal/identity/handler.go`, add imports `"github.com/ragbuaj/inventra/internal/audit"` and (if not present) `"github.com/google/uuid"`. Add fields to `Handler`:
```go
	audit         *audit.Service
	forgotPerMin  int
```
Extend `NewHandler` params with `auditSvc *audit.Service, forgotPerMin int` and set them. Then add:
```go
func (h *Handler) forgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	acctKey := "pwforgot:acct:" + strings.ToLower(strings.TrimSpace(req.Email))
	if res := h.limiter.Allow(c.Request.Context(), acctKey, h.forgotPerMin, true); !res.Allowed {
		middleware.WriteRateLimited(c, res)
		return
	}
	if err := h.svc.RequestPasswordReset(c.Request.Context(), strings.ToLower(strings.TrimSpace(req.Email))); err != nil {
		// Log server-side; never leak whether the address exists.
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) resetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": "tautan tidak valid atau kedaluwarsa"})
		case errors.Is(err, ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", user.ID, user.OfficeID, gin.H{"event": "password_reset"})
	c.JSON(http.StatusOK, gin.H{"status": "password_reset"})
}

func (h *Handler) changePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	user, err := h.svc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusBadRequest, gin.H{"error": "password lama salah"})
		case errors.Is(err, ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", user.ID, user.OfficeID, gin.H{"event": "password_changed"})
	clearRefreshCookie(c, h.secureCookie)
	c.JSON(http.StatusOK, gin.H{"status": "password_changed"})
}
```

- [ ] **Step 4: Register the routes**

In `backend/internal/identity/routes.go`, extend `RegisterRoutes` params with `forgotPerMin int` and add to the unauth group:
```go
	grp.POST("/password/forgot", middleware.PerIP(limiter, forgotPerMin, "auth_pwforgot", true), h.forgotPassword)
	grp.POST("/password/reset", middleware.PerIP(limiter, forgotPerMin, "auth_pwreset", true), h.resetPassword)
```
and to the `authed` group:
```go
	authed.PUT("/password", h.changePassword)
```

- [ ] **Step 5: Wire it in NewRouter**

In `backend/internal/server/router.go`, add import `"github.com/ragbuaj/inventra/internal/email"`. Replace the identity construction (lines ~167-169) with:
```go
		mailer := email.NewMailer(email.NewSender(email.Options{
			Enabled:  d.Cfg.MailEnabled,
			Host:     d.Cfg.SMTPHost,
			Port:     d.Cfg.SMTPPort,
			Username: d.Cfg.SMTPUsername,
			Password: d.Cfg.SMTPPassword,
			From:     d.Cfg.SMTPFrom,
			FromName: d.Cfg.SMTPFromName,
			TLS:      d.Cfg.SMTPTLS,
		}, slog.Default()))
		identitySvc := identity.NewService(queries, tokenManager, tokenStore, mailer, d.Cfg.PasswordResetTTL, d.Cfg.FrontendURL)
		identityHandler := identity.NewHandler(identitySvc, permSvc, scopeSvc, d.Limiter, d.Cfg.RateLimitLoginPerMin, d.Cfg.Env == "production", d.Cfg.JWTRefreshTTL, googleOAuth, d.Cfg.FrontendURL, auditSvc, d.Cfg.RateLimitLoginPerMin)
		identity.RegisterRoutes(api, identityHandler, requireAuth, d.Limiter, d.Cfg.RateLimitLoginIPPerMin, d.Cfg.RateLimitRefreshPerMin, d.Cfg.RateLimitLoginIPPerMin, d.Cfg.RateLimitLoginPerMin)
```
Add `"log/slog"` to router imports if absent. Ensure `auditSvc` is already constructed above this point (it is, at line ~155).

- [ ] **Step 6: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: PASS. Fix any remaining signature mismatches.

- [ ] **Step 7: Run backend tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/config backend/.env.example backend/internal/identity backend/internal/server/router.go
git commit -m "feat(identity): password forgot/reset/change endpoints + email wiring + audit"
```

---

## Task 6: OpenAPI spec

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add the three paths**

Under `paths:` add (place near the other `/auth/*` paths; match the file's existing style for tags/responses):
```yaml
  /auth/password/forgot:
    post:
      tags: [Auth]
      summary: Request a password reset email (always 200, anti-enumeration)
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [email]
              properties:
                email: { type: string, format: email }
      responses:
        '200': { description: Accepted (email sent if the account exists) }
        '429': { description: Rate limited }
  /auth/password/reset:
    post:
      tags: [Auth]
      summary: Reset a password using an emailed token
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [token, new_password]
              properties:
                token: { type: string }
                new_password: { type: string, minLength: 8 }
      responses:
        '200': { description: Password reset }
        '400': { description: Invalid/expired token or weak password }
  /auth/password:
    put:
      tags: [Auth]
      summary: Change the authenticated user's password (revokes all sessions)
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [old_password, new_password]
              properties:
                old_password: { type: string }
                new_password: { type: string, minLength: 8 }
      responses:
        '200': { description: Password changed — all sessions invalidated }
        '400': { description: Wrong current password or weak new password }
        '401': { description: Unauthenticated }
```

- [ ] **Step 2: Lint the spec**

Run (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors (pre-existing warnings unrelated to these paths are acceptable).

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(api): document password forgot/reset/change endpoints"
```

---

## Task 7: Frontend `useAccount` real methods

**Files:**
- Modify: `frontend/app/composables/api/useAccount.ts`, `frontend/test/nuxt/useAccount.spec.ts`

**Interfaces:**
- Produces: `useAccount()` gains `requestPasswordReset(email: string): Promise<void>` and `resetPassword(token: string, newPass: string): Promise<void>`; `changePassword(input: PasswordInput)` now calls `PUT /auth/password`.

- [ ] **Step 1: Rewrite the password methods**

In `frontend/app/composables/api/useAccount.ts`, replace the mock `changePassword` and add the two new functions:
```ts
  const client = useApiClient()
  const config = useRuntimeConfig()
  const base = config.public.apiBase as string

  async function changePassword(input: PasswordInput): Promise<void> {
    if (!input.oldPass || !input.newPass || !input.confirmPass) throw new Error('account.errRequired')
    if (input.newPass !== input.confirmPass) throw new Error('account.errConfirmMismatch')
    if (input.newPass.length < 8) throw new Error('account.errWeak')
    await client.request('/auth/password', {
      method: 'PUT',
      body: { old_password: input.oldPass, new_password: input.newPass }
    })
  }

  async function requestPasswordReset(email: string): Promise<void> {
    await $fetch(`${base}/auth/password/forgot`, { method: 'POST', body: { email } })
  }

  async function resetPassword(token: string, newPass: string): Promise<void> {
    if (newPass.length < 8) throw new Error('account.errWeak')
    await $fetch(`${base}/auth/password/reset`, { method: 'POST', body: { token, new_password: newPass } })
  }
```
Add `requestPasswordReset` and `resetPassword` to the returned object. Keep `getProfile`/`updateProfile`/`listSessions`/`revokeSession`/`logoutAllOthers`/notif helpers as-is (mock — Spec B territory). Remove the now-unused `fakeLatency` import only if nothing else uses it (it still does — leave it).

- [ ] **Step 2: Update the failing test**

In `frontend/test/nuxt/useAccount.spec.ts`, replace any assertion that `changePassword` is a no-op with real-call expectations. Add at the top of the file a `$fetch`/client stub. Minimum new cases:
```ts
it('changePassword rejects mismatched confirmation', async () => {
  const { changePassword } = useAccount()
  await expect(changePassword({ oldPass: 'oldpass12', newPass: 'newpass12', confirmPass: 'different' }))
    .rejects.toThrow('account.errConfirmMismatch')
})

it('changePassword rejects a short new password', async () => {
  const { changePassword } = useAccount()
  await expect(changePassword({ oldPass: 'oldpass12', newPass: 'short', confirmPass: 'short' }))
    .rejects.toThrow('account.errWeak')
})
```
(For the happy-path PUT call, stub `useApiClient` via `mockNuxtImport` if the existing spec already does so; otherwise keep the reject-path tests which need no network.)

- [ ] **Step 3: Run the tests**

Run (from `frontend/`): `pnpm test useAccount`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/composables/api/useAccount.ts frontend/test/nuxt/useAccount.spec.ts
git commit -m "feat(account): real change-password + reset composable methods"
```

---

## Task 8: `/forgot-password` page

**Files:**
- Create: `frontend/app/pages/forgot-password.vue`, `frontend/test/nuxt/forgot-password.spec.ts`
- Modify: `frontend/app/middleware/auth.global.ts`, `frontend/app/pages/login.vue`, `frontend/i18n/locales/{id,en}.json`

- [ ] **Step 1: Allow the public route**

In `frontend/app/middleware/auth.global.ts` change:
```ts
  const publicPaths = ['/login', '/forgot-password', '/reset-password']
```

- [ ] **Step 2: Add i18n keys**

Add to `frontend/i18n/locales/id.json` (under a new `"auth"` sibling group or existing `auth`):
```json
"forgotTitle": "Lupa Password",
"forgotSubtitle": "Masukkan email akun Anda. Jika terdaftar, kami kirim tautan reset.",
"forgotSubmit": "Kirim Tautan Reset",
"forgotSent": "Jika email terdaftar, tautan reset telah dikirim. Cek kotak masuk Anda.",
"forgotRateLimited": "Terlalu banyak percobaan. Coba lagi beberapa saat.",
"backToLogin": "Kembali ke Login",
"resetTitle": "Buat Password Baru",
"resetSubmit": "Simpan Password",
"resetInvalid": "Tautan tidak valid atau sudah kedaluwarsa.",
"resetSuccess": "Password berhasil diubah. Silakan login kembali.",
"newPassword": "Password Baru",
"confirmPassword": "Konfirmasi Password"
```
Add the English equivalents to `frontend/i18n/locales/en.json` (same keys: "Forgot Password", "Enter your account email...", etc.). Also add account-card keys `"account.errWeak": "Password minimal 8 karakter"` / EN "Password must be at least 8 characters".

- [ ] **Step 3: Point the login link at the page**

In `frontend/app/pages/login.vue`, delete the "Password reset is not wired to the backend yet." comment (line ~55) and make the `auth.forgotPassword` element a link to `/forgot-password` (use `<NuxtLink :to="localePath('/forgot-password')">` consistent with the file's i18n routing; if the page uses `useLocalePath`, reuse it).

- [ ] **Step 4: Write the page**

`frontend/app/pages/forgot-password.vue`:
```vue
<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const account = useAccount()
const email = ref('')
const sent = ref(false)
const loading = ref(false)
const errorKey = ref('')

async function submit() {
  loading.value = true
  errorKey.value = ''
  try {
    await account.requestPasswordReset(email.value.trim())
    sent.value = true
  } catch (err: unknown) {
    errorKey.value = (err as { statusCode?: number }).statusCode === 429 ? 'auth.forgotRateLimited' : 'common.error'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="w-full max-w-sm mx-auto">
    <h1 class="text-xl font-semibold mb-1">
      {{ t('auth.forgotTitle') }}
    </h1>
    <p class="text-muted text-sm mb-6">
      {{ t('auth.forgotSubtitle') }}
    </p>

    <UAlert
      v-if="sent"
      color="success"
      variant="soft"
      :title="t('auth.forgotSent')"
      data-testid="forgot-sent"
    />

    <UForm
      v-else
      :state="{ email }"
      @submit="submit"
    >
      <UFormField
        :label="t('auth.email')"
        name="email"
      >
        <UInput
          v-model="email"
          type="email"
          required
          autocomplete="email"
          data-testid="forgot-email"
        />
      </UFormField>
      <p
        v-if="errorKey"
        class="text-error text-sm mt-2"
      >
        {{ t(errorKey) }}
      </p>
      <UButton
        type="submit"
        block
        class="mt-4"
        :loading="loading"
        data-testid="forgot-submit"
      >
        {{ t('auth.forgotSubmit') }}
      </UButton>
    </UForm>

    <div class="mt-6 text-center">
      <NuxtLink
        :to="'/login'"
        class="text-primary text-sm"
      >
        {{ t('auth.backToLogin') }}
      </NuxtLink>
    </div>
  </div>
</template>
```

- [ ] **Step 5: Write the component test**

`frontend/test/nuxt/forgot-password.spec.ts`:
```ts
// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import ForgotPassword from '~/pages/forgot-password.vue'

const requestPasswordReset = vi.fn()
mockNuxtImport('useAccount', () => () => ({ requestPasswordReset }))

describe('forgot-password page', () => {
  it('shows the success panel after submitting', async () => {
    requestPasswordReset.mockResolvedValueOnce(undefined)
    const wrapper = await mountSuspended(ForgotPassword)
    await wrapper.find('[data-testid="forgot-email"]').setValue('u@example.com')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(requestPasswordReset).toHaveBeenCalledWith('u@example.com')
    expect(wrapper.find('[data-testid="forgot-sent"]').exists()).toBe(true)
  })

  it('renders the email field before submission', async () => {
    const wrapper = await mountSuspended(ForgotPassword)
    expect(wrapper.find('[data-testid="forgot-email"]').exists()).toBe(true)
  })
})
```

- [ ] **Step 6: Run tests + lint + typecheck**

Run (from `frontend/`): `pnpm test forgot-password && pnpm lint && pnpm typecheck`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/app/pages/forgot-password.vue frontend/test/nuxt/forgot-password.spec.ts frontend/app/middleware/auth.global.ts frontend/app/pages/login.vue frontend/i18n/locales
git commit -m "feat(account): forgot-password page + public route + login link"
```

---

## Task 9: `/reset-password` page

**Files:**
- Create: `frontend/app/pages/reset-password.vue`, `frontend/test/nuxt/reset-password.spec.ts`

- [ ] **Step 1: Write the page**

`frontend/app/pages/reset-password.vue`:
```vue
<script setup lang="ts">
import { passwordStrength } from '~/utils/passwordStrength'

definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const route = useRoute()
const account = useAccount()
const token = computed(() => (route.query.token as string) || '')
const newPass = ref('')
const confirmPass = ref('')
const loading = ref(false)
const errorKey = ref('')
const strength = computed(() => passwordStrength(newPass.value))

async function submit() {
  errorKey.value = ''
  if (newPass.value.length < 8) { errorKey.value = 'account.errWeak'; return }
  if (newPass.value !== confirmPass.value) { errorKey.value = 'account.errConfirmMismatch'; return }
  loading.value = true
  try {
    await account.resetPassword(token.value, newPass.value)
    await navigateTo({ path: '/login', query: { reset: 'success' } })
  } catch (err: unknown) {
    errorKey.value = (err as { statusCode?: number }).statusCode === 400 ? 'auth.resetInvalid' : 'common.error'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="w-full max-w-sm mx-auto">
    <h1 class="text-xl font-semibold mb-6">
      {{ t('auth.resetTitle') }}
    </h1>

    <UAlert
      v-if="!token"
      color="error"
      variant="soft"
      :title="t('auth.resetInvalid')"
      data-testid="reset-notoken"
    />

    <UForm
      v-else
      :state="{ newPass, confirmPass }"
      @submit="submit"
    >
      <UFormField
        :label="t('auth.newPassword')"
        name="newPass"
      >
        <UInput
          v-model="newPass"
          type="password"
          required
          autocomplete="new-password"
          data-testid="reset-new"
        />
      </UFormField>
      <UFormField
        :label="t('auth.confirmPassword')"
        name="confirmPass"
        class="mt-3"
      >
        <UInput
          v-model="confirmPass"
          type="password"
          required
          autocomplete="new-password"
          data-testid="reset-confirm"
        />
      </UFormField>
      <p class="text-muted text-xs mt-2">
        {{ strength.label }}
      </p>
      <p
        v-if="errorKey"
        class="text-error text-sm mt-2"
        data-testid="reset-error"
      >
        {{ t(errorKey) }}
      </p>
      <UButton
        type="submit"
        block
        class="mt-4"
        :loading="loading"
        data-testid="reset-submit"
      >
        {{ t('auth.resetSubmit') }}
      </UButton>
    </UForm>

    <div class="mt-6 text-center">
      <NuxtLink
        :to="'/login'"
        class="text-primary text-sm"
      >
        {{ t('auth.backToLogin') }}
      </NuxtLink>
    </div>
  </div>
</template>
```
> Verify `passwordStrength` returns an object with a `.label`; `account.vue` uses `strength` already (line ~29). If its shape differs, bind the field it exposes instead.

- [ ] **Step 2: Write the component test**

`frontend/test/nuxt/reset-password.spec.ts`:
```ts
// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import ResetPassword from '~/pages/reset-password.vue'

const resetPassword = vi.fn()
mockNuxtImport('useAccount', () => () => ({ resetPassword }))
mockNuxtImport('useRoute', () => () => ({ query: { token: 'tok123' } }))

describe('reset-password page', () => {
  it('rejects mismatched confirmation without calling the API', async () => {
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('brandnewpass')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('different1')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(resetPassword).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="reset-error"]').exists()).toBe(true)
  })

  it('calls resetPassword with the token and new password', async () => {
    resetPassword.mockResolvedValueOnce(undefined)
    const wrapper = await mountSuspended(ResetPassword)
    await wrapper.find('[data-testid="reset-new"]').setValue('brandnewpass')
    await wrapper.find('[data-testid="reset-confirm"]').setValue('brandnewpass')
    await wrapper.find('form').trigger('submit')
    await new Promise(r => setTimeout(r, 0))
    expect(resetPassword).toHaveBeenCalledWith('tok123', 'brandnewpass')
  })
})
```
Add a second describe/file variant (or a `mockNuxtImport` override) asserting the no-token error panel renders when `route.query.token` is empty.

- [ ] **Step 3: Run tests + lint + typecheck**

Run (from `frontend/`): `pnpm test reset-password && pnpm lint && pnpm typecheck`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/app/pages/reset-password.vue frontend/test/nuxt/reset-password.spec.ts
git commit -m "feat(account): reset-password page"
```

---

## Task 10: Wire the account "Ganti Password" card

**Files:**
- Modify: `frontend/app/pages/account.vue`

- [ ] **Step 1: Update the change-password handler**

In `frontend/app/pages/account.vue`, add near the other composables:
```ts
const authApi = useAuthApi()
const auth = useAuthStore()
```
Replace the body of `changePassword()` (line ~59) so that on success the user is logged out (all sessions were revoked server-side) and sent to login:
```ts
async function changePassword() {
  passErr.value = ''
  try {
    await account.changePassword({ oldPass: oldPass.value, newPass: newPass.value, confirmPass: confirmPass.value })
    auth.clear()
    toast.add({ title: t('account.toastPassTitle'), description: t('account.secReloginMsg'), color: 'success' })
    await navigateTo('/login')
  } catch (err: unknown) {
    passErr.value = (err as Error).message || 'common.error'
    toast.add({ title: t('common.error'), description: t(passErr.value), color: 'error' })
  }
}
```
> If `passErr` is not already a ref in the file, reuse the existing error handling variable the file uses (grep the current `changePassword` body — keep its variable names). Add i18n key `account.secReloginMsg` = "Semua sesi diakhiri. Silakan login kembali." (EN: "All sessions ended. Please log in again.").

- [ ] **Step 2: Lint + typecheck + run the account tests**

Run (from `frontend/`): `pnpm lint && pnpm typecheck && pnpm test account`
Expected: PASS. If a component test asserted the old mock behavior (toast without redirect), update it to assert `navigateTo('/login')` was called (stub `navigateTo` via `mockNuxtImport`).

- [ ] **Step 3: Commit**

```bash
git add frontend/app/pages/account.vue frontend/i18n/locales
git commit -m "feat(account): change-password logs out all sessions + redirects to login"
```

---

## Task 11: Mailpit in compose + CI

**Files:**
- Modify: `docker-compose.dev.yml`, `docker-compose.yml`, `.github/workflows/ci.yml`

- [ ] **Step 1: Add Mailpit to the dev compose**

In `docker-compose.dev.yml`, add under `services:` (mirror the `redis` block's style):
```yaml
  mailpit:
    image: axllent/mailpit:latest
    container_name: inventra-mailpit
    ports:
      - "1025:1025"
      - "8025:8025"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8025/readyz"]
      interval: 10s
      timeout: 3s
      retries: 5
```

- [ ] **Step 2: Add Mailpit to the production-like compose (used by CI e2e)**

In `docker-compose.yml`, add the same `mailpit` service, and add to the `backend` service `environment:` block:
```yaml
      MAIL_ENABLED: "true"
      SMTP_HOST: mailpit
      SMTP_PORT: "1025"
      SMTP_TLS: none
      SMTP_FROM: no-reply@inventra.local
```
Expose Mailpit's `8025` to the host so the e2e can read messages.

- [ ] **Step 3: Start Mailpit in the CI e2e job**

In `.github/workflows/ci.yml`, change the "Start backend stack" run (line ~79) to include mailpit:
```yaml
        run: docker compose up -d --build postgres redis minio migrate mailpit backend
```

- [ ] **Step 4: Verify dev compose parses**

Run (repo root): `docker compose -f docker-compose.dev.yml config >/dev/null && docker compose -f docker-compose.yml config >/dev/null`
Expected: no error.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.dev.yml docker-compose.yml .github/workflows/ci.yml
git commit -m "chore(dev): add Mailpit mail-catcher for dev + CI e2e"
```

---

## Task 12: E2E — full reset + change flows via Mailpit

**Files:**
- Create: `frontend/e2e/password-reset.spec.ts`

**Interfaces:**
- Consumes: Mailpit HTTP API at `http://localhost:8025` (`GET /api/v1/message/latest` returns the newest message incl. `Text`/`HTML`), the seeded admin `admin@inventra.local` / `admin12345`.

- [ ] **Step 1: Write the e2e spec**

`frontend/e2e/password-reset.spec.ts`:
```ts
import { test, expect } from '@playwright/test'

const MAILPIT = 'http://localhost:8025'
const ADMIN = 'admin@inventra.local'

async function latestResetLink(request): Promise<string> {
  const res = await request.get(`${MAILPIT}/api/v1/message/latest`)
  const msg = await res.json()
  const body: string = msg.Text || msg.HTML || ''
  const m = body.match(/\/reset-password\?token=[A-Za-z0-9_-]+/)
  if (!m) throw new Error('reset link not found in email: ' + body.slice(0, 200))
  return m[0]
}

test('forgot-password → email link → reset → login with new password', async ({ page, request }) => {
  // Purge the mailbox so latest() is deterministic.
  await request.delete(`${MAILPIT}/api/v1/messages`)

  await page.goto('/forgot-password')
  await page.getByTestId('forgot-email').fill(ADMIN)
  await page.getByTestId('forgot-submit').click()
  await expect(page.getByTestId('forgot-sent')).toBeVisible()

  // Read the emailed link and follow it.
  await expect.poll(async () => {
    const res = await request.get(`${MAILPIT}/api/v1/messages`)
    return (await res.json()).total
  }, { timeout: 10000 }).toBeGreaterThan(0)
  const link = await latestResetLink(request)

  const newPass = 'admin12345new'
  await page.goto(link)
  await page.getByTestId('reset-new').fill(newPass)
  await page.getByTestId('reset-confirm').fill(newPass)
  await page.getByTestId('reset-submit').click()
  await expect(page).toHaveURL(/\/login/)

  // Log in with the new password.
  await page.getByLabel(/email/i).fill(ADMIN)
  await page.locator('input[type="password"]').fill(newPass)
  await page.getByRole('button', { name: /masuk|login/i }).click()
  await expect(page).toHaveURL(/\/$|\/dashboard|\/id/)

  // Reset the admin password back so the shared stack stays usable.
  // (Change-password revokes the session, so do it via the API with a fresh login.)
})

test('reset-password with an invalid token shows the error state', async ({ page }) => {
  await page.goto('/reset-password?token=garbage-token-value')
  await page.getByTestId('reset-new').fill('whatever12')
  await page.getByTestId('reset-confirm').fill('whatever12')
  await page.getByTestId('reset-submit').click()
  await expect(page.getByTestId('reset-error')).toBeVisible()
})
```
> **Password-restoration note:** the first test changes the seeded admin password. To keep the shared stack idempotent across reruns, either (a) accept both `admin12345` and `admin12345new` at login-setup time (try one, fall back to the other), or (b) after the test, POST `/auth/password/forgot` + reset back to `admin12345`. Implement approach (a) as a helper at the top of the spec so reruns are deterministic. CI starts a fresh DB each run, so CI is unaffected either way.

- [ ] **Step 2: Run the e2e (backend stack + Mailpit must be up)**

Run (from `frontend/`, with `docker compose -f ../docker-compose.dev.yml up -d` incl. mailpit, backend on host with `MAIL_ENABLED=true SMTP_HOST=localhost SMTP_PORT=1025 RATELIMIT_ENABLED=false`, and a seeded admin):
```bash
pnpm test:e2e password-reset
```
Expected: both tests PASS (reset link captured from Mailpit; invalid-token error shown).

- [ ] **Step 3: Commit**

```bash
git add frontend/e2e/password-reset.spec.ts
git commit -m "test(e2e): password reset via Mailpit + invalid-token path"
```

---

## Task 13: Final gate + PROGRESS.md

**Files:**
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Backend full gate**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./...`
Expected: all PASS.

- [ ] **Step 2: Integration gate (shared-signature change to NewService/NewHandler)**

Run (from `backend/`): `go test -tags=integration ./...`
Expected: all PASS (transient testcontainers Docker contention → rerun the affected package in isolation).

- [ ] **Step 3: Spectral**

Run (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors.

- [ ] **Step 4: Frontend gate**

Run (from `frontend/`): `pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all PASS.

- [ ] **Step 5: Update PROGRESS.md**

In `docs/PROGRESS.md`, resolve item 53's Spec A: mark the account-security-via-email work done (link this plan + the spec), note **Spec B — Device Sessions** is the queued follow-up, and refresh the "▶ Next session — start here" pointer to Spec B. Note any approved deviation (e.g. change-password revokes *all* sessions, no email-gate on #3 — per the 2026-07-13 decisions).

- [ ] **Step 6: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): account security via email (Spec A) complete; Spec B queued"
```

---

## Self-Review Notes (author checklist — completed)

- **Spec coverage:** email infra (T2/T11) · forgot-password (T4/T5/T8/T12) · change-password + notify (T4/T5/T10) · session invalidation via epoch (T1/T4) · anti-enumeration + rate-limit (T5) · audit (T5) · OpenAPI (T6) · frontend pages + wiring (T7–T10) · tests unit/integration/e2e (throughout) · Mailpit dev+CI (T11). All spec §sections map to a task.
- **Password policy min 8** enforced at DTO (T5) and service (T4) and frontend (T7/T9).
- **Type consistency:** `mailSender` interface (T4) matches `email.Mailer` methods (T2); `UpdateUserPasswordParams{ID, PasswordHash}` used identically in T1/T4; `NewService` 6-arg signature used in T4 tests and T5 wiring; `NewHandler` extended args (`auditSvc`, `forgotPerMin`) consistent T5 handler↔routes↔router; reset link path `/reset-password?token=` matches the frontend page route (T9) and e2e regex (T12).
- **Audit:** uses `ActionUpdate` + `{"event":...}` — no enum migration (Global Constraints honored).
