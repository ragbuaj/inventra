export function useApiClient() {
  const config = useRuntimeConfig()
  const auth = useAuthStore()
  const toast = useToast()
  const { t } = useI18n()
  const base = config.public.apiBase as string

  async function refreshToken(): Promise<boolean> {
    const refresh = useRefreshCookie()
    if (!refresh.value) return false
    try {
      const res = await $fetch<{ access_token: string, refresh_token: string }>(`${base}/auth/refresh`, {
        method: 'POST',
        body: { refresh_token: refresh.value }
      })
      auth.setToken(res.access_token)
      refresh.value = res.refresh_token
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
        useRefreshCookie().value = null
        await navigateTo('/login')
      } else {
        toast.add({ title: t('common.error'), description: String(status ?? ''), color: 'error' })
      }
      throw err
    }
  }

  return { request, refreshToken }
}
