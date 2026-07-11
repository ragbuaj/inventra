// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import DashboardDonut from '~/components/dashboard/Donut.vue'
import { buildDonut } from '~/utils/dashboard'

describe('DashboardDonut', () => {
  const { total, segments } = buildDonut([58, 22, 9, 4, 3])

  it('renders the formatted total and total label in the centre', async () => {
    const wrapper = await mountSuspended(DashboardDonut, {
      props: { title: 'Aset per Status', total, totalLabel: 'Total Aset', segments }
    })
    expect(wrapper.text()).toContain('96')
    expect(wrapper.text()).toContain('Total Aset')
  })

  it('renders a legend row per segment with translated status labels and percentages', async () => {
    const wrapper = await mountSuspended(DashboardDonut, {
      props: { title: 'Aset per Status', total, totalLabel: 'Total Aset', segments }
    })
    const text = wrapper.text()
    // id locale labels (real enum keys)
    expect(text).toContain('Tersedia')
    expect(text).toContain('Digunakan')
    // counts + a percentage
    expect(text).toContain('58')
    expect(text).toContain('60%') // 58/96 ≈ 60%
  })
})
