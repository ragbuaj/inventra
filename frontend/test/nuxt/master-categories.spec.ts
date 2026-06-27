// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import CategoriesPage from '~/pages/master/categories.vue'

async function mountLoaded() {
  const wrapper = await mountSuspended(CategoriesPage)
  await new Promise(r => setTimeout(r, 350))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Master Data Kategori Aset page', () => {
  it('renders the title and seeded categories after load', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Kategori Aset')
    // '&' is encoded to '&amp;' in innerHTML; use text() for names with ampersands
    expect(wrapper.text()).toContain('Komputer & Laptop')
    expect(html).toContain('Kendaraan Bermotor')
  })

  it('renders class badges (Berwujud / Takberwujud)', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Berwujud')
    expect(html).toContain('Takberwujud')
  })

  it('renders translated method and fiscal-group labels', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Garis Lurus')
    expect(html).toContain('Saldo Menurun')
    expect(html).toContain('Bangunan Permanen')
  })

  it('renders the GL account codes', async () => {
    const wrapper = await mountLoaded()
    expect(wrapper.html()).toContain('1.2.3.01')
  })

  it('renders the filter controls', async () => {
    const wrapper = await mountLoaded()
    const html = wrapper.html()
    expect(html).toContain('Semua Kelas')
    expect(html).toContain('Semua Golongan')
    expect(html).toContain('Hanya aktif')
  })

  it('class filter narrows results — Takberwujud shows only intangible rows', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { filterClass: string }
    vm.filterClass = 'intangible'
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Software / Lisensi')
    expect(html).not.toContain('Kendaraan Bermotor')
  })

  it('active-only filter hides the inactive (Legacy) row', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { activeOnly: boolean }
    expect(wrapper.html()).toContain('Peralatan Jaringan (Legacy)')
    vm.activeOnly = true
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).not.toContain('Peralatan Jaringan (Legacy)')
  })

  it('opens the slideover with form labels when Add is triggered', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { formOpen: boolean, openCreate: () => void }
    vm.openCreate()
    await wrapper.vm.$nextTick()
    expect(vm.formOpen).toBe(true)
    const body = document.body.innerHTML
    expect(body).toContain('Nama Kategori')
    expect(body).toContain('Golongan / Kelompok Harta')
    expect(body).toContain('Akun GL (COA)')
  })
})
