// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { fieldPermStore } from '~/mock/fieldPermission'
import FieldPermissionPage from '~/pages/settings/field-permission.vue'

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => {
  fieldPermStore.reset()
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(FieldPermissionPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Field Permission page', () => {
  it('renders title, field codes, role columns and explicit/default markers', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Field-Permission')
    expect(text).toContain('nama')
    expect(text).toContain('harga_beli')
    expect(text).toContain('Superadmin')
    // unrestricted fields show the "default" badge; restricted ones don't
    expect(text).toContain('default')
  })

  it('Save is disabled until a cell is toggled', async () => {
    const wrapper = await mountAndWait()
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save!.attributes('disabled')).toBeDefined()
  })

  it('toggling a View pill marks the page dirty and enables Save', async () => {
    const wrapper = await mountAndWait()
    const viewPill = wrapper.findAll('button').find(b => b.text().trim() === 'L')
    expect(viewPill).toBeDefined()
    await viewPill!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save!.attributes('disabled')).toBeUndefined()
  })

  it('resetting an explicit field reverts it to default and marks dirty', async () => {
    const wrapper = await mountAndWait()
    const resetBtn = wrapper.findAll('button').find(b => b.attributes('title') === 'Kembalikan ke default')
    expect(resetBtn).toBeDefined()
    await resetBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
  })

  it('search filters fields and shows the empty state when nothing matches', async () => {
    const wrapper = await mountAndWait()
    const input = wrapper.find('input[type="text"]')
    await input.setValue('harga')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('harga_beli')
    expect(wrapper.text()).not.toContain('kategori')

    await input.setValue('zzz-no-field')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Tidak ada field yang cocok')
  })
})
