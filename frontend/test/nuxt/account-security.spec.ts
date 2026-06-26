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
  const w = await mountSuspended(Akun, { props: {} })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

describe('Account page — Keamanan tab', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
  })

  it('switches to the security tab and shows the password form', async () => {
    const w = await mountLoaded()
    const tabBtn = w.findAll('button').find(b => b.text().trim() === 'Keamanan')!
    await tabBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Ganti Password')
    expect(w.text()).toContain('Sesi & Perangkat')
  })

  it('shows the confirm-mismatch error', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Keamanan')!.trigger('click')
    await flushPromises()
    const pw = w.findAll('input[type="password"]')
    await pw[0]!.setValue('oldpass')
    await pw[1]!.setValue('Abcdefg1!')
    await pw[2]!.setValue('different')
    // Click the submit UButton (has text "Ganti Password" and is a UButton with color=primary)
    const submitBtn = w.findAll('button').find(b => b.text().includes('Ganti Password'))!
    await submitBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('tidak cocok')
  })

  it('shows the confirm error when Confirm is left empty on submit', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Keamanan')!.trigger('click')
    await flushPromises()
    const pw = w.findAll('input[type="password"]')
    await pw[0]!.setValue('oldpass')
    await pw[1]!.setValue('Abcdefg1!')
    // leave Confirm (pw[2]) empty
    const submitBtn = w.findAll('button').find(b => b.text().includes('Ganti Password'))!
    await submitBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('tidak cocok')
  })

  it('updates the strength meter as the new password is typed', async () => {
    const w = await mountLoaded()
    await w.findAll('button').find(b => b.text().trim() === 'Keamanan')!.trigger('click')
    await flushPromises()
    const pw = w.findAll('input[type="password"]')
    await pw[1]!.setValue('Abcdefg1!')
    await flushPromises()
    expect(w.text()).toContain('Sangat Kuat')
  })
})
