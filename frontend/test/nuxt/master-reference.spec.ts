// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import ReferencePage from '~/pages/master/reference.vue'

describe('Master Data Referensi page', () => {
  it('renders the title and the first resource rows after load', async () => {
    const wrapper = await mountSuspended(ReferencePage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Master Data Referensi')
    // default resource is the first descriptor (office-types) → seeded "office-types contoh"
    expect(html).toContain('office-types contoh')
  })
})
