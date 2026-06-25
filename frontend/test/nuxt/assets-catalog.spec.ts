// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { useConfirm } from '~/composables/useConfirm'
import { assetStore } from '~/mock/assets'
import CatalogPage from '~/pages/assets/index.vue'

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

async function mountAndWait() {
  const wrapper = await mountSuspended(CatalogPage)
  await new Promise(r => setTimeout(r, 800))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Asset Catalog page', () => {
  it('renders title, asset rows, tags, status and prices', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Katalog Aset')
    expect(text).toContain('Laptop Dell Latitude 5440')
    expect(text).toContain('JKT01-ELK-2026-00001')
    expect(text).toContain('Tersedia') // a status label
    expect(text).toContain('Rp 18.500.000') // a price
  })

  it('filters by search', async () => {
    const wrapper = await mountAndWait()
    await wrapper.find('input[type="text"]').setValue('Toyota')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Toyota Avanza 1.5 G')
    expect(wrapper.text()).not.toContain('AC Daikin FTKC50')
  })

  it('paginates 20 per page', async () => {
    const wrapper = await mountAndWait()
    // Default sort by tag asc → "Toyota Hiace Commuter" (a KEN- tag) is on page 2.
    expect(wrapper.text()).not.toContain('Toyota Hiace Commuter')
    const page2 = wrapper.findAll('button').find(b => b.text().trim() === '2')
    expect(page2).toBeDefined()
    await page2!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Toyota Hiace Commuter')
  })

  it('selecting all shows the bulk bar with a count', async () => {
    const wrapper = await mountAndWait()
    // UCheckbox renders a Reka button[role=checkbox]; the only button in <thead> is the select-all.
    const headerCheckbox = wrapper.find('thead button')
    expect(headerCheckbox.exists()).toBe(true)
    await headerCheckbox.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('20 dipilih')
  })

  it('switches to grid view', async () => {
    const wrapper = await mountAndWait()
    const gridBtn = wrapper.find('button[aria-label="Tampilan grid"]')
    expect(gridBtn.exists()).toBe(true)
    await gridBtn.trigger('click')
    await wrapper.vm.$nextTick()
    // Grid still shows asset names but no table header row.
    expect(wrapper.text()).toContain('Laptop Dell Latitude 5440')
    expect(wrapper.find('thead').exists()).toBe(false)
  })

  it('delete asks for confirmation naming the asset tag', async () => {
    const { state } = useConfirm()
    const wrapper = await mountAndWait()
    const delBtn = wrapper.findAll('button').find(b => b.attributes('aria-label') === 'Hapus')
    expect(delBtn).toBeDefined()
    await delBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(state.value.open).toBe(true)
    expect(state.value.description).toContain('JKT01-')
  })

  it('shows the empty state when nothing matches', async () => {
    const wrapper = await mountAndWait()
    await wrapper.find('input[type="text"]').setValue('zzz-no-asset')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Tidak ada aset yang cocok')
  })
})
