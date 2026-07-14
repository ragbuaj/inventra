import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Depreciation (Penyusutan) — real backend (`/depreciation` wired to
// /api/v1/depreciation/* + /api/v1/assets/:id/depreciation +
// /api/v1/assets/:id/impairment). Covers the full monthly lifecycle plus its
// downstream integration into disposal:
//
//   beforeAll (API): office-type → office → category carrying depreciation
//   defaults (straight_line, 48 months, 0 salvage, fiscal group kelompok_1 —
//   both bases resolve to 48-month straight-line, so commercial and fiscal
//   produce the SAME per-period amount) → a second "checker" user (SoD) →
//   one asset created via the asset_create maker-checker flow with
//   purchase_cost 4,800,000 (well inside the lowest asset_create threshold
//   band — single office-level approval step) and purchase_date 3 calendar
//   months before the run (so the very first Hitung has a real multi-month
//   backlog to walk, not a same-month zero-elapsed edge case).
//
//   1. `/depreciation`: the never-computed current period shows the reminder
//      banner and a "no entry yet" schedule row for this asset (the union query
//      lists capitalized assets even before a run) → Hitung → period flips to
//      Terhitung, KPI tiles populate, and the schedule (filtered to this run's
//      asset by name) shows exactly one row at 100.000 (4.800.000 / 48).
//   2. Toggle to the Fiskal basis: same row, same amount (kelompok_1 is also
//      48-month straight-line) but the KPI's reference chip switches from
//      PSAK 16 to PMK 72/2023.
//   3. Journal tab: a balanced ready-to-post recap, plus xlsx/pdf export as
//      real browser downloads.
//   4. Tutup Periode: badge flips to Ditutup; Hitung/Tutup controls disappear.
//   5. Impairment: write the asset down to a recoverable amount and verify the
//      loss + the schedule's impaired icon (see the IMPORTANT note below for
//      what this does and does NOT change).
//   6. Asset detail's Depreciation tab shows the full posted history for both
//      bases.
//   7. Full-circle integration: submitting a disposal for this asset produces
//      a request whose `amount`/`book_value_at_disposal` is the asset's
//      current depreciation-ledger book value — the same number the schedule
//      showed — proving depreciation and disposal share one source of truth.
//
// IMPORTANT — discovered against the real backend, not assumed:
//   - The Jalankan-Periode preview counts and the KPI tiles are FLEET-WIDE
//     aggregates (ComputePeriod runs over every capitalized asset in the DB;
//     the KPI schedule() call the page issues carries no search filter — see
//     depreciation.vue's loadKpis). This dev DB accumulates capitalized-but-
//     unparameterized assets from other specs, so these numbers are not
//     stable across runs. This spec only asserts they are non-empty/populated
//     and asserts exact amounts on the per-asset SEARCH-FILTERED schedule row,
//     which is deterministic regardless of DB debris.
//   - Impairment (POST /assets/:id/impairment) writes asset.book_value /
//     impairment_loss DIRECTLY — it does not post a new depreciation entry
//     (see depreciation.Service.RecordImpairment's doc comment). GET
//     /assets/:id/depreciation's `computed_book_value` is sourced from the
//     LAST POSTED ENTRY's closing (BookValueAsOf → LastEntryAtOrBefore), so it
//     is UNCHANGED immediately after an impairment — only the next
//     ComputePeriod's "lower of book_value vs. last closing" resumption
//     override would pick the write-down up prospectively. The write IS
//     immediately visible on the asset's own record (GET /assets/:id), so
//     this spec asserts the impairment there, not via the depreciation
//     endpoint. The SAME ledger-sourced value (not the impaired one) is what
//     flows into the disposal amount in test 7 — verified empirically (a
//     manual dry run against this stack: compute → close → impair to
//     1,000,000 → submit disposal → the request's amount was 4,400,000, the
//     unimpaired ledger closing, not 1,000,000).
//   - Depreciation periods are a GLOBAL monthly singleton (one row per
//     calendar month, not scoped to a RUN or an asset) — unlike every other
//     fixture in this suite. Once this spec computes-then-closes the current
//     month locally, re-running it again within the same calendar month
//     against this same (not-reset-between-runs) dev DB will find the period
//     already closed and the reminder/Hitung-availability assertions in test
//     1 will fail. CI is unaffected (its DB is reset per run, so the period
//     is always virgin). Locally, reset via
//     `DELETE FROM depreciation.depreciation_entries; DELETE FROM
//     depreciation.depreciation_periods;` before re-running this file (or the
//     full e2e suite) more than once within the same month.
//
// Robustness rules (per project e2e conventions): unique name+code per run,
// the schedule/journal/asset-detail assertions all filter to this run's
// unique asset name so DB debris from other runs cannot create ambiguity,
// wait for toasts/redirects/downloads rather than fixed sleeps, and serial
// mode (the tests share one period + one asset and mutate its state — compute
// → close → impair — across steps).
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

// First-of-month, `n` calendar months before today (UTC — the backend's
// "now" and month-walk are both UTC-normalized via firstOfMonth/monthsElapsed
// in depreciation/engine.go). Anchoring to the 1st avoids month-length
// rollover surprises (e.g. Mar 31 − 1 month).
function monthsAgoISO(n: number): string {
  const d = new Date()
  d.setUTCDate(1)
  d.setUTCMonth(d.getUTCMonth() - n)
  return d.toISOString().slice(0, 10)
}
function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

test.describe('Depreciation (Penyusutan) — real backend (compute/close/impair/disposal e2e)', () => {
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
  let assetTag: string
  const assetCost = '4800000' // 4,800,000 / 48 months SL = 100,000/period

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Depr OT ${RUN}` }
    }))
    officeId = (await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: `E2E Depr Office ${RUN}`, code: `E2EDP${RUN}`, office_type_id: ot.id }
    }))).id
    categoryId = (await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: {
        name: `E2E Depr Cat ${RUN}`,
        code: `E2EDPC${RUN}`,
        asset_class: 'intangible',
        default_depreciation_method: 'straight_line',
        default_useful_life_months: 48,
        default_salvage_rate: '0',
        default_fiscal_group: 'kelompok_1'
      }
    }))).id

    // Checker user (SoD): a second Superadmin-scoped user so it is eligible to
    // decide (global data scope + request.decide) but is NOT the maker.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')
    checkerEmail = `e2e.depr.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    checkerId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Depr Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))).id

    // Asset via the asset_create maker-checker flow — asset_class
    // 'intangible' avoids the tangible-only room_id CHECK constraint enforced
    // at approval time (see assets.spec.ts's gotcha). purchase_date 3 months
    // ago gives Hitung a real multi-month backlog to walk on its first run.
    assetName = `E2E Depr Asset ${RUN}`
    const submitted = await apiJson<{ id: string }>(await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create',
        amount: assetCost,
        office_id: officeId,
        payload: {
          name: assetName, category_id: categoryId, office_id: officeId,
          asset_class: 'intangible', purchase_cost: assetCost, purchase_date: monthsAgoISO(3)
        }
      }
    }))
    const checkerToken = await login_(api, checkerEmail, checkerPassword)
    const approved = await apiJson<{ status: string }>(await api.post(`requests/${submitted.id}/approve`, {
      headers: authHeader(checkerToken), data: { decision: 'approve' }
    }))
    expect(approved.status).toBe('approved')

    const list = await apiJson<{ data: { id: string, asset_tag: string }[] }>(
      await api.get(`assets?search=${encodeURIComponent(assetName)}`, { headers: authHeader(adminToken) }))
    expect(list.data.length).toBe(1)
    assetId = list.data[0]!.id
    assetTag = list.data[0]!.asset_tag
  })

  test.afterAll(async () => {
    if (checkerId) {
      await api.delete(`users/${checkerId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('reminder banner shows before Hitung; computing the period posts a 100.000 schedule row for the asset', async ({ page }) => {
    await login(page)
    await page.goto('/depreciation')
    await expect(page.getByRole('heading', { name: 'Depresiasi', level: 1 })).toBeVisible({ timeout: 10_000 })

    await expect(page.getByTestId('depr-reminder')).toBeVisible({ timeout: 10_000 })

    // Never computed yet: ListAssetsForScheduleUnion (service.go) returns
    // EVERY capitalized asset with no posted entry for this period+basis, not
    // just already-fully-depreciated ones — so this brand-new asset already
    // renders as a "no entry yet" union row (opening == closing == its raw
    // cost, zero expense/accumulated), not an empty schedule. Confirmed
    // against the real backend (a dry run against this stack) before writing
    // this assertion.
    await page.getByTestId('depr-search').fill(assetName)
    const row = page.locator('[data-testid="depr-schedule-row"]', { hasText: assetName })
    await expect(row).toHaveCount(1, { timeout: 10_000 })
    await expect(row).toContainText('4.800.000')

    await page.getByTestId('depr-compute').click()
    await expect(page.locator('text=Terhitung').first()).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('Sudah dihitung', { exact: false })).toBeVisible()
    await expect(page.getByTestId('depr-reminder')).toHaveCount(0)

    // KPI tiles are fleet-wide aggregates (see header note) — just assert
    // they left the loading skeleton and rendered a currency value.
    await expect(page.getByTestId('depr-kpi-book-value')).toContainText('Rp', { timeout: 10_000 })
    await expect(page.getByTestId('depr-kpi-period-expense')).toContainText('Rp')

    await expect(row).toHaveCount(1, { timeout: 10_000 })
    await expect(row).toContainText('100.000')
    await expect(row).toContainText('Garis Lurus')
  })

  test('basis toggle to Fiskal shows the same row and switches the KPI reference to PMK 72/2023', async ({ page }) => {
    await login(page)
    await page.goto('/depreciation')
    await page.getByTestId('depr-search').fill(assetName)

    await expect(page.getByTestId('depr-kpi-accumulated')).toContainText('PSAK 16', { timeout: 10_000 })
    const rowCommercial = page.locator('[data-testid="depr-schedule-row"]', { hasText: assetName })
    await expect(rowCommercial).toHaveCount(1, { timeout: 10_000 })
    await expect(rowCommercial).toContainText('100.000')

    await page.getByTestId('depr-basis-fiscal').click()
    await expect(page.getByTestId('depr-kpi-accumulated')).toContainText('PMK 72/2023', { timeout: 10_000 })

    const rowFiscal = page.locator('[data-testid="depr-schedule-row"]', { hasText: assetName })
    await expect(rowFiscal).toHaveCount(1, { timeout: 10_000 })
    await expect(rowFiscal).toContainText('100.000')
  })

  test('journal tab shows a balanced recap and exports xlsx/pdf as real downloads', async ({ page }) => {
    await login(page)
    await page.goto('/depreciation')
    await page.getByTestId('depr-tab-journal').click()

    await expect(page.getByTestId('depr-journal-balanced')).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('[data-testid="depr-journal-row"]').first()).toBeVisible()

    const [xlsx] = await Promise.all([
      page.waitForEvent('download'),
      page.getByTestId('depr-export-xlsx').click()
    ])
    expect(xlsx.suggestedFilename()).toMatch(/\.xlsx$/)

    const [pdf] = await Promise.all([
      page.waitForEvent('download'),
      page.getByTestId('depr-export-pdf').click()
    ])
    expect(pdf.suggestedFilename()).toMatch(/\.pdf$/)
  })

  test('Tutup Periode closes the period; Hitung/Tutup controls disappear', async ({ page }) => {
    await login(page)
    await page.goto('/depreciation')

    await page.getByTestId('depr-close').click()
    await expect(page.getByText('Ditutup', { exact: true }).first()).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('Periode Ditutup', { exact: true }).first()).toBeVisible()
    await expect(page.getByTestId('depr-compute')).toHaveCount(0)
    await expect(page.getByTestId('depr-close')).toHaveCount(0)
  })

  test('impairment write-down: modal computes the loss and the asset record reflects it', async ({ page }) => {
    await login(page)
    await page.goto('/depreciation')
    await page.getByTestId('depr-search').fill(assetName)
    const row = page.locator('[data-testid="depr-schedule-row"]', { hasText: assetName })
    await expect(row).toHaveCount(1, { timeout: 10_000 })

    // Impair action moved into the shared RowActionsMenu kebab (⋮).
    await row.getByRole('button', { name: 'Aksi', exact: true }).click()
    await page.getByRole('menuitem', { name: 'Catat Penurunan Nilai', exact: true }).click()
    await expect(page.getByTestId('depr-impair-current-value')).toContainText('4.400.000', { timeout: 10_000 })

    await page.getByTestId('depr-impair-recoverable').fill('1000000')
    await expect(page.getByTestId('depr-impair-loss')).toContainText('3.400.000')
    await page.getByTestId('depr-impair-reason').fill(`Kerusakan permanen e2e ${RUN}`)
    await page.getByTestId('depr-impair-save').click()

    await expect(page.getByTestId('depr-impair-save')).not.toBeVisible({ timeout: 10_000 })
    await expect(row.locator('[title="Aset telah di-impair"]')).toBeVisible({ timeout: 10_000 })

    // See the header IMPORTANT note: impairment writes asset.book_value /
    // impairment_loss directly, not a new ledger entry — verify it on the
    // asset's own record.
    const asset = await apiJson<{ book_value: string, impairment_loss: string }>(
      await api.get(`assets/${assetId}`, { headers: authHeader(adminToken) }))
    expect(asset.book_value).toBe('1000000.00')
    expect(asset.impairment_loss).toBe('3400000.00')
  })

  test('asset detail Depreciation tab shows the full posted history for both bases', async ({ page }) => {
    await login(page)
    await page.goto(`/assets/${assetTag}`)
    await page.getByRole('button', { name: 'Jadwal Depresiasi', exact: true }).click()

    const rows = page.locator('[data-testid="depr-tab-row"]')
    await expect(rows.first()).toBeVisible({ timeout: 10_000 })
    await expect(rows).toHaveCount(4)
    await expect(rows.first()).toContainText('100.000')

    await page.getByTestId('depr-tab-basis-fiscal').click()
    await expect(rows).toHaveCount(4, { timeout: 10_000 })
    await expect(rows.first()).toContainText('100.000')
  })

  test('disposal integration: the submitted request amount matches the ledger book value, not the impairment write-down', async () => {
    const submitRes = await api.post('disposals', {
      headers: authHeader(adminToken),
      data: { asset_id: assetId, method: 'sale', disposal_date: todayISO(), proceeds: '500000' }
    })
    const { request_id: requestId } = await apiJson<{ request_id: string, status: string }>(submitRes)

    const reqDetail = await apiJson<{ amount: string, payload: { book_value_at_disposal: string } }>(
      await api.get(`requests/${requestId}`, { headers: authHeader(adminToken) }))

    // 4,800,000 cost − 4 × 100,000 posted (this run's compute walked
    // Apr–Jul, or whichever 4-month window monthsAgoISO(3) landed on) =
    // 4,400,000 — the SAME commercial ledger closing the schedule/asset-detail
    // tab showed above, NOT the 1,000,000 impairment write-down (see the
    // impairment test's note: it never touches the ledger).
    expect(reqDetail.amount).toBe('4400000.00')
    expect(reqDetail.payload.book_value_at_disposal).toBe('4400000.00')
  })
})
