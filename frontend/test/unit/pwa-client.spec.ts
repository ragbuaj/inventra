// Tugas 7: konfigurasi plugin klien modul PWA.
//
// Berkas ini ada karena satu jebakan yang tidak bisa ditangkap tes komponen: tes
// runtime memalsukan `$pwa`, jadi ia akan tetap hijau walau di aplikasi sungguhan
// tombol pasang tidak pernah bisa muncul. Plugin modul hanya mendaftarkan listener
// `beforeinstallprompt` bila opsi `installPrompt` diisi, dan kunci itu juga yang
// membuat penutupan ajakan bertahan lintas pemuatan halaman. Kesamaan kunci antara
// konfigurasi dan komponen tidak lagi dijaga di sini: komponen MENGIMPOR konstanta
// yang sama, jadi divergensinya tidak mungkin terjadi, dan tes penutupan di
// test/nuxt/pwa-install-prompt.spec.ts yang membuktikannya lewat perilaku nyata.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { pwaClient, PWA_INSTALL_DISMISS_KEY } from '../../pwa/client'

const frontendRoot = fileURLToPath(new URL('../../', import.meta.url))
const nuxtConfig = readFileSync(resolve(frontendRoot, 'nuxt.config.ts'), 'utf8')

describe('pwa client options', () => {
  it('mengisi installPrompt, tanpanya modul tidak pernah menangkap beforeinstallprompt', () => {
    expect(pwaClient.installPrompt).toBe(PWA_INSTALL_DISMISS_KEY)
    expect(PWA_INSTALL_DISMISS_KEY).toBe('inventra.pwa.install-dismissed')
  })

  it('benar-benar terpasang di nuxt.config, bukan cuma diekspor', () => {
    expect(nuxtConfig).toContain('pwaClient')
    expect(nuxtConfig).toMatch(/client:\s*pwaClient/)
  })
})
