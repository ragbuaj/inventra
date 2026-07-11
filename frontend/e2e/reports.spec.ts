import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Reports — real backend (`/reports` wired to /api/v1/reports/:type[/export]).
// The 7-report builder (assets / depreciation / utilization / maintenance /
// transfers / disposals / opname) plus the report.export permission gate.
//
//   beforeAll (API): office-type → office → floor → room → category (tangible)
//   → a second Superadmin checker (SoD) → one approved tangible asset with a
//   known purchase_cost + tag → one COMPLETED maintenance record for that
//   asset (completed today, a high cost so it sorts near the top of the cost-
//   ordered maintenance report) → a Staf user (report.view but NOT
//   report.export) at the office for the permission-gate scenario.
//
//   1. assets: default card → Terapkan → the created asset's tag row appears,
//      the TOTAL footer renders, and PDF + Excel exports fire real downloads.
//   2. maintenance: switch card → Terapkan → the asset row appears and the
//      TOTAL footer shows an "Rp" total (the seeded completed record).
//   3. depreciation: switch card → Terapkan → rows OR the empty state (never
//      the error state); toggling the fiscal basis re-applies without error.
//   4. Staf permission: create a Staf user, relogin (clearCookies + localStorage
//      per the httpOnly-refresh-cookie convention) → /reports loads (report.view)
//      but the export buttons are absent (no report.export).
//   5. disposals GL: the "Rekap Jurnal GL" recap button shows only on the
//      disposals card, not on the assets card.
//
// Robustness (project e2e conventions): unique name+code per run (this dev DB
// is NOT reset between runs — never destructive cleanup), assert-after-search
// scoped to this run's unique asset tag/name, wait-for-visible over sleeps,
// and serial mode (scenario 4 swaps the logged-in user).
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
async function loginAs(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}
function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

