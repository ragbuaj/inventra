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

  it('active-only filter removes the inactive (Legacy) row from the dataset', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { activeOnly: boolean, orderedRows: { name: string }[] }
    expect(vm.orderedRows.some(r => r.name.includes('Legacy'))).toBe(true)
    vm.activeOnly = true
    await wrapper.vm.$nextTick()
    expect(vm.orderedRows.some(r => r.name.includes('Legacy'))).toBe(false)
  })

  it('paginates with page size 7 (8 seed rows span two pages)', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { orderedRows: unknown[] }
    expect(vm.orderedRows.length).toBe(8)
    // Page 1 renders the first 7; the 8th (Legacy) is on page 2, not in page-1 HTML.
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

  it('search by code narrows the dataset', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as { search: string, orderedRows: { name: string }[] }
    vm.search = 'KEN'
    await wrapper.vm.$nextTick()
    expect(vm.orderedRows.some(r => r.name.includes('Kendaraan'))).toBe(true)
    expect(vm.orderedRows.some(r => r.name.includes('Komputer'))).toBe(false)
  })

  it('parentOptions excludes the editing category and its descendants', async () => {
    const wrapper = await mountLoaded()
    const vm = wrapper.vm as unknown as {
      openEdit: (r: unknown) => void
      parentOptions: { value: string }[]
      orderedRows: { id: string, name: string }[]
    }
    const parent = vm.orderedRows.find(r => r.name === 'Perangkat IT')
    expect(parent).toBeTruthy()
    vm.openEdit(parent)
    await wrapper.vm.$nextTick()
    const ids = vm.parentOptions.map(o => o.value)
    expect(ids).not.toContain('c-it')
    expect(ids).not.toContain('c-laptop')
  })
})
