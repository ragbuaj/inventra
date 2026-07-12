import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'

// Credentials of the seeded superadmin (see CLAUDE.md `cmd/createadmin`).
// Override via env when the seed differs.
export const EMAIL = process.env.E2E_EMAIL || 'admin@inventra.local'
export const PASSWORD = process.env.E2E_PASSWORD || 'admin12345'

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
