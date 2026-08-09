// @vitest-environment nuxt
import { describe, it, expect, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import type { GuideModule } from '~/types'

// FAQ and Privacy still read their content from the i18n locale arrays via
// tm()/rt(); these tests assert the resolved (Indonesian, default locale)
// content actually reaches the screen -- not just that some HTML rendered.
//
// The guide page no longer does: its content moved to the database, so the
// section below drives it through a stubbed API instead of the locale file.
// Everything beyond "the fetched content renders" (loading, empty, error,
// locked media, locale switching) lives in guide-page.spec.ts.

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string) => {
      if (path.startsWith('/guide/modules')) return Promise.resolve({ data: MODULES })
      throw new Error(`Unhandled request: ${path}`)
    },
    requestBlob: () => Promise.reject(new Error('not used here'))
  })
}))

// eslint-disable-next-line import/first
import PrivacyPage from '~/pages/privacy.vue'
// eslint-disable-next-line import/first
import GuidePage from '~/pages/guide.vue'
// eslint-disable-next-line import/first
import FaqPage from '~/pages/faq.vue'

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
    body_en: null,
    steps: [],
    attachments: [],
    published_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z',
    ...over
  }
}

const MODULES: GuideModule[] = [
  guideModule({
    id: 'm0',
    slug: 'masuk-dan-keamanan-akun',
    title_id: 'Masuk dan Keamanan Akun',
    title_en: 'Signing In and Account Security',
    body_id: 'Masuk dengan email kantor dan kata sandi Anda.',
    steps: [
      { text_id: 'Buka halaman Masuk, isi email dan kata sandi, lalu tekan Masuk.', text_en: null },
      { text_id: 'Ganti kata sandi secara berkala melalui menu Profil Akun.', text_en: null }
    ]
  }),
  guideModule(),
  guideModule({
    id: 'm2',
    slug: 'pengajuan-dan-persetujuan',
    title_id: 'Pengajuan dan Persetujuan',
    title_en: null,
    body_id: 'Inventra menerapkan mekanisme maker-checker berjenjang.'
  })
]

describe('privacy page', () => {
  it('renders the title, subtitle, and last-updated line', async () => {
    const wrapper = await mountSuspended(PrivacyPage)
    const html = wrapper.html()
    expect(html).toContain('Kebijakan Privasi')
    expect(html).toContain('Bagaimana Inventra mengumpulkan')
    expect(html).toContain('Terakhir diperbarui')
  })

  it('renders every policy section heading', async () => {
    const wrapper = await mountSuspended(PrivacyPage)
    const html = wrapper.html()
    for (const heading of [
      '1. Data yang Kami Kumpulkan',
      '4. Penyimpanan dan Keamanan',
      '7. Hak Anda',
      '9. Kontak'
    ]) {
      expect(html).toContain(heading)
    }
  })

  it('renders section bullet points (e.g. the bcrypt hashing note)', async () => {
    const wrapper = await mountSuspended(PrivacyPage)
    expect(wrapper.html()).toContain('bcrypt')
  })

  it('builds a table of contents whose anchors match the section ids', async () => {
    const wrapper = await mountSuspended(PrivacyPage)
    const anchors = wrapper.findAll('a[href^="#privacy-sec-"]')
    // 9 sections -> 9 TOC entries, and each target section exists in the DOM.
    expect(anchors).toHaveLength(9)
    const firstHref = anchors[0]!.attributes('href')!.slice(1)
    expect(wrapper.find(`#${firstHref}`).exists()).toBe(true)
  })
})

describe('guide page', () => {
  it('renders the page chrome from i18n and the modules from the API', async () => {
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    const html = wrapper.html()
    // Title and intro still come from the locale file ...
    expect(html).toContain('Panduan Penggunaan')
    expect(html).toContain('Panduan ini membantu Anda mulai menggunakan Inventra')
    // ... while every module heading comes from the fetched rows.
    expect(html).toContain('Masuk dan Keamanan Akun')
    expect(html).toContain('Katalog Aset')
    expect(html).toContain('Pengajuan dan Persetujuan')
  })

  it('renders numbered steps for the modules that define them', async () => {
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    const html = wrapper.html()
    expect(html).toContain('Buka halaman Masuk')
    expect(html).toContain('Ganti kata sandi secara berkala')
    // Two steps on that module, and the list is numbered in order.
    const items = wrapper.findAll('ol li')
    expect(items).toHaveLength(2)
    expect(items[0]!.text()).toContain('1')
  })

  it('renders each module body', async () => {
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).toContain('Telusuri dan kelola seluruh aset')
  })

  it('shows no content from the retired i18n section list', async () => {
    // guidePage.sections is still in the locale file so the release can be
    // rolled back by reverting code alone — but the page must no longer read it.
    // "Master Data dan Pengaturan" exists only there, never in the stub above.
    const wrapper = await mountSuspended(GuidePage)
    await flushPromises()
    expect(wrapper.html()).not.toContain('Master Data dan Pengaturan')
  })
})

describe('faq page', () => {
  it('renders the title and all questions by default', async () => {
    const wrapper = await mountSuspended(FaqPage)
    const html = wrapper.html()
    expect(html).toContain('Pertanyaan yang Sering Diajukan')
    expect(html).toContain('Apa itu Inventra?')
    expect(html).toContain('Bagaimana cara mengganti kata sandi?')
    // Category headings render as section labels.
    expect(html).toContain('Umum')
    expect(html).toContain('Akun')
  })

  it('filters questions by the search box (case-insensitive)', async () => {
    const wrapper = await mountSuspended(FaqPage)
    await wrapper.find('input').setValue('BARCODE')
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('mencetak label barcode')
    // A non-matching question drops out of the list.
    expect(html).not.toContain('Apa itu Inventra?')
  })

  it('shows the empty state when nothing matches', async () => {
    const wrapper = await mountSuspended(FaqPage)
    await wrapper.find('input').setValue('zzz-tidak-ada-kecocokan-xyz')
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Tidak ada pertanyaan yang cocok')
    expect(html).not.toContain('Apa itu Inventra?')
  })
})
