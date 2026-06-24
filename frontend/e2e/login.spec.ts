import { test, expect } from '@playwright/test'

// Credentials of the seeded superadmin (see CLAUDE.md `cmd/createadmin`).
// Override via env when the seed differs.
const EMAIL = process.env.E2E_EMAIL || 'admin@inventra.local'
const PASSWORD = process.env.E2E_PASSWORD || 'admin12345'

test.describe('Login (real backend)', () => {
  test('signs in with valid credentials and reaches the dashboard', async ({ page }) => {
    await page.goto('/login')

    await page.locator('input[type="email"]').fill(EMAIL)
    await page.locator('input[type="password"]').fill(PASSWORD)
    await page.getByRole('button', { name: 'Masuk' }).click()

    // On success the app redirects to the dashboard root.
    await expect(page).toHaveURL(/\/$/)
    await expect(page.getByRole('heading', { name: 'Dasbor' })).toBeVisible()
  })

  test('shows an inline error on invalid credentials', async ({ page }) => {
    await page.goto('/login')

    await page.locator('input[type="email"]').fill('wrong@example.com')
    await page.locator('input[type="password"]').fill('definitely-wrong')
    await page.getByRole('button', { name: 'Masuk' }).click()

    await expect(page.getByText('Email atau kata sandi tidak valid')).toBeVisible()
    // Must NOT navigate away from the login page on failure.
    await expect(page).toHaveURL(/\/login$/)
  })
})
