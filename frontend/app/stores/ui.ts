import { defineStore } from 'pinia'

export const useUiStore = defineStore('ui', {
  state: () => ({
    // Desktop rail collapse (lg+). Persisted implicitly by user interaction.
    sidebarCollapsed: false,
    // Mobile off-canvas drawer (<lg). Independent from the desktop rail so the
    // two breakpoints never fight over the same flag.
    mobileNavOpen: false
  }),
  actions: {
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    },
    openMobileNav() {
      this.mobileNavOpen = true
    },
    closeMobileNav() {
      this.mobileNavOpen = false
    },
    toggleMobileNav() {
      this.mobileNavOpen = !this.mobileNavOpen
    }
  }
})
