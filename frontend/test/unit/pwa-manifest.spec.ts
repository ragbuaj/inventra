import { describe, it, expect } from 'vitest'
import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import {
  pwaManifest,
  PWA_THEME_COLOR,
  PWA_MANIFEST_HREF,
  PWA_APPLE_TOUCH_ICON
} from '../../pwa/manifest'

const frontendRoot = fileURLToPath(new URL('../../', import.meta.url))
const publicDir = resolve(frontendRoot, 'public')
const mainCss = readFileSync(resolve(frontendRoot, 'app/assets/css/main.css'), 'utf8')

/** The brand ramp is the source of truth for colour (see CLAUDE.md); read step 500 from it. */
function brand500FromCss(): string {
  const match = mainCss.match(/--color-brand-500:\s*(#[0-9a-fA-F]{6})/)
  return match?.[1]?.toLowerCase() ?? ''
}

describe('pwa manifest', () => {
  it('declares the identity fields the install prompt shows', () => {
    expect(pwaManifest.name).toBe('Inventra - Manajemen Aset')
    expect(pwaManifest.short_name).toBe('Inventra')
    expect(pwaManifest.lang).toBe('id')
    expect(pwaManifest.description).toBeTruthy()
  })

  it('scopes the whole app so /en/ routes stay inside the installed window', () => {
    expect(pwaManifest.id).toBe('/')
    expect(pwaManifest.start_url).toBe('/')
    expect(pwaManifest.scope).toBe('/')
  })

  it('opens standalone', () => {
    expect(pwaManifest.display).toBe('standalone')
  })

  it('takes its theme colour from the brand ramp, not a second hand-typed hex', () => {
    // Guards the extraction itself: a restructured main.css must fail loudly here
    // rather than silently comparing '' to ''.
    expect(brand500FromCss()).toMatch(/^#[0-9a-f]{6}$/)
    expect(brand500FromCss()).toBe('#005bfd')
    expect(PWA_THEME_COLOR.toLowerCase()).toBe(brand500FromCss())
    expect(pwaManifest.theme_color.toLowerCase()).toBe(brand500FromCss())
  })

  it('sets a background colour matching the light-mode shell', () => {
    expect(pwaManifest.background_color).toMatch(/^#[0-9a-fA-F]{6}$/)
    expect(pwaManifest.background_color.toLowerCase()).toBe('#ffffff')
  })

  it('ships the icon sizes Android needs, including exactly one maskable', () => {
    const any = pwaManifest.icons.filter(i => i.purpose !== 'maskable')
    const maskable = pwaManifest.icons.filter(i => i.purpose === 'maskable')

    expect(any.map(i => i.sizes)).toEqual(expect.arrayContaining(['192x192', '512x512']))
    expect(maskable).toHaveLength(1)
    expect(maskable[0]!.sizes).toBe('512x512')
  })

  it('declares every icon as a png', () => {
    for (const icon of pwaManifest.icons) {
      expect(icon.type).toBe('image/png')
    }
  })

  it('points every icon at a file that actually exists in public/', () => {
    for (const icon of pwaManifest.icons) {
      expect(existsSync(resolve(publicDir, icon.src.replace(/^\//, '')))).toBe(true)
    }
  })

  it('keeps every icon same-origin', () => {
    for (const icon of pwaManifest.icons) {
      expect(icon.src.startsWith('/')).toBe(true)
      expect(icon.src).not.toMatch(/^(https?:)?\/\//)
    }
  })

  it('exposes an apple touch icon that exists, since iOS ignores manifest icons', () => {
    expect(PWA_APPLE_TOUCH_ICON).toBe('/apple-touch-icon-180x180.png')
    expect(existsSync(resolve(publicDir, PWA_APPLE_TOUCH_ICON.replace(/^\//, '')))).toBe(true)
  })

  it('points the manifest link at the file the build emits', () => {
    expect(PWA_MANIFEST_HREF).toBe('/manifest.webmanifest')
  })
})
