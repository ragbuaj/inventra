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

/**
 * Map office tipe → icon name + Nuxt UI semantic color name.
 * Use `color` directly with UBadge (color="primary" etc.).
 * buildOfficeTree converts color → semantic Tailwind token classes for TreeView.
 */
export const officeTipeMeta: Record<Office['tipe'], { icon: string, color: string }> = {
  pusat: { icon: 'i-lucide-landmark', color: 'primary' },
  kanwil: { icon: 'i-lucide-building-2', color: 'info' },
  cabang: { icon: 'i-lucide-building', color: 'warning' },
  unit: { icon: 'i-lucide-map-pin', color: 'neutral' }
}

const tipeBgClass: Record<string, string> = {
  primary: 'bg-primary/10',
  info: 'bg-info/10',
  warning: 'bg-warning/10',
  neutral: 'bg-neutral/10'
}

const tipeColorClass: Record<string, string> = {
  primary: 'text-primary',
  info: 'text-info',
  warning: 'text-warning',
  neutral: 'text-dimmed'
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
        iconBg: tipeBgClass[meta.color] ?? 'bg-neutral/10',
        iconColor: tipeColorClass[meta.color] ?? 'text-dimmed',
        inactive: !o.active,
        childCount: children.length || undefined,
        children: children.length ? children : undefined
      }
    })
  }
  return build(null)
}
