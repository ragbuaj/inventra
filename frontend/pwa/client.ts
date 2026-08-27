/**
 * Opsi plugin klien modul PWA.
 *
 * `installPrompt` bukan sekadar nama kunci penyimpanan. Plugin klien modul hanya
 * memasang listener `beforeinstallprompt` bila opsi ini terisi — lihat
 * `@vite-pwa/nuxt/dist/runtime/plugins/pwa.client.js`, di sana `hideInstall`
 * langsung bernilai true saat opsinya kosong, sehingga `showInstallPrompt` tidak
 * pernah menyala dan `install()` jadi tanpa efek. Jadi tanpa baris ini tombol
 * pasang tidak akan pernah muncul di aplikasi sungguhan.
 *
 * Kuncinya dibaca lagi setiap halaman dimuat, dan itulah yang membuat penutupan
 * ajakan bertahan lintas pemuatan. `app/components/PwaInstallPrompt.vue` menulis
 * kunci yang sama saat pengguna menutup ajakan; kesamaannya dikunci
 * `test/unit/pwa-client.spec.ts`.
 *
 * Berkasnya berada di luar `app/` supaya bisa diimpor dari `nuxt.config.ts` tanpa
 * ikut terseret auto-import Nuxt — sama seperti `pwa/manifest.ts`.
 */
export const PWA_INSTALL_DISMISS_KEY = 'inventra.pwa.install-dismissed'

export const pwaClient = {
  installPrompt: PWA_INSTALL_DISMISS_KEY
}
