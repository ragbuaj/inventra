import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Master Data Pegawai — smoke tests against the real backend
//
// SCOPE: page-load + form-render only.
//
// WHY no "create employee" flow: the e2e job's backend DB has NO offices
// seeded (the Kantor screen is still mock; nothing inserts masterdata.offices,
// and seeding one requires an office_type chain). The employee form's office
// picker is REQUIRED but will be empty, so a full "pick office + submit"
// flow cannot pass in CI. Full create/edit coverage is handled by the
// 26-test component spec `frontend/test/nuxt/master-employees.spec.ts`.
//
// IMPORTANT: `pnpm test:e2e` requires the full backend stack + seeded admin
// (see CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
//
// Robustness rules applied throughout:
//  - No `selectOption` (USelect is a custom popover, not native <select>).
//  - No `.first()` / `.last()` on broad page-wide queries.
//  - No `isVisible()` / `isEnabled()` snapshot booleans driving control flow.
//  - No silent `if (...) { assert }` that silently skips assertions.
//  - No `getByText` with `{ exact: false }` that could multi-match.
//  - Dialog assertions scoped to `page.getByRole('dialog')` to avoid
//    page-vs-slideover ambiguity (FormSlideover uses USlideover which
//    renders with role="dialog").
//  - Everything auto-waits via `expect(...).toBeVisible()`.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Page load
// ---------------------------------------------------------------------------
test.describe('Master Data Pegawai — page load', () => {
  test('renders heading, Add button, and search input', async ({ page }) => {
    await login(page)
    await page.goto('/master/employees')

    await expect(
      page.getByRole('heading', { name: 'Pegawai', exact: true })
    ).toBeVisible({ timeout: 10_000 })

    await expect(
      page.getByRole('button', { name: 'Tambah Pegawai', exact: true })
    ).toBeVisible({ timeout: 8_000 })

    await expect(
      page.getByPlaceholder('Cari nama atau NIP…', { exact: true })
    ).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Open create form → form renders
// ---------------------------------------------------------------------------
test.describe('Master Data Pegawai — create form render', () => {
  test('clicking Add opens the slideover and all form fields are visible', async ({ page }) => {
    await login(page)
    await page.goto('/master/employees')
    await expect(
      page.getByRole('heading', { name: 'Pegawai', exact: true })
    ).toBeVisible({ timeout: 10_000 })

    // Click the Add button to open the slideover.
    await page.getByRole('button', { name: 'Tambah Pegawai', exact: true }).click()

    // Scope all assertions to the dialog to avoid page-vs-slideover ambiguity.
    // FormSlideover wraps USlideover which renders role="dialog".
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible({ timeout: 8_000 })

    // Assert individual form fields inside the dialog.
    // UFormField renders a <label> matching getByLabel.
    await expect(
      dialog.getByLabel('Nama Lengkap', { exact: true })
    ).toBeVisible()

    await expect(
      dialog.getByLabel('NIP', { exact: true })
    ).toBeVisible()

    // Office picker (required — will be empty in CI; we only assert it renders).
    await expect(
      dialog.getByTestId('employee-office-select')
    ).toBeVisible()

    // Department and position pickers.
    await expect(
      dialog.getByTestId('employee-dept-select')
    ).toBeVisible()

    await expect(
      dialog.getByTestId('employee-position-select')
    ).toBeVisible()

    // Scope note hint text rendered below the office picker.
    await expect(
      dialog.getByText('Pilihan kantor dibatasi sesuai lingkup peran Anda.', { exact: true })
    ).toBeVisible()
  })
})
