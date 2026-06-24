export function useApiClient() {
  const config = useRuntimeConfig()
  const auth = useAuthStore()
  const nuxtApp = useNuxtApp()
  const base = config.public.apiBase as string

  // Best-effort generic error toast. Resolved lazily and inside Nuxt context:
  // request() runs after an `await` (component setup context is gone) and the
  // client may also be built outside a component (e.g. the rehydration plugin),
  // where useToast()/useI18n() would throw "Must be called at the top of a
  // setup function". Any failure here is swallowed — never mask the real error.
  function notifyError(status?: number) {
    try {
      nuxtApp.runWithContext(() => {
        const t = (nuxtApp.$i18n as { t: (key: string) => string } | undefined)?.t
        useToast().add({
          title: t ? t('common.error') : 'Error',
          description: String(status ?? ''),
          color: 'error'
        })
      })
    } catch {
      // No UI context available (e.g. during plugin init) — skip the toast.
    }
  }

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
        await nuxtApp.runWithContext(() => navigateTo('/login'))
      } else {
        notifyError(status)
      }
      throw err
    }
  }

  return { request, refreshToken }
}
