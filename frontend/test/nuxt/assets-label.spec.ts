// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { assetStore } from '~/mock/assets'
import LabelPage from '~/pages/assets/label.vue'

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

describe('Asset Label/Barcode page', () => {
  it('renders the select panel + layout controls and an empty preview by default', async () => {
    const wrapper = await mountSuspended(LabelPage, { route: '/assets/label' })
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Label & Barcode')
    expect(text).toContain('Pilih Aset')
    expect(text).toContain('Tata Letak') // layout
    expect(text).toContain('Keduanya') // Both mode
    expect(text).toContain('Laptop Dell Latitude 5440') // an asset in the select list
    expect(text).toContain('Belum ada aset dipilih') // empty preview
  })

  it('pre-selects assets from the ?tags query and renders the label preview', async () => {
    const wrapper = await mountSuspended(LabelPage, { route: '/assets/label?tags=JKT01-ELK-2026-00001' })
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).not.toContain('Belum ada aset dipilih')
    expect(text).toContain('Label Tunggal') // single label
    expect(text).toContain('1 label')
    // The label renders the tag and an SVG-free barcode/QR (generated divs).
    expect(wrapper.html()).toContain('JKT01-ELK-2026-00001')
  })

  it('selecting all (filtered) fills the batch preview', async () => {
    const wrapper = await mountSuspended(LabelPage, { route: '/assets/label' })
    await wrapper.vm.$nextTick()
    // The select-all checkbox is the first checkbox button on the page.
    const checkbox = wrapper.find('button[role="checkbox"]')
    expect(checkbox.exists()).toBe(true)
    await checkbox.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Lembar Batch') // batch sheet
  })
})
