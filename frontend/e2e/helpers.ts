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
