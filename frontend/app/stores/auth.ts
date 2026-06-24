import { defineStore } from 'pinia'
import type { AuthUser } from '~/types'

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
    clear() {
      this.accessToken = null
      this.user = null
      this.permissions = []
    }
  }
})
