// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import ReportsPage from '~/pages/reports.vue'

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

const wait = (ms: number) => new Promise(r => setTimeout(r, ms))

beforeEach(() => grantAdmin())

async function mountPage() {
  const wrapper = await mountSuspended(ReportsPage, { route: '/reports' })
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('Reports page', () => {
  it('renders the four report cards and the pre-apply placeholder', async () => {
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).toContain('Laporan')
    expect(text).toContain('Daftar Aset & Nilai Buku')
    expect(text).toContain('Depresiasi per Periode')
    expect(text).toContain('Utilisasi / Penugasan')
    expect(text).toContain('Biaya Maintenance')
    expect(text).toContain('Pilih kriteria laporan') // placeholder
  })

  it('applies the asset report and shows KPIs, a chart and a totaled table', async () => {
    const wrapper = await mountPage()
    const apply = wrapper.findAll('button').find(b => b.text().trim() === 'Terapkan')
    await apply!.trigger('click')
    await wait(700)
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).not.toContain('Pilih kriteria laporan')
    expect(text).toContain('Total Aset') // KPI label
    expect(text).toContain('Nilai Buku per Kategori') // chart title
    expect(text).toContain('TOTAL') // footer
    expect(text).toContain('Laptop Dell Latitude 5440') // a table row
    expect(text).toContain('Rp 321.800.000') // book-value footer total
  })

  it('switching the report type after applying updates the result live', async () => {
    const wrapper = await mountPage()
    await wrapper.findAll('button').find(b => b.text().trim() === 'Terapkan')!.trigger('click')
    await wait(700)
    await wrapper.vm.$nextTick()
    const costCard = wrapper.findAll('button').find(b => b.text().includes('Biaya Maintenance'))
    await costCard!.trigger('click')
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Total Biaya') // cost KPI
    expect(text).toContain('Biaya per Kategori') // cost chart title
  })
})
