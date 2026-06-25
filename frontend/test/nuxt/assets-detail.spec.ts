// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { assetStore } from '~/mock/assets'
import DetailPage from '~/pages/assets/[tag].vue'

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => {
  assetStore.reset()
  grantAdmin()
})

async function mountTag(tag: string) {
  const wrapper = await mountSuspended(DetailPage, { route: `/assets/${tag}` })
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Asset Detail page', () => {
  it('renders the asset header, key info and Info tab fields', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const text = wrapper.text()
    expect(text).toContain('Laptop Dell Latitude 5440')
    expect(text).toContain('JKT01-ELK-2026-00001')
    expect(text).toContain('Tersedia')
    expect(text).toContain('Informasi Utama')
    expect(text).toContain('Identitas')
    expect(text).toContain('Nomor Seri')
    expect(text).toContain('Rp 18.500.000') // buy price (valuation)
  })

  it('switches to the Maintenance tab', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const maintTab = wrapper.findAll('button').find(b => b.text().trim() === 'Riwayat Maintenance')
    expect(maintTab).toBeDefined()
    await maintTab!.trigger('click')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('PT Sinar Komputindo')
    expect(text).toContain('Preventive')
  })

  it('switches to the Depreciation tab with a current-year row', async () => {
    const wrapper = await mountTag('JKT01-ELK-2026-00001')
    const deprTab = wrapper.findAll('button').find(b => b.text().trim() === 'Jadwal Depresiasi')
    await deprTab!.trigger('click')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Garis Lurus') // method
    expect(text).toContain('Berjalan') // current-year badge
    expect(text).toContain('2026')
  })

  it('shows a not-found state for an unknown tag', async () => {
    const wrapper = await mountTag('NOPE-0000')
    expect(wrapper.text()).toContain('Aset tidak ditemukan')
  })
})
