// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import GlobalSearch from '~/components/GlobalSearch.vue'
import { useCommandPalette } from '~/composables/useCommandPalette'

describe('GlobalSearch trigger', () => {
  beforeEach(() => useCommandPalette().close())

  it('opens the palette when the trigger is clicked', async () => {
    const w = await mountSuspended(GlobalSearch)
    expect(useCommandPalette().isOpen.value).toBe(false)
    await w.find('button').trigger('click')
    expect(useCommandPalette().isOpen.value).toBe(true)
  })

  it('shows the topbar placeholder and ⌘K hint', async () => {
    const w = await mountSuspended(GlobalSearch)
    expect(w.text()).toContain('⌘K')
    // The placeholder must resolve via i18n (default id locale), not render the raw key.
    expect(w.text()).toContain('Cari aset')
    expect(w.text()).not.toContain('search.topbarPlaceholder')
  })
})
