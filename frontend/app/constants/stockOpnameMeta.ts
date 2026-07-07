import type { BadgeColor } from '~/types'

export type SessionStatus = 'open' | 'counting' | 'reconciling' | 'closed'
export type ItemResult = 'pending' | 'found' | 'not_found' | 'damaged' | 'misplaced'

export const SESSION_STATUS_KEYS: SessionStatus[] = ['open', 'counting', 'reconciling', 'closed']
export const ITEM_RESULT_KEYS: ItemResult[] = ['pending', 'found', 'not_found', 'damaged', 'misplaced']

export const SESSION_STATUS_TONE: Record<SessionStatus, BadgeColor> = {
  open: 'neutral',
  counting: 'info',
  reconciling: 'warning',
  closed: 'success'
}

export const ITEM_RESULT_TONE: Record<ItemResult, BadgeColor> = {
  pending: 'neutral',
  found: 'success',
  not_found: 'error',
  damaged: 'warning',
  misplaced: 'primary'
}

export const RESULT_ACTION: Record<'not_found' | 'damaged' | 'misplaced', 'disposal' | 'maintenance' | 'transfer'> = {
  not_found: 'disposal',
  misplaced: 'transfer',
  damaged: 'maintenance'
}
