import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'
import { EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Notifications — real backend, full async pipeline. Exercises the in-app feed
// end to end: a maker (the seeded admin) submits an asset_create request via
// API, the fan-out delivers an `approval_pending` notification to the eligible
// approver (a second SoD checker user), the approver reads it in the UI and
// that read state survives a full page reload, then the approver approves the
// request and the maker receives the `approval_decided` notification.
//
// THE PIPELINE IS ASYNCHRONOUS. A notification is NOT written synchronously
// with the business transaction — it flows: business tx -> outbox -> relay
// (polls ~2s) -> Redis Stream -> consumer (polls ~2s) -> notifications row. So
// a notification can take ~4-5s to surface after the triggering action. The
// feed page fetches only on mount (no client polling), so waiting on a static
// page never surfaces a late arrival. `waitForFeedText` below therefore reloads
// the page and re-asserts via Playwright's auto-waiting `expect().toBeVisible`
// until the row appears — a not-yet-arrived notification is expected, not a
// failure. No fixed `waitForTimeout` sleep is used as the arrival mechanism.
//
// Robustness rules (per project e2e conventions): unique names/emails per run
// (this dev DB is NOT reset between runs); the checker is created fresh each
// run so its feed starts empty; assertions check PRESENCE, never exact counts,
// because this checker is a global-scope Superadmin and other specs running in
// parallel may fan additional `approval_pending` rows into its feed — that
// noise never breaks a presence assertion. Serial mode: the approve step in the
// second test consumes the pending request the first test reads, so order
// matters.
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + seeded admin AND
// the notification workers running (NOTIFICATION_WORKER_ENABLED, default true);
// see CLAUDE.md. Without the workers no notification is ever written and the
// waits below time out.
// ---------------------------------------------------------------------------

const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`
const RUN = `${Date.now()}`

// Rendered id-locale strings (default locale) the feed builds client-side from
// `type` + `params`. The message carries no run-specific token, so we match on
// the stable, type-identifying fragment rather than the whole sentence.
//   approval_pending -> "Pengajuan Registrasi Aset menunggu persetujuan Anda (tahap 1)"
//   approval_decided -> "Pengajuan Registrasi Aset Anda telah Disetujui"
const PENDING_TEXT = 'menunggu persetujuan Anda'
const DECIDED_TEXT = 'telah Disetujui'

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

// Parametrized UI login (same steps as helpers.ts `login`, generalized over
// credentials so this spec can sign in as the checker and, separately, as the
// maker). Each test gets its own browser context, so no cookie/localStorage
// clearing is needed between tests — the httpOnly-refresh-cookie restore that
// bites mid-test user switches only applies when switching users WITHIN one
// test, which this spec never does.
async function loginAs(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

// Reload-and-reassert until a feed row containing `text` is visible. The feed
// fetches on mount only, so each reload re-runs the page's `load()` and picks
// up whatever the async pipeline has since delivered. `toPass` retries the
// whole reload+assert unit until the row shows or the outer timeout elapses.
// Assumes the caller is already on `/notifications` (the 'all' tab, which a
// reload resets to, shows every row regardless of read state).
async function waitForFeedText(page: Page, text: string, timeout = 40_000): Promise<void> {
  const row = page.getByTestId('notifications-row').filter({ hasText: text }).first()
  await expect(async () => {
    await page.reload()
    await expect(row).toBeVisible({ timeout: 3_000 })
  }).toPass({ timeout })
}

test.describe('Notifications — real backend (async feed e2e)', () => {
  // Serial: the second test approves the request the first test reads, and the
  // reload timing budgets below can outlast the default per-test timeout.
  test.describe.configure({ mode: 'serial', timeout: 120_000 })

  let api: APIRequestContext
  let adminToken: string
  let checkerToken: string
  let checkerEmail: string
  let checkerPassword: string
  let checkerId: string | undefined
  let officeId: string
  let categoryId: string
  let requestId: string

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
      headers: authHeader(adminToken), data: { name: `E2E Notif OT ${RUN}` }
    }))
    const off = await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: `E2E Notif Office ${RUN}`, code: `E2ENO${RUN}`, office_type_id: ot.id }
    }))
    officeId = off.id
    const cat = await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Notif Cat ${RUN}`, code: `ENO${RUN}`, asset_class: 'intangible' }
    }))
    categoryId = cat.id

    // Checker user (SoD): a second Superadmin so it is eligible to decide
    // (global data scope + request.decide) but is NOT the maker. Created BEFORE
    // the submit below so the fan-out at submit time includes it.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    checkerEmail = `e2e.notif.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Notif Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id
    checkerToken = await login_(api, checkerEmail, checkerPassword)

    // Lowest threshold band -> a single office-level approval step, which the
    // global-scope checker is eligible to decide. Submitting as admin (the
    // maker) fans an `approval_pending` out to the checker (SoD excludes the
    // maker, so admin itself never receives one for this request).
    requestId = await submitCreate(`E2E Notif Asset ${RUN}`, '750000')
  })

  test.afterAll(async () => {
    // Best-effort cleanup: repeated local runs otherwise accumulate one checker
    // user per run. Same rationale as approval.spec.ts.
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('approver receives approval_pending and mark-read persists across reload', async ({ page }) => {
    await loginAs(page, checkerEmail, checkerPassword)
    await page.goto('/notifications')

    // 1) The async pipeline delivers the pending notification; reload-poll until
    //    its rendered i18n text shows in the feed.
    await waitForFeedText(page, PENDING_TEXT)
    const pendingRow = page.getByTestId('notifications-row').filter({ hasText: PENDING_TEXT }).first()
    await expect(pendingRow).toBeVisible()

    // 2) A fresh checker's feed starts unread, so the unread badge is present.
    await expect(page.getByTestId('notifications-unread-badge')).toBeVisible()

    // 3) Click the row: the feed marks it read server-side, then (the checker
    //    holds request.decide) navigates to /approval. The mark-read await
    //    completes before the navigation, so it is durably persisted.
    await pendingRow.click()
    await expect(page).toHaveURL(/\/approval$/)

    // 4) Back on the feed, the row now lives under the "Sudah dibaca" (read)
    //    filter — proof the mark-read persisted through the refetch. Presence,
    //    not count: parallel specs may add other (still-unread) rows, but only
    //    the row this session marked can appear on the read tab.
    await page.goto('/notifications')
    await page.getByTestId('notifications-tab-read').click()
    await expect(page.getByTestId('notifications-row').filter({ hasText: PENDING_TEXT }).first())
      .toBeVisible({ timeout: 10_000 })

    // 5) Full browser reload (filter resets to 'all'); re-open the read tab and
    //    confirm the notification is STILL read. This is the core criterion:
    //    mark-read persisted to the backend, not just to in-memory state.
    await page.reload()
    await page.getByTestId('notifications-tab-read').click()
    await expect(page.getByTestId('notifications-row').filter({ hasText: PENDING_TEXT }).first())
      .toBeVisible({ timeout: 10_000 })
  })

  test('maker receives approval_decided after the approver approves', async ({ page }) => {
    // Approve the request via API as the checker (deterministic; the UI approve
    // path is covered by approval.spec.ts). This enqueues `request_decided`,
    // which the consumer fans out to the maker as `approval_decided`.
    await apiJson(await api.post(`requests/${requestId}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve', note: `ok e2e ${RUN}` }
    }))

    // The maker (seeded admin) waits for the second event type end to end. The
    // just-created row is the newest, so it lands on page 1 of the feed.
    await loginAs(page, EMAIL, PASSWORD)
    await page.goto('/notifications')
    await waitForFeedText(page, DECIDED_TEXT)
    await expect(page.getByTestId('notifications-row').filter({ hasText: DECIDED_TEXT }).first())
      .toBeVisible()
  })
})
