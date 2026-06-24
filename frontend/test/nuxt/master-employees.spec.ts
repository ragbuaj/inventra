// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import EmployeesPage from '~/pages/master/employees.vue'

describe('Master Data Pegawai page', () => {
  it('renders the title and seeded employees after load', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Master Data Pegawai')
    expect(html).toContain('Andi Pratama')
    expect(html).toContain('Bunga Lestari')
  })

  it('renders translated status labels', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    // active → "Aktif", inactive → "Nonaktif" (id locale)
    expect(wrapper.html()).toContain('Aktif')
    expect(wrapper.html()).toContain('Nonaktif')
  })
})
