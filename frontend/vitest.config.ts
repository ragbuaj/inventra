import { defineVitestConfig } from '@nuxt/test-utils/config'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const root = fileURLToPath(new URL('.', import.meta.url))

export default defineVitestConfig({
  test: {
    // Vitest owns `test/`; Playwright owns `e2e/` (its *.spec.ts must not be
    // collected here — it imports from @playwright/test, not vitest).
    include: ['test/**/*.{spec,test}.ts'],
    // Drains reka-ui FocusScope's post-unmount focus-restore timer after each
    // test so it can't fire post-teardown and fail the run — see the file header.
    setupFiles: ['./test/setup/flush-focus-timers.ts'],
    environment: 'node',
    environmentOptions: {},
    hookTimeout: 60000
  },
  resolve: {
    alias: {
      '~': resolve(root, 'app'),
      '@': resolve(root, 'app')
    }
  }
})
