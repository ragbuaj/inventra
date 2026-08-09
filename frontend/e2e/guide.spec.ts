import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse, Page } from '@playwright/test'
import { EMAIL, PASSWORD, login, clickRowAction } from './helpers'

// ---------------------------------------------------------------------------
// Panduan Penggunaan (guide CMS) — real backend (/api/v1/guide).
//
// What only an end-to-end run can prove, and therefore what this file covers:
//
//   1. The authoring round trip against the REAL stack: a module created in
//      /settings/guide, a YouTube link parsed server-side, and a PDF that
//      actually reaches MinIO through the multipart endpoint.
//   2. The two-faced public response. The same GET /guide/modules answers a
//      guest and a signed-in reader differently (internal/guide/dto.go's
//      `attachmentResponse`), and the page renders "locked" cards purely
//      because the guest payload has nothing to render. A component test with a
//      mocked API can only assert the rendering half; only this can assert that
//      the server really withholds the video id, the file link, the filename,
//      and the size.
//   3. That a draft is invisible to everyone but an author — including to a
//      signed-in user who asks for `?status=all` (drafts are gated on
//      authority, not on the parameter).
//   4. That `guide.manage` gates the CMS at the nav, the route, AND the API.
//      The hidden menu leaf is not the control; the other two are.
//
// Deliberately NOT re-tested here (already covered fast, at the right layer):
//   - form validation, slug derivation, reorder/delete, locale fallback,
//     look-alike YouTube hosts → test/nuxt/guide-admin.spec.ts,
//     test/nuxt/guide-page.spec.ts, test/unit/guide-text.spec.ts
//   - middleware/serializer authorization matrix → backend
//     internal/guide/guide_integration_test.go (CI, testcontainers)
//
// NO EXTERNAL NETWORK: pressing play probes the video's thumbnail (i.ytimg.com)
// and then loads an iframe from youtube-nocookie.com. Both are intercepted
// (`stubYouTube`) so the assertions never depend on reaching YouTube — an
// unreachable thumbnail would otherwise flip the card into its "video tidak
// dapat diputar" state and the test would fail for a reason that has nothing to
// do with the code under test. The interceptor also COUNTS the thumbnail
// requests, which is how AC16 is proven in a real browser: zero before play.
//
// IMPORTANT: `pnpm test:e2e` needs the full backend stack (postgres/redis/minio)
// + the seeded admin, and locally the backend must run with
// RATELIMIT_ENABLED=false (CI sets it — see .github/workflows/ci.yml). This
// spec compiles + lints here; CI runs it in the e2e job.
//
// Robustness rules (project e2e conventions): unique names per run (the dev DB
// is NOT reset between local runs), assert-after-search, assert on rendered
// rows rather than on toasts (two adds in a row leave two identical toasts
// stacked, which strict mode rejects), and API-side cleanup in afterAll.
// ---------------------------------------------------------------------------

const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`
const RUN = `${Date.now()}`

// A real, minimal PDF. The backend sniffs the leading bytes (`%PDF-`, see
// internal/guide/attachment.go) rather than trusting the extension or the
// multipart content type, so the magic number has to be genuine.
const PDF_BYTES = Buffer.from('%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< >>\n%%EOF\n', 'utf-8')

// A file that lies: .pdf name, application/pdf content type, bytes that are
// neither. It passes the browser-side pre-flight (which can only look at name
// and type) and must be rejected by the server with 415.
const FAKE_PDF_BYTES = Buffer.from('this is definitely not a pdf\n', 'utf-8')

// 1x1 transparent PNG, served in place of the YouTube thumbnail.
const PNG_1PX = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64'
)

const YT_ID = 'dQw4w9WgXcQ'
const YT_URL = `https://www.youtube.com/watch?v=${YT_ID}&t=42s&list=PLignored`

interface GuideAttachmentJson {
  id: string
  kind: 'video' | 'document'
  title_id: string
  locked: boolean
  youtube_id?: string
  file_url?: string
  original_filename?: string
  size_bytes?: number
}
interface GuideModuleJson {
  id: string
  slug: string
  status: 'draft' | 'published'
  title_id: string
  attachments: GuideAttachmentJson[]
}

