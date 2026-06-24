// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import OfficesPage from '~/pages/master/offices.vue'

describe('Master Data Kantor page', () => {
  it('renders the page title and seeded offices in the tree', async () => {
    const wrapper = await mountSuspended(OfficesPage)
    // wait for fakeLatency-resolved tree()
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Master Data Kantor')
    expect(html).toContain('Kantor Pusat')
    expect(html).toContain('Kanwil Jakarta')
  })

  it('shows the select hint before any node is chosen', async () => {
    const wrapper = await mountSuspended(OfficesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Pilih kantor pada pohon untuk melihat detail')
  })
})
