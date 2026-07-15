# Device Sessions (Spec B) — Design

**Date:** 2026-07-15
**Status:** Approved (design)
**Source:** PROGRESS.md item 53 — user-picked next step "reset password dan sesi perangkat". Second of
the two-spec decomposition; **Spec A — Account Security via Email** (forgot/change password) shipped in
PR #65 and explicitly deferred device-session list/revoke here. Spec B replaces the last mock in
`useAccount` (`listSessions` / `revokeSession` / `logoutAllOthers`).

**Confirmed decisions (2026-07-15):** session metadata in **Redis** (not Postgres); revoked sessions
enforced **instantly** via a per-request session-alive check; device location shown via **GeoIP**
(MaxMind GeoLite2) with graceful fallback to raw IP when no DB is provisioned.

## 1. Objective & users

**User:** any signed-in Inventra user, on the "Sesi & Perangkat" card of the account page
(`docs/design/Profil Akun.dc.html`).

**Objective:** let a user see every device with an active session (an unexpired refresh-token lineage),
revoke any one of them, and "keluar dari semua perangkat lain" (log out everywhere but the current
device). This closes the security loop Spec A opened: Spec A invalidates **all** sessions on a password
change via the `password_changed_at` epoch; Spec B adds **selective** revocation via a per-user session
index and instant enforcement.

**In scope:** stable session id surviving refresh rotation; device metadata (User-Agent → browser/OS/
device-type, IP, GeoIP city/country) captured at login; `GET /auth/sessions`,
`DELETE /auth/sessions/:id`, `POST /auth/sessions/revoke-others`; instant access-token invalidation of a
revoked session; real `useAccount` wiring; tests.

**Out of scope (deferred):** "trusted device"/remember-me; email-on-new-login alerts; an admin viewing
another user's sessions; per-session step-up auth.

## 2. Acceptance criteria

1. Logging in from a device creates exactly one session row, labelled with that device's browser/OS,
   IP, and (if resolvable) city/country.
2. `GET /auth/sessions` lists only the caller's own sessions, current one flagged, newest activity first.
3. A refresh (`/auth/refresh`) does **not** create a new session — it updates the same session's
   `last_seen` and keeps its `sid`.
4. Revoking a session (`DELETE /auth/sessions/:id`) makes that device's **next** request fail `401`
   (access + refresh both dead) within one request cycle; revoking a sid that is not the caller's own
   returns `404`.
5. "Revoke others" kills every session except the current one; the list then shows only the current row.
6. A password change/reset clears **all** the user's sessions (uniform logout everywhere).
7. Dev/CI without a GeoIP DB still works end-to-end — location degrades to the raw IP, nothing crashes.
8. Full gate green: `go build/vet/test`, integration `internal/identity` + `internal/auth`, Spectral,
   `pnpm lint/typecheck/test/build`.

## 3. Key architecture decisions

- **Storage — Redis (chosen).** Sessions are ephemeral (they expire with the refresh TTL) and token
  state is already 100% Redis; a Postgres table would duplicate lifecycle management for no functional
  gain. No migration. Revoke *events* still go to `audit_logs`.
- **Session identity across rotation.** The refresh JTI rotates every `/auth/refresh`, so a separate
  `sid` (uuid) is minted at login, embedded as a `sid` claim in **both** access and refresh tokens, and
  carried forward unchanged through every rotation. The session record is keyed by `sid`; the current
  refresh JTI lives inside the record.
- **Instant revocation (chosen).** `RequireAuth` gains a session-alive check: a token carrying a `sid`
  whose session record is gone is rejected `401`. One extra Redis `EXISTS` per request (the denylist
  check already does one). Tokens minted before this deploy (no `sid`) skip the check and age out —
  backward compatible.
- **GeoIP (chosen).** A `internal/geoip` package with a `Locator` interface: an mmdb-backed impl
  (`oschwald/geoip2-golang`, reading a configured `GEOIP_DB_PATH`) and a `noop` impl used when the path
  is empty/unreadable (dev/CI) — mirroring the email `LogSender` fallback. Location is resolved **once at
  login** from the client IP and stored in the session record; private/loopback IPs and a missing DB both
  yield an empty location, and the UI falls back to the IP. The GeoLite2 `.mmdb` is **not** committed
  (licence + size); ops provision it and set `GEOIP_DB_PATH` in production.
- **UA parsing.** No new dependency for this part — a small pure-Go `parseUserAgent` does substring
  matching to `{browser, os, deviceType}`; unknowns degrade to a generic label, never a crash.
