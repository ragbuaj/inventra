export default defineNuxtRouteMiddleware((to) => {
  const permission = to.meta.permission as string | string[] | undefined
  if (!permission) return
  const can = useCan()
  const allowed = Array.isArray(permission)
    ? permission.some(p => can(p))
    : can(permission)
  if (!allowed) {
    return abortNavigation({ statusCode: 403, statusMessage: 'Akses ditolak' })
  }
})
