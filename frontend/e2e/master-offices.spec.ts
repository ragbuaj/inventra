import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Master Data Kantor — real backend (offices/floors/rooms wired to
// /api/v1/offices, /api/v1/floors, /api/v1/rooms). The seeded admin
// (admin@inventra.local) has global scope + masterdata.{global,office}.manage.
//
// A fresh e2e DB has NO office types seeded, and `office_type_id` is REQUIRED
// to create an office. So the create flow first creates an office type via the
// (already-wired) Referensi screen, then creates a root office selecting it.
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + seeded admin
// (see CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
//
// Robustness rules: no `selectOption` (USelect is a custom popover), no
// `.first()`/`.last()` on broad queries, no `isVisible()` booleans driving
// control flow, unique names per run (rows persist), assert via role-scoped
// locators. USelect options render as role="option" in a teleported popover.
// ---------------------------------------------------------------------------

test.describe('Master Data Kantor — page load', () => {
  test('renders the tree panel header, Add button, and search input', async ({ page }) => {
    await login(page)
    await page.goto('/master/offices')

    await expect(page.getByText('Hierarki Kantor', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('button', { name: 'Tambah Kantor', exact: true })).toBeVisible({ timeout: 8_000 })
    await expect(page.getByPlaceholder('Cari kantor…', { exact: true })).toBeVisible()
  })
})

test.describe('Master Data Kantor — create office', () => {
  const s = `${Date.now()}`
  const typeName = `E2E Type ${s}`
  const officeName = `E2E Kantor ${s}`
  const officeCode = `E2E${s}`

  test('creates an office type then a root office selecting it', async ({ page }) => {
    await login(page)

    // --- Step 1: create an office type on the Referensi screen (default resource). ---
    await page.goto('/master/reference')
    await expect(page.getByTestId('reference-panel-title')).toBeVisible({ timeout: 10_000 })
    await page.getByTestId('ref-nav-office-types').click()
    await expect(page.getByRole('heading', { name: 'Jenis Kantor', exact: true })).toBeVisible({ timeout: 8_000 })

    await page.getByRole('button', { name: 'Tambah', exact: true }).click()
    await expect(page.getByText('Tambah Data', { exact: true })).toBeVisible({ timeout: 5_000 })
    await page.getByLabel('Nama', { exact: true }).fill(typeName)
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    await expect(page.getByText(typeName, { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 2: create a root office selecting that type. ---
    await page.goto('/master/offices')
    await expect(page.getByText('Hierarki Kantor', { exact: true })).toBeVisible({ timeout: 10_000 })

    await page.getByRole('button', { name: 'Tambah Kantor', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible({ timeout: 8_000 })

    await dialog.getByLabel('Nama Kantor', { exact: true }).fill(officeName)
    await dialog.getByLabel('Kode', { exact: true }).fill(officeCode)

    // Pick the office type via the USelect trigger (data-testid) → role="option".
    await dialog.getByTestId('office-type-select').click()
    await expect(page.getByRole('option', { name: typeName, exact: true })).toBeVisible({ timeout: 8_000 })
    await page.getByRole('option', { name: typeName, exact: true }).click()

    await dialog.getByRole('button', { name: 'Simpan', exact: true }).click()

    // The slideover closes and the newly-created office is auto-selected — its
    // name renders as the detail-panel heading (unique <h1>).
    await expect(dialog).toBeHidden({ timeout: 8_000 })
    await expect(page.getByRole('heading', { name: officeName, exact: true })).toBeVisible({ timeout: 10_000 })
    // The resolved office-type name appears in the detail info card (FK id → name
    // resolution). Scoped to the stable per-field testid — the name also renders in
    // the header type badge, so a plain getByText would strict-mode-violate.
    await expect(page.getByTestId('office-detail-type')).toHaveText(typeName, { timeout: 8_000 })
  })
})
