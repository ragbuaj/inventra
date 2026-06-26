import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Settings cluster (mock-backed)', () => {
  test('User management lists seeded users', async ({ page }) => {
    await login(page)
    await page.goto('/settings/users')
    // Content-based: the table renders mock users (name also appears as linked employee → first()).
    await expect(page.getByText('Bambang Sukasno').first()).toBeVisible()
    await expect(page.getByText('Siti Aminah').first()).toBeVisible()
  })

  test('RBAC shows roles and a permission matrix', async ({ page }) => {
    await login(page)
    await page.goto('/settings/rbac')
    // Default-selected role + a permission label from the matrix.
    await expect(page.getByText('Superadmin').first()).toBeVisible()
    await expect(page.getByText('Lihat aset').first()).toBeVisible()
  })

  test('Audit trail screen loads', async ({ page }) => {
    await login(page)
    await page.goto('/settings/audit')
    await expect(page).toHaveURL(/\/settings\/audit$/)
    await expect(page.getByRole('heading', { name: 'Audit Trail' })).toBeVisible()
  })
})
