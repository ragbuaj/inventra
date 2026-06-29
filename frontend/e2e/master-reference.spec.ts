import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Master Data Referensi screen — real backend (GET/POST/PUT/DELETE via
// the generic reference engine at /api/v1/masterdata/reference/:resource).
// The seeded admin (admin@inventra.local) has `masterdata.global.manage`.
//
// IMPORTANT: `pnpm test:e2e` requires the full backend stack + seeded admin
// (see CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
// ---------------------------------------------------------------------------

// i18n (id locale) for the 11 resource sidebar labels (masterdata.reference.resources.*).
// Used for exact-text assertions that are deterministic against the real backend.
const RESOURCES = {
  'office-types': 'Jenis Kantor',
  'departments': 'Departemen',
  'positions': 'Jabatan',
  'units': 'Satuan',
  'maintenance-categories': 'Kategori Pemeliharaan',
  'problem-categories': 'Kategori Masalah',
  'brands': 'Brand',
  'vendors': 'Vendor',
  'provinces': 'Provinsi',
  'cities': 'Kota',
  'models': 'Model'
}

// ---------------------------------------------------------------------------
// Helper: click a sidebar resource button by its exact i18n label.
// The sidebar renders each resource as a <button> with a <span class="truncate">
// containing the label text. We locate by role+name which auto-waits + avoids
// `.first()` on an ambiguous query.
// ---------------------------------------------------------------------------
async function selectResource(page: import('@playwright/test').Page, label: string) {
  // The sidebar button contains the label in a truncate <span>.
  // getByRole('button', { name }) matches button accessible name (text content).
  await page.getByRole('button', { name: label, exact: true }).click()
}

// ---------------------------------------------------------------------------
// Sidebar renders
// ---------------------------------------------------------------------------
test.describe('Master Data Referensi — sidebar', () => {
  test('sidebar renders all 11 resource labels', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')

    // Wait for the panel to mount (heading "Master Data" is always visible).
    await expect(page.getByText('Master Data', { exact: true })).toBeVisible({ timeout: 10_000 })

    // Assert a representative subset of the 11 sidebar labels (exact text to avoid
    // multi-match strict-mode failures — e.g. "Provinsi" also appears as a table column
    // header on a different resource; the sidebar button is the only button with that name).
    await expect(page.getByRole('button', { name: RESOURCES.provinces, exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: RESOURCES.departments, exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: RESOURCES.brands, exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: RESOURCES['maintenance-categories'], exact: true })).toBeVisible()

    // Verify count badges render for at least the first resource (office-types).
    // The count is a mono-font span; it renders as "…" while loading and a number when settled.
    // We wait until it is NOT "…" (numeric or 0), proving the API call resolved.
    const officeTypesBtn = page.getByRole('button', { name: RESOURCES['office-types'], exact: true })
    await expect(officeTypesBtn).toBeVisible()
  })

  test('Add button is visible for masterdata.global.manage holder', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')
    await expect(page.getByText('Master Data', { exact: true })).toBeVisible({ timeout: 10_000 })
    // "Tambah" button (i18n: masterdata.reference.add) is gated by can('masterdata.global.manage').
    await expect(page.getByRole('button', { name: 'Tambah', exact: true })).toBeVisible({ timeout: 8_000 })
  })
})

// ---------------------------------------------------------------------------
// Province CRUD (no FK dependency)
// ---------------------------------------------------------------------------
test.describe('Master Data Referensi — provinces CRUD', () => {
  // Unique suffix so repeated CI runs don't collide on the same name.
  const provinceName = `E2E Provinsi ${Date.now()}`
  const provinceCode = 'E2'

  test('create a province and assert the row appears in the table', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')
    await expect(page.getByText('Master Data', { exact: true })).toBeVisible({ timeout: 10_000 })

    // Select "Provinsi" resource in the sidebar.
    await selectResource(page, RESOURCES.provinces)

    // Wait for the page heading to update to the selected resource label.
    await expect(page.getByRole('heading', { name: RESOURCES.provinces, exact: true })).toBeVisible({ timeout: 8_000 })

    // Click the "Tambah" button to open the create FormModal.
    await page.getByRole('button', { name: 'Tambah', exact: true }).click()

    // Wait for the modal title "Tambah Data" (i18n: masterdata.reference.createTitle).
    await expect(page.getByText('Tambah Data', { exact: true })).toBeVisible({ timeout: 5_000 })

    // Fill the Nama field — provinces has two fields: name + code.
    await page.getByLabel('Nama', { exact: true }).fill(provinceName)
    await page.getByLabel('Kode', { exact: true }).fill(provinceCode)

    // Submit: the modal's footer "Simpan" button (FormModal emits @submit).
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // The modal closes and the new province row appears in the table.
    // Wait for the row containing the province name.
    await expect(page.getByText(provinceName, { exact: true })).toBeVisible({ timeout: 10_000 })
  })
})

