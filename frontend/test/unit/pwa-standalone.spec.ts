// Tugas 8: poles mode standalone.
//
// Tiga hal yang hanya terlihat di ponsel terpasang dan tidak akan pernah
// ketahuan dari desktop: viewport yang tidak memakai `viewport-fit=cover` membuat
// `env(safe-area-inset-*)` selalu nol sehingga penanganan notch jadi sia-sia,
// aturan safe-area yang hilang membuat konten tertutup notch dan home indicator,
// dan latar `body` yang tidak ikut tema membuat pita putih muncul di area aman
// saat mode gelap. Ketiganya dikunci di sini terhadap berkas nyata.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { PWA_VIEWPORT, PWA_THEME_COLOR_DARK, pwaHeadMeta } from '../../pwa/head'
import { PWA_THEME_COLOR } from '../../pwa/manifest'

const frontendRoot = fileURLToPath(new URL('../../', import.meta.url))
const mainCss = readFileSync(resolve(frontendRoot, 'app/assets/css/main.css'), 'utf8')
const nuxtConfig = readFileSync(resolve(frontendRoot, 'nuxt.config.ts'), 'utf8')
const defaultLayout = readFileSync(resolve(frontendRoot, 'app/layouts/default.vue'), 'utf8')
const sidebar = readFileSync(resolve(frontendRoot, 'app/components/AppSidebar.vue'), 'utf8')

function brandStepFromCss(step: number): string {
  const marker = `--color-brand-${step}:`
  const at = mainCss.indexOf(marker)
  if (at === -1) return ''
  return mainCss.slice(at + marker.length, mainCss.indexOf(';', at)).trim().toLowerCase()
}

/** Isi satu blok aturan CSS, dicari dari selektornya. */
function cssRule(selector: string): string {
  const start = mainCss.indexOf(`${selector} {`)
  if (start === -1) return ''
  const open = mainCss.indexOf('{', start)
  return mainCss.slice(open + 1, mainCss.indexOf('}', open))
}

describe('viewport standalone', () => {
  it('memakai viewport-fit=cover, tanpanya env(safe-area-inset-*) selalu nol', () => {
    expect(PWA_VIEWPORT).toContain('viewport-fit=cover')
  })

  it('mempertahankan viewport dasar yang sudah dipakai aplikasi', () => {
    expect(PWA_VIEWPORT).toContain('width=device-width')
    expect(PWA_VIEWPORT).toContain('initial-scale=1')
  })

  it('benar-benar terpasang di nuxt.config, bukan cuma diekspor', () => {
    expect(nuxtConfig).toMatch(/viewport:\s*PWA_VIEWPORT/)
    expect(nuxtConfig).toMatch(/meta:\s*pwaHeadMeta/)
  })
})

describe('warna bilah status', () => {
  it('menerbitkan satu theme-color per skema warna', () => {
    const themeColors = pwaHeadMeta.filter(m => m.name === 'theme-color')
    expect(themeColors).toHaveLength(2)
    expect(themeColors.map(m => m.media)).toEqual([
      '(prefers-color-scheme: light)',
      '(prefers-color-scheme: dark)'
    ])
  })

  it('memakai biru merek di mode terang', () => {
    const light = pwaHeadMeta.find(m => m.media === '(prefers-color-scheme: light)')
    expect(light?.content.toLowerCase()).toBe(PWA_THEME_COLOR.toLowerCase())
    expect(light?.content.toLowerCase()).toBe(brandStepFromCss(500))
  })

  it('memakai ujung gelap ramp merek di mode gelap, bukan biru terang yang menyilaukan', () => {
    // Menjaga ekstraksinya sendiri: main.css yang berubah struktur harus gagal di
    // sini, bukan diam-diam membandingkan '' dengan ''.
    expect(brandStepFromCss(950)).toMatch(/^#[0-9a-f]{6}$/)
    const dark = pwaHeadMeta.find(m => m.media === '(prefers-color-scheme: dark)')
    expect(dark?.content.toLowerCase()).toBe(PWA_THEME_COLOR_DARK.toLowerCase())
    expect(PWA_THEME_COLOR_DARK.toLowerCase()).toBe(brandStepFromCss(950))
  })
})

describe('area aman shell', () => {
  it('memberi jarak keempat sisi dari inset perangkat', () => {
    const rule = cssRule('.app-safe-area')
    for (const side of ['top', 'bottom', 'left', 'right']) {
      expect(rule).toContain(`env(safe-area-inset-${side}`)
    }
  })

  it('memakai nol sebagai nilai cadangan supaya peramban lama tidak kehilangan padding', () => {
    const rule = cssRule('.app-safe-area')
    expect(rule.match(/env\(safe-area-inset-[a-z]+,\s*0px\)/g)).toHaveLength(4)
  })

  it('dipakai shell aplikasi, bukan cuma didefinisikan', () => {
    expect(defaultLayout).toContain('app-safe-area')
  })

  it('menjaga laci mobile yang fixed, yang tidak ikut terdorong padding shell', () => {
    expect(sidebar).toContain('app-safe-drawer')
    const rule = cssRule('.app-safe-drawer')
    expect(rule).toContain('env(safe-area-inset-top')
    expect(rule).toContain('env(safe-area-inset-bottom')
  })

  it('mematikan padding laci di desktop, tempat laci berada di dalam shell yang sudah ter-padding', () => {
    // Blok media harus menyebut kelasnya lagi; cukup pastikan resetnya ada.
    expect(mainCss).toMatch(/@media[^{]*\{[^@]*\.app-safe-drawer\s*\{\s*padding:\s*0/)
  })
})

describe('latar dokumen', () => {
  it('mengikat html dan body ke latar tema supaya area aman tidak jadi pita putih di mode gelap', () => {
    expect(mainCss).toMatch(/html,\s*body\s*\{[^}]*background-color:\s*var\(--ui-bg\)/)
  })
})
