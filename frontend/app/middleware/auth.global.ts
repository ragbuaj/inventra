export default defineNuxtRouteMiddleware((to) => {
  const auth = useAuthStore()
  // Informational pages (privacy / usage guide / FAQ) are intentionally public
  // so they stay reachable without a session — e.g. the privacy-policy URL the
  // Play Store listing requires. Signed-in users still reach them via the
  // sidebar and user menu; the `info` layout adapts its chrome to auth state.
  const publicPaths = ['/login', '/forgot-password', '/reset-password', '/privacy', '/guide', '/faq']
  const path = to.path.replace(/^\/(en)(?=\/|$)/, '') || '/'
  if (publicPaths.includes(path)) {
    if (auth.isAuthenticated && path === '/login') return navigateTo('/')
    return
  }
  if (!auth.isAuthenticated) {
    return navigateTo('/login')
  }
})
