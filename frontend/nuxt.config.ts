// https://nuxt.com/docs/api/configuration/nuxt-config

// Optional filesystem polling for dev watchers (set NUXT_DEV_POLLING=true). Off by
// default — the Docker dev stack uses `docker compose watch`, which syncs files onto
// the container's native FS so inotify works without polling. Kept as a fallback for
// bind-mount setups where inotify events aren't delivered.
const devPolling = process.env.NUXT_DEV_POLLING === 'true'

export default defineNuxtConfig({
  modules: [
    '@nuxt/eslint',
    '@nuxt/ui',
    '@pinia/nuxt',
    '@nuxtjs/i18n'
  ],

  ssr: false,

  imports: {
    dirs: ['composables', 'composables/api']
  },

  devtools: {
    enabled: true
  },

  css: ['~/assets/css/main.css', 'leaflet/dist/leaflet.css'],

  runtimeConfig: {
    public: {
      // Override with NUXT_PUBLIC_API_BASE; see .env.example.
      apiBase: 'http://localhost:8080/api/v1'
    }
  },

  watchers: {
    chokidar: devPolling ? { usePolling: true, interval: 300 } : {}
  },

  compatibilityDate: '2025-01-15',

  vite: {
    server: {
      watch: devPolling ? { usePolling: true, interval: 300 } : undefined
    }
  },

  eslint: {
    config: {
      stylistic: {
        commaDangle: 'never',
        braceStyle: '1tbs'
      }
    }
  },

  i18n: {
    strategy: 'prefix_except_default',
    defaultLocale: 'id',
    // The module prepends 'i18n/' automatically — files live in i18n/locales/.
    langDir: 'locales',
    locales: [
      { code: 'id', name: 'Bahasa Indonesia', file: 'id.json' },
      { code: 'en', name: 'English', file: 'en.json' }
    ]
  }
})
