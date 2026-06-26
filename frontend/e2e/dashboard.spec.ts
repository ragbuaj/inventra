import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Dashboard + app shell (mock-backed)', () => {
  test('renders the dashboard heading and the sidebar nav groups', async ({ page }) => {
    await login(page)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible()
    // App shell sidebar: both group labels are present for a superadmin.
    await expect(page.getByText('Operasional', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('Administrasi', { exact: true }).first()).toBeVisible()
  })

  test('navigates to a built screen via the sidebar', async ({ page }) => {
    await login(page)
    // The Operasional → Laporan item links to /reports.
    await page.getByRole('link', { name: 'Laporan' }).click()
    await expect(page).toHaveURL(/\/reports$/)
    await expect(page.getByRole('heading', { name: 'Laporan' })).toBeVisible()
  })
})
