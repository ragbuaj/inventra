export function useCan() {
  const auth = useAuthStore()
  return (permission: string): boolean => {
    if (auth.permissions.includes('*')) return true
    return auth.permissions.includes(permission)
  }
}
