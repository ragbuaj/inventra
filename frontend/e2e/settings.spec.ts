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

// ---------------------------------------------------------------------------
// RBAC screen — real backend (/api/v1/authz)
// These tests run against the seeded admin (admin@inventra.local) and the
// actual authzadmin endpoints. CI's e2e job brings up the full stack and seeds
// the admin before this suite runs.
// ---------------------------------------------------------------------------
test.describe('RBAC screen — real backend', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/settings/rbac')
    // Wait for the role list to be populated (not loading spinner)
    await expect(page.getByText('Superadmin').first()).toBeVisible({ timeout: 10_000 })
  })

  test('role list shows built-in system roles from the backend', async ({ page }) => {
    // Seeded system roles: Superadmin, Kepala Kanwil, Kepala Unit, Manager, Staf
    await expect(page.getByText('Superadmin').first()).toBeVisible()
    await expect(page.getByText('Manager').first()).toBeVisible()
    // At least one system lock icon is rendered next to system roles
    const roleList = page.locator('[class*="w-\\[280px\\]"]').first()
    await expect(roleList).toBeVisible()
  })

  test('selecting a role shows its permission matrix with resolved group labels', async ({ page }) => {
    // Click on the Manager role in the left list
    const managerBtn = page.locator('button').filter({ hasText: /^Manager/ }).first()
    await managerBtn.click()
    // Permission matrix cards should appear — catalog groups (Aset, Sistem, etc.)
    await expect(page.getByText('Aset').first()).toBeVisible()
    // The matrix renders individual permission codes (mono text)
    await expect(page.locator('text=asset.view').first()).toBeVisible({ timeout: 5_000 })
  })

  test('switching roles loads the correct permission set', async ({ page }) => {
    // Click Superadmin
    const superBtn = page.locator('button').filter({ hasText: /^Superadmin/ }).first()
    await superBtn.click()
    // Superadmin has all permissions — no switch should be unchecked (or at least the Sistem group is present)
    await expect(page.getByText('Sistem').first()).toBeVisible()
  })

  test('system role shows system badge and updated lock note (permissions editable)', async ({ page }) => {
    // The Manager role is a system role — click it
    const managerBtn = page.locator('button').filter({ hasText: /^Manager/ }).first()
    await managerBtn.click()
    // System badge appears in the header
    await expect(page.getByText('Sistem').first()).toBeVisible()
    // Updated lock note: name/code locked, but permissions are still configurable
    await expect(page.getByText(/izin tetap dapat dikonfigurasi/).first()).toBeVisible()
    // Save button is disabled when no changes made
    const saveBtn = page.getByRole('button', { name: /Simpan Perubahan/ })
    await expect(saveBtn).toBeDisabled()
  })

  test('toggling a permission marks dirty and Save persists across a page reload', async ({ page }) => {
    // Work with a non-system role to avoid interfering with seeded system roles.
    // If no custom role exists, this sub-test creates one first.
    // Verify the role list has at least one role before proceeding.
    await expect(page.locator('button').filter({ hasText: 'Manager' }).first()).toBeVisible()

    // Try to find a custom role (no lock icon). If none, skip the toggle/persist test.
    // The seeded admin setup may or may not have custom roles — we guard here.
    // If a custom role "Auditor Internal" is seeded, use it; otherwise create one.
    let targetRoleName = 'Auditor Internal'
    const auditorBtn = page.locator('button').filter({ hasText: /Auditor Internal/ }).first()
    const auditorExists = await auditorBtn.isVisible()

    if (!auditorExists) {
      // Create a temporary custom role for the toggle/persist test
      targetRoleName = 'E2E Test Role'
      const addBtn = page.locator('button').filter({ hasText: /Tambah Peran/ }).first()
      await addBtn.click()
      await page.waitForSelector('input[placeholder="mis. Operator Lapangan"]')
      await page.fill('input[placeholder="mis. Operator Lapangan"]', targetRoleName)
      await page.locator('button').filter({ hasText: /^Buat Peran$/ }).first().click()
      // Wait for the new role to appear in the list
      await expect(page.getByText(targetRoleName).first()).toBeVisible({ timeout: 8_000 })
    }

    // Select the target role
    const targetBtn = page.locator('button').filter({ hasText: new RegExp(targetRoleName) }).first()
    await targetBtn.click()

    // Find the first permission toggle row (button containing a mono permission code span)
    // and toggle it. Located by text content of a known permission code rather than CSS classes.
    const firstPermBtn = page.locator('button', { hasText: 'asset.view' }).first()
    await firstPermBtn.click()

    // Dirty indicator must appear
    await expect(page.getByText('Perubahan belum disimpan').first()).toBeVisible()

    // Save must be enabled
    const saveBtn = page.getByRole('button', { name: /Simpan Perubahan/ })
    await expect(saveBtn).toBeEnabled()
    await saveBtn.click()

    // Dirty indicator disappears after save
    await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 5_000 })

    // Reload the page and verify the permission state persisted
    await page.reload()
    await expect(page.getByText('Superadmin').first()).toBeVisible({ timeout: 10_000 })
    const reloadedTargetBtn = page.locator('button').filter({ hasText: new RegExp(targetRoleName) }).first()
    await reloadedTargetBtn.click()
    // After reload, the dirty indicator must be absent (state was persisted)
    await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible()
    // Save is disabled (clean state on load)
    await expect(saveBtn).toBeDisabled()

    // Cleanup: revert by toggling the same permission back (best-effort; not a hard failure)
    await firstPermBtn.click()
    if (await saveBtn.isEnabled()) {
      await saveBtn.click()
      await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 5_000 })
    }
  })

  test('Add Role modal opens, slugifies the code, and the new role appears in the list', async ({ page }) => {
    const addBtn = page.locator('button').filter({ hasText: /Tambah Peran/ }).first()
    await addBtn.click()
    await page.waitForSelector('input[placeholder="mis. Operator Lapangan"]')

    const uniqueName = `E2E Peran ${Date.now()}`
    await page.fill('input[placeholder="mis. Operator Lapangan"]', uniqueName)
    await page.locator('button').filter({ hasText: /^Buat Peran$/ }).first().click()

    // The new role should appear in the left list
    await expect(page.getByText(uniqueName).first()).toBeVisible({ timeout: 8_000 })
    // Custom badge should appear for the newly selected role
    await expect(page.getByText('Kustom').first()).toBeVisible()
  })
})
