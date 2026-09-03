/**
 * Tag head yang berhubungan dengan mode standalone.
 *
 * Semuanya wajib ada di shell HTML statis, bukan lewat `useHead` di `app.vue`:
 * aplikasi ini SPA (`ssr: false`), jadi tag yang dipasang saat hidrasi datang
 * terlambat untuk viewport maupun bilah status.
 *
 * Berkasnya berada di luar `app/` supaya bisa diimpor dari `nuxt.config.ts` tanpa
 * ikut terseret auto-import Nuxt — sama seperti `pwa/manifest.ts`.
 */
import { PWA_THEME_COLOR } from './manifest'

/**
 * `viewport-fit=cover` membuat halaman memenuhi layar sampai ke bawah notch dan
 * home indicator. Tanpanya iOS menyisakan pita hitam di sana dan seluruh
 * `env(safe-area-inset-*)` bernilai nol, sehingga penanganan area aman di
 * `main.css` tidak akan pernah berpengaruh.
 */
export const PWA_VIEWPORT = 'width=device-width, initial-scale=1, viewport-fit=cover'

/**
 * Warna bilah status saat perangkat memakai skema gelap: ujung gelap ramp merek
 * (`--color-brand-950`). Biru terang mode terang akan menyilaukan di sana dan
 * membuat ikon bilah status kehilangan kontras.
 */
export const PWA_THEME_COLOR_DARK = '#02194f'

export const pwaHeadMeta = [
  { name: 'theme-color', content: PWA_THEME_COLOR, media: '(prefers-color-scheme: light)' },
  { name: 'theme-color', content: PWA_THEME_COLOR_DARK, media: '(prefers-color-scheme: dark)' }
]
