import { expect, request } from '@playwright/test'
import type { APIRequestContext, Page } from '@playwright/test'

// Credentials of the seeded superadmin (see CLAUDE.md `cmd/createadmin`).
// Override via env when the seed differs.
export const EMAIL = process.env.E2E_EMAIL || 'admin@inventra.local'
export const PASSWORD = process.env.E2E_PASSWORD || 'admin12345'

// Backend API base — mirrors the frontend's runtimeConfig.public.apiBase
// default (see CLAUDE.md); Playwright's request context talks to the
// backend directly, not through the Nuxt dev/preview server.
// MUST end with a trailing slash: APIRequestContext joins baseURL + path via
// the WHATWG URL() constructor, so a relative path (no leading slash, e.g.
// 'auth/login') only appends correctly onto a base that already ends in '/' —
// otherwise the last base path segment (`v1`) is replaced instead of kept,
// and a *leading*-slash path (e.g. '/auth/login') drops the whole `/api/v1`
// base path and hits the bare origin. Every call site below therefore also
// omits the leading slash on its path.
const API_BASE = `${(process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1').replace(/\/+$/, '')}/`

/** Sign in through the real backend and land on the dashboard. */
export async function login(page: Page): Promise<void> {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

/**
 * Drives an `AsyncSearchPicker` (`app/components/AsyncSearchPicker.vue`):
 * fills its search input, waits out the 300ms server-side debounce for a
 * matching result item to appear, then clicks it.
 *
 * `testid` is the picker's own `testid` prop (e.g. `office`, `employee`) —
 * the component derives `<testid>-picker-input` / `-picker-item` from it.
 * `term` is the search string (the backend matches name OR code); `matchText`
 * narrows the result list to the intended row (pass a unique, RUN-suffixed
 * label/name to avoid ambiguity against pre-existing dev-DB rows).
 *
 * No manual `waitForTimeout` is needed for the debounce — Playwright's
 * locator auto-waits/retries `click()` until a matching `-picker-item`
 * becomes actionable, which naturally spans the debounce + search round trip.
 *
 * Callers may invoke this immediately after a *different* Nuxt UI popover
 * (e.g. a `USelectMenu`) closes — a bare `.fill()` in that spot occasionally
 * lands before Vue settles the DOM from the prior popover close, swallowing
 * keystrokes (same root cause documented in `maintenance.spec.ts` around the
 * category → interval field transition, and worked around by reordering in
 * `assignment.spec.ts`'s Peminjaman test). The explicit `.click()` below
 * (focusing the real input first) mitigates that race here too.
 */
export async function pickAsync(page: Page, testid: string, term: string, matchText: string): Promise<void> {
  const input = page.getByTestId(`${testid}-picker-input`)
  await input.click()
  await input.fill(term)
  await page.getByTestId(`${testid}-picker-item`).filter({ hasText: matchText }).first().click()
}

// ---------------------------------------------------------------------------
// Authenticated API helper — used by specs that must mutate/restore SHARED
// backend config (e.g. RBAC data-scope / field-permission rows) directly via
// `/api/v1/authz`, bypassing the UI. Primarily for failure-safe `afterEach`
// cleanup: a test that mutates a shared seed row (like the Superadmin `*`
// data-scope policy) must revert it even when an assertion throws mid-test,
// and a UI-driven revert can't run once the test body has already failed.
// ---------------------------------------------------------------------------

/**
 * Logs in against the real backend (`POST /auth/login`) and returns a
 * Playwright `APIRequestContext` pre-configured with the resulting access
 * token as a Bearer header, so callers can `.get()`/`.put()` `/authz/...`
 * endpoints directly. Callers own the returned context and must `.dispose()`
 * it when done (e.g. in `afterEach`).
 */
export async function apiContext(): Promise<APIRequestContext> {
  const anon = await request.newContext({ baseURL: API_BASE })
  try {
    const res = await anon.post('auth/login', { data: { email: EMAIL, password: PASSWORD } })
    if (!res.ok()) {
      throw new Error(`apiContext: login failed with status ${res.status()}`)
    }
    const body = await res.json() as { access_token: string }
    return await request.newContext({
      baseURL: API_BASE,
      extraHTTPHeaders: { Authorization: `Bearer ${body.access_token}` }
    })
  } finally {
    await anon.dispose()
  }
}

/**
 * Lists roles exactly as the backend/frontend see them (`GET /authz/roles`,
 * `ORDER BY name` server-side — see `backend/db/queries/identity.sql`'s
 * `ListRoles`). The frontend's role-column grids (data-scope, field-permission)
 * render in this same order without re-sorting, so `data[0]` is genuinely the
 * "first role column" a positional UI locator would hit — which, on a dev DB
 * with any role sorting alphabetically before "Superadmin" (e.g. seeded
 * "Kepala Kanwil"/"Manager", or leftover "E2E ..." custom roles), is NOT
 * Superadmin.
 */
export async function listRoles(api: APIRequestContext): Promise<Array<{ id: string, name: string }>> {
  const res = await api.get('authz/roles')
  if (!res.ok()) throw new Error(`listRoles: GET /authz/roles failed with status ${res.status()}`)
  const body = await res.json() as { data: Array<{ id: string, name: string }> }
  return body.data
}

/** Resolves a role's id by its (exact) name via `GET /authz/roles` — never hardcode role UUIDs. */
export async function findRoleIdByName(api: APIRequestContext, roleName: string): Promise<string> {
  const roles = await listRoles(api)
  const role = roles.find(r => r.name === roleName)
  if (!role) throw new Error(`findRoleIdByName: role "${roleName}" not found`)
  return role.id
}

/**
 * Forces a role's Default (`module = "*"`) data-scope policy to `level`,
 * preserving any per-module overrides untouched. Idempotent — safe to call
 * even when the policy is already at `level` (e.g. because the in-body
 * revert already ran), and safe to call to heal a policy left corrupted by
 * an earlier interrupted run.
 */
export async function restoreDefaultScope(api: APIRequestContext, roleId: string, level = 'global'): Promise<void> {
  const getRes = await api.get(`authz/roles/${roleId}/scope`)
  if (!getRes.ok()) throw new Error(`restoreDefaultScope: GET scope failed with status ${getRes.status()}`)
  const { policies } = await getRes.json() as { policies: Array<{ module: string, scope_level: string }> }
  const overrides = policies.filter(p => p.module !== '*')
  const putRes = await api.put(`authz/roles/${roleId}/scope`, {
    data: { policies: [{ module: '*', scope_level: level }, ...overrides] }
  })
  if (!putRes.ok()) throw new Error(`restoreDefaultScope: PUT scope failed with status ${putRes.status()}`)
}

/**
 * Forces a role's field-permission cell for `(entity, field)` back to
 * default-allow (no explicit row — view+edit both true by the backend's
 * default-allow rule). Idempotent — removing an already-absent row is a
 * no-op PUT of the same row set.
 */
export async function restoreFieldPermissionDefault(
  api: APIRequestContext, roleId: string, entity: string, field: string
): Promise<void> {
  const getRes = await api.get(`authz/roles/${roleId}/fields`)
  if (!getRes.ok()) throw new Error(`restoreFieldPermissionDefault: GET fields failed with status ${getRes.status()}`)
  const { fields } = await getRes.json() as { fields: Array<{ entity: string, field: string, can_view: boolean, can_edit: boolean }> }
  const next = fields.filter(f => !(f.entity === entity && f.field === field))
  const putRes = await api.put(`authz/roles/${roleId}/fields`, { data: { fields: next } })
  if (!putRes.ok()) throw new Error(`restoreFieldPermissionDefault: PUT fields failed with status ${putRes.status()}`)
}
