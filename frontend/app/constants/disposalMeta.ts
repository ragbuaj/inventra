import type { BadgeColor } from '~/types'

/** Backend shared.disposal_method values. */
export type DisposalMethod = 'sale' | 'auction' | 'donation' | 'write_off'

export const METHOD_KEYS: DisposalMethod[] = ['sale', 'auction', 'donation', 'write_off']

export const METHOD_TONE: Record<DisposalMethod, BadgeColor> = {
  sale: 'info',
  auction: 'primary',
  donation: 'success',
  write_off: 'neutral'
}
