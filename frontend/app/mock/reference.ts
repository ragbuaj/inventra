import type { ReferenceRow } from '~/types'
import type { ReferenceKey } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'
import { createStore } from './helpers'

const seeds: Partial<Record<ReferenceKey, ReferenceRow[]>> = {
  provinces: [
    { id: 'p-1', name: 'DKI Jakarta', code: '31' },
    { id: 'p-2', name: 'Jawa Barat', code: '32' }
  ],
  cities: [
    { id: 'c-1', name: 'Jakarta Selatan', code: '3171' },
    { id: 'c-2', name: 'Bandung', code: '3273' }
  ],
  units: [
    { id: 'u-1', name: 'Unit', symbol: 'pcs' },
    { id: 'u-2', name: 'Set', symbol: 'set' }
  ],
  brands: [
    { id: 'b-1', name: 'Dell' },
    { id: 'b-2', name: 'HP' }
  ],
  vendors: [
    { id: 'v-1', name: 'PT Sumber Jaya', email: 'sales@sumberjaya.co.id', phone: '021-5550001' }
  ]
}

function makeStore(key: ReferenceKey) {
  const seed = seeds[key] ?? [{ id: `${key}-1`, name: `${key} contoh` }]
  return createStore<ReferenceRow>(seed)
}

export const referenceStores = Object.fromEntries(
  referenceResources.map(r => [r.key, makeStore(r.key)])
) as Record<ReferenceKey, ReturnType<typeof makeStore>>
