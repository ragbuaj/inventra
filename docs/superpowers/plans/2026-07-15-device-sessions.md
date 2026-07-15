# Device Sessions (Spec B) — Implementation Plan

Spec: `docs/superpowers/specs/2026-07-15-device-sessions-design.md`. Branch: `feat/device-sessions`.
No DB migration (sessions Redis-only). New dep: `github.com/oschwald/geoip2-golang` (+ maxminddb).

## Dependency graph

```
T1 sid(JWT) ─┐
T2 sessionstore ─┼─▶ T5 service wiring ─▶ T6 middleware ─▶ T7 list ─┬─▶ T8 revoke ─┐
T3 geoip ─────┤                                                     └─▶ T9 revoke-others ─▶ T10 openapi
T4 ua-parse ─┘
                                                    T7/T8/T9 contract ─▶ T11 relTime ─▶ T12 useAccount ─▶ T13 account.vue
                                                                                                              └─▶ T14 verify+ship
```

Vertical slices where possible; T1–T4 are the shared substrate every slice needs, so they lead.

## Phase 1 — Backend foundation (substrate)

- **T1 — JWT sid.** `Claims.SID` + `TokenPair.SID`; `Issue(userID, roleID, sid)` threads sid through
  `sign`. **AC:** sid round-trips through `Parse`; `go build ./...` green after the sole caller updates.
- **T2 — Session store** (`internal/auth/sessionstore.go`). `SessionMeta`, `Session`, `TokenStore`
  methods Save/Touch/SessionAlive/List(prune+sort)/Delete/DeleteAll. **AC:** integration test (real
  Redis) covers create→list→touch→revoke→prune→delete-all; order = last_seen desc.
- **T3 — GeoIP** (`internal/geoip`) + `GEOIP_DB_PATH` config + dep. `Locator` iface, `mmdbLocator`,
  `noopLocator`, `New(path,logger)`. **AC:** noop returns empty city/country; `New("")` → noop; unit
  test green; `go mod tidy` clean.
- **T4 — UA parser** (`internal/identity/useragent.go`). **AC:** table test (Chrome/Edge/Safari/Firefox
  ×Win/mac/iOS/Android + empty) passes.

**Checkpoint 1:** `go build/vet/test ./...` + `go test -tags=integration -p 1 ./internal/auth/` green.

## Phase 2 — Backend auth-flow wiring (vertical: session lifecycle)

- **T5 — Service + constructors.** `Service` gains `geoip.Locator`; `startSession(ctx,user,ua,ip)`;
  thread ua/ip through `Login`/`LoginWithGoogle`/`Refresh`; `Logout` deletes caller session;
  `ChangePassword`/`ResetPassword` call `DeleteAllSessions`. Update `NewService`, router wiring, and
  **every existing caller/test** (handler.go, google_test.go, service_test.go, integration). **AC:**
  login creates 1 session; refresh keeps sid + bumps last_seen (integration); existing identity tests
  green.
- **T6 — Middleware.** `CtxSessionID` + session-alive check (skip when sid empty). **AC:** integration —
  a revoked session's access token → `401` on next request; a pre-sid token still passes.

**Checkpoint 2:** all existing auth/identity tests green with new signatures; no regression.

## Phase 3 — Backend endpoints (vertical per capability)

- **T7 — List.** `ListSessions(userID,currentSID)` + `sessionView` DTO + handler + route
  `GET /auth/sessions`. **AC:** returns only caller's sessions, current flagged, desc order.
- **T8 — Revoke one.** `RevokeSession` + `DELETE /auth/sessions/:id` + audit + `ErrNotFound`→404.
  **AC:** revoke kills session (SessionAlive false); foreign/absent sid → 404.
- **T9 — Revoke others.** `RevokeOtherSessions` + `POST /auth/sessions/revoke-others` + audit.
  **AC:** keeps only current; returns `{revoked:n}`.
- **T10 — OpenAPI.** Document the 3 endpoints + `SessionView` schema. **AC:** Spectral 0 errors.

**Checkpoint 3:** backend feature complete; handler tests + Spectral green.

## Phase 4 — Frontend

- **T11 — relTime + i18n.** `formatRelativeTime(iso,locale)` in `utils/format.ts`; i18n keys
  `account.unknownDevice`, `account.now`, relative units (id/en). **AC:** unit test table (id + en).
- **T12 — useAccount.** Real `listSessions`/`revokeSession`/`logoutAllOthers`; map `sessionView` →
  `AccountSession`. **AC:** unit test — icon per device_type, unknown-device label, location-vs-IP meta,
  current → "now".
- **T13 — account.vue.** `logoutAll` re-fetches sessions. **AC:** runtime test — rows render, current
  badge only on current, revoke removes row, logout-all re-fetches (list collapses to current).

**Checkpoint 4:** `pnpm lint && pnpm typecheck && pnpm test && pnpm build` green.

## Phase 5 — Ship

- **T14 — Verify + docs + PR.** Full gate (backend + integration `-p 1` + Spectral + frontend). Update
  `docs/PROGRESS.md` (item 61 → done, next-session block), Obsidian vault (Status & Roadmap, Peta Modul,
  session note, product decision for the GeoIP choice). Commit (Conventional Commits, no AI attribution)
  + open PR.

## Verify commands

```
go build ./... && go vet ./... && go test ./...
go test -tags=integration -p 1 ./internal/auth/ ./internal/identity/
npx --yes @stoplight/spectral-cli lint api/openapi.yaml --ruleset ../.spectral.yaml
cd ../frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build
```
