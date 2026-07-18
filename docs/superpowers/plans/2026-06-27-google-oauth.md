# Google OAuth Sign-in (link-only) Implementation Plan — C2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Google OIDC sign-in that links a verified Google email to an EXISTING user (link-only, no auto-provision) and mints the same JWT pair, with the refresh token delivered via the C1 HttpOnly cookie.

**Architecture:** A new `internal/oauth` package wraps go-oidc + oauth2 (state/PKCE in Redis) behind `Verifier`/`Exchanger`/`kv` interfaces so the link-only logic and handlers are unit-testable without a real Google round-trip. `identity.Service` gains a `userStore` interface seam and a `LoginWithGoogle` (link-only) method. Two new GET endpoints (`/auth/google`, `/auth/google/callback`) redirect to Google and back; the callback sets the C1 refresh cookie and redirects to the SPA, which calls `/auth/refresh`. The feature is gated on `GOOGLE_CLIENT_ID` so dev/CI without credentials still boot.

**Tech Stack:** Go 1.25, `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3`, Gin, `go-redis/v9`, sqlc, Nuxt 4 / Vitest.

## Global Constraints

- **Link-only** (supersedes ADR-0009 auto-create): unknown verified email → reject (`ErrNotProvisioned`); never create a user. Existing user: link `google_id` if nil; if set and ≠ the Google `sub` → `ErrGoogleMismatch`; inactive → `ErrUserInactive`.
- **Reject `email_verified == false`.** **PKCE (S256) + state** mandatory; state stored server-side in Redis, single-use (GetDel), short TTL (5 min).
- **Token handoff via the C1 cookie**: callback calls `setRefreshCookie` (existing in `internal/identity`) and redirects to `FRONTEND_URL + /login?oauth=success`; errors redirect to `/login?oauth=error&reason=<short-code>`. **Anti open-redirect**: `Location` only ever points at `FRONTEND_URL` (config); `reason` is one of a fixed set `{not_registered, account_mismatch, inactive, disabled, server}`.
- **Feature-gate**: empty `GOOGLE_CLIENT_ID` (or OIDC discovery failure) → the Service is disabled; endpoints redirect `reason=disabled`; the app still boots (no `Fatal`).
- **Never log** tokens/secrets/the full id_token; redaction (ADR-0002) + don't put them in log attrs.
- Rate-limit `/auth/google` + callback per-IP (reuse `middleware.PerIP`).
- No DB migration (the `google_id` column + unique index exist since migration 000003) — only a new `LinkGoogleID` query.
- New deps: `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3` (`go get` + `go mod tidy`).
- Backend gate `go build/vet/test ./...` + Spectral; frontend `pnpm lint/typecheck/test/build`. Real Google round-trip is **manual** (needs credentials) — NOT a CI test; the existing password-login e2e must stay green. Conventional Commits with scope, no AI/co-author trailers. Branch `feat/google-oauth` (already checked out).

---

### Task 1: Config (`GoogleIssuer`) + deps + `LinkGoogleID` query

**Files:**
- Modify: `backend/internal/config/config.go` (+ `GoogleIssuer`), `backend/.env.example`
- Modify: `backend/db/queries/identity.sql` (+ `LinkGoogleID`), then `sqlc generate`
- Test: `backend/internal/config/config_test.go` (append)

**Interfaces:**
- Produces: `config.Config.GoogleIssuer string`; sqlc method `Queries.LinkGoogleID(ctx, LinkGoogleIDParams{ID uuid.UUID, GoogleID *string}) error`.

- [ ] **Step 1: Add deps**

Run: `cd backend && go get golang.org/x/oauth2 github.com/coreos/go-oidc/v3 && go mod tidy`
Expected: go.mod/go.sum updated.

- [ ] **Step 2: Write the failing config test**

Append to `backend/internal/config/config_test.go`:

```go
func TestLoadGoogleIssuerDefault(t *testing.T) {
	t.Setenv("GOOGLE_ISSUER", "")
	if got := Load().GoogleIssuer; got != "https://accounts.google.com" {
		t.Fatalf("GoogleIssuer default: %q", got)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd backend && go test ./internal/config/ -run GoogleIssuer`
Expected: FAIL — no `GoogleIssuer` field.

- [ ] **Step 4: Add the config field**

In `backend/internal/config/config.go`, add to the `Config` struct (near the other Google fields):

```go
	GoogleIssuer string
```
and in `Load()` (next to the other Google assignments):

```go
		GoogleIssuer: getEnv("GOOGLE_ISSUER", "https://accounts.google.com"),
```

- [ ] **Step 5: Add the `LinkGoogleID` query + generate**

Append to `backend/db/queries/identity.sql`:

```sql
-- name: LinkGoogleID :exec
UPDATE identity.users
SET google_id = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;
```

Run: `cd backend && sqlc generate`
Expected: `db/sqlc/identity.sql.go` now has `LinkGoogleID` + `LinkGoogleIDParams`. Do NOT hand-edit generated files.

- [ ] **Step 6: Env example**

Append to `backend/.env.example`:

```
# Google OAuth (ADR-0009). Leave CLIENT_ID empty to disable Google sign-in.
GOOGLE_ISSUER=https://accounts.google.com
```
(`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL` already exist in `.env.example` — leave them.)

- [ ] **Step 7: Run tests + build + commit**

Run: `cd backend && go test ./internal/config/ && go build ./... && go vet ./...`
Expected: pass, clean.

```bash
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/.env.example backend/db/queries/identity.sql backend/db/sqlc/ backend/go.mod backend/go.sum
git commit -m "feat(identity): GoogleIssuer config, oauth deps, LinkGoogleID query"
```

---

