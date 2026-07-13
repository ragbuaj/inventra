import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'
import { login, pickAsync, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Maintenance (Jadwal/Catatan/Laporan Kerusakan) — real backend
// (`/maintenance` wired to /api/v1/maintenance/* + /api/v1/requests?type=
// maintenance). Covers both the Manager-facing schedule/record lifecycle and
// the Staf damage-report → approve → corrective-record path:
//
//   beforeAll (API): office-type → office → floor → room → category
//   (asset_class 'tangible', so a later assignment check-out is valid) → a
//   second Superadmin "checker" (SoD, for the asset_create maker-checker
//   setup) → two approved assets at that office (asset1 for the schedule/
//   record flow, asset2 for the Staf assignment/report flow) → a unique
//   maintenance-category + problem-category (masterdata reference engine) →
//   a Staf user linked to a fresh employee at the office → a second Manager
//   at the same office (the SoD approver for the office-level 'maintenance'
//   approval band — migration 000027) → asset2 borrowed by the Staf via the
//   assignment module's own borrow → approve maker-checker path (so the
//   Laporan tab's "Aset yang Anda pegang" picker has an option — see the
//   beforeAll comment for why this path, not a direct check-out).
//
//   1. Manager (Superadmin, holds maintenance.manage): create a schedule for
//      asset1 via the Jadwal tab (start date = today → immediate "due today"
//      badge + shows in the top due banner) → "Buat Catatan" from the
//      schedule card opens the record slideover prefilled (locked asset,
//      schedule linked) → save as in_progress → Detail-Aset shows the
//      "Maintenance" status badge → edit the record → completed with a cost
//      → Catatan row shows Selesai + the formatted cost; the schedule's
//      next-due shifts by its interval (no longer in the due banner); the
//      asset is back to "Tersedia" on Detail-Aset.
//   2. Staf → approve → record: Staf logs in, opens Laporan Kerusakan, picks
//      the assigned asset2 + a problem category, submits → success banner +
//      "Riwayat Laporan Saya" shows "Menunggu Review" → approve via API as
//      the second office-level Manager (maker ≠ checker) → the report flips
//      to "Disetujui" → as Manager, the Catatan tab now shows a corrective
//      'scheduled' record for asset2.
//   3. Negative: (a) the Laporan submit button stays disabled with an asset
//      picked but no kategori; (b) reopening the now-completed record from
//      scenario 1 shows the read-only hint and no save button.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (this dev DB is NOT reset between runs), assert-after-search, wait-for-
// dialog-hidden rather than fixed sleeps (USlideover renders role="dialog"),
// fill text fields before opening any USelectMenu popover, row-scoped
// selectors over ambiguous getByText, and API-driven setup mirroring
// assignment.spec.ts/disposals.spec.ts.
//
// IMPORTANT — bug found while building this spec, then fixed with a different
// design after review: the Laporan tab's "Aset yang Anda pegang" picker
// originally called `GET /assignments?status=active&employee_id=...` with a
// *client-supplied* employee_id. A first fix attempt (migration 000028)
// granted Staf `assignment.view` so that call would stop 403ing — but review
// caught that this reopened the door wider than intended: with
// `assignment.view` + the office-level data scope, any Staf could simply omit
// `employee_id` and read every coworker's assignments in the office. The
// final design (kept here, migration 000028 was deleted): a dedicated
// `GET /assignments/mine`, gated by `request.create` (already seeded for Staf
// in `000005` — no new grant needed), which resolves the caller's employee id
// **server-side** from the JWT, so the response can only ever contain the
// caller's own rows. See docs/PROGRESS.md item 38 for the full note.
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

test.describe('Maintenance (Jadwal/Catatan/Laporan Kerusakan) — real backend e2e', () => {
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
  let stafEmployeeId: string

  let managerApproverEmail: string
  let managerApproverPassword: string
  let managerApproverId: string | undefined

  let asset1Id: string
  let asset1Name: string
  let asset1Tag: string
  let asset2Id: string
  let asset2Name: string

  let maintCatName: string
  let problemCatName: string

  let scheduleId: string
  let record1Id: string
  const uniqueDesc = `E2E maintenance note ${RUN}`

  // Creates an asset via the asset_create maker-checker flow (submit as
  // admin, approve as a Superadmin checker) and resolves its id + tag.
  // Mirrors assignment.spec.ts/transfers.spec.ts's beforeAll pattern.
  async function createApprovedAsset(name: string, cost: string): Promise<{ id: string, tag: string }> {
    const submitRes = await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create',
        amount: cost,
        office_id: officeId,
        payload: {
          name, category_id: categoryId, office_id: officeId, room_id: roomId,
          asset_class: 'tangible', purchase_cost: cost, purchase_date: '2026-07-01'
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
    const list = await apiJson<{ data: { id: string, asset_tag: string }[] }>(listRes)
    expect(list.data.length).toBe(1)
    return { id: list.data[0]!.id, tag: list.data[0]!.asset_tag }
  }

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    // --- FK prerequisites: office-type → office → floor → room, category (tangible). ---
    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Maintenance OT ${RUN}` }
    }))
    officeId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: `E2E Maintenance Office ${RUN}`, code: `E2EMO${RUN}`, office_type_id: ot.id }
    }))).id
    const floor = await apiJson<{ id: string }>(await api.post('floors', {
      headers: authHeader(adminToken), data: { office_id: officeId, name: `E2E Maintenance Floor ${RUN}` }
    }))
    roomId = (await apiJson<{ id: string }>(await api.post('rooms', {
      headers: authHeader(adminToken), data: { floor_id: floor.id, name: `E2E Maintenance Room ${RUN}` }
    }))).id
    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Maintenance Cat ${RUN}`, code: `E2EMC${RUN}`, asset_class: 'tangible' }
    }))).id

    // --- Reference resources: a unique maintenance-category + problem-category. ---
    maintCatName = `E2E Maint Kategori ${RUN}`
    await api.post('maintenance-categories', { headers: authHeader(adminToken), data: { name: maintCatName } })
    problemCatName = `E2E Problem Kategori ${RUN}`
    await api.post('problem-categories', { headers: authHeader(adminToken), data: { name: problemCatName } })

    // --- Checker user (SoD, for the asset_create setup only): a second
    // Superadmin-scoped user, distinct from the maker (admin). ---
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    const manager = roles.data.find(r => r.name === 'Manager')
    if (!manager) throw new Error('Manager role not found in GET /authz/roles')
    const staf = roles.data.find(r => r.name === 'Staf')
    if (!staf) throw new Error('Staf role not found in GET /authz/roles')

    checkerEmail = `e2e.maint.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Maintenance Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    // --- Two approved assets at officeId (tangible → needs room_id at approval). ---
    asset1Name = `E2E Maintenance Asset1 ${RUN}`
    asset2Name = `E2E Maintenance Asset2 ${RUN}`
    const a1 = await createApprovedAsset(asset1Name, '4000000')
    asset1Id = a1.id
    asset1Tag = a1.tag
    const a2 = await createApprovedAsset(asset2Name, '4200000')
    asset2Id = a2.id

    // --- Staf user linked to a fresh employee at officeId. ---
    const empRes = await api.post('employees', {
      headers: authHeader(adminToken),
      data: { code: `E2EMEMP${RUN}`, name: `E2E Maintenance Employee ${RUN}`, office_id: officeId }
    })
    const employee = await apiJson<{ id: string }>(empRes)
    stafEmployeeId = employee.id

    stafEmail = `e2e.maint.staf.${RUN}@inventra.local`
    stafPassword = `Staf${RUN}!`
    stafId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: {
        name: `E2E Maintenance Staf ${RUN}`, email: stafEmail, password: stafPassword,
        role_id: staf.id, office_id: officeId, employee_id: stafEmployeeId
      }
    }))).id

    // --- Second Manager at the same office: the SoD approver for the
    // office-level 'maintenance' approval band (migration 000027), distinct
    // from the Staf maker. ---
    managerApproverEmail = `e2e.maint.approver.${RUN}@inventra.local`
    managerApproverPassword = `Approver${RUN}!`
    managerApproverId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: {
        name: `E2E Maintenance Approver ${RUN}`, email: managerApproverEmail, password: managerApproverPassword,
        role_id: manager.id, office_id: officeId
      }
    }))).id

    // --- Seed asset2's active assignment via the borrow → approve maker-checker
    // path (POST /assignments/borrow + approve), NOT a direct check-out. A
    // direct check-out needs `assignment.manage`, which this shared dev DB's
    // seeded Superadmin may lack (migration 000005 was amended to add it after
    // this DB had already applied it — see docs/PROGRESS.md item 36 /
    // assignment.spec.ts); borrow+approve only needs `request.create` (Staf
    // already has it, since 000005) + `request.decide` on the approver, so it
    // is immune to that drift and mirrors assignment.spec's own Peminjaman
    // flow. Deliberately NOT wrapped in a try/catch: the Laporan picker now
    // reads this back via `GET /assignments/mine`, which is also gated by
    // `request.create` only (no `assignment.view`, no migration-timing gap —
    // see the file-header comment), so there is no known permission drift left
    // to self-skip around here. A failure below is a real regression and
    // should fail the suite loudly, not be swallowed.
    const stafToken0 = await login_(api, stafEmail, stafPassword)
    const borrowed = await apiJson<{ request_id: string }>(await api.post('assignments/borrow', {
      headers: authHeader(stafToken0), data: { asset_id: asset2Id, notes: `e2e seed ${RUN}` }
    }))
    const managerApproverToken0 = await login_(api, managerApproverEmail, managerApproverPassword)
    const approvedSeed = await apiJson<{ status: string }>(await api.post(`requests/${borrowed.request_id}/approve`, {
      headers: authHeader(managerApproverToken0), data: { decision: 'approve', note: 'e2e seed borrow' }
    }))
    expect(approvedSeed.status).toBe('approved')
  })

  test.afterAll(async () => {
    for (const id of [checkerId, stafId, managerApproverId]) {
      if (id) await api.delete(`users/${id}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('Manager: create schedule → Buat Catatan → in_progress → completed', async ({ page }) => {
    await login(page)
    await page.goto('/maintenance')
    await expect(page.getByRole('heading', { name: 'Maintenance', exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Jadwal tab (default): create a schedule for asset1, start date = today. ---
    await page.getByTestId('jadwal-add-button').click()
    const scheduleDialog = page.getByRole('dialog')
    await expect(scheduleDialog).toBeVisible({ timeout: 8_000 })

    const assetPicker = scheduleDialog.getByTestId('schedule-slideover-asset-picker')
    await assetPicker.getByTestId('asset-picker-input').fill(asset1Name)
    await assetPicker.getByTestId('asset-picker-item').first().click()

    await pickAsync(page, 'schedule-slideover-category', maintCatName, maintCatName)

    // The category AsyncSearchPicker's result list stays mounted (off-screen)
    // after selection rather than being removed — a bare .fill() on the next
    // field occasionally lands before Vue settles the DOM, leaving it empty.
    // An explicit .click() first (focusing the real input) makes it reliable.
    const intervalField = scheduleDialog.getByTestId('schedule-slideover-interval')
    await intervalField.click()
    await intervalField.fill('2')
    await scheduleDialog.getByTestId('schedule-slideover-date').fill(todayISO())

    await scheduleDialog.getByRole('button', { name: 'Simpan', exact: true }).click()
    await expect(scheduleDialog).toBeHidden({ timeout: 10_000 })

    // --- Resolve the created schedule id (API) + assert it renders with a
    // "due today" badge in both the schedule card and the top due banner. ---
    const schedulesRes = await apiJson<{ data: { id: string, asset_id: string }[] }>(
      await api.get('maintenance/schedules?limit=100', { headers: authHeader(adminToken) }))
    const sched = schedulesRes.data.find(s => s.asset_id === asset1Id)
    expect(sched).toBeTruthy()
    scheduleId = sched!.id

    const scheduleCard = page.getByTestId(`schedule-card-${scheduleId}`)
    await expect(scheduleCard).toBeVisible({ timeout: 10_000 })
    await expect(scheduleCard).toContainText('Jatuh tempo hari ini')

    const dueBanner = page.getByTestId('due-banner')
    await expect(dueBanner).toBeVisible({ timeout: 10_000 })
    await expect(dueBanner).toContainText(asset1Name)

    // --- "Buat Catatan" from the schedule card → prefilled record slideover. ---
    await page.getByTestId(`schedule-make-note-${scheduleId}`).click()
    const recordDialog = page.getByRole('dialog')
    await expect(recordDialog).toBeVisible({ timeout: 8_000 })
    await expect(recordDialog.getByTestId('record-slideover-locked-asset')).toContainText(asset1Name)
    await expect(recordDialog.getByTestId('record-slideover-locked-asset')).toContainText(asset1Tag)

    await recordDialog.getByTestId('record-slideover-date').fill(todayISO())
    await recordDialog.getByTestId('record-slideover-description').fill(uniqueDesc)
    await recordDialog.getByTestId('record-slideover-status').click()
    await page.getByRole('option', { name: 'Berlangsung', exact: true }).click()

    await recordDialog.getByRole('button', { name: 'Simpan Catatan', exact: true }).click()
    await expect(recordDialog).toBeHidden({ timeout: 10_000 })

    // --- Resolve the created record id (API). ---
    const recordsRes = await apiJson<{ data: { id: string, description: string }[] }>(
      await api.get(`maintenance/records?q=${encodeURIComponent(asset1Name)}&limit=10`, { headers: authHeader(adminToken) }))
    const rec = recordsRes.data.find(r => r.description === uniqueDesc)
    expect(rec).toBeTruthy()
    record1Id = rec!.id

    // --- Detail-Aset: asset1 now shows the "Maintenance" status badge. ---
    await page.goto(`/assets/${asset1Tag}`)
    await expect(page.getByRole('heading', { name: asset1Name, exact: true })).toBeVisible({ timeout: 10_000 })
    // Scoped to <main> — the sidebar nav item is also literally "Maintenance".
    await expect(page.getByRole('main').getByText('Maintenance', { exact: true })).toBeVisible()

    // --- Edit the record: complete it with a cost. ---
    await page.goto('/maintenance')
    await page.getByRole('button', { name: 'Catatan', exact: true }).click()
    const recordRow = page.getByTestId(`record-row-${record1Id}`)
    await expect(recordRow).toBeVisible({ timeout: 10_000 })
    await recordRow.click()

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toBeVisible({ timeout: 8_000 })
    await expect(editDialog.getByTestId('record-slideover-readonly-hint')).toBeHidden()

    await editDialog.getByTestId('record-slideover-status').click()
    await page.getByRole('option', { name: 'Selesai', exact: true }).click()
    const costField = editDialog.getByTestId('record-slideover-cost')
    await costField.click()
    await costField.fill('150000')

    await editDialog.getByRole('button', { name: 'Simpan Catatan', exact: true }).click()
    await expect(editDialog).toBeHidden({ timeout: 10_000 })

    // --- Catatan row: Selesai + formatted cost. ---
    await expect(recordRow).toContainText('Selesai')
    await expect(recordRow).toContainText('Rp 150.000')

    // --- Jadwal: next-due shifted away from "today" — no longer in the due banner. ---
    await page.getByRole('button', { name: 'Jadwal', exact: true }).click()
    await expect(scheduleCard).not.toContainText('Jatuh tempo hari ini')
    await expect(scheduleCard).not.toContainText('Terlambat')
    // The whole banner may have unmounted (v-if) if this was its only near-due
    // item — only assert its content when it still renders.
    const dueBannerAfter = page.getByTestId('due-banner')
    if (await dueBannerAfter.count() > 0) {
      await expect(dueBannerAfter).not.toContainText(asset1Name)
    }

    // --- Detail-Aset: asset1 back to "Tersedia". ---
    await page.goto(`/assets/${asset1Tag}`)
    await expect(page.getByText('Tersedia', { exact: true })).toBeVisible({ timeout: 10_000 })
  })

  test('Staf → approve → record: submit damage report via UI → Menunggu Review → approve via API (maker ≠ checker) → Catatan shows corrective record', async ({ page }) => {
    await loginAs(page, stafEmail, stafPassword)
    await page.goto('/maintenance')
    await expect(page.getByRole('heading', { name: 'Maintenance', exact: true })).toBeVisible({ timeout: 10_000 })

    await page.getByRole('button', { name: 'Laporan Kerusakan', exact: true }).click()

    // Fill the description BEFORE opening any USelectMenu popover (focus-trap
    // memory — see assignment.spec.ts). Nothing clears it on asset/problem
    // selection, so this ordering is safe.
    const desc = `Layar retak ${RUN}`
    await page.getByTestId('report-description').fill(desc)

    await page.getByTestId('report-asset-picker').click()
    await page.getByRole('option', { name: new RegExp(asset2Name) }).click()

    // --- Negative (part of scenario 3): submit stays disabled with kategori empty. ---
    await expect(page.getByTestId('report-submit')).toBeDisabled()

    await pickAsync(page, 'report-problem', problemCatName, problemCatName)

    await expect(page.getByTestId('report-submit')).toBeEnabled()
    await page.getByTestId('report-submit').click()

    await expect(page.getByTestId('report-success')).toBeVisible({ timeout: 10_000 })

    // --- "Riwayat Laporan Saya": Menunggu Review. ---
    const stafToken = await login_(api, stafEmail, stafPassword)
    const reqRes = await api.get('requests?type=maintenance&status=pending&mine=true&limit=50', {
      headers: authHeader(stafToken)
    })
    const reqs = await apiJson<{ data: { id: string, target_id: string | null }[] }>(reqRes)
    const match = reqs.data.find(r => r.target_id === asset2Id)
    expect(match).toBeTruthy()
    const requestId = match!.id

    const historyRow = page.getByTestId(`report-history-${requestId}`)
    await expect(historyRow).toBeVisible({ timeout: 10_000 })
    await expect(historyRow).toContainText('Menunggu Review')

    // --- Approve via API as the second Manager (SoD: maker=Staf, checker=Manager). ---
    const approverToken = await login_(api, managerApproverEmail, managerApproverPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${requestId}/approve`, {
      headers: authHeader(approverToken), data: { decision: 'approve', note: 'e2e approve maintenance report' }
    }))
    expect(approved.status).toBe('approved')

    // --- Reload to re-fetch "Riwayat Laporan Saya" (loaded once on mount) → Disetujui. ---
    await page.goto('/maintenance')
    await page.getByRole('button', { name: 'Laporan Kerusakan', exact: true }).click()
    await expect(page.getByTestId(`report-history-${requestId}`)).toContainText('Disetujui', { timeout: 10_000 })

    // --- As Manager: Catatan shows the corrective 'scheduled' record for asset2. ---
    // Drop the Staf session first: /login redirects authenticated users (the
    // httpOnly refresh cookie would silently restore the Staf session).
    await page.context().clearCookies()
    await page.evaluate(() => window.localStorage.clear())
    await login(page)
    await page.goto('/maintenance')
    await page.getByRole('button', { name: 'Catatan', exact: true }).click()
    const correctiveRow = page.locator('[data-testid^="record-row-"]').filter({ hasText: asset2Name })
    await expect(correctiveRow).toBeVisible({ timeout: 10_000 })
    await expect(correctiveRow).toContainText('Corrective')
    await expect(correctiveRow).toContainText('Dijadwalkan')
  })

  test('Negative: completed record opens read-only (no save button)', async ({ page }) => {
    await login(page)
    await page.goto('/maintenance')
    await page.getByRole('button', { name: 'Catatan', exact: true }).click()

    const recordRow = page.getByTestId(`record-row-${record1Id}`)
    await expect(recordRow).toBeVisible({ timeout: 10_000 })
    await recordRow.click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible({ timeout: 8_000 })
    await expect(dialog.getByTestId('record-slideover-readonly-hint')).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'Simpan Catatan', exact: true })).toHaveCount(0)
  })
})
