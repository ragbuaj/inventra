import { defineVitestConfig } from '@nuxt/test-utils/config'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const root = fileURLToPath(new URL('.', import.meta.url))

export default defineVitestConfig({
  test: {
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
