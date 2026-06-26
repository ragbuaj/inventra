// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import Akun from '~/pages/account.vue'
import { useAuthStore } from '~/stores/auth'

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager' }, ['*'])
}
async function mountLoaded() {
  const w = await mountSuspended(Akun)
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Preferensi tab', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
    localStorage.clear()
  })

  it('shows appearance + notification sections', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Preferensi')!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tampilan')
    expect(w.text()).toContain('Notifikasi')
    expect(w.text()).toContain('Keputusan Approval')
  })

  it('persists a notification toggle', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Preferensi')!.trigger('click')
    await flushPromises()
    const before = localStorage.getItem('inventra.account.notif')
    // toggle the first notification switch via data-testid
    const toggle = w.find('[data-testid="notif-approval"]')
    await toggle.trigger('click')
    await flushPromises()
    expect(localStorage.getItem('inventra.account.notif')).not.toBe(before)
  })
})
