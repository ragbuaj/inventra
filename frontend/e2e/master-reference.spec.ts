import { test, expect } from '@playwright/test'
import { login, pickAsync } from './helpers'

// ---------------------------------------------------------------------------
// Master Data Referensi screen — real backend (GET/POST/PUT/DELETE via
// the generic reference engine at /api/v1/<resource>, e.g. /api/v1/office-types).
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
// Helper: click a sidebar resource button by its resource key.
// The sidebar renders each resource as a <button data-testid="ref-nav-<key>">
// whose accessible name is label + count badge (e.g. "Provinsi 9"), so a
// name-based match is unreliable — we target the stable per-resource testid.
// ---------------------------------------------------------------------------
type ReferenceKey = keyof typeof RESOURCES

async function selectResource(page: import('@playwright/test').Page, key: ReferenceKey) {
  await page.getByTestId(`ref-nav-${key}`).click()
}

// ---------------------------------------------------------------------------
// Sidebar renders
// ---------------------------------------------------------------------------
test.describe('Master Data Referensi — sidebar', () => {
  test('sidebar renders the resource labels', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')

    // Wait for the panel to mount (heading "Master Data" is always visible).
    await expect(page.getByTestId('reference-panel-title')).toBeVisible({ timeout: 10_000 })

    // Assert a representative subset of the sidebar nav buttons via their stable
    // per-resource testid, and that each renders its i18n label text. (The button's
    // accessible name also includes the count badge, so we assert the label as a
    // substring via toContainText rather than an exact name match.)
    await expect(page.getByTestId('ref-nav-provinces')).toContainText(RESOURCES.provinces)
    await expect(page.getByTestId('ref-nav-departments')).toContainText(RESOURCES.departments)
    await expect(page.getByTestId('ref-nav-brands')).toContainText(RESOURCES.brands)
    await expect(page.getByTestId('ref-nav-maintenance-categories')).toContainText(RESOURCES['maintenance-categories'])
    await expect(page.getByTestId('ref-nav-office-types')).toContainText(RESOURCES['office-types'])
  })

  test('Add button is visible for masterdata.global.manage holder', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')
    await expect(page.getByTestId('reference-panel-title')).toBeVisible({ timeout: 10_000 })
    // "Tambah" button (i18n: masterdata.reference.add) is gated by can('masterdata.global.manage').
    await expect(page.getByRole('button', { name: 'Tambah', exact: true })).toBeVisible({ timeout: 8_000 })
  })
})

// ---------------------------------------------------------------------------
// Province CRUD (no FK dependency)
// ---------------------------------------------------------------------------
test.describe('Master Data Referensi — provinces CRUD', () => {
  // Unique suffix so repeated runs don't collide on the same name — the CODE
  // must be unique per run too: provinces.code has a partial unique index
  // (WHERE deleted_at IS NULL) and this spec never deletes its rows, so a
  // fixed code passes once and then fails every later run on a dev database
  // with a 23505 duplicate (the modal stays open on the error toast).
  const provinceName = `E2E Provinsi ${Date.now()}`
  const provinceCode = `E2-${String(Date.now()).slice(-7)}`

  test('create a province and assert the row appears in the table', async ({ page }) => {
    await login(page)
    await page.goto('/master/reference')
    await expect(page.getByTestId('reference-panel-title')).toBeVisible({ timeout: 10_000 })

    // Select "Provinsi" resource in the sidebar.
    await selectResource(page, 'provinces')

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
    // Assert-after-search (project e2e rule): provinces accumulate across runs
    // on a dev database, so the new row may land beyond page 1.
    await page.getByPlaceholder('Cari', { exact: true }).fill(provinceName)
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
    await expect(page.getByTestId('reference-panel-title')).toBeVisible({ timeout: 10_000 })

    // --- Step 1: create a province ---
    await selectResource(page, 'provinces')
    await expect(page.getByRole('heading', { name: RESOURCES.provinces, exact: true })).toBeVisible({ timeout: 8_000 })
    await page.getByRole('button', { name: 'Tambah', exact: true }).click()
    await expect(page.getByText('Tambah Data', { exact: true })).toBeVisible({ timeout: 5_000 })
    await page.getByLabel('Nama', { exact: true }).fill(provinceName2)
    // Unique per run — same partial-unique-index reasoning as provinceCode above.
    await page.getByLabel('Kode', { exact: true }).fill(`E3-${String(Date.now()).slice(-7)}`)
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()
    // Province row must appear before switching resources (assert-after-search:
    // rows accumulate across runs, the new one may land beyond page 1).
    await page.getByPlaceholder('Cari', { exact: true }).fill(provinceName2)
    await expect(page.getByText(provinceName2, { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Step 2: switch to "Kota" ---
    await selectResource(page, 'cities')
    await expect(page.getByRole('heading', { name: RESOURCES.cities, exact: true })).toBeVisible({ timeout: 8_000 })

    // --- Step 3: open the create modal ---
    await page.getByRole('button', { name: 'Tambah', exact: true }).click()
    await expect(page.getByText('Tambah Data', { exact: true })).toBeVisible({ timeout: 5_000 })

    // --- Step 4: assert the Provinsi FK picker is present ---
    // The FK field renders as an AsyncSearchPicker with
    // :testid="`ref-field-${field.key}`" (i.e. `ref-field-province_id`), so its
    // input carries data-testid="ref-field-province_id-picker-input" and its
    // result rows "ref-field-province_id-picker-item" (added to reference.vue
    // so this locator is deterministic — no broad div/button filter).
    const provinsiInput = page.getByTestId('ref-field-province_id-picker-input')
    await expect(provinsiInput).toBeVisible({ timeout: 5_000 })

    // Search + pick the province we just created via the async picker.
    await pickAsync(page, 'ref-field-province_id', provinceName2, provinceName2)

    // --- Step 5: fill the city name + code (code unique per run — see above) ---
    await page.getByLabel('Nama', { exact: true }).fill(cityName)
    await page.getByLabel('Kode', { exact: true }).fill(`E31-${String(Date.now()).slice(-7)}`)

    // --- Step 6: submit ---
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // The modal closes and the city row appears (assert-after-search — same
    // accumulation reasoning as the province asserts above).
    await page.getByPlaceholder('Cari', { exact: true }).fill(cityName)
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
    await expect(page.getByTestId('reference-panel-title')).toBeVisible({ timeout: 10_000 })

    // Default resource (office-types) — the search input placeholder is i18n 'common.search' → "Cari"
    // We locate by placeholder; Nuxt UI UInput with icon renders as <input>.
    const searchInput = page.getByPlaceholder('Cari', { exact: true })
    await expect(searchInput).toBeVisible({ timeout: 8_000 })

    // Switch to departments and verify the search persists.
    await selectResource(page, 'departments')
    await expect(page.getByRole('heading', { name: RESOURCES.departments, exact: true })).toBeVisible({ timeout: 8_000 })
    await expect(searchInput).toBeVisible()
  })
})
