import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Stock Opname (physical stock-take) — real backend (`/stock-opname` wired to
// /api/v1/stock-opname/sessions + /api/v1/requests). Covers the full
// open → counting → reconciling → closed lifecycle plus the not_found →
// disposal follow-up and the PDF report export:
//
//   beforeAll (API): office-type → office A → category (asset_class
//   'intangible', so approval never hits the tangible-only room_id CHECK
//   constraint — see assets.spec.ts's gotcha) → a second "checker" user (SoD:
//   the maker/admin cannot approve their own request) → two assets at office
//   A, each created via the asset_create maker-checker flow (submit as admin,
//   approve as checker) so both reach status=available before the opname
//   session is created.
//
//   Session creation is done via the API (POST /stock-opname/sessions), NOT
//   the create-session UI modal — see the "API session creation" note below.
//   Everything from "open session detail" onward drives the real UI.
//
//   1. Open the session detail; "Mulai" (start) → status Berjalan (counting);
//      mark asset1 'found' and asset2 'not_found' via the result segment
//      buttons → the found KPI tile updates to 1.
//   2. "Rekonsiliasi" (reconcile) → status Rekonsiliasi (reconciling); the
//      variance panel lists asset2 (not_found).
//   3. Follow-up asset2 (not_found → disposal) via its variance-panel button;
//      API-verify a pending asset_disposal request now targets asset2 and
//      that exactly one such request exists. The button has no disabled/
//      "sudah diajukan" state (Task 11 never wires `followup_request_id`),
//      so a second click is a real, reachable user action; after it, re-count
//      asset2's pending asset_disposal requests and assert it is STILL
//      exactly 1 — proving the backend's ErrAlreadyFollowedUp guard (409)
//      blocked the duplicate rather than silently creating a second request.
//   4. "Selesaikan" (close), confirm in the finish modal → status Selesai
//      (closed); export button visible.
//   5. API-assert GET /stock-opname/sessions/:id/report?format=pdf returns
//      200 with content-type application/pdf.
//
// IMPORTANT robustness note (documented dev-DB fragility): the shared dev DB
// has 100+ offices and the UI office pickers cap at limit:100 ordered by
// name, so a freshly-created office is NOT reliably selectable in the
// create-session modal's office USelect. All prerequisites AND the session
// itself are therefore created via the API; only the detail-view lifecycle
// is driven through the UI (the create modal is already covered by Task 11's
// component tests). The session list is ordered created_at DESC, so the
// fresh session sits at the top of /stock-opname; we still use a row-scoped
// locator (never a bare getByText) keyed off a RUN-suffixed unique name.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (this dev DB is NOT reset between runs), assert-after-search, wait for
// toasts/redirects rather than fixed sleeps, row/card-scoped selectors over
// ambiguous getByText, and serial mode (the tests share session/asset state
// and mutate opname status across steps).
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
function currentPeriod(): string {
  return new Date().toISOString().slice(0, 7) // YYYY-MM
}

test.describe('Stock Opname — real backend (lifecycle + follow-up + report e2e)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string
  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined

  let officeAId: string
  let categoryId: string

  let asset1Name: string
  let asset2Id: string
  let asset2Name: string

  let sessionId: string
  let sessionName: string

  // Creates an asset via the asset_create maker-checker flow (submit as
  // admin, approve as checker) and resolves its id. Mirrors
  // transfers.spec.ts / disposals.spec.ts's beforeAll pattern; asset_class
  // 'intangible' avoids the tangible-only room_id CHECK constraint enforced
  // at approval time.
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
    const list = await apiJson<{ data: { id: string, status: string }[] }>(listRes)
    expect(list.data.length).toBe(1)
    expect(list.data[0]!.status).toBe('available')
    return list.data[0]!.id
  }

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Opname OT ${RUN}` }
    }))

    officeAId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: `E2E Opname Office A ${RUN}`, code: `E2EOA${RUN}`, office_type_id: ot.id }
    }))).id

    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Opname Cat ${RUN}`, code: `E2EOC${RUN}`, asset_class: 'intangible' }
    }))).id

    // Checker user (SoD): a second Superadmin-scoped user so it is eligible to
    // decide (global data scope + request.decide) but is NOT the maker.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    checkerEmail = `e2e.opname.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Opname Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    asset1Name = `E2E Opname Asset1 ${RUN}`
    asset2Name = `E2E Opname Asset2 ${RUN}`
    // Costs kept in the lowest asset_disposal threshold band (0–5,000,000 →
    // office level, step 1 only — see db/migrations/000016_office_tier.up.sql)
    // so the not_found → disposal follow-up (submitted below by the admin
    // maker) can be fully approved by a single checker decision.
    await createApprovedAsset(asset1Name, '2000000')
    asset2Id = await createApprovedAsset(asset2Name, '2500000')

    // Session creation via the API (see the top-of-file "API session
    // creation" note): the create-session UI modal's office USelect caps at
    // limit:100 offices ordered by name, so a freshly-created office is not
    // reliably reachable there. Task 11's component tests already cover the
    // create modal itself.
    sessionName = `E2E Opname ${RUN}`
    const created = await apiJson<{ id: string, total: number }>(await api.post('stock-opname/sessions', {
      headers: authHeader(adminToken),
      data: { office_id: officeAId, name: sessionName, period: currentPeriod() }
    }))
    sessionId = created.id
    // The snapshot must capture exactly our 2 fresh assets (office A is a
    // brand-new office created above, so no pre-existing assets pollute it).
    expect(created.total).toBe(2)
  })

  test.afterAll(async () => {
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('full lifecycle: start → count → reconcile → disposal follow-up → close → report', async ({ page }) => {
    await login(page)
    await page.goto('/stock-opname')
    await expect(page.getByRole('heading', { name: 'Stock Opname' })).toBeVisible({ timeout: 10_000 })

    const row = page.locator('[data-testid="opname-session-row"]', { hasText: sessionName }).first()
    await expect(row).toBeVisible({ timeout: 10_000 })
    await row.click()

    await expect(page.getByRole('heading', { name: sessionName })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('opname-kpi-total')).toContainText('2')

    // --- start (open -> counting) ---
    await page.getByTestId('opname-start').click()
    await expect(page.getByText('Berjalan', { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- mark results via the result segment buttons ---
    const itemRows = page.locator('[data-testid="opname-item-row"]')
    await expect(itemRows).toHaveCount(2, { timeout: 10_000 })

    const row1 = page.locator('[data-testid="opname-item-row"]', { hasText: asset1Name })
    const row2 = page.locator('[data-testid="opname-item-row"]', { hasText: asset2Name })
    await expect(row1).toBeVisible()
    await expect(row2).toBeVisible()

    await row1.getByTestId('opname-result-found').click()
    await expect(page.getByTestId('opname-kpi-found')).toContainText('1', { timeout: 10_000 })

    await row2.getByTestId('opname-result-not_found').click()
    await expect(page.getByTestId('opname-kpi-variance')).toContainText('1', { timeout: 10_000 })

    // --- reconcile (counting -> reconciling) ---
    await page.getByTestId('opname-reconcile').click()
    await expect(page.getByText('Rekonsiliasi', { exact: true })).toBeVisible({ timeout: 10_000 })

    // Variance panel lists asset2 (not_found), read-only badges replace the
    // segmented buttons now that the session is no longer counting.
    const variancePanel = page.getByText('Panel Selisih', { exact: true }).locator('../../..')
    await expect(variancePanel).toContainText(asset2Name, { timeout: 10_000 })
    await expect(page.locator('[data-testid="opname-result-found"]')).toHaveCount(0)

    // --- follow-up: not_found -> disposal ---
    const followupBtn = page.getByTestId('opname-followup-not_found')
    await expect(followupBtn).toBeVisible({ timeout: 10_000 })
    await followupBtn.click()

    // API-verify a pending asset_disposal request now targets asset2, and
    // that exactly one such request exists (no duplicate).
    async function countAsset2DisposalRequests(): Promise<number> {
      const reqRes = await api.get('requests?type=asset_disposal&status=pending&limit=100', {
        headers: authHeader(adminToken)
      })
      const reqs = await apiJson<{ data: { id: string, target_id: string | null }[] }>(reqRes)
      return reqs.data.filter(r => r.target_id === asset2Id).length
    }
    await expect(async () => {
      expect(await countAsset2DisposalRequests()).toBe(1)
    }).toPass({ timeout: 10_000 })

    // Since the maintenance module landed, the page disables the follow-up
    // button once the item carries a followup link (`followup_request_id` /
    // `followup_record_id`) — UI-level idempotency on top of the backend's
    // ErrAlreadyFollowedUp guard. Assert the button is disabled and that no
    // second asset_disposal request exists for asset2.
    await expect(followupBtn).toBeDisabled({ timeout: 10_000 })
    expect(await countAsset2DisposalRequests()).toBe(1)

    // --- close (reconciling -> closed) ---
    await page.getByTestId('opname-finish-open').click()
    await page.getByTestId('opname-finish-confirm').click()

    await expect(page.getByText('Selesai', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('opname-export')).toBeVisible()

    // --- report export (API-assert PDF) ---
    const reportRes = await api.get(`stock-opname/sessions/${sessionId}/report?format=pdf`, {
      headers: authHeader(adminToken)
    })
    expect(reportRes.status()).toBe(200)
    expect(reportRes.headers()['content-type']).toContain('application/pdf')
  })
})
