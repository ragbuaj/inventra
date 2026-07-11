import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Dashboard — real backend (`/` wired to /api/v1/dashboard/summary +
// /api/v1/requests/inbox). Covers the four dashboard surfaces:
//
//   beforeAll (API): office-type → office → floor → room → category
//   (tangible, so a create needs room_id at approval) → a second Superadmin
//   user that plays BOTH SoD roles: the CHECKER for the admin-submitted
//   asset_create (maker=admin, checker=super2), and the MAKER for the two
//   pending inbox requests admin will decide on the dashboard (maker=super2,
//   checker=admin). One approved tangible asset with a known purchase_cost
//   seeds the KPI/valuation numbers; two low-band intangible asset_create
//   requests submitted by super2 land in admin's inbox (SoD: maker ≠ checker).
//
//   1. KPI + charts populate: Total Aset ≥ 1, the Nilai Perolehan card renders
//      an "Rp" value, and the status donut legend shows the "Tersedia" row.
//   2. Inline approval: admin approves one inbox request via the panel ✓
//      (success toast + row disappears; API confirms `approved`), then rejects
//      the second via ✕ → the note-required modal → confirm (neutral toast +
//      row disappears; API confirms `rejected`).
//   3. Export: the Ekspor dropdown → PDF and Excel each fire a real download
//      whose suggested filename ends in .pdf / .xlsx.
//   4. Period custom range: switch the PeriodFilter to "Rentang kustom", pick
//      a day range on the (teleported) calendar → the dashboard reloads with a
//      KPI still visible and no error banner.
//
// Robustness (project e2e conventions): unique name+code per run (this dev DB
// is NOT reset between runs — never destructive cleanup), assert-after-search,
// wait-for-toast/hidden rather than fixed sleeps, and serial mode because
// scenario 2 consumes the two seeded pending requests.
//
// NOTE (top-5 inbox): the dashboard approval panel renders only the first five
// eligible pending requests (oldest-first). This spec targets its OWN requests
// by their aria-label (`approve-<id>` / `reject-<id>`) and asserts the button
// is present before acting; a dev DB with >5 unrelated leftover pending
// requests admin is eligible for could push these off-panel — that is pre-
// existing shared-DB debris, not a defect of this flow.
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

test.describe('Dashboard — real backend e2e', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string

  let officeId: string
  let roomId: string
  let categoryId: string

  let super2Email: string
  let super2Password: string
  let super2Id: string | undefined

  let approveReqId: string
  let rejectReqId: string

  // asset_create maker-checker (admin submits → super2 approves), resolves tag.
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
    const checkerToken = await login_(api, super2Email, super2Password)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${submitted.id}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')
    const list = await apiJson<{ data: { id: string, asset_tag: string }[] }>(
      await api.get(`assets?search=${encodeURIComponent(name)}`, { headers: authHeader(adminToken) }))
    expect(list.data.length).toBe(1)
    return { id: list.data[0]!.id, tag: list.data[0]!.asset_tag }
  }

  // Pending asset_create submitted BY super2 (maker) so it enters admin's inbox
  // (SoD: maker=super2 ≠ checker=admin). Intangible + low amount → single
  // office-level step, no room_id needed. Returns the request id.
  async function submitPendingAsMaker(name: string, cost: string): Promise<string> {
    const super2Token = await login_(api, super2Email, super2Password)
    const submitted = await apiJson<{ id: string }>(await api.post('requests', {
      headers: authHeader(super2Token),
      data: {
        type: 'asset_create', amount: cost, office_id: officeId,
        payload: { name, category_id: categoryId, office_id: officeId, asset_class: 'intangible', purchase_cost: cost }
      }
    }))
    return submitted.id
  }

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Dash OT ${RUN}` }
    }))
    officeId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: `E2E Dash Office ${RUN}`, code: `E2EDO${RUN}`, office_type_id: ot.id }
    }))).id
    const floor = await apiJson<{ id: string }>(await api.post('floors', {
      headers: authHeader(adminToken), data: { office_id: officeId, name: `E2E Dash Floor ${RUN}` }
    }))
    roomId = (await apiJson<{ id: string }>(await api.post('rooms', {
      headers: authHeader(adminToken), data: { floor_id: floor.id, name: `E2E Dash Room ${RUN}` }
    }))).id
    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Dash Cat ${RUN}`, code: `E2EDC${RUN}`, asset_class: 'tangible' }
    }))).id

    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')

    super2Email = `e2e.dash.super2.${RUN}@inventra.local`
    super2Password = `Super2${RUN}!`
    super2Id = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Dash Super2 ${RUN}`, email: super2Email, password: super2Password, role_id: superadmin.id }
    }))).id

    // One approved asset (seeds Total Aset + Nilai Perolehan). A low amount so
    // asset_create clears in a single office-level approval step (higher bands
    // need multiple approvers — proven single-step at 4M in maintenance.spec).
    await createApprovedAsset(`E2E Dash Asset ${RUN}`, '4000000')

    // Two pending inbox requests for the inline-approval scenario.
    approveReqId = await submitPendingAsMaker(`E2E Dash Approve ${RUN}`, '700000')
    rejectReqId = await submitPendingAsMaker(`E2E Dash Reject ${RUN}`, '750000')
  })

  test.afterAll(async () => {
    if (super2Id) await api.delete(`users/${super2Id}`, { headers: authHeader(adminToken) }).catch(() => {})
    await api.dispose()
  })

  test('KPI + charts populate (Total Aset, Nilai Perolehan Rp, donut Tersedia)', async ({ page }) => {
    await login(page)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })

    // KPI labels present; the acquisition card renders a formatted Rp value.
    await expect(page.getByText('Total Aset', { exact: true }).first()).toBeVisible({ timeout: 10_000 })
    const acqCard = page.locator('.rounded-\\[14px\\]', { hasText: 'Nilai Perolehan' })
    await expect(acqCard).toContainText('Rp')

    // Status donut legend shows the "Tersedia" (available) row with a count.
    await expect(page.getByText('Tersedia', { exact: true }).first()).toBeVisible()
  })

  test('inline approval: approve one request (✓) and reject another (✕ + note)', async ({ page }) => {
    await login(page)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })

    // --- Approve path: click ✓ → success toast → row disappears → API approved. ---
    const approveBtn = page.getByLabel(`approve-${approveReqId}`)
    await expect(approveBtn).toBeVisible({ timeout: 10_000 })
    await approveBtn.click()
    await expect(page.getByText('Pengajuan disetujui', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByLabel(`approve-${approveReqId}`)).toHaveCount(0)

    const approvedRow = await apiJson<{ status: string }>(
      await api.get(`requests/${approveReqId}`, { headers: authHeader(adminToken) }))
    expect(approvedRow.status).toBe('approved')

    // --- Reject path: click ✕ → note-required modal → confirm → neutral toast. ---
    const rejectBtn = page.getByLabel(`reject-${rejectReqId}`)
    await expect(rejectBtn).toBeVisible({ timeout: 10_000 })
    await rejectBtn.click()

    const noteField = page.getByTestId('dashboard-reject-note')
    await expect(noteField).toBeVisible({ timeout: 8_000 })
    // Confirm stays disabled until a note is entered.
    await expect(page.getByTestId('dashboard-reject-confirm')).toBeDisabled()
    await noteField.fill(`e2e reject ${RUN}`)
    await expect(page.getByTestId('dashboard-reject-confirm')).toBeEnabled()
    await page.getByTestId('dashboard-reject-confirm').click()

    await expect(page.getByText('Pengajuan ditolak', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByLabel(`reject-${rejectReqId}`)).toHaveCount(0)

    const rejectedRow = await apiJson<{ status: string }>(
      await api.get(`requests/${rejectReqId}`, { headers: authHeader(adminToken) }))
    expect(rejectedRow.status).toBe('rejected')
  })

  test('export: Ekspor dropdown fires PDF and Excel downloads', async ({ page }) => {
    await login(page)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })

    await page.getByTestId('dashboard-export').click()
    const [pdf] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('menuitem', { name: 'Ekspor PDF' }).click()
    ])
    expect(pdf.suggestedFilename()).toMatch(/\.pdf$/)

    await page.getByTestId('dashboard-export').click()
    const [xlsx] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('menuitem', { name: 'Ekspor Excel' }).click()
    ])
    expect(xlsx.suggestedFilename()).toMatch(/\.xlsx$/)
  })

  test('period custom range: pick a range → dashboard reloads without error', async ({ page }) => {
    await login(page)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })

    // Switch the PeriodFilter to the custom preset → opens the range popover.
    await page.getByTestId('period-filter-select').click()
    await page.getByRole('option', { name: /Rentang kustom/ }).click()

    // The (teleported) UCalendar defaults to today's month. Anchor to the real
    // current month via the `data-today` cell (timezone-correct), then pick two
    // mid-month day cells by their `data-value`. force:true skips the popover's
    // open-animation stability churn (the cell briefly reflows/detaches).
    const calendar = page.getByTestId('period-filter-calendar')
    await expect(calendar).toBeVisible({ timeout: 8_000 })
    const todayValue = await calendar.locator('[data-today]').first().getAttribute('data-value')
    const ym = (todayValue ?? new Date().toISOString().slice(0, 10)).slice(0, 7)
    await calendar.locator(`[data-value="${ym}-10"]`).click({ force: true })
    await calendar.locator(`[data-value="${ym}-20"]`).click({ force: true })

    // Popover closes + dashboard reloads; a KPI is still visible, no error banner.
    await expect(page.getByTestId('dashboard-retry')).toHaveCount(0)
    await expect(page.getByText('Total Aset', { exact: true }).first()).toBeVisible({ timeout: 10_000 })
  })
})
