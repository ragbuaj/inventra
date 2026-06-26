import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Peta Lokasi (Office Map page)', () => {
  test('renders office list and filters by search', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    // Wait for the page to load (fakeLatency ~500ms)
    await expect(page.getByText('Peta Lokasi Kantor')).toBeVisible()
    await expect(page.getByText('Kantor Pusat')).toBeVisible()

    // Filter by search
    const searchInput = page.getByPlaceholder('Cari kantor / kode…')
    await searchInput.fill('Bekasi')
    await expect(page.getByText('Cabang Bekasi')).toBeVisible()
    await expect(page.getByText('Kantor Pusat')).not.toBeVisible()
  })

  test('selecting an office shows the detail card', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    await expect(page.getByText('Kantor Pusat')).toBeVisible()

    // Click the office row for Kantor Pusat
    await page.getByText('Kantor Pusat').first().click()

    // Detail card should appear with name, kode, and action buttons.
    // Scope to the detail card — the office code also renders in the list row.
    const detail = page.getByTestId('office-detail-card')
    await expect(detail).toBeVisible()
    await expect(detail.getByText('PST')).toBeVisible()
    await expect(detail.getByRole('link', { name: 'Lihat Kantor' })).toBeVisible()
    await expect(detail.getByRole('link', { name: 'Buka di Maps' })).toBeVisible()
  })

  test('shows empty state when search yields no results', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    await expect(page.getByText('Kantor Pusat')).toBeVisible()

    const searchInput = page.getByPlaceholder('Cari kantor / kode…')
    await searchInput.fill('xxxxxxnotfound')

    // Exact match — emptyListSub ("Tidak ada kantor cocok dengan filter.") also contains the phrase.
    await expect(page.getByText('Tidak ada kantor', { exact: true })).toBeVisible()
  })
})
