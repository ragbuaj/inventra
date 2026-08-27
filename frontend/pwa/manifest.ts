/**
 * Satu-satunya sumber isi web app manifest.
 *
 * Diimpor `nuxt.config.ts` (yang menyerahkannya ke @vite-pwa/nuxt) dan `app/app.vue`
 * (untuk tag head), lalu dikunci `test/unit/pwa-manifest.spec.ts` — tes itu membaca
 * berkas nyata di `public/` dan ramp brand di `app/assets/css/main.css`, sehingga
 * daftar ikon maupun warnanya tidak bisa menyimpang diam-diam.
 *
 * Berkasnya berada di luar `app/` supaya bisa diimpor dari `nuxt.config.ts` tanpa
 * ikut terseret auto-import Nuxt.
 */

/** Warna brand, step 500 dari ramp di app/assets/css/main.css. */
export const PWA_THEME_COLOR = '#005bfd'

/** Nama berkas manifest yang diterbitkan build; dipakai tag `<link rel="manifest">`. */
export const PWA_MANIFEST_HREF = '/manifest.webmanifest'

/** iOS mengabaikan `icons` di manifest dan hanya membaca tag ini. */
export const PWA_APPLE_TOUCH_ICON = '/apple-touch-icon-180x180.png'

export const pwaManifest = {
  id: '/',
  name: 'Inventra - Manajemen Aset',
  short_name: 'Inventra',
  description: 'Manajemen aset tetap dan inventaris',
  lang: 'id',
  // `scope` dan `start_url` di akar: strategi i18n `prefix_except_default` menaruh
  // rute Inggris di /en/*, dan keduanya harus tetap di dalam jendela terpasang.
  start_url: '/',
  scope: '/',
  display: 'standalone' as const,
  theme_color: PWA_THEME_COLOR,
  background_color: '#ffffff',
  icons: [
    { src: '/pwa-64x64.png', sizes: '64x64', type: 'image/png' },
    { src: '/pwa-192x192.png', sizes: '192x192', type: 'image/png' },
    { src: '/pwa-512x512.png', sizes: '512x512', type: 'image/png' },
    {
      src: '/maskable-icon-512x512.png',
      sizes: '512x512',
      type: 'image/png',
      purpose: 'maskable' as const
    }
  ]
}
