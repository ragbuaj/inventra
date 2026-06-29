import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Master Data Pegawai — real backend (GET/POST/PUT/DELETE /api/v1/employees)
// The seeded admin (admin@inventra.local) has `masterdata.office.manage` and
// global data-scope, so the office picker is populated and the full list
// is visible.
//
// IMPORTANT: `pnpm test:e2e` requires the full backend stack + seeded admin
// (see CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
//
// Robustness notes:
//  - e2e rows PERSIST in the real backend. `code` (NIP) has a global partial-
//    unique index, so every run must use a unique NIP AND unique name.
//  - The table paginates, so a freshly-created row may land on a later page.
//    Always filter via the search box before asserting a created row is visible.
//  - Wait for the slideover to CLOSE (`toBeHidden`) before reopening Add or
//    making row-level assertions.
//  - USelect renders a custom popover (not a native <select>) — click the
//    trigger, then pick the option via role="option". NEVER use selectOption.
// ---------------------------------------------------------------------------

const SEARCH_PLACEHOLDER = 'Cari nama atau NIP…'

// ---------------------------------------------------------------------------
// Page load
// ---------------------------------------------------------------------------
test.describe('Master Data Pegawai — page load', () => {
  test('page renders heading and Add button for masterdata.office.manage holder', async ({ page }) => {
    await login(page)
    await page.goto('/master/employees')

    await expect(page.getByRole('heading', { name: 'Pegawai', exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('button', { name: 'Tambah Pegawai', exact: true })).toBeVisible({ timeout: 8_000 })
    await expect(page.getByPlaceholder(SEARCH_PLACEHOLDER, { exact: true })).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Create an employee — open form, fill NIP + Name, select Office via USelect
// picker (data-testid="employee-office-select"), submit; then filter by NIP
// and assert the new row appears with the resolved office name.
// ---------------------------------------------------------------------------
test.describe('Master Data Pegawai — create employee', () => {
  test('create an employee and assert the row appears after search', async ({ page }) => {
    // Per-run unique suffix: both NIP and name must be unique across all runs
    // because the backend persists rows (NIP has a partial-unique constraint).
    const s = `${Date.now()}`
    const empName = `E2E Pegawai ${s}`
    const empNip = `E2E${s}`

    await login(page)
    await page.goto('/master/employees')
    await expect(page.getByRole('heading', { name: 'Pegawai', exact: true })).toBeVisible({ timeout: 10_000 })

    // Open the create slideover.
    await page.getByRole('button', { name: 'Tambah Pegawai', exact: true }).click()
    await expect(page.getByText('Tambah Pegawai', { exact: true })).toBeVisible({ timeout: 8_000 })

    // Fill NIP (code field — mono input, first UInput inside the slideover).
    await page.getByPlaceholder('mis. 1990…', { exact: true }).fill(empNip)

    // Fill Name (Nama Lengkap field).
    await page.getByPlaceholder('mis. Andi Saputra', { exact: true }).fill(empName)

    // Select an Office via the USelect (required field).
    // data-testid="employee-office-select" is on the USelect wrapper; clicking
    // it opens the popover, then pick the first available option via role="option".
    const officeSelect = page.getByTestId('employee-office-select')
    await expect(officeSelect).toBeVisible({ timeout: 5_000 })
    await officeSelect.click()

    // Wait for the options popover to appear; pick the first option.
    // The seeded admin has global scope so at least one office exists.
    const firstOption = page.getByRole('option').first()
    await expect(firstOption).toBeVisible({ timeout: 8_000 })
    // Capture the selected office name to verify it appears in the table later.
    const officeLabelText = await firstOption.textContent()
    const officeLabel = officeLabelText?.trim() ?? ''
    await firstOption.click()

    // Submit the form.
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // Wait for the slideover to close before asserting the table.
    await expect(page.getByText('Tambah Pegawai', { exact: true })).toBeHidden({ timeout: 10_000 })

    // Filter the table by the unique NIP so the row is guaranteed to be visible
    // (the table paginates and a fresh row may land on a later page).
    const searchInput = page.getByPlaceholder(SEARCH_PLACEHOLDER, { exact: true })
    await searchInput.fill('')
    await searchInput.fill(empNip)

    // Assert the new row appears.
    await expect(page.getByText(empName, { exact: true })).toBeVisible({ timeout: 10_000 })

    // Assert the resolved office name appears in the same row (not raw UUID).
    if (officeLabel) {
      const empRow = page.locator('tr').filter({ hasText: empName })
      await expect(empRow).toBeVisible({ timeout: 8_000 })
      await expect(empRow.getByText(officeLabel, { exact: true })).toBeVisible()
    }
  })
})

// ---------------------------------------------------------------------------
// Filter bar — search input narrows the employee list
// ---------------------------------------------------------------------------
test.describe('Master Data Pegawai — search filter', () => {
  test('typing in the search input narrows the displayed rows', async ({ page }) => {
    await login(page)
    await page.goto('/master/employees')
    await expect(page.getByRole('heading', { name: 'Pegawai', exact: true })).toBeVisible({ timeout: 10_000 })

    const s = `${Date.now()}`
    const empName = `E2E Search ${s}`
    const empNip = `ES${s}`

    // Create a fresh employee so we have a known row to filter on.
    await page.getByRole('button', { name: 'Tambah Pegawai', exact: true }).click()
    await expect(page.getByText('Tambah Pegawai', { exact: true })).toBeVisible({ timeout: 8_000 })

    await page.getByPlaceholder('mis. 1990…', { exact: true }).fill(empNip)
    await page.getByPlaceholder('mis. Andi Saputra', { exact: true }).fill(empName)

    const officeSelect = page.getByTestId('employee-office-select')
    await expect(officeSelect).toBeVisible({ timeout: 5_000 })
    await officeSelect.click()
    const firstOption = page.getByRole('option').first()
    await expect(firstOption).toBeVisible({ timeout: 8_000 })
    await firstOption.click()

    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    await expect(page.getByText('Tambah Pegawai', { exact: true })).toBeHidden({ timeout: 10_000 })

    // Filter by the unique name — the row must appear.
    const searchInput = page.getByPlaceholder(SEARCH_PLACEHOLDER, { exact: true })
    await searchInput.fill(empName)
    await expect(page.getByText(empName, { exact: true })).toBeVisible({ timeout: 10_000 })

    // Reset button clears the filter (appears when anyFilterActive is true).
    await page.getByRole('button', { name: 'Reset', exact: true }).click()
    await expect(searchInput).toHaveValue('')
  })
})
