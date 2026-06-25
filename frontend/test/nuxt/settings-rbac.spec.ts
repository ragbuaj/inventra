// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { roleStore } from '~/mock/rbac'
import RbacPage from '~/pages/settings/rbac.vue'

// Auto-unmount each wrapper so teleported modals don't leak into document.body
// between tests (a stale modal input would otherwise be picked up by querySelector).
enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => {
  roleStore.reset()
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(RbacPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('RBAC page — role list', () => {
  it('lists all seven roles and the add button', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Superadmin')
    expect(text).toContain('Kepala Kanwil')
    expect(text).toContain('Staf')
    expect(text).toContain('Auditor Internal')
    expect(text).toContain('Operator Gudang')
    expect(text).toContain('Tambah Peran')
  })
})

describe('RBAC page — system role (default: Manager)', () => {
  it('shows the system badge, lock note and a disabled Save', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Manager (Asset Manager)')
    expect(text).toContain('Sistem')
    expect(text).toContain('Peran sistem bersifat bawaan')
    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    expect(save).toBeDefined()
    expect(save!.attributes('disabled')).toBeDefined()
  })

  it('renders module cards with granted/total counts', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Aset')
    expect(text).toContain('Lihat aset')
    expect(text).toContain('aset.view')
    // Manager has all 7 asset permissions.
    expect(text).toContain('7 / 7 izin')
  })
})

describe('RBAC page — editing a custom role', () => {
  async function selectAuditor(wrapper: Awaited<ReturnType<typeof mountAndWait>>) {
    const roleBtn = wrapper.findAll('button').find(b => b.text().includes('Auditor Internal'))
    expect(roleBtn).toBeDefined()
    await roleBtn!.trigger('click')
    await wrapper.vm.$nextTick()
  }

  it('is editable (no lock note) and toggling a permission marks it unsaved + enables Save', async () => {
    const wrapper = await mountAndWait()
    await selectAuditor(wrapper)
    expect(wrapper.text()).not.toContain('Peran sistem bersifat bawaan')

    // Auditor lacks 'aset.create' — toggle it on.
    const permBtn = wrapper.findAll('button').find(b => b.text().includes('aset.create'))
    expect(permBtn).toBeDefined()
    await permBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Perubahan belum disimpan')
    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    expect(save!.attributes('disabled')).toBeUndefined()
  })

  it('Save persists the draft and clears the unsaved state', async () => {
    const wrapper = await mountAndWait()
    await selectAuditor(wrapper)
    const permBtn = wrapper.findAll('button').find(b => b.text().includes('aset.create'))
    await permBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const save = wrapper.findAll('button').find(b => b.text().includes('Simpan Perubahan'))
    await save!.trigger('click')
    await new Promise(r => setTimeout(r, 350))
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).not.toContain('Perubahan belum disimpan')
    expect(roleStore.find('auditor')!.perms).toContain('aset.create')
  })

  it('Select all grants every permission in a module', async () => {
    const wrapper = await mountAndWait()
    await selectAuditor(wrapper)
    // The Assets module: Auditor has only aset.view (1/7). Click its "Pilih semua".
    const selectAll = wrapper.findAll('button').find(b => b.text().trim() === 'Pilih semua')
    expect(selectAll).toBeDefined()
    await selectAll!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('7 / 7 izin')
  })
})

describe('RBAC page — add role', () => {
  it('validates a required name', async () => {
    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah Peran'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const create = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.trim() === 'Buat Peran')
    expect(create).toBeDefined()
    create!.click()
    await wrapper.vm.$nextTick()
    expect(document.body.textContent).toContain('Wajib diisi')
  })

  it('creates a custom role and selects it', async () => {
    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah Peran'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const nameInput = document.body.querySelector('input[placeholder="mis. Operator Lapangan"]') as HTMLInputElement
    expect(nameInput).toBeTruthy()
    nameInput.value = 'Operator Lapangan'
    nameInput.dispatchEvent(new Event('input', { bubbles: true }))
    await wrapper.vm.$nextTick()
    const create = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.trim() === 'Buat Peran')
    create!.click()
    await new Promise(r => setTimeout(r, 450))
    await wrapper.vm.$nextTick()
    // Diagnostic: confirm the role reached the store (name binding + create worked).
    expect(roleStore.all().some(r => r.nama.id === 'Operator Lapangan')).toBe(true)
    expect(wrapper.text()).toContain('Operator Lapangan')
    // Newly created role becomes the selected header role with the Custom badge.
    expect(wrapper.text()).toContain('Kustom')
  })
})
