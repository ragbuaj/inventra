import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Nav access — every menu item VISIBLE to a role must actually OPEN (no 403 /
// access-denied boundary). This closes the "menu shown but not reachable" class
// of bug end-to-end: it would have caught the old binary nav (which showed items
// whose page-guard then aborted with 403) and any page-guard/endpoint mismatch.
//
// The CI e2e stack seeds only the Superadmin (see helpers.ts EMAIL/PASSWORD),
// who sees the FULL single per-permission nav — so this sweep verifies the whole
// nav renders and that no route bounces the top role to an "Akses ditolak" page.
//
// The per-role guarantee (visible menu set equals the role's permission set) is
// locked separately by the fast specs:
//   - test/unit/nav-model.spec.ts   (per-role visible-leaf-set = permission-set)
//   - test/nuxt/app-sidebar.spec.ts (per-role sidebar render + empty-group hide)
//   - backend authzadmin delegation integration tests (scope-only / user-only)
// ---------------------------------------------------------------------------

test.describe('Nav access — Superadmin full-nav reachability', () => {
  test('every visible sidebar menu opens without a 403 / access-denied boundary', async ({ page }) => {
    await login(page)

    // Distinct in-app links from the sidebar <nav> (NuxtLink renders <a href>).
    const hrefs = await page.locator('nav a[href^="/"]').evaluateAll(els =>
      Array.from(new Set(
        els
          .map(e => e.getAttribute('href') || '')
          .filter(h => h.length > 0 && !h.startsWith('/login') && !h.includes('#'))
      ))
    )

    // Sanity: the single nav model must surface the full operational + admin menu
    // for the Superadmin (far more than the old 5-item staff menu).
    expect(hrefs.length).toBeGreaterThan(12)

    for (const href of hrefs) {
      await page.goto(href)
      // The `can` middleware aborts with statusMessage 'Akses ditolak' on a
      // permission miss — the Superadmin must never hit it.
      await expect(
        page.locator('body'),
        `menu ${href} should be reachable (no 403)`
      ).not.toContainText('Akses ditolak')
      // App shell still rendered (not the Nuxt error boundary): the sidebar nav
      // is present on every real page.
      await expect(page.locator('nav').first()).toBeVisible()
    }
  })
})
