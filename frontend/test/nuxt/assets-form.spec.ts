// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { assetStore, assetSeed } from '~/mock/assets'
import AssetForm from '~/components/asset/AssetForm.vue'

enableAutoUnmount(afterEach)

beforeEach(() => assetStore.reset())

describe('AssetForm — create mode', () => {
  it('renders the title, banner, all sections and required fields', async () => {
    const wrapper = await mountSuspended(AssetForm, { props: { mode: 'new' } })
    const text = wrapper.text()
    expect(text).toContain('Tambah Aset')
    expect(text).toContain('maker-checker') // approval banner
    expect(text).toContain('Identitas')
    expect(text).toContain('Penempatan')
    expect(text).toContain('Pembelian')
    expect(text).toContain('Depresiasi')
    expect(text).toContain('Lampiran')
    expect(text).toContain('Nama Aset')
  })

  it('blocks save and shows required errors when empty', async () => {
    const wrapper = await mountSuspended(AssetForm, { props: { mode: 'new' } })
    const before = assetStore.all().length
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save).toBeDefined()
    await save!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Nama aset wajib diisi')
    expect(wrapper.text()).toContain('Kategori wajib dipilih')
    expect(assetStore.all()).toHaveLength(before) // nothing created
  })
})

describe('AssetForm — edit mode', () => {
  it('renders the edit title and prefills the asset name + tag', async () => {
    const asset = assetSeed[0]
    const wrapper = await mountSuspended(AssetForm, { props: { mode: 'edit', initial: asset } })
    const html = wrapper.html()
    expect(wrapper.text()).toContain('Edit Aset')
    // name prefilled into the input, tag shown in the (disabled) code field
    expect(html).toContain(asset.nama)
    expect(html).toContain(asset.tag)
  })
})
