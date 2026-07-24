import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'

// ---------------------------------------------------------------------------
// Account security — real backend (Task 20). Covers the two email-verified
// self-service flows added in Tasks 18/19:
//
//   1. "Ganti Password" (Keamanan tab): verifies the current password, emails
//      a reset link (reuses the forgot-password flow), and completes on the
//      shared `/reset-password` page — same as password-reset.spec.ts.
//   2. "Ubah Email" (Profil tab): verifies the current password, emails a
//      verification link to the NEW address, and completes on `/verify-email`.
//
// Same Mailpit approach as password-reset.spec.ts: purge the mailbox so the
// `/message/latest` read is deterministic, trigger the email, poll
// `/api/v1/messages` until it arrives, then regex-extract the link.
//
// Shared-admin vs throwaway user (see task-20-report.md for the full writeup):
//  - Test 1 (password change) reuses the SHARED seeded admin, exactly like
//    password-reset.spec.ts — the revert is a single direct `PUT /auth/password`
//    call, no email round-trip needed, so the failure-safe afterAll is cheap
//    and reliable.
//  - Test 2 (email change) uses a DEDICATED throwaway user created via
//    `POST /users` (same pattern as assets.spec.ts's SoD "checker" user)
//    instead of the shared admin. Restoring an email change requires ANOTHER
//    full change-request → Mailpit → confirm round trip in teardown, which is
//    both slower and a second point of failure in a shared-DB serial suite
//    (CI runs `workers: 1`, alphabetical file order, 20+ sibling specs that
//    all log in via `helpers.ts`'s `login()`, which hardcodes
//    admin@inventra.local/admin12345 with no email fallback). A dedicated
//    user with a RUN-suffixed email sidesteps that risk entirely: nothing
//    about the shared admin account is ever touched by this test.
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + Mailpit + seeded
// admin (see CLAUDE.md). This spec compiles + lints here; CI runs it in the
// e2e job.
// ---------------------------------------------------------------------------

// Credentials of the seeded superadmin (see CLAUDE.md `cmd/createadmin`).
const ADMIN = process.env.E2E_EMAIL || 'admin@inventra.local'
const PASSWORD_SEED = process.env.E2E_PASSWORD || 'admin12345'
// The password the change-password flow test resets the admin account to.
const PASSWORD_CHANGED = 'admin12345chg'

