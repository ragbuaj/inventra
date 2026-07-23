import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, BrowserContext, Page } from '@playwright/test'
import { EMAIL, PASSWORD } from './helpers'
import {
  buildAssetHappyPathCsv,
  buildAssetValidationRejectionCsv,
  buildEmployeeCsv
} from './fixtures/import-fixtures'

// ---------------------------------------------------------------------------
// Bulk Import — real backend (ImportWizard wired to /api/v1/imports; asset
// batches route through the maker-checker `asset_import` approval type,
// employee batches are created directly). Flow:
//
//   beforeAll (API): office-type -> two offices (A used for the happy-path +
//   employee batches, B only to trigger the cross-office `multiOffice` rule)
//   -> floor -> room (on office A; a tangible asset's CHECK constraint
//   requires room_id at approval-execute time, see internal/asset/service.go
//   ErrRoomRequired) -> category; a second SoD checker user (Superadmin role,
//   global scope) so it can decide the batch but is never the maker.
//
//   1. Asset import happy path (ONE test, ONE page, both users): maker
//      downloads the template, uploads a 3-row batch (2 valid + 1 unknown-
//      category row), confirms the 2 valid rows -> job goes
//      awaiting_approval. Mid-test switch to the checker (clearCookies +
//      localStorage.clear() + re-login — the httpOnly refresh cookie would
//      otherwise silently restore the maker's session) -> approve the
//      asset_import request in /approval. Switch back to the maker the same
//      way. A completed job is NOT auto-resumed by the wizard (onMounted only
//      resumes non-final jobs — see ImportWizard.vue's FINAL list), so
//      completion is asserted via the API (poll GET /imports/:id), then the
//      created asset is looked up by name and asserted present in the
//      Katalog (server-side search, not the client-capped list — see below).
//   2. Employee import: 2 valid + 2 invalid (implausible email, unknown
//      status) rows -> confirm -> completes directly (no approval) ->
//      "Unduh Baris Gagal" fires a real download -> the created employee is
//      searched for in the Pegawai list.
//   3. Validation-rejection preview: a fresh asset batch with a bad-date row
//      (`tgl_beli` not ISO) and a cross-office row -> asserts the preview
//      table surfaces both error keys without ever confirming the batch.
//
// GOTCHA — Pegawai list search is NOT server-side: master/employees.vue loads
// only the first 100 rows (ordered by name) into `allRows` and filters
// client-side over that capped set (app/pages/master/employees.vue `refresh`/
// `filteredRows`). CI's e2e job runs against a fresh per-run DB (well under
// 100 employees), so the assertion is reliable there; on a local dev DB that
// has accumulated 100+ employees across many manual e2e runs, a freshly
// imported employee could sort outside the loaded page and this assertion
// could flake. This is a pre-existing page limitation, not something this
// spec works around.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (RUN suffixes everything — this dev DB is NOT reset between local runs),
// assert-after-search, wait for toasts/text rather than fixed sleeps, and
// serial mode (test 1 must complete its asset-target job to 'completed' —
// a FINAL status — before test 3 uploads a fresh asset-target batch, since
// the wizard's resume-on-mount logic would otherwise pick up a leftover
// non-final job from an earlier test as "the" job for that target).
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + seeded admin +
// the import worker running (IMPORT_WORKER_ENABLED=true, the default — see
// backend/internal/config/config.go) with its 2s poll tick. This spec
// compiles + lints here; CI runs it in the e2e job.
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

