// Tugas 7: deteksi platform untuk ajakan pasang.
//
// Dua keputusannya bergantung penuh pada fungsi ini: Safari iOS tidak pernah
// memicu `beforeinstallprompt`, jadi hanya di sana petunjuk manual boleh muncul —
// dan tidak boleh muncul lagi begitu aplikasi berjalan standalone. Keduanya murni
// perhitungan string dan media query, jadi diuji di sini alih-alih lewat komponen.
import { describe, it, expect } from 'vitest'
import { isIosSafari, isStandaloneDisplay } from '~/utils/pwaPlatform'

const UA = {
  iphoneSafari: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1',
  ipadSafari: 'Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1',
  iphoneChrome: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.6478.54 Mobile/15E148 Safari/604.1',
  iphoneFirefox: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/127.0 Mobile/15E148 Safari/605.1.15',
  iphoneEdge: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/126.0 Mobile/15E148 Safari/605.1.15',
  androidChrome: 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36',
  desktopChrome: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36',
  macSafari: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15'
}

describe('isIosSafari', () => {
  it('mengenali Safari di iPhone', () => {
    expect(isIosSafari(UA.iphoneSafari, 5)).toBe(true)
  })

  it('mengenali Safari di iPad', () => {
    expect(isIosSafari(UA.ipadSafari, 5)).toBe(true)
  })

  it('mengenali iPadOS 13+ yang menyamar sebagai Mac dengan layar sentuh', () => {
    expect(isIosSafari(UA.macSafari, 5)).toBe(true)
  })

  it('menolak Mac sungguhan tanpa layar sentuh', () => {
    expect(isIosSafari(UA.macSafari, 0)).toBe(false)
  })

  it.each([
    ['Chrome iOS', UA.iphoneChrome],
    ['Firefox iOS', UA.iphoneFirefox],
    ['Edge iOS', UA.iphoneEdge]
  ])('menolak %s karena Tambahkan ke Layar Utama hanya ada di Safari', (_name, ua) => {
    expect(isIosSafari(ua, 5)).toBe(false)
  })

  it.each([
    ['Chrome Android', UA.androidChrome],
    ['Chrome desktop', UA.desktopChrome]
  ])('menolak %s karena peramban itu memicu beforeinstallprompt sendiri', (_name, ua) => {
    expect(isIosSafari(ua, 0)).toBe(false)
  })

  it('menolak user agent kosong', () => {
    expect(isIosSafari('', 0)).toBe(false)
  })
})

function fakeWindow(displayMode: boolean, iosStandalone?: boolean) {
  return {
    matchMedia: (query: string) => ({ matches: query.includes('standalone') && displayMode }),
    navigator: iosStandalone === undefined ? {} : { standalone: iosStandalone }
  }
}

describe('isStandaloneDisplay', () => {
  it('benar saat display-mode standalone cocok', () => {
    expect(isStandaloneDisplay(fakeWindow(true))).toBe(true)
  })

  it('benar saat navigator.standalone milik Safari iOS bernilai true', () => {
    expect(isStandaloneDisplay(fakeWindow(false, true))).toBe(true)
  })

  it('salah saat berjalan di tab peramban biasa', () => {
    expect(isStandaloneDisplay(fakeWindow(false, false))).toBe(false)
  })

  it('salah saat tidak ada satu pun penanda standalone', () => {
    expect(isStandaloneDisplay(fakeWindow(false))).toBe(false)
  })
})
