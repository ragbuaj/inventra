import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, EMAIL, PASSWORD, pickAsync } from './helpers'

// ---------------------------------------------------------------------------
// Assets cluster — real backend (Katalog/Detail/Form/Label wired to
// /api/v1/assets + /api/v1/requests). Covers the full maker-checker lifecycle:
//
//   1. API setup: create the FK prerequisites (office type → office → floor →
//      room → category), a second "checker" user (SoD: the maker cannot
//      approve their own request), submit an `asset_create` request for a
//      SMALL amount (single approval step in the lowest threshold band —
//      see db/migrations/000016_office_tier.up.sql: asset_create 0–10,000,000
//      → office level, step 1 only), then approve as the checker so the asset
//      exists server-side before the UI assertions run.
//   2. UI: Katalog search → Detail → Edit → Label (PDF) against that asset.
//   3. UI: the `/assets/new` FORM flow, submitted as a *pending* request
//      (never approved) — verified via a follow-up API list call.
//   4. Negative: Katalog search for a non-existent string → empty state.
//
// IMPORTANT gotcha for tangible assets: the DB enforces a CHECK constraint
// requiring `room_id` on tangible assets (see internal/asset/service.go
// ErrRoomRequired) — this only fires at *approval* time (the executor), not
// at submission. So the API-approved asset (step 1 above) needs a real
// room_id; the UI-submitted pending request (step 3) does not, since it's
// never approved.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (partial unique indexes only free codes after soft-delete — this dev DB is
// NOT reset between runs), assert-after-search, wait for toasts/redirects
// rather than fixed sleeps, dialog/page-scoped selectors over ambiguous
// `getByText`, and use the pickers' `data-testid` triggers (USelect is a
// custom popover, not a native <select>).
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + seeded admin
// (see CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
// ---------------------------------------------------------------------------

