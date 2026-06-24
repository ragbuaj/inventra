export function useApiClient() {
  const config = useRuntimeConfig()
  const auth = useAuthStore()
  const toast = useToast()
  const base = config.public.apiBase as string

  async function refreshToken(): Promise<boolean> {
    const refresh = useCookie<string | null>('inventra_refresh')
    if (!refresh.value) return false
    try {
      const res = await $fetch<{ access_token: string }>(`${base}/auth/refresh`, {
        method: 'POST',
        body: { refresh_token: refresh.value }
      })
      auth.setToken(res.access_token)
      return true
    } catch {
      return false
    }
  }

  async function request<T>(path: string, opts: Record<string, unknown> = {}): Promise<T> {
    const headers: Record<string, string> = { ...(opts.headers as Record<string, string> || {}) }
    if (auth.accessToken) headers.Authorization = `Bearer ${auth.accessToken}`
    try {
      return await $fetch<T>(`${base}${path}`, { ...opts, headers })
    } catch (err: unknown) {
      const status = (err as { statusCode?: number }).statusCode
      if (status === 401 && await refreshToken()) {
        headers.Authorization = `Bearer ${auth.accessToken}`
        return await $fetch<T>(`${base}${path}`, { ...opts, headers })
      }
      if (status === 401) {
        auth.clear()
        await navigateTo('/login')
      } else {
        toast.add({ title: 'Terjadi kesalahan', description: String(status ?? ''), color: 'error' })
      }
      throw err
    }
  }

  return { request, refreshToken }
}
