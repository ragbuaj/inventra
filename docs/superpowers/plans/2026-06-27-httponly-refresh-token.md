# httpOnly Refresh-Token Hardening (C1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the refresh token from a JS-readable cookie into a backend-set `HttpOnly` cookie so XSS cannot steal it; access token stays in memory.

**Architecture:** A small cookie helper in `internal/identity` sets/clears the refresh cookie (`HttpOnly`, `Secure` only in prod, `SameSite=Lax`, `Path=/api/v1/auth`). `login` sets it; `refresh`/`logout` read/clear it from the cookie instead of the request body. `tokenResponse` stops carrying the refresh token. The SPA uses `credentials: 'include'` on auth calls, stops storing the refresh token, and the rehydration plugin always attempts a refresh on cold load. OpenAPI + the Bruno collection are updated to match.

**Tech Stack:** Go 1.25, Gin (`c.SetCookie`/`c.Cookie`), Nuxt 4 / Vitest, Bruno, OpenAPI/Spectral.

## Global Constraints

- Cookie: name `inventra_refresh`, `HttpOnly=true`, `Secure = (cfg.Env == "production")`, `SameSite=Lax`, `Path=/api/v1/auth`, `Max-Age = cfg.JWTRefreshTTL` seconds. **`Secure` MUST be env-gated** — dev/CI run over plain HTTP, a `Secure` cookie would not be sent and login (incl. the e2e) would break.
- `tokenResponse` returns only `access_token`, `token_type`, `expires_in` — never `refresh_token`.
- `/auth/refresh` and `/auth/logout` read the refresh token from the cookie, not the request body.
- Frontend: `credentials: 'include'` on all auth calls (`/auth/login`, `/auth/refresh`, `/auth/logout`); no JS-stored refresh token; `useRefreshCookie` deleted; rehydration plugin always attempts `/auth/refresh` when not authenticated.
- No new deps, no DB migration. Backend gate `go build/vet/test ./...` + Spectral; frontend gate `pnpm lint/typecheck/test/build`. **E2E `login.spec.ts` must stay green** (verified before merge). Conventional Commits with scope, no AI/co-author trailers. Branch `feat/httponly-refresh` (already checked out).

---

### Task 1: Cookie helper

**Files:**
- Create: `backend/internal/identity/cookie.go`
- Test: `backend/internal/identity/cookie_test.go`

**Interfaces:**
- Produces: `const refreshCookieName = "inventra_refresh"`; `setRefreshCookie(c *gin.Context, token string, ttl time.Duration, secure bool)`; `clearRefreshCookie(c *gin.Context, secure bool)`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/identity/cookie_test.go`:

```go
package identity

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSetRefreshCookieAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setRefreshCookie(c, "rt-123", time.Hour, false)
	sc := w.Header().Get("Set-Cookie")
	for _, want := range []string{"inventra_refresh=rt-123", "HttpOnly", "Path=/api/v1/auth", "SameSite=Lax", "Max-Age=3600"} {
		if !strings.Contains(sc, want) {
			t.Fatalf("Set-Cookie missing %q: %s", want, sc)
		}
	}
	if strings.Contains(sc, "Secure") {
		t.Fatalf("Secure must be absent when secure=false: %s", sc)
	}
}

func TestSetRefreshCookieSecureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setRefreshCookie(c, "rt", time.Hour, true)
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Secure") {
		t.Fatalf("Secure must be present when secure=true: %s", w.Header().Get("Set-Cookie"))
	}
}

