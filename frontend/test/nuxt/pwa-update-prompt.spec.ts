// @vitest-environment nuxt
// Tugas 6: ajakan perbarui saat service worker baru menunggu.
//
// Komponen ini tidak pernah memuat ulang halaman sendiri — itu keputusan yang dikunci
// spec (isian formulir aset yang sedang diketik tidak boleh hilang), jadi yang diuji
// paling keras di sini ada tiga: ia diam total sampai `needRefresh` benar, tombol muat
// ulang memanggil `updateServiceWorker` tepat sekali, dan penutupan bertahan walau
// service worker tetap menunggu.
//
// `$pwa` disuntik lewat `nuxtApp.provide`, yang memakai getter non-configurable —
// sekali disuntik tidak bisa diganti. Karena itu keadaan "modul PWA tidak tersedia
// sama sekali" diuji lebih dulu, sebelum suntikan pertama, dan sesudahnya objek yang
// sama dimutasi per tes (persis seperti aslinya: `reactive(...)` di plugin modul).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { reactive } from 'vue'
import PwaUpdatePrompt from '~/components/PwaUpdatePrompt.vue'

enableAutoUnmount(afterEach)

const updateServiceWorker = vi.fn()
const pwa = reactive({ needRefresh: false, updateServiceWorker })

let provided = false
function providePwa() {
  if (!provided) {
    useNuxtApp().provide('pwa', pwa)
    provided = true
  }
  pwa.needRefresh = false
}

function setLocale(code: 'id' | 'en') {
  const i18n = useNuxtApp().$i18n as unknown as { locale: { value: string } }
  i18n.locale.value = code
}

describe('PwaUpdatePrompt', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    if (provided) setLocale('id')
  })

  // Harus tetap tes pertama di berkas ini: ia mensyaratkan `$pwa` belum disuntik.
  it('tidak merender apa pun dan tidak melempar error saat $pwa tidak tersedia', async () => {
    expect(useNuxtApp().$pwa).toBeUndefined()

    const w = await mountSuspended(PwaUpdatePrompt)

    expect(w.find('[data-testid="pwa-update-prompt"]').exists()).toBe(false)
    expect(w.text()).toBe('')
  })

  it('tidak merender apa pun selama tidak ada service worker yang menunggu', async () => {
    providePwa()

    const w = await mountSuspended(PwaUpdatePrompt)

    expect(w.find('[data-testid="pwa-update-prompt"]').exists()).toBe(false)
  })

  it('merender ajakan dalam bahasa Indonesia saat service worker baru menunggu', async () => {
    providePwa()
    const w = await mountSuspended(PwaUpdatePrompt)

    pwa.needRefresh = true
    await w.vm.$nextTick()

    const prompt = w.find('[data-testid="pwa-update-prompt"]')
    expect(prompt.exists()).toBe(true)
    const text = prompt.text().replace(/\s+/g, ' ')
    expect(text).toContain('Versi baru Inventra sudah tersedia.')
    expect(text).toContain('Muat ulang')
    expect(text).toContain('Nanti')
  })

  it('merender ajakan dalam bahasa Inggris saat locale en aktif', async () => {
    providePwa()
    setLocale('en')
    const w = await mountSuspended(PwaUpdatePrompt)

    pwa.needRefresh = true
    await w.vm.$nextTick()

    const text = w.find('[data-testid="pwa-update-prompt"]').text().replace(/\s+/g, ' ')
    expect(text).toContain('A new version of Inventra is available.')
    expect(text).toContain('Reload')
    expect(text).toContain('Later')
  })

  it('memanggil updateServiceWorker tepat sekali dengan muat ulang halaman saat tombol muat ulang ditekan', async () => {
    providePwa()
    const w = await mountSuspended(PwaUpdatePrompt)
    pwa.needRefresh = true
    await w.vm.$nextTick()

    await w.find('[data-testid="pwa-update-reload"]').trigger('click')

    expect(updateServiceWorker).toHaveBeenCalledTimes(1)
    expect(updateServiceWorker).toHaveBeenCalledWith(true)
  })

  it('menyembunyikan ajakan tanpa memanggil updateServiceWorker saat tombol nanti ditekan', async () => {
    providePwa()
    const w = await mountSuspended(PwaUpdatePrompt)
    pwa.needRefresh = true
    await w.vm.$nextTick()

    await w.find('[data-testid="pwa-update-later"]').trigger('click')

    expect(w.find('[data-testid="pwa-update-prompt"]').exists()).toBe(false)
    expect(updateServiceWorker).not.toHaveBeenCalled()
  })

  it('tidak memunculkan ajakan lagi di sesi yang sama setelah ditutup, walau service worker tetap menunggu', async () => {
    providePwa()
    const w = await mountSuspended(PwaUpdatePrompt)
    pwa.needRefresh = true
    await w.vm.$nextTick()
    await w.find('[data-testid="pwa-update-later"]').trigger('click')

    pwa.needRefresh = false
    await w.vm.$nextTick()
    pwa.needRefresh = true
    await w.vm.$nextTick()

    expect(w.find('[data-testid="pwa-update-prompt"]').exists()).toBe(false)
  })
})
