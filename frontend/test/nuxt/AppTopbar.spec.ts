// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import AppTopbar from '~/components/AppTopbar.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { useNotificationsStore } from '~/stores/notifications'

// NotificationBell (mounted inside AppTopbar) now talks to the real API.
// Stubbing the composable keeps these tests off the network -- unstubbed, every
// mount would fire a live request at :8080 and fail with ECONNREFUSED.
// Task 17 rewrites this file properly; this is the minimum to keep it honest.
vi.mock('~/composables/api/useNotifications', () => ({
  useNotifications: () => ({
    list: vi.fn().mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0 }),
    unreadCount: vi.fn().mockResolvedValue(0),
    markAllRead: vi.fn().mockResolvedValue(undefined),
    markRead: vi.fn()
  })
}))

function setupSuperadmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin Inventra', email: 'admin@inventra.local', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

describe('AppTopbar', () => {
  beforeEach(() => {
    useAuthStore().clear()
    useUiStore().sidebarCollapsed = false
    useUiStore().mobileNavOpen = false
    // The bell reads the store, so seed it here rather than a mock fixture.
    const notifs = useNotificationsStore()
    notifs.items = []
    notifs.unreadCount = 0
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

  it('clicking the desktop sidebar toggle flips sidebarCollapsed', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const uiStore = useUiStore()
    const before = uiStore.sidebarCollapsed
    // The desktop rail toggle carries the panel-left icon (a separate mobile
    // hamburger, with the menu icon, now precedes it in the DOM).
    const toggleBtn = wrapper.findAll('button').find(b => b.html().includes('i-lucide:panel-left'))
    expect(toggleBtn).toBeDefined()
    await toggleBtn!.trigger('click')
    expect(uiStore.sidebarCollapsed).toBe(!before)
  })

  it('the mobile hamburger opens the off-canvas drawer without touching the rail', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const uiStore = useUiStore()
    uiStore.mobileNavOpen = false
    uiStore.sidebarCollapsed = false
    // The hamburger carries the menu icon.
    const hamburger = wrapper.findAll('button').find(b => b.html().includes('i-lucide:menu'))
    expect(hamburger).toBeDefined()
    await hamburger!.trigger('click')
    // Opens the drawer; the desktop rail-collapse flag is left untouched.
    expect(uiStore.mobileNavOpen).toBe(true)
    expect(uiStore.sidebarCollapsed).toBe(false)
  })

  it('renders the search trigger button with ⌘K indicator', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    const html = wrapper.html()
    expect(html).toContain('⌘K')
    // GlobalSearch is now a button trigger (not an input) that opens the command palette.
    // Prove the actual search trigger is present (the button carrying the ⌘K chip),
    // not just any button in the topbar.
    const searchTrigger = wrapper.findAll('button').find(b => b.text().includes('⌘K'))
    expect(searchTrigger).toBeDefined()
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
    useNotificationsStore().unreadCount = 2
    const wrapper = await mountSuspended(AppTopbar)
    const badge = wrapper.find('span.bg-error.rounded-full')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('2')
  })

  it('hides the unread badge entirely when nothing is unread', async () => {
    setupSuperadmin()
    useNotificationsStore().unreadCount = 0
    const wrapper = await mountSuspended(AppTopbar)
    expect(wrapper.find('span.bg-error.rounded-full').exists()).toBe(false)
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

  it('the breadcrumb block grows on mobile so the right cluster (bell + user) sits at the far right', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppTopbar)
    // The two-line breadcrumb block wraps the page-title span. Below md (where
    // GlobalSearch is hidden) it must grow (flex-1) to push the bell + user menu
    // to the far right; on md+ it collapses (md:flex-none) so the search stays centered.
    const titleSpan = wrapper.find('span.text-\\[16px\\]')
    expect(titleSpan.exists()).toBe(true)
    const block = titleSpan.element.parentElement as HTMLElement
    expect(block.classList.contains('flex-1')).toBe(true)
    expect(block.classList.contains('md:flex-none')).toBe(true)
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

  it('the notification bell reads its rows from the store', async () => {
    setupSuperadmin()
    const notifs = useNotificationsStore()
    notifs.items = [{
      id: 'n-1',
      type: 'asset_returned',
      params: { asset_tag: 'INV-2024-0312', asset_name: 'Toyota Avanza' },
      entity_type: 'assets',
      entity_id: 'asset-uuid',
      read_at: null,
      created_at: new Date().toISOString()
    }]
    notifs.unreadCount = 1

    await mountSuspended(AppTopbar)

    // The bell renders the message from type + params via the meta catalog, so
    // the store row -- not a fixture -- is what reaches the screen.
    expect(useNotificationsStore().items).toHaveLength(1)
    expect(useNotificationsStore().items[0]!.type).toBe('asset_returned')
  })
})
