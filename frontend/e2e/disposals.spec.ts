import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, EMAIL, PASSWORD, clickRowAction } from './helpers'

// ---------------------------------------------------------------------------
// Disposal (Penghapusan) — real backend (`/disposals` wired to
// /api/v1/disposals + /api/v1/requests). Covers submit → approve → BAST
// attach, plus a negative history search:
//
//   beforeAll (API): office-type → office → category (asset_class
//   'intangible', so approval never hits the tangible-only room_id CHECK
//   constraint — see assets.spec.ts's gotcha) → a second "checker" user (SoD)
//   → one asset created via the asset_create maker-checker flow with
//   purchase_cost inside the LOWEST asset_disposal threshold band (see
//   db/migrations/000016_office_tier.up.sql: 0–5,000,000 → office level,
//   step 1 only — a single checker approval completes it).
//
//   1. Submit via UI (/disposals → Ajukan tab): pick the asset → the
//      valuation summary renders Perolehan (acquisition cost) with fiscal
//      book value always "—" → the approval-chain card renders ≥1 step
//      ("berdasar nilai buku …") → method "Dijual" → proceeds/date →
//      submit → post-submit view with the approval timeline (maker done,
//      step 1 "Menunggu").
//   2. Approve via API as the checker (decision: 'approve') → UI Riwayat:
//      the row shows "Selesai" with the method badge; API-verify the asset's
//      status flips to 'disposed'.
//   3. Lampirkan BAST: attach a BAST document (with a small generated file)
//      to the Selesai row → toast; API-verify a `bast_disposal` asset
//      document now exists.
//   4. Negative: history search for a nonsense string → empty state.
//
// IMPORTANT gotcha (discovered against the real backend, not assumed): a
// freshly created asset's `book_value` column is never populated at creation
// (see db/queries/assets.sql's CreateAsset — book_value/
// accumulated_depreciation aren't insert columns). Since the depreciation
// module landed, GET /assets/:id/depreciation's `computed_book_value` now
// falls back to the asset's raw `purchase_cost` whenever it has no
// depreciation entries yet (see depreciation.Service.BookValueAsOf) — so for
// this spec's never-computed asset, the commercial book value the disposal
// screen uses is simply the acquisition cost. That means the approval-chain
// card's amount note now always reads "berdasar nilai buku" (book value),
// never "berdasar nilai perolehan" — the two coincide numerically here but
// the label is unconditionally book-value-based. The fiscal book value stays
// "—" (no fiscal-basis entries exist for an uncomputed asset), so the fiscal
// valuation/gain-loss cells are unaffected.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (this dev DB is NOT reset between runs), assert-after-search, wait for
// toasts/redirects rather than fixed sleeps, row-scoped selectors over
// ambiguous getByText, and serial mode (the tests share the same asset and
// mutate its disposal status across steps).
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