- **Password-change interaction.** On reset/change-password the service **deletes all** the user's
  session records (in addition to Spec A's epoch), so the list clears and every device's access dies on
  its next request.

## 4. Backend

### 4.1 JWT (`internal/auth/jwt.go`)
`Claims.SID` + `TokenPair.SID`; `Issue(userID, roleID, sid string)` threads `sid` through `sign`. Login
mints a fresh sid; Refresh reuses `claims.SID`.

### 4.2 Session store (`internal/auth/sessionstore.go`, new — extends `TokenStore`)
Keys alongside `auth:refresh:` / `auth:denylist:`:
- `auth:session:<sid>` → hash `{user_id, ua, ip, location, refresh_jti, created_at, last_seen}`,
  TTL = refresh TTL, bumped on refresh.
- `auth:usessions:<userID>` → SET of live sids.

Methods: `SaveSession`, `TouchSession` (rotation: last_seen + refresh_jti + re-expire + self-heal index),
`SessionAlive`, `ListSessions` (hydrate, lazy-prune expired index entries, sort by last_seen desc),
`DeleteSession` (returns the refresh JTI to also drop from the whitelist), `DeleteAllSessions`.

### 4.3 GeoIP (`internal/geoip`, new)
`Locator interface { Lookup(ip string) (city, country string) }`; `mmdbLocator` (opens `GEOIP_DB_PATH`
once at startup, thread-safe reads); `noopLocator` (returns empty). `New(path, logger)` picks the impl
and logs which. Config: `GEOIP_DB_PATH` (default `""`). Wired in `cmd/api` / router deps.

### 4.4 UA parse (`internal/identity/useragent.go`, new)
`parseUserAgent(ua) (browser, os, deviceType)` — pure, table-tested.

### 4.5 Service (`internal/identity/service.go`)
- `startSession(ctx, user, ua, ip)` — mint sid, resolve location via the locator, `Issue(...,sid)`,
  `SaveRefresh`, `SaveSession`. Login + LoginWithGoogle call it.
- Thread `ua, ip` into `Login` / `LoginWithGoogle` / `Refresh`. Refresh reuses sid + `TouchSession`.
- `Logout` also `DeleteSession` for the caller's sid; `ChangePassword`/`ResetPassword` also
  `DeleteAllSessions`.
- New: `ListSessions(ctx, userID, currentSID)`, `RevokeSession(ctx, userID, sid)` (per-user index makes
  SoD automatic; foreign/absent sid → `ErrNotFound`), `RevokeOtherSessions(ctx, userID, currentSID)`.
- `Service` gains a `geoip.Locator` dependency.

### 4.6 Middleware (`internal/middleware/auth.go`)
`CtxSessionID` from `claims.SID`; after denylist, `if sid != "" && !SessionAlive(sid) → 401`.

### 4.7 Handler / routes / DTO
Authed endpoints under `/auth`: `GET /auth/sessions` → `[]sessionView`;
`DELETE /auth/sessions/:id` (`404` if not caller's); `POST /auth/sessions/revoke-others` → `{revoked:n}`.
`sessionView` (snake_case, ADR 0007): `{id, browser, os, device_type, ip_address, location, created_at,
last_seen_at, current}`. Revoke + revoke-others write audit records. Login/refresh/google/logout handlers
pass `c.Request.UserAgent()` + `c.ClientIP()` (+ access sid for logout). `openapi.yaml` updated.

## 5. Frontend

- `useAccount.ts` — real `listSessions`/`revokeSession`/`logoutAllOthers`; map `sessionView` → the
  existing `AccountSession` `{id, device, meta, icon, current}` (no row-template change):
  - `device` = `browser · os` (both unknown → `t('account.unknownDevice')`).
  - `icon` = deviceType → smartphone / tablet / monitor / globe.
  - `meta` = `location · <relative last_seen>` when location present, else `ip · <relative>`; current →
    `… · t('account.now')`.
- `formatRelativeTime(iso, locale)` util (id/en) + i18n keys (`account.unknownDevice`, `account.now`,
  relative units).
- `account.vue` — `logoutAll` re-fetches sessions afterwards so the list collapses to the current device.

## 6. Testing

- **Backend unit:** `parseUserAgent` table; jwt sid round-trip; `geoip` noop + (skip-if-no-db) mmdb.
- **Backend integration (real Redis):** session store CRUD/prune/order; login creates session; refresh
  keeps sid + bumps last_seen; list marks current; revoke → SessionAlive false; revoke-others keeps only
  current; password change clears all.
- **Backend handler:** the 3 endpoints incl. `404` on foreign sid + current-flag correctness.
- **Frontend unit:** `useAccount` mapping (icons, unknown label, location vs IP meta) + `formatRelativeTime`.
- **Frontend runtime (`account.vue`):** rows render, "current" badge only on current, revoke removes the
  row, logout-all re-fetches.

## 7. Boundaries

- **Always:** enforce SoD (a user only ever sees/revokes their own sessions); resolve the employee/user
  id from the caller's own token, never from input; keep dev/CI green without a GeoIP DB; put every UI
  string in i18n; keep OpenAPI in sync; audit revoke actions.
- **Ask first:** committing any GeoIP data file to the repo; changing the JWT claim shape beyond `sid`;
  altering Spec A's password-change/epoch behaviour.
- **Never:** fabricate a location we cannot resolve; serialize `password_hash`/`google_id`; introduce a
  DB migration for this feature; let a 401 from these endpoints trip the frontend's force-logout
  interceptor for non-auth validation failures.
