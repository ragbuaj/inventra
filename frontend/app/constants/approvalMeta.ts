import type { BadgeColor } from '~/types'

/** Backend shared.request_type values that currently have a submit path. */
export type RequestType = 'asset_create' | 'asset_disposal' | 'asset_transfer' | 'assignment' | 'maintenance' | 'valuation_exclusion'
/** Backend shared.request_status values. */
export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'

export const REQUEST_TYPE_KEYS: RequestType[] = ['asset_create', 'asset_disposal', 'asset_transfer', 'assignment', 'maintenance', 'valuation_exclusion']

export const TYPE_META: Record<RequestType, { icon: string, tone: BadgeColor, sensitive: boolean }> = {
  asset_create: { icon: 'i-lucide-package', tone: 'info', sensitive: false },
  asset_disposal: { icon: 'i-lucide-trash-2', tone: 'error', sensitive: true },
  asset_transfer: { icon: 'i-lucide-arrow-right-left', tone: 'primary', sensitive: false },
  assignment: { icon: 'i-lucide-hand', tone: 'info', sensitive: false },
  maintenance: { icon: 'i-lucide-wrench', tone: 'warning', sensitive: false },
  valuation_exclusion: { icon: 'i-lucide-coins', tone: 'warning', sensitive: true }
}

export const STATUS_TONE: Record<RequestStatus, BadgeColor> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
  cancelled: 'neutral'
}

export const STATUS_FILTERS: (RequestStatus | 'all')[] = ['pending', 'approved', 'rejected', 'cancelled', 'all']
