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
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } }
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