test.describe('Disposal (Penghapusan) — real backend (submit → approve → BAST e2e)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string
  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined

  let officeId: string
  let categoryId: string

  let assetId: string
  let assetName: string
  const assetCost = '2000000' // inside band 1 (0–5,000,000 → office level, step 1 only)

  let disposalReqId: string

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Disposal OT ${RUN}` }
    }))
    officeId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: `E2E Disposal Office ${RUN}`, code: `E2EDO${RUN}`, office_type_id: ot.id }
    }))).id
    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Disposal Cat ${RUN}`, code: `E2EDC${RUN}`, asset_class: 'intangible' }
    }))).id

    // Checker user (SoD): a second Superadmin-scoped user so it is eligible to
    // decide (global data scope + request.decide) but is NOT the maker.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    checkerEmail = `e2e.disposal.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Disposal Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    // Asset via the asset_create maker-checker flow (submit as admin, approve
    // as checker) — asset_class 'intangible' avoids the tangible-only room_id
    // CHECK constraint enforced at approval time.
    assetName = `E2E Disposal Asset ${RUN}`
    const submitted = await apiJson<{ id: string }>(await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create',
        amount: assetCost,
        office_id: officeId,
        payload: {
          name: assetName, category_id: categoryId, office_id: officeId,
          asset_class: 'intangible', purchase_cost: assetCost
        }
      }
    }))
    const checkerToken = await login_(api, checkerEmail, checkerPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${submitted.id}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')

    const list = await apiJson<{ data: { id: string }[] }>(
      await api.get(`assets?search=${encodeURIComponent(assetName)}`, { headers: authHeader(adminToken) }))
    expect(list.data.length).toBe(1)
    assetId = list.data[0]!.id
  })

  test.afterAll(async () => {
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('submit via UI: valuation + approval chain render, then the request goes pending', async ({ page }) => {
    await login(page)
    await page.goto('/disposals')
    await expect(page.getByRole('heading', { name: 'Penghapusan Aset' })).toBeVisible({ timeout: 10_000 })

    // The page lands on Riwayat; open the "Ajukan Penghapusan" form (full-view swap).
    await page.getByTestId('disposal-create').click()

    const picker = page.getByTestId('disposal-asset-picker')
    await picker.getByTestId('asset-picker-input').fill(assetName)
    await picker.getByTestId('asset-picker-item').first().click()

    const valuation = page.getByTestId('disposal-valuation')
    await expect(valuation).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('disposal-valuation-acquisition')).toContainText('2.000.000')
    await expect(page.getByTestId('disposal-valuation-book-fiscal')).toHaveText('—')

    const chainCard = page.getByTestId('disposal-chain-card')
    await expect(chainCard).toBeVisible()
    await expect(chainCard).toContainText('berdasar nilai buku')
    const chainSteps = page.getByTestId('disposal-chain-steps')
    await expect(chainSteps).toBeVisible({ timeout: 10_000 })

    await page.getByTestId('disposal-method').click()
    await page.getByRole('option', { name: 'Dijual', exact: true }).click()

    await page.getByTestId('disposal-proceeds').fill('2500000')
    await page.getByTestId('disposal-date').fill(todayISO())

    await page.getByTestId('disposal-submit').click()

    await expect(page.getByTestId('disposal-submitted-status')).toHaveText('Menunggu Approval', { timeout: 10_000 })

    const timelineRows = page.locator('[data-testid="disposal-timeline-row"]')
    await expect(timelineRows).toHaveCount(2, { timeout: 10_000 })
    await expect(timelineRows.nth(0)).toHaveAttribute('data-status', 'done')
    await expect(timelineRows.nth(1)).toHaveAttribute('data-status', 'current')
    await expect(timelineRows.nth(1)).toContainText('Menunggu')

    const reqRes = await api.get('requests?type=asset_disposal&status=pending&limit=100', {
      headers: authHeader(adminToken)
    })
    const reqs = await apiJson<{ data: { id: string, target_id: string | null }[] }>(reqRes)
    const match = reqs.data.find(r => r.target_id === assetId)
    expect(match).toBeTruthy()
    disposalReqId = match!.id
  })

  test('approve via API as checker → Riwayat shows Selesai; asset flips to disposed', async ({ page }) => {
    const checkerToken = await login_(api, checkerEmail, checkerPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${disposalReqId}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')

    await login(page)
    await page.goto('/disposals')

    const row = page.locator('[data-testid="disposal-history-row"]', { hasText: assetName }).first()
    await expect(row).toBeVisible({ timeout: 10_000 })
    await expect(row.getByText('Selesai', { exact: true })).toBeVisible()
    await expect(row.getByText('Dijual', { exact: true })).toBeVisible()

    const asset = await apiJson<{ status: string }>(
      await api.get(`assets/${assetId}`, { headers: authHeader(adminToken) }))
    expect(asset.status).toBe('disposed')
  })

  test('Lampirkan BAST: attaching a document creates a bast_disposal asset document', async ({ page }) => {
    await login(page)
    await page.goto('/disposals')

    const row = page.locator('[data-testid="disposal-history-row"]', { hasText: assetName }).first()
    await expect(row).toBeVisible({ timeout: 10_000 })
    // Attach-BAST action moved into the shared RowActionsMenu kebab (⋮).
    await clickRowAction(page, row, 'Lampirkan BAST Penghapusan')

    const bastNo = `BAP/E2E/${RUN}`
    await page.getByTestId('disposal-attach-bast-no').fill(bastNo)
    await page.getByTestId('disposal-attach-file').setInputFiles({
      name: `bast-${RUN}.txt`,
      mimeType: 'text/plain',
      buffer: Buffer.from(`e2e bast disposal content ${RUN}`)
    })
    await page.getByTestId('disposal-attach-confirm').click()

    await expect(page.getByText(`Lampiran BAST untuk "${assetName}" berhasil disimpan.`, { exact: true }))
      .toBeVisible({ timeout: 10_000 })

    const docs = await apiJson<{ data: { doc_type: string, doc_no: string | null }[] }>(
      await api.get(`assets/${assetId}/documents`, { headers: authHeader(adminToken) }))
    const bastDoc = docs.data.find(d => d.doc_type === 'bast_disposal')
    expect(bastDoc).toBeTruthy()
  })

  test('Riwayat: searching a non-existent string shows the empty state', async ({ page }) => {
    await login(page)
    await page.goto('/disposals')

    await page.getByTestId('disposal-history-search').fill(`nonexistent-zzz-${RUN}`)

    await expect(page.getByText('Belum ada riwayat', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Belum ada pengajuan penghapusan yang cocok.', { exact: true })).toBeVisible()
  })
})