// helpers.ts's `login(page)` only signs in the fixed seeded admin — this spec
// also signs in as the checker mid-test, so it needs a parametrized UI-login
// helper (same steps as helpers.ts, generalized over credentials — mirrors
// approval.spec.ts's `loginAs`).
async function loginAs(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login')
  await page.locator('input[name="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

// Mid-test user switch (project e2e convention): clearing cookies AND
// localStorage before re-login is required in the SAME test/page — otherwise
// the httpOnly refresh-token cookie silently restores the previous user's
// session regardless of which credentials are typed into the login form.
async function switchUser(page: Page, context: BrowserContext, email: string, password: string): Promise<void> {
  await context.clearCookies()
  await page.evaluate(() => localStorage.clear())
  await loginAs(page, email, password)
}

// Same "detail pane still mid-fetch" race as approval.spec.ts: a card click
// kicks off an async detail load, so `waitFor` (which polls) must be used
// instead of a non-waiting `isVisible()` snapshot.
async function detailShows(page: Page, text: string, timeout = 5_000): Promise<boolean> {
  return page.locator('text=' + text).first().waitFor({ state: 'visible', timeout })
    .then(() => true)
    .catch(() => false)
}

// Polls GET /imports/:id (as the given token) until it reaches `target`
// status or the timeout elapses. Used only where the UI itself cannot show
// the transition (a completed job is never auto-resumed by the wizard).
async function pollJobStatus(
  api: APIRequestContext, token: string, jobId: string, target: string, timeoutMs = 20_000
): Promise<{ status: string, success_rows: number, failed_rows: number }> {
  const start = Date.now()
  let last = ''
  while (Date.now() - start < timeoutMs) {
    const res = await api.get(`imports/${jobId}`, { headers: authHeader(token) })
    const job = await apiJson<{ status: string, success_rows: number, failed_rows: number }>(res)
    last = job.status
    if (job.status === target) return job
    await new Promise(resolve => setTimeout(resolve, 1_000))
  }
  throw new Error(`import job ${jobId} did not reach status "${target}" within ${timeoutMs}ms (last seen: "${last}")`)
}

test.describe('Bulk Import — real backend (asset + employee e2e)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string
  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined

  let officeAName: string
  let officeBName: string
  let roomName: string
  let categoryName: string

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Import OT ${RUN}` }
    }))

    officeAName = `E2E Import Office A ${RUN}`
    const officeA = await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeAName, code: `E2EIMA${RUN}`, office_type_id: ot.id }
    }))

    officeBName = `E2E Import Office B ${RUN}`
    await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeBName, code: `E2EIMB${RUN}`, office_type_id: ot.id }
    }))

    const floor = await apiJson<{ id: string }>(await api.post('floors', {
      headers: authHeader(adminToken),
      data: { office_id: officeA.id, name: `E2E Import Floor ${RUN}` }
    }))
    roomName = `E2E Import Room ${RUN}`
    await apiJson<{ id: string }>(await api.post('rooms', {
      headers: authHeader(adminToken),
      data: { floor_id: floor.id, name: roomName }
    }))

    categoryName = `E2E Import Category ${RUN}`
    await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: categoryName, code: `E2EIMC${RUN}`, asset_class: 'tangible' }
    }))

    // Checker user (SoD): a second Superadmin-scoped user so it is eligible to
    // decide (global data scope + request.decide) but is NOT the maker.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    checkerEmail = `e2e.import.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Import Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id
  })

  test.afterAll(async () => {
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('asset import: template download, preview errors, maker-checker approval, appears in Katalog', async ({ page, context }) => {
    const { filename, csv, row1Name, row2Name, badRowName } = buildAssetHappyPathCsv({
      officeName: officeAName, categoryName, roomName, run: RUN
    })

    // --- MAKER: upload + preview ---------------------------------------
    await loginAs(page, EMAIL, PASSWORD)
    await page.goto('/assets/import')
    await expect(page.getByRole('heading', { name: 'Import Massal Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    const [templateDl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Unduh Template', exact: true }).click()
    ])
    expect(templateDl.suggestedFilename()).toBe('asset-template.csv')

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()

    const [uploadResp] = await Promise.all([
      page.waitForResponse(res => res.url().endsWith('/imports') && res.request().method() === 'POST'),
      page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()
    ])
    expect(uploadResp.status()).toBe(201)
    const jobId = (await uploadResp.json() as { id: string }).id

    // Validate phase runs on the async worker's poll tick (~2s) — allow
    // generous time for the preview table to appear. Select by testid rather
    // than page-wide text so this can't collide with other "N Valid"/"N
    // Error" text elsewhere in the wizard.
    await expect(page.getByTestId('import-valid-count')).toHaveText('2 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('1 Error')

    // The invalid row (unknown kategori) is highlighted with its error note;
    // the two valid rows carry no note.
    const badRow = page.locator('tr', { hasText: badRowName })
    await expect(badRow).toBeVisible()
    await expect(badRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(badRow.getByText('Kategori tidak ditemukan', { exact: true })).toBeVisible()

    const goodRow = page.locator('tr', { hasText: row1Name })
    await expect(goodRow.getByText('Valid', { exact: true })).toBeVisible()

    // --- MAKER: confirm the 2 valid rows --------------------------------
    await page.getByTestId('import-confirm-button').click()
    await expect(page.getByTestId('import-awaiting-approval')).toBeVisible({ timeout: 20_000 })

    // --- SWITCH TO CHECKER: approve the asset_import request ------------
    await switchUser(page, context, checkerEmail, checkerPassword)
    await page.goto('/approval')
    const cards = page.locator('[data-testid="approval-card"]', { hasText: officeAName })
    await expect(cards.first()).toBeVisible({ timeout: 10_000 })
    const n = await cards.count()
    let found = false
    for (let i = 0; i < n; i++) {
      await cards.nth(i).click()
      if (await detailShows(page, filename)) {
        found = true
        break
      }
    }
    expect(found).toBe(true)
    await page.getByTestId('approval-approve').click()
    await expect(page.locator('text=Disetujui oleh').first()).toBeVisible({ timeout: 10_000 })

    // --- SWITCH BACK TO MAKER: job completes, asset appears in Katalog ---
    await switchUser(page, context, EMAIL, PASSWORD)

    // A completed job is a FINAL status — ImportWizard's onMounted resume
    // deliberately skips it (see FINAL in ImportWizard.vue), so completion is
    // asserted via the API rather than by navigating back to the wizard.
    const finished = await pollJobStatus(api, adminToken, jobId, 'completed')
    expect(finished.success_rows).toBe(2)
    expect(finished.failed_rows).toBe(1)

    const created = await apiJson<{ data: { asset_tag: string }[] }>(
      await api.get(`assets?search=${encodeURIComponent(row1Name)}`, { headers: authHeader(adminToken) }))
    expect(created.data.length).toBe(1)
    const assetTag = created.data[0]!.asset_tag

    await page.goto('/assets')
    await expect(page.getByRole('heading', { name: 'Katalog Aset' })).toBeVisible({ timeout: 10_000 })
    await page.getByPlaceholder('Cari nama atau kode aset…', { exact: true }).fill(row1Name)
    await expect(page.getByRole('link', { name: assetTag })).toBeVisible({ timeout: 10_000 })

    // The second valid row's asset also exists (both rows shared one batch).
    const created2 = await apiJson<{ data: { asset_tag: string }[] }>(
      await api.get(`assets?search=${encodeURIComponent(row2Name)}`, { headers: authHeader(adminToken) }))
    expect(created2.data.length).toBe(1)
  })

  test('employee import: valid + invalid rows, error-report download, appears in Pegawai', async ({ page }) => {
    const { filename, csv, nameA } = buildEmployeeCsv({ officeName: officeAName, run: RUN })

    await loginAs(page, EMAIL, PASSWORD)
    await page.goto('/master/import?target=employee')
    await expect(page.getByRole('heading', { name: 'Import Massal — Pegawai', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()

    await expect(page.getByTestId('import-valid-count')).toHaveText('2 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('2 Error')

    const emailRow = page.locator('tr', { hasText: 'not-an-email' })
    await expect(emailRow.getByText('Email tidak valid', { exact: true })).toBeVisible()
    const statusRow = page.locator('tr', { hasText: 'unknown_status' })
    await expect(statusRow.getByText('Status tidak valid', { exact: true })).toBeVisible()

    // Employee imports need no approval: confirm goes straight to completed.
    await page.getByTestId('import-confirm-button').click()
    await expect(page.getByText('Import selesai diproses', { exact: true })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('import-result-created')).toContainText('Aset dibuat')
    await expect(page.getByTestId('import-result-failed')).toContainText('Baris gagal')

    const [errDl] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('button', { name: 'Unduh Baris Gagal', exact: true }).click()
    ])
    expect(errDl.suggestedFilename()).toBe('employee-errors.csv')

    // Pegawai list search — see the file-level GOTCHA comment: reliable on a
    // fresh (CI) DB, best-effort on an accumulated local dev DB.
    await page.goto('/master/employees')
    await expect(page.getByRole('heading', { name: 'Pegawai', exact: true })).toBeVisible({ timeout: 10_000 })
    await page.getByPlaceholder('Cari nama atau NIP…', { exact: true }).fill(nameA)
    await expect(page.getByText(nameA, { exact: true })).toBeVisible({ timeout: 10_000 })
  })

  test('asset import: validation preview rejects a bad date and a cross-office row', async ({ page }) => {
    const { filename, csv, badDateRowName, multiOfficeRowName, validRowName } = buildAssetValidationRejectionCsv({
      officeAName, officeBName, categoryName, roomName, run: RUN
    })

    await loginAs(page, EMAIL, PASSWORD)
    await page.goto('/assets/import')
    await expect(page.getByRole('heading', { name: 'Import Massal Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.locator('[data-testid="import-file-input"]').setInputFiles({
      name: filename, mimeType: 'text/csv', buffer: Buffer.from(csv, 'utf-8')
    })
    await expect(page.getByText(filename, { exact: true })).toBeVisible()
    const [uploadResp] = await Promise.all([
      page.waitForResponse(res => res.url().endsWith('/imports') && res.request().method() === 'POST'),
      page.getByRole('button', { name: 'Validasi Berkas', exact: true }).click()
    ])
    const jobId = (await uploadResp.json() as { id: string }).id

    await expect(page.getByTestId('import-valid-count')).toHaveText('1 Valid', { timeout: 20_000 })
    await expect(page.getByTestId('import-error-count')).toHaveText('2 Error')

    const badDateRow = page.locator('tr', { hasText: badDateRowName })
    await expect(badDateRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(badDateRow.getByText('Tanggal tidak valid', { exact: true })).toBeVisible()

    const multiOfficeRow = page.locator('tr', { hasText: multiOfficeRowName })
    await expect(multiOfficeRow.getByText('Error', { exact: true })).toBeVisible()
    await expect(multiOfficeRow.getByText('Batch mencakup lebih dari satu kantor', { exact: true })).toBeVisible()

    const validRow = page.locator('tr', { hasText: validRowName })
    await expect(validRow.getByText('Valid', { exact: true })).toBeVisible()

    // Deliberately not confirmed — this scenario only exercises the preview.
    // Cancel the job afterward so it doesn't linger as a non-final "validated"
    // job on this (never-reset) local dev DB — the wizard's onMounted resume
    // would otherwise pick up this leftover job the next time ANY test in
    // this file navigates to /assets/import (even in a brand-new test run),
    // skipping straight past the fresh-upload step 1 that other tests expect.
    await api.post(`imports/${jobId}/cancel`, { headers: authHeader(adminToken) })
  })
})
