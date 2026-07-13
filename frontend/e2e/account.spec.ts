import { test, expect } from '@playwright/test'
import { login } from './helpers'

// NOTE: the Keamanan tab's password change used to be an inline 3-field
// (old/new/confirm) form; Task 19 replaced it with a "verify current
// password -> emailed reset link" modal (see account.vue's "Ganti Password"
// modal). This is a lightweight smoke check that the entry point still opens
// that modal; the full email round trip (Mailpit -> /reset-password -> login
// with the new password) is covered by account-security.spec.ts.
test('account: open from user menu and Keamanan tab opens the change-password modal', async ({ page }) => {
  await login(page)
  await page.goto('/account')
  await expect(page.getByRole('button', { name: 'Keamanan' })).toBeVisible()
  await page.getByRole('button', { name: 'Keamanan' }).click()
  await expect(page.getByTestId('security-change-password')).toBeVisible()
  await page.getByTestId('security-change-password').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByTestId('change-password-current')).toBeVisible()
})

test('account: switch language preference', async ({ page }) => {
  await login(page)
  await page.goto('/account?tab=preferences')
  await page.getByRole('button', { name: 'English' }).click()
  await expect(page.getByText('Appearance')).toBeVisible()
})
