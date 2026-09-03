import { defineConfig, devices } from '@playwright/test'

// Base URL of the running frontend under test. CI builds + previews on :3000.
const baseURL = process.env.E2E_BASE_URL || 'http://localhost:3000'

// Set E2E_NO_SERVER=1 to test against an already-running app (CI brings up its
// own server + the backend stack via docker compose — see .github/workflows/ci.yml).
const manageServer = !process.env.E2E_NO_SERVER

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure'
  },
  // Three projects split by file so CI can run them against DIFFERENT DB states.
  // CI applies the demo seed between phase one and phases two/three; locally
  // `playwright test` runs all three.
  //
  // `chromium`   — clean migrated DB; each spec builds its own fixtures.
  // `lampiran`   — demo seed (kantor/role per tier + aset): maker-checker suite.
  // `seeded-ui`  — demo seed as well, for UI specs that need a data-RICH list
  //                rather than fixtures they can build themselves. The compact
  //                layout only exists once a list runs past one page, and assets
  //                cannot be created directly (no POST /assets — every asset goes
  //                through maker-checker), so building 25 of them per spec is not
  //                practical. These specs read the seeded data instead.
  projects: [
    {
      name: 'chromium',
      testIgnore: [/lampiran-a-.*\.spec\.ts/, /mobile-table-ux\.spec\.ts/],
      use: { ...devices['Desktop Chrome'] }
    },
    {
      name: 'lampiran',
      testMatch: /lampiran-a-.*\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] }
    },
    {
      name: 'seeded-ui',
      testMatch: /mobile-table-ux\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] }
    }
  ],
  webServer: manageServer
    ? {
        command: 'pnpm preview',
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000
      }
    : undefined
})