### Task 2: OAuth state store (Redis, single-use) + random helpers

**Files:**
- Create: `backend/internal/oauth/state.go`
- Test: `backend/internal/oauth/state_test.go`

**Interfaces:**
- Produces: `kv` interface (`Set(ctx, key, val string, ttl time.Duration) error`, `GetDel(ctx, key string) (string, error)`); `redisKV` (wraps `*redis.Client`); `stateStore{ kv kv; ttl time.Duration }` with `Save(ctx, state, verifier string) error` + `Consume(ctx, state string) (verifier string, err error)`; `ErrStateInvalid`; `randToken(nBytes int) (string, error)`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/oauth/state_test.go`:

```go
package oauth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeKV struct {
	m   map[string]string
	err error
}

func newFakeKV() *fakeKV { return &fakeKV{m: map[string]string{}} }

func (f *fakeKV) Set(_ context.Context, key, val string, _ time.Duration) error {
	if f.err != nil {
		return f.err
	}
	f.m[key] = val
	return nil
}

func (f *fakeKV) GetDel(_ context.Context, key string) (string, error) {
	v, ok := f.m[key]
	if !ok {
		return "", errMissing
	}
	delete(f.m, key)
	return v, nil
}

func TestStateStoreSaveConsumeSingleUse(t *testing.T) {
	kv := newFakeKV()
	s := &stateStore{kv: kv, ttl: time.Minute}
	if err := s.Save(context.Background(), "st", "verifier-1"); err != nil {
		t.Fatalf("save: %v", err)
	}
	v, err := s.Consume(context.Background(), "st")
	if err != nil || v != "verifier-1" {
		t.Fatalf("consume: %q %v", v, err)
	}
	// single-use: second consume fails
	if _, err := s.Consume(context.Background(), "st"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("second consume should be ErrStateInvalid, got %v", err)
	}
}

func TestStateStoreUnknownState(t *testing.T) {
	s := &stateStore{kv: newFakeKV(), ttl: time.Minute}
	if _, err := s.Consume(context.Background(), "nope"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("unknown state should be ErrStateInvalid, got %v", err)
	}
}

