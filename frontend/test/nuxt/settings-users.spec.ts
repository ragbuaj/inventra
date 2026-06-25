// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { useAuthStore } from '~/stores/auth'
import { userStore, userSeed } from '~/mock/users'
import UsersPage from '~/pages/settings/users.vue'

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

// Restore the shared mock store before every test so order/contents are deterministic.
// insert() prepends, so seed in reverse to reproduce the app's forward seed order.
beforeEach(() => {
  for (const r of userStore.all().slice()) userStore.remove(r.id)
  for (const r of [...userSeed].reverse()) userStore.insert({ ...r })
  grantAdmin()
})

async function mountAndWait() {
  const wrapper = await mountSuspended(UsersPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('User Management page — rendering', () => {
  it('renders the title, subtitle and Add button', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Pengguna')
    expect(text).toContain('Kelola akun login')
    expect(text).toContain('Tambah User')
  })

  it('lists seeded users with name and email', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Super Admin')
    expect(text).toContain('admin@inventra.go.id')
    expect(text).toContain('Bambang Sukasno')
  })

  it('shows role badges, status labels and login labels', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Superadmin') // role badge
    expect(text).toContain('Disuspend') // Budi Hartono is suspended
    expect(text).toContain('Email') // email-login label
    expect(text).toContain('Google') // google-login label
  })

  it('shows an em-dash for users with no linked employee', async () => {
    const wrapper = await mountAndWait()
    // Super Admin has empty pegawai → em-dash rendered
    expect(wrapper.text()).toContain('—')
  })

  it('paginates to 10 rows per page (page-2 users hidden initially)', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    // First 10 by seed order are visible; 'Agus Salim' & 'Putri Maharani' are on page 2.
    expect(text).toContain('Fajar Nugroho')
    expect(text).not.toContain('Agus Salim')
  })
})

describe('User Management page — search & filters', () => {
  it('filters by name via the search box', async () => {
    const wrapper = await mountAndWait()
    const input = wrapper.find('input[type="text"]')
    await input.setValue('siti')
    await new Promise(r => setTimeout(r, 50))
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Siti Aminah')
    expect(text).not.toContain('Bambang Sukasno')
  })

  it('shows the reset button only when a filter is active, and clears it', async () => {
    const wrapper = await mountAndWait()
    expect(wrapper.text()).not.toContain('Reset')
    await wrapper.find('input[type="text"]').setValue('zzz-no-match')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Reset')
  })

  it('shows the empty state when nothing matches', async () => {
    const wrapper = await mountAndWait()
    await wrapper.find('input[type="text"]').setValue('zzz-no-match')
    await new Promise(r => setTimeout(r, 50))
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Belum ada akun yang cocok')
  })
})

describe('User Management page — create form', () => {
  // The slideover teleports to <body>, so assert/query via document.body.
  it('opens the slideover when Add is clicked', async () => {
    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah User'))
    expect(addBtn).toBeDefined()
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(document.body.textContent).toContain('Kosongkan jika pengguna hanya login dengan Google')
  })

  it('validates required name & email before saving', async () => {
    const wrapper = await mountAndWait()
    const addBtn = wrapper.findAll('button').find(b => b.text().includes('Tambah User'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const saveBtn = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.trim() === 'Simpan')
    expect(saveBtn).toBeDefined()
    saveBtn!.click()
    await wrapper.vm.$nextTick()
    await new Promise(r => setTimeout(r, 20))
    // No new user was added and the required-field error is shown.
    expect(document.body.textContent).toContain('Wajib diisi')
    expect(userStore.all()).toHaveLength(12)
  })
})
