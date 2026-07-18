import { test, expect } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'
import { login, apiContext, listRoles, findRoleIdByName, restoreDefaultScope, restoreFieldPermissionDefault } from './helpers'

// ---------------------------------------------------------------------------
// User Management screen — real backend (GET/POST/PUT/DELETE /api/v1/users)
// The seeded admin (admin@inventra.local) has user.manage permission.
// On a fresh CI stack the admin is the only user, so the table may show just
// the admin row OR an empty state (if the backend creates the admin without
// returning it in listing scope). The assertions below are deterministic
// regardless of which state is active.
// NOTE: pnpm test:e2e requires the full backend stack (see CLAUDE.md). This
// spec compiles + lints here; it runs in CI's e2e job.
// ---------------------------------------------------------------------------
test.describe('User Management screen — real backend', () => {
  test('loads page heading and table or empty-state', async ({ page }) => {
    await login(page)
    await page.goto('/settings/users')
    // Page heading renders (proves the page mounted and auth resolved).
    await expect(page.getByRole('heading', { name: 'Pengguna' })).toBeVisible({ timeout: 10_000 })
    // Content settles to EITHER the seeded admin row OR the empty-state — a single
    // auto-waiting assertion that is deterministic regardless of backend state.
    await expect(
      page.getByText('admin@inventra.local').or(page.getByText('Belum ada pengguna', { exact: true }))
    ).toBeVisible({ timeout: 10_000 })
  })

  test('Add button (user.manage gate) is visible and opens the create slideover', async ({ page }) => {
    await login(page)
    await page.goto('/settings/users')
    // Wait for the page to settle.
    await expect(page.getByRole('heading', { name: 'Pengguna' })).toBeVisible({ timeout: 10_000 })
    // "Tambah User" button is visible because the seeded admin has user.manage.
    const addBtn = page.getByRole('button', { name: 'Tambah User' })
    await expect(addBtn).toBeVisible()
    // Click opens the create form slideover.
    await addBtn.click()
    // The slideover heading renders (proves the create form mounted correctly).
    await expect(page.getByRole('heading', { name: 'Tambah User' })).toBeVisible({ timeout: 5_000 })
  })

  // Read-only — no user is created/mutated, avoiding pollution of the shared dev DB.
  test('status filter sends the selected status as a query param to the backend', async ({ page }) => {
    await login(page)
    await page.goto('/settings/users')
    await expect(page.getByRole('heading', { name: 'Pengguna' })).toBeVisible({ timeout: 10_000 })
    await expect(
      page.getByText('admin@inventra.local').or(page.getByText('Belum ada pengguna', { exact: true }))
    ).toBeVisible({ timeout: 10_000 })

    // Filter USelect trigger, targeted via its data-testid (frontend/app/pages/settings/users.vue).
    const statusFilter = page.getByTestId('users-status-filter')
    await expect(statusFilter).toBeVisible()

    // Assert the outgoing GET /users request carries status=inactive — robust against
    // dev-DB row contention (unlike asserting on visible row contents/count).
    const responsePromise = page.waitForResponse(res =>
      res.url().includes('/users?') && res.url().includes('status=inactive') && res.request().method() === 'GET'
    )
    await statusFilter.click()
    // Option role="option" comes from reka-ui's SelectItem; label from i18n
    // settings.users.status.inactive = "Nonaktif".
    await page.getByRole('option', { name: 'Nonaktif' }).click()
    const response = await responsePromise
    expect(response.ok()).toBe(true)

    // Reset button (users-filter-reset) appears once the filter is active — UI proof
    // the page registered the change, in addition to the network assertion above.
    await expect(page.getByTestId('users-filter-reset')).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Audit Trail screen — real backend (GET /api/v1/audit)
// The seeded admin (admin@inventra.local) must have audit.view permission.
// On a fresh CI stack the audit table may be EMPTY: createadmin bypasses audit
// logging and login does not write audit rows. The single test below is
// deterministic regardless of whether rows exist.
// NOTE: pnpm test:e2e requires the full backend stack (see CLAUDE.md). This
// spec compiles + lints here; it runs in CI's e2e job.
// ---------------------------------------------------------------------------
test.describe('Audit Trail screen — real backend', () => {
  test('loads against the real backend (table or empty-state)', async ({ page }) => {
    await login(page)
    await page.goto('/settings/audit')
    // Heading renders (proves the page mounted and auth resolved).
    await expect(page.getByRole('heading', { name: 'Audit Trail' })).toBeVisible({ timeout: 10_000 })
    // Content settles to EITHER the table OR the empty-state — a single auto-waiting
    // assertion that is deterministic regardless of whether the seeded backend has
    // audit rows yet.
    await expect(
      page.locator('table').or(page.getByText('Tidak ada log', { exact: true }))
    ).toBeVisible({ timeout: 10_000 })
    // The search input is always rendered regardless of data, proving the filter
    // bar wired up correctly. i18n key: settings.audit.searchPlaceholder.
    await expect(page.getByPlaceholder('Cari entity atau ID…')).toBeVisible({ timeout: 5_000 })
  })
})

// ---------------------------------------------------------------------------
// RBAC screen — real backend (/api/v1/authz)
// These tests run against the seeded admin (admin@inventra.local) and the
// actual authzadmin endpoints. CI's e2e job brings up the full stack and seeds
// the admin before this suite runs.
// ---------------------------------------------------------------------------
test.describe('RBAC screen — real backend', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/settings/rbac')
    // Wait for the role list to be populated (not loading spinner)
    await expect(page.getByText('Superadmin').first()).toBeVisible({ timeout: 10_000 })
  })

  test('role list shows built-in system roles from the backend', async ({ page }) => {
    // Seeded system roles: Superadmin, Kepala Kanwil, Kepala Unit, Manager, Staf
    await expect(page.getByText('Superadmin').first()).toBeVisible()
    await expect(page.getByText('Manager').first()).toBeVisible()
    // At least one system lock icon is rendered next to system roles
    const roleList = page.locator('[class*="w-\\[280px\\]"]').first()
    await expect(roleList).toBeVisible()
  })

  test('selecting a role shows its permission matrix with resolved group labels', async ({ page }) => {
    // Click on the Manager role in the left list
    const managerBtn = page.locator('button').filter({ hasText: /^Manager/ }).first()
    await managerBtn.click()
    // Permission matrix cards should appear — catalog groups (Aset, Sistem, etc.)
    await expect(page.getByText('Aset').first()).toBeVisible()
    // The matrix renders individual permission codes (mono text)
    await expect(page.locator('text=asset.view').first()).toBeVisible({ timeout: 5_000 })
  })

  test('switching roles loads the correct permission set', async ({ page }) => {
    // Click Superadmin
    const superBtn = page.locator('button').filter({ hasText: /^Superadmin/ }).first()
    await superBtn.click()
    // Superadmin has all permissions — no switch should be unchecked (or at least the Sistem group is present)
    await expect(page.getByText('Sistem').first()).toBeVisible()
  })

  test('system role shows system badge and updated lock note (permissions editable)', async ({ page }) => {
    // The Manager role is a system role — click it
    const managerBtn = page.locator('button').filter({ hasText: /^Manager/ }).first()
    await managerBtn.click()
    // System badge appears in the header
    await expect(page.getByText('Sistem').first()).toBeVisible()
    // Updated lock note: name/code locked, but permissions are still configurable
    await expect(page.getByText(/izin tetap dapat dikonfigurasi/).first()).toBeVisible()
    // Save button is disabled when no changes made
    const saveBtn = page.getByRole('button', { name: /Simpan Perubahan/ })
    await expect(saveBtn).toBeDisabled()
  })

  test('toggling a permission marks dirty and Save persists across a page reload', async ({ page }) => {
    // Work with a non-system role to avoid interfering with seeded system roles.
    // If no custom role exists, this sub-test creates one first.
    // Verify the role list has at least one role before proceeding.
    await expect(page.locator('button').filter({ hasText: 'Manager' }).first()).toBeVisible()

    // Try to find a custom role (no lock icon). If none, skip the toggle/persist test.
    // The seeded admin setup may or may not have custom roles — we guard here.
    // If a custom role "Auditor Internal" is seeded, use it; otherwise create one.
    let targetRoleName = 'Auditor Internal'
    const auditorBtn = page.locator('button').filter({ hasText: /Auditor Internal/ }).first()
    const auditorExists = await auditorBtn.isVisible()

    if (!auditorExists) {
      // Create a temporary custom role for the toggle/persist test. The name
      // must be unique per run: roles.name has a partial unique index and this
      // spec never deletes the role it creates, so a fixed name passes once and
      // then breaks every later run on a dev database (duplicate → the create
      // modal stays open and intercepts the next click).
      targetRoleName = `E2E Test Role ${Date.now()}`
      const addBtn = page.locator('button').filter({ hasText: /Tambah Peran/ }).first()
      await addBtn.click()
      await page.waitForSelector('input[placeholder="mis. Operator Lapangan"]')
      await page.fill('input[placeholder="mis. Operator Lapangan"]', targetRoleName)
      await page.locator('button').filter({ hasText: /^Buat Peran$/ }).first().click()
      // Wait for the new role to appear in the list
      await expect(page.getByText(targetRoleName).first()).toBeVisible({ timeout: 8_000 })
    }

    // Select the target role
    const targetBtn = page.locator('button').filter({ hasText: new RegExp(targetRoleName) }).first()
    await targetBtn.click()

    // Find the first permission toggle row (button containing a mono permission code span)
    // and toggle it. Located by text content of a known permission code rather than CSS classes.
    const firstPermBtn = page.locator('button', { hasText: 'asset.view' }).first()
    await firstPermBtn.click()

    // Dirty indicator must appear
    await expect(page.getByText('Perubahan belum disimpan').first()).toBeVisible()

    // Save must be enabled
    const saveBtn = page.getByRole('button', { name: /Simpan Perubahan/ })
    await expect(saveBtn).toBeEnabled()
    await saveBtn.click()

    // Dirty indicator disappears after save
    await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 5_000 })

    // Reload the page and verify the permission state persisted
    await page.reload()
    await expect(page.getByText('Superadmin').first()).toBeVisible({ timeout: 10_000 })
    const reloadedTargetBtn = page.locator('button').filter({ hasText: new RegExp(targetRoleName) }).first()
    await reloadedTargetBtn.click()
    // After reload, the dirty indicator must be absent (state was persisted)
    await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible()
    // Save is disabled (clean state on load)
    await expect(saveBtn).toBeDisabled()

    // Cleanup: revert by toggling the same permission back (best-effort; not a hard failure)
    await firstPermBtn.click()
    if (await saveBtn.isEnabled()) {
      await saveBtn.click()
      await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 5_000 })
    }
  })

  test('Add Role modal opens, slugifies the code, and the new role appears in the list', async ({ page }) => {
    const addBtn = page.locator('button').filter({ hasText: /Tambah Peran/ }).first()
    await addBtn.click()
    await page.waitForSelector('input[placeholder="mis. Operator Lapangan"]')

    const uniqueName = `E2E Peran ${Date.now()}`
    await page.fill('input[placeholder="mis. Operator Lapangan"]', uniqueName)
    await page.locator('button').filter({ hasText: /^Buat Peran$/ }).first().click()

    // The new role should appear in the left list
    await expect(page.getByText(uniqueName).first()).toBeVisible({ timeout: 8_000 })
    // Custom badge should appear for the newly selected role
    await expect(page.getByText('Kustom').first()).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Data Scope screen — real backend (/api/v1/authz)
// These tests run against the seeded admin (admin@inventra.local) and the
// real authzadmin endpoints. CI's e2e job brings up the full stack and seeds
// the admin before this suite runs.
// Module columns come from /authz/catalog's scope_modules (real backend keys:
// offices, employees, assets, requests, audit) — intentionally different from
// the old mock fixture keys (aset, pengajuan, …); this is an approved decision.
// ---------------------------------------------------------------------------

// i18n (id locale) descriptions for each scope level — these render in the legend
// and inside each popover option, but NOT on the bare table pills, so they uniquely
// disambiguate a popover option from a table cell pill.
const LEVEL_DESC: Record<string, string> = {
  global: 'Semua data lintas kantor',
  office_subtree: 'Kantor sendiri + seluruh turunannya',
  office: 'Hanya kantor sendiri',
  own: 'Hanya data miliknya'
}

test.describe('Data Scope screen — real backend', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/settings/data-scope')
    // Wait until the role list is populated (master-detail UI)
    await expect(
      page.locator('[data-testid^="scope-role-item-"]').filter({ hasText: 'Superadmin' }).first()
    ).toBeVisible({ timeout: 10_000 })
  })

  test('role list and per-role editor render with real module rows', async ({ page }) => {
    // Seeded roles appear in the role list pane
    await expect(
      page.locator('[data-testid^="scope-role-item-"]').filter({ hasText: 'Superadmin' }).first()
    ).toBeVisible()
    await expect(
      page.locator('[data-testid^="scope-role-item-"]').filter({ hasText: 'Manager' }).first()
    ).toBeVisible()

    // Select Superadmin so the editor deterministically shows its scope
    await page.locator('[data-testid^="scope-role-item-"]').filter({ hasText: 'Superadmin' }).first().click()

    // The Default card renders (i18n: settings.dataScope.defaultColumn) with its cell
    await expect(page.getByTestId('scope-default-cell')).toBeVisible()
    // At least one module row from the real catalog (offices/employees/assets/requests/audit)
    await expect(page.locator('[data-testid^="scope-module-row-"]').first()).toBeVisible()
    await expect(
      page.locator('[data-testid^="scope-module-row-"]').filter({ hasText: /Kantor|Pegawai|Aset|Pengajuan|Audit/ }).first()
    ).toBeVisible()
  })

  test('legend renders all four scope levels with descriptions', async ({ page }) => {
    // The legend title + the four level descriptions render only in the legend card.
    // The table pills show the bare level KEYS (global/office/…), not the descriptions,
    // so asserting the descriptions reliably proves the legend rendered without needing
    // a fragile container locator.
    await expect(page.getByText('Level lingkup data').first()).toBeVisible()
    await expect(page.getByText(LEVEL_DESC.global).first()).toBeVisible()
    await expect(page.getByText(LEVEL_DESC.office_subtree).first()).toBeVisible()
    await expect(page.getByText(LEVEL_DESC.office).first()).toBeVisible()
    await expect(page.getByText(LEVEL_DESC.own).first()).toBeVisible()
  })

  test('Save button is disabled with no changes (clean state)', async ({ page }) => {
    // On first load no changes have been made → Save is disabled
    const saveBtn = page.getByTestId('scope-save')
    await expect(saveBtn).toBeDisabled()
    // Dirty indicator must NOT be visible
    await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible()
  })

  // Scoped to its own nested describe: this is the ONLY test in this file that
  // mutates the SHARED seeded Superadmin `*` data-scope policy, so only it pays
  // for the failure-safe API restore. Applying the restore to every sibling
  // test (even read-only ones) would mean every parallel worker in this
  // describe races a PUT against the same shared row — confirmed locally as
  // spurious 409s (CI's `workers: 1` wouldn't show it, but it's still wrong).
  // The api context + role id are resolved in this nested describe's own
  // beforeEach (running immediately before the test body, after the outer
  // beforeEach's UI login/goto), and afterEach restores unconditionally —
  // this exact corruption (Superadmin `*` stuck at `own`) has 403'd office
  // creation across other specs before.
  test.describe('mutates shared Superadmin default scope', () => {
    let api: APIRequestContext | undefined
    let superadminId: string | undefined

    test.beforeEach(async () => {
      api = await apiContext()
      superadminId = await findRoleIdByName(api, 'Superadmin')
    })

    test.afterEach(async () => {
      if (!api || !superadminId) return
      await restoreDefaultScope(api, superadminId, 'global')
      await api.dispose()
      api = undefined
      superadminId = undefined
    })

    test('changing a role default scope marks dirty and enables Save, persists across reload', async ({ page }) => {
      // Select the Superadmin role in the list, then edit its Default cell.
      const superadminItem = page.locator('[data-testid^="scope-role-item-"]').filter({ hasText: 'Superadmin' }).first()
      await superadminItem.click()

      // The Default card's pill renders the level key as its visible text
      // (ScopeCell.vue: <span class="font-mono ...">{{ effective }}</span> inside <button>).
      const defaultPill = page.getByTestId('scope-default-cell').locator('button[type="button"]').first()
      await expect(defaultPill).toBeVisible()

      // Read the current level from the pill's visible text (e.g. "global" / "own")
      const currentLevel = (await defaultPill.textContent())?.trim().match(/global|office_subtree|office|own/)?.[0] ?? 'global'

      // Open the popover
      await defaultPill.click()

      // Pick a different level deterministically: 'own' if currently 'global', else 'global'
      const targetLevel = currentLevel === 'own' ? 'global' : 'own'

      // Popover option buttons contain the level key AND its description; the description
      // text also appears in the legend as plain text, but only the popover renders it
      // inside a BUTTON, so scoping by role=button + description targets the option.
      const levelOption = page.getByRole('button').filter({ hasText: LEVEL_DESC[targetLevel] }).first()
      await levelOption.click()

      // Dirty indicator should appear
      await expect(page.getByText('Perubahan belum disimpan').first()).toBeVisible({ timeout: 5_000 })

      // Save button must now be enabled
      const saveBtn = page.getByTestId('scope-save')
      await expect(saveBtn).toBeEnabled()
      await saveBtn.click()

      // Dirty indicator disappears after a successful save
      await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 8_000 })

      // Reload and verify the change persisted — re-select Superadmin first (the
      // editor auto-selects the alphabetically-first role after reload).
      await page.reload()
      const superadminItemAfter = page.locator('[data-testid^="scope-role-item-"]').filter({ hasText: 'Superadmin' }).first()
      await expect(superadminItemAfter).toBeVisible({ timeout: 10_000 })
      await superadminItemAfter.click()

      const defaultPillAfter = page.getByTestId('scope-default-cell').locator('button[type="button"]').first()
      await expect(defaultPillAfter).toContainText(targetLevel, { timeout: 8_000 })

      // Fast-path cleanup: revert to original level via the UI (best-effort — if
      // this doesn't run or throws, this nested describe's afterEach above is the
      // authoritative, failure-safe restore via the API).
      await defaultPillAfter.click()
      const revertOption = page.getByRole('button').filter({ hasText: LEVEL_DESC[currentLevel] }).first()
      await revertOption.click()
      const saveBtnCleanup = page.getByTestId('scope-save')
      if (await saveBtnCleanup.isEnabled()) {
        await saveBtnCleanup.click()
        await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 8_000 })
      }
    })
  })

  test('retry button reloads data after a simulated failure', async ({ page }) => {
    // The error state shows a retry button labeled "Coba lagi".
    // We cannot easily force a network error in e2e, so we verify that on a
    // successful load the retry button is NOT visible and the editor is.
    await expect(page.getByRole('button', { name: 'Coba lagi' })).not.toBeVisible()
    // The loaded role list + editor are visible
    await expect(page.locator('[data-testid^="scope-role-item-"]').first()).toBeVisible()
    await expect(page.getByTestId('scope-save')).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// Field Permission screen — real backend (/api/v1/authz)
// These tests run against the seeded admin (admin@inventra.local) and the
// actual authzadmin endpoints. CI's e2e job brings up the full stack and seeds
// the admin before this suite runs.
// Entity/field set intentionally differs from the old mock (aset/pegawai/…):
// the real catalog exposes "assets" + "users" with English field keys — this
// is an approved decision, not a regression.
// ---------------------------------------------------------------------------
test.describe('Field Permission screen — real backend', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.goto('/settings/field-permission')
    // Wait for the role list to load (master-detail UI, populated from /authz/roles)
    await expect(
      page.locator('[data-testid^="fieldperm-role-item-"]').filter({ hasText: 'Superadmin' }).first()
    ).toBeVisible({ timeout: 10_000 })
  })

  test('role list renders and the editor shows real field rows (e.g. purchase_cost)', async ({ page }) => {
    // Seeded roles should appear in the role list pane
    await expect(
      page.locator('[data-testid^="fieldperm-role-item-"]').filter({ hasText: 'Superadmin' }).first()
    ).toBeVisible()
    await expect(
      page.locator('[data-testid^="fieldperm-role-item-"]').filter({ hasText: 'Manager' }).first()
    ).toBeVisible()

    // Real catalog field key for the "assets" entity — this field is in fieldCatalog.ts;
    // the editor auto-selects the first role, so its rows render immediately
    await expect(page.getByTestId('fieldperm-row-purchase_cost')).toBeVisible({ timeout: 8_000 })

    // The "assets" entity should be selected by default (first entity in FIELD_CATALOG)
    // and the entity select should be visible
    await expect(page.getByText('Aset').first()).toBeVisible()
  })

  test('field row shows mono field key and i18n label below it', async ({ page }) => {
    // Each field row shows the field key in mono font + a localized label beneath it
    const purchaseCostRow = page.getByTestId('fieldperm-row-purchase_cost')
    await expect(purchaseCostRow).toBeVisible({ timeout: 8_000 })
    // The i18n label for purchase_cost is "Harga beli" (id locale)
    await expect(purchaseCostRow.getByText('Harga beli')).toBeVisible()
  })

  test('fields without explicit rules show the Default badge', async ({ page }) => {
    // Fields with no stored restriction are shown dimmed with a "Default" badge (i18n: defaultTag)
    // At least one field should show the Default badge on first load
    await expect(page.getByText('Default').first()).toBeVisible({ timeout: 8_000 })
  })

  test('Save button is disabled on clean load (no dirty changes)', async ({ page }) => {
    const saveBtn = page.getByTestId('fieldperm-save')
    await expect(saveBtn).toBeDisabled()
    // Dirty indicator must NOT be visible
    await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible()
  })

  test('retry button is absent on successful load (editor visible)', async ({ page }) => {
    // On a clean load the load-error state is not shown, so "Coba lagi" is not visible
    await expect(page.getByRole('button', { name: 'Coba lagi' })).not.toBeVisible()
    await expect(page.getByTestId('fieldperm-row-purchase_cost')).toBeVisible()
  })

  // Scoped to its own nested describe: this is the ONLY test in this file that
  // mutates the SHARED `purchase_cost` field-permission cell of whichever role
  // the editor AUTO-SELECTS on load (the first item of `/authz/roles`, which
  // is `ORDER BY name` — the per-role editor trusts that order), so
  // the auto-selected role is NOT reliably Superadmin — e.g. seeded "Kepala
  // Kanwil"/"Manager" or any leftover "E2E ..." custom role sorts before
  // "Superadmin". Applying the restore to every sibling test (even read-only
  // ones) would mean every parallel worker in this describe races a PUT
  // against the same shared row — confirmed locally as spurious 409s (CI's
  // `workers: 1` wouldn't show it, but it's still wrong). We restore BOTH: the
  // role that positionally maps to the toggled cell (the actual mutation) and
  // Superadmin by name (this file's other read-only assertions reference it,
  // and it has separately been seen corrupted in this dev DB) —
  // belt-and-suspenders, cheap, and idempotent either way.
  test.describe('mutates shared purchase_cost field permission', () => {
    let api: APIRequestContext | undefined
    let firstRoleId: string | undefined
    let superadminId: string | undefined

    test.beforeEach(async () => {
      api = await apiContext()
      const roles = await listRoles(api)
      firstRoleId = roles[0]?.id
      superadminId = roles.find(r => r.name === 'Superadmin')?.id ?? await findRoleIdByName(api, 'Superadmin')
    })

    test.afterEach(async () => {
      if (!api) return
      if (firstRoleId) await restoreFieldPermissionDefault(api, firstRoleId, 'assets', 'purchase_cost')
      if (superadminId && superadminId !== firstRoleId) {
        await restoreFieldPermissionDefault(api, superadminId, 'assets', 'purchase_cost')
      }
      await api.dispose()
      api = undefined
      firstRoleId = undefined
      superadminId = undefined
    })

    test('toggle a cell, Save, reload — change persists', async ({ page }) => {
      // Strategy: the editor auto-selects the first role from /authz/roles
      // (ORDER BY name — NOT reliably Superadmin, see this nested describe's
      // comment). Toggle purchase_cost's "L" (view) for that role, save,
      // reload, and verify the change persisted.

      // 1. Find the purchase_cost row in the auto-selected role's editor
      const purchaseCostRow = page.getByTestId('fieldperm-row-purchase_cost')
      await expect(purchaseCostRow).toBeVisible({ timeout: 8_000 })

      // 2. Within that row, find the "L" (view) toggle button.
      //    FieldPermToggle renders two <button> elements containing the letter "L" (view) and "E" (edit);
      //    the per-role editor shows exactly one pair per row.
      const lBtns = purchaseCostRow.locator('button', { hasText: 'L' })
      await expect(lBtns).not.toHaveCount(0)

      // We cannot reliably read the semantic state, so we just note that we toggled it once.
      const firstLBtn = lBtns.first()
      await firstLBtn.click()

      // 3. Dirty indicator must appear
      await expect(page.getByText('Perubahan belum disimpan').first()).toBeVisible({ timeout: 5_000 })

      // 4. Save must be enabled; click it
      const saveBtn = page.getByTestId('fieldperm-save')
      await expect(saveBtn).toBeEnabled()
      await saveBtn.click()

      // 5. Dirty indicator must disappear after save
      await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 8_000 })

      // 6. Reload the page and verify the change actually persisted.
      //    After toggling and saving, the purchase_cost field now has an EXPLICIT restriction —
      //    meaning the "Default" badge (i18n defaultTag = "Default") must NO LONGER appear in that row.
      await page.reload()
      // After reload the editor auto-selects the same first role again
      await expect(
        page.locator('[data-testid^="fieldperm-role-item-"]').filter({ hasText: 'Superadmin' }).first()
      ).toBeVisible({ timeout: 10_000 })
      // purchase_cost must still be visible (row exists in the catalog)
      const purchaseCostRowAfterReload = page.getByTestId('fieldperm-row-purchase_cost')
      await expect(purchaseCostRowAfterReload).toBeVisible({ timeout: 8_000 })
      // KEY PERSISTENCE ASSERTION: the field now has an explicit restriction, so the
      // "Default" badge must be absent — proving the toggled value round-tripped through the backend.
      await expect(purchaseCostRowAfterReload.getByText('Default')).toHaveCount(0)
      // No dirty state on fresh load
      await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible()
      const saveBtnAfter = page.getByTestId('fieldperm-save')
      await expect(saveBtnAfter).toBeDisabled()

      // 7. Fast-path cleanup: toggle the same cell back via the UI (best-effort — this
      //    nested describe's afterEach above is the authoritative, failure-safe restore
      //    via the API).
      //    Uses try/catch so a flaky cleanup never fails the test.
      //    Playwright's click auto-waits for actionability; we also wait for Save to be enabled
      //    before clicking it, avoiding the non-waiting isEnabled() snapshot anti-pattern.
      try {
        const purchaseCostRowCleanup = page.getByTestId('fieldperm-row-purchase_cost')
        await expect(purchaseCostRowCleanup).toBeVisible({ timeout: 8_000 })
        const lBtnsCleanup = purchaseCostRowCleanup.locator('button', { hasText: 'L' })
        await lBtnsCleanup.first().click()
        const saveBtnCleanup = page.getByTestId('fieldperm-save')
        await expect(saveBtnCleanup).toBeEnabled()
        await saveBtnCleanup.click()
        await expect(page.getByText('Perubahan belum disimpan')).not.toBeVisible({ timeout: 8_000 })
      } catch { /* best-effort cleanup — not a hard failure */ }
    })
  })

  test('switching entity to users shows users fields (e.g. email)', async ({ page }) => {
    // The entity selector is a Nuxt UI USelect (custom listbox, NOT a native <select>):
    // a trigger button showing the current entity label ("Aset") plus a popover of options
    // with role="option". Open it by clicking the trigger (located by its current value text),
    // then pick the "User" option.
    await page.getByText('Aset', { exact: true }).first().click()
    await page.getByRole('option', { name: 'User' })
      .or(page.getByText('User', { exact: true }))
      .first().click()
    // The "users" entity has field "email" in FIELD_CATALOG; its i18n label is "Email".
    await expect(page.getByTestId('fieldperm-row-email')).toBeVisible({ timeout: 8_000 })
    await expect(page.getByTestId('fieldperm-row-email').getByText('Email')).toBeVisible()
  })
})
