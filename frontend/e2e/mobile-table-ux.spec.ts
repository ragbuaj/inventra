import { test, expect } from '@playwright/test'
import { login } from './helpers'

/**
 * Compact-layout behaviour for data lists: the collapsed filter bar and the
 * infinite card list on the Asset Catalog.
 *
 * The viewport is set per file rather than through a new Playwright project so
 * `playwright.config.ts` and CI's two-phase run (chromium against a clean DB,
 * lampiran against the demo seed) stay untouched.
 *
 * NOTE: infinite scroll depends on IntersectionObserver, which needs a really
 * rendering page. This is the only harness in the repo that provides one — the
 * in-app browser pane suspends intersection callbacks entirely.
 */
test.use({ viewport: { width: 390, height: 844 } })

const CATALOG = '/assets'

/** Cards rendered by InfiniteList on the catalog. */
function cards(page: import('@playwright/test').Page) {
  return page.getByTestId('assets-infinite').locator('[data-testid="asset-card"]')
}

test.describe('Compact layout — collapsed filter bar', () => {
  test('hides the advanced filters behind a button and reveals them in a bottom sheet', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)

    const search = page.getByTestId('assets-filter-search')
    await expect(search).toBeVisible()

    // The advanced controls are not merely hidden — they are not rendered.
    await expect(page.getByTestId('assets-filter-panel')).toHaveCount(0)
    await expect(page.getByTestId('assets-office-filter-picker-input')).toHaveCount(0)

    const toggle = page.getByTestId('assets-filter-toggle')
    await expect(toggle).toBeVisible()
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')

    await toggle.click()
    await expect(page.getByTestId('assets-filter-panel')).toBeVisible()
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')

    // All four advanced controls travelled into the sheet.
    await expect(page.getByTestId('assets-filter-panel').getByRole('combobox')).toHaveCount(4)
  })

  test('applies a filter live and reflects it in the count badge', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)
    await expect(cards(page).first()).toBeVisible({ timeout: 15_000 })

    const toggle = page.getByTestId('assets-filter-toggle')
    // No advanced filter yet: the label carries no count.
    await expect(toggle).toHaveAttribute('aria-label', 'Filter lanjutan')

    await toggle.click()
    const panel = page.getByTestId('assets-filter-panel')
    await expect(panel).toBeVisible()

    // Filtering happens on change, with no Apply step.
    const listResponse = page.waitForResponse(res =>
      res.url().includes('/assets?') && res.url().includes('status=under_maintenance')
    )
    await panel.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'Maintenance', exact: true }).click()
    expect((await listResponse).ok()).toBe(true)

    // The badge and the accessible label both report one active filter.
    await expect(toggle).toHaveAttribute('aria-label', /1/)
    await expect(page.getByTestId('assets-filter-toggle').locator('..')).toContainText('1')

    // Closing the sheet keeps the filter applied.
    await page.getByTestId('assets-filter-apply').click()
    await expect(panel).toBeHidden()
    await expect(toggle).toHaveAttribute('aria-label', /1/)
  })

  test('resets every filter from inside the sheet', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)
    await expect(cards(page).first()).toBeVisible({ timeout: 15_000 })

    await page.getByTestId('assets-filter-toggle').click()
    const panel = page.getByTestId('assets-filter-panel')
    await panel.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'Maintenance', exact: true }).click()
    await expect(page.getByTestId('assets-filter-toggle')).toHaveAttribute('aria-label', /1/)

    await page.getByTestId('assets-filter-reset').click()
    await expect(page.getByTestId('assets-filter-toggle')).toHaveAttribute('aria-label', 'Filter lanjutan')
  })

  test('restores the inline filter row at regular width', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)
    await expect(page.getByTestId('assets-filter-toggle')).toBeVisible()

    await page.setViewportSize({ width: 1280, height: 900 })

    // Controls come back inline and the compact affordances disappear. The
    // media-query listener plus a re-render can take a beat, hence the poll.
    await expect(page.getByTestId('assets-filter-toggle')).toHaveCount(0, { timeout: 10_000 })
    await expect(page.getByTestId('assets-office-filter-picker-input')).toBeVisible({ timeout: 10_000 })
  })
})

