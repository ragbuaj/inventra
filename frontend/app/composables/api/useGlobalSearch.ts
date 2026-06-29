import type { SearchGroup, SearchItem, SearchEntityType } from '~/types'
import { fakeLatency } from '~/mock/helpers'
import { assetStore } from '~/mock/assets'
import { employeeStore } from '~/mock/employees'
import { officeStore } from '~/mock/offices'
import { userStore } from '~/mock/users'
import { approvalStore } from '~/mock/approval'

const ORDER: SearchEntityType[] = ['aset', 'pegawai', 'kantor', 'user', 'pengajuan']
const ICON: Record<SearchEntityType, string> = {
  aset: 'i-lucide-package',
  pegawai: 'i-lucide-user',
  kantor: 'i-lucide-building',
  user: 'i-lucide-shield',
  pengajuan: 'i-lucide-check-square'
}

function match(q: string, ...fields: (string | null | undefined)[]): boolean {
  return fields.some(f => String(f ?? '').toLowerCase().includes(q))
}

export function useGlobalSearch() {
  async function search(query: string): Promise<SearchGroup[]> {
    const q = query.trim().toLowerCase()
    if (!q) return []
    await fakeLatency(220)

    const byType: Record<SearchEntityType, SearchItem[]> = {
      aset: [], pegawai: [], kantor: [], user: [], pengajuan: []
    }

    for (const a of assetStore.all()) {
      if (match(q, a.nama, a.tag)) {
        byType.aset.push({ type: 'aset', title: a.nama, sub: a.tag, status: a.status, icon: ICON.aset, to: `/assets/${a.tag}` })
      }
    }
    for (const e of employeeStore.all()) {
      if (match(q, e.name, e.code)) {
        byType.pegawai.push({ type: 'pegawai', title: e.name, sub: e.code, status: null, icon: ICON.pegawai, to: '/master/employees' })
      }
    }
    for (const o of officeStore.all()) {
      if (match(q, o.nama, o.kode, o.kota)) {
        byType.kantor.push({ type: 'kantor', title: o.nama, sub: `${o.kode} · ${o.provinsi}`, status: o.active ? 'aktif' : null, icon: ICON.kantor, to: '/master/offices' })
      }
    }
    for (const u of userStore.all()) {
      if (match(q, u.nama, u.email)) {
        byType.user.push({ type: 'user', title: u.email, sub: `${u.peran} · ${u.kantor}`, status: null, icon: ICON.user, to: '/settings/users' })
      }
    }
    for (const r of approvalStore.all()) {
      if (match(q, r.judul, r.id)) {
        byType.pengajuan.push({ type: 'pengajuan', title: r.judul, sub: r.id, status: r.status, icon: ICON.pengajuan, to: '/approval' })
      }
    }

    return ORDER
      .filter(t => byType[t].length > 0)
      .map(t => ({ type: t, labelKey: `search.group.${t}`, total: byType[t].length, items: byType[t].slice(0, 5) }))
  }

  return { search }
}
