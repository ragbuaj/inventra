import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUiStore } from '~/stores/ui'

describe('stores/ui', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts with the rail expanded and the mobile drawer closed', () => {
    const ui = useUiStore()
    expect(ui.sidebarCollapsed).toBe(false)
    expect(ui.mobileNavOpen).toBe(false)
  })

  it('toggleSidebar flips only the desktop rail flag', () => {
    const ui = useUiStore()
    ui.toggleSidebar()
    expect(ui.sidebarCollapsed).toBe(true)
    expect(ui.mobileNavOpen).toBe(false)
    ui.toggleSidebar()
    expect(ui.sidebarCollapsed).toBe(false)
  })

  it('openMobileNav / closeMobileNav drive the drawer without touching the rail', () => {
    const ui = useUiStore()
    ui.sidebarCollapsed = true
    ui.openMobileNav()
    expect(ui.mobileNavOpen).toBe(true)
    expect(ui.sidebarCollapsed).toBe(true)
    ui.closeMobileNav()
    expect(ui.mobileNavOpen).toBe(false)
    expect(ui.sidebarCollapsed).toBe(true)
  })

  it('toggleMobileNav flips the drawer flag each call', () => {
    const ui = useUiStore()
    expect(ui.mobileNavOpen).toBe(false)
    ui.toggleMobileNav()
    expect(ui.mobileNavOpen).toBe(true)
    ui.toggleMobileNav()
    expect(ui.mobileNavOpen).toBe(false)
  })

  it('the two flags are independent (drawer open + rail collapsed can coexist)', () => {
    const ui = useUiStore()
    ui.toggleSidebar()
    ui.openMobileNav()
    expect(ui.sidebarCollapsed).toBe(true)
    expect(ui.mobileNavOpen).toBe(true)
  })
})
