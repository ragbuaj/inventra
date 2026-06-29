import { test, expect } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// Peta Lokasi screen — real backend (GET /api/v1/offices/map)
// The seeded admin (admin@inventra.local) has masterdata.office.manage permission.
// On a fresh CI stack no offices have lat/lng coordinates seeded, so the map
// will show an empty-state. The assertions below are deterministic regardless
// of whether the backend has offices with coordinates.
// NOTE: pnpm test:e2e requires the full backend stack (see CLAUDE.md). This
// spec compiles + lints here; it runs in CI's e2e job.
// ---------------------------------------------------------------------------
test.describe('Peta Lokasi screen — real backend', () => {
  test('loads page heading, list panel, and map legend', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    // Page heading renders (proves the page mounted and auth resolved).
    // i18n key: map.title → "Peta Lokasi Kantor"
    await expect(page.getByRole('heading', { name: 'Peta Lokasi Kantor' })).toBeVisible({ timeout: 10_000 })

    // Left list panel: either office rows (stable data-testid="office-row" hook)
    // or the empty-state message — a single auto-waiting .or() assertion that is
    // deterministic regardless of whether offices with coordinates are seeded.
    // i18n key: map.emptyListTitle → "Tidak ada kantor"
    await expect(
      page.getByText('Tidak ada kantor', { exact: false })
        .or(page.locator('[data-testid="office-row"]').first())
    ).toBeVisible({ timeout: 10_000 })

    // Map panel header: the legend renders the tier category labels regardless of data.
    // TIER_ORDER renders Pusat/Wilayah/Cabang (3 tiers; Outlet folded into Cabang — approved deviation).
    // i18n key: map.tier.pusat → "Pusat"
    await expect(page.getByText('Pusat', { exact: false })).toBeVisible({ timeout: 5_000 })
  })

  test('filter bar renders search input and two select controls', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    // Heading proves mount + auth settled
    await expect(page.getByRole('heading', { name: 'Peta Lokasi Kantor' })).toBeVisible({ timeout: 10_000 })

    // Search input — i18n key: map.searchPlaceholder → "Cari kantor / kode…"
    await expect(page.getByPlaceholder('Cari kantor / kode…')).toBeVisible()

    // Two USelect filter controls: "Semua Jenis" and "Semua Provinsi"
    // USelect renders its current value as visible text in the trigger button.
    // i18n keys: map.jenisAll, map.provAll
    await expect(page.getByText('Semua Jenis', { exact: true })).toBeVisible()
    await expect(page.getByText('Semua Provinsi', { exact: true })).toBeVisible()
  })

  test('tier filter USelect opens and shows tier options', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    // Wait for page to settle
    await expect(page.getByRole('heading', { name: 'Peta Lokasi Kantor' })).toBeVisible({ timeout: 10_000 })

    // Open the tier (Jenis) USelect by clicking its trigger.
    // USelect trigger renders the current value as its text content.
    // Robust: click the trigger by its visible label text, not a CSS selector.
    await page.getByText('Semua Jenis', { exact: true }).click()

    // After opening, the option list renders inside a listbox; assert the scoped option.
    // Scoping to the open listbox ensures this only passes when the dropdown is genuinely open
    // (avoids the vacuous fallback where the legend's "Pusat" text satisfies the assertion).
    // i18n keys: map.tier.pusat → "Pusat", map.tier.wilayah → "Wilayah", map.tier.office → "Cabang"
    await expect(
      page.getByRole('listbox').getByRole('option', { name: 'Pusat' })
    ).toBeVisible({ timeout: 5_000 })
  })

  test('retry button is absent on successful load (no load error)', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    // Heading proves mount + auth settled (no crash = no error state)
    await expect(page.getByRole('heading', { name: 'Peta Lokasi Kantor' })).toBeVisible({ timeout: 10_000 })

    // On a successful load, the error state's retry button must NOT be visible.
    // data-testid="map-retry" on the retry UButton — guards against accidental error render.
    await expect(page.getByTestId('map-retry')).not.toBeVisible()
  })

  test('usage-note info bar renders below the heading', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    await expect(page.getByRole('heading', { name: 'Peta Lokasi Kantor' })).toBeVisible({ timeout: 10_000 })

    // i18n key: map.usageNote
    await expect(
      page.getByText('Provinsi & Kota dikelola di Referensi', { exact: false })
    ).toBeVisible()
  })
})
