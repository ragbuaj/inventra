// @vitest-environment nuxt
// The public Panduan Penggunaan page, now that its content comes from the API.
//
// The property worth protecting here is the guest/reader split: a visitor with
// no session must still get the text, must NOT get a video id or a file link,
// and must never be bounced to /login by this page. The API enforces that by
// omitting the media fields; these tests assert the page renders that omission
// as a deliberate locked state rather than an empty gap.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import type { GuideAttachment, GuideModule } from '~/types'

type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

const requests: string[] = []
let handler: RequestHandler = () => ({ data: [] })
let blobHandler: () => Promise<Blob> = () => Promise.resolve(new Blob(['%PDF-1.7'], { type: 'application/pdf' }))

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => {
      requests.push(path)
      return Promise.resolve(handler(path, opts))
    },
    requestBlob: (path: string) => {
      requests.push(path)
      return blobHandler()
    }
  })
}))

// eslint-disable-next-line import/first
import GuidePage from '~/pages/guide.vue'

function attachment(over: Partial<GuideAttachment> = {}): GuideAttachment {
  return {
    id: 'a1',
    kind: 'video',
    title_id: 'Menelusuri dan menambah aset',
    title_en: 'Browsing and adding assets',
    sort_order: 1,
    locked: true,
    ...over
  }
}

function guideModule(over: Partial<GuideModule> = {}): GuideModule {
  return {
    id: 'm1',
    slug: 'katalog-aset',
    icon: 'i-lucide-package',
    sort_order: 1,
    status: 'published',
    title_id: 'Katalog Aset',
    title_en: 'Asset Catalogue',
    body_id: 'Telusuri dan kelola seluruh aset dalam lingkup kantor Anda.',
    body_en: 'Browse and manage every asset within your office scope.',
    steps: [],
    attachments: [],
    published_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z',
    ...over
  }
}

function respondWith(modules: GuideModule[]) {
  handler = () => ({ data: modules })
}

enableAutoUnmount(afterEach)

beforeEach(() => {
  requests.length = 0
  useAuthStore().clear()
  respondWith([])
  blobHandler = () => Promise.resolve(new Blob(['%PDF-1.7'], { type: 'application/pdf' }))
  vi.stubGlobal('URL', Object.assign(URL, {
    createObjectURL: vi.fn(() => 'blob:guide-preview'),
    revokeObjectURL: vi.fn()
  }))
})

afterEach(() => {
  vi.unstubAllGlobals()
})

function signIn(permissions: string[] = []) {
  useAuthStore().setSession(
    'tok',
    { id: 'u1', name: 'Uji Coba', email: 'uji@inventra.local', role_id: 'r1', role_name: 'staf', office_id: null },
    permissions
  )
}

describe('guide page — load states', () => {
  it('shows skeleton cards while the modules are still in flight', async () => {
    const deferred: { release: () => void } = { release: () => {} }
    handler = () => new Promise((resolve) => {
      deferred.release = () => resolve({ data: [] })
    })
    const wrapper = await mountSuspended(GuidePage)
    expect(wrapper.findAllComponents({ name: 'USkeleton' }).length).toBeGreaterThan(0)
    deferred.release()
    await flushPromises()
  })

  it('renders the empty state when nothing is published yet', async () => {
    respondWith([])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).toContain('Panduan belum tersedia')
  })

  it('renders the error state when the fetch fails, and retries on demand', async () => {
    handler = () => {
      throw new Error('boom')
    }
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).toContain('Panduan gagal dimuat')

    respondWith([guideModule()])
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(wrapper.html()).toContain('Katalog Aset')
    expect(wrapper.html()).not.toContain('Panduan gagal dimuat')
  })

  it('renders module title, body, and numbered steps in order', async () => {
    respondWith([guideModule({
      steps: [
        { text_id: 'Langkah pertama.', text_en: null },
        { text_id: 'Langkah kedua.', text_en: null }
      ]
    })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    const items = wrapper.findAll('ol li')
    expect(items).toHaveLength(2)
    expect(items[0]!.text()).toContain('Langkah pertama.')
    expect(items[1]!.text()).toContain('Langkah kedua.')
    expect(wrapper.html()).toContain('Telusuri dan kelola seluruh aset')
  })
})

