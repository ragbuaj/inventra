import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Operasional cluster (mock-backed)', () => {
  // Penugasan is now wired to the real backend (see e2e/assignment.spec.ts) — the
  // former mock-backed "tabs switch to history" test was removed as superseded.

  test('Maintenance — due banner and schedule render', async ({ page }) => {
    await login(page)
    await page.goto('/maintenance')
    await expect(page.getByRole('heading', { name: 'Maintenance' })).toBeVisible()
    await expect(page.getByText('Maintenance jatuh tempo')).toBeVisible()
    await expect(page.getByText('Switch Cisco Catalyst 1000')).toBeVisible()
  })

  test('Laporan — applying the asset report shows KPIs and totals', async ({ page }) => {
    await login(page)
    await page.goto('/reports')
    await expect(page.getByRole('heading', { name: 'Laporan' })).toBeVisible()
    await page.getByRole('button', { name: 'Terapkan' }).click()
    await expect(page.getByText('Total Aset')).toBeVisible()
    await expect(page.getByText('Nilai Buku per Kategori')).toBeVisible()
  })
})
