export function useRefreshCookie() {
  return useCookie<string | null>('inventra_refresh', {
    sameSite: 'lax',
    secure: !import.meta.dev
  })
}
