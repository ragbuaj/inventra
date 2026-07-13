import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, Page } from '@playwright/test'

// Credentials of the seeded superadmin (see CLAUDE.md `cmd/createadmin`).
const ADMIN = process.env.E2E_EMAIL || 'admin@inventra.local'
const PASSWORD_SEED = process.env.E2E_PASSWORD || 'admin12345'
// The password the full-flow test resets the admin account to.
const PASSWORD_RESET = 'admin12345new'

const MAILPIT = 'http://localhost:8025'
// Same convention as the other real-backend specs (dashboard.spec.ts,
// reports.spec.ts, ...): a raw APIRequestContext against the backend,
// independent of `helpers.ts`'s `apiContext()` — that helper assumes the
// seeded password already works, which is exactly what test 1 below breaks.
const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`

/**
 * Logs in trying the two passwords this spec knows about, in order: the
 * originally-seeded one, then the one the reset flow below sets it to.
 *
 * This spec mutates the shared admin account's password (via the real
 * forgot/reset flow), and runs against the same dev database across local
 * reruns. Without this fallback a second local run would find the password
 * already changed from a prior run and fail signing in. CI provisions a
 * fresh database every run, so there the seed password always matches and
 * the fallback branch never triggers.
 */
async function loginWithKnownPassword(page: Page): Promise<string> {
  for (const password of [PASSWORD_SEED, PASSWORD_RESET]) {
    await page.goto('/login')
    await page.locator('input[type="email"]').fill(ADMIN)
    await page.locator('input[type="password"]').fill(password)
    await page.getByRole('button', { name: 'Masuk', exact: true }).click()
    const signedIn = await page.waitForURL(/\/$/, { timeout: 5000 }).then(() => true).catch(() => false)
    if (signedIn) return password
  }
  throw new Error(`loginWithKnownPassword: no known password worked for ${ADMIN}`)
}

/** Pulls the reset link out of the newest Mailpit message (Text falls back to HTML). */
async function latestResetLink(request: APIRequestContext): Promise<string> {
  const res = await request.get(`${MAILPIT}/api/v1/message/latest`)
  const msg = await res.json() as { Text?: string, HTML?: string }
  const body = msg.Text || msg.HTML || ''
  const match = body.match(/\/reset-password\?token=[A-Za-z0-9_-]+/)
  if (!match) throw new Error(`reset link not found in email: ${body.slice(0, 200)}`)
  return match[0]
}

/**
 * Failure-safe restore of the shared seeded admin's password to
 * `PASSWORD_SEED`, run after every test in this file regardless of pass/fail.
 *
 * Test 1 permanently changes the admin password via the real reset flow.
 * `afterAll` doesn't receive the per-test `request` fixture, so this opens
 * its own `APIRequestContext` (mirroring the other specs' `login_`/`API_BASE`
 * pattern — see dashboard.spec.ts). In CI, the full e2e suite runs serially
 * (`workers: 1`) with one shared DB, in alphabetical file order — this spec
 * runs before reports/settings/stock-opname/transfers/etc., all of which log
 * in via `helpers.ts`'s `login()`, which hardcodes `PASSWORD_SEED` with no
 * fallback. Leaving the password changed breaks every sibling spec after
 * this one; this teardown is the primary protection against that (test 1's
 * `loginWithKnownPassword` fallback only helps *this* file's own reruns).
 *
 * Best-effort by design: any login/HTTP failure here is swallowed rather
 * than thrown, so a teardown hiccup never masks the real test result — but
 * it always attempts the restore first.
 */
test.afterAll(async () => {
  let api: APIRequestContext | undefined
  try {
    api = await request.newContext({ baseURL: API_BASE })

    // Already the seed password (e.g. test 1 never ran, or a prior teardown
    // already restored it) — nothing to do.
    const seedRes = await api.post('auth/login', { data: { email: ADMIN, password: PASSWORD_SEED } })
    if (seedRes.ok()) return

    // Password is (still) the reset value — log in with it and flip it back.
    const resetRes = await api.post('auth/login', { data: { email: ADMIN, password: PASSWORD_RESET } })
    if (!resetRes.ok()) return // neither known password works — nothing safe to do here

    const { access_token } = await resetRes.json() as { access_token: string }
    await api.put('auth/password', {
      headers: { Authorization: `Bearer ${access_token}` },
      data: { old_password: PASSWORD_RESET, new_password: PASSWORD_SEED }
    })
  } catch {
    // Swallow — teardown must never throw and mask the actual test outcome.
  } finally {
    await api?.dispose()
  }
})

test('forgot-password -> email link -> reset -> login with new password', async ({ page, request }) => {
  // Purge the mailbox so the /message/latest read below is deterministic.
  await request.delete(`${MAILPIT}/api/v1/messages`)

  await page.goto('/forgot-password')
  await page.getByTestId('forgot-email').fill(ADMIN)
  await page.getByTestId('forgot-submit').click()
  await expect(page.getByTestId('forgot-sent')).toBeVisible()

  // Wait for Mailpit to receive the reset email, then read the link out of it.
  await expect.poll(async () => {
    const res = await request.get(`${MAILPIT}/api/v1/messages`)
    return (await res.json() as { total: number }).total
  }, { timeout: 10000 }).toBeGreaterThan(0)
  const link = await latestResetLink(request)

  await page.goto(link)
  await page.getByTestId('reset-new').fill(PASSWORD_RESET)
  await page.getByTestId('reset-confirm').fill(PASSWORD_RESET)
  await page.getByTestId('reset-submit').click()
  await expect(page).toHaveURL(/\/login/)

  // Confirm the reset actually took effect by logging in with the new password.
  const activePassword = await loginWithKnownPassword(page)
  expect(activePassword).toBe(PASSWORD_RESET)
})

test('reset-password with an invalid token shows the error state', async ({ page }) => {
  await page.goto('/reset-password?token=garbage-token-value')
  await page.getByTestId('reset-new').fill('whatever12')
  await page.getByTestId('reset-confirm').fill('whatever12')
  await page.getByTestId('reset-submit').click()
  await expect(page.getByTestId('reset-error')).toBeVisible()
})
