export default defineNuxtPlugin(async () => {
  const auth = useAuthStore()
  const refresh = useRefreshCookie()
  if (auth.isAuthenticated || !refresh.value) return
  // Construct the auth API synchronously, before any `await` — the composables
  // it builds rely on the Nuxt instance context, which is lost after the first
  // await in a plugin.
  const authApi = useAuthApi()
  try {
    const ok = await authApi.refresh()
    if (ok) {
      await authApi.fetchMe()
    }
  } catch {
    // Failed rehydration — stay logged out
  }
})
