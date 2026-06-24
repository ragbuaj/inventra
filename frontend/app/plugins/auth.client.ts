export default defineNuxtPlugin(async () => {
  const auth = useAuthStore()
  const refresh = useRefreshCookie()
  if (!auth.isAuthenticated && refresh.value) {
    try {
      const ok = await useApiClient().refreshToken()
      if (ok) {
        await useAuthApi().fetchMe()
      }
    } catch {
      // Failed rehydration — stay logged out
    }
  }
})