func TestClearRefreshCookieExpires(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	clearRefreshCookie(c, false)
	sc := w.Header().Get("Set-Cookie")
	if !strings.Contains(sc, "inventra_refresh=") || !strings.Contains(sc, "Max-Age=0") {
		t.Fatalf("clear should expire the cookie (Max-Age=0): %s", sc)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/identity/ -run RefreshCookie`
Expected: FAIL — `setRefreshCookie`/`clearRefreshCookie` undefined.

- [ ] **Step 3: Implement the helper**

Create `backend/internal/identity/cookie.go`:

```go
package identity

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// refreshCookieName is the HttpOnly cookie that carries the refresh token.
const refreshCookieName = "inventra_refresh"

// refreshCookiePath scopes the cookie to the auth endpoints only, so the
// long-lived refresh token never travels to business endpoints.
const refreshCookiePath = "/api/v1/auth"

// setRefreshCookie writes the refresh token as an HttpOnly, SameSite=Lax cookie.
// secure (TLS-only) is enabled in production; dev/CI run over plain HTTP.
func setRefreshCookie(c *gin.Context, token string, ttl time.Duration, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, token, int(ttl.Seconds()), refreshCookiePath, "", secure, true)
}

// clearRefreshCookie expires the refresh cookie (logout).
func clearRefreshCookie(c *gin.Context, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", secure, true)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/identity/ -run RefreshCookie`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/identity/cookie.go backend/internal/identity/cookie_test.go
git commit -m "feat(security): HttpOnly refresh-cookie helper (set/clear)"
```

---

### Task 2: Handler + dto + router wiring (cookie-based refresh)

**Files:**
- Modify: `backend/internal/identity/dto.go` (`tokenResponse` drops `refresh_token`; remove `refreshRequest`/`logoutRequest`)
- Modify: `backend/internal/identity/handler.go` (Handler fields + `NewHandler` + login/refresh/logout)
- Modify: `backend/internal/server/router.go:126` (pass `secureCookie` + `refreshTTL`)
- Test: `backend/internal/identity/handler_cookie_test.go`

**Interfaces:**
- Consumes: `setRefreshCookie`/`clearRefreshCookie`/`refreshCookieName` (Task 1).
- Produces: `NewHandler(svc *Service, perms *authz.PermissionService, scopes *authz.ScopeService, limiter ratelimit.Allower, loginPerMin int, secureCookie bool, refreshTTL time.Duration) *Handler`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/identity/handler_cookie_test.go`:

```go
package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/auth"
)

// refresh with no cookie must 401 before touching the (nil) service.
func TestRefreshMissingCookieReturns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{} // svc nil — the cookie guard must return first
	r := gin.New()
	r.POST("/auth/refresh", h.refresh)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/refresh", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a refresh cookie, got %d", w.Code)
	}
}

// the token response must never serialize a refresh_token.
func TestTokenResponseOmitsRefreshToken(t *testing.T) {
	b, err := json.Marshal(newTokenResponse(auth.TokenPair{AccessToken: "a", AccessExpiresAt: time.Now().Add(time.Minute)}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "refresh_token") {
		t.Fatalf("token response must not contain refresh_token: %s", b)
	}
	if !strings.Contains(string(b), "access_token") {
		t.Fatalf("token response missing access_token: %s", b)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && go test ./internal/identity/ -run "RefreshMissingCookie|TokenResponseOmits"`
Expected: FAIL — `refresh` still binds a body / `tokenResponse` still has `refresh_token` (compile or assertion failure).

- [ ] **Step 3: Update dto.go**

In `backend/internal/identity/dto.go`: **delete** the `refreshRequest` and `logoutRequest` structs, and change `tokenResponse` + `newTokenResponse` to drop the refresh token:

```go
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"` // access token lifetime, seconds
}

func newTokenResponse(p auth.TokenPair) tokenResponse {
	return tokenResponse{
		AccessToken: p.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(p.AccessExpiresAt).Seconds()),
	}
}
```
(`loginRequest` is unchanged. `time` and `auth` imports stay.)

- [ ] **Step 4: Update handler.go**

In `backend/internal/identity/handler.go`:

(a) add `"time"` to imports (keep existing). Extend the struct + constructor:

```go
type Handler struct {
	svc          *Service
	perms        *authz.PermissionService
	scopes       *authz.ScopeService
	limiter      ratelimit.Allower
	loginPerMin  int
	secureCookie bool
	refreshTTL   time.Duration
}

func NewHandler(svc *Service, perms *authz.PermissionService, scopes *authz.ScopeService, limiter ratelimit.Allower, loginPerMin int, secureCookie bool, refreshTTL time.Duration) *Handler {
	return &Handler{svc: svc, perms: perms, scopes: scopes, limiter: limiter, loginPerMin: loginPerMin, secureCookie: secureCookie, refreshTTL: refreshTTL}
}
```

(b) set the cookie on successful `login` (after the existing rate-limit check + `svc.Login`):

```go
	pair, _, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.authError(c, err)
		return
	}
	setRefreshCookie(c, pair.RefreshToken, h.refreshTTL, h.secureCookie)
	c.JSON(http.StatusOK, newTokenResponse(pair))
