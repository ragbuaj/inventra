# ADR-0009 — Third-party sign-in: oauth2 + go-oidc (not goth, not hand-rolled)

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #4 · PRD FR-1.3 / FR-1.4 |

## Context and problem statement

The PRD requires **Google sign-in** (FR-1.3) with **account linking by verified email** (FR-1.4). Today
only the *schema* is prepared: `identity.users.google_id` (nullable) and a read-only `google_linked`
flag; there is **no OAuth flow, no `/auth/google` route, and no oauth dependency**. The question (#4):
when we build it, use a **library** (e.g. `markbates/goth`) or roll our own?

Two facts shape the answer:
- The backend is a **JWT API** (access/refresh tokens in Redis), **not** a server-rendered session app.
- The product is an **internal bank** system; realistically the durable need is **enterprise SSO**
  (Microsoft Entra ID / the bank's OIDC IdP), not a pile of consumer providers.

## Decision drivers

- Fit the existing **JWT/Redis** auth — sign-in should authenticate, then mint the **same app JWT** as
  local login (no second session system).
- **Verify ID tokens** properly (JWKS/discovery), not trust a userinfo blob — bank-grade.
- **Generalize** from Google to enterprise OIDC by configuration, not a rewrite.
- Minimal, battle-tested dependencies; industry standard.

## Considered options

1. **`markbates/goth`** — multi-provider, but built around **gorilla sessions** and server-rendered
   flows. Bolting its session model onto a JWT API is an impedance mismatch, and its strength
   (many consumer providers) isn't what a bank needs. Rejected.
2. **Hand-rolled** with only `golang.org/x/oauth2` — fine for the code exchange, but then we'd
   hand-write ID-token verification (JWKS rotation, issuer/audience checks, discovery). Reinventing a
   solved, security-sensitive problem. Rejected as the primary mechanism.
3. **`golang.org/x/oauth2` + `coreos/go-oidc/v3`** — the standard Go stack for OIDC in API backends:
   oauth2 drives the authorization-code flow; go-oidc does provider **discovery** + **ID-token
   verification** via JWKS. Provider-agnostic (Google now; Entra/Okta/Auth0/bank IdP later by changing
   the issuer + client config). No imposed session framework. **Chosen.**

## Decision outcome

**Chosen: Option 3 — `golang.org/x/oauth2` + `coreos/go-oidc/v3`.**

Flow (lives in `internal/auth` + exposed by `internal/identity`):
1. `GET /auth/google` → redirect to the provider with a signed/stored `state` (CSRF) + PKCE.
2. `GET /auth/google/callback` → validate `state`, exchange code (oauth2), **verify the ID token**
   (go-oidc), extract the **verified** email + `sub`.
3. **Account linking (FR-1.4):** if a user with that email exists, link `google_id`; else create a user
   with the default **Staf** role. Require `email_verified`.
4. Issue the **same JWT access/refresh** as local login (reuse `internal/auth`); Redis token store
   unchanged.

Structure it provider-agnostically (an OIDC verifier per configured issuer) so a future
`/auth/oidc/<provider>` for Microsoft Entra / the bank IdP is config, not new plumbing. Config via env
(`GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_REDIRECT_URL` already in `.env.example`), loaded
per ADR-0003.

## Consequences

- 👍 Standard, secure OIDC (real ID-token verification); fits the JWT API; one token system; extends to
  enterprise SSO by configuration.
- 👍 Small, well-maintained deps; no session framework dragged in.
- 👎 Slightly more wiring than a turnkey `goth` handler — acceptable, and it's the correct shape for a
  JWT API.
- 👎 We own the state/PKCE/linking logic → cover it with tests (ADR-0001), especially the
  link-by-verified-email and `email_verified=false` rejection paths.

## Revisit if

- The product ever needs many **consumer** providers fast and moves to a session model — reconsider goth.
- The bank standardizes on **SAML** (not OIDC) — then add a SAML library alongside (e.g. crewjam/saml).
