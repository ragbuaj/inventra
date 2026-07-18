# Spec — Google OAuth Sign-in (link-only) — C2 (ADR-0009)

| | |
|---|---|
| **Tanggal** | 2026-06-27 |
| **ADR** | [0009](../../adr/0009-third-party-signin.md) (Accepted) — **link-only men-supersede** auto-create FR-1.4 (lihat bagian 2) |
| **Konteks** | Subsistem C terakhir: C1 (httpOnly refresh, **merged**) → **C2 = Google OAuth (ini)**. Handoff token memakai cookie httpOnly C1. |
| **Terkait** | auth JWT/Redis, rate limiting (ADR-0004), logging (ADR-0002), CORS |
| **Status** | Disetujui — siap menulis implementation plan |

## 1. Tujuan & ruang lingkup

Menambah **Sign-in dengan Google (OIDC authorization-code + PKCE)** yang **menautkan** akun Google ke user
yang **sudah ada** (by verified email), lalu menerbitkan **JWT yang sama** seperti login lokal. Tidak ada
sesi kedua; refresh token diserahkan lewat **cookie httpOnly C1**.

**Dalam ruang lingkup:** paket `internal/oauth` (oauth2 + go-oidc, state/PKCE di Redis), `identity.LoginWithGoogle`
(link-only) + query `LinkGoogleID`, endpoint `GET /auth/google` + `GET /auth/google/callback`, wiring + rate-limit,
OpenAPI, tombol Google + penanganan landing di frontend, env + deps, test.

**Di luar ruang lingkup:** auto-provisioning user baru (ditolak — lihat bagian 2); provider non-Google (disiapkan
secara config, tidak diimplementasikan); UI manajemen "unlink Google"; SAML.

## 2. Keputusan desain (disepakati)

1. **Link-only (men-supersede ADR-0009/FR-1.4 auto-create).** Email Google terverifikasi yang **belum** ada
   sebagai user → **ditolak** ("akun belum terdaftar, hubungi admin"). Tidak ada self-provisioning — lebih aman
   untuk sistem internal bank. *Ini perubahan sadar terhadap ADR-0009; ADR akan ditandai diperbarui.*
2. **Handoff token via cookie httpOnly C1.** Callback `setRefreshCookie` (helper C1) + redirect; SPA memanggil
   `/auth/refresh` untuk access token. Tanpa one-time-code, tanpa token di URL.
3. **PKCE + state** wajib; state disimpan **server-side di Redis**, **sekali-pakai**, TTL pendek (CSRF).
4. **Provider-agnostik by config**: verifier dibentuk dari **issuer + client config** (bukan hardcode), agar
   Entra/IdP bank nanti = config.
5. **Testabilitas**: `identity.Service` bergantung pada interface kecil **`userStore`** (dipenuhi `*sqlc.Queries`),
   dan OIDC verify/exchange di balik interface — sehingga jalur link-only & handler dapat di-test dengan fake
   (round-trip Google asli tak bisa di-CI).
6. Deps baru: `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3`.

## 3. Berkas

```
backend/internal/oauth/oauth.go        ← OIDC provider/verifier + oauth2.Config; Verifier/Exchanger interfaces; Service (AuthCodeURL/Exchange)
backend/internal/oauth/state.go         ← Redis state store: state→PKCE verifier, single-use, TTL
backend/internal/oauth/*_test.go        ← state store, PKCE/state gen, Exchange via fake verifier
backend/internal/identity/service.go     ← userStore interface; Service.q→userStore; + LoginWithGoogle + ErrNotProvisioned/ErrGoogleMismatch
backend/internal/identity/google_test.go ← LoginWithGoogle link-only logic (fake userStore)
backend/internal/identity/handler.go      ← googleStart + googleCallback handlers (set cookie + redirect)
backend/internal/identity/routes.go       ← mount /auth/google + /auth/google/callback (+ PerIP rate-limit)
backend/db/queries/identity.sql           ← + LinkGoogleID; run sqlc generate
backend/internal/config/config.go         ← + GoogleIssuer (default https://accounts.google.com); Google* sudah ada
backend/internal/server/router.go         ← bangun oauth.Service (dari d.Cfg+d.Redis) + inject ke identity NewHandler/RegisterRoutes
backend/api/openapi.yaml                   ← dokumentasi GET /auth/google (302) + callback (302)
backend/.env.example                       ← GOOGLE_ISSUER + (sudah ada) GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL
frontend/app/pages/login.vue               ← tombol Google → redirect; tangani ?oauth=success|error (pakai authApi.refresh()+fetchMe yang sudah ada)
frontend/i18n/locales/{id,en}.json          ← pesan error/label Google

(main.go & dto.go TIDAK berubah: oauth.Service dibangun di NewRouter; callback me-redirect, tak ada body DTO baru.)
```

