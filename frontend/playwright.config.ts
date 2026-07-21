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
  // Two projects split by file so CI can run them against DIFFERENT DB states:
  // `chromium` (existing specs) runs against a clean migrated DB (each spec builds
  // its own fixtures), while `lampiran` (the Lampiran A maker-checker suite) runs
  // against the demo seed (kantor/role per tier + aset). CI runs them in two phases
  // with the seed applied in between; locally `playwright test` runs both.
  projects: [
    {
      name: 'chromium',
      testIgnore: /lampiran-a-.*\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] }
    },
    {
      name: 'lampiran',
      testMatch: /lampiran-a-.*\.spec\.ts/,
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
