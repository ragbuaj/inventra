// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import Akun from '~/pages/akun.vue'
import { useAuthStore } from '~/stores/auth'

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager' }, ['*'])
}

async function mountLoaded() {
  const w = await mountSuspended(Akun, { route: '/akun' })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Profil tab', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
  })

  it('renders the profile header and personal data', async () => {
    const w = await mountLoaded()
    expect(w.text()).toContain('Andi Saputra')
    expect(w.text()).toContain('Asset Manager')
    expect(w.text()).toContain('Data Diri')
  })

  it('shows the required error when saving with an empty name', async () => {
    const w = await mountLoaded()
    const nameInput = w.findAll('input')[0]!
    await nameInput.setValue('')
    const saveBtn = w.findAll('button').find(b => b.text().includes('Simpan Perubahan'))!
    await saveBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Wajib diisi')
  })
})