test.describe('Reports — real backend e2e', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string

  let officeId: string
  let roomId: string
  let categoryId: string

  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined

  let stafEmail: string
  let stafPassword: string
  let stafId: string | undefined

  let assetId: string
  let assetTag: string
  const assetName = `E2E Report Asset ${RUN}`

  async function createApprovedAsset(name: string, cost: string): Promise<{ id: string, tag: string }> {
    const submitted = await apiJson<{ id: string }>(await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create', amount: cost, office_id: officeId,
        payload: {
          name, category_id: categoryId, office_id: officeId, room_id: roomId,
          asset_class: 'tangible', purchase_cost: cost, purchase_date: '2026-07-01'
        }
      }
    }))
    const checkerToken = await login_(api, checkerEmail, checkerPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${submitted.id}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')
    const list = await apiJson<{ data: { id: string, asset_tag: string }[] }>(
      await api.get(`assets?search=${encodeURIComponent(name)}`, { headers: authHeader(adminToken) }))
    expect(list.data.length).toBe(1)
    return { id: list.data[0]!.id, tag: list.data[0]!.asset_tag }
  }

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Report OT ${RUN}` }
    }))
    officeId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: `E2E Report Office ${RUN}`, code: `E2ERO${RUN}`, office_type_id: ot.id }
    }))).id
    const floor = await apiJson<{ id: string }>(await api.post('floors', {
      headers: authHeader(adminToken), data: { office_id: officeId, name: `E2E Report Floor ${RUN}` }
    }))
    roomId = (await apiJson<{ id: string }>(await api.post('rooms', {
      headers: authHeader(adminToken), data: { floor_id: floor.id, name: `E2E Report Room ${RUN}` }
    }))).id
    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Report Cat ${RUN}`, code: `E2ERC${RUN}`, asset_class: 'tangible' }
    }))).id

    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    const staf = roles.data.find(r => r.name === 'Staf')
    if (!staf) throw new Error('Staf role not found in GET /authz/roles')

    checkerEmail = `e2e.report.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Report Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    // Low amount so asset_create clears in a single office-level approval step
    // (higher bands need multiple approvers — proven single-step at 4M elsewhere).
    const a = await createApprovedAsset(assetName, '4000000')
    assetId = a.id
    assetTag = a.tag

    // Completed maintenance record (high cost → near the top of the cost-ordered
    // maintenance report; completed today → inside the default this-quarter window).
    await apiJson(await api.post('maintenance/records', {
      headers: authHeader(adminToken),
      data: {
        asset_id: assetId, type: 'corrective', status: 'completed',
        completed_date: todayISO(), cost: '88000000',
        description: `E2E report maintenance ${RUN}`
      }
    }))

    // Staf user (report.view, no report.export) for the permission-gate scenario.
    stafEmail = `e2e.report.staf.${RUN}@inventra.local`
    stafPassword = `Staf${RUN}!`
    stafId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Report Staf ${RUN}`, email: stafEmail, password: stafPassword, role_id: staf.id, office_id: officeId }
    }))).id
  })

  test.afterAll(async () => {
    for (const id of [checkerId, stafId]) {
      if (id) await api.delete(`users/${id}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('assets report: apply → asset row + TOTAL footer + PDF/Excel downloads', async ({ page }) => {
    await login(page)
    await page.goto('/reports')
    await expect(page.getByRole('heading', { name: 'Laporan' })).toBeVisible({ timeout: 10_000 })

    // 'assets' is the default active card. Apply and assert this run's asset row.
    await page.getByTestId('reports-apply').click()
    await expect(page.getByText(assetTag, { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('TOTAL', { exact: true })).toBeVisible()

    const [pdf] = await Promise.all([
      page.waitForEvent('download'),
      page.getByTestId('reports-export-pdf').click()
    ])
    expect(pdf.suggestedFilename()).toMatch(/\.pdf$/)

    const [xlsx] = await Promise.all([
      page.waitForEvent('download'),
      page.getByTestId('reports-export-xlsx').click()
    ])
    expect(xlsx.suggestedFilename()).toMatch(/\.xlsx$/)
  })

  test('maintenance report: apply → asset row + Rp TOTAL', async ({ page }) => {
    await login(page)
    await page.goto('/reports')
    await page.getByTestId('reports-card-maintenance').click()
    await page.getByTestId('reports-apply').click()

    await expect(page.getByText(assetName, { exact: true })).toBeVisible({ timeout: 10_000 })
    // The footer TOTAL row shows the aggregated cost as an Rp value.
    const footerTotal = page.locator('tfoot', { hasText: 'TOTAL' })
    await expect(footerTotal).toContainText('Rp')
  })

  test('depreciation report: apply → no error; fiscal basis toggle re-applies', async ({ page }) => {
    await login(page)
    await page.goto('/reports')
    await page.getByTestId('reports-card-depreciation').click()
    await page.getByTestId('reports-apply').click()

    // Either rows or the empty state — but never the retryable error state.
    await expect(page.getByTestId('reports-retry')).toHaveCount(0, { timeout: 10_000 })

    // Toggle to the fiscal basis and re-apply; still no error.
    await page.getByTestId('reports-basis-fiscal').click()
    await page.getByTestId('reports-apply').click()
    await expect(page.getByTestId('reports-retry')).toHaveCount(0, { timeout: 10_000 })
  })

  test('disposals GL recap button shows only on the disposals card', async ({ page }) => {
    await login(page)
    await page.goto('/reports')

    // Disposals card → apply → the GL recap button is present.
    await page.getByTestId('reports-card-disposals').click()
    await page.getByTestId('reports-apply').click()
    await expect(page.getByTestId('reports-export-gl')).toBeVisible({ timeout: 10_000 })

    // Switch to the assets card → apply → the GL recap button is gone.
    await page.getByTestId('reports-card-assets').click()
    await page.getByTestId('reports-apply').click()
    await expect(page.getByText(assetTag, { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('reports-export-gl')).toHaveCount(0)
  })

  test('Staf permission: /reports loads (report.view) but export buttons are absent', async ({ page }) => {
    // Drop any existing session first — the httpOnly refresh cookie would restore it.
    await page.goto('/login')
    await page.context().clearCookies()
    await page.evaluate(() => window.localStorage.clear())
    await loginAs(page, stafEmail, stafPassword)

    await page.goto('/reports')
    await expect(page.getByRole('heading', { name: 'Laporan' })).toBeVisible({ timeout: 10_000 })

    // Staf can run a report (report.view) but must not see any export control.
    await page.getByTestId('reports-apply').click()
    await expect(page.getByTestId('reports-loading')).toHaveCount(0, { timeout: 10_000 })
    await expect(page.getByTestId('reports-export-pdf')).toHaveCount(0)
    await expect(page.getByTestId('reports-export-xlsx')).toHaveCount(0)
    await expect(page.getByTestId('reports-export-gl')).toHaveCount(0)
  })
})
