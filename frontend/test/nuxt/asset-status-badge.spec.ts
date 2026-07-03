// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import AssetStatusBadge from '~/components/asset/AssetStatusBadge.vue'

const EXPECTED_ID_LABEL: Record<string, string> = {
  available: 'Tersedia',
  assigned: 'Digunakan',
  under_maintenance: 'Maintenance',
  in_transfer: 'Dalam Mutasi',
  retired: 'Nonaktif',
  disposed: 'Dilepas',
  lost: 'Hilang'
}

describe('AssetStatusBadge', () => {
  for (const [status, label] of Object.entries(EXPECTED_ID_LABEL)) {
    it(`renders the resolved Indonesian label for status "${status}"`, async () => {
      const wrapper = await mountSuspended(AssetStatusBadge, { props: { status } })
      expect(wrapper.text().trim()).toBe(label)
    })
  }

  it('does not crash on an unknown status and falls back to a neutral badge', async () => {
    const wrapper = await mountSuspended(AssetStatusBadge, { props: { status: 'some-unknown-status' } })
    expect(wrapper.html()).toBeTruthy()
    // No i18n entry exists for this key, so vue-i18n returns the raw key.
    expect(wrapper.text().trim()).toBe('assets.status.some-unknown-status')
  })

  it('stays tolerant of the old Indonesian mock-status values still used by not-yet-rewired pages', async () => {
    const wrapper = await mountSuspended(AssetStatusBadge, { props: { status: 'tersedia' } })
    expect(wrapper.text().trim()).toBe('Tersedia')
  })
})
