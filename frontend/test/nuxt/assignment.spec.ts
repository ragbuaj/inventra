// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { assignmentStore } from '~/mock/assignment'
import AssignmentPage from '~/pages/assignment.vue'

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

const wait = (ms: number) => new Promise(r => setTimeout(r, ms))

async function mountAndWait() {
  const wrapper = await mountSuspended(AssignmentPage, { route: '/assignment' })
  await wait(1300)
  await wrapper.vm.$nextTick()
  return wrapper
}

function clickTab(wrapper: Awaited<ReturnType<typeof mountAndWait>>, label: string) {
  const btn = wrapper.findAll('button').find(b => b.text().includes(label))
  return btn!.trigger('click')
}

beforeEach(() => {
  assignmentStore.reset()
  grantAdmin()
})

describe('Assignment page — check-out tab (default)', () => {
  it('renders the title, tabs, and the check-out form fields', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Penugasan Aset')
    expect(text).toContain('Check-out')
    expect(text).toContain('Riwayat')
    expect(text).toContain('Aset (hanya yang tersedia)')
    expect(text).toContain('Hanya aset berstatus') // asset hint
    expect(text).toContain('Pegawai Penerima')
  })

  it('shows the active-assignment count badge on the check-in tab (3 seeded active)', async () => {
    const wrapper = await mountAndWait()
    // The check-in tab button carries the active count badge.
    const checkinBtn = wrapper.findAll('button').find(b => b.text().includes('Check-in'))
    expect(checkinBtn!.text()).toContain('3')
  })
})

describe('Assignment page — check-in tab', () => {
  it('renders the active-assignment selector and the needs-maintenance toggle', async () => {
    const wrapper = await mountAndWait()
    await clickTab(wrapper, 'Check-in')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Penugasan Aktif')
    expect(text).toContain('Perlu maintenance')
    expect(text).not.toContain('Tidak ada penugasan aktif')
  })
})

describe('Assignment page — history tab', () => {
  it('lists seeded assignments with status + condition', async () => {
    const wrapper = await mountAndWait()
    await clickTab(wrapper, 'Riwayat')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Televisi Samsung 55" Crystal') // a returned row
    expect(text).toContain('Proyektor Epson EB-X51') // an active row
    expect(text).toContain('Total 6 penugasan')
    expect(text).toContain('Dikembalikan')
    expect(text).toContain('Aktif')
  })

  it('filters the history by search term', async () => {
    const wrapper = await mountAndWait()
    await clickTab(wrapper, 'Riwayat')
    await wrapper.vm.$nextTick()
    const search = wrapper.find('input[type="text"]')
    await search.setValue('Honda')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Honda Vario 160')
    expect(text).not.toContain('Televisi Samsung')
    expect(text).toContain('Total 1 penugasan')
  })
})
