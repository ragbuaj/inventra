// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { approvalStore } from '~/mock/approval'
import ApprovalPage from '~/pages/approval.vue'

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

const wait = (ms: number) => new Promise(r => setTimeout(r, ms))

async function mountAndWait() {
  const wrapper = await mountSuspended(ApprovalPage, { route: '/approval' })
  await wait(800)
  await wrapper.vm.$nextTick()
  return wrapper
}

beforeEach(() => {
  approvalStore.reset()
  grantAdmin()
})

describe('Approval page — inbox + detail', () => {
  it('renders the title with pending count, the inbox list, and the first request detail', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Pengajuan & Approval')
    expect(text).toContain('5 menunggu') // pending count
    expect(text).toContain('Registrasi 12 Laptop Asus ExpertBook B1') // r1 in list + detail
    // detail sections
    expect(text).toContain('Data Diajukan')
    expect(text).toContain('Alasan Pengajuan')
    expect(text).toContain('Riwayat Approval')
    // pending → action buttons present
    expect(wrapper.findAll('button').some(b => b.text().trim() === 'Setujui')).toBe(true)
    expect(wrapper.findAll('button').some(b => b.text().trim() === 'Tolak')).toBe(true)
  })

  it('shows the sensitive warning for a sensitive request type (valuation)', async () => {
    const wrapper = await mountAndWait()
    // Select r2 (valuasi, sensitive) from the inbox.
    const card = wrapper.findAll('button').find(b => b.text().includes('Pengecualian Valuasi — Genset'))
    await card!.trigger('click')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Sensitif')
    expect(text).toContain('Tindakan sensitif') // warning banner
    // diff table headers
    expect(text).toContain('Sebelum')
    expect(text).toContain('Sesudah')
  })

  it('approving a request flips it to a decided result banner', async () => {
    const wrapper = await mountAndWait()
    const approve = wrapper.findAll('button').find(b => b.text().trim() === 'Setujui')
    await approve!.trigger('click')
    await wait(1200)
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Disetujui oleh') // result banner
    // action buttons gone once decided
    expect(wrapper.findAll('button').some(b => b.text().trim() === 'Setujui')).toBe(false)
  })

  it('switching the status filter clears the selection and shows the placeholder', async () => {
    const wrapper = await mountAndWait()
    const allTab = wrapper.findAll('button').find(b => b.text().trim() === 'Disetujui')
    await allTab!.trigger('click')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Tidak ada pengajuan dipilih') // placeholder
    expect(text).toContain('Registrasi Meja Rapat Kayu 10-Seat') // r6 (approved) now in the filtered inbox
  })
})
