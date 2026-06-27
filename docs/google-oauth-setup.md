# Google OAuth Sign-In — Setup Guide

Operational guide for enabling **"Sign in with Google"** in Inventra. For the *why* (library choice,
flow design), see [adr/0009-third-party-signin.md](adr/0009-third-party-signin.md).

## How it works (in one paragraph)

Google sign-in uses **OIDC authorization-code + PKCE (S256)**. The backend builds the consent URL,
Google redirects back to `/api/v1/auth/google/callback`, the backend verifies the ID token (audience
pinned to our client ID, `email_verified` required), then mints **our own** session JWT — the same one
local login issues — and sets the refresh token in an httpOnly cookie. We do **not** store Google
refresh tokens (`offline access` is intentionally not requested).

**Link-only — no auto-provisioning.** A Google login only succeeds when a user with that verified email
**already exists and is active**; the `google_id` is linked on first use. There is no self-registration:
an unknown email is refused. This means the consent screen can even be public without any risk of
outsiders gaining access — only pre-provisioned, active accounts can ever complete login.

## Feature gate

The feature is **off by default**. With `GOOGLE_CLIENT_ID` empty, the OAuth service stays disabled, the
`/auth/google*` endpoints respond safely (no panic), and the rest of the app — including local
password login — runs normally. A discovery failure at startup logs a warning, it does **not** crash
the server.

## 1. Create the OAuth client (Google Cloud Console)

1. **APIs & Services → Credentials → Create OAuth client ID → Web application**.
2. Add **Authorized redirect URIs** matching `GOOGLE_REDIRECT_URL` exactly:
   - Dev: `http://localhost:8080/api/v1/auth/google/callback`
   - Prod: `https://<backend-domain>/api/v1/auth/google/callback`
3. Copy the **Client ID** and **Client Secret**.

We request only the **non-sensitive** scopes `openid`, `email`, `profile`. Because none are
sensitive/restricted, **Google app verification / security assessment is not required** even in
production.

## 2. Consent screen: Testing vs Internal vs Production

| Situation | Choose | Why |
|---|---|---|
| **Development / demo** | **External → Testing** | Fast, no friction. Add yourself + colleagues as *test users* (max 100). Safe default regardless of the org's setup. |
| Org **has Google Workspace** (rollout) | **User type: Internal** | Cleanest for an internal app — auto-restricted to the org domain, no 100-user cap, no "unverified app" screen, no verification. |
| **No Workspace** (rollout) | **External → Publish to Production** | One click (scopes are non-sensitive, so no Google review). Removes the 100-user cap and the warning screen. |

To check later whether a Workspace exists: in **OAuth consent screen → User type**, if **"Internal"** is
selectable the project is under a Workspace org; if it's greyed out, only **External** is available.

Switching status later does **not** require recreating the OAuth client — the Client ID/Secret stay the
same.

## 3. Backend environment variables

Set in `backend/.env` (see [../backend/.env.example](../backend/.env.example)). Redis must be running —
it holds the single-use `state` (CSRF) and PKCE verifier via `GETDEL`.

| Variable | Required | Default | Notes |
|---|---|---|---|
| `GOOGLE_CLIENT_ID` | ✅ | _(empty → feature OFF)_ | the feature gate |
| `GOOGLE_CLIENT_SECRET` | ✅ | — | from the Console |
| `GOOGLE_REDIRECT_URL` | ⚠️ | `http://localhost:8080/api/v1/auth/google/callback` | must match the Console URI exactly |
| `FRONTEND_URL` | ⚠️ | `http://localhost:3000` | sole redirect target after callback (anti-open-redirect) |
| `GOOGLE_ISSUER` | — | `https://accounts.google.com` | rarely changed |

## 4. Provision a user (link-only prerequisite)

Because of the link-only policy, before testing make sure a user exists **with the same email** as your
Google account and is **active**. Seed one if needed (from the host, stack up):

```bash
cd backend
go run ./cmd/createadmin -email you@your-google-email.com -password <temp-password>
```

Then start the backend (`go run ./cmd/api`) and frontend (`pnpm dev`), open the login page, and use the
**Sign in with Google** button.

## Error reasons

On failure the callback redirects to `FRONTEND_URL/login?oauth=error&reason=<reason>`, where `reason`
comes from a fixed whitelist surfaced as an i18n message:

| `reason` | Meaning |
|---|---|
| `not_registered` | No active user exists for this verified Google email (link-only). |
| `account_mismatch` | A *different* Google account is claiming an email already linked to another `google_id`. |
| `inactive` | The matching user is deactivated. |
| `disabled` | Google sign-in is not configured (`GOOGLE_CLIENT_ID` empty). |
| `server` | Any other failure (state invalid/expired, exchange/verify error, user cancelled consent, …). |

## CI note

CI runs **without** `GOOGLE_CLIENT_ID`, so the feature is off there and the password e2e remains the
primary auth gate. The real Google round-trip must be verified manually with live credentials.
