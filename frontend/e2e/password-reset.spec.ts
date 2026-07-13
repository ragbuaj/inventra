import { test, expect } from '@playwright/test'
import type { APIRequestContext, Page } from '@playwright/test'

// Credentials of the seeded superadmin (see CLAUDE.md `cmd/createadmin`).
const ADMIN = process.env.E2E_EMAIL || 'admin@inventra.local'
const PASSWORD_SEED = process.env.E2E_PASSWORD || 'admin12345'
// The password the full-flow test resets the admin account to.
const PASSWORD_RESET = 'admin12345new'

const MAILPIT = 'http://localhost:8025'

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
