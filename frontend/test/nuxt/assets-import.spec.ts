// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { useAuthStore } from '~/stores/auth'
import ImportPage from '~/pages/assets/import.vue'

beforeEach(() => {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
})

const wait = (ms: number) => new Promise(r => setTimeout(r, ms))

describe('Asset Import wizard', () => {
  it('step 1 renders the upload UI with the validate button disabled until a file is picked', async () => {
    const wrapper = await mountSuspended(ImportPage)
    const text = wrapper.text()
    expect(text).toContain('Import Massal Aset')
    expect(text).toContain('asset_tag') // expected column
    const validate = wrapper.findAll('button').find(b => b.text().trim() === 'Validasi Berkas')
    expect(validate).toBeDefined()
    expect(validate!.attributes('disabled')).toBeDefined()

    // Pick a (mock) file → validate becomes enabled.
    const drop = wrapper.findAll('button').find(b => b.text().includes('Klik untuk pilih berkas'))
    await drop!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('aset-import-batch.xlsx')
    const validate2 = wrapper.findAll('button').find(b => b.text().trim() === 'Validasi Berkas')
    expect(validate2!.attributes('disabled')).toBeUndefined()
  })

  it('validates to step 2 with the row preview and error notes, then creates to step 3', async () => {
    const wrapper = await mountSuspended(ImportPage)
    await wrapper.findAll('button').find(b => b.text().includes('Klik untuk pilih berkas'))!.trigger('click')
    await wrapper.vm.$nextTick()
    await wrapper.findAll('button').find(b => b.text().trim() === 'Validasi Berkas')!.trigger('click')
    await wait(1500)
    await wrapper.vm.$nextTick()

    const text = wrapper.text()
    expect(text).toContain('Total baris')
    expect(text).toContain('Laptop Asus ExpertBook') // a preview row
    expect(text).toContain('asset_tag duplikat dengan baris #2') // an error note
    expect(text).toContain('7 Valid')
    expect(text).toContain('5 Error')

    // Create valid assets → step 3 result.
    const create = wrapper.findAll('button').find(b => b.text().includes('Buat Aset Valid'))
    expect(create).toBeDefined()
    await create!.trigger('click')
    await wait(1500)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Import selesai diproses')
    expect(wrapper.text()).toContain('Aset dibuat')
  })
})