func TestRandTokenUniqueURLSafe(t *testing.T) {
	a, err := randToken(32)
	if err != nil {
		t.Fatalf("randToken: %v", err)
	}
	b, _ := randToken(32)
	if a == b || a == "" {
		t.Fatalf("tokens should be non-empty and unique: %q %q", a, b)
	}
	for _, c := range a {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Fatalf("token not URL-safe: %q", a)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/oauth/`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Implement**

Create `backend/internal/oauth/state.go`:

```go
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrStateInvalid means the OAuth state was unknown, expired, or already used.
var ErrStateInvalid = errors.New("invalid or expired oauth state")

// errMissing is the sentinel a kv returns when a key is absent.
var errMissing = errors.New("kv: missing")

const statePrefix = "oauth:state:"

// kv is the minimal Redis surface the state store needs (seam for tests).
type kv interface {
	Set(ctx context.Context, key, val string, ttl time.Duration) error
	GetDel(ctx context.Context, key string) (string, error)
}

// redisKV adapts *redis.Client to kv. GetDel atomically reads and deletes.
type redisKV struct{ rdb *redis.Client }

func (r redisKV) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	return r.rdb.Set(ctx, key, val, ttl).Err()
}

func (r redisKV) GetDel(ctx context.Context, key string) (string, error) {
	v, err := r.rdb.GetDel(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", errMissing
	}
	return v, err
}

// stateStore persists state -> PKCE verifier for single use.
type stateStore struct {
	kv  kv
	ttl time.Duration
}

func (s *stateStore) Save(ctx context.Context, state, verifier string) error {
	return s.kv.Set(ctx, statePrefix+state, verifier, s.ttl)
}

// Consume returns the PKCE verifier for state and deletes it (single use).
func (s *stateStore) Consume(ctx context.Context, state string) (string, error) {
	v, err := s.kv.GetDel(ctx, statePrefix+state)
	if err != nil {
		return "", ErrStateInvalid
	}
	return v, nil
}

// randToken returns a URL-safe random string from nBytes of entropy.
func randToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/oauth/`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/oauth/state.go backend/internal/oauth/state_test.go
git commit -m "feat(oauth): single-use Redis state store + URL-safe token helper"
```

---

### Task 3: OAuth Service (verifier/exchanger seams, AuthCodeURL, Exchange, feature-gate)

**Files:**
- Create: `backend/internal/oauth/oauth.go`
- Test: `backend/internal/oauth/oauth_test.go`

**Interfaces:**
- Consumes: `stateStore`, `redisKV`, `randToken`, `ErrStateInvalid` (Task 2).
- Produces: `Verifier`/`Exchanger` interfaces; errors `ErrDisabled`, `ErrEmailNotVerified`; `Config` struct; `New(ctx, cfg Config, rdb *redis.Client) (*Service, error)`; `(*Service).Enabled() bool`; `(*Service).AuthCodeURL(ctx) (url, state string, err error)`; `(*Service).Exchange(ctx, code, state string) (email, sub string, err error)`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/oauth/oauth_test.go`:

```go
package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type fakeExch struct {
	raw string
	err error
}

func (f fakeExch) Exchange(_ context.Context, _, _ string) (string, error) { return f.raw, f.err }

type fakeVer struct {
	email    string
	verified bool
	sub      string
	err      error
}

func (f fakeVer) Verify(_ context.Context, _ string) (string, bool, string, error) {
	return f.email, f.verified, f.sub, f.err
}

func testService(exch Exchanger, ver Verifier, kv *fakeKV) *Service {
	return &Service{
		enabled:   true,
		oauthCfg:  &oauth2.Config{ClientID: "cid", RedirectURL: "http://localhost:8080/cb"},
		exch:      exch,
		verifier:  ver,
		state:     &stateStore{kv: kv, ttl: time.Minute},
	}
}

func TestAuthCodeURLStoresStateAndIncludesIt(t *testing.T) {
	kv := newFakeKV()
	s := testService(fakeExch{}, fakeVer{}, kv)
	url, state, err := s.AuthCodeURL(context.Background())
	if err != nil || state == "" {
		t.Fatalf("authcodeurl: %q %v", state, err)
	}
	if !strings.Contains(url, "state="+state) {
		t.Fatalf("url missing state: %s", url)
	}
	if !strings.Contains(url, "code_challenge=") {
		t.Fatalf("url missing PKCE challenge: %s", url)
	}
	// state was stored (consumable once)
	if _, err := s.state.Consume(context.Background(), state); err != nil {
		t.Fatalf("state not stored: %v", err)
	}
}

func TestExchangeSuccess(t *testing.T) {
	kv := newFakeKV()
	s := testService(fakeExch{raw: "rawtoken"}, fakeVer{email: "a@b.com", verified: true, sub: "google-sub-1"}, kv)
	_ = s.state.Save(context.Background(), "st", "pkce")
	email, sub, err := s.Exchange(context.Background(), "code", "st")
	if err != nil || email != "a@b.com" || sub != "google-sub-1" {
		t.Fatalf("exchange: %q %q %v", email, sub, err)
	}
}

func TestExchangeRejectsUnverifiedEmail(t *testing.T) {
	kv := newFakeKV()
	s := testService(fakeExch{raw: "t"}, fakeVer{email: "a@b.com", verified: false, sub: "x"}, kv)
	_ = s.state.Save(context.Background(), "st", "pkce")
	if _, _, err := s.Exchange(context.Background(), "code", "st"); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
}

func TestExchangeRejectsBadState(t *testing.T) {
	s := testService(fakeExch{raw: "t"}, fakeVer{verified: true}, newFakeKV())
	if _, _, err := s.Exchange(context.Background(), "code", "missing"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("expected ErrStateInvalid, got %v", err)
	}
}

func TestDisabledServiceRejects(t *testing.T) {
	s := &Service{enabled: false}
	if _, _, err := s.AuthCodeURL(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled AuthCodeURL: %v", err)
	}
	if _, _, err := s.Exchange(context.Background(), "c", "s"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled Exchange: %v", err)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/oauth/ -run "AuthCodeURL|Exchange|Disabled"`
Expected: FAIL — `Service`/`Exchanger`/`Verifier`/errors undefined.

- [ ] **Step 3: Implement**

Create `backend/internal/oauth/oauth.go`:

```go
// Package oauth implements Google OIDC sign-in (authorization-code + PKCE).
// Network operations sit behind Exchanger/Verifier interfaces so the flow is
// unit-testable without a real Google round-trip (ADR-0009).
package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

// Errors.
var (
	ErrDisabled         = errors.New("google sign-in is not configured")
	ErrEmailNotVerified = errors.New("google email is not verified")
)

// Verifier verifies a raw OIDC ID token and returns the claims we use.
type Verifier interface {
	Verify(ctx context.Context, rawIDToken string) (email string, emailVerified bool, sub string, err error)
}

// Exchanger swaps an auth code (with its PKCE verifier) for the raw id_token.
type Exchanger interface {
	Exchange(ctx context.Context, code, codeVerifier string) (rawIDToken string, err error)
}

// Config holds the provider/client settings.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Issuer       string
}

// Service drives the sign-in flow. When disabled (no client id / discovery
// failure), AuthCodeURL/Exchange return ErrDisabled.
type Service struct {
	enabled  bool
	oauthCfg *oauth2.Config
	exch     Exchanger
	verifier Verifier
	state    *stateStore
}

// New builds the Service. Empty ClientID → disabled (no discovery). A discovery
// error is returned to the caller, which should log it and run disabled.
func New(ctx context.Context, cfg Config, rdb *redis.Client) (*Service, error) {
	if cfg.ClientID == "" {
		return &Service{enabled: false}, nil
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return &Service{enabled: false}, err
	}
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	return &Service{
		enabled:  true,
		oauthCfg: oauthCfg,
		exch:     googleExchanger{cfg: oauthCfg},
		verifier: googleVerifier{v: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})},
		state:    &stateStore{kv: redisKV{rdb: rdb}, ttl: 5 * time.Minute},
	}, nil
}

// Enabled reports whether Google sign-in is configured.
func (s *Service) Enabled() bool { return s.enabled }

// AuthCodeURL generates a state + PKCE verifier, stores them, and returns the
// Google consent URL.
func (s *Service) AuthCodeURL(ctx context.Context) (string, string, error) {
	if !s.enabled {
		return "", "", ErrDisabled
	}
	state, err := randToken(32)
	if err != nil {
		return "", "", err
	}
	pkce := oauth2.GenerateVerifier()
	if err := s.state.Save(ctx, state, pkce); err != nil {
		return "", "", err
	}
	url := s.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(pkce))
	return url, state, nil
}

// Exchange validates the single-use state, exchanges the code, verifies the ID
// token, and returns the verified email + subject.
func (s *Service) Exchange(ctx context.Context, code, state string) (string, string, error) {
	if !s.enabled {
		return "", "", ErrDisabled
	}
	pkce, err := s.state.Consume(ctx, state)
	if err != nil {
		return "", "", err // ErrStateInvalid
	}
	raw, err := s.exch.Exchange(ctx, code, pkce)
	if err != nil {
		return "", "", err
	}
	email, verified, sub, err := s.verifier.Verify(ctx, raw)
	if err != nil {
		return "", "", err
	}
	if !verified {
		return "", "", ErrEmailNotVerified
	}
	return email, sub, nil
}

// googleExchanger wraps oauth2.Config.Exchange and extracts the raw id_token.
type googleExchanger struct{ cfg *oauth2.Config }

func (g googleExchanger) Exchange(ctx context.Context, code, codeVerifier string) (string, error) {
	tok, err := g.cfg.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return "", err
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return "", errors.New("oauth: no id_token in token response")
	}
	return raw, nil
}

// googleVerifier wraps a go-oidc verifier and pulls the claims we need.
type googleVerifier struct{ v *oidc.IDTokenVerifier }

func (g googleVerifier) Verify(ctx context.Context, raw string) (string, bool, string, error) {
	idt, err := g.v.Verify(ctx, raw)
	if err != nil {
		return "", false, "", err
	}
	var c struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idt.Claims(&c); err != nil {
		return "", false, "", err
	}
	return c.Email, c.EmailVerified, idt.Subject, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/oauth/ && go build ./... && go vet ./...`
Expected: PASS (all oauth tests).

> If `oauth2.GenerateVerifier`/`S256ChallengeOption`/`VerifierOption` are missing, the `go get` in Task 1 pulled an older `x/oauth2`; run `go get golang.org/x/oauth2@latest && go mod tidy` (these PKCE helpers exist since v0.10). Keep the test assertions unchanged.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/oauth/oauth.go backend/internal/oauth/oauth_test.go backend/go.mod backend/go.sum
git commit -m "feat(oauth): OIDC Service (PKCE + state) with feature-gate and test seams"
```

---

### Task 4: `identity.LoginWithGoogle` + `userStore` seam

**Files:**
- Modify: `backend/internal/identity/service.go` (`userStore` interface; `Service.q` type; `LoginWithGoogle` + errors)
- Test: `backend/internal/identity/google_test.go`

**Interfaces:**
- Consumes: sqlc `LinkGoogleID`/`LinkGoogleIDParams` (Task 1); `auth.TokenPair`.
- Produces: `identity.userStore` interface; `Service.LoginWithGoogle(ctx, email, googleSub string) (auth.TokenPair, sqlc.IdentityUser, error)`; `ErrNotProvisioned`, `ErrGoogleMismatch`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/identity/google_test.go`:

```go
package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
)

// fakeUserStore implements the identity userStore seam.
type fakeUserStore struct {
	user      sqlc.IdentityUser
	getErr    error
	linked    *string
	linkErr   error
}

func (f *fakeUserStore) GetUserByID(_ context.Context, _ uuid.UUID) (sqlc.IdentityUser, error) {
	return f.user, f.getErr
}
func (f *fakeUserStore) GetUserByEmail(_ context.Context, _ string) (sqlc.IdentityUser, error) {
	return f.user, f.getErr
}
func (f *fakeUserStore) LinkGoogleID(_ context.Context, p sqlc.LinkGoogleIDParams) error {
	f.linked = p.GoogleID
	return f.linkErr
}

func newGoogleSvc(store userStore) *Service {
	cfg := &config.Config{JWTSecret: "test-secret-please-change", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	// Unreachable Redis: issue()'s SaveRefresh returns an error fast (never panics),
	// so error-path tests run cleanly and the link side-effect is still observable.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	return NewService(store, auth.NewTokenManager(cfg), auth.NewTokenStore(rdb))
}

func activeUser() sqlc.IdentityUser {
	return sqlc.IdentityUser{ID: uuid.New(), Email: "a@b.com", RoleID: uuid.New(), Status: sqlc.SharedUserStatusActive}
}

func TestLoginWithGoogleNotProvisioned(t *testing.T) {
	svc := newGoogleSvc(&fakeUserStore{getErr: pgx.ErrNoRows})
	if _, _, err := svc.LoginWithGoogle(context.Background(), "x@y.com", "sub"); !errors.Is(err, ErrNotProvisioned) {
		t.Fatalf("expected ErrNotProvisioned, got %v", err)
	}
}

func TestLoginWithGoogleMismatch(t *testing.T) {
	u := activeUser()
	other := "another-sub"
	u.GoogleID = &other
	if _, _, err := newGoogleSvc(&fakeUserStore{user: u}).LoginWithGoogle(context.Background(), "a@b.com", "sub"); !errors.Is(err, ErrGoogleMismatch) {
		t.Fatalf("expected ErrGoogleMismatch, got %v", err)
	}
}

func TestLoginWithGoogleInactive(t *testing.T) {
	u := activeUser()
	u.Status = sqlc.SharedUserStatusInactive
	if _, _, err := newGoogleSvc(&fakeUserStore{user: u}).LoginWithGoogle(context.Background(), "a@b.com", "sub"); !errors.Is(err, ErrUserInactive) {
		t.Fatalf("expected ErrUserInactive, got %v", err)
	}
}

func TestLoginWithGoogleLinksWhenUnset(t *testing.T) {
	store := &fakeUserStore{user: activeUser()} // GoogleID nil → must link before issuing
	// issue() then fails (Redis unreachable) — irrelevant here; the assertion is
	// that an unlinked, active account gets its google_id linked.
	_, _, _ = newGoogleSvc(store).LoginWithGoogle(context.Background(), "a@b.com", "sub-123")
	if store.linked == nil || *store.linked != "sub-123" {
		t.Fatalf("expected LinkGoogleID called with sub-123, got %v", store.linked)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/identity/ -run LoginWithGoogle`
Expected: FAIL — `userStore`/`LoginWithGoogle`/errors undefined; `NewService` takes `*sqlc.Queries`.

- [ ] **Step 3: Add the seam + method**

In `backend/internal/identity/service.go`:

(a) add the imports `"github.com/google/uuid"` (already imported) and keep `sqlc`. Add the interface + errors:

```go
var (
	ErrNotProvisioned = errors.New("no account exists for this Google email")
	ErrGoogleMismatch = errors.New("email is linked to a different Google account")
)

// userStore is the data surface the identity Service needs (seam for tests).
// *sqlc.Queries satisfies it.
type userStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.IdentityUser, error)
	GetUserByEmail(ctx context.Context, email string) (sqlc.IdentityUser, error)
	LinkGoogleID(ctx context.Context, arg sqlc.LinkGoogleIDParams) error
}
```

(b) change the `Service` struct field and constructor parameter type from `*sqlc.Queries` to `userStore`:

```go
type Service struct {
	q     userStore
	tm    *auth.TokenManager
	store *auth.TokenStore
}

func NewService(q userStore, tm *auth.TokenManager, store *auth.TokenStore) *Service {
	return &Service{q: q, tm: tm, store: store}
}
```
(The router's `identity.NewService(queries, ...)` keeps compiling — `*sqlc.Queries` satisfies `userStore`.)

(c) add the method (after `Login`):

```go
// LoginWithGoogle links a verified Google identity to an EXISTING user (link-only)
// and issues the same token pair as local login. It never creates a user.
func (s *Service) LoginWithGoogle(ctx context.Context, email, googleSub string) (auth.TokenPair, sqlc.IdentityUser, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.TokenPair{}, sqlc.IdentityUser{}, ErrNotProvisioned
		}
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	// Mismatch and status are checked BEFORE linking, so an inactive or
	// already-differently-linked account is never modified.
	if user.GoogleID != nil && *user.GoogleID != googleSub {
		return auth.TokenPair{}, sqlc.IdentityUser{}, ErrGoogleMismatch
	}
	if user.Status != sqlc.SharedUserStatusActive {
		return auth.TokenPair{}, sqlc.IdentityUser{}, ErrUserInactive
	}
	if user.GoogleID == nil {
		if err := s.q.LinkGoogleID(ctx, sqlc.LinkGoogleIDParams{ID: user.ID, GoogleID: &googleSub}); err != nil {
			return auth.TokenPair{}, sqlc.IdentityUser{}, err
		}
	}
	pair, err := s.issue(ctx, user)
	if err != nil {
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	return pair, user, nil
}
```
> Note: the success test (`TestLoginWithGoogleLinksWhenUnset`) reaches `issue`, which calls `store.SaveRefresh` on a `*auth.TokenStore` built over a **nil** redis client. If that panics in this environment, change that one test to assert the link call only by having `LinkGoogleID` record the arg and returning BEFORE issue is reached is NOT acceptable (it must issue). Instead, if nil-redis SaveRefresh panics, wrap the store call — but first try as written: `redis.Client` methods on a nil client return an error rather than panic in go-redis v9, so `issue` returns that error and the test should set `getErr`/expect accordingly. If it errors, adjust the success test to also tolerate a non-nil issue error while still asserting `store.linked == "sub-123"` (the link happening before issue is the assertion that matters). Report whichever path you took.

- [ ] **Step 4: Run tests, build, vet, full suite**

Run: `cd backend && go test ./internal/identity/ -run LoginWithGoogle`
Expected: PASS (4 tests; see the note for the success case).
Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: all pass (existing identity tests still pass — `*sqlc.Queries` satisfies `userStore`).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/identity/service.go backend/internal/identity/google_test.go
git commit -m "feat(identity): link-only LoginWithGoogle + userStore seam"
```

---

### Task 5: Handlers + routes + router wiring + OpenAPI

**Files:**
- Modify: `backend/internal/identity/handler.go` (`googleAuth` interface; Handler `+googleOAuth` + `+frontendURL`; `NewHandler` params; `googleStart`/`googleCallback`/`redirectAuth`)
- Modify: `backend/internal/identity/routes.go` (mount the two GET routes + PerIP)
- Modify: `backend/internal/server/router.go` (build `oauth.New`, pass into `NewHandler`/`RegisterRoutes`)
- Modify: `backend/api/openapi.yaml` (document the two endpoints)
- Test: `backend/internal/identity/google_handler_test.go`

**Interfaces:**
- Consumes: `oauth.Service` (Task 3) via a `googleAuth` interface; `setRefreshCookie` (C1, identity); `LoginWithGoogle` + errors (Task 4); `middleware.PerIP` (subsystem B).
- Produces: `NewHandler(svc, perms, scopes, limiter, loginPerMin, secureCookie, refreshTTL, googleOAuth googleAuth, frontendURL string)`; `RegisterRoutes(..., limiter, loginIPPerMin, refreshPerMin, googleIPPerMin int)`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/identity/google_handler_test.go`:

```go
package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/ragbuaj/inventra/internal/oauth"
)

type fakeGoogle struct {
	url        string
	state      string
	urlErr     error
	email, sub string
	exErr      error
}

func (f fakeGoogle) AuthCodeURL(_ context.Context) (string, string, error) {
	return f.url, f.state, f.urlErr
}
func (f fakeGoogle) Exchange(_ context.Context, _, _ string) (string, string, error) {
	return f.email, f.sub, f.exErr
}

func newGoogleHandler(g googleAuth, store userStore) *Handler {
	h := &Handler{googleOAuth: g, frontendURL: "http://localhost:3000"}
	if store != nil {
		h.svc = newGoogleSvc(store) // from google_test.go
	}
	return h
}

func TestGoogleStartRedirectsToProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/google", newGoogleHandler(fakeGoogle{url: "https://accounts.google.com/o/oauth2/v2/auth?x=1", state: "st"}, nil).googleStart)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "accounts.google.com") {
		t.Fatalf("expected 302 to Google, got %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestGoogleStartDisabledRedirectsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/google", newGoogleHandler(fakeGoogle{urlErr: oauth.ErrDisabled}, nil).googleStart)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.Contains(loc, "oauth=error") || !strings.Contains(loc, "reason=disabled") {
		t.Fatalf("disabled start should redirect error: %d %s", w.Code, loc)
	}
}

func TestGoogleCallbackProviderError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/cb", newGoogleHandler(fakeGoogle{}, nil).googleCallback)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cb?error=access_denied", nil))
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.Contains(loc, "oauth=error") {
		t.Fatalf("provider error should redirect error: %d %s", w.Code, loc)
	}
}

func TestGoogleCallbackNotProvisioned(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := fakeGoogle{email: "x@y.com", sub: "sub"}
	h := newGoogleHandler(g, &fakeUserStore{getErr: pgx.ErrNoRows})
	r := gin.New()
	r.GET("/cb", h.googleCallback)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cb?code=c&state=s", nil))
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.Contains(loc, "reason=not_registered") {
		t.Fatalf("unprovisioned email should redirect reason=not_registered: %d %s", w.Code, loc)
	}
	// Never leak tokens/codes into the redirect URL.
	if strings.Contains(loc, "code=") || strings.Contains(loc, "token") {
		t.Fatalf("redirect leaked sensitive data: %s", loc)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/identity/ -run Google`
Expected: FAIL — `googleAuth`/`googleStart`/`googleCallback`/handler fields undefined.

- [ ] **Step 3: Update handler.go**

In `backend/internal/identity/handler.go`:

(a) add imports `"errors"` (already present), `"net/url"`, and the oauth package:

```go
	"net/url"

	"github.com/ragbuaj/inventra/internal/oauth"
```

(b) add the interface, struct fields, and constructor params:

```go
// googleAuth is the OAuth surface the handler needs (satisfied by *oauth.Service).
type googleAuth interface {
	AuthCodeURL(ctx context.Context) (url, state string, err error)
	Exchange(ctx context.Context, code, state string) (email, sub string, err error)
}

type Handler struct {
	svc          *Service
	perms        *authz.PermissionService
	scopes       *authz.ScopeService
	limiter      ratelimit.Allower
	loginPerMin  int
	secureCookie bool
	refreshTTL   time.Duration
	googleOAuth  googleAuth
	frontendURL  string
}

func NewHandler(svc *Service, perms *authz.PermissionService, scopes *authz.ScopeService, limiter ratelimit.Allower, loginPerMin int, secureCookie bool, refreshTTL time.Duration, googleOAuth googleAuth, frontendURL string) *Handler {
	return &Handler{svc: svc, perms: perms, scopes: scopes, limiter: limiter, loginPerMin: loginPerMin, secureCookie: secureCookie, refreshTTL: refreshTTL, googleOAuth: googleOAuth, frontendURL: frontendURL}
}
```
(`"context"` is already imported via the existing handlers' use; if not, add it.)

(c) add the handlers + redirect helper at the end of the file:

```go
// googleStart redirects the browser to Google's consent screen.
func (h *Handler) googleStart(c *gin.Context) {
	authURL, _, err := h.googleOAuth.AuthCodeURL(c.Request.Context())
	if err != nil {
		h.redirectAuthError(c, googleReason(err))
		return
	}
	c.Redirect(http.StatusFound, authURL)
}

// googleCallback completes the flow: validate, exchange, link-only login, set the
// refresh cookie, and redirect back to the SPA.
func (h *Handler) googleCallback(c *gin.Context) {
	if c.Query("error") != "" {
		h.redirectAuthError(c, "server")
		return
	}
	email, sub, err := h.googleOAuth.Exchange(c.Request.Context(), c.Query("code"), c.Query("state"))
	if err != nil {
		h.redirectAuthError(c, googleReason(err))
		return
	}
	pair, _, err := h.svc.LoginWithGoogle(c.Request.Context(), email, sub)
	if err != nil {
		h.redirectAuthError(c, googleReason(err))
		return
	}
	setRefreshCookie(c, pair.RefreshToken, h.refreshTTL, h.secureCookie)
	c.Redirect(http.StatusFound, h.frontendURL+"/login?oauth=success")
}

// redirectAuthError sends the browser back to the SPA login with a short, safe
// reason code. It never reflects user input into the Location.
func (h *Handler) redirectAuthError(c *gin.Context, reason string) {
	c.Redirect(http.StatusFound, h.frontendURL+"/login?oauth=error&reason="+url.QueryEscape(reason))
}

// googleReason maps an internal error to a fixed, non-sensitive reason code.
func googleReason(err error) string {
	switch {
	case errors.Is(err, oauth.ErrDisabled):
		return "disabled"
	case errors.Is(err, ErrNotProvisioned):
		return "not_registered"
	case errors.Is(err, ErrGoogleMismatch):
		return "account_mismatch"
	case errors.Is(err, ErrUserInactive):
		return "inactive"
	default:
		return "server"
	}
}
```

- [ ] **Step 4: Update routes.go**

In `backend/internal/identity/routes.go`, change the signature to accept the Google rate-limit band and mount the two routes:

```go
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc, limiter ratelimit.Allower, loginIPPerMin, refreshPerMin, googleIPPerMin int) {
	grp := rg.Group("/auth")
	grp.POST("/login", middleware.PerIP(limiter, loginIPPerMin, "auth_login", true), h.login)
	grp.POST("/refresh", middleware.PerIP(limiter, refreshPerMin, "auth_refresh", true), h.refresh)
	grp.GET("/google", middleware.PerIP(limiter, googleIPPerMin, "auth_google", true), h.googleStart)
	grp.GET("/google/callback", middleware.PerIP(limiter, googleIPPerMin, "auth_google", true), h.googleCallback)

	authed := grp.Group("")
	authed.Use(authMW)
	authed.POST("/logout", h.logout)
	authed.GET("/me", h.me)
	authed.GET("/permissions", h.permissions)
	authed.GET("/scope/:module", h.scope)
}
```

- [ ] **Step 5: Update router.go**

In `backend/internal/server/router.go`: build the oauth Service and pass it + the frontend URL into the identity wiring. Add the import `"github.com/ragbuaj/inventra/internal/oauth"` and `"context"` (if not present), then near the identity construction (lines ~125-127):

```go
		googleOAuth, oerr := oauth.New(context.Background(), oauth.Config{
			ClientID:     d.Cfg.GoogleClientID,
			ClientSecret: d.Cfg.GoogleClientSecret,
			RedirectURL:  d.Cfg.GoogleRedirectURL,
			Issuer:       d.Cfg.GoogleIssuer,
		}, d.Redis)
		if oerr != nil {
			d.Log.Warn("google oauth disabled (discovery failed)", "error", oerr)
		}

		identitySvc := identity.NewService(queries, tokenManager, tokenStore)
		identityHandler := identity.NewHandler(identitySvc, permSvc, scopeSvc, d.Limiter, d.Cfg.RateLimitLoginPerMin, d.Cfg.Env == "production", d.Cfg.JWTRefreshTTL, googleOAuth, d.Cfg.FrontendURL)
		identity.RegisterRoutes(api, identityHandler, requireAuth, d.Limiter, d.Cfg.RateLimitLoginIPPerMin, d.Cfg.RateLimitRefreshPerMin, d.Cfg.RateLimitLoginIPPerMin)
```
(Reuse `RateLimitLoginIPPerMin` for the Google band — no new config needed; the auth IP band fits the OAuth redirect endpoints.)

- [ ] **Step 6: Build, vet, full test**

Run: `cd backend && go test ./internal/identity/ -run Google && go build ./... && go vet ./... && go test ./...`
Expected: all green. (`*oauth.Service` satisfies `identity.googleAuth`.)

- [ ] **Step 7: Document in OpenAPI**

In `backend/api/openapi.yaml`, add two operations under `paths`, e.g.:

```yaml
  /auth/google:
    get:
      tags: [Auth]
      summary: Start Google sign-in
      description: Redirects (302) to Google's consent screen. No body.
      responses:
        '302':
          description: Redirect to the Google consent screen.
          headers:
            Location:
              schema: { type: string }
  /auth/google/callback:
    get:
      tags: [Auth]
      summary: Google sign-in callback
      description: >
        Validates state, exchanges the code, verifies the ID token, links the
        account (link-only), sets the `inventra_refresh` HttpOnly cookie, and
        redirects (302) to the SPA at `/login?oauth=success` (or `?oauth=error&reason=...`).
      parameters:
        - { in: query, name: code, schema: { type: string } }
        - { in: query, name: state, schema: { type: string } }
        - { in: query, name: error, schema: { type: string } }
      responses:
        '302':
          description: Redirect back to the SPA.
          headers:
            Location: { schema: { type: string } }
            Set-Cookie: { schema: { type: string }, description: On success, sets the inventra_refresh HttpOnly cookie. }
```
Run (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors. (Match the existing `tags`/style in the file; adapt indentation to fit.)

- [ ] **Step 8: Commit**

```bash
git add backend/internal/identity/handler.go backend/internal/identity/routes.go backend/internal/server/router.go backend/api/openapi.yaml backend/internal/identity/google_handler_test.go
git commit -m "feat(identity): /auth/google endpoints (link-only sign-in via C1 cookie)"
```

---

### Task 6: Frontend — Google button + landing handling + i18n

**Files:**
- Modify: `frontend/app/pages/login.vue`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`
- Test: `frontend/test/nuxt/login-google.spec.ts`

**Interfaces:**
- Consumes: `useAuthApi().refresh()`/`fetchMe()` (existing); `runtimeConfig.public.apiBase`.

- [ ] **Step 1: Write the failing test**

Create `frontend/test/nuxt/login-google.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import LoginPage from '~/pages/login.vue'

const refreshMock = vi.fn(() => Promise.resolve(true))
const fetchMeMock = vi.fn(() => Promise.resolve())
const navigateToMock = vi.fn()

mockNuxtImport('useAuthApi', () => () => ({
  login: vi.fn(),
  logout: vi.fn(),
  refresh: refreshMock,
  fetchMe: fetchMeMock
}))
mockNuxtImport('navigateTo', () => navigateToMock)

describe('login.vue Google landing', () => {
  it('on ?oauth=success it refreshes, fetches me, and navigates home', async () => {
    mockNuxtImport('useRoute', () => () => ({ query: { oauth: 'success' } }))
    await mountSuspended(LoginPage)
    await new Promise(r => setTimeout(r, 10))
    expect(refreshMock).toHaveBeenCalled()
    expect(fetchMeMock).toHaveBeenCalled()
    expect(navigateToMock).toHaveBeenCalledWith('/')
  })

  it('on ?oauth=error it shows the reason message', async () => {
    mockNuxtImport('useRoute', () => () => ({ query: { oauth: 'error', reason: 'not_registered' } }))
    const w = await mountSuspended(LoginPage)
    await new Promise(r => setTimeout(r, 10))
    // The not_registered message text from i18n appears on the page.
    expect(w.html()).toContain('belum terdaftar')
  })
})
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && pnpm test -- login-google`
Expected: FAIL — login.vue doesn't handle `?oauth=*` yet. (If `mockNuxtImport('useRoute', ...)` placement needs adjusting for this repo's setup, adapt the mocking but keep the two behavioral assertions.)

- [ ] **Step 3: Add i18n keys**

In `frontend/i18n/locales/id.json` under `auth`, add a `google` block:

```json
    "google": {
      "error": {
        "not_registered": "Akun untuk email Google ini belum terdaftar. Hubungi admin.",
        "account_mismatch": "Email ini sudah tertaut ke akun Google lain.",
        "inactive": "Akun Anda tidak aktif.",
        "disabled": "Masuk dengan Google belum tersedia.",
        "server": "Gagal masuk dengan Google. Coba lagi."
      }
    }
```
In `frontend/i18n/locales/en.json` under `auth`:

```json
    "google": {
      "error": {
        "not_registered": "No account exists for this Google email. Contact your admin.",
        "account_mismatch": "This email is already linked to a different Google account.",
        "inactive": "Your account is inactive.",
        "disabled": "Google sign-in is not available.",
        "server": "Google sign-in failed. Please try again."
      }
    }
```

- [ ] **Step 4: Wire login.vue**

In `frontend/app/pages/login.vue` `<script setup>`: add the config/route/handlers. Replace the `const { login } = useAuthApi()` line with `const { login, refresh, fetchMe } = useAuthApi()`, and add:

```ts
const config = useRuntimeConfig()
const route = useRoute()

function startGoogle() {
  window.location.href = `${config.public.apiBase}/auth/google`
}

const GOOGLE_REASONS = ['not_registered', 'account_mismatch', 'inactive', 'disabled', 'server']

onMounted(async () => {
  if (route.query.oauth === 'success') {
    try {
      if (await refresh()) {
        await fetchMe()
        await navigateTo('/')
        return
      }
    } catch {
      // fall through to error message
    }
    errorMsg.value = t('auth.google.error.server')
  } else if (route.query.oauth === 'error') {
    const reason = String(route.query.reason ?? 'server')
    errorMsg.value = t(`auth.google.error.${GOOGLE_REASONS.includes(reason) ? reason : 'server'}`)
  }
})
```
Then change the Google button's handler from `@click="notAvailable"` to `@click="startGoogle"`. Leave `notAvailable` for the still-unwired forgot-password link.

- [ ] **Step 5: Run the test, lint, typecheck, full test, build**

Run: `cd frontend && pnpm test -- login-google`
Expected: PASS (2 tests).
Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/pages/login.vue frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/nuxt/login-google.spec.ts
git commit -m "feat(auth): wire Google sign-in button + OAuth landing handling"
```

---

## Verification (before merge)

- [ ] Backend gate: `cd backend && go build ./... && go vet ./... && go test ./...` + Spectral clean.
- [ ] Frontend gate: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`.
- [ ] CI green incl. the existing **e2e** (password login unaffected — Google endpoints are additive and feature-gated; with no `GOOGLE_CLIENT_ID` in CI, the oauth Service is disabled and the app boots normally).
- [ ] **Manual Google round-trip** (out of CI): with real `GOOGLE_CLIENT_ID`/`SECRET`/`REDIRECT_URL`, seed a user with the tester's email, click "Masuk dengan Google", verify: consent → callback → cookie set → SPA `/login?oauth=success` → dashboard. Then test an unseeded email → `?oauth=error&reason=not_registered`. Document the result.

## Self-Review

**1. Spec coverage:** bagian 3 berkas → config/deps/query (T1), state store (T2), oauth Service (T3), identity LoginWithGoogle+userStore (T4), handlers/routes/router/openapi (T5), frontend/i18n (T6). bagian 4 oauth Service (interfaces, AuthCodeURL, Exchange, New feature-gate) — T3. bagian 5 link-only + userStore + LinkGoogleID — T4 (+ T1 query). bagian 6 endpoints + redirects + anti-open-redirect — T5. bagian 7 C1 cookie handoff — T5 (`setRefreshCookie`). bagian 8 frontend — T6. bagian 9 config/deps/feature-gate — T1/T3/T5. bagian 10 testing — each task + manual note. bagian 11 risks (open-redirect fixed FRONTEND_URL + reason whitelist; CSRF state single-use; email_verified; secrets not logged; feature-gate; rate-limit; no migration; graceful discovery) — covered.

**2. Placeholder scan:** No TBD/TODO; every code step has complete code; commands have expected output. The three "if the harness/dep differs" notes (T3 oauth2 version, T4 nil-redis issue, T6 mock placement) name concrete fallbacks, not vague hand-waves.

**3. Type consistency:** `oauth.Service` methods `AuthCodeURL(ctx)(string,string,error)` + `Exchange(ctx,code,state)(string,string,error)` match `identity.googleAuth` (T5) and the fakes (T3/T5). `userStore` (T4) defined with `GetUserByID/GetUserByEmail/LinkGoogleID` — satisfied by `*sqlc.Queries`; `NewService(q userStore, ...)` keeps the router call valid. `NewHandler(... googleOAuth googleAuth, frontendURL string)` (T5) and `RegisterRoutes(... googleIPPerMin int)` (T5) match the router wiring. `setRefreshCookie` (C1) reused in T5. `LinkGoogleIDParams{ID, GoogleID *string}` (T1 sqlc) used in T4. `ErrDisabled`/`ErrEmailNotVerified`/`ErrStateInvalid` (T2/T3) + `ErrNotProvisioned`/`ErrGoogleMismatch`/`ErrUserInactive` (T4) mapped in `googleReason` (T5). Consistent.