const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`
const RUN = `${Date.now()}`

// --- thin API helpers (own APIRequestContext — the `request` fixture is
// test-scoped and unavailable in `beforeAll`, so we construct one manually
// via the top-level `request.newContext` helper and dispose it in `afterAll`). ---
function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}

async function apiJson<T>(res: APIResponse): Promise<T> {
  if (!res.ok()) {
    throw new Error(`API call failed: ${res.status()} ${res.url()} — ${await res.text()}`)
  }
  return res.json() as Promise<T>
}

async function login_(api: APIRequestContext, email: string, password: string): Promise<string> {
  const res = await api.post('auth/login', { data: { email, password } })
  const body = await apiJson<{ access_token: string }>(res)
  return body.access_token
}

test.describe('Assets — real backend (maker-checker e2e)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string
  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined

  let officeId: string
  let officeName: string
  let categoryId: string
  let categoryName: string
  let roomId: string

  // The asset created + approved via the API in beforeAll — used by the
  // Katalog/Detail/Edit/Label UI tests below.
  let assetName: string
  let assetTag: string

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })

    adminToken = await login_(api, EMAIL, PASSWORD)

    // --- FK prerequisites: office-type → office → floor → room, category. ---
    const otRes = await api.post('office-types', {
      headers: authHeader(adminToken),
      data: { name: `E2E Assets OT ${RUN}` }
    })
    const officeType = await apiJson<{ id: string }>(otRes)

    officeName = `E2E Assets Office ${RUN}`
    const offRes = await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeName, code: `E2EAO${RUN}`, office_type_id: officeType.id }
    })
    const office = await apiJson<{ id: string }>(offRes)
    officeId = office.id

    const flRes = await api.post('floors', {
      headers: authHeader(adminToken),
      data: { office_id: officeId, name: `E2E Floor ${RUN}` }
    })
    const floor = await apiJson<{ id: string }>(flRes)

    const rmRes = await api.post('rooms', {
      headers: authHeader(adminToken),
      data: { floor_id: floor.id, name: `E2E Room ${RUN}` }
    })
    const room = await apiJson<{ id: string }>(rmRes)
    roomId = room.id

    categoryName = `E2E Assets Category ${RUN}`
    const catRes = await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: categoryName, code: `E2E${RUN}`, asset_class: 'tangible' }
    })
    const category = await apiJson<{ id: string }>(catRes)
    categoryId = category.id

    // --- Checker user (SoD): a second Superadmin-scoped user so it is
    // eligible to decide (global data scope + request.decide) but is NOT the
    // maker, satisfying the self-approval guard (approval.ErrSelfApproval). ---
    const rolesRes = await api.get('authz/roles', { headers: authHeader(adminToken) })
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(rolesRes)
    const superadminRole = roles.data.find(r => r.name === 'Superadmin')
    if (!superadminRole) throw new Error('Superadmin role not found in GET /authz/roles')

    checkerEmail = `e2e.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    const userRes = await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadminRole.id }
    })
    checkerId = (await apiJson<{ id: string }>(userRes)).id

    // --- Submit the asset_create request as the maker (admin). Amount stays
    // in the lowest threshold band (0–10,000,000 → office level, single step). ---
    assetName = `E2E Asset ${RUN}`
    const submitRes = await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create',
        amount: '750000',
        office_id: officeId,
        payload: {
          name: assetName,
          category_id: categoryId,
          office_id: officeId,
          room_id: roomId,
          asset_class: 'tangible',
          purchase_cost: '750000',
          purchase_date: '2026-07-01'
        }
      }
    })
    const submitted = await apiJson<{ id: string }>(submitRes)

    // --- Approve as the checker (SoD: different user than the maker). ---
    const checkerToken = await login_(api, checkerEmail, checkerPassword)

    // First exercise the checker-facing inbox endpoint: the submitted request
    // must show up there before we approve it, otherwise the inbox listing
    // itself would go untested by this e2e flow.
    const inboxRes = await api.get('requests/inbox', {
      headers: authHeader(checkerToken)
    })
    const inbox = await apiJson<{ data: { id: string }[] }>(inboxRes)
    expect(inbox.data.some(r => r.id === submitted.id)).toBe(true)

    const approveRes = await api.post(`requests/${submitted.id}/approve`, {
      headers: authHeader(checkerToken),
      data: { decision: 'approve', note: 'e2e approve' }
    })
    const approved = await apiJson<{ status: string }>(approveRes)
    expect(approved.status).toBe('approved')

    // --- Resolve the resulting asset's tag/id for the UI tests below. ---
    const listRes = await api.get(`assets?search=${encodeURIComponent(assetName)}`, {
      headers: authHeader(adminToken)
    })
    const list = await apiJson<{ data: { id: string, asset_tag: string }[] }>(listRes)
    expect(list.data.length).toBe(1)
    assetTag = list.data[0]!.asset_tag
  })

  test.afterAll(async () => {
    // Best-effort cleanup: repeated local runs otherwise accumulate one checker
    // user per run, pushing the seeded admin off page 1 of /settings/users and
    // breaking that screen's e2e on dev databases (CI resets its DB per run).
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('Katalog: search finds the approved asset with resolved category and Tersedia status', async ({ page }) => {
    await login(page)
    await page.goto('/assets')
    await expect(page.getByRole('heading', { name: 'Katalog Aset' })).toBeVisible({ timeout: 10_000 })

    await page.getByPlaceholder('Cari nama atau kode aset…', { exact: true }).fill(assetName)

    const rowLink = page.getByRole('link', { name: assetTag })
    await expect(rowLink).toBeVisible({ timeout: 10_000 })

    const row = page.locator('tr', { has: rowLink })
    await expect(row.getByText('Tersedia', { exact: true })).toBeVisible()
    await expect(row.getByText(categoryName, { exact: true })).toBeVisible()
    await expect(row.getByText(officeName, { exact: true })).toBeVisible()

    await rowLink.click()
    await expect(page).toHaveURL(new RegExp(`/assets/${assetTag}$`))
  })

  test('Detail: key info renders, money is visible for superadmin, empty tabs show the not-available state', async ({ page }) => {
    await login(page)
    await page.goto(`/assets/${assetTag}`)

    await expect(page.getByRole('heading', { name: assetName, exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(assetTag, { exact: true })).toBeVisible()

    // Key-info card (left column) *and* the Info tab's Identity/Placement
    // sections both resolve + render the same category/office FK names — by
    // design (two panels summarizing overlapping fields), so `.first()` here
    // targets a genuine, intentional duplicate rather than an ambiguous query.
    await expect(page.getByText(categoryName, { exact: true }).first()).toBeVisible()
    await expect(page.getByText(officeName, { exact: true }).first()).toBeVisible()

    // Info tab (default) — the purchase cost is unmasked (default-allow field
    // permission) for the superadmin caller.
    await expect(page.getByText('Rp 750.000', { exact: true })).toBeVisible()

    // Assignment tab — module not yet built; shows the shared empty-state copy.
    await page.getByRole('button', { name: 'Riwayat Penugasan', exact: true }).click()
    await expect(page.getByText('Belum ada data — modul belum tersedia', { exact: true })).toBeVisible()

    // Maintenance tab — real module (lazy-loaded); a fresh asset has no
    // records, so the tab's own empty state renders.
    await page.getByRole('button', { name: 'Riwayat Maintenance', exact: true }).click()
    await expect(page.getByText('Belum ada riwayat maintenance untuk aset ini', { exact: true })).toBeVisible()
  })

  test('Edit: changing name and notes is reflected on the detail page', async ({ page }) => {
    await login(page)
    await page.goto(`/assets/${assetTag}/edit`)
    await expect(page.getByRole('heading', { name: 'Edit Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    const updatedName = `${assetName} Updated`
    const noteText = `Catatan e2e ${RUN}`

    await page.getByLabel('Nama Aset', { exact: true }).fill(updatedName)
    await page.getByLabel('Catatan', { exact: true }).fill(noteText)
    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    await expect(page).toHaveURL(new RegExp(`/assets/${assetTag}$`), { timeout: 10_000 })
    await expect(page.getByRole('heading', { name: updatedName, exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(noteText, { exact: true })).toBeVisible()

    // Keep the shared name in sync for any later assertions in this file.
    assetName = updatedName
  })

  test('Label: printing from the catalog row generates a real PDF', async ({ page }) => {
    await login(page)
    await page.goto('/assets')
    await expect(page.getByRole('heading', { name: 'Katalog Aset' })).toBeVisible({ timeout: 10_000 })

    await page.getByPlaceholder('Cari nama atau kode aset…', { exact: true }).fill(assetName)
    const rowLink = page.getByRole('link', { name: assetTag })
    await expect(rowLink).toBeVisible({ timeout: 10_000 })

    const row = page.locator('tr', { has: rowLink })
    await row.getByRole('button', { name: 'Cetak Label', exact: true }).click()

    await expect(page).toHaveURL(new RegExp(`/assets/label\\?tags=${assetTag}$`))
    await expect(page.getByRole('heading', { name: 'Label & Barcode', exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('1 dipilih', { exact: true })).toBeVisible({ timeout: 10_000 })

    const [response] = await Promise.all([
      page.waitForResponse(res => res.url().includes('/assets/labels') && res.request().method() === 'POST'),
      page.getByRole('button', { name: 'Cetak', exact: true }).click()
    ])
    expect(response.status()).toBe(200)
    expect(response.headers()['content-type']).toBe('application/pdf')
  })

  test('Form: submitting a new asset creates a pending asset_create request', async ({ page }) => {
    const formAssetName = `E2E Form Asset ${RUN}`
    const formPrice = String(1_000_000 + (Date.now() % 900_000))

    await login(page)
    await page.goto('/assets/new')
    await expect(page.getByRole('heading', { name: 'Tambah Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.getByLabel('Nama Aset', { exact: true }).fill(formAssetName)

    // Category and office are now AsyncSearchPicker components (server-side search
    // + click the matching result), replacing the old eager USelect dropdowns.
    await pickAsync(page, 'category', categoryName, categoryName)
    await pickAsync(page, 'office', officeName, officeName)

    await page.getByLabel('Tanggal Beli', { exact: true }).fill('2026-07-01')
    await page.getByLabel('Harga Beli', { exact: true }).fill(formPrice)

    await page.getByRole('button', { name: 'Simpan', exact: true }).click()

    // Exact match targets the visible toast title only — the toast also renders a
    // hidden aria-live announcer whose text is prefixed ("Notification …"), which
    // a substring match would ambiguously hit (strict-mode violation).
    await expect(page.getByText('Pengajuan terkirim — menunggu persetujuan', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page).toHaveURL(/\/assets$/, { timeout: 10_000 })

    // Verify server-side: a pending asset_create request now exists carrying
    // the exact amount we typed (payload isn't exposed on the list endpoint,
    // so the amount — unique per run — is the correlating field).
    const reqRes = await api.get('requests?type=asset_create&status=pending&limit=20', {
      headers: authHeader(adminToken)
    })
    const reqs = await apiJson<{ data: { amount: string, status: string, type: string }[] }>(reqRes)
    const match = reqs.data.find(r => Number(r.amount) === Number(formPrice))
    expect(match).toBeTruthy()
    expect(match?.status).toBe('pending')
    expect(match?.type).toBe('asset_create')
  })

  test('Katalog: searching a non-existent string shows the empty state', async ({ page }) => {
    await login(page)
    await page.goto('/assets')
    await expect(page.getByRole('heading', { name: 'Katalog Aset' })).toBeVisible({ timeout: 10_000 })

    await page.getByPlaceholder('Cari nama atau kode aset…', { exact: true }).fill(`nonexistent-zzz-${RUN}`)

    await expect(page.getByText('Tidak ada aset yang cocok', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Coba ubah kata kunci atau atur ulang filter.', { exact: true })).toBeVisible()
  })
})
