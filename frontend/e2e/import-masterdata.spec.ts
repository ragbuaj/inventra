import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'
import {
  buildOfficeCsv,
  buildProvinceCsv,
  buildCityCsv,
  buildBrandCsv,
  buildModelCsv
} from './fixtures/import-fixtures'

// ---------------------------------------------------------------------------
// Bulk Import — office + reference master-data targets (real backend). The
// asset + employee targets are covered by frontend/e2e/import.spec.ts; this
// file covers the targets registered in Tasks 2-4 of the import-followups
// plan: office, reference:provinces, reference:cities, reference:brands,
// reference:models (reference:units is unit-tested only — skipped here, it
// is structurally identical to provinces/brands).
//
// All five targets have NeedsApproval() == false (see office/importer.go and
// reference/importer.go) — unlike the asset target, confirming a validated
// batch here goes straight to "completed" with no maker-checker detour, so
// each test is a single-user, single-phase flow: upload -> validate (2 valid
// + 1 deliberately invalid row) -> confirm -> completed -> download the
// failed-row report -> assert the 2 valid rows exist.
//
// Robustness rules (project e2e conventions, same as import.spec.ts):
// unique name/code per run (RUN suffixes every value — this dev DB is never
// reset between local runs), assert-after-search, and no fixed sleeps.
//
// GOTCHA (office only) — app/pages/master/offices.vue loads only the first
// 100 offices (api.list({ limit: 100 }), no server-side search — see its
// `refresh`/`filteredNodes`, which filters client-side over that capped set)
// exactly like the Pegawai list documented in import.spec.ts. This dev DB
// already carries 200+ offices from prior e2e runs, so a freshly imported
// office is not reliably inside that page's loaded slice. The office test
// therefore verifies creation via GET /offices?search=... — the same
// authenticated, scope-enforced backend endpoint the page would call if
// search were wired through — rather than the capped UI list. The other four
// targets' list page (app/pages/master/reference.vue) DOES pass `search` to
// the API server-side, so those assertions go through the real UI list.
// ---------------------------------------------------------------------------