const MAILPIT = 'http://localhost:8025'
const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`
const RUN = `${Date.now()}`

function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}

async function apiJson<T>(res: APIResponse): Promise<T> {
  if (!res.ok()) {
    throw new Error(`API call failed: ${res.status()} ${res.url()} — ${await res.text()}`)
  }
  return res.json() as Promise<T>
}

async function login_(api: APIRequestContext, email: string, password: string): Promise<string> {
  const res = await api.post('auth/login', { data: { email, password } })
  const body = await apiJson<{ access_token: string }>(res)
  return body.access_token
}

/** Signs in through the real backend UI and lands on the dashboard. */
async function loginAs(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login')
  await page.locator('input[name="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

/** Pulls a link matching `pattern` out of the newest Mailpit message (Text falls back to HTML). */
async function latestLink(api: APIRequestContext, pattern: RegExp): Promise<string> {
  const res = await api.get(`${MAILPIT}/api/v1/message/latest`)
  const msg = await res.json() as { Text?: string, HTML?: string }
  const body = msg.Text || msg.HTML || ''
  const match = body.match(pattern)
  if (!match) throw new Error(`link matching ${pattern} not found in email: ${body.slice(0, 200)}`)
  return match[0]
}

/** Waits for Mailpit's mailbox to receive at least one message. */
async function waitForMail(api: APIRequestContext): Promise<void> {
  await expect.poll(async () => {
    const res = await api.get(`${MAILPIT}/api/v1/messages`)
    return (await res.json() as { total: number }).total
  }, { timeout: 10000 }).toBeGreaterThan(0)
}

test.describe('Account security — real backend', () => {
  // Both tests purge + poll the shared Mailpit mailbox; serialize them so a
  // parallel local run (fullyParallel: true outside CI) can't interleave the
  // two purges and read the wrong message.
  test.describe.configure({ mode: 'serial' })

  let setupApi: APIRequestContext
  // Dedicated throwaway user for the email-change test — never the shared admin.
  let acctEmail: string
  let acctPassword: string
  // Dedicated throwaway user for the password-change test — never the shared admin.
  let pwdEmail: string
  let pwdPassword: string

  test.beforeAll(async () => {
    setupApi = await request.newContext({ baseURL: API_BASE })
    const adminToken = await login_(setupApi, ADMIN, PASSWORD_SEED)

    const rolesRes = await setupApi.get('authz/roles', { headers: authHeader(adminToken) })
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(rolesRes)
    const superadminRole = roles.data.find(r => r.name === 'Superadmin')
    if (!superadminRole) throw new Error('Superadmin role not found in GET /authz/roles')

    acctEmail = `e2e.acct.${RUN}@inventra.local`
    acctPassword = `Acct${RUN}!`
    const userRes = await setupApi.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Account ${RUN}`, email: acctEmail, password: acctPassword, role_id: superadminRole.id }
    })
    if (!userRes.ok()) throw new Error(`throwaway user create failed: ${userRes.status()} ${await userRes.text()}`)

    // A SECOND throwaway user, dedicated to the change-password test. Using the
    // shared seeded admin there left its password changed until this file's
    // afterAll restored it, and the very next spec (account.spec, which runs
    // immediately after alphabetically) raced that restore — a real, repeated
    // CI flake. A per-test user removes the cross-file coupling entirely. It is
    // separate from acctEmail because the change-email test rewrites that one's
    // address, which would break a password login here.
    pwdEmail = `e2e.pwd.${RUN}@inventra.local`
    pwdPassword = `Pwd${RUN}!`
    const pwdRes = await setupApi.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Password ${RUN}`, email: pwdEmail, password: pwdPassword, role_id: superadminRole.id }
    })
    if (!pwdRes.ok()) throw new Error(`password-test user create failed: ${pwdRes.status()} ${await pwdRes.text()}`)
  })

  /**
   * Safety net only. The tests in this file now mutate throwaway users
   * (pwdEmail / acctEmail), never the shared seeded admin, so on a healthy run
   * the first check below succeeds and this returns immediately.
   *
   * It is kept because a run interrupted BEFORE this change (or a stale DB from
   * an older revision) can still leave the admin on PASSWORD_CHANGED, and
   * `helpers.ts`'s `login()` hardcodes PASSWORD_SEED for every later spec.
   * Restoring via the direct `PUT /auth/password` endpoint needs no email
   * round-trip during teardown. See password-reset.spec.ts for the same pattern.
   */
  test.afterAll(async () => {
    await setupApi?.dispose()

    let api: APIRequestContext | undefined
    try {
      api = await request.newContext({ baseURL: API_BASE })

      // Already the seed password (test never ran, or a prior teardown
      // already restored it) — nothing to do.
      const seedRes = await api.post('auth/login', { data: { email: ADMIN, password: PASSWORD_SEED } })
      if (seedRes.ok()) return

      // Password is (still) the changed value — log in with it and flip it back.
      const changedRes = await api.post('auth/login', { data: { email: ADMIN, password: PASSWORD_CHANGED } })
      if (!changedRes.ok()) return // neither known password works — nothing safe to do here

      const { access_token } = await changedRes.json() as { access_token: string }
      await api.put('auth/password', {
        headers: authHeader(access_token),
        data: { old_password: PASSWORD_CHANGED, new_password: PASSWORD_SEED }
      })
    } catch {
      // Swallow — teardown must never throw and mask the actual test outcome.
    } finally {
      await api?.dispose()
    }
  })

  test('change-password: current password -> email link -> reset -> login with new password', async ({ page, request: req }) => {
    await req.delete(`${MAILPIT}/api/v1/messages`)

    // Uses its own throwaway user, so the shared seeded admin's password is never
    // in flux for the specs that run after this file.
    await loginAs(page, pwdEmail, pwdPassword)

    await page.goto('/account?tab=security')
    await expect(page.getByTestId('security-change-password')).toBeVisible()
    await page.getByTestId('security-change-password').click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await dialog.getByTestId('change-password-current').fill(pwdPassword)
    await dialog.getByRole('button', { name: 'Simpan', exact: true }).click()
    await expect(dialog.getByTestId('change-password-sent')).toBeVisible()

    await waitForMail(req)
    const link = await latestLink(req, /\/reset-password\?token=[A-Za-z0-9_-]+/)

    await page.goto(link)
    await page.getByTestId('reset-new').fill(PASSWORD_CHANGED)
    await page.getByTestId('reset-confirm').fill(PASSWORD_CHANGED)
    await page.getByTestId('reset-submit').click()
    await expect(page).toHaveURL(/\/login/)

    // Confirm the change actually took effect by logging in with the new password.
    await page.locator('input[name="email"]').fill(pwdEmail)
    await page.locator('input[type="password"]').fill(PASSWORD_CHANGED)
    await page.getByRole('button', { name: 'Masuk', exact: true }).click()
    await expect(page).toHaveURL(/\/$/)
  })

  test('change-email: current password -> verification link -> confirm -> new email is active', async ({ page, request: req }) => {
    await req.delete(`${MAILPIT}/api/v1/messages`)

    await loginAs(page, acctEmail, acctPassword)

    await page.goto('/account')
    await expect(page.getByTestId('profile-change-email')).toBeVisible()
    await page.getByTestId('profile-change-email').click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    const newEmail = `e2e.acct.new.${RUN}@inventra.local`
    await dialog.getByTestId('change-email-input').fill(newEmail)
    await dialog.getByTestId('change-email-password').fill(acctPassword)
    await dialog.getByRole('button', { name: 'Simpan', exact: true }).click()
    await expect(dialog.getByTestId('change-email-sent')).toBeVisible()

    await waitForMail(req)
    const link = await latestLink(req, /\/verify-email\?token=[A-Za-z0-9_-]+/)

    await page.goto(link)
    await expect(page.getByTestId('verify-email-success')).toBeVisible()

    // Confirm server-side that the account's email actually changed: the new
    // address now authenticates and GET /auth/me reflects it, while the old
    // address no longer does.
    const verifyApi = await request.newContext({ baseURL: API_BASE })
    try {
      const newLoginRes = await verifyApi.post('auth/login', { data: { email: newEmail, password: acctPassword } })
      expect(newLoginRes.ok(), `login with new email ${newEmail} should succeed`).toBe(true)
      const { access_token } = await apiJson<{ access_token: string }>(newLoginRes)

      const me = await apiJson<{ email: string }>(await verifyApi.get('auth/me', { headers: authHeader(access_token) }))
      expect(me.email).toBe(newEmail)

      const oldLoginRes = await verifyApi.post('auth/login', { data: { email: acctEmail, password: acctPassword } })
      expect(oldLoginRes.ok(), `login with old email ${acctEmail} should fail after the change`).toBe(false)
    } finally {
      await verifyApi.dispose()
    }
  })
})
