// @vitest-environment nuxt
// Tugas 8: shell benar-benar memakai kelas area aman saat dirender.
//
// Tes unit pendampingnya (test/unit/pwa-standalone.spec.ts) mengunci aturan CSS-nya;
// di sini yang dibuktikan adalah kelasnya sungguh sampai ke elemen yang dirender —
// pada shell, dan pada laci mobile yang diposisikan `fixed` sehingga tidak ikut
// terdorong padding shell.
import { describe, it, expect, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import DefaultLayout from '~/layouts/default.vue'
import AppSidebar from '~/components/AppSidebar.vue'

enableAutoUnmount(afterEach)

describe('shell mode standalone', () => {
  it('memberi kelas area aman pada wadah terluar shell', async () => {
    const w = await mountSuspended(DefaultLayout)

    const root = w.find('div')
    expect(root.classes()).toContain('app-safe-area')
    // Wadah yang sama tetap memenuhi tinggi layar; padding area aman bekerja dari
    // dalam kotaknya, bukan dengan mengubah tata letak.
    expect(root.classes()).toContain('h-screen')
  })

  it('memberi kelas area aman tersendiri pada laci navigasi', async () => {
    const w = await mountSuspended(AppSidebar)

    const aside = w.find('aside')
    expect(aside.classes()).toContain('app-safe-drawer')
  })

  it('mempertahankan laci off-canvas yang sudah ada sebagai jalan ke navigasi utama', async () => {
    const w = await mountSuspended(AppSidebar)

    const aside = w.find('aside')
    expect(aside.classes()).toEqual(expect.arrayContaining(['fixed', 'inset-y-0', 'z-50']))
    expect(aside.classes()).toContain('lg:static')
  })
})
