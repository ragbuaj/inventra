// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { useAuthStore } from '~/stores/auth'
import AuditPage from '~/pages/settings/audit.vue'

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

beforeEach(() => grantAdmin())

async function mountAndWait() {
  const wrapper = await mountSuspended(AuditPage)
  await new Promise(r => setTimeout(r, 400))
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Audit Trail page', () => {
  it('renders title, log rows and action badges', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Audit Trail')
    expect(text).toContain('Dewi Lestari')
    expect(text).toContain('Ubah valuasi Genset Cummins C22')
    expect(text).toContain('Ubah') // update action label
  })

  it('paginates 8 rows per page', async () => {
    const wrapper = await mountAndWait()
    // Page 1 = logs 1–8; log 9 ("Batasi field harga_beli") is on page 2.
    expect(wrapper.text()).toContain('Check-out Monitor LG ke Rina Putri')
    expect(wrapper.text()).not.toContain('Batasi field harga_beli')
    const page2 = wrapper.findAll('button').find(b => b.text().trim() === '2')
    expect(page2).toBeDefined()
    await page2!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Batasi field harga_beli')
  })

  it('expands a row to reveal its before→after diff', async () => {
    const wrapper = await mountAndWait()
    const firstRow = wrapper.find('tbody tr')
    await firstRow.trigger('click')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('nilai_buku')
    expect(text).toContain('Rp 45.000.000')
  })

  it('filters by search and shows the empty state when nothing matches', async () => {
    const wrapper = await mountAndWait()
    const input = wrapper.find('input[type="text"]')
    await input.setValue('Genset')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Genset')
    expect(wrapper.text()).not.toContain('Buat akun Fajar Nugroho')

    await input.setValue('zzz-no-activity')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Tidak ada log')
  })
})
