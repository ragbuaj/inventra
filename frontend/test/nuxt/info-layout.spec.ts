// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import InfoLayout from '~/layouts/info.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'

// AppTopbar (rendered in the authenticated branch) mounts NotificationBell, which
// talks to the real API; stub it so mounting stays off the network.
vi.mock('~/composables/api/useNotifications', () => ({
  useNotifications: () => ({
    list: vi.fn().mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0 }),
    unreadCount: vi.fn().mockResolvedValue(0),
    markAllRead: vi.fn().mockResolvedValue(undefined),
    markRead: vi.fn()
  })
}))

const SLOT = { slots: { default: () => 'ISI-HALAMAN-INFO' } }

describe('info layout', () => {
  beforeEach(() => {
    useAuthStore().clear()
    useUiStore().sidebarCollapsed = false
    useUiStore().mobileNavOpen = false
  })

  it('guest branch: no app sidebar, shows sign-in + public cross-links + slot', async () => {
    const wrapper = await mountSuspended(InfoLayout, SLOT)
    const html = wrapper.html()
    // The signed-in app shell (an <aside> sidebar) must NOT render for a guest.
    expect(wrapper.find('aside').exists()).toBe(false)
    // Public chrome: sign-in affordance + cross-links between the info pages.
    expect(html).toContain('Masuk')
    expect(html).toContain('Panduan Penggunaan')
    expect(html).toContain('FAQ')
    expect(html).toContain('Kebijakan Privasi')
    // The routed page content is slotted in.
    expect(html).toContain('ISI-HALAMAN-INFO')
  })

  it('authenticated branch: renders the app shell sidebar and the slot', async () => {
    useAuthStore().setSession(
      'tok',
      { id: '1', name: 'Admin Inventra', email: 'admin@inventra.local', role_id: 'r1', role_name: 'Superadmin', office_id: null },
      ['*']
    )
    const wrapper = await mountSuspended(InfoLayout, SLOT)
    // The app shell renders its <aside> sidebar for a signed-in user...
    expect(wrapper.find('aside').exists()).toBe(true)
    // ...and the slotted page content is still shown inside it.
    expect(wrapper.html()).toContain('ISI-HALAMAN-INFO')
  })
})
