import type { BadgeColor } from '~/types'

export type AssignmentStatus = 'active' | 'returned'
export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'
export type AssetCondition = 'baik' | 'ringan' | 'berat'

export const ASSIGNMENT_STATUS_TONE: Record<AssignmentStatus, BadgeColor> = {
  active: 'info',
  returned: 'neutral'
}

export const REQUEST_STATUS_TONE: Record<RequestStatus, BadgeColor> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
  cancelled: 'neutral'
}

export const CONDITION_TONE: Record<AssetCondition, BadgeColor> = {
  baik: 'success',
  ringan: 'warning',
  berat: 'error'
}

export const CONDITION_KEYS: AssetCondition[] = ['baik', 'ringan', 'berat']

const MONTHS_ID = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

/** Formats an ISO date/datetime string as "6 Jul 2026"; empty input → "". */
export function formatDateID(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getDate()} ${MONTHS_ID[d.getMonth()]} ${d.getFullYear()}`
}
