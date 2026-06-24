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
    expect(html).toContain('Pegawai')
    expect(html).toContain('Andi Pratama')
    expect(html).toContain('Bunga Lestari')
  })

  it('renders translated status labels', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Aktif')
    expect(wrapper.html()).toContain('Nonaktif')
  })

  it('renders Departemen column header', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Departemen')
  })

  it('renders Email / Telepon column header', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Email / Telepon')
  })

  it('renders avatar initials for seeded employees', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    // Andi Pratama → AP, Bunga Lestari → BL
    expect(html).toContain('AP')
    expect(html).toContain('BL')
  })

  it('renders jabatan as a badge (neutral subtle)', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    // jabatan values from seed
    const html = wrapper.html()
    expect(html).toContain('Kepala Kantor')
    expect(html).toContain('Staf')
  })

  it('renders the 4 filter dropdowns', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Semua Kantor')
    expect(html).toContain('Semua Departemen')
    expect(html).toContain('Semua Jabatan')
    expect(html).toContain('Semua Status')
  })

  it('shows email and telepon values in the table', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('andi.pratama@inventra.go.id')
    expect(html).toContain('0812-1111-2222')
  })

  it('opens slideover form and formOpen becomes true', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await wrapper.vm.$nextTick()
    // Trigger openCreate via vm
    const vm = wrapper.vm as unknown as { formOpen: boolean, openCreate: () => void }
    vm.openCreate()
    await wrapper.vm.$nextTick()
    // Verify formOpen state is true
    expect(vm.formOpen).toBe(true)
    // USlideover content renders via Teleport; check document.body for form field labels
    const bodyHtml = document.body.innerHTML
    expect(bodyHtml).toContain('NIP')
    expect(bodyHtml).toContain('Nama Lengkap')
    expect(bodyHtml).toContain('Departemen')
    expect(bodyHtml).toContain('Jabatan')
    expect(bodyHtml).toContain('Kantor')
  })

  it('dept filter narrows results — selecting Keuangan shows only matching rows', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const vm = wrapper.vm as unknown as { filterDept: string }
    vm.filterDept = 'Keuangan'
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    // Bunga Lestari is the only Keuangan employee in seed
    expect(html).toContain('Bunga Lestari')
    // Andi Pratama (Umum) should not appear when Keuangan filter is set
    expect(html).not.toContain('Andi Pratama')
  })

  it('status filter narrows results — selecting inactive shows only inactive rows', async () => {
    const wrapper = await mountSuspended(EmployeesPage)
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    // Set status filter to inactive via vm (bypassing USelect constraint)
    const vm = wrapper.vm as unknown as { filterStatus: string }
    vm.filterStatus = 'inactive'
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    // Citra Dewi is the only inactive employee in seed
    expect(html).toContain('Citra Dewi')
    // Active employees should not appear when inactive filter is set
    expect(html).not.toContain('Andi Pratama')
  })
})
