import { describe, it, expect, beforeEach } from 'vitest'
import { useAssets } from '~/composables/api/useAssets'
import { assetSeed, assetStore, ASSET_STATUS_KEYS, ASSET_CATEGORIES } from '~/mock/assets'

const { list, get, create, update, remove } = useAssets()

beforeEach(() => assetStore.reset())

describe('mock/assets', () => {
  it('seeds 26 assets, 5 statuses and 4 categories', () => {
    expect(assetSeed).toHaveLength(26)
    expect(ASSET_STATUS_KEYS).toHaveLength(5)
    expect(ASSET_CATEGORIES).toContain('Perangkat IT')
  })
})

describe('useAssets', () => {
  it('lists all assets and filters by name/tag/brand', async () => {
    expect((await list({ limit: 100 })).total).toBe(26)
    const byName = await list({ search: 'Toyota', limit: 100 })
    expect(byName.data.every(a => /toyota/i.test(a.nama) || /toyota/i.test(a.brand))).toBe(true)
    expect(byName.data.length).toBeGreaterThan(0)
    const byTag = await list({ search: 'KEN-2026-00002', limit: 100 })
    expect(byTag.data).toHaveLength(1)
    expect(byTag.data[0].nama).toBe('Toyota Hiace Commuter')
  })

  it('gets an asset by tag', async () => {
    const asset = await get('JKT01-ELK-2026-00001')
    expect(asset?.nama).toBe('Laptop Dell Latitude 5440')
  })

  it('creates, updates and removes', async () => {
    await create({ tag: 'NEW-1', nama: 'Test Asset', kategori: 'Elektronik', brand: 'X', status: 'tersedia', kantor: 'Outlet Blok M', lokasi: 'Lobi', holder: '—', tgl: '2026-06-01', harga: 1000, buku: 900 })
    expect(assetStore.find('NEW-1')?.nama).toBe('Test Asset')
    await update('NEW-1', { status: 'maintenance' })
    expect(assetStore.find('NEW-1')?.status).toBe('maintenance')
    await remove('NEW-1')
    expect(assetStore.find('NEW-1')).toBeUndefined()
  })

  it('throws the sentinel error updating a missing asset', async () => {
    await expect(update('nope', { nama: 'x' })).rejects.toThrow('assets.errNotFound')
  })
})
