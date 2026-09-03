import { defineStore } from 'pinia'
import type { AuthUser } from '~/types'
import { clearListStateCache } from '~/composables/useListStateCache'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    accessToken: null as string | null,
    user: null as AuthUser | null,
    permissions: [] as string[]
  }),
  getters: {
    isAuthenticated: state => !!state.accessToken
  },
  actions: {
    setSession(token: string, user: AuthUser, permissions: string[]) {
      this.accessToken = token
      this.user = user
      this.permissions = permissions
    },
    setToken(token: string) {
      this.accessToken = token
    },
    /**
     * Ends the session. Every path that drops a session goes through here —
     * explicit logout, a failed token refresh, a password reset — so the
     * cached list snapshots are cleared here too rather than at each call
     * site. Those snapshots hold real rows (asset values, user records) and
     * this is a SPA: without this, the next person to log in on the same tab
     * would be handed the previous user's rows from memory.
     */
    clear() {
      this.accessToken = null
      this.user = null
      this.permissions = []
      clearListStateCache()
    }
  }
})
