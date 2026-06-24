export default defineNuxtRouteMiddleware((to) => {
  const permission = to.meta.permission as string | undefined
  if (!permission) return
  const can = useCan()
  if (!can(permission)) {
    return abortNavigation({ statusCode: 403, statusMessage: 'Akses ditolak' })
  }
})