```

(c) replace `refresh` to read the cookie:

```go
func (h *Handler) refresh(c *gin.Context) {
	rt, err := c.Cookie(refreshCookieName)
	if err != nil || rt == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), rt)
	if err != nil {
		h.authError(c, err)
		return
	}
	setRefreshCookie(c, pair.RefreshToken, h.refreshTTL, h.secureCookie)
	c.JSON(http.StatusOK, newTokenResponse(pair))
}
```

(d) replace `logout` to read + clear the cookie:

```go
func (h *Handler) logout(c *gin.Context) {
	rt, _ := c.Cookie(refreshCookieName)
	jti, _ := c.Get(middleware.CtxAccessJTI)
	exp, _ := c.Get(middleware.CtxAccessExp)
	if err := h.svc.Logout(c.Request.Context(), jti.(string), exp.(time.Time), rt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}
	clearRefreshCookie(c, h.secureCookie)
	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}
```

- [ ] **Step 5: Update router.go (pass the two new args)**

In `backend/internal/server/router.go`, change the `identity.NewHandler(...)` call (currently line ~126) to pass `secureCookie` + `refreshTTL`:

```go
		identityHandler := identity.NewHandler(identitySvc, permSvc, scopeSvc, d.Limiter, d.Cfg.RateLimitLoginPerMin, d.Cfg.Env == "production", d.Cfg.JWTRefreshTTL)
```
(`identity.RegisterRoutes(...)` on the next line is unchanged.)

- [ ] **Step 6: Run tests, build, vet, full suite**

Run: `cd backend && go test ./internal/identity/ -run "RefreshMissingCookie|TokenResponseOmits"`
Expected: PASS.
Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: all pass (the existing `ratelimit_test.go` `TestLoginAccountKeyRateLimited` still passes — it denies before `svc.Login`, unaffected by the cookie).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/identity/dto.go backend/internal/identity/handler.go backend/internal/identity/handler_cookie_test.go backend/internal/server/router.go
git commit -m "feat(security): refresh/logout read refresh token from HttpOnly cookie; login sets it"
```

---

### Task 3: OpenAPI + Bruno collection + README

**Files:**
- Modify: `backend/api/openapi.yaml`
- Modify: `docs/api/bruno/Auth/Login.bru`, `docs/api/bruno/Auth/Refresh.bru`, `docs/api/bruno/Auth/Logout.bru`
- Modify: `docs/api/README.md`

**Interfaces:** none (docs/collection only).

- [ ] **Step 1: Update OpenAPI**

In `backend/api/openapi.yaml`:

(a) `TokenResponse` — drop `refresh_token` from `required` and `properties`:

```yaml
    TokenResponse:
      type: object
      required: [access_token, token_type, expires_in]
      properties:
        access_token:
          type: string
        token_type:
          type: string
          examples: ["Bearer"]
        expires_in:
          type: integer
          description: Access-token lifetime in seconds
          examples: [900]
```

(b) Remove the `requestBody` from the `POST /auth/refresh` and `POST /auth/logout` operations (they now read the `inventra_refresh` HttpOnly cookie). Add to each of those operations a short note and a 401 where relevant; e.g. for refresh:

```yaml
      description: >
        Rotates the session using the `inventra_refresh` HttpOnly cookie set at
        login. No request body. Responds 401 if the cookie is missing or invalid.
```
and for login/refresh document the cookie in the 200 response:
```yaml
          headers:
            Set-Cookie:
              schema: { type: string }
              description: Sets the `inventra_refresh` HttpOnly cookie.
```

(c) Delete the now-unused `RefreshRequest` and `LogoutRequest` schemas from `components.schemas` (and any `$ref` to them on the operations).

- [ ] **Step 2: Lint OpenAPI**

