/**
 * Satu-satunya sumber strategi caching service worker.
 *
 * Diimpor `nuxt.config.ts` (yang menyerahkannya ke @vite-pwa/nuxt) dan dikunci
 * `test/unit/pwa-workbox.spec.ts`. Berkasnya berada di luar `app/` supaya bisa
 * diimpor dari `nuxt.config.ts` tanpa ikut terseret auto-import Nuxt.
 *
 * Aturan paling keras di berkas ini: **tidak ada satu pun aturan runtime caching**.
 * Di produksi API se-origin dengan frontend, jadi satu aturan runtime saja sudah
 * cukup untuk mengendapkan respons API ke Cache Storage di tiap perangkat — dan
 * kesalahan itu tidak akan pernah terlihat saat dev, karena di dev API beda origin.
 * Service worker ini hanya menyajikan shell dan aset build; segala hal lain lewat
 * jaringan apa adanya.
 */

/**
 * Rute yang di-prerender jadi HTML statis (`nitro.prerender.routes`) dan dipakai
 * sebagai jawaban untuk navigasi apa pun saat luring. Karena `ssr: false`, HTML-nya
 * kerangka kosong tanpa data pengguna, dan locale `/en/` diselesaikan di klien dari
 * URL — satu shell melayani kedua locale.
 */
export const PWA_SHELL_ROUTE = '/'

/**
 * Navigasi yang tidak boleh dijawab shell. Diuji Workbox terhadap
 * `url.pathname + url.search` (workbox-routing `NavigationRoute._match`), sehingga
 * pola `/health` harus ikut memperhitungkan query string.
 *
 * Keduanya diikat ke awal path supaya rute aplikasi yang kebetulan memuat kata yang
 * sama (`/laporan/api-usage`, `/healthcheck-report`) tidak ikut tertahan.
 */
export const PWA_NAVIGATE_FALLBACK_DENYLIST = [
  /^\/api\//,
  /^\/health(?:[/?]|$)/
]

export const pwaWorkbox = {
  // Semua ekstensi yang benar-benar terbit ke `.output/public/`: bundel dan CSS,
  // shell hasil prerender, font, ikon, serta metadata build `_nuxt/builds/*.json`
  // yang dibaca Nuxt saat memeriksa versi aplikasi.
  globPatterns: ['**/*.{html,js,css,woff2,png,svg,ico,webmanifest,json}'],
  navigateFallback: PWA_SHELL_ROUTE,
  navigateFallbackDenylist: PWA_NAVIGATE_FALLBACK_DENYLIST,
  // Workbox membuang berkas yang melewati batas ini dari precache tanpa bersuara.
  // Bundel terbesar hari ini ratusan KB; batasnya dinaikkan dari default 2 MiB
  // supaya pertumbuhan wajar tidak diam-diam melubangi precache.
  maximumFileSizeToCacheInBytes: 6 * 1024 * 1024,
  cleanupOutdatedCaches: true
  // Tanpa `skipWaiting` dan `clientsClaim`: `registerType: 'prompt'` menyerahkan
  // saat pengaktifan versi baru ke pengguna, supaya isian formulir aset yang
  // sedang diketik tidak terbuang oleh muat ulang mendadak.
}
