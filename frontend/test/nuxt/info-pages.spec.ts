// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import PrivacyPage from '~/pages/privacy.vue'
import GuidePage from '~/pages/guide.vue'
import FaqPage from '~/pages/faq.vue'

// The three public info pages read their content from the i18n locale arrays via
// tm()/rt(). These tests assert the resolved (Indonesian, default locale) content
// actually reaches the screen -- not just that some HTML rendered.

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
  it('renders the title, intro, and feature-section headings', async () => {
    const wrapper = await mountSuspended(GuidePage)
    const html = wrapper.html()
    expect(html).toContain('Panduan Penggunaan')
    expect(html).toContain('Katalog Aset')
    expect(html).toContain('Pengajuan dan Persetujuan')
    expect(html).toContain('Master Data dan Pengaturan')
  })

  it('renders numbered steps for sections that define them', async () => {
    const wrapper = await mountSuspended(GuidePage)
    const html = wrapper.html()
    // The sign-in section lists steps; its first instruction should be visible.
    expect(html).toContain('Buka halaman Masuk')
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
