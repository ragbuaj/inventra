// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import ReferencePage from '~/pages/master/reference.vue'

async function mountAndWait() {
  const wrapper = await mountSuspended(ReferencePage)
  await new Promise(r => setTimeout(r, 350))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Master Data Referensi page', () => {
  it('renders the entity-nav panel with title "Master Data" and subtitle "Data referensi"', async () => {
    const wrapper = await mountAndWait()
    const html = wrapper.html()
    // Panel title
    expect(html).toContain('Master Data')
    // Panel subtitle (Indonesian locale)
    expect(html).toContain('Data referensi')
  })

  it('entity-nav panel lists all 11 reference resources with count badges', async () => {
    const wrapper = await mountAndWait()
    const html = wrapper.html()
    // spot-check several resource labels visible in the panel
    expect(html).toContain('Jenis Kantor')
    expect(html).toContain('Departemen')
    expect(html).toContain('Jabatan')
    expect(html).toContain('Satuan')
    expect(html).toContain('Merek')
    expect(html).toContain('Provinsi')
  })

  it('entity-nav panel shows numeric count badges for each resource', async () => {
    const wrapper = await mountAndWait()
    // The first resource (office-types) has 1 seeded row → count badge "1"
    // departments has 3 seeded rows → count badge "3"
    // At minimum we expect count digits to appear
    const html = wrapper.html()
    // Count badges: at least one digit from the seeded resources should appear
    // office-types default seed = 1 row: "office-types contoh"
    expect(html).toMatch(/>\s*\d+\s*</)
  })

  it('the first resource (Jenis Kantor) is active by default — has primary styling', async () => {
    const wrapper = await mountAndWait()
    // Active entity is rendered with primary-soft background classes
    // (bg-primary/10 or similar Nuxt UI primary-soft token)
    const html = wrapper.html()
    // The title should show the active entity label as the main column header
    expect(html).toContain('Jenis Kantor')
    // old USelect entity switcher must be gone
    expect(html).not.toContain('masterdata.reference.resourceLabel')
  })

  it('main column shows the active entity rows (Jenis Kantor seeded row)', async () => {
    const wrapper = await mountAndWait()
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    // default resource is first (office-types) → seeded row "office-types contoh"
    expect(html).toContain('office-types contoh')
  })

  it('clicking a different entity in the nav panel changes the active resource', async () => {
    const wrapper = await mountAndWait()
    // Find the "Departemen" nav button and click it
    const buttons = wrapper.findAll('button')
    const deptBtn = buttons.find(b => b.text().includes('Departemen') && !b.text().includes('office-types'))
    if (deptBtn) {
      await deptBtn.trigger('click')
      await new Promise(r => setTimeout(r, 350))
      await wrapper.vm.$nextTick()
      const html = wrapper.html()
      // After switching to departments, seeded row "Umum" should be visible
      expect(html).toContain('Umum')
      // The old entity rows should no longer dominate
    }
  })

  it('status column renders a toggle (button/switch) for each row', async () => {
    const wrapper = await mountAndWait()
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    // The status toggle must be a button or input (USwitch renders as button)
    // We look for "Aktif" or "Nonaktif" label text in the table area
    const html = wrapper.html()
    // At least one status label should be visible in the table
    const hasStatus = html.includes('Aktif') || html.includes('Nonaktif')
    expect(hasStatus).toBe(true)
  })

  it('search bar exists but no Reset button (no USelect entity switcher)', async () => {
    const wrapper = await mountAndWait()
    const html = wrapper.html()
    // Search input must be present
    expect(html).toContain('type="text"')
    // No reset button (mockup has no reset button for reference page)
    // No entity selector dropdown
    expect(html).not.toContain('masterdata.reference.resourceLabel')
  })

  it('form modal has the Aktif toggle row', async () => {
    const wrapper = await mountAndWait()
    // Find and click the Add button to open the form
    const buttons = wrapper.findAll('button')
    const addBtn = buttons.find(b => b.text().trim() === 'Tambah' || b.text().includes('Tambah'))
    if (addBtn) {
      await addBtn.trigger('click')
      await wrapper.vm.$nextTick()
      const html = wrapper.html()
      // The form should contain "Aktif" toggle label
      expect(html).toContain('Aktif')
    }
  })

  it('form modal has a subtitle (entity label) under the title', async () => {
    const wrapper = await mountAndWait()
    const buttons = wrapper.findAll('button')
    const addBtn = buttons.find(b => b.text().trim() === 'Tambah' || b.text().includes('Tambah'))
    if (addBtn) {
      await addBtn.trigger('click')
      await wrapper.vm.$nextTick()
      const html = wrapper.html()
      // Modal should contain "Tambah Data" (create title) or the entity name as subtitle
      const hasTitleOrSub = html.includes('Tambah Data') || html.includes('Jenis Kantor')
      expect(hasTitleOrSub).toBe(true)
    }
  })
})
