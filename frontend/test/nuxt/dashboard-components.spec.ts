// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import DashboardKpiCard from '~/components/dashboard/KpiCard.vue'
import DashboardBarList from '~/components/dashboard/BarList.vue'
import DashboardMaintenancePanel from '~/components/dashboard/MaintenancePanel.vue'
import DashboardApprovalPanel from '~/components/dashboard/ApprovalPanel.vue'
import { barWidths } from '~/utils/dashboard'
import type { MaintenanceItem, ApprovalItem } from '~/composables/api/useDashboard'

describe('DashboardKpiCard', () => {
  it('renders the label, value and trend text', async () => {
    const wrapper = await mountSuspended(DashboardKpiCard, {
      props: {
        label: 'Total Aset', value: '96', icon: 'i-lucide-package', iconTone: 'primary',
        trendIcon: 'i-lucide-trending-up', trendText: '+ aktif bertambah', trendTone: 'success'
      }
    })
    const text = wrapper.text()
    expect(text).toContain('Total Aset')
    expect(text).toContain('96')
    expect(text).toContain('+ aktif bertambah')
  })

  it('applies the trend tone colour class', async () => {
    const wrapper = await mountSuspended(DashboardKpiCard, {
      props: {
        label: 'Aset Overdue', value: '4', icon: 'i-lucide-clock-alert', iconTone: 'error',
        trendIcon: 'i-lucide-triangle-alert', trendText: 'perlu tindakan', trendTone: 'error'
      }
    })
    expect(wrapper.html()).toContain('text-error')
  })
})

describe('DashboardBarList', () => {
  const items = barWidths([['Elektronik', 41], ['Furnitur', 28], ['Perangkat IT', 12]])

  it('renders one row per item with label and grouped count', async () => {
    const wrapper = await mountSuspended(DashboardBarList, {
      props: { title: 'Aset per Kategori', items, color: 'primary' }
    })
    const text = wrapper.text()
    expect(text).toContain('Aset per Kategori')
    expect(text).toContain('Elektronik')
    expect(text).toContain('41')
    expect(text).toContain('Perangkat IT')
  })

  it('sets the largest bar to width 100% and scales the rest', async () => {
    const wrapper = await mountSuspended(DashboardBarList, {
      props: { title: 'Aset per Kategori', items, color: 'primary' }
    })
    const html = wrapper.html()
    expect(html).toContain('width: 100%') // Elektronik (max)
    expect(html).toContain(`width: ${Math.round(28 / 41 * 100)}%`)
  })

  it('uses the info bar colour when color="info"', async () => {
    const wrapper = await mountSuspended(DashboardBarList, {
      props: { title: 'Aset per Lokasi', items, color: 'info' }
    })
    expect(wrapper.html()).toContain('bg-info')
  })
})

describe('DashboardMaintenancePanel', () => {
  const items: MaintenanceItem[] = [
    { asset: 'Toyota Avanza · B 1234 XYZ', task: 'Servis berkala', icon: 'i-lucide-truck', urg: 1, due: 'Besok' },
    { asset: 'AC Daikin · R.301', task: 'Pembersihan filter', icon: 'i-lucide-wrench', urg: 0, due: '3 hari lagi' }
  ]

  it('renders a row per item with asset, task and due', async () => {
    const wrapper = await mountSuspended(DashboardMaintenancePanel, {
      props: { title: 'Maintenance Jatuh Tempo', seeAllLabel: 'Lihat semua', items }
    })
    const text = wrapper.text()
    expect(text).toContain('Toyota Avanza · B 1234 XYZ')
    expect(text).toContain('Servis berkala')
    expect(text).toContain('Besok')
    expect(text).toContain('3 hari lagi')
  })

  it('gives the urgent due a warning pill and the normal due a neutral pill', async () => {
    const wrapper = await mountSuspended(DashboardMaintenancePanel, {
      props: { title: 'Maintenance Jatuh Tempo', seeAllLabel: 'Lihat semua', items }
    })
    const html = wrapper.html()
    expect(html).toContain('bg-warning/10') // urgent
    expect(html).toContain('text-dimmed') // normal pill
  })

  it('emits see-all when the header link is clicked', async () => {
    const wrapper = await mountSuspended(DashboardMaintenancePanel, {
      props: { title: 'Maintenance Jatuh Tempo', seeAllLabel: 'Lihat semua', items }
    })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('seeAll')).toBeTruthy()
  })

  it('renders no rows for an empty list', async () => {
    const wrapper = await mountSuspended(DashboardMaintenancePanel, {
      props: { title: 'Maintenance Jatuh Tempo', seeAllLabel: 'Lihat semua', items: [] }
    })
    expect(wrapper.text()).not.toContain('Toyota Avanza')
  })
})

describe('DashboardApprovalPanel', () => {
  const items: ApprovalItem[] = [
    { id: 'a1', title: 'Peminjaman Proyektor', meta: 'Andi · 2 jam lalu', icon: 'i-lucide-projector', tone: 'info' },
    { id: 'a2', title: 'Mutasi 3 Laptop', meta: 'Rina · 5 jam lalu', icon: 'i-lucide-package', tone: 'primary' }
  ]

  it('renders a row per request and the count in the header', async () => {
    const wrapper = await mountSuspended(DashboardApprovalPanel, {
      props: { title: 'Pengajuan', items, emptyTitle: 'Selesai', emptySub: 'Kosong' }
    })
    const text = wrapper.text()
    expect(text).toContain('Peminjaman Proyektor')
    expect(text).toContain('Mutasi 3 Laptop')
    expect(text).toContain('2') // count badge
  })

  it('emits approve with the request id', async () => {
    const wrapper = await mountSuspended(DashboardApprovalPanel, {
      props: { title: 'Pengajuan', items, emptyTitle: 'Selesai', emptySub: 'Kosong' }
    })
    const approveBtn = wrapper.find('[aria-label="approve-a1"]')
    expect(approveBtn.exists()).toBe(true)
    await approveBtn.trigger('click')
    expect(wrapper.emitted('approve')?.[0]).toEqual(['a1'])
  })

  it('emits reject with the request id', async () => {
    const wrapper = await mountSuspended(DashboardApprovalPanel, {
      props: { title: 'Pengajuan', items, emptyTitle: 'Selesai', emptySub: 'Kosong' }
    })
    await wrapper.find('[aria-label="reject-a2"]').trigger('click')
    expect(wrapper.emitted('reject')?.[0]).toEqual(['a2'])
  })

  it('shows the empty state and a zero count when there are no requests', async () => {
    const wrapper = await mountSuspended(DashboardApprovalPanel, {
      props: { title: 'Pengajuan', items: [], emptyTitle: 'Semua ditindak', emptySub: 'Tidak ada' }
    })
    const text = wrapper.text()
    expect(text).toContain('Semua ditindak')
    expect(text).toContain('Tidak ada')
    expect(text).not.toContain('Peminjaman Proyektor')
  })
})
