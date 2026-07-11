// @vitest-environment nuxt
import { describe, expect, it, vi, beforeEach } from 'vitest'

const requestMock = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request: requestMock }) }))

// eslint-disable-next-line import/first
import { useGlobalSearch } from '~/composables/api/useGlobalSearch'

describe('useGlobalSearch (real API)', () => {
  beforeEach(() => {
    requestMock.mockReset()
  })

  it('does not call the API for queries under 2 chars', async () => {
    const { search } = useGlobalSearch()
    expect(await search('')).toEqual([])
    expect(await search(' a ')).toEqual([])
    expect(requestMock).not.toHaveBeenCalled()
  })

  it('maps asset groups to SearchGroup with route/icon/labelKey', async () => {
    requestMock.mockResolvedValue({ groups: [{ type: 'assets', total: 7, items: [
      { id: '1', title: 'Laptop Dell', subtitle: 'JKT01-X', status: 'available', asset_tag: 'JKT01-X' }
    ] }] })
    const { search } = useGlobalSearch()
    const groups = await search('laptop')
    expect(requestMock).toHaveBeenCalledWith('/search?q=laptop')
    expect(groups).toHaveLength(1)
    expect(groups[0]).toMatchObject({ type: 'aset', labelKey: 'search.group.aset', total: 7 })
    expect(groups[0]!.items[0]).toMatchObject({
      type: 'aset', title: 'Laptop Dell', sub: 'JKT01-X',
      status: 'available', icon: 'i-lucide-package', to: '/assets/JKT01-X'
    })
  })

  it('composes the requests title from type + office via i18n', async () => {
    requestMock.mockResolvedValue({ groups: [{ type: 'requests', total: 1, items: [
      { id: 'abc12345-0000', title: 'Cabang Jakarta', subtitle: 'abc12345', status: 'pending', request_type: 'asset_create' }
    ] }] })
    const { search } = useGlobalSearch()
    const groups = await search('beli')
    expect(groups[0]!.type).toBe('pengajuan')
    expect(groups[0]!.items[0]!.title).toContain('Cabang Jakarta')
    expect(groups[0]!.items[0]!.title).not.toContain('approval.type')
    expect(groups[0]!.items[0]!.to).toBe('/approval')
  })

  it('maps employees/offices/users to list routes with null status', async () => {
    requestMock.mockResolvedValue({ groups: [
      { type: 'employees', total: 1, items: [{ id: 'e1', title: 'Budi', subtitle: 'EMP1', status: null }] },
      { type: 'offices', total: 1, items: [{ id: 'o1', title: 'KC Jakarta', subtitle: 'JKT01', status: null }] },
      { type: 'users', total: 1, items: [{ id: 'u1', title: 'Admin', subtitle: 'admin@x.id', status: null }] }
    ] })
    const { search } = useGlobalSearch()
    const groups = await search('ja')
    expect(groups.map(g => g.type)).toEqual(['pegawai', 'kantor', 'user'])
    expect(groups.map(g => g.items[0]!.to)).toEqual(['/master/employees', '/master/offices', '/settings/users'])
  })

  it('returns [] when the API returns no groups', async () => {
    requestMock.mockResolvedValue({ groups: [] })
    const { search } = useGlobalSearch()
    expect(await search('zzz')).toEqual([])
  })

  it('encodes the query', async () => {
    requestMock.mockResolvedValue({ groups: [] })
    const { search } = useGlobalSearch()
    await search('a b&c')
    expect(requestMock).toHaveBeenCalledWith(`/search?q=${encodeURIComponent('a b&c')}`)
  })
})
