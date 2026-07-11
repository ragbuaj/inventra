import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'

// ---------------------------------------------------------------------------
// Global search (command palette) — real backend e2e against GET /search.
//
//   1. API setup: create an office type (reference engine) → an office
//      (mirrors assets.spec.ts's FK-prerequisite pattern), unique name+code
//      per run so partial-unique indexes never collide across local re-runs.
//   2. UI: open the palette (Ctrl/Cmd+K), search for the created office's
//      name, confirm the result row renders and navigating lands on
//      /master/offices (offices is an auth-only group — no permission gate —
//      so the seeded Superadmin always sees it; see
//      backend/internal/search/handler.go gateScoped).
//   3. UI: a no-hit query renders the palette's empty state.
//
// Debounce: CommandPalette.vue debounces the query 250ms before calling
// useGlobalSearch — every assertion below uses Playwright's auto-retrying
// expect() instead of a fixed sleep, per project e2e convention.
// ---------------------------------------------------------------------------

const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`

const run = Date.now().toString(36)
const officeName = `Kantor Search E2E ${run}`
const officeCode = `SRCH${run}`.toUpperCase().slice(0, 12)

// --- thin API helpers (own APIRequestContext — the `request` fixture is
// test-scoped and unavailable in `beforeAll`), copied from assets.spec.ts. ---
function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}

async function apiJson<T>(res: APIResponse): Promise<T> {
  if (!res.ok()) {
    throw new Error(`API call failed: ${res.status()} ${res.url()} — ${await res.text()}`)
  }
  return res.json() as Promise<T>
}

async function login_(api: APIRequestContext, email: string, password: string): Promise<string> {
  const res = await api.post('auth/login', { data: { email, password } })
  const body = await apiJson<{ access_token: string }>(res)
  return body.access_token
}

// Open the command palette via the keyboard shortcut. CommandPalette.vue
// attaches its Ctrl/Cmd+K listener in onMounted; a keypress fired right after
// login can occasionally land before the listener exists, so retry the press
// until the palette's input is visible (mirrors the pattern this spec
// previously used for the same race).
async function openPalette(page: Page): Promise<void> {
  await expect(page.getByRole('button', { name: /cari aset, pegawai/i })).toBeVisible({ timeout: 10_000 })
  await expect(async () => {
    await page.keyboard.press('ControlOrMeta+k')
    await expect(page.getByPlaceholder(/cari/i)).toBeVisible({ timeout: 2000 })
  }).toPass({ timeout: 15_000 })
}

test.describe('Global search — real backend e2e', () => {
  let api: APIRequestContext
  let adminToken: string

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const otRes = await api.post('office-types', {
      headers: authHeader(adminToken),
      data: { name: `E2E Search OT ${run}` }
    })
    const officeType = await apiJson<{ id: string }>(otRes)

    const offRes = await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeName, code: officeCode, office_type_id: officeType.id }
    })
    await apiJson<{ id: string }>(offRes)
  })

  test.afterAll(async () => {
    await api.dispose()
  })

  test('palette finds a created office and navigates', async ({ page }) => {
    await login(page)
    await openPalette(page)

    await page.getByPlaceholder(/cari/i).fill(officeName)

    // Group header — search.group.kantor — renders above the office's result row.
    await expect(page.getByText('Kantor', { exact: true })).toBeVisible({ timeout: 10_000 })

    const resultButton = page.getByRole('button', { name: new RegExp(officeName) })
    await expect(resultButton).toBeVisible({ timeout: 10_000 })
    await resultButton.click()

    await expect(page).toHaveURL(/\/master\/offices/, { timeout: 10_000 })
  })

  test('palette shows the empty state for a no-hit query', async ({ page }) => {
    await login(page)
    await openPalette(page)

    await page.getByPlaceholder(/cari/i).fill(`zzz-no-hit-${run}`)

    await expect(page.getByText(/tidak ada hasil/i)).toBeVisible({ timeout: 10_000 })
  })
})
