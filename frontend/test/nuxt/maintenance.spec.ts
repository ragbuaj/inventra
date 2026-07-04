// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { maintenanceStore } from '~/mock/maintenance'
import MaintenancePage from '~/pages/maintenance.vue'

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
  const wrapper = await mountSuspended(MaintenancePage, { route: '/maintenance' })
  await wait(1700)
  await wrapper.vm.$nextTick()
  return wrapper
}

function clickTab(wrapper: Awaited<ReturnType<typeof mountAndWait>>, label: string) {
  const btn = wrapper.findAll('button').find(b => b.text().trim() === label)
  return btn!.trigger('click')
}

beforeEach(() => {
  maintenanceStore.reset()
  grantAdmin()
})

describe('Maintenance page — schedule tab (default)', () => {
  it('renders the due banner with the overdue item and the schedule cards', async () => {
    const wrapper = await mountAndWait()
    const text = wrapper.text()
    expect(text).toContain('Maintenance')
    expect(text).toContain('Maintenance jatuh tempo') // due banner
    expect(text).toContain('Terlambat 4 hari') // Toyota Avanza overdue (due 2026-06-20)
    expect(text).toContain('Switch Cisco Catalyst 1000') // a later schedule card
    expect(text).toContain('Buat Catatan')
  })
})

describe('Maintenance page — notes tab', () => {
  it('lists seeded maintenance records and filters by search', async () => {
    const wrapper = await mountAndWait()
    await clickTab(wrapper, 'Catatan')
    await wrapper.vm.$nextTick()
    let text = wrapper.text()
    expect(text).toContain('JKT01-KEN-2025-00007') // Toyota Avanza record row (tag only shown in the table)
    expect(text).toContain('Honda Vario 160')
    expect(text).toContain('Dibatalkan') // cancelled status of Honda record

    const search = wrapper.find('input[type="text"]')
    await search.setValue('Honda')
    await wrapper.vm.$nextTick()
    text = wrapper.text()
    expect(text).toContain('Honda Vario 160')
    expect(text).not.toContain('JKT01-KEN-2025-00007') // Avanza row filtered out (banner shows no tags)
  })

  it('opens the add-note slideover from the toolbar button', async () => {
    const wrapper = await mountAndWait()
    await clickTab(wrapper, 'Catatan')
    await wrapper.vm.$nextTick()
    const addBtn = wrapper.findAll('button').find(b => b.text().trim() === 'Tambah Catatan')
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    // Slideover (teleported) — assert on document body.
    expect(document.body.textContent).toContain('Tambah Catatan Maintenance')
    expect(document.body.textContent).toContain('Kategori Perawatan')
  })
})

describe('Maintenance page — damage report tab', () => {
  it('renders the staff report form and an empty report history, with submit disabled until filled', async () => {
    const wrapper = await mountAndWait()
    await clickTab(wrapper, 'Laporan Kerusakan')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Laporkan Kerusakan Aset')
    expect(text).toContain('Tampilan Staf')
    expect(text).toContain('Belum ada laporan') // empty history
    const submit = wrapper.findAll('button').find(b => b.text().trim() === 'Kirim Laporan')
    expect(submit!.attributes('disabled')).toBeDefined()
  })
})