## 4. Paket `internal/oauth`

```go
// Verifier verifies a Google/OIDC ID token and returns the verified claims we need.
type Verifier interface {
	Verify(ctx context.Context, rawIDToken string) (email string, emailVerified bool, sub string, err error)
}

// Exchanger exchanges an auth code (with PKCE verifier) for a token set incl. the raw id_token.
type Exchanger interface {
	Exchange(ctx context.Context, code, codeVerifier string) (rawIDToken string, err error)
}

type Service struct {
	cfg      Config       // clientID/secret/redirectURL/issuer
	exch     Exchanger    // *oauth2 wrapper in prod
	verifier Verifier     // *go-oidc wrapper in prod
	state    *stateStore  // Redis
}

// AuthCodeURL generates a state + PKCE verifier, stores them, and returns the Google consent URL.
func (s *Service) AuthCodeURL(ctx context.Context) (url, state string, err error)

// Exchange validates the single-use state, exchanges the code, verifies the ID token, and returns
// the verified email + subject. Rejects when email_verified is false.
func (s *Service) Exchange(ctx context.Context, code, state string) (email, sub string, err error)
```

- `New(cfg, rdb)` melakukan OIDC discovery atas `issuer` → membangun `*oidc.IDTokenVerifier` (audience=clientID) +
  `oauth2.Config` (endpoint dari provider, scopes `openid email profile`). PKCE: `S256`.
- **state.go**: `stateStore` di Redis, prefix `oauth:state:`, value = PKCE verifier, TTL ~5 mnt; `Consume(state)`
  mengembalikan verifier dan **menghapus** key (sekali-pakai). State + verifier dibuat via `crypto/rand` (URL-safe).

## 5. Identity — link-only + testabilitas

- Interface `userStore` (di `service.go`) berisi method yang dipakai Service: `GetUserByID`, `GetUserByEmail`,
  `LinkGoogleID`. `Service.q` bertipe `userStore` (dipenuhi `*sqlc.Queries`; pemanggil `NewService(queries, …)` tak berubah).
- `LoginWithGoogle(ctx, email, googleSub) (auth.TokenPair, sqlc.IdentityUser, error)`:
  1. `GetUserByEmail(email)`; `pgx.ErrNoRows` → `ErrNotProvisioned`.
  2. `user.GoogleID == nil` → `LinkGoogleID(user.ID, googleSub)`.
  3. `user.GoogleID != nil && *user.GoogleID != googleSub` → `ErrGoogleMismatch`.
  4. `user.Status != active` → `ErrUserInactive`.
  5. `issue(ctx, user)` (token pair yang sama dengan login lokal).
- Query baru `LinkGoogleID`: `UPDATE identity.users SET google_id = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL` → `:exec`. `sqlc generate`.

## 6. Endpoint & alur

- **`GET /auth/google`** (handler `googleStart`): `url, state, _ := oauth.AuthCodeURL(ctx)` → `c.Redirect(302, url)`.
  (PerIP rate-limit, prefix `auth_google`.)
- **`GET /auth/google/callback`** (handler `googleCallback`): baca `code`,`state`,`error` query.
  - provider error / state invalid / exchange gagal → `redirectError(c, reason)`.
  - `email, sub, err := oauth.Exchange(ctx, code, state)`; err → `redirectError`.
  - `pair, _, err := identity.LoginWithGoogle(ctx, email, sub)`:
    - `ErrNotProvisioned`/`ErrGoogleMismatch`/`ErrUserInactive` → `redirectError(c, <kode aman>)`.
    - lain → `redirectError(c, "server")`.
  - sukses → `setRefreshCookie(c, pair.RefreshToken, refreshTTL, secureCookie)` + `c.Redirect(302, FRONTEND_URL+"/login?oauth=success")`.
- `redirectError(c, reason)` → `c.Redirect(302, FRONTEND_URL+"/login?oauth=error&reason="+reason)`. **Redirect hanya
  ke `FRONTEND_URL`** (fixed config) — tak pernah ke URL dari input pengguna (anti open-redirect). `reason` adalah
  kode pendek non-sensitif (`not_registered`,`account_mismatch`,`inactive`,`server`).

## 7. Handoff token (reuse C1)

Callback hanya `Set-Cookie` refresh (httpOnly, helper C1) + redirect. SPA di `/login?oauth=success` memanggil
`authApi.refresh()` → access token (memori) → `fetchMe` → dashboard. Tidak ada token di URL.

## 8. Frontend

