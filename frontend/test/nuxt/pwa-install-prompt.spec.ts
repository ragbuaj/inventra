// @vitest-environment nuxt
// Tugas 7: ajakan pasang — tombol pasang di peramban yang punya API-nya, petunjuk
// manual di Safari iOS yang tidak punya.
//
// Yang paling mudah salah dan karena itu diuji paling keras: ajakan ini tidak
// boleh muncul di layar masuk (mengganggu alur login), tidak boleh muncul lagi
// setelah ditutup walau halaman dimuat ulang (penutupan disimpan di
// localStorage), dan tidak boleh muncul sama sekali begitu aplikasi berjalan
// terpasang.
//
// Deteksi platform dimock di tingkat auto-import supaya keenam keadaannya bisa
// dipentaskan tanpa memalsukan user agent; kebenaran deteksinya sendiri diuji
// terpisah di test/unit/pwa-platform.spec.ts.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { reactive } from 'vue'
import PwaInstallPrompt from '~/components/PwaInstallPrompt.vue'

const { platform, route } = vi.hoisted(() => ({
  platform: { ios: false, standalone: false },
  route: { meta: { layout: 'default' } as Record<string, unknown> }
}))

mockNuxtImport('isIosSafari', () => () => platform.ios)
mockNuxtImport('isStandaloneDisplay', () => () => platform.standalone)
mockNuxtImport('useRoute', () => () => route)

enableAutoUnmount(afterEach)

const DISMISS_KEY = 'inventra.pwa.install-dismissed'

const install = vi.fn()
const cancelInstall = vi.fn()
const pwa = reactive({
  showInstallPrompt: false,
  isPWAInstalled: false,
  needRefresh: false,
  install,
  cancelInstall
})

let provided = false
function providePwa() {
  if (!provided) {
    useNuxtApp().provide('pwa', pwa)
    provided = true
  }
}

function setLocale(code: 'id' | 'en') {
  const i18n = useNuxtApp().$i18n as unknown as { locale: { value: string } }
  i18n.locale.value = code
}

/** Peramban Android yang sudah memicu beforeinstallprompt. */
function androidWithPrompt() {
  platform.ios = false
  platform.standalone = false
  pwa.showInstallPrompt = true
  pwa.isPWAInstalled = false
}

describe('PwaInstallPrompt', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    platform.ios = false
    platform.standalone = false
    route.meta = { layout: 'default' }
    pwa.showInstallPrompt = false
    pwa.isPWAInstalled = false
    pwa.needRefresh = false
    providePwa()
  })

  afterEach(() => {
    setLocale('id')
  })

  it('menampilkan tombol pasang di Android saat prompt bawaan tersedia', async () => {
    androidWithPrompt()

    const w = await mountSuspended(PwaInstallPrompt)

    const text = w.find('[data-testid="pwa-install-prompt"]').text().replace(/\s+/g, ' ')
    expect(text).toContain('Pasang Inventra di perangkat ini untuk akses lebih cepat.')
    expect(w.find('[data-testid="pwa-install-action"]').exists()).toBe(true)
    expect(w.find('[data-testid="pwa-install-ios-hint"]').exists()).toBe(false)
  })

  it('memanggil dialog pasang bawaan tepat sekali saat tombol pasang ditekan', async () => {
    androidWithPrompt()
    const w = await mountSuspended(PwaInstallPrompt)

    await w.find('[data-testid="pwa-install-action"]').trigger('click')

    expect(install).toHaveBeenCalledTimes(1)
  })

  it('tidak merender apa pun setelah aplikasi terpasang', async () => {
    androidWithPrompt()
    const w = await mountSuspended(PwaInstallPrompt)
    expect(w.find('[data-testid="pwa-install-prompt"]').exists()).toBe(true)

    // Yang terjadi sungguhan setelah pemasangan: prompt bawaan hilang dan
    // display-mode berubah jadi standalone.
    pwa.showInstallPrompt = false
    pwa.isPWAInstalled = true
    await w.vm.$nextTick()

    expect(w.find('[data-testid="pwa-install-prompt"]').exists()).toBe(false)
  })

  it('menampilkan petunjuk manual tanpa tombol pasang di Safari iOS yang belum standalone', async () => {
    platform.ios = true

    const w = await mountSuspended(PwaInstallPrompt)

    const hint = w.find('[data-testid="pwa-install-ios-hint"]')
    expect(hint.exists()).toBe(true)
    expect(hint.text().replace(/\s+/g, ' ')).toContain('Buka menu Bagikan di Safari, lalu pilih Tambahkan ke Layar Utama.')
    expect(w.find('[data-testid="pwa-install-action"]').exists()).toBe(false)
  })

  it('tidak merender apa pun di Safari iOS yang sudah berjalan standalone', async () => {
    platform.ios = true
    platform.standalone = true

    const w = await mountSuspended(PwaInstallPrompt)

    expect(w.find('[data-testid="pwa-install-prompt"]').exists()).toBe(false)
  })

  it('tidak merender apa pun di desktop yang tidak memicu beforeinstallprompt', async () => {
    const w = await mountSuspended(PwaInstallPrompt)

    expect(w.find('[data-testid="pwa-install-prompt"]').exists()).toBe(false)
  })

  it('menyimpan penutupan dan menyembunyikan ajakan saat tombol nanti ditekan', async () => {
    androidWithPrompt()
    const w = await mountSuspended(PwaInstallPrompt)

    await w.find('[data-testid="pwa-install-later"]').trigger('click')

    expect(w.find('[data-testid="pwa-install-prompt"]').exists()).toBe(false)
    expect(localStorage.getItem(DISMISS_KEY)).toBe('true')
    expect(cancelInstall).toHaveBeenCalledTimes(1)
    expect(install).not.toHaveBeenCalled()
  })

  it('tetap tersembunyi di pemuatan halaman berikutnya setelah pernah ditutup', async () => {
    androidWithPrompt()
    localStorage.setItem(DISMISS_KEY, 'true')

    const w = await mountSuspended(PwaInstallPrompt)

    expect(w.find('[data-testid="pwa-install-prompt"]').exists()).toBe(false)
  })

  it('tidak merender apa pun di layar masuk supaya alur login tidak terganggu', async () => {
    androidWithPrompt()
    route.meta = { layout: 'auth' }

    const w = await mountSuspended(PwaInstallPrompt)

    expect(w.find('[data-testid="pwa-install-prompt"]').exists()).toBe(false)
  })

  it('merender ajakan dalam bahasa Inggris saat locale en aktif', async () => {
    androidWithPrompt()
    setLocale('en')

    const w = await mountSuspended(PwaInstallPrompt)

    const text = w.find('[data-testid="pwa-install-prompt"]').text().replace(/\s+/g, ' ')
    expect(text).toContain('Install Inventra on this device for faster access.')
    expect(text).toContain('Install')
    expect(text).toContain('Later')
  })
})
