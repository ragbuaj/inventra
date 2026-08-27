import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login } from './helpers'

// ---------------------------------------------------------------------------
// PWA — service worker, offline shell, and cache safety.
//
// This spec exists for what a unit test structurally cannot prove: that a
// service worker really activates, that a navigation really survives the network
// going away, and — the reason this file is a SECURITY test rather than a
// convenience one — that Cache Storage really holds nothing from the API after a
// session has been used.
//
// Why that last one needs a real browser: in production the API is same-origin
// with the frontend (https://DOMAIN/api/v1, see docker-compose.prod.yml), while
// in dev it lives on a different origin (:8080). A caching rule that swept API
// responses onto the user's disk would therefore be INVISIBLE during development
// and would only ever bite in production, on bank asset data. So the assertion is
// made by reading `caches` directly, before and after logout — never by eye.
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack + seeded admin and
// RATELIMIT_ENABLED=false on the backend (see CLAUDE.md). The service worker only
// exists in a real build, so this spec runs against `pnpm preview` — never
// `pnpm dev`, where /sw.js deliberately falls through to the SPA fallback.
//
// This spec creates no backend data, so it has nothing to clean up.
// ---------------------------------------------------------------------------

/** The origin the frontend calls for data — nothing from here may ever be cached. */
const API_BASE = process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1'
const API_ORIGIN = new URL(API_BASE).origin

const SEARCH_PLACEHOLDER = 'Cari nama atau kode aset'
const LOAD_ERROR = 'Gagal memuat data.'

interface WebManifest {
  name: string
  short_name: string
  start_url: string
  scope: string
  display: string
  lang: string
  id: string
  theme_color: string
  icons: Array<{ src: string, sizes: string, type: string, purpose?: string }>
}

/** Every URL currently held in Cache Storage, across all cache buckets. */
async function cachedUrls(page: Page): Promise<string[]> {
  return page.evaluate(async () => {
    const names = await caches.keys()
    const urls: string[] = []
    for (const name of names) {
      const cache = await caches.open(name)
      for (const req of await cache.keys()) urls.push(req.url)
    }
    return urls
  })
}

/**
 * Waits until a service worker is activated AND its precache has actually
 * landed. Activation alone is not enough: Workbox fills the precache
 * asynchronously, so a `caches` read taken the instant the worker activates can
 * legitimately come back empty — which would make the "no API entries" assertion
 * below pass for entirely the wrong reason.
 */
async function waitForPrimedServiceWorker(page: Page): Promise<void> {
  await expect.poll(
    () => page.evaluate(async () => {
      const reg = await navigator.serviceWorker.getRegistration()
      return reg?.active?.state ?? 'none'
    }),
    { message: 'service worker never reached "activated"', timeout: 30_000 }
  ).toBe('activated')

  await expect.poll(
    async () => (await cachedUrls(page)).length,
    { message: 'service worker activated but its precache stayed empty', timeout: 30_000 }
  ).toBeGreaterThan(0)
}

/**
 * Every path the shipped service worker declares in its precache manifest.
 *
 * Read from `/sw.js` rather than hardcoded, so this stays correct as the bundle
 * changes. It is what turns the cache assertion below from a denylist ("no /api/
 * URLs") into an allowlist ("nothing but these"), which is the difference between
 * catching the one leak we thought of and catching every leak.
 */
async function precachedPaths(page: Page, origin: string): Promise<Set<string>> {
  const sw = await (await page.request.get('/sw.js')).text()
  const paths = new Set<string>()
  for (const match of sw.matchAll(/url:\s*"([^"]+)"/g)) {
    paths.add(new URL(match[1]!, origin).pathname)
  }
  return paths
}

/**
 * Asserts Cache Storage holds the precache and nothing else.
 *
 * Three separate claims, kept separate so a failure names which one broke: the
 * cache is not empty (an empty one would make the rest prove nothing), it holds
 * nothing from another origin, and every entry it does hold was precached at build
 * time. The last claim subsumes "no API responses" and also catches responses from
 * anywhere else — fonts, analytics, a file service added later.
 */
async function expectCacheHoldsOnlyPrecache(page: Page, when: string): Promise<void> {
  const origin = new URL(page.url()).origin
  const urls = await cachedUrls(page)
  const allowed = await precachedPaths(page, origin)

  expect(urls.length, `${when}: an empty cache would make this assertion prove nothing`).toBeGreaterThan(0)
  expect(allowed.size, `${when}: could not read the precache manifest from /sw.js`).toBeGreaterThan(0)

  const foreign = urls.filter(u => new URL(u).origin !== origin)
  expect(foreign, `${when}: Cache Storage must hold nothing from another origin`).toEqual([])

  const unexpected = urls.filter(u => !allowed.has(new URL(u).pathname))
  expect(unexpected, `${when}: Cache Storage must hold nothing that was not precached at build time`).toEqual([])

  // Kept explicit alongside the allowlist: this is the invariant the spec names, and
  // a reader should not have to derive it from the manifest to see it asserted.
  expect(urls.filter(u => new URL(u).pathname.startsWith('/api/')), `${when}: no API response may be cached`).toEqual([])
  expect(urls.filter(u => u.startsWith(API_ORIGIN)), `${when}: nothing from the API origin may be cached`).toEqual([])
}

