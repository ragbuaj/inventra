import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Master Data Kategori Aset — real backend (GET /api/v1/categories/tree,
// POST/PUT/DELETE /api/v1/categories). The seeded admin
// (admin@inventra.local) has `masterdata.global.manage`.
//
// IMPORTANT: `pnpm test:e2e` requires the full backend stack + seeded admin
// (see CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
//
// Robustness notes:
//  - e2e rows PERSIST in the real backend. Category `code` has a global
//    partial-unique index, so every name AND code must be unique per test AND
//    per CI run (uniqueSuffix()), otherwise create returns 409 on later runs.
//  - The table paginates 7 rows/page sorted by name, so a freshly-created row
//    may land on a later page. We always filter via the search box (client-side
//    over the full tree() set) before asserting a created row is visible.
// ---------------------------------------------------------------------------

const SEARCH_PLACEHOLDER = 'Cari nama atau kode…'

// Unique suffix per call — disambiguates across tests in this file AND across
// repeated CI runs (Date.now() + an in-process counter).
let seq = 0
function uniqueSuffix(): string {
  seq += 1
  return `${Date.now()}${seq}`
}

async function openAddSlideover(page: import('@playwright/test').Page) {
  await page.getByRole('button', { name: 'Tambah Kategori', exact: true }).click()
  // Slideover title (i18n: masterdata.categories.createTitle)
  await expect(page.getByText('Tambah Kategori Aset', { exact: true })).toBeVisible({ timeout: 8_000 })
}

// Fill name + code and submit; wait for the slideover to close (a successful
// save closes it) so the next openAddSlideover is not blocked by a backdrop.
async function fillAndSubmit(page: import('@playwright/test').Page, name: string, code: string) {
  await page.getByLabel('Nama Kategori', { exact: true }).fill(name)
  await page.getByLabel('Kode', { exact: true }).fill(code)
  await page.getByRole('button', { name: 'Simpan', exact: true }).click()
  await expect(page.getByText('Tambah Kategori Aset', { exact: true })).toBeHidden({ timeout: 8_000 })
}

async function searchFor(page: import('@playwright/test').Page, term: string) {
  const input = page.getByPlaceholder(SEARCH_PLACEHOLDER, { exact: true })
  await input.fill('')
  await input.fill(term)
}

// ---------------------------------------------------------------------------
// Page load
// ---------------------------------------------------------------------------
test.describe('Master Data Kategori Aset — page load', () => {
  test('page renders title and filter bar for admin', async ({ page }) => {
    await login(page)
    await page.goto('/master/categories')

    await expect(page.getByRole('heading', { name: 'Kategori Aset', exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('button', { name: 'Tambah Kategori', exact: true })).toBeVisible({ timeout: 8_000 })
    await expect(page.getByPlaceholder(SEARCH_PLACEHOLDER, { exact: true })).toBeVisible()
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

    const s = uniqueSuffix()
    const name = `E2E Induk ${s}`

    await openAddSlideover(page)
    await fillAndSubmit(page, name, `EI${s}`)

    // Filter to the new row so pagination can't hide it.
    await searchFor(page, name)
    await expect(page.getByText(name, { exact: true })).toBeVisible({ timeout: 10_000 })
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

    const s = uniqueSuffix()
    const parentName = `E2E Induk ${s}`
    const childName = `E2E Sub ${s}`

    // --- Step 1: create the parent ---
    await openAddSlideover(page)
    await fillAndSubmit(page, parentName, `EI${s}`)

    // --- Step 2: create the child, selecting the parent via the picker ---
    await openAddSlideover(page)
    await page.getByLabel('Nama Kategori', { exact: true }).fill(childName)
    await page.getByLabel('Kode', { exact: true }).fill(`ES${s}`)

    // Open the parent USelect via its stable data-testid. NEVER use selectOption —
    // USelect renders a custom popover, not a native <select>.
    const parentSelect = page.getByTestId('category-parent-select')
    await expect(parentSelect).toBeVisible({ timeout: 5_000 })
    await parentSelect.click()

    // The dropdown renders options as role="option"; pick the parent we just created.
    await expect(page.getByRole('option', { name: parentName, exact: true })).toBeVisible({ timeout: 8_000 })
    await page.getByRole('option', { name: parentName, exact: true }).click()

    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    await expect(page.getByText('Tambah Kategori Aset', { exact: true })).toBeHidden({ timeout: 8_000 })

    // --- Step 3: filter to this run's rows (parent + child both contain the suffix)
    // so both render on a single page and the child sits directly under its parent. ---
    await searchFor(page, s)
    await expect(page.getByText(childName, { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 4: the child row is indented (the parent_id branch renders the
    // ps-6 wrapper + corner-down-right icon). Scope to the unique child row. ---
    const childRow = page.locator('tr').filter({ hasText: childName })
    await expect(childRow.locator('.ps-6')).toBeVisible()
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

    const s = uniqueSuffix()
    const name = `E2E Search ${s}`

    await openAddSlideover(page)
    await fillAndSubmit(page, name, `ES${s}`)

    // Filtering by the unique name shows it on the (single) filtered page.
    await searchFor(page, name)
    await expect(page.getByText(name, { exact: true })).toBeVisible({ timeout: 10_000 })

    // Reset clears the search input (i18n: common.reset → "Reset").
    await page.getByRole('button', { name: 'Reset', exact: true }).click()
    await expect(page.getByPlaceholder(SEARCH_PLACEHOLDER, { exact: true })).toHaveValue('')
  })
})
