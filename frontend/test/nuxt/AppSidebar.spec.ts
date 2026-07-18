// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import AppSidebar from '~/components/AppSidebar.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { useInboxStore } from '~/stores/inbox'

function setupSuperadmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin Inventra', email: 'admin@inventra.local', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

// Superadmin with ENUMERATED permissions (no '*') — as the backend actually returns
function setupSuperadminEnumerated() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin Inventra', email: 'admin@inventra.local', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['user.manage', 'masterdata.office.manage', 'masterdata.global.manage', 'masterdata.reference.manage']
  )
}

describe('AppSidebar', () => {
  beforeEach(() => {
    useAuthStore().clear()
    useUiStore().sidebarCollapsed = false
    useUiStore().mobileNavOpen = false
  })

  it('renders the Operasional section label', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppSidebar)
    expect(wrapper.html()).toContain('Operasional')
  })

  it('renders the Administrasi section label', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppSidebar)
    expect(wrapper.html()).toContain('Administrasi')
  })

  it('hides section labels when sidebar is collapsed', async () => {
    setupSuperadmin()
    useUiStore().sidebarCollapsed = true
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    // Section labels should not be rendered when collapsed
    expect(html).not.toContain('Operasional')
    expect(html).not.toContain('Administrasi')
  })

  it('renders a built item (Kantor) as a NuxtLink to /master/offices', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppSidebar)
    // Kantor should be visible (parent group Master Data must be expanded)
    const links = wrapper.findAll('a')
    const kantorLink = links.find(a => a.attributes('href') === '/master/offices')
    expect(kantorLink).toBeDefined()
  })

  it('renders Peta Lokasi as a built nav link (was formerly disabled Geografi)', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    // Peta Lokasi (office map) is now a built route under Master Data
    expect(html).toContain('Peta Lokasi')
    // Must appear as an anchor link to /master/map
    const links = wrapper.findAll('a')
    const mapLink = links.find(a => a.text().includes('Peta Lokasi'))
    expect(mapLink).toBeDefined()
    expect(mapLink!.attributes('href')).toBe('/master/map')
  })

  it('renders a badge count (8) for Pengajuan & Approval item', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppSidebar)
    expect(wrapper.html()).toContain('8')
  })

  it('clicking a parent group toggles its children visibility', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppSidebar)
    // Find the Master Data parent button
    const buttons = wrapper.findAll('button')
    const masterDataBtn = buttons.find(b => b.text().includes('Master Data'))
    expect(masterDataBtn).toBeDefined()

    // Initially Master Data children should be visible (default expanded)
    const htmlBefore = wrapper.html()
    expect(htmlBefore).toContain('Kantor')

    // Click to collapse
    await masterDataBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const htmlAfter = wrapper.html()
    // After collapse, Kantor child link should be gone
    expect(htmlAfter).not.toContain('/master/offices')
  })

  it('renders the logo wordmark Inventra when expanded', async () => {
    setupSuperadmin()
    useUiStore().sidebarCollapsed = false
    const wrapper = await mountSuspended(AppSidebar)
    expect(wrapper.html()).toContain('Inventra')
  })

  it('hides the wordmark when collapsed', async () => {
    setupSuperadmin()
    useUiStore().sidebarCollapsed = true
    const wrapper = await mountSuspended(AppSidebar)
    // In collapsed mode Inventra text inside the logo area should not be visible
    // The aside should NOT contain the wordmark span
    const wordmarks = wrapper.findAll('[data-wordmark]')
    expect(wordmarks).toHaveLength(0)
  })

  it('renders the user strip with name and role when expanded', async () => {
    setupSuperadmin()
    useUiStore().sidebarCollapsed = false
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    expect(html).toContain('Admin Inventra')
    expect(html).toContain('Superadmin')
  })

  it('renders user initials in the bottom avatar', async () => {
    setupSuperadmin()
    const wrapper = await mountSuspended(AppSidebar)
    // "Admin Inventra" -> initials "AI"
    expect(wrapper.html()).toContain('AI')
  })

  it('sizes the desktop rail to 264px when expanded', async () => {
    setupSuperadmin()
    useUiStore().sidebarCollapsed = false
    const wrapper = await mountSuspended(AppSidebar)
    const aside = wrapper.find('aside')
    // The rail width is now driven by responsive classes (the mobile drawer is a
    // fixed off-canvas panel); at lg the expanded rail is 264px wide.
    expect(aside.classes()).toContain('lg:w-[264px]')
    expect(aside.classes()).not.toContain('lg:w-[76px]')
  })

  it('sizes the desktop rail to 76px when collapsed', async () => {
    setupSuperadmin()
    useUiStore().sidebarCollapsed = true
    const wrapper = await mountSuspended(AppSidebar)
    const aside = wrapper.find('aside')
    expect(aside.classes()).toContain('lg:w-[76px]')
    expect(aside.classes()).not.toContain('lg:w-[264px]')
  })

  it('renders as an off-canvas drawer: hidden by default, slid in when mobileNavOpen', async () => {
    setupSuperadmin()
    const ui = useUiStore()
    ui.mobileNavOpen = false
    const wrapper = await mountSuspended(AppSidebar)
    const aside = wrapper.find('aside')
    // Base (mobile) layout: fixed panel translated off-screen while closed.
    expect(aside.classes()).toContain('fixed')
    expect(aside.classes()).toContain('-translate-x-full')
    // No scrim while closed.
    expect(wrapper.find('.bg-black\\/40').exists()).toBe(false)

    ui.mobileNavOpen = true
    await wrapper.vm.$nextTick()
    expect(aside.classes()).toContain('translate-x-0')
    expect(aside.classes()).not.toContain('-translate-x-full')
    // Scrim appears and closes the drawer on click.
    const scrim = wrapper.find('.bg-black\\/40')
    expect(scrim.exists()).toBe(true)
    await scrim.trigger('click')
    expect(ui.mobileNavOpen).toBe(false)
  })

  it('shows a label under the icon for a leaf item when collapsed', async () => {
    setupSuperadmin()
    useUiStore().sidebarCollapsed = true
    const wrapper = await mountSuspended(AppSidebar)
    // The Dasbor leaf keeps its label (rendered under the icon) while collapsed.
    const dasbor = wrapper.find('a[href="/"]')
    expect(dasbor.exists()).toBe(true)
    expect(dasbor.text()).toContain('Dasbor')
  })

  it('opens the sidebar and expands the group when a collapsed parent is clicked', async () => {
    setupSuperadmin()
    const ui = useUiStore()
    ui.sidebarCollapsed = true
    const wrapper = await mountSuspended(AppSidebar)
    // Collapsed: Master Data children are not rendered yet.
    expect(wrapper.html()).not.toContain('/master/offices')
    const masterData = wrapper.findAll('button').find(b => b.text().includes('Master Data'))
    expect(masterData).toBeDefined()
    await masterData!.trigger('click')
    await wrapper.vm.$nextTick()
    // Clicking a parent while collapsed expands the rail and opens the group.
    expect(ui.sidebarCollapsed).toBe(false)
    expect(wrapper.html()).toContain('/master/offices')
  })
})