Run (from repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors. (If a removed schema is still `$ref`'d anywhere, fix the dangling ref.)

- [ ] **Step 3: Update the Bruno auth requests**

`docs/api/bruno/Auth/Login.bru` — keep the access-token capture, drop the refresh-token line in `script:post-response`:

```
script:post-response {
  if (res.status === 200) {
    bru.setEnvVar("accessToken", res.body.access_token);
  }
}
```

`docs/api/bruno/Auth/Refresh.bru` — change `body: json` to `body: none`, delete the `body:json { ... }` block, and drop the refresh-token capture (keep access-token capture). Bruno's cookie jar sends the `inventra_refresh` cookie automatically:

```
post {
  url: {{baseUrl}}/api/v1/auth/refresh
  body: none
  auth: none
}

script:post-response {
  if (res.status === 200) {
    bru.setEnvVar("accessToken", res.body.access_token);
  }
}
```

`docs/api/bruno/Auth/Logout.bru` — change `body: json` to `body: none` and delete the `body:json { ... }` block (keep the bearer auth):

```
post {
  url: {{baseUrl}}/api/v1/auth/logout
  body: none
  auth: bearer
}

auth:bearer {
  token: {{accessToken}}
}
```

- [ ] **Step 4: Update README**

In `docs/api/README.md`, update the line describing the Login step (currently "stores `accessToken` / `refreshToken` as env vars …") to:

```
4. Run **Auth › Login** — it stores `accessToken` as an env var; the refresh token is set as an `inventra_refresh` **HttpOnly cookie** that Bruno's cookie jar replays automatically on Refresh/Logout.
```

- [ ] **Step 5: Commit**

```bash
git add backend/api/openapi.yaml docs/api/bruno/Auth/Login.bru docs/api/bruno/Auth/Refresh.bru docs/api/bruno/Auth/Logout.bru docs/api/README.md
git commit -m "docs(api): refresh token via HttpOnly cookie (OpenAPI + Bruno + README)"
```

---

### Task 4: Frontend — credentials cookie flow

**Files:**
- Modify: `frontend/app/composables/useApiClient.ts` (`refreshToken` cookie flow; drop `useRefreshCookie` in 401 path)
- Modify: `frontend/app/composables/useAuthApi.ts` (login/logout credentials; drop `useRefreshCookie`)
- Modify: `frontend/app/plugins/auth.client.ts` (always attempt refresh)
- Delete: `frontend/app/composables/useRefreshCookie.ts`
- Test: `frontend/test/nuxt/auth-cookie.spec.ts`

**Interfaces:** none new (same composable surface; refresh token no longer in JS).

- [ ] **Step 1: Write the failing test**

Create `frontend/test/nuxt/auth-cookie.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { defineComponent } from 'vue'
import { useApiClient } from '~/composables/useApiClient'
import { useAuthApi } from '~/composables/useAuthApi'

const fetchMock = vi.fn((url: string) => {
  const u = String(url)
  if (u.includes('/auth/login') || u.includes('/auth/refresh')) return Promise.resolve({ access_token: 'acc' })
  if (u.includes('/auth/permissions')) return Promise.resolve({ permissions: [] })
  if (u.includes('/auth/logout')) return Promise.resolve({ status: 'logged_out' })
  return Promise.resolve({ id: '1', name: 'A', email: 'a@b.com', role_id: 'r' }) // /auth/me
})
vi.stubGlobal('$fetch', fetchMock)
mockNuxtImport('navigateTo', () => vi.fn())

const Harness = defineComponent({
  setup() {
    return { client: useApiClient(), authApi: useAuthApi() }
  },
  template: '<div />'
})

function callFor(part: string): [string, Record<string, unknown>] | undefined {
  const c = fetchMock.mock.calls.find(([u]) => String(u).includes(part))
  return c as [string, Record<string, unknown>] | undefined
}

describe('auth httpOnly cookie flow', () => {
  beforeEach(() => fetchMock.mockClear())

  it('refreshToken posts /auth/refresh with credentials include and no body', async () => {
    const w = await mountSuspended(Harness)
    const ok = await (w.vm as unknown as { client: ReturnType<typeof useApiClient> }).client.refreshToken()
    expect(ok).toBe(true)
    const call = callFor('/auth/refresh')
    expect(call).toBeTruthy()
    expect(call![1].credentials).toBe('include')
    expect(call![1].body).toBeUndefined()
  })

  it('login posts /auth/login with credentials include', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { authApi: ReturnType<typeof useAuthApi> }).authApi.login('a@b.com', 'pw')
    const call = callFor('/auth/login')
    expect(call).toBeTruthy()
    expect(call![1].credentials).toBe('include')
  })

  it('logout posts /auth/logout with credentials include', async () => {
    const w = await mountSuspended(Harness)
    await (w.vm as unknown as { authApi: ReturnType<typeof useAuthApi> }).authApi.logout()
    const call = callFor('/auth/logout')
    expect(call).toBeTruthy()
    expect(call![1].credentials).toBe('include')
  })
})
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && pnpm test -- auth-cookie`
Expected: FAIL — current code sends no `credentials` and refresh still uses a body. (If `mockNuxtImport`/`vi.stubGlobal` need ordering tweaks for this repo's setup, adjust the harness but keep the three behavioral assertions unchanged.)

- [ ] **Step 3: Update `useApiClient.ts`**

Replace `refreshToken` and remove the `useRefreshCookie` line from the 401 branch:

```ts
  async function refreshToken(): Promise<boolean> {
    try {
      const res = await $fetch<{ access_token: string }>(`${base}/auth/refresh`, {
        method: 'POST',
        credentials: 'include'
      })
      auth.setToken(res.access_token)
      return true
    } catch {
      return false
    }
  }
```
And in `request()`'s 401 handler, delete the line `useRefreshCookie().value = null` (keep `auth.clear()` and the redirect). Remove the now-unused `useRefreshCookie` import/usage if present.

- [ ] **Step 4: Update `useAuthApi.ts`**

Remove `const refreshCookie = useRefreshCookie()`. Change `login` and `logout`:

```ts
  async function login(email: string, password: string): Promise<void> {
    const res = await $fetch<{ access_token: string }>(`${base}/auth/login`, {
      method: 'POST',
      body: { email, password },
      credentials: 'include'
    })
    auth.setToken(res.access_token)
    await fetchMe()
  }
```
```ts
  async function logout(): Promise<void> {
    try {
      await client.request('/auth/logout', { method: 'POST', credentials: 'include' })
    } finally {
      auth.clear()
      await navigateTo('/login')
    }
  }
```

- [ ] **Step 5: Update the rehydration plugin**

Replace `frontend/app/plugins/auth.client.ts` with:

```ts
export default defineNuxtPlugin(async () => {
  const auth = useAuthStore()
  if (auth.isAuthenticated) return
  // The refresh token is an HttpOnly cookie JS cannot read, so attempt a refresh
  // unconditionally; a 401 simply means the user is not logged in.
  const authApi = useAuthApi()
  try {
    if (await authApi.refresh()) await authApi.fetchMe()
  } catch {
    // Stay logged out.
  }
})
```

- [ ] **Step 6: Delete the obsolete composable**

```bash
git rm frontend/app/composables/useRefreshCookie.ts
```
Then confirm nothing still imports it: `cd frontend && grep -rn "useRefreshCookie" app test` → expected: no matches.

- [ ] **Step 7: Run the new test, lint, typecheck, full test, build**

Run: `cd frontend && pnpm test -- auth-cookie`
Expected: PASS (3 tests).
Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green (the existing `useApiClient.spec.ts` X-Request-ID tests still pass — `request()` behavior is unchanged).

- [ ] **Step 8: Commit**

```bash
git add frontend/app/composables/useApiClient.ts frontend/app/composables/useAuthApi.ts frontend/app/plugins/auth.client.ts frontend/test/nuxt/auth-cookie.spec.ts
git commit -m "feat(security): SPA uses HttpOnly refresh cookie (credentials include); drop JS refresh storage"
```

---

## Verification (before merge — the critical risk)

Changing auth cookie handling is exactly the class of change that can pass unit tests yet break the real login (the CORS bug precedent). After all tasks:

- [ ] Backend gate green: `cd backend && go build ./... && go vet ./... && go test ./...` + Spectral lint clean.
- [ ] Frontend gate green: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`.
- [ ] **E2E login must be confirmed green end-to-end** against the real stack — push the branch and confirm the CI `e2e` job passes (it builds the SPA + runs the real backend over HTTP, so `Secure=false` in dev must let the cookie flow). If iterating locally, a browser login check (login → dashboard, then reload → still logged in via cookie refresh) is the equivalent evidence. Do NOT claim done until the e2e is green.

## Self-Review

**1. Spec coverage:**
- bagian 3 berkas: cookie helper (T1), handler/dto/router (T2), openapi/bruno/readme (T3 = spec bagian 6b), frontend + delete useRefreshCookie (T4). All covered.
- bagian 4 cookie attrs + env-gated Secure — T1 + T2 (router passes `Env=="production"`). bagian 5 handler login/refresh/logout — T2. bagian 6 frontend credentials + plugin — T4. bagian 6b consumers — T3. bagian 7 CORS (no change) — noted. bagian 8 testing — T1/T2/T4 + Verification (e2e). bagian 9 risks (Secure env-gated, SameSite, rehydration, no deps/migration) — covered.

**2. Placeholder scan:** No TBD/TODO; every code step has full code; commands have expected output. The two "if the test harness needs tweaks" notes (T4 Step 2) name a concrete fallback (registerEndpoint/mockNuxtImport) without changing assertions — not vague.

**3. Type consistency:** `setRefreshCookie(c, token, ttl, secure)` / `clearRefreshCookie(c, secure)` / `refreshCookieName` identical T1↔T2. `NewHandler(... secureCookie bool, refreshTTL time.Duration)` defined T2 and called in router T2 Step 5 with `d.Cfg.Env == "production"`, `d.Cfg.JWTRefreshTTL`. `tokenResponse` (no refresh) consistent T2↔T3 (OpenAPI mirrors it). Frontend `refreshToken()` returns `boolean`, `login`/`logout` signatures unchanged — consistent with the plugin's `authApi.refresh()` usage.
