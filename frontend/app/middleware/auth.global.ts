export default defineNuxtRouteMiddleware((to) => {
  const auth = useAuthStore()
  const publicPaths = ['/login']
  const path = to.path.replace(/^\/(en)(?=\/|$)/, '') || '/'
  if (publicPaths.includes(path)) {
    if (auth.isAuthenticated && path === '/login') return navigateTo('/')
    return
  }
  if (!auth.isAuthenticated) {
    return navigateTo('/login')
  }
})
