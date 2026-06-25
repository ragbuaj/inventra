import type { Asset, ListQuery, Paginated } from '~/types'
import { fakeLatency, filterBy, paginate } from '~/mock/helpers'
import { assetStore } from '~/mock/assets'

export interface AssetInput {
  tag: string
  nama: string
  kategori: string
  brand: string
  status: Asset['status']
  kantor: string
  lokasi: string
  holder: string
  tgl: string
  harga: number
  buku: number
}

export function useAssets() {
  async function list(query: ListQuery = {}): Promise<Paginated<Asset>> {
    await fakeLatency(700)
    return paginate(filterBy(assetStore.all(), query, ['nama', 'tag', 'brand']), query)
  }

  async function get(tag: string): Promise<Asset | undefined> {
    await fakeLatency()
    return assetStore.find(tag)
  }

  async function create(input: AssetInput): Promise<Asset> {
    await fakeLatency()
    return assetStore.insert({ ...input })
  }

  async function update(tag: string, input: Partial<AssetInput>): Promise<Asset> {
    await fakeLatency()
    const row = assetStore.update(tag, input)
    if (!row) throw new Error('assets.errNotFound')
    return row
  }

  async function remove(tag: string): Promise<void> {
    await fakeLatency()
    assetStore.remove(tag)
  }

  return { list, get, create, update, remove }
}
