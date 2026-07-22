// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useAuthStore } from '~/stores/auth'
// eslint-disable-next-line import/first
import MapPage from '~/pages/master/map.vue'

const OFFICES = [
  { id: 'o1', name: 'Kantor Pusat', code: 'PST', office_type_name: 'Kantor Pusat', tier: 'pusat', province_name: 'DKI Jakarta', city_name: 'Jakarta Pusat', address: 'Jl. Merdeka 1', asset_count: 12, latitude: -6.1754, longitude: 106.8272 },
  { id: 'o2', name: 'Cabang Bekasi', code: 'BKS01', office_type_name: 'Kantor Cabang', tier: 'office', province_name: 'Jawa Barat', city_name: 'Bekasi', address: 'Jl. A. Yani 1', asset_count: 3, latitude: -6.2383, longitude: 106.9756 },
  { id: 'o3', name: 'Cabang Tanpa Koordinat', code: 'NOC', office_type_name: 'Kantor Cabang', tier: 'office', province_name: 'Banten', city_name: 'Tangerang', address: 'Jl. X', asset_count: 0, latitude: null, longitude: null }
]

beforeEach(() => {
  request.mockReset()
  useAuthStore().setSession('t', { id: 'u', name: 'Admin', email: 'a@x.id', role_id: 'r', role_name: '', office_id: null }, ['*'])
})

async function mountMap() {
  const wrapper = await mountSuspended(MapPage, { global: { stubs: { OfficeMap: true } } })
  await new Promise(r => setTimeout(r, 50))
  return wrapper
}

describe('Peta Lokasi page', () => {
  it('renders office rows with resolved names + tier labels', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    const text = wrapper.text()
    expect(request).toHaveBeenCalledWith('/offices/map')
    expect(text).toContain('Kantor Pusat')
    expect(text).toContain('Cabang Bekasi')
    expect(text).toContain('Jakarta Pusat')
    expect(text).toContain('Bekasi')
  })

  it('filters the list by province', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    ;(wrapper.vm as unknown as { fProv: string }).fProv = 'Jawa Barat'
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Cabang Bekasi')
    expect(text).not.toContain('Kantor Pusat')
  })

  it('selecting an office shows its detail (address + asset count)', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    ;(wrapper.vm as unknown as { selId: string | null }).selId = 'o1'
    await wrapper.vm.$nextTick()
    const card = wrapper.find('[data-testid="office-detail-card"]')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('Jl. Merdeka 1')
    expect(card.text()).toContain('12')
  })

  it('detail card sits above the Leaflet map (z-index)', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    ;(wrapper.vm as unknown as { selId: string | null }).selId = 'o1'
    await wrapper.vm.$nextTick()
    const card = wrapper.find('[data-testid="office-detail-card"]')
    expect(card.exists()).toBe(true)
    expect(card.classes()).toContain('z-[1100]')
  })

  it('the map area is an isolated stacking context so its z-[1000]/z-[1100] controls stay below the mobile drawer', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    // The zoom/reset controls + detail card live inside the map area; `isolate`
    // confines their high z-index so the sidebar drawer (z-50) can still overlay them.
    ;(wrapper.vm as unknown as { selId: string | null }).selId = 'o1'
    await wrapper.vm.$nextTick()
    const isolated = wrapper.find('.isolate')
    expect(isolated.exists()).toBe(true)
    // the detail card (z-[1100]) must be a descendant of the isolated map area
    expect(isolated.find('[data-testid="office-detail-card"]').exists()).toBe(true)
  })

  it('the "Lihat Kantor" action deep-links to the office detail (query carries the id)', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    ;(wrapper.vm as unknown as { selId: string | null }).selId = 'o2'
    await wrapper.vm.$nextTick()
    const card = wrapper.find('[data-testid="office-detail-card"]')
    const viewLink = card.findAll('a').find(a => (a.attributes('href') ?? '').includes('/master/offices'))
    expect(viewLink).toBeTruthy()
    expect(viewLink!.attributes('href')).toContain('office=o2')
  })

  it('shows the error state + retry on load failure, then recovers', async () => {
    request.mockRejectedValueOnce(new Error('500'))
    const wrapper = await mountMap()
    expect(wrapper.text()).toContain('Gagal memuat peta kantor.')
    request.mockResolvedValueOnce({ data: OFFICES })
    await wrapper.find('[data-testid="map-retry"]').trigger('click')
    await new Promise(r => setTimeout(r, 50))
    expect(wrapper.text()).toContain('Kantor Pusat')
  })

  it('renders the empty state when no offices', async () => {
    request.mockResolvedValueOnce({ data: [] })
    const wrapper = await mountMap()
    expect(wrapper.text()).toContain('Tidak ada kantor')
  })
})
