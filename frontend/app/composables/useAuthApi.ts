import type { AuthUser } from '~/types'

// Shape returned by GET /auth/me (openapi.yaml User schema).
// role_name is NOT in the backend response; AuthUser.role_name is populated as ''.
interface MeResponse {
  id: string
  name: string
  email: string
  role_id: string
  office_id: string | null
  employee_id: string | null
  status: string
  avatar_url: string | null
  google_linked: boolean
  created_at: string | null
  updated_at: string | null
}

export function useAuthApi() {
  const config = useRuntimeConfig()
  const auth = useAuthStore()
  const base = config.public.apiBase as string
  // Build the API client synchronously here, during setup. The Nuxt composables
  // it relies on (useRuntimeConfig/useNuxtApp) must be called before the first
  // `await`, or the Nuxt instance context is lost and they throw — which
  // previously surfaced as a bogus "connection error" right after a successful
  // login, when fetchMe() built the client after awaiting /auth/login.
  const client = useApiClient()

  async function login(email: string, password: string): Promise<void> {
    const res = await $fetch<{ access_token: string }>(`${base}/auth/login`, {
      method: 'POST',
      body: { email, password },
      credentials: 'include'
    })
    auth.setToken(res.access_token)
    await fetchMe()
  }

  async function fetchMe(): Promise<void> {
    const me = await client.request<MeResponse>('/auth/me')
    const perms = await client.request<{ permissions: string[] }>('/auth/permissions')
    const user: AuthUser = {
      id: me.id,
      name: me.name,
      email: me.email,
      role_id: me.role_id,
      role_name: '',
      office_id: me.office_id,
      employee_id: me.employee_id
    }
    auth.setSession(auth.accessToken as string, user, perms.permissions)
  }

  async function logout(): Promise<void> {
    try {
      await client.request('/auth/logout', { method: 'POST', credentials: 'include' })
    } finally {
      auth.clear()
      await navigateTo('/login')
    }
  }

  function refresh(): Promise<boolean> {
    return client.refreshToken()
  }

  return { login, fetchMe, logout, refresh }
}
