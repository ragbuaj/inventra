export default defineNuxtPlugin(async () => {
  const auth = useAuthStore()
  if (auth.isAuthenticated) return
  // The refresh token is an HttpOnly cookie JS cannot read, so attempt a refresh
  // unconditionally; a 401 simply means the user is not logged in.
  const authApi = useAuthApi()
  try {
    if (await authApi.refresh()) await authApi.fetchMe()
  } catch {
    // Stay logged out.
  }
})
