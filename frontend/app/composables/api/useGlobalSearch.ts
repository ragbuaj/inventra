import type { SearchGroup, SearchItem, SearchEntityType } from '~/types'

type ApiGroupType = 'assets' | 'employees' | 'offices' | 'users' | 'requests'

interface ApiSearchItem {
  id: string
  title: string
  subtitle: string
  status: string | null
  asset_tag?: string
  request_type?: string
}

interface ApiSearchGroup {
  type: ApiGroupType
  total: number
  items: ApiSearchItem[]
}

const TYPE_MAP: Record<ApiGroupType, SearchEntityType> = {
  assets: 'aset',
  employees: 'pegawai',
  offices: 'kantor',
  users: 'user',
  requests: 'pengajuan'
}

const ICON: Record<SearchEntityType, string> = {
  aset: 'i-lucide-package',
  pegawai: 'i-lucide-user',
  kantor: 'i-lucide-building',
  user: 'i-lucide-shield',
  pengajuan: 'i-lucide-check-square'
}

const ROUTE: Record<SearchEntityType, string> = {
  aset: '/assets',
  pegawai: '/master/employees',
  kantor: '/master/offices',
  user: '/settings/users',
  pengajuan: '/approval'
}

export function useGlobalSearch() {
  const { request } = useApiClient()
  // useI18n() requires an active component instance (getCurrentInstance());
  // this composable can be called from plain code (as this file's own spec
  // does) so resolve `t` off the nuxt app instance instead — same pattern as
  // useApiClient.ts's notifyError().
  const { t } = useNuxtApp().$i18n as { t: (key: string) => string }

  // Requests items carry the office name as `title` and the enum as
  // `request_type` (see internal/search) — compose the display title the same
  // way approval.vue's rowTitle() does: `${approval.type.<type>} · ${office}`.
  function itemTitle(type: SearchEntityType, it: ApiSearchItem): string {
    if (type !== 'pengajuan') return it.title
    const label = it.request_type ? t(`approval.type.${it.request_type}`) : ''
    return it.title ? `${label} · ${it.title}` : label
  }

  function itemTo(type: SearchEntityType, it: ApiSearchItem): string {
    if (type === 'aset' && it.asset_tag) return `/assets/${it.asset_tag}`
    return ROUTE[type]
  }

  async function search(query: string): Promise<SearchGroup[]> {
    const q = query.trim()
    if (q.length < 2) return []
    const res = await request<{ groups: ApiSearchGroup[] }>(`/search?q=${encodeURIComponent(q)}`)
    return (res.groups ?? []).map((g) => {
      const type = TYPE_MAP[g.type]
      return {
        type,
        labelKey: `search.group.${type}`,
        total: g.total,
        items: g.items.map<SearchItem>(it => ({
          type,
          title: itemTitle(type, it),
          sub: it.subtitle,
          status: it.status,
          icon: ICON[type],
          to: itemTo(type, it)
        }))
      }
    })
  }

  return { search }
}
