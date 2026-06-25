// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { dataScopeStore } from '~/mock/dataScope'
import DataScopePage from '~/pages/settings/data-scope.vue'

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => {
  dataScopeStore.reset()
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(DataScopePage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Data Scope page', () => {
  it('renders the title, legend levels, roles and module columns', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Data Scope')
    expect(text).toContain('Level lingkup data')
    // levels
    expect(text).toContain('global')
    expect(text).toContain('office_subtree')
    expect(text).toContain('own')
    // roles
    expect(text).toContain('Superadmin')
    expect(text).toContain('Manager')
    // module column header + default column
    expect(text).toContain('Default')
    expect(text).toContain('Aset')
  })

  it('Save is disabled until a scope cell is changed', async () => {
    const wrapper = await mountAndWait()
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save).toBeDefined()
    expect(save!.attributes('disabled')).toBeDefined()
  })

  it('changing a scope level marks the page dirty and enables Save', async () => {
    const wrapper = await mountAndWait()
    // Superadmin's Default cell shows "global"; open its popover.
    const pill = wrapper.findAll('button').find(b => b.text().includes('global'))
    expect(pill).toBeDefined()
    await pill!.trigger('click')
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 20))
    // Pick the "own" level (its description is unique) from the teleported popover.
    const ownOpt = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.includes('Hanya data miliknya'))
    expect(ownOpt).toBeDefined()
    ownOpt!.click()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().trim() === 'Simpan')
    expect(save!.attributes('disabled')).toBeUndefined()
  })
})
