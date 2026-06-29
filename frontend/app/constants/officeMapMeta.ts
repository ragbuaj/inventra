import type { OfficeTier } from '~/types'

/**
 * Office tier → i18n label key, pin CSS var, and soft badge classes.
 * 3 buckets (office_types.tier): pusat / wilayah / office (Cabang).
 */
export const tierMeta: Record<OfficeTier, {
  labelKey: string
  pinVar: string
  softBg: string
  softText: string
  icon: string
}> = {
  pusat: { labelKey: 'map.tier.pusat', pinVar: '--pin-pusat', softBg: 'bg-primary/10', softText: 'text-primary', icon: 'i-lucide-landmark' },
  wilayah: { labelKey: 'map.tier.wilayah', pinVar: '--pin-wilayah', softBg: 'bg-info/10', softText: 'text-info', icon: 'i-lucide-building-2' },
  office: { labelKey: 'map.tier.office', pinVar: '--pin-cabang', softBg: 'bg-warning/10', softText: 'text-warning', icon: 'i-lucide-building' }
}

export const TIER_ORDER: OfficeTier[] = ['pusat', 'wilayah', 'office']
