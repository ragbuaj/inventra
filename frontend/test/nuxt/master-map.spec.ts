// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { useAuthStore } from '~/stores/auth'
import MapPage from '~/pages/master/map.vue'

/**
 * Stub the client-only OfficeMap (Leaflet) so it never loads in JSDOM.
 * The stub renders a placeholder div so the rest of the page renders normally.
 */
const stubs = {
  OfficeMap: { template: '<div class="office-map-stub" />' }
}

// Grant a superadmin session so the `can` middleware (masterdata.office.manage) passes.
function admin() {
  useAuthStore().setSession(
    't',
    { id: '1', name: 'A', email: 'a@e.com', role_id: 'r', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => {
  useAuthStore().clear()
  admin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(MapPage, { global: { stubs } })
  // Wait for fakeLatency (500ms) + one tick
  await new Promise(r => setTimeout(r, 600))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Peta Lokasi (Office Map) page', () => {
  it('renders office rows after loading', async () => {
    const wrapper = await mountAndWait()
    const html = wrapper.html()
    // All 9 mock offices should be visible
    expect(html).toContain('Kantor Pusat')
    expect(html).toContain('Cabang Jakarta Selatan')
    expect(html).toContain('Cabang Bekasi')
    // Summary strip is rendered
    expect(html).toContain('kantor')
  })

  it('filters offices by search query', async () => {
    const wrapper = await mountAndWait()
    const input = wrapper.find('input')
    await input.setValue('Bekasi')
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Cabang Bekasi')
    // Offices not matching should be absent from the list area
    expect(html).not.toContain('Kantor Pusat')
  })

  it('shows the detail card when an office row is clicked', async () => {
    const wrapper = await mountAndWait()
    // Find the office-row button for Kantor Pusat
    const buttons = wrapper.findAll('button')
    const pusatBtn = buttons.find(b => b.text().includes('Kantor Pusat'))
    expect(pusatBtn).toBeDefined()
    await pusatBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    // Detail card shows name, kode, and asset count
    expect(html).toContain('Kantor Pusat')
    expect(html).toContain('PST')
    expect(html).toContain('94')
    // Action buttons present
    expect(html).toContain('Lihat Kantor')
    expect(html).toContain('Buka di Maps')
  })

  it('shows empty state when no offices match the filter', async () => {
    const wrapper = await mountAndWait()
    const input = wrapper.find('input')
    await input.setValue('xxxxxxnotfound')
    await wrapper.vm.$nextTick()
    const html = wrapper.html()
    expect(html).toContain('Tidak ada kantor')
  })
})
