/**
 * Deteksi platform untuk ajakan pasang PWA.
 *
 * Safari iOS adalah satu-satunya peramban arus utama yang tidak memicu
 * `beforeinstallprompt` dan tidak menyediakan API pasang sama sekali — di sana
 * pemasangan hanya bisa lewat Bagikan lalu Tambahkan ke Layar Utama, jadi yang
 * bisa diberikan aplikasi hanyalah petunjuk manual. Peramban lain di iOS (Chrome,
 * Firefox, Edge) memakai mesin WebKit yang sama tapi tidak punya menu itu, jadi
 * memberi mereka petunjuk yang sama justru menyesatkan.
 */

/** Peramban di iOS yang bukan Safari; semuanya tetap membawa kata "Safari" di UA-nya. */
const NON_SAFARI_IOS = /CriOS|FxiOS|EdgiOS|OPiOS|Chrome/

/**
 * Benar hanya untuk Safari di iPhone, iPad, atau iPod.
 *
 * `maxTouchPoints` ikut diperiksa karena iPadOS 13 ke atas mengirim user agent
 * desktop ("Macintosh") secara default; layar sentuh adalah pembeda satu-satunya
 * dari Mac sungguhan.
 */
export function isIosSafari(userAgent: string, maxTouchPoints: number): boolean {
  if (!userAgent || NON_SAFARI_IOS.test(userAgent)) return false
  if (/iPhone|iPad|iPod/.test(userAgent)) return true
  return /Macintosh/.test(userAgent) && maxTouchPoints > 1
}

/**
 * Bentuk `window` seminimal yang dibutuhkan pemeriksaan standalone.
 *
 * `navigator` sengaja `unknown`: `navigator.standalone` tidak ada di tipe DOM
 * standar (ia milik Safari saja), jadi mendeklarasikannya sebagai objek beropsi
 * membuat `window` sungguhan tidak lagi bisa dioper ke sini.
 */
export interface StandaloneWindow {
  matchMedia: (query: string) => { matches: boolean }
  navigator: unknown
}

/**
 * Benar saat aplikasi berjalan sebagai aplikasi terpasang, bukan di tab peramban.
 *
 * Dua penanda karena Safari iOS tidak mengimplementasikan `display-mode` sampai
 * versi lama masih beredar: ia menandainya lewat `navigator.standalone`.
 */
export function isStandaloneDisplay(win: StandaloneWindow): boolean {
  if (win.matchMedia('(display-mode: standalone)').matches) return true
  return (win.navigator as { standalone?: boolean }).standalone === true
}
