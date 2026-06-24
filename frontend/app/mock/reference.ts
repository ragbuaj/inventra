import type { ReferenceRow } from '~/types'
import type { ReferenceKey } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'
import { createStore } from './helpers'

const seeds: Partial<Record<ReferenceKey, ReferenceRow[]>> = {
  provinces: [
    { id: 'p-1', name: 'DKI Jakarta', code: '31', active: true },
    { id: 'p-2', name: 'Jawa Barat', code: '32', active: true }
  ],
  cities: [
    { id: 'c-1', name: 'Jakarta Selatan', code: '3171', active: true },
    { id: 'c-2', name: 'Bandung', code: '3273', active: true }
  ],
  units: [
    { id: 'u-1', name: 'Unit', symbol: 'pcs', active: true },
    { id: 'u-2', name: 'Set', symbol: 'set', active: true }
  ],
  brands: [
    { id: 'b-1', name: 'Dell', active: true },
    { id: 'b-2', name: 'HP', active: false }
  ],
  vendors: [
    { id: 'v-1', name: 'PT Sumber Jaya', email: 'sales@sumberjaya.co.id', phone: '021-5550001', active: true }
  ]
}

function makeStore(key: ReferenceKey) {
  const seed = seeds[key] ?? [{ id: `${key}-1`, name: `${key} contoh`, active: true }]
  return createStore<ReferenceRow>(seed)
}

export const referenceStores = Object.fromEntries(
  referenceResources.map(r => [r.key, makeStore(r.key)])
) as Record<ReferenceKey, ReturnType<typeof makeStore>>
