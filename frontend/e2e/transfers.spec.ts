import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, pickAsync, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Transfer (Mutasi) — real backend (`/transfers` wired to /api/v1/transfers +
// /api/v1/requests). Covers the full submit → approve → ship → receive
// lifecycle, plus the destination-office reject-receive path:
//
//   beforeAll (API): office-type → two offices (A origin, B destination) →
//   category (asset_class 'intangible', so approval never hits the tangible
//   room_id CHECK constraint — see assets.spec.ts's gotcha) → a second
//   "checker" user (SoD: the maker/admin cannot approve their own request) →
//   two assets at office A, each created via the asset_create maker-checker
//   flow (submit as admin, approve as checker) so they exist server-side
//   as status=available before the UI tests run.
//
//   1. Submit via UI (/transfers → Ajukan tab): pick asset 1 by typing its
//      unique name into the AssetSearchPicker, destination B, date, condition
//      "rusak_ringan", submit → success banner; API-verify a pending
//      asset_transfer request now targets this asset.
//   2. Approve that request via API as the checker (decision: 'approve', NOT
//      'approved') → UI Riwayat shows the row "Disetujui" with a Kirim
//      button → ship it → row becomes "Dalam Pengiriman".
//   3. Kotak Masuk (admin — Superadmin has office_id=null i.e. global scope,
//      so the inbox is unfiltered and shows every in-transit transfer): the
//      card shows the "Rusak Ringan" condition badge → Terima with a BAST
//      number → Riwayat shows "Diterima" with that BAST number; API-verify
//      the asset now lives at office B.
//   4. Reject-receive: asset 2 goes through submit→approve→ship purely via
//      API (mirroring the composable calls the UI would make), then the UI
//      Kotak Masuk's "Tolak Terima" flow returns it — row "Dikembalikan";
//      API-verify the asset still lives at office A.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (this dev DB is NOT reset between runs), assert-after-search, wait for
// toasts/redirects rather than fixed sleeps, row/card-scoped selectors over
// ambiguous getByText, and serial mode (the tests share office/asset state
// and mutate transfer status across steps).
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + seeded admin (see
// CLAUDE.md). This spec compiles + lints here; CI runs it in the e2e job.
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
function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

test.describe('Transfer (Mutasi) — real backend (submit → approve → ship → receive e2e)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string
  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined

  let officeAId: string
  let officeAName: string
  let officeBId: string
  let officeBName: string
  let categoryId: string

  let asset1Id: string
  let asset1Name: string
  let asset2Id: string
  let asset2Name: string

  let transfer1ReqId: string

  // Creates an asset via the asset_create maker-checker flow (submit as
  // admin, approve as checker) and resolves its id. Mirrors assets.spec.ts's
  // beforeAll pattern; asset_class 'intangible' avoids the tangible-only
  // room_id CHECK constraint enforced at approval time.
  async function createApprovedAsset(name: string, cost: string): Promise<string> {
    const submitRes = await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create',
        amount: cost,
        office_id: officeAId,
        payload: {
          name, category_id: categoryId, office_id: officeAId,
          asset_class: 'intangible', purchase_cost: cost
        }
      }
    })
    const submitted = await apiJson<{ id: string }>(submitRes)
    const checkerToken = await login_(api, checkerEmail, checkerPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${submitted.id}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')

    const listRes = await api.get(`assets?search=${encodeURIComponent(name)}`, { headers: authHeader(adminToken) })
    const list = await apiJson<{ data: { id: string }[] }>(listRes)
    expect(list.data.length).toBe(1)
    return list.data[0]!.id
  }

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Transfer OT ${RUN}` }
    }))

    officeAName = `E2E Transfer Office A ${RUN}`
    officeAId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeAName, code: `E2ETA${RUN}`, office_type_id: ot.id }
    }))).id

    officeBName = `E2E Transfer Office B ${RUN}`
    officeBId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeBName, code: `E2ETB${RUN}`, office_type_id: ot.id }
    }))).id

    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Transfer Cat ${RUN}`, code: `E2ETC${RUN}`, asset_class: 'intangible' }
    }))).id

    // Checker user (SoD): a second Superadmin-scoped user so it is eligible to
    // decide (global data scope + request.decide) but is NOT the maker.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    checkerEmail = `e2e.transfer.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Transfer Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    asset1Name = `E2E Transfer Asset1 ${RUN}`
    asset2Name = `E2E Transfer Asset2 ${RUN}`
    asset1Id = await createApprovedAsset(asset1Name, '5000000')
    asset2Id = await createApprovedAsset(asset2Name, '5500000')
  })

  test.afterAll(async () => {
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('submit via UI creates a pending asset_transfer request', async ({ page }) => {
    await login(page)
    await page.goto('/transfers')
    await expect(page.getByRole('heading', { name: 'Mutasi Aset' })).toBeVisible({ timeout: 10_000 })

    const picker = page.getByTestId('transfer-asset-picker')
    await picker.getByTestId('asset-picker-input').fill(asset1Name)
    await picker.getByTestId('asset-picker-item').first().click()

    await pickAsync(page, 'to-office', officeBName, officeBName)

    await page.getByTestId('transfer-date').fill(todayISO())

    await page.getByTestId('transfer-condition').click()
    await page.getByRole('option', { name: 'Rusak Ringan', exact: true }).click()

    await page.getByTestId('transfer-submit').click()

    await expect(page.getByText(`Mutasi "${asset1Name}" ke ${officeBName} berhasil diajukan.`, { exact: true }))
      .toBeVisible({ timeout: 10_000 })

    const reqRes = await api.get('requests?type=asset_transfer&status=pending&limit=100', {
      headers: authHeader(adminToken)
    })
    const reqs = await apiJson<{ data: { id: string, target_id: string | null, office_id: string | null }[] }>(reqRes)
    const match = reqs.data.find(r => r.target_id === asset1Id)
    expect(match).toBeTruthy()
    expect(match?.office_id).toBe(officeAId)
    transfer1ReqId = match!.id
  })

  test('approve via API as checker → Riwayat shows Disetujui with Kirim → ship → Dalam Pengiriman', async ({ page }) => {
    const checkerToken = await login_(api, checkerEmail, checkerPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${transfer1ReqId}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')

    await login(page)
    await page.goto('/transfers')
    await page.getByTestId('transfer-tab-history').click()

    const row = page.locator('[data-testid="transfer-history-row"]', { hasText: asset1Name }).first()
    await expect(row).toBeVisible({ timeout: 10_000 })
    await expect(row.getByTestId('transfer-history-status')).toHaveText('Disetujui')

    const shipBtn = row.getByTestId('transfer-ship')
    await expect(shipBtn).toBeVisible()
    await shipBtn.click()

    await page.getByTestId('transfer-ship-confirm').click()

    await expect(page.getByText(`Aset "${asset1Name}" dikirim. Status diperbarui menjadi Dalam Pengiriman.`, { exact: true }))
      .toBeVisible({ timeout: 10_000 })
    await expect(row.getByTestId('transfer-history-status')).toHaveText('Dalam Pengiriman', { timeout: 10_000 })
  })

  test('Kotak Masuk: admin receives with a BAST number → Diterima, asset relocates to office B', async ({ page }) => {
    await login(page)
    await page.goto('/transfers')
    await page.getByTestId('transfer-tab-inbox').click()

    const card = page.locator('[data-testid="transfer-inbox-card"]', { hasText: asset1Name }).first()
    await expect(card).toBeVisible({ timeout: 10_000 })
    await expect(card.getByText('Rusak Ringan', { exact: true })).toBeVisible()

    await card.getByTestId('transfer-accept').click()

    const bastNo = `BAST/E2E/${RUN}`
    await page.getByTestId('transfer-accept-bast').fill(bastNo)
    await page.getByTestId('transfer-accept-confirm').click()

    await expect(page.getByText(
      `Aset "${asset1Name}" diterima. Lokasi & status diperbarui menjadi Tersedia di kantor Anda.`,
      { exact: true }
    )).toBeVisible({ timeout: 10_000 })

    await page.getByTestId('transfer-tab-history').click()
    const row = page.locator('[data-testid="transfer-history-row"]', { hasText: asset1Name }).first()
    await expect(row.getByTestId('transfer-history-status')).toHaveText('Diterima', { timeout: 10_000 })
    await expect(row).toContainText(bastNo)

    const asset = await apiJson<{ office_id: string }>(
      await api.get(`assets/${asset1Id}`, { headers: authHeader(adminToken) }))
    expect(asset.office_id).toBe(officeBId)
  })

  test('reject-receive: second asset is returned and stays at office A', async ({ page }) => {
    // Drive submit → approve → ship purely via API (the composable-equivalent
    // endpoints), leaving only the destination office's reject-receive
    // decision to be exercised through the UI.
    const submitted = await apiJson<{ request_id: string }>(await api.post('transfers', {
      headers: authHeader(adminToken),
      data: {
        asset_id: asset2Id,
        to_office_id: officeBId,
        condition_sent: 'baik',
        transfer_date: todayISO(),
        reason: `e2e reject-receive ${RUN}`
      }
    }))

    const checkerToken = await login_(api, checkerEmail, checkerPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${submitted.request_id}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')

    const trList = await apiJson<{ data: { id: string, status: string }[] }>(
      await api.get(`assets/${asset2Id}/transfers`, { headers: authHeader(adminToken) }))
    const openTransfer = trList.data.find(t => t.status === 'approved')
    expect(openTransfer).toBeTruthy()

    const shipped = await apiJson<{ status: string }>(await api.post(`transfers/${openTransfer!.id}/ship`, {
      headers: authHeader(adminToken), data: {}
    }))
    expect(shipped.status).toBe('in_transit')

    await login(page)
    await page.goto('/transfers')
    await page.getByTestId('transfer-tab-inbox').click()

    const card = page.locator('[data-testid="transfer-inbox-card"]', { hasText: asset2Name }).first()
    await expect(card).toBeVisible({ timeout: 10_000 })
    await card.getByTestId('transfer-reject-receive').click()

    await page.getByTestId('transfer-reject-note').fill(`kondisi tidak sesuai ${RUN}`)
    await page.getByTestId('transfer-reject-confirm').click()

    await expect(page.getByText(
      `Penerimaan "${asset2Name}" ditolak dan dikembalikan ke kantor asal.`,
      { exact: true }
    )).toBeVisible({ timeout: 10_000 })

    await page.getByTestId('transfer-tab-history').click()
    const row = page.locator('[data-testid="transfer-history-row"]', { hasText: asset2Name }).first()
    await expect(row.getByTestId('transfer-history-status')).toHaveText('Dikembalikan', { timeout: 10_000 })

    const asset = await apiJson<{ office_id: string }>(
      await api.get(`assets/${asset2Id}`, { headers: authHeader(adminToken) }))
    expect(asset.office_id).toBe(officeAId)
  })
})
