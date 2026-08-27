// https://nuxt.com/docs/api/configuration/nuxt-config
import {
  pwaManifest,
  PWA_APPLE_TOUCH_ICON,
  PWA_MANIFEST_HREF,
  PWA_THEME_COLOR
} from './pwa/manifest'

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
    '@nuxtjs/i18n',
    '@vite-pwa/nuxt'
  ],

  ssr: false,

  imports: {
    dirs: ['composables', 'composables/api']
  },

  devtools: {
    enabled: true
  },

  // Tag PWA harus ada di shell HTML statis, bukan lewat useHead di app.vue: aplikasi ini
  // SPA, jadi useHead baru berlaku setelah hidrasi — terlambat untuk kriteria pemasangan
  // dan untuk Safari iOS, yang membaca apple-touch-icon saat halaman dimuat. Ditulis
  // tangan alih-alih memakai komponen bawaan modul (`NuxtPwaAssets`), yang bergantung pada
  // virtual module dari integrasi assets-generator yang sengaja tidak dipasang.
  app: {
    head: {
      meta: [
        { name: 'theme-color', content: PWA_THEME_COLOR }
      ],
      link: [
        { rel: 'manifest', href: PWA_MANIFEST_HREF },
        { rel: 'apple-touch-icon', sizes: '180x180', href: PWA_APPLE_TOUCH_ICON }
      ]
    }
  },

  css: ['~/assets/css/main.css', 'leaflet/dist/leaflet.css'],

  runtimeConfig: {
    public: {
      // Override with NUXT_PUBLIC_API_BASE; see .env.example.
      apiBase: 'http://localhost:8080/api/v1'
    }
  },

  // Cache policy for the SPA. Hashed build assets are content-addressed, so they
  // are safe to cache forever; the HTML shell must always be revalidated or a
  // browser keeps serving a stale shell that references the previous deploy's
  // (now-404) chunk URLs — the "still broken until I hard-refresh" trap.
  routeRules: {
    '/_nuxt/**': { headers: { 'cache-control': 'public, max-age=31536000, immutable' } },
    '/**': { headers: { 'cache-control': 'no-cache' } }
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
  },

  // PWA. Isi manifest tinggal di pwa/manifest.ts supaya app.vue memakai konstanta yang
  // sama dan tes unit bisa menguncinya. Strategi precache menyusul di tugas berikutnya.
  pwa: {
    registerType: 'prompt',
    manifest: pwaManifest,
    // Service worker sengaja mati saat `pnpm dev` — pengujiannya lewat `pnpm preview`.
    devOptions: {
      enabled: false
    }
  }
})