const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`
const RUN = `${Date.now()}`

function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}
async function apiJson<T>(res: APIResponse): Promise<T> {
  if (!res.ok()) throw new Error(`API call failed: ${res.status()} ${res.url()} — ${await res.text()}`)
  return res.json() as Promise<T>
}
async function login_(api: APIRequestContext, email: string, password: string): Promise<string> {
  const res = await api.post('auth/login', { data: { email, password } })
  return (await apiJson<{ access_token: string }>(res)).access_token
}

test.describe('Bulk Import — master-data e2e (office + reference targets)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)
  })

  test.afterAll(async () => {
    await api.dispose()
  })

  test('office import: 2 valid root offices + 1 dupKode row, verified via GET /offices?search=', async ({ page }) => {
    const ot = await apiJson<{ id: string, name: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Office Import OT ${RUN}` }
    }))
    const { filename, csv, nameA, nameB, dupRowName } = buildOfficeCsv({ officeTypeName: ot.name, run: RUN })

    await login(page)
    await page.goto('/master/import?target=office')
    await expect(page.getByRole('heading', { name: 'Import Massal — Kantor', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()

    await expect(page.getByTestId('import-valid-count')).toHaveText('2 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('1 Error')

    const dupRow = page.locator('tr', { hasText: dupRowName })
    await expect(dupRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(dupRow.getByText('Kode duplikat', { exact: true })).toBeVisible()

    const goodRow = page.locator('tr', { hasText: nameA })
    await expect(goodRow.getByText('Valid', { exact: true })).toBeVisible()

    await page.getByTestId('import-confirm-button').click()
    await expect(page.getByText('Import selesai diproses', { exact: true })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('import-result-created')).toContainText('Aset dibuat')
    await expect(page.getByTestId('import-result-failed')).toContainText('Baris gagal')

    const [errDl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Unduh Baris Gagal', exact: true }).click()
    ])
    expect(errDl.suggestedFilename()).toContain('errors.csv')

    // See file-level GOTCHA comment: verified via the search-capable backend
    // endpoint rather than the client-capped /master/offices tree.
    const createdA = await apiJson<{ data: { name: string, code: string }[] }>(
      await api.get(`offices?search=${encodeURIComponent(nameA)}`, { headers: authHeader(adminToken) }))
    expect(createdA.data.length).toBe(1)
    const createdB = await apiJson<{ data: { name: string, code: string }[] }>(
      await api.get(`offices?search=${encodeURIComponent(nameB)}`, { headers: authHeader(adminToken) }))
    expect(createdB.data.length).toBe(1)
    // The rejected (dup-kode) row must never have been created.
    const dupCheck = await apiJson<{ data: { name: string }[] }>(
      await api.get(`offices?search=${encodeURIComponent(dupRowName)}`, { headers: authHeader(adminToken) }))
    expect(dupCheck.data.length).toBe(0)
  })

  test('reference:provinces import: 2 valid + 1 dupKode row, appears in Referensi', async ({ page }) => {
    const { filename, csv, nameA, nameB, dupRowName } = buildProvinceCsv({ run: RUN })

    await login(page)
    await page.goto('/master/import?target=reference:provinces')
    await expect(page.getByRole('heading', { name: 'Import Massal — Provinsi', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()

    await expect(page.getByTestId('import-valid-count')).toHaveText('2 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('1 Error')

    const dupRow = page.locator('tr', { hasText: dupRowName })
    await expect(dupRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(dupRow.getByText('Kode duplikat', { exact: true })).toBeVisible()

    await page.getByTestId('import-confirm-button').click()
    await expect(page.getByText('Import selesai diproses', { exact: true })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('import-result-created')).toContainText('Aset dibuat')
    await expect(page.getByTestId('import-result-failed')).toContainText('Baris gagal')

    const [errDl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Unduh Baris Gagal', exact: true }).click()
    ])
    expect(errDl.suggestedFilename()).toContain('errors.csv')

    // Referensi's list DOES pass `search` to the API server-side (see
    // app/pages/master/reference.vue refresh) — a reliable UI assertion
    // regardless of how many provinces already exist in this dev DB.
    await page.goto('/master/reference')
    await page.getByTestId('ref-nav-provinces').click()
    await expect(page.getByRole('heading', { name: 'Provinsi', exact: true })).toBeVisible({ timeout: 10_000 })
    await page.getByPlaceholder('Cari', { exact: true }).fill(nameA)
    await expect(page.getByText(nameA, { exact: true })).toBeVisible({ timeout: 10_000 })
    await page.getByPlaceholder('Cari', { exact: true }).fill(nameB)
    await expect(page.getByText(nameB, { exact: true })).toBeVisible({ timeout: 10_000 })
  })

  test('reference:cities import: FK provinsi resolves + bad provinsi row rejected, appears in Referensi', async ({ page }) => {
    const provinceName = `E2E Kota Import Provinsi ${RUN}`
    await apiJson<{ id: string, name: string }>(await api.post('provinces', {
      headers: authHeader(adminToken), data: { name: provinceName }
    }))
    const { filename, csv, nameA, badProvinceRowName } = buildCityCsv({ provinceName, run: RUN })

    await login(page)
    await page.goto('/master/import?target=reference:cities')
    await expect(page.getByRole('heading', { name: 'Import Massal — Kota', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()

    await expect(page.getByTestId('import-valid-count')).toHaveText('2 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('1 Error')

    const badRow = page.locator('tr', { hasText: badProvinceRowName })
    await expect(badRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(badRow.getByText('Provinsi tidak ditemukan', { exact: true })).toBeVisible()

    const goodRow = page.locator('tr', { hasText: nameA })
    await expect(goodRow.getByText('Valid', { exact: true })).toBeVisible()

    await page.getByTestId('import-confirm-button').click()
    await expect(page.getByText('Import selesai diproses', { exact: true })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('import-result-created')).toContainText('Aset dibuat')
    await expect(page.getByTestId('import-result-failed')).toContainText('Baris gagal')

    const [errDl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Unduh Baris Gagal', exact: true }).click()
    ])
    expect(errDl.suggestedFilename()).toContain('errors.csv')

    await page.goto('/master/reference')
    await page.getByTestId('ref-nav-cities').click()
    await expect(page.getByRole('heading', { name: 'Kota', exact: true })).toBeVisible({ timeout: 10_000 })
    await page.getByPlaceholder('Cari', { exact: true }).fill(nameA)
    await expect(page.getByText(nameA, { exact: true })).toBeVisible({ timeout: 10_000 })
  })

  test('reference:brands import: 2 valid + 1 dupNama row, appears in Referensi', async ({ page }) => {
    const { filename, csv, nameA, nameB } = buildBrandCsv({ run: RUN })

    await login(page)
    await page.goto('/master/import?target=reference:brands')
    await expect(page.getByRole('heading', { name: 'Import Massal — Brand', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()

    await expect(page.getByTestId('import-valid-count')).toHaveText('2 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('1 Error')

    // The duplicate row repeats nameA verbatim, so scope the "second"
    // occurrence via the row index rather than a distinct name.
    const rows = page.locator('tbody tr')
    await expect(rows).toHaveCount(3)
    const dupRow = rows.nth(2)
    await expect(dupRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(dupRow.getByText('Nama duplikat', { exact: true })).toBeVisible()

    const goodRow = rows.nth(0)
    await expect(goodRow.getByText('Valid', { exact: true })).toBeVisible()

    await page.getByTestId('import-confirm-button').click()
    await expect(page.getByText('Import selesai diproses', { exact: true })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('import-result-created')).toContainText('Aset dibuat')
    await expect(page.getByTestId('import-result-failed')).toContainText('Baris gagal')

    const [errDl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Unduh Baris Gagal', exact: true }).click()
    ])
    expect(errDl.suggestedFilename()).toContain('errors.csv')

    await page.goto('/master/reference')
    await page.getByTestId('ref-nav-brands').click()
    await expect(page.getByRole('heading', { name: 'Brand', exact: true })).toBeVisible({ timeout: 10_000 })
    await page.getByPlaceholder('Cari', { exact: true }).fill(nameA)
    await expect(page.getByText(nameA, { exact: true })).toBeVisible({ timeout: 10_000 })

    // Only ONE brand named nameA exists — the duplicate row was rejected, not
    // silently created twice.
    const createdA = await apiJson<{ data: { name: string }[] }>(
      await api.get(`brands?search=${encodeURIComponent(nameA)}`, { headers: authHeader(adminToken) }))
    expect(createdA.data.length).toBe(1)
    const createdB = await apiJson<{ data: { name: string }[] }>(
      await api.get(`brands?search=${encodeURIComponent(nameB)}`, { headers: authHeader(adminToken) }))
    expect(createdB.data.length).toBe(1)
  })

  test('reference:models import: FK merek resolves + unknown merek row rejected, appears in Referensi', async ({ page }) => {
    const brandName = `E2E Model Import Brand ${RUN}`
    await apiJson<{ id: string, name: string }>(await api.post('brands', {
      headers: authHeader(adminToken), data: { name: brandName }
    }))
    const { filename, csv, nameA, unknownBrandRowName } = buildModelCsv({ brandName, run: RUN })

    await login(page)
    await page.goto('/master/import?target=reference:models')
    await expect(page.getByRole('heading', { name: 'Import Massal — Model', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()

    await expect(page.getByTestId('import-valid-count')).toHaveText('2 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('1 Error')

    const badRow = page.locator('tr', { hasText: unknownBrandRowName })
    await expect(badRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(badRow.getByText('Brand tidak ditemukan', { exact: true })).toBeVisible()

    const goodRow = page.locator('tr', { hasText: nameA })
    await expect(goodRow.getByText('Valid', { exact: true })).toBeVisible()

    await page.getByTestId('import-confirm-button').click()
    await expect(page.getByText('Import selesai diproses', { exact: true })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('import-result-created')).toContainText('Aset dibuat')
    await expect(page.getByTestId('import-result-failed')).toContainText('Baris gagal')

    const [errDl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Unduh Baris Gagal', exact: true }).click()
    ])
    expect(errDl.suggestedFilename()).toContain('errors.csv')

    await page.goto('/master/reference')
    await page.getByTestId('ref-nav-models').click()
    await expect(page.getByRole('heading', { name: 'Model', exact: true })).toBeVisible({ timeout: 10_000 })
    await page.getByPlaceholder('Cari', { exact: true }).fill(nameA)
    await expect(page.getByText(nameA, { exact: true })).toBeVisible({ timeout: 10_000 })
  })
})
