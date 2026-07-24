import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'
import { EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Approval — real backend (Pengajuan & Approval `/approval` screen wired to
// /api/v1/requests). Flow:
//   beforeAll (API): office-type → office → category; a second SoD checker
//   user (Superadmin role); submit THREE asset_create requests (amounts in
//   the lowest threshold band — single office-level approval step), then
//   cancel the third one as the maker (admin).
//   UI as checker: inbox shows the two pending cards → open detail (Data
//   section renders from payload) → approve #1 with a note (timeline +
//   green result banner) → reject #2 (red result banner). Then check the
//   Cancelled tab shows #3 with the neutral "cancelled by requester" banner,
//   and the Approved tab shows #1 no longer sitting in the pending inbox.
//
// IMPORTANT gotcha: all three requests share one office (so the inbox cards
// are visibly correlated to this run), which means two cards in the pending
// tab both carry the same "type · office" title — cards don't show the asset
// name, only the Data section in the detail pane does after a click. The
// find-card-by-detail loops below click each candidate card and check the
// detail pane for the expected asset name before acting on it.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (this dev DB is NOT reset between runs), assert-after-search, wait for
// toasts/text rather than fixed sleeps, and serial mode — approving/rejecting
// a request changes which tab its card sits in, so test order matters.
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

// `helpers.ts`'s `login(page)` only signs in the fixed seeded admin (no
// email/password params) — this spec needs to sign in as the checker user
// created in `beforeAll`, so it uses its own parametrized UI-login helper
// (same steps as helpers.ts, generalized over credentials).
async function loginAs(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login')
  await page.locator('input[name="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

// `Locator.isVisible({ timeout })` does NOT wait — Playwright's own types mark
// the `timeout` option deprecated/ignored there, so it returns immediately
// based on whatever's in the DOM *right now*. Right after a card click, the
// detail pane is still mid-fetch (`detail.value = await api.get(id)`), so an
// immediate isVisible() check races the async load and can wrongly report
// "not this card". `waitFor` actually polls up to its timeout, so use that
// instead for the find-card-by-detail loops below.
async function detailShows(page: Page, text: string, timeout = 5_000): Promise<boolean> {
  return page.locator('text=' + text).first().waitFor({ state: 'visible', timeout })
    .then(() => true)
    .catch(() => false)
}

test.describe('Approval — real backend (inbox + decide e2e)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string
  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined
  let officeId: string
  let officeName: string
  let categoryId: string
  let approveName: string
  let rejectName: string
  let cancelName: string
  let cancelReqId: string

  async function submitCreate(name: string, cost: string): Promise<string> {
    const res = await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create',
        amount: cost,
        office_id: officeId,
        payload: {
          name, category_id: categoryId, office_id: officeId,
          asset_class: 'intangible', purchase_cost: cost
        }
      }
    })
    return (await apiJson<{ id: string }>(res)).id
  }

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Appr OT ${RUN}` }
    }))
    officeName = `E2E Appr Office ${RUN}`
    const off = await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeName, code: `E2EAP${RUN}`, office_type_id: ot.id }
    }))
    officeId = off.id
    const cat = await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Appr Cat ${RUN}`, code: `EAP${RUN}`, asset_class: 'intangible' }
    }))
    categoryId = cat.id

    // Checker user (SoD): a second Superadmin-scoped user so it is eligible to
    // decide (global data scope + request.decide) but is NOT the maker.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    checkerEmail = `e2e.appr.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Appr Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    approveName = `E2E Appr Laptop ${RUN}`
    rejectName = `E2E Appr Printer ${RUN}`
    cancelName = `E2E Appr Cancelled ${RUN}`
    await submitCreate(approveName, '750000')
    await submitCreate(rejectName, '850000')
    cancelReqId = await submitCreate(cancelName, '650000')
    await apiJson(await api.post(`requests/${cancelReqId}/cancel`, {
      headers: authHeader(adminToken), data: {}
    }))
  })

  test.afterAll(async () => {
    // Best-effort cleanup: repeated local runs otherwise accumulate one
    // checker user per run (see assets.spec.ts for the same rationale).
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('checker inbox lists the pending requests with maker + office names', async ({ page }) => {
    await loginAs(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    const approveCard = page.locator('[data-testid="approval-card"]', { hasText: officeName }).first()
    await expect(approveCard).toBeVisible({ timeout: 10_000 })
  })

  test('detail renders the payload Data section and approve works with a note', async ({ page }) => {
    await loginAs(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    // Two cards from this run share the office name; open each and check the
    // detail pane's Data section for the expected asset name (cards only
    // carry a "type · office" title, not the asset name).
    const cards = page.locator('[data-testid="approval-card"]', { hasText: officeName })
    await expect(cards.first()).toBeVisible({ timeout: 10_000 })
    const n = await cards.count()
    let found = false
    for (let i = 0; i < n; i++) {
      await cards.nth(i).click()
      if (await detailShows(page, approveName)) {
        found = true
        break
      }
    }
    expect(found).toBe(true)

    await page.getByTestId('approval-note').fill(`ok e2e ${RUN}`)
    await page.getByTestId('approval-approve').click()
    await expect(page.locator('text=Disetujui oleh').first()).toBeVisible({ timeout: 10_000 })
    // Timeline shows the checker's decision note.
    await expect(page.locator(`text=ok e2e ${RUN}`)).toBeVisible()
  })

  test('reject flow renders the red result banner', async ({ page }) => {
    await loginAs(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    const cards = page.locator('[data-testid="approval-card"]', { hasText: officeName })
    await expect(cards.first()).toBeVisible({ timeout: 10_000 })
    const n = await cards.count()
    let found = false
    for (let i = 0; i < n; i++) {
      await cards.nth(i).click()
      if (await detailShows(page, rejectName)) {
        found = true
        break
      }
    }
    expect(found).toBe(true)
    await page.getByTestId('approval-reject').click()
    await expect(page.locator('text=Ditolak oleh').first()).toBeVisible({ timeout: 10_000 })
  })

  test('cancelled request appears under the Cancelled tab with neutral banner', async ({ page }) => {
    await loginAs(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    await page.getByTestId('approval-tab-cancelled').click()
    const cards = page.locator('[data-testid="approval-card"]', { hasText: officeName })
    await expect(cards.first()).toBeVisible({ timeout: 10_000 })
    const n = await cards.count()
    let found = false
    for (let i = 0; i < n; i++) {
      await cards.nth(i).click()
      if (await detailShows(page, cancelName)) {
        found = true
        break
      }
    }
    expect(found).toBe(true)
    await expect(page.locator('text=Dibatalkan oleh pengaju').first()).toBeVisible({ timeout: 10_000 })
  })

  test('approved request no longer sits in the pending inbox', async ({ page }) => {
    await loginAs(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    await page.getByTestId('approval-tab-approved').click()
    const approvedCards = page.locator('[data-testid="approval-card"]', { hasText: officeName })
    await expect(approvedCards.first()).toBeVisible({ timeout: 10_000 })
    const n = await approvedCards.count()
    let found = false
    for (let i = 0; i < n; i++) {
      await approvedCards.nth(i).click()
      if (await detailShows(page, approveName)) {
        found = true
        break
      }
    }
    expect(found).toBe(true)

    // Both this run's requests (approve + reject) have now been decided, and
    // the cancelled one never entered the pending inbox — no card from this
    // run's office should remain in the Pending tab.
    await page.getByTestId('approval-tab-pending').click()
    await expect(page.locator('[data-testid="approval-card"]', { hasText: officeName })).toHaveCount(0)
  })
})