- `login.vue`: tombol **"Masuk dengan Google"** → `window.location.href = \`${apiBase}/auth/google\``
  (redirect halaman penuh; `apiBase` dari runtimeConfig).
- `login.vue` saat mount/route: bila `route.query.oauth === 'success'` → `await authApi.refresh()`; sukses →
  `fetchMe` → `navigateTo('/')`. Bila `=== 'error'` → tampilkan pesan i18n berdasar `route.query.reason`
  (mapping reason→pesan; default pesan umum). Bersihkan query setelah ditangani.
- i18n: label tombol (sudah ada) + pesan error (`auth.google.error.not_registered`, `account_mismatch`, dst).

## 9. Config & deps

- `config.Config` + `GoogleIssuer` (`getEnv("GOOGLE_ISSUER", "https://accounts.google.com")`). `GoogleClientID`/
  `Secret`/`RedirectURL` sudah ada. `FrontendURL` (ada) dipakai untuk redirect landing.
- `.env.example`: tambah `GOOGLE_ISSUER=https://accounts.google.com` (+ komentar bahwa CLIENT_ID/SECRET wajib agar fitur aktif).
- **Feature-gate**: bila `GoogleClientID == ""` → OIDC discovery dilewati; `/auth/google` membalas redirect error
  `reason=disabled` (atau 404). Stack dev/CI tanpa kredensial Google **tetap boot** tanpa error.
- Deps: `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3` (`go get` + `go mod tidy`).

## 10. Testing (proaktif & luas)

- **state store** (Redis): simpan→consume mengembalikan verifier lalu key hilang (sekali-pakai); state tak dikenal → error; TTL diset. (Fake/abstraksi Redis atau testcontainers; unit pakai interface seperti pola lain.)
- **state/PKCE generation**: panjang cukup, URL-safe, unik antar panggilan; PKCE `S256` challenge cocok dengan verifier.
- **`oauth.Service.Exchange`**: dengan **fake Exchanger+Verifier** — sukses (email_verified true) → email+sub; `email_verified=false` → ditolak; state invalid → ditolak (tanpa memanggil exchanger); state dikonsumsi (sekali-pakai).
- **`identity.LoginWithGoogle`** (fake `userStore`): not-provisioned→`ErrNotProvisioned`; google_id nil→memanggil `LinkGoogleID` lalu issue; mismatch→`ErrGoogleMismatch`; inactive→`ErrUserInactive`; sukses→token pair.
- **handler `googleCallback`** (fake oauth.Service + identity): provider error→redirect error; state invalid→redirect error; sukses→`Set-Cookie` refresh + redirect `?oauth=success`; tiap error-reason memetakan ke redirect `?oauth=error&reason=…`. Assert **tak ada** token/secret di URL/log.
- **Feature-gate**: ClientID kosong → start endpoint membalas `reason=disabled` tanpa panik; `New` aman.
- **Frontend**: `login.vue` menangani `?oauth=success` (memanggil refresh+fetchMe) dan `?oauth=error` (pesan i18n) — runtime test; tombol memicu redirect ke `…/auth/google` (assert href/lokasi).
- **Round-trip Google asli**: verifikasi **manual** dengan kredensial nyata (di luar CI) — dicatat di README/spec. Tak ada e2e Google di CI (butuh secret + akun).
- **Gate**: `go build/vet/test ./...` + Spectral; `pnpm lint/typecheck/test/build`. (E2E login lokal/CI tetap hijau — endpoint baru tak mengubah login password.)

## 11. Risiko & catatan

- **Anti open-redirect**: callback hanya redirect ke `FRONTEND_URL` (config), `reason` terbatas pada kode pendek terdaftar — tak pernah memantulkan input pengguna ke `Location`.
- **CSRF**: state acak server-side, sekali-pakai, TTL pendek; cocokkan `state` yang kembali. PKCE melindungi penukaran code.
- **email_verified wajib**: tolak bila `false` (ADR-0009).
- **Rahasia**: `GOOGLE_CLIENT_SECRET` via env (ADR-0003); tak pernah di-log (redaksi ADR-0002 + tak menaruh token/secret di attr log).
- **Feature-gate** agar dev/CI tanpa kredensial tetap jalan & e2e password tak terpengaruh.
- **Rate-limit** pada `/auth/google` & callback (reuse subsistem B) untuk membatasi penyalahgunaan.
- **Tak ada migrasi DB**: kolom `google_id` + unique index sudah ada (migrasi 000003); hanya query `LinkGoogleID` baru.
- **OIDC discovery saat startup**: bila issuer tak terjangkau saat boot dan ClientID di-set → tangani gracefully (log warning, fitur Google nonaktif) agar service tetap naik; jangan `Fatal`.