function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}

async function apiJson<T>(res: APIResponse): Promise<T> {
  if (!res.ok()) throw new Error(`API call failed: ${res.status()} ${res.url()} — ${await res.text()}`)
  return res.json() as Promise<T>
}

async function loginApi(api: APIRequestContext, email: string, password: string): Promise<string> {
  const res = await api.post('auth/login', { data: { email, password } })
  return (await apiJson<{ access_token: string }>(res)).access_token
}

// helpers.ts's `login(page)` only signs in the fixed seeded admin; the
// no-permission scenario needs a parametrized UI login (same steps, mirrors
// import.spec.ts / approval.spec.ts).
async function loginAs(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login')
  await page.locator('input[name="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password)
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
}

/** Modules as `token` sees them; pass no token for the anonymous projection. */
async function listModules(
  api: APIRequestContext, token?: string, includeDrafts = false
): Promise<{ res: APIResponse, data: GuideModuleJson[] }> {
  const res = await api.get(`guide/modules${includeDrafts ? '?status=all' : ''}`, {
    headers: token ? authHeader(token) : undefined
  })
  const body = await apiJson<{ data: GuideModuleJson[] }>(res)
  return { res, data: body.data }
}

async function findModule(
  api: APIRequestContext, token: string | undefined, titleID: string, includeDrafts = false
): Promise<GuideModuleJson | undefined> {
  const { data } = await listModules(api, token, includeDrafts)
  return data.find(m => m.title_id === titleID)
}

/**
 * Serves the YouTube thumbnail locally and blocks the embed, so the reader
 * page's video states are decided by the code under test instead of by whether
 * the CI runner can reach YouTube. Blocking the embed does not affect the
 * assertion on it — the iframe's `src` attribute is set by us before the
 * browser tries to load it.
 *
 * Returns a counter of thumbnail requests. AC16 says opening the guide must
 * reach no YouTube domain at all, and a counter is the only way to prove a
 * request that should not happen did not happen — asserting on the DOM only
 * shows what rendered, not what the browser fetched.
 */
async function stubYouTube(page: Page): Promise<() => number> {
  let thumbRequests = 0
  await page.route(
    url => url.hostname === 'i.ytimg.com',
    (route) => {
      thumbRequests++
      return route.fulfill({ status: 200, contentType: 'image/png', body: PNG_1PX })
    }
  )
  await page.route(
    url => url.hostname.endsWith('youtube-nocookie.com'),
    route => route.abort()
  )
  return () => thumbRequests
}

/**
 * The media block of one attachment on the reader page: the element that
 * immediately follows the attachment's title row inside GuideMediaCard (the
 * locked box, the player facade, or the document card).
 *
 * This is a structural locator because GuideMediaCard exposes no test id on its
 * root — see the "Temuan" note in the test report. Scoping matters: on a dev DB
 * that has accumulated modules from earlier runs, a bare page-level
 * `getByText('Video panduan terkunci')` would match some other run's card and
 * pass for the wrong reason.
 */
function mediaBlockFor(page: Page, attachmentTitle: string) {
  return page.getByText(attachmentTitle, { exact: true }).locator('xpath=../following-sibling::*[1]')
}

/**
 * Opens the public reader page and waits for its static shell.
 *
 * The navigation is retried as a unit. Against a Vite DEV server (the local
 * `docker compose -f docker-compose.dev.yml --profile app watch` stack) an
 * on-demand route transform occasionally stalls: the document stays blank, the
 * app never mounts, no request for /guide/modules is ever made, and nothing is
 * logged — observed once in six local runs, and never against the built app CI
 * serves through `pnpm preview`. A reload clears it.
 *
 * The retry cannot hide a real failure: all it waits for is the page's own
 * <h1>, which renders before the fetch resolves. Every assertion about guide
 * CONTENT is made afterwards, unretried.
 */
async function openGuidePage(page: Page): Promise<void> {
  await expect(async () => {
    await page.goto('/guide')
    await expect(page.getByRole('heading', { name: 'Panduan Penggunaan', exact: true }).first())
      .toBeVisible({ timeout: 12_000 })
  }).toPass({ timeout: 40_000 })
}

// ===========================================================================
// A. Authoring round trip, publication, and what a guest is allowed to see.
//    Serial: every test builds on the module the first one creates.
// ===========================================================================

test.describe('Panduan Penggunaan — CMS end-to-end', () => {
  test.describe.configure({ mode: 'serial' })

  const moduleTitle = `E2E Panduan ${RUN}`
  const moduleTitleEn = `E2E Guide ${RUN}`
  const step1 = `Langkah pertama e2e ${RUN}`
  const step2 = `Langkah kedua e2e ${RUN}`
  const videoTitle = `Video panduan e2e ${RUN}`
  const docTitle = `Dokumen panduan e2e ${RUN}`
  const pdfFilename = `panduan-e2e-${RUN}.pdf`

  let api: APIRequestContext
  let adminToken: string
  let moduleId: string | undefined

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await loginApi(api, EMAIL, PASSWORD)
  })

  test.afterAll(async () => {
    // Soft-deletes the module and, with it, its attachments (and their MinIO
    // objects). Runs even when a test failed midway, so a broken run does not
    // leave a published E2E module on the public page.
    if (moduleId) {
      await api.delete(`guide/modules/${moduleId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
    await api.dispose()
  })

  test('an author creates a draft module with numbered steps', async ({ page }) => {
    await login(page)

    // Positive control for the nav gate asserted negatively in describe B: the
    // Superadmin holds guide.manage, so the CMS leaf is in the sidebar.
    await expect(page.locator('nav a[href="/settings/guide"]').first()).toBeVisible()

    await page.goto('/settings/guide')
    await expect(page.getByTestId('guide-add')).toBeVisible({ timeout: 10_000 })

    await page.getByTestId('guide-add').click()
    const editor = page.getByRole('dialog')
    await expect(editor).toBeVisible({ timeout: 8_000 })

    await page.getByTestId('guide-title-id').fill(moduleTitle)
    await page.getByTestId('guide-title-en').fill(moduleTitleEn)

    await page.getByTestId('guide-add-step').click()
    await page.getByTestId('guide-add-step').click()
    const steps = page.getByTestId('guide-step-row')
    await expect(steps).toHaveCount(2)
    await steps.nth(0).locator('textarea').first().fill(step1)
    await steps.nth(1).locator('textarea').first().fill(step2)

    // A module that does not exist yet cannot carry attachments; the editor says
    // so instead of showing a manager that would have nothing to POST to.
    await expect(editor).toContainText('Simpan modul ini lebih dulu')

    await page.getByTestId('guide-save').click()
    await expect(editor).toBeHidden({ timeout: 10_000 })
    await expect(page.getByText('Modul panduan tersimpan.', { exact: true })).toBeVisible({ timeout: 8_000 })

    // Assert-after-search: nine seeded modules plus whatever earlier local runs
    // left behind share this table.
    await page.getByTestId('guide-search').fill(moduleTitle)
    const row = page.locator('tr').filter({ hasText: moduleTitle })
    await expect(row).toBeVisible({ timeout: 8_000 })
    // New modules are drafts — publishing is a separate, deliberate act.
    await expect(row).toContainText('Draf')
    // Column order is modul | urutan | status | langkah | lampiran | diperbarui
    // | aksi; asserting the cell rather than the row keeps "2" from matching a
    // digit that happens to sit in the date or the order column.
    await expect(row.locator('td').nth(3)).toHaveText('2')
    await expect(row.locator('td').nth(4)).toHaveText('—')

    const created = await findModule(api, adminToken, moduleTitle, true)
    expect(created, 'the created module must be visible to an author via the API').toBeTruthy()
    moduleId = created!.id
    expect(created!.status).toBe('draft')
    expect(created!.attachments).toHaveLength(0)
  })

  test('an author attaches a YouTube video and uploads a PDF', async ({ page }) => {
    test.setTimeout(60_000)

    await login(page)
    await page.goto('/settings/guide')
    await page.getByTestId('guide-search').fill(moduleTitle)
    const row = page.locator('tr').filter({ hasText: moduleTitle })
    await expect(row).toBeVisible({ timeout: 10_000 })

    await clickRowAction(page, row, 'Kelola lampiran')
    const editor = page.getByRole('dialog')
    await expect(editor).toBeVisible({ timeout: 8_000 })
    await expect(page.getByTestId('guide-att-tab-video')).toBeVisible()

    // --- YouTube link. The pasted URL carries &t= and &list=; the server keeps
    // only the 11-character id, which the editor echoes back. ---------------
    await page.getByTestId('guide-att-url').fill(YT_URL)
    await expect(page.getByTestId('guide-att-url-ok')).toContainText(`youtube_id: ${YT_ID}`)
    await page.getByTestId('guide-att-video-title').fill(videoTitle)
    await page.getByTestId('guide-att-add-video').click()
    await expect(page.getByTestId('guide-att-row').filter({ hasText: videoTitle }))
      .toBeVisible({ timeout: 10_000 })

    // --- PDF upload. Real multipart, real object storage. ------------------
    await page.getByTestId('guide-att-tab-pdf').click()
    await page.getByTestId('guide-att-file-input').setInputFiles({
      name: pdfFilename, mimeType: 'application/pdf', buffer: PDF_BYTES
    })
    await expect(page.getByTestId('guide-att-file')).toContainText(pdfFilename)
    await page.getByTestId('guide-att-pdf-title').fill(docTitle)
    await page.getByTestId('guide-att-add-pdf').click()
    const docRow = page.getByTestId('guide-att-row').filter({ hasText: docTitle })
    await expect(docRow).toBeVisible({ timeout: 15_000 })
    // The row's mono meta line is filename + size — proof the bytes were stored,
    // not just a row inserted.
    await expect(docRow).toContainText(pdfFilename)

    // The list behind the editor counts both kinds without an extra endpoint:
    // one video badge and one document badge in the "Lampiran" cell.
    await page.getByRole('button', { name: 'Batal', exact: true }).click()
    await expect(editor).toBeHidden({ timeout: 8_000 })
    await expect(page.locator('tr').filter({ hasText: moduleTitle }).locator('td').nth(4))
      .toHaveText(/1\s*1/)

    const stored = await findModule(api, adminToken, moduleTitle, true)
    expect(stored!.attachments).toHaveLength(2)
    const video = stored!.attachments.find(a => a.kind === 'video')
    const doc = stored!.attachments.find(a => a.kind === 'document')
    expect(video!.youtube_id).toBe(YT_ID)
    expect(doc!.original_filename).toBe(pdfFilename)
    expect(doc!.size_bytes).toBe(PDF_BYTES.length)
  })

  test('a file that only pretends to be a PDF is rejected by the server', async ({ page }) => {
    // The browser-side pre-flight passes this file (right extension, right
    // content type); only the server reads its bytes. This is the failure path
    // no mocked test can prove.
    await login(page)
    await page.goto('/settings/guide')
    await page.getByTestId('guide-search').fill(moduleTitle)
    const row = page.locator('tr').filter({ hasText: moduleTitle })
    await expect(row).toBeVisible({ timeout: 10_000 })

    await clickRowAction(page, row, 'Kelola lampiran')
    await expect(page.getByRole('dialog')).toBeVisible({ timeout: 8_000 })
    await expect(page.getByTestId('guide-att-row')).toHaveCount(2)

    await page.getByTestId('guide-att-tab-pdf').click()
    await page.getByTestId('guide-att-file-input').setInputFiles({
      name: `bukan-pdf-${RUN}.pdf`, mimeType: 'application/pdf', buffer: FAKE_PDF_BYTES
    })
    await expect(page.getByTestId('guide-att-file')).toBeVisible()
    await page.getByTestId('guide-att-pdf-title').fill(`Palsu ${RUN}`)
    await page.getByTestId('guide-att-add-pdf').click()

    await expect(page.getByTestId('guide-att-error')).toContainText('bukan PDF', { timeout: 15_000 })
    // Nothing was created: the rejection is not a cosmetic warning.
    await expect(page.getByTestId('guide-att-row')).toHaveCount(2)
    const stored = await findModule(api, adminToken, moduleTitle, true)
    expect(stored!.attachments).toHaveLength(2)
  })

  test('a draft module is invisible to a guest, page and API alike', async ({ page }) => {
    test.setTimeout(60_000)
    // The public page loads for a visitor with no session at all — the point of
    // the whole optional-auth design.
    await openGuidePage(page)
    await expect(page.getByText(moduleTitle, { exact: true })).toHaveCount(0)

    const anonApi = await request.newContext({ baseURL: API_BASE })
    try {
      // Even asking for drafts explicitly yields none: `?status=all` is honoured
      // on authority, not on the parameter.
      const { data } = await listModules(anonApi, undefined, true)
      expect(data.some(m => m.title_id === moduleTitle)).toBe(false)
      expect(data.every(m => m.status === 'published')).toBe(true)
    } finally {
      await anonApi.dispose()
    }
  })

  test('publishing puts the module, its video, and its PDF in front of a reader', async ({ page }) => {
    test.setTimeout(90_000)
    const thumbRequests = await stubYouTube(page)
    await login(page)
    await page.goto('/settings/guide')
    await page.getByTestId('guide-search').fill(moduleTitle)
    const row = page.locator('tr').filter({ hasText: moduleTitle })
    await expect(row).toBeVisible({ timeout: 10_000 })

    await clickRowAction(page, row, 'Terbitkan')
    await expect(page.getByText('Modul diterbitkan.', { exact: true })).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('tr').filter({ hasText: moduleTitle })).toContainText('Terbit')

    // --- the reader page, same session ------------------------------------
    await openGuidePage(page)
    await expect(page.getByRole('heading', { name: moduleTitle, exact: true }))
      .toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(step1, { exact: true })).toBeVisible()
    await expect(page.getByText(step2, { exact: true })).toBeVisible()

    // Video: the facade renders, NOTHING is requested from any YouTube domain
    // until the reader presses play (AC16) — not the embed and not the
    // thumbnail — and the embed that then appears is the one the app builds
    // from the stored id, never a URL echoed back from the editor.
    const videoBlock = mediaBlockFor(page, videoTitle)
    await expect(videoBlock).toBeVisible()
    await expect(page.locator('iframe')).toHaveCount(0)
    await expect(page.locator('img[src*="ytimg.com"]')).toHaveCount(0)
    expect(thumbRequests()).toBe(0)

    await videoBlock.click()
    await expect(page.locator(`iframe[src*="youtube-nocookie.com/embed/${YT_ID}"]`)).toHaveCount(1)
    // Pressing play is what releases the probe, and only then.
    expect(thumbRequests()).toBeGreaterThan(0)

    // Document: filename, size, and the two actions a reader gets.
    const docBlock = mediaBlockFor(page, docTitle)
    await expect(docBlock).toContainText(pdfFilename)
    await expect(docBlock.getByRole('button', { name: 'Unduh', exact: true })).toBeVisible()
    await expect(docBlock.getByRole('button', { name: 'Pratinjau', exact: true })).toBeVisible()

    // No locked chrome anywhere for a signed-in reader.
    await expect(page.getByText('Video panduan terkunci', { exact: true })).toHaveCount(0)
    await expect(page.getByText('Dokumen panduan terkunci', { exact: true })).toHaveCount(0)
  })

  test('a guest reads the text but gets locked cards, and the payload really is stripped', async ({ page }) => {
    test.setTimeout(60_000)
    await stubYouTube(page)
    // No login: a fresh Playwright context carries no session.
    await openGuidePage(page)
    await expect(page.getByRole('heading', { name: moduleTitle, exact: true }))
      .toBeVisible({ timeout: 10_000 })

    // The text is public — that is the whole point of keeping /guide open.
    await expect(page.getByText(step1, { exact: true })).toBeVisible()
    await expect(page.getByText(step2, { exact: true })).toBeVisible()

    // Both attachments announce themselves and offer the way in, without
    // exposing anything playable or downloadable.
    const videoBlock = mediaBlockFor(page, videoTitle)
    await expect(videoBlock).toContainText('Video panduan terkunci')
    await expect(videoBlock.getByRole('link', { name: 'Masuk', exact: true })).toBeVisible()

    const docBlock = mediaBlockFor(page, docTitle)
    await expect(docBlock).toContainText('Dokumen panduan terkunci')
    await expect(docBlock.getByRole('link', { name: 'Masuk', exact: true })).toBeVisible()

    await expect(page.locator('iframe')).toHaveCount(0)

    // Nothing leaked into the document: not the video id (which would let anyone
    // reconstruct the embed), not the filename of an internal document.
    const html = await page.content()
    expect(html).not.toContain(YT_ID)
    expect(html).not.toContain(pdfFilename)

    // --- and the same, one layer down, at the API ------------------------
    const anonApi = await request.newContext({ baseURL: API_BASE })
    try {
      const { res, data } = await listModules(anonApi)
      // A shared cache must never be able to replay a signed-in payload to the
      // next guest (plan risk R2).
      expect(res.headers()['cache-control']).toContain('no-store')
      expect(res.headers()['vary']).toContain('Authorization')

      const mine = data.find(m => m.title_id === moduleTitle)
      expect(mine, 'a published module must be readable without a session').toBeTruthy()
      expect(mine!.attachments).toHaveLength(2)
      for (const a of mine!.attachments) {
        expect(a.locked).toBe(true)
        expect(a.youtube_id).toBeUndefined()
        expect(a.file_url).toBeUndefined()
        expect(a.original_filename).toBeUndefined()
        expect(a.size_bytes).toBeUndefined()
      }

      // The bytes themselves stay behind a session even when the id is known.
      const doc = mine!.attachments.find(a => a.kind === 'document')!
      const content = await anonApi.get(`guide/attachments/${doc.id}/content`)
      expect(content.status()).toBe(401)
    } finally {
      await anonApi.dispose()
    }
  })
})

// ===========================================================================
// B. A signed-in user WITHOUT guide.manage.
//    The role holds user.manage so the "Pengaturan" group still renders — the
//    assertion is then about the guide leaf specifically, not about a whole
//    group being hidden, which would pass for the wrong reason.
// ===========================================================================

test.describe('Panduan Penggunaan — a user without guide.manage', () => {
  test.describe.configure({ mode: 'serial' })

  const readerEmail = `e2e.guide.reader.${RUN}@inventra.local`
  const readerPassword = `Reader${RUN}!`
  const draftTitle = `E2E Panduan Draf ${RUN}`
  const openTitle = `E2E Panduan Terbit ${RUN}`
  const openPdfName = `panduan-terbit-${RUN}.pdf`

  let api: APIRequestContext
  let adminToken: string
  let readerToken: string
  let roleId: string | undefined
  let userId: string | undefined
  let draftId: string | undefined
  let openId: string | undefined
  let openDocId: string

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await loginApi(api, EMAIL, PASSWORD)

    // A purpose-built role: one Pengaturan-group permission, and explicitly not
    // guide.manage. Cloning Superadmin would defeat the test; picking a role
    // with no admin permission at all would hide the whole group.
    const role = await apiJson<{ id: string }>(await api.post('authz/roles', {
      headers: authHeader(adminToken),
      data: { code: `E2EGUIDE${RUN}`, name: `E2E Guide NoManage ${RUN}` }
    }))
    roleId = role.id
    await apiJson(await api.put(`authz/roles/${roleId}/permissions`, {
      headers: authHeader(adminToken),
      data: { permissions: ['user.manage'] }
    }))

    userId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: {
        name: `E2E Guide Reader ${RUN}`, email: readerEmail,
        password: readerPassword, role_id: roleId
      }
    }))).id
    readerToken = await loginApi(api, readerEmail, readerPassword)

    // An unpublished module this user must never see, whatever it asks for.
    // Creation always yields a draft, so this needs no second step.
    draftId = (await apiJson<{ id: string }>(await api.post('guide/modules', {
      headers: authHeader(adminToken),
      data: {
        slug: `e2e-draf-${RUN}`, icon: 'i-lucide-book-open', sort_order: 900,
        title_id: draftTitle, steps: []
      }
    }))).id

    // ...and a published one WITH a document, to prove the other half of the
    // rule: reading attachments needs a session, nothing more. Built here via
    // the API so this describe never depends on describe A having run.
    // Publishing is a second request by design — POST cannot publish.
    openId = (await apiJson<{ id: string }>(await api.post('guide/modules', {
      headers: authHeader(adminToken),
      data: {
        slug: `e2e-terbit-${RUN}`, icon: 'i-lucide-book-open', sort_order: 901,
        title_id: openTitle, steps: []
      }
    }))).id
    await apiJson(await api.patch(`guide/modules/${openId}`, {
      headers: authHeader(adminToken),
      data: {
        slug: `e2e-terbit-${RUN}`, icon: 'i-lucide-book-open', sort_order: 901,
        status: 'published', title_id: openTitle, steps: []
      }
    }))
    openDocId = (await apiJson<{ id: string }>(await api.post(`guide/modules/${openId}/attachments/document`, {
      headers: authHeader(adminToken),
      multipart: {
        file: { name: openPdfName, mimeType: 'application/pdf', buffer: PDF_BYTES },
        title_id: `Dokumen terbit ${RUN}`,
        sort_order: '1'
      }
    }))).id
  })

  test.afterAll(async () => {
    if (draftId) await api.delete(`guide/modules/${draftId}`, { headers: authHeader(adminToken) }).catch(() => {})
    if (openId) await api.delete(`guide/modules/${openId}`, { headers: authHeader(adminToken) }).catch(() => {})
    if (userId) await api.delete(`users/${userId}`, { headers: authHeader(adminToken) }).catch(() => {})
    if (roleId) await api.delete(`authz/roles/${roleId}`, { headers: authHeader(adminToken) }).catch(() => {})
    await api.dispose()
  })

  test('the Panduan leaf is absent from Administrasi > Pengaturan', async ({ page }) => {
    await loginAs(page, readerEmail, readerPassword)

    // The group renders (its Users leaf is visible), so a missing guide leaf is
    // the guide permission talking, not an empty group.
    await expect(page.locator('nav a[href="/settings/users"]').first()).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('nav a[href="/settings/guide"]')).toHaveCount(0)

    // The READER entry keeps its place: /guide carries no permission, and it
    // shares its label with the CMS leaf — which is exactly why these two
    // assertions are written against hrefs rather than against link text.
    await expect(page.locator('nav a[href="/guide"]').first()).toBeVisible()
  })

  test('the CMS route refuses the same user even when opened directly', async ({ page }) => {
    test.setTimeout(60_000)
    await loginAs(page, readerEmail, readerPassword)

    // The `can` middleware aborts with statusMessage 'Akses ditolak'. The
    // navigation is retried for the same dev-server reason documented on
    // openGuidePage; the assertion is positive, so a retry can only ever wait
    // for the refusal to appear, never invent one.
    await expect(async () => {
      await page.goto('/settings/guide')
      await expect(page.locator('body')).toContainText('Akses ditolak', { timeout: 12_000 })
    }).toPass({ timeout: 40_000 })
    await expect(page.getByTestId('guide-add')).toHaveCount(0)
  })

  test('the API refuses the same user: no writes, no drafts', async () => {
    const create = await api.post('guide/modules', {
      headers: authHeader(readerToken),
      data: {
        slug: `e2e-tolak-${RUN}`, icon: 'i-lucide-book-open', sort_order: 901,
        status: 'draft', title_id: `E2E Ditolak ${RUN}`, steps: []
      }
    })
    expect(create.status()).toBe(403)

    const { data } = await listModules(api, readerToken, true)
    expect(data.some(m => m.title_id === draftTitle)).toBe(false)
    expect(data.every(m => m.status === 'published')).toBe(true)
  })

  test('but that user still reads published attachments — guide.manage governs writing, not reading', async () => {
    const open = await findModule(api, readerToken, openTitle)
    expect(open, 'a published module must be readable by any signed-in user').toBeTruthy()
    const doc = open!.attachments.find(a => a.kind === 'document')
    expect(doc).toBeTruthy()
    expect(doc!.locked).toBe(false)
    expect(doc!.original_filename).toBe(openPdfName)

    // And the bytes come through: authorization for reading media is the
    // session, not the permission.
    const content = await api.get(`guide/attachments/${openDocId}/content`, {
      headers: authHeader(readerToken)
    })
    expect(content.status()).toBe(200)
    expect(content.headers()['content-type']).toContain('application/pdf')
  })
})
