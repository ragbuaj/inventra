import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'

const EMAIL = process.env.E2E_EMAIL || 'admin@inventra.local'
const PASSWORD = process.env.E2E_PASSWORD || 'admin12345'

async function login(page: Page) {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

test.describe('Master Data Kantor (mock-backed)', () => {
  test('creates an office and sees it in the tree', async ({ page }) => {
    await login(page)
    await page.goto('/master/offices')

    // The rebuilt Kantor screen is a split panel; the tree panel header is "Hierarki Kantor".
    await expect(page.getByText('Hierarki Kantor')).toBeVisible()
    // Seeded office is present in the tree.
    await expect(page.getByText('Kantor Pusat')).toBeVisible()

    // Open the create form and add a new office.
    await page.getByRole('button', { name: 'Tambah Kantor' }).click()
    await page.getByLabel('Nama Kantor').fill('Cabang E2E')
    await page.getByLabel('Kode').fill('E2E01')
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // New office appears in the tree.
    await expect(page.getByText('Cabang E2E')).toBeVisible()
  })
})
