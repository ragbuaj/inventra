// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import IndexPage from '~/pages/index.vue'

/** Mount the dashboard and wait past the mock 700ms latency so the loaded state renders. */
async function mountLoaded() {
  const wrapper = await mountSuspended(IndexPage)
  await new Promise(r => setTimeout(r, 850))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Dashboard page — loading', () => {
  it('shows the header immediately but no KPI figures while loading', async () => {
    const wrapper = await mountSuspended(IndexPage)
    // Header (scope name) is always visible; the period/scope controls too.
    expect(wrapper.text()).toContain('Kantor Cabang Jakarta Selatan')
    // KPI/chart figures are still behind the skeleton — the total (96) is absent.
    expect(wrapper.text()).not.toContain('96')
  })
})

describe('Dashboard page — loaded (jaksel default scope)', () => {
  it('renders all six KPI cards with the scope figures', async () => {
    const wrapper = await mountLoaded()
    const text = wrapper.text()
    expect(text).toContain('Total Aset')
    expect(text).toContain('96')
    expect(text).toContain('Nilai Perolehan')
    expect(text).toContain('Rp 3,82 M')
    expect(text).toContain('Total Biaya Maintenance')
    expect(text).toContain('Rp 42,5 Jt')
  })

  it('renders the status donut total and legend', async () => {
    const wrapper = await mountLoaded()
    const text = wrapper.text()
    expect(text).toContain('Aset per Status')
    expect(text).toContain('Tersedia')
    expect(text).toContain('Dipinjam')
  })

  it('renders the category and location bar lists', async () => {
    const wrapper = await mountLoaded()
    const text = wrapper.text()
    expect(text).toContain('Aset per Kategori')
    expect(text).toContain('Elektronik')
    expect(text).toContain('41')
    expect(text).toContain('Aset per Lokasi / Kantor')
    expect(text).toContain('Gudang Aset')
  })

  it('renders the maintenance-due panel rows', async () => {
    const wrapper = await mountLoaded()
    const text = wrapper.text()
    expect(text).toContain('Maintenance Jatuh Tempo')
    expect(text).toContain('Toyota Avanza · B 1234 XYZ')
    expect(text).toContain('Besok')
  })

  it('renders the approval queue rows', async () => {
    const wrapper = await mountLoaded()
    const text = wrapper.text()
    expect(text).toContain('Pengajuan Menunggu Approval')
    expect(text).toContain('Peminjaman Proyektor Epson EB-X51')
  })
})

describe('Dashboard page — approval actions', () => {
  it('removes a request from the queue when approved', async () => {
    const wrapper = await mountLoaded()
    expect(wrapper.text()).toContain('Peminjaman Proyektor Epson EB-X51')
    await wrapper.find('[aria-label="approve-a1"]').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).not.toContain('Peminjaman Proyektor Epson EB-X51')
  })

  it('removes a request from the queue when rejected', async () => {
    const wrapper = await mountLoaded()
    await wrapper.find('[aria-label="reject-a2"]').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).not.toContain('Mutasi 3 Laptop ke Outlet Blok M')
  })

  it('shows the all-handled empty state once every request is actioned', async () => {
    const wrapper = await mountLoaded()
    for (const id of ['a1', 'a2', 'a3']) {
      await wrapper.find(`[aria-label="approve-${id}"]`).trigger('click')
      await wrapper.vm.$nextTick()
    }
    expect(wrapper.text()).toContain('Semua pengajuan ditindak')
    expect(wrapper.text()).toContain('Tidak ada yang menunggu persetujuan.')
  })
})
