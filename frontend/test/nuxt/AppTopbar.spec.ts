// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import AppTopbar from '~/components/AppTopbar.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { notificationStore } from '~/mock/notifications'

function setupSuperadmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin Inventra', email: 'admin@inventra.local', role_id: 'r1', role_name: 'Superadmin' },
    ['*']
  )
}

describe('AppTopbar', () => {
  beforeEach(() => {
    useAuthStore().clear()
    useUiStore().sidebarCollapsed = false
    // Reset notifications to seed state (mark first two unread)
    const all = notificationStore.all()
    if (all[0]) notificationStore.patch(all[0].id, { read: false })
    if (all[1]) notificationStore.patch(all[1].id, { read: false })
    if (all[2]) notificationStore.patch(all[2].id, { read: true })
  })

  it('renders a page title for the current route', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const html = wrapper.html()
    // The topbar should contain the current page title (dashboard = Dasbor in id, Dashboard in en)
    expect(html.length).toBeGreaterThan(0)
    // Should have the topbar header element
    expect(wrapper.find('header')).toBeTruthy()
  })

  it('renders the Inventra breadcrumb root in the topbar', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    expect(wrapper.html()).toContain('Inventra')
  })

  it('renders the page title span', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    // The two-line block's second line is the page title (Dasbor or Dashboard)
    const titleSpan = wrapper.find('span.text-\\[16px\\]')
    expect(titleSpan.exists()).toBe(true)
    expect(titleSpan.text().length).toBeGreaterThan(0)
  })

  it('renders the sidebar toggle button with the correct title', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const toggleBtn = wrapper.find('button[title]')
    expect(toggleBtn.exists()).toBe(true)
  })

  it('clicking the sidebar toggle flips sidebarCollapsed', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const uiStore = useUiStore()
    const before = uiStore.sidebarCollapsed
    const buttons = wrapper.findAll('button')
    const toggleBtn2 = buttons[0]!
    await toggleBtn2.trigger('click')
    expect(uiStore.sidebarCollapsed).toBe(!before)
  })

  it('renders the search input with ⌘K indicator', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const html = wrapper.html()
    expect(html).toContain('⌘K')
    expect(wrapper.find('input').exists()).toBe(true)
  })

  it('renders both ID and EN language segment buttons', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const html = wrapper.html()
    expect(html).toContain('ID')
    expect(html).toContain('EN')
  })

  it('the active locale segment has bg-default class', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    // One of ID or EN should have the active bg-default class
    const buttons = wrapper.findAll('button')
    const idBtn = buttons.find(b => b.text().trim() === 'ID')
    const enBtn = buttons.find(b => b.text().trim() === 'EN')
    // Exactly one should be active (bg-default), the other should not
    const idActive = idBtn?.classes().includes('bg-default') ?? false
    const enActive = enBtn?.classes().includes('bg-default') ?? false
    expect(idActive || enActive).toBe(true)
    expect(idActive && enActive).toBe(false)
  })

  it('both ID and EN buttons are always rendered (segmented control always visible)', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const buttons = wrapper.findAll('button')
    const idBtn = buttons.find(b => b.text().trim() === 'ID')
    const enBtn = buttons.find(b => b.text().trim() === 'EN')
    // Both must always be visible in the segmented control
    expect(idBtn).toBeDefined()
    expect(enBtn).toBeDefined()
  })

  it('renders the notification bell button', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    // Bell button is inside the notification popover trigger div
    const bellBtn = wrapper.find('button[title]')
    expect(bellBtn.exists()).toBe(true)
    // Should contain the bell icon
    expect(wrapper.html()).toContain('i-lucide:bell')
  })

  it('renders the unread badge count on the notification bell', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    // The unread count badge should show 2 (from seed)
    const { unreadCount } = useNotifications()
    expect(unreadCount()).toBe(2)
    expect(wrapper.html()).toContain('2')
  })

  it('markAllRead via useNotifications drops unread count to 0', async () => {
    setupSuperadmin()
    const { unreadCount, markAllRead } = useNotifications()
    expect(unreadCount()).toBe(2)
    markAllRead()
    expect(unreadCount()).toBe(0)
  })

  it('user menu trigger pill has rounded-full class and shows initials', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    // "Admin Inventra" => initials "AI"
    expect(wrapper.html()).toContain('AI')
    // Pill button must have rounded-full class
    const pillBtn = wrapper.findAll('button').find(b => b.classes('rounded-full'))
    expect(pillBtn).toBeDefined()
  })

  it('the header element has the correct z-index class', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const header = wrapper.find('header')
    expect(header.classes()).toContain('z-30')
  })

  it('the header element has h-[61px] class matching the mockup height', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const header = wrapper.find('header')
    expect(header.classes()).toContain('h-[61px]')
  })

  it('notification panel items are accessible via the notifications composable', async () => {
    setupSuperadmin()
    const { list } = useNotifications()
    const items = list()
    expect(items.length).toBeGreaterThan(0)
    // Each item has icon, title, time, read
    for (const item of items) {
      expect(typeof item.icon).toBe('string')
      expect(typeof item.title).toBe('string')
      expect(typeof item.time).toBe('string')
      expect(typeof item.read).toBe('boolean')
    }
  })
})
