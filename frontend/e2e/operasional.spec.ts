import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Operasional cluster (mock-backed)', () => {
  // Penugasan is now wired to the real backend (see e2e/assignment.spec.ts) — the
  // former mock-backed "tabs switch to history" test was removed as superseded.
  // Maintenance is now wired to the real backend too (see e2e/maintenance.spec.ts) —
  // the former mock-backed "due banner and schedule render" test was removed as
  // superseded (a fresh database has no schedules, so no banner renders).

  test('Laporan — applying the asset report shows KPIs and totals', async ({ page }) => {
    await login(page)
    await page.goto('/reports')
    await expect(page.getByRole('heading', { name: 'Laporan' })).toBeVisible()
    await page.getByRole('button', { name: 'Terapkan' }).click()
    await expect(page.getByText('Total Aset')).toBeVisible()
    await expect(page.getByText('Nilai Buku per Kategori')).toBeVisible()
  })
})
