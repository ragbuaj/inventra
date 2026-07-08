import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Assignment (Penugasan/Peminjaman) — real backend (`/assignment` Manager
// screen + `/peminjaman` Staf page, wired to /api/v1/assignments +
// /api/v1/requests?type=assignment). Covers both submission paths:
//
//   beforeAll (API): office-type → office → floor → room → category
//   (asset_class 'tangible', so the checkout data-scope path matches the
//   real Manager module=assignments scope), two assets at that office each
//   created via the asset_create maker-checker flow (submit as admin,
//   approve as a Superadmin checker) so they exist server-side as
//   status=available before the UI tests run. Then a Staf user (linked to a
//   fresh employee + this office, for the borrow flow) and a second Manager
//   user at the same office (the SoD approver for the office-level
//   'assignment' approval band — migration 000026) plus the Manager
//   role id for direct check-out/check-in (needs assignment.manage, which
//   only Superadmin/Manager/Kepala Kanwil/Kepala Unit hold — the seeded
//   admin is Superadmin, so the Manager screen flow below runs as the admin).
//
//   1. Direct (Manager/Superadmin): on /assignment, check out asset 1 to the
//      Staf's linked employee → Riwayat shows Aktif and /assets/:tag shows
//      "Digunakan" (assets.status.assigned) with the borrow button disabled →
//      check it back in → Riwayat shows Dikembalikan and the asset is
//      available again (borrow button re-enabled).
//   2. Peminjaman (Staf→approve): log in as the Staf user, submit a borrow for
//      asset 2 through the /peminjaman UI → appears in "Pengajuan Saya" as
//      Menunggu → approve via API as the second Manager (maker != checker,
//      office-level SoD) → re-open "Pengajuan Saya" → the row shows Disetujui
//      and the asset now carries an active assignment (API-verified).
//   3. Negative: submitting the Peminjaman form with an empty Alasan is
//      blocked client-side (no request created) — verified via API list.
//
// Robustness rules (per project e2e conventions): unique name+code per run
// (this dev DB is NOT reset between runs), assert-after-search, wait for
// toasts/redirects/modal-closed rather than fixed sleeps, and API-driven
// setup for offices/assets (the dev-DB office-picker `limit:100` debris means
// a freshly-created office cannot reliably be selected through a UI dropdown).
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + seeded admin (see
// CLAUDE.md), with RATELIMIT_ENABLED=false. This spec compiles + lints here;
// CI runs it in the e2e job.
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
// email/password params) — the Peminjaman flow needs to sign in as the Staf
// user created in beforeAll, so it uses its own parametrized UI-login helper
// (same steps as helpers.ts, generalized over credentials — mirrors approval.spec.ts).
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

test.describe('Assignment (Penugasan/Peminjaman) — real backend e2e', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string

  let officeId: string
  let officeName: string
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

  let asset1Name: string
  let asset1Tag: string
  let asset2Id: string
  let asset2Name: string

  // Creates an asset via the asset_create maker-checker flow (submit as
  // admin, approve as a Superadmin checker) and resolves its id + tag.
  // Mirrors assets.spec.ts / transfers.spec.ts's beforeAll pattern.
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
      headers: authHeader(adminToken), data: { name: `E2E Assignment OT ${RUN}` }
    }))
    officeName = `E2E Assignment Office ${RUN}`
    officeId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeName, code: `E2EGN${RUN}`, office_type_id: ot.id }
    }))).id
    const floor = await apiJson<{ id: string }>(await api.post('floors', {
      headers: authHeader(adminToken), data: { office_id: officeId, name: `E2E Assignment Floor ${RUN}` }
    }))
    roomId = (await apiJson<{ id: string }>(await api.post('rooms', {
      headers: authHeader(adminToken), data: { floor_id: floor.id, name: `E2E Assignment Room ${RUN}` }
    }))).id
    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Assignment Cat ${RUN}`, code: `E2EGC${RUN}`, asset_class: 'tangible' }
    }))).id

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

    checkerEmail = `e2e.assignment.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Assignment Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    // --- Two available assets at officeId (tangible → needs room_id at approval). ---
    asset1Name = `E2E Assignment Asset1 ${RUN}`
    asset2Name = `E2E Assignment Asset2 ${RUN}`
    const a1 = await createApprovedAsset(asset1Name, '3000000')
    asset1Tag = a1.tag
    const a2 = await createApprovedAsset(asset2Name, '3200000')
    asset2Id = a2.id

    // --- Staf user linked to a fresh employee at officeId (assignment.borrow
    // requires the caller to have BOTH office_id and employee_id set — see
    // internal/assignment/handler.go `available`/`borrow`). ---
    const empRes = await api.post('employees', {
      headers: authHeader(adminToken),
      data: { code: `E2EEMP${RUN}`, name: `E2E Assignment Employee ${RUN}`, office_id: officeId }
    })
    const employee = await apiJson<{ id: string }>(empRes)
    stafEmployeeId = employee.id

    stafEmail = `e2e.assignment.staf.${RUN}@inventra.local`
    stafPassword = `Staf${RUN}!`
    stafId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: {
        name: `E2E Assignment Staf ${RUN}`, email: stafEmail, password: stafPassword,
        role_id: staf.id, office_id: officeId, employee_id: stafEmployeeId
      }
    }))).id

    // --- Second Manager at the same office: the SoD approver for the
    // office-level 'assignment' approval band (migration 000026), distinct
    // from the Staf maker. ---
    managerApproverEmail = `e2e.assignment.approver.${RUN}@inventra.local`
    managerApproverPassword = `Approver${RUN}!`
    managerApproverId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: {
        name: `E2E Assignment Approver ${RUN}`, email: managerApproverEmail, password: managerApproverPassword,
        role_id: manager.id, office_id: officeId
      }
    }))).id
  })

  test.afterAll(async () => {
    for (const id of [checkerId, stafId, managerApproverId]) {
      if (id) await api.delete(`users/${id}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('Direct (Manager): check out asset 1 → Riwayat Aktif + Detail shows Digunakan; check in → Dikembalikan + available', async ({ page }) => {
    // The seeded admin is Superadmin, which holds assignment.manage —
    // exercises the Manager-screen direct check-out/check-in path.
    //
    // NOTE: every picker on this screen is a Nuxt UI USelect/USelectMenu (a
    // custom popover, NOT a native <select> — see categories/employees specs),
    // so it is driven by clicking its trigger (by its placeholder text) then a
    // role="option" in the open listbox. `selectOption` would NOT work here.
    const employeeLabel = `E2E Assignment Employee ${RUN}`
    await login(page)
    await page.goto('/assignment')
    await expect(page.getByRole('heading', { name: 'Penugasan Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Check-out tab (default) ---
    // Asset USelectMenu (searchable): open by its placeholder trigger text,
    // then pick the option (label = "<name> · <tag>", matched by unique name).
    await page.getByText('Cari nama / kode aset…', { exact: true }).first().click()
    await page.getByRole('option', { name: new RegExp(asset1Name) }).click()

    // Recipient USelect: trigger shows the placeholder until a value is picked.
    await page.getByText('Pilih pegawai…', { exact: true }).click()
    await page.getByRole('option', { name: employeeLabel, exact: true }).click()

    await page.locator('input[type="date"]').first().fill(todayISO())

    // Submit button (its accessible name "Check-out" collides with the tab of the
    // same name, so target the submit by testid).
    await page.getByTestId('assignment-checkout-submit').click()

    // checkout.ok = `Aset "{name}" berhasil di-check-out ke {holder}.` where
    // {name} is the picker label "<name> · <tag>" and {holder} the employee name.
    await expect(page.getByText(`Aset "${asset1Name} · ${asset1Tag}" berhasil di-check-out ke ${employeeLabel}.`, { exact: true }))
      .toBeVisible({ timeout: 10_000 })

    // --- Riwayat tab: Aktif ---
    await page.getByRole('button', { name: 'Riwayat', exact: true }).click()
    await page.getByPlaceholder('Cari aset / pemegang…', { exact: true }).fill(asset1Name)
    const historyRow = page.locator('tr', { hasText: asset1Name })
    await expect(historyRow).toBeVisible({ timeout: 10_000 })
    await expect(historyRow.getByText('Aktif', { exact: true })).toBeVisible()

    // --- Detail-Aset: status Digunakan (assets.status.assigned), borrow disabled ---
    await page.goto(`/assets/${asset1Tag}`)
    await expect(page.getByRole('heading', { name: asset1Name, exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Digunakan', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Ajukan Peminjaman', exact: true })).toBeDisabled()

    // --- Check back in via the Manager screen ---
    await page.goto('/assignment')
    await page.getByRole('button', { name: 'Check-in', exact: true }).click()

    // Active-assignment USelect: option label = "<tag> · <name> — <holder>"; the
    // unique tag scopes it to this run's assignment. Open by placeholder trigger.
    await page.getByText('Pilih penugasan…', { exact: true }).click()
    await page.getByRole('option', { name: new RegExp(asset1Tag) }).click()

    await page.locator('input[type="date"]').first().fill(todayISO())
    // Submit button (name collides with the "Check-in" tab — target by testid).
    await page.getByTestId('assignment-checkin-submit').click()

    await expect(page.getByText(`Aset "${asset1Name}" berhasil dikembalikan.`, { exact: true })).toBeVisible({ timeout: 10_000 })

    await page.getByRole('button', { name: 'Riwayat', exact: true }).click()
    await page.getByPlaceholder('Cari aset / pemegang…', { exact: true }).fill(asset1Name)
    await expect(page.locator('tr', { hasText: asset1Name }).getByText('Dikembalikan', { exact: true })).toBeVisible({ timeout: 10_000 })

    // --- Detail-Aset: back to Tersedia, borrow re-enabled ---
    await page.goto(`/assets/${asset1Tag}`)
    await expect(page.getByText('Tersedia', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('button', { name: 'Ajukan Peminjaman', exact: true })).toBeEnabled()
  })

  test('Peminjaman (Staf → approve): submit via UI → Menunggu → approve via API (maker != checker) → Disetujui + assignment exists', async ({ page }) => {
    await loginAs(page, stafEmail, stafPassword)
    await page.goto('/peminjaman')
    await expect(page.getByRole('heading', { name: 'Peminjaman Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    // Fill Alasan BEFORE opening the asset picker. The USelectMenu popover's focus
    // management swallows keystrokes typed into the textarea immediately after the
    // popover closes (the value never lands — confirmed via CI trace), so type the
    // reason first, while no popover has been involved. Nothing clears `notes` on
    // asset selection, so this ordering is safe.
    const reason = `perlu untuk presentasi ${RUN}`
    const alasanField = page.getByRole('textbox', { name: /Alasan/ })
    await alasanField.fill(reason)
    await expect(alasanField).toHaveValue(reason)

    const picker = page.getByTestId('peminjaman-asset-picker')
    await picker.click()
    await page.getByRole('option', { name: new RegExp(asset2Name) }).click()

    await page.getByTestId('peminjaman-submit').click()

    await expect(page.getByText('Pengajuan peminjaman terkirim', { exact: true })).toBeVisible({ timeout: 10_000 })

    // "Pengajuan Saya" defaults to filter=all; find the just-submitted row by
    // its catatan-free Menunggu status (assert-after-search via the filter tab).
    await page.getByTestId('peminjaman-filter-pending').click()

    const reqRes = await api.get('requests?type=assignment&status=pending&mine=true&limit=50', {
      headers: authHeader(await login_(api, stafEmail, stafPassword))
    })
    const reqs = await apiJson<{ data: { id: string, target_id: string | null }[] }>(reqRes)
    const match = reqs.data.find(r => r.target_id === asset2Id)
    expect(match).toBeTruthy()
    const requestId = match!.id

    const pendingRow = page.getByTestId(`peminjaman-row-${requestId}`)
    await expect(pendingRow).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId(`peminjaman-status-${requestId}`)).toHaveText('Menunggu')

    // --- Approve via API as the second Manager (SoD: maker=Staf, checker=Manager). ---
    const approverToken = await login_(api, managerApproverEmail, managerApproverPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${requestId}/approve`, {
      headers: authHeader(approverToken), data: { decision: 'approve', note: 'e2e approve peminjaman' }
    }))
    expect(approved.status).toBe('approved')

    // --- Re-open "Pengajuan Saya" (Disetujui filter) and assert the status flip. ---
    await page.getByTestId('peminjaman-filter-approved').click()
    await expect(page.getByTestId(`peminjaman-status-${requestId}`)).toHaveText('Disetujui', { timeout: 10_000 })

    // --- API-verify an active assignment now exists for asset 2 and it is assigned. ---
    const assignmentsRes = await api.get(`assets/${asset2Id}/assignments`, { headers: authHeader(adminToken) })
    const assignments = await apiJson<{ data: { status: string, employee_id: string }[] }>(assignmentsRes)
    const active = assignments.data.find(a => a.status === 'active')
    expect(active).toBeTruthy()
    expect(active?.employee_id).toBe(stafEmployeeId)

    const asset = await apiJson<{ status: string }>(await api.get(`assets/${asset2Id}`, { headers: authHeader(adminToken) }))
    expect(asset.status).toBe('assigned')
  })

  test('Peminjaman: submitting with empty Alasan is blocked client-side (no request created)', async ({ page }) => {
    await loginAs(page, stafEmail, stafPassword)
    await page.goto('/peminjaman')
    await expect(page.getByRole('heading', { name: 'Peminjaman Aset', exact: true })).toBeVisible({ timeout: 10_000 })

    const stafToken = await login_(api, stafEmail, stafPassword)
    const before = await apiJson<{ total: number }>(
      await api.get('requests?type=assignment&mine=true&limit=1', { headers: authHeader(stafToken) }))

    // Submit with the Alasan (reason) field left empty — the client-side guard
    // (`notesError = !reason`) must block the submit before any request is built,
    // so no asset needs to be picked. Asserting the inline field error proves the
    // guard fired; the request-count check proves nothing was sent server-side.
    await page.getByTestId('peminjaman-submit').click()

    await expect(page.getByText('Alasan wajib diisi.', { exact: true })).toBeVisible({ timeout: 5_000 })

    const after = await apiJson<{ total: number }>(
      await api.get('requests?type=assignment&mine=true&limit=1', { headers: authHeader(stafToken) }))
    expect(after.total).toBe(before.total)
  })
})