/** True when the document in front of us is our own shell, not the browser's network-error page. */
async function isAppShell(page: Page): Promise<boolean> {
  return page.evaluate(() => Boolean(
    document.querySelector('link[rel="manifest"]') && document.querySelector('#__nuxt')
  ))
}

/** Signs out through the real UI, so the post-logout cache read follows the path a user takes. */
async function logoutViaUi(page: Page): Promise<void> {
  await page.getByTestId('user-menu-trigger').click()
  await page.getByTestId('user-menu-logout').click()
  await expect(page).toHaveURL(/\/login$/)
}

test.describe('PWA manifest and icons', () => {
  test('serves a manifest whose identity fields drive the install prompt', async ({ page }) => {
    const res = await page.request.get('/manifest.webmanifest')
    expect(res.status()).toBe(200)
    expect(res.headers()['content-type']).toContain('manifest')

    const manifest = await res.json() as WebManifest
    expect(manifest.name).toBe('Inventra - Manajemen Aset')
    expect(manifest.short_name).toBe('Inventra')
    expect(manifest.lang).toBe('id')
    expect(manifest.display).toBe('standalone')
    expect(manifest.theme_color.toLowerCase()).toBe('#005bfd')
    // Root scope, so the /en/ locale routes stay inside the installed window.
    expect(manifest.id).toBe('/')
    expect(manifest.start_url).toBe('/')
    expect(manifest.scope).toBe('/')
  })

  test('serves every icon the manifest declares as a real image', async ({ page }) => {
    const manifest = await (await page.request.get('/manifest.webmanifest')).json() as WebManifest
    expect(manifest.icons.length).toBeGreaterThan(0)

    for (const icon of manifest.icons) {
      const res = await page.request.get(icon.src)
      expect(res.status(), `icon ${icon.src} must be served`).toBe(200)
      expect(res.headers()['content-type'], `icon ${icon.src} must be an image`).toContain('image/')
    }
  })

  test('serves the apple touch icon, the only icon iOS reads', async ({ page }) => {
    const res = await page.request.get('/apple-touch-icon-180x180.png')
    expect(res.status()).toBe(200)
    expect(res.headers()['content-type']).toContain('image/')
  })

  test('ships the head tags in the delivered HTML, not only after hydration', async ({ page }) => {
    // The app is ssr:false, so a tag added by useHead would land too late for the
    // install criteria and for Safari, which reads apple-touch-icon at load time.
    const html = await (await page.request.get('/')).text()
    expect(html).toContain('rel="manifest"')
    expect(html).toContain('name="theme-color"')
    expect(html).toContain('rel="apple-touch-icon"')
  })
})

test.describe('PWA service worker', () => {
  test('activates and precaches the app shell alongside the build assets', async ({ page }) => {
    await page.goto('/')
    await waitForPrimedServiceWorker(page)

    const scriptURL = await page.evaluate(async () => {
      const reg = await navigator.serviceWorker.getRegistration()
      return reg?.active?.scriptURL ?? ''
    })
    expect(scriptURL).toContain('/sw.js')

    const urls = await cachedUrls(page)
    // The prerendered shell is precached under the bare route, which is what
    // navigateFallback binds to — without it, no navigation can survive offline.
    expect(urls.some(u => new URL(u).pathname === '/')).toBe(true)
    expect(urls.some(u => u.endsWith('.js'))).toBe(true)
    expect(urls.some(u => u.endsWith('.css'))).toBe(true)
  })
})