test.describe('Compact layout — infinite card list', () => {
  test('swaps page buttons for an accumulating list', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)
    await expect(cards(page).first()).toBeVisible({ timeout: 15_000 })

    await expect(page.getByTestId('pagination-next')).toHaveCount(0)
    await expect(page.getByTestId('infinite-list-sentinel')).toHaveCount(1)
    await expect(page.getByTestId('infinite-list-status')).toHaveAttribute('aria-live', 'polite')
  })

  test('appends the next page on scroll without disturbing the rows already shown', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)
    await expect(cards(page).first()).toBeVisible({ timeout: 15_000 })

    // Anchor on the asset tag: the rest of a card's text fills in
    // asynchronously as the office/brand lookups resolve.
    const firstTag = await page.getByTestId('asset-card-tag').first().innerText()
    const before = await cards(page).count()
    expect(before).toBeGreaterThan(0)

    await page.locator('main').evaluate(el => el.scrollTo({ top: el.scrollHeight }))
    await expect.poll(() => cards(page).count(), { timeout: 15_000 }).toBeGreaterThan(before)

    // Appended, not replaced: the first card is still the first card.
    expect(await page.getByTestId('asset-card-tag').first().innerText()).toBe(firstTag)
  })

  test('keeps page buttons at regular width and loads nothing on scroll', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 })
    await login(page)
    await page.goto(CATALOG)
    await expect(page.getByTestId('pagination-next')).toBeVisible({ timeout: 15_000 })

    const rowCount = await page.locator('tbody tr').count()
    await page.locator('main').evaluate(el => el.scrollTo({ top: el.scrollHeight }))
    await page.waitForTimeout(1_500)
    expect(await page.locator('tbody tr').count()).toBe(rowCount)
  })

  test('starts over from the top when the search term changes', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)
    await expect(cards(page).first()).toBeVisible({ timeout: 15_000 })

    await page.locator('main').evaluate(el => el.scrollTo({ top: el.scrollHeight }))
    await expect.poll(() => cards(page).count(), { timeout: 15_000 }).toBeGreaterThan(10)

    await page.getByTestId('assets-filter-search').fill('Laptop')
    await expect.poll(() => cards(page).count(), { timeout: 15_000 }).toBeLessThanOrEqual(10)
    await expect.poll(() => page.locator('main').evaluate(el => el.scrollTop)).toBe(0)
  })

  test('restores the accumulated rows and the scroll position on back navigation', async ({ page }) => {
    await login(page)
    await page.goto(CATALOG)
    await expect(cards(page).first()).toBeVisible({ timeout: 15_000 })

    await page.locator('main').evaluate(el => el.scrollTo({ top: el.scrollHeight }))
    await expect.poll(() => cards(page).count(), { timeout: 15_000 }).toBeGreaterThan(10)

    const accumulated = await cards(page).count()

    const scrollTop = await page.locator('main').evaluate(el => el.scrollTop)
    expect(scrollTop).toBeGreaterThan(0)

    // Open the LAST card, not the first: Playwright scrolls a target into view
    // before clicking, and the first card sits at the top of the list — so
    // clicking it would scroll the container back to 0 and the test would be
    // measuring its own side effect. The last card is already on screen here,
    // so the click cannot move the container.
    await page.getByTestId('asset-card-open').last().click()
    await expect(page).toHaveURL(/\/assets\/[^/]+$/)

    await page.goBack()
    await expect(cards(page).first()).toBeVisible({ timeout: 15_000 })

    // Same rows, same place — not a reload from offset zero. It may be *more*
    // than what was accumulated: landing back near the bottom puts the sentinel
    // in view again, which legitimately pulls the following page.
    await expect
      .poll(() => cards(page).count(), { timeout: 10_000 })
      .toBeGreaterThanOrEqual(accumulated)
    await expect
      .poll(() => page.locator('main').evaluate(el => el.scrollTop), { timeout: 10_000 })
      .toBeGreaterThan(scrollTop / 2)
  })
})

test.describe('Compact layout — table path (Manajemen User)', () => {
  test('replaces page buttons with a sentinel and accumulates rows', async ({ page }) => {
    await login(page)
    await page.goto('/settings/users')
    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 15_000 })

    await expect(page.getByTestId('pagination-next')).toHaveCount(0)
    await expect(page.getByTestId('resource-table-infinite-sentinel')).toHaveCount(1)

    const before = await page.locator('tbody tr').count()
    await page.locator('main').evaluate(el => el.scrollTo({ top: el.scrollHeight }))
    await expect.poll(() => page.locator('tbody tr').count(), { timeout: 15_000 }).toBeGreaterThan(before)
  })
})