describe('guide page — who asks for what', () => {
  it('a guest asks for published modules only', async () => {
    respondWith([guideModule()])
    await mountSuspended(GuidePage)
    await flushPromises()
    expect(requests).toContain('/guide/modules')
    expect(requests.some(p => p.includes('status=all'))).toBe(false)
  })

  it('a signed-in reader without guide.manage also asks for published only', async () => {
    signIn(['asset.view'])
    respondWith([guideModule()])
    await mountSuspended(GuidePage)
    await flushPromises()
    expect(requests.some(p => p.includes('status=all'))).toBe(false)
  })

  it('an author asks for drafts too and sees them badged', async () => {
    signIn(['guide.manage'])
    respondWith([guideModule({ status: 'draft' })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(requests).toContain('/guide/modules?status=all')
    expect(wrapper.html()).toContain('Draf')
  })
})

describe('guide page — locked media for a guest', () => {
  it('renders a locked video card with a sign-in link and no player', async () => {
    respondWith([guideModule({ attachments: [attachment({ locked: true })] })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    const html = wrapper.html()
    expect(html).toContain('Video panduan terkunci')
    expect(html).toContain('Lampiran hanya terbuka bagi pengguna yang sudah masuk')
    // Nothing playable, and above all no embed URL to lift a video id out of.
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(html).not.toContain('youtube-nocookie.com/embed')
  })

  it('renders a locked document card without filename or size', async () => {
    respondWith([guideModule({
      attachments: [attachment({ kind: 'document', locked: true, title_id: 'Ringkasan registrasi' })]
    })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    const html = wrapper.html()
    expect(html).toContain('Dokumen panduan terkunci')
    expect(html).toContain('Ringkasan registrasi')
    expect(html).not.toContain('.pdf')
  })

  it('still shows the module text around a locked attachment', async () => {
    respondWith([guideModule({ attachments: [attachment()] })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).toContain('Telusuri dan kelola seluruh aset')
  })
})

describe('guide page — unlocked media for a reader', () => {
  // AC16: opening the guide must reach NO YouTube domain — not the embed, and
  // not i.ytimg.com either. The idle facade therefore carries no image at all,
  // matching docs/design/Panduan Media.dc.html.
  it('renders the facade without touching YouTube, and embeds only after play', async () => {
    signIn(['asset.view'])
    respondWith([guideModule({
      attachments: [attachment({ locked: false, youtube_id: 'dQw4w9WgXcQ' })]
    })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()

    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('img').exists()).toBe(false)
    // Nothing anywhere in the markup points at a YouTube host yet.
    expect(wrapper.html()).not.toContain('ytimg.com')
    expect(wrapper.html()).not.toContain('youtube-nocookie.com/embed')

    const play = wrapper.find('button[aria-label*="Putar video"]')
    expect(play.exists()).toBe(true)
    await play.trigger('click')

    // Pressing play probes the thumbnail first; that is the earliest a YouTube
    // domain is contacted, and the embed waits for the probe to succeed.
    const probe = wrapper.find('[data-testid="guide-video-probing"] img')
    expect(probe.attributes('src')).toContain('dQw4w9WgXcQ')
    expect(wrapper.find('iframe').exists()).toBe(false)

    await probe.trigger('load')
    await flushPromises()

    const frame = wrapper.find('iframe')
    expect(frame.exists()).toBe(true)
    expect(frame.attributes('src')).toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0')
  })

  it('falls back to the broken-video card when the probe 404s, without embedding', async () => {
    signIn(['asset.view'])
    respondWith([guideModule({
      attachments: [attachment({ locked: false, youtube_id: 'dQw4w9WgXcQ' })]
    })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    await wrapper.find('button[aria-label*="Putar video"]').trigger('click')
    await wrapper.find('[data-testid="guide-video-probing"] img').trigger('error')
    await flushPromises()

    expect(wrapper.html()).toContain('Video tidak dapat diputar')
    expect(wrapper.find('iframe').exists()).toBe(false)
    // The rest of the module keeps rendering: the failure is contained (AC17).
    expect(wrapper.html()).toContain('Telusuri dan kelola seluruh aset')
  })

  it('retries playback from the broken card rather than only dismissing it', async () => {
    signIn(['asset.view'])
    respondWith([guideModule({
      attachments: [attachment({ locked: false, youtube_id: 'dQw4w9WgXcQ' })]
    })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    await wrapper.find('button[aria-label*="Putar video"]').trigger('click')
    await wrapper.find('[data-testid="guide-video-probing"] img').trigger('error')
    await flushPromises()

    const retry = wrapper.findAll('button').find(b => b.text().includes('Muat ulang pemutar'))!
    await retry.trigger('click')
    // A video that came back plays without a page reload.
    await wrapper.find('[data-testid="guide-video-probing"] img').trigger('load')
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(true)
  })

  it('shows filename and size for a document, and fetches the bytes to preview', async () => {
    signIn(['asset.view'])
    respondWith([guideModule({
      attachments: [attachment({
        id: 'doc1',
        kind: 'document',
        locked: false,
        original_filename: 'panduan-registrasi-aset.pdf',
        size_bytes: 1_400_000
      })]
    })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).toContain('panduan-registrasi-aset.pdf')
    expect(wrapper.html()).toMatch(/1,3 MB|1.3 MB/)

    const preview = wrapper.findAll('button').find(b => b.text().includes('Pratinjau'))!
    await preview.trigger('click')
    await flushPromises()

    // Authenticated endpoint: the bytes are fetched and handed to a sandboxed
    // iframe as an object URL, never used as a bare src.
    expect(requests).toContain('/guide/attachments/doc1/content')
    const frame = wrapper.find('iframe')
    expect(frame.attributes('src')).toBe('blob:guide-preview')
    expect(frame.attributes('sandbox')).toBe('')
  })

  it('degrades to a preview error instead of breaking the page when the file is gone', async () => {
    signIn(['asset.view'])
    blobHandler = () => Promise.reject(new Error('404'))
    respondWith([guideModule({
      attachments: [attachment({ id: 'doc1', kind: 'document', locked: false, original_filename: 'hilang.pdf' })]
    })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    const preview = wrapper.findAll('button').find(b => b.text().includes('Pratinjau'))!
    await preview.trigger('click')
    await flushPromises()
    expect(wrapper.html()).toContain('Berkas tidak dapat dibuka')
    // The rest of the module is untouched.
    expect(wrapper.html()).toContain('Katalog Aset')
  })
})

describe('guide page — language resolution', () => {
  it('renders the Indonesian text on the default locale', async () => {
    respondWith([guideModule()])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).toContain('Katalog Aset')
    expect(wrapper.html()).not.toContain('Asset Catalogue')
  })

  it('falls back to Indonesian for a module with no English title', async () => {
    respondWith([guideModule({ title_id: 'Dashboard', title_en: null, body_en: null })])
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).toContain('Dashboard')
  })
})