test.describe('PWA cache safety', () => {
  test('ships a service worker that registers no caching strategy at all', async ({ page }) => {
    // The strongest form of the "no runtime caching" invariant: asserted against the
    // worker actually delivered to the device, not against the config that produced
    // it. A config-level test can be bypassed (importScripts, a switch to
    // injectManifest, a hand-written worker); the shipped artefact cannot.
    //
    // This matters most because the runtime check below is blind to the case that
    // actually bites: here the API is cross-origin (:8080), in production it is
    // same-origin, and Workbox's RegExpRoute only applies cross-origin when the
    // pattern matches at index 0 of the full URL. A same-origin-only caching rule
    // would therefore stay invisible to every other test in this file.
    const sw = await (await page.request.get('/sw.js')).text()

    for (const strategy of ['NetworkFirst', 'CacheFirst', 'StaleWhileRevalidate', 'CacheOnly', 'NetworkOnly']) {
      expect(sw, `service worker must not register a ${strategy} route`).not.toContain(strategy)
    }

    // Exactly one route, and it is the navigation fallback to the precached shell.
    const routes = sw.match(/registerRoute\(/g) ?? []
    expect(routes, 'the shipped worker must register exactly one route').toHaveLength(1)
    expect(sw).toContain('NavigationRoute')
  })

  test('keeps every API response out of Cache Storage, before and after logout', async ({ page }) => {
    await page.goto('/')
    await waitForPrimedServiceWorker(page)

    await login(page)
    await page.goto('/assets')
    // Wait until the screen has genuinely talked to the API, so the assertion is
    // made against a cache that had every chance to be polluted.
    await expect(page.getByPlaceholder(SEARCH_PLACEHOLDER)).toBeVisible()
    await expect(page.getByText(LOAD_ERROR)).toHaveCount(0)

    await expectCacheHoldsOnlyPrecache(page, 'while signed in')

    await logoutViaUi(page)

    await expectCacheHoldsOnlyPrecache(page, 'after logout')
  })
})

test.describe('PWA offline behaviour', () => {
  test('serves the app shell offline instead of the browser error page', async ({ page, context }) => {
    await page.goto('/')
    await waitForPrimedServiceWorker(page)

    await context.setOffline(true)
    await page.goto('/assets')

    expect(await isAppShell(page), 'offline navigation fell through to the browser error page').toBe(true)
    // The shell alone is just markup; the app must actually boot from precached
    // JS and CSS, which it proves by rendering the login screen — offline the
    // session cannot be restored, so the route guard sends us there.
    await expect(page.locator('input[name="email"]')).toBeVisible({ timeout: 30_000 })
  })

  test('serves the same shell for the /en/ locale offline', async ({ page, context }) => {
    await page.goto('/')
    await waitForPrimedServiceWorker(page)

    await context.setOffline(true)
    await page.goto('/en/')

    expect(await isAppShell(page)).toBe(true)
    await expect(page.locator('input[name="email"]')).toBeVisible({ timeout: 30_000 })
  })

  test('lets a denylisted request fail offline rather than answering it with the shell', async ({ page, context }) => {
    // The navigation fallback must not swallow /api/ and /health. Online this is
    // impossible to observe — Nitro's own SPA fallback returns the same shell for
    // an unknown path, so both behaviours look identical. Offline separates them:
    // a denylisted navigation has no fallback left and simply fails, which is the
    // honest answer. Were it missing from the denylist, the caller would instead
    // receive a 200 page of HTML where it expected JSON.
    await page.goto('/')
    await waitForPrimedServiceWorker(page)
    await context.setOffline(true)

    // Control first: a real app route in this offline context DOES get the shell,
    // so the assertions below cannot pass merely because nothing works offline.
    // It also has to run before the failing navigations — those leave the tab on
    // chrome-error://, whose pending navigation would interrupt this one.
    await page.goto('/assets')
    expect(await isAppShell(page)).toBe(true)

    for (const path of ['/api/v1/assets', '/health']) {
      let servedShell: boolean
      try {
        await page.goto(path)
        servedShell = await isAppShell(page)
      } catch {
        // Navigation rejected outright — exactly the intended outcome.
        servedShell = false
      }
      expect(servedShell, `${path} must not be answered with the app shell`).toBe(false)
    }
  })

  test('shows the load-error state on a data screen instead of stale rows', async ({ page, context }) => {
    await page.goto('/')
    await waitForPrimedServiceWorker(page)

    await login(page)
    await page.goto('/assets')
    const search = page.getByPlaceholder(SEARCH_PLACEHOLDER)
    await expect(search).toBeVisible()

    // Cutting the network and forcing a refetch must surface the app's own error
    // state. If any runtime caching rule existed for the API, this would instead
    // answer quietly from cache — stale bank asset data presented as current.
    await context.setOffline(true)
    await search.fill('inventra-offline-probe')

    await expect(page.getByText(LOAD_ERROR)).toBeVisible({ timeout: 30_000 })
    await expect(page.getByRole('button', { name: 'Coba lagi' })).toBeVisible()
  })

  test('recovers to a working app once the network returns', async ({ page, context }) => {
    await page.goto('/')
    await waitForPrimedServiceWorker(page)

    await context.setOffline(true)
    await page.goto('/assets')
    await expect(page.locator('input[name="email"]')).toBeVisible({ timeout: 30_000 })

    await context.setOffline(false)
    await page.reload()
    await login(page)
    await page.goto('/assets')
    await expect(page.getByPlaceholder(SEARCH_PLACEHOLDER)).toBeVisible()
    await expect(page.getByText(LOAD_ERROR)).toHaveCount(0)
  })
})
