import type { Office, TreeNode } from '~/types'
import { createStore } from './helpers'

export const officeSeed: Office[] = [
  { id: 'o-pusat', nama: 'Kantor Pusat', kode: 'PST', tipe: 'pusat', parent_id: null, provinsi: 'DKI Jakarta', kota: 'Jakarta Pusat', alamat: 'Jl. Merdeka No. 1', created_at: '2026-01-02' },
  { id: 'o-jkt', nama: 'Kanwil Jakarta', kode: 'JKT01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Sudirman No. 10', created_at: '2026-01-03' },
  { id: 'o-jkt-a', nama: 'Cabang Kebayoran', kode: 'JKT01-A', tipe: 'cabang', parent_id: 'o-jkt', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Kebayoran No. 5', created_at: '2026-01-04' },
  { id: 'o-bdg', nama: 'Kanwil Bandung', kode: 'BDG01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'Jawa Barat', kota: 'Bandung', alamat: 'Jl. Asia Afrika No. 8', created_at: '2026-01-05' }
]

export const officeStore = createStore<Office>(officeSeed)

export function buildOfficeTree(offices: Office[]): TreeNode[] {
  const byParent = new Map<string | null, Office[]>()
  for (const o of offices) {
    const list = byParent.get(o.parent_id) ?? []
    list.push(o)
    byParent.set(o.parent_id, list)
  }
  function build(parentId: string | null): TreeNode[] {
    return (byParent.get(parentId) ?? []).map((o) => {
      const children = build(o.id)
      return {
        id: o.id,
        label: o.nama,
        icon: 'i-lucide-building-2',
        childCount: children.length || undefined,
        children: children.length ? children : undefined
      }
    })
  }
  return build(null)
}
