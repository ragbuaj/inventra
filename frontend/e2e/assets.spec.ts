import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Assets — Katalog (mock-backed)', () => {
  test('lists seeded assets and filters by search', async ({ page }) => {
    await login(page)
    await page.goto('/assets')

    await expect(page.getByRole('heading', { name: 'Katalog Aset' })).toBeVisible()
    await expect(page.getByText('Laptop Dell Latitude 5440')).toBeVisible()

    // Search narrows the table to matching rows.
    await page.getByPlaceholder('Cari nama atau kode aset…').fill('Toyota')
    await expect(page.getByText('Toyota Hiace Commuter')).toBeVisible()
    await expect(page.getByText('Laptop Dell Latitude 5440')).toHaveCount(0)
  })

  test('opens an asset detail from the catalog', async ({ page }) => {
    await login(page)
    await page.goto('/assets')
    // The asset-tag cell is a link to the detail route.
    await page.getByRole('link', { name: 'JKT01-ELK-2026-00001' }).click()
    await expect(page).toHaveURL(/\/assets\/JKT01-ELK-2026-00001$/)
    await expect(page.getByText('Laptop Dell Latitude 5440').first()).toBeVisible()
  })
})