describe('AppSidebar — live pending-approval badge (nav.approval)', () => {
  beforeEach(() => {
    useAuthStore().clear()
    useUiStore().sidebarCollapsed = false
    useInboxStore().pendingCount = 0
  })

  it('renders the inbox store pendingCount as the badge on the approval leaf', async () => {
    setupSuperadmin()
    useInboxStore().pendingCount = 3
    const wrapper = await mountSuspended(AppSidebar)
    const links = wrapper.findAll('a')
    const approvalLink = links.find(a => a.attributes('href') === '/approval')
    expect(approvalLink).toBeDefined()
    expect(approvalLink!.text()).toContain('3')
  })

  it('hides the badge on the approval leaf when pendingCount is 0', async () => {
    setupSuperadmin()
    useInboxStore().pendingCount = 0
    const wrapper = await mountSuspended(AppSidebar)
    const links = wrapper.findAll('a')
    const approvalLink = links.find(a => a.attributes('href') === '/approval')
    expect(approvalLink).toBeDefined()
    // No badge span should render for a 0 count
    expect(approvalLink!.find('.bg-error').exists()).toBe(false)
  })

  it('renders the inbox store pendingCount as the badge when the sidebar is collapsed', async () => {
    setupSuperadmin()
    useInboxStore().pendingCount = 5
    useUiStore().sidebarCollapsed = true
    const wrapper = await mountSuspended(AppSidebar)
    const links = wrapper.findAll('a')
    const approvalLink = links.find(a => a.attributes('href') === '/approval')
    expect(approvalLink).toBeDefined()
    expect(approvalLink!.text()).toContain('5')
  })
})

describe('AppSidebar — enumerated permissions (no wildcard)', () => {
  beforeEach(() => {
    useAuthStore().clear()
    useUiStore().sidebarCollapsed = false
  })

  it('renders the Master Data group when master-data permissions are enumerated without wildcard', async () => {
    // Backend returns specific keys, never '*', so can('*') would always be false.
    // The single per-permission nav must gate each item on its real permission.
    setupSuperadminEnumerated()
    const wrapper = await mountSuspended(AppSidebar)
    const html = wrapper.html()
    // A user holding masterdata.*.manage keys sees the "Master Data" group.
    expect(html).toContain('Master Data')
  })

  it('renders Kantor link (/master/offices) for enumerated superadmin', async () => {
    setupSuperadminEnumerated()
    const wrapper = await mountSuspended(AppSidebar)
    const links = wrapper.findAll('a')
    const kantorLink = links.find(a => a.attributes('href')?.includes('/master/offices'))
    expect(kantorLink).toBeDefined()
  })
})
