import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import {
  pwaWorkbox,
  PWA_SHELL_ROUTE,
  PWA_NAVIGATE_FALLBACK_DENYLIST
} from '../../pwa/workbox'

const caddyfile = readFileSync(
  resolve(fileURLToPath(new URL('../../', import.meta.url)), '../ops/caddy/Caddyfile'),
  'utf8'
)

/**
 * The paths Caddy routes to the Go backend instead of to this app.
 *
 * These and the navigation denylist are two lists that must agree, and until now
 * nothing made them. Add a third backend path to the Caddyfile without touching
 * `pwa/workbox.ts` and, offline, a navigation to it is answered with 200 of HTML
 * where the caller expects JSON — invisible online, because Nitro's own SPA
 * fallback returns the same shell either way.
 */
function caddyBackendPaths(): string[] {
  const match = caddyfile.match(/^\s*@api\s+path\s+(.+)$/m)
  return match?.[1]?.trim().split(/\s+/) ?? []
}

/** A concrete navigation target standing in for a Caddy path pattern. */
function concreteNavigation(caddyPath: string): string {
  return caddyPath.endsWith('/*') ? `${caddyPath.slice(0, -1)}probe` : caddyPath
}

/**
 * Every key this config is allowed to declare. The list is deliberately exhaustive
 * rather than a denylist of keys we happened to think of: asserting the whole key
 * set means ANY new option — `runtimeCaching`, `importScripts`, a strategy switch —
 * turns this test red until someone consciously adds it here and justifies it.
 *
 * The narrow version of this test (checking `runtimeCaching` alone) left three ways
 * in that nothing caught: `importScripts` loading a script that registers routes,
 * switching to `injectManifest` with a hand-written worker, and options added by a
 * future version of the module.
 */
const ALLOWED_WORKBOX_KEYS = [
  'cleanupOutdatedCaches',
  'globPatterns',
  'maximumFileSizeToCacheInBytes',
  'navigateFallback',
  'navigateFallbackDenylist'
]

/**
 * Workbox tests `navigateFallbackDenylist` against `url.pathname + url.search`
 * (workbox-routing `NavigationRoute._match`), so the fixtures below are written in
 * exactly that shape rather than as bare pathnames.
 */
function isDeniedNavigation(pathnameAndSearch: string): boolean {
  return PWA_NAVIGATE_FALLBACK_DENYLIST.some(re => re.test(pathnameAndSearch))
}

describe('pwa workbox strategy', () => {
  it('falls back to the prerendered shell for navigations', () => {
    expect(PWA_SHELL_ROUTE).toBe('/')
    expect(pwaWorkbox.navigateFallback).toBe(PWA_SHELL_ROUTE)
  })

  it('declares no runtime caching at all, so no API response can ever be stored', () => {
    // The single most important assertion in this file: in production the API is
    // same-origin with the frontend, so one runtime rule here would quietly persist
    // asset data to Cache Storage on every device.
    expect('runtimeCaching' in pwaWorkbox).toBe(false)
  })

  it('declares no option outside the allowlist, so no caching can arrive unnoticed', () => {
    expect(Object.keys(pwaWorkbox).sort()).toEqual(ALLOWED_WORKBOX_KEYS)
  })

  it('never precaches anything under /api/', () => {
    for (const pattern of pwaWorkbox.globPatterns) {
      expect(pattern).not.toContain('api')
    }
  })

  it('precaches the build assets the shell needs to boot offline', () => {
    const patterns = pwaWorkbox.globPatterns.join(' ')
    for (const ext of ['html', 'js', 'css', 'woff2', 'png', 'svg', 'ico', 'webmanifest', 'json']) {
      expect(patterns).toContain(ext)
    }
  })

  it('raises the file-size ceiling above the largest build chunk', () => {
    // Workbox drops oversized files from the precache *silently*; the biggest Nuxt
    // entry chunk is a few hundred KB, so the 2 MiB default would be a trap only if
    // it shrank. Keep it explicit and comfortably above the real bundle.
    expect(pwaWorkbox.maximumFileSizeToCacheInBytes).toBeGreaterThanOrEqual(4 * 1024 * 1024)
  })

  it('sweeps caches left behind by a previous deploy', () => {
    expect(pwaWorkbox.cleanupOutdatedCaches).toBe(true)
  })

  it('does not activate itself behind the user back, matching registerType prompt', () => {
    expect(pwaWorkbox.skipWaiting).not.toBe(true)
    expect(pwaWorkbox.clientsClaim).not.toBe(true)
  })

  it('denies the API prefix so an offline fetch fails instead of returning the shell', () => {
    expect(isDeniedNavigation('/api/v1/assets')).toBe(true)
    expect(isDeniedNavigation('/api/v1/assets?limit=20')).toBe(true)
    expect(isDeniedNavigation('/api/')).toBe(true)
  })

  it('denies the health endpoint, with or without a query string', () => {
    expect(isDeniedNavigation('/health')).toBe(true)
    expect(isDeniedNavigation('/health?probe=1')).toBe(true)
  })

  it('still serves the shell for real app routes, including the /en/ locale', () => {
    for (const route of ['/', '/aset', '/aset/123', '/en/', '/en/aset', '/pengaturan?tab=profil']) {
      expect(isDeniedNavigation(route)).toBe(false)
    }
  })

  it('denies every path Caddy routes to the backend, so the two lists cannot drift', () => {
    const paths = caddyBackendPaths()

    // Guards the extraction itself: a restructured Caddyfile must fail loudly here
    // rather than silently asserting over an empty list.
    expect(paths, 'could not read the @api matcher from ops/caddy/Caddyfile').toContain('/api/*')
    expect(paths.length).toBeGreaterThanOrEqual(2)

    for (const path of paths) {
      const navigation = concreteNavigation(path)
      expect(
        isDeniedNavigation(navigation),
        `Caddy sends ${path} to the backend, so ${navigation} must not be answered with the shell`
      ).toBe(true)
    }
  })

  it('does not deny app routes that merely contain the denied words', () => {
    // `/^\/api\//` must not become `/api/`, which would also swallow these.
    for (const route of ['/rapid', '/laporan/api-usage', '/healthcheck-report']) {
      expect(isDeniedNavigation(route)).toBe(false)
    }
  })
})
