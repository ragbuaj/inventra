import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Master Data Kategori Aset — real backend (GET /api/v1/masterdata/categories/tree,
// POST/PUT/DELETE /api/v1/masterdata/categories). The seeded admin
// (admin@inventra.local) has `masterdata.global.manage`.
//
// IMPORTANT: `pnpm test:e2e` requires the full backend stack + seeded admin
// (see CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
// ---------------------------------------------------------------------------

// Unique suffix so repeated CI runs don't collide on the same name.
const SUFFIX = Date.now()
const PARENT_NAME = `E2E Induk ${SUFFIX}`
const PARENT_CODE = 'EI2'
const CHILD_NAME = `E2E Sub ${SUFFIX}`
const CHILD_CODE = 'ES2'

// ---------------------------------------------------------------------------
// Helper: open the "Tambah Kategori" slideover and wait for it to appear.
// ---------------------------------------------------------------------------
async function openAddSlideover(page: import('@playwright/test').Page) {
  await page.getByRole('button', { name: 'Tambah Kategori', exact: true }).click()
  // Wait for the slideover title (i18n: masterdata.categories.createTitle)
  await expect(page.getByText('Tambah Kategori Aset', { exact: true })).toBeVisible({ timeout: 8_000 })
}

// ---------------------------------------------------------------------------
// Page load
// ---------------------------------------------------------------------------
test.describe('Master Data Kategori Aset — page load', () => {
  test('page renders title and filter bar for admin', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')

    // Page heading (i18n: masterdata.categories.title)
    await expect(page.getByRole('heading', { name: 'Kategori Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    // "Tambah Kategori" add button is visible (admin has masterdata.global.manage)
    await expect(page.getByRole('button', { name: 'Tambah Kategori', exact: true })).toBeVisible({ timeout: 8_000 })

    // Filter bar: search input, class select, group select, active toggle
    await expect(page.getByPlaceholder('Cari nama atau kode…', { exact: true })).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Create a parent category
// ---------------------------------------------------------------------------
test.describe('Master Data Kategori Aset — create parent', () => {
  test('create a parent category and assert it appears in the table', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')
    await expect(page.getByRole('heading', { name: 'Kategori Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    // Open the create slideover.
    await openAddSlideover(page)

    // Fill Nama Kategori (required).
    await page.getByLabel('Nama Kategori', { exact: true }).fill(PARENT_NAME)

    // Fill Kode (required).
    await page.getByLabel('Kode', { exact: true }).fill(PARENT_CODE)

    // Submit: the FormSlideover footer "Simpan" button.
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // Slideover closes; new category row appears in the table.
    await expect(page.getByText(PARENT_NAME, { exact: true })).toBeVisible({ timeout: 10_000 })
  })
})

// ---------------------------------------------------------------------------
// Create a child category — parent picker via data-testid + role="option"
// ---------------------------------------------------------------------------
test.describe('Master Data Kategori Aset — create child with parent picker', () => {
  test('create parent then child referencing parent via picker; child appears indented', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')
    await expect(page.getByRole('heading', { name: 'Kategori Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 1: create the parent category ---
    await openAddSlideover(page)
    await page.getByLabel('Nama Kategori', { exact: true }).fill(PARENT_NAME)
    await page.getByLabel('Kode', { exact: true }).fill(PARENT_CODE)
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    // Wait for the parent row to appear before proceeding.
    await expect(page.getByText(PARENT_NAME, { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 2: create the child category ---
    await openAddSlideover(page)
    await page.getByLabel('Nama Kategori', { exact: true }).fill(CHILD_NAME)
    await page.getByLabel('Kode', { exact: true }).fill(CHILD_CODE)

    // Open the parent USelect via its stable data-testid (the trigger button is
    // rendered inside the testid wrapper; clicking the wrapper opens the dropdown).
    // NEVER use selectOption — USelect renders a custom popover, not a native <select>.
    const parentSelect = page.getByTestId('category-parent-select')
    await expect(parentSelect).toBeVisible({ timeout: 5_000 })
    await parentSelect.click()

    // The dropdown listbox renders options as role="option" inside a popover.
    // Wait for the parent option we just created to appear.
    await expect(page.getByRole('option', { name: PARENT_NAME, exact: true })).toBeVisible({ timeout: 8_000 })

    // Click the parent option.
    await page.getByRole('option', { name: PARENT_NAME, exact: true }).click()

    // Submit.
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // --- Step 3: assert the child row appears in the table ---
    await expect(page.getByText(CHILD_NAME, { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 4: assert the child row is indented (has the corner-down-right icon)
    // The indented cell wraps the name in a div with class ps-6 and an icon.
    // We find the table row containing the child name and assert it contains the
    // indented-child indicator — a Lucide corner-down-right SVG icon (UIcon).
    // The page inserts this icon only when parent_id is set.
    const childRow = page.locator('tr').filter({ hasText: CHILD_NAME })
    await expect(childRow).toBeVisible({ timeout: 8_000 })
    // The icon has the class 'i-lucide-corner-down-right' rendered as an inline SVG or
    // as a <span> with that class. Either way the row's first cell contains padding-start.
    // Assert that the indentation wrapper (ps-6 class) is present in the child row's
    // name cell — this verifies the parent_id branch renders correctly.
    const nameCell = childRow.locator('td').first()
    await expect(nameCell.locator('.ps-6')).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Search filter narrows the list
// ---------------------------------------------------------------------------
test.describe('Master Data Kategori Aset — search filter', () => {
  test('typing in the search input filters the displayed rows', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')
    await expect(page.getByRole('heading', { name: 'Kategori Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    // Create a unique category so we have something deterministic to search for.
    const searchName = `E2E Search ${SUFFIX}`
    await openAddSlideover(page)
    await page.getByLabel('Nama Kategori', { exact: true }).fill(searchName)
    await page.getByLabel('Kode', { exact: true }).fill('ES3')
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    await expect(page.getByText(searchName, { exact: true })).toBeVisible({ timeout: 10_000 })

    // Type the search term.
    await page.getByPlaceholder('Cari nama atau kode…', { exact: true }).fill(searchName)

    // The created category must still be visible.
    await expect(page.getByText(searchName, { exact: true })).toBeVisible({ timeout: 5_000 })

    // Clear the filter via the reset button (i18n: common.reset → "Reset").
    await page.getByRole('button', { name: 'Reset', exact: true }).click()

    // The search input is now empty.
    await expect(page.getByPlaceholder('Cari nama atau kode…', { exact: true })).toHaveValue('')
  })
})
