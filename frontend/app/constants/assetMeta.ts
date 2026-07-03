import type { AssetStatus, AssetClass } from '~/types'

export const ASSET_STATUSES: AssetStatus[] = ['available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost']
export const ASSET_CLASSES: AssetClass[] = ['tangible', 'intangible']

// statusMeta[s] = { labelKey: `assets.status.${s}`, color: <Nuxt UI badge color> }
export const statusMeta: Record<AssetStatus, { labelKey: string, color: 'success' | 'info' | 'warning' | 'error' | 'neutral' }> = {
  available: { labelKey: 'assets.status.available', color: 'success' },
  assigned: { labelKey: 'assets.status.assigned', color: 'info' },
  under_maintenance: { labelKey: 'assets.status.under_maintenance', color: 'warning' },
  in_transfer: { labelKey: 'assets.status.in_transfer', color: 'info' },
  retired: { labelKey: 'assets.status.retired', color: 'neutral' },
  disposed: { labelKey: 'assets.status.disposed', color: 'neutral' },
  lost: { labelKey: 'assets.status.lost', color: 'error' }
}

export const classMeta: Record<AssetClass, { labelKey: string }> = {
  tangible: { labelKey: 'assets.class.tangible' },
  intangible: { labelKey: 'assets.class.intangible' }
}
