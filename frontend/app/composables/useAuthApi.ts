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
  const refreshCookie = useCookie<string | null>('inventra_refresh', { sameSite: 'lax' })

  async function login(email: string, password: string): Promise<void> {
    const res = await $fetch<{ access_token: string, refresh_token: string }>(`${base}/auth/login`, {
      method: 'POST',
      body: { email, password }
    })
    auth.setToken(res.access_token)
    refreshCookie.value = res.refresh_token
    await fetchMe()
  }

  async function fetchMe(): Promise<void> {
    const client = useApiClient()
    const me = await client.request<MeResponse>('/auth/me')
    const perms = await client.request<{ permissions: string[] }>('/auth/permissions')
    const user: AuthUser = {
      id: me.id,
      name: me.name,
      email: me.email,
      role_id: me.role_id,
      role_name: ''
    }
    auth.setSession(auth.accessToken as string, user, perms.permissions)
  }

  async function logout(): Promise<void> {
    try {
      await useApiClient().request('/auth/logout', {
        method: 'POST',
        body: { refresh_token: refreshCookie.value ?? undefined }
      })
    } finally {
      auth.clear()
      refreshCookie.value = null
      await navigateTo('/login')
    }
  }

  return { login, fetchMe, logout }
}
