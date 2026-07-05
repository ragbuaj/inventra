import type { BadgeColor } from '~/types'

/** Row status of a transfer (mutasi) as tracked in the transfer inbox/history. */
export type TransferRowStatus = 'approved' | 'in_transit' | 'received' | 'returned'
/** Physical condition recorded when an asset is shipped/received. */
export type TransferCondition = 'baik' | 'rusak_ringan' | 'rusak_berat'

export const TRANSFER_STATUS_TONE: Record<TransferRowStatus, BadgeColor> = {
  approved: 'info',
  in_transit: 'info',
  received: 'success',
  returned: 'error'
}

export const CONDITION_TONE: Record<TransferCondition, BadgeColor> = {
  baik: 'success',
  rusak_ringan: 'warning',
  rusak_berat: 'error'
}

export const CONDITION_KEYS: TransferCondition[] = ['baik', 'rusak_ringan', 'rusak_berat']
