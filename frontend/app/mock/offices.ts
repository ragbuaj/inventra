import type { Office, TreeNode } from '~/types'
import { createStore } from './helpers'

export const officeSeed: Office[] = [
  { id: 'o-pusat', nama: 'Kantor Pusat', kode: 'PST', tipe: 'pusat', parent_id: null, provinsi: 'DKI Jakarta', kota: 'Jakarta Pusat', alamat: 'Jl. Merdeka No. 1', active: true, created_at: '2026-01-02' },
  { id: 'o-jkt', nama: 'Kanwil Jakarta', kode: 'JKT01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Sudirman No. 10', active: true, created_at: '2026-01-03' },
  { id: 'o-jkt-a', nama: 'Cabang Kebayoran', kode: 'JKT01-A', tipe: 'cabang', parent_id: 'o-jkt', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Kebayoran No. 5', active: true, created_at: '2026-01-04' },
  { id: 'o-bdg', nama: 'Kanwil Bandung', kode: 'BDG01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'Jawa Barat', kota: 'Bandung', alamat: 'Jl. Asia Afrika No. 8', active: true, created_at: '2026-01-05' },
  { id: 'o-bdg-a', nama: 'Cabang Cimahi', kode: 'BDG01-A', tipe: 'cabang', parent_id: 'o-bdg', provinsi: 'Jawa Barat', kota: 'Cimahi', alamat: 'Jl. Cimahi Raya No. 3', active: false, created_at: '2026-01-06' }
]

export const officeStore = createStore<Office>(officeSeed)

/** Map office tipe → tree icon + bg/color tokens for the colored type badge */
export const officeTipeMeta: Record<Office['tipe'], { icon: string, bg: string, color: string }> = {
  pusat: { icon: 'i-lucide-landmark', bg: 'bg-primary/10', color: 'text-primary' },
  kanwil: { icon: 'i-lucide-building-2', bg: 'bg-blue-500/10', color: 'text-blue-600' },
  cabang: { icon: 'i-lucide-building', bg: 'bg-amber-500/10', color: 'text-amber-600' },
  unit: { icon: 'i-lucide-map-pin', bg: 'bg-purple-500/10', color: 'text-purple-600' }
}

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
      const meta = officeTipeMeta[o.tipe]
      return {
        id: o.id,
        label: o.nama,
        icon: meta.icon,
        iconBg: meta.bg,
        iconColor: meta.color,
        inactive: !o.active,
        childCount: children.length || undefined,
        children: children.length ? children : undefined
      }
    })
  }
  return build(null)
}
