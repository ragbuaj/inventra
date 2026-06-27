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

test.describe('Master Data Kategori Aset (mock-backed)', () => {
  test('lists seeded categories and creates a new one', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')

    // Seeded categories are visible.
    await expect(page.getByText('Komputer & Laptop')).toBeVisible()
    await expect(page.getByText('Kendaraan Bermotor')).toBeVisible()

    // Open the create slideover and add a category.
    await page.getByRole('button', { name: 'Tambah Kategori' }).click()
    await page.getByLabel('Nama Kategori').fill('Genset E2E')
    await page.getByLabel('Kode').fill('GEN')
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // New category appears in the table.
    await expect(page.getByText('Genset E2E')).toBeVisible()
  })

  test('search narrows the list', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')
    await page.getByPlaceholder('Cari nama atau kode…').fill('Kendaraan')
    await expect(page.getByText('Kendaraan Bermotor')).toBeVisible()
    await expect(page.getByText('Komputer & Laptop')).toHaveCount(0)
  })
})