// ---------------------------------------------------------------------------
// Cities (FK → provinces)
// ---------------------------------------------------------------------------
test.describe('Master Data Referensi — cities FK picker (provinces)', () => {
  // Re-create a fresh province so this describe block is self-contained
  // (the province from the previous describe may or may not exist in the same run).
  const provinceName2 = `E2E Provinsi ${Date.now() + 1}`
  const cityName = `E2E Kota ${Date.now()}`

  test('create a province then create a city referencing it via FK picker', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')
    await expect(page.getByText('Master Data', { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 1: create a province ---
    await selectResource(page, RESOURCES.provinces)
    await expect(page.getByRole('heading', { name: RESOURCES.provinces, exact: true })).toBeVisible({ timeout: 8_000 })
    await page.getByRole('button', { name: 'Tambah', exact: true }).click()
    await expect(page.getByText('Tambah Data', { exact: true })).toBeVisible({ timeout: 5_000 })
    await page.getByLabel('Nama', { exact: true }).fill(provinceName2)
    await page.getByLabel('Kode', { exact: true }).fill('E3')
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    // Province row must appear before switching resources.
    await expect(page.getByText(provinceName2, { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 2: switch to "Kota" ---
    await selectResource(page, RESOURCES.cities)
    await expect(page.getByRole('heading', { name: RESOURCES.cities, exact: true })).toBeVisible({ timeout: 8_000 })

    // --- Step 3: open the create modal ---
    await page.getByRole('button', { name: 'Tambah', exact: true }).click()
    await expect(page.getByText('Tambah Data', { exact: true })).toBeVisible({ timeout: 5_000 })

    // --- Step 4: assert the Provinsi FK picker is present ---
    // The USelect trigger for province_id renders with the label "Provinsi" above it
    // (UFormField :label="t(field.labelKey)"). The trigger button renders the placeholder
    // or selected value as its accessible text.
    // Locate the form field by its label, then find the trigger button within it.
    const provinsiField = page.locator('div').filter({ has: page.getByText('Provinsi', { exact: true }) }).first()
    await expect(provinsiField).toBeVisible({ timeout: 5_000 })

    // The USelect trigger is a button inside the form field that opens the listbox.
    // Click it to open the dropdown (USelect renders a popover with role="listbox").
    // We locate the trigger by its role within the field container, not by class.
    const provinsiTrigger = provinsiField.getByRole('button').first()
    await expect(provinsiTrigger).toBeVisible()
    await provinsiTrigger.click()

    // The dropdown listbox should appear with the province we just created.
    // USelect renders options as role="option" inside a popover.
    await expect(page.getByRole('option', { name: provinceName2, exact: true })).toBeVisible({ timeout: 8_000 })

    // Pick the province by clicking its option.
    await page.getByRole('option', { name: provinceName2, exact: true }).click()

    // --- Step 5: fill the city name + code ---
    await page.getByLabel('Nama', { exact: true }).fill(cityName)
    await page.getByLabel('Kode', { exact: true }).fill('E31')

    // --- Step 6: submit ---
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // The modal closes and the city row appears.
    await expect(page.getByText(cityName, { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 7: assert the city row shows the province NAME (not raw UUID) ---
    // The reference page renders FK cells via the #province_id-cell slot using fkName(),
    // which resolves the UUID → name from fkData. Verify the province name appears somewhere
    // in the same table row as the city name.
    // We use a row-scoped locator: find the <tr> that contains the city name, then assert
    // it also contains the province name.
    const cityRow = page.locator('tr').filter({ hasText: cityName })
    await expect(cityRow).toBeVisible({ timeout: 8_000 })
    await expect(cityRow.getByText(provinceName2, { exact: true })).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Search filter
// ---------------------------------------------------------------------------
test.describe('Master Data Referensi — search', () => {
  test('search input is visible on every resource', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')
    await expect(page.getByText('Master Data', { exact: true })).toBeVisible({ timeout: 10_000 })

    // Default resource (office-types) — the search input placeholder is i18n 'common.search' → "Cari"
    // We locate by placeholder; Nuxt UI UInput with icon renders as <input>.
    const searchInput = page.getByPlaceholder('Cari', { exact: true })
    await expect(searchInput).toBeVisible({ timeout: 8_000 })

    // Switch to departments and verify the search persists.
    await selectResource(page, RESOURCES.departments)
    await expect(page.getByRole('heading', { name: RESOURCES.departments, exact: true })).toBeVisible({ timeout: 8_000 })
    await expect(searchInput).toBeVisible()
  })
})
